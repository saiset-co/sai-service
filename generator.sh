#!/bin/bash

# SAI Service Generator
# Генератор проектов на базе sai-service framework

set -euo pipefail

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Значения по умолчанию
DEFAULT_PORT=8080
DEFAULT_TEMPLATE="api"

# Переменные
PROJECT_NAME=""
PORT=$DEFAULT_PORT
TEMPLATE=$DEFAULT_TEMPLATE
FEATURES=""
INCLUDE_TESTS=false
MODULE_NAME=""
DEFAULT_MODULE=""
CI_TYPE="none"
INTERACTIVE=true

# Доступные опции
AVAILABLE_TEMPLATES=("basic" "api" "microservice" "full")
AVAILABLE_FEATURES=("cache" "metrics" "docs" "cron" "actions" "tls" "middleware" "health" "client")
AVAILABLE_CI=("none" "github" "gitlab")

# Функции для вывода
log_info() {
   echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
   echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
   echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
   echo -e "${RED}[ERROR]${NC} $1"
}

# Показать помощь
show_help() {
   cat << EOF
SAI Service Generator

Usage: $0 [OPTIONS]

OPTIONS:
   --name NAME          Project name (required)
   --module MODULE      Go module name (default: project-name)
   --port PORT          HTTP port (default: $DEFAULT_PORT)
   --template TEMPLATE  Project template (default: $DEFAULT_TEMPLATE)
                        Available: ${AVAILABLE_TEMPLATES[*]}
   --features LIST      Comma-separated list of features
                        Available: ${AVAILABLE_FEATURES[*]}
   --tests              Include integration tests
   --ci TYPE            CI/CD type (default: none)
                        Available: ${AVAILABLE_CI[*]}
   --non-interactive    Skip interactive prompts
   --help               Show this help

TEMPLATES:
   basic        - Minimal HTTP server
   api          - REST API with basic middleware
   microservice - Full microservice with cache, metrics, health
   full         - All features enabled

EXAMPLES:
   $0 --name "user-service" --template api --features "cache,metrics,docs"
   $0 --name "user-service" --module "github.com/company/user-service" --template api
   $0 --name "gateway" --module "github.com/myorg/gateway" --template full --tests --ci github
   $0 --name "simple-api" --template basic --port 8081

EOF
}

# Проверить зависимости
check_dependencies() {
   log_info "Checking dependencies..."

   if ! command -v go &> /dev/null; then
       log_error "Go is not installed. Please install Go 1.19 or later."
       exit 1
   fi

   GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
   if [ "$(echo "$GO_VERSION 1.19" | tr ' ' '\n' | sort -V | head -n1)" != "1.19" ]; then
       log_error "Go 1.19 or later is required. Current version: $GO_VERSION"
       exit 1
   fi

   log_success "Dependencies OK"
}

# Парсинг аргументов командной строки
parse_args() {
   while [[ $# -gt 0 ]]; do
       case $1 in
           --name)
               PROJECT_NAME="$2"
               shift 2
               ;;
           --module)
               MODULE_NAME="$2"
               shift 2
               ;;
           --port)
               PORT="$2"
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
           --tests)
               INCLUDE_TESTS=true
               shift
               ;;
           --ci)
               CI_TYPE="$2"
               shift 2
               ;;
           --non-interactive)
               INTERACTIVE=false
               shift
               ;;
           --help)
               show_help
               exit 0
               ;;
           *)
               log_error "Unknown option: $1"
               show_help
               exit 1
               ;;
       esac
   done
}

# Валидация параметров
validate_params() {
   # Проверка имени проекта
   if [[ -z "$PROJECT_NAME" ]]; then
       log_error "Project name is required"
       exit 1
   fi

   if [[ ! "$PROJECT_NAME" =~ ^[a-zA-Z][a-zA-Z0-9-]*$ ]]; then
       log_error "Project name must start with a letter and contain only letters, numbers, and hyphens"
       exit 1
   fi

   PROJECT_NAME=$(echo "$PROJECT_NAME" | tr '[:upper:]' '[:lower:]')

   if [[ -z "$MODULE_NAME" ]]; then
       MODULE_NAME="$PROJECT_NAME"
       DEFAULT_MODULE="$PROJECT_NAME"
   fi

   # Валидация имени модуля
   if [[ ! "$MODULE_NAME" =~ ^[a-zA-Z0-9._/-]+$ ]]; then
       log_error "Module name contains invalid characters. Use letters, numbers, dots, slashes, and hyphens"
       exit 1
   fi

   # Проверка порта
   if ! [[ "$PORT" =~ ^[0-9]+$ ]] || [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
       log_error "Port must be a number between 1 and 65535"
       exit 1
   fi

   # Проверка шаблона
   if [[ ! " ${AVAILABLE_TEMPLATES[*]} " =~ " ${TEMPLATE} " ]]; then
       log_error "Invalid template: $TEMPLATE. Available: ${AVAILABLE_TEMPLATES[*]}"
       exit 1
   fi

   # Проверка CI типа
   if [[ ! " ${AVAILABLE_CI[*]} " =~ " ${CI_TYPE} " ]]; then
       log_error "Invalid CI type: $CI_TYPE. Available: ${AVAILABLE_CI[*]}"
       exit 1
   fi

   # Проверка существования директории
   if [[ -d "$PROJECT_NAME" ]]; then
       log_error "Directory $PROJECT_NAME already exists"
       exit 1
   fi
}

# Интерактивный режим
interactive_mode() {
   if [[ "$INTERACTIVE" == "false" ]]; then
       return
   fi

   echo
   log_info "Welcome to SAI Service Generator!"
   echo

   # Имя проекта
   if [[ -z "$PROJECT_NAME" ]]; then
       while true; do
           read -p "Project name: " PROJECT_NAME
           if [[ -n "$PROJECT_NAME" ]]; then
               # Проверяем формат
               if [[ "$PROJECT_NAME" =~ ^[a-zA-Z][a-zA-Z0-9-]*$ ]]; then
                   # Конвертируем в lowercase
                   PROJECT_NAME=$(echo "$PROJECT_NAME" | tr '[:upper:]' '[:lower:]')
                   break
               else
                   log_error "Project name must start with a letter and contain only letters, numbers, and hyphens"
               fi
           else
               log_error "Project name is required"
           fi
       done
   fi

   if [[ -z "$MODULE_NAME" ]]; then
       DEFAULT_MODULE="$PROJECT_NAME"
       while true; do
           read -p "Go module name [$DEFAULT_MODULE]: " input_module
           input_module=${input_module:-$DEFAULT_MODULE}
           if [[ "$input_module" =~ ^[a-zA-Z0-9._/-]+$ ]]; then
               MODULE_NAME=$input_module
               break
           else
               log_error "Module name contains invalid characters. Use letters, numbers, dots, slashes, and hyphens"
               echo "Examples: github.com/company/project, gitlab.com/user/repo, project-name"
           fi
       done
   fi

   # Порт
   while true; do
       read -p "Port [$PORT]: " input_port
       input_port=${input_port:-$PORT}
       if [[ "$input_port" =~ ^[0-9]+$ ]] && [ "$input_port" -ge 1 ] && [ "$input_port" -le 65535 ]; then
           PORT=$input_port
           break
       else
           log_error "Port must be a number between 1 and 65535"
       fi
   done

   # Шаблон
   while true; do
       echo "Available templates: ${AVAILABLE_TEMPLATES[*]}"
       read -p "Select template [$TEMPLATE]: " input_template
       input_template=${input_template:-$TEMPLATE}
       if [[ " ${AVAILABLE_TEMPLATES[*]} " =~ " ${input_template} " ]]; then
           TEMPLATE=$input_template
           break
       else
           log_error "Invalid template. Available: ${AVAILABLE_TEMPLATES[*]}"
       fi
   done

   # Фичи (если не full шаблон)
   if [[ "$TEMPLATE" != "full" ]]; then
       echo "Available features: ${AVAILABLE_FEATURES[*]}"
       read -p "Enable features (comma-separated) [$FEATURES]: " input_features
       FEATURES=${input_features:-$FEATURES}
   fi

   # Тесты
   while true; do
       read -p "Include integration tests? [y/N]: " input_tests
       case "$input_tests" in
           [Yy]|[Yy][Ee][Ss])
               INCLUDE_TESTS=true
               break
               ;;
           [Nn]|[Nn][Oo]|"")
               INCLUDE_TESTS=false
               break
               ;;
           *)
               log_error "Please answer 'y' for yes or 'n' for no"
               ;;
       esac
   done

   # CI/CD
   while true; do
       echo "Available CI/CD: ${AVAILABLE_CI[*]}"
       read -p "Generate CI/CD files [$CI_TYPE]: " input_ci
       input_ci=${input_ci:-$CI_TYPE}

       # Обрабатываем сокращенные ответы
       case "$input_ci" in
           [Yy]|[Yy][Ee][Ss])
               CI_TYPE="github"  # По умолчанию GitHub если просто "y"
               break
               ;;
           [Nn]|[Nn][Oo])
               CI_TYPE="none"
               break
               ;;
           *)
               if [[ " ${AVAILABLE_CI[*]} " =~ " ${input_ci} " ]]; then
                   CI_TYPE=$input_ci
                   break
               else
                   log_error "Invalid CI type. Available: ${AVAILABLE_CI[*]} (or 'y' for github, 'n' for none)"
               fi
               ;;
       esac
   done

   echo
   log_info "Configuration:"
   echo "   • Project: $PROJECT_NAME"
   echo "   • Module: $MODULE_NAME"
   echo "   • Port: $PORT"
   echo "   • Template: $TEMPLATE"
   echo "   • Features: $FEATURES"
   echo "   • Tests: $([ "$INCLUDE_TESTS" == "true" ] && echo "Yes" || echo "No")"
   echo "   • CI/CD: $CI_TYPE"
   echo

   read -p "Proceed with generation? [Y/n]: " confirm
   case "$confirm" in
       [Nn]|[Nn][Oo])
           log_info "Generation cancelled"
           exit 0
           ;;
   esac
}

