package health

import (
	"context"
	"fmt"
	"github.com/saiset-co/sai-service/utils"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
	"time"

	"github.com/saiset-co/sai-service/types"
)

type Manager struct {
	ctx       context.Context
	config    types.ConfigManager
	logger    types.Logger
	checkers  map[string]types.HealthChecker
	results   map[string]types.HealthCheck
	startTime time.Time
	mu        sync.RWMutex
	running   int32
}

func NewManager(ctx context.Context, config types.ConfigManager, logger types.Logger) (*Manager, error) {
	manager := &Manager{
		ctx:      ctx,
		config:   config,
		logger:   logger,
		checkers: make(map[string]types.HealthChecker),
		results:  make(map[string]types.HealthCheck),
	}

	return manager, nil
}

func (hm *Manager) RegisterRoutes(router types.HTTPRouter) {
	config := &types.RouteConfig{
		Cache: &types.CacheHandlerConfig{
			Enabled: false,
		},
		Timeout:             time.Duration(5) * time.Second,
		DisabledMiddlewares: []string{"Auth", "BodyLimit", "Cache"},
		Doc:                 nil, //TODO: add docs?
	}

	router.Add("GET", "/version", hm.handleVersion, config)
	router.Add("GET", "/health", hm.handleHealth, config)
}

func (hm *Manager) handleVersion(ctx *fasthttp.RequestCtx) {
	version := hm.config.GetConfig().Version
	buildInfo := getBuildInfo()

	versionInfo := types.VersionInfo{
		Version:   version,
		BuildInfo: buildInfo,
	}

	ctx.Response.Header.SetContentType("application/json")
	ctx.Response.Header.SetStatusCode(fasthttp.StatusOK)

	response := fmt.Sprintf(`{"version":"%s","build_info":"%s"}`,
		versionInfo.Version, versionInfo.BuildInfo)

	_, err := ctx.Response.BodyWriter().Write([]byte(response))
	if err != nil {
		hm.logger.Error("Failed to write http writer", zap.Error(err))
	}
}

func (hm *Manager) handleHealth(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)

	report := hm.Check(ctx)

	specData, err := utils.Marshal(report)
	if err != nil {
		hm.logger.Error("Failed to encode OpenAPI spec", zap.Error(err))
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
	}

	_, err = ctx.Write(specData)
	if err != nil {
		hm.logger.Error("Failed to encode OpenAPI spec", zap.Error(err))
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		return
	}
}

func (hm *Manager) RegisterChecker(name string, checker types.HealthChecker) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.checkers[name] = checker
}

func (hm *Manager) Check(ctx context.Context) types.HealthReport {
	hm.mu.RLock()
	checkers := make(map[string]types.HealthChecker, len(hm.checkers))
	for name, checker := range hm.checkers {
		checkers[name] = checker
	}
	hm.mu.RUnlock()

	results := make(map[string]types.HealthCheck, len(checkers))
	var wg sync.WaitGroup
	var resultMu sync.Mutex

	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker types.HealthChecker) {
			defer wg.Done()

			result := hm.executeCheck(ctx, name, checker)

			resultMu.Lock()
			results[name] = result
			resultMu.Unlock()
		}(name, checker)
	}

	wg.Wait()

	hm.mu.Lock()
	hm.results = results
	hm.mu.Unlock()

	return hm.buildReport(results)
}

func (hm *Manager) GetStatus() types.HealthReport {
	hm.mu.RLock()
	results := make(map[string]types.HealthCheck, len(hm.results))
	for name, result := range hm.results {
		results[name] = result
	}
	hm.mu.RUnlock()

	return hm.buildReport(results)
}

func (hm *Manager) Start() error {
	if !atomic.CompareAndSwapInt32(&hm.running, 0, 1) {
		hm.logger.Warn("Health manager is already running")
		return types.ErrServerAlreadyRunning
	}

	hm.startTime = time.Now()

	return nil
}

func (hm *Manager) Stop() error {
	if !atomic.CompareAndSwapInt32(&hm.running, 1, 0) {
		hm.logger.Warn("Health manager is not running")
		return types.ErrServerNotRunning
	}

	return nil
}

func (hm *Manager) IsRunning() bool {
	return atomic.LoadInt32(&hm.running) == 1
}

