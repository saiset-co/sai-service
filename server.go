package saiService

import (
	"log"
	"net"
	"net/http"
	"strconv"

	"golang.org/x/net/websocket"
)

func (s *Service) StartHttp() {
	port := s.GetConfig("common.http.port", 8080).(int)
	log.Println("Http server has been started:", port)

	http.HandleFunc("/", s.handleHttpConnections)

	err := http.ListenAndServe(":"+strconv.Itoa(port), nil)

	if err != nil {
		log.Println("Http server error: ", err)
	}
}

func (s *Service) StartWS() {
	port := s.GetConfig("common.ws.port", 8081).(int)
	log.Println("WS server has been started:", port)

	r := http.NewServeMux()

	r.Handle("/ws", websocket.Handler(s.handleWSConnections))

	err := http.ListenAndServe(":"+strconv.Itoa(port), r)

	if err != nil {
		log.Println("WS server error: ", err)
	}
}

func (s *Service) StartSocket() {
	port := s.GetConfig("common.socket.port", 8000).(int)
	log.Println("Socket server has been started:", port)

	ln, nErr := net.Listen("tcp", ":"+strconv.Itoa(port))

	if nErr != nil {
		log.Fatalf("networkErr: %v", nErr)
	}

	conn, cErr := ln.Accept()

	if cErr != nil {
		log.Fatalf("networkErr: %v", cErr)
	}

	s.handleSocketConnections(conn)
}
