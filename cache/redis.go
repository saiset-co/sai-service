package cache

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type RedisConfig struct {
	Host                string        `json:"host"`
	Port                int           `json:"port"`
	Password            string        `json:"password"`
	DB                  int           `json:"db"`
	PoolSize            int           `json:"pool_size"`
	MinIdleConnections  int           `json:"min_idle_connections"`
	DialTimeout         time.Duration `json:"dial_timeout"`
	ReadTimeout         time.Duration `json:"read_timeout"`
	WriteTimeout        time.Duration `json:"write_timeout"`
	KeyPrefix           string        `json:"key_prefix"`
	MaxDependencies     int           `json:"max_dependencies"`
	MaxDependentsPerKey int           `json:"max_dependents_per_key"`
}

type RedisCache struct {
	ctx          context.Context
	logger       types.Logger
	health       types.HealthManager
	config       *RedisConfig
	client       *redis.Client
	revisions    map[string]uint64
	dependencies map[string][]string
	shutdownCh   chan struct{}
	revMu        sync.RWMutex
	depMu        sync.RWMutex
	started      int32
}

func NewRedisCache(ctx context.Context, logger types.Logger, config *types.CacheConfig, health types.HealthManager) (types.CacheManager, error) {
	var redisConfig = &RedisConfig{
		Host:                "localhost",
		Port:                6379,
		Password:            "",
		DB:                  0,
		PoolSize:            10,
		MinIdleConnections:  2,
		DialTimeout:         5 * time.Second,
		ReadTimeout:         3 * time.Second,
		WriteTimeout:        3 * time.Second,
		KeyPrefix:           "sai-service",
		MaxDependencies:     10000,
		MaxDependentsPerKey: 1000,
	}

	if config.Config != nil {
		err := utils.UnmarshalConfig(config.Config, redisConfig)
		if err != nil {
			return nil, types.WrapError(err, "failed to marshal redis cache config")
		}
	}

	cache := &RedisCache{
		ctx:          ctx,
		logger:       logger,
		health:       health,
		config:       redisConfig,
		revisions:    make(map[string]uint64),
		dependencies: make(map[string][]string),
		shutdownCh:   make(chan struct{}),
		started:      0,
	}

	if err := cache.initRedisClient(); err != nil {
		return nil, types.WrapError(err, "failed to initialize redis client")
	}

	if err := cache.ping(); err != nil {
		return nil, types.WrapError(err, "failed to connect to redis")
	}

	return cache, nil
}

func (r *RedisCache) Get(key string) (interface{}, bool) {
	if key == "" {
		return nil, false
	}

	fullKey := r.buildFullKey(key)

	result, err := r.client.Get(r.ctx, fullKey).Result()
	if err != nil {
		if types.IsError(err, redis.Nil) {
			return nil, false
		}
		r.logger.Error("failed to get cache entry", zap.String("key", key), zap.Error(err))
		return nil, false
	}

	var entry types.CacheEntry
	if err := utils.Unmarshal([]byte(result), &entry); err != nil {
		r.logger.Error("failed to unmarshal cache entry", zap.String("key", key), zap.Error(err))
		r.client.Del(r.ctx, fullKey)
		return nil, false
	}

	if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
		err := r.Delete(key)
		if err != nil {
			r.logger.Error("Failed to delete cache key", zap.Error(err))
		}
		return nil, false
	}

	return entry.Value, true
}

func (r *RedisCache) Set(key string, value interface{}, ttl time.Duration) error {
	if key == "" {
		return types.ErrCacheKeyEmpty
	}

	fullKey := r.buildFullKey(key)

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	entry := &types.CacheEntry{
		Key:       key,
		Value:     value,
		TTL:       ttl,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
		Metadata:  make(map[string]string),
	}

	data, err := utils.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	var setErr error
	if ttl > 0 {
		setErr = r.client.Set(r.ctx, fullKey, data, ttl).Err()
	} else {
		setErr = r.client.Set(r.ctx, fullKey, data, 0).Err()
	}

	if setErr != nil {
		r.logger.Error("failed to set cache entry", zap.String("key", key), zap.Error(setErr))
		return types.WrapError(setErr, "failed to set cache entry")
	}

	return nil
}

func (r *RedisCache) Delete(key string) error {
	if key == "" {
		return nil
	}

	fullKey := r.buildFullKey(key)

	err := r.client.Del(r.ctx, fullKey).Err()
	if err != nil {
		r.logger.Error("failed to delete cache key", zap.String("key", key), zap.Error(err))
		return types.WrapError(err, "failed to delete cache key")
	}

	r.cleanupDependenciesForKey(fullKey)

	return nil
}

