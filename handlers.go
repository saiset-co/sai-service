package saiService

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"reflect"

	"github.com/rs/cors"
	"golang.org/x/net/websocket"
)

type (
	Handler map[string]HandlerElement

	HandlerElement struct {
		Name        string // name to execute, can be path
		Description string
		Function    func(interface{}) (*SaiResponse, error)
	}

	jsonRequestType struct {
		Method  string
		Headers http.Header
		Data    interface{}
	}

	SaiResponse struct {
		Data       interface{} `json:"Data,omitempty"`
		StatusCode int         `json:"StatusCode,omitempty"`
		Headers    http.Header `json:"Headers,omitempty"`
	}

	j map[string]interface{}
)

func (s *Service) handleSocketConnections(conn net.Conn) {
	for {
		var message jsonRequestType
		socketMessage, _ := bufio.NewReader(conn).ReadString('\n')

		if socketMessage != "" {
			_ = json.Unmarshal([]byte(socketMessage), &message)

			if message.Method == "" {
				err := j{"Status": "NOK", "Error": "Wrong message format"}
				errBody, _ := json.Marshal(err)
				log.Println(err)
				conn.Write(append(errBody, eos...))
				continue
			}

			result, resultErr := s.processPath(&message)

			if resultErr != nil {
				err := j{"Status": "NOK", "Error": resultErr.Error()}
				errBody, _ := json.Marshal(err)
				log.Println(err)
				conn.Write(append(errBody, eos...))
				continue
			}

			body, marshalErr := json.Marshal(result)

			if marshalErr != nil {
				err := j{"Status": "NOK", "Error": marshalErr.Error()}
				errBody, _ := json.Marshal(err)
				log.Println(err)
				conn.Write(append(errBody, eos...))
				continue
			}

			conn.Write(append(body, eos...))
		}
	}
}

// handle cli command
func (s *Service) handleCliCommand(data []byte) ([]byte, error) {

	var message jsonRequestType
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data provided")
	}

	err := json.Unmarshal(data, &message)
	if err != nil {
		return nil, err
	}

	if message.Method == "" {
		return nil, fmt.Errorf("empty message method got")

	}

	result, err := s.processPath(&message)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (s *Service) handleWSConnections(conn *websocket.Conn) {
	for {
		message := jsonRequestType{}
		if rErr := websocket.JSON.Receive(conn, &message); rErr != nil {
			err := j{"Status": "NOK", "Error": "Wrong message format"}
			log.Println(err)
			websocket.JSON.Send(conn, err)
			continue
		}

		if message.Method == "" {
			err := j{"Status": "NOK", "Error": "Wrong message format"}
			log.Println(err)
			websocket.JSON.Send(conn, err)
			continue
		}

		message.Headers = conn.Request().Header
		token := message.Headers.Get("Token")
		if s.GetConfig("token", "").(string) != "" {
			if token != s.GetConfig("token", "") {
				err := j{"Status": "NOK", "Error": "Wrong token"}
				log.Println(err)
				websocket.JSON.Send(conn, err)
				continue
			}
		}

		result, resultErr := s.processPath(&message)

		if resultErr != nil {
			err := j{"Status": "NOK", "Error": resultErr.Error()}
			log.Println(err)
			websocket.JSON.Send(conn, err)
			continue
		}

		sErr := websocket.JSON.Send(conn, result)

		if sErr != nil {
			err := j{"Status": "NOK", "Error": sErr.Error()}
			log.Println(err)
			websocket.JSON.Send(conn, err)
		}
	}
}

