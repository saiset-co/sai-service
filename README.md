# SAI Service Framework

A high-performance Go microservice framework built on FastHTTP with comprehensive features for building scalable web services and APIs.

## Features

🚀 **High Performance**
- Built on FastHTTP for maximum throughput
- Optimized memory management and zero-allocation patterns
- Advanced routing with caching and parameter extraction

🔧 **Comprehensive Toolkit**
- Configuration management with YAML support
- Structured logging with Zap
- Metrics collection (Memory/Prometheus)
- Caching (Memory/Redis) with dependency tracking
- Health checks and monitoring
- Cron job scheduling
- HTTP client with circuit breaker
- Middleware system with custom ordering

📚 **Developer Experience**
- Automatic OpenAPI/Swagger documentation generation
- Project generator for rapid development
- Hot reload support
- Comprehensive testing utilities

🛡️ **Production Ready**
- TLS/HTTPS support with auto-certificates (Let's Encrypt)
- Rate limiting and request throttling
- CORS, compression, and security middlewares
- Graceful shutdown and error recovery
- WebSocket support for real-time features

## Quick Start

### Installation

```bash
go get -u github.com/saiset-co/sai-service
```

### Using the Project Generator

The fastest way to start is using the built-in project generator:

```bash
# Download and run the generator
curl -sSL https://raw.githubusercontent.com/saiset-co/sai-service/main/generator.sh | bash

# Or clone and run locally
git clone https://github.com/saiset-co/sai-service.git
cd sai-service
chmod +x generator.sh
./generator.sh
```

#### Generator Options

**Interactive Mode (Recommended)**
```bash
./generator.sh
# Follow the interactive prompts
```

**Command Line Mode**
```bash
./generator.sh --name "my-api" --template api --features "cache,metrics,docs"
```

**Available Templates:**
- `basic` - Minimal HTTP server
- `api` - REST API with basic middleware
- `microservice` - Full microservice with cache, metrics, health
- `full` - All features enabled

**Available Features:**
- `cache` - Memory/Redis caching
- `metrics` - Prometheus/Memory metrics
- `docs` - OpenAPI/Swagger documentation
- `cron` - Job scheduling
- `actions` - WebSocket action broker
- `tls` - HTTPS/auto-certificates
- `middleware` - Full middleware stack
- `health` - Health checks
- `client` - HTTP client with circuit breaker

**Example Commands:**
```bash
# Create a simple API service
./generator.sh --name "user-service" --template api --features "cache,metrics,docs"

# Create a full microservice
./generator.sh --name "order-service" --template microservice --tests --ci github

# Create with custom module name
./generator.sh --name "gateway" --module "github.com/company/gateway" --template full
```

### Manual Setup

If you prefer manual setup, here's a minimal example:

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
    
    // Create service
    srv, err := service.NewService(ctx, "config.yml")
    if err != nil {
        log.Fatal(err)
    }
    
    // Register routes
    router := sai.Router()
    router.GET("/hello", handleHello).
        WithDoc("Hello World", "Simple greeting endpoint", "Demo", nil, nil)
    
    // Start service
    if err := srv.Run(); err != nil {
        log.Fatal(err)
    }
}

func handleHello(ctx *fasthttp.RequestCtx) {
    ctx.SetContentType("application/json")
    ctx.SetBodyString(`{"message": "Hello, World!"}`)
}
```

### Configuration

Create a `config.yml` file:

```yaml
name: "my-service"
version: "1.0.0"

server:
  http:
    host: "0.0.0.0"
    port: 8080

logger:
  level: "info"

# Enable features as needed
cache:
  enabled: true
  type: "memory"

metrics:
  enabled: true
  type: "memory"

docs:
  enabled: true
  path: "/docs"
```

## Architecture

### Core Components

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   HTTP Server   │────│     Router       │────│   Handlers      │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                       │                       │
         │              ┌──────────────────┐             │
         │              │   Middlewares    │             │
         │              └──────────────────┘             │
         │                       │                       │
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Config Mgr    │    │   Service Bus    │    │   Cache Mgr     │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                       │                       │
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Logger        │    │   Metrics        │    │   Health        │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

### Request Flow

1. **HTTP Request** → FastHTTP Server
2. **Routing** → URL pattern matching with parameters
3. **Middleware Chain** → Authentication, logging, rate limiting, etc.
4. **Handler Execution** → Business logic
5. **Response** → JSON serialization and HTTP response

## API Examples

### Basic REST API

```go
// Register routes with documentation
api := router.Group("/api/v1")

api.GET("/users", handlers.GetUsers).
    WithCache("users_list", 300, "users").
    WithDoc("Get Users", "Retrieve all users", "Users", 
        models.GetUsersRequest{}, models.UsersResponse{})

api.POST("/users", handlers.CreateUser).
    WithDoc("Create User", "Create a new user", "Users",
        models.CreateUserRequest{}, models.UserResponse{})

api.GET("/users/{id}", handlers.GetUser).
    WithCache("user_{id}", 600, "users").
    WithDoc("Get User", "Get user by ID", "Users",
        nil, models.UserResponse{})
```

### Advanced Features

**Caching with Dependencies**
```go
// Cache will be invalidated when "users" dependency changes
api.GET("/users", handler).
    WithCache("users_list", 300, "users", "permissions")
```

**Middleware Configuration**
```go
// Custom middleware chain
api.POST("/admin/users", handler).
    WithMiddlewares("Auth", "RateLimit").
    WithTimeout(30 * time.Second)
