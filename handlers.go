package saiService

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/net/websocket"
	"log"
	"net"
	"net/http"
)

type Handler map[string]HandlerElement

type Middleware func(next HandlerFunc, data interface{}, metadata interface{}) (interface{}, int, error)

type HandlerElement struct {
	Name        string
	Description string
	Function    HandlerFunc
	Middlewares []Middleware
}

type HandlerFunc = func(interface{}, interface{}) (interface{}, int, error)

type JsonRequestType struct {
	Method   string
	Metadata interface{}
	Data     interface{}
}

type ErrorResponse map[string]interface{}

func (s *Service) handleSocketConnections(conn net.Conn) {
	for {
		var message JsonRequestType
		socketMessage, _ := bufio.NewReader(conn).ReadString('\n')

		if socketMessage != "" {
			_ = json.Unmarshal([]byte(socketMessage), &message)

			if message.Method == "" {
				err := ErrorResponse{"Status": "NOK", "Error": "Wrong message format"}
				errBody, _ := json.Marshal(err)
				log.Println(err)
				conn.Write(append(errBody, eos...))
				continue
			}

			result, _, resultErr := s.processPath(&message)

			if resultErr != nil {
				err := ErrorResponse{"Status": "NOK", "Error": resultErr.Error()}
				errBody, _ := json.Marshal(err)
				log.Println(err)
				conn.Write(append(errBody, eos...))
				continue
			}

			body, marshalErr := json.Marshal(result)

			if marshalErr != nil {
				err := ErrorResponse{"Status": "NOK", "Error": marshalErr.Error()}
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

	var message JsonRequestType
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

	result, _, err := s.processPath(&message)
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
		var message JsonRequestType
		if rErr := websocket.JSON.Receive(conn, &message); rErr != nil {
			err := ErrorResponse{"Status": "NOK", "Error": "Wrong message format"}
			log.Println(err)
			websocket.JSON.Send(conn, err)
			continue
		}

		if message.Method == "" {
			err := ErrorResponse{"Status": "NOK", "Error": "Wrong message format"}
			log.Println(err)
			websocket.JSON.Send(conn, err)
			continue
		}

		headers := conn.Request().Header
		token := headers.Get("Token")
		if s.GetConfig("token", "").(string) != "" {
			if token != s.GetConfig("token", "") {
				err := ErrorResponse{"Status": "NOK", "Error": "Wrong token"}
				log.Println(err)
				websocket.JSON.Send(conn, err)
				continue
			}
		}

		result, _, resultErr := s.processPath(&message)

		if resultErr != nil {
			err := ErrorResponse{"Status": "NOK", "Error": resultErr.Error()}
			log.Println(err)
			websocket.JSON.Send(conn, err)
			continue
		}

		sErr := websocket.JSON.Send(conn, result)

		if sErr != nil {
			err := ErrorResponse{"Status": "NOK", "Error": sErr.Error()}
			log.Println(err)
			websocket.JSON.Send(conn, err)
		}
	}
}

func (s *Service) healthCheck(resp http.ResponseWriter, req *http.Request) {
	data := map[string]interface{}{"Status": "OK"}
	body, _ := json.Marshal(data)
	resp.WriteHeader(http.StatusOK)
	resp.Write(body)
	return
}

func (s *Service) versionCheck(resp http.ResponseWriter, req *http.Request) {
	data := map[string]interface{}{
		"Version": s.GetConfig("common.version", "0.1").(string),
		"Built":   s.GetBuild("no build date"),
	}
	body, _ := json.Marshal(data)
	resp.WriteHeader(http.StatusOK)
	resp.Write(body)
	return
}

func (s *Service) handleHttpConnections(resp http.ResponseWriter, req *http.Request) {
	var message JsonRequestType
	decoder := json.NewDecoder(req.Body)
	decoderErr := decoder.Decode(&message)

	resp.Header().Set("Content-Type", "application/json")

	if decoderErr != nil {
		err := ErrorResponse{"Status": "NOK", "Error": decoderErr.Error()}
		errBody, _ := json.Marshal(err)
		log.Println(err)
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write(errBody)
		return
	}

	if message.Method == "" {
		err := ErrorResponse{"Status": "NOK", "Error": "Wrong message format"}
		errBody, _ := json.Marshal(err)
		log.Println(err)
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write(errBody)
		return
	}

	headers := req.Header
	token := headers.Get("Token")
	if s.GetConfig("common.token", "").(string) != "" {
		if token != s.GetConfig("common.token", "") {
			err := ErrorResponse{"Status": "NOK", "Error": "Wrong token"}
			errBody, _ := json.Marshal(err)
			log.Println(err)
			resp.WriteHeader(http.StatusUnauthorized)
			resp.Write(errBody)
		}
	}

	result, statusCode, resultErr := s.processPath(&message)

	if resultErr != nil {
		err := ErrorResponse{"Status": "NOK", "Error": resultErr.Error()}
		errBody, _ := json.Marshal(err)
		log.Println(err)
		resp.WriteHeader(statusCode)
		resp.Write(errBody)
		return
	}

	body, marshalErr := json.Marshal(result)

	if marshalErr != nil {
		err := ErrorResponse{"Status": "NOK", "Error": marshalErr.Error()}
		errBody, _ := json.Marshal(err)
		log.Println(err)
		resp.WriteHeader(http.StatusInternalServerError)
		resp.Write(errBody)
		return
	}
	resp.WriteHeader(statusCode)
	resp.Write(body)
}

func (s *Service) applyMiddleware(handler HandlerElement, data interface{}, metadata interface{}) (interface{}, int, error) {
	closures := make([]HandlerFunc, len(s.Middlewares)+len(handler.Middlewares)+1)
	closures[0] = handler.Function

	// Function to create a closure for the middleware with the correct next function
	createMiddlewareClosure := func(middleware Middleware, next HandlerFunc) HandlerFunc {
		return func(data interface{}, metadata interface{}) (interface{}, int, error) {
			return middleware(next, data, metadata)
		}
	}

	last := closures[0]

	// Apply global middlewares
	for _, middleware := range s.Middlewares {
		newClosure := createMiddlewareClosure(middleware, last)
		last = newClosure
		closures = append(closures, newClosure)
	}

	// Apply local middlewares
	for _, middleware := range handler.Middlewares {
		newClosure := createMiddlewareClosure(middleware, last)
		last = newClosure
		closures = append(closures, newClosure)
	}

	return last(data, metadata)
}

func (s *Service) processPath(msg *JsonRequestType) (interface{}, int, error) {
	h, ok := s.Handlers[msg.Method]

	if !ok {
		return nil, http.StatusNotFound, errors.New("no handler")
	}

	//todo: Rutina na process

	// Apply middleware
	return s.applyMiddleware(h, msg.Data, msg.Metadata)
}
