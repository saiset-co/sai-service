package server

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
)

type RouterState int32

const (
	RouterStateStopped RouterState = iota
	RouterStateStarting
	RouterStateRunning
	RouterStateStopping
)

type LazyRoute struct {
	MethodIdx uint8
	Pattern   string
	Handler   types.FastHTTPHandler
	Config    *types.RouteConfig
}

type LazyHTTPRouter struct {
	ctx             context.Context
	cancel          context.CancelFunc
	logger          types.Logger
	staticRoutes    map[string]*types.RouteInfo
	dynamicRoutes   []*LazyRoute
	pendingRoutes   []types.RouteBuilder
	mu              sync.RWMutex
	state           atomic.Value
	shutdownTimeout time.Duration
}

func NewFastHTTPRouter(ctx context.Context, logger types.Logger) (*LazyHTTPRouter, error) {
	routerCtx, cancel := context.WithCancel(ctx)

	router := &LazyHTTPRouter{
		ctx:             routerCtx,
		cancel:          cancel,
		logger:          logger,
		staticRoutes:    make(map[string]*types.RouteInfo),
		dynamicRoutes:   make([]*LazyRoute, 0),
		pendingRoutes:   make([]types.RouteBuilder, 0),
		shutdownTimeout: 10 * time.Second,
	}

	router.state.Store(RouterStateStopped)

	return router, nil
}

func (r *LazyHTTPRouter) Start() error {
	if !r.transitionState(RouterStateStopped, RouterStateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if r.getState() == RouterStateStarting {
			r.setState(RouterStateRunning)
		}
	}()

	if err := r.finalizePendingRoutes(); err != nil {
		r.setState(RouterStateStopped)
		return types.WrapError(err, "failed to finalize pending routes")
	}

	r.logger.Info("HTTP router started",
		zap.Int("static_routes", len(r.staticRoutes)),
		zap.Int("dynamic_routes", len(r.dynamicRoutes)),
		zap.Int("total_routes", len(r.staticRoutes)+len(r.dynamicRoutes)))

	return nil
}

func (r *LazyHTTPRouter) Stop() error {
	if !r.transitionState(RouterStateRunning, RouterStateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		r.setState(RouterStateStopped)
		r.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), r.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			r.mu.Lock()
			r.staticRoutes = make(map[string]*types.RouteInfo)
			r.dynamicRoutes = r.dynamicRoutes[:0]
			r.pendingRoutes = r.pendingRoutes[:0]
			r.mu.Unlock()
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			r.logger.Warn("Router stop timeout, some components may not have stopped gracefully")
		default:
			r.logger.Error("Error during router shutdown", zap.Error(err))
		}
	} else {
		r.logger.Info("HTTP router stopped gracefully")
	}

	return nil
}

func (r *LazyHTTPRouter) IsRunning() bool {
	return r.getState() == RouterStateRunning
}

func (r *LazyHTTPRouter) getState() RouterState {
	return r.state.Load().(RouterState)
}

func (r *LazyHTTPRouter) setState(newState RouterState) bool {
	currentState := r.getState()
	return r.state.CompareAndSwap(currentState, newState)
}

func (r *LazyHTTPRouter) transitionState(from, to RouterState) bool {
	return r.state.CompareAndSwap(from, to)
}

func (r *LazyHTTPRouter) Add(method, path string, handler types.FastHTTPHandler, config *types.RouteConfig) {
	methodIdx, exists := methodIndex[method]
	if !exists {
		r.logger.Warn("Unknown HTTP method", zap.String("method", method))
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !strings.Contains(path, "{") && !strings.Contains(path, ":") {
		key := method + ":" + path
		r.staticRoutes[key] = &types.RouteInfo{
			Handler: handler,
			Config:  config,
		}
		return
	}

	route := &LazyRoute{
		MethodIdx: methodIdx,
		Pattern:   path,
		Handler:   handler,
		Config:    config,
	}

	r.dynamicRoutes = append(r.dynamicRoutes, route)
}

func (r *LazyHTTPRouter) route(method string, path string, handler types.FastHTTPHandler, gb types.GroupBuilder) types.RouteBuilder {
	rb := &RouteBuilder{
		router:     r,
		method:     method,
		path:       path,
		handler:    handler,
		config:     &types.RouteConfig{},
		routeGroup: gb,
	}

	r.addPendingRoute(rb)

	return rb
}

func (r *LazyHTTPRouter) addPendingRoute(rb types.RouteBuilder) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pendingRoutes = append(r.pendingRoutes, rb)
}

