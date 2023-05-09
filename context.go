package saiService

import (
	"context"
	"strings"
)

const (
	corsAllowOrigin  = "Access-Control-Allow-Origin"
	corsAllowMethods = "Access-Control-Allow-Methods"
	corsAllowHeaders = "Access-Control-Allow-Headers"
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
		case []interface{}:
			is := val.([]interface{})
			if len(is) == 0 {
				continue
			}
			switch is[0].(type) {
			case string:
				strSlice := make([]string, 0)
				for _, v := range is {
					strSlice = append(strSlice, v.(string))
				}
				return strSlice
			case int:
				intSlice := make([]int, 0)
				for _, v := range is {
					intSlice = append(intSlice, v.(int))
				}
				return intSlice
			}

		default:
			return def
		}
	}

	return def
}
