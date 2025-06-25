package middleware

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
)

const (
	MaxMiddlewares = 64
)

type State int32

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
)

type Manager struct {
	ctx                context.Context
	cancel             context.CancelFunc
	config             types.ConfigManager
	logger             types.Logger
	metrics            types.MetricsManager
	cache              types.CacheManager
	health             types.HealthManager
	authProvider       types.AuthProviderManager
	compiledChain      func(*types.RequestCtx, func(*types.RequestCtx), *types.RouteConfig)
	orderedMiddlewares []types.MiddlewareEntry
	middlewareMap      map[string]*types.MiddlewareEntry
	mu                 sync.RWMutex
	state              int32
	shutdownTimeout    time.Duration
}

func NewManager(ctx context.Context, config types.ConfigManager, logger types.Logger, metrics types.MetricsManager, cache types.CacheManager, health types.HealthManager, authProvider types.AuthProviderManager) (*Manager, error) {
	managerCtx, cancel := context.WithCancel(ctx)

	manager := &Manager{
		ctx:             managerCtx,
		cancel:          cancel,
		config:          config,
		logger:          logger,
		metrics:         metrics,
		cache:           cache,
		health:          health,
		authProvider:    authProvider,
		middlewareMap:   make(map[string]*types.MiddlewareEntry),
		shutdownTimeout: 10 * time.Second,
	}

	return manager, nil
}

func (m *Manager) Start() error {
	if !m.transitionState(StateStopped, StateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if m.getState() == StateStarting {
			m.setState(StateRunning)
		}
	}()

	if err := m.registerMiddlewares(); err != nil {
		m.setState(StateStopped)
		return types.WrapError(err, "failed to register middlewares")
	}

	m.logger.Info("Middleware manager started")
	return nil
}

func (m *Manager) Stop() error {
	if !m.transitionState(StateRunning, StateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		m.setState(StateStopped)
		m.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), m.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		m.clearResources()
		return nil
	})

	if err := g.Wait(); err != nil {
		select {
		case <-gCtx.Done():
			m.logger.Warn("Middleware manager stop timeout, some components may not have stopped gracefully")
		default:
			m.logger.Error("Error during middleware manager shutdown", zap.Error(err))
		}
	} else {
		m.logger.Info("Middleware manager stopped gracefully")
	}

	return nil
}

func (m *Manager) IsRunning() bool {
	return m.getState() == StateRunning
}

func (m *Manager) getState() State {
	return State(atomic.LoadInt32(&m.state))
}

func (m *Manager) setState(newState State) bool {
	atomic.StoreInt32(&m.state, int32(newState))
	return true
}

func (m *Manager) transitionState(from, to State) bool {
	return atomic.CompareAndSwapInt32(&m.state, int32(from), int32(to))
}

func (m *Manager) registerMiddlewares() error {
	config := m.config.GetConfig()

	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	middlewares := []struct {
		name       string
		enabled    bool
		createFunc func() (types.Middleware, error)
	}{
		{"recovery", config.Middlewares.Recovery.Enabled, func() (types.Middleware, error) {
			return NewRecoveryMiddleware(m.config, m.logger, m.metrics), nil
		}},
		{"logging", config.Middlewares.Logging.Enabled, func() (types.Middleware, error) {
			return NewLoggingMiddleware(m.config, m.logger, m.metrics), nil
		}},
		{"rate-limit", config.Middlewares.RateLimit.Enabled, func() (types.Middleware, error) {
			return NewRateLimitMiddleware(m.ctx, m.config, m.logger, m.metrics), nil
		}},
		{"compression", config.Middlewares.Compression.Enabled, func() (types.Middleware, error) {
			return NewCompressionMiddleware(m.config, m.logger, m.metrics), nil
		}},
		{"body-limit", config.Middlewares.BodyLimit.Enabled, func() (types.Middleware, error) {
			return NewBodyLimitMiddleware(m.config, m.logger, m.metrics), nil
		}},
		{"cache", config.Middlewares.Cache.Enabled && m.cache != nil, func() (types.Middleware, error) {
			return NewCacheMiddleware(m.config, m.logger, m.metrics, m.cache), nil
		}},
		{"auth", config.Middlewares.Auth.Enabled, func() (types.Middleware, error) {
			return NewAuthMiddleware(m.authProvider, m.config, m.logger, m.metrics)
		}},
		{"cors", config.Middlewares.CORS.Enabled, func() (types.Middleware, error) {
			return NewCORSMiddleware(m.config, m.logger, m.metrics), nil
		}},
	}

	middlewareResults := make(chan struct {
		middleware types.Middleware
		name       string
		err        error
	}, len(middlewares))

	for _, mw := range middlewares {
		if !mw.enabled {
			continue
		}

		mw := mw
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				middleware, err := mw.createFunc()
				middlewareResults <- struct {
					middleware types.Middleware
					name       string
					err        error
				}{middleware, mw.name, err}
				return err
			}
		})
	}

	go func() {
		_ = g.Wait()
		close(middlewareResults)
	}()

	var registrationErrors []error
	for result := range middlewareResults {
		if result.err != nil {
			registrationErrors = append(registrationErrors, result.err)
			continue
		}

		if err := m.Register(result.middleware); err != nil {
			registrationErrors = append(registrationErrors, err)
			continue
		}

		m.logger.Info("Middleware registered", zap.String("name", result.name))
	}

	if len(registrationErrors) > 0 {
		return types.NewErrorf("failed to register %d middlewares", len(registrationErrors))
	}

	return m.finalizeConfiguration()
}

