package types

import (
	"github.com/valyala/fasthttp"
	"time"
)

type HTTPServer interface {
	LifecycleManager
	HandleRequest(ctx *fasthttp.RequestCtx, handler FastHTTPHandler, config *RouteConfig)
}

type HTTPRouter interface {
	FinalizePendingRoutes() error
	Add(method, path string, handler FastHTTPHandler, config *RouteConfig)
	Group(prefix string) GroupBuilder
	Route(method string, path string, handler FastHTTPHandler, gb GroupBuilder) RouteBuilder
	GET(path string, handler FastHTTPHandler) RouteBuilder
	POST(path string, handler FastHTTPHandler) RouteBuilder
	PUT(path string, handler FastHTTPHandler) RouteBuilder
	DELETE(path string, handler FastHTTPHandler) RouteBuilder
	Handler(ctx *fasthttp.RequestCtx, server HTTPServer)
	GetAllRoutes() map[string]*RouteInfo
}

type RouteBuilder interface {
	WithCache(key string, ttl int, dependencies ...string) RouteBuilder
	WithMiddlewares(names ...string) RouteBuilder
	WithoutMiddlewares(names ...string) RouteBuilder
	WithTimeout(duration time.Duration) RouteBuilder
	WithDoc(title, description, tag string, requestType, responseType interface{}) RouteBuilder
	Finalize() error
}

type GroupBuilder interface {
	WithCache(key string, ttl int, dependencies ...string) GroupBuilder
	WithMiddlewares(names ...string) GroupBuilder
	WithoutMiddlewares(names ...string) GroupBuilder
	WithTimeout(duration time.Duration) GroupBuilder
	Route(method, path string, handler FastHTTPHandler) RouteBuilder
	GET(path string, handler FastHTTPHandler) RouteBuilder
	POST(path string, handler FastHTTPHandler) RouteBuilder
	PUT(path string, handler FastHTTPHandler) RouteBuilder
	DELETE(path string, handler FastHTTPHandler) RouteBuilder
	Group(prefix string) GroupBuilder
}

type RouteConfig struct {
	Cache               *CacheHandlerConfig
	Middlewares         []string
	DisabledMiddlewares []string
	Timeout             time.Duration
	Doc                 *DocConfig
}

type RouteInfo struct {
	Handler FastHTTPHandler
	Config  *RouteConfig
}
