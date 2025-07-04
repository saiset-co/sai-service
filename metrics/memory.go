package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
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

type MemoryConfig struct {
	RetentionPeriod time.Duration `yaml:"retention_period" json:"retention_period"`
	MaxMetrics      int           `yaml:"max_metrics" json:"max_metrics"`
	CleanupInterval time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`
}

type MemoryMetrics struct {
	ctx             context.Context
	cancel          context.CancelFunc
	logger          types.Logger
	router          types.HTTPRouter
	health          types.HealthManager
	config          *MemoryConfig
	counters        map[string]*MemoryCounter
	gauges          map[string]*MemoryGauge
	histograms      map[string]*MemoryHistogram
	summaries       map[string]*MemorySummary
	systemMetrics   atomic.Pointer[*SystemMetricsCollector]
	state           atomic.Value
	stopCleanup     chan struct{}
	shutdownTimeout time.Duration
	collections     uint64
	errors          uint64
	buf             [256]byte
	mu              sync.RWMutex
}

type MetricValue struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels"`
	Timestamp time.Time         `json:"timestamp"`
	Help      string            `json:"help"`
}

type MetricStats struct {
	TotalMetrics     int       `json:"total_metrics"`
	CounterMetrics   int       `json:"counter_metrics"`
	GaugeMetrics     int       `json:"gauge_metrics"`
	HistogramMetrics int       `json:"histogram_metrics"`
	SummaryMetrics   int       `json:"summary_metrics"`
	LastUpdate       time.Time `json:"last_update"`
	MemoryUsage      int64     `json:"memory_usage"`
	Collections      uint64    `json:"collections"`
	Errors           uint64    `json:"errors"`
}

type DashboardData struct {
	Stats  *MetricStats
	Groups map[string][]MetricValue
	Error  string
}

