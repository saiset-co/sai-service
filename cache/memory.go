package cache

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type MemoryState int32

const (
	MemoryStateStopped MemoryState = iota
	MemoryStateStarting
	MemoryStateRunning
	MemoryStateStopping
)

const (
	MaxTTL     = 24 * time.Hour
	DefaultTTL = 1 * time.Hour
)

type MemoryConfig struct {
	MaxEntries      int    `json:"max_entries"`
	CleanupInterval string `json:"cleanup_interval"`
	MaxMemory       uint64 `json:"max_memory"`
	EvictionPolicy  string `json:"eviction_policy"`
}

type MemoryCache struct {
	ctx             context.Context
	cancel          context.CancelFunc
	config          *MemoryConfig
	logger          types.Logger
	health          types.HealthManager
	data            map[string]*types.CacheEntry
	revisions       map[string]uint64
	dependencies    map[string][]string
	hits            uint64
	misses          uint64
	evictions       uint64
	mu              sync.RWMutex
	revMu           sync.RWMutex
	depMu           sync.RWMutex
	state           atomic.Value
	stopCleanup     chan struct{}
	cleanupDone     chan struct{}
	entryPool       sync.Pool
	stringSlicePool sync.Pool
	keyBuilderPool  sync.Pool
	shutdownTimeout time.Duration
}

type KeyBuilder struct {
	buf []byte
}

func NewMemoryCache(ctx context.Context, logger types.Logger, config *types.CacheConfig, health types.HealthManager) (types.CacheManager, error) {
	var memConfig = &MemoryConfig{
		MaxEntries:      10000,
		CleanupInterval: "5m",
		MaxMemory:       0,
		EvictionPolicy:  "fifo",
	}

	if config.Config != nil {
		err := utils.UnmarshalConfig(config.Config, memConfig)
		if err != nil {
			return nil, types.WrapError(err, "failed to marshal memory cache config")
		}
	}

	cacheCtx, cancel := context.WithCancel(ctx)

	cache := &MemoryCache{
		ctx:             cacheCtx,
		cancel:          cancel,
		logger:          logger,
		health:          health,
		config:          memConfig,
		data:            make(map[string]*types.CacheEntry),
		revisions:       make(map[string]uint64),
		dependencies:    make(map[string][]string),
		stopCleanup:     make(chan struct{}),
		cleanupDone:     make(chan struct{}),
		shutdownTimeout: 10 * time.Second,
		entryPool: sync.Pool{
			New: func() interface{} {
				return &types.CacheEntry{
					Metadata: make(map[string]string, 4),
				}
			},
		},
		stringSlicePool: sync.Pool{
			New: func() interface{} {
				return make([]string, 0, 8)
			},
		},
		keyBuilderPool: sync.Pool{
			New: func() interface{} { return &KeyBuilder{buf: make([]byte, 0, 512)} },
		},
	}

	cache.state.Store(MemoryStateStopped)

	return cache, nil
}

func (m *MemoryCache) Get(key string) (interface{}, bool) {
	now := time.Now().UnixNano()

	m.mu.RLock()
	entry, exists := m.data[key]
	if !exists {
		m.mu.RUnlock()
		atomic.AddUint64(&m.misses, 1)
		return nil, false
	}

	if !entry.ExpiresAt.IsZero() && now > entry.ExpiresAt.UnixNano() {
		m.mu.RUnlock()
		m.mu.Lock()
		if entry, exists := m.data[key]; exists && now > entry.ExpiresAt.UnixNano() {
			m.removeEntryUnsafe(key)
			m.returnEntryToPool(entry)
		}
		m.mu.Unlock()

		atomic.AddUint64(&m.misses, 1)
		return nil, false
	}

	value := entry.Value
	m.mu.RUnlock()

	atomic.AddUint64(&m.hits, 1)

	return value, true
}

func (m *MemoryCache) Set(key string, value interface{}, ttl time.Duration) error {
	if key == "" {
		m.logger.Error("Attempted to set cache entry with empty key")
		return types.ErrCacheKeyEmpty
	}

	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if ttl > MaxTTL {
		ttl = MaxTTL
	}

	now := time.Now()
	entry := m.entryPool.Get().(*types.CacheEntry)
	entry.Key = key
	entry.Value = value
	entry.TTL = ttl
	entry.CreatedAt = now
	entry.ExpiresAt = now.Add(ttl)

	for k := range entry.Metadata {
		delete(entry.Metadata, k)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.config.MaxEntries > 0 {
		if _, exists := m.data[key]; !exists && len(m.data) >= m.config.MaxEntries {
			if err := m.evictOneUnsafe(); err != nil {
				m.returnEntryToPool(entry)
				m.logger.Error("Failed to evict cache entry", zap.Error(err))
				return err
			}
		}
	}

	if oldEntry, exists := m.data[key]; exists {
		m.removeDependenciesUnsafe(key, oldEntry.Dependencies)
		m.returnEntryToPool(oldEntry)
	}

	m.data[key] = entry
	return nil
}

func (m *MemoryCache) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, exists := m.data[key]; exists {
		m.removeDependenciesUnsafe(key, entry.Dependencies)
		m.returnEntryToPool(entry)
	}

	m.removeEntryUnsafe(key)
	return nil
}

