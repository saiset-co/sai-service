package logger

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"go.uber.org/zap/zapcore"
)

const DefaultLogBufferSize = 500

type LogEntry struct {
	Level zapcore.Level
	Text  string
}

type LogRingBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	max     int
	head    int
	count   int
}

func NewLogRingBuffer(max int) *LogRingBuffer {
	if max <= 0 {
		max = DefaultLogBufferSize
	}
	return &LogRingBuffer{entries: make([]LogEntry, max), max: max}
}

func (b *LogRingBuffer) Enabled(zapcore.Level) bool { return true }

func (b *LogRingBuffer) With([]zapcore.Field) zapcore.Core { return b }

func (b *LogRingBuffer) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return ce.AddCore(e, b)
}

var skipLogFields = map[string]bool{
	"response_body_truncated": true,
	"user_agent":              true,
}

func zapFieldToString(f zapcore.Field) string {
	switch f.Type {
	case zapcore.StringType:
		return f.String
	case zapcore.BoolType:
		if f.Integer == 1 {
			return "true"
		}
		return "false"
	case zapcore.Int8Type, zapcore.Int16Type, zapcore.Int32Type, zapcore.Int64Type,
		zapcore.Uint8Type, zapcore.Uint16Type, zapcore.Uint32Type, zapcore.Uint64Type:
		return fmt.Sprintf("%d", f.Integer)
	case zapcore.Float32Type:
		return fmt.Sprintf("%g", math.Float32frombits(uint32(f.Integer)))
	case zapcore.Float64Type:
		return fmt.Sprintf("%g", math.Float64frombits(uint64(f.Integer)))
	case zapcore.DurationType:
		return fmt.Sprintf("%dns", f.Integer)
	case zapcore.ErrorType:
		if f.Interface != nil {
			return fmt.Sprintf("%v", f.Interface)
		}
		return ""
	default:
		if f.Interface != nil {
			return fmt.Sprintf("%v", f.Interface)
		}
		if f.String != "" {
			return f.String
		}
		return fmt.Sprintf("%d", f.Integer)
	}
}

func (b *LogRingBuffer) Write(e zapcore.Entry, fields []zapcore.Field) error {
	var sb strings.Builder
	sb.WriteString(e.Time.Format("15:04:05"))
	sb.WriteByte(' ')
	switch e.Level {
	case zapcore.ErrorLevel, zapcore.FatalLevel, zapcore.PanicLevel:
		sb.WriteString("[ERR]")
	case zapcore.WarnLevel:
		sb.WriteString("[WRN]")
	case zapcore.InfoLevel:
		sb.WriteString("[INF]")
	default:
		sb.WriteString("[DBG]")
	}
	sb.WriteByte(' ')
	sb.WriteString(e.Message)
	for _, f := range fields {
		if skipLogFields[f.Key] {
			continue
		}
		s := zapFieldToString(f)
		if len(s) > 200 {
			s = s[:197] + "..."
		}
		sb.WriteByte(' ')
		sb.WriteString(f.Key)
		sb.WriteByte('=')
		sb.WriteString(s)
	}

	entry := LogEntry{Level: e.Level, Text: sb.String()}
	b.mu.Lock()
	b.entries[b.head] = entry
	b.head = (b.head + 1) % b.max
	if b.count < b.max {
		b.count++
	}
	b.mu.Unlock()
	return nil
}

func (b *LogRingBuffer) Sync() error { return nil }

func (b *LogRingBuffer) GetAll() []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.count == 0 {
		return nil
	}
	result := make([]LogEntry, b.count)
	start := (b.head - b.count + b.max) % b.max
	for i := 0; i < b.count; i++ {
		result[i] = b.entries[(start+i)%b.max]
	}
	return result
}
