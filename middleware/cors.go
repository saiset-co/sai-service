package middleware

import (
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type CORSMiddleware struct {
	config     types.ConfigManager
	logger     types.Logger
	metrics    types.MetricsManager
	corsConfig *CORSConfig
}

type CORSConfig struct {
	ExposedHeaders   []string `json:"ExposedHeaders"`
	AllowedOrigins   []string `json:"AllowedOrigins"`
	AllowedMethods   []string `json:"AllowedMethods"`
	AllowedHeaders   []string `json:"AllowedHeaders"`
	AllowCredentials bool     `json:"AllowCredentials"`
	MaxAge           int      `json:"MaxAge"`
}

func NewCORSMiddleware(config types.ConfigManager, logger types.Logger, metrics types.MetricsManager) *CORSMiddleware {
	var corsConfig = &CORSConfig{
		ExposedHeaders:   []string{},
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-API-Key", "X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           86400,
	}

	if config.GetConfig().Middlewares.CORS.Params != nil {
		err := utils.UnmarshalConfig(config.GetConfig().Middlewares.CORS.Params, corsConfig)
		if err != nil {
			logger.Error("Failed to unmarshal CORS middleware config", zap.Error(err))
		}
	}

	return &CORSMiddleware{
		config:     config,
		logger:     logger,
		metrics:    metrics,
		corsConfig: corsConfig,
	}
}

func (c *CORSMiddleware) Name() string { return "cors" }
func (c *CORSMiddleware) Weight() int  { return 15 }

func (c *CORSMiddleware) Handle(ctx *fasthttp.RequestCtx, next func(*fasthttp.RequestCtx), _ *types.RouteConfig) {
	start := time.Now()

	origin := string(ctx.Request.Header.Peek("Origin"))
	if origin == "" {
		next(ctx)
		return
	}

	if !c.isOriginAllowed(origin) {
		duration := time.Since(start)

		c.logger.Warn("CORS request blocked",
			zap.String("origin", origin),
			zap.String("method", string(ctx.Method())),
			zap.String("path", string(ctx.Path())),
			zap.Duration("duration", duration))

		c.createCORSErrorResponse(ctx)
		return
	}

	if string(ctx.Method()) == fasthttp.MethodOptions {
		duration := time.Since(start)

		c.logger.Debug("CORS preflight request",
			zap.String("origin", origin),
			zap.String("method", string(ctx.Request.Header.Peek("Access-Control-Request-Method"))),
			zap.String("headers", string(ctx.Request.Header.Peek("Access-Control-Request-Headers"))),
			zap.Duration("duration", duration))

		c.createPreflightResponse(ctx, origin)
		return
	}

	c.addCORSHeaders(ctx, origin)

	next(ctx)

	duration := time.Since(start)

	c.logger.Debug("CORS request processed",
		zap.String("origin", origin),
		zap.String("method", string(ctx.Method())),
		zap.String("path", string(ctx.Path())),
		zap.Duration("duration", duration))
}

func (c *CORSMiddleware) isOriginAllowed(origin string) bool {
	for _, allowedOrigin := range c.corsConfig.AllowedOrigins {
		if allowedOrigin == "*" {
			return true
		}
		if allowedOrigin == origin {
			return true
		}

		if strings.HasPrefix(allowedOrigin, "*.") {
			domain := strings.TrimPrefix(allowedOrigin, "*.")
			if strings.HasSuffix(origin, "."+domain) || strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}
	return false
}

func (c *CORSMiddleware) addCORSHeaders(ctx *fasthttp.RequestCtx, origin string) {
	if len(c.corsConfig.AllowedOrigins) == 1 && c.corsConfig.AllowedOrigins[0] == "*" {
		ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
	} else {
		ctx.Response.Header.Set("Access-Control-Allow-Origin", origin)
	}

	if len(c.corsConfig.ExposedHeaders) > 0 {
		ctx.Response.Header.Set("Access-Control-Expose-Headers", strings.Join(c.corsConfig.ExposedHeaders, ", "))
	}

	if c.corsConfig.AllowCredentials {
		ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
	}

	ctx.Response.Header.Add("Vary", "Origin")
}

func (c *CORSMiddleware) createPreflightResponse(ctx *fasthttp.RequestCtx, origin string) {
	ctx.SetStatusCode(fasthttp.StatusOK)

	if len(c.corsConfig.AllowedOrigins) == 1 && c.corsConfig.AllowedOrigins[0] == "*" {
		ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
	} else {
		ctx.Response.Header.Set("Access-Control-Allow-Origin", origin)
	}

	ctx.Response.Header.Set("Access-Control-Allow-Methods", strings.Join(c.corsConfig.AllowedMethods, ", "))
	ctx.Response.Header.Set("Access-Control-Allow-Headers", strings.Join(c.corsConfig.AllowedHeaders, ", "))
	ctx.Response.Header.Set("Access-Control-Max-Age", strconv.Itoa(c.corsConfig.MaxAge))

	if c.corsConfig.AllowCredentials {
		ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
	}

	ctx.Response.Header.Set("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
	ctx.SetBodyString("")
}

func (c *CORSMiddleware) createCORSErrorResponse(ctx *fasthttp.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusForbidden)
	ctx.SetContentType("application/json")
	ctx.SetBodyString(`{"error":"CORS policy violation","message":"Origin not allowed"}`)
}
