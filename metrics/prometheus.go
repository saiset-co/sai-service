package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type PrometheusConfig struct {
	Path            string            `yaml:"path" json:"path"`
	Registry        string            `yaml:"registry" json:"registry"`
	Namespace       string            `yaml:"namespace" json:"namespace"`
	Subsystem       string            `yaml:"subsystem" json:"subsystem"`
	Labels          map[string]string `yaml:"labels" json:"labels"`
	Gatherer        string            `yaml:"gatherer" json:"gatherer"`
	EnableGoMetrics bool              `yaml:"enable_go_metrics" json:"enable_go_metrics"`
}

type PrometheusMetrics struct {
	ctx           context.Context
	logger        types.Logger
	health        types.HealthManager
	config        *PrometheusConfig
	registry      *prometheus.Registry
	counters      map[string]*prometheus.CounterVec
	gauges        map[string]*prometheus.GaugeVec
	histograms    map[string]*prometheus.HistogramVec
	summaries     map[string]*prometheus.SummaryVec
	systemMetrics *SystemMetricsCollector
	mu            sync.RWMutex
	running       int32
}

func NewPrometheusMetrics(ctx context.Context, logger types.Logger, config *types.MetricsConfig, health types.HealthManager) (types.MetricsManager, error) {
	var promConfig = &PrometheusConfig{
		Path:            "/metrics",
		Namespace:       "sai_service",
		Subsystem:       "",
		Labels:          make(map[string]string),
		EnableGoMetrics: true,
	}

	if config.Config != nil {
		err := utils.UnmarshalConfig(config.Config, promConfig)
		if err != nil {
			return nil, types.WrapError(err, "failed to unmarshal prometheus config")
		}
	}

	registry := prometheus.NewRegistry()
	if promConfig.EnableGoMetrics {
		registry.MustRegister(collectors.NewGoCollector())
		registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}

	metrics := &PrometheusMetrics{
		ctx:        ctx,
		logger:     logger,
		health:     health,
		config:     promConfig,
		registry:   registry,
		counters:   make(map[string]*prometheus.CounterVec),
		gauges:     make(map[string]*prometheus.GaugeVec),
		histograms: make(map[string]*prometheus.HistogramVec),
		summaries:  make(map[string]*prometheus.SummaryVec),
		running:    0,
	}

	logger.Info("Prometheus metrics initialized",
		zap.String("namespace", promConfig.Namespace),
		zap.String("subsystem", promConfig.Subsystem),
		zap.Bool("go_metrics", promConfig.EnableGoMetrics))

	return metrics, nil
}

func (p *PrometheusMetrics) Start() error {
	if !atomic.CompareAndSwapInt32(&p.running, 0, 1) {
		p.logger.Warn("Prometheus metrics is already running")
		return types.ErrServerAlreadyRunning
	}

	p.logger.Info("prometheus metrics started")

	return nil
}

func (p *PrometheusMetrics) Stop() error {
	if !atomic.CompareAndSwapInt32(&p.running, 1, 0) {
		p.logger.Warn("Prometheus metrics is not running")
		return types.ErrServerNotRunning
	}

	err := p.StopSystemCollection()
	if err != nil {
		return err
	}

	p.logger.Info("prometheus metrics stopped")

	return nil
}

func (p *PrometheusMetrics) IsRunning() bool {
	return atomic.LoadInt32(&p.running) == 1
}

func (p *PrometheusMetrics) Counter(name string, labels map[string]string) types.Counter {
	key := p.buildKey(name)

	p.mu.Lock()
	defer p.mu.Unlock()

	if counter, exists := p.counters[key]; exists {
		return &PrometheusCounter{logger: p.logger, counter: counter, labels: labels}
	}

	labelNames := p.getLabelNames(labels)
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        name,
			Help:        fmt.Sprintf("Counter metric %s", name),
			ConstLabels: p.config.Labels,
		},
		labelNames,
	)

	p.registry.MustRegister(counter)
	p.counters[key] = counter

	p.logger.Debug("Prometheus counter created", zap.String("name", name))
	return &PrometheusCounter{logger: p.logger, counter: counter, labels: labels}
}

