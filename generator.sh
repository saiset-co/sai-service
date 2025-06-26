#!/bin/bash

# SAI Service Generator
# Generates a complete microservice project based on SAI Service library

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Default values
DEFAULT_PORT="8080"
DEFAULT_TEMPLATE="api"
DEFAULT_CACHE_TYPE="memory"
DEFAULT_METRICS_TYPE="memory"
DEFAULT_BROKER_TYPE="websocket"

# Available options
TEMPLATES=("basic" "api" "microservice" "full")
FEATURES=("cache" "metrics" "docs" "cron" "actions" "tls" "middleware" "health" "client")
MIDDLEWARES=("cache" "auth" "bodylimit" "compression" "cors" "logging" "ratelimit" "recovery")
CACHE_TYPES=("memory" "redis")
METRICS_TYPES=("memory" "prometheus")
BROKER_TYPES=("websocket")
CICD_TYPES=("none" "github" "gitlab")

# Project configuration
PROJECT_NAME=""
MODULE_NAME=""
PORT=""
TEMPLATE=""
SELECTED_FEATURES=()
SELECTED_MIDDLEWARES=()
CACHE_TYPE=""
METRICS_TYPE=""
BROKER_TYPE=""
INCLUDE_TESTS="false"
CICD_TYPE="none"

# Helper functions
print_header() {
    echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║                   SAI Service Generator                      ║${NC}"
    echo -e "${CYAN}║              Fast Microservice Project Creation             ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

prompt_input() {
    local prompt="$1"
    local default="$2"
    local result=""

    if [ -n "$default" ]; then
        echo -ne "${YELLOW}$prompt [$default]: ${NC}" >&2
    else
        echo -ne "${YELLOW}$prompt: ${NC}" >&2
    fi

    read -r result
    if [ -z "$result" ] && [ -n "$default" ]; then
        result="$default"
    fi
    echo "$result"
}

prompt_yes_no() {
    local prompt="$1"
    local default="$2"
    local result=""

    if [ "$default" = "y" ]; then
        echo -ne "${YELLOW}$prompt [Y/n]: ${NC}" >&2
    else
        echo -ne "${YELLOW}$prompt [y/N]: ${NC}" >&2
    fi

    read -r result
    if [ -z "$result" ]; then
        result="$default"
    fi

    case "$result" in
        [Yy]|[Yy][Ee][Ss]) echo "true" ;;
        *) echo "false" ;;
    esac
}

select_from_array() {
    local prompt="$1"
    local default="$2"
    shift 2
    local options=("$@")

    echo -e "${PURPLE}Available $prompt:${NC} ${options[*]}" >&2
    local result

    if [ -n "$default" ]; then
        echo -ne "${YELLOW}Select $prompt [$default]: ${NC}" >&2
    else
        echo -ne "${YELLOW}Select $prompt: ${NC}" >&2
    fi

    read -r result

    # Если пустой ввод, используем дефолтное значение
    if [ -z "$result" ]; then
        echo "$default"
        return
    fi

    # Validate selection
    for option in "${options[@]}"; do
        if [ "$option" = "$result" ]; then
            echo "$result"
            return
        fi
    done

    echo -e "${YELLOW}⚠ Invalid selection '$result', using default '$default'${NC}" >&2
    echo "$default"
}

select_multiple_from_array() {
    local prompt="$1"
    shift
    local options=("$@")

    echo -e "${PURPLE}Available $prompt:${NC} ${options[*]}" >&2
    echo -ne "${YELLOW}Enable $prompt (comma-separated): ${NC}" >&2

    local result
    read -r result

    if [ -z "$result" ]; then
        echo ""
        return
    fi

    # Split by comma and validate
    IFS=',' read -ra SELECTED <<< "$result"
    local valid_selections=()

    for selection in "${SELECTED[@]}"; do
        selection=$(echo "$selection" | xargs) # trim whitespace
        for option in "${options[@]}"; do
            if [ "$option" = "$selection" ]; then
                valid_selections+=("$selection")
                break
            fi
        done
    done

    echo "${valid_selections[@]}"
}

# Validation functions
validate_project_name() {
    local name="$1"
    # Проверяем, что имя не пустое и содержит только допустимые символы
    if [[ -z "$name" ]]; then
        print_error "Project name cannot be empty"
        return 1
    fi

    # Проверяем, что имя начинается с буквы
    if [[ ! "$name" =~ ^[a-zA-Z] ]]; then
        print_error "Project name must start with a letter"
        return 1
    fi

    # Проверяем, что имя содержит только буквы, цифры, дефисы и подчеркивания
    if [[ ! "$name" =~ ^[a-zA-Z][a-zA-Z0-9_-]*$ ]]; then
        print_error "Project name can only contain letters, numbers, hyphens, and underscores"
        return 1
    fi

    return 0
}

validate_module_name() {
    local name="$1"
    if [[ -z "$name" ]]; then
        print_error "Module name cannot be empty"
        return 1
    fi

    # Более мягкая проверка для Go модулей
    if [[ ! "$name" =~ ^[a-zA-Z0-9][a-zA-Z0-9_./-]*[a-zA-Z0-9]$ ]] && [[ ! "$name" =~ ^[a-zA-Z0-9]$ ]]; then
        print_error "Module name must be a valid Go module path (e.g., github.com/user/project or simple-name)"
        return 1
    fi

    return 0
}

validate_port() {
    local port="$1"
    if [[ ! "$port" =~ ^[0-9]+$ ]] || [ "$port" -lt 1 ] || [ "$port" -gt 65535 ]; then
        print_error "Port must be a number between 1 and 65535"
        return 1
    fi
    return 0
}

