package middleware

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

const (
	CacheSize      = 512
	MaxMiddlewares = 64
)

type Manager struct {
	ctx                context.Context
	config             types.ConfigManager
	logger             types.Logger
	metrics            types.MetricsManager
	cache              types.CacheManager
	health             types.HealthManager
	orderedMiddlewares []types.MiddlewareEntry
	defaultEnabledMask uint64
	nameToIndex        map[string]int
	mu                 sync.RWMutex
	maskCache          [CacheSize]*CacheEntry
	cacheIndex         map[string]int
	cacheLRU           []int
	cacheSize          int32
	cacheMu            sync.RWMutex
	compiledChains     map[uint64]*CompiledChain
	chainsMu           sync.RWMutex
	initialized        int32
	middlewareMap      map[string]*types.MiddlewareEntry
	keyBuilderPool     sync.Pool
}

type CacheEntry struct {
	key  string
	mask uint64
	used int64
}

type CompiledChain struct {
	mask        uint64
	middlewares []types.Middleware
	handler     func(*fasthttp.RequestCtx, func(*fasthttp.RequestCtx), *types.RouteConfig)
}

func NewManager(ctx context.Context, config types.ConfigManager, logger types.Logger, metrics types.MetricsManager, cache types.CacheManager, health types.HealthManager) (*Manager, error) {
	manager := &Manager{
		ctx:            ctx,
		config:         config,
		logger:         logger,
		metrics:        metrics,
		cache:          cache,
		health:         health,
		nameToIndex:    make(map[string]int),
		cacheIndex:     make(map[string]int),
		cacheLRU:       make([]int, 0, CacheSize),
		compiledChains: make(map[uint64]*CompiledChain),
		middlewareMap:  make(map[string]*types.MiddlewareEntry),
		keyBuilderPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 128)
			},
		},
	}

	for i := 0; i < CacheSize; i++ {
		manager.maskCache[i] = &CacheEntry{}
	}

	return manager, nil
}

func (m *Manager) RegisterMiddlewares() error {
	config := m.config.GetConfig()

	if config.Middlewares.Recovery.Enabled {
		recoveryMw := NewRecoveryMiddleware(m.config, m.logger, m.metrics)
		if err := m.Register(recoveryMw); err != nil {
			return err
		}
		m.logger.Info(" Recovery middleware registered")
	}

	if config.Middlewares.Logging.Enabled {
		loggingMw := NewLoggingMiddleware(m.config, m.logger, m.metrics)
		if err := m.Register(loggingMw); err != nil {
			return err
		}
		m.logger.Info(" Logging middleware registered")
	}

	if config.Middlewares.Metadata.Enabled {
		metadataMw := NewMetadataMiddleware(m.config, m.logger, m.metrics)
		if err := m.Register(metadataMw); err != nil {
			return err
		}
		m.logger.Info(" Metadata middleware registered")
	}

	if config.Middlewares.RateLimit.Enabled {
		rateLimitMw := NewRateLimitMiddleware(m.ctx, m.config, m.logger, m.metrics)
		if err := m.Register(rateLimitMw); err != nil {
			return err
		}
		m.logger.Info(" RateLimit middleware registered")
	}

	if config.Middlewares.Compression.Enabled {
		compressionMw := NewCompressionMiddleware(m.config, m.logger, m.metrics)
		if err := m.Register(compressionMw); err != nil {
			return err
		}
		m.logger.Info(" Compression middleware registered")
	}

	if config.Middlewares.BodyLimit.Enabled {
		bodyLimitMw := NewBodyLimitMiddleware(m.config, m.logger, m.metrics)
		if err := m.Register(bodyLimitMw); err != nil {
			return err
		}
		m.logger.Info(" BodyLimit middleware registered")
	}

	if config.Middlewares.Cache.Enabled {
		cacheMw := NewCacheMiddleware(m.config, m.logger, m.metrics, m.cache)
		if err := m.Register(cacheMw); err != nil {
			return err
		}
		m.logger.Info(" Cache middleware registered")
	}

	if config.Middlewares.Auth.Enabled {
		authMw := NewAuthMiddleware(m.config, m.logger, m.metrics)
		if err := m.Register(authMw); err != nil {
			return err
		}
		m.logger.Info(" Auth middleware registered")
	}

	if config.Middlewares.CORS.Enabled {
		corsMw := NewCORSMiddleware(m.config, m.logger, m.metrics)
		if err := m.Register(corsMw); err != nil {
			return err
		}
		m.logger.Info(" CORS middleware registered")
	}

	return m.finalizeConfiguration()
}