func (m *Manager) Register(middleware types.Middleware) error {
	if middleware == nil {
		return types.ErrMiddlewareInvalidType
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.middlewareMap) >= MaxMiddlewares {
		return types.NewErrorf("maximum middleware count exceeded: %d", MaxMiddlewares)
	}

	name := middleware.Name()

	entry := &types.MiddlewareEntry{
		Name:       name,
		Middleware: middleware,
		Weight:     middleware.Weight(),
	}

	m.middlewareMap[name] = entry
	return nil
}

func (m *Manager) finalizeConfiguration() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	weights := make(map[int]string)
	for name, entry := range m.middlewareMap {
		if existingName, exists := weights[entry.Weight]; exists {
			return types.NewErrorf("duplicate weight %d for middlewares '%s' and '%s'",
				entry.Weight, existingName, name)
		}
		weights[entry.Weight] = name
	}

	m.orderedMiddlewares = make([]types.MiddlewareEntry, 0, len(m.middlewareMap))
	for _, entry := range m.middlewareMap {
		m.orderedMiddlewares = append(m.orderedMiddlewares, types.MiddlewareEntry{
			Name:       entry.Name,
			Middleware: entry.Middleware,
			Weight:     entry.Weight,
		})
	}

	sort.Slice(m.orderedMiddlewares, func(i, j int) bool {
		return m.orderedMiddlewares[i].Weight < m.orderedMiddlewares[j].Weight
	})

	m.compiledChain = m.compileChain(m.orderedMiddlewares)
	m.middlewareMap = nil

	return nil
}

func (m *Manager) Execute(ctx *types.RequestCtx, handler func(*types.RequestCtx), config *types.RouteConfig) {
	if m.compiledChain != nil {
		m.compiledChain(ctx, handler, config)
	} else {
		handler(ctx)
	}
}

func (m *Manager) compileChain(middlewareEntries []types.MiddlewareEntry) func(*types.RequestCtx, func(*types.RequestCtx), *types.RouteConfig) {
	if len(middlewareEntries) == 0 {
		return func(ctx *types.RequestCtx, handler func(*types.RequestCtx), config *types.RouteConfig) {
			handler(ctx)
		}
	}

	middlewares := make([]types.Middleware, len(middlewareEntries))
	for i, entry := range middlewareEntries {
		middlewares[i] = entry.Middleware
	}

	return func(ctx *types.RequestCtx, handler func(*types.RequestCtx), config *types.RouteConfig) {
		var index int

		var next func(*types.RequestCtx)
		next = func(ctx *types.RequestCtx) {
			if index >= len(middlewares) {
				handler(ctx)
				return
			}

			mw := middlewares[index]
			index++
			mw.Handle(ctx, next, config)
		}

		next(ctx)
	}
}

func (m *Manager) clearResources() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.orderedMiddlewares = nil
	m.compiledChain = nil
}
