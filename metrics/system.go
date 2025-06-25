package metrics

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
)

type SystemState int32

const (
	SystemStateStopped SystemState = iota
	SystemStateStarting
	SystemStateRunning
	SystemStateStopping
)

type SystemMetricsCollector struct {
	ctx             context.Context
	cancel          context.CancelFunc
	logger          types.Logger
	metrics         types.MetricsManager
	state           atomic.Value
	shutdownTimeout time.Duration
	startTime       time.Time
	lastMemStats    runtime.MemStats
	lastMemUpdate   int64
	memStatsMu      sync.RWMutex
	lastCPUTime     int64
	lastSampleTime  int64
	cpuPercent      float64
	lastGoroutines  int
	lastGCTime      int64
	lastGCCount     uint32
	stopChan        chan struct{}
}

func NewSystemMetricsCollector(ctx context.Context, logger types.Logger, metricsManager types.MetricsManager) *SystemMetricsCollector {
	systemCtx, cancel := context.WithCancel(ctx)

	collector := &SystemMetricsCollector{
		ctx:             systemCtx,
		cancel:          cancel,
		logger:          logger,
		metrics:         metricsManager,
		shutdownTimeout: 10 * time.Second,
		stopChan:        make(chan struct{}),
	}

	collector.state.Store(SystemStateStopped)

	return collector
}

func (smc *SystemMetricsCollector) Start() error {
	if !smc.transitionState(SystemStateStopped, SystemStateStarting) {
		smc.logger.Warn("System metrics is already running")
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if smc.getState() == SystemStateStarting {
			smc.setState(SystemStateRunning)
		}
	}()

	smc.startTime = time.Now()

	ctx, cancel := context.WithTimeout(smc.ctx, 30*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			go smc.collectLoop()
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			smc.setState(SystemStateStopped)
			return types.NewErrorf("system metrics start timeout")
		default:
			smc.setState(SystemStateStopped)
			return types.WrapError(err, "failed to start system metrics")
		}
	}

	smc.logger.Info("System metrics collection started")
	return nil
}

func (smc *SystemMetricsCollector) Stop() error {
	if !smc.transitionState(SystemStateRunning, SystemStateStopping) {
		smc.logger.Warn("System metrics is not running")
		return types.ErrServerNotRunning
	}

	defer func() {
		smc.setState(SystemStateStopped)
		smc.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), smc.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			close(smc.stopChan)
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			smc.logger.Warn("System metrics stop timeout, some components may not have stopped gracefully")
		default:
			smc.logger.Error("Error during system metrics shutdown", zap.Error(err))
		}
	} else {
		smc.logger.Info("System metrics collection stopped gracefully")
	}

	return nil
}

func (smc *SystemMetricsCollector) IsRunning() bool {
	return smc.getState() == SystemStateRunning
}

func (smc *SystemMetricsCollector) getState() SystemState {
	return smc.state.Load().(SystemState)
}

func (smc *SystemMetricsCollector) setState(newState SystemState) bool {
	currentState := smc.getState()
	return smc.state.CompareAndSwap(currentState, newState)
}

func (smc *SystemMetricsCollector) transitionState(from, to SystemState) bool {
	return smc.state.CompareAndSwap(from, to)
}

func (smc *SystemMetricsCollector) collectLoop() {
	heavyTicker := time.NewTicker(15 * time.Second)
	lightTicker := time.NewTicker(5 * time.Second)
	defer heavyTicker.Stop()
	defer lightTicker.Stop()

	smc.collectHeavyMetrics()
	smc.collectLightMetrics()

	for {
		select {
		case <-heavyTicker.C:
			if !smc.IsRunning() {
				return
			}
			smc.collectHeavyMetrics()

		case <-lightTicker.C:
			if !smc.IsRunning() {
				return
			}
			smc.collectLightMetrics()

		case <-smc.stopChan:
			return
		case <-smc.ctx.Done():
			return
		}
	}
}

func (smc *SystemMetricsCollector) collectHeavyMetrics() {
	if smc.metrics == nil {
		return
	}

	now := time.Now().UnixNano()

	smc.memStatsMu.RLock()
	lastUpdate := atomic.LoadInt64(&smc.lastMemUpdate)
	smc.memStatsMu.RUnlock()

	if now-lastUpdate > 10*int64(time.Second) {
		smc.memStatsMu.Lock()
		if now-atomic.LoadInt64(&smc.lastMemUpdate) > 10*int64(time.Second) {
			runtime.ReadMemStats(&smc.lastMemStats)
			atomic.StoreInt64(&smc.lastMemUpdate, now)
		}
		m := smc.lastMemStats
		smc.memStatsMu.Unlock()

		smc.updateMemoryMetrics(&m)
		smc.updateGCMetrics(&m)
	} else {
		smc.memStatsMu.RLock()
		m := smc.lastMemStats
		smc.memStatsMu.RUnlock()
		smc.updateMemoryMetrics(&m)
		smc.updateGCMetrics(&m)
	}
}