# Определение фич по шаблону
setup_template_features() {
   case "$TEMPLATE" in
       "basic")
           TEMPLATE_FEATURES=""
           ;;
       "api")
           TEMPLATE_FEATURES="middleware,health,docs"
           ;;
       "microservice")
           TEMPLATE_FEATURES="cache,metrics,middleware,health,docs,client"
           ;;
       "full")
           TEMPLATE_FEATURES="cache,metrics,docs,cron,actions,tls,middleware,health,client"
           ;;
   esac

   # Объединяем фичи из шаблона и пользовательские
   if [[ -n "$FEATURES" ]]; then
       ALL_FEATURES="$TEMPLATE_FEATURES,$FEATURES"
   else
       ALL_FEATURES="$TEMPLATE_FEATURES"
   fi

   # Удаляем дубликаты
   ALL_FEATURES=$(echo "$ALL_FEATURES" | tr ',' '\n' | sort -u | grep -v '^$' | tr '\n' ',' | sed 's/,$//')
}

# Проверка включенной фичи
has_feature() {
   [[ ",$ALL_FEATURES," =~ ",$1," ]]
}

# Создание структуры проекта
create_project_structure() {
   log_info "Creating project structure..."

   mkdir -p "$PROJECT_NAME"/{cmd,internal/{handlers,models},pkg}

   if [[ "$INCLUDE_TESTS" == "true" ]]; then
       mkdir -p "$PROJECT_NAME"/tests/{integration,helpers}
       mkdir -p "$PROJECT_NAME"/tests/integration/fixtures
   fi

   if [[ "$CI_TYPE" == "github" ]]; then
       mkdir -p "$PROJECT_NAME"/.github/workflows
   fi

   log_success "Project structure created"
}

# Генерация go.mod
generate_go_mod() {
   log_info "Generating go.mod..."

   cat > "$PROJECT_NAME/go.mod" << EOF
module $MODULE_NAME

go 1.21

require (
   github.com/saiset-co/sai-service v1.0.0
)
EOF
}

# Генерация main.go
generate_main() {
   log_info "Generating main.go..."

   cat > "$PROJECT_NAME/cmd/main.go" << EOF
package main

import (
   "context"
   "log"
   "os"
   "os/signal"
   "syscall"

   "$MODULE_NAME/internal"
)

func main() {
   ctx, cancel := context.WithCancel(context.Background())
   defer cancel()

   // Обработка сигналов
   sigChan := make(chan os.Signal, 1)
   signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

   go func() {
       <-sigChan
       log.Println("Shutting down...")
       cancel()
   }()

   // Запуск сервиса
   service, err := internal.NewService(ctx, "config.yml")
   if err != nil {
       log.Fatalf("Failed to create service: %v", err)
   }

   if err := service.Run(); err != nil {
       log.Fatalf("Service failed: %v", err)
   }

   log.Println("Service stopped gracefully")
}
EOF
}

# Генерация service.go
generate_service() {
   log_info "Generating service.go..."

   cat > "$PROJECT_NAME/internal/service.go" << EOF
package internal

import (
   "context"

   "github.com/saiset-co/sai-service/service"
   "github.com/saiset-co/sai-service/sai"
   "$MODULE_NAME/internal/handlers"
)

func NewService(ctx context.Context, configPath string) (*service.Service, error) {
   // Создаем основной сервис
   srv, err := service.NewService(ctx, configPath)
   if err != nil {
       return nil, err
   }

   // Регистрируем обработчики
   handlers := handlers.NewHandler()
   handlers.RegisterRoutes(sai.Router())

   return srv, nil
}
EOF
}

