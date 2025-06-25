package client

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
)

type CircuitBreakerConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	FailureThreshold int           `yaml:"failure_threshold" json:"failure_threshold"`
	RecoveryTimeout  time.Duration `yaml:"recovery_timeout" json:"recovery_timeout"`
	HalfOpenRequests int           `yaml:"half_open_requests" json:"half_open_requests"`
}

type CircuitBreakerState int32

const (
	StateBreakerClosed CircuitBreakerState = iota
	StateBreakerOpen
	StateBreakerHalfOpen
	StateBreakerStopped
)

type CircuitBreaker struct {
	ctx             context.Context
	cancel          context.CancelFunc
	config          *CircuitBreakerConfig
	logger          types.Logger
	serviceName     string
	state           atomic.Value
	failures        atomic.Int32
	successes       atomic.Int32
	lastFail        atomic.Int64
	mutex           sync.RWMutex
	shutdownTimeout time.Duration
	monitorTicker   *time.Ticker
}

func NewCircuitBreaker(config *CircuitBreakerConfig, logger types.Logger, serviceName string) *CircuitBreaker {
	if config == nil || !config.Enabled {
		cb := &CircuitBreaker{
			config:      &CircuitBreakerConfig{Enabled: false},
			logger:      logger,
			serviceName: serviceName,
		}
		cb.state.Store(StateBreakerStopped)
		return cb
	}

	ctx, cancel := context.WithCancel(context.Background())

	cb := &CircuitBreaker{
		ctx:             ctx,
		cancel:          cancel,
		config:          config,
		logger:          logger,
		serviceName:     serviceName,
		shutdownTimeout: 5 * time.Second,
		monitorTicker:   time.NewTicker(time.Minute),
	}

	cb.state.Store(StateBreakerClosed)

	go cb.monitorLoop()

	return cb
}

func (cb *CircuitBreaker) CanExecute() bool {
	if cb == nil || !cb.config.Enabled {
		return true
	}

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	state := cb.getStateUnsafe()

	switch state {
	case StateBreakerClosed:
		return true
	case StateBreakerOpen:
		if time.Since(time.Unix(cb.lastFail.Load(), 0)) > cb.config.RecoveryTimeout {
			cb.transitionToHalfOpen()
			return true
		}
		return false
	case StateBreakerHalfOpen:
		return true
	case StateBreakerStopped:
		return false
	default:
		return true
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	if cb == nil || !cb.config.Enabled {
		return
	}

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	state := cb.getStateUnsafe()

	switch state {
	case StateBreakerClosed:
		cb.failures.Store(0)
	case StateBreakerOpen:
		cb.logger.Warn("Success recorded in open circuit breaker state",
			zap.String("service", cb.serviceName))
	case StateBreakerHalfOpen:
		successes := cb.successes.Add(1)
		cb.logger.Debug("Success recorded in half-open state",
			zap.String("service", cb.serviceName),
			zap.Int32("successes", successes),
			zap.Int("required", cb.config.HalfOpenRequests))

		if successes >= int32(cb.config.HalfOpenRequests) {
			cb.transitionToClosed()
		}
	case StateBreakerStopped:
		return
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	if cb == nil || !cb.config.Enabled {
		return
	}

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	state := cb.getStateUnsafe()

	cb.lastFail.Store(time.Now().Unix())

	switch state {
	case StateBreakerStopped:
		return
	case StateBreakerClosed:
		failures := cb.failures.Add(1)
		cb.logger.Debug("Failure recorded in closed state",
			zap.String("service", cb.serviceName),
			zap.Int32("failures", failures),
			zap.Int("threshold", cb.config.FailureThreshold))

		if failures >= int32(cb.config.FailureThreshold) {
			cb.transitionToOpen()
		}
	case StateBreakerOpen:
	case StateBreakerHalfOpen:
		cb.transitionToOpen()
	}
}

func (cb *CircuitBreaker) GetState() (state int32, failures int32, lastFail int64) {
	if cb == nil {
		return 0, 0, 0
	}

	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return int32(cb.getStateUnsafe()), cb.failures.Load(), cb.lastFail.Load()
}

func (cb *CircuitBreaker) GetStateString() string {
	if cb == nil || !cb.config.Enabled {
		return "disabled"
	}

	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return cb.stateToString(cb.getStateUnsafe())
}

func (cb *CircuitBreaker) Reset() {
	if cb == nil || !cb.config.Enabled {
		return
	}

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	oldState := cb.getStateUnsafe()
	if oldState == StateBreakerStopped {
		return
	}

	cb.transitionToClosed()

	cb.logger.Info("Circuit breaker manually reset",
		zap.String("service", cb.serviceName),
		zap.String("old_state", cb.stateToString(oldState)),
		zap.String("new_state", "closed"))
}

func (cb *CircuitBreaker) Stop() error {
	if cb == nil || !cb.config.Enabled {
		return nil
	}

	cb.mutex.Lock()
	currentState := cb.getStateUnsafe()
	cb.mutex.Unlock()

	if currentState == StateBreakerStopped || !cb.transitionState(currentState, StateBreakerStopped) {
		return types.ErrServerNotRunning
	}

	defer cb.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), cb.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			if cb.monitorTicker != nil {
				cb.monitorTicker.Stop()
			}
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			cb.logger.Warn("Circuit breaker stop timeout",
				zap.String("service", cb.serviceName))
		default:
			cb.logger.Error("Error during circuit breaker shutdown",
				zap.String("service", cb.serviceName),
				zap.Error(err))
		}
	} else {
		cb.logger.Debug("Circuit breaker stopped gracefully",
			zap.String("service", cb.serviceName))
	}

	return nil
}

