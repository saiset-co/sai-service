package logger

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

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

func NewDefaultLogger(config *types.LoggerConfig) (*zap.Logger, error) {
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

	zapConfig, err := buildZapConfig(lConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build logger config: %w", err)
	}

	logger, err := zapConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	logger.Info("Logger initialized",
		zap.String("level", lConfig.Level),
		zap.String("format", lConfig.Format),
		zap.String("output", lConfig.Output),
	)

	return logger, nil
}

func buildZapConfig(config *ZapLoggerConfig) (zapConfig zap.Config, err error) {
	level := parseLogLevel(config.Level)

	if config.Format == "console" {
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		zapConfig = zap.NewProductionConfig()
		zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	zapConfig.Level = zap.NewAtomicLevelAt(level)

	err = configureOutput(&zapConfig, config)
	if err != nil {
		return zapConfig, nil
	}

	return zapConfig, nil
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

func configureOutput(zapConfig *zap.Config, config *ZapLoggerConfig) error {
	switch config.Output {
	case "stdout":
		zapConfig.OutputPaths = []string{"stdout"}
		zapConfig.ErrorOutputPaths = []string{"stderr"}

	case "stderr":
		zapConfig.OutputPaths = []string{"stderr"}
		zapConfig.ErrorOutputPaths = []string{"stderr"}

	case "file":
		if config.File != "" {
			err := ensureLogDir(config.File)
			if err != nil {
				return err
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

	return nil
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
