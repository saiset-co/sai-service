package client

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/auth_providers"
	"github.com/saiset-co/sai-service/types"
)

type ManagerState int32

const (
	ManagerStateStopped ManagerState = iota
	ManagerStateStarting
	ManagerStateRunning
	ManagerStateStopping
)

type HTTPClientConfig struct {
	DefaultTimeout     time.Duration         `yaml:"default_timeout" json:"default_timeout"`
	MaxIdleConnections int                   `yaml:"max_idle_connections" json:"max_idle_connections"`
	IdleConnTimeout    time.Duration         `yaml:"idle_conn_timeout" json:"idle_conn_timeout"`
	DefaultRetries     int                   `yaml:"default_retries" json:"default_retries"`
	CircuitBreaker     *CircuitBreakerConfig `yaml:"circuit_breaker" json:"circuit_breaker"`
}

type Manager struct {
	ctx               context.Context
	cancel            context.CancelFunc
	config            types.ConfigManager
	logger            types.Logger
	metrics           types.MetricsManager
	health            types.HealthManager
	middlewareManager types.MiddlewareManager
	authProvider      types.AuthProviderManager
	clients           map[string]*HTTPClient
	clientConfig      *HTTPClientConfig
	serviceConfigs    map[string]*types.ServiceClientConfig
	mu                sync.RWMutex
	state             atomic.Value
	shutdownTimeout   time.Duration
	callTimeout       time.Duration
}

func NewManager(ctx context.Context, config types.ConfigManager, logger types.Logger, metrics types.MetricsManager, health types.HealthManager, middlewareManager types.MiddlewareManager, authProvider types.AuthProviderManager) (types.ClientManager, error) {
	clientConfig := &HTTPClientConfig{
		DefaultTimeout:     30 * time.Second,
		MaxIdleConnections: 100,
		IdleConnTimeout:    90 * time.Second,
		DefaultRetries:     3,
		CircuitBreaker: &CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 5,
			RecoveryTimeout:  60 * time.Second,
			HalfOpenRequests: 3,
		},
	}

	managerCtx, cancel := context.WithCancel(ctx)

	manager := &Manager{
		ctx:               managerCtx,
		cancel:            cancel,
		config:            config,
		logger:            logger,
		metrics:           metrics,
		health:            health,
		middlewareManager: middlewareManager,
		authProvider:      authProvider,
		clients:           make(map[string]*HTTPClient),
		clientConfig:      clientConfig,
		serviceConfigs:    make(map[string]*types.ServiceClientConfig),
		shutdownTimeout:   10 * time.Second,
		callTimeout:       30 * time.Second,
	}

	manager.state.Store(ManagerStateStopped)

	return manager, nil
}

func (m *Manager) Start() error {
	if !m.transitionState(ManagerStateStopped, ManagerStateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if m.getState() == ManagerStateStarting {
			m.setState(ManagerStateRunning)
		}
	}()

	if err := m.initializeClients(); err != nil {
		m.setState(ManagerStateStopped)
		return types.WrapError(err, "failed to initialize HTTP clients")
	}

	m.logger.Info("Client manager started")
	return nil
}

func (m *Manager) Stop() error {
	if !m.transitionState(ManagerStateRunning, ManagerStateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		m.setState(ManagerStateStopped)
		m.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), m.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	m.mu.RLock()
	clients := make([]*HTTPClient, 0, len(m.clients))
	for _, client := range m.clients {
		clients = append(clients, client)
	}
	m.mu.RUnlock()

	for _, client := range clients {
		c := client
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				c.Close()
				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			m.logger.Warn("Client manager stop timeout, some clients may not have stopped gracefully")
		default:
			m.logger.Error("Error during client manager shutdown", zap.Error(err))
		}
	} else {
		m.logger.Info("Client manager stopped gracefully",
			zap.Int("clients_closed", len(clients)))
	}

	return nil
}

func (m *Manager) IsRunning() bool {
	return m.getState() == ManagerStateRunning
}

