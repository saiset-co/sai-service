package auth_providers

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/saiset-co/sai-service/types"
)

const basicAuthCookieName = "_sai_auth"
const basicAuthCookieDefaultTTL = 24 * time.Hour

type session struct {
	username string
	expireAt time.Time
}

type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*session
	ttl      time.Duration
}

func newSessionStore(ttl time.Duration) *sessionStore {
	s := &sessionStore{
		sessions: make(map[string]*session),
		ttl:      ttl,
	}
	go s.cleanupLoop()
	return s
}

func (s *sessionStore) create(username string) string {
	b := make([]byte, 32)
	rand.Read(b)
	token := base64.RawURLEncoding.EncodeToString(b)
	s.mu.Lock()
	s.sessions[token] = &session{username: username, expireAt: time.Now().Add(s.ttl)}
	s.mu.Unlock()
	return token
}

func (s *sessionStore) touch(token string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[token]
	if !ok || time.Now().After(sess.expireAt) {
		delete(s.sessions, token)
		return "", false
	}
	sess.expireAt = time.Now().Add(s.ttl)
	return sess.username, true
}


func (s *sessionStore) cleanupLoop() {
	ticker := time.NewTicker(s.ttl / 2)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for token, sess := range s.sessions {
			if now.After(sess.expireAt) {
				delete(s.sessions, token)
			}
		}
		s.mu.Unlock()
	}
}

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

	if token, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
		return token
	}

	if token, ok := strings.CutPrefix(authHeader, "Token "); ok {
		return token
	}

	return authHeader
}

type BasicAuthProvider struct {
	username string
	password string
	realm    string
	store    *sessionStore
	ttl      time.Duration
}

func NewBasicAuthProvider(username, password string, cookieTTL time.Duration) *BasicAuthProvider {
	if cookieTTL <= 0 {
		cookieTTL = basicAuthCookieDefaultTTL
	}
	return &BasicAuthProvider{
		username: username,
		password: password,
		realm:    "Protected Area",
		store:    newSessionStore(cookieTTL),
		ttl:      cookieTTL,
	}
}

func (p *BasicAuthProvider) Type() string {
	return "basic"
}

func (p *BasicAuthProvider) ApplyToIncomingRequest(ctx *types.RequestCtx) error {
	if token := string(ctx.Request.Header.Cookie(basicAuthCookieName)); token != "" {
		if username, ok := p.store.touch(token); ok {
			ctx.SetUserValue("authenticated_user", username)
			ctx.SetUserValue("auth_type", "basic")
			p.setSessionCookie(ctx, token)
			return nil
		}
	}

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

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return p.sendAuthChallenge(ctx, "Invalid authentication format")
	}

	if parts[0] != p.username || parts[1] != p.password {
		return p.sendAuthChallenge(ctx, "Invalid username or password")
	}

	ctx.SetUserValue("authenticated_user", parts[0])
	ctx.SetUserValue("auth_type", "basic")

	token := p.store.create(parts[0])
	p.setSessionCookie(ctx, token)

	return nil
}

func (p *BasicAuthProvider) setSessionCookie(ctx *types.RequestCtx, token string) {
	var cookie fasthttp.Cookie
	cookie.SetKey(basicAuthCookieName)
	cookie.SetValue(token)
	cookie.SetPath("/")
	cookie.SetHTTPOnly(true)
	cookie.SetExpire(time.Now().Add(p.ttl))
	ctx.Response.Header.SetCookie(&cookie)
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
