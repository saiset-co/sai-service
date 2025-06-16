package service

import (
	"context"
	"fmt"
	"github.com/saiset-co/sai-service/tls"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"

	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/action"
	"github.com/saiset-co/sai-service/cache"
	"github.com/saiset-co/sai-service/client"
	"github.com/saiset-co/sai-service/config"
	"github.com/saiset-co/sai-service/cron"
	"github.com/saiset-co/sai-service/documentations"
	"github.com/saiset-co/sai-service/health"
	"github.com/saiset-co/sai-service/logger"
	"github.com/saiset-co/sai-service/metrics"
	"github.com/saiset-co/sai-service/middleware"
	"github.com/saiset-co/sai-service/sai"
	"github.com/saiset-co/sai-service/server"
	"github.com/saiset-co/sai-service/types"
)

type Service struct {
	ctx        context.Context
	cancel     context.CancelFunc
	configPath string
	done       chan struct{}
	wg         sync.WaitGroup
	running    int32
}

func NewService(ctx context.Context, configPath string) (*Service, error) {
	if configPath == "" {
		return nil, types.ErrConfigInvalidPath
	}

	_, err := os.Stat(configPath)
	if err != nil {
		return nil, types.WrapError(err, "file does not exist")
	}

	serviceCtx, cancel := context.WithCancel(ctx)
	container := sai.InitContainer()

	service := &Service{
		ctx:        serviceCtx,
		cancel:     cancel,
		configPath: configPath,
		done:       make(chan struct{}),
		running:    0,
	}

	if err := registerProviders(container, ctx, configPath); err != nil {
		cancel()
		return nil, types.WrapError(err, "failed to register providers")
	}

	sai.SetContainer(container)

	return service, nil
}

func (s *Service) Run() error {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		sai.Logger().Warn("Service is already running")
		return types.ErrServerAlreadyRunning
	}

	var runErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				runErr = fmt.Errorf("%v", r)
				sai.Logger().Error("Service run", zap.Stack(string(buf[:n])))
				atomic.StoreInt32(&s.running, 0)
			}
		}()

		runErr = s.run()
	}()

	return runErr
}

func (s *Service) run() error {
	sai.Logger().Info("Starting service")

	if err := s.startComponents(); err != nil {
		atomic.StoreInt32(&s.running, 0)
		return types.WrapError(err, "failed to start components")
	}

	s.setupSignalHandling()

	s.wg.Add(1)
	go s.contextMonitor()

	sai.Logger().Info("Service started successfully")

	<-s.done

	if err := s.stopComponents(); err != nil {
		sai.Logger().Error("Error during service shutdown", zap.Error(err))
	}

	s.wg.Wait()

	atomic.StoreInt32(&s.running, 0)

	sai.Logger().Info("Service stopped gracefully")
	return nil
}

func (s *Service) Stop() error {
	if !atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		sai.Logger().Warn("Service is not running")
		return types.ErrServiceIsNotRunning
	}

	sai.Logger().Info("Stopping service...")

	s.cancel()

	return nil
}

func (s *Service) Done() <-chan struct{} {
	return s.done
}

func (s *Service) Cancel() {
	s.cancel()
}

func (s *Service) Context() context.Context {
	return s.ctx
}

func (s *Service) IsRunning() bool {
	return atomic.LoadInt32(&s.running) == 1
}

