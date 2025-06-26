# SAI Service Framework

A modern, high-performance Go microservice framework designed for building scalable web services with built-in monitoring, caching, authentication, and extensibility.

## Features

### 🚀 Core Components
- **Fast HTTP Server** - Built on FastHTTP for maximum performance
- **Flexible Router** - Dynamic and static route compilation with parameter extraction
- **Middleware System** - Composable middleware chain with configurable weights
- **Configuration Management** - YAML-based configuration with flexible access methods
- **Lifecycle Management** - Graceful startup and shutdown for all components

### 📊 Observability
- **Metrics Collection** - Built-in Prometheus and memory-based metrics
- **Health Checks** - Comprehensive health monitoring with custom checkers
- **Structured Logging** - Zap-based logging with configurable outputs
- **System Metrics** - Automatic collection of runtime and system metrics

### 🔒 Security & Auth
- **Authentication Providers** - Token-based and Basic auth out of the box
- **TLS Management** - Automatic certificate management with Let's Encrypt
- **CORS Support** - Configurable cross-origin resource sharing
- **Rate Limiting** - Configurable request rate limiting

### 💾 Data & Caching
- **Cache Management** - Memory and Redis cache implementations
- **Circuit Breaker** - Automatic circuit breaking for external services
- **HTTP Client** - Instrumented HTTP client with retries and metrics

### 🔄 Background Processing
- **Cron Scheduler** - Robust cron job scheduling with monitoring
- **Worker Management** - Lifecycle management for background services
- **Event System** - Webhook and message broker support for events

### 📝 Documentation
- **OpenAPI Generation** - Automatic API documentation generation
- **Swagger UI** - Built-in Swagger UI for API exploration

## Quick Start

### Installation

```bash
go get github.com/saiset-co/sai-service
```

### Basic Usage

```go
package main

import (
	"context"
	"log"

	"github.com/saiset-co/sai-service/service"
	"github.com/saiset-co/sai-service/sai"
)

func main() {
	ctx := context.Background()

	// Create service with config file
	srv, err := service.NewService(ctx, "./config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Register routes
	router := sai.Router()
	router.GET("/hello", handleHello).
		WithDoc("Hello World", "Returns a greeting", "example", nil, nil)

	// Start service
	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}

	// Wait for shutdown
	<-srv.Done()
}

func handleHello(ctx *types.RequestCtx) {
	ctx.WriteJSON(map[string]string{
		"message": "Hello, World!",
	})
}
```

### Configuration

Create a `config.yaml` file:

```yaml
name: "my-service"
version: "1.0.0"
```

## Architecture

### Component Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP Server   │    │   Router        │    │   Middleware    │
│                 │◄──►│                 │◄──►│   Manager       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Config        │    │   Logger        │    │   Metrics       │
│   Manager       │    │   Manager       │    │   Manager       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Cache         │    │   Health        │    │   TLS           │
│   Manager       │    │   Manager       │    │   Manager       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Service Container

The framework uses a dependency injection container (`sai.Container`) that manages all components:

```go
// Access components anywhere in your application
config := sai.Config()
logger := sai.Logger()
router := sai.Router()
```

## Advanced Features

### Route Groups

```go
api := router.Group("/api/v1")
api.WithMiddlewares("auth", "rate-limit")

api.GET("/users", handleGetUsers)
api.POST("/users", handleCreateUser)
api.PUT("/users/:id", handleUpdateUser)
```

### Cron Jobs

```go
cron := sai.Cron()
cron.Add("cleanup", "0 2 * * *", func() {
// Daily cleanup at 2 AM
log.Println("Running cleanup job")
})
```

### Health Checks

```go
health := sai.Health()
health.RegisterChecker("database", func(ctx context.Context) types.HealthCheck {
// Check database connectivity
if err := db.Ping(); err != nil {
return types.HealthCheck{
Status:  types.StatusUnhealthy,
Message: "Database connection failed",
}
}
return types.HealthCheck{
Status:  types.StatusHealthy,
Message: "Database is healthy",
}
})
```

### Background Workers

