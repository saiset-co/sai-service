package config

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

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

type ConfigurationManager struct {
	ctx             context.Context
	cancel          context.CancelFunc
	config          atomic.Pointer[types.ServiceConfig]
	rawData         atomic.Pointer[map[string]interface{}]
	configPath      string
	loader          *Loader
	parser          atomic.Pointer[Parser]
	state           atomic.Value
	mu              sync.RWMutex
	shutdownTimeout time.Duration
	loadTimeout     time.Duration
}

func NewConfigurationManager(
	ctx context.Context,
	configPath string,
) (*ConfigurationManager, error) {
	managerCtx, cancel := context.WithCancel(ctx)

	cm := &ConfigurationManager{
		ctx:             managerCtx,
		cancel:          cancel,
		configPath:      configPath,
		shutdownTimeout: 10 * time.Second,
		loadTimeout:     30 * time.Second,
	}

	cm.state.Store(StateStopped)

	loader, err := NewLoader()
	if err != nil {
		cancel()
		return nil, types.WrapError(err, "failed to create loader")
	}
	cm.loader = loader

	if err := cm.Load(); err != nil {
		cancel()
		return nil, types.WrapError(err, "failed to load initial configuration")
	}

	return cm, nil
}

func (cm *ConfigurationManager) Start() error {
	if !cm.transitionState(StateStopped, StateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if cm.getState() == StateStarting {
			cm.setState(StateRunning)
		}
	}()

	return nil
}

func (cm *ConfigurationManager) Stop() error {
	if !cm.transitionState(StateRunning, StateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		cm.setState(StateStopped)
		cm.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), cm.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			cm.mu.Lock()
			defer cm.mu.Unlock()

			cm.config.Store(nil)
			cm.parser.Store(nil)
			cm.rawData.Store(nil)
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
		default:

		}
	}

	return nil
}

func (cm *ConfigurationManager) IsRunning() bool {
	return cm.getState() == StateRunning
}

func (cm *ConfigurationManager) Load() error {
	loadCtx, cancel := context.WithTimeout(cm.ctx, cm.loadTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(loadCtx)

	var config *types.ServiceConfig
	var rawData *map[string]interface{}
	var parser *Parser
	var loadErr error

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			var err error
			config, rawData, err = cm.loader.LoadFromFile(loadCtx, cm.configPath)
			if err != nil {
				loadErr = types.WrapError(err, "failed to load configuration from file")
				return loadErr
			}
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-loadCtx.Done():
			return types.WrapError(loadCtx.Err(), "configuration load timeout")
		default:
			if loadErr != nil {
				return loadErr
			}
			return types.WrapError(err, "failed to load configuration")
		}
	}

	parser = NewParser(rawData)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.config.Store(config)
	cm.parser.Store(parser)
	cm.rawData.Store(rawData)

	return nil
}

func (cm *ConfigurationManager) GetConfig() *types.ServiceConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if config := cm.config.Load(); config != nil {
		return config
	}
	return nil
}

func (cm *ConfigurationManager) GetValue(path string, defaultValue interface{}) interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	parser := cm.parser.Load()
	if parser == nil {
		return defaultValue
	}
	return (*parser).GetValue(path, defaultValue)
}

func (cm *ConfigurationManager) GetAs(path string, target interface{}) error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	parser := cm.parser.Load()
	if parser == nil {
		return types.ErrActionNotInitialized
	}
	return (*parser).GetAs(path, target)
}

func (cm *ConfigurationManager) GetRawData() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	rawData := cm.rawData.Load()
	if rawData == nil {
		return make(map[string]interface{})
	}

	result := make(map[string]interface{})
	for k, v := range *rawData {
		result[k] = v
	}
	return result
}

func (cm *ConfigurationManager) GetAllPaths() ([]string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	parser := cm.parser.Load()
	if parser == nil {
		return nil, types.ErrActionNotInitialized
	}

	return (*parser).GetAllPaths()
}

func (cm *ConfigurationManager) getState() State {
	return cm.state.Load().(State)
}

func (cm *ConfigurationManager) setState(newState State) bool {
	currentState := cm.getState()
	return cm.state.CompareAndSwap(currentState, newState)
}

func (cm *ConfigurationManager) transitionState(from, to State) bool {
	return cm.state.CompareAndSwap(from, to)
}
