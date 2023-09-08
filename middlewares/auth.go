package middlewares

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/saiset-co/saiService"
	_ "github.com/saiset-co/saiService"
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
			log.Println("authMiddleware: " + err.Error())
			return unauthorizedResponse("marshaling -> " + err.Error())
		}

		req, err := http.NewRequest("POST", authServiceURL, bytes.NewBuffer(jsonData))
		if err != nil {
			log.Println("authMiddleware: error creating request")
			log.Println("authMiddleware: " + err.Error())
			return unauthorizedResponse("creating request -> " + err.Error())
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Println("authMiddleware: error sending request to auth")
			log.Println("authMiddleware: " + err.Error())
			return unauthorizedResponse("sending request -> " + err.Error())
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			log.Println("authMiddleware: error reading body from auth")
			log.Println("authMiddleware: " + err.Error())
			return unauthorizedResponse("reading body -> " + err.Error())
		}

		var res map[string]string
		err = json.Unmarshal(body, &res)
		if err != nil {
			log.Println("authMiddleware: error unmarshalling body from auth")
			log.Println("authMiddleware: " + err.Error())
			return unauthorizedResponse("Unmarshal -> " + err.Error())
		}

		if res["result"] != "Ok" {
			log.Println("authMiddleware: response-body -> result is not `Ok`")
			log.Println("authMiddleware: " + string(body))
			return unauthorizedResponse("Result -> " + string(body))
		}

		return next(data, metadata)
	}
}

func unauthorizedResponse(info string) (interface{}, int, error) {
	return nil, http.StatusUnauthorized, errors.New("unauthorized:" + info)
}
