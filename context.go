package saiService

import (
	"context"
	"strings"
)

type Context struct {
	Configuration map[string]interface{}
	Context       context.Context
}

func NewContext() *Context {
	return &Context{
		Configuration: map[string]interface{}{},
		Context:       context.Background(),
	}
}

func (c *Context) SetValue(key string, value interface{}) {
	c.Context = context.WithValue(context.Background(), key, value)
}

func (c *Context) GetConfig(path string, def interface{}) any {
	steps := strings.Split(path, ".")
	configuration := c.Configuration

	if len(steps) == 0 {
		return def
	}

	for _, step := range steps {
		val, ok := configuration[step]

		if !ok {
			return def
		}

		switch val.(type) {
		case map[string]interface{}:
			configuration = val.(map[string]interface{})
			break
		default:
			return val
		}
	}

	return configuration
}
