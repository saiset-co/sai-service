package auth_providers

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/saiset-co/sai-service/types"
)

type TokenAuthProvider struct {
	token string
}

func NewTokenAuthProvider(token string) *TokenAuthProvider {
	return &TokenAuthProvider{
		token: token,
	}
}

func (p *TokenAuthProvider) Type() string {
	return "token"
}

func (p *TokenAuthProvider) ApplyToIncomingRequest(ctx *types.RequestCtx) error {
	token := p.extractToken(ctx)
	if token != p.token {
		return errors.New("invalid Token")
	}
	return nil
}

func (p *TokenAuthProvider) ApplyToOutgoingRequest(req *fasthttp.Request, authConfig *types.ServiceAuthConfig) error {
	if authConfig == nil || authConfig.Payload == nil {
		return errors.New("auth config is required for API key authentication")
	}

	token, ok := authConfig.Payload["token"].(string)
	if !ok {
		return errors.New("token not found in auth payload")
	}

	req.Header.Set("Token", token)
	return nil
}

func (p *TokenAuthProvider) extractToken(ctx *types.RequestCtx) string {
	if token := string(ctx.Request.Header.Peek("Token")); token != "" {
		return token
	}

	authHeader := string(ctx.Request.Header.Peek("Authorization"))
	if authHeader == "" {
		return ""
	}

	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	if strings.HasPrefix(authHeader, "Token ") {
		return strings.TrimPrefix(authHeader, "Token ")
	}

	return authHeader
}

type BasicAuthProvider struct {
	username string
	password string
	realm    string
}

func NewBasicAuthProvider(username, password string) *BasicAuthProvider {
	return &BasicAuthProvider{
		username: username,
		password: password,
		realm:    "Protected Area",
	}
}

func (p *BasicAuthProvider) Type() string {
	return "basic"
}

func (p *BasicAuthProvider) ApplyToIncomingRequest(ctx *types.RequestCtx) error {
	authHeader := string(ctx.Request.Header.Peek("Authorization"))

	if authHeader == "" {
		return p.sendAuthChallenge(ctx, "Authorization header required")
	}

	if !strings.HasPrefix(authHeader, "Basic ") {
		return p.sendAuthChallenge(ctx, "Basic authentication required")
	}

	encoded := strings.TrimPrefix(authHeader, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return p.sendAuthChallenge(ctx, "Invalid authentication encoding")
	}

	credentials := string(decoded)
	parts := strings.SplitN(credentials, ":", 2)
	if len(parts) != 2 {
		return p.sendAuthChallenge(ctx, "Invalid authentication format")
	}

	username, password := parts[0], parts[1]

	if username != p.username || password != p.password {
		return p.sendAuthChallenge(ctx, "Invalid username or password")
	}

	ctx.SetUserValue("authenticated_user", username)
	ctx.SetUserValue("auth_type", "basic")

	return nil
}

func (p *BasicAuthProvider) sendAuthChallenge(ctx *types.RequestCtx, message string) error {
	ctx.SetStatusCode(fasthttp.StatusUnauthorized)

	authHeader := fmt.Sprintf(`Basic realm="%s"`, p.realm)
	ctx.Response.Header.Set("WWW-Authenticate", authHeader)
	ctx.Response.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Response.Header.Set("Pragma", "no-cache")
	ctx.Response.Header.Set("Expires", "0")

	ctx.SetContentType("application/json")

	response := fmt.Sprintf(`{
		"error": "Authentication Required",
		"message": "%s",
		"realm": "%s",
		"type": "basic_auth_challenge"
	}`, message, p.realm)

	ctx.SetBodyString(response)

	return errors.New("basic_auth_challenge_sent")
}

func (p *BasicAuthProvider) ApplyToOutgoingRequest(req *fasthttp.Request, authConfig *types.ServiceAuthConfig) error {
	if authConfig == nil || authConfig.Payload == nil {
		return errors.New("auth config is required for basic authentication")
	}

	username, okUser := authConfig.Payload["username"].(string)
	password, okPass := authConfig.Payload["password"].(string)

	if !okUser || !okPass {
		return errors.New("username and password not found in auth payload")
	}

	credentials := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	req.Header.Set("Authorization", "Basic "+credentials)

	return nil
}

func (p *BasicAuthProvider) SetRealm(realm string) {
	p.realm = realm
}

func (p *BasicAuthProvider) GetRealm() string {
	return p.realm
}
