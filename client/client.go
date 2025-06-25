package client

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type State int32

const (
	StateRunning State = iota
	StateStopping
	StateStopped
)

type HTTPClient struct {
	ctx             context.Context
	cancel          context.CancelFunc
	logger          types.Logger
	name            string
	client          *fasthttp.Client
	baseURL         string
	config          *ServiceClientConfig
	circuitBreaker  *CircuitBreaker
	state           atomic.Value
	shutdownTimeout time.Duration
	requestTimeout  time.Duration
}

type ServiceClientConfig struct {
	BaseURL        string                `yaml:"base_url" json:"base_url"`
	Timeout        time.Duration         `yaml:"timeout" json:"timeout"`
	Retries        int                   `yaml:"retries" json:"retries"`
	CircuitBreaker *CircuitBreakerConfig `yaml:"circuit_breaker" json:"circuit_breaker"`
}

func NewHTTPClient(ctx context.Context, logger types.Logger, serviceName string, config *ServiceClientConfig) *HTTPClient {
	clientCtx, cancel := context.WithCancel(ctx)

	httpClient := &fasthttp.Client{
		ReadTimeout:  config.Timeout,
		WriteTimeout: config.Timeout,
	}

	circuitBreaker := NewCircuitBreaker(config.CircuitBreaker, logger, serviceName)

	client := &HTTPClient{
		ctx:             clientCtx,
		cancel:          cancel,
		logger:          logger,
		name:            serviceName,
		client:          httpClient,
		baseURL:         config.BaseURL,
		config:          config,
		circuitBreaker:  circuitBreaker,
		shutdownTimeout: 10 * time.Second,
		requestTimeout:  config.Timeout,
	}

	client.state.Store(StateRunning)

	return client
}

func (c *HTTPClient) Call(method, path string, data interface{}, opts *types.CallOptions) ([]byte, int, error) {
	if !c.IsRunning() {
		return nil, 500, types.ErrActionNotInitialized
	}

	url := c.baseURL + path

	callCtx, cancel := context.WithTimeout(c.ctx, c.requestTimeout)
	defer cancel()

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(method)

	if data != nil {
		jsonData, err := utils.Marshal(data)
		if err != nil {
			return nil, 500, types.WrapError(err, "failed to marshal request data")
		}
		req.SetBody(jsonData)
		req.Header.SetContentType("application/json")
	}

	timeout := c.requestTimeout
	retries := c.config.Retries

	if opts != nil {
		for key, value := range opts.Headers {
			req.Header.Set(key, value)
		}

		if opts.Timeout > 0 {
			timeout = opts.Timeout
			callCtx, cancel = context.WithTimeout(c.ctx, timeout)
			defer cancel()
		}

		if opts.Retry > 0 {
			retries = opts.Retry
		}
	}

	originalTimeout := c.client.ReadTimeout
	c.client.ReadTimeout = timeout
	defer func() { c.client.ReadTimeout = originalTimeout }()

	var responseBody []byte
	var statusCode int
	var err error

	done := make(chan struct{})
	go func() {
		defer close(done)
		responseBody, statusCode, err = c.executeWithRetries(req, resp, retries)
	}()

	select {
	case <-done:
	case <-callCtx.Done():
		err = types.NewErrorf("call timeout for service: %s", c.name)
		statusCode = 500
		return nil, statusCode, err
	case <-c.ctx.Done():
		err = types.NewErrorf("client shutting down, aborting call to service: %s", c.name)
		statusCode = 500
		return nil, statusCode, err
	}

	return responseBody, statusCode, err
}

func (c *HTTPClient) Close() {
	if !c.transitionClientState(StateRunning, StateStopping) {
		return
	}

	defer func() {
		c.setClientState(StateStopped)
		c.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), c.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			c.logger.Warn("HTTP client close timeout",
				zap.String("service", c.name))
		default:
			c.logger.Error("Error during HTTP client shutdown",
				zap.String("service", c.name),
				zap.Error(err))
		}
	} else {
		c.logger.Debug("HTTP client closed gracefully",
			zap.String("service", c.name))
	}
}

func (c *HTTPClient) IsRunning() bool {
	return c.getClientState() == StateRunning
}

func (c *HTTPClient) getState() (state int32, failures int32, lastFail int64) {
	if c.circuitBreaker == nil {
		return 0, 0, 0
	}
	return c.circuitBreaker.GetState()
}

func (c *HTTPClient) getClientState() State {
	return c.state.Load().(State)
}

func (c *HTTPClient) setClientState(newState State) bool {
	currentState := c.getClientState()
	return c.state.CompareAndSwap(currentState, newState)
}

func (c *HTTPClient) transitionClientState(from, to State) bool {
	return c.state.CompareAndSwap(from, to)
}

func (c *HTTPClient) executeWithRetries(req *fasthttp.Request, resp *fasthttp.Response, maxRetries int) ([]byte, int, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if !c.IsRunning() {
			return nil, 500, types.ErrActionNotInitialized
		}

		if !c.circuitBreaker.CanExecute() {
			return nil, 500, types.ErrCircuitBreakerOpen
		}

		err := c.client.DoTimeout(req, resp, c.config.Timeout)
		statusCode := resp.StatusCode()

		if IsSuccessfulResponse(statusCode, err) {
			c.circuitBreaker.RecordSuccess()

			responseBody := make([]byte, len(resp.Body()))
			copy(responseBody, resp.Body())

			return responseBody, resp.StatusCode(), nil
		}

		if IsCircuitBreakerFailure(statusCode, err) {
			c.circuitBreaker.RecordFailure()
		}

		lastErr = err
		if err == nil {
			lastErr = types.Errorf(types.ErrClientResponseInvalid, "HTTP %d", statusCode)
		}

		if attempt < maxRetries {
			if statusCode >= 400 && statusCode < 500 &&
				statusCode != 429 && statusCode != 408 {
				c.logger.Debug("Not retrying client error",
					zap.String("service", c.name),
					zap.Int("status_code", statusCode))
				break
			}

			backoff := time.Duration(attempt+1) * time.Second

			select {
			case <-time.After(backoff):
				c.logger.Debug("Retrying request",
					zap.String("service", c.name),
					zap.Duration("backoff", backoff),
					zap.Error(lastErr))
			case <-c.ctx.Done():
				return nil, 500, types.NewErrorf("client shutting down during retry for service: %s", c.name)
			}
		}
	}

	return nil, 500, types.Errorf(types.ErrClientRequestFailed, "all %d attempts failed for service %s: %v", maxRetries+1, c.name, lastErr)
}
