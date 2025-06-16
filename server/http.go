package server

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

type FastHTTPServer struct {
	ctx                context.Context
	config             types.ConfigManager
	logger             types.Logger
	metrics            types.MetricsManager
	middlewares        types.MiddlewareManager
	router             types.HTTPRouter
	server             *fasthttp.Server
	httpConfig         *types.HTTPConfig
	tlsConfig          *types.TLSConfig
	tlsManager         types.TLSManager
	running            int32
	responseWriterPool sync.Pool
	requestWrapperPool sync.Pool
	contextPool        sync.Pool
}

func NewHTTPServer(
	ctx context.Context,
	config types.ConfigManager,
	logger types.Logger,
	metrics types.MetricsManager,
	middlewares types.MiddlewareManager,
	tlsManager types.TLSManager,
	router types.HTTPRouter) (*FastHTTPServer, error) {
	server := &FastHTTPServer{
		ctx:         ctx,
		config:      config,
		logger:      logger,
		metrics:     metrics,
		middlewares: middlewares,
		tlsManager:  tlsManager,
		router:      router,
		httpConfig:  config.GetConfig().Server.HTTP,
		tlsConfig:   config.GetConfig().Server.TLS,
		responseWriterPool: sync.Pool{
			New: func() interface{} {
				return &types.FastResponseWriter{}
			},
		},
		requestWrapperPool: sync.Pool{
			New: func() interface{} {
				return make(map[string]interface{}, 8)
			},
		},
		contextPool: sync.Pool{
			New: func() interface{} {
				return &types.FastRequestContext{}
			},
		},
		running: 0,
	}

	return server, nil
}

func (h *FastHTTPServer) Start() error {
	if !atomic.CompareAndSwapInt32(&h.running, 0, 1) {
		return types.ErrServerAlreadyRunning
	}

	h.server = &fasthttp.Server{
		Handler:                       h.createMainHandler(),
		ReadTimeout:                   time.Duration(h.httpConfig.ReadTimeout) * time.Second,
		WriteTimeout:                  time.Duration(h.httpConfig.WriteTimeout) * time.Second,
		IdleTimeout:                   time.Duration(h.httpConfig.IdleTimeout) * time.Second,
		MaxConnsPerIP:                 1000000,
		MaxRequestsPerConn:            1000000,
		TCPKeepalive:                  true,
		ReduceMemoryUsage:             false,
		DisablePreParseMultipartForm:  true,
		NoDefaultServerHeader:         false,
		NoDefaultDate:                 false,
		NoDefaultContentType:          false,
		KeepHijackedConns:             false,
		CloseOnShutdown:               true,
		StreamRequestBody:             false,
		DisableHeaderNamesNormalizing: false,
	}

	addr := fmt.Sprintf("%s:%d", h.httpConfig.Host, h.httpConfig.Port)

	go func() {
		var err error
		if h.tlsConfig.Enabled {
			ln, err := h.tlsManager.Serve(addr)
			if err != nil {
				h.logger.Error("TLS HTTP server failed", zap.Error(err))
				atomic.StoreInt32(&h.running, 0)
			}
			err = h.server.Serve(ln)
		} else {
			err = h.server.ListenAndServe(addr)
		}

		if err != nil {
			h.logger.Error("HTTP server failed", zap.Error(err))
			atomic.StoreInt32(&h.running, 0)
		}
	}()

	h.logger.Info("HTTP server started successfully",
		zap.String("address", addr),
		zap.Bool("tls", h.tlsConfig.Enabled))

	return nil
}

func (h *FastHTTPServer) Stop() error {
	if !atomic.CompareAndSwapInt32(&h.running, 1, 0) {
		return types.ErrServerNotRunning
	}

	if err := h.server.Shutdown(); err != nil {
		h.logger.Error("HTTP server shutdown error", zap.Error(err))
		return err
	}

	h.logger.Info("HTTP server stopped")
	return nil
}

func (h *FastHTTPServer) IsRunning() bool {
	return atomic.LoadInt32(&h.running) == 1
}

func (h *FastHTTPServer) HandleRequest(ctx *fasthttp.RequestCtx, handler types.FastHTTPHandler, config *types.RouteConfig) {
	start := time.Now()

	h.recordInFlightMetric(1)
	defer h.recordInFlightMetric(-1)

	if handler == nil {
		ctx.Error("Path not found", fasthttp.StatusNotFound)
		h.recordHTTPMetrics(ctx, 404, 0, time.Since(start), "path_not_found")
		return
	}

	if config == nil {
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		h.recordHTTPMetrics(ctx, 500, 0, time.Since(start), "config_error")
		return
	}

	if h.middlewares != nil {
		finalHandler := func(ctx *fasthttp.RequestCtx) {
			if config.Timeout > 0 {
				h.executeWithTimeout(ctx, handler, config.Timeout)
			} else {
				handler(ctx)
			}
		}

		h.middlewares.Execute(ctx, finalHandler, config)
	} else {
		if config.Timeout > 0 {
			h.executeWithTimeout(ctx, handler, config.Timeout)
		} else {
			handler(ctx)
		}
	}

	duration := time.Since(start)
	h.recordHTTPMetrics(ctx, ctx.Response.StatusCode(), int64(ctx.Response.Header.ContentLength()), duration, "completed")
}