const dashboardTemplate = `<!DOCTYPE html>
<html>
<head>
	<title>SAI Service Metrics Dashboard</title>
	<meta http-equiv="refresh" content="5">
	<style>
		body { 
			font-family: 'Segoe UI', Arial, sans-serif; 
			margin: 0; 
			padding: 20px; 
			background: #f5f7fa;
		}
		.header {
			background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
			color: white;
			padding: 20px;
			border-radius: 10px;
			margin-bottom: 20px;
			box-shadow: 0 4px 15px rgba(0,0,0,0.1);
			text-align: center;
		}
		.header h1 { margin: 0; font-size: 28px; }
		.header p { margin: 10px 0 0 0; opacity: 0.9; }
		
		.stats-overview {
			display: grid;
			grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
			gap: 15px;
			margin-bottom: 25px;
		}
		
		.stat-card {
			background: white;
			padding: 20px;
			border-radius: 10px;
			box-shadow: 0 2px 10px rgba(0,0,0,0.1);
			text-align: center;
			border-left: 4px solid #ddd;
			transition: transform 0.2s ease;
		}
		.stat-card:hover { transform: translateY(-2px); }
		
		.stat-card.primary { border-left-color: #3498db; }
		.stat-card.success { border-left-color: #2ecc71; }
		.stat-card.warning { border-left-color: #f39c12; }
		.stat-card.danger { border-left-color: #e74c3c; }
		.stat-card.info { border-left-color: #9b59b6; }
		
		.stat-value {
			font-size: 24px;
			font-weight: bold;
			color: #333;
			margin-bottom: 5px;
		}
		.stat-label {
			font-size: 12px;
			color: #666;
			text-transform: uppercase;
			letter-spacing: 0.5px;
		}
		.stat-subtitle {
			font-size: 10px;
			color: #999;
			margin-top: 5px;
		}
		
		.health-indicator {
			display: inline-block;
			width: 8px;
			height: 8px;
			border-radius: 50%;
			margin-right: 5px;
		}
		.health-good { background: #2ecc71; }
		.health-warning { background: #f39c12; }
		.health-error { background: #e74c3c; }
		
		.metrics-grid {
			display: grid;
			grid-template-columns: repeat(auto-fit, minmax(450px, 1fr));
			gap: 20px;
		}
		.metric-group { 
			background: white;
			border-radius: 10px;
			padding: 20px;
			box-shadow: 0 2px 10px rgba(0,0,0,0.1);
			border-left: 4px solid #ddd;
		}
		.metric-group.system { border-left-color: #4CAF50; }
		.metric-group.http { border-left-color: #2196F3; }
		.metric-group.action { border-left-color: #FF9800; }
		.metric-group.webhook { border-left-color: #9C27B0; }
		.metric-group.cron { border-left-color: #607D8B; }
		.metric-group.other { border-left-color: #795548; }
		
		.metric-group h3 { 
			margin: 0 0 15px 0; 
			color: #333;
			font-size: 18px;
			display: flex;
			align-items: center;
			gap: 10px;
			border-bottom: 1px solid #eee;
			padding-bottom: 10px;
		}
		.metric-item { 
			display: flex;
			justify-content: space-between;
			align-items: center;
			margin: 10px 0; 
			padding: 12px 15px;
			background: #f8f9fa;
			border-radius: 8px;
			transition: all 0.2s ease;
		}
		.metric-item:hover {
			background: #e9ecef;
			transform: translateY(-1px);
		}
		
		.metric-left {
			flex: 1;
		}
		.metric-type {
			font-weight: bold;
			padding: 3px 8px;
			border-radius: 4px;
			font-size: 10px;
			text-transform: uppercase;
			display: inline-block;
			margin-bottom: 5px;
		}
		.counter { background: #ffebee; color: #c62828; }
		.gauge { background: #e3f2fd; color: #1565c0; }
		.histogram { background: #fff3e0; color: #ef6c00; }
		.summary { background: #f3e5f5; color: #7b1fa2; }
		
		.metric-value {
			font-weight: bold;
			font-size: 18px;
			color: #333;
		}
		.metric-labels {
			font-size: 11px;
			color: #666;
			margin-top: 3px;
			font-style: italic;
		}
		.metric-right {
			text-align: right;
		}
		.timestamp {
			font-size: 10px;
			color: #999;
		}
		.icon { 
			font-size: 20px; 
		}
		.refresh-notice {
			text-align: center;
			margin-top: 20px;
			font-size: 12px;
			color: #666;
			background: #e8f5e8;
			padding: 10px;
			border-radius: 5px;
		}
		.empty-state {
			text-align: center;
			padding: 40px;
			color: #666;
		}
		.error-state {
			text-align: center;
			padding: 40px;
			color: #e74c3c;
			background: #fff5f5;
			border-radius: 10px;
			margin-bottom: 20px;
		}
	</style>
</head>
<body>
	<div class="header">
		<h1>📊 SAI Service Metrics Dashboard</h1>
		<p>Real-time monitoring • Auto-refresh every 5 seconds</p>
	</div>
	
	{{if .Error}}
	<div class="error-state">
		<h3>❌ Error Loading Metrics</h3>
		<p>{{.Error}}</p>
	</div>
	{{else if .Stats}}
	
	<div class="stats-overview">
		<div class="stat-card primary">
			<div class="stat-value">{{.Stats.TotalMetrics}}</div>
			<div class="stat-label">Total Metrics</div>
			<div class="stat-subtitle">{{formatDuration .Stats.LastUpdate}} ago</div>
		</div>
		
		<div class="stat-card success">
			<div class="stat-value">
				<span class="health-indicator {{getHealthStatus .Stats.Errors}}"></span>
				{{.Stats.Collections}}
			</div>
			<div class="stat-label">Collections</div>
			<div class="stat-subtitle">{{if gt .Stats.Errors 0}}{{.Stats.Errors}} errors{{else}}No errors{{end}}</div>
		</div>
		
		<div class="stat-card warning">
			<div class="stat-value">{{.Stats.CounterMetrics}}</div>
			<div class="stat-label">Counters</div>
			<div class="stat-subtitle">{{printf "%.1f%%" (percentage .Stats.CounterMetrics .Stats.TotalMetrics)}}</div>
		</div>
		
		<div class="stat-card info">
			<div class="stat-value">{{.Stats.GaugeMetrics}}</div>
			<div class="stat-label">Gauges</div>
			<div class="stat-subtitle">{{printf "%.1f%%" (percentage .Stats.GaugeMetrics .Stats.TotalMetrics)}}</div>
		</div>
		
		<div class="stat-card danger">
			<div class="stat-value">{{.Stats.HistogramMetrics}}</div>
			<div class="stat-label">Histograms</div>
			<div class="stat-subtitle">{{printf "%.1f%%" (percentage .Stats.HistogramMetrics .Stats.TotalMetrics)}}</div>
		</div>
		
		{{if gt .Stats.SummaryMetrics 0}}
		<div class="stat-card primary">
			<div class="stat-value">{{.Stats.SummaryMetrics}}</div>
			<div class="stat-label">Summaries</div>
			<div class="stat-subtitle">{{printf "%.1f%%" (percentage .Stats.SummaryMetrics .Stats.TotalMetrics)}}</div>
		</div>
		{{end}}
	</div>

	{{if .Groups}}
	<div class="metrics-grid">
		{{range $category, $metrics := .Groups}}
		<div class="metric-group {{$category}}">
			<h3>
				<span class="icon">{{getIcon $category}}</span> 
				{{title $category}} Metrics
				<small style="margin-left: auto; font-size: 12px; color: #666;">({{len $metrics}})</small>
			</h3>
			{{range $metrics}}
			<div class="metric-item">
				<div class="metric-left">
					<div class="metric-type {{.Type}}">{{.Type}}</div>
					<div class="metric-value">{{formatValue .Value .Name}}</div>
					{{if .Labels}}
					<div class="metric-labels">{{formatLabels .Labels}}</div>
					{{end}}
				</div>
				<div class="metric-right">
					<div class="timestamp">{{.Timestamp.Format "15:04:05"}}</div>
				</div>
			</div>
			{{end}}
		</div>
		{{end}}
	</div>
	{{else}}
	<div class="empty-state">
		<h3>🔍 No metrics available</h3>
		<p>Metrics are being collected...</p>
	</div>
	{{end}}
	
	{{end}}

	<div class="refresh-notice">
		🔄 Page automatically refreshes every 5 seconds • Last updated: {{now}}
		{{if .Stats}} • Metrics last collected: {{.Stats.LastUpdate.Format "15:04:05"}}{{end}}
	</div>
</body>
</html>`