func (m *Manager) RegisterWebhook(serviceName, event, webhookURL string) ([]byte, int, error) {
	if !m.IsRunning() {
		return nil, 500, types.ErrActionNotInitialized
	}

	start := time.Now()
	defer func() {
		m.recordMetric("webhook_register", "attempt", serviceName, time.Since(start))
	}()

	webhookData := map[string]interface{}{
		"event": event,
		"url":   webhookURL,
	}

	opts := &types.CallOptions{
		Headers: make(map[string]string),
		Timeout: 30 * time.Second,
		Retry:   3,
	}

	resp, statusCode, err := m.Call(serviceName, "POST", "/api/webhooks", webhookData, opts)

	result := "success"
	if err != nil {
		result = "error"
	}
	m.recordMetric("webhook_register", result, serviceName, time.Since(start))

	return resp, statusCode, err
}

func (m *Manager) Call(serviceName, method, path string, data interface{}, opts *types.CallOptions) ([]byte, int, error) {
	if !m.IsRunning() {
		return nil, 500, types.ErrActionNotInitialized
	}

	start := time.Now()
	defer func() {
		m.recordMetric("call", "attempt", serviceName, time.Since(start))
	}()

	callCtx, cancel := context.WithTimeout(m.ctx, m.callTimeout)
	defer cancel()

	if opts == nil {
		opts = &types.CallOptions{
			Headers: make(map[string]string),
		}
	}

	client, err := m.getClient(serviceName)
	if err != nil {
		m.recordMetric("call", "client_error", serviceName, time.Since(start))
		return nil, 500, err
	}

	serviceConfig, err := m.getServiceConfig(serviceName)
	if err != nil {
		m.recordMetric("call", "config_error", serviceName, time.Since(start))
		return nil, 500, err
	}

	if serviceConfig.Auth != nil {
		if err := m.addAuthenticationHeaders(opts, serviceConfig.Auth); err != nil {
			m.recordMetric("call", "auth_error", serviceName, time.Since(start))
			return nil, 500, types.WrapError(err, "failed to add authentication headers")
		}
	}

	var resp []byte
	var statusCode int

	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, statusCode, err = client.Call(method, path, data, opts)
	}()

	select {
	case <-done:
	case <-callCtx.Done():
		err = types.NewErrorf("call timeout for service: %s", serviceName)
		statusCode = 500
		m.recordMetric("call", "timeout", serviceName, time.Since(start))
		return nil, statusCode, err
	case <-m.ctx.Done():
		err = types.NewErrorf("manager shutting down, aborting call to service: %s", serviceName)
		statusCode = 500
		m.recordMetric("call", "canceled", serviceName, time.Since(start))
		return nil, statusCode, err
	}

	duration := time.Since(start)
	status := "success"
	if err != nil {
		status = "error"
	}

	m.recordMetrics(serviceName, method, status, 0, duration)
	m.updateCircuitBreakerMetrics(serviceName, client)

	result := "success"
	if err != nil {
		result = "error"
	}
	m.recordMetric("call", result, serviceName, time.Since(start))

	return resp, statusCode, err
}

func (m *Manager) getState() ManagerState {
	return m.state.Load().(ManagerState)
}

func (m *Manager) setState(newState ManagerState) bool {
	currentState := m.getState()
	return m.state.CompareAndSwap(currentState, newState)
}

func (m *Manager) transitionState(from, to ManagerState) bool {
	return m.state.CompareAndSwap(from, to)
}

