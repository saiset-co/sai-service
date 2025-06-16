package client

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
)

type HTTPClientConfig struct {
	DefaultTimeout     time.Duration         `yaml:"default_timeout" json:"default_timeout"`
	MaxIdleConnections int                   `yaml:"max_idle_connections" json:"max_idle_connections"`
	IdleConnTimeout    time.Duration         `yaml:"idle_conn_timeout" json:"idle_conn_timeout"`
	DefaultRetries     int                   `yaml:"default_retries" json:"default_retries"`
	CircuitBreaker     *CircuitBreakerConfig `yaml:"circuit_breaker" json:"circuit_breaker"`
}

type Manager struct {
	ctx                  context.Context
	config               types.ConfigManager
	logger               types.Logger
	metrics              types.MetricsManager
	health               types.HealthManager
	clients              map[string]*HTTPClient
	clientConfig         *HTTPClientConfig
	mu                   sync.RWMutex
	running              int32
	requestsTotal        types.Counter
	requestDuration      types.Histogram
	requestsInFlight     types.Gauge
	responseSize         types.Histogram
	circuitBreakerStatus types.Gauge
}

func NewManager(ctx context.Context, config types.ConfigManager, logger types.Logger, metrics types.MetricsManager, health types.HealthManager) (types.ClientManager, error) {
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

	manager := &Manager{
		ctx:          ctx,
		config:       config,
		logger:       logger,
		metrics:      metrics,
		health:       health,
		clients:      make(map[string]*HTTPClient),
		clientConfig: clientConfig,
		running:      0,
	}

	manager.initMetrics()

	if err := manager.initializeClients(ctx, logger); err != nil {
		logger.Error("Failed to initialize HTTP clients", zap.Error(err))
		return nil, err
	}

	return manager, nil
}

func (m *Manager) initializeClients(ctx context.Context, logger types.Logger) error {
	services := m.config.GetConfig().Services

	for serviceName, serviceAddr := range services {
		baseURL := fmt.Sprintf("https://%s:%d", serviceAddr.Host, serviceAddr.Port)

		clientConfig := &ServiceClientConfig{
			BaseURL:        baseURL,
			Timeout:        m.clientConfig.DefaultTimeout,
			Retries:        m.clientConfig.DefaultRetries,
			CircuitBreaker: m.clientConfig.CircuitBreaker,
		}

		client := NewHTTPClient(ctx, logger, serviceName, clientConfig)

		if err := m.healthCheckService(client); err != nil {
			m.logger.Warn("Service health check failed during initialization",
				zap.String("service", serviceName),
				zap.Error(err))
		}

		m.clients[serviceName] = client

		m.logger.Info("HTTP client created",
			zap.String("service", serviceName),
			zap.String("base_url", baseURL))
	}

	return nil
}

func (m *Manager) healthCheckService(client *HTTPClient) error {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(client.baseURL + "/health")
	req.Header.SetMethod("GET")

	err := client.client.DoTimeout(req, resp, 5*time.Second)
	if err != nil {
		return err
	}

	if resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
		return nil
	}

	return types.Errorf(types.ErrHealthCheckFailed, "status: %d", resp.StatusCode())
}

func (m *Manager) Start() error {
	if !atomic.CompareAndSwapInt32(&m.running, 0, 1) {
		m.logger.Warn("Client manager is already running")
		return types.ErrServerAlreadyRunning
	}

	m.logger.Info("Client manager started")
	return nil
}

func (m *Manager) Stop() error {
	if !atomic.CompareAndSwapInt32(&m.running, 1, 0) {
		m.logger.Warn("Client manager is not running")
		return types.ErrServerNotRunning
	}

	m.mu.RLock()
	clients := make([]*HTTPClient, 0, len(m.clients))
	for _, client := range m.clients {
		clients = append(clients, client)
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup
	for _, client := range clients {
		wg.Add(1)
		go func(c *HTTPClient) {
			defer wg.Done()
			c.Close()
		}(client)
	}
	wg.Wait()

	m.logger.Info("Client manager stopped",
		zap.Int("clients_closed", len(clients)))

	return nil
}

func (m *Manager) IsRunning() bool {
	return atomic.LoadInt32(&m.running) == 1
}

func (m *Manager) GetClient(serviceName string) (types.HttpClient, error) {
	m.mu.RLock()
	client, exists := m.clients[serviceName]
	m.mu.RUnlock()

	if !exists {
		return nil, types.Errorf(types.ErrClientNotFound, "service: %s", serviceName)
	}

	return client, nil
}

func (m *Manager) Call(serviceName, method, path string, data interface{}, opts types.CallOptions) error {
	start := time.Now()
	inflightGauge := m.metrics.Gauge("http_client_requests_in_flight", map[string]string{
		"service": serviceName,
	})
	inflightGauge.Inc()
	defer inflightGauge.Dec()

	client, err := m.GetClient(serviceName)
	if err != nil {
		return err
	}

	err = client.Call(method, path, data, opts)

	duration := time.Since(start)
	status := "200"
	if err != nil {
		status = "error"
	}

	m.recordMetrics(serviceName, method, status, 0, duration)
	m.updateCircuitBreakerMetrics(serviceName, client)

	return err
}

func (m *Manager) Get(serviceName, path string, opts types.CallOptions) (map[string]interface{}, error) {
	client, err := m.GetClient(serviceName)
	if err != nil {
		return nil, err
	}

	if err := client.Call("GET", path, nil, opts); err != nil {
		return nil, err
	}

	return map[string]interface{}{"success": true}, nil
}

func (m *Manager) Post(serviceName, path string, data interface{}, opts types.CallOptions) (map[string]interface{}, error) {
	client, err := m.GetClient(serviceName)
	if err != nil {
		return nil, err
	}

	if err := client.Call("POST", path, data, opts); err != nil {
		return nil, err
	}

	return map[string]interface{}{"success": true}, nil
}

func (m *Manager) initMetrics() {
	metrics := m.metrics

	m.requestsTotal = metrics.Counter("http_client_requests_total", map[string]string{
		"service": "",
		"method":  "",
		"status":  "",
	})

	m.requestDuration = metrics.Histogram("http_client_request_duration_seconds",
		[]float64{0.01, 0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0},
		map[string]string{"service": "", "method": ""},
	)

	m.requestsInFlight = metrics.Gauge("http_client_requests_in_flight", map[string]string{
		"service": "",
	})

	m.responseSize = metrics.Histogram("http_client_response_size_bytes",
		[]float64{100, 1000, 10000, 100000, 1000000},
		map[string]string{"service": "", "method": ""},
	)

	m.circuitBreakerStatus = metrics.Gauge("http_client_circuit_breaker_status", map[string]string{
		"service": "",
		"state":   "",
	})
}

func (m *Manager) recordMetrics(serviceName, method, status string, responseSize int64, duration time.Duration) {
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

func (m *Manager) updateCircuitBreakerMetrics(serviceName string, client types.HttpClient) {
	state, _, _ := client.GetState()

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