func NewMemoryMetrics(ctx context.Context, logger types.Logger, config *types.MetricsConfig, router types.HTTPRouter, health types.HealthManager) (types.MetricsManager, error) {
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

	memoryCtx, cancel := context.WithCancel(ctx)

	metrics := &MemoryMetrics{
		ctx:             memoryCtx,
		cancel:          cancel,
		logger:          logger,
		router:          router,
		health:          health,
		config:          memConfig,
		counters:        make(map[string]*MemoryCounter),
		gauges:          make(map[string]*MemoryGauge),
		histograms:      make(map[string]*MemoryHistogram),
		summaries:       make(map[string]*MemorySummary),
		stopCleanup:     make(chan struct{}),
		shutdownTimeout: 10 * time.Second,
	}

	metrics.state.Store(MemoryStateStopped)

	return metrics, nil
}

func (m *MemoryMetrics) Start() error {
	if !m.transitionState(MemoryStateStopped, MemoryStateStarting) {
		m.logger.Warn("Memory metrics is already running")
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if m.getState() == MemoryStateStarting {
			m.setState(MemoryStateRunning)
		}
	}()

	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			m.registerRoutes()
			return nil
		}
	})

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			go m.cleanupRoutine()
			return nil
		}
	})

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			if err := m.RegisterSystemMetrics(); err != nil {
				m.logger.Warn("Failed to register system metrics", zap.Error(err))
			}
			if err := m.StartSystemCollection(); err != nil {
				m.logger.Warn("Failed to start system collection", zap.Error(err))
			}
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			m.setState(MemoryStateStopped)
			return types.NewErrorf("memory metrics start timeout")
		default:
			m.setState(MemoryStateStopped)
			return types.WrapError(err, "failed to start memory metrics")
		}
	}

	m.logger.Info("Memory metrics started")
	return nil
}

