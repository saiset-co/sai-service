package internal

import (
	"strconv"

	"github.com/saiset-co/saiService"
)

func (is InternalService) NewHandler() saiService.Handler {
	return saiService.Handler{
		"get": saiService.HandlerElement{
			Name:        "get",
			Description: "Get value from the storage",
			Function: func(data, meta interface{}) (interface{}, int, error) {
				return is.get(data)
			},
		},
		"post": saiService.HandlerElement{
			Name:        "post",
			Description: "Post value to the storage with specified key",
			Function: func(data, meta interface{}) (interface{}, int, error) {
				return is.post(data)
			},
		},
	}
}

func (is InternalService) get(data interface{}) (string, int, error) {
	return "Get:" + strconv.Itoa(is.Context.GetConfig("common.http.port", 80).(int)), 200, nil
}

func (is InternalService) post(data interface{}) (string, int, error) {
	return "Post:" + is.Context.GetConfig("test", "80").(string) + ":" + data.(string), 200, nil
}
