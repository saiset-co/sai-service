package types

import "github.com/valyala/fasthttp"

type AuthProviderManager interface {
	Register(name string, provider AuthProvider) error
	GetProvider(name string) (AuthProvider, error)
}

type AuthProvider interface {
	Type() string
	ApplyToIncomingRequest(ctx *RequestCtx) error
	ApplyToOutgoingRequest(req *fasthttp.Request, authConfig *ServiceAuthConfig) error
}
