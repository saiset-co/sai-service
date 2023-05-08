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
			Function: func(data interface{}) (*saiService.SaiResponse, error) {
				return is.get(data)

			},
		},
		"post": saiService.HandlerElement{
			Name:        "post",
			Description: "Post value to the storage with specified key",
			Function: func(data interface{}) (*saiService.SaiResponse, error) {
				return is.post(data)
			},
		},
	}
}

func (is InternalService) get(data interface{}) (*saiService.SaiResponse, error) {
	resp := saiService.NewSaiResponse()
	resp.SetData("Get:" + strconv.Itoa(is.Context.GetConfig("common.http.port", 80).(int)))
	return resp, nil
}

func (is InternalService) post(data interface{}) (*saiService.SaiResponse, error) {
	resp := saiService.NewSaiResponse()
	resp.SetData("Post:" + strconv.Itoa(is.Context.GetConfig("common.http.port", 80).(int)))
	return resp, nil
}
