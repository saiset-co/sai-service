package main

import (
	"Boilerplate/internal"
	"github.com/Limpid-LLC/saiService"
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

	svc.RegisterMiddlewares(
		is.NewMiddlewares(),
	)

	svc.Start()

}
