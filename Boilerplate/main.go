package main

import (
	"fmt"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/saiset-co/Boilerplate/internal"
	"github.com/saiset-co/saiService"
)

func main() {
	svc := saiService.NewService("{project_name}")
	is := internal.InternalService{Context: svc.Context}

	svc.RegisterConfig("config.yml")

	svc.RegisterInitTask(is.Init)

	svc.RegisterTasks([]func(){
		is.Process,
	})

	svc.RegisterHandlers(
		is.NewHandler(),
	)

	if svc.Context.GetConfig("common.profiling", true).(bool) {
		mr := gin.Default()
		pprof.Register(mr)
		go mr.Run(fmt.Sprintf(":%d", svc.Context.GetConfig("common.profiling_port", 8082)))
	}

	svc.Start()

}