func (p *PrometheusMetrics) Gauge(name string, labels map[string]string) types.Gauge {
	key := p.buildKey(name)

	p.mu.Lock()
	defer p.mu.Unlock()

	if gauge, exists := p.gauges[key]; exists {
		return &PrometheusGauge{logger: p.logger, gauge: gauge, labels: labels}
	}

	labelNames := p.getLabelNames(labels)
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        name,
			Help:        fmt.Sprintf("Gauge metric %s", name),
			ConstLabels: p.config.Labels,
		},
		labelNames,
	)

	p.registry.MustRegister(gauge)
	p.gauges[key] = gauge

	p.logger.Debug("Prometheus gauge created", zap.String("name", name))
	return &PrometheusGauge{logger: p.logger, gauge: gauge, labels: labels}
}

func (p *PrometheusMetrics) Histogram(name string, buckets []float64, labels map[string]string) types.Histogram {
	key := p.buildKey(name)

	p.mu.Lock()
	defer p.mu.Unlock()

	if histogram, exists := p.histograms[key]; exists {
		return &PrometheusHistogram{histogram: histogram, labels: labels}
	}

	labelNames := p.getLabelNames(labels)
	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        name,
			Help:        fmt.Sprintf("Histogram metric %s", name),
			Buckets:     buckets,
			ConstLabels: p.config.Labels,
		},
		labelNames,
	)

	p.registry.MustRegister(histogram)
	p.histograms[key] = histogram

	p.logger.Debug("Prometheus histogram created", zap.String("name", name))
	return &PrometheusHistogram{histogram: histogram, labels: labels}
}

func (p *PrometheusMetrics) Summary(name string, objectives map[float64]float64, labels map[string]string) types.Summary {
	key := p.buildKey(name)

	p.mu.Lock()
	defer p.mu.Unlock()

	if summary, exists := p.summaries[key]; exists {
		return &PrometheusSummary{summary: summary, labels: labels}
	}

	labelNames := p.getLabelNames(labels)
	summary := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        name,
			Help:        fmt.Sprintf("Summary metric %s", name),
			Objectives:  objectives,
			ConstLabels: p.config.Labels,
		},
		labelNames,
	)

	p.registry.MustRegister(summary)
	p.summaries[key] = summary

	p.logger.Debug("Prometheus summary created", zap.String("name", name))
	return &PrometheusSummary{summary: summary, labels: labels}
}

func (p *PrometheusMetrics) RegisterSystemMetrics() error {
	p.Gauge("system_memory_usage_bytes", map[string]string{"type": "heap_inuse"})
	p.Gauge("system_memory_usage_bytes", map[string]string{"type": "heap_alloc"})
	p.Gauge("system_memory_usage_bytes", map[string]string{"type": "sys"})
	p.Gauge("system_memory_usage_bytes", map[string]string{"type": "stack_inuse"})
	p.Gauge("system_goroutines_count", nil)
	p.Gauge("system_heap_objects_count", nil)
	p.Gauge("system_uptime_seconds", nil)
	p.Gauge("system_cpu_usage_percent", nil)
	p.Gauge("system_last_gc_timestamp", nil)
	p.Histogram("system_gc_duration_seconds", []float64{0.001, 0.01, 0.1, 1.0}, nil)

	p.logger.Info("Prometheus system metrics registered")
	return nil
}

func (p *PrometheusMetrics) StartSystemCollection() error {
	if p.systemMetrics == nil {
		p.systemMetrics = NewSystemMetricsCollector(p.ctx, p.logger, p)
	}
	return p.systemMetrics.Start()
}

func (p *PrometheusMetrics) StopSystemCollection() error {
	if p.systemMetrics != nil {
		return p.systemMetrics.Stop()
	}
	return nil
}

