#!/bin/bash

# Полный автоматический тест производительности SAI-Service
# Этот скрипт запускает сервис, проводит тесты, собирает метрики и генерирует отчеты

set -e  # Выходить при ошибках

SERVICE_NAME="perf-test-service"
SERVICE_PORT=8081
SERVICE_URL="http://localhost:$SERVICE_PORT"
TEST_DURATION=${1:-300}  # 5 минут по умолчанию
CONCURRENT_USERS=${2:-100}
RESULTS_DIR="./test_results_$(date +%Y%m%d_%H%M%S)"

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Функция логирования
log() {
    echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[$(date '+%H:%M:%S')] ✅ $1${NC}"
}

log_error() {
    echo -e "${RED}[$(date '+%H:%M:%S')] ❌ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}[$(date '+%H:%M:%S')] ⚠️  $1${NC}"
}

# Функция очистки ресурсов
cleanup() {
    log "Cleaning up resources..."

    # Останавливаем сервис
    if [ -f service.pid ]; then
        SERVICE_PID=$(cat service.pid)
        if kill -0 "$SERVICE_PID" 2>/dev/null; then
            log "Stopping service (PID: $SERVICE_PID)..."
            kill "$SERVICE_PID"
            sleep 2
            kill -9 "$SERVICE_PID" 2>/dev/null || true
        fi
        rm -f service.pid
    fi

    # Останавливаем мониторинг
    if [ -f monitor.pid ]; then
        MONITOR_PID=$(cat monitor.pid)
        if kill -0 "$MONITOR_PID" 2>/dev/null; then
            log "Stopping monitor (PID: $MONITOR_PID)..."
            kill "$MONITOR_PID" 2>/dev/null || true
        fi
        rm -f monitor.pid
    fi

    # Очищаем временные файлы
    rm -f service.log monitor.log
}

# Устанавливаем обработчик сигналов
trap cleanup EXIT INT TERM

# Заголовок
echo "🚀 SAI-Service Complete Performance Test Suite"
echo "=============================================="
echo "Test Duration: ${TEST_DURATION}s ($(($TEST_DURATION / 60)) minutes)"
echo "Concurrent Users: $CONCURRENT_USERS"
echo "Results Directory: $RESULTS_DIR"
echo ""

# Создаем директорию для результатов
mkdir -p "$RESULTS_DIR"

# 1. Проверяем зависимости
log "Checking dependencies..."

MISSING_TOOLS=()

if ! command -v go >/dev/null 2>&1; then
    MISSING_TOOLS+=("go")
fi

if ! command -v hey >/dev/null 2>&1; then
    log_warning "Installing hey..."
    go install github.com/rakyll/hey@latest || MISSING_TOOLS+=("hey")
fi

if ! command -v wrk >/dev/null 2>&1; then
    log_warning "wrk not found, some tests will be skipped"
fi

