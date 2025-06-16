package server

import (
	"bytes"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/valyala/fasthttp"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
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
	getBytes    = []byte("GET")
	postBytes   = []byte("POST")
	putBytes    = []byte("PUT")
	deleteBytes = []byte("DELETE")
	patchBytes  = []byte("PATCH")
)

type FastHTTPRouter struct {
	root          *RouteNode
	staticRoutes  map[string]*types.RouteInfo
	mu            sync.RWMutex
	routeCache    *RouteCache
	pathPool      sync.Pool
	nodePool      sync.Pool
	paramsPool    sync.Pool
	keyBuilder    sync.Pool
	pendingRoutes []types.RouteBuilder
	segments      []string
	segmentRefs   []stringRef
}

type stringRef struct {
	ptr unsafe.Pointer
	len int
}

type RouteNode struct {
	staticChildren map[string]*RouteNode
	paramChild     *RouteNode
	paramName      string
	methodMask     uint8
	handlers       [8]types.FastHTTPHandler
	configs        [8]*types.RouteConfig
	flags          uint8
}

type RouteCache struct {
	entries sync.Map
	evictMu sync.Mutex
	maxSize int
	size    int32
}

type CacheEntry struct {
	key     string
	handler types.FastHTTPHandler
	config  *types.RouteConfig
	params  map[string]string
	hits    int64
}

const (
	flagIsLeaf    uint8 = 1 << 0
	flagHasParam  uint8 = 1 << 1
	flagHasStatic uint8 = 1 << 2
	MaxCacheSize        = 2048
)

func NewFastHTTPRouter() (*FastHTTPRouter, error) {
	router := &FastHTTPRouter{
		root:         &RouteNode{staticChildren: make(map[string]*RouteNode)},
		staticRoutes: make(map[string]*types.RouteInfo),
		routeCache: &RouteCache{
			maxSize: MaxCacheSize,
		},
		pathPool: sync.Pool{
			New: func() interface{} {
				return make([]string, 0, 8)
			},
		},
		nodePool: sync.Pool{
			New: func() interface{} {
				return &RouteNode{
					staticChildren: make(map[string]*RouteNode),
				}
			},
		},
		paramsPool: sync.Pool{
			New: func() interface{} {
				return make(map[string]string, 4)
			},
		},
		keyBuilder: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 64)
			},
		},
	}

	return router, nil
}