func (cb *CircuitBreaker) IsRunning() bool {
	if cb == nil {
		return false
	}

	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return cb.getStateUnsafe() != StateBreakerStopped
}

func (cb *CircuitBreaker) getStateUnsafe() CircuitBreakerState {
	state := cb.state.Load()
	if state == nil {
		return StateBreakerClosed
	}
	return state.(CircuitBreakerState)
}

func (cb *CircuitBreaker) transitionState(from, to CircuitBreakerState) bool {
	return cb.state.CompareAndSwap(from, to)
}

func (cb *CircuitBreaker) transitionToClosed() {
	currentState := cb.getStateUnsafe()
	if cb.transitionState(currentState, StateBreakerClosed) {
		cb.failures.Store(0)
		cb.successes.Store(0)
		cb.lastFail.Store(0)
		cb.logger.Info("Circuit breaker closed",
			zap.String("service", cb.serviceName))
	}
}

func (cb *CircuitBreaker) transitionToOpen() {
	currentState := cb.getStateUnsafe()
	if cb.transitionState(currentState, StateBreakerOpen) {
		cb.failures.Store(1)
		cb.successes.Store(0)
		cb.logger.Warn("Circuit breaker opened",
			zap.String("service", cb.serviceName),
			zap.Int32("failures", cb.failures.Load()),
			zap.Int("threshold", cb.config.FailureThreshold))
	}
}

func (cb *CircuitBreaker) transitionToHalfOpen() {
	currentState := cb.getStateUnsafe()
	if cb.transitionState(currentState, StateBreakerHalfOpen) {
		cb.successes.Store(0)
		cb.logger.Info("Circuit breaker transitioned to half-open",
			zap.String("service", cb.serviceName))
	}
}

func (cb *CircuitBreaker) monitorLoop() {
	defer func() {
		cb.logger.Debug("Circuit breaker monitor loop stopped",
			zap.String("service", cb.serviceName))
	}()

	for {
		select {
		case <-cb.ctx.Done():
			return
		case <-cb.monitorTicker.C:
			cb.mutex.RLock()
			state := cb.getStateUnsafe()
			cb.mutex.RUnlock()

			if state == StateBreakerStopped {
				return
			}

			cb.performHealthCheck()
		}
	}
}

func (cb *CircuitBreaker) performHealthCheck() {
	cb.mutex.RLock()
	state := cb.getStateUnsafe()
	failures := cb.failures.Load()
	successes := cb.successes.Load()
	lastFailTime := cb.lastFail.Load()
	cb.mutex.RUnlock()

	cb.logger.Debug("Circuit breaker health check",
		zap.String("service", cb.serviceName),
		zap.String("state", cb.stateToString(state)),
		zap.Int32("failures", failures),
		zap.Int32("successes", successes),
		zap.Int64("last_fail", lastFailTime))
}

func (cb *CircuitBreaker) stateToString(state CircuitBreakerState) string {
	switch state {
	case StateBreakerClosed:
		return "closed"
	case StateBreakerOpen:
		return "open"
	case StateBreakerHalfOpen:
		return "half-open"
	case StateBreakerStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

func IsCircuitBreakerFailure(statusCode int, err error) bool {
	if err != nil {
		return true
	}

	switch {
	case statusCode >= 500:
		return true
	case statusCode == 429:
		return true
	case statusCode == 408:
		return true
	case statusCode >= 400:
		return false
	default:
		return false
	}
}

func IsSuccessfulResponse(statusCode int, err error) bool {
	if err != nil {
		return false
	}

	switch {
	case statusCode >= 200 && statusCode < 300:
		return true
	case statusCode >= 400 && statusCode < 500:
		return statusCode != 429 && statusCode != 408
	default:
		return false
	}
}
