package saiService

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/rs/cors"

	"golang.org/x/net/websocket"
)

func (s *Service) StartHttp() {
	port := s.GetConfig("common.http.port", 8080).(int)
	log.Println("Http server has been started:", port)

	defaultOpts := &cors.Options{}
	corsOpts, err := s.getCorsOptions(defaultOpts)
	if err != nil {
		log.Fatalf("get cors opts from config error: %s", err.Error())
	}

	fmt.Println(corsOpts)

	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleHttpConnections)
	c := cors.New(*corsOpts)
	handler := c.Handler(mux)

	err = http.ListenAndServe(":"+strconv.Itoa(port), handler)

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
