package saiService

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"

	"golang.org/x/net/websocket"
)

type Handler map[string]HandlerElement

type HandlerElement struct {
	Name        string // name to execute, can be path
	Description string
	Function    func(interface{}) (interface{}, int, error)
}

type jsonRequestType struct {
	Method string
	Data   interface{}
}

type j map[string]interface{}

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

			result, _, resultErr := s.processPath(&message)

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
		var message jsonRequestType
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

		headers := conn.Request().Header
		token := headers.Get("Token")
		if s.GetConfig("token", "").(string) != "" {
			if token != s.GetConfig("token", "") {
				err := j{"Status": "NOK", "Error": "Wrong token"}
				log.Println(err)
				websocket.JSON.Send(conn, err)
				continue
			}
		}

		result, _, resultErr := s.processPath(&message)

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
	var message jsonRequestType
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

	headers := req.Header
	token := headers.Get("Token")
	if s.GetConfig("common.token", "").(string) != "" {
		if token != s.GetConfig("common.token", "") {
			err := j{"Status": "NOK", "Error": "Wrong token"}
			errBody, _ := json.Marshal(err)
			log.Println(err)
			resp.WriteHeader(http.StatusUnauthorized)
			resp.Write(errBody)
		}
	}

	result, statusCode, resultErr := s.processPath(&message)

	if resultErr != nil {
		err := j{"Status": "NOK", "Error": resultErr.Error()}
		errBody, _ := json.Marshal(err)
		log.Println(err)
		resp.WriteHeader(statusCode)
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
	resp.WriteHeader(statusCode)
	resp.Write(body)
}

func (s *Service) processPath(msg *jsonRequestType) (interface{}, int, error) {
	h, ok := s.Handlers[msg.Method]

	if !ok {
		return nil, http.StatusNotFound, errors.New("no handler")
	}

	//todo: Rutina na process

	return h.Function(msg.Data)
}
