# SAI Service Generator

🚀 **Мощный генератор Go-сервисов с полным набором функций для современной разработки**

SAI Service Generator - это интеллектуальный инструмент для создания высококачественных Go-сервисов с поддержкой REST API, WebSocket, кэширования, метрик, аутентификации и многого другого.

## ✨ Основные возможности

### 🏗️ Готовые шаблоны
- **Basic** - Минимальный веб-сервер
- **API** - REST API сервис с CRUD операциями
- **Microservice** - Микросервис с системой событий
- **Full** - Полнофункциональный сервис со всеми возможностями
- **Custom** - Настраиваемая конфигурация

### 🔧 Встроенные компоненты
- ⚡ **FastHTTP** - Высокопроизводительный HTTP сервер
- 🔐 **Аутентификация** - Basic Auth, Token Auth
- 💾 **Кэширование** - Memory, Redis
- 📊 **Метрики** - Memory, Prometheus
- 📚 **Документация** - Автогенерация OpenAPI/Swagger
- ⏰ **Планировщик** - Cron задачи
- 🔄 **События** - WebSocket, Webhooks
- 🛡️ **TLS/SSL** - Автоматические сертификаты
- 🌐 **HTTP клиент** - Circuit breaker, retry
- ❤️ **Health checks** - Мониторинг состояния

### 🚧 Middleware
- 🛡️ Recovery - Обработка паник
- 📝 Logging - Структурированные логи
- 🚦 Rate Limiting - Ограничение запросов
- 📏 Body Limit - Ограничение размера тела
- 🌍 CORS - Настройка политик
- 🔒 Auth - Аутентификация
- 🗜️ Compression - Сжатие ответов
- 💾 Cache - Кэширование ответов

## 🚀 Быстрый старт

### Установка

```bash
# Клонируйте репозиторий
git clone <repository-url>
cd sai-service-generator

# Сделайте скрипт исполняемым
chmod +x generate.sh
```

### Использование

#### Интерактивный режим (рекомендуется)
```bash
./generate.sh
```

#### Командная строка
```bash
./generate.sh --name "My API" --pkg "github.com/user/my-api" --features "auth,cache,metrics"
```

## 📋 Примеры использования

### 1. Простой API сервис
```bash
./generate.sh \
  --name "User API" \
  --pkg "github.com/company/user-api" \
  --features "auth,cache,docs" \
  --auth "token" \
  --cache "redis" \
  --middlewares "auth,recovery,logging,cors"
```

### 2. Микросервис с событиями
```bash
./generate.sh \
  --name "Notification Service" \
  --pkg "github.com/company/notifications" \
  --features "actions,webhooks,metrics,health" \
  --actions "websocket,webhook" \
  --metrics "prometheus"
```

### 3. Полнофункциональный сервис
```bash
./generate.sh \
  --name "Enterprise API" \
  --pkg "github.com/company/enterprise-api" \
  --features "auth,cache,metrics,docs,cron,actions,tls,middlewares,health,client" \
  --auth "basic,token" \
  --cache "redis" \
  --metrics "prometheus" \
  --test \
  --cicd "github"
```

## 🎯 Параметры командной строки

### Основные параметры
| Параметр | Описание | Пример |
|----------|----------|---------|
| `--name` | Название проекта | `"My Service"` |
| `--pkg` | Go пакет | `"github.com/user/project"` |
| `--features` | Список функций через запятую | `"auth,cache,metrics"` |

### Функции (--features)
| Функция | Описание |
|---------|----------|
| `auth` | Система аутентификации |
| `cache` | Система кэширования |
| `metrics` | Сбор метрик |
| `docs` | API документация |
| `cron` | Планировщик задач |
| `actions` | Система событий |
| `tls` | TLS/SSL поддержка |
| `middlewares` | Middleware компоненты |
| `health` | Health checks |
| `client` | HTTP клиент |

### Дополнительные параметры
| Параметр | Значения | Описание |
|----------|----------|----------|
| `--auth` | `basic,token` | Типы аутентификации |
| `--cache` | `memory,redis` | Тип кэша |
| `--metrics` | `memory,prometheus` | Тип метрик |
| `--actions` | `websocket,webhook` | Типы событий |
| `--middlewares` | См. список ниже | Middleware |
| `--test` | - | Включить тесты |
| `--cicd` | `github,gitlab,none` | CI/CD система |

### Middleware (--middlewares)
```
auth,bodylimit,cache,compression,cors,logging,ratelimit,recovery
```

## 📁 Структура проекта

Генератор создает следующую структуру:

