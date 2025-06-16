package metrics

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/saiset-co/sai-service/types"
)

type SystemMetricsCollector struct {
	ctx            context.Context
	logger         types.Logger
	metrics        types.MetricsManager
	stopChan       chan struct{}
	running        int32
	startTime      time.Time
	lastMemStats   runtime.MemStats
	lastMemUpdate  int64
	memStatsMu     sync.RWMutex
	lastCPUTime    int64
	lastSampleTime int64
	cpuPercent     float64
	lastGoroutines int
	lastGCTime     int64
	lastGCCount    uint32
}

func NewSystemMetricsCollector(ctx context.Context, logger types.Logger, metricsManager types.MetricsManager) *SystemMetricsCollector {
	return &SystemMetricsCollector{
		ctx:      ctx,
		logger:   logger,
		metrics:  metricsManager,
		stopChan: make(chan struct{}),
		running:  0,
	}
}

func (smc *SystemMetricsCollector) Start() error {
	if !atomic.CompareAndSwapInt32(&smc.running, 0, 1) {
		smc.logger.Warn("System metrics is already running")
		return types.ErrServerAlreadyRunning
	}

	smc.startTime = time.Now()

	go smc.collectLoop()

	smc.logger.Info("System metrics collection started")

	return nil
}

func (smc *SystemMetricsCollector) Stop() error {
	if !atomic.CompareAndSwapInt32(&smc.running, 1, 0) {
		smc.logger.Warn("System metrics is not running")
		return types.ErrServerNotRunning
	}

	close(smc.stopChan)

	smc.logger.Info("System metrics collection stopped")
	return nil
}

func (smc *SystemMetricsCollector) IsRunning() bool {
	return atomic.LoadInt32(&smc.running) == 1
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
			smc.collectHeavyMetrics()

		case <-lightTicker.C:
			smc.collectLightMetrics()

		case <-smc.stopChan:
			return
		case <-smc.ctx.Done():
			return
		}
	}
}

func (smc *SystemMetricsCollector) collectHeavyMetrics() {
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

func (smc *SystemMetricsCollector) measureCPUUsageProduction() float64 {
	goroutines := float64(runtime.NumGoroutine())
	maxProcs := float64(runtime.GOMAXPROCS(0))

	baseLoad := (goroutines / (maxProcs * 10)) * 100
	if baseLoad > 100 {
		baseLoad = 100
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	gcImpact := m.GCCPUFraction * 100

	totalCPU := baseLoad + gcImpact
	if totalCPU > 100 {
		totalCPU = 100
	}

	return totalCPU
}