func (p *PrometheusMetrics) GetMetrics() ([]byte, error) {
	gatherer := prometheus.Gatherers{p.registry}
	gathering, err := gatherer.Gather()
	if err != nil {
		p.logger.Error("Failed to gather prometheus metrics", zap.Error(err))
		return nil, err
	}

	var metrics []types.MetricValue
	for _, mf := range gathering {
		for _, m := range mf.GetMetric() {
			labels := make(map[string]string)
			for _, label := range m.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}

			var value float64
			if m.Counter != nil {
				value = m.Counter.GetValue()
			} else if m.Gauge != nil {
				value = m.Gauge.GetValue()
			} else if m.Histogram != nil {
				value = m.Histogram.GetSampleSum()
			} else if m.Summary != nil {
				value = m.Summary.GetSampleSum()
			}

			metrics = append(metrics, types.MetricValue{
				Name:      mf.GetName(),
				Type:      mf.GetType().String(),
				Value:     value,
				Labels:    labels,
				Timestamp: time.Now(),
				Help:      mf.GetHelp(),
			})
		}
	}

	return utils.Marshal(metrics)
}

func (p *PrometheusMetrics) GetStats() ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := types.MetricsStats{
		TotalMetrics:     len(p.counters) + len(p.gauges) + len(p.histograms) + len(p.summaries),
		CounterMetrics:   len(p.counters),
		GaugeMetrics:     len(p.gauges),
		HistogramMetrics: len(p.histograms),
		SummaryMetrics:   len(p.summaries),
		LastUpdate:       time.Now(),
	}

	return utils.Marshal(stats)
}

func (p *PrometheusMetrics) RegisterRoutes(router types.HTTPRouter) {
	config := &types.RouteConfig{
		Cache: &types.CacheHandlerConfig{
			Enabled: false,
		},
		Timeout:             time.Duration(5) * time.Second,
		DisabledMiddlewares: []string{"Auth", "BodyLimit", "Cache", "Cors", "Logging"},
		Doc: &types.DocConfig{
			Path:            p.config.Path,
			Method:          "GET",
			DocTitle:        "Prometheus metrics",
			DocDescription:  "",
			DocTag:          "System",
			DocRequestType:  nil,
			DocResponseType: nil,
		},
	}

	fastHandler := func(ctx *fasthttp.RequestCtx) {
		ctx.SetContentType("text/plain; version=0.0.4; charset=utf-8")
		req, _ := http.NewRequest("GET", string(ctx.RequestURI()), nil)
		w := types.NewFastResponseWriter(ctx)
		promHandler := promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{})
		promHandler.ServeHTTP(w, req)
	}

	router.Add("GET", p.config.Path, fastHandler, config)
}

func (p *PrometheusMetrics) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.counters = make(map[string]*prometheus.CounterVec)
	p.gauges = make(map[string]*prometheus.GaugeVec)
	p.histograms = make(map[string]*prometheus.HistogramVec)
	p.summaries = make(map[string]*prometheus.SummaryVec)

	p.logger.Info("Prometheus metrics closed")
	return nil
}

func (p *PrometheusMetrics) buildKey(name string) string {
	if p.config.Subsystem != "" {
		return fmt.Sprintf("%s_%s_%s", p.config.Namespace, p.config.Subsystem, name)
	}
	return fmt.Sprintf("%s_%s", p.config.Namespace, name)
}

func (p *PrometheusMetrics) getLabelNames(labels map[string]string) []string {
	names := make([]string, 0, len(labels))
	for name := range labels {
		names = append(names, name)
	}
	return names
}

type PrometheusCounter struct {
	logger  types.Logger
	counter *prometheus.CounterVec
	labels  map[string]string
}

func (c *PrometheusCounter) Inc() {
	c.counter.With(c.labels).Inc()
}

func (c *PrometheusCounter) Add(value float64) {
	c.counter.With(c.labels).Add(value)
}

func (c *PrometheusCounter) Get() float64 {
	metric := &dto.Metric{}
	err := c.counter.With(c.labels).Write(metric)
	if err != nil {
		c.logger.Error("Failed to write counter", zap.Error(err))
	}
	return metric.GetCounter().GetValue()
}

type PrometheusGauge struct {
	logger types.Logger
	gauge  *prometheus.GaugeVec
	labels map[string]string
}

func (g *PrometheusGauge) Set(value float64) {
	g.gauge.With(g.labels).Set(value)
}

func (g *PrometheusGauge) Inc() {
	g.gauge.With(g.labels).Inc()
}

func (g *PrometheusGauge) Dec() {
	g.gauge.With(g.labels).Dec()
}

