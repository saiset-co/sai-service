package cache

import (
	"context"
	"time"

	"github.com/saiset-co/sai-service/types"
)

var customCacheCreators = make(map[string]types.CacheManagerCreator)

func RegisterCacheManager(cacheManagerName string, creator types.CacheManagerCreator) {
	customCacheCreators[cacheManagerName] = creator
}

func NewCacheManager(ctx context.Context, config types.ConfigManager, logger types.Logger, metrics types.MetricsManager, health types.HealthManager) (types.CacheManager, error) {
	cacheConfig := config.GetConfig().Cache

	if !cacheConfig.Enabled {
		return nil, types.ErrCacheIsDisabled
	}

	cacheManagerName := cacheConfig.Type

	var impl types.CacheManager
	var err error

	switch cacheManagerName {
	case "memory":
		impl, err = NewMemoryCache(ctx, logger, cacheConfig, health)
	case "redis":
		impl, err = NewRedisCache(ctx, logger, cacheConfig, health)
	default:
		if creator, exists := customCacheCreators[cacheManagerName]; exists {
			impl, err = creator(cacheConfig)
		} else {
			return nil, types.Errorf(types.ErrCacheTypeUnknown, "type: %s", cacheManagerName)
		}
	}

	if err != nil {
		return nil, err
	}

	return newInstrumentedCacheManager(logger, metrics, impl), nil
}

type instrumentedCacheManager struct {
	impl    types.CacheManager
	logger  types.Logger
	metrics types.MetricsManager
}

func newInstrumentedCacheManager(logger types.Logger, metrics types.MetricsManager, impl types.CacheManager) types.CacheManager {
	instrumented := &instrumentedCacheManager{
		impl:    impl,
		logger:  logger,
		metrics: metrics,
	}

	return instrumented
}

func (icm *instrumentedCacheManager) Get(key string) (interface{}, bool) {
	start := time.Now()
	value, exists := icm.impl.Get(key)
	duration := time.Since(start)

	result := "miss"
	if exists {
		result = "hit"
	}

	icm.recordMetric("get", result, duration)
	return value, exists
}

func (icm *instrumentedCacheManager) Set(key string, value interface{}, ttl time.Duration) error {
	start := time.Now()
	err := icm.impl.Set(key, value, ttl)
	duration := time.Since(start)

	result := "success"
	if err != nil {
		result = "error"
	}

	icm.recordMetric("set", result, duration)
	return err
}

func (icm *instrumentedCacheManager) Delete(key string) error {
	start := time.Now()
	err := icm.impl.Delete(key)
	duration := time.Since(start)

	result := "success"
	if err != nil {
		result = "error"
	}

	icm.recordMetric("delete", result, duration)
	return err
}

func (icm *instrumentedCacheManager) Invalidate(keys ...string) error {
	start := time.Now()
	err := icm.impl.Invalidate(keys...)
	duration := time.Since(start)

	result := "success"
	if err != nil {
		result = "error"
	}

	icm.recordMetric("invalidate", result, duration)
	return err
}

func (icm *instrumentedCacheManager) GetRevision(key string) uint64 {
	return icm.impl.GetRevision(key)
}

func (icm *instrumentedCacheManager) SetRevision(key string, revision uint64) {
	icm.impl.SetRevision(key, revision)
}

func (icm *instrumentedCacheManager) BuildCacheKey(requestPath string, dependencies []string, metadata map[string]string) string {
	return icm.impl.BuildCacheKey(requestPath, dependencies, metadata)
}

func (icm *instrumentedCacheManager) Start() error {
	start := time.Now()
	err := icm.impl.Start()
	duration := time.Since(start)

	result := "success"
	if err != nil {
		result = "error"
	}

	icm.recordMetric("start", result, duration)

	return err
}

func (icm *instrumentedCacheManager) Stop() error {
	return icm.impl.Stop()
}

func (icm *instrumentedCacheManager) IsRunning() bool {
	return icm.impl.IsRunning()
}

func (icm *instrumentedCacheManager) recordMetric(operation, result string, duration time.Duration) {
	opCounter := icm.metrics.Counter("cache_operations_total", map[string]string{
		"operation": operation,
		"result":    result,
	})
	opCounter.Inc()

	opDuration := icm.metrics.Histogram("cache_operation_duration_seconds",
		[]float64{0.0001, 0.001, 0.01, 0.1, 1.0},
		map[string]string{"operation": operation},
	)
	opDuration.Observe(duration.Seconds())
}