func (m *MemoryMetrics) Stop() error {
	if !m.transitionState(MemoryStateRunning, MemoryStateStopping) {
		m.logger.Warn("Memory metrics is not running")
		return types.ErrServerNotRunning
	}

	defer func() {
		m.setState(MemoryStateStopped)
		m.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), m.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	if collector := m.systemMetrics.Load(); collector != nil {
		g.Go(func() error {
			if err := (*collector).Stop(); err != nil {
				m.logger.Error("Failed to stop system collection", zap.Error(err))
				return err
			}
			return nil
		})
	}

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			close(m.stopCleanup)
			return m.cleanup()
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			m.logger.Warn("Memory metrics stop timeout, some components may not have stopped gracefully")
		default:
			m.logger.Error("Error during memory metrics shutdown", zap.Error(err))
		}
	} else {
		m.logger.Info("Memory metrics stopped gracefully")
	}

	m.systemMetrics.Store(nil)
	return nil
}

func (m *MemoryMetrics) IsRunning() bool {
	return m.getState() == MemoryStateRunning
}

func (m *MemoryMetrics) getState() MemoryState {
	return m.state.Load().(MemoryState)
}

func (m *MemoryMetrics) setState(newState MemoryState) bool {
	currentState := m.getState()
	return m.state.CompareAndSwap(currentState, newState)
}

func (m *MemoryMetrics) transitionState(from, to MemoryState) bool {
	return m.state.CompareAndSwap(from, to)
}

func (m *MemoryMetrics) cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counters = make(map[string]*MemoryCounter)
	m.gauges = make(map[string]*MemoryGauge)
	m.histograms = make(map[string]*MemoryHistogram)
	m.summaries = make(map[string]*MemorySummary)

	m.logger.Info("Memory metrics cleaned up")
	return nil
}

func (m *MemoryMetrics) registerRoutes() {
	config := &types.RouteConfig{
		Cache: &types.CacheHandlerConfig{
			Enabled: false,
		},
		Timeout:             time.Duration(5) * time.Second,
		DisabledMiddlewares: []string{"cache"},
		Doc:                 nil,
	}

	m.router.Add("GET", "/metrics_ui", m.handleMetricsUI, config)
	m.router.Add("GET", "/metrics", m.handleMetrics, config)
	m.router.Add("GET", "/stats", m.handleStats, config)
}

func (m *MemoryMetrics) handleMetricsUI(ctx *types.RequestCtx) {
	if !m.IsRunning() {
		ctx.Error(types.ErrMetricsNotRunning, fasthttp.StatusServiceUnavailable)
		return
	}

	data := &DashboardData{}

	statsData, err := m.GetStats()
	if err != nil {
		data.Error = fmt.Sprintf("Failed to get stats: %v", err)
	} else {
		var stats MetricStats
		if err := json.Unmarshal(statsData, &stats); err != nil {
			data.Error = fmt.Sprintf("Failed to decode stats: %v", err)
		} else {
			data.Stats = &stats
		}
	}

	if data.Error == "" {
		metricsData, err := m.GetMetrics()
		if err != nil {
			data.Error = fmt.Sprintf("Failed to get metrics: %v", err)
		} else {
			var metrics []MetricValue
			if err := json.Unmarshal(metricsData, &metrics); err != nil {
				data.Error = fmt.Sprintf("Failed to decode metrics: %v", err)
			} else {
				data.Groups = groupMetricsByCategory(metrics)
			}
		}
	}

	tmpl := template.Must(template.New("dashboard").Funcs(template.FuncMap{
		"title":           strings.ToTitle,
		"getIcon":         getMetricIcon,
		"formatValue":     formatMetricValue,
		"formatLabels":    formatMetricLabels,
		"formatDuration":  formatDurationSince,
		"getHealthStatus": getHealthStatusClass,
		"percentage":      calculatePercentage,
		"now":             func() string { return time.Now().Format("15:04:05") },
	}).Parse(dashboardTemplate))

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		m.logger.Error("Template execution failed", zap.Error(err))
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetContentType("text/html; charset=utf-8")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_, err = ctx.Write([]byte(buf.String()))
	if err != nil {
		m.logger.Error("Failed to write response", zap.Error(err))
	}
}

func (m *MemoryMetrics) handleMetrics(ctx *types.RequestCtx) {
	if !m.IsRunning() {
		ctx.Error(types.ErrMetricsNotRunning, fasthttp.StatusServiceUnavailable)
		return
	}

	metricsData, err := m.GetMetrics()
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	ctx.Response.Header.SetContentType("application/json")
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
	_, err = ctx.Write(metricsData)
	if err != nil {
		m.logger.Error("Failed to write metrics", zap.Error(err))
		return
	}
}

