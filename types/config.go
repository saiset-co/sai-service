package types

import (
	"time"
)

type ConfigManager interface {
	GetConfig() *ServiceConfig
	GetValue(path string, defaultValue interface{}) interface{}
	GetAs(path string, target interface{}) error
}

type ServiceConfig struct {
	Name          string               `yaml:"name" json:"name" validate:"required"`
	Version       string               `yaml:"version" json:"version" validate:"required"`
	Server        *ServerConfig        `yaml:"server" json:"server"`
	Logger        *LoggerConfig        `yaml:"logger" json:"logger"`
	Cache         *CacheConfig         `yaml:"cache" json:"cache"`
	Database      *DatabaseConfig      `yaml:"database" json:"database"`
	Actions       *ActionsConfig       `yaml:"actions" json:"actions"`
	Cron          *CronConfig          `yaml:"cron" json:"cron"`
	AuthProviders *AuthProvidersConfig `yaml:"auth_providers" json:"auth_providers"`
	Middlewares   *MiddlewaresConfig   `yaml:"middlewares" json:"middlewares"`
	Docs          *DocsConfig          `yaml:"docs" json:"docs"`
	Metrics       *MetricsConfig       `yaml:"metrics" json:"metrics"`
	Clients       *ClientsConfig       `yaml:"clients" json:"clients"`
	Health        *HealthConfig        `yaml:"health" json:"health"`
}

type ServerConfig struct {
	HTTP *HTTPConfig `yaml:"http" json:"http"`
	TLS  *TLSConfig  `yaml:"tls" json:"tls"`
}

type HTTPConfig struct {
	Host            string `yaml:"host" json:"host"`
	Port            int    `yaml:"port" json:"port" validate:"min=1,max=65535"`
	ReadTimeout     int    `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout    int    `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout     int    `yaml:"idle_timeout" json:"idle_timeout"`
	ShutdownTimeout int    `yaml:"shutdown_timeout" json:"shutdown_timeout"`
}

type TLSConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	CertFile      string   `yaml:"cert_file,omitempty" json:"cert_file,omitempty"`
	KeyFile       string   `yaml:"key_file,omitempty" json:"key_file,omitempty"`
	AutoCert      bool     `yaml:"auto_cert" json:"auto_cert"`
	Domains       []string `yaml:"domains,omitempty" json:"domains,omitempty"`
	Email         string   `yaml:"email,omitempty" json:"email,omitempty"`
	CacheDir      string   `yaml:"cache_dir,omitempty" json:"cache_dir,omitempty"`
	ACMEDirectory string   `yaml:"acme_directory,omitempty" json:"acme_directory,omitempty"`
}

type LoggerConfig struct {
	Type   string      `yaml:"type" json:"type"`
	Level  string      `yaml:"level" json:"level" validate:"required_if=Enabled true"`
	Config interface{} `yaml:"config" json:"config"`
}

type CacheConfig struct {
	Enabled    bool          `yaml:"enabled" json:"enabled"`
	Type       string        `yaml:"type" json:"type" validate:"required_if=Enabled true"`
	Config     interface{}   `yaml:"config" json:"config"`
	DefaultTTL time.Duration `yaml:"default_ttl" json:"default_ttl" validate:"min=0"`
}

type DatabaseConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Type    string `yaml:"type" json:"type" validate:"required_if=Enabled true"`
	Path    string `yaml:"path" json:"path"`
}

type ActionsConfig struct {
	Enabled  bool                   `yaml:"enabled" json:"enabled"`
	Broker   *BrokerConfig          `yaml:"broker" json:"broker"`
	Webhooks *WebhooksConfig        `yaml:"webhooks" json:"webhooks"`
	Events   map[string]EventSchema `yaml:"events" json:"events"`
}

type BrokerConfig struct {
	Enabled bool        `yaml:"enabled" json:"enabled"`
	Type    string      `yaml:"type" json:"type"`
	Config  interface{} `yaml:"config" json:"config"`
}

type WebhooksConfig struct {
	Enabled bool        `yaml:"enabled" json:"enabled"`
	Config  interface{} `yaml:"config" json:"config"`
}

type EventSchema struct {
	Fields map[string]string `yaml:",inline" json:",inline"`
}

type CronConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Timezone string `yaml:"timezone" json:"timezone" validate:"required_if=Enabled true"`
}

