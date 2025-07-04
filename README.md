## ❤️ Health Manager# SAI Service Framework

🚀 **A powerful, production-ready Go framework for building high-performance microservices and APIs**

## Table of Contents

- [Project Description](#-project-description)
- [Quick Start](#-quick-start)
- [Manual Setup](#-manual-setup)
- [Global Access Objects](#-global-access-objects)
- [Configuration](#-configuration)
- [Data Handling and Error Management](#-data-handling-and-error-management)
- [Logger System](#-logger-system)
- [Basic CRUD API](#-basic-crud-api)
- [Authentication](#-authentication)
- [Cache System](#-cache-system)
- [Middleware](#-middleware)
- [Documentation Manager](#-documentation-manager)
- [Clients System](#-clients-system)
- [Event System](#-event-system)
- [Webhooks](#-webhooks)
- [Cron Jobs](#-cron-jobs)
- [Health Manager](#-health-manager)
- [Metrics Manager](#-metrics-manager)
- [TLS Manager](#-tls-manager)

## 📋 Project Description

SAI Service Framework is a comprehensive, enterprise-grade Go framework designed for building scalable, maintainable, and observable microservices. The framework provides a complete set of production-ready components that eliminate boilerplate code and allow developers to focus on business logic.

### Key Features:
- **Zero-config startup** - Works out of the box with sensible defaults
- **Modular architecture** - Enable only the components you need
- **Performance-first** - Built on FastHTTP for maximum throughput
- **Production-ready** - Comprehensive logging, metrics, and health checks
- **Developer-friendly** - Intuitive APIs and extensive documentation

## 🚀 Quick Start

The fastest way to get started is using our service generator:

```bash
# Clone the repository
git clone <repository-url>
cd sai-service-framework

# Make the generator executable
chmod +x generator.sh

# Run interactive generator
./generator.sh

# Follow the prompts to configure your service
```
More [GENERATOR DOCS](./GENERATOR.md)

### Generator Options

```bash
# Create a basic API service
./generator.sh --name "My API" --features "auth,cache,docs"

# Create a full-featured microservice
./generator.sh --name "User Service" --features "auth,cache,metrics,cron,actions,health"

# Create with specific configurations
./generator.sh \
  --name "Enterprise API" \
  --features "auth,cache,metrics,docs,tls" \
  --auth "token,basic" \
  --cache "redis" \
  --metrics "prometheus"
```

Generated project structure:
```
my-service/
├── cmd/main.go              # Entry point
├── internal/
│   ├── handlers.go          # HTTP handlers
│   └── service.go           # Business logic
├── .env.example             # Configuration
├── go.mod                   # Configuration
├── config.template.yml      # Configuration
├── docker-compose.yml       # Docker setup
├── Dockerfile               # Container image
├── Makefile                 # Build commands
└── README.md                # Project documentation
```

## 🔧 Manual Setup

### Installation

```bash
# Initialize new Go module
go mod init github.com/your-org/your-service

# Add SAI Service Framework
go get github.com/saiset-co/sai-service
```

### Basic Service Setup

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/saiset-co/sai-service/service"
    "github.com/saiset-co/sai-service/sai"
    "github.com/saiset-co/sai-service/types"
)

func main() {
    ctx := context.Background()
    
    // Create service with config file
    svc, err := service.NewService(ctx, "config.yml")
    if err != nil {
        log.Fatal(err)
    }
    
    // Setup routes
    setupRoutes()
    
    // Start service (non-blocking)
    if err := svc.Start(); err != nil {
        log.Fatal(err)
    }
}

func setupRoutes() {
    router := sai.Router()
    
    // Basic endpoint
    router.GET("/api/v1/hello", func(ctx *types.RequestCtx) {
        ctx.SuccessJSON(map[string]string{
            "message": "Hello, World!",
            "service": "SAI Service",
        })
    })
    
    // Protected endpoint with cache
    router.GET("/api/v1/data", func(ctx *types.RequestCtx) {
        data := map[string]interface{}{
            "timestamp": time.Now(),
            "data":      []string{"item1", "item2", "item3"},
        }
        ctx.SuccessJSON(data)
    }).WithMiddlewares("auth").WithCache("api_data", 5*time.Minute)
}
```

## 🌐 Global Access Objects

The framework provides convenient global access to all major components through the `sai` package:

### Available Global Objects

```go
import "github.com/saiset-co/sai-service/sai"

// Core components
router := sai.Router()           // HTTP router
logger := sai.Logger()           // Logger instance
config := sai.Config()           // Configuration manager

// Optional components (if enabled in config)
cache := sai.Cache()             // Cache manager (panic if disabled)
metrics := sai.Metrics()         // Metrics manager (panic if disabled)
cron := sai.Cron()              // Cron scheduler (panic if disabled)
actions := sai.Actions()         // Event broker (panic if disabled)
clientManager := sai.ClientManager() // HTTP clients (panic if disabled)

// Custom services (set by your application)
sai.Set("database", dbInstance)
sai.Set("emailService", emailSvc)

// Retrieve custom services
var db *sql.DB
if sai.Load("database", &db) {
    // Use database
}

// Check if service exists
if sai.Has("emailService") {
    emailSvc, _ := sai.Get("emailService")
    // Use email service
}
```

### Usage Examples

```go
func handleUser(ctx *types.RequestCtx) {
    // Log with global logger
    sai.Logger().Info("Processing user request",
        zap.String("user_id", ctx.UserValue("user_id").(string)))
    
    // Get from cache
    if data, found := sai.Cache().Get("user_data"); found {
        ctx.SuccessJSON(data)
        return
    }
    
    // Get configuration value
    maxRetries := sai.Config().GetValue("api.max_retries", 3).(int)
    
    // Record metrics
    counter := sai.Metrics().Counter("api_requests", map[string]string{
        "endpoint": "users",
    })
    counter.Inc()
    
    // Process request...
}
```

## ⚙️ Configuration

### Configuration Manager

The configuration system supports YAML files with environment variable substitution and type-safe access:

```go
// Get entire config
config := sai.Config().GetConfig()

// Get specific values with defaults
dbHost := sai.Config().GetValue("database.host", "localhost")
port := sai.Config().GetValue("server.http.port", 8080)

// Type-safe configuration reading
var dbConfig DatabaseConfig
err := sai.Config().GetAs("database", &dbConfig)
```

### Minimal Configuration

```yaml
# config.yml - Minimal working configuration
name: "My Service"
version: "1.0.0"
```

### Complete Configuration

```yaml
name: "Enterprise Service"           # Service name (required)
version: "2.0.0"                    # Service version (required)

server:
  http:
    host: "0.0.0.0"                 # Bind address
    port: 8080                      # HTTP port
    read_timeout: 30                # Read timeout in seconds
    write_timeout: 30               # Write timeout in seconds  
    idle_timeout: 120               # Keep-alive timeout in seconds
    shutdown_timeout: 15            # Graceful shutdown timeout
  tls:
    enabled: true                   # Enable HTTPS
    auto_cert: true                 # Use Let's Encrypt auto certificates
    domains: ["api.example.com"]    # Domains for auto certificates
    email: "admin@example.com"      # Let's Encrypt email
    cert_file: "/path/cert.pem"     # Manual certificate file
    key_file: "/path/key.pem"       # Manual private key file
    cache_dir: "./certs"            # Certificate cache directory

logger:
  level: "info"                     # Log level
  type: "default"                   # Logger type: default, custom
  config:                           # Logger-specific configuration
    format: "console"               # Format: console, json
    output: "stdout"                # Output: stdout, stderr, file
    file: "/var/log/service.log"    # Log file path (if output=file)

auth_providers:                     # Authentication providers
  token:                            # Token-based authentication
    params:
      token: "your-secret-token"    # API token
  basic:                            # Basic HTTP authentication
    params:
      username: "admin"             # Username
      password: "secure-password"   # Password

middlewares:                        # Middleware configuration
  enabled: true                     # Enable middleware system
  recovery:                         # Panic recovery middleware
    enabled: true                   # Enable recovery
    weight: 10                      # Execution order (lower = earlier)
    params:
      stack_trace: true             # Include stack trace in logs
  logging:                          # Request logging middleware
    enabled: true
    weight: 20
    params:
      log_level: "info"             # Log level for requests
      log_headers: false            # Log request headers
      log_body: false               # Log request/response body
  rate_limit:                       # Rate limiting middleware
    enabled: true
    weight: 30
    params:
      requests_per_minute: 100      # Max requests per minute per IP
  body_limit:                       # Request body size limiting
    enabled: true
    weight: 40
    params:
      max_body_size: 10485760       # Max body size in bytes (10MB)
  cors:                             # Cross-Origin Resource Sharing
    enabled: true
    weight: 50
    params:
      AllowedOrigins: ["*"]         # Allowed origins
      AllowedMethods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
      AllowedHeaders: ["Content-Type", "Authorization"]
      MaxAge: 86400                 # Preflight cache duration
  auth:                             # Authentication middleware
    enabled: true
    weight: 60
    params:
      token: "your-api-token"       # Default token
  compression:                      # Response compression
    enabled: true
    weight: 70
    params:
      algorithm: "gzip"             # Compression algorithm
      level: 6                      # Compression level (1-9)
      threshold: 1024               # Minimum response size to compress
  cache:                            # Response caching middleware
    enabled: true
    weight: 80
    params:
      default_ttl: "5m"             # Default cache TTL

cache:                              # Cache system
  enabled: true                     # Enable caching
  type: "redis"                     # Cache type: memory, redis, custom
  default_ttl: "1h"                 # Default TTL for cache entries
  config:                           # Cache-specific configuration
    host: "localhost:6379"          # Redis host:port
    password: ""                    # Redis password
    db: 0                          # Redis database number
    pool_size: 10                  # Connection pool size

metrics:                            # Metrics collection
  enabled: true                     # Enable metrics
  type: "prometheus"                # Metrics type: memory, prometheus, custom
  prefix: "myservice"               # Metrics prefix
  config:
    namespace: "myservice"          # Prometheus namespace
    subsystem: "api"                # Prometheus subsystem
  http:                             # Metrics HTTP endpoint
    enabled: true                   # Enable HTTP metrics endpoint
    path: "/metrics"                # Metrics endpoint path
    port: 9090                      # Metrics server port (0 = same as main)
  collectors:                       # Built-in collectors
    system: true                    # System metrics (CPU, memory)
    runtime: true                   # Go runtime metrics
    http: true                      # HTTP request metrics
    cache: true                     # Cache metrics
    middleware: true                # Middleware metrics

health:                             # Health check system
  enabled: true                     # Enable health checks

docs:                               # API documentation
  enabled: true                     # Enable OpenAPI/Swagger docs
  path: "/docs"                     # Documentation endpoint path

cron:                               # Cron job scheduler
  enabled: true                     # Enable cron scheduler
  timezone: "UTC"                   # Timezone for cron jobs

actions:                            # Event system
  enabled: true                     # Enable event system
  broker:                           # Event broker
    enabled: true                   # Enable broker
    type: "websocket"               # Broker type: websocket, custom
    config:                         # Broker-specific config
      port: 8081                    # WebSocket port
  webhooks:                         # Webhook system
    enabled: true                   # Enable webhooks
    config:
      max_retries: 3                # Max webhook delivery retries
      timeout: "30s"                # Webhook delivery timeout

clients:                            # HTTP client system
  enabled: true                     # Enable HTTP clients
  default_timeout: "30s"            # Default request timeout
  max_idle_connections: 100         # Max idle connections
  idle_conn_timeout: "90s"          # Idle connection timeout
  default_retries: 3                # Default retry count
  circuit_breaker:                  # Circuit breaker configuration
    enabled: true                   # Enable circuit breaker
    failure_threshold: 5            # Failures before opening circuit
    recovery_timeout: "60s"         # Time before attempting recovery
    half_open_requests: 3           # Requests in half-open state
  services:                         # External services
    user_service:                   # Service name
      url: "http://user-service:8080"  # Base URL
      auth:                         # Authentication config
        provider: "token"           # Auth provider to use
        payload:
          token: "service-token"    # Authentication token
      events: ["user.created"]      # Events to subscribe to
```

### Environment Variable Substitution

Configuration files support environment variable substitution in config.template.yml:

```yaml
database:
  host: "${DB_HOST:localhost}"      # Use DB_HOST env var, default to localhost
  port: "${DB_PORT:5432}"           # Use DB_PORT env var, default to 5432
  password: "${DB_PASSWORD}"        # Use DB_PASSWORD env var, required

cache:
  enabled: "${CACHE_ENABLED:true}"  # Use CACHE_ENABLED env var, default to true
```

## 📊 Data Handling and Error Management

The framework provides convenient methods for handling HTTP requests and responses:

### Response Methods

```go
func handleSuccess(ctx *types.RequestCtx) {
    // JSON response with 200 status
    data := map[string]interface{}{
        "id":   123,
        "name": "John Doe",
        "active": true,
    }
    ctx.SuccessJSON(data)
}

func handleCustomResponse(ctx *types.RequestCtx) {
    // Custom response with headers
    htmlData := []byte("<h1>Hello World</h1>")
    htmlHeader := []byte("text/html; charset=UTF-8")
    ctx.Success(htmlData, htmlHeader)
}

func handlePlainText(ctx *types.RequestCtx) {
    // Plain text response (uses default text/html header)
    textData := []byte("Plain text response")
    ctx.Success(textData, nil)
}
```

### Request Data Reading

```go
type UserRequest struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"min=0,max=150"`
}

func handleCreateUser(ctx *types.RequestCtx) {
    var req UserRequest
    
    // Read and unmarshal JSON request body
    if err := ctx.ReadJSON(&req); err != nil {
        ctx.Error(err, 400)
        return
    }
    
    // Process request...
    user := createUser(req)
    ctx.SuccessJSON(user)
}

// Alternative reading methods
func handleAlternativeReading(ctx *types.RequestCtx) {
    // Read raw body
    body := ctx.PostBody()
    
    // Manual unmarshaling
    var data map[string]interface{}
    if err := ctx.Unmarshal(body, &data); err != nil {
        ctx.Error(err, 400)
        return
    }
    
    // Manual marshaling
    response, err := ctx.Marshal(data)
    if err != nil {
        ctx.Error(err, 500)
        return
    }
    
    ctx.Success(response, []byte("application/json"))
}
```

### Error Handling

```go
func handleWithErrors(ctx *types.RequestCtx) {
    userID := string(ctx.QueryArgs().Peek("user_id"))
    if userID == "" {
        // Custom error with 400 status
        ctx.Error(types.NewError("user_id is required"), 400)
        return
    }
    
    user, err := getUserByID(userID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            // Not found error
            ctx.Error(types.NewError("user not found"), 404)
        } else {
            // Internal server error
            ctx.Error(types.WrapError(err, "failed to get user"), 500)
        }
        return
    }
    
    ctx.SuccessJSON(user)
}

// Error response format:
// {
//   "error": "Bad Request",
//   "message": "user_id is required"
// }
```

### Request Context Access

```go
func handleRequestInfo(ctx *types.RequestCtx) {
    // HTTP method
    method := string(ctx.Method())
    
    // Request path
    path := string(ctx.Path())
    
    // Query parameters
    limit := string(ctx.QueryArgs().Peek("limit"))
    
    // Headers
    authHeader := string(ctx.Request.Header.Peek("Authorization"))
    
    // User values (set by middleware)
    userID := ctx.UserValue("user_id")
    
    // Set response headers
    ctx.Response.Header.Set("X-Request-ID", generateRequestID())
    
    info := map[string]interface{}{
        "method":      method,
        "path":        path,
        "limit":       limit,
        "has_auth":    authHeader != "",
        "user_id":     userID,
    }
    
    ctx.SuccessJSON(info)
}
```

## 📝 Logger System

### Built-in Logger Usage

```go
func useLogger() {
    logger := sai.Logger()
    
    // Basic logging
    logger.Debug("Debug message")
    logger.Info("Info message")
    logger.Warn("Warning message")
    logger.Error("Error message")
    
    // Structured logging with fields
    logger.Info("User created",
        zap.String("user_id", "123"),
        zap.String("email", "user@example.com"),
        zap.Duration("processing_time", time.Millisecond*150))
    
    // Error logging with stack trace
    err := errors.New("something went wrong")
    logger.ErrorWithErrStack("Operation failed", err,
        zap.String("operation", "create_user"))
    
    // Custom log level
    logger.Log(zapcore.FatalLevel, "Fatal error occurred")
}

func handleRequestWithLogging(ctx *types.RequestCtx) {
    requestID := generateRequestID()
    
    sai.Logger().Info("Request started",
        zap.String("request_id", requestID),
        zap.String("method", string(ctx.Method())),
        zap.String("path", string(ctx.Path())))
    
    // Process request...
    
    sai.Logger().Info("Request completed",
        zap.String("request_id", requestID),
        zap.Int("status", 200))
}
```

### Custom Logger Implementation

```go
// Create custom logger
type CustomLogger struct {
    zapLogger *zap.Logger
    service   string
}

func NewCustomLogger(service string) types.Logger {
    config := zap.NewProductionConfig()
    config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
    
    zapLogger, _ := config.Build()
    
    return &CustomLogger{
        zapLogger: zapLogger,
        service:   service,
    }
}

func (c *CustomLogger) Info(msg string, fields ...zap.Field) {
    // Add service field to all logs
    allFields := append(fields, zap.String("service", c.service))
    c.zapLogger.Info(msg, allFields...)
}

func (c *CustomLogger) Error(msg string, fields ...zap.Field) {
    allFields := append(fields, zap.String("service", c.service))
    c.zapLogger.Error(msg, allFields...)
}

// Implement other required methods...

// Register custom logger
func init() {
    logger.RegisterLogger("custom", func(config interface{}) (types.Logger, error) {
        // Parse config and create logger
        return NewCustomLogger("my-service"), nil
    })
}
```

Configuration for custom logger:
```yaml
logger:
  type: "custom"
  level: "info"
  config:
    service_name: "my-service"
    output_format: "json"
```

## 🎯 Basic CRUD API

The middleware system applies all enabled middleware to routes by default. You can disable specific middleware for groups or individual routes, and re-enable them as needed.

### Default Middleware Behavior

```go
func setupCRUDAPI() {
    // All enabled middleware apply to all routes by default
    router := sai.Router()
    
    // API group - disable auth for public endpoints
    api := router.Group("/api/v1").
        WithoutMiddlewares("auth")  // Disable auth for entire group
    
    // Public endpoints (no auth required)
    api.GET("/status", handleStatus)
    api.POST("/register", handleRegister)
    
    // Users group - re-enable auth for protected endpoints
    users := api.Group("/users").
        WithMiddlewares("auth")  // Re-enable auth for users group
    
    users.POST("/", createUser).
        WithDoc("Create User", "Creates a new user", "users", CreateUserRequest{}, User{})
    
    users.GET("/", listUsers).
        WithCache("users_list", 5*time.Minute, "users").
        WithDoc("List Users", "Returns paginated list of users", "users", nil, []User{})
    
    users.GET("/{id}", getUser).
        WithDoc("Get User", "Returns user by ID", "users", nil, User{})
    
    users.PUT("/{id}", updateUser).
        WithDoc("Update User", "Updates existing user", "users", UpdateUserRequest{}, User{})
    
    users.DELETE("/{id}", deleteUser).
        WithoutMiddlewares("cache").  // Disable cache for delete operations
        WithDoc("Delete User", "Deletes user by ID", "users", nil, nil)
        
    // Admin endpoints - additional middleware
    admin := api.Group("/admin").
        WithMiddlewares("auth", "rate_limit").  // Enable auth and rate limiting
        WithTimeout(30 * time.Second)
    
    admin.GET("/stats", getAdminStats)
    admin.POST("/maintenance", enableMaintenance)
}
```

### CRUD Implementation

```go
type User struct {
    ID       string    `json:"id" doc:"User unique identifier"`
    Name     string    `json:"name" doc:"Full name" validate:"required"`
    Email    string    `json:"email" doc:"Email address" validate:"required,email"`
    Active   bool      `json:"active" doc:"Account status"`
    Created  time.Time `json:"created" doc:"Creation timestamp"`
    Updated  time.Time `json:"updated" doc:"Last update timestamp"`
}

type CreateUserRequest struct {
    Name  string `json:"name" validate:"required" doc:"User full name"`
    Email string `json:"email" validate:"required,email" doc:"User email address"`
}

type UpdateUserRequest struct {
    Name   *string `json:"name,omitempty" doc:"User full name"`
    Email  *string `json:"email,omitempty" validate:"omitempty,email" doc:"User email"`
    Active *bool   `json:"active,omitempty" doc:"Account active status"`
}

type ListUsersRequest struct {
    Page     int    `query:"page" doc:"Page number" example:"1"`
    Limit    int    `query:"limit" doc:"Items per page" example:"20"`
    Search   string `query:"search" doc:"Search term"`
    Active   *bool  `query:"active" doc:"Filter by active status"`
}

func createUser(ctx *types.RequestCtx) {
    var req CreateUserRequest
    if err := ctx.ReadJSON(&req); err != nil {
        ctx.Error(types.WrapError(err, "invalid request body"), 400)
        return
    }
    
    // Check if user exists
    if userExists(req.Email) {
        ctx.Error(types.NewError("user with this email already exists"), 409)
        return
    }
    
    user := &User{
        ID:      generateID(),
        Name:    req.Name,
        Email:   req.Email,
        Active:  true,
        Created: time.Now(),
        Updated: time.Now(),
    }
    
    if err := saveUser(user); err != nil {
        sai.Logger().Error("Failed to save user", 
            zap.Error(err),
            zap.String("email", req.Email))
        ctx.Error(types.WrapError(err, "failed to create user"), 500)
        return
    }
    
    // Invalidate cache
    sai.Cache().Invalidate("users")
    
    // Publish event
    sai.Actions().Publish("user.created", map[string]interface{}{
        "user_id": user.ID,
        "email":   user.Email,
    })
    
    sai.Logger().Info("User created",
        zap.String("user_id", user.ID),
        zap.String("email", user.Email))
    
    ctx.SuccessJSON(user)
}

func listUsers(ctx *types.RequestCtx) {
    var req ListUsersRequest
    
    // Parse query parameters
    req.Page = parseInt(string(ctx.QueryArgs().Peek("page")), 1)
    req.Limit = parseInt(string(ctx.QueryArgs().Peek("limit")), 20)
    req.Search = string(ctx.QueryArgs().Peek("search"))
    
    if activeStr := string(ctx.QueryArgs().Peek("active")); activeStr != "" {
        if active, err := strconv.ParseBool(activeStr); err == nil {
            req.Active = &active
        }
    }
    
    // Validate pagination
    if req.Page < 1 {
        req.Page = 1
    }
    if req.Limit < 1 || req.Limit > 100 {
        req.Limit = 20
    }
    
    users, total, err := getUsersList(req)
    if err != nil {
        sai.Logger().Error("Failed to get users list", zap.Error(err))
        ctx.Error(types.WrapError(err, "failed to get users"), 500)
        return
    }
    
    response := map[string]interface{}{
        "users":      users,
        "total":      total,
        "page":       req.Page,
        "limit":      req.Limit,
        "total_pages": (total + req.Limit - 1) / req.Limit,
    }
    
    ctx.SuccessJSON(response)
}

func getUser(ctx *types.RequestCtx) {
    userID := ctx.UserValue("id").(string)
    
    user, err := getUserByID(userID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            ctx.Error(types.NewError("user not found"), 404)
        } else {
            sai.Logger().Error("Failed to get user", 
                zap.Error(err),
                zap.String("user_id", userID))
            ctx.Error(types.WrapError(err, "failed to get user"), 500)
        }
        return
    }
    
    ctx.SuccessJSON(user)
}

func updateUser(ctx *types.RequestCtx) {
    userID := ctx.UserValue("id").(string)
    
    var req UpdateUserRequest
    if err := ctx.Read(&req); err != nil {
        ctx.Error(types.WrapError(err, "invalid request body"), 400)
        return
    }
    
    user, err := getUserByID(userID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            ctx.Error(types.NewError("user not found"), 404)
        } else {
            ctx.Error(types.WrapError(err, "failed to get user"), 500)
        }
        return
    }
    
    // Update fields
    if req.Name != nil {
        user.Name = *req.Name
    }
    if req.Email != nil {
        user.Email = *req.Email
    }
    if req.Active != nil {
        user.Active = *req.Active
    }
    user.Updated = time.Now()
    
    if err := saveUser(user); err != nil {
        sai.Logger().Error("Failed to update user",
            zap.Error(err),
            zap.String("user_id", userID))
        ctx.Error(types.WrapError(err, "failed to update user"), 500)
        return
    }
    
    // Invalidate cache
    sai.Cache().Invalidate("users")
    
    // Publish event
    sai.Actions().Publish("user.updated", map[string]interface{}{
        "user_id": user.ID,
        "changes": req,
    })
    
    ctx.SuccessJSON(user)
}

func deleteUser(ctx *types.RequestCtx) {
    userID := ctx.UserValue("id").(string)
    
    if err := deleteUserByID(userID); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            ctx.Error(types.NewError("user not found"), 404)
        } else {
            sai.Logger().Error("Failed to delete user",
                zap.Error(err),
                zap.String("user_id", userID))
            ctx.Error(types.WrapError(err, "failed to delete user"), 500)
        }
        return
    }
    
    // Invalidate cache
    sai.Cache().Invalidate("users")
    
    // Publish event
    sai.Actions().Publish("user.deleted", map[string]interface{}{
        "user_id": userID,
    })
    
    ctx.SuccessJSON(map[string]string{
        "message": "user deleted successfully",
    })
}
```

## 🔐 Authentication

The framework provides a flexible authentication system with multiple providers and middleware integration.

### Built-in Auth Providers

Just an auth provider type description section, does not enable auth

#### Token Authentication

```yaml
auth_providers:
  token:
    params:
      token: "your-secret-api-token"
```

```go
func setupTokenAuth() {
    // Token can be sent in multiple ways:
    // 1. Authorization header: "Bearer your-token"
    // 2. Authorization header: "Token your-token"  
    // 3. Authorization header: "your-token"
    // 4. Token header: "your-token"
    
    router := sai.Router()
    
    // Protected endpoint
    router.GET("/api/protected", func(ctx *types.RequestCtx) {
        // User info is available after auth middleware
        userInfo := ctx.UserValue("auth_type")  // "token"
        
        ctx.SuccessJSON(map[string]interface{}{
            "message":   "Access granted",
            "auth_type": userInfo,
        })
    }).WithMiddlewares("auth")
}
```

#### Basic Authentication
```yaml
auth_providers:
  basic:
    params:
      username: "admin"
      password: "secure-password"
```

```go
func setupBasicAuth() {
    router := sai.Router()
    
    router.GET("/api/admin", func(ctx *types.RequestCtx) {
        // User info available after auth
        username := ctx.UserValue("authenticated_user").(string)
        authType := ctx.UserValue("auth_type").(string)
        
        ctx.SuccessJSON(map[string]interface{}{
            "message":  "Admin access granted",
            "username": username,
            "auth_type": authType,  // "basic"
        })
    }).WithMiddlewares("auth")
}
```

### Custom Auth Provider

```go
// Custom JWT auth provider
type JWTAuthProvider struct {
    secretKey []byte
    realm     string
}

func NewJWTAuthProvider(secretKey []byte) *JWTAuthProvider {
    return &JWTAuthProvider{
        secretKey: secretKey,
        realm:     "Protected Area",
    }
}

func (p *JWTAuthProvider) Type() string {
    return "jwt"
}

func (p *JWTAuthProvider) ApplyToIncomingRequest(ctx *types.RequestCtx) error {
    authHeader := string(ctx.Request.Header.Peek("Authorization"))
    if authHeader == "" {
        return p.sendAuthChallenge(ctx, "Authorization header required")
    }
    
    if !strings.HasPrefix(authHeader, "Bearer ") {
        return p.sendAuthChallenge(ctx, "Bearer token required")
    }
    
    tokenString := strings.TrimPrefix(authHeader, "Bearer ")
    
    // Parse and validate JWT token
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method")
        }
        return p.secretKey, nil
    })
    
    if err != nil || !token.Valid {
        return p.sendAuthChallenge(ctx, "Invalid token")
    }
    
    if claims, ok := token.Claims.(jwt.MapClaims); ok {
        ctx.SetUserValue("authenticated_user", claims["sub"])
        ctx.SetUserValue("user_claims", claims)
        ctx.SetUserValue("auth_type", "jwt")
    }
    
    return nil
}

func (p *JWTAuthProvider) ApplyToOutgoingRequest(req *fasthttp.Request, authConfig *types.ServiceAuthConfig) error {
    if authConfig == nil || authConfig.Payload == nil {
        return errors.New("auth config required for JWT")
    }
    
    token, ok := authConfig.Payload["token"].(string)
    if !ok {
        return errors.New("JWT token not found in auth payload")
    }
    
    req.Header.Set("Authorization", "Bearer "+token)
    return nil
}

func (p *JWTAuthProvider) sendAuthChallenge(ctx *types.RequestCtx, message string) error {
    ctx.SetStatusCode(fasthttp.StatusUnauthorized)
    ctx.Response.Header.Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s"`, p.realm))
    
    response := map[string]interface{}{
        "error":   "Authentication Required",
        "message": message,
        "type":    "bearer_auth_challenge",
    }
    
    ctx.SuccessJSON(response)
    return errors.New("jwt_auth_challenge_sent")
}

// Register custom provider
func setupCustomAuth() {
    authProvider := sai.AuthProvider()
    jwtProvider := NewJWTAuthProvider([]byte("your-jwt-secret"))
    
    authProvider.Register("jwt", jwtProvider)
}
```

### Auth Middleware Configuration

Using for protect incoming requests. Enable auth for all routes.

```yaml
middlewares:
  auth:
    enabled: true
    weight: 60  # Execute after CORS, rate limiting, etc.
    params:
      provider: "token" # Provider type
```

### Route-level Auth Control

```go
func setupAuthRoutes() {
    router := sai.Router()
    
    // Public routes (no auth)
    public := router.Group("/api/public").
        WithoutMiddlewares("auth")
    
    public.GET("/status", handleStatus)
    public.POST("/register", handleRegister)
    
    // Protected routes (auth required)
    protected := router.Group("/api/protected").
        WithMiddlewares("auth")
    
    protected.GET("/profile", handleProfile)
    protected.PUT("/profile", handleUpdateProfile)
    
    // Admin routes (auth + additional checks)
    admin := router.Group("/api/admin").
        WithMiddlewares("auth")
    
    admin.GET("/users", func(ctx *types.RequestCtx) {
        // Additional authorization check
        claims := ctx.UserValue("user_claims").(jwt.MapClaims)
        role, ok := claims["role"].(string)
        if !ok || role != "admin" {
            ctx.Error(types.NewError("insufficient permissions"), 403)
            return
        }
        
        // Admin logic...
        ctx.SuccessJSON(map[string]string{"message": "Admin access granted"})
    })
}
```

## 💾 Cache System

The framework provides a flexible caching system with multiple backends and middleware integration.

### Cache Configuration

Enable cache manager. Does not enable cache on routes in this place.

```yaml
cache:
  enabled: true
  type: "redis"        # memory, redis, custom
  default_ttl: "1h"    # Default TTL for cache entries
  config:
    host: "localhost:6379"
    password: ""
    db: 0
    pool_size: 10
    max_retries: 3
    retry_delay: "1s"
```

### Programmatic Cache Usage

```go
func useCacheDirectly() {
    cache := sai.Cache()
    
    // Set cache entry
    cache.Set("user:123", userData, 15*time.Minute)
    
    // Get cache entry
    if data, found := cache.Get("user:123"); found {
        user := data.(*User)
        // Use cached data
    }
    
    // Delete specific key
    cache.Delete("user:123")
    
    // Invalidate multiple keys
    cache.Invalidate("users", "user:123", "stats:daily")
    
    // Cache with dependencies
    cache.Set("user_stats", statsData, time.Hour)
    // When user data changes, invalidate dependent caches
    cache.Invalidate("user_stats")
}

func handleCachedData(ctx *types.RequestCtx) {
    userID := ctx.UserValue("id").(string)
    cacheKey := fmt.Sprintf("user:%s", userID)
    
    // Try cache first
    if userData, found := sai.Cache().Get(cacheKey); found {
        sai.Logger().Debug("Cache hit", zap.String("key", cacheKey))
        ctx.SuccessJSON(userData)
        return
    }
    
    // Cache miss - fetch from database
    user, err := getUserByID(userID)
    if err != nil {
        ctx.Error(err, 500)
        return
    }
    
    // Cache the result
    sai.Cache().Set(cacheKey, user, 10*time.Minute)
    
    sai.Logger().Debug("Cache miss - data cached", zap.String("key", cacheKey))
    ctx.SuccessJSON(user)
}
```

### Cache Middleware

Does not enable cache for routes here. Allow to configure per route cache configuration.

```yaml
middlewares:
  cache:
    enabled: true
    weight: 80  # Execute late in the chain
    params:
      default_ttl: "5m"
      cache_private: false
      cache_public: true
```

Route cache parameters.

```go
func setupCacheMiddleware() {
    router := sai.Router()
    
    // Cache response for 5 minutes
    router.GET("/api/users", listUsers).
        WithCache("users_list", 5*time.Minute)
    
    // Cache with dependencies - invalidated when users change
    router.GET("/api/users/{id}", getUser).
        WithCache("user_detail", 15*time.Minute, "users")
    
    // Dynamic cache key
    router.GET("/api/users/{id}/posts", func(ctx *types.RequestCtx) {
        userID := ctx.UserValue("id").(string)
        
        // Cache key will include user ID
        posts := getUserPosts(userID)
        ctx.SuccessJSON(posts)
    }).WithCache("user_posts_{id}", 10*time.Minute, "posts", "users")
    
    // No cache for this endpoint
    router.POST("/api/users", createUser).
        WithoutMiddlewares("cache")
}
```

### Custom Cache Provider

```go
// Custom cache implementation
type RedisClusterCache struct {
    client *redis.ClusterClient
    logger types.Logger
}

func NewRedisClusterCache(addrs []string, password string, logger types.Logger) *RedisClusterCache {
    client := redis.NewClusterClient(&redis.ClusterOptions{
        Addrs:    addrs,
        Password: password,
    })
    
    return &RedisClusterCache{
        client: client,
        logger: logger,
    }
}

func (c *RedisClusterCache) Get(key string) (interface{}, bool) {
    val, err := c.client.Get(context.Background(), key).Result()
    if err == redis.Nil {
        return nil, false
    }
    if err != nil {
        c.logger.Error("Cache get error", zap.Error(err), zap.String("key", key))
        return nil, false
    }
    
    var data interface{}
    if err := json.Unmarshal([]byte(val), &data); err != nil {
        c.logger.Error("Cache unmarshal error", zap.Error(err))
        return nil, false
    }
    
    return data, true
}

func (c *RedisClusterCache) Set(key string, value interface{}, ttl time.Duration) error {
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }
    
    return c.client.Set(context.Background(), key, data, ttl).Err()
}

func (c *RedisClusterCache) Delete(key string) error {
    return c.client.Del(context.Background(), key).Err()
}

func (c *RedisClusterCache) Invalidate(keys ...string) error {
    if len(keys) == 0 {
        return nil
    }
    return c.client.Del(context.Background(), keys...).Err()
}

// Implement other required methods...

// Register custom cache provider
func init() {
    cache.RegisterCacheManager("redis-cluster", func(config interface{}) (types.CacheManager, error) {
        cfg := config.(map[string]interface{})
        addrs := cfg["addrs"].([]string)
        password := cfg["password"].(string)
        
        return NewRedisClusterCache(addrs, password, sai.Logger()), nil
    })
}
```

Configuration for custom cache:
```yaml
cache:
  enabled: true
  type: "redis-cluster"
  config:
    addrs: ["localhost:7000", "localhost:7001", "localhost:7002"]
    password: ""
```

## 🚧 Middleware

The framework includes a comprehensive middleware system with built-in components and support for custom middleware.

### Recovery Middleware

Handles panics:

```yaml
middlewares:
  recovery:
    enabled: true
    weight: 10  # Execute first
    params:
      stack_trace: true      # Include stack trace in logs
      log_panics: true       # Log panic details
      include_request: true  # Include request details in logs
```

```go
// Recovery middleware captures panics automatically
func handlePanic(ctx *types.RequestCtx) {
    // This will be caught by recovery middleware
    panic("something went wrong")
    
    // Recovery middleware will:
    // 1. Log the panic with stack trace
    // 2. Return 500 Internal Server Error
    // 3. Continue processing other requests
}
```

### Logging Middleware

Logs all HTTP requests and responses:

```yaml
middlewares:
  logging:
    enabled: true
    weight: 20
    params:
      log_level: "info"       # Log level for requests
      log_headers: false      # Log request headers
      log_body: false         # Log request/response body
      log_response: true      # Log response details
```

### Rate Limiting Middleware

Implements rate limiting per IP address:

```yaml
middlewares:
  rate_limit:
    enabled: true
    weight: 30
    params:
      requests_per_minute: 100  # Max requests per minute per IP
      burst: 10                 # Burst capacity
      cleanup_interval: "1m"    # Cleanup interval for old entries
```

```go
// Rate limiting is applied automatically
// Returns 429 Too Many Requests when limit exceeded
func setupRateLimiting() {
    router := sai.Router()
    
    // Different rate limits for different endpoints
    router.GET("/api/public", handlePublic).
        WithoutMiddlewares("rate_limit")  // No rate limiting
    
    router.POST("/api/upload", handleUpload).
        WithMiddlewares("rate_limit")     // Apply rate limiting
}
```

### Body Limit Middleware

Limits request body size:

```yaml
middlewares:
  body_limit:
    enabled: true
    weight: 40
    params:
      max_body_size: 10485760  # 10MB in bytes
      skip_content_length: false
```

### CORS Middleware

Handles Cross-Origin Resource Sharing:

```yaml
middlewares:
  cors:
    enabled: true
    weight: 50
    params:
      AllowedOrigins: ["*"]
      AllowedMethods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
      AllowedHeaders: ["Content-Type", "Authorization", "X-API-Key"]
      ExposedHeaders: ["X-Request-ID"]
      AllowCredentials: true
      MaxAge: 86400  # Preflight cache duration in seconds
```

### Compression Middleware

Compresses HTTP responses:

```yaml
middlewares:
  compression:
    enabled: true
    weight: 70
    params:
      algorithm: "gzip"       # Compression algorithm
      level: 6                # Compression level (1-9)
      threshold: 1024         # Minimum response size to compress
      allowed_types:          # Content types to compress
        - "application/json"
        - "text/html"
        - "text/plain"
        - "application/xml"
      exclude_extensions: [".jpg", ".png", ".gif"]
```

### Custom Middleware

```go
// Request ID middleware
type RequestIDMiddleware struct {
    logger types.Logger
}

func NewRequestIDMiddleware(logger types.Logger) *RequestIDMiddleware {
    return &RequestIDMiddleware{logger: logger}
}

func (m *RequestIDMiddleware) Name() string {
    return "request-id"
}

func (m *RequestIDMiddleware) Weight() int {
    return 5  // Execute very early
}

func (m *RequestIDMiddleware) Handle(ctx *types.RequestCtx, next func(*types.RequestCtx), config *types.RouteConfig) {
    // Generate request ID
    requestID := generateRequestID()
    
    // Store in context
    ctx.SetUserValue("request_id", requestID)
    
    // Add to response headers
    ctx.Response.Header.Set("X-Request-ID", requestID)
    
    m.logger.Debug("Request started",
        zap.String("request_id", requestID),
        zap.String("method", string(ctx.Method())),
        zap.String("path", string(ctx.Path())))
    
    start := time.Now()
    
    // Continue to next middleware
    next(ctx)
    
    duration := time.Since(start)
    statusCode := ctx.Response.StatusCode()
    
    m.logger.Info("Request completed",
        zap.String("request_id", requestID),
        zap.Int("status", statusCode),
        zap.Duration("duration", duration))
}

// Register middleware (before service starts)
func registerCustomMiddleware() {
    middlewareManager := getMiddlewareManager() // Get from service initialization
    middlewareManager.Register(NewRequestIDMiddleware(sai.Logger()))
}
```

## 📚 Documentation Manager

### Automatic Documentation Generation

```go
func setupDocumentedAPI() {
    api := sai.Router().Group("/api/v1")
    
    // Document with request/response types
    api.POST("/users", createUser).
        WithDoc(
            "Create User",                    // Title
            "Creates a new user account",     // Description
            "users",                         // Tag for grouping
            CreateUserRequest{},             // Request body type
            User{},                          // Response type
        )
    
    // Document with query parameters
    api.GET("/users", listUsers).
        WithDoc(
            "List Users",
            "Returns a paginated list of users with optional filtering",
            "users",
            ListUsersQuery{},  // Query parameters type
            UserListResponse{}, // Response type
        )
    
    // Document path parameters
    api.GET("/users/{id}", getUser).
        WithDoc(
            "Get User",
            "Returns user details by ID",
            "users",
            nil,    // No request body
            User{}, // Response type
        )
}
```

### Documentation with Struct Tags

```go
type CreateUserRequest struct {
    Name     string `json:"name" validate:"required" doc:"User's full name" example:"John Doe"`
    Email    string `json:"email" validate:"required,email" doc:"User's email address" example:"john@example.com"`
    Age      int    `json:"age" validate:"min=0,max=150" doc:"User's age" example:"30"`
    Active   bool   `json:"active" doc:"Whether the user account is active" example:"true"`
    Tags     []string `json:"tags" doc:"User tags" example:"admin,premium"`
    Metadata map[string]interface{} `json:"metadata" doc:"Additional user metadata"`
}

type User struct {
    ID       string    `json:"id" doc:"Unique user identifier" example:"usr_123456"`
    Name     string    `json:"name" doc:"User's full name"`
    Email    string    `json:"email" doc:"User's email address"`
    Age      int       `json:"age" doc:"User's age"`
    Active   bool      `json:"active" doc:"Account status"`
    Created  time.Time `json:"created" doc:"Account creation timestamp"`
    Updated  time.Time `json:"updated" doc:"Last update timestamp"`
}

type UserListResponse struct {
    Users      []User `json:"users" doc:"List of users"`
    Total      int    `json:"total" doc:"Total number of users"`
    Page       int    `json:"page" doc:"Current page number"`
    Limit      int    `json:"limit" doc:"Items per page"`
    TotalPages int    `json:"total_pages" doc:"Total number of pages"`
}

type ListUsersQuery struct {
    Page   int    `query:"page" doc:"Page number for pagination" example:"1"`
    Limit  int    `query:"limit" doc:"Number of items per page" example:"20"`
    Search string `query:"search" doc:"Search term for filtering users" example:"john"`
    Active *bool  `query:"active" doc:"Filter by account status" example:"true"`
}
```

### Accessing Documentation

Once configured, documentation is automatically available at:
- `/docs` - Swagger UI interface, see config section
- `/openapi.json` - OpenAPI specification in JSON format

The documentation includes:
- All documented endpoints
- Request/response schemas
- Parameter descriptions
- Example values
- Authentication requirements
- Error responses

## 🌐 Clients System

The framework provides a robust HTTP client system with circuit breakers, retries, and service discovery.

### Configuration

```yaml
clients:
  enabled: true
  default_timeout: "30s"
  max_idle_connections: 100
  idle_conn_timeout: "90s"
  default_retries: 3
  circuit_breaker:
    enabled: true
    failure_threshold: 5
    recovery_timeout: "60s"
    half_open_requests: 3
  services:
    user_service:
      url: "http://user-service:8080"
      auth:
        provider: "token"
        payload:
          token: "service-to-service-token"
      events: ["user.created", "user.updated"]
    notification_service:
      url: "http://notification-service:8080"
      auth:
        provider: "basic"
        payload:
          username: "service"
          password: "secret"
```

### Using HTTP Clients

```go
func useHTTPClients(ctx *types.RequestCtx) {
    clientManager := sai.ClientManager()
    
    // Simple GET request
    response, statusCode, err := clientManager.Call(
        "user_service",           // Service name
        "GET",                    // HTTP method
        "/api/v1/users/123",      // Path
        nil,                      // Request body
        nil,                      // Options
    )
    
    if err != nil {
        sai.Logger().Error("Failed to call user service", zap.Error(err))
        return
    }
    
    if statusCode == 200 {
        var user User
		ctx.Unmarshal(response, &user)
        // Use user data
    }
}

func callWithOptions(ctx *types.RequestCtx) {
    clientManager := sai.ClientManager()
    
    // POST request with custom options
    requestData := map[string]interface{}{
        "name":  "John Doe",
        "email": "john@example.com",
    }
    
    options := &types.CallOptions{
        Headers: map[string]string{
            "X-Request-ID": "req-123",
            "X-Source":     "api-gateway",
        },
        Timeout: 45 * time.Second,
        Retry:   5,
    }
    
    response, statusCode, err := clientManager.Call(
        "user_service",
        "POST",
        "/api/v1/users",
        requestData,
        options,
    )
    
    if err != nil {
        // Handle error (could be network, timeout, or HTTP error)
        sai.Logger().Error("User creation failed",
            zap.Error(err),
            zap.Int("status_code", statusCode))
        return
    }
    
    if statusCode == 201 {
        var newUser User
		ctx.Unmarshal(response, &newUser)
        // User created successfully
    }
}
```

### Circuit Breaker

The client system includes automatic circuit breaker functionality:

```go
func handleCircuitBreaker() {
    // Circuit breaker states:
    // 1. Closed: Normal operation
    // 2. Open: Service is down, requests fail fast
    // 3. Half-Open: Testing if service recovered
    
    for i := 0; i < 10; i++ {
        response, statusCode, err := sai.ClientManager().Call(
            "unreliable_service",
            "GET",
            "/api/data",
            nil,
            nil,
        )
        
        if err != nil {
            if strings.Contains(err.Error(), "circuit breaker") {
                sai.Logger().Warn("Circuit breaker is open for unreliable_service")
                // Implement fallback logic
                handleFallback()
                continue
            }
            // Handle other errors
        }
        
        // Process successful response
        handleResponse(response, statusCode)
    }
}

func handleFallback() {
    // Implement fallback logic when service is unavailable
    // - Return cached data
    // - Use alternative service
    // - Return default response
}
```

## 🔄 Event System

The framework provides a powerful event system supporting WebSocket and custom brokers.

### Configuration

```yaml
actions:
  enabled: true
  broker:
    enabled: true
    type: "websocket"
    config:
      port: 8081              # WebSocket server port
      path: "/ws"             # WebSocket endpoint path
      max_connections: 1000   # Maximum concurrent connections
      read_buffer_size: 1024  # Read buffer size
      write_buffer_size: 1024 # Write buffer size
  webhooks:
    enabled: true
    config:
      max_retries: 3
      timeout: "30s"
```

### Publishing Events

```go
func publishEvents() {
    actions := sai.Actions()
    
    // Simple event
    err := actions.Publish("user.created", map[string]interface{}{
        "user_id": "123",
        "email":   "user@example.com",
        "timestamp": time.Now(),
    })
    
    if err != nil {
        sai.Logger().Error("Failed to publish event", zap.Error(err))
    }
    
    // Complex event with metadata
    eventData := map[string]interface{}{
        "order_id":    "ord_123456",
        "customer_id": "cust_789",
        "amount":      99.99,
        "currency":    "USD",
        "items": []map[string]interface{}{
            {"id": "item_1", "quantity": 2, "price": 29.99},
            {"id": "item_2", "quantity": 1, "price": 39.99},
        },
    }
    
    actions.Publish("order.completed", eventData)
}

// Publish from HTTP handlers
func handleCreateOrder(ctx *types.RequestCtx) {
    var req CreateOrderRequest
    if err := ctx.ReadJSON(&req); err != nil {
        ctx.Error(err, 400)
        return
    }
    
    // Process order
    order, err := processOrder(req)
    if err != nil {
        ctx.Error(err, 500)
        return
    }
    
    // Publish event asynchronously
    go func() {
        sai.Actions().Publish("order.created", map[string]interface{}{
            "order_id":    order.ID,
            "customer_id": order.CustomerID,
            "amount":      order.Amount,
            "status":      order.Status,
        })
    }()
    
    ctx.SuccessJSON(order)
}
```

### Subscribing to Events

```go
func setupEventHandlers() {
    actions := sai.Actions()
    
    // Subscribe to user events
    actions.Subscribe("user.created", handleUserCreated)
    actions.Subscribe("user.updated", handleUserUpdated)
    actions.Subscribe("user.deleted", handleUserDeleted)
    
    // Subscribe to order events
    actions.Subscribe("order.created", handleOrderCreated)
    actions.Subscribe("order.completed", handleOrderCompleted)
    actions.Subscribe("order.cancelled", handleOrderCancelled)
}

func handleUserCreated(msg *types.ActionMessage) error {
    sai.Logger().Info("User created event received",
        zap.String("action", msg.Action),
        zap.Time("timestamp", msg.Timestamp))
    
    // Extract user data
    userData := msg.Payload.(map[string]interface{})
    userID := userData["user_id"].(string)
    email := userData["email"].(string)
    
    // Send welcome email
    if err := sendWelcomeEmail(userID, email); err != nil {
        sai.Logger().Error("Failed to send welcome email",
            zap.Error(err),
            zap.String("user_id", userID))
        return err
    }
    
    // Update analytics
    updateUserMetrics("created")
    
    // Cache user data
    sai.Cache().Set(fmt.Sprintf("user:%s", userID), userData, time.Hour)
    
    return nil
}

func handleOrderCompleted(msg *types.ActionMessage) error {
    orderData := msg.Payload.(map[string]interface{})
    orderID := orderData["order_id"].(string)
    customerID := orderData["customer_id"].(string)
    
    // Generate invoice
    if err := generateInvoice(orderID); err != nil {
        return err
    }
    
    // Update inventory
    if err := updateInventory(orderData); err != nil {
        return err
    }
    
    // Send confirmation email
    if err := sendOrderConfirmation(customerID, orderID); err != nil {
        return err
    }
    
    // Trigger fulfillment
    sai.Actions().Publish("fulfillment.requested", map[string]interface{}{
        "order_id":    orderID,
        "customer_id": customerID,
        "priority":    "normal",
    })
    
    return nil
}
```

### Custom Event Broker

```go
// Custom Redis-based event broker
type RedisEventBroker struct {
    client      *redis.Client
    logger      types.Logger
    subscribers map[string][]types.ActionHandler
    mu          sync.RWMutex
    ctx         context.Context
    cancel      context.CancelFunc
}

func NewRedisEventBroker(redisURL string, logger types.Logger) *RedisEventBroker {
    opt, err := redis.ParseURL(redisURL)
    if err != nil {
        logger.Error("Failed to parse Redis URL", zap.Error(err))
        return nil
    }
    
    client := redis.NewClient(opt)
    ctx, cancel := context.WithCancel(context.Background())
    
    return &RedisEventBroker{
        client:      client,
        logger:      logger,
        subscribers: make(map[string][]types.ActionHandler),
        ctx:         ctx,
        cancel:      cancel,
    }
}

func (b *RedisEventBroker) Start() error {
    // Start message processing goroutine
    go b.processMessages()
    return nil
}

func (b *RedisEventBroker) Stop() error {
    b.cancel()
    return b.client.Close()
}

func (b *RedisEventBroker) IsRunning() bool {
    return b.ctx.Err() == nil
}

func (b *RedisEventBroker) Publish(action string, payload interface{}) error {
    message := &types.ActionMessage{
        Action:    action,
        Payload:   payload,
        Timestamp: time.Now(),
        Source:    "redis-broker",
        MessageID: generateMessageID(),
    }
    
    data, err := json.Marshal(message)
    if err != nil {
        return err
    }
    
    return b.client.Publish(b.ctx, action, data).Err()
}

func (b *RedisEventBroker) Subscribe(action string, handler types.ActionHandler) error {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    if b.subscribers[action] == nil {
        // First subscriber for this action - start Redis subscription
        go b.subscribeToRedisChannel(action)
    }
    
    b.subscribers[action] = append(b.subscribers[action], handler)
    return nil
}

func (b *RedisEventBroker) Unsubscribe(action string) error {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    delete(b.subscribers, action)
    return nil
}

func (b *RedisEventBroker) subscribeToRedisChannel(action string) {
    pubsub := b.client.Subscribe(b.ctx, action)
    defer pubsub.Close()
    
    ch := pubsub.Channel()
    
    for {
        select {
        case msg := <-ch:
            b.handleMessage(action, msg.Payload)
        case <-b.ctx.Done():
            return
        }
    }
}

func (b *RedisEventBroker) handleMessage(action string, data string) {
    var message types.ActionMessage
    if err := json.Unmarshal([]byte(data), &message); err != nil {
        b.logger.Error("Failed to unmarshal message", zap.Error(err))
        return
    }
    
    b.mu.RLock()
    handlers := b.subscribers[action]
    b.mu.RUnlock()
    
    for _, handler := range handlers {
        go func(h types.ActionHandler) {
            if err := h(&message); err != nil {
                b.logger.Error("Event handler failed",
                    zap.String("action", action),
                    zap.Error(err))
            }
        }(handler)
    }
}

// Register custom broker
func init() {
    action.RegisterActionBroker("redis", func(config interface{}) (types.ActionBroker, error) {
        cfg := config.(map[string]interface{})
        redisURL := cfg["url"].(string)
        
        return NewRedisEventBroker(redisURL, sai.Logger()), nil
    })
}
```

Configuration for custom broker:
```yaml
actions:
  broker:
    enabled: true
    type: "redis"
    config:
      url: "redis://localhost:6379/0"
```

## 🔗 Webhooks

The framework provides a comprehensive webhook system for receiving and managing webhooks.

### Configuration

```yaml
actions:
  webhooks:
    enabled: true
    config:
      max_retries: 3
      timeout: "30s"
      signature_header: "X-Signature"
      timestamp_tolerance: "5m"
```

### Webhook Management API

The framework automatically provides webhook management endpoints:

```bash
# Create webhook
POST /api/webhooks
{
  "event": "user.created",
  "url": "https://external-service.com/webhooks/user-created",
  "headers": {
    "Authorization": "Bearer token",
    "X-Source": "my-service"
  },
  "enabled": true
}

# List webhooks
GET /api/webhooks

# Get specific webhook
GET /api/webhooks/{webhook_id}

# Update webhook
PUT /api/webhooks/{webhook_id}
{
  "enabled": false
}

# Delete webhook
DELETE /api/webhooks/{webhook_id}

```
### Auto create webhhok

If event list provided in the client section:

```yaml
services:
    user_service:
      url: "http://user-service:8080"
      auth:
        provider: "token"
        payload:
          token: "service-to-service-token"
      events: ["user.created", "user.updated"]
```

The service automatically creates the webhook when your authentication credentials are correct. All you need to do now is subscribe.

### Receiving Webhooks

```go
func setupWebhookHandlers() {
    actions := sai.Actions()
    
    // Handle incoming webhooks from external services
    actions.Subscribe("external.payment.completed", handlePaymentWebhook)
    actions.Subscribe("external.user.verification", handleVerificationWebhook)
}

func handlePaymentWebhook(msg *types.ActionMessage) error {
    sai.Logger().Info("Payment webhook received",
        zap.String("source", msg.Source),
        zap.Time("timestamp", msg.Timestamp))
    
    // Verify webhook authenticity
    if msg.Source != "webhook" {
        return types.NewError("invalid webhook source")
    }
    
    // Extract payment data
    paymentData := msg.Payload.(map[string]interface{})
    paymentID := paymentData["payment_id"].(string)
    status := paymentData["status"].(string)
    
    // Update payment status in database
    if err := updatePaymentStatus(paymentID, status); err != nil {
        return err
    }
    
    // Publish internal event
    sai.Actions().Publish("payment.status.updated", map[string]interface{}{
        "payment_id": paymentID,
        "status":     status,
        "updated_at": time.Now(),
    })
    
    return nil
}
```

### Webhook Security

```go
func verifyWebhookSignature(payload []byte, signature, secret string) bool {
    // HMAC SHA256 verification
    h := hmac.New(sha256.New, []byte(secret))
    h.Write(payload)
    expectedSignature := hex.EncodeToString(h.Sum(nil))
    
    return hmac.Equal([]byte(signature), []byte("sha256="+expectedSignature))
}

func verifyGitHubSignature(signature string, payload []byte, secret string) bool {
    if !strings.HasPrefix(signature, "sha256=") {
        return false
    }
    
    signature = strings.TrimPrefix(signature, "sha256=")
    return verifyWebhookSignature(payload, signature, secret)
}

func verifyStripeSignature(payload []byte, signature, secret string) bool {
    // Stripe signature format: t=timestamp,v1=signature
    elements := strings.Split(signature, ",")
    
    var timestamp, sig string
    for _, element := range elements {
        parts := strings.Split(element, "=")
        if len(parts) == 2 {
            switch parts[0] {
            case "t":
                timestamp = parts[1]
            case "v1":
                sig = parts[1]
            }
        }
    }
    
    // Verify timestamp tolerance
    ts, err := strconv.ParseInt(timestamp, 10, 64)
    if err != nil {
        return false
    }
    
    if time.Since(time.Unix(ts, 0)) > 5*time.Minute {
        return false
    }
    
    // Verify signature
    signedPayload := timestamp + "." + string(payload)
    return verifyWebhookSignature([]byte(signedPayload), sig, secret)
}
```

## ⏰ Cron Jobs

The framework provides a robust cron job scheduler with monitoring and error handling.

### Configuration

```yaml
cron:
  enabled: true
  timezone: "UTC"  # or "America/New_York", "Europe/London", etc.
```

### Basic Cron Jobs

```go
func setupCronJobs() {
    cron := sai.Cron()
    
    // Daily cleanup at 2:00 AM
    cron.Add("daily_cleanup", "0 2 * * *", func() {
        sai.Logger().Info("Starting daily cleanup")
        
        if err := cleanupExpiredSessions(); err != nil {
            sai.Logger().Error("Session cleanup failed", zap.Error(err))
        }
        
        if err := cleanupOldLogs(); err != nil {
            sai.Logger().Error("Log cleanup failed", zap.Error(err))
        }
        
        sai.Logger().Info("Daily cleanup completed")
    })
    
    // Health check every 5 minutes
    cron.Add("health_check", "*/5 * * * *", func() {
        if err := performSystemHealthCheck(); err != nil {
            sai.Logger().Error("Health check failed", zap.Error(err))
            
            // Send alert
            sai.Actions().Publish("system.health.critical", map[string]interface{}{
                "error":     err.Error(),
                "timestamp": time.Now(),
            })
        }
    })
    
    // Generate reports every Monday at 9:00 AM
    cron.Add("weekly_report", "0 9 * * 1", func() {
        sai.Logger().Info("Generating weekly report")
        
        report, err := generateWeeklyReport()
        if err != nil {
            sai.Logger().Error("Report generation failed", zap.Error(err))
            return
        }
        
        if err := emailReport(report); err != nil {
            sai.Logger().Error("Failed to email report", zap.Error(err))
        }
        
        sai.Logger().Info("Weekly report generated and sent")
    })
    
    // Cache warming every hour
    cron.Add("cache_warming", "0 * * * *", func() {
        warmupCaches()
    })
    
    // Metrics collection every minute
    cron.Add("metrics_collection", "* * * * *", func() {
        collectCustomMetrics()
    })
}
```

### Advanced Cron Jobs

```go
func setupAdvancedCronJobs() {
    cron := sai.Cron()
    
    // Database backup every day at 3:00 AM
    cron.Add("db_backup", "0 3 * * *", func() {
        backupDatabase()
    })
    
    // Process pending emails every 2 minutes
    cron.Add("email_processor", "*/2 * * * *", func() {
        processEmailQueue()
    })
    
    // Clean up temporary files every 6 hours
    cron.Add("temp_cleanup", "0 */6 * * *", func() {
        cleanupTempFiles()
    })
    
    // Update exchange rates daily at midnight
    cron.Add("exchange_rates", "0 0 * * *", func() {
        updateExchangeRates()
    })
    
    // Generate thumbnails for new images every 30 seconds
    cron.Add("thumbnail_generator", "*/30 * * * * *", func() {
        generatePendingThumbnails()
    })
}

func backupDatabase() {
    sai.Logger().Info("Starting database backup")
    
    // Create backup filename with timestamp
    timestamp := time.Now().Format("20060102_150405")
    backupFile := fmt.Sprintf("/backups/db_backup_%s.sql", timestamp)
    
    // Perform backup
    if err := createDatabaseBackup(backupFile); err != nil {
        sai.Logger().Error("Database backup failed", zap.Error(err))
        
        // Send alert
        sai.Actions().Publish("backup.failed", map[string]interface{}{
            "type":      "database",
            "file":      backupFile,
            "error":     err.Error(),
            "timestamp": time.Now(),
        })
        return
    }
    
    // Upload to cloud storage
    if err := uploadToCloud(backupFile); err != nil {
        sai.Logger().Error("Backup upload failed", zap.Error(err))
    }
    
    // Clean up old backups (keep last 7 days)
    cleanupOldBackups(7)
    
    sai.Logger().Info("Database backup completed", zap.String("file", backupFile))
}

func processEmailQueue() {
    emails, err := getPendingEmails(100) // Get up to 100 pending emails
    if err != nil {
        sai.Logger().Error("Failed to get pending emails", zap.Error(err))
        return
    }
    
    if len(emails) == 0 {
        return // No emails to process
    }
    
    sai.Logger().Info("Processing email queue", zap.Int("count", len(emails)))
    
    for _, email := range emails {
        if err := sendEmail(email); err != nil {
            sai.Logger().Error("Failed to send email",
                zap.Error(err),
                zap.String("email_id", email.ID))
            
            // Mark as failed and retry later
            markEmailFailed(email.ID, err.Error())
        } else {
            // Mark as sent
            markEmailSent(email.ID)
        }
    }
}

func generatePendingThumbnails() {
    images, err := getImagesNeedingThumbnails(50)
    if err != nil {
        sai.Logger().Error("Failed to get images needing thumbnails", zap.Error(err))
        return
    }
    
    if len(images) == 0 {
        return
    }
    
    for _, image := range images {
        if err := generateThumbnail(image); err != nil {
            sai.Logger().Error("Thumbnail generation failed",
                zap.Error(err),
                zap.String("image_id", image.ID))
        } else {
            markThumbnailGenerated(image.ID)
        }
    }
}
```

### Cron Expression Examples

```go
// Cron expression format: second minute hour day month dayOfWeek
// (seconds are optional - use 5 fields for minute precision)

var cronExamples = map[string]string{
    // Every minute
    "* * * * *": "every minute",
    
    // Every 5 minutes
    "*/5 * * * *": "every 5 minutes",
    
    // Every hour at minute 30
    "30 * * * *": "every hour at minute 30",
    
    // Every day at 2:30 AM
    "30 2 * * *": "every day at 2:30 AM",
    
    // Every Monday at 9:00 AM
    "0 9 * * 1": "every Monday at 9:00 AM",
    
    // Every weekday at 6:00 PM
    "0 18 * * 1-5": "every weekday at 6:00 PM",
    
    // First day of every month at midnight
    "0 0 1 * *": "first day of every month at midnight",
    
    // Every 30 seconds (6-field format)
    "*/30 * * * * *": "every 30 seconds",
    
    // Every quarter hour
    "0 */15 * * *": "every quarter hour",
    
    // Twice daily (8 AM and 8 PM)
    "0 8,20 * * *": "twice daily at 8 AM and 8 PM",
}
```

## ❤️ Health Manager

The framework provides comprehensive health monitoring with built-in and custom health checks.

### Configuration

```yaml
health:
  enabled: true
```

### Built-in Health Endpoints

- `GET /health` - Comprehensive health report
- `GET /version` - Service version and build information

### Built-in Health Checks

```go
func setupHealthChecks() {
    health := sai.Health()
    
    // Database health check
    health.RegisterChecker("database", func(ctx context.Context) types.HealthCheck {
        // Check database connectivity
        if err := db.PingContext(ctx); err != nil {
            return types.HealthCheck{
                Status:  types.StatusUnhealthy,
                Message: "License has expired",
                Details: map[string]interface{}{
                    "expired_at": license.ExpiresAt,
                    "days_expired": int(time.Since(license.ExpiresAt).Hours() / 24),
                },
            }
        }
        
        daysUntilExpiry := int(time.Until(license.ExpiresAt).Hours() / 24)
        
        status := types.StatusHealthy
        message := "License is valid"
        
        if daysUntilExpiry <= 7 {
            status = types.StatusUnhealthy
            message = "License expires soon"
        } else if daysUntilExpiry <= 30 {
            message = "License expires within 30 days"
        }
        
        return types.HealthCheck{
            Status:  status,
            Message: message,
            Details: map[string]interface{}{
                "expires_at":        license.ExpiresAt,
                "days_until_expiry": daysUntilExpiry,
                "license_type":      license.Type,
            },
        }
    })
    
    // Check feature flags service
    health.RegisterChecker("feature_flags", func(ctx context.Context) types.HealthCheck {
        start := time.Now()
        flags, err := getFeatureFlags()
        responseTime := time.Since(start)
        
        if err != nil {
            return types.HealthCheck{
                Status:  types.StatusUnhealthy,
                Message: "Feature flags service unavailable",
                Details: map[string]interface{}{
                    "error": err.Error(),
                    "response_time_ms": responseTime.Milliseconds(),
                },
            }
        }
        
        status := types.StatusHealthy
        if responseTime > 2*time.Second {
            status = types.StatusUnhealthy
        }
        
        return types.HealthCheck{
            Status:  status,
            Message: "Feature flags service is operational",
            Details: map[string]interface{}{
                "flags_count":      len(flags),
                "response_time_ms": responseTime.Milliseconds(),
            },
        }
    })
}
```

### Health Check Response Format

```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "uptime": "72h15m30s",
  "service": {
    "name": "User Service",
    "version": "2.1.0",
    "host": "api.example.com",
    "port": 8080
  },
  "checks": {
    "database": {
      "status": "healthy",
      "message": "Database is operational",
      "last_check": "2024-01-15T10:30:00Z",
      "duration": "15ms",
      "details": {
        "query_time_ms": 12,
        "connections": 5
      }
    },
    "redis": {
      "status": "healthy",
      "message": "Redis is operational",
      "last_check": "2024-01-15T10:30:00Z",
      "duration": "8ms",
      "details": {
        "ping_time_ms": 5,
        "memory_usage": "45MB"
      }
    },
    "user_service": {
      "status": "unhealthy",
      "message": "User service returned 503",
      "last_check": "2024-01-15T10:30:00Z",
      "duration": "5s",
      "details": {
        "status_code": 503,
        "error": "Service temporarily unavailable"
      }
    }
  },
  "summary": {
    "total": 3,
    "healthy": 2,
    "unhealthy": 1,
    "unknown": 0
  }
}
```

### Using Health Data

```go
func monitorHealth() {
    health := sai.Health()
    
    // Get current health status
    report := health.Check(context.Background())
    
    if report.Status != types.StatusHealthy {
        sai.Logger().Error("Service health check failed",
            zap.String("overall_status", string(report.Status)),
            zap.Int("unhealthy_checks", report.Summary.Unhealthy))
        
        // Send alert
        sendHealthAlert(report)
    }
    
    // Log health metrics
    for name, check := range report.Checks {
        sai.Logger().Debug("Health check result",
            zap.String("check", name),
            zap.String("status", string(check.Status)),
            zap.Duration("duration", check.Duration))
    }
}

func sendHealthAlert(report types.HealthReport) {
    // Find failed checks
    var failedChecks []string
    for name, check := range report.Checks {
        if check.Status == types.StatusUnhealthy {
            failedChecks = append(failedChecks, name)
        }
    }
    
    // Send notification
    sai.Actions().Publish("health.alert", map[string]interface{}{
        "service":       report.Service.Name,
        "status":        report.Status,
        "failed_checks": failedChecks,
        "timestamp":     report.Timestamp,
        "uptime":        report.Uptime.String(),
    })
}
```

## 📊 Metrics Manager

The framework provides comprehensive metrics collection with support for Prometheus and custom providers.

### Configuration

```yaml
metrics:
  enabled: true
  type: "prometheus"  # memory, prometheus, custom
  prefix: "myservice"
  config:
    namespace: "myservice"
    subsystem: "api"
  http:
    enabled: true
    path: "/metrics"
    port: 9090  # 0 = same port as main server
  collectors:
    system: true      # CPU, memory, disk metrics
    runtime: true     # Go runtime metrics
    http: true        # HTTP request metrics
    cache: true       # Cache operation metrics
    middleware: true  # Middleware metrics
```

### Built-in Metrics

The framework automatically collects the following metrics:

#### HTTP Metrics
- `http_requests_total` - Total HTTP requests
- `http_request_duration_seconds` - Request duration histogram
- `http_request_size_bytes` - Request size histogram
- `http_response_size_bytes` - Response size histogram

#### System Metrics
- `system_cpu_usage` - CPU usage percentage
- `system_memory_usage_bytes` - Memory usage
- `system_disk_usage_bytes` - Disk usage
- `system_load_average` - System load average

#### Runtime Metrics
- `go_goroutines` - Number of goroutines
- `go_threads` - Number of OS threads
- `go_gc_duration_seconds` - GC duration
- `go_memstats_*` - Memory statistics

### Custom Metrics Usage

```go
func useCustomMetrics() {
    metrics := sai.Metrics()
    
    // Counter - monotonically increasing value
    userRegistrations := metrics.Counter("user_registrations_total", map[string]string{
        "source": "web",
    })
    
    // Gauge - value that can go up or down
    activeConnections := metrics.Gauge("active_connections", nil)
    
    // Histogram - distribution of values
    requestDuration := metrics.Histogram(
        "api_request_duration_seconds",
        []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0},
        map[string]string{"endpoint": "users"},
    )
    
    // Summary - quantiles over sliding time window
    responseSize := metrics.Summary(
        "api_response_size_bytes",
        map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
        map[string]string{"endpoint": "users"},
    )
    
    // Use metrics
    userRegistrations.Inc()
    activeConnections.Set(42)
    requestDuration.Observe(1.2)
    responseSize.Observe(1024)
}

func setupBusinessMetrics() {
    metrics := sai.Metrics()
    
    // E-commerce metrics
    ordersCounter := metrics.Counter("orders_total", map[string]string{
        "status": "completed",
    })
    
    revenueGauge := metrics.Gauge("revenue_total", map[string]string{
        "currency": "USD",
    })
    
    orderValueHistogram := metrics.Histogram(
        "order_value_dollars",
        []float64{10, 50, 100, 250, 500, 1000},
        nil,
    )
    
    // Processing time metrics
    processingDuration := metrics.Histogram(
        "order_processing_duration_seconds",
        []float64{0.1, 0.5, 1.0, 5.0, 10.0, 30.0},
        map[string]string{"step": "validation"},
    )
    
    // Usage metrics
    apiCallsCounter := metrics.Counter("api_calls_total", map[string]string{
        "method":   "GET",
        "endpoint": "/api/v1/users",
        "status":   "200",
    })
    
    cacheHitRate := metrics.Gauge("cache_hit_rate", map[string]string{
        "cache_type": "redis",
    })
}
```

### Metrics in Handlers

```go
func handleWithMetrics(ctx *types.RequestCtx) {
    start := time.Now()
    
    // Get metrics
    metrics := sai.Metrics()
    requestCounter := metrics.Counter("api_requests_total", map[string]string{
        "method": string(ctx.Method()),
        "path":   string(ctx.Path()),
    })
    
    requestDuration := metrics.Histogram(
        "api_request_duration_seconds",
        []float64{0.001, 0.01, 0.1, 1.0, 5.0},
        map[string]string{"path": string(ctx.Path())},
    )
    
    activeRequests := metrics.Gauge("api_active_requests", nil)
    
    // Track active requests
    activeRequests.Inc()
    defer activeRequests.Dec()
    
    // Track request duration
    defer requestDuration.ObserveDuration(start)
    
    // Process request
    result, err := processRequest(ctx)
    
    // Record metrics based on result
    if err != nil {
        errorCounter := metrics.Counter("api_errors_total", map[string]string{
            "path":  string(ctx.Path()),
            "error": "processing_failed",
        })
        errorCounter.Inc()
        
        ctx.Error(err, 500)
        requestCounter.Add(1)  // Count failed requests
        return
    }
    
    // Record success
    requestCounter.Inc()
    
    // Record business metrics
    if result.OrderCreated {
        orderMetrics := metrics.Counter("orders_created_total", map[string]string{
            "source": "api",
        })
        orderMetrics.Inc()
        
        orderValue := metrics.Histogram(
            "order_value_dollars",
            []float64{10, 50, 100, 250, 500, 1000},
            nil,
        )
        orderValue.Observe(result.OrderValue)
    }
    
    ctx.SuccessJSON(result)
}
```

### Custom Metrics Provider

```go
// Custom DataDog metrics provider
type DataDogMetrics struct {
    client dogstatsd.ClientInterface
    logger types.Logger
    prefix string
}

func NewDataDogMetrics(addr, prefix string, logger types.Logger) *DataDogMetrics {
    client, err := dogstatsd.New(addr)
    if err != nil {
        logger.Error("Failed to create DataDog client", zap.Error(err))
        return nil
    }
    
    return &DataDogMetrics{
        client: client,
        logger: logger,
        prefix: prefix,
    }
}

func (d *DataDogMetrics) Counter(name string, labels map[string]string) types.Counter {
    return &DataDogCounter{
        client: d.client,
        name:   d.prefix + "." + name,
        tags:   d.labelsToTags(labels),
    }
}

func (d *DataDogMetrics) Gauge(name string, labels map[string]string) types.Gauge {
    return &DataDogGauge{
        client: d.client,
        name:   d.prefix + "." + name,
        tags:   d.labelsToTags(labels),
    }
}

func (d *DataDogMetrics) Histogram(name string, buckets []float64, labels map[string]string) types.Histogram {
    return &DataDogHistogram{
        client: d.client,
        name:   d.prefix + "." + name,
        tags:   d.labelsToTags(labels),
    }
}

func (d *DataDogMetrics) labelsToTags(labels map[string]string) []string {
    var tags []string
    for k, v := range labels {
        tags = append(tags, fmt.Sprintf("%s:%s", k, v))
    }
    return tags
}

// Implement DataDogCounter, DataDogGauge, DataDogHistogram...

// Register custom metrics provider
func init() {
    metrics.RegisterMetricsManager("datadog", func(config interface{}) (types.MetricsManager, error) {
        cfg := config.(map[string]interface{})
        addr := cfg["addr"].(string)
        prefix := cfg["prefix"].(string)
        
        return NewDataDogMetrics(addr, prefix, sai.Logger()), nil
    })
}
```

Configuration for custom metrics:
```yaml
metrics:
  enabled: true
  type: "datadog"
  config:
    addr: "localhost:8125"
    prefix: "myservice"
```

### Metrics Dashboard

When using Prometheus, you can create Grafana dashboards with these queries:

```promql
# Request rate
rate(http_requests_total[5m])

# Error rate
rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m])

# Response time percentiles
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Active connections
go_goroutines

# Memory usage
go_memstats_alloc_bytes

# Cache hit rate
cache_hit_rate

# Business metrics
rate(orders_total[5m])
increase(revenue_total[1h])
```

## 🛡️ TLS Manager

The framework provides automatic TLS certificate management with Let's Encrypt integration.

### Configuration

```yaml
server:
  tls:
    enabled: true
    auto_cert: true                    # Use Let's Encrypt
    domains: ["api.example.com"]       # Domains for certificates
    email: "admin@example.com"         # Let's Encrypt email
    cache_dir: "./certs"               # Certificate cache directory
    acme_directory: ""                 # Custom ACME directory (optional)
    # Manual certificates (alternative to auto_cert)
    cert_file: "/path/to/cert.pem"     # Certificate file
    key_file: "/path/to/key.pem"       # Private key file
```

### Automatic Certificates (Let's Encrypt)

```go
func setupAutoTLS() {
    // TLS is configured automatically from config.yml
    // The framework will:
    // 1. Request certificates from Let's Encrypt
    // 2. Handle ACME challenges automatically
    // 3. Renew certificates before expiration
    // 4. Serve HTTPS traffic
    
    router := sai.Router()
    
    // All routes automatically use HTTPS when TLS is enabled
    router.GET("/api/secure", func(ctx *types.RequestCtx) {
        ctx.SuccessJSON(map[string]interface{}{
            "secure":     true,
            "protocol":   "https",
            "cert_info":  getCertificateInfo(ctx),
        })
    })
}

func getCertificateInfo(ctx *types.RequestCtx) map[string]interface{} {
    // Extract certificate information from request
    return map[string]interface{}{
        "tls_version": "TLS 1.3",
        "cipher":      "ECDHE-RSA-AES256-GCM-SHA384",
        "server_name": string(ctx.Host()),
    }
}
```

### Manual Certificates

```yaml
server:
  tls:
    enabled: true
    auto_cert: false
    cert_file: "/etc/ssl/certs/server.crt"
    key_file: "/etc/ssl/private/server.key"
```

### Certificate Monitoring

```go
func setupCertificateMonitoring() {
    // The TLS manager automatically provides certificate status
    router := sai.Router()
    
    router.GET("/admin/certificates", func(ctx *types.RequestCtx) {
        // This endpoint would be protected with admin auth
        tlsManager := getTLSManager() // Get from service container
        
        if tlsManager == nil {
            ctx.Error(types.NewError("TLS not enabled"), 404)
            return
        }
        
        status := tlsManager.GetCertificateStatus()
        ctx.SuccessJSON(status)
    }).WithMiddlewares("auth") // Admin authentication required
}

// Certificate status response format:
// {
//   "api.example.com": {
//     "domain": "api.example.com",
//     "status": "valid",
//     "issuer": "Let's Encrypt Authority X3",
//     "subject": "CN=api.example.com",
//     "not_before": "2024-01-01T00:00:00Z",
//     "not_after": "2024-04-01T00:00:00Z",
//     "days_until_expiry": 45
//   }
// }
```

### TLS Security Headers

```go
func setupSecurityHeaders() {
    // Add security middleware for HTTPS
    router := sai.Router()
    
    // All routes get security headers when TLS is enabled
    router.Use(func(ctx *types.RequestCtx, next func(*types.RequestCtx), config *types.RouteConfig) {
        if isTLSEnabled() {
            // HSTS - force HTTPS for future requests
            ctx.Response.Header.Set("Strict-Transport-Security", 
                "max-age=31536000; includeSubDomains; preload")
            
            // Prevent downgrade attacks
            ctx.Response.Header.Set("Upgrade-Insecure-Requests", "1")
            
            // Content security
            ctx.Response.Header.Set("X-Content-Type-Options", "nosniff")
            ctx.Response.Header.Set("X-Frame-Options", "DENY")
            ctx.Response.Header.Set("X-XSS-Protection", "1; mode=block")
            
            // Referrer policy
            ctx.Response.Header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
        }
        
        next(ctx)
    })
}
```

### HTTP to HTTPS Redirect

```go
func setupHTTPSRedirect() {
    // When TLS is enabled, automatically redirect HTTP to HTTPS
    
    if isTLSEnabled() {
        // Start HTTP server for redirects
        go func() {
            redirectServer := &fasthttp.Server{
                Handler: func(ctx *fasthttp.RequestCtx) {
                    // Redirect to HTTPS
                    httpsURL := fmt.Sprintf("https://%s%s", 
                        ctx.Host(), ctx.RequestURI())
                    
                    ctx.Redirect(httpsURL, fasthttp.StatusMovedPermanently)
                },
            }
            
            httpAddr := fmt.Sprintf("%s:80", getServerHost())
            sai.Logger().Info("Starting HTTP redirect server", 
                zap.String("addr", httpAddr))
            
            if err := redirectServer.ListenAndServe(httpAddr); err != nil {
                sai.Logger().Error("HTTP redirect server failed", zap.Error(err))
            }
        }()
    }
}
```

### Production TLS Setup

```bash
# Production environment variables
export TLS_ENABLED=true
export TLS_AUTO_CERT=true
export TLS_DOMAINS=api.example.com,www.api.example.com
export TLS_EMAIL=admin@example.com

# Docker deployment with TLS
docker run -d \
  -p 80:80 \
  -p 443:443 \
  -v /etc/letsencrypt:/app/certs \
  -e TLS_ENABLED=true \
  -e TLS_AUTO_CERT=true \
  -e TLS_DOMAINS=api.example.com \
  -e TLS_EMAIL=admin@example.com \
  myservice:latest
```

### Certificate Renewal Monitoring

```go
func setupCertificateAlerts() {
    // Monitor certificate expiration
    cron := sai.Cron()
    
    cron.Add("certificate_check", "0 */12 * * *", func() {
        tlsManager := getTLSManager()
        if tlsManager == nil {
            return
        }
        
        status := tlsManager.GetCertificateStatus()
        
        for domain, cert := range status {
            if cert.Status == "expiring_soon" || cert.DaysUntilExpiry <= 7 {
                // Send alert
                sai.Actions().Publish("certificate.expiring", map[string]interface{}{
                    "domain":             domain,
                    "days_until_expiry":  cert.DaysUntilExpiry,
                    "not_after":          cert.NotAfter,
                })
                
                sai.Logger().Warn("Certificate expiring soon",
                    zap.String("domain", domain),
                    zap.Int("days_until_expiry", cert.DaysUntilExpiry))
            }
        }
    })
}
```

---

## 📄 License

MIT License - see LICENSE file for details.

## 🆘 Support

- 📧 Email: support@sai-service.com
- 💬 Discord: [SAI Community](https://discord.gg/sai)
- 📖 Documentation: [docs.sai-service.com](https://docs.sai-service.com)
- 🐛 Issues: [GitHub Issues](https://github.com/saiset-co/sai-service/issues)

---

**Build powerful Go services in minutes, not days!**