if [ ${#MISSING_TOOLS[@]} -ne 0 ]; then
    log_error "Missing required tools: ${MISSING_TOOLS[*]}"
    echo "Please install missing tools and try again"
    exit 1
fi

log_success "All dependencies satisfied"

# 2. Сборка сервиса
log "Building service..."
if ! go mod tidy; then
    log_error "Failed to tidy go modules"
    exit 1
fi

if ! go build -o "$SERVICE_NAME" .; then
    log_error "Failed to build service"
    exit 1
fi

log_success "Service built successfully"

# 3. Запуск сервиса
log "Starting service on port $SERVICE_PORT..."
"./$SERVICE_NAME" > service.log 2>&1 &
SERVICE_PID=$!
echo $SERVICE_PID > service.pid

# Ждем запуска сервиса
log "Waiting for service to start..."
for i in {1..30}; do
    if curl -s "$SERVICE_URL/health" >/dev/null 2>&1; then
        log_success "Service is running (PID: $SERVICE_PID)"
        break
    fi

    if [ $i -eq 30 ]; then
        log_error "Service failed to start within 30 seconds"
        cat service.log
        exit 1
    fi

    sleep 1
done

# 4. Проверка базовой функциональности
log "Testing basic functionality..."

# Тестируем все эндпоинты
ENDPOINTS=("/ping" "/hello/test" "/data" "/health")
for endpoint in "${ENDPOINTS[@]}"; do
    if ! curl -s "$SERVICE_URL$endpoint" >/dev/null; then
        log_error "Endpoint $endpoint is not responding"
        exit 1
    fi
done

log_success "All endpoints are responding"

# 5. Запуск мониторинга в фоне
log "Starting performance monitoring..."
chmod +x monitor.sh
./monitor.sh "$SERVICE_URL" "$TEST_DURATION" > monitor.log 2>&1 &
MONITOR_PID=$!
echo $MONITOR_PID > monitor.pid

log_success "Monitor started (PID: $MONITOR_PID)"

# 6. Baseline тест
log "Running baseline performance test..."
BASELINE_FILE="$RESULTS_DIR/baseline_test.txt"

{
    echo "BASELINE PERFORMANCE TEST"
    echo "========================"
    echo "Date: $(date)"
    echo "Service: $SERVICE_URL"
    echo ""

    echo "=== /ping Endpoint ==="
    hey -n 1000 -c 10 "$SERVICE_URL/ping"
    echo ""

    echo "=== /hello/test Endpoint ==="
    hey -n 1000 -c 10 "$SERVICE_URL/hello/test"
    echo ""

    echo "=== /echo Endpoint (POST) ==="
    echo '{"name":"baseline","data":"test"}' | hey -n 1000 -c 10 -m POST -T "application/json" -D /dev/stdin "$SERVICE_URL/echo"
    echo ""

} > "$BASELINE_FILE"

log_success "Baseline test completed"

# 7. Основные тесты производительности
log "Running main performance tests..."

# Тест 1: Нормальная нагрузка
log "Test 1/4: Normal load test..."
hey -z 60s -c 50 -q 500 "$SERVICE_URL/ping" > "$RESULTS_DIR/normal_load_test.txt"

# Тест 2: Пиковая нагрузка
log "Test 2/4: Peak load test..."
hey -z 60s -c 100 -q 1000 "$SERVICE_URL/ping" > "$RESULTS_DIR/peak_load_test.txt"

# Тест 3: Стресс тест
log "Test 3/4: Stress test..."
hey -z 60s -c 200 -q 0 "$SERVICE_URL/ping" > "$RESULTS_DIR/stress_test.txt"

# Тест 4: Выносливость (если позволяет время)
if [ $TEST_DURATION -gt 180 ]; then
    log "Test 4/4: Endurance test..."
    hey -z $((TEST_DURATION - 180))s -c $CONCURRENT_USERS "$SERVICE_URL/ping" > "$RESULTS_DIR/endurance_test.txt"
else
    log "Test 4/4: Quick endurance test..."
    hey -z 60s -c $CONCURRENT_USERS "$SERVICE_URL/ping" > "$RESULTS_DIR/endurance_test.txt"
fi

log_success "Performance tests completed"

# 8. Тесты различных эндпоинтов
log "Testing different endpoints..."

{
    echo "ENDPOINT COMPARISON TEST"
    echo "======================="
    echo "Date: $(date)"
    echo ""

    for endpoint in "/ping" "/hello/testuser" "/data"; do
        echo "=== Testing $endpoint ==="
        hey -n 5000 -c 50 "$SERVICE_URL$endpoint"
        echo ""
    done

    echo "=== Testing /echo (POST) ==="
    echo '{"name":"endpoint_test","data":"comparison"}' | hey -n 5000 -c 50 -m POST -T "application/json" -D /dev/stdin "$SERVICE_URL/echo"

} > "$RESULTS_DIR/endpoint_comparison.txt"

log_success "Endpoint testing completed"

# 9. Ждем завершения мониторинга
log "Waiting for monitoring to complete..."
wait $MONITOR_PID 2>/dev/null || true

# Копируем файлы мониторинга в результаты
cp metrics_*.csv "$RESULTS_DIR/" 2>/dev/null || true
cp metrics_*.log "$RESULTS_DIR/" 2>/dev/null || true

# 10. Генерация визуализаций (если доступен Python)
if command -v python3 >/dev/null 2>&1; then
    log "Generating visualizations..."

    # Проверяем наличие Python пакетов
    if python3 -c "import pandas, matplotlib, seaborn" 2>/dev/null; then
        chmod +x visualize_results.py

        # Находим CSV файл с метриками
        METRICS_CSV=$(find . -name "metrics_*.csv" -type f | head -1)
        if [ -n "$METRICS_CSV" ]; then
            python3 visualize_results.py "$METRICS_CSV" || log_warning "Visualization generation failed"

            # Копируем визуализации в результаты
            cp -r performance_charts/* "$RESULTS_DIR/" 2>/dev/null || true
        else
            log_warning "No metrics CSV file found for visualization"
        fi
    else
        log_warning "Python visualization packages not available"
        echo "Install with: pip install pandas matplotlib seaborn"
    fi
else
    log_warning "Python3 not available for visualization generation"
fi

# 11. Генерация сводного отчета
log "Generating summary report..."

SUMMARY_FILE="$RESULTS_DIR/test_summary.txt"

{
    echo "SAI-SERVICE PERFORMANCE TEST SUMMARY"
    echo "===================================="
    echo "Test Date: $(date)"
    echo "Test Duration: ${TEST_DURATION}s ($(($TEST_DURATION / 60)) minutes)"
    echo "Concurrent Users: $CONCURRENT_USERS"
    echo "Service URL: $SERVICE_URL"
    echo ""

    echo "TEST RESULTS OVERVIEW:"
    echo "====================="

    # Извлекаем ключевые метрики из каждого теста
    echo ""
    echo "1. BASELINE TEST (/ping - 1000 requests, 10 concurrent):"
    if [ -f "$BASELINE_FILE" ]; then
        grep -A 1 "Requests/sec:" "$BASELINE_FILE" | head -1 || echo "   Data not available"
        grep -A 1 "Average:" "$BASELINE_FILE" | head -1 || echo "   Latency data not available"
    fi

    echo ""
    echo "2. NORMAL LOAD TEST (50 concurrent, 60s):"
    if [ -f "$RESULTS_DIR/normal_load_test.txt" ]; then
        grep "Requests/sec:" "$RESULTS_DIR/normal_load_test.txt" || echo "   Data not available"
        grep "Average:" "$RESULTS_DIR/normal_load_test.txt" || echo "   Latency data not available"
    fi

    echo ""
    echo "3. PEAK LOAD TEST (100 concurrent, 60s):"
    if [ -f "$RESULTS_DIR/peak_load_test.txt" ]; then
        grep "Requests/sec:" "$RESULTS_DIR/peak_load_test.txt" || echo "   Data not available"
        grep "Average:" "$RESULTS_DIR/peak_load_test.txt" || echo "   Latency data not available"
    fi

    echo ""
    echo "4. STRESS TEST (200 concurrent, unlimited QPS, 60s):"
    if [ -f "$RESULTS_DIR/stress_test.txt" ]; then
        grep "Requests/sec:" "$RESULTS_DIR/stress_test.txt" || echo "   Data not available"
        grep "Average:" "$RESULTS_DIR/stress_test.txt" || echo "   Latency data not available"
    fi

    echo ""
    echo "FILES GENERATED:"
    echo "==============="
    find "$RESULTS_DIR" -type f -name "*.txt" -o -name "*.csv" -o -name "*.html" -o -name "*.png" | sort

    echo ""
    echo "SYSTEM INFO:"
    echo "============"
    echo "OS: $(uname -s) $(uname -r)"
    echo "CPU: $(nproc) cores"
    echo "Memory: $(free -h | grep Mem | awk '{print $2}') total"
    echo "Go Version: $(go version)"

} > "$SUMMARY_FILE"

log_success "Summary report generated"

# 12. Финальная очистка и вывод результатов
cleanup

echo ""
echo "🎉 PERFORMANCE TEST COMPLETED SUCCESSFULLY!"
echo "==========================================="
echo ""
echo "📁 Results saved to: $RESULTS_DIR"
echo ""
echo "📊 Key Files:"
echo "   • test_summary.txt      - Test overview and key metrics"
echo "   • baseline_test.txt     - Baseline performance data"
echo "   • normal_load_test.txt  - Normal load test results"
echo "   • peak_load_test.txt    - Peak load test results"
echo "   • stress_test.txt       - Stress test results"
echo "   • endpoint_comparison.txt - Endpoint performance comparison"
echo ""

if [ -f "$RESULTS_DIR/performance_report.html" ]; then
    echo "🌐 Interactive Report: $RESULTS_DIR/performance_report.html"
    echo "   Open this file in your browser for detailed analysis"
    echo ""
fi

echo "🔍 Quick Analysis:"
echo "   View summary: cat $SUMMARY_FILE"
echo "   View baseline: cat $BASELINE_FILE"
echo ""

echo "💡 Next Steps:"
echo "   • Review the test_summary.txt for key performance metrics"
echo "   • Open performance_report.html in browser for detailed analysis"
echo "   • Compare results with your performance requirements"
echo "   • Use the data to identify optimization opportunities"
echo ""

log_success "Test suite execution completed!"