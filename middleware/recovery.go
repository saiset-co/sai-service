package middleware

import (
	"go.uber.org/zap"
	"runtime"
	"sync"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type RecoveryMiddleware struct {
	config         types.ConfigManager
	logger         types.Logger
	metrics        types.MetricsManager
	recoveryConfig *RecoveryConfig
	name           string
	weight         int
	stackBufPool   sync.Pool
	fieldsPool     sync.Pool
	panicLabels    map[string]string
	durationLabels map[string]string
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
		name:           "recovery",
		weight:         config.GetConfig().Middlewares.Recovery.Weight,
		config:         config,
		logger:         logger,
		metrics:        metrics,
		recoveryConfig: recoveryConfig,

		stackBufPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 4096)
			},
		},

		fieldsPool: sync.Pool{
			New: func() interface{} {
				return make([]zap.Field, 0, 8)
			},
		},

		panicLabels: map[string]string{
			"middleware": "recovery",
		},
		durationLabels: map[string]string{
			"middleware": "recovery",
		},
	}
}

func (r *RecoveryMiddleware) Name() string          { return r.name }
func (r *RecoveryMiddleware) Weight() int           { return r.weight }
func (r *RecoveryMiddleware) Provider() interface{} { return nil }

func (r *RecoveryMiddleware) Handle(ctx *types.RequestCtx, next func(*types.RequestCtx), _ *types.RouteConfig) {
	defer func() {
		if rec := recover(); rec != nil {

			var stack string
			if r.recoveryConfig.StackTrace {
				stack = r.getStackTrace()
			}

			r.logPanic(rec, stack, ctx)
			types.CreateErrorResponse(ctx)
		}
	}()

	next(ctx)
}

func (r *RecoveryMiddleware) logPanic(rec interface{}, stack string, ctx *types.RequestCtx) {
	fields := r.fieldsPool.Get().(*[]zap.Field)
	defer func() {
		*fields = (*fields)[:0]
		r.fieldsPool.Put(fields)
	}()

	*fields = append(*fields,
		zap.Any("panic", rec),
		zap.ByteString("method", ctx.Method()),
		zap.ByteString("path", ctx.Path()),
		zap.ByteString("remote_addr", ctx.RemoteIP()),
	)

	if r.recoveryConfig.StackTrace && stack != "" {
		*fields = append(*fields, zap.String("stack", stack))
	}

	if requestID := ctx.Request.Header.Peek("X-Request-ID"); len(requestID) > 0 {
		*fields = append(*fields, zap.ByteString("request_id", requestID))
	}

	if userAgent := ctx.UserAgent(); len(userAgent) > 0 {
		*fields = append(*fields, zap.ByteString("user_agent", userAgent))
	}

	r.logger.Error("Recovered from panic", *fields...)
}

func (r *RecoveryMiddleware) getStackTrace() string {
	buf := r.stackBufPool.Get().(*[]byte)
	defer func() {
		*buf = (*buf)[:0]
		r.stackBufPool.Put(buf)
	}()

	n := runtime.Stack(*buf, false)

	if n == len(*buf) {
		newBuf := make([]byte, 16384)
		n = runtime.Stack(newBuf, false)

		if n == len(newBuf) {
			newBuf = make([]byte, 65536)
			n = runtime.Stack(newBuf, false)
		}

		return utils.BytesToString(newBuf[:n])
	}

	return utils.BytesToString((*buf)[:n])
}
