package types

import (
	"time"
)

type MetricsManager interface {
	LifecycleManager
	RegisterRoutes(router HTTPRouter)
	Counter(name string, labels map[string]string) Counter
	Gauge(name string, labels map[string]string) Gauge
	Histogram(name string, buckets []float64, labels map[string]string) Histogram
	Summary(name string, objectives map[float64]float64, labels map[string]string) Summary
	RegisterSystemMetrics() error
	StartSystemCollection() error
	StopSystemCollection() error
	GetMetrics() ([]byte, error)
	GetStats() ([]byte, error)
}

type Counter interface {
	Inc()
	Add(value float64)
	Get() float64
}

type Gauge interface {
	Set(value float64)
	Inc()
	Dec()
	Add(value float64)
	Sub(value float64)
	Get() float64
}

type Histogram interface {
	Observe(value float64)
	ObserveDuration(start time.Time)
	GetCount() uint64
	GetSum() float64
}

type Summary interface {
	Observe(value float64)
	ObserveDuration(start time.Time)
	GetCount() uint64
	GetSum() float64
}

type MetricsManagerCreator func(config interface{}) (MetricsManager, error)

type MetricsStats struct {
	TotalMetrics     int                    `json:"total_metrics"`
	CounterMetrics   int                    `json:"counter_metrics"`
	GaugeMetrics     int                    `json:"gauge_metrics"`
	HistogramMetrics int                    `json:"histogram_metrics"`
	SummaryMetrics   int                    `json:"summary_metrics"`
	Labels           map[string]int         `json:"labels"`
	LastUpdate       time.Time              `json:"last_update"`
	MemoryUsage      int64                  `json:"memory_usage"`
	Collections      uint64                 `json:"collections"`
	Errors           uint64                 `json:"errors"`
	Details          map[string]interface{} `json:"details"`
}

type MetricValue struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels"`
	Timestamp time.Time         `json:"timestamp"`
	Help      string            `json:"help"`
}
