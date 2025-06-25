package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/saiset-co/sai-service/sai"
	"github.com/saiset-co/sai-service/service"
	"github.com/saiset-co/sai-service/types"
)

// Простая структура ответа
type Response struct {
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
	Status    string `json:"status"`
	RequestID string `json:"request_id,omitempty"`
}

// Структура для POST запросов
type Request struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

func main() {
	ctx := context.Background()

	srv, err := service.NewService(ctx, "config.yaml")
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	sai.Router().Group("").GET("/ping", handlePing)
	sai.Router().Group("").GET("/hello/:name", handleHello)
	sai.Router().Group("").POST("/echo", handleEcho)
	sai.Router().Group("").POST("/data", handleData)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if err := srv.Start(); err != nil {
		log.Fatalf("Service failed: %v", err)
	}
}

// handlePing - самый простой эндпоинт для базовых тестов RPS
func handlePing(ctx *types.RequestCtx) {
	ctx.Response.Header.SetContentType("application/json")
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
	ctx.Response.SetBodyString(`{"Message":"pong","Status":"ok"}`)
}

// handleHello - тест с извлечением параметров
func handleHello(ctx *types.RequestCtx) {
	name := ctx.UserValue("name")

	response := Response{
		Message:   "Hello, " + name.(string) + "!",
		Timestamp: time.Now().Unix(),
		Status:    "ok",
	}

	ctx.WriteJSON(response)
}

// handleEcho - тест с парсингом JSON
func handleEcho(ctx *types.RequestCtx) {
	var req Request
	err := ctx.ReadJSON(&req)
	if err != nil {
		return
	}

	response := Response{
		Message:   "Echo: " + req.Name + " - " + req.Data,
		Timestamp: time.Now().Unix(),
		Status:    "ok",
		RequestID: generateRequestID(),
	}

	ctx.WriteJSON(response)
}

// handleData - тест с большим объемом данных
func handleData(ctx *types.RequestCtx) {
	data := make([]map[string]interface{}, 100)
	for i := 0; i < 100; i++ {
		data[i] = map[string]interface{}{
			"id":        i,
			"name":      "Item " + string(rune(i)),
			"value":     i * 10,
			"timestamp": time.Now(),
			"active":    i%2 == 0,
		}
	}

	response := map[string]interface{}{
		"status": "ok",
		"count":  len(data),
		"data":   data,
	}

	ctx.WriteJSON(response)
}

// generateRequestID - простая генерация ID для трейсинга
func generateRequestID() string {
	return "req_" + string(rune(time.Now().UnixNano()%1000000))
}
