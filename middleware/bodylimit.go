package middleware

import (
	"bytes"
	"fmt"

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
	name            string
	weight          int
	errorResponse   []byte
}

type BodyLimitConfig struct {
	MaxBodySize int64 `json:"max_body_size"`
}

var methods = []byte("POSTPUTPATCHDELETE")

func NewBodyLimitMiddleware(config types.ConfigManager, logger types.Logger, metrics types.MetricsManager) *BodyLimitMiddleware {
	var bodyLimitConfig = &BodyLimitConfig{
		MaxBodySize: 1024 * 1024,
	}

	if config.GetConfig().Middlewares.BodyLimit.Params != nil {
		err := utils.UnmarshalConfig(config.GetConfig().Middlewares.BodyLimit.Params, bodyLimitConfig)
		if err != nil {
			logger.Error("Failed to unmarshal BodyLimit middleware config", zap.Error(err))
		}
	}

	bl := &BodyLimitMiddleware{
		name:            "body-limit",
		config:          config,
		logger:          logger,
		metrics:         metrics,
		bodyLimitConfig: bodyLimitConfig,
		weight:          config.GetConfig().Middlewares.BodyLimit.Weight,
	}

	bl.errorResponse = []byte(fmt.Sprintf(
		`{"error":"Request entity too large","message":"Request body exceeds maximum size of %d bytes","max_size":%d,"error_code":"BODY_TOO_LARGE"}`,
		bodyLimitConfig.MaxBodySize, bodyLimitConfig.MaxBodySize))

	return bl
}

func (bl *BodyLimitMiddleware) Name() string          { return bl.name }
func (bl *BodyLimitMiddleware) Weight() int           { return bl.weight }
func (bl *BodyLimitMiddleware) Provider() interface{} { return nil }

func (bl *BodyLimitMiddleware) Handle(ctx *types.RequestCtx, next func(*types.RequestCtx), _ *types.RouteConfig) {
	if !bytes.Contains(methods, ctx.Method()) {
		next(ctx)
		return
	}

	contentLength := ctx.Request.Header.ContentLength()

	if contentLength > 0 {
		if int64(contentLength) > bl.bodyLimitConfig.MaxBodySize {
			bl.createBodyLimitResponse(ctx)
			return
		}
	}

	if contentLength <= 0 || bl.isChunkedEncoding(ctx) {
		bodySize := int64(len(ctx.PostBody()))
		if bodySize > bl.bodyLimitConfig.MaxBodySize {
			bl.createBodyLimitResponse(ctx)
			return
		}
	}

	next(ctx)
}

func (bl *BodyLimitMiddleware) isChunkedEncoding(ctx *types.RequestCtx) bool {
	transferEncoding := ctx.Request.Header.Peek("Transfer-Encoding")
	if len(transferEncoding) == 0 {
		return false
	}

	return len(transferEncoding) == 7 &&
		transferEncoding[0] == 'c' && transferEncoding[1] == 'h' &&
		transferEncoding[2] == 'u' && transferEncoding[3] == 'n' &&
		transferEncoding[4] == 'k' && transferEncoding[5] == 'e' &&
		transferEncoding[6] == 'd'
}

func (bl *BodyLimitMiddleware) createBodyLimitResponse(ctx *types.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusRequestEntityTooLarge)
	ctx.SetContentType("application/json")
	ctx.SetConnectionClose()

	ctx.SetBody(bl.errorResponse)
}
