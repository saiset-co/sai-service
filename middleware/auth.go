package middleware

import (
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type AuthMiddleware struct {
	config     types.ConfigManager
	logger     types.Logger
	metrics    types.MetricsManager
	authConfig *AuthConfig
}

type AuthConfig struct {
	Token string `json:"token"`
}

func NewAuthMiddleware(config types.ConfigManager, logger types.Logger, metrics types.MetricsManager) *AuthMiddleware {
	var authConfig = &AuthConfig{}

	if config.GetConfig().Middlewares.Auth.Params != nil {
		err := utils.UnmarshalConfig(config.GetConfig().Middlewares.Auth.Params, authConfig)
		if err != nil {
			logger.Error("Failed to unmarshal Auth middleware config", zap.Error(err))
		}
	}

	return &AuthMiddleware{
		config:     config,
		logger:     logger,
		metrics:    metrics,
		authConfig: authConfig,
	}
}

func (a *AuthMiddleware) Name() string { return "auth" }
func (a *AuthMiddleware) Weight() int  { return 50 }

func (a *AuthMiddleware) Handle(ctx *fasthttp.RequestCtx, next func(*fasthttp.RequestCtx), _ *types.RouteConfig) {
	start := time.Now()
	token := a.extractToken(ctx)
	authenticated := a.validateToken(token)
	duration := time.Since(start)

	if authenticated {
		a.logger.Debug("Authentication successful",
			zap.String("path", string(ctx.Path())),
			zap.Duration("duration", duration))

		next(ctx)
		return
	}

	a.logger.Warn("Authentication failed",
		zap.String("path", string(ctx.Path())),
		zap.String("remote_addr", ctx.RemoteIP().String()),
		zap.Duration("duration", duration))

	utils.CreateUnauthorizedResponse(ctx)
}

func (a *AuthMiddleware) extractToken(ctx *fasthttp.RequestCtx) string {
	authHeader := string(ctx.Request.Header.Peek("Authorization"))
	if authHeader == "" {
		return string(ctx.Request.Header.Peek("X-API-Key"))
	}

	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	if strings.HasPrefix(authHeader, "Token ") {
		return strings.TrimPrefix(authHeader, "Token ")
	}

	return authHeader
}

func (a *AuthMiddleware) validateToken(token string) bool {
	if a.authConfig.Token == "" || token == "" {
		return false
	}

	return token == a.authConfig.Token
}