```go
type EmailWorker struct {
queue chan string
}

func (w *EmailWorker) Start(ctx context.Context) error {
w.queue = make(chan string, 100)
return nil
}

func (w *EmailWorker) Run(ctx context.Context) error {
for {
select {
case email := <-w.queue:
// Send email
log.Printf("Sending email to: %s", email)
case <-ctx.Done():
return nil
}
}
}

func (w *EmailWorker) Stop() error {
close(w.queue)
return nil
}

// Register worker
workers := sai.Workers()
workers.Register("email", &EmailWorker{})
```

### Event System

Register event handlers and publish events:

```go
// Register event handler
actions := sai.Actions()
actions.Subscribe("user.created", func(msg *types.ActionMessage) error {
user := msg.Payload.(map[string]interface{})
sai.Logger().Info("User created",
zap.String("email", user["email"].(string)))
return nil
})

// Publish event
actions.Publish("user.created", map[string]interface{}{
"id":    123,
"email": "user@example.com",
"name":  "John Doe",
})
```

### Automatic Webhook Registration

The framework supports automatic webhook registration between microservices.

**Server-side configuration** (webhook sender - publishes events):
```yaml
actions:
  enabled: true
  webhooks:
    enabled: true
  events:
    user.created:
      id: "string"
      name: "string"
      email: "string"
    user.updated:
      id: "string"
      name: "string"
      email: "string"
    user.deleted:
      id: "string"
```

**Client-side configuration** (webhook receiver - subscribes to events):
```yaml
actions:
  enabled: true
  webhooks:
    enabled: true

client:
  enabled: true
  services:
    user-service:
      url: "http://user-service:8080"
      events: ["user.created", "user.updated"]
      auth:
        provider: "token"
        payload:
          token: "service-token"
    order-service:
      url: "http://order-service:8080"
      events: ["order.created", "order.completed"]
      auth:
        provider: "basic"
        payload:
          username: "service-user"
          password: "service-password"
```

The client automatically registers webhooks with target services and receives events. Each microservice should define unique events in the `events` section to avoid conflicts.

### Webhook API Endpoints

The framework provides REST API endpoints for webhook management:

```bash
# Get available events
GET /api/webhooks/events
Response: {
  "success": true,
  "data": {
    "user.created": {
      "id": "string",
      "name": "string", 
      "email": "string"
    },
    "user.updated": {
      "id": "string",
      "name": "string",
      "email": "string"
    }
  }
}

# Create webhook
POST /api/webhooks
{
  "event": "user.created",
  "url": "http://target-service:8080/webhook/user-created",
  "headers": {
    "Authorization": "Bearer token123"
  },
  "enabled": true
}

# List all webhooks
GET /api/webhooks
Response: {
  "success": true,
  "data": [
    {
      "id": "wh_123456789",
      "event": "user.created",
      "url": "http://target-service:8080/webhook/user-created",
      "enabled": true,
      "created_at": "2024-01-01T12:00:00Z"
    }
  ],
  "total": 1
}

# Get specific webhook
GET /api/webhooks/get/{webhook_id}
Response: {
  "success": true,
  "data": {
    "id": "wh_123456789",
    "event": "user.created",
    "url": "http://target-service:8080/webhook/user-created",
    "secret": "generated-secret",
    "enabled": true,
    "created_at": "2024-01-01T12:00:00Z"
  }
}

# Update webhook
PUT /api/webhooks/update/{webhook_id}
{
  "enabled": false,
  "url": "http://new-target:8080/webhook/user-created"
}

# Delete webhook
DELETE /api/webhooks/delete/{webhook_id}
Response: {
  "success": true,
  "message": "Webhook deleted successfully"
}

# Test webhook delivery
POST /api/webhooks/test/{webhook_id}
Response: {
  "success": true,
  "delivered_at": "2024-01-01T12:00:00Z"
}
```

### Authentication

#### Configure Auth Providers

```yaml
auth_providers:
  token:
    params:
      token: "your-secret-api-key"
  basic:
    params:
      username: "admin"
      password: "secret"
```

#### Enable Auth Middleware

```yaml
middlewares:
  enabled: true
  auth:
    enabled: true
    weight: 60
    params:
      token: "your-secret-api-key"
```

#### Client Authentication

