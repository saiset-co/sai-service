package cache

import (
	"context"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
)

type State int32

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
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
	impl            types.CacheManager
	logger          types.Logger
	metrics         types.MetricsManager
	state           atomic.Value
	shutdownTimeout time.Duration
}

func newInstrumentedCacheManager(logger types.Logger, metrics types.MetricsManager, impl types.CacheManager) types.CacheManager {
	instrumented := &instrumentedCacheManager{
		impl:            impl,
		logger:          logger,
		metrics:         metrics,
		shutdownTimeout: 10 * time.Second,
	}

	instrumented.state.Store(StateStopped)
	return instrumented
}

func (icm *instrumentedCacheManager) Start() error {
	if !icm.transitionState(StateStopped, StateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if icm.getState() == StateStarting {
			icm.setState(StateRunning)
		}
	}()

	err := icm.impl.Start()
	if err != nil {
		icm.setState(StateStopped)
		return err
	}

	icm.logger.Info("Cache manager started")
	return nil
}

func (icm *instrumentedCacheManager) Stop() error {
	if !icm.transitionState(StateRunning, StateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		icm.setState(StateStopped)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), icm.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := icm.impl.Stop(); err != nil {
			icm.logger.Error("Failed to stop cache implementation", zap.Error(err))
			return err
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		select {
		case <-gCtx.Done():
			icm.logger.Warn("Cache manager stop timeout, some components may not have stopped gracefully")
		default:
			icm.logger.Error("Error during cache manager shutdown", zap.Error(err))
		}
	} else {
		icm.logger.Info("Cache manager stopped gracefully")
	}

	return nil
}

func (icm *instrumentedCacheManager) IsRunning() bool {
	return icm.getState() == StateRunning
}

func (icm *instrumentedCacheManager) Get(key string) (interface{}, bool) {
	value, exists := icm.impl.Get(key)
	return value, exists
}

func (icm *instrumentedCacheManager) Set(key string, value interface{}, ttl time.Duration) error {
	err := icm.impl.Set(key, value, ttl)
	return err
}

func (icm *instrumentedCacheManager) Delete(key string) error {
	err := icm.impl.Delete(key)
	return err
}

func (icm *instrumentedCacheManager) Invalidate(keys ...string) error {
	err := icm.impl.Invalidate(keys...)
	return err
}

func (icm *instrumentedCacheManager) GetRevision(key string) uint64 {
	return icm.impl.GetRevision(key)
}

func (icm *instrumentedCacheManager) SetRevision(key string, revision uint64) {
	icm.impl.SetRevision(key, revision)
}

func (icm *instrumentedCacheManager) BuildCacheKey(requestPath []byte, dependencies []string, metadata map[string][]byte) string {
	return icm.impl.BuildCacheKey(requestPath, dependencies, metadata)
}

func (icm *instrumentedCacheManager) getState() State {
	return icm.state.Load().(State)
}

func (icm *instrumentedCacheManager) setState(newState State) bool {
	currentState := icm.getState()
	return icm.state.CompareAndSwap(currentState, newState)
}

func (icm *instrumentedCacheManager) transitionState(from, to State) bool {
	return icm.state.CompareAndSwap(from, to)
}
