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
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type PrometheusState int32

const (
	PrometheusStateStopped PrometheusState = iota
	PrometheusStateStarting
	PrometheusStateRunning
	PrometheusStateStopping
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
	ctx             context.Context
	cancel          context.CancelFunc
	logger          types.Logger
	router          types.HTTPRouter
	health          types.HealthManager
	config          *PrometheusConfig
	registry        *prometheus.Registry
	counters        map[string]*prometheus.CounterVec
	gauges          map[string]*prometheus.GaugeVec
	histograms      map[string]*prometheus.HistogramVec
	summaries       map[string]*prometheus.SummaryVec
	systemMetrics   atomic.Pointer[*SystemMetricsCollector]
	state           atomic.Value
	shutdownTimeout time.Duration
	mu              sync.RWMutex
}

func NewPrometheusMetrics(ctx context.Context, logger types.Logger, config *types.MetricsConfig, router types.HTTPRouter, health types.HealthManager) (types.MetricsManager, error) {
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

	promCtx, cancel := context.WithCancel(ctx)

	registry := prometheus.NewRegistry()
	if promConfig.EnableGoMetrics {
		registry.MustRegister(collectors.NewGoCollector())
		registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}

	metrics := &PrometheusMetrics{
		ctx:             promCtx,
		cancel:          cancel,
		logger:          logger,
		router:          router,
		health:          health,
		config:          promConfig,
		registry:        registry,
		counters:        make(map[string]*prometheus.CounterVec),
		gauges:          make(map[string]*prometheus.GaugeVec),
		histograms:      make(map[string]*prometheus.HistogramVec),
		summaries:       make(map[string]*prometheus.SummaryVec),
		shutdownTimeout: 10 * time.Second,
	}

	metrics.state.Store(PrometheusStateStopped)

	logger.Info("Prometheus metrics initialized",
		zap.String("namespace", promConfig.Namespace),
		zap.String("subsystem", promConfig.Subsystem),
		zap.Bool("go_metrics", promConfig.EnableGoMetrics))

	return metrics, nil
}

func (p *PrometheusMetrics) Start() error {
	if !p.transitionState(PrometheusStateStopped, PrometheusStateStarting) {
		p.logger.Warn("Prometheus metrics is already running")
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if p.getState() == PrometheusStateStarting {
			p.setState(PrometheusStateRunning)
		}
	}()

	ctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			p.registerRoutes()
			return nil
		}
	})

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			if err := p.RegisterSystemMetrics(); err != nil {
				p.logger.Warn("Failed to register system metrics", zap.Error(err))
			}
			if err := p.StartSystemCollection(); err != nil {
				p.logger.Warn("Failed to start system collection", zap.Error(err))
			}
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			p.setState(PrometheusStateStopped)
			return types.NewErrorf("prometheus metrics start timeout")
		default:
			p.setState(PrometheusStateStopped)
			return types.WrapError(err, "failed to start prometheus metrics")
		}
	}

	p.logger.Info("prometheus metrics started")
	return nil
}

func (p *PrometheusMetrics) Stop() error {
	if !p.transitionState(PrometheusStateRunning, PrometheusStateStopping) {
		p.logger.Warn("Prometheus metrics is not running")
		return types.ErrServerNotRunning
	}

	defer func() {
		p.setState(PrometheusStateStopped)
		p.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), p.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	if collector := p.systemMetrics.Load(); collector != nil {
		g.Go(func() error {
			if err := (*collector).Stop(); err != nil {
				p.logger.Error("Failed to stop system collection", zap.Error(err))
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
			return p.cleanup()
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			p.logger.Warn("Prometheus metrics stop timeout, some components may not have stopped gracefully")
		default:
			p.logger.Error("Error during prometheus metrics shutdown", zap.Error(err))
		}
	} else {
		p.logger.Info("prometheus metrics stopped gracefully")
	}

	p.systemMetrics.Store(nil)
	return nil
}

func (p *PrometheusMetrics) IsRunning() bool {
	return p.getState() == PrometheusStateRunning
}

func (p *PrometheusMetrics) getState() PrometheusState {
	return p.state.Load().(PrometheusState)
}

func (p *PrometheusMetrics) setState(newState PrometheusState) bool {
	currentState := p.getState()
	return p.state.CompareAndSwap(currentState, newState)
}

func (p *PrometheusMetrics) transitionState(from, to PrometheusState) bool {
	return p.state.CompareAndSwap(from, to)
}

func (p *PrometheusMetrics) cleanup() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.counters = make(map[string]*prometheus.CounterVec)
	p.gauges = make(map[string]*prometheus.GaugeVec)
	p.histograms = make(map[string]*prometheus.HistogramVec)
	p.summaries = make(map[string]*prometheus.SummaryVec)

	p.logger.Info("Prometheus metrics cleaned up")
	return nil
}

func (p *PrometheusMetrics) Counter(name string, labels map[string]string) types.Counter {
	if !p.IsRunning() {
		return &PrometheusCounter{logger: p.logger}
	}

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
	if !p.IsRunning() {
		return &PrometheusGauge{logger: p.logger}
	}

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
	if !p.IsRunning() {
		return &PrometheusHistogram{}
	}

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
	if !p.IsRunning() {
		return &PrometheusSummary{}
	}

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
	state := p.getState()
	if state != PrometheusStateRunning && state != PrometheusStateStarting {
		return types.ErrMetricsNotRunning
	}

	ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
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
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		return types.WrapError(err, "failed to register system metrics")
	}

	p.logger.Info("Prometheus system metrics registered")
	return nil
}