func (s *Service) startComponents() error {
	_config := sai.Config().GetConfig()

	if _config.Health.Enabled {
		if err := sai.Health().Start(); err != nil {
			sai.Logger().Error("Failed to start health manager", zap.Error(err))
		}
	}

	if _config.Middlewares.Enabled {
		if err := sai.Middlewares().RegisterMiddlewares(); err != nil {
			return fmt.Errorf("failed to register middlewares: %w", err)
		}
	}

	if _config.Docs.Enabled {
		if err := sai.Documentation().Start(); err != nil {
			sai.Logger().Error("Failed to generate documentation", zap.Error(err))
		}
	}

	if _config.Metrics.Enabled {
		if err := sai.Metrics().Start(); err != nil {
			sai.Logger().Error("Failed to start metrics manager", zap.Error(err))
		}
	}

	if _config.Actions.Enabled {
		if err := sai.Actions().Start(); err != nil {
			sai.Logger().Error("Failed to start action broker", zap.Error(err))
		}
	}

	if _config.Server.TLS.Enabled {
		if err := sai.TLSManager().Start(); err != nil {
			sai.Logger().Error("Failed to start TLS manager", zap.Error(err))
		}
	}

	if err := sai.Router().FinalizePendingRoutes(); err != nil {
		return types.WrapError(err, "failed to finalize routes")
	}

	if err := sai.HTTPServer().Start(); err != nil {
		return types.WrapError(err, "failed to start HTTP server")
	}

	if _config.Cron.Enabled {
		if err := sai.Cron().Start(); err != nil {
			sai.Logger().Error("Failed to start cron manager", zap.Error(err))
		}
	}

	if _config.Cache.Enabled {
		if err := sai.Cache().Start(); err != nil {
			sai.Logger().Error("Failed to start cache manager", zap.Error(err))
		}
	}

	if _config.Client.Enabled {
		if err := sai.ClientManager().Start(); err != nil {
			sai.Logger().Error("Failed to start client manager", zap.Error(err))
		}
	}

	sai.Logger().Info("All components started successfully")

	return nil
}

func (s *Service) stopComponents() error {
	var _errors []error
	_config := sai.Config().GetConfig()

	sai.Logger().Info("Stopping service components...")

	if _config.Actions.Enabled && sai.Actions() != nil {
		if err := sai.Actions().Stop(); err != nil {
			sai.Logger().Error("Failed to stop action broker", zap.Error(err))
			_errors = append(_errors, err)
		}
	}

	if _config.Cron.Enabled && sai.Cron() != nil {
		if err := sai.Cron().Stop(); err != nil {
			sai.Logger().Error("Failed to stop cron manager", zap.Error(err))
			_errors = append(_errors, err)
		}
	}

	if sai.HTTPServer() != nil {
		if err := sai.HTTPServer().Stop(); err != nil {
			sai.Logger().Error("Failed to stop HTTP server", zap.Error(err))
			_errors = append(_errors, err)
		}
	}

	if _config.Server.TLS.Enabled && sai.TLSManager() != nil {
		if err := sai.TLSManager().Stop(); err != nil {
			sai.Logger().Error("Failed to stop TLS manager", zap.Error(err))
			_errors = append(_errors, err)
		}
	}

	if _config.Client.Enabled && sai.ClientManager() != nil {
		if err := sai.ClientManager().Stop(); err != nil {
			sai.Logger().Error("Failed to stop client manager", zap.Error(err))
			_errors = append(_errors, err)
		}
	}

	if _config.Cache.Enabled && sai.Cache() != nil {
		if err := sai.Cache().Stop(); err != nil {
			sai.Logger().Error("Failed to stop cache", zap.Error(err))
			_errors = append(_errors, err)
		}
	}

	if _config.Metrics.Enabled && sai.Metrics() != nil {
		if err := sai.Metrics().Stop(); err != nil {
			sai.Logger().Error("Failed to stop metrics manager", zap.Error(err))
			_errors = append(_errors, err)
		}
	}

	if _config.Health.Enabled && sai.Health() != nil {
		if err := sai.Health().Stop(); err != nil {
			sai.Logger().Error("Failed to stop health manager", zap.Error(err))
			_errors = append(_errors, err)
		}
	}

	if _config.Docs.Enabled && sai.Documentation() != nil {
		if err := sai.Documentation().Stop(); err != nil {
			sai.Logger().Error("Failed to stop documentation manager", zap.Error(err))
			_errors = append(_errors, err)
		}
	}

	if _config.Middlewares.Enabled && sai.Middlewares() != nil {
		sai.Middlewares().Clear()
	}

	if len(_errors) > 0 {
		return types.NewErrorf("errors during shutdown: %v", _errors)
	}

	sai.Logger().Info("All components stopped successfully")

	return nil
}

func (s *Service) setupSignalHandling() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		select {
		case sig := <-sigChan:
			sai.Logger().Info("Received shutdown signal", zap.String("signal", sig.String()))
			s.cancel()

		case <-s.ctx.Done():
			sai.Logger().Info("Service context cancelled")
		}
	}()
}

