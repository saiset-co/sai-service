package internal

import (
	"github.com/saiset-co/saiService"
	"strconv"
)

func (is InternalService) NewHandler() saiService.Handler {
	return saiService.Handler{
		"get": saiService.HandlerElement{
			Name:        "get",
			Description: "Get value from the storage",
			Function: func(data interface{}) (interface{}, error) {
				return is.get(data), nil
			},
		},
		"post": saiService.HandlerElement{
			Name:        "post",
			Description: "Post value to the storage with specified key",
			Function: func(data interface{}) (interface{}, error) {
				return is.post(data), nil
			},
		},
	}
}

func (is InternalService) get(data interface{}) string {
	return "Get:" + strconv.Itoa(is.Ctx.GetConfig("common.http.port", 80).(int))
}

func (is InternalService) post(data interface{}) string {
	return "Post:" + is.Ctx.GetConfig("common.test", "80").(string) + ":" + data.(string)
}