func (p *PrometheusMetrics) StartSystemCollection() error {
	state := p.getState()
	if state != PrometheusStateRunning && state != PrometheusStateStarting {
		return types.ErrMetricsNotRunning
	}

	if p.systemMetrics.Load() == nil {
		systemMetrics := NewSystemMetricsCollector(p.ctx, p.logger, p)
		p.systemMetrics.Store(&systemMetrics)
	}

	if collector := p.systemMetrics.Load(); collector != nil {
		return (*collector).Start()
	}

	return nil
}

func (p *PrometheusMetrics) StopSystemCollection() error {
	if collector := p.systemMetrics.Load(); collector != nil {
		return (*collector).Stop()
	}
	return nil
}

func (p *PrometheusMetrics) GetMetrics() ([]byte, error) {
	if !p.IsRunning() {
		return nil, types.ErrMetricsNotRunning
	}

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
	if !p.IsRunning() {
		return nil, types.ErrMetricsNotRunning
	}

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

func (p *PrometheusMetrics) registerRoutes() {
	config := &types.RouteConfig{
		Cache: &types.CacheHandlerConfig{
			Enabled: false,
		},
		Timeout:             time.Duration(5) * time.Second,
		DisabledMiddlewares: []string{"cache"},
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

	fastHandler := func(ctx *types.RequestCtx) {
		ctx.SetContentType("text/plain; version=0.0.4; charset=utf-8")
		req, _ := http.NewRequest("GET", string(ctx.RequestURI()), nil)
		w := types.NewFastResponseWriter(ctx)
		promHandler := promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{})
		promHandler.ServeHTTP(w, req)
	}

	p.router.Add("GET", p.config.Path, fastHandler, config)
}

func (p *PrometheusMetrics) Close() error {
	return p.Stop()
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
	if c.counter != nil {
		c.counter.With(c.labels).Inc()
	}
}

func (c *PrometheusCounter) Add(value float64) {
	if c.counter != nil {
		c.counter.With(c.labels).Add(value)
	}
}

func (c *PrometheusCounter) Get() float64 {
	if c.counter == nil {
		return 0
	}

	metric := &dto.Metric{}
	err := c.counter.With(c.labels).Write(metric)
	if err != nil {
		if c.logger != nil {
			c.logger.Error("Failed to write counter", zap.Error(err))
		}
		return 0
	}
	return metric.GetCounter().GetValue()
}

type PrometheusGauge struct {
	logger types.Logger
	gauge  *prometheus.GaugeVec
	labels map[string]string
}

func (g *PrometheusGauge) Set(value float64) {
	if g.gauge != nil {
		g.gauge.With(g.labels).Set(value)
	}
}

func (g *PrometheusGauge) Inc() {
	if g.gauge != nil {
		g.gauge.With(g.labels).Inc()
	}
}

func (g *PrometheusGauge) Dec() {
	if g.gauge != nil {
		g.gauge.With(g.labels).Dec()
	}
}

func (g *PrometheusGauge) Add(value float64) {
	if g.gauge != nil {
		g.gauge.With(g.labels).Add(value)
	}
}

func (g *PrometheusGauge) Sub(value float64) {
	if g.gauge != nil {
		g.gauge.With(g.labels).Sub(value)
	}
}

func (g *PrometheusGauge) Get() float64 {
	if g.gauge == nil {
		return 0
	}

	metric := &dto.Metric{}
	err := g.gauge.With(g.labels).Write(metric)
	if err != nil {
		if g.logger != nil {
			g.logger.Error("Failed to write gauge", zap.Error(err))
		}
		return 0
	}
	return metric.GetGauge().GetValue()
}

type PrometheusHistogram struct {
	histogram *prometheus.HistogramVec
	labels    map[string]string
}

func (h *PrometheusHistogram) Observe(value float64) {
	if h.histogram != nil {
		h.histogram.With(h.labels).Observe(value)
	}
}

func (h *PrometheusHistogram) ObserveDuration(start time.Time) {
	if h.histogram != nil {
		duration := time.Since(start).Seconds()
		h.histogram.With(h.labels).Observe(duration)
	}
}

func (h *PrometheusHistogram) GetCount() uint64 {
	if h.histogram == nil {
		return 0
	}

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
	if h.histogram == nil {
		return 0
	}

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

func (h *PrometheusHistogram) GetBuckets() map[float64]uint64 {
	if h.histogram == nil {
		return nil
	}

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

type PrometheusSummary struct {
	summary *prometheus.SummaryVec
	labels  map[string]string
}

func (s *PrometheusSummary) Observe(value float64) {
	if s.summary != nil {
		s.summary.With(s.labels).Observe(value)
	}
}

func (s *PrometheusSummary) ObserveDuration(start time.Time) {
	if s.summary != nil {
		duration := time.Since(start).Seconds()
		s.summary.With(s.labels).Observe(duration)
	}
}

func (s *PrometheusSummary) GetCount() uint64 {
	if s.summary == nil {
		return 0
	}

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
	if s.summary == nil {
		return 0
	}

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

func (s *PrometheusSummary) GetQuantiles() map[float64]float64 {
	if s.summary == nil {
		return nil
	}

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
