# SAI Service Framework

Высокопроизводительный Go-фреймворк для микросервисов, построенный на FastHTTP с комплексными возможностями для создания масштабируемых веб-сервисов и API.

## Возможности

🚀 **Высокая производительность**
- Построен на FastHTTP для максимальной пропускной способности
- Оптимизированное управление памятью и паттерны без аллокаций
- Продвинутая маршрутизация с кэшированием и извлечением параметров

🔧 **Комплексный инструментарий**
- Управление конфигурацией с поддержкой YAML
- Структурированное логирование с Zap
- Сбор метрик (Memory/Prometheus)
- Кэширование (Memory/Redis) с отслеживанием зависимостей
- Проверки здоровья и мониторинг
- Планирование cron-задач
- HTTP-клиент с circuit breaker
- Система middleware с пользовательской сортировкой

📚 **Удобство разработки**
- Автоматическая генерация документации OpenAPI/Swagger
- Генератор проектов для быстрой разработки
- Поддержка горячей перезагрузки
- Комплексные утилиты для тестирования

🛡️ **Готовность к продакшену**
- Поддержка TLS/HTTPS с авто-сертификатами (Let's Encrypt)
- Ограничение скорости и троттлинг запросов
- CORS, сжатие и middleware безопасности
- Graceful shutdown и восстановление после ошибок
- Поддержка WebSocket для real-time функций

## Быстрый старт

### Установка

```bash
go get -u github.com/saiset-co/sai-service
```

### Использование генератора проектов

Самый быстрый способ начать - использовать встроенный генератор проектов:

```bash
# Скачать и запустить генератор
curl -sSL https://raw.githubusercontent.com/saiset-co/sai-service/main/generator.sh | bash

# Или клонировать и запустить локально
git clone https://github.com/saiset-co/sai-service.git
cd sai-service
chmod +x generator.sh
./generator.sh
```

#### Опции генератора

**Интерактивный режим (рекомендуется)**
```bash
./generator.sh
# Следуйте интерактивным подсказкам
```

**Режим командной строки**
```bash
./generator.sh --name "my-api" --template api --features "cache,metrics,docs"
```

**Доступные шаблоны:**
- `basic` - Минимальный HTTP-сервер
- `api` - REST API с базовыми middleware
- `microservice` - Полный микросервис с кэшем, метриками, health checks
- `full` - Все функции включены

**Доступные функции:**
- `cache` - Кэширование Memory/Redis
- `metrics` - Метрики Prometheus/Memory
- `docs` - Документация OpenAPI/Swagger
- `cron` - Планирование задач
- `actions` - WebSocket action broker
- `tls` - HTTPS/авто-сертификаты
- `middleware` - Полный стек middleware
- `health` - Проверки здоровья
- `client` - HTTP-клиент с circuit breaker

**Примеры команд:**
```bash
# Создать простой API-сервис
./generator.sh --name "user-service" --template api --features "cache,metrics,docs"

# Создать полный микросервис
./generator.sh --name "order-service" --template microservice --tests --ci github

# Создать с пользовательским именем модуля
./generator.sh --name "gateway" --module "github.com/company/gateway" --template full
```

### Ручная настройка

Если вы предпочитаете ручную настройку, вот минимальный пример:

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
    
    // Создать сервис
    srv, err := service.NewService(ctx, "config.yml")
    if err != nil {
        log.Fatal(err)
    }
    
    // Зарегистрировать маршруты
    router := sai.Router()
    router.GET("/hello", handleHello).
        WithDoc("Hello World", "Простая конечная точка приветствия", "Demo", nil, nil)
    
    // Запустить сервис
    if err := srv.Run(); err != nil {
        log.Fatal(err)
    }
}

func handleHello(ctx *fasthttp.RequestCtx) {
    ctx.SetContentType("application/json")
    ctx.SetBodyString(`{"message": "Привет, мир!"}`)
}
```

### Конфигурация

Создайте файл `config.yml`:

```yaml
name: "my-service"
version: "1.0.0"

server:
  http:
    host: "0.0.0.0"
    port: 8080

logger:
  level: "info"

# Включите функции по необходимости
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

## Архитектура

### Основные компоненты

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

### Поток запроса

