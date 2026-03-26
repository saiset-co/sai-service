package middleware

import (
	"bytes"
	"go.uber.org/zap/zapcore"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type LoggingMiddleware struct {
	config           types.ConfigManager
	logger           types.Logger
	metrics          types.MetricsManager
	loggingConfig    *LoggingConfig
	headerSlicePool  sync.Pool
	fieldsPool       sync.Pool
	stackBufPool     sync.Pool
	sensitiveHeaders map[string]bool
	logLevel         int
	name             string
	weight           int
	serviceName      []byte
	requestCounter   uint64
}

type LoggingConfig struct {
	LogLevel   string `json:"log_level"`
	LogHeaders bool   `json:"log_headers"`
	LogBody    bool   `json:"log_body"`
}

var (
	arrowBytes      = []byte("->")
	requestIDHeader = []byte("X-Request-ID")
	prefix          = []byte("_n_")
)

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

	logLevel := 1
	switch loggingConfig.LogLevel {
	case "debug":
		logLevel = 0
	case "info":
		logLevel = 1
	case "warn":
		logLevel = 2
	case "error":
		logLevel = 3
	}

	return &LoggingMiddleware{
		name:          "logging",
		weight:        config.GetConfig().Middlewares.Logging.Weight,
		config:        config,
		logger:        logger,
		metrics:       metrics,
		loggingConfig: loggingConfig,
		logLevel:      logLevel,
		sensitiveHeaders: map[string]bool{
			"authorization": true,
			"x-api-key":     true,
			"cookie":        true,
			"set-cookie":    true,
			"x-auth-token":  true,
		},
		stackBufPool: sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 4096)
				return &buf
			},
		},
		headerSlicePool: sync.Pool{
			New: func() interface{} {
				buf := make([][]byte, 0, 16)
				return &buf
			},
		},
		fieldsPool: sync.Pool{
			New: func() interface{} {
				fields := make([]zapcore.Field, 0, 8)
				return &fields
			},
		},
	}
}

func (l *LoggingMiddleware) Name() string          { return l.name }
func (l *LoggingMiddleware) Weight() int           { return l.weight }
func (l *LoggingMiddleware) Provider() interface{} { return nil }

func (l *LoggingMiddleware) Handle(ctx *types.RequestCtx, next func(*types.RequestCtx), _ *types.RouteConfig) {
	if requestID := ctx.Request.Header.Peek("X-Request-ID"); len(requestID) > 0 {
		ctx.Request.Header.SetBytesKV(requestIDHeader, l.appendServiceToRequestID(requestID))
	} else {
		ctx.Request.Header.SetBytesKV(requestIDHeader, l.generateRequestID())
	}

	start := time.Now()
	l.logRequest(ctx)

	next(ctx)

	duration := time.Since(start)
	l.logResponse(ctx, duration)
}

func (l *LoggingMiddleware) logRequest(ctx *types.RequestCtx) {
	fields := l.fieldsPool.Get().(*[]zap.Field)
	defer func() {
		*fields = (*fields)[:0]
		l.fieldsPool.Put(fields)
	}()

	*fields = append(*fields,
		zap.ByteString("method", ctx.Method()),
		zap.ByteString("path", ctx.Path()),
		zap.ByteString("remote_addr", l.getRemoteAddr(ctx)),
		zap.ByteString("user_agent", ctx.UserAgent()),
	)

	if len(ctx.QueryArgs().QueryString()) > 0 {
		*fields = append(*fields, zap.ByteString("query", ctx.QueryArgs().QueryString()))
	}

	if userID := ctx.Request.Header.Peek("X-User-ID"); len(userID) > 0 {
		*fields = append(*fields, zap.ByteString("user_id", userID))
	}

	if requestID := ctx.Request.Header.Peek("X-Request-ID"); len(requestID) > 0 {
		*fields = append(*fields, zap.ByteString("request_id", requestID))
	}

	if l.loggingConfig.LogHeaders {
		sanitizedHeaders := l.sanitizeHeaders(ctx)
		if len(sanitizedHeaders) > 0 {
			*fields = append(*fields, zap.Any("headers", sanitizedHeaders))
		}
	}

	l.logWithLevel("Request started", *fields...)
}

