#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

PROJECT_NAME=""
PKG_NAME=""
TEMPLATE=""
FEATURES=""
AUTH_TYPES=""
CACHE_TYPE=""
DATABASE_TYPE=""
METRICS_TYPE=""
ACTIONS=""
MIDDLEWARES=""
INCLUDE_TESTS="false"
CICD_TYPE=""

print_info() {
    echo -e "${BLUE}ℹ ${1}${NC}"
}

print_success() {
    echo -e "${GREEN}✓ ${1}${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ ${1}${NC}"
}

print_error() {
    echo -e "${RED}✗ ${1}${NC}"
}

print_header() {
    echo -e "${PURPLE}${1}${NC}"
}

show_help() {
    echo -e "${CYAN}SAI Service Generator${NC}"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --name STRING          Project name"
    echo "  --pkg STRING           Go package name (e.g., github.com/user/project)"
    echo "  --template STRING      Template type: basic,api,microservice,full,custom"
    echo "  --features STRING      Comma-separated features: auth,cache,database,metrics,docs,cron,actions,tls,middlewares,health,client"
    echo "  --auth STRING          Auth types: basic,token (comma-separated)"
    echo "  --cache STRING         Cache type: memory,redis"
    echo "  --database STRING      Database type: clover,memory"
    echo "  --metrics STRING       Metrics type: memory,prometheus"
    echo "  --actions STRING       Actions: websocket,webhook (comma-separated)"
    echo "  --middlewares STRING   Middlewares: auth,bodylimit,cache,compression,cors,logging,ratelimit,recovery (comma-separated)"
    echo "  --test                 Include integration tests"
    echo "  --cicd STRING          CI/CD type: none,github,gitlab"
    echo "  --help                 Show this help message"
    echo ""
    echo "Templates:"
    echo "  basic       - Minimal web server"
    echo "  api         - REST API service with CRUD"
    echo "  microservice- Microservice with actions"
    echo "  full        - Full-featured service"
    echo "  custom      - Custom configuration"
    echo ""
    echo "Examples:"
    echo "  $0 --name \"Hello World\" --pkg \"github.com/user/hello\" --features \"auth,cache\" --middlewares \"auth,recovery,logging\""
    echo "  $0"
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --name)
                PROJECT_NAME="$2"
                shift 2
                ;;
            --pkg)
                PKG_NAME="$2"
                shift 2
                ;;
            --template)
                TEMPLATE="$2"
                shift 2
                ;;
            --features)
                FEATURES="$2"
                shift 2
                ;;
            --auth)
                AUTH_TYPES="$2"
                shift 2
                ;;
            --cache)
                CACHE_TYPE="$2"
                shift 2
                ;;
            --database)
                DATABASE_TYPE="$2"
                shift 2
                ;;
            --metrics)
                METRICS_TYPE="$2"
                shift 2
                ;;
            --actions)
                ACTIONS="$2"
                shift 2
                ;;
            --middlewares)
                MIDDLEWARES="$2"
                shift 2
                ;;
            --test)
                INCLUDE_TESTS="true"
                shift
                ;;
            --cicd)
                CICD_TYPE="$2"
                shift 2
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

interactive_mode() {
    print_header "=== SAI Service Generator - Interactive Mode ==="
    echo ""

    read -p "Project name: " PROJECT_NAME
    while [[ -z "$PROJECT_NAME" ]]; do
        print_warning "Project name is required"
        read -p "Project name: " PROJECT_NAME
    done

    local default_pkg=$(echo "$PROJECT_NAME" | tr '[:upper:]' '[:lower:]' | sed 's/ /-/g')
    read -p "Go pkg name [$default_pkg]: " PKG_NAME
    if [[ -z "$PKG_NAME" ]]; then
        PKG_NAME="$default_pkg"
    fi

    echo ""
    print_info "Available templates: custom basic api microservice full"
    read -p "Select template [custom]: " TEMPLATE
    if [[ -z "$TEMPLATE" ]]; then
        TEMPLATE="custom"
    fi

    if [[ "$TEMPLATE" != "custom" ]]; then
        apply_template_preset "$TEMPLATE"
    else
        configure_custom_template
    fi

    echo ""
    read -p "Include integration tests? [y/N]: " test_choice
    if [[ "$test_choice" =~ ^[Yy]$ ]]; then
        INCLUDE_TESTS="true"
    fi

    echo ""
    print_info "Available CI/CD: none github gitlab"
    read -p "Generate CI/CD files [none]: " CICD_TYPE
    if [[ -z "$CICD_TYPE" ]]; then
        CICD_TYPE="none"
    fi
}

apply_template_preset() {
    local template=$1

    case $template in
        basic)
            FEATURES="health,cache"
            MIDDLEWARES="recovery,logging"
            AUTH_TYPES=""
            CACHE_TYPE="memory"
            METRICS_TYPE=""
            ACTIONS=""
            ;;
        api)
            FEATURES="health,middlewares,docs,cache,database"
            MIDDLEWARES="auth,cache,recovery,logging,cors,bodylimit"
            AUTH_TYPES="token"
            CACHE_TYPE="redis"
            DATABASE_TYPE="clover"
            METRICS_TYPE=""
            ACTIONS=""
            ;;
        microservice)
            FEATURES="health,middlewares,docs,cache,database,client,actions"
            MIDDLEWARES="auth,cache,recovery,logging,cors,bodylimit"
            AUTH_TYPES="token"
            CACHE_TYPE="redis"
            DATABASE_TYPE="clover"
            METRICS_TYPE=""
            ACTIONS="webhook"
            ;;
        full)
            FEATURES="auth,cache,database,metrics,docs,cron,actions,tls,middlewares,health,client"
            MIDDLEWARES="auth,bodylimit,cache,compression,cors,logging,ratelimit,recovery"
            AUTH_TYPES="basic,token"
            CACHE_TYPE="redis"
            DATABASE_TYPE="clover"
            METRICS_TYPE="prometheus"
            ACTIONS="websocket,webhook"
            ;;
    esac
}

