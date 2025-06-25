package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/action"
	"github.com/saiset-co/sai-service/auth_providers"
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
	"github.com/saiset-co/sai-service/tls"
	"github.com/saiset-co/sai-service/types"
)

type State int32

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
)

type Service struct {
	ctx             context.Context
	cancel          context.CancelFunc
	configPath      string
	done            chan struct{}
	wg              sync.WaitGroup
	state           atomic.Value
	shutdownTimeout time.Duration
	startTimeout    time.Duration
	container       *sai.Container
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
		ctx:             serviceCtx,
		cancel:          cancel,
		configPath:      configPath,
		container:       container,
		done:            make(chan struct{}),
		shutdownTimeout: 30 * time.Second,
		startTimeout:    60 * time.Second,
	}

	service.state.Store(StateStopped)

	if err := registerProviders(container, ctx, configPath); err != nil {
		cancel()
		return nil, types.WrapError(err, "failed to register providers")
	}

	sai.SetContainer(container)
	return service, nil
}

func (s *Service) Start() error {
	if !s.transitionState(StateStopped, StateStarting) {
		sai.Logger().Warn("Service is already running")
		return types.ErrServerAlreadyRunning
	}

	var runErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				runErr = fmt.Errorf("service panic: %v", r)
				sai.Logger().Error("Service run panic", zap.Stack(string(buf[:n])))
				s.setState(StateStopped)
			}
		}()

		runErr = s.run()
	}()

	return runErr
}

func (s *Service) run() error {
	sai.Logger().Info("Starting service")

	ctx, cancel := context.WithTimeout(s.ctx, s.startTimeout)
	defer cancel()

	if err := s.startComponents(ctx); err != nil {
		s.setState(StateStopped)
		return types.WrapError(err, "failed to start components")
	}

	s.setState(StateRunning)
	s.setupSignalHandling()

	s.wg.Add(1)
	go s.contextMonitor()

	sai.Logger().Info("Service started successfully")

	<-s.done

	if err := s.stopComponents(); err != nil {
		sai.Logger().Error("Error during service shutdown", zap.Error(err))
	}

	s.wg.Wait()
	s.setState(StateStopped)

	sai.Logger().Info("Service stopped gracefully")
	return nil
}

func (s *Service) Stop() error {
	if !s.transitionState(StateRunning, StateStopping) {
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
	return s.getState() == StateRunning
}

func (s *Service) getState() State {
	return s.state.Load().(State)
}

func (s *Service) setState(newState State) bool {
	currentState := s.getState()
	return s.state.CompareAndSwap(currentState, newState)
}

func (s *Service) transitionState(from, to State) bool {
	return s.state.CompareAndSwap(from, to)
}

func (s *Service) startComponents(ctx context.Context) error {
	_config := sai.Config().GetConfig()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		if ptr := s.container.Config.Load(); ptr != nil {
			manager := (*ptr).(types.LifecycleManager)
			if err := manager.Start(); err != nil {
				return types.WrapError(err, "failed to start config manager")
			}
		}
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		if ptr := s.container.Logger.Load(); ptr != nil {
			manager := (*ptr).(types.LifecycleManager)
			if err := manager.Start(); err != nil {
				return types.WrapError(err, "failed to start logger")
			}
		}
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		if ptr := s.container.AuthProvider.Load(); ptr != nil {
			manager := (*ptr).(types.LifecycleManager)
			if err := manager.Start(); err != nil {
				sai.Logger().Error("Failed to start auth provider", zap.Error(err))
			}
		}
	}

	if _config.Health.Enabled {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if ptr := s.container.Health.Load(); ptr != nil {
				manager := (*ptr).(types.LifecycleManager)
				if err := manager.Start(); err != nil {
					sai.Logger().Error("Failed to start health manager", zap.Error(err))
				}
			}
		}
	}

	if _config.Middlewares.Enabled {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if ptr := s.container.Middlewares.Load(); ptr != nil {
				manager := (*ptr).(types.LifecycleManager)
				if err := manager.Start(); err != nil {
					sai.Logger().Error("Failed to start middleware manager", zap.Error(err))
				}
			}
		}
	}

	g, gCtx := errgroup.WithContext(ctx)

	if _config.Docs.Enabled {
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if ptr := s.container.Documentation.Load(); ptr != nil {
					manager := (*ptr).(types.LifecycleManager)
					if err := manager.Start(); err != nil {
						sai.Logger().Error("Failed to start documentation manager", zap.Error(err))
					}
				}
				return nil
			}
		})
	}

	if _config.Metrics.Enabled {
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if ptr := s.container.Metrics.Load(); ptr != nil {
					manager := (*ptr).(types.LifecycleManager)
					if err := manager.Start(); err != nil {
						sai.Logger().Error("Failed to start metrics manager", zap.Error(err))
					}
				}
				return nil
			}
		})
	}

	if _config.Cache.Enabled {
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if ptr := s.container.Cache.Load(); ptr != nil {
					manager := (*ptr).(types.LifecycleManager)
					if err := manager.Start(); err != nil {
						sai.Logger().Error("Failed to start cache manager", zap.Error(err))
					}
				}
				return nil
			}
		})
	}

	if _config.Server.TLS.Enabled {
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if ptr := s.container.TLSManager.Load(); ptr != nil {
					manager := (*ptr).(types.LifecycleManager)
					if err := manager.Start(); err != nil {
						sai.Logger().Error("Failed to start tls manager", zap.Error(err))
					}
				}
				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			return types.NewErrorf("component startup timeout: %v", ctx.Err())
		default:
			return err
		}
	}

	if _config.Client.Enabled {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if ptr := s.container.ClientManager.Load(); ptr != nil {
				if err := (*ptr).(types.LifecycleManager).Start(); err != nil {
					sai.Logger().Error("Failed to start client manager", zap.Error(err))
				}
			}
		}
	}

	if _config.Actions != nil && (_config.Actions.Broker != nil || _config.Actions.Webhooks != nil) && _config.Actions.Broker.Enabled {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if ptr := s.container.Actions.Load(); ptr != nil {
				if err := (*ptr).(types.LifecycleManager).Start(); err != nil {
					sai.Logger().Error("Failed to start action dispatcher", zap.Error(err))
				}
			}
		}
	}

	if ptr := s.container.Router.Load(); ptr != nil {
		if err := (*ptr).(types.LifecycleManager).Start(); err != nil {
			return types.WrapError(err, "failed to start router")
		}
	}

	if ptr := s.container.HTTPServer.Load(); ptr != nil {
		_server := *ptr
		if err := _server.Start(); err != nil {
			sai.Logger().Error("Failed to start HTTP server", zap.Error(err))
		}
	}

	if _config.Cron.Enabled {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if ptr := s.container.Cron.Load(); ptr != nil {
				if err := (*ptr).(types.LifecycleManager).Start(); err != nil {
					sai.Logger().Error("Failed to start cron manager", zap.Error(err))
				}
			}
		}
	}

	sai.Logger().Info("All components started successfully")
	return nil
}