func (g *PrometheusGauge) Add(value float64) {
	g.gauge.With(g.labels).Add(value)
}

func (g *PrometheusGauge) Sub(value float64) {
	g.gauge.With(g.labels).Sub(value)
}

func (g *PrometheusGauge) Get() float64 {
	metric := &dto.Metric{}
	err := g.gauge.With(g.labels).Write(metric)
	if err != nil {
		g.logger.Error("Failed to write counter", zap.Error(err))
	}
	return metric.GetGauge().GetValue()
}

type PrometheusHistogram struct {
	histogram *prometheus.HistogramVec
	labels    map[string]string
}

func (h *PrometheusHistogram) Observe(value float64) {
	h.histogram.With(h.labels).Observe(value)
}

func (h *PrometheusHistogram) ObserveDuration(start time.Time) {
	duration := time.Since(start).Seconds()
	h.histogram.With(h.labels).Observe(duration)
}

func (h *PrometheusHistogram) GetCount() uint64 {
	metric := &dto.Metric{}
	observer := h.histogram.With(h.labels)

	if promMetric, ok := observer.(prometheus.Metric); ok {
		if err := promMetric.Write(metric); err != nil {
			return 0
		}

		if histogram := metric.GetHistogram(); histogram != nil {
			return histogram.GetSampleCount()
		}
	}

	return 0
}

func (h *PrometheusHistogram) GetSum() float64 {
	metric := &dto.Metric{}
	observer := h.histogram.With(h.labels)

	if promMetric, ok := observer.(prometheus.Metric); ok {
		if err := promMetric.Write(metric); err != nil {
			return 0
		}

		if histogram := metric.GetHistogram(); histogram != nil {
			return histogram.GetSampleSum()
		}
	}

	return 0
}

type PrometheusSummary struct {
	summary *prometheus.SummaryVec
	labels  map[string]string
}

func (s *PrometheusSummary) Observe(value float64) {
	s.summary.With(s.labels).Observe(value)
}

func (s *PrometheusSummary) ObserveDuration(start time.Time) {
	duration := time.Since(start).Seconds()
	s.summary.With(s.labels).Observe(duration)
}

func (s *PrometheusSummary) GetCount() uint64 {
	metric := &dto.Metric{}
	observer := s.summary.With(s.labels)

	if promMetric, ok := observer.(prometheus.Metric); ok {
		if err := promMetric.Write(metric); err != nil {
			return 0
		}

		if summary := metric.GetSummary(); summary != nil {
			return summary.GetSampleCount()
		}
	}

	return 0
}

func (s *PrometheusSummary) GetSum() float64 {
	metric := &dto.Metric{}
	observer := s.summary.With(s.labels)

	if promMetric, ok := observer.(prometheus.Metric); ok {
		if err := promMetric.Write(metric); err != nil {
			return 0
		}

		if summary := metric.GetSummary(); summary != nil {
			return summary.GetSampleSum()
		}
	}

	return 0
}

func (h *PrometheusHistogram) GetBuckets() map[float64]uint64 {
	metric := &dto.Metric{}
	observer := h.histogram.With(h.labels)

	if promMetric, ok := observer.(prometheus.Metric); ok {
		if err := promMetric.Write(metric); err != nil {
			return nil
		}

		histogram := metric.GetHistogram()
		if histogram == nil {
			return nil
		}

		buckets := make(map[float64]uint64)
		for _, bucket := range histogram.GetBucket() {
			buckets[bucket.GetUpperBound()] = bucket.GetCumulativeCount()
		}

		return buckets
	}

	return nil
}

func (s *PrometheusSummary) GetQuantiles() map[float64]float64 {
	metric := &dto.Metric{}
	observer := s.summary.With(s.labels)

	if promMetric, ok := observer.(prometheus.Metric); ok {
		if err := promMetric.Write(metric); err != nil {
			return nil
		}

		summary := metric.GetSummary()
		if summary == nil {
			return nil
		}

		quantiles := make(map[float64]float64)
		for _, quantile := range summary.GetQuantile() {
			quantiles[quantile.GetQuantile()] = quantile.GetValue()
		}

		return quantiles
	}

	return nil
}
