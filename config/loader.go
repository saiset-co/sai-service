package config

import (
	"context"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"

	"github.com/saiset-co/sai-service/types"
)

type Loader struct {
	validator *validator.Validate
}

func NewLoader() *Loader {
	return &Loader{
		validator: validator.New(validator.WithRequiredStructEnabled()),
	}
}

func (l *Loader) LoadFromFile(configPath string) (config *types.ServiceConfig, err error) {
	if configPath == "" {
		return config, types.ErrConfigNotFound
	}

	if _, err = os.Stat(configPath); os.IsNotExist(err) {
		return config, types.WrapError(err, "file not found: "+configPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data, err := l.ReadFileWithTimeout(ctx, configPath)
	if err != nil {
		return config, types.WrapError(err, "failed to read config file")
	}

	config = l.Defaults()

	if err := yaml.Unmarshal(data, config); err != nil {
		return config, types.WrapError(err, "failed to parse YAML config")

	}

	if err := l.validator.Struct(config); err != nil {
		return config, types.WrapError(err, "config validation failed")
	}

	return config, nil
}

func (l *Loader) ReadFileWithTimeout(ctx context.Context, filepath string) ([]byte, error) {
	type result struct {
		data []byte
		err  error
	}

	resultChan := make(chan result, 1)

	go func() {
		data, err := os.ReadFile(filepath)
		resultChan <- result{data: data, err: err}
	}()

	select {
	case res := <-resultChan:
		return res.data, res.err
	case <-ctx.Done():
		return nil, types.WrapError(ctx.Err(), "file read timeout")
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
				Enabled: false,
			},
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
			Type:    "",
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
				Enabled: true,
				Params: map[string]interface{}{
					"stack_trace": true,
				},
				Weight: 10,
			},
			Logging: &types.MiddlewareItemConfig{
				Enabled: true,
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
				Enabled: true,
				Params: map[string]interface{}{
					"AllowedOrigins": []string{"*"},
					"AllowedMethods": []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
					"AllowedHeaders": []string{"Content-Type", "Authorization", "X-API-Key", "X-Request-ID"},
					"MaxAge":         86400,
				},
				Weight: 50,
			},
			Metadata: &types.MiddlewareItemConfig{
				Enabled: false,
				Params: map[string]interface{}{
					"generate_request_id": true,
					"propagated_headers":  []string{"Token", "X-User-ID", "X-Real-IP", "X-Request-ID", "X-Trace-ID"},
				},
				Weight: 60,
			},
			Auth: &types.MiddlewareItemConfig{
				Enabled: false,
				Params: map[string]interface{}{
					"token": "123",
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
			Compression: &types.MiddlewareItemConfig{
				Enabled: false,
				Params: map[string]interface{}{
					"stack_trace": true,
				},
				Weight: 90,
			},
		},
	}
}