configure_custom_template() {
    echo ""
    print_info "Available features: auth cache database metrics docs cron actions tls middlewares health client"
    read -p "Enable features (comma-separated): " FEATURES

    if [[ "$FEATURES" == *"actions"* ]]; then
        echo ""
        read -p "Enable websocket actions? [y/N]: " websocket_choice
        read -p "Enable webhooks actions? [y/N]: " webhook_choice

        ACTIONS=""
        if [[ "$websocket_choice" =~ ^[Yy]$ ]]; then
            ACTIONS="websocket"
        fi
        if [[ "$webhook_choice" =~ ^[Yy]$ ]]; then
            if [[ -n "$ACTIONS" ]]; then
                ACTIONS="$ACTIONS,webhook"
            else
                ACTIONS="webhook"
            fi
        fi
    fi

    if [[ "$FEATURES" == *"auth"* ]]; then
        echo ""
        read -p "Enable basic auth support? [y/N]: " basic_choice
        read -p "Enable token auth support? [y/N]: " token_choice

        AUTH_TYPES=""
        if [[ "$basic_choice" =~ ^[Yy]$ ]]; then
            AUTH_TYPES="basic"
        fi
        if [[ "$token_choice" =~ ^[Yy]$ ]]; then
            if [[ -n "$AUTH_TYPES" ]]; then
                AUTH_TYPES="$AUTH_TYPES,token"
            else
                AUTH_TYPES="token"
            fi
        fi
    fi

    if [[ "$FEATURES" == *"cache"* ]]; then
        echo ""
        print_info "Available cache: memory redis"
        read -p "Select cache type: " CACHE_TYPE
    fi

    if [[ "$FEATURES" == *"database"* ]]; then
        echo ""
        print_info "Available database: clover memory"
        read -p "Select database type: " DATABASE_TYPE
    fi

    if [[ "$FEATURES" == *"metrics"* ]]; then
        echo ""
        print_info "Available metrics: memory prometheus"
        read -p "Select metrics type: " METRICS_TYPE
    fi

    if [[ "$FEATURES" == *"middlewares"* ]]; then
        echo ""
        local available_mw="recovery,logging,ratelimit,bodylimit,cors"

        if [[ "$FEATURES" == *"cache"* ]]; then
            available_mw="$available_mw,cache"
        fi
        if [[ "$FEATURES" == *"auth"* ]]; then
            available_mw="$available_mw,auth"
        fi

        available_mw="$available_mw,compression"

        print_info "Available middlewares: $available_mw"
        read -p "Enable middlewares (comma-separated): " MIDDLEWARES
    fi
}

validate_configuration() {
    echo ""
    print_header "=== Configuration ==="
    echo "   • Project: $PROJECT_NAME"
    echo "   • Module: $PKG_NAME"
    if [[ -n "$TEMPLATE" && "$TEMPLATE" != "custom" ]]; then
        echo "   • Template: $TEMPLATE"
    fi
    if [[ -n "$FEATURES" ]]; then
        echo "   • Features: $FEATURES"
    fi
    if [[ -n "$AUTH_TYPES" ]]; then
        echo "   • Auth: $AUTH_TYPES"
    fi
    if [[ -n "$CACHE_TYPE" ]]; then
        echo "   • Cache: $CACHE_TYPE"
    fi
    if [[ -n "$DATABASE_TYPE" ]]; then
        echo "   • Database: $DATABASE_TYPE"
    fi
    if [[ -n "$METRICS_TYPE" ]]; then
        echo "   • Metrics: $METRICS_TYPE"
    fi
    if [[ -n "$ACTIONS" ]]; then
        echo "   • Actions: $ACTIONS"
    fi
    if [[ -n "$MIDDLEWARES" ]]; then
        echo "   • Middlewares: $MIDDLEWARES"
    fi
    echo "   • Tests: $INCLUDE_TESTS"
    echo "   • CI/CD: $CICD_TYPE"
    echo ""
}

create_project_structure() {
    local project_dir=$(echo "$PROJECT_NAME" | tr '[:upper:]' '[:lower:]' | sed 's/ /-/g')

    print_info "Creating project structure in: $project_dir"

    mkdir -p "$project_dir"/{cmd,internal,scripts,types}
    
    if [[ "$FEATURES" == *"database"* ]]; then
        mkdir -p "$project_dir"/data/db
    fi

    cd "$project_dir"
}

create_go_mod() {
    print_info "Creating go.mod..."

    cat > go.mod << EOF
module ${PKG_NAME}

go 1.21

require (
    github.com/saiset-co/sai-service v1.0.0
)

replace github.com/saiset-co/sai-service => ../sai-service
EOF
}

create_main_go() {
    print_info "Creating cmd/main.go..."

    cat > cmd/main.go << EOF
package main

import (
	"context"
	"log"

	"github.com/saiset-co/sai-service/sai"
	"github.com/saiset-co/sai-service/service"
	"${PKG_NAME}/internal"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := service.NewService(ctx, "./config.yml")
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	businessService := internal.NewBusinessService(ctx)
	if err := businessService.Initialize(); err != nil {
		log.Fatalf("Failed to initialize business service: %v", err)
	}

	if err = srv.Start(); err != nil {
		sai.Logger().Error("Failed to start service")
		cancel()
		return
	}

	cancel()
}
EOF
}

create_handlers() {
    print_info "Creating internal/handlers.go..."

    if [[ "$TEMPLATE" == "api" ]]; then
        create_api_handlers
    else
        create_basic_handlers
    fi
}

create_basic_handlers() {
    local handlers_content='package internal

import ('

    if [[ "$FEATURES" == *"cache"* ]]; then
        handlers_content+='
	"time"'
    fi

    handlers_content+='

	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/sai"
	"github.com/saiset-co/sai-service/types"
)

func RegisterRoutes() {
	api := sai.Router().Group("/api/v1")'

    if [[ "$FEATURES" == *"cache"* ]]; then
        handlers_content+='.
		WithCache("api_cache", time.Hour)'
    fi

    handlers_content+='

	api.GET("/hello", handleHello)
}

func handleHello(ctx *types.RequestCtx) {
	response := map[string]interface{}{
		"message": "Hello from '"${PROJECT_NAME}"'!",
		"status":  "ok",
	}

	_, err := ctx.SuccessJSON(response)
	if err != nil {
		sai.Logger().Error("Failed to write response", zap.Error(err))
	}
}'

    echo "$handlers_content" > internal/handlers.go
}

