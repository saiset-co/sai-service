package logger

import (
	"fmt"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"strings"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type ZapLoggerConfig struct {
	Level      string `yaml:"level" json:"level"`
	Format     string `yaml:"format" json:"format"`
	Output     string `yaml:"output" json:"output"`
	File       string `yaml:"file" json:"file"`
	MaxSize    int    `yaml:"max_size" json:"max_size"`
	MaxBackups int    `yaml:"max_backups" json:"max_backups"`
	MaxAge     int    `yaml:"max_age" json:"max_age"`
	Compress   bool   `yaml:"compress" json:"compress"`
}

func NewDefaultLogger(config *types.LoggerConfig) (types.Logger, error) {
	lConfig := &ZapLoggerConfig{
		Format:     "console",
		Output:     "stdout",
		File:       "",
		MaxSize:    100,
		MaxBackups: 10,
		MaxAge:     10,
		Compress:   false,
		Level:      config.Level,
	}

	if config.Config != nil {
		err := utils.UnmarshalConfig(config.Config, lConfig)
		if err != nil {
			return nil, types.WrapError(err, "failed to unmarshal logger config")
		}
	}

	logger, err := buildZapLogger(lConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	l := NewZapWrapper(logger)

	l.Info("Logger initialized",
		zap.String("level", lConfig.Level),
		zap.String("format", lConfig.Format),
		zap.String("output", lConfig.Output),
	)

	return l, nil
}

func buildZapLogger(config *ZapLoggerConfig) (*zap.Logger, error) {
	level := parseLogLevel(config.Level)

	var zapConfig zap.Config
	if config.Format == "console" {
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		zapConfig.EncoderConfig.EncodeCaller = ideCallerEncoder
	} else {
		zapConfig = zap.NewProductionConfig()
		zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	zapConfig.DisableStacktrace = true
	zapConfig.Level = zap.NewAtomicLevelAt(level)

	switch config.Output {
	case "stderr":
		zapConfig.OutputPaths = []string{"stderr"}
		zapConfig.ErrorOutputPaths = []string{"stderr"}
	case "file":
		if config.File != "" {
			err := ensureLogDir(config.File)
			if err != nil {
				return nil, err
			}
			zapConfig.OutputPaths = []string{config.File}
			zapConfig.ErrorOutputPaths = []string{config.File}
		} else {
			zapConfig.OutputPaths = []string{"stdout"}
			zapConfig.ErrorOutputPaths = []string{"stderr"}
		}
	default:
		zapConfig.OutputPaths = []string{"stdout"}
		zapConfig.ErrorOutputPaths = []string{"stderr"}
	}

	logger, err := zapConfig.Build(zap.AddCaller())
	if err != nil {
		return nil, err
	}

	return logger, nil
}

func ideCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(fmt.Sprintf("%s:%d", caller.File, caller.Line))
}

func parseLogLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

func ensureLogDir(logFile string) error {
	if logFile == "" {
		return types.ErrLogFileIsEmpty
	}

	lastSlash := strings.LastIndex(logFile, "/")
	if lastSlash == -1 {
		return types.ErrLogFileWrongFormat
	}

	dir := logFile[:lastSlash]
	err := os.MkdirAll(dir, 0755)

	return types.WrapError(err, "access denied to log directory")
}

type ZapWrapper struct {
	Logger *zap.Logger
}

func NewZapWrapper(logger *zap.Logger) types.Logger {
	return &ZapWrapper{Logger: logger}
}

func (z *ZapWrapper) Error(msg string, fields ...zap.Field) {
	z.Logger.WithOptions(zap.AddCallerSkip(2)).Error(msg, fields...)
}

func (z *ZapWrapper) Warn(msg string, fields ...zap.Field) {
	z.Logger.WithOptions(zap.AddCallerSkip(2)).Warn(msg, fields...)
}

func (z *ZapWrapper) Info(msg string, fields ...zap.Field) {
	z.Logger.WithOptions(zap.AddCallerSkip(2)).Info(msg, fields...)
}

func (z *ZapWrapper) Debug(msg string, fields ...zap.Field) {
	z.Logger.WithOptions(zap.AddCallerSkip(2)).Debug(msg, fields...)
}

func (z *ZapWrapper) Log(lvl zapcore.Level, msg string, fields ...zap.Field) {
	z.Logger.WithOptions(zap.AddCallerSkip(2)).Log(lvl, msg, fields...)
}

func (z *ZapWrapper) ErrorWithStack(msg string, stack string, fields ...zap.Field) {
	z.Logger.WithOptions(zap.AddCallerSkip(2)).Error(msg, fields...)
	z.logPrettyStack(stack)
}

func (z *ZapWrapper) ErrorWithErrStack(msg string, err error, fields ...zap.Field) {
	if err == nil {
		z.Error(msg, fields...)
		return
	}

	allFields := make([]zap.Field, 0, len(fields)+1)
	allFields = append(allFields, zap.String("error", errors.Cause(err).Error()))
	allFields = append(allFields, fields...)

	z.Logger.WithOptions(zap.AddCallerSkip(2)).Error(msg, allFields...)

	if stackStr := extractStackFromError(err); stackStr != "" {
		z.logPrettyStack(stackStr)
	}
}

type stackTracer interface {
	StackTrace() errors.StackTrace
}

func extractStackFromError(err error) string {
	if err == nil {
		return ""
	}

	stack := fmt.Sprintf("%+v", err)

	if st, ok := err.(stackTracer); ok {
		stack = fmt.Sprintf("%+v", st.StackTrace())
	}

	err = errors.Cause(err)
	if st, ok := err.(stackTracer); ok {
		stack = fmt.Sprintf("%+v", st.StackTrace())
	}

	return stack
}

func (z *ZapWrapper) logPrettyStack(stackStr string) {
	lines := strings.Split(stackStr, "\n")

	fmt.Printf("🔥 ERROR STACK TRACE\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(line, "types.NewError") ||
			strings.Contains(line, "types.NewErrorf") ||
			strings.Contains(line, "types.WrapError") ||
			strings.Contains(line, "types/errors.go:") ||
			strings.Contains(line, "(*RequestCtx).Error") ||
			strings.Contains(line, "types/fasthttp.go:") ||
			strings.Contains(line, "runtime.goexit") ||
			strings.Contains(line, "asm_amd64.s:") ||
			strings.Contains(line, "panic") {
			continue
		}

		displayLine := line
		if len(line) > 90 {
			displayLine = line[:87] + "..."
		}

		fmt.Printf("%-95s\n", displayLine)
	}
}
