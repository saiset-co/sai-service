package middleware

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type MetadataMiddleware struct {
	config         types.ConfigManager
	logger         types.Logger
	metrics        types.MetricsManager
	metadataConfig *MetadataConfig
}

type MetadataConfig struct {
	PropagatedHeaders []string `json:"propagated_headers"`
	GenerateRequestID bool     `json:"generate_request_id"`
}

func NewMetadataMiddleware(config types.ConfigManager, logger types.Logger, metrics types.MetricsManager) *MetadataMiddleware {
	var metadataConfig = &MetadataConfig{
		GenerateRequestID: true,
		PropagatedHeaders: []string{
			"Authorization",
			"X-User-ID",
			"X-Real-IP",
			"X-Forwarded-For",
			"X-Request-ID",
			"X-Trace-ID",
			"X-Client-ID",
			"X-API-Key",
		},
	}

	if config.GetConfig().Middlewares.Metadata.Params != nil {
		err := utils.UnmarshalConfig(config.GetConfig().Middlewares.Metadata.Params, metadataConfig)
		if err != nil {
			logger.Error("Failed to unmarshal Metadata middleware config", zap.Error(err))
		}
	}

	return &MetadataMiddleware{
		config:         config,
		logger:         logger,
		metrics:        metrics,
		metadataConfig: metadataConfig,
	}
}

func (m *MetadataMiddleware) Name() string { return "metadata" }
func (m *MetadataMiddleware) Weight() int  { return 30 }

func (m *MetadataMiddleware) Handle(ctx *fasthttp.RequestCtx, next func(*fasthttp.RequestCtx), _ *types.RouteConfig) {
	start := time.Now()
	metadata := m.extractMetadata(ctx)

	if m.metadataConfig.GenerateRequestID && metadata["request_id"] == "" {
		metadata["request_id"] = m.genRequestID()
	}

	m.enrichRequest(ctx, metadata)

	next(ctx)

	duration := time.Since(start)

	m.logger.Debug("Metadata processed",
		zap.String("request_id", metadata["request_id"]),
		zap.String("user_id", metadata["user_id"]),
		zap.String("real_ip", metadata["real_ip"]),
		zap.String("path", string(ctx.Path())),
		zap.Duration("duration", duration))
}

func (m *MetadataMiddleware) extractMetadata(ctx *fasthttp.RequestCtx) map[string]string {
	metadata := make(map[string]string)

	headerMappings := map[string]string{
		"X-User-ID":     "user_id",
		"X-Request-ID":  "request_id",
		"X-Trace-ID":    "trace_id",
		"X-Client-ID":   "client_id",
		"Authorization": "authorization",
		"X-API-Key":     "api_key",
	}

	for header, key := range headerMappings {
		if value := string(ctx.Request.Header.Peek(header)); value != "" {
			metadata[key] = value
		}
	}

	metadata["real_ip"] = m.extractRealIP(ctx)

	return metadata
}

func (m *MetadataMiddleware) extractRealIP(ctx *fasthttp.RequestCtx) string {
	if realIP := string(ctx.Request.Header.Peek("X-Real-IP")); realIP != "" {
		return realIP
	}

	if forwarded := string(ctx.Request.Header.Peek("X-Forwarded-For")); forwarded != "" {
		if comma := strings.Index(forwarded, ","); comma > 0 {
			return strings.TrimSpace(forwarded[:comma])
		}
		return strings.TrimSpace(forwarded)
	}

	return ctx.RemoteIP().String()
}

func (m *MetadataMiddleware) enrichRequest(ctx *fasthttp.RequestCtx, metadata map[string]string) {
	ctx.SetUserValue("metadata", metadata)

	propagationHeaders := make(map[string]string)
	propagatedCount := 0

	headerToKey := map[string]string{
		"x-user-id":     "user_id",
		"x-request-id":  "request_id",
		"x-trace-id":    "trace_id",
		"x-real-ip":     "real_ip",
		"x-client-id":   "client_id",
		"authorization": "authorization",
		"x-api-key":     "api_key",
	}

	for _, headerName := range m.metadataConfig.PropagatedHeaders {
		lowerHeader := strings.ToLower(headerName)
		if key, exists := headerToKey[lowerHeader]; exists {
			if value := metadata[key]; value != "" {
				propagationHeaders[headerName] = value
				propagatedCount++
			}
		}
	}

	if propagatedCount > 0 {
		ctx.SetUserValue("propagation_headers", propagationHeaders)
	}
}

func (m *MetadataMiddleware) genRequestID() string {
	return fmt.Sprintf("req_%d_%d", time.Now().UnixNano(), rand.Intn(10000))
}
