package saiService

import (
	"context"
	"strings"
)

type CoreCtx struct {
	Configuration map[string]interface{}
	Ctx           context.Context
}

func (c *CoreCtx) SetCtx(key string, value interface{}) {
	c.Ctx = context.WithValue(context.Background(), key, value)
}

func (c *CoreCtx) GetConfig(path string, def interface{}) interface{} {
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