func (l *LoggingMiddleware) logResponse(ctx *types.RequestCtx, duration time.Duration) {
	fields := l.fieldsPool.Get().(*[]zap.Field)
	defer func() {
		*fields = (*fields)[:0]
		l.fieldsPool.Put(fields)
	}()

	*fields = append(*fields,
		zap.Duration("duration", duration),
		zap.ByteString("method", ctx.Method()),
		zap.ByteString("path", ctx.Path()),
		zap.Int("status", ctx.Response.StatusCode()),
	)

	if requestID := ctx.Request.Header.Peek("X-Request-ID"); len(requestID) > 0 {
		*fields = append(*fields, zap.ByteString("request_id", requestID))
	}

	if l.loggingConfig.LogBody && len(ctx.Response.Body()) > 0 {
		body := ctx.Response.Body()
		if len(body) > 1000 {
			*fields = append(*fields,
				zap.ByteString("response", body[:1000]),
				zap.Int("response_body_truncated", len(body)),
			)
		} else {
			*fields = append(*fields, zap.ByteString("response", body))
		}
	}

	statusCode := ctx.Response.StatusCode()
	if statusCode >= 500 {
		if errI := ctx.UserValue("error"); errI != nil {
			if err, ok := errI.(error); err != nil && ok {
				l.logger.ErrorWithErrStack("Request completed", err, *fields...)
				return
			} else {

			}
		}
		l.logger.Error("Request completed", *fields...)
	} else if statusCode >= 400 {
		if errI := ctx.UserValue("error"); errI != nil {
			if err := errI.(error); err != nil {
				l.logger.ErrorWithErrStack("Request completed", err, *fields...)
				return
			}
		}
		l.logger.Error("Request completed", *fields...)
	} else {
		l.logWithLevel("Request completed", *fields...)
	}
}

func (l *LoggingMiddleware) sanitizeHeaders(ctx *types.RequestCtx) map[string]string {
	result := make(map[string]string, 8)

	ctx.Request.Header.VisitAll(func(key, value []byte) {
		keyStr := string(key)
		lowerKey := strings.ToLower(keyStr)

		if l.sensitiveHeaders[lowerKey] {
			result[keyStr] = "[REDACTED]"
		} else {
			result[keyStr] = string(value)
		}
	})

	return result
}

func (l *LoggingMiddleware) getRemoteAddr(ctx *types.RequestCtx) []byte {
	if forwarded := ctx.Request.Header.Peek("X-Forwarded-For"); len(forwarded) > 0 {
		if comma := bytes.Index(forwarded, []byte(",")); comma > 0 {
			return bytes.TrimSpace(forwarded[:comma])
		}
		return forwarded
	}

	if realIP := ctx.Request.Header.Peek("X-Real-IP"); len(realIP) > 0 {
		return realIP
	}

	return []byte(ctx.RemoteIP().String())
}

func (l *LoggingMiddleware) logWithLevel(msg string, fields ...zap.Field) {
	switch l.logLevel {
	case 0:
		l.logger.Debug(msg, fields...)
	case 1:
		l.logger.Info(msg, fields...)
	case 2:
		l.logger.Warn(msg, fields...)
	case 3:
		l.logger.Error(msg, fields...)
	default:
		l.logger.Info(msg, fields...)
	}
}

func (l *LoggingMiddleware) appendServiceToRequestID(existingID []byte) []byte {
	result := make([]byte, 0, len(existingID)+2+len(l.serviceName))
	result = append(result, existingID...)
	result = append(result, arrowBytes...)
	result = append(result, l.serviceName...)

	return result
}

func (l *LoggingMiddleware) generateRequestID() []byte {
	id := atomic.AddUint64(&l.requestCounter, 1)

	totalSize := len(l.serviceName) + len(prefix) + 36
	result := make([]byte, 0, totalSize)

	result = append(result, l.serviceName...)
	result = append(result, prefix...)
	result = strconv.AppendUint(result, id, 10)

	return result
}