func (m *Manager) addAuthenticationHeaders(opts *types.CallOptions, authConfig *types.ServiceAuthConfig) error {
	if m.authProvider == nil {
		return types.NewErrorf("auth provider manager not available")
	}

	provider, err := m.authProvider.(*auth_providers.AuthProviderManager).GetProvider(authConfig.Provider)
	if err != nil {
		return types.WrapError(err, "failed to get auth provider")
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	if err := provider.ApplyToOutgoingRequest(req, authConfig); err != nil {
		return types.WrapError(err, "failed to apply authentication to request")
	}

	req.Header.VisitAll(func(key, value []byte) {
		opts.Headers[string(key)] = string(value)
	})

	return nil
}

func (m *Manager) initializeClients() error {
	clientConfig := m.config.GetConfig().Client
	if clientConfig == nil || !clientConfig.Enabled {
		m.logger.Info("Client configuration disabled or not found")
		return nil
	}

	services := clientConfig.Services
	if services == nil {
		m.logger.Info("No services configured in clients.services")
		return nil
	}

	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	for serviceName, serviceConfig := range services {
		name := serviceName
		config := serviceConfig

		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				httpClientConfig := &ServiceClientConfig{
					BaseURL:        config.Url,
					Timeout:        m.clientConfig.DefaultTimeout,
					Retries:        m.clientConfig.DefaultRetries,
					CircuitBreaker: m.clientConfig.CircuitBreaker,
				}

				client := NewHTTPClient(m.ctx, m.logger, name, httpClientConfig)

				m.clients[name] = client
				m.serviceConfigs[name] = config

				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			return types.NewErrorf("client initialization timeout")
		default:
			return types.WrapError(err, "failed to initialize clients")
		}
	}

	m.logger.Info("All HTTP clients initialized successfully",
		zap.Int("client_count", len(m.clients)))

	return nil
}

func (m *Manager) getServiceConfig(serviceName string) (*types.ServiceClientConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, exists := m.serviceConfigs[serviceName]
	if !exists {
		return nil, types.Errorf(types.ErrClientNotFound, "service config not found: %s", serviceName)
	}

	return config, nil
}

func (m *Manager) getClient(serviceName string) (*HTTPClient, error) {
	m.mu.RLock()
	client, exists := m.clients[serviceName]
	m.mu.RUnlock()

	if !exists {
		return nil, types.Errorf(types.ErrClientNotFound, "service: %s", serviceName)
	}

	return client, nil
}

func (m *Manager) recordMetrics(serviceName, method, status string, responseSize int64, duration time.Duration) {
	if m.metrics == nil {
		return
	}

	requestCounter := m.metrics.Counter("http_client_requests_total", map[string]string{
		"service": serviceName,
		"method":  method,
		"status":  status,
	})
	requestCounter.Inc()

	durationHist := m.metrics.Histogram("http_client_request_duration_seconds",
		[]float64{0.01, 0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0},
		map[string]string{"service": serviceName, "method": method},
	)
	durationHist.Observe(duration.Seconds())

	if responseSize > 0 {
		sizeHist := m.metrics.Histogram("http_client_response_size_bytes",
			[]float64{100, 1000, 10000, 100000, 1000000},
			map[string]string{"service": serviceName, "method": method},
		)
		sizeHist.Observe(float64(responseSize))
	}

	m.logger.Debug("HTTP client metrics recorded",
		zap.String("service", serviceName),
		zap.String("method", method),
		zap.String("status", status),
		zap.Duration("duration", duration),
		zap.Int64("response_size", responseSize))
}

func (m *Manager) recordMetric(operation, result, service string, duration time.Duration) {
	if m.metrics == nil {
		return
	}

	counter := m.metrics.Counter("client_operations_total", map[string]string{
		"operation": operation,
		"result":    result,
		"service":   service,
	})
	counter.Inc()

	histogram := m.metrics.Histogram("client_operation_duration_seconds",
		[]float64{0.001, 0.01, 0.1, 1.0, 5.0, 10.0, 30.0},
		map[string]string{"operation": operation, "service": service},
	)
	histogram.Observe(duration.Seconds())
}

func (m *Manager) updateCircuitBreakerMetrics(serviceName string, client *HTTPClient) {
	if m.metrics == nil {
		return
	}

	state, _, _ := client.getState()

	states := []string{"closed", "open", "half-open"}
	for _, s := range states {
		stateGauge := m.metrics.Gauge("http_client_circuit_breaker_status", map[string]string{
			"service": serviceName,
			"state":   s,
		})
		stateGauge.Set(0)
	}

	currentState := "closed"
	switch state {
	case 0:
		currentState = "closed"
	case 1:
		currentState = "open"
	case 2:
		currentState = "half-open"
	}

	currentStateGauge := m.metrics.Gauge("http_client_circuit_breaker_status", map[string]string{
		"service": serviceName,
		"state":   currentState,
	})
	currentStateGauge.Set(1)
}
