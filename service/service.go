package service

import (
	"fmt"
	"go.uber.org/zap/zapcore"
	"log"
	"os"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type Service struct {
	Name        string
	Context     *Context
	Handlers    Handler
	Tasks       []func()
	InitTask    func()
	Logger      *zap.Logger
	Middlewares []Middleware
}

var svc = new(Service)
var eos = []byte("\n")

func NewService(name string) *Service {
	svc.Name = name
	svc.Context = NewContext()
	return svc
}

func (s *Service) RegisterConfig(path string) {
	yamlData, err := os.ReadFile(path)

	if err != nil {
		log.Printf("yamlErr:  %v", err)
	}

	err = yaml.Unmarshal(yamlData, &s.Context.Configuration)

	if err != nil {
		log.Fatalf("yamlErr: %v", err)
	}
	svc.SetLogger()
	svc.Context.SetValue("logger", svc.Logger)
}

func (s *Service) RegisterHandlers(handlers Handler) {
	s.Handlers = handlers
}

func (s *Service) RegisterMiddlewares(middlewares []Middleware) {
	s.Middlewares = middlewares
}

func (s *Service) RegisterTasks(tasks []func()) {
	s.Tasks = tasks
}

func (s *Service) RegisterInitTask(initTask func()) {
	s.InitTask = initTask
}

func (s *Service) GetConfig(path string, def interface{}) interface{} {
	return s.Context.GetConfig(path, def)
}

func (s *Service) GetBuild(def string) string {
	buildData, err := os.ReadFile("build.info")

	if err != nil {
		log.Printf("yamlErr:  %v", err)
		return def
	}

	return string(buildData)
}

func (s *Service) Start() {
	if s.InitTask != nil {
		s.InitTask()
	}

	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:  "start",
				Usage: "Start services",
				Action: func(*cli.Context) error {
					s.StartServices()
					return nil
				},
			},
		},
	}

	for method, handler := range s.Handlers {
		command := new(cli.Command)
		command.Name = method
		command.Usage = handler.Description
		command.Action = func(c *cli.Context) error {
			err := s.ExecuteCommand(c.Command.Name, c.Args().Get(0)) // add args
			if err != nil {
				return fmt.Errorf("error while executing command %s : %w", command.Name, err)
			}
			return nil
		}

		app.Commands = append(app.Commands, command)
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func (s *Service) ExecuteCommand(path string, data string) error {
	b := []byte(data)
	result, err := s.handleCliCommand(b)
	if err != nil {
		return err
	}
	fmt.Println(string(result))
	return nil
}

func (s *Service) StartServices() {
	useHttp := s.GetConfig("common.http.enabled", true).(bool)
	useWS := s.GetConfig("common.ws.enabled", true).(bool)

	if useHttp {
		go s.StartHttp()
	}

	if useWS {
		go s.StartWS()
	}

	s.StartTasks()

	log.Printf("%s has been started!", s.Name)

	//s.StartSocket() -- Commented because overload CPU usage

	select {}
}

func (s *Service) StartTasks() {
	for _, task := range s.Tasks {
		go task()
	}
}

func (s *Service) SetLogger() {
	var logger *zap.Logger

	debugMode := s.GetConfig("common.log_mode", "debug")
	switch debugMode {
	case "debug":
		config := zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		logger, _ = config.Build()
	default:
		config := zap.NewProductionConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		logger, _ = config.Build()
	}

	s.Logger = logger
}