func (r *FastHTTPRouter) Add(method, path string, handler types.FastHTTPHandler, config *types.RouteConfig) {
	methodIdx, exists := methodIndex[method]
	if !exists {
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

	r.addToTrie(methodIdx, path, handler, config)
}

func (r *FastHTTPRouter) addToTrie(methodIdx uint8, path string, handler types.FastHTTPHandler, config *types.RouteConfig) {
	node := r.root
	segments := r.parsePath(path)
	defer r.pathPool.Put(segments[:0])

	for i, segment := range segments {
		isLast := i == len(segments)-1

		if len(segment) > 0 && (segment[0] == '{' || segment[0] == ':') {
			if node.paramChild == nil {
				node.paramChild = r.nodePool.Get().(*RouteNode)
				node.paramChild.staticChildren = make(map[string]*RouteNode)
				node.flags |= flagHasParam

				if segment[0] == '{' && len(segment) > 2 && segment[len(segment)-1] == '}' {
					node.paramChild.paramName = segment[1 : len(segment)-1]
				} else if segment[0] == ':' && len(segment) > 1 {
					node.paramChild.paramName = segment[1:]
				}
			}
			node = node.paramChild
		} else {
			child, exists := node.staticChildren[segment]
			if !exists {
				child = r.nodePool.Get().(*RouteNode)
				child.staticChildren = make(map[string]*RouteNode)
				node.staticChildren[segment] = child
				node.flags |= flagHasStatic
			}
			node = child
		}

		if isLast {
			node.flags |= flagIsLeaf
		}
	}

	node.handlers[methodIdx] = handler
	node.configs[methodIdx] = config
	node.methodMask |= 1 << methodIdx
}

func (r *FastHTTPRouter) Handler(ctx *fasthttp.RequestCtx, server types.HTTPServer) {
	methodBytes := ctx.Method()
	pathBytes := ctx.Path()

	var methodIdx uint8
	var exists bool

	switch {
	case bytes.Equal(methodBytes, getBytes):
		methodIdx, exists = 0, true
	case bytes.Equal(methodBytes, postBytes):
		methodIdx, exists = 1, true
	case bytes.Equal(methodBytes, putBytes):
		methodIdx, exists = 2, true
	case bytes.Equal(methodBytes, deleteBytes):
		methodIdx, exists = 3, true
	case bytes.Equal(methodBytes, patchBytes):
		methodIdx, exists = 4, true
	default:
		methodIdx, exists = methodIndex[b2s(methodBytes)]
	}

	if !exists {
		ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
		return
	}

	if handler, config := r.findStaticFast(methodBytes, pathBytes); handler != nil {
		server.HandleRequest(ctx, handler, config)
		return
	}

	cacheKey := r.buildCacheKey(methodBytes, pathBytes)
	if cached := r.routeCache.get(cacheKey); cached != nil {
		r.setParams(ctx, cached.params)
		server.HandleRequest(ctx, cached.handler, cached.config)
		return
	}

	r.mu.RLock()
	handler, params, routeConfig := r.findInTrie(b2s(pathBytes), methodIdx)
	r.mu.RUnlock()

	if handler == nil {
		if params != nil {
			r.releaseParams(params)
		}
		ctx.Error("Not found", fasthttp.StatusNotFound)
		return
	}

	shouldCache := bytes.Equal(methodBytes, getBytes) && len(params) <= 4

	if shouldCache {
		r.routeCache.set(cacheKey, &CacheEntry{
			key:     cacheKey,
			handler: handler,
			config:  routeConfig,
			params:  params,
		})
	} else {
		defer r.releaseParams(params)
	}

	r.setParams(ctx, params)
	server.HandleRequest(ctx, handler, routeConfig)
}

func (r *FastHTTPRouter) buildCacheKey(method, path []byte) string {
	buf := r.keyBuilder.Get().([]byte)
	buf = buf[:0]

	buf = append(buf, method...)
	buf = append(buf, ':')
	buf = append(buf, path...)

	result := utils.Intern(buf)

	r.keyBuilder.Put(buf)
	return result
}

func (r *FastHTTPRouter) findStaticFast(method, path []byte) (types.FastHTTPHandler, *types.RouteConfig) {
	if bytes.ContainsAny(path, "{}:") {
		return nil, nil
	}

	if len(method)+len(path) <= 30 {
		var buf [32]byte
		n := copy(buf[:], method)
		buf[n] = ':'
		copy(buf[n+1:], path)
		key := string(buf[:n+1+len(path)])

		r.mu.RLock()
		info := r.staticRoutes[key]
		r.mu.RUnlock()

		if info != nil {
			return info.Handler, info.Config
		}
		return nil, nil
	}

	key := r.buildCacheKey(method, path)
	r.mu.RLock()
	info := r.staticRoutes[key]
	r.mu.RUnlock()

	if info != nil {
		return info.Handler, info.Config
	}
	return nil, nil
}

func (r *FastHTTPRouter) releaseParams(params map[string]string) {
	if params == nil {
		return
	}

	for k := range params {
		delete(params, k)
	}
	r.paramsPool.Put(params)
}

func (r *FastHTTPRouter) Route(method string, path string, handler types.FastHTTPHandler, gb types.GroupBuilder) types.RouteBuilder {
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

func (r *FastHTTPRouter) addPendingRoute(rb types.RouteBuilder) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pendingRoutes = append(r.pendingRoutes, rb)
}

func (r *FastHTTPRouter) Group(prefix string) types.GroupBuilder {
	return &GroupBuilder{
		router: r,
		prefix: prefix,
		config: &types.RouteConfig{},
	}
}

func (r *FastHTTPRouter) FinalizePendingRoutes() error {
	r.mu.Lock()
	routeCount := len(r.pendingRoutes)
	r.mu.Unlock()

	if routeCount == 0 {
		return nil
	}

	r.mu.Lock()
	routes := make([]types.RouteBuilder, len(r.pendingRoutes))
	copy(routes, r.pendingRoutes)
	r.pendingRoutes = r.pendingRoutes[:0]
	r.mu.Unlock()

	successCount := 0
	var finalizeErrors []error

	for _, route := range routes {
		if err := route.Finalize(); err != nil {
			finalizeErrors = append(finalizeErrors, err)
		} else {
			successCount++
		}
	}

	if len(finalizeErrors) > 0 {
		return types.Errorf(types.ErrRouteFinalizationFailed, "%d errors occurred", len(finalizeErrors))
	}

	return nil
}

func (r *FastHTTPRouter) GetAllRoutes() map[string]*types.RouteInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := make(map[string]*types.RouteInfo)
	for key, info := range r.staticRoutes {
		routes[key] = info
	}

	r.collectTrieRoutes(r.root, "", routes)

	return routes
}