func (r *RedisCache) Invalidate(keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	var errs []string
	for _, key := range keys {
		if key == "" {
			continue
		}

		r.SetRevision(key, r.GetRevision(key)+1)

		if err := r.invalidateDependencies(key); err != nil {
			r.logger.Error("failed to invalidate dependencies", zap.String("key", key), zap.Error(err))
			errs = append(errs, fmt.Sprintf("key %s: %v", key, err))
		}
	}

	if len(errs) > 0 {
		return types.NewErrorf("invalidation errors: %s", strings.Join(errs, "; "))
	}

	return nil
}

func (r *RedisCache) GetRevision(key string) uint64 {
	if key == "" {
		return 0
	}

	r.revMu.RLock()
	if revision, exists := r.revisions[key]; exists {
		r.revMu.RUnlock()
		return revision
	}
	r.revMu.RUnlock()

	revisionKey := r.buildRevisionKey(key)
	result, err := r.client.Get(r.ctx, revisionKey).Result()
	if err != nil {
		if !types.IsError(err, redis.Nil) {
			r.logger.Warn("failed to get revision from redis",
				zap.String("key", key), zap.Error(err))
		}
		return 0
	}

	revision, err := strconv.ParseUint(result, 10, 64)
	if err != nil {
		r.logger.Error("failed to parse revision",
			zap.String("key", key), zap.Error(err))
		return 0
	}

	r.revMu.Lock()
	if existingRevision, exists := r.revisions[key]; !exists || revision > existingRevision {
		r.revisions[key] = revision
	} else {
		revision = existingRevision
	}
	r.revMu.Unlock()

	return revision
}

func (r *RedisCache) SetRevision(key string, revision uint64) {
	if key == "" {
		return
	}

	revisionKey := r.buildRevisionKey(key)

	if err := r.client.Set(r.ctx, revisionKey, revision, 0).Err(); err != nil {
		r.logger.Error("failed to set revision in redis", zap.String("key", key), zap.Uint64("revision", revision), zap.Error(err))
	}

	r.revMu.Lock()
	r.revisions[key] = revision
	r.revMu.Unlock()
}

func (r *RedisCache) BuildCacheKey(requestPath string, dependencies []string, metadata map[string]string) string {
	if requestPath == "" {
		return ""
	}

	var keyParts []string
	keyParts = append(keyParts, requestPath)

	for _, dep := range dependencies {
		if dep != "" {
			revision := r.GetRevision(dep)
			keyParts = append(keyParts, fmt.Sprintf("%s|%d", dep, revision))
		}
	}

	for key, value := range metadata {
		if key != "" && value != "" {
			keyParts = append(keyParts, fmt.Sprintf("%s:%s", key, value))
		}
	}

	cacheKey := strings.Join(keyParts, "|")

	r.registerDependencies(cacheKey, dependencies)

	return cacheKey
}

func (r *RedisCache) Start() error {
	if !atomic.CompareAndSwapInt32(&r.started, 0, 1) {
		return nil
	}

	go r.startCleanupWorker()

	r.logger.Info("Redis cache started")

	return nil
}

func (r *RedisCache) Stop() error {
	if !atomic.CompareAndSwapInt32(&r.started, 1, 0) {
		return nil
	}

	select {
	case <-r.shutdownCh:
	default:
		close(r.shutdownCh)
	}

	shutdownTimeout := time.NewTimer(5 * time.Second)
	defer shutdownTimeout.Stop()

	done := make(chan struct{})
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
		r.logger.Info("Cache cleanup worker stopped gracefully")
	case <-shutdownTimeout.C:
		r.logger.Warn("Cache cleanup worker shutdown timeout")
	}

	if r.client != nil {
		if err := r.client.Close(); err != nil {
			r.logger.Error("Failed to close Redis client", zap.Error(err))
			return types.WrapError(err, "failed to close redis client")
		}
	}

	r.logger.Info("Redis cache closed successfully")
	return nil
}

func (r *RedisCache) IsRunning() bool {
	return atomic.LoadInt32(&r.started) == 1
}

func (r *RedisCache) cleanup() error {
	r.clearLocalCache()
	return nil
}

