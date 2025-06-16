package metrics

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type MemoryMetrics struct {
	ctx           context.Context
	logger        types.Logger
	health        types.HealthManager
	config        *MemoryConfig
	counters      map[string]*MemoryCounter
	gauges        map[string]*MemoryGauge
	histograms    map[string]*MemoryHistogram
	summaries     map[string]*MemorySummary
	systemMetrics *SystemMetricsCollector
	mu            sync.RWMutex
	stopCleanup   chan struct{}
	running       int32
	collections   uint64
	errors        uint64
	buf           [256]byte
	strings       sync.Map
}

type MemoryConfig struct {
	RetentionPeriod time.Duration `yaml:"retention_period" json:"retention_period"`
	MaxMetrics      int           `yaml:"max_metrics" json:"max_metrics"`
	CleanupInterval time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`
}

func NewMemoryMetrics(ctx context.Context, logger types.Logger, config *types.MetricsConfig, health types.HealthManager) (types.MetricsManager, error) {
	var memConfig = &MemoryConfig{
		RetentionPeriod: 24 * time.Hour,
		MaxMetrics:      10000,
		CleanupInterval: time.Hour,
	}

	if config.Config != nil {
		err := utils.UnmarshalConfig(config.Config, memConfig)
		if err != nil {
			return nil, types.WrapError(err, "failed to unmarshal memory metrics config")
		}
	}

	metrics := &MemoryMetrics{
		ctx:         ctx,
		logger:      logger,
		health:      health,
		config:      memConfig,
		counters:    make(map[string]*MemoryCounter),
		gauges:      make(map[string]*MemoryGauge),
		histograms:  make(map[string]*MemoryHistogram),
		summaries:   make(map[string]*MemorySummary),
		stopCleanup: make(chan struct{}),
		running:     0,
	}

	return metrics, nil
}

func (m *MemoryMetrics) RegisterRoutes(router types.HTTPRouter) {
	config := &types.RouteConfig{
		Cache: &types.CacheHandlerConfig{
			Enabled: false,
		},
		Timeout:             time.Duration(5) * time.Second,
		DisabledMiddlewares: []string{"Auth", "BodyLimit", "Cache", "Cors", "Logging"},
		Doc:                 nil, //TODO: Add doc?
	}

	router.Add("GET", "/metrics", m.handleMetrics, config)
	router.Add("GET", "/stats", m.handleStats, config)
}

func (m *MemoryMetrics) handleMetrics(ctx *fasthttp.RequestCtx) {
	metricsData, err := m.GetMetrics()
	if err != nil {
		ctx.Error("failed to get metrics", fasthttp.StatusInternalServerError)
	}

	ctx.Response.Header.SetContentType("application/json")
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
	_, err = ctx.Write(metricsData)
	if err != nil {
		m.logger.Error("Failed to write metrics", zap.Error(err))
		return
	}
}

func (m *MemoryMetrics) handleStats(ctx *fasthttp.RequestCtx) {
	statsData, err := m.GetStats()
	if err != nil {
		ctx.Error("failed to get metrics", fasthttp.StatusInternalServerError)
	}

	ctx.Response.Header.SetContentType("application/json")
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
	_, err = ctx.Write(statsData)
	if err != nil {
		m.logger.Error("Failed to write metrics", zap.Error(err))
		return
	}
}

func (m *MemoryMetrics) Counter(name string, labels map[string]string) types.Counter {
	key := m.buildKey(name, labels)

	m.mu.Lock()
	defer m.mu.Unlock()

	if counter, exists := m.counters[key]; exists {
		return counter
	}

	counter := &MemoryCounter{
		name:   name,
		labels: labels,
		value:  0,
	}
	m.counters[key] = counter

	return counter
}

func (m *MemoryMetrics) Gauge(name string, labels map[string]string) types.Gauge {
	key := m.buildKey(name, labels)

	m.mu.Lock()
	defer m.mu.Unlock()

	if gauge, exists := m.gauges[key]; exists {
		return gauge
	}

	gauge := &MemoryGauge{
		name:   name,
		labels: labels,
		value:  0,
	}
	m.gauges[key] = gauge

	return gauge
}

func (m *MemoryMetrics) Histogram(name string, buckets []float64, labels map[string]string) types.Histogram {
	key := m.buildKey(name, labels)

	m.mu.Lock()
	defer m.mu.Unlock()

	if histogram, exists := m.histograms[key]; exists {
		return histogram
	}

	histogram := &MemoryHistogram{
		name:    name,
		labels:  labels,
		buckets: buckets,
		counts:  make([]uint64, len(buckets)+1),
		sum:     0,
		count:   0,
	}
	m.histograms[key] = histogram

	return histogram
}

func (m *MemoryMetrics) Summary(name string, objectives map[float64]float64, labels map[string]string) types.Summary {
	key := m.buildKey(name, labels)

	m.mu.Lock()
	defer m.mu.Unlock()

	if summary, exists := m.summaries[key]; exists {
		return summary
	}

	summary := &MemorySummary{
		name:       name,
		labels:     labels,
		objectives: objectives,
		values:     make([]float64, 0),
		sum:        0,
		count:      0,
	}
	m.summaries[key] = summary

	m.logger.Debug("Summary created", zap.String("name", name))
	return summary
}

func (m *MemoryMetrics) RegisterSystemMetrics() error {
	m.Gauge("system_memory_usage_bytes", map[string]string{"type": "heap_inuse"})
	m.Gauge("system_memory_usage_bytes", map[string]string{"type": "heap_alloc"})
	m.Gauge("system_memory_usage_bytes", map[string]string{"type": "sys"})
	m.Gauge("system_memory_usage_bytes", map[string]string{"type": "stack_inuse"})
	m.Gauge("system_goroutines_count", nil)
	m.Gauge("system_heap_objects_count", nil)
	m.Gauge("system_uptime_seconds", nil)
	m.Gauge("system_cpu_usage_percent", nil)
	m.Gauge("system_last_gc_timestamp", nil)
	m.Histogram("system_gc_duration_seconds", []float64{0.001, 0.01, 0.1, 1.0}, nil)

	m.logger.Info("System metrics registered")
	return nil
}

func (m *MemoryMetrics) StartSystemCollection() error {
	if m.systemMetrics == nil {
		m.systemMetrics = NewSystemMetricsCollector(m.ctx, m.logger, m)
	}
	return m.systemMetrics.Start()
}

func (m *MemoryMetrics) StopSystemCollection() error {
	if m.systemMetrics != nil {
		return m.systemMetrics.Stop()
	}
	return nil
}

func (m *MemoryMetrics) GetMetrics() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var metrics []types.MetricValue

	for _, counter := range m.counters {
		metrics = append(metrics, types.MetricValue{
			Name:      counter.name,
			Type:      "counter",
			Value:     counter.Get(),
			Labels:    counter.labels,
			Timestamp: time.Now(),
		})
	}

	for _, gauge := range m.gauges {
		metrics = append(metrics, types.MetricValue{
			Name:      gauge.name,
			Type:      "gauge",
			Value:     gauge.Get(),
			Labels:    gauge.labels,
			Timestamp: time.Now(),
		})
	}

	atomic.AddUint64(&m.collections, 1)
	return utils.Marshal(metrics)
}

func (m *MemoryMetrics) GetStats() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := types.MetricsStats{
		TotalMetrics:     len(m.counters) + len(m.gauges) + len(m.histograms) + len(m.summaries),
		CounterMetrics:   len(m.counters),
		GaugeMetrics:     len(m.gauges),
		HistogramMetrics: len(m.histograms),
		SummaryMetrics:   len(m.summaries),
		LastUpdate:       time.Now(),
		Collections:      atomic.LoadUint64(&m.collections),
		Errors:           atomic.LoadUint64(&m.errors),
	}

	return utils.Marshal(stats)
}

func (m *MemoryMetrics) Start() error {
	if !atomic.CompareAndSwapInt32(&m.running, 0, 1) {
		m.logger.Warn("Memory metrics is already running")
		return types.ErrServerAlreadyRunning
	}

	go m.cleanupRoutine()

	m.logger.Info("Memory metrics started")
	return nil
}

func (m *MemoryMetrics) Stop() error {
	if !atomic.CompareAndSwapInt32(&m.running, 1, 0) {
		m.logger.Warn("Memory metrics is not running")
		return types.ErrServerNotRunning
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counters = make(map[string]*MemoryCounter)
	m.gauges = make(map[string]*MemoryGauge)
	m.histograms = make(map[string]*MemoryHistogram)
	m.summaries = make(map[string]*MemorySummary)

	close(m.stopCleanup)

	err := m.StopSystemCollection()
	if err != nil {
		return err
	}

	m.logger.Info("Memory metrics stopped")
	return nil
}

func (m *MemoryMetrics) IsRunning() bool {
	return atomic.LoadInt32(&m.running) == 1
}

func (m *MemoryMetrics) buildKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}

	buf := m.buf[:0]
	buf = append(buf, name...)

	for k, v := range labels {
		buf = append(buf, '_')
		buf = append(buf, k...)
		buf = append(buf, '_')
		buf = append(buf, v...)
	}

	return utils.Intern(buf)
}

func (m *MemoryMetrics) cleanupRoutine() {
	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.ctx.Done():
		case <-m.stopCleanup:
			return
		}
	}
}

func (m *MemoryMetrics) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	totalMetrics := len(m.counters) + len(m.gauges) + len(m.histograms) + len(m.summaries)
	if totalMetrics <= m.config.MaxMetrics {
		return
	}

	toRemove := totalMetrics - m.config.MaxMetrics
	removed := 0

	for key := range m.counters {
		if removed >= toRemove {
			break
		}
		delete(m.counters, key)
		removed++
	}

	m.logger.Debug("Memory metrics cleanup completed", zap.Int("removed", removed))
}

type MemoryCounter struct {
	name   string
	labels map[string]string
	value  uint64
}

func (c *MemoryCounter) Inc() {
	atomic.AddUint64(&c.value, 1)
}
func (c *MemoryCounter) Add(value float64) {
	atomic.AddUint64(&c.value, uint64(value))
}
func (c *MemoryCounter) Get() float64 {
	return float64(atomic.LoadUint64(&c.value))
}

type MemoryGauge struct {
	name   string
	labels map[string]string
	value  uint64
}

func (g *MemoryGauge) Set(value float64) {
	atomic.StoreUint64(&g.value, math.Float64bits(value))
}
func (g *MemoryGauge) Inc() {
	for {
		old := atomic.LoadUint64(&g.value)
		oldFloat := math.Float64frombits(old)
		newFloat := oldFloat + 1
		if atomic.CompareAndSwapUint64(&g.value, old, math.Float64bits(newFloat)) {
			break
		}
	}
}
func (g *MemoryGauge) Dec() {
	for {
		old := atomic.LoadUint64(&g.value)
		oldFloat := math.Float64frombits(old)
		newFloat := oldFloat - 1
		if atomic.CompareAndSwapUint64(&g.value, old, math.Float64bits(newFloat)) {
			break
		}
	}
}
func (g *MemoryGauge) Add(value float64) {
	for {
		old := atomic.LoadUint64(&g.value)
		oldFloat := math.Float64frombits(old)
		newFloat := oldFloat + value
		if atomic.CompareAndSwapUint64(&g.value, old, math.Float64bits(newFloat)) {
			break
		}
	}
}
func (g *MemoryGauge) Sub(value float64) {
	for {
		old := atomic.LoadUint64(&g.value)
		oldFloat := math.Float64frombits(old)
		newFloat := oldFloat - value
		if atomic.CompareAndSwapUint64(&g.value, old, math.Float64bits(newFloat)) {
			break
		}
	}
}
func (g *MemoryGauge) Get() float64 {
	return math.Float64frombits(atomic.LoadUint64(&g.value))
}

type MemoryHistogram struct {
	name    string
	labels  map[string]string
	buckets []float64
	counts  []uint64
	sum     uint64
	count   uint64
}

func (h *MemoryHistogram) Observe(value float64) {
	atomic.AddUint64(&h.count, 1)
	atomic.AddUint64(&h.sum, uint64(value*1000000))

	bucketIndex := len(h.buckets)
	for i, bucket := range h.buckets {
		if value <= bucket {
			bucketIndex = i
			break
		}
	}
	atomic.AddUint64(&h.counts[bucketIndex], 1)
}
func (h *MemoryHistogram) ObserveDuration(start time.Time) {
	duration := time.Since(start).Seconds()
	h.Observe(duration)
}
func (h *MemoryHistogram) GetCount() uint64 {
	return atomic.LoadUint64(&h.count)
}
func (h *MemoryHistogram) GetSum() float64 {
	return float64(atomic.LoadUint64(&h.sum)) / 1000000
}

type MemorySummary struct {
	name       string
	labels     map[string]string
	objectives map[float64]float64
	values     []float64
	sum        uint64
	count      uint64
}

func (s *MemorySummary) Observe(value float64) {
	atomic.AddUint64(&s.count, 1)
	atomic.AddUint64(&s.sum, uint64(value*1000000))

	s.values = append(s.values, value)
	if len(s.values) > 1000 {
		s.values = s.values[1:]
	}
}
func (s *MemorySummary) ObserveDuration(start time.Time) {
	duration := time.Since(start).Seconds()
	s.Observe(duration)
}
func (s *MemorySummary) GetCount() uint64 {
	return atomic.LoadUint64(&s.count)
}
func (s *MemorySummary) GetSum() float64 {
	return float64(atomic.LoadUint64(&s.sum)) / 1000000
}
