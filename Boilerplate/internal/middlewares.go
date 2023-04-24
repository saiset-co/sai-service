package internal

import (
	"github.com/Limpid-LLC/saiService"
)

func (is InternalService) NewMiddlewares() saiService.Handler {
	return saiService.Handler{
		"get": saiService.HandlerElement{
			Name:        "get",
			Description: "Get value from the storage",
			Function: func(data interface{}) (interface{}, int, error) {
				return is.get(data)
			},
		},
		"post": saiService.HandlerElement{
			Name:        "post",
			Description: "Post value to the storage with specified key",
			Function: func(data interface{}) (interface{}, int, error) {
				return is.post(data)
			},
		},
	}
}

func logRequest(data saiService.JsonRequestType, next saiService.HandlerElement) interface{} {
	return next.Function(data)
}
