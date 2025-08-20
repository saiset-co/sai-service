package health

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

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

type Manager struct {
	ctx             context.Context
	cancel          context.CancelFunc
	config          types.ConfigManager
	logger          types.Logger
	router          types.HTTPRouter
	checkers        map[string]types.HealthChecker
	results         map[string]types.HealthCheck
	startTime       time.Time
	mu              sync.RWMutex
	state           atomic.Value
	shutdownTimeout time.Duration
	checkTimeout    time.Duration
}

func NewManager(ctx context.Context, config types.ConfigManager, logger types.Logger, router types.HTTPRouter) (*Manager, error) {
	managerCtx, cancel := context.WithCancel(ctx)

	manager := &Manager{
		ctx:             managerCtx,
		cancel:          cancel,
		config:          config,
		logger:          logger,
		router:          router,
		checkers:        make(map[string]types.HealthChecker),
		results:         make(map[string]types.HealthCheck),
		shutdownTimeout: 10 * time.Second,
		checkTimeout:    5 * time.Second,
	}

	manager.state.Store(StateStopped)

	return manager, nil
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

	checkCtx, cancel := context.WithTimeout(ctx, hm.checkTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(checkCtx)
	results := make(map[string]types.HealthCheck, len(checkers))
	var resultMu sync.Mutex

	for name, checker := range checkers {
		name, checker := name, checker
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				result := hm.executeCheck(gCtx, name, checker)

				resultMu.Lock()
				results[name] = result
				resultMu.Unlock()
				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		select {
		case <-checkCtx.Done():
			hm.logger.Warn("Health check timeout, some checks may not have completed")
		default:
			hm.logger.Error("Error during health checks", zap.Error(err))
		}
	}

	hm.mu.Lock()
	hm.results = results
	hm.mu.Unlock()

	return hm.buildReport(results)
}

func (hm *Manager) Start() error {
	if !hm.transitionState(StateStopped, StateStarting) {
		hm.logger.Warn("Health manager is already running")
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if hm.getState() == StateStarting {
			hm.setState(StateRunning)
		}
	}()

	hm.startTime = time.Now()
	hm.registerRoutes()

	hm.logger.Info("Health manager started")
	return nil
}

func (hm *Manager) Stop() error {
	if !hm.transitionState(StateRunning, StateStopping) {
		hm.logger.Warn("Health manager is not running")
		return types.ErrServerNotRunning
	}

	defer func() {
		hm.setState(StateStopped)
		hm.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), hm.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		hm.mu.Lock()
		defer hm.mu.Unlock()
		hm.checkers = make(map[string]types.HealthChecker)
		return nil
	})

	if err := g.Wait(); err != nil {
		select {
		case <-gCtx.Done():
			hm.logger.Warn("Health manager stop timeout, some components may not have stopped gracefully")
		default:
			hm.logger.Error("Error during health manager shutdown", zap.Error(err))
		}
	} else {
		hm.logger.Info("Health manager stopped gracefully")
	}

	return nil
}

func (hm *Manager) IsRunning() bool {
	return hm.getState() == StateRunning
}

func (hm *Manager) getState() State {
	return hm.state.Load().(State)
}

func (hm *Manager) setState(newState State) bool {
	currentState := hm.getState()
	return hm.state.CompareAndSwap(currentState, newState)
}

func (hm *Manager) transitionState(from, to State) bool {
	return hm.state.CompareAndSwap(from, to)
}

func (hm *Manager) registerRoutes() {
	config := &types.RouteConfig{
		Cache: &types.CacheHandlerConfig{
			Enabled: false,
		},
		Timeout:             time.Duration(5) * time.Second,
		DisabledMiddlewares: []string{"auth", "cache"},
		Doc:                 nil,
	}

	hm.router.Add("GET", "/version", hm.handleVersion, config)
	hm.router.Add("GET", "/health", hm.handleHealth, config)
}

func (hm *Manager) handleVersion(ctx *types.RequestCtx) {
	if !hm.IsRunning() {
		ctx.Error(types.ErrHealthIsNotRunning, fasthttp.StatusServiceUnavailable)
		return
	}

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

func (hm *Manager) handleHealth(ctx *types.RequestCtx) {
	if !hm.IsRunning() {
		ctx.Error(types.ErrHealthIsNotRunning, fasthttp.StatusServiceUnavailable)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)

	report := hm.Check(ctx)

	specData, err := utils.Marshal(report)
	if err != nil {
		hm.logger.Error("Failed to encode health report", zap.Error(err))
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	_, err = ctx.Write(specData)
	if err != nil {
		hm.logger.Error("Failed to write health report", zap.Error(err))
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}
}

func (hm *Manager) executeCheck(ctx context.Context, name string, checker types.HealthChecker) types.HealthCheck {
	start := time.Now()

	checkCtx, cancel := context.WithTimeout(ctx, hm.checkTimeout)
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
			Message:   "Health manager shutting down",
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