create_api_handlers() {
    local handlers_content='package internal

import (
	"fmt"'

    if [[ "$FEATURES" == *"cache"* ]]; then
        handlers_content+='
	"time"'
    fi

    handlers_content+='

	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/sai"
	"github.com/saiset-co/sai-service/types"
	projectTypes "'${PKG_NAME}'/types"
)

func RegisterRoutes() {
	api := sai.Router().Group("/api/v1")'

    if [[ "$FEATURES" == *"cache"* ]]; then
        handlers_content+='.
		WithCache("api_cache", time.Hour)'
    fi

    handlers_content+='

	crud := api.Group("/documents")

'

    if [[ "$FEATURES" == *"docs"* ]]; then
        handlers_content+='
	crud.POST("/", handleCreate).
		WithDoc("Create", "Create multiple documents in a collection", "documents", &projectTypes.CreateRequest{}, &projectTypes.CreateResponse{})
	crud.GET("/", handleRead).
		WithDoc("Read", "Get documents with filtering and pagination. Add ?count=1 to include total count", "documents", &projectTypes.ReadRequest{}, &projectTypes.ReadResponse{})
	crud.PUT("/", handleUpdate).
		WithDoc("Update", "Update multiple documents by filter", "documents", &projectTypes.UpdateRequest{}, &projectTypes.UpdateResponse{})
	crud.DELETE("/", handleDelete).
		WithDoc("Delete", "Delete multiple documents by filter", "documents", &projectTypes.DeleteRequest{}, &projectTypes.DeleteResponse{})'
    else
        handlers_content+='
	crud.POST("/", handleCreate)
	crud.GET("/", handleRead)
	crud.PUT("/", handleUpdate)
	crud.DELETE("/", handleDelete)'
    fi

    handlers_content+='

	api.GET("/hello", handleHello)
}

func handleCreate(ctx *types.RequestCtx) {
	var req projectTypes.CreateRequest
	if err := ctx.Read(&req); err != nil {
		ctx.Error(err, 400)
		return
	}

	response := &projectTypes.CreateResponse{
		Success: true,
		Created: len(req.Documents),
		IDs:     generateIDs(len(req.Documents)),
	}

	_, err := ctx.SuccessJSON(response)
	if err != nil {
		sai.Logger().Error("Failed to write response", zap.Error(err))
	}
}

func handleRead(ctx *types.RequestCtx) {
	var req projectTypes.ReadRequest

	documents := []map[string]interface{}{
		{"id": "1", "name": "Document 1", "content": "Sample content 1"},
		{"id": "2", "name": "Document 2", "content": "Sample content 2"},
	}

	response := &projectTypes.ReadResponse{
		Success:   true,
		Documents: documents,
		Total:     len(documents),
		Page:      req.Page,
		Limit:     req.Limit,
	}

	_, err := ctx.SuccessJSON(response)
	if err != nil {
		sai.Logger().Error("Failed to write response", zap.Error(err))
	}
}

func handleUpdate(ctx *types.RequestCtx) {
	var req projectTypes.UpdateRequest
	if err := ctx.Read(&req); err != nil {
		ctx.Error(err, 400)
		return
	}

	response := &projectTypes.UpdateResponse{
		Success: true,
		Updated: 1,
	}

	_, err := ctx.SuccessJSON(response)
	if err != nil {
		sai.Logger().Error("Failed to write response", zap.Error(err))
	}
}

func handleDelete(ctx *types.RequestCtx) {
	var req projectTypes.DeleteRequest
	if err := ctx.Read(&req); err != nil {
		ctx.Error(err, 400)
		return
	}

	response := &projectTypes.DeleteResponse{
		Success: true,
		Deleted: 1,
	}

	_, err := ctx.SuccessJSON(response)
	if err != nil {
		sai.Logger().Error("Failed to write response", zap.Error(err))
	}
}

func handleHello(ctx *types.RequestCtx) {
	response := map[string]interface{}{
		"message": "Hello from '"${PROJECT_NAME}"'!",
		"status":  "ok",
	}

	_, err := ctx.SuccessJSON(response)
	if err != nil {
		sai.Logger().Error("Failed to write response", zap.Error(err))
	}
}

func generateIDs(count int) []string {
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = fmt.Sprintf("doc_%d_%d", time.Now().Unix(), i)
	}
	return ids
}'

    echo "$handlers_content" > internal/handlers.go
}

create_service() {
    print_info "Creating internal/service.go..."

    local service_imports='import (
	"context"'
	
    if [[ "$FEATURES" == *"client"* || "$FEATURES" == *"cron"* ]]; then
        service_imports+='
	"time"'
    fi
    
    service_imports+='

	"github.com/saiset-co/sai-service/sai"'
    
    if [[ "$FEATURES" == *"client"* ]]; then
        service_imports+='
	"github.com/saiset-co/sai-service/types"'
    fi
    
    if [[ "$FEATURES" == *"client"* ]]; then
        service_imports+='
	"go.uber.org/zap"'
    fi
    
    service_imports+='
)'

    cat > internal/service.go << EOF
package internal

${service_imports}

type BusinessService struct {
	ctx context.Context
}

func NewBusinessService(ctx context.Context) *BusinessService {
	return &BusinessService{
		ctx: ctx,
	}
}

func (s *BusinessService) Initialize() error {
	RegisterRoutes()

	sai.Logger().Info("Business service initialized")

	return nil
}
EOF
}

create_types() {
    print_info "Creating types/types.go..."

    if [[ "$TEMPLATE" == "api" ]]; then
        create_api_types
    fi
}

