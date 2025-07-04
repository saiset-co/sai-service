# SAI Service Generator

🚀 **Powerful Go service generator with a complete feature set for modern development**

SAI Service Generator is an intelligent tool for creating high-quality Go services with support for REST API, WebSocket, caching, metrics, authentication, and much more.

## ✨ Key Features

### 🏗️ Ready-to-use Templates
- **Basic** - Minimal web server
- **API** - REST API service with CRUD operations
- **Microservice** - Microservice with event system
- **Full** - Full-featured service with all capabilities
- **Custom** - Customizable configuration

### 🔧 Built-in Components
- ⚡ **FastHTTP** - High-performance HTTP server
- 🔐 **Authentication** - Basic Auth, Token Auth
- 💾 **Caching** - Memory, Redis
- 📊 **Metrics** - Memory, Prometheus
- 📚 **Documentation** - Auto-generated OpenAPI/Swagger
- ⏰ **Scheduler** - Cron jobs
- 🔄 **Events** - WebSocket, Webhooks
- 🛡️ **TLS/SSL** - Automatic certificates
- 🌐 **HTTP Client** - Circuit breaker, retry
- ❤️ **Health Checks** - Service monitoring

### 🚧 Middleware
- 🛡️ Recovery - Panic handling
- 📝 Logging - Structured logs
- 🚦 Rate Limiting - Request throttling
- 📏 Body Limit - Request size limiting
- 🌍 CORS - Cross-origin policies
- 🔒 Auth - Authentication
- 🗜️ Compression - Response compression
- 💾 Cache - Response caching

## 🚀 Quick Start

### Installation

```bash
# Clone the repository
git clone <repository-url>
cd sai-service-generator

# Make script executable
chmod +x generate.sh
```

### Usage

#### Interactive Mode (Recommended)
```bash
./generate.sh
```

#### Command Line
```bash
./generate.sh --name "My API" --pkg "github.com/user/my-api" --features "auth,cache,metrics"
```

## 📋 Usage Examples

### 1. Simple API Service
```bash
./generate.sh \
  --name "User API" \
  --pkg "github.com/company/user-api" \
  --features "auth,cache,docs" \
  --auth "token" \
  --cache "redis" \
  --middlewares "auth,recovery,logging,cors"
```

### 2. Microservice with Events
```bash
./generate.sh \
  --name "Notification Service" \
  --pkg "github.com/company/notifications" \
  --features "actions,webhooks,metrics,health" \
  --actions "websocket,webhook" \
  --metrics "prometheus"
```

### 3. Full-featured Service
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

## 🎯 Command Line Parameters

### Basic Parameters
| Parameter | Description | Example |
|-----------|-------------|---------|
| `--name` | Project name | `"My Service"` |
| `--pkg` | Go package | `"github.com/user/project"` |
| `--features` | Comma-separated features | `"auth,cache,metrics"` |

### Features (--features)
| Feature | Description |
|---------|-------------|
| `auth` | Authentication system |
| `cache` | Caching system |
| `metrics` | Metrics collection |
| `docs` | API documentation |
| `cron` | Task scheduler |
| `actions` | Event system |
| `tls` | TLS/SSL support |
| `middlewares` | Middleware components |
| `health` | Health checks |
| `client` | HTTP client |

### Additional Parameters
| Parameter | Values | Description |
|-----------|--------|-------------|
| `--auth` | `basic,token` | Authentication types |
| `--cache` | `memory,redis` | Cache type |
| `--metrics` | `memory,prometheus` | Metrics type |
| `--actions` | `websocket,webhook` | Event types |
| `--middlewares` | See list below | Middleware |
| `--test` | - | Include tests |
| `--cicd` | `github,gitlab,none` | CI/CD system |

### Middleware (--middlewares)
```
auth,bodylimit,cache,compression,cors,logging,ratelimit,recovery
```

## 📁 Project Structure

The generator creates the following structure:

```
my-service/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── handlers.go          # HTTP handlers
│   └── service.go           # Business logic
├── types/
│   └── types.go             # Data types
├── scripts/
│   └── docker-entrypoint.sh # Docker entrypoint
├── tests/                   # Integration tests
├── .github/workflows/       # GitHub Actions (optional)
├── config.template.yml      # Configuration template
├── .env.example            # Environment variables
├── docker-compose.yml      # Docker Compose
├── Dockerfile              # Docker image
├── Makefile               # Build commands
├── go.mod                 # Go module
└── README.md              # Documentation
```

## 🛠️ Build Commands

After project generation, the following commands are available:

```bash
# Build and run
make run

# Build only
make build

# Testing
make test

# Code formatting
make fmt

# Linting
make lint

# Docker
make docker-build
make docker-run
make docker-stop

# Clean
make clean
```

## 🔧 Configuration

### Environment Variables
1. Copy `.env.example` to `.env`
2. Configure variables for your needs
3. Configuration is automatically generated from template

### Main Settings
```env
# Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# Logging
LOGGER_LEVEL=info

# Cache (Redis)
CACHE_ENABLED=true
REDIS_HOST=localhost
REDIS_PORT=6379

# Metrics (Prometheus)
METRICS_ENABLED=true
METRICS_HTTP_PORT=9090

# Authentication
AUTH_TOKEN=your-secret-token
```

## 📊 API Endpoints

### Basic Endpoints
- `GET /api/v1/hello` - Test endpoint
- `GET /health` - Health check
- `GET /version` - Service version

### CRUD API (API template)
- `POST /api/v1/documents/` - Create
- `GET /api/v1/documents/` - Read
- `PUT /api/v1/documents/` - Update
- `DELETE /api/v1/documents/` - Delete

### Additional Endpoints
- `GET /metrics` - Prometheus metrics
- `GET /docs` - Swagger documentation
- `POST /api/webhooks` - Webhook management

## 🐳 Docker

### Local Development
```bash
# Start all services
docker-compose up -d

# Application only
docker-compose up app

# View logs
docker-compose logs -f app
```

### Production Build
```bash
# Build image
docker build -t my-service:latest .

# Run container
docker run -p 8080:8080 --env-file .env my-service:latest
```

## 🔄 CI/CD

### GitHub Actions
Generator can create ready workflows for:
- Code testing
- Binary builds
- Docker images
- Deployment

### GitLab CI
GitLab CI support with:
- Parallel testing
- Dependency caching
- Multi-stage builds

## 🧪 Testing

```bash
# Unit tests
go test ./...

# Integration tests
make test

# With coverage
go test -cover ./...
```

## 📈 Monitoring

### Prometheus Metrics
- HTTP requests and responses
- Execution time
- Errors and status codes
- System metrics
- Custom metrics

### Health Checks
- Component status
- Database availability
- Performance metrics

## 🔒 Security

### Authentication
- Bearer token authentication
- Basic Auth with realm
- Middleware for endpoint protection

### TLS/SSL
- Automatic Let's Encrypt certificates
- Custom certificates
- HTTP -> HTTPS redirect

## 🎭 Template Examples

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

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Create a Pull Request

## 📄 License

MIT License - see LICENSE file for details.

**Create powerful Go services in minutes, not days! 🚀**