Configure authentication for each client service:

```yaml
client:
  enabled: true
  services:
    user-service:
      url: "http://user-service:8080"
      auth:
        provider: "token"
        payload:
          token: "service-to-service-token"
    admin-service:
      url: "http://admin-service:8080"
      auth:
        provider: "basic"
        payload:
          username: "service-user"
          password: "service-password"
```

### Client Manager

Make HTTP calls to other microservices:

```go
client := sai.ClientManager()

// Simple GET request
resp, statusCode, err := client.Call("user-service", "GET", "/api/users/123", nil, nil)
if err != nil {
return err
}

// POST request with data and options
userData := map[string]interface{}{
"name":  "John Doe",
"email": "john@example.com",
}

options := &types.CallOptions{
Headers: map[string]string{
"X-Request-ID": "req-123",
},
Timeout: 30 * time.Second,
Retry:   3,
}

resp, statusCode, err := client.Call("user-service", "POST", "/api/users", userData, options)
```

### Configuration Access

Access configuration values with type safety:

```go
config := sai.Config()

// Get typed values
var dbConfig struct {
Host     string `yaml:"host"`
Port     int    `yaml:"port"`
Database string `yaml:"database"`
}
err := config.GetAs("database", &dbConfig)

// Get raw values with defaults
maxRetries := config.GetValue("client.max_retries", 3).(int)
timeout := config.GetValue("client.timeout", "30s").(string)

// Get nested configuration
cacheSize := config.GetValue("cache.config.max_size", 1000).(int)
redisAddr := config.GetValue("cache.config.addr", "localhost:6379").(string)
```

### Logging

Use structured logging throughout your application:

```go
logger := sai.Logger()

// Basic logging
logger.Info("Service started")
logger.Error("Database connection failed")

// Structured logging with fields
logger.Info("User created",
zap.String("user_id", "123"),
zap.String("email", "user@example.com"),
zap.Duration("processing_time", time.Since(start)))

// Error logging with context
logger.Error("Failed to process request",
zap.String("request_id", requestID),
zap.String("endpoint", "/api/users"),
zap.Error(err))

// Debug logging
logger.Debug("Cache hit",
zap.String("key", "user:123"),
zap.Duration("lookup_time", lookupTime))

// Conditional logging
if logger.Core().Enabled(zapcore.DebugLevel) {
expensiveDebugData := computeExpensiveDebugInfo()
logger.Debug("Debug info", zap.Any("data", expensiveDebugData))
}
```

## API Documentation

The framework automatically generates OpenAPI 3.0 documentation from your route definitions:

```go
// Define request and response types
type CreateUserRequest struct {
Name     string `json:"name" validate:"required" doc:"User's full name"`
Email    string `json:"email" validate:"required,email" doc:"User's email address"`
Password string `json:"password" validate:"required,min=8" doc:"User's password (min 8 characters)"`
Role     string `json:"role" validate:"required,oneof=admin user" doc:"User's role" example:"user"`
}

type CreateUserResponse struct {
ID        int       `json:"id" doc:"User's unique identifier"`
Name      string    `json:"name" doc:"User's full name"`
Email     string    `json:"email" doc:"User's email address"`
Role      string    `json:"role" doc:"User's role"`
CreatedAt time.Time `json:"created_at" doc:"User creation timestamp"`
}

router.POST("/users", handleCreateUser).
WithDoc(
"Create User",
"Creates a new user account",
"users",
CreateUserRequest{},
CreateUserResponse{},
)
```

Access the documentation at `/docs` (Swagger UI) or `/openapi.json` (raw OpenAPI spec).

## Monitoring & Metrics

### Built-in Metrics

- HTTP request metrics (duration, status codes, response sizes)
- System metrics (CPU, memory, goroutines)
- Cache metrics (hit/miss ratios, evictions)
- Circuit breaker metrics
- Custom application metrics

### Prometheus Integration

```yaml
metrics:
  enabled: true
  type: "prometheus"
  config:
    namespace: "myapp"
    subsystem: "api"
    path: "/metrics"
```

## Configuration Reference

### Server Configuration

