package types

import "github.com/valyala/fasthttp"

type MiddlewareManager interface {
	RegisterMiddlewares() error
	Register(middleware Middleware) error
	Execute(ctx *fasthttp.RequestCtx, handler func(*fasthttp.RequestCtx), config *RouteConfig)
	Clear()
}

type Middleware interface {
	Handle(ctx *fasthttp.RequestCtx, next func(*fasthttp.RequestCtx), config *RouteConfig)
	Name() string
	Weight() int
}

type MiddlewareEntry struct {
	Name       string
	Middleware Middleware
	Weight     int
}
