# Makefile для тестирования производительности микросервиса

.PHONY: help build run test-perf test-quick test-stress install-tools clean

# Переменные
SERVICE_NAME = perf-test-service
SERVICE_PORT = 8080
SERVICE_URL = http://localhost:$(SERVICE_PORT)

help: ## Показать доступные команды
	@echo "Доступные команды:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

install-tools: ## Установить инструменты для тестирования
	@echo "🔧 Installing performance testing tools..."
	@if ! command -v hey >/dev/null 2>&1; then \
		echo "Installing hey..."; \
		go install github.com/rakyll/hey@latest; \
	fi
	@if ! command -v wrk >/dev/null 2>&1; then \
		echo "Installing wrk (требует apt/brew)..."; \
		if command -v apt >/dev/null 2>&1; then \
			sudo apt update && sudo apt install -y wrk; \
		elif command -v brew >/dev/null 2>&1; then \
			brew install wrk; \
		else \
			echo "❌ Please install wrk manually for your system"; \
		fi \
	fi
	@echo "✅ Tools installation completed"

build: ## Собрать сервис
	@echo "🔨 Building $(SERVICE_NAME)..."
	@go mod tidy
	@go build -ldflags="-s -w" -gcflags="-B -C" -o $(SERVICE_NAME) .
	@echo "✅ Build completed"

run: ## Запустить сервис
	@echo "🚀 Starting $(SERVICE_NAME) on port $(SERVICE_PORT)..."
	@go run . &
	@echo "Service PID: $$!"
	@echo "Service URL: $(SERVICE_URL)"

stop: ## Остановить сервис
	@echo "🛑 Stopping $(SERVICE_NAME)..."
	@pkill -f "go run\\|$(SERVICE_NAME)" || true
	@echo "✅ Service stopped"

test-quick: install-tools ## Быстрый тест производительности (5 минут)
	@echo "⚡ Running quick performance test..."
	@chmod +x performance_test.sh
	@SERVICE_URL=$(SERVICE_URL) QUICK_TEST=true ./performance_test.sh

test-perf: install-tools ## Полный тест производительности (15 минут)
	@echo "🔥 Running full performance test..."
	@chmod +x performance_test.sh
	@SERVICE_URL=$(SERVICE_URL) ./performance_test.sh

test-stress: install-tools ## Стресс-тест (высокая нагрузка)
	@echo "💪 Running stress test..."
	@hey -z 60s -c 1000 -q 0 $(SERVICE_URL)/ping
	@echo "Stress test completed"

test-baseline: install-tools ## Базовый тест для определения baseline производительности
	@echo "📊 Running baseline performance test..."
	@echo "Testing /ping endpoint..."
	@hey -n 10000 -c 100 $(SERVICE_URL)/ping | head -20
	@echo ""
	@echo "Testing /hello/test endpoint..."
	@hey -n 5000 -c 50 $(SERVICE_URL)/hello/test | head -20

monitor: ## Мониторинг ресурсов сервиса в реальном времени
	@echo "📈 Monitoring service resources..."
	@echo "Press Ctrl+C to stop monitoring"
	@watch -n 1 'ps aux | grep -E "(go run|$(SERVICE_NAME))" | grep -v grep; echo ""; netstat -an | grep :$(SERVICE_PORT)'

benchmark-endpoints: install-tools ## Бенчмарк всех эндпоинтов
	@echo "🎯 Benchmarking all endpoints..."
	@echo "=== /ping ==="
	@hey -n 1000 -c 10 $(SERVICE_URL)/ping | grep -E "(Requests/sec|Average|99%)"
	@echo ""
	@echo "=== /hello/benchtest ==="
	@hey -n 1000 -c 10 $(SERVICE_URL)/hello/benchtest | grep -E "(Requests/sec|Average|99%)"
	@echo ""
	@echo "=== /echo (POST) ==="
	@echo '{"name":"bench","data":"test"}' | hey -n 1000 -c 10 -m POST -T "application/json" -D /dev/stdin $(SERVICE_URL)/echo | grep -E "(Requests/sec|Average|99%)"
	@echo ""
	@echo "=== /data ==="
	@hey -n 500 -c 10 $(SERVICE_URL)/data | grep -E "(Requests/sec|Average|99%)"

check-health: ## Проверить здоровье сервиса
	@echo "🏥 Checking service health..."
	@curl -s $(SERVICE_URL)/health | jq . || curl -s $(SERVICE_URL)/health

load-test-scenarios: install-tools ## Запустить различные сценарии нагрузочного тестирования
	@echo "🎭 Running load test scenarios..."
	@echo ""
	@echo "Scenario 1: Light load (normal usage)"
	@hey -z 30s -c 10 -q 100 $(SERVICE_URL)/ping | grep -E "(Total|Requests/sec|Average)"
	@echo ""
	@echo "Scenario 2: Medium load (peak hours)"
	@hey -z 30s -c 50 -q 500 $(SERVICE_URL)/ping | grep -E "(Total|Requests/sec|Average)"
	@echo ""
	@echo "Scenario 3: Heavy load (stress)"
	@hey -z 30s -c 200 -q 0 $(SERVICE_URL)/ping | grep -E "(Total|Requests/sec|Average)"