func (r *RedisCache) initRedisClient() error {
	addr := fmt.Sprintf("%s:%d", r.config.Host, r.config.Port)

	r.client = redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     r.config.Password,
		DB:           r.config.DB,
		PoolSize:     r.config.PoolSize,
		MinIdleConns: r.config.MinIdleConnections,
		DialTimeout:  r.config.DialTimeout,
		ReadTimeout:  r.config.ReadTimeout,
		WriteTimeout: r.config.WriteTimeout,
	})

	return nil
}

func (r *RedisCache) ping() error {
	ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
	defer cancel()

	return r.client.Ping(ctx).Err()
}

func (r *RedisCache) buildFullKey(key string) string {
	if r.config.KeyPrefix != "" {
		return fmt.Sprintf("%s:%s", r.config.KeyPrefix, key)
	}
	return key
}

func (r *RedisCache) buildRevisionKey(key string) string {
	return r.buildFullKey(fmt.Sprintf("rev:%s", key))
}

func (r *RedisCache) invalidateDependencies(key string) error {
	r.depMu.RLock()
	dependents := r.dependencies[key]
	if len(dependents) == 0 {
		r.depMu.RUnlock()
		return nil
	}

	dependentsCopy := make([]string, len(dependents))
	copy(dependentsCopy, dependents)
	r.depMu.RUnlock()

	var errs []string
	for _, dependent := range dependentsCopy {
		if err := r.Delete(dependent); err != nil {
			errs = append(errs, fmt.Sprintf("dependent %s: %v", dependent, err))
		}
	}

	if len(errs) > 0 {
		return types.NewErrorf("dependency invalidation errors: %s", strings.Join(errs, "; "))
	}

	return nil
}

func (r *RedisCache) registerDependencies(cacheKey string, dependencies []string) {
	if cacheKey == "" || len(dependencies) == 0 {
		return
	}

	r.depMu.Lock()
	defer r.depMu.Unlock()

	maxDependencies := r.config.MaxDependencies
	maxDependentsPerKey := r.config.MaxDependentsPerKey

	if len(r.dependencies) >= maxDependencies {
		r.depMu.Unlock()
		r.forceDependencyCleanup(maxDependencies * 4 / 5)
		r.depMu.Lock()
	}

	for _, dep := range dependencies {
		if dep == "" {
			continue
		}

		if r.dependencies[dep] == nil {
			r.dependencies[dep] = make([]string, 0)
		}

		dependents := r.dependencies[dep]

		if len(dependents) >= maxDependentsPerKey {
			r.logger.Warn("Force cleaning dependents for dependency",
				zap.String("dependency", dep),
				zap.Int("current_dependents", len(dependents)))

			keepCount := maxDependentsPerKey * 4 / 5
			if len(dependents) > keepCount {
				r.dependencies[dep] = dependents[len(dependents)-keepCount:]
				dependents = r.dependencies[dep]
			}
		}

		if len(dependents) > 10 {
			dependentsMap := make(map[string]bool, len(dependents))
			for _, existing := range dependents {
				dependentsMap[existing] = true
			}
			if !dependentsMap[cacheKey] {
				r.dependencies[dep] = append(dependents, cacheKey)
			}
		} else {
			found := false
			for _, existing := range dependents {
				if existing == cacheKey {
					found = true
					break
				}
			}
			if !found {
				r.dependencies[dep] = append(dependents, cacheKey)
			}
		}
	}
}

func (r *RedisCache) cleanupDependenciesForKey(cacheKey string) {
	if cacheKey == "" {
		return
	}

	r.depMu.Lock()
	defer r.depMu.Unlock()

	for dep, dependents := range r.dependencies {
		newDependents := make([]string, 0, len(dependents))
		for _, dependent := range dependents {
			if dependent != cacheKey {
				newDependents = append(newDependents, dependent)
			}
		}

		if len(newDependents) == 0 {
			delete(r.dependencies, dep)
		} else {
			r.dependencies[dep] = newDependents
		}
	}
}

func (r *RedisCache) clearLocalCache() {
	r.revMu.Lock()
	r.revisions = make(map[string]uint64)
	r.revMu.Unlock()

	r.depMu.Lock()
	r.dependencies = make(map[string][]string)
	r.depMu.Unlock()
}

