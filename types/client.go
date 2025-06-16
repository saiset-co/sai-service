package types

import (
	"time"
)

type ClientManager interface {
	LifecycleManager
	GetClient(serviceName string) (HttpClient, error)
	Call(serviceName, method, path string, data interface{}, opts CallOptions) error
	Get(serviceName, path string, opts CallOptions) (map[string]interface{}, error)
	Post(serviceName, path string, data interface{}, opts CallOptions) (map[string]interface{}, error)
}

type HttpClient interface {
	Call(method string, path string, data interface{}, opts CallOptions) error
	GetState() (state int32, failures int32, lastFail int64)
	Close()
}

type CallOptions struct {
	Threshold int
	Timeout   time.Duration
	Retry     int
	Headers   map[string]string
}
