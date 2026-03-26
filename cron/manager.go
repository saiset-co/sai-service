package cron

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
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
	metrics         types.MetricsManager
	health          types.HealthManager
	cron            *cron.Cron
	timezone        *time.Location
	jobs            map[string]*types.JobEntry
	state           atomic.Value
	mu              sync.RWMutex
	activeJobs      map[string]context.CancelFunc
	activeJobsMu    sync.RWMutex
	shutdown        chan struct{}
	shutdownOnce    sync.Once
	shutdownTimeout time.Duration
	jobTimeout      time.Duration
}

func NewManager(ctx context.Context, config types.ConfigManager, logger types.Logger, metrics types.MetricsManager, health types.HealthManager) (types.CronManager, error) {
	timezoneStr := config.GetConfig().Cron.Timezone
	timezone, err := time.LoadLocation(timezoneStr)
	if err != nil {
		timezone = time.UTC
	}

	cronL := safeCronLogger{
		logger: logger,
	}

	cronOptions := []cron.Option{
		cron.WithLocation(timezone),
		cron.WithSeconds(),
		cron.WithChain(cron.Recover(cronL)),
	}

	managerCtx, cancel := context.WithCancel(ctx)

	manager := &Manager{
		ctx:             managerCtx,
		cancel:          cancel,
		config:          config,
		logger:          logger,
		metrics:         metrics,
		health:          health,
		cron:            cron.New(cronOptions...),
		jobs:            make(map[string]*types.JobEntry),
		timezone:        timezone,
		activeJobs:      make(map[string]context.CancelFunc),
		shutdown:        make(chan struct{}),
		shutdownTimeout: 10 * time.Second,
		jobTimeout:      30 * time.Minute,
	}

	manager.state.Store(StateStopped)

	return manager, nil
}

func (m *Manager) Add(jobName, spec string, job func()) error {
	if jobName == "" {
		return types.ErrCronJobNameIsEmpty
	}

	if spec == "" {
		return types.ErrCronExpressionInvalid
	}

	if job == nil {
		return types.ErrCronJobIsNil
	}

	wrappedJob := m.wrapJob(jobName, job)

	return m.addJob(jobName, spec, wrappedJob)
}

func (m *Manager) Start() error {
	if !m.transitionState(StateStopped, StateStarting) {
		return types.ErrCronIsRunning
	}

	defer func() {
		if m.getState() == StateStarting {
			m.setState(StateRunning)
		}
	}()

	if err := m.start(); err != nil {
		m.setState(StateStopped)
		return err
	}

	m.setSchedulerStatus(1)
	m.logger.Info("Cron manager started")
	return nil
}

func (m *Manager) Stop() error {
	if !m.transitionState(StateRunning, StateStopping) &&
		!m.transitionState(StateStarting, StateStopping) {
		return types.ErrServerNotRunning
	}

	var err error
	m.shutdownOnce.Do(func() {
		defer func() {
			m.setState(StateStopped)
			m.cancel()
		}()

		err = m.stop()
		m.setSchedulerStatus(0)
		m.setActiveJobsGauge(0)

		if err == nil {
			m.logger.Info("Cron scheduler stopped gracefully")
		}
		close(m.shutdown)
	})

	return err
}

func (m *Manager) IsRunning() bool {
	state := m.getState()
	return state == StateRunning
}

func (m *Manager) getState() State {
	return m.state.Load().(State)
}

func (m *Manager) setState(newState State) bool {
	currentState := m.getState()
	return m.state.CompareAndSwap(currentState, newState)
}

func (m *Manager) transitionState(from, to State) bool {
	return m.state.CompareAndSwap(from, to)
}