func (m *MemoryCache) Invalidate(keys ...string) error {
	for _, key := range keys {
		oldRevision := m.GetRevision(key)
		newRevision := oldRevision + 1
		m.SetRevision(key, newRevision)

		if err := m.invalidateDependencies(key); err != nil {
			return err
		}
	}

	return nil
}

func (m *MemoryCache) GetRevision(key string) uint64 {
	m.revMu.RLock()
	defer m.revMu.RUnlock()
	return m.revisions[key]
}

func (m *MemoryCache) SetRevision(key string, revision uint64) {
	m.revMu.Lock()
	defer m.revMu.Unlock()
	m.revisions[key] = revision
}

func (m *MemoryCache) BuildCacheKey(requestPath []byte, dependencies []string, metadata map[string][]byte) string {
	builder := m.keyBuilderPool.Get().(*KeyBuilder)
	defer m.keyBuilderPool.Put(builder)

	estimatedSize := len(requestPath) + len(dependencies)*20 + len(metadata)*30
	if cap(builder.buf) < estimatedSize {
		builder.buf = make([]byte, 0, estimatedSize)
	}

	builder.buf = builder.buf[:0]
	builder.buf = append(builder.buf, requestPath...)

	for _, dep := range dependencies {
		revision := m.GetRevision(dep)
		builder.buf = append(builder.buf, '|')
		builder.buf = append(builder.buf, dep...)
		builder.buf = append(builder.buf, '|')
		builder.buf = strconv.AppendUint(builder.buf, revision, 10)
	}

	for key, value := range metadata {
		builder.buf = append(builder.buf, '|')
		builder.buf = append(builder.buf, key...)
		builder.buf = append(builder.buf, ':')
		builder.buf = append(builder.buf, value...)
	}

	cacheKey := utils.BytesToString(builder.buf)
	m.registerDependencies(cacheKey, dependencies)

	return cacheKey
}

func (m *MemoryCache) Start() error {
	if !m.transitionState(MemoryStateStopped, MemoryStateStarting) {
		m.logger.Warn("Cache manager is already running")
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if m.getState() == MemoryStateStarting {
			m.setState(MemoryStateRunning)
		}
	}()

	if m.config.CleanupInterval != "" {
		go m.startCleanupRoutine()
	}

	m.logger.Info("Memory cache started")
	return nil
}

func (m *MemoryCache) Stop() error {
	if !m.transitionState(MemoryStateRunning, MemoryStateStopping) {
		m.logger.Warn("Memory cache is not running")
		return types.ErrServerNotRunning
	}

	defer func() {
		m.setState(MemoryStateStopped)
	}()

	m.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), m.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case m.stopCleanup <- struct{}{}:
		case <-time.After(time.Second):
		}

		select {
		case <-m.cleanupDone:
			m.logger.Debug("Cleanup routine stopped")
		case <-time.After(5 * time.Second):
			m.logger.Warn("Cleanup routine stop timeout")
		}

		return nil
	})

	g.Go(func() error {
		m.mu.Lock()
		m.revMu.Lock()
		m.depMu.Lock()

		entriesCount := len(m.data)
		dependenciesCount := len(m.dependencies)

		for _, entry := range m.data {
			m.returnEntryToPool(entry)
		}

		m.data = make(map[string]*types.CacheEntry)
		m.revisions = make(map[string]uint64)
		m.dependencies = make(map[string][]string)

		m.depMu.Unlock()
		m.revMu.Unlock()
		m.mu.Unlock()

		m.logger.Info("Memory cache cleared",
			zap.Int("cleared_entries", entriesCount),
			zap.Int("cleared_dependencies", dependenciesCount))
		return nil
	})

	if err := g.Wait(); err != nil {
		select {
		case <-gCtx.Done():
			m.logger.Warn("Memory cache stop timeout, some components may not have stopped gracefully")
		default:
			m.logger.Error("Error during memory cache shutdown", zap.Error(err))
		}
	} else {
		m.logger.Info("Memory cache stopped gracefully")
	}

	return nil
}

func (m *MemoryCache) IsRunning() bool {
	return m.getState() == MemoryStateRunning
}

func (m *MemoryCache) getState() MemoryState {
	return m.state.Load().(MemoryState)
}

func (m *MemoryCache) setState(newState MemoryState) bool {
	currentState := m.getState()
	return m.state.CompareAndSwap(currentState, newState)
}

func (m *MemoryCache) transitionState(from, to MemoryState) bool {
	return m.state.CompareAndSwap(from, to)
}