type AuthProvidersConfig struct {
	Token *AuthProviderItemConfig `json:"token" yaml:"token"`
	Basic *AuthProviderItemConfig `json:"basic" yaml:"basic"`
}

type AuthProviderItemConfig struct {
	Params map[string]interface{} `yaml:"params" json:"params"`
}

type MiddlewaresConfig struct {
	Enabled     bool                  `yaml:"enabled" json:"enabled"`
	Auth        *MiddlewareItemConfig `yaml:"auth" json:"auth"`
	Metadata    *MiddlewareItemConfig `yaml:"metadata" json:"metadata"`
	Logging     *MiddlewareItemConfig `yaml:"logging" json:"logging"`
	Cache       *MiddlewareItemConfig `yaml:"cache" json:"cache"`
	Recovery    *MiddlewareItemConfig `yaml:"recovery" json:"recovery"`
	Compression *MiddlewareItemConfig `yaml:"compression" json:"compression"`
	CORS        *MiddlewareItemConfig `yaml:"cors" json:"cors"`
	RateLimit   *MiddlewareItemConfig `yaml:"rate_limit" json:"rate_limit"`
	BodyLimit   *MiddlewareItemConfig `yaml:"body_limit" json:"body_limit"`
}

type MiddlewareItemConfig struct {
	Enabled bool                   `yaml:"enabled" json:"enabled"`
	Weight  int                    `yaml:"weight" json:"weight" validate:"min=0"`
	Params  map[string]interface{} `yaml:"params" json:"params"`
}

type DocsConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Path    string `yaml:"path" json:"path" validate:"required_if=Enabled true"`
}

type CacheHandlerConfig struct {
	Enabled bool          `validate:"required"`
	Key     string        `validate:"required,min=1"`
	TTL     time.Duration `validate:"min=0"`
	Deps    []string      `validate:"dive,min=1"`
}

type VersionInfo struct {
	Version   string `json:"version"`
	BuildInfo string `json:"build_info"`
}

type MetricsConfig struct {
	Enabled    bool                   `yaml:"enabled" json:"enabled"`
	Type       string                 `yaml:"type" json:"type" validate:"required_if=Enabled true"`
	Config     interface{}            `yaml:"config" json:"config"`
	Prefix     string                 `yaml:"prefix" json:"prefix"`
	Labels     map[string]string      `yaml:"labels" json:"labels"`
	HTTP       MetricsHTTPConfig      `yaml:"http" json:"http"`
	Collectors MetricsCollectorConfig `yaml:"collectors" json:"collectors"`
}

type MetricsHTTPConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Path    string `yaml:"path" json:"path"`
	Port    int    `yaml:"port" json:"port"`
}

type MetricsCollectorConfig struct {
	System     bool `yaml:"system" json:"system"`
	Runtime    bool `yaml:"runtime" json:"runtime"`
	HTTP       bool `yaml:"http" json:"http"`
	Cache      bool `yaml:"cache" json:"cache"`
	Middleware bool `yaml:"middleware" json:"middleware"`
}

type HealthConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

type ClientsConfig struct {
	Enabled            bool                            `yaml:"enabled" json:"enabled"`
	DefaultTimeout     time.Duration                   `yaml:"default_timeout" json:"default_timeout"`
	MaxIdleConnections int                             `yaml:"max_idle_connections" json:"max_idle_connections"`
	IdleConnTimeout    time.Duration                   `yaml:"idle_conn_timeout" json:"idle_conn_timeout"`
	DefaultRetries     int                             `yaml:"default_retries" json:"default_retries"`
	CircuitBreaker     *CircuitBreakerConfig           `yaml:"circuit_breaker" json:"circuit_breaker"`
	Services           map[string]*ServiceClientConfig `yaml:"services" json:"services"`
}

type ServiceClientConfig struct {
	Url    string             `yaml:"url" json:"url" validate:"required"`
	Auth   *ServiceAuthConfig `yaml:"auth" json:"auth"`
	Events []string           `yaml:"events" json:"events"`
}

type ServiceAuthConfig struct {
	Provider string                 `yaml:"provider" json:"provider"`
	Payload  map[string]interface{} `yaml:"payload" json:"payload"`
}

type CircuitBreakerConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	FailureThreshold int           `yaml:"failure_threshold" json:"failure_threshold"`
	RecoveryTimeout  time.Duration `yaml:"recovery_timeout" json:"recovery_timeout"`
	HalfOpenRequests int           `yaml:"half_open_requests" json:"half_open_requests"`
}