func (s *Service) stopComponents() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	var errors []error

	sai.Logger().Info("Stopping service components...")

	g, gCtx := errgroup.WithContext(ctx)

	if ptr := s.container.Cron.Load(); ptr != nil {
		manager := (*ptr).(types.LifecycleManager)
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if err := manager.Stop(); err != nil {
					sai.Logger().Error("Failed to stop action dispatcher", zap.Error(err))
					return err
				}
				return nil
			}
		})
	}

	if ptr := s.container.Cron.Load(); ptr != nil {
		manager := (*ptr).(types.LifecycleManager)
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if err := manager.Stop(); err != nil {
					sai.Logger().Error("Failed to stop cron manager", zap.Error(err))
					return err
				}
				return nil
			}
		})
	}

	if ptr := s.container.ClientManager.Load(); ptr != nil {
		manager := (*ptr).(types.LifecycleManager)
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if err := manager.Stop(); err != nil {
					sai.Logger().Error("Failed to stop client manager", zap.Error(err))
					return err
				}
				return nil
			}
		})
	}

	if ptr := s.container.Documentation.Load(); ptr != nil {
		manager := (*ptr).(types.LifecycleManager)
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if err := manager.Stop(); err != nil {
					sai.Logger().Error("Failed to stop documentation manager", zap.Error(err))
					return err
				}
				return nil
			}
		})
	}

	if ptr := s.container.Middlewares.Load(); ptr != nil {
		manager := (*ptr).(types.LifecycleManager)
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if err := manager.Stop(); err != nil {
					sai.Logger().Error("Failed to stop middleware manager", zap.Error(err))
					return err
				}
				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			sai.Logger().Warn("Component shutdown timeout, some components may not have stopped gracefully")
		default:
			errors = append(errors, err)
		}
	}

	if ptr := s.container.AuthProvider.Load(); ptr != nil {
		manager := (*ptr).(types.LifecycleManager)
		if err := manager.Stop(); err != nil {
			sai.Logger().Error("Failed to stop auth provider", zap.Error(err))
			errors = append(errors, err)
		}
	}

	if ptr := s.container.Router.Load(); ptr != nil {
		if err := (*ptr).(types.LifecycleManager).Stop(); err != nil {
			return types.WrapError(err, "failed to start router")
		}
	}

	if ptr := s.container.HTTPServer.Load(); ptr != nil {
		manager := (*ptr).(types.LifecycleManager)
		if err := manager.Stop(); err != nil {
			sai.Logger().Error("Failed to stop HTTP server", zap.Error(err))
			errors = append(errors, err)
		}
	}

	g, gCtx = errgroup.WithContext(context.Background())

	if ptr := s.container.TLSManager.Load(); ptr != nil {
		manager := (*ptr).(types.LifecycleManager)
		g.Go(func() error {
			if err := manager.Stop(); err != nil {
				sai.Logger().Error("Failed to stop TLS manager", zap.Error(err))
				return err
			}
			return nil
		})
	}

	if ptr := s.container.Cache.Load(); ptr != nil {
		manager := (*ptr).(types.LifecycleManager)
		g.Go(func() error {
			if err := manager.Stop(); err != nil {
				sai.Logger().Error("Failed to stop cache manager", zap.Error(err))
				return err
			}
			return nil
		})
	}

	if ptr := s.container.Metrics.Load(); ptr != nil {
		manager := (*ptr).(types.LifecycleManager)
		g.Go(func() error {
			if err := manager.Stop(); err != nil {
				sai.Logger().Error("Failed to stop metrics manager", zap.Error(err))
				return err
			}
			return nil
		})
	}

	if ptr := s.container.Health.Load(); ptr != nil {
		manager := (*ptr).(types.LifecycleManager)
		g.Go(func() error {
			if err := manager.Stop(); err != nil {
				sai.Logger().Error("Failed to stop health manager", zap.Error(err))
				return err
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		errors = append(errors, err)
	}

	if ptr := s.container.Config.Load(); ptr != nil {
		if err := (*ptr).(types.LifecycleManager).Stop(); err != nil {
			sai.Logger().Error("Failed to stop config manager", zap.Error(err))
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return types.NewErrorf("errors during shutdown: %v", errors)
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
			if s.transitionState(StateRunning, StateStopping) {
				s.cancel()
			}

		case <-s.ctx.Done():
			sai.Logger().Info("Service context cancelled")
		}

		signal.Stop(sigChan)
		close(sigChan)
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
	var clientManager types.ClientManager

	configManager, err := config.NewConfigurationManager(ctx, configPath)
	if err != nil {
		return types.WrapError(err, "failed to register config manager")
	}
	container.SetConfig(configManager)

	_config := configManager.GetConfig()

	loggerManager, err := logger.NewManager(ctx, configManager)
	if err != nil {
		return types.WrapError(err, "failed to register logger")
	}
	container.SetLogger(loggerManager)

	router, err := server.NewFastHTTPRouter(ctx, loggerManager)
	if err != nil {
		return types.WrapError(err, "failed to register router")
	}
	container.SetRouter(router)

	authProvider, err := auth_providers.NewAuthProviderManager(ctx, configManager, loggerManager)
	if err != nil {
		return types.WrapError(err, "failed to register auth provider")
	}
	container.SetAuthProvider(authProvider)

	if _config.Health.Enabled {
		healthManager, err = health.NewManager(ctx, configManager, loggerManager, router)
		if err != nil {
			return types.WrapError(err, "failed to register health manager")
		}
		container.SetHealth(healthManager)
	}

	if _config.Metrics.Enabled {
		metricsManager, err = metrics.NewManager(ctx, configManager, loggerManager, router, healthManager)
		if err != nil {
			return types.WrapError(err, "failed to register metrics manager")
		}
		container.SetMetrics(metricsManager)
	}

	if _config.Server.TLS.Enabled {
		tlsManager, err = tls.NewCertManager(ctx, loggerManager, configManager)
		if err != nil {
			return types.WrapError(err, "failed to register TLS manager")
		}
		container.SetTLSManager(tlsManager)
	}

	if _config.Cron.Enabled {
		cronManager, err := cron.NewManager(ctx, configManager, loggerManager, metricsManager, healthManager)
		if err != nil {
			return types.WrapError(err, "failed to register cron manager")
		}
		container.SetCron(cronManager)
	}

	if _config.Cache.Enabled {
		cacheManager, err = cache.NewCacheManager(ctx, configManager, loggerManager, metricsManager, healthManager)
		if err != nil {
			return types.WrapError(err, "failed to register cache manager")
		}
		container.SetCache(cacheManager)
	}

	if _config.Middlewares.Enabled {
		middlewareManager, err = middleware.NewManager(ctx, configManager, loggerManager, metricsManager, cacheManager, healthManager, authProvider)
		if err != nil {
			return types.WrapError(err, "failed to register middleware manager")
		}
		container.SetMiddlewares(middlewareManager)
	}

	if _config.Client.Enabled {
		clientManager, err = client.NewManager(ctx, configManager, loggerManager, metricsManager, healthManager, middlewareManager, authProvider)
		if err != nil {
			return types.WrapError(err, "failed to register client manager")
		}
		container.SetClientManager(clientManager)
	}

	if _config.Actions != nil && (_config.Actions.Broker != nil || _config.Actions.Webhooks != nil) {
		actionDispatcher, err := action.NewDispatcher(ctx, configManager, loggerManager, router, metricsManager, healthManager, clientManager)
		if err != nil {
			return types.WrapError(err, "failed to register unified event dispatcher")
		}
		container.SetActions(actionDispatcher)
	}

	if _config.Docs.Enabled {
		documentationManager, err := documentations.NewDocumentationManager(configManager, loggerManager, healthManager, router)
		if err != nil {
			return types.WrapError(err, "failed to register documentation manager")
		}
		container.SetDocumentation(documentationManager)
	}

	httpServer, err := server.NewHTTPServer(ctx, configManager, loggerManager, metricsManager, middlewareManager, tlsManager, router)
	if err != nil {
		return types.WrapError(err, "failed to register HTTP server")
	}
	container.SetHTTPServer(httpServer)

	return nil
}