create_api_types() {
    cat > types/types.go << 'EOF'
package types

type CreateRequest struct {
	Documents []map[string]interface{} `json:"documents" validate:"required,min=1"`
}

type CreateResponse struct {
	Success bool     `json:"success"`
	Created int      `json:"created"`
	IDs     []string `json:"ids"`
}

type ReadRequest struct {
	Filter map[string]interface{} `json:"filter,omitempty"`
	Page   int                    `json:"page,omitempty"`
	Limit  int                    `json:"limit,omitempty"`
	Count  bool                   `json:"count,omitempty"`
}

type ReadResponse struct {
	Success   bool                     `json:"success"`
	Documents []map[string]interface{} `json:"documents"`
	Total     int                      `json:"total,omitempty"`
	Page      int                      `json:"page,omitempty"`
	Limit     int                      `json:"limit,omitempty"`
}

type UpdateRequest struct {
	Filter map[string]interface{} `json:"filter" validate:"required"`
	Update map[string]interface{} `json:"update" validate:"required"`
}

type UpdateResponse struct {
	Success bool `json:"success"`
	Updated int  `json:"updated"`
}

type DeleteRequest struct {
	Filter map[string]interface{} `json:"filter" validate:"required"`
}

type DeleteResponse struct {
	Success bool `json:"success"`
	Deleted int  `json:"deleted"`
}
EOF
}

create_config() {
    print_info "Creating config.template.yml..."

    local config_content='name: "${SERVICE_NAME}"
version: "${SERVICE_VERSION}"

server:
  http:
    host: "${SERVER_HOST}"
    port: ${SERVER_PORT}
    read_timeout: ${SERVER_READ_TIMEOUT}
    write_timeout: ${SERVER_WRITE_TIMEOUT}
    idle_timeout: ${SERVER_IDLE_TIMEOUT}'

    if [[ "$FEATURES" == *"tls"* ]]; then
        config_content+='
  tls:
    enabled: ${TLS_ENABLED}
    auto_cert: ${TLS_AUTO_CERT}
    domains: [${TLS_DOMAINS}]
    email: "${TLS_EMAIL}"
    cache_dir: "${TLS_CACHE_DIR}"'
    fi

    config_content+='

logger:
  level: "${LOG_LEVEL}"
  format: "${LOG_FORMAT}"
  output: "${LOG_OUTPUT}"'

    if [[ -n "$AUTH_TYPES" ]]; then
        config_content+='

auth_providers:'
        if [[ "$AUTH_TYPES" == *"token"* ]]; then
            config_content+='
  token:
    params:
      token: "${AUTH_TOKEN}"'
        fi
        if [[ "$AUTH_TYPES" == *"basic"* ]]; then
            config_content+='
  basic:
    params:
      username: "${AUTH_USERNAME}"
      password: "${AUTH_PASSWORD}"'
        fi
    fi

    if [[ "$FEATURES" == *"cache"* ]]; then
        config_content+='

cache:
  enabled: ${CACHE_ENABLED}
  type: "${CACHE_TYPE}"
  default_ttl: "${CACHE_DEFAULT_TTL}"'

        if [[ "$CACHE_TYPE" == "redis" ]]; then
            config_content+='
  config:
    host: "${REDIS_HOST}"
    port: "${REDIS_PORT}"
    password: "${REDIS_PASSWORD}"
    db: ${REDIS_DB}'
        fi
    fi

    if [[ "$FEATURES" == *"database"* ]]; then
        config_content+='

database:
  enabled: ${DATABASE_ENABLED}
  type: "${DATABASE_TYPE}"
  path: "${DATABASE_PATH}"
  name: "${DATABASE_NAME}"'
    fi

    if [[ "$FEATURES" == *"metrics"* ]]; then
        config_content+='

metrics:
  enabled: ${METRICS_ENABLED}
  type: "${METRICS_TYPE}"
  http:
    enabled: ${METRICS_HTTP_ENABLED}
    path: "${METRICS_HTTP_PATH}"
    port: ${METRICS_HTTP_PORT}'

        if [[ "$METRICS_TYPE" == "prometheus" ]]; then
            config_content+='
  collectors:
    system: ${METRICS_COLLECTORS_SYSTEM}
    runtime: ${METRICS_COLLECTORS_RUNTIME}
    http: ${METRICS_COLLECTORS_HTTP}'
        fi
    fi

    if [[ "$FEATURES" == *"actions"* ]]; then
        config_content+='

actions:
  enabled: ${ACTIONS_ENABLED}'

        if [[ "$ACTIONS" == *"websocket"* ]]; then
            config_content+='
  broker:
    enabled: ${ACTIONS_BROKER_ENABLED}
    type: "${ACTIONS_BROKER_TYPE}"
    config:
      path: "${ACTIONS_BROKER_PATH}"'
        fi

        if [[ "$ACTIONS" == *"webhook"* ]]; then
            config_content+='
  webhooks:
    enabled: ${ACTIONS_WEBHOOKS_ENABLED}'
        fi
    fi

    if [[ "$FEATURES" == *"cron"* ]]; then
        config_content+='

cron:
  enabled: ${CRON_ENABLED}
  timezone: "${CRON_TIMEZONE}"'
    fi

    if [[ "$FEATURES" == *"middlewares"* && -n "$MIDDLEWARES" ]]; then
        config_content+='

middlewares:
  enabled: ${MIDDLEWARES_ENABLED}'

        IFS=',' read -ra MW_ARRAY <<< "$MIDDLEWARES"
        for mw in "${MW_ARRAY[@]}"; do
            mw=$(echo "$mw" | xargs)
            case $mw in
                recovery)
                    config_content+='
  recovery:
    enabled: ${MIDDLEWARE_RECOVERY_ENABLED}
    weight: ${MIDDLEWARE_RECOVERY_WEIGHT}
    params:
      stack_trace: ${MIDDLEWARE_RECOVERY_STACK_TRACE}'
                    ;;
                logging)
                    config_content+='
  logging:
    enabled: ${MIDDLEWARE_LOGGING_ENABLED}
    weight: ${MIDDLEWARE_LOGGING_WEIGHT}
    params:
      log_level: "${MIDDLEWARE_LOGGING_LEVEL}"
      log_headers: ${MIDDLEWARE_LOGGING_HEADERS}
      log_body: ${MIDDLEWARE_LOGGING_BODY}'
                    ;;
                ratelimit)
                    config_content+='
  rate_limit:
    enabled: ${MIDDLEWARE_RATELIMIT_ENABLED}
    weight: ${MIDDLEWARE_RATELIMIT_WEIGHT}
    params:
      requests_per_minute: ${MIDDLEWARE_RATELIMIT_RPM}'
                    ;;
                bodylimit|bodyimit)
                    config_content+='
  body_limit:
    enabled: ${MIDDLEWARE_BODYLIMIT_ENABLED}
    weight: ${MIDDLEWARE_BODYLIMIT_WEIGHT}
    params:
      max_body_size: ${MIDDLEWARE_BODYLIMIT_SIZE}'
                    ;;
                cors)
                    config_content+='
  cors:
    enabled: ${MIDDLEWARE_CORS_ENABLED}
    weight: ${MIDDLEWARE_CORS_WEIGHT}
    params:
      allowed_origins: ["${MIDDLEWARE_CORS_ORIGINS}"]
      allowed_methods: ["${MIDDLEWARE_CORS_METHODS}"]
      allowed_headers: ["${MIDDLEWARE_CORS_HEADERS}"]
      max_age: ${MIDDLEWARE_CORS_MAX_AGE}'
                    ;;
                auth)
                    config_content+='
  auth:
    enabled: ${MIDDLEWARE_AUTH_ENABLED}
    weight: ${MIDDLEWARE_AUTH_WEIGHT}
    params:
      token: "${AUTH_TOKEN}"'
                    ;;
                compression|compresion)
                    config_content+='
  compression:
    enabled: ${MIDDLEWARE_COMPRESSION_ENABLED}
    weight: ${MIDDLEWARE_COMPRESSION_WEIGHT}
    params:
      algorithm: "${MIDDLEWARE_COMPRESSION_ALGORITHM}"
      level: ${MIDDLEWARE_COMPRESSION_LEVEL}
      threshold: ${MIDDLEWARE_COMPRESSION_THRESHOLD}'
                    ;;
                cache)
                    config_content+='
  cache:
    enabled: ${MIDDLEWARE_CACHE_ENABLED}
    weight: ${MIDDLEWARE_CACHE_WEIGHT}
    params:
      default_ttl: "${MIDDLEWARE_CACHE_TTL}"'
                    ;;
            esac
        done
    fi

    if [[ "$FEATURES" == *"docs"* ]]; then
        config_content+='