1. **HTTP-запрос** → FastHTTP Server
2. **Маршрутизация** → Сопоставление URL-шаблонов с параметрами
3. **Цепочка Middleware** → Аутентификация, логирование, ограничение скорости и т.д.
4. **Выполнение обработчика** → Бизнес-логика
5. **Ответ** → JSON-сериализация и HTTP-ответ

## Примеры API

### Базовый REST API

```go
// Регистрация маршрутов с документацией
api := router.Group("/api/v1")

api.GET("/users", handlers.GetUsers).
    WithCache("users_list", 300, "users").
    WithDoc("Получить пользователей", "Получить всех пользователей", "Users", 
        models.GetUsersRequest{}, models.UsersResponse{})

api.POST("/users", handlers.CreateUser).
    WithDoc("Создать пользователя", "Создать нового пользователя", "Users",
        models.CreateUserRequest{}, models.UserResponse{})

api.GET("/users/{id}", handlers.GetUser).
    WithCache("user_{id}", 600, "users").
    WithDoc("Получить пользователя", "Получить пользователя по ID", "Users",
        nil, models.UserResponse{})
```

### Продвинутые возможности

**Кэширование с зависимостями**
```go
// Кэш будет инвалидирован при изменении зависимости "users"
api.GET("/users", handler).
    WithCache("users_list", 300, "users", "permissions")
```

**Конфигурация Middleware**
```go
// Пользовательская цепочка middleware
api.POST("/admin/users", handler).
    WithMiddlewares("Auth", "RateLimit").
    WithTimeout(30 * time.Second)
```

**Использование клиента**
```go
// HTTP-клиент с circuit breaker
client, _ := sai.ClientManager().GetClient("user-service")
err := client.Call("POST", "/users", userData, types.CallOptions{
    Timeout: 10 * time.Second,
    Retry:   3,
})
```

## Справочник по конфигурации

### Конфигурация сервера

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

### Конфигурация Middleware

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

### Конфигурация кэша

```yaml
cache:
  enabled: true
  type: "redis"  # или "memory"
  config:
    host: "localhost"
    port: 6379
    password: ""
    db: 0
```

### Конфигурация метрик

```yaml
metrics:
  enabled: true
  type: "prometheus"  # или "memory"
  config:
    path: "/metrics"
    namespace: "myapp"
```

## Мониторинг и наблюдаемость

### Проверки здоровья

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

### Метрики

Посетите `/metrics` для метрик Prometheus или `/stats` для JSON-формата.

### Документация API

Посетите `/docs` для интерактивной документации Swagger UI.

## Продвинутое использование

### Пользовательский Middleware

```go
type CustomMiddleware struct {
    config types.ConfigManager
    logger types.Logger
}

func (m *CustomMiddleware) Name() string { return "custom" }
func (m *CustomMiddleware) Weight() int { return 25 }

func (m *CustomMiddleware) Handle(ctx *fasthttp.RequestCtx, next func(*fasthttp.RequestCtx), config *types.RouteConfig) {
    // Пользовательская логика здесь
    next(ctx)
}

// Регистрация middleware
middleware.RegisterMiddleware("custom", func() types.Middleware {
    return &CustomMiddleware{}
})
```

### Cron-задачи

```go
cron := sai.Cron()
err := cron.Add("cleanup", "0 2 * * *", func() {
    // Логика очистки
    log.Println("Выполнение задачи очистки")
})
```

### WebSocket Actions

```go
actions := sai.Actions()

// Подписка на события
actions.Subscribe("user.created", func(msg *types.ActionMessage) error {
    log.Printf("Пользователь создан: %v", msg.Payload)
    return nil
})

// Публикация событий
actions.Publish("user.created", map[string]interface{}{
    "id": 123,
    "name": "Иван Иванов",
})
```

## Тестирование

### Модульные тесты

```go
func TestGetUser(t *testing.T) {
    // Настройка теста
    service, _ := service.NewService(context.Background(), "test-config.yml")
    
    // Тестовый запрос
    req := fasthttp.AcquireRequest()
    resp := fasthttp.AcquireResponse()
    defer fasthttp.ReleaseRequest(req)
    defer fasthttp.ReleaseResponse(resp)
    
    req.SetRequestURI("http://localhost/api/v1/users/123")
    
    // Выполнение теста
    service.Handler(req, resp)
    
    // Утверждения
    assert.Equal(t, 200, resp.StatusCode())
}
```