```
my-service/
├── cmd/
│   └── main.go              # Точка входа
├── internal/
│   ├── handlers.go          # HTTP обработчики
│   └── service.go           # Бизнес-логика
├── types/
│   └── types.go             # Типы данных
├── scripts/
│   └── docker-entrypoint.sh # Docker entrypoint
├── tests/                   # Интеграционные тесты
├── .github/workflows/       # GitHub Actions (опционально)
├── config.template.yml      # Шаблон конфигурации
├── .env.example            # Переменные окружения
├── docker-compose.yml      # Docker Compose
├── Dockerfile              # Docker образ
├── Makefile               # Команды сборки
├── go.mod                 # Go модуль
└── README.md              # Документация
```

## 🛠️ Команды сборки

После генерации проекта доступны следующие команды:

```bash
# Сборка и запуск
make run

# Только сборка
make build

# Тестирование
make test

# Форматирование кода
make fmt

# Линтинг
make lint

# Docker
make docker-build
make docker-run
make docker-stop

# Очистка
make clean
```

## 🔧 Конфигурация

### Переменные окружения
1. Скопируйте `.env.example` в `.env`
2. Настройте переменные под ваши нужды
3. Конфигурация автоматически генерируется из шаблона

### Основные настройки
```env
# Сервер
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# Логирование
LOGGER_LEVEL=info

# Кэш (Redis)
CACHE_ENABLED=true
REDIS_HOST=localhost
REDIS_PORT=6379

# Метрики (Prometheus)
METRICS_ENABLED=true
METRICS_HTTP_PORT=9090

# Аутентификация
AUTH_TOKEN=your-secret-token
```

## 📊 API Endpoints

### Базовые endpoints
- `GET /api/v1/hello` - Тестовый endpoint
- `GET /health` - Health check
- `GET /version` - Версия сервиса

### CRUD API (шаблон API)
- `POST /api/v1/documents/` - Создание
- `GET /api/v1/documents/` - Чтение
- `PUT /api/v1/documents/` - Обновление
- `DELETE /api/v1/documents/` - Удаление

### Дополнительные endpoints
- `GET /metrics` - Метрики Prometheus
- `GET /docs` - Swagger документация
- `POST /api/webhooks` - Управление webhooks

## 🐳 Docker

### Локальная разработка
```bash
# Запуск всех сервисов
docker-compose up -d

# Только приложение
docker-compose up app

# Просмотр логов
docker-compose logs -f app
```

### Production build
```bash
# Сборка образа
docker build -t my-service:latest .

# Запуск контейнера
docker run -p 8080:8080 --env-file .env my-service:latest
```

## 🔄 CI/CD

### GitHub Actions
Генератор может создать готовые workflows для:
- Тестирование кода
- Сборка бинарных файлов
- Docker образы
- Деплой

### GitLab CI
Поддержка GitLab CI с:
- Параллельным тестированием
- Кэшированием зависимостей
- Multi-stage сборкой

## 🧪 Тестирование

```bash
# Юнит тесты
go test ./...

# Интеграционные тесты
make test

# С покрытием
go test -cover ./...
```

## 📈 Мониторинг

### Prometheus метрики
- HTTP запросы и ответы
- Время выполнения
- Ошибки и статус коды
- Системные метрики
- Custom метрики

### Health checks
- Состояние компонентов
- Доступность баз данных
- Производительность

## 🔒 Безопасность

### Аутентификация
- Bearer token аутентификация
- Basic Auth с realm
- Middleware для защиты endpoints

### TLS/SSL
- Автоматические Let's Encrypt сертификаты
- Настраиваемые сертификаты
- HTTP -> HTTPS редирект

## 🎭 Примеры шаблонов

### Basic Template
```yaml
features: "health,cache"
middlewares: "recovery,logging"
cache_type: "memory"
```

### API Template
```yaml
features: "health,middlewares,docs,cache"
middlewares: "auth,cache,recovery,logging,cors,bodylimit"
auth_types: "token"
cache_type: "redis"
```

### Full Template
```yaml
features: "auth,cache,metrics,docs,cron,actions,tls,middlewares,health,client"
middlewares: "auth,bodylimit,cache,compression,cors,logging,ratelimit,recovery"
auth_types: "basic,token"
cache_type: "redis"
metrics_type: "prometheus"
actions: "websocket,webhook"
```

## 🤝 Вклад в проект

1. Форкните репозиторий
2. Создайте feature branch
3. Внесите изменения
4. Добавьте тесты
5. Создайте Pull Request

## 📄 Лицензия

MIT License - см. файл LICENSE для деталей.

## 🆘 Поддержка

- 📧 Email: support@sai-service.com
- 💬 Discord: [Сообщество SAI](https://discord.gg/sai)
- 📖 Документация: [docs.sai-service.com](https://docs.sai-service.com)
- 🐛 Issues: [GitHub Issues](https://github.com/sai-service/generator/issues)

---

**Создавайте мощные Go-сервисы за минуты, а не дни! 🚀**