# Генерация handlers.go
generate_handlers() {
   log_info "Generating handlers.go..."

   # Определяем, какие методы нужно генерировать
   local cache_methods=""
   local doc_methods=""

   if has_feature "cache"; then
       cache_methods=".WithCache"
   fi

   if has_feature "docs"; then
       doc_methods=".WithDoc"
   fi

   cat > "$PROJECT_NAME/internal/handlers/handlers.go" << EOF
package handlers

import (
   "strconv"
   "time"

   "github.com/valyala/fasthttp"
   "github.com/saiset-co/sai-service/sai"
   "github.com/saiset-co/sai-service/types"
   "github.com/saiset-co/sai-service/utils"
   "$MODULE_NAME/internal/models"
)

type Handler struct {
   // Здесь можно добавить зависимости (БД, кеш, логгер и т.д.)
}

func NewHandler() *Handler {
   return &Handler{}
}

func (h *Handler) RegisterRoutes(router types.HTTPRouter) {
   api := router.Group("/api/v1")

   // GET /api/v1/items - получить все
EOF

   # Генерируем GET /items
   echo -n "    api.GET(\"/items\", h.GetItems)" >> "$PROJECT_NAME/internal/handlers/handlers.go"

   if has_feature "cache"; then
       echo -n ".
       WithCache(\"items_list\", 300, \"items\")" >> "$PROJECT_NAME/internal/handlers/handlers.go"
   fi

   if has_feature "docs"; then
       echo -n ".
       WithDoc(
           \"Get all items\",
           \"Retrieve a list of all items with optional filtering\",
           \"Items\",
           models.GetItemsRequest{},
           models.ItemsResponse{},
       )" >> "$PROJECT_NAME/internal/handlers/handlers.go"
   fi
   echo >> "$PROJECT_NAME/internal/handlers/handlers.go"
   echo >> "$PROJECT_NAME/internal/handlers/handlers.go"

   # Генерируем GET /items/{id}
   echo -n "    // GET /api/v1/items/{id} - получить по ID
   api.GET(\"/items/{id}\", h.GetItem)" >> "$PROJECT_NAME/internal/handlers/handlers.go"

   if has_feature "cache"; then
       echo -n ".
       WithCache(\"item_{id}\", 600, \"items\")" >> "$PROJECT_NAME/internal/handlers/handlers.go"
   fi

   if has_feature "docs"; then
       echo -n ".
       WithDoc(
           \"Get item by ID\",
           \"Retrieve a specific item by its unique identifier\",
           \"Items\",
           nil,
           models.ItemResponse{},
       )" >> "$PROJECT_NAME/internal/handlers/handlers.go"
   fi
   echo >> "$PROJECT_NAME/internal/handlers/handlers.go"
   echo >> "$PROJECT_NAME/internal/handlers/handlers.go"

   # Генерируем POST /items
   echo -n "    // POST /api/v1/items - создать
   api.POST(\"/items\", h.CreateItem)" >> "$PROJECT_NAME/internal/handlers/handlers.go"

   if has_feature "docs"; then
       echo -n ".
       WithDoc(
           \"Create new item\",
           \"Create a new item with the provided data\",
           \"Items\",
           models.CreateItemRequest{},
           models.ItemResponse{},
       )" >> "$PROJECT_NAME/internal/handlers/handlers.go"
   fi
   echo >> "$PROJECT_NAME/internal/handlers/handlers.go"
   echo >> "$PROJECT_NAME/internal/handlers/handlers.go"

   # Генерируем PUT /items/{id}
   echo -n "    // PUT /api/v1/items/{id} - обновить
   api.PUT(\"/items/{id}\", h.UpdateItem)" >> "$PROJECT_NAME/internal/handlers/handlers.go"

   if has_feature "docs"; then
       echo -n ".
       WithDoc(
           \"Update item\",
           \"Update an existing item by ID\",
           \"Items\",
           models.UpdateItemRequest{},
           models.ItemResponse{},
       )" >> "$PROJECT_NAME/internal/handlers/handlers.go"
   fi
   echo >> "$PROJECT_NAME/internal/handlers/handlers.go"
   echo >> "$PROJECT_NAME/internal/handlers/handlers.go"

   # Генерируем DELETE /items/{id}
   echo -n "    // DELETE /api/v1/items/{id} - удалить
   api.DELETE(\"/items/{id}\", h.DeleteItem)" >> "$PROJECT_NAME/internal/handlers/handlers.go"

   if has_feature "docs"; then
       echo -n ".
       WithDoc(
           \"Delete item\",
           \"Delete an item by its ID\",
           \"Items\",
           nil,
           models.StatusResponse{},
       )" >> "$PROJECT_NAME/internal/handlers/handlers.go"
   fi
   echo >> "$PROJECT_NAME/internal/handlers/handlers.go"
   echo "}" >> "$PROJECT_NAME/internal/handlers/handlers.go"

   # Добавляем реализации методов
   cat >> "$PROJECT_NAME/internal/handlers/handlers.go" << 'EOF'

// GetItems получает список всех элементов
func (h *Handler) GetItems(ctx *fasthttp.RequestCtx) {
   // Парсинг query параметров
   var req models.GetItemsRequest
   if err := h.parseQuery(ctx, &req); err != nil {
       h.writeErrorResponse(ctx, 400, "Invalid query parameters", err)
       return
   }

   // Пример данных (в реальном приложении здесь будет обращение к БД)
   items := []*models.Item{
       {
           ID:          "1",
           Name:        "Sample Item 1",
           Description: "This is a sample item",
           Status:      "active",
           CreatedAt:   time.Now().Add(-24 * time.Hour),
           UpdatedAt:   time.Now(),
       },
       {
           ID:          "2",
           Name:        "Sample Item 2",
           Description: "This is another sample item",
           Status:      "inactive",
           CreatedAt:   time.Now().Add(-48 * time.Hour),
           UpdatedAt:   time.Now().Add(-time.Hour),
       },
   }

   response := models.ItemsResponse{
       Success: true,
       Data:    items,
       Total:   len(items),
   }

   h.writeJSONResponse(ctx, 200, response)
}

// GetItem получает элемент по ID
func (h *Handler) GetItem(ctx *fasthttp.RequestCtx) {
   id := h.getParamString(ctx, "id")
   if id == "" {
       h.writeErrorResponse(ctx, 400, "ID is required", nil)
       return
   }

   // Пример данных (в реальном приложении здесь будет обращение к БД)
   item := &models.Item{
       ID:          id,
       Name:        "Sample Item " + id,
       Description: "This is a sample item with ID " + id,
       Status:      "active",
       CreatedAt:   time.Now().Add(-24 * time.Hour),
       UpdatedAt:   time.Now(),
   }

   response := models.ItemResponse{
       Success: true,
       Data:    item,
   }

   h.writeJSONResponse(ctx, 200, response)
}

// CreateItem создает новый элемент
func (h *Handler) CreateItem(ctx *fasthttp.RequestCtx) {
   var req models.CreateItemRequest
   if err := h.parseJSON(ctx, &req); err != nil {
       h.writeErrorResponse(ctx, 400, "Invalid JSON", err)
       return
   }

   if err := h.validateStruct(req); err != nil {
       h.writeErrorResponse(ctx, 400, "Validation failed", err)
       return
   }

   // Создание элемента (в реальном приложении здесь будет сохранение в БД)
   item := &models.Item{
       ID:          "new-" + strconv.FormatInt(time.Now().Unix(), 10),
       Name:        req.Name,
       Description: req.Description,
       Status:      "active",
       CreatedAt:   time.Now(),
       UpdatedAt:   time.Now(),
   }

EOF

   # Добавляем инвалидацию кеша если включен
   if has_feature "cache"; then
       cat >> "$PROJECT_NAME/internal/handlers/handlers.go" << 'EOF'
   // Инвалидация кеша после создания
   if cache := sai.Cache(); cache != nil {
       cache.Invalidate("items")
   }

EOF
   fi

   cat >> "$PROJECT_NAME/internal/handlers/handlers.go" << 'EOF'
   response := models.ItemResponse{
       Success: true,
       Data:    item,
   }

   h.writeJSONResponse(ctx, 201, response)
}

// UpdateItem обновляет существующий элемент
func (h *Handler) UpdateItem(ctx *fasthttp.RequestCtx) {
   id := h.getParamString(ctx, "id")
   if id == "" {
       h.writeErrorResponse(ctx, 400, "ID is required", nil)
       return
   }

   var req models.UpdateItemRequest
   if err := h.parseJSON(ctx, &req); err != nil {
       h.writeErrorResponse(ctx, 400, "Invalid JSON", err)
       return
   }

   if err := h.validateStruct(req); err != nil {
       h.writeErrorResponse(ctx, 400, "Validation failed", err)
       return
   }

   // Обновление элемента (в реальном приложении здесь будет обновление в БД)
   item := &models.Item{
       ID:          id,
       Name:        req.Name,
       Description: req.Description,
       Status:      req.Status,
       CreatedAt:   time.Now().Add(-24 * time.Hour), // Пример старой даты
       UpdatedAt:   time.Now(),
   }

EOF

   # Добавляем инвалидацию кеша если включен
   if has_feature "cache"; then
       cat >> "$PROJECT_NAME/internal/handlers/handlers.go" << 'EOF'
   // Инвалидация кеша после обновления
   if cache := sai.Cache(); cache != nil {
       cache.Invalidate("items")
   }

EOF
   fi

   cat >> "$PROJECT_NAME/internal/handlers/handlers.go" << 'EOF'
   response := models.ItemResponse{
       Success: true,
       Data:    item,
   }

   h.writeJSONResponse(ctx, 200, response)
}

// DeleteItem удаляет элемент
func (h *Handler) DeleteItem(ctx *fasthttp.RequestCtx) {
   id := h.getParamString(ctx, "id")
   if id == "" {
       h.writeErrorResponse(ctx, 400, "ID is required", nil)
       return
   }

   // Удаление элемента (в реальном приложении здесь будет удаление из БД)

EOF

   # Добавляем инвалидацию кеша если включен
   if has_feature "cache"; then
       cat >> "$PROJECT_NAME/internal/handlers/handlers.go" << 'EOF'
   // Инвалидация кеша после удаления
   if cache := sai.Cache(); cache != nil {
       cache.Invalidate("items")
   }

EOF
   fi

   cat >> "$PROJECT_NAME/internal/handlers/handlers.go" << 'EOF'
   response := models.StatusResponse{
       Success: true,
       Message: "Item deleted successfully",
   }

   h.writeJSONResponse(ctx, 200, response)
}

// Вспомогательные методы

func (h *Handler) parseQuery(ctx *fasthttp.RequestCtx, target interface{}) error {
   // Здесь должна быть реализация парсинга query параметров
   // Для примера возвращаем nil
   return nil
}

func (h *Handler) parseJSON(ctx *fasthttp.RequestCtx, target interface{}) error {
   return utils.Unmarshal(ctx.PostBody(), &target)
}

func (h *Handler) validateStruct(s interface{}) error {
   // Здесь должна быть реализация валидации
   // Для примера возвращаем nil
   return nil
}

func (h *Handler) getParamString(ctx *fasthttp.RequestCtx, key string) string {
   if params, ok := ctx.UserValue("route_params").(map[string]string); ok {
       return params[key]
   }
   return ""
}

func (h *Handler) writeJSONResponse(ctx *fasthttp.RequestCtx, statusCode int, data interface{}) {
   ctx.SetContentType("application/json")
   ctx.SetStatusCode(statusCode)

   jsonData, err := utils.Marshal(data)
   if err != nil {
       ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
       return
   }

   ctx.Write(jsonData)
}

func (h *Handler) writeErrorResponse(ctx *fasthttp.RequestCtx, statusCode int, message string, err error) {
   response := models.StatusResponse{
       Success: false,
       Message: message,
   }

   if err != nil {
       response.Error = err.Error()
   }

   h.writeJSONResponse(ctx, statusCode, response)
}
EOF
}

# Генерация models.go
generate_models() {
   log_info "Generating models.go..."

   cat > "$PROJECT_NAME/internal/models/model.go" << 'EOF'
package models

import "time"

// Item представляет основную модель данных
type Item struct {
   ID          string    `json:"id" validate:"required" example:"item-123" doc:"Unique item identifier"`
   Name        string    `json:"name" validate:"required,min=3,max=100" example:"Sample Item" doc:"Item name"`
   Description string    `json:"description" validate:"max=500" example:"This is a sample item" doc:"Item description"`
   Status      string    `json:"status" validate:"required,oneof=active inactive" example:"active" doc:"Item status"`
   CreatedAt   time.Time `json:"created_at" example:"2023-01-01T00:00:00Z" doc:"Creation timestamp"`
   UpdatedAt   time.Time `json:"updated_at" example:"2023-01-01T00:00:00Z" doc:"Last update timestamp"`
}

// Запросы

type GetItemsRequest struct {
   Page   int    `json:"page" query:"page" validate:"min=1" example:"1" doc:"Page number for pagination"`
   Limit  int    `json:"limit" query:"limit" validate:"min=1,max=100" example:"10" doc:"Number of items per page"`
   Status string `json:"status" query:"status" validate:"omitempty,oneof=active inactive" example:"active" doc:"Filter by status"`
   Search string `json:"search" query:"search" validate:"omitempty,max=100" example:"search term" doc:"Search in name and description"`
}

type CreateItemRequest struct {
   Name        string `json:"name" validate:"required,min=3,max=100" example:"New Item" doc:"Item name"`
   Description string `json:"description" validate:"max=500" example:"Description of the new item" doc:"Item description"`
}

type UpdateItemRequest struct {
   Name        string `json:"name" validate:"omitempty,min=3,max=100" example:"Updated Item" doc:"Updated item name"`
   Description string `json:"description" validate:"omitempty,max=500" example:"Updated description" doc:"Updated item description"`
   Status      string `json:"status" validate:"omitempty,oneof=active inactive" example:"inactive" doc:"Updated item status"`
}

// Ответы

type ItemResponse struct {
   Success bool   `json:"success" example:"true" doc:"Operation success status"`
   Data    *Item  `json:"data,omitempty" doc:"Item data"`
   Error   string `json:"error,omitempty" example:"" doc:"Error message if operation failed"`
}

type ItemsResponse struct {
   Success bool    `json:"success" example:"true" doc:"Operation success status"`
   Data    []*Item `json:"data,omitempty" doc:"List of items"`
   Total   int     `json:"total" example:"100" doc:"Total number of items"`
   Error   string  `json:"error,omitempty" example:"" doc:"Error message if operation failed"`
}

type StatusResponse struct {
   Success bool   `json:"success" example:"true" doc:"Operation success status"`
   Message string `json:"message" example:"Operation completed successfully" doc:"Status message"`
   Error   string `json:"error,omitempty" example:"" doc:"Error message if operation failed"`
}
EOF
}

# Генерация конфигурации
generate_config() {
   log_info "Generating config.yml..."

   cat > "$PROJECT_NAME/config.yml" << EOF
name: "$PROJECT_NAME"
version: "1.0.0"

server:
 http:
   host: "0.0.0.0"
   port: $PORT
   read_timeout: 30
   write_timeout: 30
   idle_timeout: 120
EOF

   if has_feature "tls"; then
       cat >> "$PROJECT_NAME/config.yml" << EOF
 tls:
   enabled: true
   auto_cert: false
   cert_file: ""
   key_file: ""
   domains: []
   email: ""
   cache_dir: "./certs"
EOF
   else
       cat >> "$PROJECT_NAME/config.yml" << EOF
 tls:
   enabled: false
EOF
   fi

   cat >> "$PROJECT_NAME/config.yml" << EOF

logger:
 level: "info"
 type: "default"
 config:
   format: "console"
   output: "stdout"
EOF

   if has_feature "cache"; then
       cat >> "$PROJECT_NAME/config.yml" << EOF

cache:
 enabled: true
 type: "memory"
 default_ttl: "5m"
 config:
   max_entries: 10000
   cleanup_interval: "5m"
EOF
   else
       cat >> "$PROJECT_NAME/config.yml" << EOF

cache:
 enabled: false
EOF
   fi

   if has_feature "metrics"; then
       cat >> "$PROJECT_NAME/config.yml" << EOF

metrics:
 enabled: true
 type: "memory"
 config:
   retention_period: "24h"
   max_metrics: 10000
   cleanup_interval: "1h"
EOF
   else
       cat >> "$PROJECT_NAME/config.yml" << EOF

metrics:
 enabled: false
EOF
   fi

   if has_feature "actions"; then
       cat >> "$PROJECT_NAME/config.yml" << EOF

actions:
 enabled: true
 webhook: false
 type: "websocket"
 config:
   url: "ws://localhost:8081/ws"
   reconnect_delay: "5s"
   max_retries: 10
EOF
   else
       cat >> "$PROJECT_NAME/config.yml" << EOF

actions:
 enabled: false
EOF
   fi

   if has_feature "cron"; then
       cat >> "$PROJECT_NAME/config.yml" << EOF

cron:
 enabled: true
 timezone: "UTC"
EOF
   else
       cat >> "$PROJECT_NAME/config.yml" << EOF

cron:
 enabled: false
EOF
   fi

   if has_feature "docs"; then
       cat >> "$PROJECT_NAME/config.yml" << EOF

docs:
 enabled: true
 path: "/docs"
EOF
   else
     cat >> "$PROJECT_NAME/config.yml" << EOF

docs:
enabled: false
EOF
  fi

  if has_feature "health"; then
      cat >> "$PROJECT_NAME/config.yml" << EOF

health:
enabled: true
EOF
  else
      cat >> "$PROJECT_NAME/config.yml" << EOF

health:
enabled: false
EOF
  fi

  if has_feature "client"; then
      cat >> "$PROJECT_NAME/config.yml" << EOF

client:
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
EOF
  else
      cat >> "$PROJECT_NAME/config.yml" << EOF

client:
enabled: false
EOF
  fi

  if has_feature "middleware"; then
      cat >> "$PROJECT_NAME/config.yml" << EOF

middlewares:
enabled: true
recovery:
  enabled: true
  weight: 10
  params:
    stack_trace: true
logging:
  enabled: true
  weight: 20
  params:
    log_level: "info"
    log_headers: false
    log_body: false
metadata:
  enabled: true
  weight: 30
  params:
    generate_request_id: true
    propagated_headers: ["Authorization", "X-User-ID", "X-Request-ID"]
rate_limit:
  enabled: false
  weight: 40
  params:
    requests_per_minute: 100
body_limit:
  enabled: true
  weight: 50
  params:
    max_body_size: 10485760
cors:
  enabled: true
  weight: 60
  params:
    AllowedOrigins: ["*"]
    AllowedMethods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    AllowedHeaders: ["Content-Type", "Authorization", "X-API-Key"]
    MaxAge: 86400
auth:
  enabled: false
  weight: 70
  params:
    token: "your-secret-token"
cache:
  enabled: $(has_feature "cache" && echo "true" || echo "false")
  weight: 80
  params:
    default_ttl: "5m"
compression:
  enabled: false
  weight: 90
  params:
    algorithm: "gzip"
    level: 6
    threshold: 1024
EOF
  else
      cat >> "$PROJECT_NAME/config.yml" << EOF

middlewares:
enabled: false
EOF
  fi

  cat >> "$PROJECT_NAME/config.yml" << EOF

# Внешние сервисы (для клиентских подключений)
services: {}
# example-service:
#   host: "example.com"
#   port: 443
EOF
}

# Генерация Makefile
generate_makefile() {
  log_info "Generating Makefile..."

  cat > "$PROJECT_NAME/Makefile" << EOF
.PHONY: build run test clean docker-build docker-run docker-compose lint format help

# Переменные
APP_NAME := $PROJECT_NAME
DOCKER_IMAGE := \$(APP_NAME):latest
DOCKER_COMPOSE_FILE := docker-compose.yml

# По умолчанию
all: build

# Сборка приложения
build:
  @echo "Building \$(APP_NAME)..."
  go build -o bin/\$(APP_NAME) ./cmd

# Запуск приложения
run: build
  @echo "Running \$(APP_NAME)..."
  ./bin/\$(APP_NAME)

# Запуск в dev режиме
dev:
  @echo "Running \$(APP_NAME) in development mode..."
  go run ./cmd

# Тесты
test:
  @echo "Running tests..."
  go test -v ./...

EOF

  if [[ "$INCLUDE_TESTS" == "true" ]]; then
      cat >> "$PROJECT_NAME/Makefile" << EOF
# Интеграционные тесты
test-integration:
  @echo "Running integration tests..."
  go test -v ./tests/integration/...

# Тесты с покрытием
test-coverage:
  @echo "Running tests with coverage..."
  go test -v -coverprofile=coverage.out ./...
  go tool cover -html=coverage.out -o coverage.html

EOF
  fi

  cat >> "$PROJECT_NAME/Makefile" << EOF
# Линтер
lint:
  @echo "Running linter..."
  golangci-lint run

# Форматирование кода
format:
  @echo "Formatting code..."
  go fmt ./...
  go mod tidy

# Очистка
clean:
  @echo "Cleaning..."
  rm -rf bin/
  rm -f coverage.out coverage.html

# Docker сборка
docker-build:
  @echo "Building Docker image..."
  docker build -t \$(DOCKER_IMAGE) .

# Docker запуск
docker-run: docker-build
  @echo "Running Docker container..."
  docker run -p $PORT:$PORT \$(DOCKER_IMAGE)

# Docker Compose
docker-compose:
  @echo "Starting with Docker Compose..."
  docker-compose -f \$(DOCKER_COMPOSE_FILE) up --build

# Docker Compose в фоне
docker-compose-up:
  @echo "Starting with Docker Compose (detached)..."
  docker-compose -f \$(DOCKER_COMPOSE_FILE) up -d --build

# Остановка Docker Compose
docker-compose-down:
  @echo "Stopping Docker Compose..."
  docker-compose -f \$(DOCKER_COMPOSE_FILE) down

EOF

  if [[ "$INCLUDE_TESTS" == "true" ]]; then
      cat >> "$PROJECT_NAME/Makefile" << EOF
# Тесты в Docker
docker-test:
  @echo "Running tests in Docker..."
  docker-compose -f docker-compose.test.yml up --build --abort-on-container-exit

EOF
  fi

  cat >> "$PROJECT_NAME/Makefile" << EOF
# Помощь
help:
  @echo "Available commands:"
  @echo "  build              - Build the application"
  @echo "  run                - Build and run the application"
  @echo "  dev                - Run in development mode"
  @echo "  test               - Run unit tests"
EOF

  if [[ "$INCLUDE_TESTS" == "true" ]]; then
      cat >> "$PROJECT_NAME/Makefile" << EOF
  @echo "  test-integration   - Run integration tests"
  @echo "  test-coverage      - Run tests with coverage"
EOF
  fi

  cat >> "$PROJECT_NAME/Makefile" << EOF
  @echo "  lint               - Run linter"
  @echo "  format             - Format code"
  @echo "  clean              - Clean build artifacts"
  @echo "  docker-build       - Build Docker image"
  @echo "  docker-run         - Run in Docker"
  @echo "  docker-compose     - Start with Docker Compose"
  @echo "  docker-compose-up  - Start with Docker Compose (detached)"
  @echo "  docker-compose-down- Stop Docker Compose"
EOF

  if [[ "$INCLUDE_TESTS" == "true" ]]; then
      cat >> "$PROJECT_NAME/Makefile" << EOF
  @echo "  docker-test        - Run tests in Docker"
EOF
  fi

  cat >> "$PROJECT_NAME/Makefile" << EOF
  @echo "  help               - Show this help"
EOF
}

# Генерация Dockerfile
generate_dockerfile() {
  log_info "Generating Dockerfile..."

  cat > "$PROJECT_NAME/Dockerfile" << EOF
# Multi-stage build
FROM golang:1.21-alpine AS builder

# Установка зависимостей для сборки
RUN apk add --no-cache git ca-certificates tzdata

# Создание пользователя для приложения
RUN adduser -D -g '' appuser

# Рабочая директория
WORKDIR /build

# Копирование go.mod и go.sum
COPY go.mod go.sum ./

# Загрузка зависимостей
RUN go mod download

# Копирование исходного кода
COPY . .

# Сборка приложения
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \\
  -ldflags='-w -s -extldflags "-static"' \\
  -a -installsuffix cgo \\
  -o app ./cmd

# Финальный образ
FROM scratch

# Импорт пользователя из builder
COPY --from=builder /etc/passwd /etc/passwd

# Импорт CA сертификатов
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Импорт временных зон
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Копирование бинарника
COPY --from=builder /build/app /app

# Копирование конфигурации
COPY --from=builder /build/config.yml /config.yml

# Пользователь
USER appuser

# Порт
EXPOSE $PORT

# Healthcheck
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\
  CMD ["/app", "--healthcheck"]

# Запуск
ENTRYPOINT ["/app"]
EOF
}

# Генерация docker-compose.yml
generate_docker_compose() {
  log_info "Generating docker-compose.yml..."

  cat > "$PROJECT_NAME/docker-compose.yml" << EOF
version: '3.8'

services:
  app:
    build: .
    ports:
      - "$PORT:$PORT"
    environment:
      - ENV=production
    volumes:
      - ./config.yml:/config.yml:ro
EOF

  if has_feature "cache" && [[ "$ALL_FEATURES" =~ "redis" ]]; then
      cat >> "$PROJECT_NAME/docker-compose.yml" << EOF
    depends_on:
      - redis
EOF
  fi

  if has_feature "metrics" && [[ "$ALL_FEATURES" =~ "prometheus" ]]; then
      cat >> "$PROJECT_NAME/docker-compose.yml" << EOF
    depends_on:
      - prometheus
EOF
  fi

  cat >> "$PROJECT_NAME/docker-compose.yml" << EOF
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "/app", "--healthcheck"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

EOF

  # Добавляем Redis если нужен
  if has_feature "cache"; then
      cat >> "$PROJECT_NAME/docker-compose.yml" << EOF
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 30s
      timeout: 10s
      retries: 3

EOF
  fi

  # Добавляем Prometheus если нужен
  if has_feature "metrics"; then
      cat >> "$PROJECT_NAME/docker-compose.yml" << EOF
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
      - '--storage.tsdb.retention.time=200h'
      - '--web.enable-lifecycle'
    restart: unless-stopped

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    volumes:
      - grafana_data:/var/lib/grafana
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    restart: unless-stopped

EOF
  fi

  # Добавляем volumes
  cat >> "$PROJECT_NAME/docker-compose.yml" << EOF
volumes:
EOF

  if has_feature "cache"; then
      cat >> "$PROJECT_NAME/docker-compose.yml" << EOF
  redis_data:
EOF
  fi

  if has_feature "metrics"; then
      cat >> "$PROJECT_NAME/docker-compose.yml" << EOF
  prometheus_data:
  grafana_data:
EOF
  fi

  # Генерируем конфигурацию Prometheus если нужна
  if has_feature "metrics"; then
      cat > "$PROJECT_NAME/prometheus.yml" << EOF
global:
  scrape_interval: 15s
  evaluation_interval: 15s

  scrape_configs:
  - job_name: '$PROJECT_NAME'
    static_configs:
      - targets: ['app:$PORT']
    metrics_path: /metrics
    scrape_interval: 5s
EOF
  fi
}

# Генерация тестов
generate_tests() {
  if [[ "$INCLUDE_TESTS" != "true" ]]; then
      return
  fi

  log_info "Generating tests..."

  # Основной тестовый файл
  cat > "$PROJECT_NAME/tests/integration/main_test.go" << EOF
package integration

import (
  "context"
  "os"
  "testing"
  "time"

  "github.com/saiset-co/sai-service/service"
  "$MODULE_NAME/internal"
)

var (
  testService *service.Service
EOF

  if has_feature "cache"; then
      echo "    cacheEnabled = true" >> "$PROJECT_NAME/tests/integration/main_test.go"
  else
      echo "    cacheEnabled = false" >> "$PROJECT_NAME/tests/integration/main_test.go"
  fi

  if has_feature "docs"; then
      echo "    docsEnabled = true" >> "$PROJECT_NAME/tests/integration/main_test.go"
  else
      echo "    docsEnabled = false" >> "$PROJECT_NAME/tests/integration/main_test.go"
  fi

  cat >> "$PROJECT_NAME/tests/integration/main_test.go" << EOF
)

func TestMain(m *testing.M) {
  // Настройка тестового окружения
  ctx := context.Background()

  var err error
  testService, err = internal.NewService(ctx, "../../config.yml")
  if err != nil {
      panic(err)
  }

  // Запуск сервиса в фоне
  go func() {
      if err := testService.Run(); err != nil {
          panic(err)
      }
  }()

  // Ждем запуска сервиса
  time.Sleep(2 * time.Second)

  // Запуск тестов
  code := m.Run()

  // Остановка сервиса
  testService.Stop()

  os.Exit(code)
}
EOF

  # Тесты для handlers
  cat > "$PROJECT_NAME/tests/integration/handlers_test.go" << 'EOF'
package integration

import (
  "bytes"
  "encoding/json"
  "net/http"
  "testing"
  "time"

  "github.com/stretchr/testify/assert"
  "github.com/stretchr/testify/require"
)

const baseURL = "http://localhost:8080"

func TestGetItems(t *testing.T) {
  resp, err := http.Get(baseURL + "/api/v1/items")
  require.NoError(t, err)
  defer resp.Body.Close()

  assert.Equal(t, http.StatusOK, resp.StatusCode)
  assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

  var response map[string]interface{}
  err = json.NewDecoder(resp.Body).Decode(&response)
  require.NoError(t, err)

  assert.True(t, response["success"].(bool))
  assert.NotNil(t, response["data"])
}

func TestGetItem(t *testing.T) {
  resp, err := http.Get(baseURL + "/api/v1/items/1")
  require.NoError(t, err)
  defer resp.Body.Close()

  assert.Equal(t, http.StatusOK, resp.StatusCode)

  var response map[string]interface{}
  err = json.NewDecoder(resp.Body).Decode(&response)
  require.NoError(t, err)

  assert.True(t, response["success"].(bool))
  assert.NotNil(t, response["data"])
}

func TestCreateItem(t *testing.T) {
  requestBody := map[string]interface{}{
      "name":        "Test Item",
      "description": "Test Description",
  }

  jsonBody, err := json.Marshal(requestBody)
  require.NoError(t, err)

  resp, err := http.Post(baseURL + "/api/v1/items", "application/json", bytes.NewBuffer(jsonBody))
  require.NoError(t, err)
  defer resp.Body.Close()

  assert.Equal(t, http.StatusCreated, resp.StatusCode)

  var response map[string]interface{}
  err = json.NewDecoder(resp.Body).Decode(&response)
  require.NoError(t, err)

  assert.True(t, response["success"].(bool))
  assert.NotNil(t, response["data"])
}

func TestUpdateItem(t *testing.T) {
  requestBody := map[string]interface{}{
      "name":        "Updated Item",
      "description": "Updated Description",
      "status":      "inactive",
  }

  jsonBody, err := json.Marshal(requestBody)
  require.NoError(t, err)

  req, err := http.NewRequest("PUT", baseURL + "/api/v1/items/1", bytes.NewBuffer(jsonBody))
  require.NoError(t, err)
  req.Header.Set("Content-Type", "application/json")

  client := &http.Client{}
  resp, err := client.Do(req)
  require.NoError(t, err)
  defer resp.Body.Close()

  assert.Equal(t, http.StatusOK, resp.StatusCode)

  var response map[string]interface{}
  err = json.NewDecoder(resp.Body).Decode(&response)
  require.NoError(t, err)

  assert.True(t, response["success"].(bool))
}

func TestDeleteItem(t *testing.T) {
  req, err := http.NewRequest("DELETE", baseURL + "/api/v1/items/1", nil)
  require.NoError(t, err)

  client := &http.Client{}
  resp, err := client.Do(req)
  require.NoError(t, err)
  defer resp.Body.Close()

  assert.Equal(t, http.StatusOK, resp.StatusCode)

  var response map[string]interface{}
  err = json.NewDecoder(resp.Body).Decode(&response)
  require.NoError(t, err)

  assert.True(t, response["success"].(bool))
}

func TestGetItemsWithCache(t *testing.T) {
  if !cacheEnabled {
      t.Skip("Cache not enabled")
  }

  // Первый запрос
  start1 := time.Now()
  resp1, err := http.Get(baseURL + "/api/v1/items")
  duration1 := time.Since(start1)
  require.NoError(t, err)
  resp1.Body.Close()

  assert.Equal(t, http.StatusOK, resp1.StatusCode)

  // Второй запрос (должен быть из кеша)
  start2 := time.Now()
  resp2, err := http.Get(baseURL + "/api/v1/items")
  duration2 := time.Since(start2)
  require.NoError(t, err)
  resp2.Body.Close()

  assert.Equal(t, http.StatusOK, resp2.StatusCode)
  // Кеш должен быть быстрее
  assert.Less(t, duration2, duration1)
}

func TestOpenAPIDocumentation(t *testing.T) {
  if !docsEnabled {
      t.Skip("Documentation not enabled")
  }

  // Проверяем Swagger UI
  resp, err := http.Get(baseURL + "/docs")
  require.NoError(t, err)
  defer resp.Body.Close()

  assert.Equal(t, http.StatusOK, resp.StatusCode)

  // Проверяем OpenAPI JSON
  resp, err = http.Get(baseURL + "/openapi.json")
  require.NoError(t, err)
  defer resp.Body.Close()

  assert.Equal(t, http.StatusOK, resp.StatusCode)

  var spec map[string]interface{}
  err = json.NewDecoder(resp.Body).Decode(&spec)
  require.NoError(t, err)

  assert.Contains(t, spec, "openapi")
  assert.Contains(t, spec, "paths")
}

func TestHealthCheck(t *testing.T) {
  resp, err := http.Get(baseURL + "/health")
  require.NoError(t, err)
  defer resp.Body.Close()

  assert.Equal(t, http.StatusOK, resp.StatusCode)

  var health map[string]interface{}
  err = json.NewDecoder(resp.Body).Decode(&health)
  require.NoError(t, err)

  assert.Contains(t, health, "status")
}

// Вспомогательные функции
func makeRequest(t *testing.T, method, path string, body interface{}) *http.Response {
  var reqBody *bytes.Buffer
  if body != nil {
      jsonBody, err := json.Marshal(body)
      require.NoError(t, err)
      reqBody = bytes.NewBuffer(jsonBody)
  }

  var req *http.Request
  var err error

  if reqBody != nil {
      req, err = http.NewRequest(method, baseURL + path, reqBody)
      req.Header.Set("Content-Type", "application/json")
  } else {
      req, err = http.NewRequest(method, baseURL + path, nil)
  }
  require.NoError(t, err)

  client := &http.Client{Timeout: 10 * time.Second}
  resp, err := client.Do(req)
  require.NoError(t, err)

  return resp
}
EOF

  # Test helpers
  cat > "$PROJECT_NAME/tests/helpers/test_helpers.go" << 'EOF'
package helpers

import (
  "bytes"
  "encoding/json"
  "net/http"
  "testing"

  "github.com/stretchr/testify/require"
)

// HTTPTestHelper предоставляет утилиты для HTTP тестирования
type HTTPTestHelper struct {
  BaseURL string
  Client  *http.Client
}

// NewHTTPTestHelper создает новый помощник для HTTP тестов
func NewHTTPTestHelper(baseURL string) *HTTPTestHelper {
  return &HTTPTestHelper{
      BaseURL: baseURL,
      Client:  &http.Client{},
  }
}

// GET выполняет GET запрос
func (h *HTTPTestHelper) GET(t *testing.T, path string) *http.Response {
  resp, err := h.Client.Get(h.BaseURL + path)
  require.NoError(t, err)
  return resp
}

// POST выполняет POST запрос с JSON телом
func (h *HTTPTestHelper) POST(t *testing.T, path string, body interface{}) *http.Response {
  jsonBody, err := json.Marshal(body)
  require.NoError(t, err)

  resp, err := h.Client.Post(h.BaseURL + path, "application/json", bytes.NewBuffer(jsonBody))
  require.NoError(t, err)
  return resp
}

// PUT выполняет PUT запрос с JSON телом
func (h *HTTPTestHelper) PUT(t *testing.T, path string, body interface{}) *http.Response {
  jsonBody, err := json.Marshal(body)
  require.NoError(t, err)

  req, err := http.NewRequest("PUT", h.BaseURL + path, bytes.NewBuffer(jsonBody))
  require.NoError(t, err)
  req.Header.Set("Content-Type", "application/json")

  resp, err := h.Client.Do(req)
  require.NoError(t, err)
  return resp
}

// DELETE выполняет DELETE запрос
func (h *HTTPTestHelper) DELETE(t *testing.T, path string) *http.Response {
  req, err := http.NewRequest("DELETE", h.BaseURL + path, nil)
  require.NoError(t, err)

  resp, err := h.Client.Do(req)
  require.NoError(t, err)
  return resp
}

// DecodeJSON декодирует JSON ответ
func (h *HTTPTestHelper) DecodeJSON(t *testing.T, resp *http.Response, target interface{}) {
  defer resp.Body.Close()
  err := json.NewDecoder(resp.Body).Decode(target)
  require.NoError(t, err)
}
EOF

  # Docker compose для тестов
  if has_feature "cache" || has_feature "metrics"; then
      cat > "$PROJECT_NAME/docker-compose.test.yml" << EOF
version: '3.8'

services:
  test:
    build: .
    command: make test-integration
    environment:
      - ENV=test
    volumes:
      - ./config.yml:/config.yml:ro
EOF

      if has_feature "cache"; then
          cat >> "$PROJECT_NAME/docker-compose.test.yml" << EOF
    depends_on:
      - redis-test
EOF
      fi

      cat >> "$PROJECT_NAME/docker-compose.test.yml" << EOF

EOF

      if has_feature "cache"; then
          cat >> "$PROJECT_NAME/docker-compose.test.yml" << EOF
  redis-test:
    image: redis:7-alpine
    command: redis-server --save "" --appendonly no
    tmpfs:
      - /data

EOF
      fi
  fi
}

# Генерация CI/CD файлов
generate_ci() {
  if [[ "$CI_TYPE" == "none" ]]; then
      return
  fi

  log_info "Generating CI/CD files for $CI_TYPE..."

  if [[ "$CI_TYPE" == "github" ]]; then
      cat > "$PROJECT_NAME/.github/workflows/ci.yml" << EOF
name: CI

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'

    - name: Cache Go modules
      uses: actions/cache@v3
      with:
        path: |
          ~/.cache/go-build
          ~/go/pkg/mod
        key: \${{ runner.os }}-go-\${{ hashFiles('**/go.sum') }}
        restore-keys: |
          \${{ runner.os }}-go-

    - name: Download dependencies
      run: go mod download

    - name: Run tests
      run: make test

    - name: Run linter
      uses: golangci/golangci-lint-action@v3
      with:
        version: latest

EOF

      if [[ "$INCLUDE_TESTS" == "true" ]]; then
          cat >> "$PROJECT_NAME/.github/workflows/ci.yml" << EOF
  integration-test:
    runs-on: ubuntu-latest

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'

    - name: Run integration tests
      run: make docker-test

EOF
      fi

      cat >> "$PROJECT_NAME/.github/workflows/ci.yml" << EOF
  build:
    runs-on: ubuntu-latest
    needs: test

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v3

    - name: Build Docker image
      run: make docker-build

    - name: Test Docker image
      run: |
        docker run --rm -d -p $PORT:$PORT --name test-container $PROJECT_NAME:latest
        sleep 10
        curl -f http://localhost:$PORT/health || exit 1
        docker stop test-container

  security:
    runs-on: ubuntu-latest

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Run Gosec Security Scanner
      uses: securecodewarrior/github-action-gosec@master
      with:
        args: './...'
EOF

  elif [[ "$CI_TYPE" == "gitlab" ]]; then
      cat > "$PROJECT_NAME/.gitlab-ci.yml" << EOF
stages:
  - test
  - build
  - security

variables:
  GO_VERSION: "1.21"
  DOCKER_IMAGE: \$CI_REGISTRY_IMAGE:\$CI_COMMIT_SHA

before_script:
- echo "Starting CI/CD pipeline for $PROJECT_NAME"

# Тесты
test:
  stage: test
  image: golang:\$GO_VERSION
  script:
    - go mod download
    - make test
    - make lint
  coverage: '/coverage: \d+\.\d+% of statements/'

EOF

      if [[ "$INCLUDE_TESTS" == "true" ]]; then
          cat >> "$PROJECT_NAME/.gitlab-ci.yml" << EOF
integration-test:
  stage: test
  services:
EOF

          if has_feature "cache"; then
              cat >> "$PROJECT_NAME/.gitlab-ci.yml" << EOF
    - name: redis:7-alpine
      alias: redis
EOF
          fi

          cat >> "$PROJECT_NAME/.gitlab-ci.yml" << EOF
  script:
    - make test-integration

EOF
      fi

      cat >> "$PROJECT_NAME/.gitlab-ci.yml" << EOF
# Сборка
build:
  stage: build
  image: docker:latest
  services:
    - docker:dind
  script:
    - docker build -t \$DOCKER_IMAGE .
    - docker push \$DOCKER_IMAGE
  only:
    - main
    - develop

# Безопасность
security:
  stage: security
  image: golang:\$GO_VERSION
  script:
    - go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
    - gosec ./...
  allow_failure: true
EOF
  fi
}

# Генерация README.md
generate_readme() {
  log_info "Generating README.md..."

  cat > "$PROJECT_NAME/README.md" << EOF
# $PROJECT_NAME

A microservice built with SAI Service framework.

## Features

EOF

  # Добавляем список включенных фич
  if has_feature "cache"; then
      echo "- ✅ Cache (Memory/Redis)" >> "$PROJECT_NAME/README.md"
  fi
  if has_feature "metrics"; then
      echo "- ✅ Metrics (Memory/Prometheus)" >> "$PROJECT_NAME/README.md"
  fi
  if has_feature "docs"; then
      echo "- ✅ API Documentation (OpenAPI/Swagger)" >> "$PROJECT_NAME/README.md"
  fi
  if has_feature "health"; then
      echo "- ✅ Health Checks" >> "$PROJECT_NAME/README.md"
  fi
  if has_feature "middleware"; then
      echo "- ✅ Middleware (Logging, Recovery, CORS, etc.)" >> "$PROJECT_NAME/README.md"
  fi
  if has_feature "cron"; then
      echo "- ✅ Cron Jobs" >> "$PROJECT_NAME/README.md"
  fi
  if has_feature "actions"; then
      echo "- ✅ Action Broker (WebSocket)" >> "$PROJECT_NAME/README.md"
  fi
  if has_feature "tls"; then
      echo "- ✅ TLS/Auto-certificates" >> "$PROJECT_NAME/README.md"
  fi
  if has_feature "client"; then
      echo "- ✅ HTTP Client Manager" >> "$PROJECT_NAME/README.md"
  fi
  if [[ "$INCLUDE_TESTS" == "true" ]]; then
      echo "- ✅ Integration Tests" >> "$PROJECT_NAME/README.md"
  fi

  cat >> "$PROJECT_NAME/README.md" << EOF

## Quick Start

### Prerequisites

- Go 1.21 or later
- Docker and Docker Compose (optional)

### Local Development

1. Clone and setup:
\`\`\`bash
git clone <repository-url>
cd $PROJECT_NAME
go mod download
\`\`\`

2. Run the service:
\`\`\`bash
make run
\`\`\`

3. The service will be available at http://localhost:$PORT

### Docker

1. Build and run with Docker:
\`\`\`bash
make docker-run
\`\`\`

2. Or use Docker Compose:
\`\`\`bash
make docker-compose
\`\`\`

## API Endpoints

The service provides the following REST API endpoints:

### Items API

- \`GET /api/v1/items\` - Get all items
- \`GET /api/v1/items/{id}\` - Get item by ID
- \`POST /api/v1/items\` - Create new item
- \`PUT /api/v1/items/{id}\` - Update item
- \`DELETE /api/v1/items/{id}\` - Delete item

### System Endpoints

- \`GET /health\` - Health check
- \`GET /version\` - Service version
EOF

  if has_feature "metrics"; then
      echo "- \`GET /metrics\` - Prometheus metrics" >> "$PROJECT_NAME/README.md"
  fi

  if has_feature "docs"; then
      echo "- \`GET /docs\` - API documentation (Swagger UI)" >> "$PROJECT_NAME/README.md"
      echo "- \`GET /openapi.json\` - OpenAPI specification" >> "$PROJECT_NAME/README.md"
  fi

  cat >> "$PROJECT_NAME/README.md" << EOF

### Example Requests

#### Create Item
\`\`\`bash
curl -X POST http://localhost:$PORT/api/v1/items \\
-H "Content-Type: application/json" \\
-d '{
  "name": "Sample Item",
  "description": "This is a sample item"
}'
\`\`\`

#### Get All Items
\`\`\`bash
curl http://localhost:$PORT/api/v1/items
\`\`\`

#### Get Item by ID
\`\`\`bash
curl http://localhost:$PORT/api/v1/items/1
\`\`\`

## Configuration

The service is configured via \`config.yml\`:

\`\`\`yaml
name: "$PROJECT_NAME"
version: "1.0.0"

server:
http:
  host: "0.0.0.0"
  port: $PORT
EOF

  if has_feature "cache"; then
      cat >> "$PROJECT_NAME/README.md" << EOF

cache:
enabled: true
type: "memory"  # or "redis"
EOF
  fi

  if has_feature "metrics"; then
      cat >> "$PROJECT_NAME/README.md" << EOF

metrics:
enabled: true
type: "memory"  # or "prometheus"
EOF
  fi

  cat >> "$PROJECT_NAME/README.md" << EOF
\`\`\`

See \`config.yml\` for full configuration options.

## Development

### Available Commands

\`\`\`bash
make build              # Build the application
make run                # Build and run
make dev                # Run in development mode
make test               # Run unit tests
EOF

  if [[ "$INCLUDE_TESTS" == "true" ]]; then
      cat >> "$PROJECT_NAME/README.md" << EOF
make test-integration   # Run integration tests
make test-coverage      # Run tests with coverage
EOF
  fi

  cat >> "$PROJECT_NAME/README.md" << EOF
make lint               # Run linter
make format             # Format code
make docker-build       # Build Docker image
make docker-compose     # Start with dependencies
make clean              # Clean build artifacts
\`\`\`

### Project Structure

\`\`\`
$PROJECT_NAME/
├── cmd/                    # Application entrypoint
│   └── main.go
├── internal/               # Private application code
│   ├── service.go         # Service initialization
│   ├── handlers/          # HTTP handlers
│   │   └── handlers.go
│   └── models/            # Data models
│       └── model.go
EOF

  if [[ "$INCLUDE_TESTS" == "true" ]]; then
      cat >> "$PROJECT_NAME/README.md" << EOF
├── tests/                 # Integration tests
│   ├── integration/
│   └── helpers/
EOF
  fi

  cat >> "$PROJECT_NAME/README.md" << EOF
├── config.yml             # Configuration
├── Dockerfile             # Docker image definition
├── docker-compose.yml     # Docker Compose setup
├── Makefile              # Development commands
└── README.md             # This file
\`\`\`

EOF

  if [[ "$INCLUDE_TESTS" == "true" ]]; then
      cat >> "$PROJECT_NAME/README.md" << EOF
### Running Tests

#### Unit Tests
\`\`\`bash
make test
\`\`\`

#### Integration Tests
\`\`\`bash
make test-integration
\`\`\`

#### Tests with Coverage
\`\`\`bash
make test-coverage
\`\`\`

#### Tests in Docker
\`\`\`bash
make docker-test
\`\`\`

EOF
  fi

  if has_feature "docs"; then
      cat >> "$PROJECT_NAME/README.md" << EOF
### API Documentation

When the service is running, you can access:

- **Swagger UI**: http://localhost:$PORT/docs
- **OpenAPI JSON**: http://localhost:$PORT/openapi.json

The API documentation is automatically generated from the code annotations.

EOF
  fi

  if has_feature "metrics"; then
      cat >> "$PROJECT_NAME/README.md" << EOF
### Monitoring

#### Metrics
- **Application metrics**: http://localhost:$PORT/metrics
- **Prometheus** (if using Docker Compose): http://localhost:9090
- **Grafana** (if using Docker Compose): http://localhost:3000 (admin/admin)

#### Health Check
\`\`\`bash
curl http://localhost:$PORT/health
\`\`\`

EOF
  fi

  if has_feature "cache"; then
      cat >> "$PROJECT_NAME/README.md" << EOF
### Cache

The service includes caching capabilities:

- **Memory Cache**: In-memory caching for single instance deployments
- **Redis Cache**: Distributed caching for multi-instance deployments

Cache is automatically used for GET requests and invalidated on data modifications.

EOF
  fi

  cat >> "$PROJECT_NAME/README.md" << EOF
## Deployment

### Environment Variables

The service can be configured using environment variables:

- \`ENV\` - Environment (development/production)
- \`PORT\` - HTTP port (overrides config)
- \`LOG_LEVEL\` - Logging level (debug/info/warn/error)

### Docker Deployment

1. Build the image:
\`\`\`bash
docker build -t $PROJECT_NAME .
\`\`\`

2. Run the container:
\`\`\`bash
docker run -p $PORT:$PORT -v \$(pwd)/config.yml:/config.yml $PROJECT_NAME
\`\`\`

### Docker Compose Deployment

\`\`\`bash
docker-compose up -d
\`\`\`

This will start the service along with its dependencies.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run tests and linting
6. Submit a pull request

### Code Style

- Use \`gofmt\` for formatting
- Follow Go naming conventions
- Add comments for exported functions
- Write tests for new features

## License

[Your License Here]

## Support

For support and questions, please [create an issue](https://github.com/your-org/$PROJECT_NAME/issues).
EOF
}

# Генерация .gitignore
generate_gitignore() {
  log_info "Generating .gitignore..."

  cat > "$PROJECT_NAME/.gitignore" << EOF
# Binaries for programs and plugins
*.exe
*.exe~
*.dll
*.so
*.dylib
bin/
dist/

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

# OS files
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

# Environment files
.env
.env.local
.env.*.local

# Cache directories
.cache/
certs/

# Temporary files
tmp/
temp/

# Docker
.dockerignore

# CI/CD
.github/workflows/*.yml.bak
.gitlab-ci.yml.bak

# Application specific
data/
uploads/
downloads/
EOF
}

# Финализация проекта
finalize_project() {
  log_info "Finalizing project..."

  cd "$PROJECT_NAME"

  # Инициализация Go модуля
  go mod init "$PROJECT_NAME" 2>/dev/null || true
  go mod tidy 2>/dev/null || true

  # Форматирование кода
  go fmt ./... 2>/dev/null || true

  cd ..

  log_success "Project $PROJECT_NAME created successfully!"
}

# Показать итоги
show_summary() {
  echo
  log_success "🎉 Project $PROJECT_NAME has been generated!"
  echo
  log_info "📁 Project details:"
  echo "   • Name: $PROJECT_NAME"
  echo "   • Module: $MODULE_NAME"
  echo "   • Port: $PORT"
  echo "   • Template: $TEMPLATE"
  echo "   • Features: $ALL_FEATURES"
  echo "   • Tests: $([ "$INCLUDE_TESTS" == "true" ] && echo "✅ Included" || echo "❌ Not included")"
  echo "   • CI/CD: $CI_TYPE"
  echo

  log_info "🚀 Next steps:"
  echo "   1. cd $PROJECT_NAME"
  echo "   2. go mod tidy"
  echo "   3. make run"
  echo

  log_info "📖 Available commands:"
  echo "   • make run          - Start the service"
  echo "   • make dev          - Run in development mode"
  echo "   • make test         - Run tests"

  if [[ "$INCLUDE_TESTS" == "true" ]]; then
      echo "   • make test-integration - Run integration tests"
  fi

  echo "   • make docker-compose - Start with Docker"
  echo

  log_info "🌐 Service will be available at:"
  echo "   • API: http://localhost:$PORT/api/v1"
  echo "   • Health: http://localhost:$PORT/health"

  if has_feature "docs"; then
      echo "   • Docs: http://localhost:$PORT/docs"
  fi

  if has_feature "metrics"; then
      echo "   • Metrics: http://localhost:$PORT/metrics"
  fi

  echo
  log_info "📚 Documentation:"
  echo "   • README.md - Full documentation"
  echo "   • config.yml - Configuration options"

  if has_feature "docs"; then
      echo "   • /docs endpoint - Interactive API documentation"
  fi

  echo
  log_success "Happy coding! 🚀"
}

# Основная функция
main() {
  echo
  log_info "SAI Service Generator v1.0.0"
  echo

  # Проверка зависимостей
  check_dependencies

  # Парсинг аргументов
  parse_args "$@"

  # Интерактивный режим
  interactive_mode

  # Валидация параметров
  validate_params

  # Настройка фич по шаблону
  setup_template_features

  log_info "Generating project with the following configuration:"
  echo "   • Name: $PROJECT_NAME"
  echo "   • Module: $MODULE_NAME"
  echo "   • Port: $PORT"
  echo "   • Template: $TEMPLATE"
  echo "   • Features: $ALL_FEATURES"
  echo "   • Tests: $([ "$INCLUDE_TESTS" == "true" ] && echo "Yes" || echo "No")"
  echo "   • CI/CD: $CI_TYPE"
  echo

  # Создание проекта
  create_project_structure
  generate_go_mod
  generate_main
  generate_service
  generate_handlers
  generate_models
  generate_config
  generate_makefile
  generate_dockerfile
  generate_docker_compose
  generate_tests
  generate_ci
  generate_readme
  generate_gitignore
  finalize_project

  # Показать итоги
  show_summary
}

# Запуск
main "$@"