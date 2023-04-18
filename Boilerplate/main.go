package main

import (
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

	svc.Start()

}
