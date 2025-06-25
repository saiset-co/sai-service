#!/bin/bash

# Скрипт для тестирования высоких нагрузок SAI-Service
# Постепенное увеличение нагрузки для поиска пределов производительности

SERVICE_URL="http://localhost:8080"
RESULTS_DIR="./high_load_results_$(date +%Y%m%d_%H%M%S)"

mkdir -p "$RESULTS_DIR"

echo "🔥 HIGH LOAD PERFORMANCE TESTING"
echo "================================="
echo "Service: $SERVICE_URL"
echo "Results: $RESULTS_DIR"
echo ""

# Проверяем доступность сервиса
if ! curl -s "$SERVICE_URL/health" > /dev/null; then
    echo "❌ Service is not available"
    exit 1
fi

echo "✅ Service is running, starting high load tests..."
echo ""

# Функция для запуска теста с детальным выводом
run_load_test() {
    local test_name=$1
    local duration=$2
    local connections=$3
    local qps=$4
    local endpoint=$5

    echo "🚀 Testing: $test_name"
    echo "   Duration: ${duration}s, Connections: $connections, QPS: $qps"
    echo "   Endpoint: $endpoint"

    local output_file="$RESULTS_DIR/${test_name}.txt"

    if [ "$qps" = "0" ]; then
        # Unlimited QPS
        hey -z ${duration}s -c $connections "$SERVICE_URL$endpoint" > "$output_file"
    else
        # Limited QPS
        hey -z ${duration}s -c $connections -q $qps "$SERVICE_URL$endpoint" > "$output_file"
    fi

    # Извлекаем ключевые метрики
    local rps=$(grep "Requests/sec:" "$output_file" | awk '{print $2}')
    local avg_latency=$(grep "Average:" "$output_file" | awk '{print $2}')
    local p99_latency=$(grep "99% in" "$output_file" | awk '{print $3}')
    local success_rate=$(grep -c "200" "$output_file")
    local total_requests=$(grep "Total:" "$output_file" | head -1 | awk '{print $2}')

    echo "   📊 Results: RPS=$rps, Avg=${avg_latency}, P99=${p99_latency}, Success=${success_rate}/${total_requests}"
    echo ""

    # Сохраняем краткую сводку
    echo "$test_name: RPS=$rps, Avg=$avg_latency, P99=$p99_latency" >> "$RESULTS_DIR/summary.txt"
}

# 1. СЕРИЯ ТЕСТОВ С УВЕЛИЧЕНИЕМ CONNECTIONS
echo "=== PHASE 1: SCALING CONNECTIONS (unlimited QPS) ==="

run_load_test "connections_100" 30 100 0 "/ping"
run_load_test "connections_300" 30 300 0 "/ping"
run_load_test "connections_500" 30 500 0 "/ping"
run_load_test "connections_1000" 30 1000 0 "/ping"
run_load_test "connections_2000" 30 2000 0 "/ping"
run_load_test "connections_3000" 30 3000 0 "/ping"
run_load_test "connections_5000" 30 5000 0 "/ping"

echo "=== PHASE 2: EXTREME CONNECTIONS TEST ==="

# Экстремальные тесты (осторожно!)
run_load_test "extreme_10k_connections" 20 10000 0 "/ping"
run_load_test "extreme_20k_connections" 15 20000 0 "/ping"

echo "=== PHASE 3: HIGH QPS WITH MODERATE CONNECTIONS ==="

# Тесты с высоким QPS
run_load_test "high_qps_50k" 30 500 50000 "/ping"
run_load_test "high_qps_100k" 30 1000 100000 "/ping"
run_load_test "high_qps_200k" 30 1000 200000 "/ping"
run_load_test "high_qps_unlimited" 30 1000 0 "/ping"

echo "=== PHASE 4: DIFFERENT ENDPOINTS UNDER HIGH LOAD ==="

# Тестируем разные эндпоинты под высокой нагрузкой
run_load_test "hello_high_load" 30 1000 0 "/hello/loadtest"
run_load_test "data_high_load" 30 500 0 "/data"

# POST запросы под нагрузкой
echo "🔄 Testing POST endpoint under high load..."
echo '{"name":"loadtest","data":"high_load_data"}' | hey -z 30s -c 1000 -m POST -T "application/json" -D /dev/stdin "$SERVICE_URL/echo" > "$RESULTS_DIR/post_high_load.txt"

echo "=== PHASE 5: ENDURANCE TESTS ==="

# Длительные тесты стабильности
run_load_test "endurance_5min" 300 1000 0 "/ping"
echo "⏰ Running 10-minute endurance test..."
run_load_test "endurance_10min" 600 500 0 "/ping"

echo "=== PHASE 6: RESOURCE EXHAUSTION TESTS ==="

# Попытки исчерпать ресурсы
echo "🔥 Attempting to exhaust server resources..."
echo "WARNING: These tests may impact system stability!"
echo ""

# Очень высокие нагрузки (будьте осторожны!)
run_load_test "resource_test_1" 10 50000 0 "/ping"
run_load_test "resource_test_2" 10 100000 0 "/ping"

echo ""
echo "🏁 HIGH LOAD TESTING COMPLETED!"
echo "================================"
echo ""

# Генерируем сводный отчет
echo "📊 PERFORMANCE ANALYSIS:"
echo "========================"

echo "Connection Scaling Results:"
grep "connections_" "$RESULTS_DIR/summary.txt" | while read line; do
    echo "  $line"
done

echo ""
echo "High QPS Results:"
grep "qps_" "$RESULTS_DIR/summary.txt" | while read line; do
    echo "  $line"
done

echo ""
echo "Endurance Results:"
grep "endurance_" "$RESULTS_DIR/summary.txt" | while read line; do
    echo "  $line"
done

echo ""
echo "🔍 RECOMMENDATIONS:"
echo "==================="

# Анализ результатов для рекомендаций
BEST_RPS=$(grep "RPS=" "$RESULTS_DIR/summary.txt" | sed 's/.*RPS=\([0-9.]*\).*/\1/' | sort -nr | head -1)
echo "• Maximum observed RPS: $BEST_RPS"

# Проверяем, где начинается деградация
echo "• Check detailed results in: $RESULTS_DIR/"
echo "• Look for latency spikes and error rates in individual test files"
echo "• Compare P99 latencies across different connection counts"

echo ""
echo "💡 SYSTEM OPTIMIZATION TIPS:"
echo "============================"
echo "If you're hitting limits, try:"
echo "• ulimit -n 1000000  # Increase file descriptor limit"
echo "• echo 'net.core.somaxconn = 65536' | sudo tee -a /etc/sysctl.conf"
echo "• echo 'net.ipv4.tcp_max_syn_backlog = 30000' | sudo tee -a /etc/sysctl.conf"
echo "• echo 'net.core.netdev_max_backlog = 30000' | sudo tee -a /etc/sysctl.conf"
echo "• sudo sysctl -p"
echo ""
echo "Go optimizations:"
echo "• export GOMAXPROCS=\$(nproc)"
echo "• export GOGC=50  # More aggressive GC"
echo ""

echo "📁 All results saved to: $RESULTS_DIR"