func (m *Manager) wrapJob(jobName string, job func()) func() {
	return func() {
		defer func() {
			if r := recover(); r != nil {
				m.logger.Error("Critical panic in cron job wrapper",
					zap.String("job_name", jobName),
					zap.Any("panic", r))
			}
		}()

		select {
		case <-m.shutdown:
			m.logger.Info("Job skipped due to shutdown", zap.String("job_name", jobName))
			return
		default:
		}

		startTime := time.Now()

		m.logger.Debug("Cron job started", zap.String("job_name", jobName))

		m.safeUpdateJobStatsStart(jobName, startTime)

		jobCtx, cancel := context.WithTimeout(m.ctx, m.jobTimeout)
		defer cancel()

		if !m.registerActiveJob(jobName, cancel) {
			m.logger.Info("Job cancelled due to manager shutdown", zap.String("job_name", jobName))
			return
		}
		defer m.cancelActiveJob(jobName)

		m.incActiveJobsGauge()
		defer m.decActiveJobsGauge()

		var err error
		done := make(chan struct{})
		var jobFinished int32

		go func() {
			defer func() {
				if r := recover(); r != nil {
					err = types.Errorf(types.ErrCronJobFailed, "job panic: %v", r)
					m.logger.Error("Job panicked",
						zap.String("job_name", jobName),
						zap.Any("panic", r))
				}
				atomic.StoreInt32(&jobFinished, 1)
				close(done)
			}()

			func() {
				defer func() {
					if r := recover(); r != nil {
						err = types.Errorf(types.ErrCronJobFailed, "job execution panic: %v", r)
					}
				}()
				job()
			}()
		}()

		select {
		case <-done:
		case <-jobCtx.Done():
			if types.IsError(jobCtx.Err(), context.DeadlineExceeded) {
				err = types.Errorf(types.ErrCronJobTimeout, "timeout after %v", m.jobTimeout)
			} else {
				err = types.WrapError(jobCtx.Err(), "job canceled")
			}

			m.logger.Error("Cron job interrupted",
				zap.String("job_name", jobName),
				zap.Error(err))

			gracefulShutdownTimer := time.NewTimer(5 * time.Second)
			select {
			case <-done:
				gracefulShutdownTimer.Stop()
			case <-gracefulShutdownTimer.C:
				if atomic.LoadInt32(&jobFinished) == 0 {
					m.logger.Warn("Job goroutine did not finish gracefully",
						zap.String("job_name", jobName))
				}
			}
		}

		duration := time.Since(startTime)

		result := "success"
		if err != nil {
			result = "error"
			m.incJobErrorsCounter(jobName)
		}

		m.incJobExecutionsCounter(jobName, result)
		m.observeJobDuration(jobName, duration.Seconds())
		m.safeUpdateJobStatsFinish(jobName, duration, err)

		if err != nil {
			m.logger.Error("Cron job failed",
				zap.String("job_name", jobName),
				zap.Duration("duration", duration),
				zap.Error(err))
		} else {
			m.logger.Info("Cron job completed",
				zap.String("job_name", jobName),
				zap.Duration("duration", duration))
		}
	}
}

func (m *Manager) addJob(jobName, spec string, job func()) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	select {
	case <-m.shutdown:
		return types.ErrCronSchedulerStopped
	default:
	}

	if _, exists := m.jobs[jobName]; exists {
		return types.ErrCronJobExists
	}

	entryID, err := m.cron.AddFunc(spec, job)
	if err != nil {
		return types.WrapError(err, "failed to add cron job")
	}

	entry := &types.JobEntry{
		ID:            entryID,
		Name:          jobName,
		Spec:          spec,
		Job:           job,
		AddedAt:       time.Now(),
		LastDuration:  0,
		TotalDuration: 0,
		AvgDuration:   0,
	}

	if cronEntry := m.cron.Entry(entryID); cronEntry.ID != 0 {
		entry.NextRun = cronEntry.Next
	}

	m.jobs[jobName] = entry

	m.logger.Info("Cron job added",
		zap.String("job_name", jobName),
		zap.String("spec", spec))

	return nil
}

func (m *Manager) start() error {
	m.cron.Start()
	return nil
}

func (m *Manager) stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), m.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		m.activeJobsMu.Lock()
		activeJobs := make(map[string]context.CancelFunc, len(m.activeJobs))
		for jobName, cancel := range m.activeJobs {
			activeJobs[jobName] = cancel
		}

		m.activeJobs = make(map[string]context.CancelFunc)
		m.activeJobsMu.Unlock()

		for jobName, cancel := range activeJobs {
			cancel()
			m.logger.Debug("Cancelled job during shutdown", zap.String("job_name", jobName))
		}
		return nil
	})

	g.Go(func() error {
		stopCtx := m.cron.Stop()

		select {
		case <-stopCtx.Done():
			return nil
		case <-gCtx.Done():
			return types.ErrCronJobTimeout
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			m.logger.Warn("Cron manager stop timeout, some components may not have stopped gracefully")
		default:
			m.logger.Error("Error during cron manager shutdown", zap.Error(err))
		}
		return err
	}

	return nil
}

func (m *Manager) registerActiveJob(jobName string, cancel context.CancelFunc) bool {
	m.activeJobsMu.Lock()
	defer m.activeJobsMu.Unlock()

	select {
	case <-m.shutdown:
		return false
	default:
	}

	if oldCancel, exists := m.activeJobs[jobName]; exists {
		oldCancel()
	}

	m.activeJobs[jobName] = cancel
	return true
}

func (m *Manager) cancelActiveJob(jobName string) {
	m.activeJobsMu.Lock()
	defer m.activeJobsMu.Unlock()

	if cancel, exists := m.activeJobs[jobName]; exists {
		cancel()
		delete(m.activeJobs, jobName)
		m.logger.Debug("Cancelled active job", zap.String("job_name", jobName))
	}
}

