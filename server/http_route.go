package server

import (
	"reflect"
	"time"

	"github.com/saiset-co/sai-service/types"
)

const maxMiddlewareSliceSize = 100

type RouteBuilder struct {
	router     types.HTTPRouter
	method     string
	path       string
	handler    types.FastHTTPHandler
	config     *types.RouteConfig
	routeGroup types.GroupBuilder
}

func (rb *RouteBuilder) WithCache(key string, ttl int, dependencies ...string) types.RouteBuilder {
	rb.config.Cache = &types.CacheHandlerConfig{
		Enabled: true,
		Key:     key,
		TTL:     ttl,
		Deps:    dependencies,
	}
	return rb
}

func (rb *RouteBuilder) WithMiddlewares(names ...string) types.RouteBuilder {
	rb.config.Middlewares = append(rb.config.Middlewares, names...)
	return rb
}

func (rb *RouteBuilder) WithoutMiddlewares(names ...string) types.RouteBuilder {
	rb.config.DisabledMiddlewares = append(rb.config.DisabledMiddlewares, names...)
	return rb
}

func (rb *RouteBuilder) WithTimeout(duration time.Duration) types.RouteBuilder {
	rb.config.Timeout = duration
	return rb
}

func (rb *RouteBuilder) WithDoc(title, description, tag string, requestType, responseType interface{}) types.RouteBuilder {
	rb.config.Doc = &types.DocConfig{
		DocTitle:        title,
		DocDescription:  description,
		DocTag:          tag,
		DocRequestType:  reflect.TypeOf(requestType),
		DocResponseType: reflect.TypeOf(responseType),
	}
	return rb
}

func (rb *RouteBuilder) Finalize() error {
	return rb.finalize()
}

func (rb *RouteBuilder) finalize() error {
	if len(rb.config.Middlewares) > maxMiddlewareSliceSize {
		return types.ErrMiddlewareOrderInvalid
	}

	if len(rb.config.DisabledMiddlewares) > maxMiddlewareSliceSize {
		return types.ErrMiddlewareOrderInvalid
	}

	if rb.config.Doc != nil {
		rb.config.Doc.Path = rb.path
		rb.config.Doc.Method = rb.method
	}

	configCopy := &types.RouteConfig{
		Cache:               rb.config.Cache,
		Middlewares:         make([]string, len(rb.config.Middlewares)),
		DisabledMiddlewares: make([]string, len(rb.config.DisabledMiddlewares)),
		Timeout:             rb.config.Timeout,
		Doc:                 rb.config.Doc,
	}

	if len(rb.config.Middlewares) > 0 {
		configCopy.Middlewares = append([]string(nil), rb.config.Middlewares...)
	}
	if len(rb.config.DisabledMiddlewares) > 0 {
		configCopy.DisabledMiddlewares = append([]string(nil), rb.config.DisabledMiddlewares...)
	}

	rb.router.Add(rb.method, rb.path, rb.handler, configCopy)

	return nil
}
