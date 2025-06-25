package types

import (
	"time"
)

type ClientManager interface {
	Call(serviceName, method, path string, data interface{}, opts *CallOptions) ([]byte, int, error)
	//RegisterWebhook(serviceName, event, webhookURL string) ([]byte, int, error)
}

type CallOptions struct {
	Threshold int
	Timeout   time.Duration
	Retry     int
	Headers   map[string]string
}