func (m *Manager) safeUpdateJobStatsStart(jobName string, startTime time.Time) {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("Panic in updateJobStatsStart",
				zap.String("job_name", jobName),
				zap.Any("panic", r))
		}
	}()

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.jobs[jobName]
	if !exists {
		m.logger.Warn("Job entry not found during stats update",
			zap.String("job_name", jobName))
		return
	}

	entry.LastRun = startTime
	entry.Error = nil

	func() {
		defer func() {
			if r := recover(); r != nil {
				m.logger.Debug("Failed to get cron entry",
					zap.String("job_name", jobName),
					zap.Any("panic", r))
			}
		}()

		if cronEntry := m.cron.Entry(entry.ID); cronEntry.ID != 0 {
			entry.NextRun = cronEntry.Next
		}
	}()
}

func (m *Manager) safeUpdateJobStatsFinish(jobName string, duration time.Duration, err error) {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("Panic in updateJobStatsFinish",
				zap.String("job_name", jobName),
				zap.Any("panic", r))
		}
	}()

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.jobs[jobName]
	if !exists {
		m.logger.Warn("Job entry not found during stats finish",
			zap.String("job_name", jobName))
		return
	}

	entry.LastDuration = duration
	entry.TotalDuration += duration
	entry.RunCount++
	entry.Error = err

	if entry.RunCount > 0 {
		entry.AvgDuration = entry.TotalDuration / time.Duration(entry.RunCount)
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				m.logger.Debug("Failed to get cron entry in finish",
					zap.String("job_name", jobName),
					zap.Any("panic", r))
			}
		}()

		if cronEntry := m.cron.Entry(entry.ID); cronEntry.ID != 0 {
			entry.NextRun = cronEntry.Next
		}
	}()

	m.logger.Debug("Job performance updated",
		zap.String("job_name", jobName),
		zap.Duration("last_duration", entry.LastDuration),
		zap.Duration("avg_duration", entry.AvgDuration),
		zap.Int64("run_count", entry.RunCount))
}

func (m *Manager) incJobExecutionsCounter(jobName, result string) {
	if m.metrics == nil {
		return
	}

	counter := m.metrics.Counter("cron_job_executions_total", map[string]string{
		"job_name": jobName,
		"result":   result,
	})
	counter.Inc()
}

func (m *Manager) incJobErrorsCounter(jobName string) {
	if m.metrics == nil {
		return
	}

	counter := m.metrics.Counter("cron_job_errors_total", map[string]string{
		"job_name": jobName,
	})
	counter.Inc()
}

func (m *Manager) observeJobDuration(jobName string, seconds float64) {
	if m.metrics == nil {
		return
	}

	histogram := m.metrics.Histogram("cron_job_duration_seconds",
		[]float64{0.1, 1.0, 10.0, 60.0, 300.0, 1800.0},
		map[string]string{"job_name": jobName},
	)
	histogram.Observe(seconds)
}

func (m *Manager) incActiveJobsGauge() {
	if m.metrics == nil {
		return
	}

	m.metrics.Gauge("cron_active_jobs", nil).Inc()
}

func (m *Manager) decActiveJobsGauge() {
	if m.metrics == nil {
		return
	}

	m.metrics.Gauge("cron_active_jobs", nil).Dec()
}

func (m *Manager) setActiveJobsGauge(value float64) {
	if m.metrics == nil {
		return
	}
	m.metrics.Gauge("cron_active_jobs", nil).Set(value)
}

func (m *Manager) setSchedulerStatus(value float64) {
	if m.metrics == nil {
		return
	}
	m.metrics.Gauge("cron_scheduler_running", nil).Set(value)
}

type safeCronLogger struct {
	logger types.Logger
}

func (l safeCronLogger) Info(msg string, keysAndValues ...interface{}) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("CRON LOGGER PANIC in Info: %v\n", r)
		}
	}()

	fields := make([]zap.Field, 0, len(keysAndValues)/2)

	for i := 0; i < len(keysAndValues)-1; i += 2 {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fields = append(fields, zap.String("field_error", "panic in field conversion"))
				}
			}()

			key := fmt.Sprintf("%v", keysAndValues[i])
			value := keysAndValues[i+1]
			fields = append(fields, zap.Any(key, value))
		}()
	}

	l.logger.Info(msg, fields...)
}

func (l safeCronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("CRON LOGGER PANIC in Error: %v\n", r)
		}
	}()

	fields := make([]zap.Field, 0, len(keysAndValues)/2)

	for i := 0; i < len(keysAndValues)-1; i += 2 {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fields = append(fields, zap.String("field_error", "panic in field conversion"))
				}
			}()

			key := fmt.Sprintf("%v", keysAndValues[i])
			value := keysAndValues[i+1]
			fields = append(fields, zap.Any(key, value))
		}()
	}

	fields = append(fields, zap.Error(err))
	l.logger.Error(msg, fields...)
}