func (r *RedisCache) cleanupExpiredDependencies() {
	r.logger.Debug("Starting cleanup of expired dependencies")

	start := time.Now()
	cleanedDeps := 0
	cleanedKeys := 0
	totalChecked := 0

	r.depMu.RLock()
	dependenciesSnapshot := make(map[string][]string)
	uniqueDependents := make(map[string]bool)

	for dep, dependents := range r.dependencies {
		dependentsCopy := make([]string, len(dependents))
		copy(dependentsCopy, dependents)
		dependenciesSnapshot[dep] = dependentsCopy

		for _, dependent := range dependents {
			uniqueDependents[dependent] = true
		}
	}
	r.depMu.RUnlock()

	if len(uniqueDependents) == 0 {
		r.logger.Debug("No dependents to check")
		return
	}

	dependentKeys := make([]string, 0, len(uniqueDependents))
	for key := range uniqueDependents {
		dependentKeys = append(dependentKeys, key)
	}

	const batchSize = 100
	expiredKeys := make(map[string]bool)

	for i := 0; i < len(dependentKeys); i += batchSize {
		select {
		case <-r.ctx.Done():
			r.logger.Info("Cleanup cancelled due to context cancellation")
			return
		default:
		}

		end := i + batchSize
		if end > len(dependentKeys) {
			end = len(dependentKeys)
		}

		batch := dependentKeys[i:end]
		totalChecked += len(batch)

		ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
		results, err := r.client.Exists(ctx, batch...).Result()
		cancel()

		if err != nil {
			r.logger.Error("Failed to check key existence during cleanup",
				zap.Error(err),
				zap.Int("batch_size", len(batch)))
			continue
		}

		if results == 0 {
			for _, key := range batch {
				expiredKeys[key] = true
			}
		} else if results < int64(len(batch)) {
			for _, key := range batch {
				select {
				case <-r.ctx.Done():
					return
				default:
				}

				ctx, cancel := context.WithTimeout(r.ctx, 2*time.Second)
				exists, err := r.client.Exists(ctx, key).Result()
				cancel()

				if err != nil {
					r.logger.Warn("Failed to check individual key existence",
						zap.String("key", key),
						zap.Error(err))
					continue
				}

				if exists == 0 {
					expiredKeys[key] = true
				}
			}
		}
	}

	r.depMu.Lock()
	defer r.depMu.Unlock()

	for dependency, dependents := range dependenciesSnapshot {
		if len(dependents) == 0 {
			continue
		}

		activeDependents := make([]string, 0, len(dependents))
		removedCount := 0

		for _, dependent := range dependents {
			if !expiredKeys[dependent] {
				activeDependents = append(activeDependents, dependent)
			} else {
				removedCount++
			}
		}

		if len(activeDependents) == 0 {
			delete(r.dependencies, dependency)
			cleanedDeps++
		} else if removedCount > 0 {
			r.dependencies[dependency] = activeDependents
		}

		cleanedKeys += removedCount
	}

	elapsed := time.Since(start)
	currentSize := len(r.dependencies)

	r.logger.Info("Completed cleanup of expired dependencies",
		zap.Duration("elapsed", elapsed),
		zap.Int("total_checked", totalChecked),
		zap.Int("expired_keys", len(expiredKeys)),
		zap.Int("cleaned_dependencies", cleanedDeps),
		zap.Int("cleaned_dependent_keys", cleanedKeys),
		zap.Int("remaining_dependencies", len(r.dependencies)))

	const maxDependencies = 10000
	const emergencyCleanupThreshold = maxDependencies * 9 / 10

	if currentSize > emergencyCleanupThreshold {
		r.logger.Warn("Dependencies size still high after cleanup, triggering emergency cleanup",
			zap.Int("current_size", currentSize),
			zap.Int("threshold", emergencyCleanupThreshold))

		r.depMu.Unlock()
		r.forceDependencyCleanup(maxDependencies * 2 / 3)
		r.depMu.Lock()
	}

	if elapsed > 30*time.Second {
		r.logger.Warn("Dependency cleanup took too long",
			zap.Duration("elapsed", elapsed),
			zap.Int("total_keys_checked", totalChecked))
	}
}

