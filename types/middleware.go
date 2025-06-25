package types

type MiddlewareManager interface {
	LifecycleManager
	Register(middleware Middleware) error
	Execute(ctx *RequestCtx, handler func(*RequestCtx), config *RouteConfig)
}

type Middleware interface {
	Handle(ctx *RequestCtx, next func(*RequestCtx), config *RouteConfig)
	Name() string
	Weight() int
}

type MiddlewareEntry struct {
	Name       string
	Middleware Middleware
	Weight     int
}