func (r *FastHTTPRouter) GET(path string, handler types.FastHTTPHandler) types.RouteBuilder {
	return r.Route("GET", path, handler, nil)
}

func (r *FastHTTPRouter) POST(path string, handler types.FastHTTPHandler) types.RouteBuilder {
	return r.Route("POST", path, handler, nil)
}

func (r *FastHTTPRouter) PUT(path string, handler types.FastHTTPHandler) types.RouteBuilder {
	return r.Route("PUT", path, handler, nil)
}

func (r *FastHTTPRouter) DELETE(path string, handler types.FastHTTPHandler) types.RouteBuilder {
	return r.Route("DELETE", path, handler, nil)
}

func (r *FastHTTPRouter) collectTrieRoutes(node *RouteNode, currentPath string, routes map[string]*types.RouteInfo) {
	if (node.flags & flagIsLeaf) != 0 {
		for methodIdx, methodName := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "TRACE"} {
			if (node.methodMask & (1 << methodIdx)) != 0 {
				key := methodName + ":" + currentPath
				routes[key] = &types.RouteInfo{
					Handler: node.handlers[methodIdx],
					Config:  node.configs[methodIdx],
				}
			}
		}
	}

	if (node.flags & flagHasStatic) != 0 {
		for segment, child := range node.staticChildren {
			newPath := currentPath
			if currentPath == "" || currentPath == "/" {
				newPath = "/" + segment
			} else {
				newPath = currentPath + "/" + segment
			}
			r.collectTrieRoutes(child, newPath, routes)
		}
	}

	if (node.flags&flagHasParam) != 0 && node.paramChild != nil {
		paramSegment := "{" + node.paramChild.paramName + "}"

		newPath := currentPath
		if currentPath == "" || currentPath == "/" {
			newPath = "/" + paramSegment
		} else {
			newPath = currentPath + "/" + paramSegment
		}
		r.collectTrieRoutes(node.paramChild, newPath, routes)
	}
}

func (r *FastHTTPRouter) findStatic(method, path string) (types.FastHTTPHandler, *types.RouteConfig) {
	key := method + ":" + path
	r.mu.RLock()
	info := r.staticRoutes[key]
	r.mu.RUnlock()

	if info != nil {
		return info.Handler, info.Config
	}
	return nil, nil
}