func (m *MemoryMetrics) handleStats(ctx *types.RequestCtx) {
	if !m.IsRunning() {
		ctx.Error(types.ErrMetricsNotRunning, fasthttp.StatusServiceUnavailable)
		return
	}

	statsData, err := m.GetStats()
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	ctx.Response.Header.SetContentType("application/json")
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
	_, err = ctx.Write(statsData)
	if err != nil {
		m.logger.Error("Failed to write stats", zap.Error(err))
		return
	}
}

func (m *MemoryMetrics) Counter(name string, labels map[string]string) types.Counter {
	if !m.IsRunning() {
		return &MemoryCounter{}
	}

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
	if !m.IsRunning() {
		return &MemoryGauge{}
	}

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
	if !m.IsRunning() {
		return &MemoryHistogram{}
	}

	key := m.buildKey(name, labels)

	m.mu.Lock()
	defer m.mu.Unlock()

	if histogram, exists := m.histograms[key]; exists {
		return histogram
	}

	histogram := &MemoryHistogram{
		name:    name,
		labels:  labels,
		buckets: make([]float64, len(buckets)),
		counts:  make([]uint64, len(buckets)+1),
		sum:     0,
		count:   0,
	}

	copy(histogram.buckets, buckets)

	m.histograms[key] = histogram

	return histogram
}

func (m *MemoryMetrics) Summary(name string, objectives map[float64]float64, labels map[string]string) types.Summary {
	if !m.IsRunning() {
		return &MemorySummary{}
	}

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
	state := m.getState()
	if state != MemoryStateRunning && state != MemoryStateStarting {
		return types.ErrMetricsNotRunning
	}

	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
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
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		return types.WrapError(err, "failed to register system metrics")
	}

	m.logger.Info("System metrics registered")
	return nil
}

func (m *MemoryMetrics) StartSystemCollection() error {
	state := m.getState()
	if state != MemoryStateRunning && state != MemoryStateStarting {
		return types.ErrMetricsNotRunning
	}

	if m.systemMetrics.Load() == nil {
		systemMetrics := NewSystemMetricsCollector(m.ctx, m.logger, m)
		m.systemMetrics.Store(&systemMetrics)
	}

	if collector := m.systemMetrics.Load(); collector != nil {
		return (*collector).Start()
	}

	return nil
}

func (m *MemoryMetrics) StopSystemCollection() error {
	if collector := m.systemMetrics.Load(); collector != nil {
		return (*collector).Stop()
	}
	return nil
}

func (m *MemoryMetrics) GetMetrics() ([]byte, error) {
	if !m.IsRunning() {
		return nil, types.ErrMetricsNotRunning
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var metrics []types.MetricValue

	counterAgg := make(map[string]*types.MetricValue)
	for _, counter := range m.counters {
		key := m.buildMetricKey(counter.name, counter.labels)
		if existing, exists := counterAgg[key]; exists {
			existing.Value += counter.Get()
		} else {
			counterAgg[key] = &types.MetricValue{
				Name:      counter.name,
				Type:      "counter",
				Value:     counter.Get(),
				Labels:    counter.labels,
				Timestamp: time.Now(),
			}
		}
	}

	for _, metric := range counterAgg {
		metrics = append(metrics, *metric)
	}

	gaugeAgg := make(map[string]*types.MetricValue)
	for _, gauge := range m.gauges {
		key := m.buildMetricKey(gauge.name, gauge.labels)
		gaugeAgg[key] = &types.MetricValue{
			Name:      gauge.name,
			Type:      "gauge",
			Value:     gauge.Get(),
			Labels:    gauge.labels,
			Timestamp: time.Now(),
		}
	}

	for _, metric := range gaugeAgg {
		metrics = append(metrics, *metric)
	}

	for _, histogram := range m.histograms {
		metrics = append(metrics, types.MetricValue{
			Name:      histogram.name,
			Type:      "histogram",
			Value:     histogram.GetSum(),
			Labels:    histogram.labels,
			Timestamp: time.Now(),
		})
	}

	for _, summary := range m.summaries {
		metrics = append(metrics, types.MetricValue{
			Name:      summary.name,
			Type:      "summary",
			Value:     summary.GetSum(),
			Labels:    summary.labels,
			Timestamp: time.Now(),
		})
	}

	atomic.AddUint64(&m.collections, 1)
	return utils.Marshal(metrics)
}

func (m *MemoryMetrics) buildMetricKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}

	key := name
	for k, v := range labels {
		key += "_" + k + "_" + v
	}
	return key
}

