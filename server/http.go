package server

import (
	"bytes"
	"context"
	"fmt"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type State int32

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
)

var methodIndex = map[string]uint8{
	"GET":     0,
	"POST":    1,
	"PUT":     2,
	"DELETE":  3,
	"PATCH":   4,
	"HEAD":    5,
	"OPTIONS": 6,
	"TRACE":   7,
}

var (
	getBytes     = []byte("GET")
	postBytes    = []byte("POST")
	putBytes     = []byte("PUT")
	deleteBytes  = []byte("DELETE")
	patchBytes   = []byte("PATCH")
	headBytes    = []byte("HEAD")
	optionsBytes = []byte("OPTIONS")
	traceBytes   = []byte("TRACE")
)

type CompiledRoute struct {
	methodIdx  uint8
	pattern    string
	handler    types.FastHTTPHandler
	config     *types.RouteConfig
	paramNames []string
	segments   []string
}

type FastHTTPServer struct {
	ctx             context.Context
	cancel          context.CancelFunc
	config          types.ConfigManager
	logger          types.Logger
	metrics         types.MetricsManager
	middlewares     types.MiddlewareManager
	router          types.HTTPRouter
	server          *fasthttp.Server
	listener        net.Listener
	httpConfig      *types.HTTPConfig
	tlsConfig       *types.TLSConfig
	tlsManager      types.TLSManager
	state           atomic.Value
	shutdownTimeout time.Duration
	staticRoutes    map[string]*types.RouteInfo
	compiledRoutes  []*CompiledRoute
	routingMu       sync.RWMutex
}

func NewHTTPServer(
	ctx context.Context,
	config types.ConfigManager,
	logger types.Logger,
	metrics types.MetricsManager,
	middlewares types.MiddlewareManager,
	tlsManager types.TLSManager,
	router types.HTTPRouter) (*FastHTTPServer, error) {
	serverCtx, cancel := context.WithCancel(ctx)

	server := &FastHTTPServer{
		ctx:             serverCtx,
		cancel:          cancel,
		config:          config,
		logger:          logger,
		metrics:         metrics,
		middlewares:     middlewares,
		tlsManager:      tlsManager,
		router:          router,
		httpConfig:      config.GetConfig().Server.HTTP,
		tlsConfig:       config.GetConfig().Server.TLS,
		shutdownTimeout: 5 * time.Second,
		staticRoutes:    make(map[string]*types.RouteInfo),
		compiledRoutes:  make([]*CompiledRoute, 0),
	}

	server.state.Store(StateStopped)

	return server, nil
}

func (h *FastHTTPServer) Start() error {
	if !h.transitionState(StateStopped, StateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if h.getState() == StateStarting {
			h.setState(StateRunning)
		}
	}()

	if err := h.compileRoutes(); err != nil {
		h.setState(StateStopped)
		return types.WrapError(err, "failed to compile routes")
	}

	h.server = &fasthttp.Server{
		Handler:                       h.mainHandler(),
		ReadTimeout:                   time.Duration(h.httpConfig.ReadTimeout) * time.Second,
		WriteTimeout:                  time.Duration(h.httpConfig.WriteTimeout) * time.Second,
		IdleTimeout:                   time.Duration(h.httpConfig.IdleTimeout) * time.Second,
		MaxConnsPerIP:                 100000000,
		MaxRequestsPerConn:            100000000,
		Concurrency:                   1000000,
		TCPKeepalive:                  true,
		ReduceMemoryUsage:             false,
		DisablePreParseMultipartForm:  true,
		NoDefaultServerHeader:         false,
		NoDefaultDate:                 false,
		NoDefaultContentType:          false,
		KeepHijackedConns:             false,
		CloseOnShutdown:               true,
		StreamRequestBody:             false,
		DisableHeaderNamesNormalizing: false,
	}

	addr := fmt.Sprintf("%s:%d", h.httpConfig.Host, h.httpConfig.Port)

	go func() {
		var err error
		if h.tlsConfig.Enabled {
			h.listener, err = h.tlsManager.Serve(addr)
			if err != nil {
				h.logger.Error("TLS HTTP server failed", zap.Error(err))
				return
			}
			err = h.server.Serve(h.listener)
		} else {
			h.listener, err = net.Listen("tcp", addr)
			if err != nil {
				h.logger.Error("HTTP listener failed", zap.Error(err))
				return
			}
			err = h.server.Serve(h.listener)
		}

		if err != nil {
			h.logger.Error("HTTP server failed", zap.Error(err))
			h.setState(StateStopped)
		}
	}()

	h.logger.Info("HTTP server started successfully",
		zap.String("address", addr),
		zap.Bool("tls", h.tlsConfig.Enabled))

	return nil
}

