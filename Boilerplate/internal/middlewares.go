package internal

import (
	"github.com/Limpid-LLC/saiService"
	"log"
)

func (is InternalService) NewMiddlewares() []saiService.Middleware {
	return []saiService.Middleware{
		loggingMiddleware,
	}
}
func loggingMiddleware(next saiService.HandlerFunc, data interface{}) (interface{}, int, error) {
	log.Println("Request received")
	result, status, err := next(data)
	log.Println("Request processed")
	return result, status, err
}