func (s *Service) contextMonitor() {
	defer s.wg.Done()
	defer close(s.done)

	<-s.ctx.Done()

	switch err := s.ctx.Err(); {
	case types.IsError(err, context.Canceled):
		sai.Logger().Info("Service shutdown: context cancelled")
	case types.IsError(err, context.DeadlineExceeded):
		sai.Logger().Warn("Service shutdown: context deadline exceeded")
	default:
		sai.Logger().Info("Service shutdown: context done")
	}
}

func registerProviders(container *sai.Container, ctx context.Context, configPath string) error {
	var metricsManager types.MetricsManager
	var cacheManager types.CacheManager
	var middlewareManager types.MiddlewareManager
	var healthManager types.HealthManager
	var tlsManager types.TLSManager

	configManager, err := config.NewConfigurationManager(configPath)
	if err != nil {
		return types.WrapError(err, "failed to register config manager")
	}
	container.SetConfig(configManager)

	_config := configManager.GetConfig()

	loggerManager, err := logger.NewLogger(configManager)
	if err != nil {
		return types.WrapError(err, "failed to register logger")
	}
	container.SetLogger(loggerManager)

	router, err := server.NewFastHTTPRouter()
	if err != nil {
		return types.WrapError(err, "failed to register router")
	}
	container.SetRouter(router)

	if _config.Health.Enabled {
		healthManager, err = health.NewManager(ctx, configManager, loggerManager)
		if err != nil {
			return types.WrapError(err, "failed to register health manager")
		}
		healthManager.RegisterRoutes(router)
		container.SetHealth(healthManager)
	}

	if _config.Metrics.Enabled {
		metricsManager, err = metrics.NewMetricsManager(ctx, configManager, loggerManager, healthManager)
		if err != nil {
			return types.WrapError(err, "failed to register metrics manager")
		}
		metricsManager.RegisterRoutes(router)
		container.SetMetrics(metricsManager)
	}

	if _config.Server.TLS.Enabled {
		tlsManager, err = tls.NewCertManager(ctx, loggerManager, configManager)
		if err != nil {
			return types.WrapError(err, "failed to register TLS manager")
		}
		container.SetTLSManager(tlsManager)
		//registerTLSRoutes(router, tlsManager, loggerManager)
	}

	if _config.Cron.Enabled {
		cronManager, err := cron.NewManager(ctx, configManager, loggerManager, metricsManager, healthManager)
		if err != nil {
			return types.WrapError(err, "failed to register cron manager")
		}
		container.SetCron(cronManager)
	}

	if _config.Actions.Enabled {
		actionBroker, err := action.NewActionBroker(ctx, configManager, loggerManager, metricsManager, healthManager)
		if err != nil {
			return types.WrapError(err, "failed to register action broker")
		}
		actionBroker.RegisterRoutes(router)
		container.SetActions(actionBroker)
	}

	if _config.Docs.Enabled {
		documentationManager, err := documentations.NewDocumentationManager(configManager, loggerManager, healthManager, router)
		if err != nil {
			return types.WrapError(err, "failed to register documentation manager")
		}
		documentationManager.RegisterRoutes(router)
		container.SetDocumentation(documentationManager)
	}

	if _config.Client.Enabled {
		clientManager, err := client.NewManager(ctx, configManager, loggerManager, metricsManager, healthManager)
		if err != nil {
			return types.WrapError(err, "failed to register client manager")
		}
		container.SetClientManager(clientManager)
	}

	if _config.Cache.Enabled {
		cacheManager, err = cache.NewCacheManager(ctx, configManager, loggerManager, metricsManager, healthManager)
		if err != nil {
			return types.WrapError(err, "failed to register cache manager")
		}
		container.SetCache(cacheManager)
	}

	if _config.Middlewares.Enabled {
		middlewareManager, err = middleware.NewManager(ctx, configManager, loggerManager, metricsManager, cacheManager, healthManager)
		if err != nil {
			return types.WrapError(err, "failed to register middleware manager")
		}
		container.SetMiddlewares(middlewareManager)
	}

	httpServer, err := server.NewHTTPServer(ctx, configManager, loggerManager, metricsManager, middlewareManager, tlsManager, router)
	if err != nil {
		return types.WrapError(err, "failed to register HTTP server")
	}

	container.SetHTTPServer(httpServer)

	return nil
}