func (m *Manager) Register(middleware types.Middleware) error {
	if middleware == nil {
		return types.ErrMiddlewareInvalidType
	}

	if atomic.LoadInt32(&m.initialized) == 1 {
		return types.NewErrorf("cannot register middleware after finalization")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.middlewareMap) >= MaxMiddlewares {
		return types.NewErrorf("maximum middleware count exceeded: %d", MaxMiddlewares)
	}

	name := middleware.Name()

	entry := &types.MiddlewareEntry{
		Name:       name,
		Middleware: middleware,
		Weight:     middleware.Weight(),
	}

	m.middlewareMap[name] = entry
	return nil
}

func (m *Manager) finalizeConfiguration() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if atomic.LoadInt32(&m.initialized) == 1 {
		return types.NewErrorf("configuration already finalized")
	}

	weights := make(map[int]string)
	for name, entry := range m.middlewareMap {
		if existingName, exists := weights[entry.Weight]; exists {
			return types.NewErrorf("duplicate weight %d for middlewares '%s' and '%s'",
				entry.Weight, existingName, name)
		}
		weights[entry.Weight] = name
	}

	m.orderedMiddlewares = make([]types.MiddlewareEntry, 0, len(m.middlewareMap))
	for _, entry := range m.middlewareMap {
		m.orderedMiddlewares = append(m.orderedMiddlewares, types.MiddlewareEntry{
			Name:       entry.Name,
			Middleware: entry.Middleware,
			Weight:     entry.Weight,
		})
	}

	sort.Slice(m.orderedMiddlewares, func(i, j int) bool {
		return m.orderedMiddlewares[i].Weight < m.orderedMiddlewares[j].Weight
	})

	m.nameToIndex = make(map[string]int, len(m.orderedMiddlewares))
	m.defaultEnabledMask = 0
	m.middlewareMap = nil

	atomic.StoreInt32(&m.initialized, 1)

	return nil
}

func (m *Manager) Execute(ctx *fasthttp.RequestCtx, handler func(*fasthttp.RequestCtx), config *types.RouteConfig) {
	if atomic.LoadInt32(&m.initialized) == 0 {
		handler(ctx)
		return
	}

	mask := m.computeRouteMaskFast(config)
	if mask == 0 {
		handler(ctx)
		return
	}

	if compiled := m.getCompiledChain(mask); compiled != nil {
		compiled.handler(ctx, handler, config)
	} else {
		m.executeAndCompile(ctx, handler, mask, config)
	}
}

func (m *Manager) computeRouteMaskFast(config *types.RouteConfig) uint64 {
	if config == nil {
		return m.defaultEnabledMask
	}

	if len(config.Middlewares) == 0 && len(config.DisabledMiddlewares) == 0 {
		return m.defaultEnabledMask
	}

	cacheKey := m.buildCacheKeyFast(config)

	if mask, found := m.getCachedMaskFast(cacheKey); found {
		return mask
	}

	mask := m.calculateMaskFast(config)
	m.setCachedMaskFast(cacheKey, mask)

	return mask
}

func (m *Manager) getCachedMaskFast(key string) (uint64, bool) {
	m.cacheMu.RLock()
	if idx, exists := m.cacheIndex[key]; exists {
		entry := m.maskCache[idx]
		if entry.key == key {
			atomic.StoreInt64(&entry.used, time.Now().UnixNano())
			mask := entry.mask
			m.cacheMu.RUnlock()
			return mask, true
		}
	}
	m.cacheMu.RUnlock()
	return 0, false
}

func (m *Manager) setCachedMaskFast(key string, mask uint64) {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	if idx, exists := m.cacheIndex[key]; exists {
		entry := m.maskCache[idx]
		entry.mask = mask
		atomic.StoreInt64(&entry.used, time.Now().UnixNano())
		return
	}

	idx := m.findCacheSlot()
	entry := m.maskCache[idx]
	if entry.key != "" {
		delete(m.cacheIndex, entry.key)
	}

	entry.key = key
	entry.mask = mask
	atomic.StoreInt64(&entry.used, time.Now().UnixNano())
	m.cacheIndex[key] = idx
}

