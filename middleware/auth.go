package middleware

import (
	"bytes"
	"github.com/valyala/fasthttp"

	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/auth_providers"
	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

var (
	challengeError = []byte("basic_auth_challenge_sent")
	optionsMethod  = []byte("OPTIONS")
	successMsg     = "Authentication successful"
	challengeMsg   = "Basic auth challenge sent to browser"
	failedMsg      = "Authentication failed"
)

type AuthMiddleware struct {
	config     types.ConfigManager
	logger     types.Logger
	metrics    types.MetricsManager
	provider   types.AuthProvider
	authConfig *AuthConfig
	name       string
	weight     int
}

type AuthConfig struct {
	Provider string `json:"provider"`
}

func NewAuthMiddleware(provider types.AuthProviderManager, config types.ConfigManager, logger types.Logger, metrics types.MetricsManager) (*AuthMiddleware, error) {
	var authConfig = &AuthConfig{
		Provider: "token",
	}

	if config.GetConfig().Middlewares.Auth.Params != nil {
		err := utils.UnmarshalConfig(config.GetConfig().Middlewares.Auth.Params, authConfig)
		if err != nil {
			logger.Error("Failed to unmarshal Auth middleware config", zap.Error(err))
			return nil, err
		}
	}

	authProvider, err := provider.(*auth_providers.AuthProviderManager).GetProvider(authConfig.Provider)
	if err != nil {
		logger.Error("Failed to get auth provider", zap.Error(err))
		return nil, err
	}

	am := &AuthMiddleware{
		name:       "auth",
		config:     config,
		logger:     logger,
		metrics:    metrics,
		provider:   authProvider,
		authConfig: authConfig,
		weight:     config.GetConfig().Middlewares.Auth.Weight,
	}

	return am, nil
}

func (a *AuthMiddleware) Name() string          { return a.name }
func (a *AuthMiddleware) Weight() int           { return a.weight }
func (a *AuthMiddleware) Provider() interface{} { return a.provider }

func (a *AuthMiddleware) Handle(ctx *types.RequestCtx, next func(*types.RequestCtx), config *types.RouteConfig) {
	if bytes.Equal(ctx.Method(), optionsMethod) {
		next(ctx)
		return
	}

	if a.isDisabledPath(config) {
		next(ctx)
		return
	}
	err := a.provider.ApplyToIncomingRequest(ctx)

	if err == nil {
		a.logger.Debug(successMsg,
			zap.ByteString("path", ctx.Path()),
			zap.String("provider_type", a.provider.Type()))
		next(ctx)
		return
	}

	if a.isBasicAuthChallenge(err) {
		a.logger.Debug(challengeMsg,
			zap.ByteString("path", ctx.Path()))
		return
	}

	a.logger.Warn(failedMsg,
		zap.ByteString("path", ctx.Path()),
		zap.String("provider_type", a.provider.Type()),
		zap.Error(err))

	ctx.Error(types.NewError("Authentication require"), fasthttp.StatusUnauthorized)
}

func (a *AuthMiddleware) isBasicAuthChallenge(err error) bool {
	return bytes.Contains([]byte(err.Error()), challengeError)
}

func (a *AuthMiddleware) isDisabledPath(config *types.RouteConfig) bool {
	for _, middleware := range config.DisabledMiddlewares {
		if middleware == a.name {
			return true
		}
	}

	return false
}
