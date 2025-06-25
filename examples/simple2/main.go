package main

import (
	"context"
	"fmt"
	"github.com/saiset-co/sai-service/types"
	"github.com/valyala/fasthttp"
	"os"

	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/sai"
	"github.com/saiset-co/sai-service/service"
)

func main() {
	mainCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc, err := service.NewService(mainCtx, "config.yml")
	if err != nil {
		fmt.Printf("Failed to create service: %v\n", err)
		os.Exit(1)
	}

	sai.Actions().Subscribe("user.created", func(payload *types.ActionMessage) error {
		sai.Logger().Info("Webhook user.created received")
		return nil
	})

	group := sai.Router().Group("/api/v1")

	group.Route("GET", "/test", func(ctx *types.RequestCtx) {
		data, statusCode, err := sai.ClientManager().Call("simple", "GET", "/api/v1/test", nil, nil)
		if err != nil {
			ctx.Error(err.Error(), statusCode)
		}

		ctx.SetStatusCode(statusCode)
		ctx.Write(data)
	})

	if err := svc.Start(); err != nil {
		sai.Logger().Error("Failed to start service", zap.Error(err))
	}
}