### Интеграционные тесты

Сгенерированные проекты включают комплексные интеграционные тесты:

```bash
# Запустить все тесты
make test

# Запустить интеграционные тесты
make test-integration

# Запустить с покрытием
make test-coverage
```

## Развертывание

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

## Производительность

SAI Service оптимизирован для высокой производительности:

- **Пропускная способность**: 100k+ RPS на современном оборудовании
- **Память**: Паттерны без аллокаций где возможно
- **Задержка**: Время отклика менее миллисекунды
- **Масштабируемость**: Горизонтальное масштабирование с балансировщиками нагрузки

### Бенчмарки

```bash
# Запустить бенчмарки
go test -bench=. -benchmem ./...

# Нагрузочное тестирование с hey
hey -n 10000 -c 100 http://localhost:8080/api/v1/users
```

## Участие в разработке

Мы приветствуем вклад в развитие проекта! Пожалуйста, ознакомьтесь с нашим [Руководством по участию](CONTRIBUTING.md) для получения подробной информации.

### Настройка среды разработки

```bash
git clone https://github.com/saiset-co/sai-service.git
cd sai-service
go mod download
make test
```

### Стиль кода

- Следуйте соглашениям именования Go
- Используйте `gofmt` для форматирования
- Добавляйте тесты для новых функций
- Обновляйте документацию

## Лицензия

MIT License - см. файл [LICENSE](LICENSE) для подробностей.

## Поддержка

