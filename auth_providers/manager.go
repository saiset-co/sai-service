package auth_providers

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
)

type State int32

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
)

type AuthProviderManager struct {
	ctx             context.Context
	cancel          context.CancelFunc
	config          types.ConfigManager
	logger          types.Logger
	mu              sync.RWMutex
	providers       map[string]types.AuthProvider
	state           atomic.Value
	shutdownTimeout time.Duration
}

func NewAuthProviderManager(
	ctx context.Context,
	config types.ConfigManager,
	logger types.Logger,
) (types.AuthProviderManager, error) {
	managerCtx, cancel := context.WithCancel(ctx)

	manager := &AuthProviderManager{
		ctx:             managerCtx,
		cancel:          cancel,
		config:          config,
		logger:          logger,
		providers:       make(map[string]types.AuthProvider),
		shutdownTimeout: 10 * time.Second,
	}

	manager.state.Store(StateStopped)

	if err := manager.initializeDefaultProviders(); err != nil {
		cancel()
		return nil, types.WrapError(err, "failed to start auth provider")
	}

	return manager, nil
}

func (pm *AuthProviderManager) Start() error {
	if !pm.transitionState(StateStopped, StateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if pm.getState() == StateStarting {
			pm.setState(StateRunning)
		}
	}()

	ctx, cancel := context.WithTimeout(pm.ctx, 30*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	pm.mu.RLock()
	for name, provider := range pm.providers {
		name, provider := name, provider
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if startable, ok := provider.(interface{ Start() error }); ok {
					if err := startable.Start(); err != nil {
						pm.logger.Error("Failed to start auth provider",
							zap.String("provider", name),
							zap.Error(err))
						return types.WrapError(err, "failed to start provider "+name)
					}
				}
				return nil
			}
		})
	}
	pm.mu.RUnlock()

	if err := g.Wait(); err != nil {
		pm.setState(StateStopped)
		select {
		case <-ctx.Done():
			pm.logger.Error("Auth provider manager start timeout")
			return types.NewErrorf("start timeout")
		default:
			return types.WrapError(err, "failed to start some auth providers")
		}
	}

	pm.logger.Info("Provider manager started")
	return nil
}

func (pm *AuthProviderManager) Stop() error {
	if !pm.transitionState(StateRunning, StateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		pm.setState(StateStopped)
		pm.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), pm.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	pm.mu.RLock()
	for name, provider := range pm.providers {
		name, provider := name, provider
		g.Go(func() error {
			if stoppable, ok := provider.(interface{ Stop() error }); ok {
				if err := stoppable.Stop(); err != nil {
					pm.logger.Error("Failed to stop auth provider",
						zap.String("provider", name),
						zap.Error(err))
					return err
				}
			}
			return nil
		})
	}
	pm.mu.RUnlock()

	if err := g.Wait(); err != nil {
		select {
		case <-gCtx.Done():
			pm.logger.Warn("Auth provider manager stop timeout, some providers may not have stopped gracefully")
		default:
			pm.logger.Error("Error during auth provider manager shutdown", zap.Error(err))
		}
	}

	return nil
}

func (pm *AuthProviderManager) IsRunning() bool {
	return pm.getState() == StateRunning
}

func (pm *AuthProviderManager) GetProvider(name string) (types.AuthProvider, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if provider, ok := pm.providers[name]; ok {
		return provider, nil
	}
	return nil, types.NewErrorf("provider %s not found", name)
}

func (pm *AuthProviderManager) Register(name string, provider types.AuthProvider) error {
	if pm.IsRunning() {
		pm.logger.Warn("Provider manager is already running")
		return types.ErrServerAlreadyRunning
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.providers[name]; ok {
		return types.NewErrorf("provider %s already registered", name)
	}

	pm.providers[name] = provider
	return nil
}

func (pm *AuthProviderManager) getState() State {
	return pm.state.Load().(State)
}

func (pm *AuthProviderManager) setState(newState State) bool {
	currentState := pm.getState()
	return pm.state.CompareAndSwap(currentState, newState)
}

func (pm *AuthProviderManager) transitionState(from, to State) bool {
	return pm.state.CompareAndSwap(from, to)
}

func (pm *AuthProviderManager) initializeDefaultProviders() error {
	if pm.IsRunning() {
		pm.logger.Warn("Provider manager is already running")
		return types.ErrServerAlreadyRunning
	}

	providersConfig := pm.config.GetConfig().AuthProviders
	if providersConfig == nil {
		pm.logger.Warn("No providers enabled")
		return nil
	}

	if token, ok := providersConfig.Token.Params["token"].(string); ok {
		err := pm.Register("token", NewTokenAuthProvider(token))
		if err != nil {
			return err
		}
	}

	if username, ok := providersConfig.Basic.Params["username"].(string); ok {
		if password, ok := providersConfig.Basic.Params["password"].(string); ok {
			err := pm.Register("basic", NewBasicAuthProvider(username, password))
			if err != nil {
				return err
			}
		}
	}

	return nil
}