```yaml
server:
  http:
    host: "0.0.0.0"          # Server host
    port: 8080               # Server port
    read_timeout: 30         # Read timeout in seconds
    write_timeout: 30        # Write timeout in seconds
    idle_timeout: 120        # Idle timeout in seconds
  tls:
    enabled: false           # Enable TLS
    auto_cert: false         # Enable automatic certificates
    domains: []              # Domains for auto certificates
    cert_file: ""            # Certificate file path
    key_file: ""             # Private key file path
```

### Middleware Configuration

```yaml
middlewares:
  enabled: true
  recovery:
    enabled: true
    weight: 10               # Execution order (lower = earlier)
    params:
      stack_trace: true
  logging:
    enabled: true
    weight: 20
    params:
      log_level: "info"
      log_headers: false
      log_body: false
  rate_limit:
    enabled: true
    weight: 30
    params:
      requests_per_minute: 100
  body_limit:
    enabled: true
    weight: 40
    params:
      max_body_size: 10485760  # 10MB
  cors:
    enabled: true
    weight: 50
    params:
      AllowedOrigins: ["*"]
      AllowedMethods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
      AllowedHeaders: ["Content-Type", "Authorization", "X-API-Key", "X-Request-ID"]
      MaxAge: 86400
  auth:
    enabled: true
    weight: 60
    params:
      token: "your-api-key"
  compression:
    enabled: true
    weight: 70
    params:
      algorithm: "gzip"
      level: 6
      threshold: 1024
      allowed_types:
        - "application/json"
        - "application/xml"
        - "text/*"
      timeout: 30
  cache:
    enabled: true
    weight: 80
    params:
      default_ttl: "5m"
```

### Cache Configuration

```yaml
cache:
  enabled: true
  type: "memory"            # "memory" or "redis"
  default_ttl: "1h"
  config:
    max_size: 1000          # Memory cache specific
    # Redis configuration
    addr: "localhost:6379"
    password: ""
    db: 0
```

## Extensibility

### Custom Middleware

Create your own middleware by implementing the `Middleware` interface:

```go
type CustomMiddleware struct {
logger types.Logger
}

func (m *CustomMiddleware) Handle(ctx *types.RequestCtx, next func(*types.RequestCtx), config *types.RouteConfig) {
start := time.Now()

// Pre-processing
m.logger.Info("Request started", zap.String("path", string(ctx.Path())))

// Call next middleware
next(ctx)

// Post-processing
duration := time.Since(start)
m.logger.Info("Request completed", zap.Duration("duration", duration))
}

func (m *CustomMiddleware) Name() string { return "custom" }
func (m *CustomMiddleware) Weight() int { return 100 }

// Register middleware
middleware := sai.Middlewares()
middleware.Register(&CustomMiddleware{logger: sai.Logger()})
```

### Custom Action Broker

Register custom message brokers for event handling:

```go
type RedisActionBroker struct {
client *redis.Client
}

func (b *RedisActionBroker) Publish(action string, payload interface{}) error {
data, _ := json.Marshal(payload)
return b.client.Publish(action, data).Err()
}

func (b *RedisActionBroker) Subscribe(action string, handler types.ActionHandler) error {
// Implementation for Redis pub/sub
return nil
}

func (b *RedisActionBroker) Unsubscribe(action string) error {
// Implementation
return nil
}

// Register custom action broker
sai.RegisterActionBroker("redis", func(config interface{}) (types.ActionBroker, error) {
cfg := config.(map[string]interface{})
client := redis.NewClient(&redis.Options{
Addr: cfg["addr"].(string),
})
return &RedisActionBroker{client: client}, nil
})
```

### Custom Cache Manager

Implement custom caching solutions:

```go
type CustomCacheManager struct {
data map[string]interface{}
mu   sync.RWMutex
}

func (c *CustomCacheManager) Get(key string) (interface{}, bool) {
c.mu.RLock()
defer c.mu.RUnlock()
value, exists := c.data[key]
return value, exists
}

func (c *CustomCacheManager) Set(key string, value interface{}, ttl time.Duration) error {
c.mu.Lock()
defer c.mu.Unlock()
c.data[key] = value
return nil
}

// Implement other required methods...

// Register custom cache manager
sai.RegisterCacheManager("custom", func(config interface{}) (types.CacheManager, error) {
return &CustomCacheManager{
data: make(map[string]interface{}),
}, nil
})
```