func (hm *Manager) executeCheck(ctx context.Context, name string, checker types.HealthChecker) types.HealthCheck {
	start := time.Now()

	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resultChan := make(chan types.HealthCheck, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- types.HealthCheck{
					Name:      name,
					Status:    types.StatusUnhealthy,
					Message:   fmt.Sprintf("Health check panicked: %v", r),
					LastCheck: time.Now(),
					Duration:  time.Since(start),
				}
			}
		}()

		result := checker(checkCtx)
		result.Name = name
		result.LastCheck = time.Now()
		result.Duration = time.Since(start)
		resultChan <- result
	}()

	select {
	case result := <-resultChan:
		return result
	case <-hm.ctx.Done():
		return types.HealthCheck{
			Name:      name,
			Status:    types.StatusUnhealthy,
			Message:   "Health check timeout",
			LastCheck: time.Now(),
			Duration:  time.Since(start),
		}
	case <-checkCtx.Done():
		return types.HealthCheck{
			Name:      name,
			Status:    types.StatusUnhealthy,
			Message:   "Health check timeout",
			LastCheck: time.Now(),
			Duration:  time.Since(start),
		}
	}
}

func (hm *Manager) buildReport(results map[string]types.HealthCheck) types.HealthReport {
	config := hm.config.GetConfig()

	summary := types.HealthSummary{
		Total: len(results),
	}

	overallStatus := types.StatusHealthy
	for _, result := range results {
		switch result.Status {
		case types.StatusHealthy:
			summary.Healthy++
		case types.StatusUnhealthy:
			summary.Unhealthy++
			overallStatus = types.StatusUnhealthy
		case types.StatusUnknown:
			summary.Unknown++
			if overallStatus == types.StatusHealthy {
				overallStatus = types.StatusUnknown
			}
		}
	}

	return types.HealthReport{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Uptime:    time.Since(hm.startTime),
		Service: types.ServiceInfo{
			Name:    config.Name,
			Version: config.Version,
			Host:    config.Server.HTTP.Host,
			Port:    config.Server.HTTP.Port,
		},
		Checks:  results,
		Summary: summary,
	}
}

//func (hm *Manager) registerDefaultCheckers() {
//	hm.RegisterChecker("cache", func(ctx context.Context) types.HealthCheck {
//		cache := hm.cache
//		if cache == nil {
//			return types.HealthCheck{
//				Status:  types.StatusUnhealthy,
//				Message: "Cache manager not initialized",
//			}
//		}
//
//		done := make(chan error, 1)
//		testKey := "health_check_test"
//		testValue := "test"
//
//		go func() {
//			if err := cache.Set(testKey, testValue, time.Minute); err != nil {
//				done <- err
//				return
//			}
//
//			if value, exists := cache.Get(testKey); !exists || value != testValue {
//				done <- fmt.Errorf("cache read verification failed")
//				return
//			}
//
//			err := cache.Delete(testKey)
//			if err != nil {
//				hm.logger.Error("Failed to delete key", zap.Error(err))
//			}
//
//			done <- nil
//		}()
//
//		select {
//		case err := <-done:
//			if err != nil {
//				return types.HealthCheck{
//					Status:  types.StatusUnhealthy,
//					Message: fmt.Sprintf("Cache check failed: %v", err),
//				}
//			}
//			return types.HealthCheck{
//				Status:  types.StatusHealthy,
//				Message: "Cache is working",
//			}
//		case <-ctx.Done():
//			return types.HealthCheck{
//				Status:  types.StatusUnhealthy,
//				Message: "Cache health check timeout",
//			}
//		}
//	})
//	hm.RegisterChecker("http_server", func(ctx context.Context) types.HealthCheck {
//		server := hm.server
//		if server == nil {
//			return types.HealthCheck{
//				Status:  types.StatusUnhealthy,
//				Message: "HTTP server not initialized",
//			}
//		}
//
//		if !server.IsRunning() {
//			return types.HealthCheck{
//				Status:  types.StatusUnhealthy,
//				Message: "HTTP server is not running",
//			}
//		}
//
//		return types.HealthCheck{
//			Status:  types.StatusHealthy,
//			Message: "HTTP server is running",
//			Details: map[string]interface{}{
//				"address": server.GetAddr(),
//			},
//		}
//	})
//	hm.RegisterChecker("cron", func(ctx context.Context) types.HealthCheck {
//		cron := hm.cron
//		if cron == nil {
//			return types.HealthCheck{
//				Status:  types.StatusUnknown,
//				Message: "Cron scheduler not enabled",
//			}
//		}
//
//		if !cron.IsRunning() {
//			return types.HealthCheck{
//				Status:  types.StatusUnhealthy,
//				Message: "Cron scheduler is not running",
//			}
//		}
//
//		jobs := cron.GetJobs()
//		return types.HealthCheck{
//			Status:  types.StatusHealthy,
//			Message: "Cron scheduler is running",
//			Details: map[string]interface{}{
//				"jobs_count": len(jobs),
//			},
//		}
//	})
//}
