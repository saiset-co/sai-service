package middleware

import (
	"bytes"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"sync"
	"time"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type CacheMiddleware struct {
	config       types.ConfigManager
	logger       types.Logger
	metrics      types.MetricsManager
	cache        types.CacheManager
	cacheConfig  *CacheConfig
	name         string
	weight       int
	stringPool   sync.Pool
	headersPool  sync.Pool
	metadataPool sync.Pool
	methodGet    []byte
	noCacheBytes []byte
	noStoreBytes []byte
}

type CacheConfig struct {
	Enabled    bool          `json:"enabled"`
	DefaultTTL time.Duration `json:"default_ttl"`
}

type CachedResponse struct {
	Status  int               `json:"status"`
	Body    []byte            `json:"body"`
	Headers map[string]string `json:"headers"`
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
		name:        "cache",
		config:      config,
		logger:      logger,
		metrics:     metrics,
		cache:       cache,
		cacheConfig: cacheConfig,
		weight:      config.GetConfig().Middlewares.Cache.Weight,

		methodGet:    []byte(fasthttp.MethodGet),
		noCacheBytes: []byte("no-cache"),
		noStoreBytes: []byte("no-store"),

		stringPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 256)
			},
		},

		headersPool: sync.Pool{
			New: func() interface{} {
				return make(map[string]string, 16)
			},
		},

		metadataPool: sync.Pool{
			New: func() interface{} {
				return make(map[string][]byte, 2)
			},
		},
	}
}

func (c *CacheMiddleware) Name() string          { return c.name }
func (c *CacheMiddleware) Weight() int           { return c.weight }
func (c *CacheMiddleware) Provider() interface{} { return nil }

func (c *CacheMiddleware) Handle(ctx *types.RequestCtx, next func(*types.RequestCtx), config *types.RouteConfig) {
	if !bytes.Equal(ctx.Method(), c.methodGet) {
		next(ctx)
		return
	}

	cacheKey := c.buildCacheKey(ctx, config)

	if cached, exists := c.cache.Get(cacheKey); exists {
		if cachedResp, ok := cached.(*CachedResponse); ok {
			c.restoreResponse(ctx, cachedResp)
			return
		}
	}

	next(ctx)

	if c.shouldCacheResponse(ctx) {
		responseData := &CachedResponse{
			Status:  ctx.Response.StatusCode(),
			Body:    append([]byte(nil), ctx.Response.Body()...),
			Headers: c.extractResponseHeaders(ctx),
		}

		ttl := c.cacheConfig.DefaultTTL
		if config.Cache != nil {
			ttl = config.Cache.TTL
		}

		if setErr := c.cache.Set(cacheKey, responseData, ttl); setErr != nil {
			c.logger.Error("Failed to set cache",
				zap.String("cache_key", cacheKey),
				zap.Error(setErr))
		}
	}
}

func (c *CacheMiddleware) shouldCacheResponse(ctx *types.RequestCtx) bool {
	statusCode := ctx.Response.StatusCode()
	if statusCode < 200 || statusCode >= 300 {
		return false
	}

	if len(ctx.Response.Body()) == 0 {
		return false
	}

	cacheControl := ctx.Response.Header.Peek("Cache-Control")
	if len(cacheControl) > 0 {
		lowerCacheControl := bytes.ToLower(cacheControl)
		if bytes.Contains(lowerCacheControl, c.noCacheBytes) ||
			bytes.Contains(lowerCacheControl, c.noStoreBytes) {
			return false
		}
	}

	return true
}

func (c *CacheMiddleware) buildCacheKey(ctx *types.RequestCtx, config *types.RouteConfig) string {
	metadata := c.metadataPool.Get().(map[string][]byte)
	defer func() {
		for k := range metadata {
			delete(metadata, k)
		}
		c.metadataPool.Put(metadata)
	}()

	requestPath := ctx.RequestURI()
	metadata["method"] = ctx.Method()

	if userID := ctx.Request.Header.Peek("X-User-ID"); len(userID) > 0 {
		metadata["user_id"] = userID
	}

	var deps []string
	if config.Cache != nil {
		deps = config.Cache.Deps
	}

	return c.cache.BuildCacheKey(requestPath, deps, metadata)
}

func (c *CacheMiddleware) extractResponseHeaders(ctx *types.RequestCtx) map[string]string {
	headers := c.headersPool.Get().(map[string]string)

	for k := range headers {
		delete(headers, k)
	}

	ctx.Response.Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = string(value)
	})

	result := make(map[string]string, len(headers))
	for k, v := range headers {
		result[k] = v
	}

	c.headersPool.Put(headers)
	return result
}

func (c *CacheMiddleware) restoreResponse(ctx *types.RequestCtx, cachedResp *CachedResponse) {
	ctx.SetStatusCode(cachedResp.Status)
	ctx.SetBody(cachedResp.Body)

	for key, value := range cachedResp.Headers {
		ctx.Response.Header.Set(key, value)
	}
}
