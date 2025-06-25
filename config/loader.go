package config

import (
	"context"
	"os"
	"sync/atomic"
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/saiset-co/sai-service/types"
)

type LoaderState int32

const (
	LoaderStateStopped LoaderState = iota
	LoaderStateStarting
	LoaderStateRunning
	LoaderStateStopping
)

type Loader struct {
	ctx             context.Context
	cancel          context.CancelFunc
	validator       *validator.Validate
	state           atomic.Value
	shutdownTimeout time.Duration
	readTimeout     time.Duration
}

func NewLoader() (*Loader, error) {
	ctx, cancel := context.WithCancel(context.Background())

	loader := &Loader{
		ctx:             ctx,
		cancel:          cancel,
		validator:       validator.New(validator.WithRequiredStructEnabled()),
		shutdownTimeout: 5 * time.Second,
		readTimeout:     30 * time.Second,
	}

	loader.state.Store(LoaderStateStopped)

	return loader, nil
}

func (l *Loader) Start() error {
	if !l.transitionState(LoaderStateStopped, LoaderStateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if l.getState() == LoaderStateStarting {
			l.setState(LoaderStateRunning)
		}
	}()

	return nil
}

func (l *Loader) Stop() error {
	if !l.transitionState(LoaderStateRunning, LoaderStateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		l.setState(LoaderStateStopped)
		l.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), l.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
		default:
		}
	}

	return nil
}

func (l *Loader) IsRunning() bool {
	return l.getState() == LoaderStateRunning
}

func (l *Loader) LoadFromFile(ctx context.Context, configPath string) (*types.ServiceConfig, *map[string]interface{}, error) {
	if configPath == "" {
		return nil, nil, types.ErrConfigNotFound
	}

	loadCtx, cancel := context.WithTimeout(ctx, l.readTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(loadCtx)

	var data []byte
	var config *types.ServiceConfig
	var rawData map[string]interface{}
	var readErr, parseErr, validateErr error

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				readErr = types.WrapError(err, "file not found: "+configPath)
				return readErr
			}

			var err error
			data, err = l.ReadFileWithTimeout(gCtx, configPath)
			if err != nil {
				readErr = types.WrapError(err, "failed to read config file")
				return readErr
			}
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-loadCtx.Done():
			return nil, nil, types.WrapError(loadCtx.Err(), "file read timeout")
		default:
			if readErr != nil {
				return nil, nil, readErr
			}
			return nil, nil, types.WrapError(err, "failed to read configuration file")
		}
	}

	g, gCtx = errgroup.WithContext(loadCtx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			rawData = make(map[string]interface{})
			config = l.Defaults()

			if err := yaml.Unmarshal(data, &rawData); err != nil {
				parseErr = types.WrapError(err, "failed to parse YAML to raw data")
				return parseErr
			}

			if err := yaml.Unmarshal(data, config); err != nil {
				parseErr = types.WrapError(err, "failed to parse YAML config to struct")
				return parseErr
			}

			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-loadCtx.Done():
			return nil, nil, types.WrapError(loadCtx.Err(), "config parse timeout")
		default:
			if parseErr != nil {
				return nil, nil, parseErr
			}
			return nil, nil, types.WrapError(err, "failed to parse configuration")
		}
	}

	g, gCtx = errgroup.WithContext(loadCtx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			if err := l.validator.Struct(config); err != nil {
				validateErr = types.WrapError(err, "config validation failed")
				return validateErr
			}
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-loadCtx.Done():
			return nil, nil, types.WrapError(loadCtx.Err(), "config validation timeout")
		default:
			if validateErr != nil {
				return nil, nil, validateErr
			}
			return nil, nil, types.WrapError(err, "failed to validate configuration")
		}
	}

	return config, &rawData, nil
}

func (l *Loader) ReadFileWithTimeout(ctx context.Context, filepath string) ([]byte, error) {
	type result struct {
		data []byte
		err  error
	}

	resultChan := make(chan result, 1)
	readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- result{err: types.NewErrorf("file read panicked: %v", r)}
			}
		}()

		data, err := os.ReadFile(filepath)
		resultChan <- result{data: data, err: err}
	}()

	select {
	case res := <-resultChan:
		if res.err != nil {
			return nil, types.WrapError(res.err, "failed to read file")
		}
		return res.data, nil
	case <-readCtx.Done():
		return nil, types.WrapError(readCtx.Err(), "file read timeout")
	case <-ctx.Done():
		return nil, types.WrapError(ctx.Err(), "operation canceled")
	}
}

