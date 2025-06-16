package types

import (
	"errors"
	"fmt"
)

var (
	ErrConfigNotFound       = errors.New("config not found")
	ErrConfigInvalidPath    = errors.New("config invalid path")
	ErrConfigParseFailed    = errors.New("config parse failed")
	ErrConfigIsNil          = errors.New("config is nil")
	ErrConfigLoadFailed     = errors.New("config load failed")
	ErrConfigValidateFailed = errors.New("config validate failed")
)

var (
	ErrServerNotRunning        = errors.New("server not running")
	ErrServerAlreadyRunning    = errors.New("server already running")
	ErrServerStartFailed       = errors.New("server start failed")
	ErrServerStopFailed        = errors.New("server stop failed")
	ErrRouteFinalizationFailed = errors.New("route finalization failed")
	ErrHandlerIsNil            = errors.New("handler is nil")
)

var (
	ErrMiddlewareNotFound     = errors.New("middleware not found")
	ErrMiddlewareInvalidType  = errors.New("middleware invalid type")
	ErrMiddlewareOrderInvalid = errors.New("middleware order invalid")
	ErrAuthTokenInvalid       = errors.New("auth token invalid")
	ErrBodyTooLarge           = errors.New("body too large")
	ErrRateLimitExceeded      = errors.New("rate limit exceeded")
)

var (
	ErrCacheNotFound         = errors.New("cache not found")
	ErrCacheKeyEmpty         = errors.New("cache key empty")
	ErrCacheConnectionFailed = errors.New("cache connection failed")
	ErrCacheTypeUnknown      = errors.New("cache type unknown")
	ErrCacheOperationFailed  = errors.New("cache operation failed")
	ErrCacheIsDisabled       = errors.New("cache manager is disabled")
)

var (
	ErrActionNotInitialized   = errors.New("action not initialized")
	ErrActionPublishFailed    = errors.New("action publish failed")
	ErrActionConnectionFailed = errors.New("action connection failed")
	ErrActionConfigInvalid    = errors.New("action config invalid")
	ErrActionTypeUnknown      = errors.New("action type unknown")
	ErrActionIsDisabled       = errors.New("action broker is disabled")
)

var (
	ErrCronJobNotFound       = errors.New("cron job not found")
	ErrCronIsRunning         = errors.New("cron is running")
	ErrCronSchedulerStopped  = errors.New("cron scheduler stopped")
	ErrCronJobExists         = errors.New("cron job exists")
	ErrCronExpressionInvalid = errors.New("cron expression invalid")
	ErrCronJobFailed         = errors.New("cron job failed")
	ErrCronJobNameIsEmpty    = errors.New("cron job name is empty")
	ErrCronJobIsNil          = errors.New("cron job is nil")
	ErrCronJobTimeout        = errors.New("cron job timeout")
)

var (
	ErrMetricsTypeUnknown   = errors.New("metrics type unknown")
	ErrMetricsStartFailed   = errors.New("metrics start failed")
	ErrMetricsConfigInvalid = errors.New("metrics config invalid")
	ErrMetricsIsDisabled    = errors.New("metrics manager is disabled")
)

var (
	ErrClientNotFound        = errors.New("client not found")
	ErrClientCreateFailed    = errors.New("client create failed")
	ErrClientRequestFailed   = errors.New("client request failed")
	ErrClientResponseInvalid = errors.New("client response invalid")
	ErrClientTimeout         = errors.New("client timeout")
	ErrCircuitBreakerOpen    = errors.New("circuit breaker open")
)

var (
	ErrHealthCheckFailed  = errors.New("health check failed")
	ErrHealthCheckTimeout = errors.New("health check timeout")
)

var (
	ErrLogFileIsEmpty     = errors.New("log file is empty")
	ErrLogFileWrongFormat = errors.New("log file wrong format")
	ErrLoggerTypeUnknown  = errors.New("logger type unknown")
)

var (
	ErrServiceIsRunning     = errors.New("service is running")
	ErrServiceIsNotRunning  = errors.New("service is not running")
	ErrComponentNotFound    = errors.New("component not found")
	ErrComponentStartFailed = errors.New("component start failed")
	ErrComponentStopFailed  = errors.New("component stop failed")
)

var (
	ErrInvalidParameter = errors.New("invalid parameter")
	ErrOperationFailed  = errors.New("operation failed")
	ErrNotImplemented   = errors.New("not implemented")
	ErrPermissionDenied = errors.New("permission denied")
	ErrResourceNotFound = errors.New("resource not found")
	ErrInternalError    = errors.New("internal error")
	ErrContextCancelled = errors.New("context cancelled")
	ErrContextTimeout   = errors.New("context timeout")
	ErrInvalidState     = errors.New("invalid state")
	ErrNotSupported     = errors.New("not supported")
)

func Errorf(baseErr error, format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", baseErr, fmt.Sprintf(format, args...))
}

func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

func NewErrorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

func IsError(err, target error) bool {
	return errors.Is(err, target)
}