clean: ## Очистить результаты тестов и артефакты
	@echo "🧹 Cleaning up..."
	@rm -rf ./perf_results
	@rm -f $(SERVICE_NAME)
	@echo "✅ Cleanup completed"

dev: ## Запустить в режиме разработки с live reload
	@echo "🔄 Starting development mode..."
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "Installing air for live reload..."; \
		go install github.com/cosmtrek/air@latest; \
		air; \
	fi

# Комбинированные команды
test-all: clean build run-background test-perf ## Полный цикл: сборка, запуск, тест
	@echo "🎉 Full test cycle completed"

run-background: ## Запустить сервис в фоне
	@echo "🚀 Starting service in background..."
	@nohup go run . > service.log 2>&1 & echo $$! > service.pid
	@sleep 2
	@echo "Service PID: $$(cat service.pid)"
	@echo "Service log: service.log"

stop-background: ## Остановить фоновый сервис
	@if [ -f service.pid ]; then \
		kill $$(cat service.pid) && rm service.pid; \
		echo "✅ Background service stopped"; \
	else \
		echo "❌ No background service found"; \
	fi

# Новые команды для высоких нагрузок
test-high: install-tools ## Высоконагруженный тест (5000 соединений)
	@echo "🔥 Running high load test..."
	@chmod +x quick_high_load.sh
	@./quick_high_load.sh 5000 60 /ping

test-extreme: install-tools ## Экстремальный тест нагрузки
	@echo "💥 Running extreme load test..."
	@chmod +x extreme_load_test.sh
	@./extreme_load_test.sh

test-find-limits: install-tools ## Поиск пределов производительности
	@echo "🔍 Finding performance limits..."
	@chmod +x high_load_test.sh
	@./high_load_test.sh

optimize-system: ## Оптимизация системы для высоких нагрузок
	@echo "🔧 Optimizing system for high load..."
	@chmod +x optimize_system.sh
	@./optimize_system.sh

check-limits: ## Проверка текущих лимитов системы
	@echo "📊 Current system limits:"
	@echo "File descriptors: $(ulimit -n)"
	@echo "Max processes: $(ulimit -u)"
	@echo "Available memory: $(free -h | grep Mem | awk '{print $7}')"
	@echo "CPU cores: $(nproc)"
	@echo "Current connections: $(ss -t | wc -l)"

# Быстрые тесты разной интенсивности
test-light: install-tools ## Легкий тест (100 соединений)
	@hey -n 10000 -c 100 $(SERVICE_URL)/ping

test-medium: install-tools ## Средний тест (1000 соединений)
	@hey -z 60s -c 1000 $(SERVICE_URL)/ping

test-heavy: install-tools ## Тяжелый тест (5000 соединений)
	@hey -z 60s -c 5000 $(SERVICE_URL)/ping

test-insane: install-tools ## Безумный тест (20000 соединений)
	@echo "⚠️  WARNING: This may crash your system!"
	@sleep 3
	@hey -z 30s -c 20000 $(SERVICE_URL)/ping

# Тесты по времени
test-5min: install-tools ## 5-минутный тест выносливости
	@./quick_high_load.sh 1000 300 /ping

test-30min: install-tools ## 30-минутный тест стабильности
	@./quick_high_load.sh 500 1800 /ping

# Сравнительные тесты
compare-endpoints: install-tools ## Сравнение производительности эндпоинтов
	@echo "🎯 Comparing endpoint performance..."
	@echo "=== /ping ==="
	@hey -n 5000 -c 100 $(SERVICE_URL)/ping | grep -E "(Requests/sec|Average)"
	@echo ""
	@echo "=== /hello/test ==="
	@hey -n 5000 -c 100 $(SERVICE_URL)/hello/test | grep -E "(Requests/sec|Average)"
	@echo ""
	@echo "=== /data ==="
	@hey -n 2000 -c 100 $(SERVICE_URL)/data | grep -E "(Requests/sec|Average)"
	@echo ""
	@echo "=== /echo (POST) ==="
	@echo '{"name":"test","data":"benchmark"}' | hey -n 5000 -c 100 -m POST -T "application/json" -D /dev/stdin $(SERVICE_URL)/echo | grep -E "(Requests/sec|Average)"

# Рекомендуемые тесты на основе baseline
recommended-tests: install-tools ## Рекомендуемые тесты на основе ваших 16K RPS
	@echo "🎯 Recommended tests based on your 16K RPS baseline:"
	@echo ""
	@echo "1. Finding sustained maximum (10K connections)..."
	@./quick_high_load.sh 10000 120 /ping
	@echo ""
	@echo "2. Finding breaking point (20K connections)..."
	@./quick_high_load.sh 20000 60 /ping
	@echo ""
	@echo "3. Testing different endpoint..."
	@./quick_high_load.sh 5000 60 /hello/test