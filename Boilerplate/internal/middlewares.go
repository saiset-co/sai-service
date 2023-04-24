package internal

import (
	"github.com/Limpid-LLC/saiService"
	"log"
)

func (is InternalService) NewMiddlewares() []saiService.Middleware {
	return []saiService.Middleware{
		loggingMiddleware,
		secondMiddleware,
	}
}
func loggingMiddleware(next saiService.HandlerFunc, data interface{}) (interface{}, int, error) {
	log.Println("loggingMiddleware: Request received")
	result, status, err := next(data)
	log.Println("loggingMiddleware: Request processed")
	return result, status, err
}

func secondMiddleware(next saiService.HandlerFunc, data interface{}) (interface{}, int, error) {
	log.Println("secondMiddleware: Request received")
	result, status, err := next(data)
	log.Println("secondMiddleware: Request processed")
	return result, status, err
}
