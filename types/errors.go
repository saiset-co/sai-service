package types

import (
	"github.com/pkg/errors"
)

var (
	ErrConfigNotFound       = NewError("config not found")
	ErrLoggerConfigInvalid  = NewError("config invalid")
	ErrConfigInvalidPath    = NewError("config invalid path")
	ErrConfigParseFailed    = NewError("config parse failed")
	ErrConfigIsNil          = NewError("config is nil")
	ErrConfigLoadFailed     = NewError("config load failed")
	ErrConfigValidateFailed = NewError("config validate failed")
)

var (
	ErrServerNotRunning        = NewError("server not running")
	ErrServerAlreadyRunning    = NewError("server already running")
	ErrServerStartFailed       = NewError("server start failed")
	ErrServerStopFailed        = NewError("server stop failed")
	ErrRouteFinalizationFailed = NewError("route finalization failed")
	ErrHandlerIsNil            = NewError("handler is nil")
)

var (
	ErrMiddlewareNotFound     = NewError("middleware not found")
	ErrMiddlewareInvalidType  = NewError("middleware invalid type")
	ErrMiddlewareOrderInvalid = NewError("middleware order invalid")
	ErrAuthTokenInvalid       = NewError("auth token invalid")
	ErrBodyTooLarge           = NewError("body too large")
	ErrRateLimitExceeded      = NewError("rate limit exceeded")
)

var (
	ErrCacheNotFound         = NewError("cache not found")
	ErrCacheKeyEmpty         = NewError("cache key empty")
	ErrCacheConnectionFailed = NewError("cache connection failed")
	ErrCacheTypeUnknown      = NewError("cache type unknown")
	ErrCacheOperationFailed  = NewError("cache operation failed")
	ErrCacheIsDisabled       = NewError("cache manager is disabled")
)

var (
	ErrActionNotInitialized   = NewError("action not initialized")
	ErrActionPublishFailed    = NewError("action publish failed")
	ErrActionConnectionFailed = NewError("action connection failed")
	ErrActionConfigInvalid    = NewError("action config invalid")
	ErrActionTypeUnknown      = NewError("action type unknown")
	ErrActionIsDisabled       = NewError("action broker is disabled")
	ErrActionIsRunning        = NewError("action is running")
)

var (
	ErrCronJobNotFound       = NewError("cron job not found")
	ErrCronIsRunning         = NewError("cron is running")
	ErrCronSchedulerStopped  = NewError("cron scheduler stopped")
	ErrCronJobExists         = NewError("cron job exists")
	ErrCronExpressionInvalid = NewError("cron expression invalid")
	ErrCronJobFailed         = NewError("cron job failed")
	ErrCronJobNameIsEmpty    = NewError("cron job name is empty")
	ErrCronJobIsNil          = NewError("cron job is nil")
	ErrCronJobTimeout        = NewError("cron job timeout")
)

var (
	ErrMetricsNotRunning     = NewError("metrics manager not running")
	ErrMetricsTypeUnknown    = NewError("metrics type unknown")
	ErrMetricsStartFailed    = NewError("metrics start failed")
	ErrMetricsConfigInvalid  = NewError("metrics config invalid")
	ErrMetricsIsDisabled     = NewError("metrics manager is disabled")
	ErrTemplateFailed        = NewError("metrics manager template build failed")
	ErrMetricsGetFailed      = NewError("metrics manager get failed")
	ErrMetricsStatsGetFailed = NewError("metrics manager stats get failed")
)

var (
	ErrClientNotFound        = NewError("client not found")
	ErrClientCreateFailed    = NewError("client create failed")
	ErrClientRequestFailed   = NewError("client request failed")
	ErrClientResponseInvalid = NewError("client response invalid")
	ErrClientTimeout         = NewError("client timeout")
	ErrCircuitBreakerOpen    = NewError("circuit breaker open")
)

var (
	ErrHealthIsNotRunning = NewError("health manager is not running")
	ErrHealthCheckFailed  = NewError("health check failed")
	ErrHealthCheckTimeout = NewError("health check timeout")
)

var (
	ErrDocsIsNotRunning   = NewError("documentation manager is not running")
	ErrDocsGenerateFailed = NewError("documentation generation failed")
)

var (
	ErrLogFileIsEmpty     = NewError("log file is empty")
	ErrLogFileWrongFormat = NewError("log file wrong format")
	ErrLoggerTypeUnknown  = NewError("logger type unknown")
)

var (
	ErrServiceIsRunning     = NewError("service is running")
	ErrServiceIsNotRunning  = NewError("service is not running")
	ErrComponentNotFound    = NewError("component not found")
	ErrComponentStartFailed = NewError("component start failed")
	ErrComponentStopFailed  = NewError("component stop failed")
)

var (
	ErrInvalidParameter = NewError("invalid parameter")
	ErrOperationFailed  = NewError("operation failed")
	ErrNotImplemented   = NewError("not implemented")
	ErrPermissionDenied = NewError("permission denied")
	ErrResourceNotFound = NewError("resource not found")
	ErrInternalError    = NewError("internal error")
	ErrContextCancelled = NewError("context cancelled")
	ErrContextTimeout   = NewError("context timeout")
	ErrInvalidState     = NewError("invalid state")
	ErrNotSupported     = NewError("not supported")
	ErrMarshaling       = NewError("error during marshalling")
	ErrUnMarshaling     = NewError("error during unmarshalling")
	ErrWriteContext     = NewError("error during write context")
	ErrPathNotFound     = NewError("path not found")
)

func Errorf(baseErr error, format string, args ...interface{}) error {
	return errors.Wrapf(baseErr, format, args...)
}

func WrapError(err error, message string) error {
	return errors.Wrap(err, message)
}

func WrapErrorf(err error, format string, args ...interface{}) error {
	return errors.Wrapf(err, format, args...)
}

func NewError(message string) error {
	return errors.WithStack(errors.New(message))
}

func NewErrorf(format string, args ...interface{}) error {
	return errors.WithStack(errors.Errorf(format, args...))
}

func IsError(err, target error) bool {
	return errors.Is(err, target)
}
