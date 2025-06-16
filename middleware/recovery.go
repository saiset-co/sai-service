package middleware

import (
	"runtime"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type RecoveryMiddleware struct {
	config         types.ConfigManager
	logger         types.Logger
	metrics        types.MetricsManager
	recoveryConfig *RecoveryConfig
}

type RecoveryConfig struct {
	StackTrace bool `json:"stack_trace"`
}

func NewRecoveryMiddleware(config types.ConfigManager, logger types.Logger, metrics types.MetricsManager) *RecoveryMiddleware {
	var recoveryConfig = &RecoveryConfig{
		StackTrace: true,
	}

	if config.GetConfig().Middlewares.Recovery.Params != nil {
		err := utils.UnmarshalConfig(config.GetConfig().Middlewares.Recovery.Params, recoveryConfig)
		if err != nil {
			logger.Error("Failed to unmarshal Recovery middleware config", zap.Error(err))
		}
	}

	return &RecoveryMiddleware{
		config:         config,
		logger:         logger,
		metrics:        metrics,
		recoveryConfig: recoveryConfig,
	}
}

func (r *RecoveryMiddleware) Name() string { return "recovery" }
func (r *RecoveryMiddleware) Weight() int  { return 10 }

func (r *RecoveryMiddleware) Handle(ctx *fasthttp.RequestCtx, next func(*fasthttp.RequestCtx), _ *types.RouteConfig) {
	start := time.Now()

	defer func() {
		if rec := recover(); rec != nil {
			duration := time.Since(start)
			r.recordPanicMetrics(duration)

			var stack string
			if r.recoveryConfig.StackTrace {
				stack = r.getStackTrace()
			}

			r.logPanic(rec, stack, duration, ctx)

			utils.CreateErrorResponse(ctx)
		}
	}()

	next(ctx)
}

func (r *RecoveryMiddleware) recordPanicMetrics(duration time.Duration) {
	if r.metrics == nil {
		return
	}

	counter := r.metrics.Counter("middleware_panics_total", map[string]string{
		"middleware": "recovery",
	})
	counter.Inc()

	histogram := r.metrics.Histogram("panic_recovery_duration_seconds",
		[]float64{0.001, 0.01, 0.1, 0.5, 1.0},
		map[string]string{"middleware": "recovery"})
	histogram.Observe(duration.Seconds())
}

func (r *RecoveryMiddleware) logPanic(rec interface{}, stack string, duration time.Duration, ctx *fasthttp.RequestCtx) {
	fields := []zap.Field{
		zap.Any("panic", rec),
		zap.Duration("duration", duration),
		zap.String("method", string(ctx.Method())),
		zap.String("path", string(ctx.Path())),
		zap.String("remote_addr", ctx.RemoteIP().String()),
	}

	if r.recoveryConfig.StackTrace && stack != "" {
		fields = append(fields, zap.String("stack", stack))
	}

	if requestID := string(ctx.Request.Header.Peek("X-Request-ID")); requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}

	if userAgent := string(ctx.UserAgent()); userAgent != "" {
		fields = append(fields, zap.String("user_agent", userAgent))
	}

	r.logger.Error("Recovered from panic", fields...)
}

func (r *RecoveryMiddleware) getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)

	if n == len(buf) {
		buf = make([]byte, 16384)
		n = runtime.Stack(buf, false)
	}

	return utils.Intern(buf[:n])
}