func (h *FastHTTPServer) Stop() error {
	if !h.transitionState(StateRunning, StateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		h.setState(StateStopped)
		h.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), h.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if h.server != nil {
			if h.listener != nil {
				if err := h.listener.Close(); err != nil {
					h.logger.Error("Failed to close listener", zap.Error(err))
				}
			}

			if err := h.server.ShutdownWithContext(ctx); err != nil {
				return nil
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		select {
		case <-gCtx.Done():
			h.logger.Warn("Server stop timeout, some components may not have stopped gracefully")
		default:
			h.logger.Error("Error during server shutdown", zap.Error(err))
		}
	} else {
		h.logger.Info("HTTP server stopped gracefully")
	}

	return nil
}

func (h *FastHTTPServer) IsRunning() bool {
	return h.getState() == StateRunning
}

func (h *FastHTTPServer) getState() State {
	return h.state.Load().(State)
}

func (h *FastHTTPServer) setState(newState State) bool {
	currentState := h.getState()
	return h.state.CompareAndSwap(currentState, newState)
}

func (h *FastHTTPServer) transitionState(from, to State) bool {
	return h.state.CompareAndSwap(from, to)
}

func (h *FastHTTPServer) compileRoutes() error {
	lazyRouter := h.router.(*LazyHTTPRouter)
	staticRoutes, dynamicRoutes := lazyRouter.GetCompiledRoutes()

	h.routingMu.Lock()
	defer h.routingMu.Unlock()

	h.staticRoutes = make(map[string]*types.RouteInfo, len(staticRoutes))
	for key, info := range staticRoutes {
		h.staticRoutes[key] = info
	}

	h.compiledRoutes = make([]*CompiledRoute, 0, len(dynamicRoutes))
	for _, route := range dynamicRoutes {
		compiled := &CompiledRoute{
			methodIdx:  route.MethodIdx,
			pattern:    route.Pattern,
			handler:    route.Handler,
			config:     route.Config,
			paramNames: h.extractParamNames(route.Pattern),
			segments:   h.parsePathSegments(route.Pattern),
		}
		h.compiledRoutes = append(h.compiledRoutes, compiled)
	}

	return nil
}

func (h *FastHTTPServer) mainHandler() fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		methodBytes := ctx.Method()
		pathBytes := ctx.Path()

		if handler, config := h.findStaticRoute(methodBytes, pathBytes); handler != nil {
			h.executeHandler(&types.RequestCtx{RequestCtx: ctx}, handler, config)
			return
		}

		if handler, config, params := h.findDynamicRoute(methodBytes, pathBytes); handler != nil {
			if params != nil {
				for name, value := range params {
					ctx.SetUserValue(name, value)
				}
			}
			h.executeHandler(&types.RequestCtx{RequestCtx: ctx}, handler, config)
			return
		}

		if string(methodBytes) == "OPTIONS" {
			h.executeHandler(&types.RequestCtx{RequestCtx: ctx}, func(ctx *types.RequestCtx) {}, &types.RouteConfig{})
			return
		}

		ctx.Error("Not found", fasthttp.StatusNotFound)
	}
}

