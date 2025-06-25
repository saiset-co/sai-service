package utils

import (
	"bytes"
	"fmt"
	"github.com/bytedance/sonic"
	"sync"
)

type JSONBufferPool struct {
	pool sync.Pool
}

func (p *JSONBufferPool) Get() *bytes.Buffer {
	if buf := p.pool.Get(); buf != nil {
		return buf.(*bytes.Buffer)
	}
	return bytes.NewBuffer(make([]byte, 0, 1024))
}

func (p *JSONBufferPool) Put(buf *bytes.Buffer) {
	buf.Reset()
	if buf.Cap() < 16*1024 {
		p.pool.Put(buf)
	}
}

var jsonPool = &JSONBufferPool{}

func MarshalToBuffer(data interface{}, buf *bytes.Buffer) error {
	buf.Reset()
	encoder := sonic.ConfigDefault.NewEncoder(buf)
	return encoder.Encode(data)
}

func Marshal(data interface{}) ([]byte, error) {
	buf := jsonPool.Get()
	defer jsonPool.Put(buf)

	if err := MarshalToBuffer(data, buf); err != nil {
		return nil, err
	}

	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
}

func Unmarshal[T any](data []byte, target *T) error {
	return sonic.ConfigDefault.Unmarshal(data, target)
}

func UnmarshalConfig[T any](config interface{}, target *T) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	if typed, ok := config.(*T); ok {
		*target = *typed
		return nil
	}

	configBytes, err := sonic.ConfigDefault.Marshal(config)
	if err != nil {
		return err
	}

	return sonic.ConfigDefault.Unmarshal(configBytes, target)
}