# Configuration collection
collect_configuration() {
    echo -e "${BLUE}Let's configure your new service:${NC}\n"

    # Project name
    while true; do
        PROJECT_NAME=$(prompt_input "Enter project name" "")
        if [ -n "$PROJECT_NAME" ] && validate_project_name "$PROJECT_NAME"; then
            break
        fi
        if [ -z "$PROJECT_NAME" ]; then
            print_error "Project name is required"
        fi
    done

    # Module name
    while true; do
        MODULE_NAME=$(prompt_input "Enter Go module name" "$PROJECT_NAME")
        if validate_module_name "$MODULE_NAME"; then
            break
        fi
    done

    # Port
    while true; do
        PORT=$(prompt_input "Enter port number" "$DEFAULT_PORT")
        if validate_port "$PORT"; then
            break
        fi
    done

    # Template
    TEMPLATE=$(select_from_array "template" "$DEFAULT_TEMPLATE" "${TEMPLATES[@]}")

    # Features
    local features_str
    features_str=$(select_multiple_from_array "features" "${FEATURES[@]}")
    if [ -n "$features_str" ]; then
        IFS=' ' read -ra SELECTED_FEATURES <<< "$features_str"
    fi

    # Cache type if cache is enabled
    if [[ " ${SELECTED_FEATURES[*]} " == *" cache "* ]]; then
        CACHE_TYPE=$(select_from_array "cache type" "$DEFAULT_CACHE_TYPE" "${CACHE_TYPES[@]}")
    fi

    # Metrics type if metrics is enabled
    if [[ " ${SELECTED_FEATURES[*]} " == *" metrics "* ]]; then
        METRICS_TYPE=$(select_from_array "metrics type" "$DEFAULT_METRICS_TYPE" "${METRICS_TYPES[@]}")
    fi

    # Broker type if actions is enabled
    if [[ " ${SELECTED_FEATURES[*]} " == *" actions "* ]]; then
        BROKER_TYPE=$(select_from_array "broker type" "$DEFAULT_BROKER_TYPE" "${BROKER_TYPES[@]}")
    fi

    # Middlewares if middleware is enabled
    if [[ " ${SELECTED_FEATURES[*]} " == *" middleware "* ]]; then
        local middlewares_str
        middlewares_str=$(select_multiple_from_array "middlewares" "${MIDDLEWARES[@]}")
        if [ -n "$middlewares_str" ]; then
            IFS=' ' read -ra SELECTED_MIDDLEWARES <<< "$middlewares_str"
        fi
    fi

    # Tests
    INCLUDE_TESTS=$(prompt_yes_no "Include integration tests?" "n")

    # CI/CD
    CICD_TYPE=$(select_from_array "CI/CD" "none" "${CICD_TYPES[@]}")
}