func (r *FastHTTPRouter) findInTrie(path string, methodIdx uint8) (types.FastHTTPHandler, map[string]string, *types.RouteConfig) {
	segments := r.parsePath(path)
	defer r.pathPool.Put(segments[:0])

	params := r.paramsPool.Get().(map[string]string)

	for k := range params {
		delete(params, k)
	}

	handler, config := r.findInNode(r.root, segments, 0, methodIdx, params)

	if len(params) == 0 {
		r.paramsPool.Put(params)
		return handler, nil, config
	}

	return handler, params, config
}

func (r *FastHTTPRouter) findInNode(node *RouteNode, segments []string, index int, methodIdx uint8, params map[string]string) (types.FastHTTPHandler, *types.RouteConfig) {
	if index >= len(segments) {
		if (node.flags&flagIsLeaf) != 0 && (node.methodMask&(1<<methodIdx)) != 0 {
			return node.handlers[methodIdx], node.configs[methodIdx]
		}
		return nil, nil
	}

	segment := segments[index]

	if (node.flags & flagHasStatic) != 0 {
		if child, exists := node.staticChildren[segment]; exists {
			if handler, config := r.findInNode(child, segments, index+1, methodIdx, params); handler != nil {
				return handler, config
			}
		}
	}

	if (node.flags&flagHasParam) != 0 && node.paramChild != nil {
		oldLen := len(params)
		params[node.paramChild.paramName] = segment

		if handler, config := r.findInNode(node.paramChild, segments, index+1, methodIdx, params); handler != nil {
			return handler, config
		}

		delete(params, node.paramChild.paramName)
		for k := range params {
			if len(params) <= oldLen {
				break
			}
			delete(params, k)
		}
	}

	return nil, nil
}

func (r *FastHTTPRouter) parsePath(path string) []string {
	r.segments = r.segments[:0]

	if path == "/" {
		if cap(r.segments) == 0 {
			r.segments = make([]string, 0, 8)
		}
		return r.segments
	}

	segmentCount := 1
	for i := 1; i < len(path); i++ {
		if path[i] == '/' {
			segmentCount++
		}
	}

	if cap(r.segments) < segmentCount {
		r.segments = make([]string, 0, segmentCount)
	}

	start := 1
	for i := 1; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			if i > start {
				segment := path[start:i]
				r.segments = append(r.segments, segment)
			}
			start = i + 1
		}
	}

	return r.segments
}

func (r *FastHTTPRouter) setParams(ctx *fasthttp.RequestCtx, params map[string]string) {
	if len(params) == 0 {
		ctx.SetUserValue("route_params", nil)
		return
	}

	ctx.SetUserValue("route_params", params)
}

func (rc *RouteCache) get(key string) *CacheEntry {
	if value, ok := rc.entries.Load(key); ok {
		entry := value.(*CacheEntry)
		atomic.AddInt64(&entry.hits, 1)
		return entry
	}
	return nil
}

func (rc *RouteCache) set(key string, entry *CacheEntry) {
	currentSize := atomic.LoadInt32(&rc.size)
	if currentSize >= int32(rc.maxSize) {
		rc.evictLeastUsed()
	}

	if _, loaded := rc.entries.LoadOrStore(key, entry); !loaded {
		atomic.AddInt32(&rc.size, 1)
	}
}

func (rc *RouteCache) evictLeastUsed() {
	rc.evictMu.Lock()
	defer rc.evictMu.Unlock()

	if atomic.LoadInt32(&rc.size) < int32(rc.maxSize) {
		return
	}

	var leastUsedKey string
	var minHits int64 = -1

	rc.entries.Range(func(key, value interface{}) bool {
		entry := value.(*CacheEntry)
		hits := atomic.LoadInt64(&entry.hits)

		if minHits == -1 || hits < minHits {
			minHits = hits
			leastUsedKey = key.(string)
		}
		return true
	})

	if leastUsedKey != "" {
		rc.entries.Delete(leastUsedKey)
		atomic.AddInt32(&rc.size, -1)
	}
}

func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