### Custom Metrics Manager

Create custom metrics collectors:

```go
type CustomMetricsManager struct {
counters   map[string]float64
gauges     map[string]float64
histograms map[string][]float64
}

func (m *CustomMetricsManager) Counter(name string, labels map[string]string) types.Counter {
return &customCounter{name: name, manager: m}
}

func (m *CustomMetricsManager) Gauge(name string, labels map[string]string) types.Gauge {
return &customGauge{name: name, manager: m}
}

// Implement other required methods...

// Register custom metrics manager
sai.RegisterMetricsManager("custom", func(config interface{}) (types.MetricsManager, error) {
return &CustomMetricsManager{
counters:   make(map[string]float64),
gauges:     make(map[string]float64),
histograms: make(map[string][]float64),
}, nil
})
```

### Custom Logger

Implement custom logging solutions:

```go
type CustomLogger struct {
level zapcore.Level
out   io.Writer
}

func (l *CustomLogger) Error(msg string, fields ...zap.Field) {
l.log(zapcore.ErrorLevel, msg, fields...)
}

func (l *CustomLogger) Info(msg string, fields ...zap.Field) {
l.log(zapcore.InfoLevel, msg, fields...)
}

func (l *CustomLogger) log(level zapcore.Level, msg string, fields ...zap.Field) {
if l.level.Enabled(level) {
// Custom logging implementation
fmt.Fprintf(l.out, "[%s] %s\n", level, msg)
}
}

// Implement other required methods...

// Register custom logger
sai.RegisterLogger("custom", func(config interface{}) (types.Logger, error) {
return &CustomLogger{
level: zapcore.InfoLevel,
out:   os.Stdout,
}, nil
})
```

### Usage in Configuration

After registering custom implementations, use them in your configuration:

```yaml
# Custom action broker
actions:
  enabled: true
  broker:
    enabled: true
    type: "redis"
    config:
      addr: "localhost:6379"

# Custom cache manager
cache:
  enabled: true
  type: "custom"
  config:
    custom_param: "value"

# Custom metrics manager
metrics:
  enabled: true
  type: "custom"
  config:
    output_file: "/var/log/metrics.log"

# Custom logger
logger:
  type: "custom"
  level: "info"
  config:
    output_format: "json"
```

The framework provides testing utilities for easy unit and integration testing:

```go
func TestHandler(t *testing.T) {
// Create test server
srv, _ := service.NewService(context.Background(), "./test-config.yaml")

// Register test route
router := sai.Router()
router.GET("/test", handleTest)

// Make test request
req := fasthttp.AcquireRequest()
req.SetRequestURI("/test")

resp := fasthttp.AcquireResponse()
err := srv.ServeHTTP(req, resp)

assert.NoError(t, err)
assert.Equal(t, 200, resp.StatusCode())
}
```

## Performance

The framework is built for high performance:

- **FastHTTP** - Up to 10x faster than net/http
- **Zero-allocation routing** - Efficient route matching
- **Connection pooling** - Reusable HTTP connections
- **Compiled middleware chains** - Minimal runtime overhead
- **Memory-efficient caching** - LRU cache with TTL support

### Benchmarks

```
BenchmarkRouter-8         5000000    250 ns/op    0 allocs/op
BenchmarkMiddleware-8     3000000    400 ns/op    1 allocs/op
BenchmarkCache-8         10000000    150 ns/op    0 allocs/op
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Setup

```bash
# Clone repository
git clone https://github.com/saiset-co/sai-service.git
cd sai-service

# Install dependencies
go mod download

# Run tests
go test ./...

# Run example
go run examples/basic/main.go
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- 📚 [Documentation](https://docs.saiset.co/sai-service)
- 🐛 [Issue Tracker](https://github.com/saiset-co/sai-service/issues)
- 💬 [Discussions](https://github.com/saiset-co/sai-service/discussions)
- 📧 [Email Support](mailto:support@saiset.co)

---

Built with ❤️ by the SAI Team
