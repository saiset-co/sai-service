package middleware

import (
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type LoggingMiddleware struct {
	config        types.ConfigManager
	logger        types.Logger
	metrics       types.MetricsManager
	loggingConfig *LoggingConfig
	headerMapPool sync.Pool
}

type LoggingConfig struct {
	LogLevel   string `json:"log_level"`
	LogHeaders bool   `json:"log_headers"`
	LogBody    bool   `json:"log_body"`
}

func NewLoggingMiddleware(config types.ConfigManager, logger types.Logger, metrics types.MetricsManager) *LoggingMiddleware {
	var loggingConfig = &LoggingConfig{
		LogLevel:   "info",
		LogHeaders: false,
		LogBody:    false,
	}

	if config.GetConfig().Middlewares.Logging.Params != nil {
		err := utils.UnmarshalConfig(config.GetConfig().Middlewares.Logging.Params, loggingConfig)
		if err != nil {
			logger.Error("Failed to unmarshal Logging middleware config", zap.Error(err))
		}
	}

	return &LoggingMiddleware{
		config:        config,
		logger:        logger,
		metrics:       metrics,
		loggingConfig: loggingConfig,
		headerMapPool: sync.Pool{
			New: func() interface{} {
				return make(map[string]string, 16)
			},
		},
	}
}

func (l *LoggingMiddleware) Name() string { return "logging" }
func (l *LoggingMiddleware) Weight() int  { return 20 }

func (l *LoggingMiddleware) Handle(ctx *fasthttp.RequestCtx, next func(*fasthttp.RequestCtx), _ *types.RouteConfig) {
	start := time.Now()

	l.logRequest(ctx)

	next(ctx)

	duration := time.Since(start)
	l.logResponse(ctx, duration)
}

func (l *LoggingMiddleware) logRequest(ctx *fasthttp.RequestCtx) {
	fields := []zap.Field{
		zap.String("method", string(ctx.Method())),
		zap.String("path", string(ctx.Path())),
		zap.String("remote_addr", l.getRemoteAddr(ctx)),
		zap.String("user_agent", string(ctx.UserAgent())),
	}

	if len(ctx.QueryArgs().QueryString()) > 0 {
		fields = append(fields, zap.String("query", string(ctx.QueryArgs().QueryString())))
	}

	if userID := string(ctx.Request.Header.Peek("X-User-ID")); userID != "" {
		fields = append(fields, zap.String("user_id", userID))
	}

	if requestID := string(ctx.Request.Header.Peek("X-Request-ID")); requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}

	if l.loggingConfig.LogHeaders {
		sanitizedHeaders := l.sanitizeHeaders(ctx)
		fields = append(fields, zap.Any("headers", sanitizedHeaders))
	}

	l.logWithLevel("Request started", fields...)
}

func (l *LoggingMiddleware) logResponse(ctx *fasthttp.RequestCtx, duration time.Duration) {
	fields := []zap.Field{
		zap.Duration("duration", duration),
	}

	fields = append(fields,
		zap.String("method", string(ctx.Method())),
		zap.String("path", string(ctx.Path())),
	)

	if requestID := string(ctx.Request.Header.Peek("X-Request-ID")); requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}

	if l.loggingConfig.LogBody && len(ctx.Response.Body()) > 0 {
		body := ctx.Response.Body()
		if len(body) > 1000 {
			fields = append(fields, zap.String("response", string(body[:1000])+"..."))
			fields = append(fields, zap.Int("response_body_truncated", len(body)))
		} else {
			fields = append(fields, zap.String("response", string(body)))
		}
	}

	if ctx.Response.StatusCode() >= 500 {
		l.logger.Error("Request completed", fields...)
	} else if ctx.Response.StatusCode() >= 400 {
		l.logger.Warn("Request completed", fields...)
	} else {
		l.logWithLevel("Request completed", fields...)
	}
}

func (l *LoggingMiddleware) sanitizeHeaders(ctx *fasthttp.RequestCtx) map[string]string {
	sanitized := l.headerMapPool.Get().(map[string]string)
	defer func() {
		for k := range sanitized {
			delete(sanitized, k)
		}
		l.headerMapPool.Put(sanitized)
	}()

	sensitiveHeaders := map[string]bool{
		"authorization": true,
		"x-api-key":     true,
		"cookie":        true,
		"set-cookie":    true,
	}

	ctx.Request.Header.VisitAll(func(key, value []byte) {
		keyStr := string(key)
		lowerKey := strings.ToLower(keyStr)

		if sensitiveHeaders[lowerKey] {
			sanitized[keyStr] = "[REDACTED]"
		} else {
			sanitized[keyStr] = string(value)
		}
	})

	return sanitized
}

func (l *LoggingMiddleware) getRemoteAddr(ctx *fasthttp.RequestCtx) string {
	if forwarded := string(ctx.Request.Header.Peek("X-Forwarded-For")); forwarded != "" {
		if comma := strings.Index(forwarded, ","); comma > 0 {
			return strings.TrimSpace(forwarded[:comma])
		}
		return forwarded
	}

	if realIP := string(ctx.Request.Header.Peek("X-Real-IP")); realIP != "" {
		return realIP
	}

	return ctx.RemoteIP().String()
}

func (l *LoggingMiddleware) logWithLevel(msg string, fields ...zap.Field) {
	switch l.loggingConfig.LogLevel {
	case "debug":
		l.logger.Debug(msg, fields...)
	case "info":
		l.logger.Info(msg, fields...)
	case "warn":
		l.logger.Warn(msg, fields...)
	case "error":
		l.logger.Error(msg, fields...)
	default:
		l.logger.Info(msg, fields...)
	}
}