func (h *FastHTTPServer) recordInFlightMetric(delta float64) {
	if h.metrics == nil {
		return
	}

	gauge := h.metrics.Gauge("http_server_requests_in_flight", nil)
	if delta > 0 {
		gauge.Inc()
	} else {
		gauge.Dec()
	}
}

func (h *FastHTTPServer) createMainHandler() fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		h.router.Handler(ctx, h)
	}
}

func (h *FastHTTPServer) executeWithTimeout(ctx *fasthttp.RequestCtx, handler types.FastHTTPHandler, timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("Handler panicked", zap.Any("panic", r))
				ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
			}
			close(done)
		}()
		handler(ctx)
	}()

	select {
	case <-done:
	case <-timer.C:
		ctx.Error("Request timeout", fasthttp.StatusRequestTimeout)
	}
}

func (h *FastHTTPServer) recordHTTPMetrics(ctx *fasthttp.RequestCtx, statusCode int, responseSize int64, duration time.Duration, requestType string) {
	if h.metrics == nil {
		return
	}

	method := string(ctx.Method())
	path := string(ctx.Path())
	status := fmt.Sprintf("%d", statusCode)

	statusClass := h.getStatusClass(statusCode)

	counter := h.metrics.Counter("http_server_requests_total", map[string]string{
		"method":       method,
		"status":       status,
		"status_class": statusClass,
		"route":        path,
		"type":         requestType,
		"protocol":     h.getProtocol(ctx),
	})
	counter.Inc()

	durationHist := h.metrics.Histogram("http_server_request_duration_seconds",
		[]float64{0.001, 0.01, 0.1, 0.5, 1.0, 2.5, 5.0, 10.0},
		map[string]string{
			"method":       method,
			"route":        path,
			"status_class": statusClass,
			"protocol":     h.getProtocol(ctx),
		})
	durationHist.Observe(duration.Seconds())

	if responseSize > 0 {
		sizeHist := h.metrics.Histogram("http_server_response_size_bytes",
			[]float64{100, 1000, 10000, 100000, 1000000},
			map[string]string{
				"method":   method,
				"route":    path,
				"protocol": h.getProtocol(ctx),
			})
		sizeHist.Observe(float64(responseSize))
	}

	h.recordMiddlewareMetrics(ctx, statusCode)

	h.logger.Debug("HTTP request completed",
		zap.String("method", method),
		zap.String("route", path),
		zap.String("status", status),
		zap.String("type", requestType),
		zap.String("protocol", h.getProtocol(ctx)),
		zap.Duration("duration", duration),
		zap.Int64("response_size", responseSize))
}

func (h *FastHTTPServer) getProtocol(ctx *fasthttp.RequestCtx) string {
	if ctx.IsTLS() {
		if string(ctx.Request.Header.Peek("HTTP2-Settings")) != "" {
			return "http2"
		}
		return "https"
	}
	return "http"
}

func (h *FastHTTPServer) recordMiddlewareMetrics(ctx *fasthttp.RequestCtx, statusCode int) {
	if h.metrics == nil {
		return
	}

	if origin := string(ctx.Request.Header.Peek("Origin")); origin != "" {
		corsResult := "allowed"
		if statusCode == 403 {
			corsResult = "blocked"
		} else if string(ctx.Method()) == fasthttp.MethodOptions {
			corsResult = "preflight"
		}

		corsCounter := h.metrics.Counter("cors_requests_total", map[string]string{
			"result": corsResult,
		})
		corsCounter.Inc()
	}

	if hasAuthHeaders(ctx) {
		authResult := "success"
		if statusCode == 401 {
			authResult = "failure"
		}

		authCounter := h.metrics.Counter("auth_requests_total", map[string]string{
			"result": authResult,
		})
		authCounter.Inc()
	}

	if statusCode == fasthttp.StatusRequestEntityTooLarge {
		bodyLimitCounter := h.metrics.Counter("body_limit_requests_total", map[string]string{
			"result": "blocked",
		})
		bodyLimitCounter.Inc()
	}

	if statusCode == fasthttp.StatusTooManyRequests {
		rateLimitCounter := h.metrics.Counter("rate_limit_requests_total", map[string]string{
			"result": "blocked",
		})
		rateLimitCounter.Inc()
	}
}

func (h *FastHTTPServer) getStatusClass(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500:
		return "5xx"
	default:
		return "1xx"
	}
}

func (h *FastHTTPServer) GetCertificateStatus() map[string]types.CertificateStatus {
	if h.tlsManager != nil {
		return h.tlsManager.GetCertificateStatus()
	}
	return nil
}

func hasAuthHeaders(ctx *fasthttp.RequestCtx) bool {
	return len(ctx.Request.Header.Peek("Authorization")) > 0 ||
		len(ctx.Request.Header.Peek("X-API-Key")) > 0 ||
		len(ctx.Request.Header.Peek("X-Auth-Token")) > 0
}