func (m *MemoryCache) returnEntryToPool(entry *types.CacheEntry) {
	if entry == nil {
		return
	}

	entry.Key = ""
	entry.Value = nil
	entry.TTL = 0
	entry.CreatedAt = time.Time{}
	entry.ExpiresAt = time.Time{}
	entry.Dependencies = nil

	for k := range entry.Metadata {
		delete(entry.Metadata, k)
	}

	m.entryPool.Put(entry)
}

func (m *MemoryCache) cleanup() error {
	now := time.Now().UnixNano()

	m.mu.Lock()

	expired := m.stringSlicePool.Get().([]string)
	expired = expired[:0]

	for key, entry := range m.data {
		if !entry.ExpiresAt.IsZero() && now > entry.ExpiresAt.UnixNano() {
			expired = append(expired, key)
		}
	}

	expiredCount := len(expired)
	for _, key := range expired {
		if entry := m.data[key]; entry != nil {
			m.removeDependenciesUnsafe(key, entry.Dependencies)
			m.returnEntryToPool(entry)
		}
		m.removeEntryUnsafe(key)
	}

	m.stringSlicePool.Put(&expired)
	m.mu.Unlock()

	if expiredCount > 0 {
		m.logger.Debug("Cleanup completed", zap.Int("expired_entries", expiredCount))
	}

	return nil
}

func (m *MemoryCache) startCleanupRoutine() {
	defer close(m.cleanupDone)

	cleanupInterval, err := time.ParseDuration(m.config.CleanupInterval)
	if err != nil {
		m.logger.Error("Invalid cleanup interval, using default 5m",
			zap.String("interval", m.config.CleanupInterval),
			zap.Error(err))
		cleanupInterval = 5 * time.Minute
	}

	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			m.logger.Debug("Cleanup routine stopped by context")
			return
		case <-m.stopCleanup:
			m.logger.Debug("Cleanup routine stopped by signal")
			return
		case <-ticker.C:
			if err := m.cleanup(); err != nil {
				m.logger.Error("Cleanup failed", zap.Error(err))
			}
		}
	}
}

func (m *MemoryCache) evictOneUnsafe() error {
	if len(m.data) == 0 {
		return nil
	}

	victimKey := m.findFIFOVictim()

	if victimKey != "" {
		if entry := m.data[victimKey]; entry != nil {
			m.removeDependenciesUnsafe(victimKey, entry.Dependencies)
			m.returnEntryToPool(entry)
		}
		m.removeEntryUnsafe(victimKey)
		atomic.AddUint64(&m.evictions, 1)
	}

	return nil
}

func (m *MemoryCache) findFIFOVictim() string {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range m.data {
		if oldestKey == "" || entry.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreatedAt
		}
	}

	return oldestKey
}

func (m *MemoryCache) invalidateDependencies(dependencyKey string) error {
	m.depMu.RLock()
	dependentKeys := make([]string, len(m.dependencies[dependencyKey]))
	copy(dependentKeys, m.dependencies[dependencyKey])
	m.depMu.RUnlock()

	if len(dependentKeys) == 0 {
		return nil
	}

	m.mu.Lock()
	for _, cacheKey := range dependentKeys {
		if entry := m.data[cacheKey]; entry != nil {
			m.returnEntryToPool(entry)
		}
		m.removeEntryUnsafe(cacheKey)
	}
	m.mu.Unlock()

	m.depMu.Lock()
	delete(m.dependencies, dependencyKey)
	m.depMu.Unlock()

	return nil
}

func (m *MemoryCache) registerDependencies(cacheKey string, dependencies []string) {
	if len(dependencies) == 0 {
		return
	}

	m.depMu.Lock()
	defer m.depMu.Unlock()

	for _, dep := range dependencies {
		if m.dependencies[dep] == nil {
			m.dependencies[dep] = make([]string, 0)
		}

		found := false
		for _, existing := range m.dependencies[dep] {
			if existing == cacheKey {
				found = true
				break
			}
		}

		if !found {
			m.dependencies[dep] = append(m.dependencies[dep], cacheKey)
		}
	}

	m.mu.Lock()
	if entry, exists := m.data[cacheKey]; exists {
		entry.Dependencies = make([]string, len(dependencies))
		copy(entry.Dependencies, dependencies)
	}
	m.mu.Unlock()
}

func (m *MemoryCache) removeDependenciesUnsafe(cacheKey string, dependencies []string) {
	if len(dependencies) == 0 {
		return
	}

	m.depMu.Lock()
	defer m.depMu.Unlock()

	for _, dep := range dependencies {
		if dependents, exists := m.dependencies[dep]; exists {
			for i, dependent := range dependents {
				if dependent == cacheKey {
					m.dependencies[dep] = append(dependents[:i], dependents[i+1:]...)
					break
				}
			}

			if len(m.dependencies[dep]) == 0 {
				delete(m.dependencies, dep)
			}
		}
	}
}

func (m *MemoryCache) removeEntryUnsafe(key string) {
	delete(m.data, key)
}
