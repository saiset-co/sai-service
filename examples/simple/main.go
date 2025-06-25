package main

import (
	"context"
	"fmt"
	"github.com/saiset-co/sai-service/sai"
	"github.com/saiset-co/sai-service/service"
	"github.com/saiset-co/sai-service/types"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"os"
)

func main() {
	mainCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc, err := service.NewService(mainCtx, "config.yml")
	if err != nil {
		fmt.Printf("Failed to create service: %v\n", err)
		os.Exit(1)
	}

	value := sai.Config().GetValue("blabla", 0)
	sai.Logger().Info("Config", zap.Any("value", value))

	group := sai.Router().Group("/api/v1")

	var response string

	group.Route("GET", "test", handleRoute).
		WithDoc("Main", "Main endpoint", "Tag", nil, &response)

	if err := svc.Start(); err != nil {
		sai.Logger().Error("Failed to start service", zap.Error(err))
	}
}

func handleRoute(ctx *types.RequestCtx) {
	ctx.Write([]byte("aaaaa"))
	sai.Actions().Publish("user.created", "aaaaa")

}
