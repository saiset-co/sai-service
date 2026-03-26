package types

import (
	"time"
)

type ActionBroker interface {
	Publish(action string, payload interface{}) error
	Subscribe(action string, handler ActionHandler) error
	Unsubscribe(action string) error
}

type Dispatcher interface {
	ActionBroker
}

type ActionHandler func(payload *ActionMessage) error
type ActionBrokerCreator func(config interface{}) (ActionBroker, error)

type ActionMessage struct {
	Action    string            `json:"action"`
	Payload   interface{}       `json:"payload"`
	Timestamp time.Time         `json:"timestamp"`
	Source    string            `json:"source"`
	Metadata  map[string]string `json:"metadata"`
	MessageID string            `json:"message_id"`
}
