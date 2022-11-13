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

	for _, step := range steps {
		val, _ := configuration[step]

		switch val.(type) {
		case map[string]interface{}:
			configuration = val.(map[string]interface{})
			break
		case string:
			return val.(string)
		case int:
			return val.(int)
		case bool:
			return val.(bool)
		default:
			return def
		}
	}

	return def
}
