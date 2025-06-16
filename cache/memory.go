package cache

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type MemoryConfig struct {
	MaxEntries      int    `json:"max_entries"`
	CleanupInterval string `json:"cleanup_interval"`
	MaxMemory       uint64 `json:"max_memory"`
	EvictionPolicy  string `json:"eviction_policy"`
}

type MemoryCache struct {
	ctx             context.Context
	config          *MemoryConfig
	logger          types.Logger
	health          types.HealthManager
	data            map[string]*types.CacheEntry
	revisions       map[string]uint64
	dependencies    map[string][]string
	accessTimes     map[string]int64
	accessCounts    map[string]int64
	hits            uint64
	misses          uint64
	evictions       uint64
	mu              sync.RWMutex
	revMu           sync.RWMutex
	depMu           sync.RWMutex
	stopCleanup     chan struct{}
	running         int32
	entryPool       sync.Pool
	stringSlicePool sync.Pool
	keyBuilderPool  sync.Pool
}

type KeyBuilder struct {
	buf    []byte
	strBuf strings.Builder
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

	cache := &MemoryCache{
		ctx:          ctx,
		logger:       logger,
		health:       health,
		config:       memConfig,
		data:         make(map[string]*types.CacheEntry),
		revisions:    make(map[string]uint64),
		dependencies: make(map[string][]string),
		accessTimes:  make(map[string]int64),
		accessCounts: make(map[string]int64),
		stopCleanup:  make(chan struct{}),
		running:      0,
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

	return cache, nil
}

func (m *MemoryCache) Get(key string) (interface{}, bool) {
	now := time.Now().UnixNano()

	m.mu.RLock()
	entry, exists := m.data[key]
	if !exists {
		m.mu.RUnlock()
		atomic.AddUint64(&m.misses, 1)
		m.logger.Debug("Cache miss", zap.String("key", key))
		return nil, false
	}

	if !entry.ExpiresAt.IsZero() && now > entry.ExpiresAt.UnixNano() {
		m.mu.RUnlock()
		m.mu.Lock()
		if entry, exists := m.data[key]; exists && now > entry.ExpiresAt.UnixNano() {
			m.logger.Debug("Cache entry expired", zap.String("key", key), zap.Time("expired_at", entry.ExpiresAt))
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

	m.mu.Lock()
	m.accessTimes[key] = now
	m.accessCounts[key]++
	m.mu.Unlock()

	m.logger.Debug("Cache hit", zap.String("key", key))
	return value, true
}

func (m *MemoryCache) Set(key string, value interface{}, ttl time.Duration) error {
	if key == "" {
		m.logger.Error("Attempted to set cache entry with empty key")
		return types.ErrCacheKeyEmpty
	}

	now := time.Now()
	nowNano := now.UnixNano()

	entry := m.entryPool.Get().(*types.CacheEntry)
	entry.Key = key
	entry.Value = value
	entry.TTL = ttl
	entry.CreatedAt = now

	for k := range entry.Metadata {
		delete(entry.Metadata, k)
	}

	if ttl > 0 {
		entry.ExpiresAt = now.Add(ttl)
	} else {
		entry.ExpiresAt = time.Time{}
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
	m.accessTimes[key] = nowNano
	m.accessCounts[key] = 1

	m.logger.Debug("Cache entry set",
		zap.String("key", key),
		zap.Duration("ttl", ttl),
		zap.Time("expires_at", entry.ExpiresAt))

	return nil
}

func (m *MemoryCache) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, exists := m.data[key]; exists {
		m.removeDependenciesUnsafe(key, entry.Dependencies)
		m.returnEntryToPool(entry)
		m.logger.Debug("Cache entry deleted", zap.String("key", key))
	}

	m.removeEntryUnsafe(key)
	return nil
}

func (m *MemoryCache) Invalidate(keys ...string) error {
	m.logger.Debug("Invalidating cache keys", zap.Strings("keys", keys))

	for _, key := range keys {
		oldRevision := m.GetRevision(key)
		newRevision := oldRevision + 1
		m.SetRevision(key, newRevision)

		m.logger.Debug("Revision updated",
			zap.String("key", key),
			zap.Uint64("old_revision", oldRevision),
			zap.Uint64("new_revision", newRevision))

		if err := m.invalidateDependencies(key); err != nil {
			m.logger.Error("Failed to invalidate dependencies",
				zap.String("key", key),
				zap.Error(err))
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

func (m *MemoryCache) BuildCacheKey(requestPath string, dependencies []string, metadata map[string]string) string {
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
		builder.buf = strconv.AppendUint(builder.buf, revision, 10) // Zero alloc
	}

	for key, value := range metadata {
		builder.buf = append(builder.buf, '|')
		builder.buf = append(builder.buf, key...)
		builder.buf = append(builder.buf, ':')
		builder.buf = append(builder.buf, value...)
	}

	cacheKey := string(builder.buf)

	m.registerDependencies(cacheKey, dependencies)

	m.logger.Debug("Built cache key",
		zap.String("request_path", requestPath),
		zap.Strings("dependencies", dependencies),
		zap.String("cache_key", cacheKey))

	return cacheKey
}

func (m *MemoryCache) Start() error {
	if !atomic.CompareAndSwapInt32(&m.running, 0, 1) {
		m.logger.Warn("Cache manager is already running")
		return types.ErrServerAlreadyRunning
	}

	if m.config.CleanupInterval != "" {
		go m.startCleanupRoutine()
	}

	m.logger.Info("Memory cache started")

	return nil
}

func (m *MemoryCache) Stop() error {
	if !atomic.CompareAndSwapInt32(&m.running, 1, 0) {
		m.logger.Warn("Memory cache is not running")
		return types.ErrServerNotRunning
	}

	close(m.stopCleanup)

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
	m.accessTimes = make(map[string]int64)
	m.accessCounts = make(map[string]int64)

	m.depMu.Unlock()
	m.revMu.Unlock()
	m.mu.Unlock()

	m.logger.Info("Memory cache closed",
		zap.Int("cleared_entries", entriesCount),
		zap.Int("cleared_dependencies", dependenciesCount))

	return nil
}

func (m *MemoryCache) IsRunning() bool {
	return atomic.LoadInt32(&m.running) == 1
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

	for _, key := range expired {
		if entry := m.data[key]; entry != nil {
			m.removeDependenciesUnsafe(key, entry.Dependencies)
			m.returnEntryToPool(entry)
		}
		m.removeEntryUnsafe(key)
	}

	expiredCount := len(expired)
	m.stringSlicePool.Put(expired)

	m.mu.Unlock()

	if expiredCount > 0 {
		m.logger.Info("Cleanup completed",
			zap.Int("expired_entries", expiredCount))
	}

	return nil
}

func (m *MemoryCache) startCleanupRoutine() {
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
			m.logger.Info("Cleanup routine stopped")
			return
		case <-ticker.C:
			if err := m.cleanup(); err != nil {
				m.logger.Error("Cleanup failed", zap.Error(err))
			}
		case <-m.stopCleanup:
			return
		}
	}
}

func (m *MemoryCache) evictOneUnsafe() error {
	if len(m.data) == 0 {
		return nil
	}

	var victimKey string

	switch m.config.EvictionPolicy {
	case "lru":
		victimKey = m.findLRUVictim()
	case "lfu":
		victimKey = m.findLFUVictim()
	case "fifo":
		fallthrough
	default:
		victimKey = m.findFIFOVictim()
	}

	if victimKey != "" {
		if entry := m.data[victimKey]; entry != nil {
			m.removeDependenciesUnsafe(victimKey, entry.Dependencies)
			m.returnEntryToPool(entry)
		}
		m.removeEntryUnsafe(victimKey)
		atomic.AddUint64(&m.evictions, 1)

		m.logger.Debug("Cache entry evicted",
			zap.String("key", victimKey),
			zap.String("policy", m.config.EvictionPolicy))
	}

	return nil
}

func (m *MemoryCache) findLRUVictim() string {
	var oldestKey string
	var oldestTime int64 = -1

	for key, accessTime := range m.accessTimes {
		if oldestTime == -1 || accessTime < oldestTime {
			oldestKey = key
			oldestTime = accessTime
		}
	}

	return oldestKey
}

func (m *MemoryCache) findLFUVictim() string {
	var victimKey string
	var minCount int64 = -1

	for key, count := range m.accessCounts {
		if minCount == -1 || count < minCount {
			victimKey = key
			minCount = count
		}
	}

	return victimKey
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

	m.logger.Debug("Dependencies invalidated",
		zap.String("dependency", dependencyKey),
		zap.Strings("invalidated_keys", dependentKeys))

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
	delete(m.accessTimes, key)
	delete(m.accessCounts, key)
}