func (smc *SystemMetricsCollector) collectLightMetrics() {
	if smc.metrics == nil {
		return
	}

	currentGoroutines := runtime.NumGoroutine()
	if currentGoroutines != smc.lastGoroutines {
		smc.metrics.Gauge("system_goroutines_count", nil).Set(float64(currentGoroutines))
		smc.lastGoroutines = currentGoroutines
	}

	uptime := time.Since(smc.startTime)
	smc.metrics.Gauge("system_uptime_seconds", nil).Set(uptime.Seconds())

	cpuPercent := smc.measureCPUUsageEfficient()
	smc.metrics.Gauge("system_cpu_usage_percent", nil).Set(cpuPercent)

	smc.updateSystemInfo()
}

func (smc *SystemMetricsCollector) updateMemoryMetrics(m *runtime.MemStats) {
	metrics := []struct {
		name   string
		labels map[string]string
		value  float64
	}{
		{"system_memory_usage_bytes", map[string]string{"type": "heap_inuse"}, float64(m.HeapInuse)},
		{"system_memory_usage_bytes", map[string]string{"type": "heap_alloc"}, float64(m.HeapAlloc)},
		{"system_memory_usage_bytes", map[string]string{"type": "sys"}, float64(m.Sys)},
		{"system_memory_usage_bytes", map[string]string{"type": "stack_inuse"}, float64(m.StackInuse)},
		{"system_heap_objects_count", nil, float64(m.HeapObjects)},
		{"system_heap_size_bytes", nil, float64(m.HeapSys)},
		{"system_heap_idle_bytes", nil, float64(m.HeapIdle)},
		{"system_heap_released_bytes", nil, float64(m.HeapReleased)},
		{"system_next_gc_bytes", nil, float64(m.NextGC)},
		{"system_mallocs_total", nil, float64(m.Mallocs)},
		{"system_frees_total", nil, float64(m.Frees)},
		{"system_stack_sys_bytes", nil, float64(m.StackSys)},
	}

	for _, metric := range metrics {
		smc.metrics.Gauge(metric.name, metric.labels).Set(metric.value)
	}
}

func (smc *SystemMetricsCollector) updateGCMetrics(m *runtime.MemStats) {
	if m.NumGC != smc.lastGCCount {
		smc.metrics.Gauge("system_gc_cycles_total", nil).Set(float64(m.NumGC))
		smc.metrics.Gauge("system_gc_cpu_percent", nil).Set(m.GCCPUFraction * 100)
		smc.lastGCCount = m.NumGC

		if m.NumGC > 0 {
			smc.metrics.Gauge("system_last_gc_timestamp", nil).Set(float64(m.LastGC) / 1e9)

			lastPauseIndex := (m.NumGC + 255) % 256
			lastPause := m.PauseNs[lastPauseIndex]

			if lastPause > 0 && int64(lastPause) != smc.lastGCTime {
				smc.metrics.Histogram("system_gc_duration_seconds",
					[]float64{0.001, 0.01, 0.1, 1.0},
					nil,
				).Observe(float64(lastPause) / 1e9)
				smc.lastGCTime = int64(lastPause)
			}
		}
	}
}

func (smc *SystemMetricsCollector) updateSystemInfo() {
	static := time.Now().Unix()
	if static%60 == 0 {
		smc.metrics.Gauge("system_max_procs", nil).Set(float64(runtime.GOMAXPROCS(0)))
		smc.metrics.Gauge("system_go_version", map[string]string{
			"version": runtime.Version(),
		}).Set(1)
	}
}

func (smc *SystemMetricsCollector) measureCPUUsageEfficient() float64 {
	now := time.Now().UnixNano()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	currentCPUTime := now - int64(m.PauseTotalNs)

	lastCPU := atomic.LoadInt64(&smc.lastCPUTime)
	lastTime := atomic.LoadInt64(&smc.lastSampleTime)

	if lastTime > 0 {
		cpuDelta := float64(currentCPUTime - lastCPU)
		timeDelta := float64(now - lastTime)

		if timeDelta > 0 {
			numCPU := float64(runtime.NumCPU())
			cpuPercent := (cpuDelta / timeDelta) * 100.0 / numCPU

			if cpuPercent < 0 {
				cpuPercent = 0
			}
			if cpuPercent > 100 {
				cpuPercent = 100
			}

			smc.cpuPercent = cpuPercent
		}
	}

	atomic.StoreInt64(&smc.lastCPUTime, currentCPUTime)
	atomic.StoreInt64(&smc.lastSampleTime, now)

	return smc.cpuPercent
}
