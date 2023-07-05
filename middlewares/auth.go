package middlewares

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/Limpid-LLC/saiService"
	_ "github.com/Limpid-LLC/saiService"
)

type Request struct {
	Method string      `json:"method"`
	Data   RequestData `json:"data"`
}

type RequestData struct {
	Microservice string      `json:"microservice"`
	Method       string      `json:"method"`
	Metadata     interface{} `json:"metadata"`
	Data         interface{} `json:"data"`
}

func CreateAuthMiddleware(authServiceURL string, microserviceName string, method string) func(next saiService.HandlerFunc, data interface{}, metadata interface{}) (interface{}, int, error) {
	return func(next saiService.HandlerFunc, data interface{}, metadata interface{}) (interface{}, int, error) {
		if authServiceURL == "" {
			log.Println("authMiddleware: auth service url is empty")
			return unauthorizedResponse("authServiceURL")
		}

		var dataMap map[string]interface{}

		dataBytes, _ := json.Marshal(data)

		_ = json.Unmarshal(dataBytes, &dataMap)

		if metadata == nil {
			log.Println("authMiddleware: metadata is nil")
			return unauthorizedResponse("empty metadata")
		}

		metadataMap := metadata.(map[string]interface{})

		if metadataMap["token"] == nil {
			log.Println("authMiddleware: metadata token is nil")
			return unauthorizedResponse("empty metadata token")
		}

		dataMap["token"] = metadataMap["token"]

		authReq := Request{
			Method: "check",
			Data: RequestData{
				Microservice: microserviceName,
				Method:       method,
				Data:         dataMap,
			},
		}

		jsonData, err := json.Marshal(authReq)
		if err != nil {
			log.Println("authMiddleware: error marshaling data")
			return unauthorizedResponse("marshaling")
		}

		req, err := http.NewRequest("POST", authServiceURL, bytes.NewBuffer(jsonData))
		if err != nil {
			log.Println("authMiddleware: error creating request")
			return unauthorizedResponse("creating")
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Println("authMiddleware: error sending request to auth")
			return unauthorizedResponse("sending")
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			return unauthorizedResponse("body")
		}

		var res map[string]string
		err = json.Unmarshal(body, &res)
		if err != nil {
			return unauthorizedResponse("Unmarshal")
		}

		if res["result"] != "Ok" {
			return unauthorizedResponse("Result")
		}

		return next(data, metadata)
	}
}

func unauthorizedResponse(info string) (interface{}, int, error) {
	return nil, http.StatusUnauthorized, errors.New("unauthorized:" + info)
}