func (r *RedisCache) cleanupExpiredRevisions() {
	r.logger.Debug("Starting cleanup of expired revisions")

	start := time.Now()

	r.revMu.RLock()
	if len(r.revisions) == 0 {
		r.revMu.RUnlock()
		return
	}

	revisionKeys := make([]string, 0, len(r.revisions))
	keyToOriginal := make(map[string]string, len(r.revisions))

	for originalKey := range r.revisions {
		revKey := r.buildRevisionKey(originalKey)
		revisionKeys = append(revisionKeys, revKey)
		keyToOriginal[revKey] = originalKey
	}
	r.revMu.RUnlock()

	const batchSize = 100
	expiredOriginalKeys := make(map[string]bool)

	for i := 0; i < len(revisionKeys); i += batchSize {
		select {
		case <-r.ctx.Done():
			r.logger.Info("Revision cleanup cancelled due to context cancellation")
			return
		default:
		}

		end := i + batchSize
		if end > len(revisionKeys) {
			end = len(revisionKeys)
		}

		batch := revisionKeys[i:end]

		ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
		results, err := r.client.Exists(ctx, batch...).Result()
		cancel()

		if err != nil {
			r.logger.Error("Failed to check revision key existence",
				zap.Error(err))
			continue
		}

		if results < int64(len(batch)) {
			for _, revKey := range batch {
				select {
				case <-r.ctx.Done():
					return
				default:
				}

				ctx, cancel := context.WithTimeout(r.ctx, 2*time.Second)
				exists, err := r.client.Exists(ctx, revKey).Result()
				cancel()

				if err != nil {
					r.logger.Warn("Failed to check revision key",
						zap.String("revision_key", revKey),
						zap.Error(err))
					continue
				}

				if exists == 0 {
					originalKey := keyToOriginal[revKey]
					expiredOriginalKeys[originalKey] = true
				}
			}
		}
	}

	r.revMu.Lock()
	cleanedRevisions := 0
	for originalKey := range expiredOriginalKeys {
		if _, exists := r.revisions[originalKey]; exists {
			delete(r.revisions, originalKey)
			cleanedRevisions++
		}
	}
	r.revMu.Unlock()

	elapsed := time.Since(start)

	if cleanedRevisions > 0 {
		r.logger.Info("Completed cleanup of expired revisions",
			zap.Duration("elapsed", elapsed),
			zap.Int("cleaned_revisions", cleanedRevisions),
			zap.Int("remaining_revisions", len(r.revisions)))
	}
}

func (r *RedisCache) startCleanupWorker() {
	go func() {
		dependencyTicker := time.NewTicker(1 * time.Hour)
		defer dependencyTicker.Stop()

		revisionTicker := time.NewTicker(30 * time.Minute)
		defer revisionTicker.Stop()

		forceCleanupTicker := time.NewTicker(6 * time.Hour)
		defer forceCleanupTicker.Stop()

		r.logger.Info("Started cache cleanup worker")

		for {
			select {
			case <-dependencyTicker.C:
				func() {
					defer func() {
						if rec := recover(); rec != nil {
							r.logger.Error("Panic in dependency cleanup",
								zap.Any("panic", rec))
						}
					}()
					r.cleanupExpiredDependencies()
				}()

			case <-revisionTicker.C:
				func() {
					defer func() {
						if rec := recover(); rec != nil {
							r.logger.Error("Panic in revision cleanup",
								zap.Any("panic", rec))
						}
					}()
					r.cleanupExpiredRevisions()
				}()

			case <-forceCleanupTicker.C:
				func() {
					defer func() {
						if rec := recover(); rec != nil {
							r.logger.Error("Panic in force cleanup",
								zap.Any("panic", rec))
						}
					}()

					r.depMu.RLock()
					currentSize := len(r.dependencies)
					r.depMu.RUnlock()

					maxDependencies := r.config.MaxDependencies
					cleanupThreshold := maxDependencies * 3 / 4

					if currentSize > cleanupThreshold {
						r.logger.Info("Starting scheduled force cleanup",
							zap.Int("current_size", currentSize),
							zap.Int("threshold", cleanupThreshold))

						r.forceDependencyCleanup(maxDependencies / 2)
					}
				}()

			case <-r.shutdownCh:
				r.logger.Info("Stopping cache cleanup worker due to shutdown")
				return

			case <-r.ctx.Done():
				r.logger.Info("Stopping cache cleanup worker due to context cancellation")
				return
			}
		}
	}()
}

func (r *RedisCache) forceDependencyCleanup(targetSize int) {
	r.depMu.Lock()
	defer r.depMu.Unlock()

	currentSize := len(r.dependencies)
	if currentSize <= targetSize {
		return
	}

	r.logger.Info("Starting force dependency cleanup",
		zap.Int("current_size", currentSize),
		zap.Int("target_size", targetSize))

	cleanupCount := currentSize - targetSize

	keysToDelete := make([]string, 0, cleanupCount)
	count := 0

	for dep := range r.dependencies {
		if count >= cleanupCount {
			break
		}
		keysToDelete = append(keysToDelete, dep)
		count++
	}

	for _, dep := range keysToDelete {
		delete(r.dependencies, dep)
	}

	r.logger.Info("Force dependency cleanup completed",
		zap.Int("removed_dependencies", len(keysToDelete)),
		zap.Int("remaining_size", len(r.dependencies)))
}
