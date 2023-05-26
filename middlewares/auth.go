package middlewares

import (
	"bytes"
	"encoding/json"
	"github.com/Limpid-LLC/saiService"
	_ "github.com/Limpid-LLC/saiService"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"net/http"
)

type Request struct {
	Microservice string      `json:"microservice"`
	Payload      interface{} `json:"payload"`
}

func CreateAuthMiddleware(authServiceURL string, microserviceName string) func(next saiService.HandlerFunc, data interface{}) (interface{}, int, error) {
	return func(next saiService.HandlerFunc, data interface{}) (interface{}, int, error) {
		if authServiceURL == "" {
			log.Println("authMiddleware: auth service url is empty")
			return unauthorizedResponse()
		}

		authReq := Request{
			Microservice: microserviceName,
			Payload:      data,
		}

		jsonData, err := json.Marshal(authReq)
		if err != nil {
			log.Println("authMiddleware: error marshaling data")
			return unauthorizedResponse()
		}

		req, err := http.NewRequest("POST", authServiceURL, bytes.NewBuffer(jsonData))
		if err != nil {
			log.Println("authMiddleware: error creating request")
			return unauthorizedResponse()
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Println("authMiddleware: error sending request to auth")
			return unauthorizedResponse()
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)

		if err != nil || string(body) != "true" {
			return unauthorizedResponse()
		}

		return next(data)
	}
}

func unauthorizedResponse() (interface{}, int, error) {
	return nil, http.StatusUnauthorized, errors.New("unauthorized")
}
