package client

import (
	"sync"
	"time"

	"go.uber.org/zap"

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
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	config      *CircuitBreakerConfig
	logger      types.Logger
	serviceName string

	mutex     sync.RWMutex
	state     CircuitBreakerState
	failures  int32
	successes int32
	lastFail  int64
}

func NewCircuitBreaker(config *CircuitBreakerConfig, logger types.Logger, serviceName string) *CircuitBreaker {
	if config == nil || !config.Enabled {
		return nil
	}

	return &CircuitBreaker{
		config:      config,
		logger:      logger,
		serviceName: serviceName,
		state:       StateClosed,
	}
}

func (cb *CircuitBreaker) CanExecute() bool {
	if cb == nil {
		return true
	}

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(time.Unix(cb.lastFail, 0)) > cb.config.RecoveryTimeout {
			cb.state = StateHalfOpen
			cb.successes = 0
			cb.logger.Info("Circuit breaker transitioned to half-open",
				zap.String("service", cb.serviceName))
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return true
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	if cb == nil {
		return
	}

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	switch cb.state {
	case StateClosed:
		cb.failures = 0
	case StateOpen:
		cb.logger.Warn("Success recorded in open circuit breaker state",
			zap.String("service", cb.serviceName))
	case StateHalfOpen:
		cb.successes++
		cb.logger.Debug("Success recorded in half-open state",
			zap.String("service", cb.serviceName),
			zap.Int32("successes", cb.successes),
			zap.Int("required", cb.config.HalfOpenRequests))

		if cb.successes >= int32(cb.config.HalfOpenRequests) {
			cb.state = StateClosed
			cb.failures = 0
			cb.successes = 0
			cb.logger.Info("Circuit breaker closed",
				zap.String("service", cb.serviceName))
		}
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	if cb == nil {
		return
	}

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.lastFail = time.Now().Unix()

	switch cb.state {
	case StateClosed:
		cb.failures++
		cb.logger.Debug("Failure recorded in closed state",
			zap.String("service", cb.serviceName),
			zap.Int32("failures", cb.failures),
			zap.Int("threshold", cb.config.FailureThreshold))

		if cb.failures >= int32(cb.config.FailureThreshold) {
			cb.state = StateOpen
			cb.logger.Warn("Circuit breaker opened",
				zap.String("service", cb.serviceName),
				zap.Int32("failures", cb.failures),
				zap.Int("threshold", cb.config.FailureThreshold))
		}
	case StateOpen:
	case StateHalfOpen:
		cb.state = StateOpen
		cb.failures = 1
		cb.successes = 0
		cb.logger.Warn("Circuit breaker reopened from half-open",
			zap.String("service", cb.serviceName))
	}
}

func (cb *CircuitBreaker) GetState() (state int32, failures int32, lastFail int64) {
	if cb == nil {
		return 0, 0, 0
	}

	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return int32(cb.state), cb.failures, cb.lastFail
}

func (cb *CircuitBreaker) GetStateString() string {
	if cb == nil {
		return "disabled"
	}

	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	switch cb.state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

func (cb *CircuitBreaker) Reset() {
	if cb == nil {
		return
	}

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	oldState := cb.state
	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.lastFail = 0

	cb.logger.Info("Circuit breaker manually reset",
		zap.String("service", cb.serviceName),
		zap.String("old_state", cb.stateToString(oldState)),
		zap.String("new_state", "closed"))
}

func (cb *CircuitBreaker) stateToString(state CircuitBreakerState) string {
	switch state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
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
