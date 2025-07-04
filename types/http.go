package types

import (
	"time"
)

type HTTPServer interface {
	LifecycleManager
}

type HTTPRouter interface {
	Add(method, path string, handler FastHTTPHandler, config *RouteConfig)
	Group(prefix string) GroupBuilder
	GET(path string, handler FastHTTPHandler) RouteBuilder
	POST(path string, handler FastHTTPHandler) RouteBuilder
	PUT(path string, handler FastHTTPHandler) RouteBuilder
	DELETE(path string, handler FastHTTPHandler) RouteBuilder
	GetAllRoutes() map[string]*RouteInfo
}

type RouteBuilder interface {
	WithCache(key string, ttl time.Duration, dependencies ...string) RouteBuilder
	WithMiddlewares(names ...string) RouteBuilder
	WithoutMiddlewares(names ...string) RouteBuilder
	WithTimeout(duration time.Duration) RouteBuilder
	WithDoc(title, description, tag string, requestType, responseType interface{}) RouteBuilder
}

type GroupBuilder interface {
	WithCache(key string, ttl time.Duration, dependencies ...string) GroupBuilder
	WithMiddlewares(names ...string) GroupBuilder
	WithoutMiddlewares(names ...string) GroupBuilder
	WithTimeout(duration time.Duration) GroupBuilder
	Route(method, path string, handler FastHTTPHandler) RouteBuilder
	GET(path string, handler FastHTTPHandler) RouteBuilder
	POST(path string, handler FastHTTPHandler) RouteBuilder
	PATCH(path string, handler FastHTTPHandler) RouteBuilder
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

type RouteDefinition struct {
	Method  string
	Path    string
	Handler FastHTTPHandler
	Config  *RouteConfig
}
