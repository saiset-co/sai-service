package logger

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap/zapcore"
)

const DefaultLogBufferSize = 500

type LogEntry struct {
	Level  zapcore.Level
	Text   string
	Fields string
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

func zapFieldToValue(f zapcore.Field) any {
	switch f.Type {
	case zapcore.StringType:
		return f.String
	case zapcore.BoolType:
		return f.Integer == 1
	case zapcore.Int8Type, zapcore.Int16Type, zapcore.Int32Type, zapcore.Int64Type,
		zapcore.Uint8Type, zapcore.Uint16Type, zapcore.Uint32Type, zapcore.Uint64Type:
		return f.Integer
	case zapcore.Float32Type:
		return math.Float32frombits(uint32(f.Integer))
	case zapcore.Float64Type:
		return math.Float64frombits(uint64(f.Integer))
	case zapcore.ByteStringType:
		if bs, ok := f.Interface.([]byte); ok {
			return string(bs)
		}
		return ""
	case zapcore.DurationType:
		return time.Duration(f.Integer).String()
	case zapcore.ErrorType:
		if f.Interface != nil {
			return fmt.Sprintf("%v", f.Interface)
		}
		return ""
	default:
		if f.Interface != nil {
			return f.Interface
		}
		if f.String != "" {
			return f.String
		}
		return f.Integer
	}
}

func (b *LogRingBuffer) Write(e zapcore.Entry, fields []zapcore.Field) error {
	var header string
	switch e.Level {
	case zapcore.ErrorLevel, zapcore.FatalLevel, zapcore.PanicLevel:
		header = "[ERR]"
	case zapcore.WarnLevel:
		header = "[WRN]"
	case zapcore.InfoLevel:
		header = "[INF]"
	default:
		header = "[DBG]"
	}
	text := e.Time.Format("2006-01-02 15:04:05") + " " + header + " " + e.Message

	var fieldsJSON string
	if len(fields) > 0 {
		m := make(map[string]any, len(fields))
		for _, f := range fields {
			if skipLogFields[f.Key] {
				continue
			}
			m[f.Key] = zapFieldToValue(f)
		}
		if len(m) > 0 {
			if b, err := json.Marshal(m); err == nil {
				fieldsJSON = string(b)
			}
		}
	}

	entry := LogEntry{Level: e.Level, Text: text, Fields: fieldsJSON}
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
