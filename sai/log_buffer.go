package sai

import (
	"github.com/saiset-co/sai-service/logger"
	"go.uber.org/zap/zapcore"
)

var globalLogBuffer *logger.LogRingBuffer

func InstallLogBuffer() {
	globalLogBuffer = logger.NewLogRingBuffer(logger.DefaultLogBufferSize)
	l, ok := Logger().(*logger.Manager)
	if !ok {
		return
	}
	l.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, globalLogBuffer)
	})
}

func LogBuffer() *logger.LogRingBuffer {
	return globalLogBuffer
}
