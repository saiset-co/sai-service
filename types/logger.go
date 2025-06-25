package types

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LoggerManager interface {
	Logger
}

type Logger interface {
	Error(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Debug(msg string, fields ...zap.Field)
	Log(lvl zapcore.Level, msg string, fields ...zap.Field)
}

type LoggerCreator func(config interface{}) (Logger, error)