```

**Client Usage**
```go
// HTTP client with circuit breaker
client, _ := sai.ClientManager().GetClient("user-service")
err := client.Call("POST", "/users", userData, types.CallOptions{
    Timeout: 10 * time.Second,
    Retry:   3,
})
```

## Configuration Reference

### Server Configuration

```yaml
server:
  http:
    host: "0.0.0.0"
    port: 8080
    read_timeout: 30
    write_timeout: 30
    idle_timeout: 120
  tls:
    enabled: true
    auto_cert: true
    domains: ["api.example.com"]
    email: "admin@example.com"
```

### Middleware Configuration

```yaml
middlewares:
  enabled: true
  recovery:
    enabled: true
    weight: 10
  logging:
    enabled: true
    weight: 20
    params:
      log_level: "info"
  auth:
    enabled: true
    weight: 70
    params:
      token: "your-secret-token"
  cors:
    enabled: true
    weight: 60
    params:
      AllowedOrigins: ["*"]
      AllowedMethods: ["GET", "POST", "PUT", "DELETE"]
```

### Cache Configuration

```yaml
cache:
  enabled: true
  type: "redis"  # or "memory"
  config:
    host: "localhost"
    port: 6379
    password: ""
    db: 0
```

### Metrics Configuration

```yaml
metrics:
  enabled: true
  type: "prometheus"  # or "memory"
  config:
    path: "/metrics"
    namespace: "myapp"
```

## Monitoring and Observability

### Health Checks

```bash
curl http://localhost:8080/health
```

```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T12:00:00Z",
  "uptime": "24h30m45s",
  "service": {
    "name": "my-service",
    "version": "1.0.0"
  },
  "checks": {
    "cache": {"status": "healthy"},
    "database": {"status": "healthy"}
  }
}
```

### Metrics

Visit `/metrics` for Prometheus metrics or `/stats` for JSON format.

### API Documentation

Visit `/docs` for interactive Swagger UI documentation.

## Advanced Usage

### Custom Middleware

```go
type CustomMiddleware struct {
    config types.ConfigManager
    logger types.Logger
}

func (m *CustomMiddleware) Name() string { return "custom" }
func (m *CustomMiddleware) Weight() int { return 25 }

func (m *CustomMiddleware) Handle(ctx *fasthttp.RequestCtx, next func(*fasthttp.RequestCtx), config *types.RouteConfig) {
    // Custom logic here
    next(ctx)
}

// Register middleware
middleware.RegisterMiddleware("custom", func() types.Middleware {
    return &CustomMiddleware{}
})
```

### Cron Jobs

```go
cron := sai.Cron()
err := cron.Add("cleanup", "0 2 * * *", func() {
    // Cleanup logic
    log.Println("Running cleanup job")
})
```

### WebSocket Actions

```go
actions := sai.Actions()

// Subscribe to events
actions.Subscribe("user.created", func(msg *types.ActionMessage) error {
    log.Printf("User created: %v", msg.Payload)
    return nil
})

// Publish events
actions.Publish("user.created", map[string]interface{}{
    "id": 123,
    "name": "John Doe",
})
```

## Testing

### Unit Tests

```go
func TestGetUser(t *testing.T) {
    // Test setup
    service, _ := service.NewService(context.Background(), "test-config.yml")
    
    // Test request
    req := fasthttp.AcquireRequest()
    resp := fasthttp.AcquireResponse()
    defer fasthttp.ReleaseRequest(req)
    defer fasthttp.ReleaseResponse(resp)
    
    req.SetRequestURI("http://localhost/api/v1/users/123")
    
    // Execute test
    service.Handler(req, resp)
    
    // Assertions
    assert.Equal(t, 200, resp.StatusCode())
}
```

### Integration Tests

Generated projects include comprehensive integration tests:

```bash
# Run all tests
make test

# Run integration tests
make test-integration

# Run with coverage
make test-coverage
```

## Deployment

### Docker

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o main ./cmd

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
COPY --from=builder /app/config.yml .
EXPOSE 8080
CMD ["./main"]
```

### Docker Compose

```yaml
version: '3.8'
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - ENV=production
    depends_on:
      - redis
      
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-service
  template:
    metadata:
      labels:
        app: my-service
    spec:
      containers:
      - name: my-service
        image: my-service:latest
        ports:
        - containerPort: 8080
        env:
        - name: ENV
          value: "production"
```

## Performance

SAI Service is optimized for high performance:

- **Throughput**: 100k+ RPS on modern hardware
- **Memory**: Zero-allocation patterns where possible
- **Latency**: Sub-millisecond response times
- **Scalability**: Horizontal scaling with load balancers

### Benchmarks

```bash
# Run benchmarks
go test -bench=. -benchmem ./...

# Load testing with hey
hey -n 10000 -c 100 http://localhost:8080/api/v1/users
```

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Setup

```bash
git clone https://github.com/saiset-co/sai-service.git
cd sai-service
go mod download
make test
```

### Code Style

- Follow Go naming conventions
- Use `gofmt` for formatting
- Add tests for new features
- Update documentation

## License

MIT License - see [LICENSE](LICENSE) file for details.

## RoadMap

1. Register health checker all modules 
2. Finish TLS autocert
3. Check all modules
- Actions
- Cache
- Client
- Cron
- Documents
- Health
- Metrics
- Middlewares
- Tls

**Made with ❤️ by the SAI Team**