func (m *MemoryMetrics) GetStats() ([]byte, error) {
	if !m.IsRunning() {
		return nil, types.ErrMetricsNotRunning
	}

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
			m.performCleanup()
		case <-m.ctx.Done():
			return
		case <-m.stopCleanup:
			return
		}
	}
}

func (m *MemoryMetrics) performCleanup() {
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

func (m *MemoryMetrics) Close() error {
	return m.Stop()
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
	if h == nil || len(h.buckets) == 0 || len(h.counts) == 0 {
		return
	}

	atomic.AddUint64(&h.count, 1)
	atomic.AddUint64(&h.sum, uint64(value*1000000))

	bucketIndex := len(h.buckets)
	for i, bucket := range h.buckets {
		if value <= bucket {
			bucketIndex = i
			break
		}
	}

	if bucketIndex < len(h.counts) {
		atomic.AddUint64(&h.counts[bucketIndex], 1)
	}
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

func groupMetricsByCategory(metrics []MetricValue) map[string][]MetricValue {
	groups := make(map[string][]MetricValue)

	for _, metric := range metrics {
		category := getMetricCategory(metric.Name)
		groups[category] = append(groups[category], metric)
	}

	for category := range groups {
		sort.Slice(groups[category], func(i, j int) bool {
			return groups[category][i].Name < groups[category][j].Name
		})
	}

	return groups
}

func getMetricCategory(name string) string {
	switch {
	case strings.HasPrefix(name, "system_"):
		return "system"
	case strings.HasPrefix(name, "http_"):
		return "http"
	case strings.HasPrefix(name, "action_"):
		return "action"
	case strings.HasPrefix(name, "webhook_"):
		return "webhook"
	case strings.HasPrefix(name, "cron_"):
		return "cron"
	default:
		return "other"
	}
}

func getMetricIcon(category string) string {
	icons := map[string]string{
		"system":  "🖥️",
		"http":    "🌐",
		"action":  "⚡",
		"webhook": "🪝",
		"cron":    "⏰",
		"other":   "📊",
	}
	if icon, exists := icons[category]; exists {
		return icon
	}
	return "📊"
}

func getHealthStatusClass(errors uint64) string {
	if errors == 0 {
		return "health-good"
	} else if errors < 5 {
		return "health-warning"
	}
	return "health-error"
}

func calculatePercentage(value, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(value) / float64(total) * 100
}

func formatDurationSince(t time.Time) string {
	duration := time.Since(t)
	if duration.Seconds() < 60 {
		return fmt.Sprintf("%.0fs", duration.Seconds())
	} else if duration.Minutes() < 60 {
		return fmt.Sprintf("%.0fm", duration.Minutes())
	}
	return fmt.Sprintf("%.0fh", duration.Hours())
}

func formatMetricValue(value float64, metricName string) string {
	switch {
	case strings.Contains(metricName, "bytes"):
		return formatBytes(value)
	case strings.Contains(metricName, "seconds") || strings.Contains(metricName, "duration"):
		return formatDuration(value)
	case strings.Contains(metricName, "percent"):
		return fmt.Sprintf("%.1f%%", value)
	case strings.Contains(metricName, "timestamp"):
		return time.Unix(int64(value), 0).Format("15:04:05")
	case value == float64(int64(value)):
		return fmt.Sprintf("%.0f", value)
	default:
		return fmt.Sprintf("%.3f", value)
	}
}

func formatBytes(bytes float64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%.0f B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", bytes/float64(div), "KMGTPE"[exp])
}

func formatDuration(seconds float64) string {
	if seconds < 0.001 {
		return fmt.Sprintf("%.0fμs", seconds*1000000)
	} else if seconds < 1 {
		return fmt.Sprintf("%.3fs", seconds)
	} else if seconds < 60 {
		return fmt.Sprintf("%.1fs", seconds)
	} else {
		return fmt.Sprintf("%.1fm", seconds/60)
	}
}

func formatMetricLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	var parts []string
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
