package metrics

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
)

type ManagerState int32

const (
	ManagerStateStopped ManagerState = iota
	ManagerStateStarting
	ManagerStateRunning
	ManagerStateStopping
)

type Manager struct {
	ctx             context.Context
	cancel          context.CancelFunc
	logger          types.Logger
	manager         types.MetricsManager
	state           atomic.Value
	shutdownTimeout time.Duration
}

var customMetricsCreators = sync.Map{}

func RegisterMetricsManager(metricsManagerName string, creator types.MetricsManagerCreator) {
	customMetricsCreators.Store(metricsManagerName, creator)
}

func NewManager(ctx context.Context, config types.ConfigManager, logger types.Logger, router types.HTTPRouter, health types.HealthManager) (types.MetricsManager, error) {
	metricsConfig := config.GetConfig().Metrics

	if !metricsConfig.Enabled {
		return nil, types.ErrMetricsIsDisabled
	}

	managerCtx, cancel := context.WithCancel(ctx)

	wrapper := &Manager{
		ctx:             managerCtx,
		cancel:          cancel,
		logger:          logger,
		shutdownTimeout: 10 * time.Second,
	}

	wrapper.state.Store(ManagerStateStopped)

	if err := wrapper.initializeManager(metricsConfig, router, health); err != nil {
		cancel()
		return nil, types.WrapError(err, "failed to initialize metrics manager")
	}

	return wrapper, nil
}

func (w *Manager) initializeManager(metricsConfig *types.MetricsConfig, router types.HTTPRouter, health types.HealthManager) error {
	metricsManagerName := metricsConfig.Type

	var manager types.MetricsManager
	var err error

	switch metricsManagerName {
	case "memory":
		manager, err = NewMemoryMetrics(w.ctx, w.logger, metricsConfig, router, health)
	case "prometheus":
		manager, err = NewPrometheusMetrics(w.ctx, w.logger, metricsConfig, router, health)
	default:
		if creator, exists := customMetricsCreators.Load(metricsManagerName); exists {
			manager, err = creator.(types.MetricsManagerCreator)(metricsConfig)
		} else {
			return types.Errorf(types.ErrMetricsTypeUnknown, "type: %s", metricsManagerName)
		}
	}

	if err != nil {
		return err
	}

	w.manager = manager
	w.logger.Info("Metrics manager initialized", zap.String("type", metricsManagerName))
	return nil
}

func (w *Manager) Start() error {
	if !w.transitionState(ManagerStateStopped, ManagerStateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if w.getState() == ManagerStateStarting {
			w.setState(ManagerStateRunning)
		}
	}()

	ctx, cancel := context.WithTimeout(w.ctx, 30*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	if w.manager != nil {
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				return w.manager.Start()
			}
		})
	}

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			w.setState(ManagerStateStopped)
			return types.NewErrorf("metrics manager start timeout")
		default:
			w.setState(ManagerStateStopped)
			return types.WrapError(err, "failed to start metrics manager")
		}
	}

	w.logger.Info("Metrics manager started successfully")
	return nil
}

func (w *Manager) Stop() error {
	if !w.transitionState(ManagerStateRunning, ManagerStateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		w.setState(ManagerStateStopped)
		w.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), w.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	if w.manager != nil {
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				return w.manager.Stop()
			}
		})
	}

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			w.logger.Warn("Metrics manager stop timeout, some components may not have stopped gracefully")
		default:
			w.logger.Error("Error during metrics manager shutdown", zap.Error(err))
		}
	} else {
		w.logger.Info("Metrics manager stopped gracefully")
	}

	return nil
}

func (w *Manager) IsRunning() bool {
	return w.getState() == ManagerStateRunning
}

func (w *Manager) getState() ManagerState {
	return w.state.Load().(ManagerState)
}

func (w *Manager) setState(newState ManagerState) bool {
	currentState := w.getState()
	return w.state.CompareAndSwap(currentState, newState)
}

func (w *Manager) transitionState(from, to ManagerState) bool {
	return w.state.CompareAndSwap(from, to)
}

func (w *Manager) Counter(name string, labels map[string]string) types.Counter {
	if w.manager != nil && w.IsRunning() {
		return w.manager.Counter(name, labels)
	}
	return &emptyCounter{}
}

func (w *Manager) Gauge(name string, labels map[string]string) types.Gauge {
	if w.manager != nil && w.IsRunning() {
		return w.manager.Gauge(name, labels)
	}
	return &emptyGauge{}
}

func (w *Manager) Histogram(name string, buckets []float64, labels map[string]string) types.Histogram {
	if w.manager != nil && w.IsRunning() {
		return w.manager.Histogram(name, buckets, labels)
	}
	return &emptyHistogram{}
}

func (w *Manager) Summary(name string, objectives map[float64]float64, labels map[string]string) types.Summary {
	if w.manager != nil && w.IsRunning() {
		return w.manager.Summary(name, objectives, labels)
	}
	return &emptySummary{}
}

func (w *Manager) RegisterSystemMetrics() error {
	if w.manager != nil && w.IsRunning() {
		return w.manager.RegisterSystemMetrics()
	}
	return types.ErrMetricsNotRunning
}

func (w *Manager) StartSystemCollection() error {
	if w.manager != nil && w.IsRunning() {
		return w.manager.StartSystemCollection()
	}
	return types.ErrMetricsNotRunning
}

func (w *Manager) StopSystemCollection() error {
	if w.manager != nil {
		return w.manager.StopSystemCollection()
	}
	return nil
}

func (w *Manager) GetMetrics() ([]byte, error) {
	if w.manager != nil && w.IsRunning() {
		return w.manager.GetMetrics()
	}
	return nil, types.ErrMetricsNotRunning
}

func (w *Manager) GetStats() ([]byte, error) {
	if w.manager != nil && w.IsRunning() {
		return w.manager.GetStats()
	}
	return nil, types.ErrMetricsNotRunning
}

func (w *Manager) Close() error {
	return w.Stop()
}

type emptyCounter struct{}

func (c *emptyCounter) Inc()          {}
func (c *emptyCounter) Add(_ float64) {}
func (c *emptyCounter) Get() float64  { return 0 }

type emptyGauge struct{}

func (g *emptyGauge) Set(_ float64) {}
func (g *emptyGauge) Inc()          {}
func (g *emptyGauge) Dec()          {}
func (g *emptyGauge) Add(_ float64) {}
func (g *emptyGauge) Sub(_ float64) {}
func (g *emptyGauge) Get() float64  { return 0 }

type emptyHistogram struct{}

func (h *emptyHistogram) Observe(_ float64)              {}
func (h *emptyHistogram) ObserveDuration(_ time.Time)    {}
func (h *emptyHistogram) GetCount() uint64               { return 0 }
func (h *emptyHistogram) GetSum() float64                { return 0 }
func (h *emptyHistogram) GetBuckets() map[float64]uint64 { return nil }

type emptySummary struct{}

func (s *emptySummary) Observe(_ float64)                 {}
func (s *emptySummary) ObserveDuration(_ time.Time)       {}
func (s *emptySummary) GetCount() uint64                  { return 0 }
func (s *emptySummary) GetSum() float64                   { return 0 }
func (s *emptySummary) GetQuantiles() map[float64]float64 { return nil }
