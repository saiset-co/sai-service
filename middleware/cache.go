package middleware

import (
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type CacheMiddleware struct {
	config      types.ConfigManager
	logger      types.Logger
	metrics     types.MetricsManager
	cache       types.CacheManager
	cacheConfig *CacheConfig
}

type CacheConfig struct {
	Enabled    bool          `json:"enabled"`
	DefaultTTL time.Duration `json:"default_ttl"`
}

func NewCacheMiddleware(config types.ConfigManager, logger types.Logger, metrics types.MetricsManager, cache types.CacheManager) *CacheMiddleware {
	enabled := config.GetConfig().Middlewares.Cache.Enabled

	var cacheConfig = &CacheConfig{
		Enabled:    enabled,
		DefaultTTL: 5 * time.Minute,
	}

	if config.GetConfig().Middlewares.Cache.Params != nil {
		err := utils.UnmarshalConfig(config.GetConfig().Middlewares.Cache.Params, cacheConfig)
		if err != nil {
			logger.Error("Failed to unmarshal Cache middleware config", zap.Error(err))
		}
	}

	return &CacheMiddleware{
		config:      config,
		logger:      logger,
		metrics:     metrics,
		cache:       cache,
		cacheConfig: cacheConfig,
	}
}

func (c *CacheMiddleware) Name() string { return "cache" }
func (c *CacheMiddleware) Weight() int  { return 40 }

func (c *CacheMiddleware) Handle(ctx *fasthttp.RequestCtx, next func(*fasthttp.RequestCtx), config *types.RouteConfig) {
	if !c.cacheConfig.Enabled || c.cache == nil {
		next(ctx)
		return
	}

	if string(ctx.Method()) != fasthttp.MethodGet {
		next(ctx)
		return
	}

	if config == nil || config.Cache == nil || !config.Cache.Enabled {
		next(ctx)
		return
	}

	start := time.Now()
	cacheKey := c.buildCacheKey(ctx, config)

	if cached, exists := c.cache.Get(cacheKey); exists {
		duration := time.Since(start)

		c.logger.Debug("Cache hit",
			zap.String("cache_key", cacheKey),
			zap.String("path", string(ctx.Path())),
			zap.Duration("duration", duration))

		if cachedResp, ok := cached.(map[string]interface{}); ok {
			c.restoreResponse(ctx, cachedResp)
		}
		return
	}

	if c.shouldCacheResponse(ctx) {
		responseData := map[string]interface{}{
			"status":  ctx.Response.StatusCode(),
			"body":    ctx.Response.Body(),
			"headers": c.extractResponseHeaders(ctx),
		}

		ttl := c.getTTL(config.Cache)

		if setErr := c.cache.Set(cacheKey, responseData, ttl); setErr != nil {
			c.logger.Error("Failed to set cache",
				zap.String("cache_key", cacheKey),
				zap.Error(setErr))
		} else {
			c.logger.Debug("Cache set",
				zap.String("cache_key", cacheKey),
				zap.String("path", string(ctx.Path())))
		}
	}

	totalDuration := time.Since(start)

	c.logger.Debug("Cache miss and set",
		zap.String("cache_key", cacheKey),
		zap.String("path", string(ctx.Path())),
		zap.Duration("total_duration", totalDuration))
}

func (c *CacheMiddleware) shouldCacheResponse(ctx *fasthttp.RequestCtx) bool {
	statusCode := ctx.Response.StatusCode()
	if statusCode < 200 || statusCode >= 300 {
		return false
	}

	if len(ctx.Response.Body()) == 0 {
		return false
	}

	cacheControl := string(ctx.Response.Header.Peek("Cache-Control"))
	if strings.Contains(strings.ToLower(cacheControl), "no-cache") ||
		strings.Contains(strings.ToLower(cacheControl), "no-store") {
		return false
	}

	return true
}

func (c *CacheMiddleware) buildCacheKey(ctx *fasthttp.RequestCtx, config *types.RouteConfig) string {
	if config.Cache.Key != "" {
		return config.Cache.Key
	}

	requestPath := string(ctx.Path())
	if len(ctx.QueryArgs().QueryString()) > 0 {
		requestPath += "?" + string(ctx.QueryArgs().QueryString())
	}

	metadata := map[string]string{
		"method": string(ctx.Method()),
	}

	if userID := string(ctx.Request.Header.Peek("X-User-ID")); userID != "" {
		metadata["user_id"] = userID
	}

	return c.cache.BuildCacheKey(requestPath, config.Cache.Deps, metadata)
}

func (c *CacheMiddleware) getTTL(config *types.CacheHandlerConfig) time.Duration {
	if config.TTL > 0 {
		return time.Duration(config.TTL) * time.Second
	}
	return c.cacheConfig.DefaultTTL
}

func (c *CacheMiddleware) extractResponseHeaders(ctx *fasthttp.RequestCtx) map[string]string {
	headers := make(map[string]string)

	ctx.Response.Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = string(value)
	})

	return headers
}

func (c *CacheMiddleware) restoreResponse(ctx *fasthttp.RequestCtx, cachedResp map[string]interface{}) {
	if status, ok := cachedResp["status"].(int); ok {
		ctx.SetStatusCode(status)
	}

	if body, ok := cachedResp["body"].([]byte); ok {
		ctx.SetBody(body)
	} else if bodyStr, ok := cachedResp["body"].(string); ok {
		ctx.SetBodyString(bodyStr)
	}

	if headers, ok := cachedResp["headers"].(map[string]string); ok {
		for key, value := range headers {
			ctx.Response.Header.Set(key, value)
		}
	}
}