- 📖 [Документация](https://github.com/saiset-co/sai-service/wiki)
- 🐛 [Трекер проблем](https://github.com/saiset-co/sai-service/issues)
- 💬 [Обсуждения](https://github.com/saiset-co/sai-service/discussions)
- 📧 [Поддержка по email](mailto:support@saiset.co)

## Дорожная карта

- [ ] Поддержка GraphQL
- [ ] Интеграция gRPC
- [ ] Распределенная трассировка
- [ ] Интеграция с service mesh
- [ ] Продвинутая панель мониторинга
- [ ] Поддержка нескольких баз данных

## Детальное описание генератора

Генератор проектов SAI Service - это мощный инструмент для быстрого создания микросервисов с предустановленными конфигурациями и лучшими практиками.

### Возможности генератора

#### Шаблоны проектов

**basic** - Минимальный HTTP-сервер
- Базовый HTTP-сервер на FastHTTP
- Простая конфигурация
- Базовое логирование
- Подходит для простых API или прототипов

**api** - REST API с базовыми middleware
- HTTP-сервер с маршрутизацией
- Middleware для CORS, логирования, восстановления
- Проверки здоровья
- Автоматическая документация OpenAPI
- Подходит для большинства REST API

**microservice** - Полный микросервис
- Все возможности API-шаблона
- Кэширование (Memory/Redis)
- Метрики (Memory/Prometheus)
- HTTP-клиент с circuit breaker
- Подходит для production-микросервисов

**full** - Все функции включены
- Все возможности микросервис-шаблона
- Cron-планировщик
- WebSocket action broker
- TLS/HTTPS с авто-сертификатами
- Полный стек middleware
- Подходит для сложных enterprise-систем

#### Интерактивный режим

Генератор предоставляет удобный интерактивный интерфейс:

```bash
./generator.sh

# Пример интерактивной сессии:
# Welcome to SAI Service Generator!
# 
# Project name: user-management-api
# Go module name [user-management-api]: github.com/company/user-management-api
# Port [8080]: 8081
# Available templates: basic api microservice full
# Select template [api]: microservice
# Available features: cache metrics docs cron actions tls middleware health client
# Enable features (comma-separated): cache,metrics,docs,health
# Include integration tests? [y/N]: y
# Available CI/CD: none github gitlab
# Generate CI/CD files [none]: github
# 
# Configuration:
#    • Project: user-management-api
#    • Module: github.com/company/user-management-api
#    • Port: 8081
#    • Template: microservice
#    • Features: cache,metrics,docs,health
#    • Tests: Yes
#    • CI/CD: github
# 
# Proceed with generation? [Y/n]: y
```

#### Командная строка

Для автоматизации и CI/CD пайплайнов:

```bash
# Основные параметры
./generator.sh \
  --name "order-service" \
  --module "github.com/ecommerce/order-service" \
  --port 8080 \
  --template microservice \
  --features "cache,metrics,docs,health,client" \
  --tests \
  --ci github \
  --non-interactive

# Создание Gateway-сервиса
./generator.sh \
  --name "api-gateway" \
  --template full \
  --port 8080 \
  --features "cache,metrics,docs,tls,middleware,health,client,actions" \
  --tests \
  --ci github

# Простой API
./generator.sh \
  --name "notification-service" \
  --template api \
  --features "docs,health" \
  --port 8082
```

### Структура сгенерированного проекта

```
my-service/
├── cmd/                        # Точка входа приложения
│   └── main.go
├── internal/                   # Приватный код приложения
│   ├── service.go             # Инициализация сервиса
│   ├── handlers/              # HTTP-обработчики
│   │   └── handlers.go
│   └── models/                # Модели данных
│       └── model.go
├── tests/                     # Интеграционные тесты (опционально)
│   ├── integration/
│   └── helpers/
├── .github/workflows/         # GitHub Actions (опционально)
│   └── ci.yml
├── config.yml                # Конфигурация
├── Dockerfile                # Docker-образ
├── docker-compose.yml        # Docker Compose
├── Makefile                  # Команды разработки
├── go.mod                    # Go модуль
├── go.sum                    # Зависимости
├── README.md                 # Документация
└── .gitignore                # Git ignore
```

### Готовые примеры API

Генератор создает полнофункциональный CRUD API для демонстрации:

**GET /api/v1/items** - Получить все элементы
```bash
curl http://localhost:8080/api/v1/items
```

**GET /api/v1/items/{id}** - Получить элемент по ID
```bash
curl http://localhost:8080/api/v1/items/1
```

**POST /api/v1/items** - Создать элемент
```bash
curl -X POST http://localhost:8080/api/v1/items \
  -H "Content-Type: application/json" \
  -d '{"name":"New Item","description":"Description"}'
```

**PUT /api/v1/items/{id}** - Обновить элемент
```bash
curl -X PUT http://localhost:8080/api/v1/items/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Updated Item","status":"inactive"}'
```

**DELETE /api/v1/items/{id}** - Удалить элемент
```bash
curl -X DELETE http://localhost:8080/api/v1/items/1
```

### Команды разработки

Сгенерированные проекты включают удобный Makefile:

```bash
# Сборка и запуск
make build              # Собрать приложение
make run                # Собрать и запустить
make dev                # Запустить в режиме разработки

# Тестирование
make test               # Запустить модульные тесты
make test-integration   # Запустить интеграционные тесты
make test-coverage      # Тесты с покрытием

# Качество кода
make lint               # Запустить линтер
make format             # Форматировать код

# Docker
make docker-build       # Собрать Docker-образ
make docker-run         # Запустить в Docker
make docker-compose     # Запустить с зависимостями

# Очистка
make clean              # Очистить артефакты сборки
```

### CI/CD интеграция

#### GitHub Actions

Генератор создает `.github/workflows/ci.yml`:

```yaml
name: CI
on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    - run: make test
    - run: make lint
    
  integration-test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
    - run: make docker-test
    
  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
    - run: make docker-build
```

#### GitLab CI

Для GitLab создается `.gitlab-ci.yml`:

```yaml
stages:
  - test
  - build
  - deploy

test:
  stage: test
  image: golang:1.21
  script:
    - make test
    - make lint

build:
  stage: build
  script:
    - docker build -t $CI_REGISTRY_IMAGE:$CI_COMMIT_SHA .
```

### Настройка после генерации

1. **Обновите зависимости:**
```bash
cd my-service
go mod tidy
```

2. **Настройте конфигурацию:**
   Отредактируйте `config.yml` под ваши нужды

3. **Запустите сервис:**
```bash
make run
```

4. **Проверьте работу:**
- API: http://localhost:8080/api/v1/items
- Документация: http://localhost:8080/docs
- Здоровье: http://localhost:8080/health
- Метрики: http://localhost:8080/metrics

5. **Начните разработку:**
   Добавьте свои модели в `internal/models/`
   Добавьте обработчики в `internal/handlers/`
   Настройте маршруты в `internal/service.go`

---

**Создано с ❤️ командой SAI**