# Configuration summary
show_configuration() {
    echo
    echo -e "${CYAN}Configuration Summary:${NC}"
    echo -e "${GREEN}   • Project:${NC} $PROJECT_NAME"
    echo -e "${GREEN}   • Module:${NC} $MODULE_NAME"
    echo -e "${GREEN}   • Port:${NC} $PORT"
    echo -e "${GREEN}   • Template:${NC} $TEMPLATE"

    if [ ${#SELECTED_FEATURES[@]} -gt 0 ]; then
        echo -e "${GREEN}   • Features:${NC} ${SELECTED_FEATURES[*]}"
    fi

    if [[ " ${SELECTED_FEATURES[*]} " == *" cache "* ]]; then
        echo -e "${GREEN}   • Cache Type:${NC} $CACHE_TYPE"
    fi

    if [[ " ${SELECTED_FEATURES[*]} " == *" metrics "* ]]; then
        echo -e "${GREEN}   • Metrics Type:${NC} $METRICS_TYPE"
    fi

    if [[ " ${SELECTED_FEATURES[*]} " == *" actions "* ]]; then
        echo -e "${GREEN}   • Broker Type:${NC} $BROKER_TYPE"
    fi

    if [ ${#SELECTED_MIDDLEWARES[@]} -gt 0 ]; then
        echo -e "${GREEN}   • Middlewares:${NC} ${SELECTED_MIDDLEWARES[*]}"
    fi

    echo -e "${GREEN}   • Tests:${NC} $INCLUDE_TESTS"
    echo -e "${GREEN}   • CI/CD:${NC} $CICD_TYPE"
    echo
}

# File generation functions
generate_main_go() {
    cat > "$PROJECT_NAME/cmd/main.go" << EOF
package main

import (
	"context"
	"log"

	"github.com/saiset-co/sai-service/service"
	"$MODULE_NAME/internal"
)

func main() {
	ctx := context.Background()

	// Initialize service
	srv, err := service.NewService(ctx, "config.yml")
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	// Register business logic
	if err := internal.RegisterBusinessLogic(); err != nil {
		log.Fatalf("Failed to register business logic: %v", err)
	}

	// Start service
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start service: %v", err)
	}

	// Wait for shutdown (handled by library)
	<-srv.Done()
	log.Println("Service stopped gracefully")
}
EOF
}

generate_service_go() {
    local middleware_routes=""

    if [[ " ${SELECTED_FEATURES[*]} " == *" middleware "* ]]; then
        if [[ " ${SELECTED_MIDDLEWARES[*]} " == *" auth "* ]]; then
            middleware_routes=".
		WithMiddlewares(\"auth\")"
        fi
        if [[ " ${SELECTED_MIDDLEWARES[*]} " == *" cache "* ]]; then
            middleware_routes="${middleware_routes}.
		WithCache(\"user_list\", 5*time.Minute)"
        fi
    fi

    cat > "$PROJECT_NAME/internal/service.go" << EOF
package internal

import (
	"github.com/saiset-co/sai-service/sai"
	"$MODULE_NAME/internal/handlers"
)

func RegisterBusinessLogic() error {
	router := sai.Router()

	// Health check endpoint
	router.GET("/health", handlers.HealthHandler).
		WithDoc("Health Check", "Check service health", "system", nil, nil)

	// API routes
	api := router.Group("/api/v1")

	// Users endpoints
	api.GET("/users", handlers.GetUsers)${middleware_routes}.
		WithDoc("List Users", "Get list of all users", "users", nil, handlers.UserListResponse{})

	api.GET("/users/{id}", handlers.GetUser).
		WithDoc("Get User", "Get user by ID", "users", nil, handlers.UserResponse{})

	api.POST("/users", handlers.CreateUser).
		WithDoc("Create User", "Create a new user", "users", handlers.CreateUserRequest{}, handlers.UserResponse{})

	api.PUT("/users/{id}", handlers.UpdateUser).
		WithDoc("Update User", "Update existing user", "users", handlers.UpdateUserRequest{}, handlers.UserResponse{})

	api.DELETE("/users/{id}", handlers.DeleteUser).
		WithDoc("Delete User", "Delete user by ID", "users", nil, nil)

	return nil
}
EOF
}

generate_handlers_go() {
    cat > "$PROJECT_NAME/internal/handlers/handlers.go" << EOF
package handlers

import (
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/saiset-co/sai-service/types"
	"$MODULE_NAME/internal/models"
)

// In-memory storage for demo purposes
var (
	users = make(map[int]*models.User)
	nextID = 1
	usersMu sync.RWMutex
)

// Request/Response models
type CreateUserRequest struct {
	Name  string \`json:"name" validate:"required,min=2,max=50"\`
	Email string \`json:"email" validate:"required,email"\`
	Age   int    \`json:"age" validate:"min=1,max=120"\`
}

type UpdateUserRequest struct {
	Name  *string \`json:"name,omitempty" validate:"omitempty,min=2,max=50"\`
	Email *string \`json:"email,omitempty" validate:"omitempty,email"\`
	Age   *int    \`json:"age,omitempty" validate:"omitempty,min=1,max=120"\`
}

type UserResponse struct {
	Success bool         \`json:"success"\`
	Data    *models.User \`json:"data,omitempty"\`
	Error   string       \`json:"error,omitempty"\`
}

type UserListResponse struct {
	Success bool           \`json:"success"\`
	Data    []*models.User \`json:"data,omitempty"\`
	Total   int            \`json:"total"\`
	Error   string         \`json:"error,omitempty"\`
}

// Initialize with sample data
func init() {
	users[1] = &models.User{
		ID:        1,
		Name:      "John Doe",
		Email:     "john@example.com",
		Age:       30,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	users[2] = &models.User{
		ID:        2,
		Name:      "Jane Smith",
		Email:     "jane@example.com",
		Age:       25,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	nextID = 3
}

func HealthHandler(ctx *types.RequestCtx) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"service":   "$PROJECT_NAME",
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	json.NewEncoder(ctx).Encode(response)
}

func GetUsers(ctx *types.RequestCtx) {
	usersMu.RLock()
	defer usersMu.RUnlock()

	userList := make([]*models.User, 0, len(users))
	for _, user := range users {
		userList = append(userList, user)
	}

	response := UserListResponse{
		Success: true,
		Data:    userList,
		Total:   len(userList),
	}

	ctx.WriteJSON(response)
}

func GetUser(ctx *types.RequestCtx) {
	idParam := ctx.UserValue("id")
	if idParam == nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.WriteJSON(UserResponse{
			Success: false,
			Error:   "User ID is required",
		})
		return
	}

	idStr, ok := idParam.(string)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.WriteJSON(UserResponse{
			Success: false,
			Error:   "Invalid user ID format",
		})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.WriteJSON(UserResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}

	usersMu.RLock()
	user, exists := users[id]
	usersMu.RUnlock()

	if !exists {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		ctx.WriteJSON(UserResponse{
			Success: false,
			Error:   "User not found",
		})
		return
	}

	ctx.WriteJSON(UserResponse{
		Success: true,
		Data:    user,
	})
}

func CreateUser(ctx *types.RequestCtx) {
	var req CreateUserRequest
	if err := ctx.ReadJSON(&req); err != nil {
		return // ReadJSON already sets error response
	}

	// Validate required fields
	if req.Name == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.WriteJSON(UserResponse{
			Success: false,
			Error:   "Name is required",
		})
		return
	}

	if req.Email == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.WriteJSON(UserResponse{
			Success: false,
			Error:   "Email is required",
		})
		return
	}

	if req.Age <= 0 {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.WriteJSON(UserResponse{
			Success: false,
			Error:   "Age must be positive",
		})
		return
	}

	usersMu.Lock()
	defer usersMu.Unlock()

	user := &models.User{
		ID:        nextID,
		Name:      req.Name,
		Email:     req.Email,
		Age:       req.Age,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	users[nextID] = user
	nextID++

	ctx.SetStatusCode(fasthttp.StatusCreated)
	ctx.WriteJSON(UserResponse{
		Success: true,
		Data:    user,
	})
}

func UpdateUser(ctx *types.RequestCtx) {
	idParam := ctx.UserValue("id")
	if idParam == nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.WriteJSON(UserResponse{
			Success: false,
			Error:   "User ID is required",
		})
		return
	}

	idStr, ok := idParam.(string)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.WriteJSON(UserResponse{
			Success: false,
			Error:   "Invalid user ID format",
		})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.WriteJSON(UserResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}

	var req UpdateUserRequest
	if err := ctx.ReadJSON(&req); err != nil {
		return // ReadJSON already sets error response
	}

	usersMu.Lock()
	defer usersMu.Unlock()

	user, exists := users[id]
	if !exists {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		ctx.WriteJSON(UserResponse{
			Success: false,
			Error:   "User not found",
		})
		return
	}

	// Update fields if provided
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Age != nil {
		user.Age = *req.Age
	}
	user.UpdatedAt = time.Now()

	ctx.WriteJSON(UserResponse{
		Success: true,
		Data:    user,
	})
}

func DeleteUser(ctx *types.RequestCtx) {
	idParam := ctx.UserValue("id")
	if idParam == nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.WriteJSON(UserResponse{
			Success: false,
			Error:   "User ID is required",
		})
		return
	}

	idStr, ok := idParam.(string)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.WriteJSON(UserResponse{
			Success: false,
			Error:   "Invalid user ID format",
		})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.WriteJSON(UserResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}

	usersMu.Lock()
	defer usersMu.Unlock()

	_, exists := users[id]
	if !exists {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		ctx.WriteJSON(UserResponse{
			Success: false,
			Error:   "User not found",
		})
		return
	}

	delete(users, id)

	ctx.SetStatusCode(fasthttp.StatusNoContent)
}
EOF
}

generate_models_go() {
    cat > "$PROJECT_NAME/internal/models/model.go" << EOF
package models

import (
	"time"
)

// User represents a user entity
type User struct {
	ID        int       \`json:"id" db:"id"\`
	Name      string    \`json:"name" db:"name" validate:"required,min=2,max=50"\`
	Email     string    \`json:"email" db:"email" validate:"required,email"\`
	Age       int       \`json:"age" db:"age" validate:"min=1,max=120"\`
	CreatedAt time.Time \`json:"created_at" db:"created_at"\`
	UpdatedAt time.Time \`json:"updated_at" db:"updated_at"\`
}

// Validate performs validation on the User model
func (u *User) Validate() error {
	// Custom validation logic can be added here
	return nil
}

// TableName returns the database table name for the User model
func (u *User) TableName() string {
	return "users"
}
EOF
}

generate_config_yml() {
    local cache_config=""
    local metrics_config=""
    local actions_config=""
    local middleware_config=""

    # Cache configuration
    if [[ " ${SELECTED_FEATURES[*]} " == *" cache "* ]]; then
        if [ "$CACHE_TYPE" = "redis" ]; then
            cache_config="cache:
  enabled: true
  type: redis
  default_ttl: 1h
  config:
    host: localhost
    port: 6379
    password: \"\"
    db: 0
    pool_size: 10"
        else
            cache_config="cache:
  enabled: true
  type: memory
  default_ttl: 1h"
        fi
    else
        cache_config="cache:
  enabled: false"
    fi

    # Metrics configuration
    if [[ " ${SELECTED_FEATURES[*]} " == *" metrics "* ]]; then
        if [ "$METRICS_TYPE" = "prometheus" ]; then
            metrics_config="metrics:
  enabled: true
  type: prometheus
  http:
    enabled: true
    path: /metrics
    port: 9090
  collectors:
    system: true
    runtime: true
    http: true"
        else
            metrics_config="metrics:
  enabled: true
  type: memory
  collectors:
    system: true
    runtime: true
    http: true"
        fi
    else
        metrics_config="metrics:
  enabled: false"
    fi

    # Actions configuration
    if [[ " ${SELECTED_FEATURES[*]} " == *" actions "* ]]; then
        actions_config="actions:
  enabled: true
  broker:
    enabled: true
    type: $BROKER_TYPE
  webhooks:
    enabled: true"
    else
        actions_config="actions:
  enabled: false"
    fi

    # Middleware configuration
    if [[ " ${SELECTED_FEATURES[*]} " == *" middleware "* ]]; then
        middleware_config="middlewares:
  enabled: true"

        for middleware in "${SELECTED_MIDDLEWARES[@]}"; do
            case "$middleware" in
                "auth")
                    middleware_config="${middleware_config}
  auth:
    enabled: true
    weight: 60
    params:
      token: \"your-secret-token\""
                    ;;
                "cache")
                    middleware_config="${middleware_config}
  cache:
    enabled: true
    weight: 80
    params:
      default_ttl: 5m"
                    ;;
                "recovery")
                    middleware_config="${middleware_config}
  recovery:
    enabled: true
    weight: 10
    params:
      stack_trace: true"
                    ;;
                "logging")
                    middleware_config="${middleware_config}
  logging:
    enabled: true
    weight: 20
    params:
      log_level: info
      log_headers: false
      log_body: false"
                    ;;
                "ratelimit")
                    middleware_config="${middleware_config}
  rate_limit:
    enabled: true
    weight: 30
    params:
      requests_per_minute: 100"
                    ;;
                "bodylimit")
                    middleware_config="${middleware_config}
  body_limit:
    enabled: true
    weight: 40
    params:
      max_body_size: 10485760"
                    ;;
                "cors")
                    middleware_config="${middleware_config}
  cors:
    enabled: true
    weight: 50
    params:
      allowed_origins: [\"*\"]
      allowed_methods: [\"GET\", \"POST\", \"PUT\", \"DELETE\", \"OPTIONS\"]
      allowed_headers: [\"Content-Type\", \"Authorization\", \"X-API-Key\"]
      max_age: 86400"
                    ;;
                "compression")
                    middleware_config="${middleware_config}
  compression:
    enabled: true
    weight: 70
    params:
      algorithm: gzip
      level: 6
      threshold: 1024"
                    ;;
            esac
        done
    else
        middleware_config="middlewares:
  enabled: false"
    fi

    # Generate the config file
    cat > "$PROJECT_NAME/config.yml" << EOF
name: $PROJECT_NAME
version: 1.0.0

server:
  http:
    host: 0.0.0.0
    port: $PORT
    read_timeout: 30
    write_timeout: 30
    idle_timeout: 120
  tls:
    enabled: false

logger:
  level: info
  type: default
  config:
    format: console
    output: stdout

$cache_config

$actions_config

cron:
  enabled: $([ "${SELECTED_FEATURES[*]}" == *"cron"* ] && echo "true" || echo "false")
  timezone: UTC

auth_providers:
  token:
    params:
      token: "your-secret-token"
  basic:
    params:
      username: "admin"
      password: "admin"

$middleware_config

docs:
  enabled: $([ "${SELECTED_FEATURES[*]}" == *"docs"* ] && echo "true" || echo "false")
  path: /docs

$metrics_config

health:
  enabled: $([ "${SELECTED_FEATURES[*]}" == *"health"* ] && echo "true" || echo "false")

client:
  enabled: $([ "${SELECTED_FEATURES[*]}" == *"client"* ] && echo "true" || echo "false")
  default_timeout: 30s
  max_idle_connections: 100
  idle_conn_timeout: 90s
  default_retries: 3
  circuit_breaker:
    enabled: true
    failure_threshold: 5
    recovery_timeout: 60s
    half_open_requests: 3
EOF
}

generate_go_mod() {
    cat > "$PROJECT_NAME/go.mod" << EOF
module $MODULE_NAME

go 1.21

require (
	github.com/saiset-co/sai-service v1.1.0
	github.com/valyala/fasthttp v1.51.0
)

require (
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/bytedance/sonic v1.10.2 // indirect
	github.com/go-playground/validator/v10 v10.16.0 // indirect
	github.com/klauspost/compress v1.17.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	go.uber.org/zap v1.26.0 // indirect
	golang.org/x/sync v0.5.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
EOF
}

generate_dockerfile() {
    cat > "$PROJECT_NAME/Dockerfile" << EOF
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

# Copy binary and config
COPY --from=builder /app/main .
COPY --from=builder /app/config.yml .

# Create non-root user
RUN adduser -D -s /bin/sh appuser
USER appuser

EXPOSE $PORT

CMD ["./main"]
EOF
}

generate_docker_compose() {
    local services="  $PROJECT_NAME:
    build: .
    ports:
      - \"$PORT:$PORT\"
    environment:
      - ENV=development
    volumes:
      - ./config.yml:/root/config.yml"

    local additional_services=""
    local depends_on=""

    # Add Redis if cache type is redis
    if [ "$CACHE_TYPE" = "redis" ]; then
        additional_services="${additional_services}
  redis:
    image: redis:7-alpine
    ports:
      - \"6379:6379\"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes"
        depends_on="${depends_on}
      - redis"
    fi

    # Add Prometheus if metrics type is prometheus
    if [ "$METRICS_TYPE" = "prometheus" ]; then
        additional_services="${additional_services}
  prometheus:
    image: prom/prometheus:latest
    ports:
      - \"9090:9090\"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
      - '--storage.tsdb.retention.time=200h'
      - '--web.enable-lifecycle'"
        depends_on="${depends_on}
      - prometheus"
    fi

    # Add depends_on if we have dependencies
    if [ -n "$depends_on" ]; then
        services="${services}
    depends_on:${depends_on}"
    fi

    local volumes=""
    if [ "$CACHE_TYPE" = "redis" ]; then
        volumes="${volumes}
  redis_data:"
    fi
    if [ "$METRICS_TYPE" = "prometheus" ]; then
        volumes="${volumes}
  prometheus_data:"
    fi

    cat > "$PROJECT_NAME/docker-compose.yml" << EOF
version: '3.8'

services:
${services}${additional_services}
$(if [ -n "$volumes" ]; then echo "
volumes:${volumes}"; fi)
EOF
}

generate_prometheus_config() {
    if [ "$METRICS_TYPE" = "prometheus" ]; then
        cat > "$PROJECT_NAME/prometheus.yml" << EOF
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  # - "first_rules.yml"
  # - "second_rules.yml"

scrape_configs:
  - job_name: '$PROJECT_NAME'
    static_configs:
      - targets: ['$PROJECT_NAME:9090']
    scrape_interval: 5s
    metrics_path: /metrics

  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']
EOF
    fi
}

generate_makefile() {
    cat > "$PROJECT_NAME/Makefile" << EOF
.PHONY: build run test clean docker-build docker-run docker-stop install-deps fmt lint

# Go parameters
GOCMD=go
GOBUILD=\$(GOCMD) build
GOCLEAN=\$(GOCMD) clean
GOTEST=\$(GOCMD) test
GOGET=\$(GOCMD) get
GOMOD=\$(GOCMD) mod
BINARY_NAME=$PROJECT_NAME
BINARY_PATH=./cmd

# Build the application
build:
	\$(GOBUILD) -o \$(BINARY_NAME) \$(BINARY_PATH)

# Run the application
run:
	\$(GOBUILD) -o \$(BINARY_NAME) \$(BINARY_PATH) && ./\$(BINARY_NAME)

# Run with live reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	air

# Test the application
test:
	\$(GOTEST) -v ./...

# Test with coverage
test-coverage:
	\$(GOTEST) -v -coverprofile=coverage.out ./...
	\$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	\$(GOCLEAN)
	rm -f \$(BINARY_NAME)
	rm -f coverage.out coverage.html

# Install dependencies
install-deps:
	\$(GOMOD) download
	\$(GOMOD) tidy

# Format code
fmt:
	\$(GOCMD) fmt ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Security check (requires gosec)
security:
	gosec ./...

# Build Docker image
docker-build:
	docker build -t \$(BINARY_NAME):latest .

# Run with Docker Compose
docker-run:
	docker-compose up --build

# Stop Docker Compose
docker-stop:
	docker-compose down

# Run in background
docker-run-bg:
	docker-compose up -d --build

# View logs
docker-logs:
	docker-compose logs -f

# Database operations (if using database)
db-migrate:
	# Add your migration commands here
	@echo "Add database migration commands"

db-seed:
	# Add your seeding commands here
	@echo "Add database seeding commands"

# Generate API documentation
docs:
	# Swagger/OpenAPI docs generation if needed
	@echo "API docs available at http://localhost:\$(PORT)/docs"

# Performance test (requires wrk or ab)
perf-test:
	wrk -t12 -c400 -d30s http://localhost:$PORT/health

# Load test specific endpoint
load-test:
	wrk -t12 -c400 -d30s http://localhost:$PORT/api/v1/users

# Help
help:
	@echo "Available commands:"
	@echo "  build          - Build the application"
	@echo "  run            - Build and run the application"
	@echo "  dev            - Run with live reload (requires air)"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  clean          - Clean build artifacts"
	@echo "  install-deps   - Install/update dependencies"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
	@echo "  security       - Run security checks"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run with Docker Compose"
	@echo "  docker-stop    - Stop Docker Compose"
	@echo "  docker-run-bg  - Run in background"
	@echo "  docker-logs    - View logs"
	@echo "  perf-test      - Run performance test"
	@echo "  load-test      - Run load test"
	@echo "  help           - Show this help"
EOF
}

generate_readme() {
    local features_list=""
    if [ ${#SELECTED_FEATURES[@]} -gt 0 ]; then
        features_list="## Features

"
        for feature in "${SELECTED_FEATURES[@]}"; do
            features_list="${features_list}- ${feature}
"
        done
    fi

    local middleware_list=""
    if [ ${#SELECTED_MIDDLEWARES[@]} -gt 0 ]; then
        middleware_list="
### Enabled Middlewares

"
        for middleware in "${SELECTED_MIDDLEWARES[@]}"; do
            middleware_list="${middleware_list}- ${middleware}
"
        done
    fi

    cat > "$PROJECT_NAME/README.md" << EOF
# $PROJECT_NAME

A high-performance microservice built with [SAI Service](https://github.com/saiset-co/sai-service) framework.

$features_list

## Quick Start

### Prerequisites

- Go 1.21 or later
- Docker and Docker Compose (optional)

### Local Development

1. Clone and navigate to the project:
\`\`\`bash
cd $PROJECT_NAME
\`\`\`

2. Install dependencies:
\`\`\`bash
make install-deps
\`\`\`

3. Run the service:
\`\`\`bash
make run
\`\`\`

The service will be available at \`http://localhost:$PORT\`

### With Docker

1. Build and run with Docker Compose:
\`\`\`bash
make docker-run
\`\`\`

2. Stop the service:
\`\`\`bash
make docker-stop
\`\`\`

## API Endpoints

### Health Check
- \`GET /health\` - Service health status

### Users API
- \`GET /api/v1/users\` - List all users
- \`GET /api/v1/users/{id}\` - Get user by ID
- \`POST /api/v1/users\` - Create new user
- \`PUT /api/v1/users/{id}\` - Update user
- \`DELETE /api/v1/users/{id}\` - Delete user

$([ "$([[ " ${SELECTED_FEATURES[*]} " == *" docs "* ]] && echo "true" || echo "false")" = "true" ] && echo "### API Documentation
Interactive API documentation is available at \`http://localhost:$PORT/docs\`")

## Configuration

The service is configured via \`config.yml\`. Key configuration sections:

- **Server**: HTTP server settings
- **Logger**: Logging configuration$([ "$([[ " ${SELECTED_FEATURES[*]} " == *" cache "* ]] && echo "true" || echo "false")" = "true" ] && echo "
- **Cache**: Caching configuration ($CACHE_TYPE)")$([ "$([[ " ${SELECTED_FEATURES[*]} " == *" metrics "* ]] && echo "true" || echo "false")" = "true" ] && echo "
- **Metrics**: Metrics collection ($METRICS_TYPE)")$([ "$([[ " ${SELECTED_FEATURES[*]} " == *" actions "* ]] && echo "true" || echo "false")" = "true" ] && echo "
- **Actions**: Event handling and webhooks")$middleware_list

## Development

### Available Commands

\`\`\`bash
make help  # Show all available commands
\`\`\`

### Testing

Run tests:
\`\`\`bash
make test
\`\`\`

Run tests with coverage:
\`\`\`bash
make test-coverage
\`\`\`

### Code Quality

Format code:
\`\`\`bash
make fmt
\`\`\`

Lint code:
\`\`\`bash
make lint
\`\`\`

Security check:
\`\`\`bash
make security
\`\`\`

### Performance Testing

Run performance tests:
\`\`\`bash
make perf-test
\`\`\`

## Project Structure

\`\`\`
$PROJECT_NAME/
├── cmd/                    # Application entry point
│   └── main.go
├── internal/               # Private application code
│   ├── service.go         # Service initialization
│   ├── handlers/          # HTTP handlers
│   │   └── handlers.go
│   └── models/            # Data models
│       └── model.go$([ "$INCLUDE_TESTS" = "true" ] && echo "
├── tests/                 # Integration tests
│   ├── integration/
│   └── helpers/")$([ "$CICD_TYPE" != "none" ] && echo "
├── .github/workflows/     # CI/CD workflows
│   └── ci.yml")
├── config.yml             # Configuration
├── Dockerfile             # Docker image
├── docker-compose.yml     # Multi-container setup
├── Makefile              # Development commands
├── go.mod                # Go module
└── README.md             # This file
\`\`\`

## Monitoring

$([ "$([[ " ${SELECTED_FEATURES[*]} " == *" health "* ]] && echo "true" || echo "false")" = "true" ] && echo "### Health Checks
- Service health: \`GET /health\`")

$([ "$([[ " ${SELECTED_FEATURES[*]} " == *" metrics "* ]] && echo "true" || echo "false")" = "true" ] && echo "### Metrics
$([ "$METRICS_TYPE" = "prometheus" ] && echo "- Prometheus metrics: \`http://localhost:9090/metrics\`
- Prometheus UI: \`http://localhost:9090\`" || echo "- Metrics endpoint: \`GET /metrics\`")")

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run tests and linting
6. Submit a pull request

## License

This project is licensed under the MIT License.
EOF
}

generate_gitignore() {
    cat > "$PROJECT_NAME/.gitignore" << EOF
# Binaries for programs and plugins
*.exe
*.exe~
*.dll
*.so
*.dylib
$PROJECT_NAME

# Test binary, built with \`go test -c\`
*.test

# Output of the go coverage tool
*.out
coverage.html

# Dependency directories
vendor/

# Go workspace file
go.work

# IDE files
.vscode/
.idea/
*.swp
*.swo
*~

# OS generated files
.DS_Store
.DS_Store?
._*
.Spotlight-V100
.Trashes
ehthumbs.db
Thumbs.db

# Logs
*.log
logs/

# Local environment files
.env
.env.local
.env.development.local
.env.test.local
.env.production.local

# Database files
*.db
*.sqlite
*.sqlite3

# Docker volumes
docker-volumes/

# Build artifacts
dist/
build/

# Temporary files
tmp/
temp/
*.tmp

# Configuration overrides
config.local.yml
config.*.yml
!config.yml

# Cache directories
.cache/
node_modules/

# Application specific
webhooks.db
certs/
EOF
}

generate_integration_tests() {
    if [ "$INCLUDE_TESTS" = "true" ]; then
        mkdir -p "$PROJECT_NAME/tests/integration"
        mkdir -p "$PROJECT_NAME/tests/helpers"

        cat > "$PROJECT_NAME/tests/integration/api_test.go" << EOF
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"$MODULE_NAME/tests/helpers"
)

func TestUserAPI(t *testing.T) {
	// Setup test server
	baseURL := helpers.SetupTestServer(t)

	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("Health Check", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/health")
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Get Users", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/api/v1/users")
		if err != nil {
			t.Fatalf("Get users failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Create User", func(t *testing.T) {
		user := map[string]interface{}{
			"name":  "Test User",
			"email": "test@example.com",
			"age":   25,
		}

		jsonData, _ := json.Marshal(user)
		resp, err := client.Post(
			baseURL+"/api/v1/users",
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			t.Fatalf("Create user failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", resp.StatusCode)
		}
	})

	t.Run("Get User by ID", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/api/v1/users/1")
		if err != nil {
			t.Fatalf("Get user by ID failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Update User", func(t *testing.T) {
		user := map[string]interface{}{
			"name": "Updated User",
		}

		jsonData, _ := json.Marshal(user)
		req, err := http.NewRequest(
			http.MethodPut,
			baseURL+"/api/v1/users/1",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			t.Fatalf("Create request failed: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Update user failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Delete User", func(t *testing.T) {
		req, err := http.NewRequest(
			http.MethodDelete,
			baseURL+"/api/v1/users/1",
			nil,
		)
		if err != nil {
			t.Fatalf("Create request failed: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Delete user failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", resp.StatusCode)
		}
	})
}

func TestErrorHandling(t *testing.T) {
	baseURL := helpers.SetupTestServer(t)

	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("Get Non-existent User", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/api/v1/users/999")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("Create User with Invalid Data", func(t *testing.T) {
		user := map[string]interface{}{
			"name": "", // Invalid: empty name
			"email": "invalid-email",
			"age": -1,
		}

		jsonData, _ := json.Marshal(user)
		resp, err := client.Post(
			baseURL+"/api/v1/users",
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}
EOF

        cat > "$PROJECT_NAME/tests/helpers/setup.go" << EOF
package helpers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/saiset-co/sai-service/service"
	"$MODULE_NAME/internal"
)

// SetupTestServer creates a test server for integration tests
// Returns the base URL of the running server
func SetupTestServer(t *testing.T) string {
	ctx := context.Background()

	// Create service with test configuration
	// Path is relative to the project root where tests are run from
	srv, err := service.NewService(ctx, "config.yml")
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Register business logic
	if err := internal.RegisterBusinessLogic(); err != nil {
		t.Fatalf("Failed to register business logic: %v", err)
	}

	// Start service in background
	go func() {
		if err := srv.Start(); err != nil {
			t.Logf("Service start error: %v", err)
		}
	}()

	// Wait for service to be ready
	time.Sleep(2 * time.Second)

	// Get server configuration for base URL
	config := srv.Context() // Assuming we can get config somehow
	baseURL := "http://localhost:8080" // Default for tests

	// Cleanup function
	t.Cleanup(func() {
		srv.Stop()
	})

	return baseURL
}

// CreateTestUser creates a test user for use in tests
func CreateTestUser() map[string]interface{} {
	return map[string]interface{}{
		"name":  "Test User",
		"email": "test@example.com",
		"age":   25,
	}
}
EOF
    fi
}

generate_github_workflow() {
    if [ "$CICD_TYPE" = "github" ]; then
        mkdir -p "$PROJECT_NAME/.github/workflows"

        cat > "$PROJECT_NAME/.github/workflows/ci.yml" << EOF
name: CI/CD Pipeline

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

env:
  GO_VERSION: '1.21'
  REGISTRY: ghcr.io
  IMAGE_NAME: \${{ github.repository }}

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest

    services:$([ "$CACHE_TYPE" = "redis" ] && echo "
      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379
        options: >-
          --health-cmd \"redis-cli ping\"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5")$([ "$METRICS_TYPE" = "prometheus" ] && echo "
      prometheus:
        image: prom/prometheus:latest
        ports:
          - 9090:9090")

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: \${{ env.GO_VERSION }}

    - name: Cache Go modules
      uses: actions/cache@v3
      with:
        path: ~/go/pkg/mod
        key: \${{ runner.os }}-go-\${{ hashFiles('**/go.sum') }}
        restore-keys: |
          \${{ runner.os }}-go-

    - name: Install dependencies
      run: go mod download

    - name: Run tests
      run: go test -v -race -coverprofile=coverage.out ./...

    - name: Upload coverage to Codecov
      uses: codecov/codecov-action@v3
      with:
        file: ./coverage.out
        flags: unittests
        name: codecov-umbrella

  lint:
    name: Lint
    runs-on: ubuntu-latest

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: \${{ env.GO_VERSION }}

    - name: Run golangci-lint
      uses: golangci/golangci-lint-action@v3
      with:
        version: latest
        args: --timeout=5m

  security:
    name: Security Scan
    runs-on: ubuntu-latest

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: \${{ env.GO_VERSION }}

    - name: Run Gosec Security Scanner
      uses: securecodewarrior/github-action-gosec@master
      with:
        args: './...'

  build:
    name: Build
    runs-on: ubuntu-latest
    needs: [test, lint]

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: \${{ env.GO_VERSION }}

    - name: Build binary
      run: |
        CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $PROJECT_NAME ./cmd

    - name: Upload binary
      uses: actions/upload-artifact@v3
      with:
        name: $PROJECT_NAME-binary
        path: $PROJECT_NAME

  docker:
    name: Build Docker Image
    runs-on: ubuntu-latest
    needs: [test, lint]
    if: github.event_name == 'push'

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v3

    - name: Log in to Container Registry
      uses: docker/login-action@v3
      with:
        registry: \${{ env.REGISTRY }}
        username: \${{ github.actor }}
        password: \${{ secrets.GITHUB_TOKEN }}

    - name: Extract metadata
      id: meta
      uses: docker/metadata-action@v5
      with:
        images: \${{ env.REGISTRY }}/\${{ env.IMAGE_NAME }}
        tags: |
          type=ref,event=branch
          type=ref,event=pr
          type=sha,prefix={{branch}}-
          type=raw,value=latest,enable={{is_default_branch}}

    - name: Build and push Docker image
      uses: docker/build-push-action@v5
      with:
        context: .
        platforms: linux/amd64,linux/arm64
        push: true
        tags: \${{ steps.meta.outputs.tags }}
        labels: \${{ steps.meta.outputs.labels }}
        cache-from: type=gha
        cache-to: type=gha,mode=max

  deploy:
    name: Deploy
    runs-on: ubuntu-latest
    needs: [docker]
    if: github.ref == 'refs/heads/main'
    environment: production

    steps:
    - name: Deploy to production
      run: |
        echo "Add your deployment steps here"
        echo "For example: kubectl, docker-compose, etc."
EOF
    fi
}

generate_gitlab_ci() {
    if [ "$CICD_TYPE" = "gitlab" ]; then
        cat > "$PROJECT_NAME/.gitlab-ci.yml" << EOF
stages:
  - test
  - build
  - deploy

variables:
  GO_VERSION: "1.21"
  DOCKER_DRIVER: overlay2
  DOCKER_TLS_CERTDIR: "/certs"

before_script:
  - go version
  - go mod download

test:
  stage: test
  image: golang:\$GO_VERSION
  services:$([ "$CACHE_TYPE" = "redis" ] && echo "
    - redis:7-alpine")$([ "$METRICS_TYPE" = "prometheus" ] && echo "
    - prom/prometheus:latest")
  script:
    - go test -v -race -coverprofile=coverage.out ./...
    - go tool cover -html=coverage.out -o coverage.html
  artifacts:
    reports:
      coverage_report:
        coverage_format: cobertura
        path: coverage.xml
    paths:
      - coverage.html
      - coverage.out
  coverage: '/coverage: \d+\.\d+% of statements/'

lint:
  stage: test
  image: golangci/golangci-lint:latest
  script:
    - golangci-lint run --timeout=5m

security:
  stage: test
  image: golang:\$GO_VERSION
  script:
    - go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
    - gosec ./...

build:
  stage: build
  image: golang:\$GO_VERSION
  script:
    - CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $PROJECT_NAME ./cmd
  artifacts:
    paths:
      - $PROJECT_NAME
    expire_in: 1 week

docker:
  stage: build
  image: docker:latest
  services:
    - docker:dind
  script:
    - docker build -t \$CI_REGISTRY_IMAGE:\$CI_COMMIT_SHA .
    - docker push \$CI_REGISTRY_IMAGE:\$CI_COMMIT_SHA
  only:
    - main
    - develop

deploy:
  stage: deploy
  image: alpine:latest
  script:
    - echo "Add your deployment steps here"
  only:
    - main
  environment:
    name: production
EOF
    fi
}

# Directory creation
create_project_structure() {
    print_info "Creating project structure..."

    mkdir -p "$PROJECT_NAME"/{cmd,internal/{handlers,models}}

    if [ "$INCLUDE_TESTS" = "true" ]; then
        mkdir -p "$PROJECT_NAME/tests"/{integration,helpers}
    fi

    if [ "$CICD_TYPE" = "github" ]; then
        mkdir -p "$PROJECT_NAME/.github/workflows"
    fi

    print_success "Project structure created"
}

# File generation
generate_project_files() {
    print_info "Generating project files..."

    generate_main_go
    generate_service_go
    generate_handlers_go
    generate_models_go
    generate_config_yml
    generate_go_mod
    generate_dockerfile
    generate_docker_compose
    generate_prometheus_config
    generate_makefile
    generate_readme
    generate_gitignore

    if [ "$INCLUDE_TESTS" = "true" ]; then
        generate_integration_tests
    fi

    if [ "$CICD_TYPE" = "github" ]; then
        generate_github_workflow
    elif [ "$CICD_TYPE" = "gitlab" ]; then
        generate_gitlab_ci
    fi

    print_success "Project files generated"
}

# Post-generation tasks
post_generation() {
    print_info "Finalizing project..."

    cd "$PROJECT_NAME"

    # Initialize git repository
    if command -v git &> /dev/null; then
        git init
        git add .
        git commit -m "Initial commit: Generated $PROJECT_NAME with SAI Service"
        print_success "Git repository initialized"
    fi

    # Generate go.sum
    if command -v go &> /dev/null; then
        go mod tidy
        print_success "Go modules initialized"
    fi

    cd ..
}

# Main execution
main() {
    print_header

    # Collect configuration
    collect_configuration

    # Check if project directory exists after input
    if [ -d "$PROJECT_NAME" ]; then
        print_error "Directory '$PROJECT_NAME' already exists!"
        exit 1
    fi

    # Show configuration summary
    show_configuration

    # Confirm generation
    proceed=$(prompt_yes_no "Proceed with generation?" "y")
    if [ "$proceed" != "true" ]; then
        echo -e "${YELLOW}Generation cancelled.${NC}"
        exit 0
    fi

    echo
    print_info "Starting project generation..."

    # Create project structure
    create_project_structure

    # Generate all files
    generate_project_files

    # Post-generation tasks
    post_generation

    # Success message
    echo
    print_success "Project '$PROJECT_NAME' generated successfully!"
    echo
    echo -e "${CYAN}Next steps:${NC}"
    echo -e "${GREEN}  1.${NC} cd $PROJECT_NAME"
    echo -e "${GREEN}  2.${NC} make install-deps"
    echo -e "${GREEN}  3.${NC} make run"
    echo
    echo -e "${BLUE}Available commands:${NC}"
    echo -e "${GREEN}  make help${NC}          - Show all available commands"
    echo -e "${GREEN}  make run${NC}           - Run the service locally"
    echo -e "${GREEN}  make docker-run${NC}    - Run with Docker Compose"
    echo -e "${GREEN}  make test${NC}          - Run tests"
    echo -e "${GREEN}  make fmt${NC}           - Format code"
    echo

    if [[ " ${SELECTED_FEATURES[*]} " == *" docs "* ]]; then
        echo -e "${BLUE}Documentation:${NC}"
        echo -e "${GREEN}  http://localhost:$PORT/docs${NC} - API Documentation"
        echo
    fi

    if [[ " ${SELECTED_FEATURES[*]} " == *" health "* ]]; then
        echo -e "${BLUE}Health Check:${NC}"
        echo -e "${GREEN}  http://localhost:$PORT/health${NC} - Service Health"
        echo
    fi

    if [ "$METRICS_TYPE" = "prometheus" ]; then
        echo -e "${BLUE}Monitoring:${NC}"
        echo -e "${GREEN}  http://localhost:9090${NC} - Prometheus UI"
        echo -e "${GREEN}  http://localhost:$PORT/metrics${NC} - Metrics Endpoint"
        echo
    fi

    echo -e "${PURPLE}Happy coding! 🚀${NC}"
}

# Check dependencies
check_dependencies() {
    local missing_deps=()

    if ! command -v go &> /dev/null; then
        missing_deps+=("go")
    fi

    if [ ${#missing_deps[@]} -gt 0 ]; then
        print_warning "Missing dependencies: ${missing_deps[*]}"
        print_info "Please install the missing dependencies and run the generator again."
        print_info "The project files will still be generated, but you may need to run 'go mod tidy' manually."
    fi
}

# Help function
show_help() {
    cat << EOF
SAI Service Generator

USAGE:
    $0 [PROJECT_NAME]

DESCRIPTION:
    Generates a complete microservice project based on SAI Service library.
    If PROJECT_NAME is provided, it will be used as the default project name.

EXAMPLES:
    $0                          # Interactive mode
    $0 my-awesome-api          # Set project name upfront

OPTIONS:
    -h, --help                 Show this help message

FEATURES:
    Templates:    basic, api, microservice, full
    Features:     cache, metrics, docs, cron, actions, tls, middleware, health, client
    Middlewares:  cache, auth, bodylimit, compression, cors, logging, ratelimit, recovery
    Cache Types:  memory, redis
    Metrics:      memory, prometheus
    CI/CD:        none, github, gitlab

GENERATED STRUCTURE:
    project-name/
    ├── cmd/                   # Application entry point
    ├── internal/              # Private application code
    ├── tests/                 # Integration tests (optional)
    ├── .github/workflows/     # CI/CD workflows (optional)
    ├── config.yml             # Configuration
    ├── Dockerfile             # Container image
    ├── docker-compose.yml     # Multi-container setup
    ├── Makefile              # Development commands
    ├── go.mod                # Go module
    └── README.md             # Documentation

EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -*)
                print_error "Unknown option: $1"
                show_help
                exit 1
                ;;
            *)
                if [ -z "$PROJECT_NAME" ]; then
                    PROJECT_NAME="$1"
                else
                    print_error "Multiple project names provided: '$PROJECT_NAME' and '$1'"
                    exit 1
                fi
                ;;
        esac
        shift
    done
}

# Script entry point
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    # Parse command line arguments
    parse_args "$@"

    # Check dependencies
    check_dependencies

    # Run main function
    main
fi