docs:
  enabled: ${DOCS_ENABLED}
  path: "${DOCS_PATH}"'
    fi

    if [[ "$FEATURES" == *"health"* ]]; then
        config_content+='

health:
  enabled: ${HEALTH_ENABLED}'
    fi

    if [[ "$FEATURES" == *"client"* ]]; then
        config_content+='

clients:
  enabled: ${CLIENTS_ENABLED}
  default_timeout: "${CLIENTS_DEFAULT_TIMEOUT}"
  max_idle_connections: ${CLIENTS_MAX_IDLE_CONNECTIONS}
  idle_conn_timeout: "${CLIENTS_IDLE_CONN_TIMEOUT}"
  default_retries: ${CLIENTS_DEFAULT_RETRIES}
  circuit_breaker:
    enabled: ${CLIENTS_CIRCUIT_BREAKER_ENABLED}
    failure_threshold: ${CLIENTS_CIRCUIT_BREAKER_FAILURE_THRESHOLD}
    recovery_timeout: "${CLIENTS_CIRCUIT_BREAKER_RECOVERY_TIMEOUT}"
    half_open_requests: ${CLIENTS_CIRCUIT_BREAKER_HALF_OPEN_REQUESTS}
  services: {}'
    fi

    echo "$config_content" > config.template.yml
}

create_env_example() {
    print_info "Creating .env.example..."

    local env_content="DOCKERFILE=Dockerfile
SERVICE_NAME=\"${PROJECT_NAME}\"
SERVICE_VERSION=1.0.0

SERVER_HOST=127.0.0.1
SERVER_PORT=8080
SERVER_EXTERNAL_PORT=8080
SERVER_READ_TIMEOUT=30
SERVER_WRITE_TIMEOUT=30
SERVER_IDLE_TIMEOUT=120

LOG_LEVEL=debug
LOG_OUTPUT=stdout
LOG_FORMAT=console"

    if [[ "$FEATURES" == *"tls"* ]]; then
        env_content+="

TLS_ENABLED=false
TLS_AUTO_CERT=false
TLS_EMAIL=
TLS_CACHE_DIR=./certs
TLS_DOMAINS=\"example.com\",\"www.example.com\""
    fi

    if [[ -n "$AUTH_TYPES" ]]; then

        if [[ "$AUTH_TYPES" == *"token"* ]]; then
            env_content+="
AUTH_TOKEN=your-secret-token-here"
        fi
        if [[ "$AUTH_TYPES" == *"basic"* ]]; then
            env_content+="
AUTH_USERNAME=admin
AUTH_PASSWORD=secure-password"
        fi
    fi

    if [[ "$FEATURES" == *"cache"* ]]; then
        env_content+="

CACHE_ENABLED=true
CACHE_TYPE=$CACHE_TYPE
CACHE_DEFAULT_TTL=1h"

        if [[ "$CACHE_TYPE" == "redis" ]]; then
            env_content+="
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0"
        fi
    fi

    if [[ "$FEATURES" == *"database"* ]]; then
        env_content+="

DATABASE_ENABLED=true
DATABASE_TYPE=$DATABASE_TYPE
DATABASE_PATH=./data/db
DATABASE_NAME=\${SERVICE_NAME}"
    fi

    if [[ "$FEATURES" == *"metrics"* ]]; then
        env_content+="

METRICS_ENABLED=true
METRICS_TYPE=$METRICS_TYPE
METRICS_HTTP_ENABLED=true
METRICS_HTTP_PATH=/metrics
METRICS_HTTP_PORT=9090"

        if [[ "$METRICS_TYPE" == "prometheus" ]]; then
            env_content+="
METRICS_COLLECTORS_SYSTEM=true
METRICS_COLLECTORS_RUNTIME=true
METRICS_COLLECTORS_HTTP=true"
        fi
    fi

    if [[ "$FEATURES" == *"actions"* ]]; then
        env_content+="

ACTIONS_ENABLED=true"

        if [[ "$ACTIONS" == *"websocket"* ]]; then
            env_content+="
ACTIONS_BROKER_ENABLED=true
ACTIONS_BROKER_TYPE=websocket
ACTIONS_BROKER_PATH=/ws"
        fi

        if [[ "$ACTIONS" == *"webhook"* ]]; then
            env_content+="
ACTIONS_WEBHOOKS_ENABLED=true"
        fi
    fi

    if [[ "$FEATURES" == *"cron"* ]]; then
        env_content+="

CRON_ENABLED=true
CRON_TIMEZONE=UTC"
    fi

    if [[ "$FEATURES" == *"middlewares"* && -n "$MIDDLEWARES" ]]; then
        env_content+="

MIDDLEWARES_ENABLED=true"

        IFS=',' read -ra MW_ARRAY <<< "$MIDDLEWARES"
        for mw in "${MW_ARRAY[@]}"; do
            mw=$(echo "$mw" | xargs)
            case $mw in
                recovery)
                    env_content+="
