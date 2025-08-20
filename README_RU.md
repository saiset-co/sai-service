# SAI Service Framework

🚀 **Мощный, готовый к продакшену Go фреймворк для создания высокопроизводительных микросервисов и API**

## Содержание

- [Описание проекта](#-описание-проекта)
- [Быстрый старт](#-быстрый-старт)
- [Ручная установка](#-ручная-установка)
- [Глобальные объекты доступа](#-глобальные-объекты-доступа)
- [Конфигурация](#-конфигурация)
- [Обработка данных и управление ошибками](#-обработка-данных-и-управление-ошибками)
- [Система логирования](#-система-логирования)
- [Базовый CRUD API](#-базовый-crud-api)
- [Аутентификация](#-аутентификация)
- [Система кэширования](#-система-кэширования)
- [Менеджер базы данных](#-менеджер-базы-данных)
- [Промежуточное ПО](#-промежуточное-по)
- [Менеджер документации](#-менеджер-документации)
- [Система клиентов](#-система-клиентов)
- [Система событий](#-система-событий)
- [Веб-хуки](#-веб-хуки)
- [Cron задачи](#-cron-задачи)
- [Менеджер здоровья](#-менеджер-здоровья)
- [Менеджер метрик](#-менеджер-метрик)
- [TLS Менеджер](#-tls-менеджер)

## 📋 Описание проекта

SAI Service Framework - это комплексный, корпоративного уровня Go фреймворк, предназначенный для создания масштабируемых, сопровождаемых и наблюдаемых микросервисов. Фреймворк предоставляет полный набор готовых к продакшену компонентов, которые устраняют шаблонный код и позволяют разработчикам сосредоточиться на бизнес-логике.

### Ключевые особенности:
- **Старт без конфигурации** - Работает из коробки с разумными настройками по умолчанию
- **Модульная архитектура** - Включайте только нужные компоненты
- **Производительность прежде всего** - Построен на FastHTTP для максимальной пропускной способности
- **Легковесная база данных** - Встроенная CloverDB с MongoDB-подобными запросами
- **Готовность к продакшену** - Комплексное логирование, метрики и проверки здоровья
- **Дружелюбность к разработчику** - Интуитивные API и обширная документация
- **Совместимость с sai-storage** - Легкая миграция от легковесной к полноценной БД

## 🚀 Быстрый старт

Самый быстрый способ начать - использовать наш генератор сервисов:

```bash
# Клонируйте репозиторий
git clone <repository-url>
cd sai-service-framework

# Сделайте генератор исполняемым
chmod +x generator.sh

# Запустите интерактивный генератор
./generator.sh

# Следуйте подсказкам для настройки вашего сервиса
```
Больше информации в [ДОКУМЕНТАЦИИ ГЕНЕРАТОРА](./GENERATOR_RU.md)

### Опции генератора

```bash
# Создать базовый API сервис
./generator.sh --name "My API" --features "auth,cache,docs"

# Создать полнофункциональный микросервис
./generator.sh --name "User Service" --features "auth,cache,metrics,cron,actions,health"

# Создать с конкретными конфигурациями
./generator.sh \
  --name "Enterprise API" \
  --features "auth,cache,metrics,docs,tls" \
  --auth "token,basic" \
  --cache "redis" \
  --metrics "prometheus"
```

Структура сгенерированного проекта:
```
my-service/
├── cmd/main.go              # Точка входа
├── internal/
│   ├── handlers.go          # HTTP обработчики
│   └── service.go           # Бизнес-логика
├── .env.example             # Конфигурация
├── go.mod                   # Конфигурация
├── config.template.yml      # Конфигурация
├── docker-compose.yml       # Docker настройка
├── Dockerfile               # Образ контейнера
├── Makefile                 # Команды сборки
└── README.md                # Документация проекта
```

## 🔧 Ручная установка

### Установка

```bash
# Инициализируйте новый Go модуль
go mod init github.com/your-org/your-service

# Добавьте SAI Service Framework
go get github.com/saiset-co/sai-service
```

### Базовая настройка сервиса

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
    
    // Создайте сервис с файлом конфигурации
    svc, err := service.NewService(ctx, "config.yml")
    if err != nil {
        log.Fatal(err)
    }
    
    // Настройте маршруты
    setupRoutes()
    
    // Запустите сервис (неблокирующий)
    if err := svc.Start(); err != nil {
        log.Fatal(err)
    }
}

func setupRoutes() {
    router := sai.Router()
    
    // Базовая конечная точка
    router.GET("/api/v1/hello", func(ctx *types.RequestCtx) {
        ctx.SuccessJSON(map[string]string{
            "message": "Привет, мир!",
            "service": "SAI Service",
        })
    })
    
    // Защищённая конечная точка с кэшем
    router.GET("/api/v1/data", func(ctx *types.RequestCtx) {
        data := map[string]interface{}{
            "timestamp": time.Now(),
            "data":      []string{"элемент1", "элемент2", "элемент3"},
        }
        ctx.SuccessJSON(data)
    }).WithMiddlewares("auth").WithCache("api_data", 5*time.Minute)
}
```

## 🌐 Глобальные объекты доступа

Фреймворк предоставляет удобный глобальный доступ ко всем основным компонентам через пакет `sai`:

### Доступные глобальные объекты

```go
import "github.com/saiset-co/sai-service/sai"

// Основные компоненты
router := sai.Router()           // HTTP роутер
logger := sai.Logger()           // Экземпляр логгера
config := sai.Config()           // Менеджер конфигурации

// Опциональные компоненты (если включены в конфигурации)
cache := sai.Cache()             // Менеджер кэша (паника если отключен)
metrics := sai.Metrics()         // Менеджер метрик (паника если отключен)
cron := sai.Cron()              // Планировщик Cron (паника если отключен)
actions := sai.Actions()         // Брокер событий (паника если отключен)
clientManager := sai.ClientManager() // HTTP клиенты (паника если отключены)

// Пользовательские сервисы (устанавливаются вашим приложением)
sai.Set("database", dbInstance)
sai.Set("emailService", emailSvc)

// Получить пользовательские сервисы
var db *sql.DB
if sai.Load("database", &db) {
    // Использовать базу данных
}

// Проверить существование сервиса
if sai.Has("emailService") {
    emailSvc, _ := sai.Get("emailService")
    // Использовать email сервис
}
```

### Примеры использования

```go
func handleUser(ctx *types.RequestCtx) {
    // Логирование с глобальным логгером
    sai.Logger().Info("Обработка пользовательского запроса",
        zap.String("user_id", ctx.UserValue("user_id").(string)))
    
    // Получить из кэша
    if data, found := sai.Cache().Get("user_data"); found {
        ctx.SuccessJSON(data)
        return
    }
    
    // Получить значение конфигурации
    maxRetries := sai.Config().GetValue("api.max_retries", 3).(int)
    
    // Записать метрики
    counter := sai.Metrics().Counter("api_requests", map[string]string{
        "endpoint": "users",
    })
    counter.Inc()
    
    // Обработать запрос...
}
```

## ⚙️ Конфигурация

### Менеджер конфигурации

Система конфигурации поддерживает YAML файлы с подстановкой переменных среды и типобезопасным доступом:

```go
// Получить всю конфигурацию
config := sai.Config().GetConfig()

// Получить конкретные значения с умолчаниями
dbHost := sai.Config().GetValue("database.host", "localhost")
port := sai.Config().GetValue("server.http.port", 8080)

// Типобезопасное чтение конфигурации
var dbConfig DatabaseConfig
err := sai.Config().GetAs("database", &dbConfig)
```

### Минимальная конфигурация

```yaml
# config.yml - Минимальная рабочая конфигурация
name: "Мой Сервис"
version: "1.0.0"
```

### Полная конфигурация

```yaml
name: "Корпоративный Сервис"           # Название сервиса (обязательно)
version: "2.0.0"                    # Версия сервиса (обязательно)

server:
  http:
    host: "0.0.0.0"                 # Адрес привязки
    port: 8080                      # HTTP порт
    read_timeout: 30                # Таймаут чтения в секундах
    write_timeout: 30               # Таймаут записи в секундах  
    idle_timeout: 120               # Таймаут keep-alive в секундах
    shutdown_timeout: 15            # Таймаут корректного завершения
  tls:
    enabled: true                   # Включить HTTPS
    auto_cert: true                 # Использовать автосертификаты Let's Encrypt
    domains: ["api.example.com"]    # Домены для автосертификатов
    email: "admin@example.com"      # Email для Let's Encrypt
    cert_file: "/path/cert.pem"     # Файл сертификата (ручной)
    key_file: "/path/key.pem"       # Файл приватного ключа (ручной)
    cache_dir: "./certs"            # Директория кэша сертификатов

logger:
  level: "info"                     # Уровень логирования
  type: "default"                   # Тип логгера: default, custom
  config:                           # Конфигурация, специфичная для логгера
    format: "console"               # Формат: console, json
    output: "stdout"                # Вывод: stdout, stderr, file
    file: "/var/log/service.log"    # Путь к файлу лога (если output=file)

auth_providers:                     # Провайдеры аутентификации
  token:                            # Токен-основанная аутентификация
    params:
      token: "ваш-секретный-токен"    # API токен
  basic:                            # Базовая HTTP аутентификация
    params:
      username: "admin"             # Имя пользователя
      password: "безопасный-пароль"   # Пароль

middlewares:                        # Конфигурация промежуточного ПО
  enabled: true                     # Включить систему промежуточного ПО
  recovery:                         # Промежуточное ПО восстановления от паники
    enabled: true                   # Включить восстановление
    weight: 10                      # Порядок выполнения (меньше = раньше)
    params:
      stack_trace: true             # Включить трассировку стека в логи
  logging:                          # Промежуточное ПО логирования запросов
    enabled: true
    weight: 20
    params:
      log_level: "info"             # Уровень логирования для запросов
      log_headers: false            # Логировать заголовки запросов
      log_body: false               # Логировать тело запроса/ответа
  rate_limit:                       # Промежуточное ПО ограничения скорости
    enabled: true
    weight: 30
    params:
      requests_per_minute: 100      # Макс запросов в минуту на IP
  body_limit:                       # Ограничение размера тела запроса
    enabled: true
    weight: 40
    params:
      max_body_size: 10485760       # Макс размер тела в байтах (10MB)
  cors:                             # Cross-Origin Resource Sharing
    enabled: true
    weight: 50
    params:
      AllowedOrigins: ["*"]         # Разрешённые источники
      AllowedMethods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
      AllowedHeaders: ["Content-Type", "Authorization"]
      MaxAge: 86400                 # Длительность кэша preflight
  auth:                             # Промежуточное ПО аутентификации
    enabled: true
    weight: 60
    params:
      token: "ваш-api-токен"       # Токен по умолчанию
  compression:                      # Сжатие ответов
    enabled: true
    weight: 70
    params:
      algorithm: "gzip"             # Алгоритм сжатия
      level: 6                      # Уровень сжатия (1-9)
      threshold: 1024               # Минимальный размер ответа для сжатия
  cache:                            # Промежуточное ПО кэширования ответов
    enabled: true
    weight: 80
    params:
      default_ttl: "5m"             # TTL кэша по умолчанию

cache:                              # Система кэширования
  enabled: true                     # Включить кэширование
  type: "redis"                     # Тип кэша: memory, redis, custom
  default_ttl: "1h"                 # TTL по умолчанию для записей кэша
  config:                           # Конфигурация, специфичная для кэша
    host: "localhost:6379"          # Redis хост:порт
    password: ""                    # Пароль Redis
    db: 0                          # Номер базы данных Redis
    pool_size: 10                  # Размер пула соединений

metrics:                            # Сбор метрик
  enabled: true                     # Включить метрики
  type: "prometheus"                # Тип метрик: memory, prometheus, custom
  prefix: "myservice"               # Префикс метрик
  config:
    namespace: "myservice"          # Пространство имён Prometheus
    subsystem: "api"                # Подсистема Prometheus
  http:                             # HTTP конечная точка метрик
    enabled: true                   # Включить HTTP конечную точку метрик
    path: "/metrics"                # Путь конечной точки метрик
    port: 9090                      # Порт сервера метрик (0 = тот же что и основной)
  collectors:                       # Встроенные коллекторы
    system: true                    # Системные метрики (CPU, память)
    runtime: true                   # Метрики среды выполнения Go
    http: true                      # Метрики HTTP запросов
    cache: true                     # Метрики кэша
    middleware: true                # Метрики промежуточного ПО

health:                             # Система проверки здоровья
  enabled: true                     # Включить проверки здоровья

docs:                               # Документация API
  enabled: true                     # Включить документацию OpenAPI/Swagger
  path: "/docs"                     # Путь конечной точки документации

cron:                               # Планировщик Cron задач
  enabled: true                     # Включить планировщик cron
  timezone: "UTC"                   # Часовой пояс для cron задач

actions:                            # Система событий
  enabled: true                     # Включить систему событий
  broker:                           # Брокер событий
    enabled: true                   # Включить брокер
    type: "websocket"               # Тип брокера: websocket, custom
    config:                         # Конфигурация, специфичная для брокера
      port: 8081                    # Порт WebSocket
  webhooks:                         # Система веб-хуков
    enabled: true                   # Включить веб-хуки
    config:
      max_retries: 3                # Макс повторы доставки веб-хука
      timeout: "30s"                # Таймаут доставки веб-хука

clients:                            # Система HTTP клиентов
  enabled: true                     # Включить HTTP клиенты
  default_timeout: "30s"            # Таймаут запроса по умолчанию
  max_idle_connections: 100         # Макс неактивных соединений
  idle_conn_timeout: "90s"          # Таймаут неактивного соединения
  default_retries: 3                # Количество повторов по умолчанию
  circuit_breaker:                  # Конфигурация автоматического выключателя
    enabled: true                   # Включить автоматический выключатель
    failure_threshold: 5            # Сбои до открытия цепи
    recovery_timeout: "60s"         # Время до попытки восстановления
    half_open_requests: 3           # Запросы в полуоткрытом состоянии
  services:                         # Внешние сервисы
    user_service:                   # Название сервиса
      url: "http://user-service:8080"  # Базовый URL
      auth:                         # Конфигурация аутентификации
        provider: "token"           # Провайдер аутентификации для использования
        payload:
          token: "токен-сервиса"    # Токен аутентификации
      events: ["user.created"]      # События для подписки
```

### Подстановка переменных среды

Файлы конфигурации поддерживают подстановку переменных среды в config.template.yml:

```yaml
database:
  host: "${DB_HOST:localhost}"      # Использовать переменную DB_HOST, по умолчанию localhost
  port: "${DB_PORT:5432}"           # Использовать переменную DB_PORT, по умолчанию 5432
  password: "${DB_PASSWORD}"        # Использовать переменную DB_PASSWORD, обязательно

cache:
  enabled: "${CACHE_ENABLED:true}"  # Использовать переменную CACHE_ENABLED, по умолчанию true
```

## 📊 Обработка данных и управление ошибками

Фреймворк предоставляет удобные методы для обработки HTTP запросов и ответов:

### Методы ответов

```go
func handleSuccess(ctx *types.RequestCtx) {
    // JSON ответ со статусом 200
    data := map[string]interface{}{
        "id":   123,
        "name": "Иван Иванов",
        "active": true,
    }
    ctx.SuccessJSON(data)
}

func handleCustomResponse(ctx *types.RequestCtx) {
    // Пользовательский ответ с заголовками
    htmlData := []byte("<h1>Привет мир</h1>")
    htmlHeader := []byte("text/html; charset=UTF-8")
    ctx.Success(htmlData, htmlHeader)
}

func handlePlainText(ctx *types.RequestCtx) {
    // Ответ в виде простого текста (использует заголовок text/html по умолчанию)
    textData := []byte("Ответ в виде простого текста")
    ctx.Success(textData, nil)
}
```

### Чтение данных запроса

```go
type UserRequest struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"min=0,max=150"`
}

func handleCreateUser(ctx *types.RequestCtx) {
    var req UserRequest
    
    // Прочитать и десериализовать JSON тело запроса
    if err := ctx.ReadJSON(&req); err != nil {
        ctx.Error(err, 400)
        return
    }
    
    // Обработать запрос...
    user := createUser(req)
    ctx.SuccessJSON(user)
}

// Альтернативные методы чтения
func handleAlternativeReading(ctx *types.RequestCtx) {
    // Прочитать сырое тело
    body := ctx.PostBody()
    
    // Ручная десериализация
    var data map[string]interface{}
    if err := ctx.Unmarshal(body, &data); err != nil {
        ctx.Error(err, 400)
        return
    }
    
    // Ручная сериализация
    response, err := ctx.Marshal(data)
    if err != nil {
        ctx.Error(err, 500)
        return
    }
    
    ctx.Success(response, []byte("application/json"))
}
```

### Обработка ошибок

```go
func handleWithErrors(ctx *types.RequestCtx) {
    userID := string(ctx.QueryArgs().Peek("user_id"))
    if userID == "" {
        // Пользовательская ошибка со статусом 400
        ctx.Error(types.NewError("user_id обязателен"), 400)
        return
    }
    
    user, err := getUserByID(userID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            // Ошибка "не найдено"
            ctx.Error(types.NewError("пользователь не найден"), 404)
        } else {
            // Внутренняя ошибка сервера
            ctx.Error(types.WrapError(err, "не удалось получить пользователя"), 500)
        }
        return
    }
    
    ctx.SuccessJSON(user)
}

// Формат ответа ошибки:
// {
//   "error": "Bad Request",
//   "message": "user_id обязателен"
// }
```

### Доступ к контексту запроса

```go
func handleRequestInfo(ctx *types.RequestCtx) {
    // HTTP метод
    method := string(ctx.Method())
    
    // Путь запроса
    path := string(ctx.Path())
    
    // Параметры запроса
    limit := string(ctx.QueryArgs().Peek("limit"))
    
    // Заголовки
    authHeader := string(ctx.Request.Header.Peek("Authorization"))
    
    // Пользовательские значения (установленные промежуточным ПО)
    userID := ctx.UserValue("user_id")
    
    // Установить заголовки ответа
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

## 📝 Система логирования

### Использование встроенного логгера

```go
func useLogger() {
    logger := sai.Logger()
    
    // Базовое логирование
    logger.Debug("Отладочное сообщение")
    logger.Info("Информационное сообщение")
    logger.Warn("Предупреждение")
    logger.Error("Сообщение об ошибке")
    
    // Структурированное логирование с полями
    logger.Info("Пользователь создан",
        zap.String("user_id", "123"),
        zap.String("email", "user@example.com"),
        zap.Duration("processing_time", time.Millisecond*150))
    
    // Логирование ошибки с трассировкой стека
    err := errors.New("что-то пошло не так")
    logger.ErrorWithErrStack("Операция провалилась", err,
        zap.String("operation", "create_user"))
    
    // Пользовательский уровень лога
    logger.Log(zapcore.FatalLevel, "Произошла фатальная ошибка")
}

func handleRequestWithLogging(ctx *types.RequestCtx) {
    requestID := generateRequestID()
    
    sai.Logger().Info("Запрос начат",
        zap.String("request_id", requestID),
        zap.String("method", string(ctx.Method())),
        zap.String("path", string(ctx.Path())))
    
    // Обработать запрос...
    
    sai.Logger().Info("Запрос завершён",
        zap.String("request_id", requestID),
        zap.Int("status", 200))
}
```

### Пользовательская реализация логгера

```go
// Создать пользовательский логгер
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
    // Добавить поле сервиса ко всем логам
    allFields := append(fields, zap.String("service", c.service))
    c.zapLogger.Info(msg, allFields...)
}

func (c *CustomLogger) Error(msg string, fields ...zap.Field) {
    allFields := append(fields, zap.String("service", c.service))
    c.zapLogger.Error(msg, allFields...)
}

// Реализовать другие необходимые методы...

// Зарегистрировать пользовательский логгер
func init() {
    logger.RegisterLogger("custom", func(config interface{}) (types.Logger, error) {
        // Разобрать конфигурацию и создать логгер
        return NewCustomLogger("мой-сервис"), nil
    })
}
```

Конфигурация для пользовательского логгера:
```yaml
logger:
  type: "custom"
  level: "info"
  config:
    service_name: "мой-сервис"
    output_format: "json"
```

## 🎯 Базовый CRUD API

Система промежуточного ПО применяет всё включённое промежуточное ПО к маршрутам по умолчанию. Вы можете отключить конкретное промежуточное ПО для групп или отдельных маршрутов и повторно включить его по необходимости.

### Поведение промежуточного ПО по умолчанию

```go
func setupCRUDAPI() {
    // Всё включённое промежуточное ПО применяется ко всем маршрутам по умолчанию
    router := sai.Router()
    
    // API группа - отключить аутентификацию для публичных конечных точек
    api := router.Group("/api/v1").
        WithoutMiddlewares("auth")  // Отключить аутентификацию для всей группы
    
    // Публичные конечные точки (аутентификация не требуется)
    api.GET("/status", handleStatus)
    api.POST("/register", handleRegister)
    
    // Группа пользователей - повторно включить аутентификацию для защищённых конечных точек
    users := api.Group("/users").
        WithMiddlewares("auth")  // Повторно включить аутентификацию для группы пользователей
    
    users.POST("/", createUser).
        WithDoc("Создать пользователя", "Создаёт нового пользователя", "users", CreateUserRequest{}, User{})
    
    users.GET("/", listUsers).
        WithCache("users_list", 5*time.Minute, "users").
        WithDoc("Список пользователей", "Возвращает постраничный список пользователей", "users", nil, []User{})
    
    users.GET("/{id}", getUser).
        WithDoc("Получить пользователя", "Возвращает пользователя по ID", "users", nil, User{})
    
    users.PUT("/{id}", updateUser).
        WithDoc("Обновить пользователя", "Обновляет существующего пользователя", "users", UpdateUserRequest{}, User{})
    
    users.DELETE("/{id}", deleteUser).
        WithoutMiddlewares("cache").  // Отключить кэш для операций удаления
        WithDoc("Удалить пользователя", "Удаляет пользователя по ID", "users", nil, nil)
        
    // Административные конечные точки - дополнительное промежуточное ПО
    admin := api.Group("/admin").
        WithMiddlewares("auth", "rate_limit").  // Включить аутентификацию и ограничение скорости
        WithTimeout(30 * time.Second)
    
    admin.GET("/stats", getAdminStats)
    admin.POST("/maintenance", enableMaintenance)
}
```

### Реализация CRUD

```go
type User struct {
    ID       string    `json:"id" doc:"Уникальный идентификатор пользователя"`
    Name     string    `json:"name" doc:"Полное имя" validate:"required"`
    Email    string    `json:"email" doc:"Email адрес" validate:"required,email"`
    Active   bool      `json:"active" doc:"Статус аккаунта"`
    Created  time.Time `json:"created" doc:"Метка времени создания"`
    Updated  time.Time `json:"updated" doc:"Метка времени последнего обновления"`
}

type CreateUserRequest struct {
    Name  string `json:"name" validate:"required" doc:"Полное имя пользователя"`
    Email string `json:"email" validate:"required,email" doc:"Email адрес пользователя"`
}

type UpdateUserRequest struct {
    Name   *string `json:"name,omitempty" doc:"Полное имя пользователя"`
    Email  *string `json:"email,omitempty" validate:"omitempty,email" doc:"Email пользователя"`
    Active *bool   `json:"active,omitempty" doc:"Статус активности аккаунта"`
}

type ListUsersRequest struct {
    Page     int    `query:"page" doc:"Номер страницы" example:"1"`
    Limit    int    `query:"limit" doc:"Элементов на странице" example:"20"`
    Search   string `query:"search" doc:"Поисковый запрос"`
    Active   *bool  `query:"active" doc:"Фильтр по статусу активности"`
}

func createUser(ctx *types.RequestCtx) {
    var req CreateUserRequest
    if err := ctx.ReadJSON(&req); err != nil {
        ctx.Error(types.WrapError(err, "неверное тело запроса"), 400)
        return
    }
    
    // Проверить существование пользователя
    if userExists(req.Email) {
        ctx.Error(types.NewError("пользователь с таким email уже существует"), 409)
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
        sai.Logger().Error("Не удалось сохранить пользователя", 
            zap.Error(err),
            zap.String("email", req.Email))
        ctx.Error(types.WrapError(err, "не удалось создать пользователя"), 500)
        return
    }
    
    // Аннулировать кэш
    sai.Cache().Invalidate("users")
    
    // Опубликовать событие
    sai.Actions().Publish("user.created", map[string]interface{}{
        "user_id": user.ID,
        "email":   user.Email,
    })
    
    sai.Logger().Info("Пользователь создан",
        zap.String("user_id", user.ID),
        zap.String("email", user.Email))
    
    ctx.SuccessJSON(user)
}

func listUsers(ctx *types.RequestCtx) {
    var req ListUsersRequest
    
    // Разобрать параметры запроса
    req.Page = parseInt(string(ctx.QueryArgs().Peek("page")), 1)
    req.Limit = parseInt(string(ctx.QueryArgs().Peek("limit")), 20)
    req.Search = string(ctx.QueryArgs().Peek("search"))
    
    if activeStr := string(ctx.QueryArgs().Peek("active")); activeStr != "" {
        if active, err := strconv.ParseBool(activeStr); err == nil {
            req.Active = &active
        }
    }
    
    // Валидировать пагинацию
    if req.Page < 1 {
        req.Page = 1
    }
    if req.Limit < 1 || req.Limit > 100 {
        req.Limit = 20
    }
    
    users, total, err := getUsersList(req)
    if err != nil {
        sai.Logger().Error("Не удалось получить список пользователей", zap.Error(err))
        ctx.Error(types.WrapError(err, "не удалось получить пользователей"), 500)
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
            ctx.Error(types.NewError("пользователь не найден"), 404)
        } else {
            sai.Logger().Error("Не удалось получить пользователя", 
                zap.Error(err),
                zap.String("user_id", userID))
            ctx.Error(types.WrapError(err, "не удалось получить пользователя"), 500)
        }
        return
    }
    
    ctx.SuccessJSON(user)
}

func updateUser(ctx *types.RequestCtx) {
    userID := ctx.UserValue("id").(string)
    
    var req UpdateUserRequest
    if err := ctx.Read(&req); err != nil {
        ctx.Error(types.WrapError(err, "неверное тело запроса"), 400)
        return
    }
    
    user, err := getUserByID(userID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            ctx.Error(types.NewError("пользователь не найден"), 404)
        } else {
            ctx.Error(types.WrapError(err, "не удалось получить пользователя"), 500)
        }
        return
    }
    
    // Обновить поля
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
        sai.Logger().Error("Не удалось обновить пользователя",
            zap.Error(err),
            zap.String("user_id", userID))
        ctx.Error(types.WrapError(err, "не удалось обновить пользователя"), 500)
        return
    }
    
    // Аннулировать кэш
    sai.Cache().Invalidate("users")
    
    // Опубликовать событие
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
            ctx.Error(types.NewError("пользователь не найден"), 404)
        } else {
            sai.Logger().Error("Не удалось удалить пользователя",
                zap.Error(err),
                zap.String("user_id", userID))
            ctx.Error(types.WrapError(err, "не удалось удалить пользователя"), 500)
        }
        return
    }
    
    // Аннулировать кэш
    sai.Cache().Invalidate("users")
    
    // Опубликовать событие
    sai.Actions().Publish("user.deleted", map[string]interface{}{
        "user_id": userID,
    })
    
    ctx.SuccessJSON(map[string]string{
        "message": "пользователь успешно удалён",
    })
}
```

## 🔐 Аутентификация

Фреймворк предоставляет гибкую систему аутентификации с множественными провайдерами и интеграцией промежуточного ПО.

### Встроенные провайдеры аутентификации

Просто описание типа провайдера аутентификации, не включает аутентификацию

#### Токен аутентификация

```yaml
auth_providers:
  token:
    params:
      token: "ваш-секретный-api-токен"
```

```go
func setupTokenAuth() {
    // Токен может быть отправлен несколькими способами:
    // 1. Заголовок Authorization: "Bearer ваш-токен"
    // 2. Заголовок Authorization: "Token ваш-токен"  
    // 3. Заголовок Authorization: "ваш-токен"
    // 4. Заголовок Token: "ваш-токен"
    
    router := sai.Router()
    
    // Защищённая конечная точка
    router.GET("/api/protected", func(ctx *types.RequestCtx) {
        // Информация о пользователе доступна после промежуточного ПО аутентификации
        userInfo := ctx.UserValue("auth_type")  // "token"
        
        ctx.SuccessJSON(map[string]interface{}{
            "message":   "Доступ разрешён",
            "auth_type": userInfo,
        })
    }).WithMiddlewares("auth")
}
```

#### Базовая аутентификация
```yaml
auth_providers:
  basic:
    params:
      username: "admin"
      password: "безопасный-пароль"
```

```go
func setupBasicAuth() {
    router := sai.Router()
    
    router.GET("/api/admin", func(ctx *types.RequestCtx) {
        // Информация о пользователе доступна после аутентификации
        username := ctx.UserValue("authenticated_user").(string)
        authType := ctx.UserValue("auth_type").(string)
        
        ctx.SuccessJSON(map[string]interface{}{
            "message":  "Доступ администратора разрешён",
            "username": username,
            "auth_type": authType,  // "basic"
        })
    }).WithMiddlewares("auth")
}
```

### Пользовательский провайдер аутентификации

```go
// Пользовательский JWT провайдер аутентификации
type JWTAuthProvider struct {
    secretKey []byte
    realm     string
}

func NewJWTAuthProvider(secretKey []byte) *JWTAuthProvider {
    return &JWTAuthProvider{
        secretKey: secretKey,
        realm:     "Защищённая область",
    }
}

func (p *JWTAuthProvider) Type() string {
    return "jwt"
}

func (p *JWTAuthProvider) ApplyToIncomingRequest(ctx *types.RequestCtx) error {
    authHeader := string(ctx.Request.Header.Peek("Authorization"))
    if authHeader == "" {
        return p.sendAuthChallenge(ctx, "Требуется заголовок Authorization")
    }
    
    if !strings.HasPrefix(authHeader, "Bearer ") {
        return p.sendAuthChallenge(ctx, "Требуется Bearer токен")
    }
    
    tokenString := strings.TrimPrefix(authHeader, "Bearer ")
    
    // Разобрать и валидировать JWT токен
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("неожиданный метод подписи")
        }
        return p.secretKey, nil
    })
    
    if err != nil || !token.Valid {
        return p.sendAuthChallenge(ctx, "Неверный токен")
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
        return errors.New("требуется конфигурация аутентификации для JWT")
    }
    
    token, ok := authConfig.Payload["token"].(string)
    if !ok {
        return errors.New("JWT токен не найден в данных аутентификации")
    }
    
    req.Header.Set("Authorization", "Bearer "+token)
    return nil
}

func (p *JWTAuthProvider) sendAuthChallenge(ctx *types.RequestCtx, message string) error {
    ctx.SetStatusCode(fasthttp.StatusUnauthorized)
    ctx.Response.Header.Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s"`, p.realm))
    
    response := map[string]interface{}{
        "error":   "Требуется аутентификация",
        "message": message,
        "type":    "bearer_auth_challenge",
    }
    
    ctx.SuccessJSON(response)
    return errors.New("jwt_auth_challenge_sent")
}

// Зарегистрировать пользовательский провайдер
func setupCustomAuth() {
    authProvider := sai.AuthProvider()
    jwtProvider := NewJWTAuthProvider([]byte("ваш-jwt-секрет"))
    
    authProvider.Register("jwt", jwtProvider)
}
```

### Конфигурация промежуточного ПО аутентификации

Используется для защиты входящих запросов. Включает аутентификацию для всех маршрутов.

```yaml
middlewares:
  auth:
    enabled: true
    weight: 60  # Выполняется после CORS, ограничения скорости и т.д.
    params:
      provider: "token" # Тип провайдера
```

### Управление аутентификацией на уровне маршрутов

```go
func setupAuthRoutes() {
    router := sai.Router()
    
    // Публичные маршруты (без аутентификации)
    public := router.Group("/api/public").
        WithoutMiddlewares("auth")
    
    public.GET("/status", handleStatus)
    public.POST("/register", handleRegister)
    
    // Защищённые маршруты (требуется аутентификация)
    protected := router.Group("/api/protected").
        WithMiddlewares("auth")
    
    protected.GET("/profile", handleProfile)
    protected.PUT("/profile", handleUpdateProfile)
    
    // Административные маршруты (аутентификация + дополнительные проверки)
    admin := router.Group("/api/admin").
        WithMiddlewares("auth")
    
    admin.GET("/users", func(ctx *types.RequestCtx) {
        // Дополнительная проверка авторизации
        claims := ctx.UserValue("user_claims").(jwt.MapClaims)
        role, ok := claims["role"].(string)
        if !ok || role != "admin" {
            ctx.Error(types.NewError("недостаточно прав"), 403)
            return
        }
        
        // Логика администратора...
        ctx.SuccessJSON(map[string]string{"message": "Доступ администратора разрешён"})
    })
}
```

## 💾 Система кэширования

Фреймворк предоставляет гибкую систему кэширования с множественными бэкендами и интеграцией промежуточного ПО.

### Конфигурация кэша

Включает менеджер кэша. Не включает кэш на маршрутах в этом месте.

```yaml
cache:
  enabled: true
  type: "redis"        # memory, redis, custom
  default_ttl: "1h"    # TTL по умолчанию для записей кэша
  config:
    host: "localhost:6379"
    password: ""
    db: 0
    pool_size: 10
    max_retries: 3
    retry_delay: "1s"
```

### Программное использование кэша

```go
func useCacheDirectly() {
    cache := sai.Cache()
    
    // Установить запись кэша
    cache.Set("user:123", userData, 15*time.Minute)
    
    // Получить запись кэша
    if data, found := cache.Get("user:123"); found {
        user := data.(*User)
        // Использовать кэшированные данные
    }
    
    // Удалить конкретный ключ
    cache.Delete("user:123")
    
    // Аннулировать множественные ключи
    cache.Invalidate("users", "user:123", "stats:daily")
    
    // Кэш с зависимостями
    cache.Set("user_stats", statsData, time.Hour)
    // Когда данные пользователя изменяются, аннулировать зависимые кэши
    cache.Invalidate("user_stats")
}

func handleCachedData(ctx *types.RequestCtx) {
    userID := ctx.UserValue("id").(string)
    cacheKey := fmt.Sprintf("user:%s", userID)
    
    // Сначала попробовать кэш
    if userData, found := sai.Cache().Get(cacheKey); found {
        sai.Logger().Debug("Попадание в кэш", zap.String("key", cacheKey))
        ctx.SuccessJSON(userData)
        return
    }
    
    // Промах кэша - получить из базы данных
    user, err := getUserByID(userID)
    if err != nil {
        ctx.Error(err, 500)
        return
    }
    
    // Кэшировать результат
    sai.Cache().Set(cacheKey, user, 10*time.Minute)
    
    sai.Logger().Debug("Промах кэша - данные кэшированы", zap.String("key", cacheKey))
    ctx.SuccessJSON(user)
}
```

### Промежуточное ПО кэширования

Не включает кэш для маршрутов здесь. Позволяет настраивать конфигурацию кэша для каждого маршрута.

```yaml
middlewares:
  cache:
    enabled: true
    weight: 80  # Выполняется поздно в цепочке
    params:
      default_ttl: "5m"
      cache_private: false
      cache_public: true
```

Параметры кэша маршрутов.

```go
func setupCacheMiddleware() {
    router := sai.Router()
    
    // Кэшировать ответ на 5 минут
    router.GET("/api/users", listUsers).
        WithCache("users_list", 5*time.Minute)
    
    // Кэш с зависимостями - аннулируется при изменении пользователей
    router.GET("/api/users/{id}", getUser).
        WithCache("user_detail", 15*time.Minute, "users")
    
    // Динамический ключ кэша
    router.GET("/api/users/{id}/posts", func(ctx *types.RequestCtx) {
        userID := ctx.UserValue("id").(string)
        
        // Ключ кэша будет включать ID пользователя
        posts := getUserPosts(userID)
        ctx.SuccessJSON(posts)
    }).WithCache("user_posts_{id}", 10*time.Minute, "posts", "users")
    
    // Без кэша для этой конечной точки
    router.POST("/api/users", createUser).
        WithoutMiddlewares("cache")
}
```

### Пользовательский провайдер кэша

```go
// Пользовательская реализация кэша
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
        c.logger.Error("Ошибка получения из кэша", zap.Error(err), zap.String("key", key))
        return nil, false
    }
    
    var data interface{}
    if err := json.Unmarshal([]byte(val), &data); err != nil {
        c.logger.Error("Ошибка десериализации кэша", zap.Error(err))
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

// Реализовать другие необходимые методы...

// Зарегистрировать пользовательский провайдер кэша
func init() {
    cache.RegisterCacheManager("redis-cluster", func(config interface{}) (types.CacheManager, error) {
        cfg := config.(map[string]interface{})
        addrs := cfg["addrs"].([]string)
        password := cfg["password"].(string)
        
        return NewRedisClusterCache(addrs, password, sai.Logger()), nil
    })
}
```

Конфигурация для пользовательского кэша:
```yaml
cache:
  enabled: true
  type: "redis-cluster"
  config:
    addrs: ["localhost:7000", "localhost:7001", "localhost:7002"]
    password: ""
```

## 🗄️ Менеджер базы данных

Фреймворк предоставляет легковесный менеджер базы данных с поддержкой CloverDB для небольших микросервисов, где полноценные решения баз данных, такие как sai-storage, могут быть избыточными. Он поддерживает совместимость API с sai-storage для легкой миграции.

### Конфигурация базы данных

```yaml
database:
  enabled: true
  type: "clover"        # clover, memory, или custom
  path: "./data/db"     # Путь к файлу базы данных (для CloverDB)
  name: "myapp"         # Имя базы данных
```

### Поддерживаемые типы баз данных

#### CloverDB (Встроенная NoSQL)
Идеально подходит для малых и средних микросервисов:
```yaml
database:
  enabled: true
  type: "clover"
  path: "./data/myapp.db"
  name: "myapp"
```

#### База данных в памяти
Для тестирования и разработки:
```yaml
database:
  enabled: true
  type: "memory"
  name: "test_db"
```

### Использование базы данных

```go
// Создание документов
createReq := types.CreateDocumentsRequest{
    Collection: "users",
    Data: []interface{}{
        map[string]interface{}{
            "name":  "Иван Иванов",
            "email": "ivan@example.com",
            "age":   30,
        },
    },
}

ids, err := sai.Database().CreateDocuments(ctx, createReq)
if err != nil {
    return err
}

// Чтение документов с MongoDB-подобными фильтрами
readReq := types.ReadDocumentsRequest{
    Collection: "users",
    Filter: map[string]interface{}{
        "age": map[string]interface{}{
            "$gte": 18,
        },
    },
    Limit: 10,
    Skip:  0,
}

documents, total, err := sai.Database().ReadDocuments(ctx, readReq)
if err != nil {
    return err
}

// Обновление документов
updateReq := types.UpdateDocumentsRequest{
    Collection: "users",
    Filter: map[string]interface{}{
        "email": "ivan@example.com",
    },
    Data: map[string]interface{}{
        "$set": map[string]interface{}{
            "age": 31,
        },
    },
    Upsert: false,
}

updated, err := sai.Database().UpdateDocuments(ctx, updateReq)

// Удаление документов
deleteReq := types.DeleteDocumentsRequest{
    Collection: "users",
    Filter: map[string]interface{}{
        "age": map[string]interface{}{
            "$lt": 18,
        },
    },
}

deleted, err := sai.Database().DeleteDocuments(ctx, deleteReq)
```

### MongoDB-подобные операторы запросов

Менеджер базы данных поддерживает привычные операторы запросов MongoDB:

```go
// Операторы сравнения
filter := map[string]interface{}{
    "age": map[string]interface{}{
        "$eq":  25,           // Равно
        "$ne":  25,           // Не равно
        "$gt":  18,           // Больше
        "$gte": 18,           // Больше или равно
        "$lt":  65,           // Меньше
        "$lte": 65,           // Меньше или равно
        "$in":  []int{25, 30, 35}, // В массиве
        "$nin": []int{25, 30},     // Не в массиве
    },
    "status": map[string]interface{}{
        "$exists": true,      // Поле существует
    },
}

// Операторы обновления
update := map[string]interface{}{
    "$set": map[string]interface{}{
        "status": "активен",
        "updated_at": time.Now(),
    },
    "$inc": map[string]interface{}{
        "login_count": 1,
    },
    "$unset": map[string]interface{}{
        "temp_field": "",
    },
}
```

### Управление коллекциями

```go
// Создание коллекции
err := sai.Database().CreateCollection("new_collection")

// Удаление коллекции
err := sai.Database().DropCollection("old_collection")
```

## 🚧 Промежуточное ПО

Фреймворк включает комплексную систему промежуточного ПО со встроенными компонентами и поддержкой пользовательского промежуточного ПО.

### Промежуточное ПО восстановления

Обрабатывает паники:

```yaml
middlewares:
  recovery:
    enabled: true
    weight: 10  # Выполняется первым
    params:
      stack_trace: true      # Включить трассировку стека в логи
      log_panics: true       # Логировать детали паники
      include_request: true  # Включить детали запроса в логи
```

```go
// Промежуточное ПО восстановления автоматически перехватывает паники
func handlePanic(ctx *types.RequestCtx) {
    // Это будет перехвачено промежуточным ПО восстановления
    panic("что-то пошло не так")
    
    // Промежуточное ПО восстановления:
    // 1. Залогирует панику с трассировкой стека
    // 2. Вернёт 500 Internal Server Error
    // 3. Продолжит обработку других запросов
}
```

### Промежуточное ПО логирования

Логирует все HTTP запросы и ответы:

```yaml
middlewares:
  logging:
    enabled: true
    weight: 20
    params:
      log_level: "info"       # Уровень логирования для запросов
      log_headers: false      # Логировать заголовки запросов
      log_body: false         # Логировать тело запроса/ответа
      log_response: true      # Логировать детали ответа
```

### Промежуточное ПО ограничения скорости

Реализует ограничение скорости по IP адресу:

```yaml
middlewares:
  rate_limit:
    enabled: true
    weight: 30
    params:
      requests_per_minute: 100  # Макс запросов в минуту на IP
      burst: 10                 # Ёмкость всплеска
      cleanup_interval: "1m"    # Интервал очистки старых записей
```

```go
// Ограничение скорости применяется автоматически
// Возвращает 429 Too Many Requests при превышении лимита
func setupRateLimiting() {
    router := sai.Router()
    
    // Разные ограничения скорости для разных конечных точек
    router.GET("/api/public", handlePublic).
        WithoutMiddlewares("rate_limit")  // Без ограничения скорости
    
    router.POST("/api/upload", handleUpload).
        WithMiddlewares("rate_limit")     // Применить ограничение скорости
}
```

### Промежуточное ПО ограничения размера тела

Ограничивает размер тела запроса:

```yaml
middlewares:
  body_limit:
    enabled: true
    weight: 40
    params:
      max_body_size: 10485760  # 10MB в байтах
      skip_content_length: false
```

### CORS промежуточное ПО

Обрабатывает Cross-Origin Resource Sharing:

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
      MaxAge: 86400  # Длительность кэша preflight в секундах
```

### Промежуточное ПО сжатия

Сжимает HTTP ответы:

```yaml
middlewares:
  compression:
    enabled: true
    weight: 70
    params:
      algorithm: "gzip"       # Алгоритм сжатия
      level: 6                # Уровень сжатия (1-9)
      threshold: 1024         # Минимальный размер ответа для сжатия
      allowed_types:          # Типы контента для сжатия
        - "application/json"
        - "text/html"
        - "text/plain"
        - "application/xml"
      exclude_extensions: [".jpg", ".png", ".gif"]
```

### Пользовательское промежуточное ПО

```go
// Промежуточное ПО ID запроса
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
    return 5  // Выполняется очень рано
}

func (m *RequestIDMiddleware) Handle(ctx *types.RequestCtx, next func(*types.RequestCtx), config *types.RouteConfig) {
    // Сгенерировать ID запроса
    requestID := generateRequestID()
    
    // Сохранить в контексте
    ctx.SetUserValue("request_id", requestID)
    
    // Добавить в заголовки ответа
    ctx.Response.Header.Set("X-Request-ID", requestID)
    
    m.logger.Debug("Запрос начат",
        zap.String("request_id", requestID),
        zap.String("method", string(ctx.Method())),
        zap.String("path", string(ctx.Path())))
    
    start := time.Now()
    
    // Перейти к следующему промежуточному ПО
    next(ctx)
    
    duration := time.Since(start)
    statusCode := ctx.Response.StatusCode()
    
    m.logger.Info("Запрос завершён",
        zap.String("request_id", requestID),
        zap.Int("status", statusCode),
        zap.Duration("duration", duration))
}

// Зарегистрировать промежуточное ПО (до запуска сервиса)
func registerCustomMiddleware() {
    middlewareManager := getMiddlewareManager() // Получить из инициализации сервиса
    middlewareManager.Register(NewRequestIDMiddleware(sai.Logger()))
}
```

## 📚 Менеджер документации

### Автоматическая генерация документации

```go
func setupDocumentedAPI() {
    api := sai.Router().Group("/api/v1")
    
    // Документировать с типами запроса/ответа
    api.POST("/users", createUser).
        WithDoc(
            "Создать пользователя",                    // Заголовок
            "Создаёт новый аккаунт пользователя",     // Описание
            "users",                         // Тег для группировки
            CreateUserRequest{},             // Тип тела запроса
            User{},                          // Тип ответа
        )
    
    // Документировать с параметрами запроса
    api.GET("/users", listUsers).
        WithDoc(
            "Список пользователей",
            "Возвращает постраничный список пользователей с опциональной фильтрацией",
            "users",
            ListUsersQuery{},  // Тип параметров запроса
            UserListResponse{}, // Тип ответа
        )
    
    // Документировать параметры пути
    api.GET("/users/{id}", getUser).
        WithDoc(
            "Получить пользователя",
            "Возвращает детали пользователя по ID",
            "users",
            nil,    // Нет тела запроса
            User{}, // Тип ответа
        )
}
```

### Документация с тегами структур

```go
type CreateUserRequest struct {
    Name     string `json:"name" validate:"required" doc:"Полное имя пользователя" example:"Иван Иванов"`
    Email    string `json:"email" validate:"required,email" doc:"Email адрес пользователя" example:"ivan@example.com"`
    Age      int    `json:"age" validate:"min=0,max=150" doc:"Возраст пользователя" example:"30"`
    Active   bool   `json:"active" doc:"Активен ли аккаунт пользователя" example:"true"`
    Tags     []string `json:"tags" doc:"Теги пользователя" example:"admin,premium"`
    Metadata map[string]interface{} `json:"metadata" doc:"Дополнительные метаданные пользователя"`
}

type User struct {
    ID       string    `json:"id" doc:"Уникальный идентификатор пользователя" example:"usr_123456"`
    Name     string    `json:"name" doc:"Полное имя пользователя"`
    Email    string    `json:"email" doc:"Email адрес пользователя"`
    Age      int       `json:"age" doc:"Возраст пользователя"`
    Active   bool      `json:"active" doc:"Статус аккаунта"`
    Created  time.Time `json:"created" doc:"Метка времени создания аккаунта"`
    Updated  time.Time `json:"updated" doc:"Метка времени последнего обновления"`
}

type UserListResponse struct {
    Users      []User `json:"users" doc:"Список пользователей"`
    Total      int    `json:"total" doc:"Общее количество пользователей"`
    Page       int    `json:"page" doc:"Номер текущей страницы"`
    Limit      int    `json:"limit" doc:"Элементов на странице"`
    TotalPages int    `json:"total_pages" doc:"Общее количество страниц"`
}

type ListUsersQuery struct {
    Page   int    `query:"page" doc:"Номер страницы для пагинации" example:"1"`
    Limit  int    `query:"limit" doc:"Количество элементов на странице" example:"20"`
    Search string `query:"search" doc:"Поисковый запрос для фильтрации пользователей" example:"иван"`
    Active *bool  `query:"active" doc:"Фильтр по статусу аккаунта" example:"true"`
}
```

### Доступ к документации

После настройки документация автоматически доступна по адресам:
- `/docs` - интерфейс Swagger UI, см. раздел конфигурации
- `/openapi.json` - спецификация OpenAPI в формате JSON

Документация включает:
- Все задокументированные конечные точки
- Схемы запросов/ответов
- Описания параметров
- Примеры значений
- Требования к аутентификации
- Ответы об ошибках

## 🌐 Система клиентов

Фреймворк предоставляет надёжную систему HTTP клиентов с автоматическими выключателями, повторами и обнаружением сервисов.

### Конфигурация

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
          token: "токен-сервис-к-сервису"
      events: ["user.created", "user.updated"]
    notification_service:
      url: "http://notification-service:8080"
      auth:
        provider: "basic"
        payload:
          username: "service"
          password: "secret"
```

### Использование HTTP клиентов

```go
func useHTTPClients(ctx *types.RequestCtx) {
    clientManager := sai.ClientManager()
    
    // Простой GET запрос
    response, statusCode, err := clientManager.Call(
        "user_service",           // Название сервиса
        "GET",                    // HTTP метод
        "/api/v1/users/123",      // Путь
        nil,                      // Тело запроса
        nil,                      // Опции
    )
    
    if err != nil {
        sai.Logger().Error("Не удалось вызвать пользовательский сервис", zap.Error(err))
        return
    }
    
    if statusCode == 200 {
        var user User
        ctx.Unmarshal(response, &user)
        // Использовать данные пользователя
    }
}

func callWithOptions(ctx *types.RequestCtx) {
    clientManager := sai.ClientManager()
    
    // POST запрос с пользовательскими опциями
    requestData := map[string]interface{}{
        "name":  "Иван Иванов",
        "email": "ivan@example.com",
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
        // Обработать ошибку (может быть сетевая, таймаут или HTTP ошибка)
        sai.Logger().Error("Создание пользователя провалилось",
            zap.Error(err),
            zap.Int("status_code", statusCode))
        return
    }
    
    if statusCode == 201 {
        var newUser User
        ctx.Unmarshal(response, &newUser)
        // Пользователь успешно создан
    }
}
```

### Автоматический выключатель

Клиентская система включает автоматическую функциональность автоматического выключателя:

```go
func handleCircuitBreaker() {
    // Состояния автоматического выключателя:
    // 1. Закрыт: Нормальная работа
    // 2. Открыт: Сервис недоступен, запросы быстро завершаются с ошибкой
    // 3. Полуоткрыт: Тестирование восстановления сервиса
    
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
                sai.Logger().Warn("Автоматический выключатель открыт для unreliable_service")
                // Реализовать резервную логику
                handleFallback()
                continue
            }
            // Обработать другие ошибки
        }
        
        // Обработать успешный ответ
        handleResponse(response, statusCode)
    }
}

func handleFallback() {
    // Реализовать резервную логику когда сервис недоступен
    // - Вернуть кэшированные данные
    // - Использовать альтернативный сервис
    // - Вернуть ответ по умолчанию
}
```

## 🔄 Система событий

Фреймворк предоставляет мощную систему событий, поддерживающую WebSocket и пользовательских брокеров.

### Конфигурация

```yaml
actions:
  enabled: true
  broker:
    enabled: true
    type: "websocket"
    config:
      port: 8081              # Порт WebSocket сервера
      path: "/ws"             # Путь конечной точки WebSocket
      max_connections: 1000   # Максимум одновременных соединений
      read_buffer_size: 1024  # Размер буфера чтения
      write_buffer_size: 1024 # Размер буфера записи
  webhooks:
    enabled: true
    config:
      max_retries: 3
      timeout: "30s"
```

### Публикация событий

```go
func publishEvents() {
    actions := sai.Actions()
    
    // Простое событие
    err := actions.Publish("user.created", map[string]interface{}{
        "user_id": "123",
        "email":   "user@example.com",
        "timestamp": time.Now(),
    })
    
    if err != nil {
        sai.Logger().Error("Не удалось опубликовать событие", zap.Error(err))
    }
    
    // Сложное событие с метаданными
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

// Публикация из HTTP обработчиков
func handleCreateOrder(ctx *types.RequestCtx) {
    var req CreateOrderRequest
    if err := ctx.ReadJSON(&req); err != nil {
        ctx.Error(err, 400)
        return
    }
    
    // Обработать заказ
    order, err := processOrder(req)
    if err != nil {
        ctx.Error(err, 500)
        return
    }
    
    // Опубликовать событие асинхронно
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

### Подписка на события

```go
func setupEventHandlers() {
    actions := sai.Actions()
    
    // Подписаться на события пользователей
    actions.Subscribe("user.created", handleUserCreated)
    actions.Subscribe("user.updated", handleUserUpdated)
    actions.Subscribe("user.deleted", handleUserDeleted)
    
    // Подписаться на события заказов
    actions.Subscribe("order.created", handleOrderCreated)
    actions.Subscribe("order.completed", handleOrderCompleted)
    actions.Subscribe("order.cancelled", handleOrderCancelled)
}

func handleUserCreated(msg *types.ActionMessage) error {
    sai.Logger().Info("Получено событие создания пользователя",
        zap.String("action", msg.Action),
        zap.Time("timestamp", msg.Timestamp))
    
    // Извлечь данные пользователя
    userData := msg.Payload.(map[string]interface{})
    userID := userData["user_id"].(string)
    email := userData["email"].(string)
    
    // Отправить приветственное письмо
    if err := sendWelcomeEmail(userID, email); err != nil {
        sai.Logger().Error("Не удалось отправить приветственное письмо",
            zap.Error(err),
            zap.String("user_id", userID))
        return err
    }
    
    // Обновить аналитику
    updateUserMetrics("created")
    
    // Кэшировать данные пользователя
    sai.Cache().Set(fmt.Sprintf("user:%s", userID), userData, time.Hour)
    
    return nil
}

func handleOrderCompleted(msg *types.ActionMessage) error {
    orderData := msg.Payload.(map[string]interface{})
    orderID := orderData["order_id"].(string)
    customerID := orderData["customer_id"].(string)
    
    // Сгенерировать счёт
    if err := generateInvoice(orderID); err != nil {
        return err
    }
    
    // Обновить инвентарь
    if err := updateInventory(orderData); err != nil {
        return err
    }
    
    // Отправить подтверждение по email
    if err := sendOrderConfirmation(customerID, orderID); err != nil {
        return err
    }
    
    // Запустить выполнение заказа
    sai.Actions().Publish("fulfillment.requested", map[string]interface{}{
        "order_id":    orderID,
        "customer_id": customerID,
        "priority":    "normal",
    })
    
    return nil
}
```

### Пользовательский брокер событий

```go
// Пользовательский брокер событий на основе Redis
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
        logger.Error("Не удалось разобрать Redis URL", zap.Error(err))
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
    // Запустить горутину обработки сообщений
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
        // Первый подписчик на это действие - запустить подписку Redis
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
        b.logger.Error("Не удалось десериализовать сообщение", zap.Error(err))
        return
    }
    
    b.mu.RLock()
    handlers := b.subscribers[action]
    b.mu.RUnlock()
    
    for _, handler := range handlers {
        go func(h types.ActionHandler) {
            if err := h(&message); err != nil {
                b.logger.Error("Обработчик событий провалился",
                    zap.String("action", action),
                    zap.Error(err))
            }
        }(handler)
    }
}

// Зарегистрировать пользовательский брокер
func init() {
    action.RegisterActionBroker("redis", func(config interface{}) (types.ActionBroker, error) {
        cfg := config.(map[string]interface{})
        redisURL := cfg["url"].(string)
        
        return NewRedisEventBroker(redisURL, sai.Logger()), nil
    })
}
```

Конфигурация для пользовательского брокера:
```yaml
actions:
  broker:
    enabled: true
    type: "redis"
    config:
      url: "redis://localhost:6379/0"
```

## 🔗 Веб-хуки

Фреймворк предоставляет комплексную систему веб-хуков для получения и управления веб-хуками.

### Конфигурация

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

### API управления веб-хуками

Фреймворк автоматически предоставляет конечные точки управления веб-хуками:

```bash
# Создать веб-хук
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

# Список веб-хуков
GET /api/webhooks

# Получить конкретный веб-хук
GET /api/webhooks/{webhook_id}

# Обновить веб-хук
PUT /api/webhooks/{webhook_id}
{
  "enabled": false
}

# Удалить веб-хук
DELETE /api/webhooks/{webhook_id}
```

### Автоматическое создание веб-хука

Если список событий предоставлен в разделе клиента:

```yaml
services:
    user_service:
      url: "http://user-service:8080"
      auth:
        provider: "token"
        payload:
          token: "токен-сервис-к-сервису"
      events: ["user.created", "user.updated"]
```

Сервис автоматически создаёт веб-хук когда ваши учётные данные аутентификации корректны. Всё что вам нужно сделать теперь - это подписаться.

### Получение веб-хуков

```go
func setupWebhookHandlers() {
    actions := sai.Actions()
    
    // Обработать входящие веб-хуки от внешних сервисов
    actions.Subscribe("external.payment.completed", handlePaymentWebhook)
    actions.Subscribe("external.user.verification", handleVerificationWebhook)
}

func handlePaymentWebhook(msg *types.ActionMessage) error {
    sai.Logger().Info("Получен веб-хук платежа",
        zap.String("source", msg.Source),
        zap.Time("timestamp", msg.Timestamp))
    
    // Проверить подлинность веб-хука
    if msg.Source != "webhook" {
        return types.NewError("неверный источник веб-хука")
    }
    
    // Извлечь данные платежа
    paymentData := msg.Payload.(map[string]interface{})
    paymentID := paymentData["payment_id"].(string)
    status := paymentData["status"].(string)
    
    // Обновить статус платежа в базе данных
    if err := updatePaymentStatus(paymentID, status); err != nil {
        return err
    }
    
    // Опубликовать внутреннее событие
    sai.Actions().Publish("payment.status.updated", map[string]interface{}{
        "payment_id": paymentID,
        "status":     status,
        "updated_at": time.Now(),
    })
    
    return nil
}
```

### Безопасность веб-хуков

```go
func verifyWebhookSignature(payload []byte, signature, secret string) bool {
    // Проверка HMAC SHA256
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
    // Формат подписи Stripe: t=timestamp,v1=signature
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
    
    // Проверить допустимость временной метки
    ts, err := strconv.ParseInt(timestamp, 10, 64)
    if err != nil {
        return false
    }
    
    if time.Since(time.Unix(ts, 0)) > 5*time.Minute {
        return false
    }
    
    // Проверить подпись
    signedPayload := timestamp + "." + string(payload)
    return verifyWebhookSignature([]byte(signedPayload), sig, secret)
}
```

## ⏰ Cron задачи

Фреймворк предоставляет надёжный планировщик cron задач с мониторингом и обработкой ошибок.

### Конфигурация

```yaml
cron:
  enabled: true
  timezone: "UTC"  # или "Europe/Moscow", "America/New_York" и т.д.
```

### Базовые Cron задачи

```go
func setupCronJobs() {
    cron := sai.Cron()
    
    // Ежедневная очистка в 2:00 утра
    cron.Add("daily_cleanup", "0 2 * * *", func() {
        sai.Logger().Info("Начинаем ежедневную очистку")
        
        if err := cleanupExpiredSessions(); err != nil {
            sai.Logger().Error("Очистка сессий провалилась", zap.Error(err))
        }
        
        if err := cleanupOldLogs(); err != nil {
            sai.Logger().Error("Очистка логов провалилась", zap.Error(err))
        }
        
        sai.Logger().Info("Ежедневная очистка завершена")
    })
    
    // Проверка здоровья каждые 5 минут
    cron.Add("health_check", "*/5 * * * *", func() {
        if err := performSystemHealthCheck(); err != nil {
            sai.Logger().Error("Проверка здоровья провалилась", zap.Error(err))
            
            // Отправить уведомление
            sai.Actions().Publish("system.health.critical", map[string]interface{}{
                "error":     err.Error(),
                "timestamp": time.Now(),
            })
        }
    })
    
    // Генерировать отчёты каждый понедельник в 9:00 утра
    cron.Add("weekly_report", "0 9 * * 1", func() {
        sai.Logger().Info("Генерируем недельный отчёт")
        
        report, err := generateWeeklyReport()
        if err != nil {
            sai.Logger().Error("Генерация отчёта провалилась", zap.Error(err))
            return
        }
        
        if err := emailReport(report); err != nil {
            sai.Logger().Error("Не удалось отправить отчёт по email", zap.Error(err))
        }
        
        sai.Logger().Info("Недельный отчёт сгенерирован и отправлен")
    })
    
    // Прогрев кэша каждый час
    cron.Add("cache_warming", "0 * * * *", func() {
        warmupCaches()
    })
    
    // Сбор метрик каждую минуту
    cron.Add("metrics_collection", "* * * * *", func() {
        collectCustomMetrics()
    })
}
```

### Продвинутые Cron задачи

```go
func setupAdvancedCronJobs() {
    cron := sai.Cron()
    
    // Резервное копирование базы данных каждый день в 3:00 утра
    cron.Add("db_backup", "0 3 * * *", func() {
        backupDatabase()
    })
    
    // Обработка ожидающих писем каждые 2 минуты
    cron.Add("email_processor", "*/2 * * * *", func() {
        processEmailQueue()
    })
    
    // Очистка временных файлов каждые 6 часов
    cron.Add("temp_cleanup", "0 */6 * * *", func() {
        cleanupTempFiles()
    })
    
    // Обновление валютных курсов ежедневно в полночь
    cron.Add("exchange_rates", "0 0 * * *", func() {
        updateExchangeRates()
    })
    
    // Генерация миниатюр для новых изображений каждые 30 секунд
    cron.Add("thumbnail_generator", "*/30 * * * * *", func() {
        generatePendingThumbnails()
    })
}

func backupDatabase() {
    sai.Logger().Info("Начинаем резервное копирование базы данных")
    
    // Создать имя файла резервной копии с временной меткой
    timestamp := time.Now().Format("20060102_150405")
    backupFile := fmt.Sprintf("/backups/db_backup_%s.sql", timestamp)
    
    // Выполнить резервное копирование
    if err := createDatabaseBackup(backupFile); err != nil {
        sai.Logger().Error("Резервное копирование базы данных провалилось", zap.Error(err))
        
        // Отправить уведомление
        sai.Actions().Publish("backup.failed", map[string]interface{}{
            "type":      "database",
            ""file":      backupFile,
            "error":     err.Error(),
            "timestamp": time.Now(),
        })
        return
    }
    
    // Загрузить в облачное хранилище
    if err := uploadToCloud(backupFile); err != nil {
        sai.Logger().Error("Загрузка резервной копии провалилась", zap.Error(err))
    }
    
    // Очистить старые резервные копии (сохранить последние 7 дней)
    cleanupOldBackups(7)
    
    sai.Logger().Info("Резервное копирование базы данных завершено", zap.String("file", backupFile))
}

func processEmailQueue() {
    emails, err := getPendingEmails(100) // Получить до 100 ожидающих писем
    if err != nil {
        sai.Logger().Error("Не удалось получить ожидающие письма", zap.Error(err))
        return
    }
    
    if len(emails) == 0 {
        return // Нет писем для обработки
    }
    
    sai.Logger().Info("Обработка очереди писем", zap.Int("count", len(emails)))
    
    for _, email := range emails {
        if err := sendEmail(email); err != nil {
            sai.Logger().Error("Не удалось отправить письмо",
                zap.Error(err),
                zap.String("email_id", email.ID))
            
            // Отметить как провалившееся и повторить позже
            markEmailFailed(email.ID, err.Error())
        } else {
            // Отметить как отправленное
            markEmailSent(email.ID)
        }
    }
}

func generatePendingThumbnails() {
    images, err := getImagesNeedingThumbnails(50)
    if err != nil {
        sai.Logger().Error("Не удалось получить изображения, требующие миниатюр", zap.Error(err))
        return
    }
    
    if len(images) == 0 {
        return
    }
    
    for _, image := range images {
        if err := generateThumbnail(image); err != nil {
            sai.Logger().Error("Генерация миниатюры провалилась",
                zap.Error(err),
                zap.String("image_id", image.ID))
        } else {
            markThumbnailGenerated(image.ID)
        }
    }
}
```

### Примеры Cron выражений

```go
// Формат cron выражений: секунда минута час день месяц деньНедели
// (секунды опциональны - используйте 5 полей для точности до минуты)

var cronExamples = map[string]string{
    // Каждую минуту
    "* * * * *": "каждую минуту",
    
    // Каждые 5 минут
    "*/5 * * * *": "каждые 5 минут",
    
    // Каждый час на 30-й минуте
    "30 * * * *": "каждый час на 30-й минуте",
    
    // Каждый день в 2:30 утра
    "30 2 * * *": "каждый день в 2:30 утра",
    
    // Каждый понедельник в 9:00 утра
    "0 9 * * 1": "каждый понедельник в 9:00 утра",
    
    // Каждый рабочий день в 6:00 вечера
    "0 18 * * 1-5": "каждый рабочий день в 6:00 вечера",
    
    // Первый день каждого месяца в полночь
    "0 0 1 * *": "первый день каждого месяца в полночь",
    
    // Каждые 30 секунд (6-польный формат)
    "*/30 * * * * *": "каждые 30 секунд",
    
    // Каждые четверть часа
    "0 */15 * * *": "каждые четверть часа",
    
    // Дважды в день (8 утра и 8 вечера)
    "0 8,20 * * *": "дважды в день в 8 утра и 8 вечера",
}
```

## ❤️ Менеджер здоровья

Фреймворк предоставляет комплексный мониторинг здоровья со встроенными и пользовательскими проверками здоровья.

### Конфигурация

```yaml
health:
  enabled: true
```

### Встроенные конечные точки здоровья

- `GET /health` - Комплексный отчёт о здоровье
- `GET /version` - Версия сервиса и информация о сборке

### Встроенные проверки здоровья

```go
func setupHealthChecks() {
    health := sai.Health()
    
    // Проверка здоровья базы данных
    health.RegisterChecker("database", func(ctx context.Context) types.HealthCheck {
        // Проверить подключение к базе данных
        if err := db.PingContext(ctx); err != nil {
            return types.HealthCheck{
                Status:  types.StatusUnhealthy,
                Message: "Срок действия лицензии истёк",
                Details: map[string]interface{}{
                    "expired_at": license.ExpiresAt,
                    "days_expired": int(time.Since(license.ExpiresAt).Hours() / 24),
                },
            }
        }
        
        daysUntilExpiry := int(time.Until(license.ExpiresAt).Hours() / 24)
        
        status := types.StatusHealthy
        message := "Лицензия действительна"
        
        if daysUntilExpiry <= 7 {
            status = types.StatusUnhealthy
            message = "Срок действия лицензии скоро истекает"
        } else if daysUntilExpiry <= 30 {
            message = "Срок действия лицензии истекает в течение 30 дней"
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
    
    // Проверить сервис флагов функций
    health.RegisterChecker("feature_flags", func(ctx context.Context) types.HealthCheck {
        start := time.Now()
        flags, err := getFeatureFlags()
        responseTime := time.Since(start)
        
        if err != nil {
            return types.HealthCheck{
                Status:  types.StatusUnhealthy,
                Message: "Сервис флагов функций недоступен",
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
            Message: "Сервис флагов функций работает",
            Details: map[string]interface{}{
                "flags_count":      len(flags),
                "response_time_ms": responseTime.Milliseconds(),
            },
        }
    })
}
```

### Формат ответа проверки здоровья

```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "uptime": "72h15m30s",
  "service": {
    "name": "Пользовательский Сервис",
    "version": "2.1.0",
    "host": "api.example.com",
    "port": 8080
  },
  "checks": {
    "database": {
      "status": "healthy",
      "message": "База данных работает",
      "last_check": "2024-01-15T10:30:00Z",
      "duration": "15ms",
      "details": {
        "query_time_ms": 12,
        "connections": 5
      }
    },
    "redis": {
      "status": "healthy",
      "message": "Redis работает",
      "last_check": "2024-01-15T10:30:00Z",
      "duration": "8ms",
      "details": {
        "ping_time_ms": 5,
        "memory_usage": "45MB"
      }
    },
    "user_service": {
      "status": "unhealthy",
      "message": "Пользовательский сервис вернул 503",
      "last_check": "2024-01-15T10:30:00Z",
      "duration": "5s",
      "details": {
        "status_code": 503,
        "error": "Сервис временно недоступен"
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

### Использование данных здоровья

```go
func monitorHealth() {
    health := sai.Health()
    
    // Получить текущий статус здоровья
    report := health.Check(context.Background())
    
    if report.Status != types.StatusHealthy {
        sai.Logger().Error("Проверка здоровья сервиса провалилась",
            zap.String("overall_status", string(report.Status)),
            zap.Int("unhealthy_checks", report.Summary.Unhealthy))
        
        // Отправить уведомление
        sendHealthAlert(report)
    }
    
    // Залогировать метрики здоровья
    for name, check := range report.Checks {
        sai.Logger().Debug("Результат проверки здоровья",
            zap.String("check", name),
            zap.String("status", string(check.Status)),
            zap.Duration("duration", check.Duration))
    }
}

func sendHealthAlert(report types.HealthReport) {
    // Найти провалившиеся проверки
    var failedChecks []string
    for name, check := range report.Checks {
        if check.Status == types.StatusUnhealthy {
            failedChecks = append(failedChecks, name)
        }
    }
    
    // Отправить уведомление
    sai.Actions().Publish("health.alert", map[string]interface{}{
        "service":       report.Service.Name,
        "status":        report.Status,
        "failed_checks": failedChecks,
        "timestamp":     report.Timestamp,
        "uptime":        report.Uptime.String(),
    })
}
```

## 📊 Менеджер метрик

Фреймворк предоставляет комплексный сбор метрик с поддержкой Prometheus и пользовательских провайдеров.

### Конфигурация

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
    port: 9090  # 0 = тот же порт что и основной сервер
  collectors:
    system: true      # Метрики CPU, памяти, диска
    runtime: true     # Метрики среды выполнения Go
    http: true        # Метрики HTTP запросов
    cache: true       # Метрики операций кэша
    middleware: true  # Метрики промежуточного ПО
```

### Встроенные метрики

Фреймворк автоматически собирает следующие метрики:

#### HTTP метрики
- `http_requests_total` - Общее количество HTTP запросов
- `http_request_duration_seconds` - Гистограмма длительности запросов
- `http_request_size_bytes` - Гистограмма размера запросов
- `http_response_size_bytes` - Гистограмма размера ответов

#### Системные метрики
- `system_cpu_usage` - Процент использования CPU
- `system_memory_usage_bytes` - Использование памяти
- `system_disk_usage_bytes` - Использование диска
- `system_load_average` - Средняя нагрузка системы

#### Метрики среды выполнения
- `go_goroutines` - Количество горутин
- `go_threads` - Количество OS потоков
- `go_gc_duration_seconds` - Длительность GC
- `go_memstats_*` - Статистика памяти

### Использование пользовательских метрик

```go
func useCustomMetrics() {
    metrics := sai.Metrics()
    
    // Счётчик - монотонно возрастающее значение
    userRegistrations := metrics.Counter("user_registrations_total", map[string]string{
        "source": "web",
    })
    
    // Датчик - значение которое может увеличиваться или уменьшаться
    activeConnections := metrics.Gauge("active_connections", nil)
    
    // Гистограмма - распределение значений
    requestDuration := metrics.Histogram(
        "api_request_duration_seconds",
        []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0},
        map[string]string{"endpoint": "users"},
    )
    
    // Сводка - квантили в скользящем временном окне
    responseSize := metrics.Summary(
        "api_response_size_bytes",
        map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
        map[string]string{"endpoint": "users"},
    )
    
    // Использовать метрики
    userRegistrations.Inc()
    activeConnections.Set(42)
    requestDuration.Observe(1.2)
    responseSize.Observe(1024)
}

func setupBusinessMetrics() {
    metrics := sai.Metrics()
    
    // Метрики электронной коммерции
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
    
    // Метрики времени обработки
    processingDuration := metrics.Histogram(
        "order_processing_duration_seconds",
        []float64{0.1, 0.5, 1.0, 5.0, 10.0, 30.0},
        map[string]string{"step": "validation"},
    )
    
    // Метрики использования
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

### Метрики в обработчиках

```go
func handleWithMetrics(ctx *types.RequestCtx) {
    start := time.Now()
    
    // Получить метрики
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
    
    // Отслеживать активные запросы
    activeRequests.Inc()
    defer activeRequests.Dec()
    
    // Отслеживать длительность запроса
    defer requestDuration.ObserveDuration(start)
    
    // Обработать запрос
    result, err := processRequest(ctx)
    
    // Записать метрики на основе результата
    if err != nil {
        errorCounter := metrics.Counter("api_errors_total", map[string]string{
            "path":  string(ctx.Path()),
            "error": "processing_failed",
        })
        errorCounter.Inc()
        
        ctx.Error(err, 500)
        requestCounter.Add(1)  // Подсчитать провалившиеся запросы
        return
    }
    
    // Записать успех
    requestCounter.Inc()
    
    // Записать бизнес метрики
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

### Пользовательский провайдер метрик

```go
// Пользовательский провайдер метрик DataDog
type DataDogMetrics struct {
    client dogstatsd.ClientInterface
    logger types.Logger
    prefix string
}

func NewDataDogMetrics(addr, prefix string, logger types.Logger) *DataDogMetrics {
    client, err := dogstatsd.New(addr)
    if err != nil {
        logger.Error("Не удалось создать DataDog клиент", zap.Error(err))
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

// Реализовать DataDogCounter, DataDogGauge, DataDogHistogram...

// Зарегистрировать пользовательский провайдер метрик
func init() {
    metrics.RegisterMetricsManager("datadog", func(config interface{}) (types.MetricsManager, error) {
        cfg := config.(map[string]interface{})
        addr := cfg["addr"].(string)
        prefix := cfg["prefix"].(string)
        
        return NewDataDogMetrics(addr, prefix, sai.Logger()), nil
    })
}
```

Конфигурация для пользовательских метрик:
```yaml
metrics:
  enabled: true
  type: "datadog"
  config:
    addr: "localhost:8125"
    prefix: "myservice"
```

### Панель метрик

При использовании Prometheus вы можете создать панели Grafana с этими запросами:

```promql
# Скорость запросов
rate(http_requests_total[5m])

# Скорость ошибок
rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m])

# Перцентили времени ответа
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Активные соединения
go_goroutines

# Использование памяти
go_memstats_alloc_bytes

# Коэффициент попаданий в кэш
cache_hit_rate

# Бизнес метрики
rate(orders_total[5m])
increase(revenue_total[1h])
```

## 🛡️ TLS Менеджер

Фреймворк предоставляет автоматическое управление TLS сертификатами с интеграцией Let's Encrypt.

### Конфигурация

```yaml
server:
  tls:
    enabled: true
    auto_cert: true                    # Использовать Let's Encrypt
    domains: ["api.example.com"]       # Домены для сертификатов
    email: "admin@example.com"         # Email для Let's Encrypt
    cache_dir: "./certs"               # Директория кэша сертификатов
    acme_directory: ""                 # Пользовательская ACME директория (опционально)
    # Ручные сертификаты (альтернатива auto_cert)
    cert_file: "/path/to/cert.pem"     # Файл сертификата
    key_file: "/path/to/key.pem"       # Файл приватного ключа
```

### Автоматические сертификаты (Let's Encrypt)

```go
func setupAutoTLS() {
    // TLS настраивается автоматически из config.yml
    // Фреймворк будет:
    // 1. Запрашивать сертификаты от Let's Encrypt
    // 2. Автоматически обрабатывать ACME вызовы
    // 3. Обновлять сертификаты до истечения срока
    // 4. Обслуживать HTTPS трафик
    
    router := sai.Router()
    
    // Все маршруты автоматически используют HTTPS когда TLS включён
    router.GET("/api/secure", func(ctx *types.RequestCtx) {
        ctx.SuccessJSON(map[string]interface{}{
            "secure":     true,
            "protocol":   "https",
            "cert_info":  getCertificateInfo(ctx),
        })
    })
}

func getCertificateInfo(ctx *types.RequestCtx) map[string]interface{} {
    // Извлечь информацию о сертификате из запроса
    return map[string]interface{}{
        "tls_version": "TLS 1.3",
        "cipher":      "ECDHE-RSA-AES256-GCM-SHA384",
        "server_name": string(ctx.Host()),
    }
}
```

### Ручные сертификаты

```yaml
server:
  tls:
    enabled: true
    auto_cert: false
    cert_file: "/etc/ssl/certs/server.crt"
    key_file: "/etc/ssl/private/server.key"
```

### Мониторинг сертификатов

```go
func setupCertificateMonitoring() {
    // TLS менеджер автоматически предоставляет статус сертификата
    router := sai.Router()
    
    router.GET("/admin/certificates", func(ctx *types.RequestCtx) {
        // Эта конечная точка должна быть защищена аутентификацией администратора
        tlsManager := getTLSManager() // Получить из контейнера сервиса
        
        if tlsManager == nil {
            ctx.Error(types.NewError("TLS не включён"), 404)
            return
        }
        
        status := tlsManager.GetCertificateStatus()
        ctx.SuccessJSON(status)
    }).WithMiddlewares("auth") // Требуется аутентификация администратора
}

// Формат ответа статуса сертификата:
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

### TLS заголовки безопасности

```go
func setupSecurityHeaders() {
    // Добавить промежуточное ПО безопасности для HTTPS
    router := sai.Router()
    
    // Все маршруты получают заголовки безопасности когда TLS включён
    router.Use(func(ctx *types.RequestCtx, next func(*types.RequestCtx), config *types.RouteConfig) {
        if isTLSEnabled() {
            // HSTS - принудительный HTTPS для будущих запросов
            ctx.Response.Header.Set("Strict-Transport-Security", 
                "max-age=31536000; includeSubDomains; preload")
            
            // Предотвратить атаки понижения версии
            ctx.Response.Header.Set("Upgrade-Insecure-Requests", "1")
            
            // Безопасность контента
            ctx.Response.Header.Set("X-Content-Type-Options", "nosniff")
            ctx.Response.Header.Set("X-Frame-Options", "DENY")
            ctx.Response.Header.Set("X-XSS-Protection", "1; mode=block")
            
            // Политика реферера
            ctx.Response.Header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
        }
        
        next(ctx)
    })
}
```

### Перенаправление HTTP на HTTPS

```go
func setupHTTPSRedirect() {
    // Когда TLS включён, автоматически перенаправлять HTTP на HTTPS
    
    if isTLSEnabled() {
        // Запустить HTTP сервер для перенаправлений
        go func() {
            redirectServer := &fasthttp.Server{
                Handler: func(ctx *fasthttp.RequestCtx) {
                    // Перенаправить на HTTPS
                    httpsURL := fmt.Sprintf("https://%s%s", 
                        ctx.Host(), ctx.RequestURI())
                    
                    ctx.Redirect(httpsURL, fasthttp.StatusMovedPermanently)
                },
            }
            
            httpAddr := fmt.Sprintf("%s:80", getServerHost())
            sai.Logger().Info("Запуск HTTP сервера перенаправлений", 
                zap.String("addr", httpAddr))
            
            if err := redirectServer.ListenAndServe(httpAddr); err != nil {
                sai.Logger().Error("HTTP сервер перенаправлений провалился", zap.Error(err))
            }
        }()
    }
}
```

### Продакшн настройка TLS

```bash
# Переменные среды продакшн окружения
export TLS_ENABLED=true
export TLS_AUTO_CERT=true
export TLS_DOMAINS=api.example.com,www.api.example.com
export TLS_EMAIL=admin@example.com

# Docker развёртывание с TLS
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

### Мониторинг обновления сертификатов

```go
func setupCertificateAlerts() {
    // Мониторить истечение срока сертификатов
    cron := sai.Cron()
    
    cron.Add("certificate_check", "0 */12 * * *", func() {
        tlsManager := getTLSManager()
        if tlsManager == nil {
            return
        }
        
        status := tlsManager.GetCertificateStatus()
        
        for domain, cert := range status {
            if cert.Status == "expiring_soon" || cert.DaysUntilExpiry <= 7 {
                // Отправить уведомление
                sai.Actions().Publish("certificate.expiring", map[string]interface{}{
                    "domain":             domain,
                    "days_until_expiry":  cert.DaysUntilExpiry,
                    "not_after":          cert.NotAfter,
                })
                
                sai.Logger().Warn("Срок действия сертификата скоро истекает",
                    zap.String("domain", domain),
                    zap.Int("days_until_expiry", cert.DaysUntilExpiry))
            }
        }
    })
}
```

---

## 📄 Лицензия

MIT Лицензия - см. файл LICENSE для подробностей.

## 🆘 Поддержка

- 📧 Email: support@sai-service.com
- 💬 Discord: [SAI Сообщество](https://discord.gg/sai)
- 📖 Документация: [docs.sai-service.com](https://docs.sai-service.com)
- 🐛 Проблемы: [GitHub Issues](https://github.com/saiset-co/sai-service/issues)

---

**Создавайте мощные Go сервисы за минуты, а не дни!**
## ❤️ Менеджер здоровья# SAI Service Framework

🚀 **Мощный, готовый к продакшену Go фреймворк для создания высокопроизводительных микросервисов и API**

## Содержание

- [Описание проекта](#-описание-проекта)
- [Быстрый старт](#-быстрый-старт)
- [Ручная установка](#-ручная-установка)
- [Глобальные объекты доступа](#-глобальные-объекты-доступа)
- [Конфигурация](#-конфигурация)
- [Обработка данных и управление ошибками](#-обработка-данных-и-управление-ошибками)
- [Система логирования](#-система-логирования)
- [Базовый CRUD API](#-базовый-crud-api)
- [Аутентификация](#-аутентификация)
- [Система кэширования](#-система-кэширования)
- [Менеджер базы данных](#-менеджер-базы-данных)
- [Промежуточное ПО](#-промежуточное-по)
- [Менеджер документации](#-менеджер-документации)
- [Система клиентов](#-система-клиентов)
- [Система событий](#-система-событий)
- [Веб-хуки](#-веб-хуки)
- [Cron задачи](#-cron-задачи)
- [Менеджер здоровья](#-менеджер-здоровья)
- [Менеджер метрик](#-менеджер-метрик)
- [TLS Менеджер](#-tls-менеджер)

## 📋 Описание проекта

SAI Service Framework - это комплексный, корпоративного уровня Go фреймворк, предназначенный для создания масштабируемых, сопровождаемых и наблюдаемых микросервисов. Фреймворк предоставляет полный набор готовых к продакшену компонентов, которые устраняют шаблонный код и позволяют разработчикам сосредоточиться на бизнес-логике.

### Ключевые особенности:
- **Старт без конфигурации** - Работает из коробки с разумными настройками по умолчанию
- **Модульная архитектура** - Включайте только нужные компоненты
- **Производительность прежде всего** - Построен на FastHTTP для максимальной пропускной способности
- **Легковесная база данных** - Встроенная CloverDB с MongoDB-подобными запросами
- **Готовность к продакшену** - Комплексное логирование, метрики и проверки здоровья
- **Дружелюбность к разработчику** - Интуитивные API и обширная документация
- **Совместимость с sai-storage** - Легкая миграция от легковесной к полноценной БД

## 🚀 Быстрый старт

Самый быстрый способ начать - использовать наш генератор сервисов:

```bash
# Клонируйте репозиторий
git clone <repository-url>
cd sai-service-framework

# Сделайте генератор исполняемым
chmod +x generator.sh

# Запустите интерактивный генератор
./generator.sh

# Следуйте подсказкам для настройки вашего сервиса
```
Больше информации в [ДОКУМЕНТАЦИИ ГЕНЕРАТОРА](./GENERATOR.md)

### Опции генератора

```bash
# Создать базовый API сервис
./generator.sh --name "My API" --features "auth,cache,docs"

# Создать полнофункциональный микросервис
./generator.sh --name "User Service" --features "auth,cache,metrics,cron,actions,health"

# Создать с конкретными конфигурациями
./generator.sh \
  --name "Enterprise API" \
  --features "auth,cache,metrics,docs,tls" \
  --auth "token,basic" \
  --cache "redis" \
  --metrics "prometheus"
```

Структура сгенерированного проекта:
```
my-service/
├── cmd/main.go              # Точка входа
├── internal/
│   ├── handlers.go          # HTTP обработчики
│   └── service.go           # Бизнес-логика
├── .env.example             # Конфигурация
├── go.mod                   # Конфигурация
├── config.template.yml      # Конфигурация
├── docker-compose.yml       # Docker настройка
├── Dockerfile               # Образ контейнера
├── Makefile                 # Команды сборки
└── README.md                # Документация проекта
```

## 🔧 Ручная установка

### Установка

```bash
# Инициализируйте новый Go модуль
go mod init github.com/your-org/your-service

# Добавьте SAI Service Framework
go get github.com/saiset-co/sai-service
```

### Базовая настройка сервиса

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
    
    // Создайте сервис с файлом конфигурации
    svc, err := service.NewService(ctx, "config.yml")
    if err != nil {
        log.Fatal(err)
    }
    
    // Настройте маршруты
    setupRoutes()
    
    // Запустите сервис (неблокирующий)
    if err := svc.Start(); err != nil {
        log.Fatal(err)
    }
}

func setupRoutes() {
    router := sai.Router()
    
    // Базовая конечная точка
    router.GET("/api/v1/hello", func(ctx *types.RequestCtx) {
        ctx.SuccessJSON(map[string]string{
            "message": "Привет, мир!",
            "service": "SAI Service",
        })
    })
    
    // Защищённая конечная точка с кэшем
    router.GET("/api/v1/data", func(ctx *types.RequestCtx) {
        data := map[string]interface{}{
            "timestamp": time.Now(),
            "data":      []string{"элемент1", "элемент2", "элемент3"},
        }
        ctx.SuccessJSON(data)
    }).WithMiddlewares("auth").WithCache("api_data", 5*time.Minute)
}
```

## 🌐 Глобальные объекты доступа

Фреймворк предоставляет удобный глобальный доступ ко всем основным компонентам через пакет `sai`:

### Доступные глобальные объекты

```go
import "github.com/saiset-co/sai-service/sai"

// Основные компоненты
router := sai.Router()           // HTTP роутер
logger := sai.Logger()           // Экземпляр логгера
config := sai.Config()           // Менеджер конфигурации

// Опциональные компоненты (если включены в конфигурации)
cache := sai.Cache()             // Менеджер кэша (паника если отключен)
metrics := sai.Metrics()         // Менеджер метрик (паника если отключен)
cron := sai.Cron()              // Планировщик Cron (паника если отключен)
actions := sai.Actions()         // Брокер событий (паника если отключен)
clientManager := sai.ClientManager() // HTTP клиенты (паника если отключены)

// Пользовательские сервисы (устанавливаются вашим приложением)
sai.Set("database", dbInstance)
sai.Set("emailService", emailSvc)

// Получить пользовательские сервисы
var db *sql.DB
if sai.Load("database", &db) {
    // Использовать базу данных
}

// Проверить существование сервиса
if sai.Has("emailService") {
    emailSvc, _ := sai.Get("emailService")
    // Использовать email сервис
}
```

### Примеры использования

```go
func handleUser(ctx *types.RequestCtx) {
    // Логирование с глобальным логгером
    sai.Logger().Info("Обработка пользовательского запроса",
        zap.String("user_id", ctx.UserValue("user_id").(string)))
    
    // Получить из кэша
    if data, found := sai.Cache().Get("user_data"); found {
        ctx.SuccessJSON(data)
        return
    }
    
    // Получить значение конфигурации
    maxRetries := sai.Config().GetValue("api.max_retries", 3).(int)
    
    // Записать метрики
    counter := sai.Metrics().Counter("api_requests", map[string]string{
        "endpoint": "users",
    })
    counter.Inc()
    
    // Обработать запрос...
}
```

## ⚙️ Конфигурация

### Менеджер конфигурации

Система конфигурации поддерживает YAML файлы с подстановкой переменных среды и типобезопасным доступом:

```go
// Получить всю конфигурацию
config := sai.Config().GetConfig()

// Получить конкретные значения с умолчаниями
dbHost := sai.Config().GetValue("database.host", "localhost")
port := sai.Config().GetValue("server.http.port", 8080)

// Типобезопасное чтение конфигурации
var dbConfig DatabaseConfig
err := sai.Config().GetAs("database", &dbConfig)
```

### Минимальная конфигурация

```yaml
# config.yml - Минимальная рабочая конфигурация
name: "Мой Сервис"
version: "1.0.0"
```

### Полная конфигурация

```yaml
name: "Корпоративный Сервис"           # Название сервиса (обязательно)
version: "2.0.0"                    # Версия сервиса (обязательно)

server:
  http:
    host: "0.0.0.0"                 # Адрес привязки
    port: 8080                      # HTTP порт
    read_timeout: 30                # Таймаут чтения в секундах
    write_timeout: 30               # Таймаут записи в секундах  
    idle_timeout: 120               # Таймаут keep-alive в секундах
    shutdown_timeout: 15            # Таймаут корректного завершения
  tls:
    enabled: true                   # Включить HTTPS
    auto_cert: true                 # Использовать автосертификаты Let's Encrypt
    domains: ["api.example.com"]    # Домены для автосертификатов
    email: "admin@example.com"      # Email для Let's Encrypt
    cert_file: "/path/cert.pem"     # Файл сертификата (ручной)
    key_file: "/path/key.pem"       # Файл приватного ключа (ручной)
    cache_dir: "./certs"            # Директория кэша сертификатов

logger:
  level: "info"                     # Уровень логирования
  type: "default"                   # Тип логгера: default, custom
  config:                           # Конфигурация, специфичная для логгера
    format: "console"               # Формат: console, json
    output: "stdout"                # Вывод: stdout, stderr, file
    file: "/var/log/service.log"    # Путь к файлу лога (если output=file)

auth_providers:                     # Провайдеры аутентификации
  token:                            # Токен-основанная аутентификация
    params:
      token: "ваш-секретный-токен"    # API токен
  basic:                            # Базовая HTTP аутентификация
    params:
      username: "admin"             # Имя пользователя
      password: "безопасный-пароль"   # Пароль

middlewares:                        # Конфигурация промежуточного ПО
  enabled: true                     # Включить систему промежуточного ПО
  recovery:                         # Промежуточное ПО восстановления от паники
    enabled: true                   # Включить восстановление
    weight: 10                      # Порядок выполнения (меньше = раньше)
    params:
      stack_trace: true             # Включить трассировку стека в логи
  logging:                          # Промежуточное ПО логирования запросов
    enabled: true
    weight: 20
    params:
      log_level: "info"             # Уровень логирования для запросов
      log_headers: false            # Логировать заголовки запросов
      log_body: false               # Логировать тело запроса/ответа
  rate_limit:                       # Промежуточное ПО ограничения скорости
    enabled: true
    weight: 30
    params:
      requests_per_minute: 100      # Макс запросов в минуту на IP
  body_limit:                       # Ограничение размера тела запроса
    enabled: true
    weight: 40
    params:
      max_body_size: 10485760       # Макс размер тела в байтах (10MB)
  cors:                             # Cross-Origin Resource Sharing
    enabled: true
    weight: 50
    params:
      AllowedOrigins: ["*"]         # Разрешённые источники
      AllowedMethods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
      AllowedHeaders: ["Content-Type", "Authorization"]
      MaxAge: 86400                 # Длительность кэша preflight
  auth:                             # Промежуточное ПО аутентификации
    enabled: true
    weight: 60
    params:
      token: "ваш-api-токен"       # Токен по умолчанию
  compression:                      # Сжатие ответов
    enabled: true
    weight: 70
    params:
      algorithm: "gzip"             # Алгоритм сжатия
      level: 6                      # Уровень сжатия (1-9)
      threshold: 1024               # Минимальный размер ответа для сжатия
  cache:                            # Промежуточное ПО кэширования ответов
    enabled: true
    weight: 80
    params:
      default_ttl: "5m"             # TTL кэша по умолчанию

cache:                              # Система кэширования
  enabled: true                     # Включить кэширование
  type: "redis"                     # Тип кэша: memory, redis, custom
  default_ttl: "1h"                 # TTL по умолчанию для записей кэша
  config:                           # Конфигурация, специфичная для кэша
    host: "localhost:6379"          # Redis хост:порт
    password: ""                    # Пароль Redis
    db: 0                          # Номер базы данных Redis
    pool_size: 10                  # Размер пула соединений

metrics:                            # Сбор метрик
  enabled: true                     # Включить метрики
  type: "prometheus"                # Тип метрик: memory, prometheus, custom
  prefix: "myservice"               # Префикс метрик
  config:
    namespace: "myservice"          # Пространство имён Prometheus
    subsystem: "api"                # Подсистема Prometheus
  http:                             # HTTP конечная точка метрик
    enabled: true                   # Включить HTTP конечную точку метрик
    path: "/metrics"                # Путь конечной точки метрик
    port: 9090                      # Порт сервера метрик (0 = тот же что и основной)
  collectors:                       # Встроенные коллекторы
    system: true                    # Системные метрики (CPU, память)
    runtime: true                   # Метрики среды выполнения Go
    http: true                      # Метрики HTTP запросов
    cache: true                     # Метрики кэша
    middleware: true                # Метрики промежуточного ПО

health:                             # Система проверки здоровья
  enabled: true                     # Включить проверки здоровья

docs:                               # Документация API
  enabled: true                     # Включить документацию OpenAPI/Swagger
  path: "/docs"                     # Путь конечной точки документации

cron:                               # Планировщик Cron задач
  enabled: true                     # Включить планировщик cron
  timezone: "UTC"                   # Часовой пояс для cron задач

actions:                            # Система событий
  enabled: true                     # Включить систему событий
  broker:                           # Брокер событий
    enabled: true                   # Включить брокер
    type: "websocket"               # Тип брокера: websocket, custom
    config:                         # Конфигурация, специфичная для брокера
      port: 8081                    # Порт WebSocket
  webhooks:                         # Система веб-хуков
    enabled: true                   # Включить веб-хуки
    config:
      max_retries: 3                # Макс повторы доставки веб-хука
      timeout: "30s"                # Таймаут доставки веб-хука

clients:                            # Система HTTP клиентов
  enabled: true                     # Включить HTTP клиенты
  default_timeout: "30s"            # Таймаут запроса по умолчанию
  max_idle_connections: 100         # Макс неактивных соединений
  idle_conn_timeout: "90s"          # Таймаут неактивного соединения
  default_retries: 3                # Количество повторов по умолчанию
  circuit_breaker:                  # Конфигурация автоматического выключателя
    enabled: true                   # Включить автоматический выключатель
    failure_threshold: 5            # Сбои до открытия цепи
    recovery_timeout: "60s"         # Время до попытки восстановления
    half_open_requests: 3           # Запросы в полуоткрытом состоянии
  services:                         # Внешние сервисы
    user_service:                   # Название сервиса
      url: "http://user-service:8080"  # Базовый URL
      auth:                         # Конфигурация аутентификации
        provider: "token"           # Провайдер аутентификации для использования
        payload:
          token: "токен-сервиса"    # Токен аутентификации
      events: ["user.created"]      # События для подписки
```

### Подстановка переменных среды

Файлы конфигурации поддерживают подстановку переменных среды в config.template.yml:

```yaml
database:
  host: "${DB_HOST:localhost}"      # Использовать переменную DB_HOST, по умолчанию localhost
  port: "${DB_PORT:5432}"           # Использовать переменную DB_PORT, по умолчанию 5432
  password: "${DB_PASSWORD}"        # Использовать переменную DB_PASSWORD, обязательно

cache:
  enabled: "${CACHE_ENABLED:true}"  # Использовать переменную CACHE_ENABLED, по умолчанию true
```

## 📊 Обработка данных и управление ошибками

Фреймворк предоставляет удобные методы для обработки HTTP запросов и ответов:

### Методы ответов

```go
func handleSuccess(ctx *types.RequestCtx) {
    // JSON ответ со статусом 200
    data := map[string]interface{}{
        "id":   123,
        "name": "Иван Иванов",
        "active": true,
    }
    ctx.SuccessJSON(data)
}

func handleCustomResponse(ctx *types.RequestCtx) {
    // Пользовательский ответ с заголовками
    htmlData := []byte("<h1>Привет мир</h1>")
    htmlHeader := []byte("text/html; charset=UTF-8")
    ctx.Success(htmlData, htmlHeader)
}

func handlePlainText(ctx *types.RequestCtx) {
    // Ответ в виде простого текста (использует заголовок text/html по умолчанию)
    textData := []byte("Ответ в виде простого текста")
    ctx.Success(textData, nil)
}
```

### Чтение данных запроса

```go
type UserRequest struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"min=0,max=150"`
}

func handleCreateUser(ctx *types.RequestCtx) {
    var req UserRequest
    
    // Прочитать и десериализовать JSON тело запроса
    if err := ctx.ReadJSON(&req); err != nil {
        ctx.Error(err, 400)
        return
    }
    
    // Обработать запрос...
    user := createUser(req)
    ctx.SuccessJSON(user)
}

// Альтернативные методы чтения
func handleAlternativeReading(ctx *types.RequestCtx) {
    // Прочитать сырое тело
    body := ctx.PostBody()
    
    // Ручная десериализация
    var data map[string]interface{}
    if err := ctx.Unmarshal(body, &data); err != nil {
        ctx.Error(err, 400)
        return
    }
    
    // Ручная сериализация
    response, err := ctx.Marshal(data)
    if err != nil {
        ctx.Error(err, 500)
        return
    }
    
    ctx.Success(response, []byte("application/json"))
}
```

### Обработка ошибок

```go
func handleWithErrors(ctx *types.RequestCtx) {
    userID := string(ctx.QueryArgs().Peek("user_id"))
    if userID == "" {
        // Пользовательская ошибка со статусом 400
        ctx.Error(types.NewError("user_id обязателен"), 400)
        return
    }
    
    user, err := getUserByID(userID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            // Ошибка "не найдено"
            ctx.Error(types.NewError("пользователь не найден"), 404)
        } else {
            // Внутренняя ошибка сервера
            ctx.Error(types.WrapError(err, "не удалось получить пользователя"), 500)
        }
        return
    }
    
    ctx.SuccessJSON(user)
}

// Формат ответа ошибки:
// {
//   "error": "Bad Request",
//   "message": "user_id обязателен"
// }
```

### Доступ к контексту запроса

```go
func handleRequestInfo(ctx *types.RequestCtx) {
    // HTTP метод
    method := string(ctx.Method())
    
    // Путь запроса
    path := string(ctx.Path())
    
    // Параметры запроса
    limit := string(ctx.QueryArgs().Peek("limit"))
    
    // Заголовки
    authHeader := string(ctx.Request.Header.Peek("Authorization"))
    
    // Пользовательские значения (установленные промежуточным ПО)
    userID := ctx.UserValue("user_id")
    
    // Установить заголовки ответа
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

## 📝 Система логирования

### Использование встроенного логгера

```go
func useLogger() {
    logger := sai.Logger()
    
    // Базовое логирование
    logger.Debug("Отладочное сообщение")
    logger.Info("Информационное сообщение")
    logger.Warn("Предупреждение")
    logger.Error("Сообщение об ошибке")
    
    // Структурированное логирование с полями
    logger.Info("Пользователь создан",
        zap.String("user_id", "123"),
        zap.String("email", "user@example.com"),
        zap.Duration("processing_time", time.Millisecond*150))
    
    // Логирование ошибки с трассировкой стека
    err := errors.New("что-то пошло не так")
    logger.ErrorWithErrStack("Операция провалилась", err,
        zap.String("operation", "create_user"))
    
    // Пользовательский уровень лога
    logger.Log(zapcore.FatalLevel, "Произошла фатальная ошибка")
}

func handleRequestWithLogging(ctx *types.RequestCtx) {
    requestID := generateRequestID()
    
    sai.Logger().Info("Запрос начат",
        zap.String("request_id", requestID),
        zap.String("method", string(ctx.Method())),
        zap.String("path", string(ctx.Path())))
    
    // Обработать запрос...
    
    sai.Logger().Info("Запрос завершён",
        zap.String("request_id", requestID),
        zap.Int("status", 200))
}
```

### Пользовательская реализация логгера

```go
// Создать пользовательский логгер
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
    // Добавить поле сервиса ко всем логам
    allFields := append(fields, zap.String("service", c.service))
    c.zapLogger.Info(msg, allFields...)
}

func (c *CustomLogger) Error(msg string, fields ...zap.Field) {
    allFields := append(fields, zap.String("service", c.service))
    c.zapLogger.Error(msg, allFields...)
}

// Реализовать другие необходимые методы...

// Зарегистрировать пользовательский логгер
func init() {
    logger.RegisterLogger("custom", func(config interface{}) (types.Logger, error) {
        // Разобрать конфигурацию и создать логгер
        return NewCustomLogger("мой-сервис"), nil
    })
}
```

Конфигурация для пользовательского логгера:
```yaml
logger:
  type: "custom"
  level: "info"
  config:
    service_name: "мой-сервис"
    output_format: "json"
```

## 🎯 Базовый CRUD API

Система промежуточного ПО применяет всё включённое промежуточное ПО к маршрутам по умолчанию. Вы можете отключить конкретное промежуточное ПО для групп или отдельных маршрутов и повторно включить его по необходимости.

### Поведение промежуточного ПО по умолчанию

```go
func setupCRUDAPI() {
    // Всё включённое промежуточное ПО применяется ко всем маршрутам по умолчанию
    router := sai.Router()
    
    // API группа - отключить аутентификацию для публичных конечных точек
    api := router.Group("/api/v1").
        WithoutMiddlewares("auth")  // Отключить аутентификацию для всей группы
    
    // Публичные конечные точки (аутентификация не требуется)
    api.GET("/status", handleStatus)
    api.POST("/register", handleRegister)
    
    // Группа пользователей - повторно включить аутентификацию для защищённых конечных точек
    users := api.Group("/users").
        WithMiddlewares("auth")  // Повторно включить аутентификацию для группы пользователей
    
    users.POST("/", createUser).
        WithDoc("Создать пользователя", "Создаёт нового пользователя", "users", CreateUserRequest{}, User{})
    
    users.GET("/", listUsers).
        WithCache("users_list", 5*time.Minute, "users").
        WithDoc("Список пользователей", "Возвращает постраничный список пользователей", "users", nil, []User{})
    
    users.GET("/{id}", getUser).
        WithDoc("Получить пользователя", "Возвращает пользователя по ID", "users", nil, User{})
    
    users.PUT("/{id}", updateUser).
        WithDoc("Обновить пользователя", "Обновляет существующего пользователя", "users", UpdateUserRequest{}, User{})
    
    users.DELETE("/{id}", deleteUser).
        WithoutMiddlewares("cache").  // Отключить кэш для операций удаления
        WithDoc("Удалить пользователя", "Удаляет пользователя по ID", "users", nil, nil)
        
    // Административные конечные точки - дополнительное промежуточное ПО
    admin := api.Group("/admin").
        WithMiddlewares("auth", "rate_limit").  // Включить аутентификацию и ограничение скорости
        WithTimeout(30 * time.Second)
    
    admin.GET("/stats", getAdminStats)
    admin.POST("/maintenance", enableMaintenance)
}
```

### Реализация CRUD

```go
type User struct {
    ID       string    `json:"id" doc:"Уникальный идентификатор пользователя"`
    Name     string    `json:"name" doc:"Полное имя" validate:"required"`
    Email    string    `json:"email" doc:"Email адрес" validate:"required,email"`
    Active   bool      `json:"active" doc:"Статус аккаунта"`
    Created  time.Time `json:"created" doc:"Метка времени создания"`
    Updated  time.Time `json:"updated" doc:"Метка времени последнего обновления"`
}

type CreateUserRequest struct {
    Name  string `json:"name" validate:"required" doc:"Полное имя пользователя"`
    Email string `json:"email" validate:"required,email" doc:"Email адрес пользователя"`
}

type UpdateUserRequest struct {
    Name   *string `json:"name,omitempty" doc:"Полное имя пользователя"`
    Email  *string `json:"email,omitempty" validate:"omitempty,email" doc:"Email пользователя"`
    Active *bool   `json:"active,omitempty" doc:"Статус активности аккаунта"`
}

type ListUsersRequest struct {
    Page     int    `query:"page" doc:"Номер страницы" example:"1"`
    Limit    int    `query:"limit" doc:"Элементов на странице" example:"20"`
    Search   string `query:"search" doc:"Поисковый запрос"`
    Active   *bool  `query:"active" doc:"Фильтр по статусу активности"`
}

func createUser(ctx *types.RequestCtx) {
    var req CreateUserRequest
    if err := ctx.ReadJSON(&req); err != nil {
        ctx.Error(types.WrapError(err, "неверное тело запроса"), 400)
        return
    }
    
    // Проверить существование пользователя
    if userExists(req.Email) {
        ctx.Error(types.NewError("пользователь с таким email уже существует"), 409)
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
        sai.Logger().Error("Не удалось сохранить пользователя", 
            zap.Error(err),
            zap.String("email", req.Email))
        ctx.Error(types.WrapError(err, "не удалось создать пользователя"), 500)
        return
    }
    
    // Аннулировать кэш
    sai.Cache().Invalidate("users")
    
    // Опубликовать событие
    sai.Actions().Publish("user.created", map[string]interface{}{
        "user_id": user.ID,
        "email":   user.Email,
    })
    
    sai.Logger().Info("Пользователь создан",
        zap.String("user_id", user.ID),
        zap.String("email", user.Email))
    
    ctx.SuccessJSON(user)
}

func listUsers(ctx *types.RequestCtx) {
    var req ListUsersRequest
    
    // Разобрать параметры запроса
    req.Page = parseInt(string(ctx.QueryArgs().Peek("page")), 1)
    req.Limit = parseInt(string(ctx.QueryArgs().Peek("limit")), 20)
    req.Search = string(ctx.QueryArgs().Peek("search"))
    
    if activeStr := string(ctx.QueryArgs().Peek("active")); activeStr != "" {
        if active, err := strconv.ParseBool(activeStr); err == nil {
            req.Active = &active
        }
    }
    
    // Валидировать пагинацию
    if req.Page < 1 {
        req.Page = 1
    }
    if req.Limit < 1 || req.Limit > 100 {
        req.Limit = 20
    }
    
    users, total, err := getUsersList(req)
    if err != nil {
        sai.Logger().Error("Не удалось получить список пользователей", zap.Error(err))
        ctx.Error(types.WrapError(err, "не удалось получить пользователей"), 500)
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
            ctx.Error(types.NewError("пользователь не найден"), 404)
        } else {
            sai.Logger().Error("Не удалось получить пользователя", 
                zap.Error(err),
                zap.String("user_id", userID))
            ctx.Error(types.WrapError(err, "не удалось получить пользователя"), 500)
        }
        return
    }
    
    ctx.SuccessJSON(user)
}

func updateUser(ctx *types.RequestCtx) {
    userID := ctx.UserValue("id").(string)
    
    var req UpdateUserRequest
    if err := ctx.Read(&req); err != nil {
        ctx.Error(types.WrapError(err, "неверное тело запроса"), 400)
        return
    }
    
    user, err := getUserByID(userID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            ctx.Error(types.NewError("пользователь не найден"), 404)
        } else {
            ctx.Error(types.WrapError(err, "не удалось получить пользователя"), 500)
        }
        return
    }
    
    // Обновить поля
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
        sai.Logger().Error("Не удалось обновить пользователя",
            zap.Error(err),
            zap.String("user_id", userID))
        ctx.Error(types.WrapError(err, "не удалось обновить пользователя"), 500)
        return
    }
    
    // Аннулировать кэш
    sai.Cache().Invalidate("users")
    
    // Опубликовать событие
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
            ctx.Error(types.NewError("пользователь не найден"), 404)
        } else {
            sai.Logger().Error("Не удалось удалить пользователя",
                zap.Error(err),
                zap.String("user_id", userID))
            ctx.Error(types.WrapError(err, "не удалось удалить пользователя"), 500)
        }
        return
    }
    
    // Аннулировать кэш
    sai.Cache().Invalidate("users")
    
    // Опубликовать событие
    sai.Actions().Publish("user.deleted", map[string]interface{}{
        "user_id": userID,
    })
    
    ctx.SuccessJSON(map[string]string{
        "message": "пользователь успешно удалён",
    })
}
```

## 🔐 Аутентификация

Фреймворк предоставляет гибкую систему аутентификации с множественными провайдерами и интеграцией промежуточного ПО.

### Встроенные провайдеры аутентификации

Просто описание типа провайдера аутентификации, не включает аутентификацию

#### Токен аутентификация

```yaml
auth_providers:
  token:
    params:
      token: "ваш-секретный-api-токен"
```

```go
func setupTokenAuth() {
    // Токен может быть отправлен несколькими способами:
    // 1. Заголовок Authorization: "Bearer ваш-токен"
    // 2. Заголовок Authorization: "Token ваш-токен"  
    // 3. Заголовок Authorization: "ваш-токен"
    // 4. Заголовок Token: "ваш-токен"
    
    router := sai.Router()
    
    // Защищённая конечная точка
    router.GET("/api/protected", func(ctx *types.RequestCtx) {
        // Информация о пользователе доступна после промежуточного ПО аутентификации
        userInfo := ctx.UserValue("auth_type")  // "token"
        
        ctx.SuccessJSON(map[string]interface{}{
            "message":   "Доступ разрешён",
            "auth_type": userInfo,
        })
    }).WithMiddlewares("auth")
}
```

#### Базовая аутентификация
```yaml
auth_providers:
  basic:
    params:
      username: "admin"
      password: "безопасный-пароль"
```

```go
func setupBasicAuth() {
    router := sai.Router()
    
    router.GET("/api/admin", func(ctx *types.RequestCtx) {
        // Информация о пользователе доступна после аутентификации
        username := ctx.UserValue("authenticated_user").(string)
        authType := ctx.UserValue("auth_type").(string)
        
        ctx.SuccessJSON(map[string]interface{}{
            "message":  "Доступ администратора разрешён",
            "username": username,
            "auth_type": authType,  // "basic"
        })
    }).WithMiddlewares("auth")
}
```

### Пользовательский провайдер аутентификации

```go
// Пользовательский JWT провайдер аутентификации
type JWTAuthProvider struct {
    secretKey []byte
    realm     string
}

func NewJWTAuthProvider(secretKey []byte) *JWTAuthProvider {
    return &JWTAuthProvider{
        secretKey: secretKey,
        realm:     "Защищённая область",
    }
}

func (p *JWTAuthProvider) Type() string {
    return "jwt"
}

func (p *JWTAuthProvider) ApplyToIncomingRequest(ctx *types.RequestCtx) error {
    authHeader := string(ctx.Request.Header.Peek("Authorization"))
    if authHeader == "" {
        return p.sendAuthChallenge(ctx, "Требуется заголовок Authorization")
    }
    
    if !strings.HasPrefix(authHeader, "Bearer ") {
        return p.sendAuthChallenge(ctx, "Требуется Bearer токен")
    }
    
    tokenString := strings.TrimPrefix(authHeader, "Bearer ")
    
    // Разобрать и валидировать JWT токен
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("неожиданный метод подписи")
        }
        return p.secretKey, nil
    })
    
    if err != nil || !token.Valid {
        return p.sendAuthChallenge(ctx, "Неверный токен")
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
        return errors.New("требуется конфигурация аутентификации для JWT")
    }
    
    token, ok := authConfig.Payload["token"].(string)
    if !ok {
        return errors.New("JWT токен не найден в данных аутентификации")
    }
    
    req.Header.Set("Authorization", "Bearer "+token)
    return nil
}

func (p *JWTAuthProvider) sendAuthChallenge(ctx *types.RequestCtx, message string) error {
    ctx.SetStatusCode(fasthttp.StatusUnauthorized)
    ctx.Response.Header.Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s"`, p.realm))
    
    response := map[string]interface{}{
        "error":   "Требуется аутентификация",
        "message": message,
        "type":    "bearer_auth_challenge",
    }
    
    ctx.SuccessJSON(response)
    return errors.New("jwt_auth_challenge_sent")
}

// Зарегистрировать пользовательский провайдер
func setupCustomAuth() {
    authProvider := sai.AuthProvider()
    jwtProvider := NewJWTAuthProvider([]byte("ваш-jwt-секрет"))
    
    authProvider.Register("jwt", jwtProvider)
}
```

### Конфигурация промежуточного ПО аутентификации

Используется для защиты входящих запросов. Включает аутентификацию для всех маршрутов.

```yaml
middlewares:
  auth:
    enabled: true
    weight: 60  # Выполняется после CORS, ограничения скорости и т.д.
    params:
      provider: "token" # Тип провайдера
```

### Управление аутентификацией на уровне маршрутов

```go
func setupAuthRoutes() {
    router := sai.Router()
    
    // Публичные маршруты (без аутентификации)
    public := router.Group("/api/public").
        WithoutMiddlewares("auth")
    
    public.GET("/status", handleStatus)
    public.POST("/register", handleRegister)
    
    // Защищённые маршруты (требуется аутентификация)
    protected := router.Group("/api/protected").
        WithMiddlewares("auth")
    
    protected.GET("/profile", handleProfile)
    protected.PUT("/profile", handleUpdateProfile)
    
    // Административные маршруты (аутентификация + дополнительные проверки)
    admin := router.Group("/api/admin").
        WithMiddlewares("auth")
    
    admin.GET("/users", func(ctx *types.RequestCtx) {
        // Дополнительная проверка авторизации
        claims := ctx.UserValue("user_claims").(jwt.MapClaims)
        role, ok := claims["role"].(string)
        if !ok || role != "admin" {
            ctx.Error(types.NewError("недостаточно прав"), 403)
            return
        }
        
        // Логика администратора...
        ctx.SuccessJSON(map[string]string{"message": "Доступ администратора разрешён"})
    })
}
```

## 💾 Система кэширования

Фреймворк предоставляет гибкую систему кэширования с множественными бэкендами и интеграцией промежуточного ПО.

### Конфигурация кэша

Включает менеджер кэша. Не включает кэш на маршрутах в этом месте.

```yaml
cache:
  enabled: true
  type: "redis"        # memory, redis, custom
  default_ttl: "1h"    # TTL по умолчанию для записей кэша
  config:
    host: "localhost:6379"
    password: ""
    db: 0
    pool_size: 10
    max_retries: 3
    retry_delay: "1s"
```

### Программное использование кэша

```go
func useCacheDirectly() {
    cache := sai.Cache()
    
    // Установить запись кэша
    cache.Set("user:123", userData, 15*time.Minute)
    
    // Получить запись кэша
    if data, found := cache.Get("user:123"); found {
        user := data.(*User)
        // Использовать кэшированные данные
    }
    
    // Удалить конкретный ключ
    cache.Delete("user:123")
    
    // Аннулировать множественные ключи
    cache.Invalidate("users", "user:123", "stats:daily")
    
    // Кэш с зависимостями
    cache.Set("user_stats", statsData, time.Hour)
    // Когда данные пользователя изменяются, аннулировать зависимые кэши
    cache.Invalidate("user_stats")
}

func handleCachedData(ctx *types.RequestCtx) {
    userID := ctx.UserValue("id").(string)
    cacheKey := fmt.Sprintf("user:%s", userID)
    
    // Сначала попробовать кэш
    if userData, found := sai.Cache().Get(cacheKey); found {
        sai.Logger().Debug("Попадание в кэш", zap.String("key", cacheKey))
        ctx.SuccessJSON(userData)
        return
    }
    
    // Промах кэша - получить из базы данных
    user, err := getUserByID(userID)
    if err != nil {
        ctx.Error(err, 500)
        return
    }
    
    // Кэшировать результат
    sai.Cache().Set(cacheKey, user, 10*time.Minute)
    
    sai.Logger().Debug("Промах кэша - данные кэшированы", zap.String("key", cacheKey))
    ctx.SuccessJSON(user)
}
```

### Промежуточное ПО кэширования

Не включает кэш для маршрутов здесь. Позволяет настраивать конфигурацию кэша для каждого маршрута.

```yaml
middlewares:
  cache:
    enabled: true
    weight: 80  # Выполняется поздно в цепочке
    params:
      default_ttl: "5m"
      cache_private: false
      cache_public: true
```

Параметры кэша маршрутов.

```go
func setupCacheMiddleware() {
    router := sai.Router()
    
    // Кэшировать ответ на 5 минут
    router.GET("/api/users", listUsers).
        WithCache("users_list", 5*time.Minute)
    
    // Кэш с зависимостями - аннулируется при изменении пользователей
    router.GET("/api/users/{id}", getUser).
        WithCache("user_detail", 15*time.Minute, "users")
    
    // Динамический ключ кэша
    router.GET("/api/users/{id}/posts", func(ctx *types.RequestCtx) {
        userID := ctx.UserValue("id").(string)
        
        // Ключ кэша будет включать ID пользователя
        posts := getUserPosts(userID)
        ctx.SuccessJSON(posts)
    }).WithCache("user_posts_{id}", 10*time.Minute, "posts", "users")
    
    // Без кэша для этой конечной точки
    router.POST("/api/users", createUser).
        WithoutMiddlewares("cache")
}
```

### Пользовательский провайдер кэша

```go
// Пользовательская реализация кэша
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
        c.logger.Error("Ошибка получения из кэша", zap.Error(err), zap.String("key", key))
        return nil, false
    }
    
    var data interface{}
    if err := json.Unmarshal([]byte(val), &data); err != nil {
        c.logger.Error("Ошибка десериализации кэша", zap.Error(err))
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

// Реализовать другие необходимые методы...

// Зарегистрировать пользовательский провайдер кэша
func init() {
    cache.RegisterCacheManager("redis-cluster", func(config interface{}) (types.CacheManager, error) {
        cfg := config.(map[string]interface{})
        addrs := cfg["addrs"].([]string)
        password := cfg["password"].(string)
        
        return NewRedisClusterCache(addrs, password, sai.Logger()), nil
    })
}
```

Конфигурация для пользовательского кэша:
```yaml
cache:
  enabled: true
  type: "redis-cluster"
  config:
    addrs: ["localhost:7000", "localhost:7001", "localhost:7002"]
    password: ""
```

## 🗄️ Менеджер базы данных

Фреймворк предоставляет легковесный менеджер базы данных с поддержкой CloverDB для небольших микросервисов, где полноценные решения баз данных, такие как sai-storage, могут быть избыточными. Он поддерживает совместимость API с sai-storage для легкой миграции.

### Конфигурация базы данных

```yaml
database:
  enabled: true
  type: "clover"        # clover, memory, или custom
  path: "./data/db"     # Путь к файлу базы данных (для CloverDB)
  name: "myapp"         # Имя базы данных
```

### Поддерживаемые типы баз данных

#### CloverDB (Встроенная NoSQL)
Идеально подходит для малых и средних микросервисов:
```yaml
database:
  enabled: true
  type: "clover"
  path: "./data/myapp.db"
  name: "myapp"
```

#### База данных в памяти
Для тестирования и разработки:
```yaml
database:
  enabled: true
  type: "memory"
  name: "test_db"
```

### Использование базы данных

```go
// Создание документов
createReq := types.CreateDocumentsRequest{
    Collection: "users",
    Data: []interface{}{
        map[string]interface{}{
            "name":  "Иван Иванов",
            "email": "ivan@example.com",
            "age":   30,
        },
    },
}

ids, err := sai.Database().CreateDocuments(ctx, createReq)
if err != nil {
    return err
}

// Чтение документов с MongoDB-подобными фильтрами
readReq := types.ReadDocumentsRequest{
    Collection: "users",
    Filter: map[string]interface{}{
        "age": map[string]interface{}{
            "$gte": 18,
        },
    },
    Limit: 10,
    Skip:  0,
}

documents, total, err := sai.Database().ReadDocuments(ctx, readReq)
if err != nil {
    return err
}

// Обновление документов
updateReq := types.UpdateDocumentsRequest{
    Collection: "users",
    Filter: map[string]interface{}{
        "email": "ivan@example.com",
    },
    Data: map[string]interface{}{
        "$set": map[string]interface{}{
            "age": 31,
        },
    },
    Upsert: false,
}

updated, err := sai.Database().UpdateDocuments(ctx, updateReq)

// Удаление документов
deleteReq := types.DeleteDocumentsRequest{
    Collection: "users",
    Filter: map[string]interface{}{
        "age": map[string]interface{}{
            "$lt": 18,
        },
    },
}

deleted, err := sai.Database().DeleteDocuments(ctx, deleteReq)
```

### MongoDB-подобные операторы запросов

Менеджер базы данных поддерживает привычные операторы запросов MongoDB:

```go
// Операторы сравнения
filter := map[string]interface{}{
    "age": map[string]interface{}{
        "$eq":  25,           // Равно
        "$ne":  25,           // Не равно
        "$gt":  18,           // Больше
        "$gte": 18,           // Больше или равно
        "$lt":  65,           // Меньше
        "$lte": 65,           // Меньше или равно
        "$in":  []int{25, 30, 35}, // В массиве
        "$nin": []int{25, 30},     // Не в массиве
    },
    "status": map[string]interface{}{
        "$exists": true,      // Поле существует
    },
}

// Операторы обновления
update := map[string]interface{}{
    "$set": map[string]interface{}{
        "status": "активен",
        "updated_at": time.Now(),
    },
    "$inc": map[string]interface{}{
        "login_count": 1,
    },
    "$unset": map[string]interface{}{
        "temp_field": "",
    },
}
```

### Управление коллекциями

```go
// Создание коллекции
err := sai.Database().CreateCollection("new_collection")

// Удаление коллекции
err := sai.Database().DropCollection("old_collection")
```

## 🚧 Промежуточное ПО

Фреймворк включает комплексную систему промежуточного ПО со встроенными компонентами и поддержкой пользовательского промежуточного ПО.

### Промежуточное ПО восстановления

Обрабатывает паники:

```yaml
middlewares:
  recovery:
    enabled: true
    weight: 10  # Выполняется первым
    params:
      stack_trace: true      # Включить трассировку стека в логи
      log_panics: true       # Логировать детали паники
      include_request: true  # Включить детали запроса в логи
```

```go
// Промежуточное ПО восстановления автоматически перехватывает паники
func handlePanic(ctx *types.RequestCtx) {
    // Это будет перехвачено промежуточным ПО восстановления
    panic("что-то пошло не так")
    
    // Промежуточное ПО восстановления:
    // 1. Залогирует панику с трассировкой стека
    // 2. Вернёт 500 Internal Server Error
    // 3. Продолжит обработку других запросов
}
```

### Промежуточное ПО логирования

Логирует все HTTP запросы и ответы:

```yaml
middlewares:
  logging:
    enabled: true
    weight: 20
    params:
      log_level: "info"       # Уровень логирования для запросов
      log_headers: false      # Логировать заголовки запросов
      log_body: false         # Логировать тело запроса/ответа
      log_response: true      # Логировать детали ответа
```

### Промежуточное ПО ограничения скорости

Реализует ограничение скорости по IP адресу:

```yaml
middlewares:
  rate_limit:
    enabled: true
    weight: 30
    params:
      requests_per_minute: 100  # Макс запросов в минуту на IP
      burst: 10                 # Ёмкость всплеска
      cleanup_interval: "1m"    # Интервал очистки старых записей
```

```go
// Ограничение скорости применяется автоматически
// Возвращает 429 Too Many Requests при превышении лимита
func setupRateLimiting() {
    router := sai.Router()
    
    // Разные ограничения скорости для разных конечных точек
    router.GET("/api/public", handlePublic).
        WithoutMiddlewares("rate_limit")  // Без ограничения скорости
    
    router.POST("/api/upload", handleUpload).
        WithMiddlewares("rate_limit")     // Применить ограничение скорости
}
```

### Промежуточное ПО ограничения размера тела

Ограничивает размер тела запроса:

```yaml
middlewares:
  body_limit:
    enabled: true
    weight: 40
    params:
      max_body_size: 10485760  # 10MB в байтах
      skip_content_length: false
```

### CORS промежуточное ПО

Обрабатывает Cross-Origin Resource Sharing:

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
      MaxAge: 86400  # Длительность кэша preflight в секундах
```

### Промежуточное ПО сжатия

Сжимает HTTP ответы:

```yaml
middlewares:
  compression:
    enabled: true
    weight: 70
    params:
      algorithm: "gzip"       # Алгоритм сжатия
      level: 6                # Уровень сжатия (1-9)
      threshold: 1024         # Минимальный размер ответа для сжатия
      allowed_types:          # Типы контента для сжатия
        - "application/json"
        - "text/html"
        - "text/plain"
        - "application/xml"
      exclude_extensions: [".jpg", ".png", ".gif"]
```

### Пользовательское промежуточное ПО

```go
// Промежуточное ПО ID запроса
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
    return 5  // Выполняется очень рано
}

func (m *RequestIDMiddleware) Handle(ctx *types.RequestCtx, next func(*types.RequestCtx), config *types.RouteConfig) {
    // Сгенерировать ID запроса
    requestID := generateRequestID()
    
    // Сохранить в контексте
    ctx.SetUserValue("request_id", requestID)
    
    // Добавить в заголовки ответа
    ctx.Response.Header.Set("X-Request-ID", requestID)
    
    m.logger.Debug("Запрос начат",
        zap.String("request_id", requestID),
        zap.String("method", string(ctx.Method())),
        zap.String("path", string(ctx.Path())))
    
    start := time.Now()
    
    // Перейти к следующему промежуточному ПО
    next(ctx)
    
    duration := time.Since(start)
    statusCode := ctx.Response.StatusCode()
    
    m.logger.Info("Запрос завершён",
        zap.String("request_id", requestID),
        zap.Int("status", statusCode),
        zap.Duration("duration", duration))
}

// Зарегистрировать промежуточное ПО (до запуска сервиса)
func registerCustomMiddleware() {
    middlewareManager := getMiddlewareManager() // Получить из инициализации сервиса
    middlewareManager.Register(NewRequestIDMiddleware(sai.Logger()))
}
```

## 📚 Менеджер документации

### Автоматическая генерация документации

```go
func setupDocumentedAPI() {
    api := sai.Router().Group("/api/v1")
    
    // Документировать с типами запроса/ответа
    api.POST("/users", createUser).
        WithDoc(
            "Создать пользователя",                    // Заголовок
            "Создаёт новый аккаунт пользователя",     // Описание
            "users",                         // Тег для группировки
            CreateUserRequest{},             // Тип тела запроса
            User{},                          // Тип ответа
        )
    
    // Документировать с параметрами запроса
    api.GET("/users", listUsers).
        WithDoc(
            "Список пользователей",
            "Возвращает постраничный список пользователей с опциональной фильтрацией",
            "users",
            ListUsersQuery{},  // Тип параметров запроса
            UserListResponse{}, // Тип ответа
        )
    
    // Документировать параметры пути
    api.GET("/users/{id}", getUser).
        WithDoc(
            "Получить пользователя",
            "Возвращает детали пользователя по ID",
            "users",
            nil,    // Нет тела запроса
            User{}, // Тип ответа
        )
}
```

### Документация с тегами структур

```go
type CreateUserRequest struct {
    Name     string `json:"name" validate:"required" doc:"Полное имя пользователя" example:"Иван Иванов"`
    Email    string `json:"email" validate:"required,email" doc:"Email адрес пользователя" example:"ivan@example.com"`
    Age      int    `json:"age" validate:"min=0,max=150" doc:"Возраст пользователя" example:"30"`
    Active   bool   `json:"active" doc:"Активен ли аккаунт пользователя" example:"true"`
    Tags     []string `json:"tags" doc:"Теги пользователя" example:"admin,premium"`
    Metadata map[string]interface{} `json:"metadata" doc:"Дополнительные метаданные пользователя"`
}

type User struct {
    ID       string    `json:"id" doc:"Уникальный идентификатор пользователя" example:"usr_123456"`
    Name     string    `json:"name" doc:"Полное имя пользователя"`
    Email    string    `json:"email" doc:"Email адрес пользователя"`
    Age      int       `json:"age" doc:"Возраст пользователя"`
    Active   bool      `json:"active" doc:"Статус аккаунта"`
    Created  time.Time `json:"created" doc:"Метка времени создания аккаунта"`
    Updated  time.Time `json:"updated" doc:"Метка времени последнего обновления"`
}

type UserListResponse struct {
    Users      []User `json:"users" doc:"Список пользователей"`
    Total      int    `json:"total" doc:"Общее количество пользователей"`
    Page       int    `json:"page" doc:"Номер текущей страницы"`
    Limit      int    `json:"limit" doc:"Элементов на странице"`
    TotalPages int    `json:"total_pages" doc:"Общее количество страниц"`
}

type ListUsersQuery struct {
    Page   int    `query:"page" doc:"Номер страницы для пагинации" example:"1"`
    Limit  int    `query:"limit" doc:"Количество элементов на странице" example:"20"`
    Search string `query:"search" doc:"Поисковый запрос для фильтрации пользователей" example:"иван"`
    Active *bool  `query:"active" doc:"Фильтр по статусу аккаунта" example:"true"`
}
```

### Доступ к документации

После настройки документация автоматически доступна по адресам:
- `/docs` - интерфейс Swagger UI, см. раздел конфигурации
- `/openapi.json` - спецификация OpenAPI в формате JSON

Документация включает:
- Все задокументированные конечные точки
- Схемы запросов/ответов
- Описания параметров
- Примеры значений
- Требования к аутентификации
- Ответы об ошибках

## 🌐 Система клиентов

Фреймворк предоставляет надёжную систему HTTP клиентов с автоматическими выключателями, повторами и обнаружением сервисов.

### Конфигурация

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
          token: "токен-сервис-к-сервису"
      events: ["user.created", "user.updated"]
    notification_service:
      url: "http://notification-service:8080"
      auth:
        provider: "basic"
        payload:
          username: "service"
          password: "secret"
```

### Использование HTTP клиентов

```go
func useHTTPClients(ctx *types.RequestCtx) {
    clientManager := sai.ClientManager()
    
    // Простой GET запрос
    response, statusCode, err := clientManager.Call(
        "user_service",           // Название сервиса
        "GET",                    // HTTP метод
        "/api/v1/users/123",      // Путь
        nil,                      // Тело запроса
        nil,                      // Опции
    )
    
    if err != nil {
        sai.Logger().Error("Не удалось вызвать пользовательский сервис", zap.Error(err))
        return
    }
    
    if statusCode == 200 {
        var user User
        ctx.Unmarshal(response, &user)
        // Использовать данные пользователя
    }
}

func callWithOptions(ctx *types.RequestCtx) {
    clientManager := sai.ClientManager()
    
    // POST запрос с пользовательскими опциями
    requestData := map[string]interface{}{
        "name":  "Иван Иванов",
        "email": "ivan@example.com",
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
        // Обработать ошибку (может быть сетевая, таймаут или HTTP ошибка)
        sai.Logger().Error("Создание пользователя провалилось",
            zap.Error(err),
            zap.Int("status_code", statusCode))
        return
    }
    
    if statusCode == 201 {
        var newUser User
        ctx.Unmarshal(response, &newUser)
        // Пользователь успешно создан
    }
}
```

### Автоматический выключатель

Клиентская система включает автоматическую функциональность автоматического выключателя:

```go
func handleCircuitBreaker() {
    // Состояния автоматического выключателя:
    // 1. Закрыт: Нормальная работа
    // 2. Открыт: Сервис недоступен, запросы быстро завершаются с ошибкой
    // 3. Полуоткрыт: Тестирование восстановления сервиса
    
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
                sai.Logger().Warn("Автоматический выключатель открыт для unreliable_service")
                // Реализовать резервную логику
                handleFallback()
                continue
            }
            // Обработать другие ошибки
        }
        
        // Обработать успешный ответ
        handleResponse(response, statusCode)
    }
}

func handleFallback() {
    // Реализовать резервную логику когда сервис недоступен
    // - Вернуть кэшированные данные
    // - Использовать альтернативный сервис
    // - Вернуть ответ по умолчанию
}
```

## 🔄 Система событий

Фреймворк предоставляет мощную систему событий, поддерживающую WebSocket и пользовательских брокеров.

### Конфигурация

```yaml
actions:
  enabled: true
  broker:
    enabled: true
    type: "websocket"
    config:
      port: 8081              # Порт WebSocket сервера
      path: "/ws"             # Путь конечной точки WebSocket
      max_connections: 1000   # Максимум одновременных соединений
      read_buffer_size: 1024  # Размер буфера чтения
      write_buffer_size: 1024 # Размер буфера записи
  webhooks:
    enabled: true
    config:
      max_retries: 3
      timeout: "30s"
```

### Публикация событий

```go
func publishEvents() {
    actions := sai.Actions()
    
    // Простое событие
    err := actions.Publish("user.created", map[string]interface{}{
        "user_id": "123",
        "email":   "user@example.com",
        "timestamp": time.Now(),
    })
    
    if err != nil {
        sai.Logger().Error("Не удалось опубликовать событие", zap.Error(err))
    }
    
    // Сложное событие с метаданными
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

// Публикация из HTTP обработчиков
func handleCreateOrder(ctx *types.RequestCtx) {
    var req CreateOrderRequest
    if err := ctx.ReadJSON(&req); err != nil {
        ctx.Error(err, 400)
        return
    }
    
    // Обработать заказ
    order, err := processOrder(req)
    if err != nil {
        ctx.Error(err, 500)
        return
    }
    
    // Опубликовать событие асинхронно
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

### Подписка на события

```go
func setupEventHandlers() {
    actions := sai.Actions()
    
    // Подписаться на события пользователей
    actions.Subscribe("user.created", handleUserCreated)
    actions.Subscribe("user.updated", handleUserUpdated)
    actions.Subscribe("user.deleted", handleUserDeleted)
    
    // Подписаться на события заказов
    actions.Subscribe("order.created", handleOrderCreated)
    actions.Subscribe("order.completed", handleOrderCompleted)
    actions.Subscribe("order.cancelled", handleOrderCancelled)
}

func handleUserCreated(msg *types.ActionMessage) error {
    sai.Logger().Info("Получено событие создания пользователя",
        zap.String("action", msg.Action),
        zap.Time("timestamp", msg.Timestamp))
    
    // Извлечь данные пользователя
    userData := msg.Payload.(map[string]interface{})
    userID := userData["user_id"].(string)
    email := userData["email"].(string)
    
    // Отправить приветственное письмо
    if err := sendWelcomeEmail(userID, email); err != nil {
        sai.Logger().Error("Не удалось отправить приветственное письмо",
            zap.Error(err),
            zap.String("user_id", userID))
        return err
    }
    
    // Обновить аналитику
    updateUserMetrics("created")
    
    // Кэшировать данные пользователя
    sai.Cache().Set(fmt.Sprintf("user:%s", userID), userData, time.Hour)
    
    return nil
}

func handleOrderCompleted(msg *types.ActionMessage) error {
    orderData := msg.Payload.(map[string]interface{})
    orderID := orderData["order_id"].(string)
    customerID := orderData["customer_id"].(string)
    
    // Сгенерировать счёт
    if err := generateInvoice(orderID); err != nil {
        return err
    }
    
    // Обновить инвентарь
    if err := updateInventory(orderData); err != nil {
        return err
    }
    
    // Отправить подтверждение по email
    if err := sendOrderConfirmation(customerID, orderID); err != nil {
        return err
    }
    
    // Запустить выполнение заказа
    sai.Actions().Publish("fulfillment.requested", map[string]interface{}{
        "order_id":    orderID,
        "customer_id": customerID,
        "priority":    "normal",
    })
    
    return nil
}
```

### Пользовательский брокер событий

```go
// Пользовательский брокер событий на основе Redis
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
        logger.Error("Не удалось разобрать Redis URL", zap.Error(err))
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
    // Запустить горутину обработки сообщений
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
        // Первый подписчик на это действие - запустить подписку Redis
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
        b.logger.Error("Не удалось десериализовать сообщение", zap.Error(err))
        return
    }
    
    b.mu.RLock()
    handlers := b.subscribers[action]
    b.mu.RUnlock()
    
    for _, handler := range handlers {
        go func(h types.ActionHandler) {
            if err := h(&message); err != nil {
                b.logger.Error("Обработчик событий провалился",
                    zap.String("action", action),
                    zap.Error(err))
            }
        }(handler)
    }
}

// Зарегистрировать пользовательский брокер
func init() {
    action.RegisterActionBroker("redis", func(config interface{}) (types.ActionBroker, error) {
        cfg := config.(map[string]interface{})
        redisURL := cfg["url"].(string)
        
        return NewRedisEventBroker(redisURL, sai.Logger()), nil
    })
}
```

Конфигурация для пользовательского брокера:
```yaml
actions:
  broker:
    enabled: true
    type: "redis"
    config:
      url: "redis://localhost:6379/0"
```

## 🔗 Веб-хуки

Фреймворк предоставляет комплексную систему веб-хуков для получения и управления веб-хуками.

### Конфигурация

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

### API управления веб-хуками

Фреймворк автоматически предоставляет конечные точки управления веб-хуками:

```bash
# Создать веб-хук
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

# Список веб-хуков
GET /api/webhooks

# Получить конкретный веб-хук
GET /api/webhooks/{webhook_id}

# Обновить веб-хук
PUT /api/webhooks/{webhook_id}
{
  "enabled": false
}

# Удалить веб-хук
DELETE /api/webhooks/{webhook_id}
```

### Автоматическое создание веб-хука

Если список событий предоставлен в разделе клиента:

```yaml
services:
    user_service:
      url: "http://user-service:8080"
      auth:
        provider: "token"
        payload:
          token: "токен-сервис-к-сервису"
      events: ["user.created", "user.updated"]
```

Сервис автоматически создаёт веб-хук когда ваши учётные данные аутентификации корректны. Всё что вам нужно сделать теперь - это подписаться.

### Получение веб-хуков

```go
func setupWebhookHandlers() {
    actions := sai.Actions()
    
    // Обработать входящие веб-хуки от внешних сервисов
    actions.Subscribe("external.payment.completed", handlePaymentWebhook)
    actions.Subscribe("external.user.verification", handleVerificationWebhook)
}

func handlePaymentWebhook(msg *types.ActionMessage) error {
    sai.Logger().Info("Получен веб-хук платежа",
        zap.String("source", msg.Source),
        zap.Time("timestamp", msg.Timestamp))
    
    // Проверить подлинность веб-хука
    if msg.Source != "webhook" {
        return types.NewError("неверный источник веб-хука")
    }
    
    // Извлечь данные платежа
    paymentData := msg.Payload.(map[string]interface{})
    paymentID := paymentData["payment_id"].(string)
    status := paymentData["status"].(string)
    
    // Обновить статус платежа в базе данных
    if err := updatePaymentStatus(paymentID, status); err != nil {
        return err
    }
    
    // Опубликовать внутреннее событие
    sai.Actions().Publish("payment.status.updated", map[string]interface{}{
        "payment_id": paymentID,
        "status":     status,
        "updated_at": time.Now(),
    })
    
    return nil
}
```

### Безопасность веб-хуков

```go
func verifyWebhookSignature(payload []byte, signature, secret string) bool {
    // Проверка HMAC SHA256
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
    // Формат подписи Stripe: t=timestamp,v1=signature
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
    
    // Проверить допустимость временной метки
    ts, err := strconv.ParseInt(timestamp, 10, 64)
    if err != nil {
        return false
    }
    
    if time.Since(time.Unix(ts, 0)) > 5*time.Minute {
        return false
    }
    
    // Проверить подпись
    signedPayload := timestamp + "." + string(payload)
    return verifyWebhookSignature([]byte(signedPayload), sig, secret)
}
```

## ⏰ Cron задачи

Фреймворк предоставляет надёжный планировщик cron задач с мониторингом и обработкой ошибок.

### Конфигурация

```yaml
cron:
  enabled: true
  timezone: "UTC"  # или "Europe/Moscow", "America/New_York" и т.д.
```

### Базовые Cron задачи

```go
func setupCronJobs() {
    cron := sai.Cron()
    
    // Ежедневная очистка в 2:00 утра
    cron.Add("daily_cleanup", "0 2 * * *", func() {
        sai.Logger().Info("Начинаем ежедневную очистку")
        
        if err := cleanupExpiredSessions(); err != nil {
            sai.Logger().Error("Очистка сессий провалилась", zap.Error(err))
        }
        
        if err := cleanupOldLogs(); err != nil {
            sai.Logger().Error("Очистка логов провалилась", zap.Error(err))
        }
        
        sai.Logger().Info("Ежедневная очистка завершена")
    })
    
    // Проверка здоровья каждые 5 минут
    cron.Add("health_check", "*/5 * * * *", func() {
        if err := performSystemHealthCheck(); err != nil {
            sai.Logger().Error("Проверка здоровья провалилась", zap.Error(err))
            
            // Отправить уведомление
            sai.Actions().Publish("system.health.critical", map[string]interface{}{
                "error":     err.Error(),
                "timestamp": time.Now(),
            })
        }
    })
    
    // Генерировать отчёты каждый понедельник в 9:00 утра
    cron.Add("weekly_report", "0 9 * * 1", func() {
        sai.Logger().Info("Генерируем недельный отчёт")
        
        report, err := generateWeeklyReport()
        if err != nil {
            sai.Logger().Error("Генерация отчёта провалилась", zap.Error(err))
            return
        }
        
        if err := emailReport(report); err != nil {
            sai.Logger().Error("Не удалось отправить отчёт по email", zap.Error(err))
        }
        
        sai.Logger().Info("Недельный отчёт сгенерирован и отправлен")
    })
    
    // Прогрев кэша каждый час
    cron.Add("cache_warming", "0 * * * *", func() {
        warmupCaches()
    })
    
    // Сбор метрик каждую минуту
    cron.Add("metrics_collection", "* * * * *", func() {
        collectCustomMetrics()
    })
}
```

### Продвинутые Cron задачи

```go
func setupAdvancedCronJobs() {
    cron := sai.Cron()
    
    // Резервное копирование базы данных каждый день в 3:00 утра
    cron.Add("db_backup", "0 3 * * *", func() {
        backupDatabase()
    })
    
    // Обработка ожидающих писем каждые 2 минуты
    cron.Add("email_processor", "*/2 * * * *", func() {
        processEmailQueue()
    })
    
    // Очистка временных файлов каждые 6 часов
    cron.Add("temp_cleanup", "0 */6 * * *", func() {
        cleanupTempFiles()
    })
    
    // Обновление валютных курсов ежедневно в полночь
    cron.Add("exchange_rates", "0 0 * * *", func() {
        updateExchangeRates()
    })
    
    // Генерация миниатюр для новых изображений каждые 30 секунд
    cron.Add("thumbnail_generator", "*/30 * * * * *", func() {
        generatePendingThumbnails()
    })
}

func backupDatabase() {
    sai.Logger().Info("Начинаем резервное копирование базы данных")
    
    // Создать имя файла резервной копии с временной меткой
    timestamp := time.Now().Format("20060102_150405")
    backupFile := fmt.Sprintf("/backups/db_backup_%s.sql", timestamp)
    
    // Выполнить резервное копирование
    if err := createDatabaseBackup(backupFile); err != nil {
        sai.Logger().Error("Резервное копирование базы данных провалилось", zap.Error(err))
        
        // Отправить уведомление
        sai.Actions().Publish("backup.failed", map[string]interface{}{
            "type":      "database",
            ""file":      backupFile,
            "error":     err.Error(),
            "timestamp": time.Now(),
        })
        return
    }
    
    // Загрузить в облачное хранилище
    if err := uploadToCloud(backupFile); err != nil {
        sai.Logger().Error("Загрузка резервной копии провалилась", zap.Error(err))
    }
    
    // Очистить старые резервные копии (сохранить последние 7 дней)
    cleanupOldBackups(7)
    
    sai.Logger().Info("Резервное копирование базы данных завершено", zap.String("file", backupFile))
}

func processEmailQueue() {
    emails, err := getPendingEmails(100) // Получить до 100 ожидающих писем
    if err != nil {
        sai.Logger().Error("Не удалось получить ожидающие письма", zap.Error(err))
        return
    }
    
    if len(emails) == 0 {
        return // Нет писем для обработки
    }
    
    sai.Logger().Info("Обработка очереди писем", zap.Int("count", len(emails)))
    
    for _, email := range emails {
        if err := sendEmail(email); err != nil {
            sai.Logger().Error("Не удалось отправить письмо",
                zap.Error(err),
                zap.String("email_id", email.ID))
            
            // Отметить как провалившееся и повторить позже
            markEmailFailed(email.ID, err.Error())
        } else {
            // Отметить как отправленное
            markEmailSent(email.ID)
        }
    }
}

func generatePendingThumbnails() {
    images, err := getImagesNeedingThumbnails(50)
    if err != nil {
        sai.Logger().Error("Не удалось получить изображения, требующие миниатюр", zap.Error(err))
        return
    }
    
    if len(images) == 0 {
        return
    }
    
    for _, image := range images {
        if err := generateThumbnail(image); err != nil {
            sai.Logger().Error("Генерация миниатюры провалилась",
                zap.Error(err),
                zap.String("image_id", image.ID))
        } else {
            markThumbnailGenerated(image.ID)
        }
    }
}
```

### Примеры Cron выражений

```go
// Формат cron выражений: секунда минута час день месяц деньНедели
// (секунды опциональны - используйте 5 полей для точности до минуты)

var cronExamples = map[string]string{
    // Каждую минуту
    "* * * * *": "каждую минуту",
    
    // Каждые 5 минут
    "*/5 * * * *": "каждые 5 минут",
    
    // Каждый час на 30-й минуте
    "30 * * * *": "каждый час на 30-й минуте",
    
    // Каждый день в 2:30 утра
    "30 2 * * *": "каждый день в 2:30 утра",
    
    // Каждый понедельник в 9:00 утра
    "0 9 * * 1": "каждый понедельник в 9:00 утра",
    
    // Каждый рабочий день в 6:00 вечера
    "0 18 * * 1-5": "каждый рабочий день в 6:00 вечера",
    
    // Первый день каждого месяца в полночь
    "0 0 1 * *": "первый день каждого месяца в полночь",
    
    // Каждые 30 секунд (6-польный формат)
    "*/30 * * * * *": "каждые 30 секунд",
    
    // Каждые четверть часа
    "0 */15 * * *": "каждые четверть часа",
    
    // Дважды в день (8 утра и 8 вечера)
    "0 8,20 * * *": "дважды в день в 8 утра и 8 вечера",
}
```

## ❤️ Менеджер здоровья

Фреймворк предоставляет комплексный мониторинг здоровья со встроенными и пользовательскими проверками здоровья.

### Конфигурация

```yaml
health:
  enabled: true
```

### Встроенные конечные точки здоровья

- `GET /health` - Комплексный отчёт о здоровье
- `GET /version` - Версия сервиса и информация о сборке

### Встроенные проверки здоровья

```go
func setupHealthChecks() {
    health := sai.Health()
    
    // Проверка здоровья базы данных
    health.RegisterChecker("database", func(ctx context.Context) types.HealthCheck {
        // Проверить подключение к базе данных
        if err := db.PingContext(ctx); err != nil {
            return types.HealthCheck{
                Status:  types.StatusUnhealthy,
                Message: "Срок действия лицензии истёк",
                Details: map[string]interface{}{
                    "expired_at": license.ExpiresAt,
                    "days_expired": int(time.Since(license.ExpiresAt).Hours() / 24),
                },
            }
        }
        
        daysUntilExpiry := int(time.Until(license.ExpiresAt).Hours() / 24)
        
        status := types.StatusHealthy
        message := "Лицензия действительна"
        
        if daysUntilExpiry <= 7 {
            status = types.StatusUnhealthy
            message = "Срок действия лицензии скоро истекает"
        } else if daysUntilExpiry <= 30 {
            message = "Срок действия лицензии истекает в течение 30 дней"
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
    
    // Проверить сервис флагов функций
    health.RegisterChecker("feature_flags", func(ctx context.Context) types.HealthCheck {
        start := time.Now()
        flags, err := getFeatureFlags()
        responseTime := time.Since(start)
        
        if err != nil {
            return types.HealthCheck{
                Status:  types.StatusUnhealthy,
                Message: "Сервис флагов функций недоступен",
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
            Message: "Сервис флагов функций работает",
            Details: map[string]interface{}{
                "flags_count":      len(flags),
                "response_time_ms": responseTime.Milliseconds(),
            },
        }
    })
}
```

### Формат ответа проверки здоровья

```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "uptime": "72h15m30s",
  "service": {
    "name": "Пользовательский Сервис",
    "version": "2.1.0",
    "host": "api.example.com",
    "port": 8080
  },
  "checks": {
    "database": {
      "status": "healthy",
      "message": "База данных работает",
      "last_check": "2024-01-15T10:30:00Z",
      "duration": "15ms",
      "details": {
        "query_time_ms": 12,
        "connections": 5
      }
    },
    "redis": {
      "status": "healthy",
      "message": "Redis работает",
      "last_check": "2024-01-15T10:30:00Z",
      "duration": "8ms",
      "details": {
        "ping_time_ms": 5,
        "memory_usage": "45MB"
      }
    },
    "user_service": {
      "status": "unhealthy",
      "message": "Пользовательский сервис вернул 503",
      "last_check": "2024-01-15T10:30:00Z",
      "duration": "5s",
      "details": {
        "status_code": 503,
        "error": "Сервис временно недоступен"
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

### Использование данных здоровья

```go
func monitorHealth() {
    health := sai.Health()
    
    // Получить текущий статус здоровья
    report := health.Check(context.Background())
    
    if report.Status != types.StatusHealthy {
        sai.Logger().Error("Проверка здоровья сервиса провалилась",
            zap.String("overall_status", string(report.Status)),
            zap.Int("unhealthy_checks", report.Summary.Unhealthy))
        
        // Отправить уведомление
        sendHealthAlert(report)
    }
    
    // Залогировать метрики здоровья
    for name, check := range report.Checks {
        sai.Logger().Debug("Результат проверки здоровья",
            zap.String("check", name),
            zap.String("status", string(check.Status)),
            zap.Duration("duration", check.Duration))
    }
}

func sendHealthAlert(report types.HealthReport) {
    // Найти провалившиеся проверки
    var failedChecks []string
    for name, check := range report.Checks {
        if check.Status == types.StatusUnhealthy {
            failedChecks = append(failedChecks, name)
        }
    }
    
    // Отправить уведомление
    sai.Actions().Publish("health.alert", map[string]interface{}{
        "service":       report.Service.Name,
        "status":        report.Status,
        "failed_checks": failedChecks,
        "timestamp":     report.Timestamp,
        "uptime":        report.Uptime.String(),
    })
}
```

## 📊 Менеджер метрик

Фреймворк предоставляет комплексный сбор метрик с поддержкой Prometheus и пользовательских провайдеров.

### Конфигурация

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
    port: 9090  # 0 = тот же порт что и основной сервер
  collectors:
    system: true      # Метрики CPU, памяти, диска
    runtime: true     # Метрики среды выполнения Go
    http: true        # Метрики HTTP запросов
    cache: true       # Метрики операций кэша
    middleware: true  # Метрики промежуточного ПО
```

### Встроенные метрики

Фреймворк автоматически собирает следующие метрики:

#### HTTP метрики
- `http_requests_total` - Общее количество HTTP запросов
- `http_request_duration_seconds` - Гистограмма длительности запросов
- `http_request_size_bytes` - Гистограмма размера запросов
- `http_response_size_bytes` - Гистограмма размера ответов

#### Системные метрики
- `system_cpu_usage` - Процент использования CPU
- `system_memory_usage_bytes` - Использование памяти
- `system_disk_usage_bytes` - Использование диска
- `system_load_average` - Средняя нагрузка системы

#### Метрики среды выполнения
- `go_goroutines` - Количество горутин
- `go_threads` - Количество OS потоков
- `go_gc_duration_seconds` - Длительность GC
- `go_memstats_*` - Статистика памяти

### Использование пользовательских метрик

```go
func useCustomMetrics() {
    metrics := sai.Metrics()
    
    // Счётчик - монотонно возрастающее значение
    userRegistrations := metrics.Counter("user_registrations_total", map[string]string{
        "source": "web",
    })
    
    // Датчик - значение которое может увеличиваться или уменьшаться
    activeConnections := metrics.Gauge("active_connections", nil)
    
    // Гистограмма - распределение значений
    requestDuration := metrics.Histogram(
        "api_request_duration_seconds",
        []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0},
        map[string]string{"endpoint": "users"},
    )
    
    // Сводка - квантили в скользящем временном окне
    responseSize := metrics.Summary(
        "api_response_size_bytes",
        map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
        map[string]string{"endpoint": "users"},
    )
    
    // Использовать метрики
    userRegistrations.Inc()
    activeConnections.Set(42)
    requestDuration.Observe(1.2)
    responseSize.Observe(1024)
}

func setupBusinessMetrics() {
    metrics := sai.Metrics()
    
    // Метрики электронной коммерции
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
    
    // Метрики времени обработки
    processingDuration := metrics.Histogram(
        "order_processing_duration_seconds",
        []float64{0.1, 0.5, 1.0, 5.0, 10.0, 30.0},
        map[string]string{"step": "validation"},
    )
    
    // Метрики использования
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

### Метрики в обработчиках

```go
func handleWithMetrics(ctx *types.RequestCtx) {
    start := time.Now()
    
    // Получить метрики
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
    
    // Отслеживать активные запросы
    activeRequests.Inc()
    defer activeRequests.Dec()
    
    // Отслеживать длительность запроса
    defer requestDuration.ObserveDuration(start)
    
    // Обработать запрос
    result, err := processRequest(ctx)
    
    // Записать метрики на основе результата
    if err != nil {
        errorCounter := metrics.Counter("api_errors_total", map[string]string{
            "path":  string(ctx.Path()),
            "error": "processing_failed",
        })
        errorCounter.Inc()
        
        ctx.Error(err, 500)
        requestCounter.Add(1)  // Подсчитать провалившиеся запросы
        return
    }
    
    // Записать успех
    requestCounter.Inc()
    
    // Записать бизнес метрики
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

### Пользовательский провайдер метрик

```go
// Пользовательский провайдер метрик DataDog
type DataDogMetrics struct {
    client dogstatsd.ClientInterface
    logger types.Logger
    prefix string
}

func NewDataDogMetrics(addr, prefix string, logger types.Logger) *DataDogMetrics {
    client, err := dogstatsd.New(addr)
    if err != nil {
        logger.Error("Не удалось создать DataDog клиент", zap.Error(err))
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

// Реализовать DataDogCounter, DataDogGauge, DataDogHistogram...

// Зарегистрировать пользовательский провайдер метрик
func init() {
    metrics.RegisterMetricsManager("datadog", func(config interface{}) (types.MetricsManager, error) {
        cfg := config.(map[string]interface{})
        addr := cfg["addr"].(string)
        prefix := cfg["prefix"].(string)
        
        return NewDataDogMetrics(addr, prefix, sai.Logger()), nil
    })
}
```

Конфигурация для пользовательских метрик:
```yaml
metrics:
  enabled: true
  type: "datadog"
  config:
    addr: "localhost:8125"
    prefix: "myservice"
```

### Панель метрик

При использовании Prometheus вы можете создать панели Grafana с этими запросами:

```promql
# Скорость запросов
rate(http_requests_total[5m])

# Скорость ошибок
rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m])

# Перцентили времени ответа
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Активные соединения
go_goroutines

# Использование памяти
go_memstats_alloc_bytes

# Коэффициент попаданий в кэш
cache_hit_rate

# Бизнес метрики
rate(orders_total[5m])
increase(revenue_total[1h])
```

## 🛡️ TLS Менеджер

Фреймворк предоставляет автоматическое управление TLS сертификатами с интеграцией Let's Encrypt.

### Конфигурация

```yaml
server:
  tls:
    enabled: true
    auto_cert: true                    # Использовать Let's Encrypt
    domains: ["api.example.com"]       # Домены для сертификатов
    email: "admin@example.com"         # Email для Let's Encrypt
    cache_dir: "./certs"               # Директория кэша сертификатов
    acme_directory: ""                 # Пользовательская ACME директория (опционально)
    # Ручные сертификаты (альтернатива auto_cert)
    cert_file: "/path/to/cert.pem"     # Файл сертификата
    key_file: "/path/to/key.pem"       # Файл приватного ключа
```

### Автоматические сертификаты (Let's Encrypt)

```go
func setupAutoTLS() {
    // TLS настраивается автоматически из config.yml
    // Фреймворк будет:
    // 1. Запрашивать сертификаты от Let's Encrypt
    // 2. Автоматически обрабатывать ACME вызовы
    // 3. Обновлять сертификаты до истечения срока
    // 4. Обслуживать HTTPS трафик
    
    router := sai.Router()
    
    // Все маршруты автоматически используют HTTPS когда TLS включён
    router.GET("/api/secure", func(ctx *types.RequestCtx) {
        ctx.SuccessJSON(map[string]interface{}{
            "secure":     true,
            "protocol":   "https",
            "cert_info":  getCertificateInfo(ctx),
        })
    })
}

func getCertificateInfo(ctx *types.RequestCtx) map[string]interface{} {
    // Извлечь информацию о сертификате из запроса
    return map[string]interface{}{
        "tls_version": "TLS 1.3",
        "cipher":      "ECDHE-RSA-AES256-GCM-SHA384",
        "server_name": string(ctx.Host()),
    }
}
```

### Ручные сертификаты

```yaml
server:
  tls:
    enabled: true
    auto_cert: false
    cert_file: "/etc/ssl/certs/server.crt"
    key_file: "/etc/ssl/private/server.key"
```

### Мониторинг сертификатов

```go
func setupCertificateMonitoring() {
    // TLS менеджер автоматически предоставляет статус сертификата
    router := sai.Router()
    
    router.GET("/admin/certificates", func(ctx *types.RequestCtx) {
        // Эта конечная точка должна быть защищена аутентификацией администратора
        tlsManager := getTLSManager() // Получить из контейнера сервиса
        
        if tlsManager == nil {
            ctx.Error(types.NewError("TLS не включён"), 404)
            return
        }
        
        status := tlsManager.GetCertificateStatus()
        ctx.SuccessJSON(status)
    }).WithMiddlewares("auth") // Требуется аутентификация администратора
}

// Формат ответа статуса сертификата:
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

### TLS заголовки безопасности

```go
func setupSecurityHeaders() {
    // Добавить промежуточное ПО безопасности для HTTPS
    router := sai.Router()
    
    // Все маршруты получают заголовки безопасности когда TLS включён
    router.Use(func(ctx *types.RequestCtx, next func(*types.RequestCtx), config *types.RouteConfig) {
        if isTLSEnabled() {
            // HSTS - принудительный HTTPS для будущих запросов
            ctx.Response.Header.Set("Strict-Transport-Security", 
                "max-age=31536000; includeSubDomains; preload")
            
            // Предотвратить атаки понижения версии
            ctx.Response.Header.Set("Upgrade-Insecure-Requests", "1")
            
            // Безопасность контента
            ctx.Response.Header.Set("X-Content-Type-Options", "nosniff")
            ctx.Response.Header.Set("X-Frame-Options", "DENY")
            ctx.Response.Header.Set("X-XSS-Protection", "1; mode=block")
            
            // Политика реферера
            ctx.Response.Header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
        }
        
        next(ctx)
    })
}
```

### Перенаправление HTTP на HTTPS

```go
func setupHTTPSRedirect() {
    // Когда TLS включён, автоматически перенаправлять HTTP на HTTPS
    
    if isTLSEnabled() {
        // Запустить HTTP сервер для перенаправлений
        go func() {
            redirectServer := &fasthttp.Server{
                Handler: func(ctx *fasthttp.RequestCtx) {
                    // Перенаправить на HTTPS
                    httpsURL := fmt.Sprintf("https://%s%s", 
                        ctx.Host(), ctx.RequestURI())
                    
                    ctx.Redirect(httpsURL, fasthttp.StatusMovedPermanently)
                },
            }
            
            httpAddr := fmt.Sprintf("%s:80", getServerHost())
            sai.Logger().Info("Запуск HTTP сервера перенаправлений", 
                zap.String("addr", httpAddr))
            
            if err := redirectServer.ListenAndServe(httpAddr); err != nil {
                sai.Logger().Error("HTTP сервер перенаправлений провалился", zap.Error(err))
            }
        }()
    }
}
```

### Продакшн настройка TLS

```bash
# Переменные среды продакшн окружения
export TLS_ENABLED=true
export TLS_AUTO_CERT=true
export TLS_DOMAINS=api.example.com,www.api.example.com
export TLS_EMAIL=admin@example.com

# Docker развёртывание с TLS
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

### Мониторинг обновления сертификатов

```go
func setupCertificateAlerts() {
    // Мониторить истечение срока сертификатов
    cron := sai.Cron()
    
    cron.Add("certificate_check", "0 */12 * * *", func() {
        tlsManager := getTLSManager()
        if tlsManager == nil {
            return
        }
        
        status := tlsManager.GetCertificateStatus()
        
        for domain, cert := range status {
            if cert.Status == "expiring_soon" || cert.DaysUntilExpiry <= 7 {
                // Отправить уведомление
                sai.Actions().Publish("certificate.expiring", map[string]interface{}{
                    "domain":             domain,
                    "days_until_expiry":  cert.DaysUntilExpiry,
                    "not_after":          cert.NotAfter,
                })
                
                sai.Logger().Warn("Срок действия сертификата скоро истекает",
                    zap.String("domain", domain),
                    zap.Int("days_until_expiry", cert.DaysUntilExpiry))
            }
        }
    })
}
```

---

## 📄 Лицензия

MIT Лицензия - см. файл LICENSE для подробностей.

**Создавайте мощные Go сервисы за минуты, а не дни!**
