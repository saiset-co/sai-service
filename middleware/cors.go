package middleware

import (
	"bytes"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"strconv"
	"strings"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

var (
	trueBytes        = []byte("true")
	asteriskBytes    = []byte("*")
	optionsBytes     = []byte("OPTIONS")
	varyOriginStr    = []byte("Origin")
	varyPreflightStr = []byte("Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
)

type CORSMiddleware struct {
	config             types.ConfigManager
	logger             types.Logger
	metrics            types.MetricsManager
	corsConfig         *CORSConfig
	name               string
	weight             int
	allowsAll          bool
	allowedOriginsMap  map[string]bool
	wildcardDomains    []string
	allowedMethodsStr  []byte
	allowedHeadersStr  []byte
	exposedHeadersStr  []byte
	maxAgeStr          []byte
	errorResponseBytes []byte
	allowCredentials   bool
	hasExposedHeaders  bool
}

type CORSConfig struct {
	ExposedHeaders   []string `json:"exposed_headers"`
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           int      `json:"max_age"`
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

	cm := &CORSMiddleware{
		name:               "cors",
		weight:             config.GetConfig().Middlewares.CORS.Weight,
		config:             config,
		logger:             logger,
		metrics:            metrics,
		corsConfig:         corsConfig,
		allowCredentials:   corsConfig.AllowCredentials,
		hasExposedHeaders:  len(corsConfig.ExposedHeaders) > 0,
		errorResponseBytes: []byte(`{"error":"CORS policy violation","message":"Origin not allowed"}`),
	}

	cm.precompileConfiguration()

	return cm
}

func (c *CORSMiddleware) Name() string          { return c.name }
func (c *CORSMiddleware) Weight() int           { return c.weight }
func (c *CORSMiddleware) Provider() interface{} { return nil }

func (c *CORSMiddleware) Handle(ctx *types.RequestCtx, next func(*types.RequestCtx), _ *types.RouteConfig) {
	origin := ctx.Request.Header.Peek("Origin")
	if len(origin) == 0 {
		next(ctx)
		return
	}

	if !c.isOriginAllowedFast(origin) {
		c.logger.Warn("CORS request blocked",
			zap.ByteString("origin", origin),
			zap.ByteString("method", ctx.Method()),
			zap.ByteString("path", ctx.Path()))

		c.createCORSErrorResponseFast(ctx)
		return
	}

	if c.isOptionsMethod(ctx.Method()) {
		c.createPreflightResponseFast(ctx, origin)
		return
	}

	c.addCORSHeadersFast(ctx, origin)
	next(ctx)
}

func (c *CORSMiddleware) isOptionsMethod(method []byte) bool {
	return bytes.Equal(method, optionsBytes)
}

func (c *CORSMiddleware) isOriginAllowedFast(origin []byte) bool {
	if c.allowsAll {
		return true
	}

	originStr := string(origin)

	if c.allowedOriginsMap[originStr] {
		return true
	}

	for _, domain := range c.wildcardDomains {
		if c.matchesWildcardDomain(originStr, domain) {
			return true
		}
	}

	return false
}

func (c *CORSMiddleware) matchesWildcardDomain(origin, domain string) bool {
	if origin == domain {
		return true
	}

	suffix := "." + domain
	if strings.HasSuffix(origin, suffix) {
		prefixLen := len(origin) - len(suffix)
		if prefixLen > 0 {
			return origin[prefixLen-1] != '.'
		}
	}

	return false
}

func (c *CORSMiddleware) addCORSHeadersFast(ctx *types.RequestCtx, origin []byte) {
	if c.allowsAll {
		ctx.Response.Header.SetBytesV("Access-Control-Allow-Origin", asteriskBytes)
	} else {
		ctx.Response.Header.SetBytesV("Access-Control-Allow-Origin", origin)
	}

	if c.hasExposedHeaders {
		ctx.Response.Header.SetBytesV("Access-Control-Expose-Headers", c.exposedHeadersStr)
	}

	if c.allowCredentials {
		ctx.Response.Header.SetBytesV("Access-Control-Allow-Credentials", trueBytes)
	}

	ctx.Response.Header.AddBytesV("Vary", varyOriginStr)
}

func (c *CORSMiddleware) createPreflightResponseFast(ctx *types.RequestCtx, origin []byte) {
	ctx.SetStatusCode(fasthttp.StatusOK)

	if c.allowsAll {
		ctx.Response.Header.SetBytesV("Access-Control-Allow-Origin", asteriskBytes)
	} else {
		ctx.Response.Header.SetBytesV("Access-Control-Allow-Origin", origin)
	}

	ctx.Response.Header.SetBytesV("Access-Control-Allow-Methods", c.allowedMethodsStr)
	ctx.Response.Header.SetBytesV("Access-Control-Allow-Headers", c.allowedHeadersStr)
	ctx.Response.Header.SetBytesV("Access-Control-Max-Age", c.maxAgeStr)

	if c.allowCredentials {
		ctx.Response.Header.SetBytesV("Access-Control-Allow-Credentials", trueBytes)
	}

	ctx.Response.Header.SetBytesV("Vary", varyPreflightStr)
	ctx.SetBody(nil)
}

func (c *CORSMiddleware) createCORSErrorResponseFast(ctx *types.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusForbidden)
	ctx.Response.Header.SetContentType("application/json")
	ctx.SetBody(c.errorResponseBytes)
}

func (c *CORSMiddleware) precompileConfiguration() {
	c.allowsAll = len(c.corsConfig.AllowedOrigins) == 1 && c.corsConfig.AllowedOrigins[0] == "*"

	if !c.allowsAll {
		c.allowedOriginsMap = make(map[string]bool, len(c.corsConfig.AllowedOrigins))
		c.wildcardDomains = make([]string, 0)

		for _, origin := range c.corsConfig.AllowedOrigins {
			if strings.HasPrefix(origin, "*.") {
				domain := strings.TrimPrefix(origin, "*.")
				c.wildcardDomains = append(c.wildcardDomains, domain)
			} else {
				c.allowedOriginsMap[origin] = true
			}
		}
	}

	c.allowedMethodsStr = []byte(strings.Join(c.corsConfig.AllowedMethods, ", "))
	c.allowedHeadersStr = []byte(strings.Join(c.corsConfig.AllowedHeaders, ", "))

	if c.hasExposedHeaders {
		c.exposedHeadersStr = []byte(strings.Join(c.corsConfig.ExposedHeaders, ", "))
	}

	c.maxAgeStr = []byte(strconv.Itoa(c.corsConfig.MaxAge))
}