MIDDLEWARE_RECOVERY_ENABLED=true
MIDDLEWARE_RECOVERY_WEIGHT=10
MIDDLEWARE_RECOVERY_STACK_TRACE=true"
                    ;;
                logging)
                    env_content+="
MIDDLEWARE_LOGGING_ENABLED=true
MIDDLEWARE_LOGGING_WEIGHT=20
MIDDLEWARE_LOGGING_LEVEL=info
MIDDLEWARE_LOGGING_HEADERS=false
MIDDLEWARE_LOGGING_BODY=false"
                    ;;
                ratelimit)
                    env_content+="
MIDDLEWARE_RATELIMIT_ENABLED=true
MIDDLEWARE_RATELIMIT_WEIGHT=30
MIDDLEWARE_RATELIMIT_RPM=100"
                    ;;
                bodylimit|bodyimit)
                    env_content+="
MIDDLEWARE_BODYLIMIT_ENABLED=true
MIDDLEWARE_BODYLIMIT_WEIGHT=40
MIDDLEWARE_BODYLIMIT_SIZE=10485760"
                    ;;
                cors)
                    env_content+="
MIDDLEWARE_CORS_ENABLED=true
MIDDLEWARE_CORS_WEIGHT=50
MIDDLEWARE_CORS_ORIGINS=*
MIDDLEWARE_CORS_METHODS=GET,POST,PUT,DELETE,OPTIONS
MIDDLEWARE_CORS_HEADERS=Content-Type,Authorization,X-API-Key
MIDDLEWARE_CORS_MAX_AGE=86400"
                    ;;
                auth)
                    env_content+="
MIDDLEWARE_AUTH_ENABLED=true
MIDDLEWARE_AUTH_WEIGHT=60"
                    ;;
                compression|compresion)
                    env_content+="
MIDDLEWARE_COMPRESSION_ENABLED=true
MIDDLEWARE_COMPRESSION_WEIGHT=70
MIDDLEWARE_COMPRESSION_ALGORITHM=gzip
MIDDLEWARE_COMPRESSION_LEVEL=6
MIDDLEWARE_COMPRESSION_THRESHOLD=1024"
                    ;;
                cache)
                    env_content+="
MIDDLEWARE_CACHE_ENABLED=true
MIDDLEWARE_CACHE_WEIGHT=80
MIDDLEWARE_CACHE_TTL=5m"
                    ;;
            esac
        done
    fi

    if [[ "$FEATURES" == *"docs"* ]]; then
        env_content+="

DOCS_ENABLED=true
DOCS_PATH=/docs"
    fi

    if [[ "$FEATURES" == *"health"* ]]; then
        env_content+="

HEALTH_ENABLED=true"
    fi

    if [[ "$FEATURES" == *"client"* ]]; then
        env_content+="

CLIENTS_ENABLED=true
CLIENTS_DEFAULT_TIMEOUT=30s
CLIENTS_MAX_IDLE_CONNECTIONS=100
CLIENTS_IDLE_CONN_TIMEOUT=90s
CLIENTS_DEFAULT_RETRIES=3
CLIENTS_CIRCUIT_BREAKER_ENABLED=true
CLIENTS_CIRCUIT_BREAKER_FAILURE_THRESHOLD=5
CLIENTS_CIRCUIT_BREAKER_RECOVERY_TIMEOUT=60s
CLIENTS_CIRCUIT_BREAKER_HALF_OPEN_REQUESTS=3"
    fi

    printf "%s\n" "$env_content" > .env.example
}

create_dockerfile() {
    print_info "Creating Dockerfile..."

    local project_name_lower=$(echo "$PROJECT_NAME" | tr '[:upper:]' '[:lower:]' | sed 's/ /-/g')

    cat > Dockerfile << EOF
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata gcc musl-dev

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \\
    -ldflags='-w -s -extldflags "-static"' \\
    -a -installsuffix cgo \\
    -o ${project_name_lower} \\
    ./cmd/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata gettext

RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /build/${project_name_lower} .
COPY --from=builder /build/scripts/docker-entrypoint.sh /usr/local/bin/

RUN chmod +x /usr/local/bin/docker-entrypoint.sh
RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["./${project_name_lower}"]
EOF
}