func (m *Manager) findCacheSlot() int {
	currentSize := atomic.LoadInt32(&m.cacheSize)

	if int(currentSize) < CacheSize {
		newSize := atomic.AddInt32(&m.cacheSize, 1)
		return int(newSize) - 1
	}

	oldestIdx := 0
	oldestTime := atomic.LoadInt64(&m.maskCache[0].used)

	for i := 1; i < CacheSize; i++ {
		_time := atomic.LoadInt64(&m.maskCache[i].used)
		if _time < oldestTime {
			oldestTime = _time
			oldestIdx = i
		}
	}

	return oldestIdx
}

func (m *Manager) buildCacheKeyFast(config *types.RouteConfig) string {
	if config == nil || (len(config.Middlewares) == 0 && len(config.DisabledMiddlewares) == 0) {
		return "default"
	}

	if len(config.Middlewares) == 0 && len(config.DisabledMiddlewares) == 1 {
		switch config.DisabledMiddlewares[0] {
		case "Auth":
			return "d:Auth"
		case "Cache":
			return "d:Cache"
		}
	}

	buf := m.keyBuilderPool.Get().([]byte)
	defer m.keyBuilderPool.Put(buf[:0])

	size := 10 + len(config.Middlewares)*10 + len(config.DisabledMiddlewares)*10
	if cap(buf) < size {
		buf = make([]byte, 0, size)
	}
	buf = buf[:0]

	buf = append(buf, "e:"...)
	for i, name := range config.Middlewares {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, name...)
	}

	buf = append(buf, "|d:"...)
	for _, name := range config.DisabledMiddlewares {
		buf = append(buf, name...)
		buf = append(buf, ',')
	}

	return utils.Intern(buf)
}

func (m *Manager) calculateMaskFast(config *types.RouteConfig) uint64 {
	finalMask := m.defaultEnabledMask

	for _, name := range config.Middlewares {
		if index, exists := m.nameToIndex[name]; exists {
			finalMask |= 1 << uint(index)
		}
	}

	for _, name := range config.DisabledMiddlewares {
		if index, exists := m.nameToIndex[name]; exists {
			finalMask &= ^(1 << uint(index))
		}
	}

	return finalMask
}

func (m *Manager) getCompiledChain(mask uint64) *CompiledChain {
	m.chainsMu.RLock()
	chain := m.compiledChains[mask]
	m.chainsMu.RUnlock()
	return chain
}

func (m *Manager) executeAndCompile(ctx *fasthttp.RequestCtx, handler func(*fasthttp.RequestCtx), mask uint64, config *types.RouteConfig) {
	activeMiddlewares := make([]types.Middleware, 0, len(m.orderedMiddlewares))

	for i, entry := range m.orderedMiddlewares {
		if mask&(1<<uint(i)) != 0 {
			if mw, ok := entry.Middleware.(types.Middleware); ok {
				activeMiddlewares = append(activeMiddlewares, mw)
			}
		}
	}

	compiledHandler := m.compileChain(activeMiddlewares)

	compiled := &CompiledChain{
		mask:        mask,
		middlewares: activeMiddlewares,
		handler:     compiledHandler,
	}

	m.chainsMu.Lock()
	m.compiledChains[mask] = compiled
	m.chainsMu.Unlock()

	compiledHandler(ctx, handler, config)
}

func (m *Manager) compileChain(middlewares []types.Middleware) func(*fasthttp.RequestCtx, func(*fasthttp.RequestCtx), *types.RouteConfig) {
	if len(middlewares) == 0 {
		return func(ctx *fasthttp.RequestCtx, handler func(*fasthttp.RequestCtx), config *types.RouteConfig) {
			handler(ctx)
		}
	}

	return func(ctx *fasthttp.RequestCtx, handler func(*fasthttp.RequestCtx), config *types.RouteConfig) {
		var index int

		var next func(*fasthttp.RequestCtx)
		next = func(ctx *fasthttp.RequestCtx) {
			if index >= len(middlewares) {
				handler(ctx)
				return
			}

			mw := middlewares[index]
			index++
			mw.Handle(ctx, next, config)
		}

		next(ctx)
	}
}

func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.orderedMiddlewares = nil
	m.nameToIndex = make(map[string]int)
	m.defaultEnabledMask = 0

	for i := 0; i < CacheSize; i++ {
		m.maskCache[i] = &CacheEntry{}
	}

	atomic.StoreInt32(&m.initialized, 0)

	m.logger.Info("Middleware manager stopped")
}