func (s *Service) handleHttpConnections(resp http.ResponseWriter, req *http.Request) {
	message := jsonRequestType{}
	decoder := json.NewDecoder(req.Body)
	decoderErr := decoder.Decode(&message)

	if decoderErr != nil {
		err := j{"Status": "NOK", "Error": decoderErr.Error()}
		errBody, _ := json.Marshal(err)
		log.Println(err)
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write(errBody)
		return
	}

	if message.Method == "" {
		err := j{"Status": "NOK", "Error": "Wrong message format"}
		errBody, _ := json.Marshal(err)
		log.Println(err)
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write(errBody)
		return
	}

	message.Headers = req.Header
	token := message.Headers.Get("Token")
	if s.GetConfig("common.token", "").(string) != "" {
		if token != s.GetConfig("common.token", "") {
			err := j{"Status": "NOK", "Error": "Wrong token"}
			errBody, _ := json.Marshal(err)
			log.Println(err)
			resp.WriteHeader(http.StatusUnauthorized)
			resp.Write(errBody)
		}
	}

	result, resultErr := s.processPath(&message)

	if resultErr != nil {
		err := j{"Status": "NOK", "Error": resultErr.Error()}
		errBody, _ := json.Marshal(err)
		log.Println(err)
		resp.WriteHeader(result.StatusCode)
		resp.Write(errBody)
		return
	}

	body, marshalErr := json.Marshal(result)

	if marshalErr != nil {
		err := j{"Status": "NOK", "Error": marshalErr.Error()}
		errBody, _ := json.Marshal(err)
		log.Println(err)
		resp.WriteHeader(http.StatusInternalServerError)
		resp.Write(errBody)
		return
	}

	resp.WriteHeader(result.StatusCode)
	resp.Write(body)
}

func (s *Service) processPath(msg *jsonRequestType) (*SaiResponse, error) {
	h, ok := s.Handlers[msg.Method]

	if !ok {
		return nil, errors.New("no handler")
	}

	//todo: Rutina na process

	return h.Function(msg.Data)
}

// get cors options from config
func (s *Service) getCorsOptions(opts *cors.Options) (*cors.Options, error) {
	allowOrigin, ok := s.GetConfig("common.cors", []string{"*"}).([]string)
	if !ok {
		return nil, fmt.Errorf("wrong type of allow origin value from config, value : %s, type : %s", allowOrigin, reflect.TypeOf(allowOrigin))
	}

	allowMethods, ok := s.GetConfig("common.methods", []string{"POST", "GET", "OPTIONS", "DELETE"}).([]string)
	if !ok {
		return nil, fmt.Errorf("wrong type of allow origin value from config, value : %s, type : %s", allowMethods, reflect.TypeOf(allowMethods))
	}

	allowHeaders, ok := s.GetConfig("common.headers", []string{"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization"}).([]string)
	if !ok {
		return nil, fmt.Errorf("wrong type of allow origin value from config, value : %s, type : %s", allowHeaders, reflect.TypeOf(allowHeaders))
	}

	opts.AllowedOrigins = allowOrigin
	opts.AllowedMethods = allowMethods
	opts.AllowedHeaders = allowHeaders

	return opts, nil
}

// return new saiResponse with 200 status code and 'OK' as default
func DefaultSaiResponse() *SaiResponse {
	return &SaiResponse{
		StatusCode: 200,
		Data:       "OK",
	}
}

// set data to saiResponse
func (r *SaiResponse) SetData(data interface{}) {
	r.Data = data
}

// set status code to saiResponse
func (r *SaiResponse) SetStatus(statusCode int) {
	r.StatusCode = statusCode
}

// add header to saiResponse
func (r *SaiResponse) AddHeader(key, value string) {
	r.Headers.Add(key, value)
}

// return saiResponse depends on params count
func NewSaiResponse(params ...interface{}) (*SaiResponse, error) {
	if len(params) == 0 {
		return DefaultSaiResponse(), nil
	}
	resp := &SaiResponse{}

	if len(params) == 1 {
		resp.SetData(params[0])
		resp.SetStatus(http.StatusOK)
		return resp, nil
	}

	if len(params) == 2 {
		resp.SetData(params[0])
		status, ok := params[1].(int)
		if !ok {
			return nil, fmt.Errorf("ReturnSaiResponse - wrong status param, want = int, have = %s", reflect.TypeOf(status).String())
		}
		resp.SetStatus(status)
		return resp, nil
	}

	if len(params) == 3 {
		resp.SetData(params[0])
		status, ok := params[1].(int)
		if !ok {
			return nil, fmt.Errorf("ReturnSaiResponse - wrong status param, want = int, have = %s", reflect.TypeOf(status).String())
		}
		resp.SetStatus(status)
		headers, ok := params[2].(http.Header)
		if !ok {
			return nil, fmt.Errorf("ReturnSaiResponse - wrong headers param, want = http.Header, have = %s", reflect.TypeOf(headers).String())
		}
		resp.Headers = headers
		return resp, nil
	}

	return nil, fmt.Errorf("ReturnSaiResponse - wrong params count, want equal or less than 3, got = %d", len(params))

}