create_docker_compose() {
    print_info "Creating docker-compose.yml..."

    local project_name_lower=$(echo "$PROJECT_NAME" | tr '[:upper:]' '[:lower:]' | sed 's/ /-/g')

    local compose_content="services:
  ${project_name_lower}:
    build:
      context: ..
      dockerfile: ./${project_name_lower}/\${DOCKERFILE}
    container_name: ${project_name_lower}
    ports:
      - \${SERVER_EXTERNAL_PORT}:\${SERVER_PORT}
    env_file:
      - .env
    networks:
      - sai-network
    restart: unless-stopped
    volumes:
      - ./config.template.yml:/app/config.template.yml"

    local dependencies=()
    if [[ "$CACHE_TYPE" == "redis" ]]; then
        dependencies+=("redis")
    fi
    if [[ "$METRICS_TYPE" == "prometheus" ]]; then
        dependencies+=("prometheus")
    fi

    if [[ ${#dependencies[@]} -gt 0 ]]; then
        compose_content+="
    depends_on:"
        for dep in "${dependencies[@]}"; do
            compose_content+="
      - $dep"
        done
    fi

    if [[ "$CACHE_TYPE" == "redis" ]]; then
        compose_content+="

  redis:
    image: redis:7-alpine
    container_name: ${project_name_lower}-redis
    ports:
      - \"6379:6379\"
    networks:
      - sai-network
    restart: unless-stopped"
    fi

    if [[ "$METRICS_TYPE" == "prometheus" ]]; then
        compose_content+="

  prometheus:
    image: prom/prometheus:latest
    container_name: ${project_name_lower}-prometheus
    ports:
      - \"9091:9090\"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    networks:
      - sai-network
    restart: unless-stopped"
    fi

    compose_content+="

networks:
  sai-network:
    name: sai-network
    driver: bridge"

    echo "$compose_content" > docker-compose.yml
}

create_makefile() {
    print_info "Creating Makefile..."

    local project_name_lower=$(echo "$PROJECT_NAME" | tr '[:upper:]' '[:lower:]' | sed 's/ /-/g')

    cat > Makefile << EOF
.PHONY: help config build run test clean fmt lint security docker-build docker-run docker-stop docker-logs docker-log

.DEFAULT_GOAL := help

BINARY_NAME=main
BUILD_DIR=./bin

help: ## Show this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*\$\$' \$(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\\033[36m%-20s\\033[0m %s\\n", \$\$1, \$\$2}'

config: ## Build config.yml from template and .env
	@echo "Building configuration..."
	@if [ ! -f .env ]; then cp .env.example .env; fi
	@set -a && . ./.env && set +a && envsubst < config.template.yml > config.yml
	@echo "Configuration built successfully"

build: config ## Build the application
	@echo "Building application..."
	@if [ ! -f .env ]; then cp .env.example .env; fi
	@go mod tidy
	@mkdir -p \$(BUILD_DIR)
	@go build -o \$(BUILD_DIR)/\$(BINARY_NAME) ./cmd
	@echo "Build completed successfully"

run: build ## Build and run the application
	@echo "Starting application..."
	@\$(BUILD_DIR)/\$(BINARY_NAME)

test: build ## Run tests
	@echo "Running tests..."
	@go test -v ./...

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf \$(BUILD_DIR)
	@rm -f config.yml
	@echo "Clean completed"

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Formatting completed"

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run
	@echo "Linting completed"

security: ## Run security checks
	@echo "Running security checks..."
	@gosec ./...
	@echo "Security check completed"

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t \$(shell basename \$(CURDIR)):latest .
	@echo "Docker image built successfully"

docker-run: ## Run with Docker Compose
	@echo "Starting services with Docker Compose..."
	@docker-compose up -d
	@echo "Services started"

docker-stop: ## Stop Docker Compose
	@echo "Stopping services..."
	@docker-compose down
	@echo "Services stopped"

docker-logs: ## Show Docker Compose logs
	@docker-compose logs -f

docker-log: ## Show main service logs
	@docker-compose logs -f app
EOF
}

create_docker_entrypoint() {
    print_info "Creating scripts/docker-entrypoint.sh..."

    cat > scripts/docker-entrypoint.sh << 'EOF'
#!/bin/sh

set -e

echo "Building configuration from template..."
envsubst < "./config.template.yml" > "./config.yml"

echo "Configuration built successfully"
echo "Starting application..."

exec "$@"
EOF

    chmod +x scripts/docker-entrypoint.sh
}

create_tests() {
    if [[ "$INCLUDE_TESTS" == "true" ]]; then
        print_info "Creating integration tests..."

        mkdir -p tests

        cat > tests/integration_test.go << 'EOF'
package tests

import (
  "context"
  "testing"
  "time"

  "github.com/stretchr/testify/assert"
  "github.com/stretchr/testify/require"
  "github.com/valyala/fasthttp"
)

func TestHealthCheck(t *testing.T) {

  req := fasthttp.AcquireRequest()
  resp := fasthttp.AcquireResponse()
  defer fasthttp.ReleaseRequest(req)
  defer fasthttp.ReleaseResponse(resp)

  req.SetRequestURI("http://localhost:8080/api/v1/hello")
  req.Header.SetMethod("GET")

  client := &fasthttp.Client{
    ReadTimeout:  time.Second * 10,
    WriteTimeout: time.Second * 10,
  }

  ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
  defer cancel()

  err := client.DoTimeout(req, resp, 10*time.Second)
  if err != nil {
    t.Skip("Service not running, skipping integration test")
    return
  }

  assert.Equal(t, fasthttp.StatusOK, resp.StatusCode())
}

func TestServiceAPI(t *testing.T) {
  req := fasthttp.AcquireRequest()
  resp := fasthttp.AcquireResponse()
  defer fasthttp.ReleaseRequest(req)
  defer fasthttp.ReleaseResponse(resp)

  req.SetRequestURI("http://localhost:8080/api/v1/hello")
  req.Header.SetMethod("GET")

  client := &fasthttp.Client{
    ReadTimeout:  time.Second * 10,
    WriteTimeout: time.Second * 10,
  }

  err := client.DoTimeout(req, resp, 10*time.Second)
  if err != nil {
    t.Skip("Service not running, skipping integration test")
    return
  }

  assert.Equal(t, fasthttp.StatusOK, resp.StatusCode())

  contentType := string(resp.Header.Peek("Content-Type"))
  assert.Contains(t, contentType, "application/json")
}
EOF

        cat >> go.mod << 'EOF'

require (
  github.com/stretchr/testify v1.8.4
  github.com/valyala/fasthttp v1.51.0
)
EOF
    fi
}

create_cicd() {
    case $CICD_TYPE in
        github)
            print_info "Creating GitHub Actions workflow..."
            mkdir -p .github/workflows

            cat > .github/workflows/ci.yml << 'EOF'
name: CI/CD Pipeline

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
    - uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'

    - name: Cache Go modules
      uses: actions/cache@v3
      with:
        path: ~/go/pkg/mod
        key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
        restore-keys: |
          ${{ runner.os }}-go-

    - name: Install dependencies
      run: go mod download

    - name: Run tests
      run: make test
      env:
        REDIS_HOST: localhost
        REDIS_PORT: 6379

    - name: Run linter
      uses: golangci/golangci-lint-action@v3
      with:
        version: v1.54

    - name: Run security scan
      uses: securecodewarrior/github-action-gosec@v1
      with:
        args: './...'

  build:
    needs: test
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'

    - name: Build binary
      run: make build

    - name: Build Docker image
      run: make docker-build

    - name: Upload artifacts
      uses: actions/upload-artifact@v3
      with:
        name: binary
        path: bin/
EOF
            ;;

        gitlab)
            print_info "Creating GitLab CI pipeline..."

            cat > .gitlab-ci.yml << 'EOF'
stages:
  - test
  - build
  - deploy

variables:
  GO_VERSION: "1.21"
  DOCKER_DRIVER: overlay2

before_script:
  - apk add --no-cache git make gettext
  - go version

test:
  stage: test
  image: golang:${GO_VERSION}-alpine

  services:
    - redis:7-alpine

  variables:
    REDIS_HOST: redis
    REDIS_PORT: 6379

  script:
    - go mod download
    - make test

  coverage: '/coverage: \d+\.\d+% of statements/'

  artifacts:
    reports:
      junit: report.xml
      coverage_report:
        coverage_format: cobertura
        path: coverage.xml

build:
  stage: build
  image: golang:${GO_VERSION}-alpine

  script:
    - make build
    - make docker-build

  artifacts:
    paths:
      - bin/
    expire_in: 1 week

  only:
    - main
    - develop

deploy:
  stage: deploy
  image: docker:latest

  services:
    - docker:dind

  script:
    - echo "Deploy stage - implement your deployment logic here"

  only:
    - main
EOF
            ;;
    esac
}

create_feature_files() {
    if [[ "$METRICS_TYPE" == "prometheus" ]]; then
        print_info "Creating prometheus.yml..."

        cat > prometheus.yml << 'EOF'
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'app'
    static_configs:
      - targets: ['app:9090']
    scrape_interval: 5s
    metrics_path: '/metrics'
EOF
    fi

    if [[ "$FEATURES" == *"cron"* ]]; then
        print_info "Adding cron job example to service.go..."

        cat >> internal/service.go << 'EOF'

func (s *BusinessService) InitializeCronJobs() error {
  err := sai.Cron().Add("example_job", "0 * * * * *", func() {
    sai.Logger().Info("Example cron job executed")
  })

  if err != nil {
    return err
  }

  sai.Logger().Info("Cron jobs initialized")
  return nil
}
EOF
    fi

    if [[ "$FEATURES" == *"client"* ]]; then
        print_info "Adding client example to service.go..."

        cat >> internal/service.go << 'EOF'

func (s *BusinessService) ExampleAPICall() error {
  response, statusCode, err := sai.ClientManager().Call(
    "example-service",
    "GET",
    "/api/status",
    nil,
    &types.CallOptions{
      Headers: map[string]string{
        "User-Agent": "SAI-Service-Client/1.0",
      },
      Timeout: 30 * time.Second,
      Retry:   3,
    },
  )

  if err != nil {
    return err
  }

  sai.Logger().Info("API call completed",
    zap.Int("status_code", statusCode),
    zap.ByteString("response", response))

  return nil
}
EOF
    fi
}

update_main_go() {
    if [[ "$FEATURES" == *"cron"* ]]; then
        print_info "Updating main.go with cron features..."

        sed -i '/businessService.Initialize()/a\\n\tif err := businessService.InitializeCronJobs(); err != nil {\n\t\tlog.Fatalf("Failed to initialize cron jobs: %v", err)\n\t}' cmd/main.go
    fi
}

generate_project() {
    print_header "=== Starting Project Generation ==="

    create_project_structure
    create_go_mod
    create_main_go
    create_handlers
    create_service
    create_types
    create_config
    create_env_example
    create_dockerfile
    create_docker_compose
    create_makefile
    create_docker_entrypoint
    create_tests
    create_cicd
    create_feature_files
    update_main_go

    print_success "Project generated successfully!"
    print_info "Next steps:"
    echo "  1. cd $(echo "$PROJECT_NAME" | tr '[:upper:]' '[:lower:]' | sed 's/ /-/g')"
    echo "  2. cp .env.example .env"
    echo "  3. Edit .env file with your settings"
    echo "  4. make run"
}

main() {
    if [[ $# -eq 0 ]]; then
        interactive_mode
    else
        parse_args "$@"

        if [[ -z "$PROJECT_NAME" ]]; then
            print_error "Project name is required. Use --name parameter or run without arguments for interactive mode."
            exit 1
        fi

        if [[ -z "$PKG_NAME" ]]; then
            PKG_NAME=$(echo "$PROJECT_NAME" | tr '[:upper:]' '[:lower:]' | sed 's/ /-/g')
        fi

        if [[ -z "$INCLUDE_TESTS" ]]; then
            INCLUDE_TESTS="false"
        fi

        if [[ -z "$CICD_TYPE" ]]; then
            CICD_TYPE="none"
        fi
    fi

    validate_configuration

    generate_project
}

main "$@"
