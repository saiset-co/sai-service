package server

import (
	"time"

	"github.com/saiset-co/sai-service/types"
)

type GroupBuilder struct {
	router *LazyHTTPRouter
	prefix string
	config *types.RouteConfig
}

func (gb *GroupBuilder) WithCache(key string, ttl time.Duration, dependencies ...string) types.GroupBuilder {
	gb.config.Cache = &types.CacheHandlerConfig{
		Enabled: true,
		Key:     key,
		TTL:     ttl,
		Deps:    dependencies,
	}
	return gb
}

func (gb *GroupBuilder) WithMiddlewares(names ...string) types.GroupBuilder {
	gb.config.Middlewares = append(gb.config.Middlewares, names...)
	return gb
}

func (gb *GroupBuilder) WithoutMiddlewares(names ...string) types.GroupBuilder {
	gb.config.DisabledMiddlewares = append(gb.config.DisabledMiddlewares, names...)
	return gb
}

func (gb *GroupBuilder) WithTimeout(duration time.Duration) types.GroupBuilder {
	gb.config.Timeout = duration
	return gb
}

func (gb *GroupBuilder) Route(method, path string, handler types.FastHTTPHandler) types.RouteBuilder {
	rb := gb.router.route(method, gb.prefix+path, handler, gb)

	if gb.config != nil {
		routeBuilder := rb.(*RouteBuilder)

		if gb.config.Cache != nil {
			routeBuilder.config.Cache = gb.config.Cache
		}
		if gb.config.Timeout > 0 {
			routeBuilder.config.Timeout = gb.config.Timeout
		}
		if gb.config.Doc != nil {
			routeBuilder.config.Doc = gb.config.Doc
		}

		routeBuilder.config.Middlewares = append(routeBuilder.config.Middlewares, gb.config.Middlewares...)
		routeBuilder.config.DisabledMiddlewares = append(routeBuilder.config.DisabledMiddlewares, gb.config.DisabledMiddlewares...)
	}

	return rb
}

func (gb *GroupBuilder) GET(path string, handler types.FastHTTPHandler) types.RouteBuilder {
	return gb.Route("GET", path, handler)
}

func (gb *GroupBuilder) POST(path string, handler types.FastHTTPHandler) types.RouteBuilder {
	return gb.Route("POST", path, handler)
}

func (gb *GroupBuilder) PUT(path string, handler types.FastHTTPHandler) types.RouteBuilder {
	return gb.Route("PUT", path, handler)
}

func (gb *GroupBuilder) DELETE(path string, handler types.FastHTTPHandler) types.RouteBuilder {
	return gb.Route("DELETE", path, handler)
}

func (gb *GroupBuilder) Group(prefix string) types.GroupBuilder {
	return &GroupBuilder{
		router: gb.router,
		prefix: gb.prefix + prefix,
		config: &types.RouteConfig{},
	}
}