func (r *LazyHTTPRouter) Group(prefix string) types.GroupBuilder {
	return &GroupBuilder{
		router: r,
		prefix: prefix,
		config: &types.RouteConfig{},
	}
}

func (r *LazyHTTPRouter) finalizePendingRoutes() error {
	r.mu.Lock()
	routeCount := len(r.pendingRoutes)
	r.mu.Unlock()

	if routeCount == 0 {
		r.logger.Info("No pending routes to finalize")
		return nil
	}

	r.mu.Lock()
	routes := make([]types.RouteBuilder, len(r.pendingRoutes))
	copy(routes, r.pendingRoutes)
	r.pendingRoutes = r.pendingRoutes[:0]
	r.mu.Unlock()

	ctx, cancel := context.WithTimeout(r.ctx, 30*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	var successCount int32
	var errorCount int32

	for _, route := range routes {
		rt := route
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if err := rt.(*RouteBuilder).Finalize(); err != nil {
					atomic.AddInt32(&errorCount, 1)
					r.logger.Error("Failed to finalize route",
						zap.Error(err))
					return err
				} else {
					atomic.AddInt32(&successCount, 1)
					return nil
				}
			}
		})
	}

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			return types.NewErrorf("route finalization timeout, processed %d/%d routes",
				atomic.LoadInt32(&successCount), len(routes))
		default:
			if atomic.LoadInt32(&successCount) > 0 {
				r.logger.Warn("Some routes failed to finalize",
					zap.Int32("success_count", atomic.LoadInt32(&successCount)),
					zap.Int32("error_count", atomic.LoadInt32(&errorCount)),
					zap.Error(err))
			} else {
				return types.Errorf(types.ErrRouteFinalizationFailed, "%d errors occurred", atomic.LoadInt32(&errorCount))
			}
		}
	}

	return nil
}

func (r *LazyHTTPRouter) GetCompiledRoutes() (map[string]*types.RouteInfo, []*LazyRoute) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	staticRoutes := make(map[string]*types.RouteInfo, len(r.staticRoutes))
	for key, info := range r.staticRoutes {
		staticRoutes[key] = info
	}

	dynamicRoutes := make([]*LazyRoute, len(r.dynamicRoutes))
	copy(dynamicRoutes, r.dynamicRoutes)

	return staticRoutes, dynamicRoutes
}

func (r *LazyHTTPRouter) GetAllRoutes() map[string]*types.RouteInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := make(map[string]*types.RouteInfo)

	for key, info := range r.staticRoutes {
		routes[key] = info
	}

	for _, route := range r.dynamicRoutes {
		methodName := r.getMethodName(route.MethodIdx)
		key := methodName + ":" + route.Pattern
		routes[key] = &types.RouteInfo{
			Handler: route.Handler,
			Config:  route.Config,
		}
	}

	return routes
}

func (r *LazyHTTPRouter) getMethodName(methodIdx uint8) string {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "TRACE"}
	if int(methodIdx) < len(methods) {
		return methods[methodIdx]
	}
	return "UNKNOWN"
}

func (r *LazyHTTPRouter) GET(path string, handler types.FastHTTPHandler) types.RouteBuilder {
	return r.route("GET", path, handler, nil)
}

func (r *LazyHTTPRouter) POST(path string, handler types.FastHTTPHandler) types.RouteBuilder {
	return r.route("POST", path, handler, nil)
}

func (r *LazyHTTPRouter) PUT(path string, handler types.FastHTTPHandler) types.RouteBuilder {
	return r.route("PUT", path, handler, nil)
}

func (r *LazyHTTPRouter) DELETE(path string, handler types.FastHTTPHandler) types.RouteBuilder {
	return r.route("DELETE", path, handler, nil)
}

func (r *LazyHTTPRouter) PATCH(path string, handler types.FastHTTPHandler) types.RouteBuilder {
	return r.route("PATCH", path, handler, nil)
}

func (r *LazyHTTPRouter) HEAD(path string, handler types.FastHTTPHandler) types.RouteBuilder {
	return r.route("HEAD", path, handler, nil)
}

func (r *LazyHTTPRouter) OPTIONS(path string, handler types.FastHTTPHandler) types.RouteBuilder {
	return r.route("OPTIONS", path, handler, nil)
}