func (h *FastHTTPServer) findStaticRoute(method, path []byte) (types.FastHTTPHandler, *types.RouteConfig) {
	if bytes.ContainsAny(path, "{}:") {
		return nil, nil
	}

	path = normalizePathBytes(path)

	var buf [32]byte
	n := copy(buf[:], method)
	buf[n] = ':'
	copy(buf[n+1:], path)

	h.routingMu.RLock()
	info := h.staticRoutes[string(buf[:n+1+len(path)])]
	h.routingMu.RUnlock()

	if info != nil {
		return info.Handler, info.Config
	}
	return nil, nil
}

func (h *FastHTTPServer) findDynamicRoute(method, path []byte) (types.FastHTTPHandler, *types.RouteConfig, map[string]string) {
	methodIdx := h.getMethodIndex(method)
	if methodIdx == 255 {
		return nil, nil, nil
	}

	path = normalizePathBytes(path)
	pathStr := utils.BytesToString(path)
	pathSegments := h.parsePathSegments(pathStr)

	h.routingMu.RLock()
	defer h.routingMu.RUnlock()

	for _, route := range h.compiledRoutes {
		if route.methodIdx == methodIdx {
			if params := h.matchRoute(pathSegments, route); params != nil {
				return route.handler, route.config, params
			}
		}
	}

	return nil, nil, nil
}

func (h *FastHTTPServer) getMethodIndex(method []byte) uint8 {
	switch {
	case bytes.Equal(method, getBytes):
		return 0
	case bytes.Equal(method, postBytes):
		return 1
	case bytes.Equal(method, putBytes):
		return 2
	case bytes.Equal(method, deleteBytes):
		return 3
	case bytes.Equal(method, patchBytes):
		return 4
	case bytes.Equal(method, headBytes):
		return 5
	case bytes.Equal(method, optionsBytes):
		return 6
	case bytes.Equal(method, traceBytes):
		return 7
	default:
		return 255
	}
}

func (h *FastHTTPServer) parsePathSegments(path string) []string {
	if path == "/" {
		return []string{}
	}

	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}

	return strings.Split(path, "/")
}

func (h *FastHTTPServer) extractParamNames(pattern string) []string {
	segments := h.parsePathSegments(pattern)
	var params []string

	for _, seg := range segments {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			params = append(params, seg[1:len(seg)-1])
		} else if strings.HasPrefix(seg, ":") {
			params = append(params, seg[1:])
		}
	}

	return params
}

func (h *FastHTTPServer) matchRoute(pathSegments []string, route *CompiledRoute) map[string]string {
	if len(pathSegments) != len(route.segments) {
		return nil
	}

	var params map[string]string
	paramIdx := 0

	for i, routeSegment := range route.segments {
		if strings.HasPrefix(routeSegment, "{") || strings.HasPrefix(routeSegment, ":") {
			if params == nil {
				params = make(map[string]string, len(route.paramNames))
			}
			if paramIdx < len(route.paramNames) {
				params[route.paramNames[paramIdx]] = pathSegments[i]
				paramIdx++
			}
		} else if routeSegment != pathSegments[i] {
			return nil
		}
	}

	return params
}

func (h *FastHTTPServer) executeHandler(ctx *types.RequestCtx, handler types.FastHTTPHandler, config *types.RouteConfig) {
	if handler == nil {
		ctx.Error(types.ErrPathNotFound, fasthttp.StatusNotFound)
		return
	}

	if config == nil {
		ctx.Error(types.ErrConfigNotFound, fasthttp.StatusInternalServerError)
		return
	}

	if h.middlewares != nil {
		finalHandler := func(ctx *types.RequestCtx) {
			handler(ctx)
		}
		h.middlewares.Execute(ctx, finalHandler, config)
	} else {
		handler(ctx)
	}
}