func (l *Loader) ValidateConfig(config *types.ServiceConfig) error {
	if config == nil {
		return types.ErrConfigNotFound
	}

	validateCtx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- types.NewErrorf("config validation panicked: %v", r)
			}
		}()

		if err := l.validator.Struct(config); err != nil {
			done <- types.WrapError(err, "validation failed")
		} else {
			done <- nil
		}
	}()

	select {
	case err := <-done:
		return err
	case <-validateCtx.Done():
		return types.WrapError(validateCtx.Err(), "validation timeout")
	case <-l.ctx.Done():
		return types.WrapError(l.ctx.Err(), "loader shutting down")
	}
}

func (l *Loader) Defaults() *types.ServiceConfig {
	return &types.ServiceConfig{
		Server: &types.ServerConfig{
			HTTP: &types.HTTPConfig{
				Host:         "localhost",
				Port:         8080,
				ReadTimeout:  30,
				WriteTimeout: 30,
				IdleTimeout:  120,
			},
			TLS: &types.TLSConfig{
				Enabled:  false,
				AutoCert: false,
			},
		},
		AuthProviders: &types.AuthProvidersConfig{
			Token: &types.AuthProviderItemConfig{},
			Basic: &types.AuthProviderItemConfig{},
		},
		Logger: &types.LoggerConfig{
			Level: "debug",
		},
		Cache: &types.CacheConfig{
			Enabled:    false,
			Type:       "memory",
			DefaultTTL: time.Hour,
		},
		Cron: &types.CronConfig{
			Enabled:  false,
			Timezone: "UTC",
		},
		Actions: &types.ActionsConfig{
			Enabled: false,
			Broker: &types.BrokerConfig{
				Enabled: false,
			},
			Webhooks: &types.WebhooksConfig{
				Enabled: false,
			},
		},
		Docs: &types.DocsConfig{
			Enabled: false,
			Path:    "/docs",
		},
		Metrics: &types.MetricsConfig{
			Enabled: false,
			Type:    "memory",
		},
		Health: &types.HealthConfig{
			Enabled: false,
		},
		Client: &types.ClientConfig{
			Enabled: false,
		},
		Middlewares: &types.MiddlewaresConfig{
			Enabled: false,
			Recovery: &types.MiddlewareItemConfig{
				Enabled: false,
				Params: map[string]interface{}{
					"stack_trace": true,
				},
				Weight: 10,
			},
			Logging: &types.MiddlewareItemConfig{
				Enabled: false,
				Params: map[string]interface{}{
					"log_level":   "info",
					"log_headers": false,
					"log_body":    false,
				},
				Weight: 20,
			},
			RateLimit: &types.MiddlewareItemConfig{
				Enabled: false,
				Params: map[string]interface{}{
					"requests_per_minute": 100,
				},
				Weight: 30,
			},
			BodyLimit: &types.MiddlewareItemConfig{
				Enabled: false,
				Params: map[string]interface{}{
					"max_body_size": 10485760,
				},
				Weight: 40,
			},
			CORS: &types.MiddlewareItemConfig{
				Enabled: false,
				Params: map[string]interface{}{
					"AllowedOrigins": []string{"*"},
					"AllowedMethods": []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
					"AllowedHeaders": []string{"Content-Type", "Authorization", "X-API-Key", "X-Request-ID"},
					"MaxAge":         86400,
				},
				Weight: 50,
			},
			Auth: &types.MiddlewareItemConfig{
				Enabled: false,
				Params: map[string]interface{}{
					"token": "123",
				},
				Weight: 60,
			},
			Compression: &types.MiddlewareItemConfig{
				Enabled: false,
				Params: map[string]interface{}{
					"algorithm": "gzip",
					"level":     6,
					"threshold": 1024,
					"allowed_types": []string{
						"application/json",
						"application/xml",
						"application/javascript",
						"text/*",
						"application/rss+xml",
						"application/atom+xml",
					},
					"timeout": 30,
				},
				Weight: 70,
			},
			Cache: &types.MiddlewareItemConfig{
				Enabled: false,
				Params: map[string]interface{}{
					"default_ttl": 5 * time.Minute,
				},
				Weight: 80,
			},
		},
	}
}

func (l *Loader) getState() LoaderState {
	return l.state.Load().(LoaderState)
}

func (l *Loader) setState(newState LoaderState) bool {
	currentState := l.getState()
	return l.state.CompareAndSwap(currentState, newState)
}

func (l *Loader) transitionState(from, to LoaderState) bool {
	return l.state.CompareAndSwap(from, to)
}
