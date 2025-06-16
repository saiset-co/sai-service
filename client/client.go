package client

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type HTTPClient struct {
	ctx            context.Context
	logger         types.Logger
	name           string
	client         *fasthttp.Client
	baseURL        string
	config         *ServiceClientConfig
	circuitBreaker *CircuitBreaker
	closed         int32
}

type ServiceClientConfig struct {
	BaseURL        string                `yaml:"base_url" json:"base_url"`
	Timeout        time.Duration         `yaml:"timeout" json:"timeout"`
	Retries        int                   `yaml:"retries" json:"retries"`
	CircuitBreaker *CircuitBreakerConfig `yaml:"circuit_breaker" json:"circuit_breaker"`
}

func NewHTTPClient(ctx context.Context, logger types.Logger, serviceName string, config *ServiceClientConfig) *HTTPClient {
	httpClient := &fasthttp.Client{
		ReadTimeout:  config.Timeout,
		WriteTimeout: config.Timeout,
	}

	circuitBreaker := NewCircuitBreaker(config.CircuitBreaker, logger, serviceName)

	return &HTTPClient{
		ctx:            ctx,
		logger:         logger,
		name:           serviceName,
		client:         httpClient,
		baseURL:        config.BaseURL,
		config:         config,
		circuitBreaker: circuitBreaker,
	}
}

func (c *HTTPClient) executeWithRetries(req *fasthttp.Request, resp *fasthttp.Response, maxRetries int) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if !c.circuitBreaker.CanExecute() {
			return types.ErrCircuitBreakerOpen
		}

		c.logger.Debug("HTTP request attempt",
			zap.String("service", c.name),
			zap.String("method", string(req.Header.Method())),
			zap.String("url", string(req.URI().FullURI())),
			zap.Int("attempt", attempt+1),
			zap.String("cb_state", c.circuitBreaker.GetStateString()))

		err := c.client.DoTimeout(req, resp, c.config.Timeout)
		statusCode := resp.StatusCode()

		if IsSuccessfulResponse(statusCode, err) {
			c.circuitBreaker.RecordSuccess()
			return nil
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
			c.logger.Debug("Retrying request",
				zap.String("service", c.name),
				zap.Duration("backoff", backoff),
				zap.Error(lastErr))

			time.Sleep(backoff)
		}
	}

	return types.Errorf(types.ErrClientRequestFailed, "all %d attempts failed for service %s: %v", maxRetries+1, c.name, lastErr)
}

func (c *HTTPClient) Call(method, path string, data interface{}, opts types.CallOptions) error {
	url := c.baseURL + path

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(method)

	if data != nil {
		jsonData, err := utils.Marshal(data)
		if err != nil {
			return types.WrapError(err, "failed to marshal request data")
		}
		req.SetBody(jsonData)
		req.Header.SetContentType("application/json")
	}

	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	timeout := c.config.Timeout
	if opts.Timeout > 0 {
		timeout = opts.Timeout
	}

	retries := c.config.Retries
	if opts.Retry > 0 {
		retries = opts.Retry
	}

	originalTimeout := c.client.ReadTimeout
	c.client.ReadTimeout = timeout
	defer func() { c.client.ReadTimeout = originalTimeout }()

	return c.executeWithRetries(req, resp, retries)
}

func (c *HTTPClient) GetState() (state int32, failures int32, lastFail int64) {
	return c.circuitBreaker.GetState()
}

func (c *HTTPClient) Close() {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return
	}
}

func (c *HTTPClient) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	return c.client.Do(req, resp)
}

func (c *HTTPClient) DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error {
	return c.client.DoTimeout(req, resp, timeout)
}

func (c *HTTPClient) DoRedirects(req *fasthttp.Request, resp *fasthttp.Response, maxRedirectsCount int) error {
	return c.client.DoRedirects(req, resp, maxRedirectsCount)
}

func (c *HTTPClient) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	return c.client.DoDeadline(req, resp, deadline)
}

func (c *HTTPClient) Get(dst []byte, url string) (statusCode int, body []byte, err error) {
	return c.client.Get(dst, url)
}

func (c *HTTPClient) GetTimeout(dst []byte, url string, timeout time.Duration) (statusCode int, body []byte, err error) {
	return c.client.GetTimeout(dst, url, timeout)
}

func (c *HTTPClient) GetDeadline(dst []byte, url string, deadline time.Time) (statusCode int, body []byte, err error) {
	return c.client.GetDeadline(dst, url, deadline)
}

func (c *HTTPClient) Post(dst []byte, url string, postArgs *fasthttp.Args) (statusCode int, body []byte, err error) {
	return c.client.Post(dst, url, postArgs)
}
