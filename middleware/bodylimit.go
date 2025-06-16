package middleware

import (
	"fmt"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type BodyLimitMiddleware struct {
	config          types.ConfigManager
	logger          types.Logger
	metrics         types.MetricsManager
	bodyLimitConfig *BodyLimitConfig
}

type BodyLimitConfig struct {
	MaxBodySize int64 `json:"max_body_size"`
}

func NewBodyLimitMiddleware(config types.ConfigManager, logger types.Logger, metrics types.MetricsManager) *BodyLimitMiddleware {
	var bodyLimitConfig = &BodyLimitConfig{}

	if config.GetConfig().Middlewares.BodyLimit.Params != nil {
		err := utils.UnmarshalConfig(config.GetConfig().Middlewares.BodyLimit.Params, bodyLimitConfig)
		if err != nil {
			logger.Error("Failed to unmarshal BodyLimit middleware config", zap.Error(err))
		}
	}

	return &BodyLimitMiddleware{
		config:          config,
		logger:          logger,
		metrics:         metrics,
		bodyLimitConfig: bodyLimitConfig,
	}
}

func (bl *BodyLimitMiddleware) Name() string { return "body-limit" }
func (bl *BodyLimitMiddleware) Weight() int  { return 35 }

func (bl *BodyLimitMiddleware) Handle(ctx *fasthttp.RequestCtx, next func(*fasthttp.RequestCtx), _ *types.RouteConfig) {
	start := time.Now()

	method := ctx.Method()
	if !bl.hasBody(method) {
		next(ctx)
		return
	}

	contentLength := ctx.Request.Header.ContentLength()
	if contentLength > 0 && int64(contentLength) > bl.bodyLimitConfig.MaxBodySize {
		duration := time.Since(start)

		bl.logger.Warn("Request body too large",
			zap.String("method", string(method)),
			zap.String("path", string(ctx.Path())),
			zap.Int("content_length", contentLength),
			zap.Int64("max_allowed", bl.bodyLimitConfig.MaxBodySize),
			zap.Duration("duration", duration))

		bl.createBodyLimitResponse(ctx, bl.bodyLimitConfig.MaxBodySize)
		return
	}

	if contentLength <= 0 || bl.isChunkedEncoding(ctx) {
		bodySize := int64(len(ctx.PostBody()))
		if bodySize > bl.bodyLimitConfig.MaxBodySize {
			duration := time.Since(start)

			bl.logger.Warn("Request body too large during streaming",
				zap.String("method", string(method)),
				zap.String("path", string(ctx.Path())),
				zap.Int64("body_size", bodySize),
				zap.Int64("max_allowed", bl.bodyLimitConfig.MaxBodySize),
				zap.Duration("duration", duration))

			bl.createBodyLimitResponse(ctx, bl.bodyLimitConfig.MaxBodySize)
			return
		}
	}

	next(ctx)
}

func (bl *BodyLimitMiddleware) hasBody(method []byte) bool {
	switch string(method) {
	case fasthttp.MethodPost, fasthttp.MethodPut, fasthttp.MethodPatch:
		return true
	case fasthttp.MethodDelete:
		return true
	case fasthttp.MethodGet, fasthttp.MethodHead, fasthttp.MethodOptions:
		return false
	default:
		return true
	}
}

func (bl *BodyLimitMiddleware) isChunkedEncoding(ctx *fasthttp.RequestCtx) bool {
	transferEncoding := string(ctx.Request.Header.Peek("Transfer-Encoding"))
	return transferEncoding == "chunked"
}

func (bl *BodyLimitMiddleware) createBodyLimitResponse(ctx *fasthttp.RequestCtx, limit int64) {
	ctx.SetStatusCode(fasthttp.StatusRequestEntityTooLarge)
	ctx.SetContentType("application/json")
	ctx.SetConnectionClose()

	response := fmt.Sprintf(`{"error":"Request entity too large","message":"Request body exceeds maximum size of %d bytes","max_size":%d,"error_code":"BODY_TOO_LARGE"}`,
		limit, limit)

	ctx.SetBodyString(response)
}
