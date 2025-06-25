#!/bin/bash

# Экстремальный тест производительности с детальным мониторингом
# Находит реальные пределы производительности вашего SAI-Service

SERVICE_URL="http://localhost:8080"
RESULTS_DIR="./extreme_load_$(date +%Y%m%d_%H%M%S)"
MONITOR_INTERVAL=2
MAX_CONNECTIONS=50000

mkdir -p "$RESULTS_DIR"

echo "💥 EXTREME LOAD TESTING WITH MONITORING"
echo "======================================="
echo "Service: $SERVICE_URL"
echo "Max connections to test: $MAX_CONNECTIONS"
echo "Results: $RESULTS_DIR"
echo ""

# Проверяем доступность сервиса
if ! curl -s "$SERVICE_URL/health" > /dev/null; then
    echo "❌ Service is not available"
    exit 1
fi

# Проверяем лимиты системы
echo "📊 SYSTEM LIMITS CHECK:"
echo "======================="
echo "Current ulimit -n: $(ulimit -n)"
echo "Max processes: $(ulimit -u)"
echo "Available memory: $(free -h | grep Mem | awk '{print $7}')"
echo "CPU cores: $(nproc)"
echo ""

if [ "$(ulimit -n)" -lt 10000 ]; then
    echo "⚠️  WARNING: File descriptor limit is low ($(ulimit -n))"
    echo "   Run: ulimit -n 100000"
    echo "   Or optimize system first: sudo ./optimize_system.sh"
    echo ""
fi

# Функция для мониторинга системных ресурсов
start_monitoring() {
    local test_name=$1
    local monitor_file="$RESULTS_DIR/${test_name}_monitor.csv"

    echo "timestamp,connections,memory_mb,cpu_percent,load_avg,open_files" > "$monitor_file"

    while [ -f "/tmp/monitoring_${test_name}" ]; do
        local timestamp=$(date "+%H:%M:%S")
        local connections=$(ss -t | wc -l)
        local memory_mb=$(ps aux | grep -E "(go run|perf-test)" | awk '{sum += $6} END {print sum/1024}')
        local cpu_percent=$(ps aux | grep -E "(go run|perf-test)" | awk '{sum += $3} END {print sum}')
        local load_avg=$(cat /proc/loadavg | awk '{print $1}')
        local open_files=$(lsof 2>/dev/null | wc -l)

        echo "$timestamp,$connections,${memory_mb:-0},${cpu_percent:-0},$load_avg,$open_files" >> "$monitor_file"
        sleep $MONITOR_INTERVAL
    done &

    echo $! > "/tmp/monitor_pid_${test_name}"
}

stop_monitoring() {
    local test_name=$1
    rm -f "/tmp/monitoring_${test_name}"

    if [ -f "/tmp/monitor_pid_${test_name}" ]; then
        local monitor_pid=$(cat "/tmp/monitor_pid_${test_name}")
        kill "$monitor_pid" 2>/dev/null || true
        rm -f "/tmp/monitor_pid_${test_name}"
    fi
}

# Функция для запуска экстремального теста
run_extreme_test() {
    local test_name=$1
    local duration=$2
    local connections=$3
    local endpoint=$4

    echo "🔥 EXTREME TEST: $test_name"
    echo "   Duration: ${duration}s"
    echo "   Connections: $connections"
    echo "   Endpoint: $endpoint"
    echo "   Started at: $(date)"

    # Запускаем мониторинг
    touch "/tmp/monitoring_${test_name}"
    start_monitoring "$test_name"

    # Запускаем тест
    local output_file="$RESULTS_DIR/${test_name}.txt"
    local start_time=$(date +%s)

    # Используем timeout для защиты от зависших тестов
    timeout $((duration + 30)) hey -z ${duration}s -c $connections -t 30 "$SERVICE_URL$endpoint" > "$output_file" 2>&1
    local test_exit_code=$?

    # Останавливаем мониторинг
    stop_monitoring "$test_name"

    local end_time=$(date +%s)
    local actual_duration=$((end_time - start_time))

    # Анализируем результаты
    if [ $test_exit_code -eq 0 ] && [ -f "$output_file" ]; then
        local rps=$(grep "Requests/sec:" "$output_file" | awk '{print $2}' | head -1)
        local avg_latency=$(grep "Average:" "$output_file" | awk '{print $2}' | head -1)
        local p99_latency=$(grep "99% in" "$output_file" | awk '{print $3}' | head -1)
        local error_rate=$(grep -c "Non-2xx" "$output_file" || echo "0")
        local total_requests=$(grep "Total:" "$output_file" | head -1 | awk '{print $2}')

        echo "   ✅ COMPLETED"
        echo "   📊 RPS: ${rps:-N/A}"
        echo "   ⏱️  Avg Latency: ${avg_latency:-N/A}"
        echo "   📈 P99 Latency: ${p99_latency:-N/A}"
        echo "   ❌ Errors: ${error_rate:-0}"
        echo "   📊 Total Requests: ${total_requests:-N/A}"
        echo "   ⏰ Actual Duration: ${actual_duration}s"

        # Записываем в сводку
        echo "$test_name,$connections,$rps,$avg_latency,$p99_latency,$error_rate,$actual_duration" >> "$RESULTS_DIR/extreme_summary.csv"

        return 0
    else
        echo "   ❌ FAILED (exit code: $test_exit_code)"
        echo "   ⏰ Duration: ${actual_duration}s"

        # Записываем ошибку в сводку
        echo "$test_name,$connections,FAILED,FAILED,FAILED,FAILED,$actual_duration" >> "$RESULTS_DIR/extreme_summary.csv"

        return 1
    fi

    echo ""
}

# Создаем заголовок для CSV сводки
echo "test_name,connections,rps,avg_latency,p99_latency,errors,duration" > "$RESULTS_DIR/extreme_summary.csv"

echo "🚀 Starting extreme load testing..."
echo ""

# PHASE 1: Прогрессивное увеличение нагрузки
echo "=== PHASE 1: PROGRESSIVE LOAD INCREASE ==="

# Начинаем с разумных значений и увеличиваем
CONNECTIONS=(100 500 1000 2000 5000 10000 15000 20000 30000 50000)

for conn in "${CONNECTIONS[@]}"; do
    if ! run_extreme_test "progressive_${conn}" 20 $conn "/ping"; then
        echo "⚠️  Test failed at $conn connections - this might be the limit"
        break
    fi

#    # Проверяем, не исчерпали ли мы ресурсы
#    local current_files=$(lsof 2>/dev/null | wc -l)
#    local ulimit_files=$(ulimit -n)
#
#    if [ "$current_files" -gt $((ulimit_files * 80 / 100)) ]; then
#        echo "⚠️  Approaching file descriptor limit ($current_files/$ulimit_files)"
#    fi

    # Небольшая пауза для стабилизации системы
    sleep 5
done

echo ""
echo "=== PHASE 2: SUSTAINED HIGH LOAD ==="

# Находим максимальную стабильную нагрузку
# Берем последний успешный тест и тестируем дольше
LAST_SUCCESSFUL=$(grep -v "FAILED" "$RESULTS_DIR/extreme_summary.csv" | tail -1 | cut -d',' -f2)

if [ -n "$LAST_SUCCESSFUL" ] && [ "$LAST_SUCCESSFUL" != "connections" ]; then
    echo "🔥 Testing sustained load with $LAST_SUCCESSFUL connections for 5 minutes..."
    run_extreme_test "sustained_high" 300 "$LAST_SUCCESSFUL" "/ping"
else
    echo "⚠️  No successful high load test found, using 1000 connections"
    run_extreme_test "sustained_high" 300 1000 "/ping"
fi

echo ""
echo "=== PHASE 3: DIFFERENT ENDPOINTS UNDER EXTREME LOAD ==="

# Тестируем разные эндпоинты
EXTREME_CONN=${LAST_SUCCESSFUL:-5000}

run_extreme_test "extreme_hello" 30 $EXTREME_CONN "/hello/extreme"
run_extreme_test "extreme_data" 30 $((EXTREME_CONN / 2)) "/data"  # Меньше соединений для тяжелого эндпоинта

# POST тест
echo "🔄 Testing POST under extreme load..."
echo '{"name":"extreme","data":"load_test"}' | timeout 60 hey -z 30s -c $EXTREME_CONN -m POST -T "application/json" -D /dev/stdin "$SERVICE_URL/echo" > "$RESULTS_DIR/extreme_post.txt" 2>&1

echo ""
echo "=== PHASE 4: RESOURCE EXHAUSTION ATTEMPTS ==="

echo "⚠️  WARNING: The following tests may crash the service or system!"
echo "Press Ctrl+C within 10 seconds to cancel..."
sleep 10

# Попытки исчерпать ресурсы
run_extreme_test "exhaustion_1" 15 100000 "/ping" || true
run_extreme_test "exhaustion_2" 10 200000 "/ping" || true

echo ""
echo "💥 EXTREME LOAD TESTING COMPLETED!"
echo "=================================="

# Генерируем детальный анализ
echo ""
echo "📊 PERFORMANCE ANALYSIS:"
echo "========================"

echo "Test Results Summary:"
echo "--------------------"
cat "$RESULTS_DIR/extreme_summary.csv" | column -t -s ','

echo ""
echo "Peak Performance Analysis:"
echo "-------------------------"

# Находим пиковые значения
PEAK_RPS=$(grep -v "FAILED\|rps" "$RESULTS_DIR/extreme_summary.csv" | awk -F',' '{print $3}' | sort -nr | head -1)
PEAK_CONN=$(grep "$PEAK_RPS" "$RESULTS_DIR/extreme_summary.csv" | awk -F',' '{print $2}')

echo "🏆 Peak RPS: $PEAK_RPS at $PEAK_CONN connections"

# Анализируем деградацию производительности
echo ""
echo "Performance Degradation Analysis:"
echo "--------------------------------"

python3 << EOF 2>/dev/null || echo "Python analysis skipped"
import csv
import sys

try:
    with open('$RESULTS_DIR/extreme_summary.csv', 'r') as f:
        reader = csv.DictReader(f)
        data = [row for row in reader if row['rps'] != 'FAILED']

    if len(data) > 1:
        print("Connection vs RPS trend:")
        for row in data:
            conn = int(row['connections'])
            rps = float(row['rps'])
            latency = row['avg_latency']
            print(f"  {conn:6d} conn -> {rps:8.0f} RPS (latency: {latency})")

        # Находим точку деградации
        max_rps = max(float(row['rps']) for row in data)
        for i, row in enumerate(data):
            current_rps = float(row['rps'])
            if current_rps < max_rps * 0.8:  # 20% снижение
                print(f"\n⚠️  Performance degradation detected at {row['connections']} connections")
                print(f"   RPS dropped to {current_rps:.0f} ({(current_rps/max_rps*100):.0f}% of peak)")
                break

except Exception as e:
    print(f"Analysis error: {e}")
EOF

echo ""
echo "📈 RECOMMENDATIONS:"
echo "==================="

echo "Based on test results:"

# Анализируем последний успешный тест
LAST_SUCCESS=$(grep -v "FAILED\|test_name" "$RESULTS_DIR/extreme_summary.csv" | tail -1)
if [ -n "$LAST_SUCCESS" ]; then
    LAST_CONN=$(echo "$LAST_SUCCESS" | cut -d',' -f2)
    LAST_RPS=$(echo "$LAST_SUCCESS" | cut -d',' -f3)
    LAST_LATENCY=$(echo "$LAST_SUCCESS" | cut -d',' -f4)

    echo "• Maximum stable load: $LAST_CONN concurrent connections"
    echo "• Peak performance: $LAST_RPS RPS"
    echo "• Recommended production load: $((LAST_CONN / 2)) connections (50% of max)"
    echo "• Monitor latency - current P99 at max load: $(grep -v "FAILED\|test_name" "$RESULTS_DIR/extreme_summary.csv" | tail -1 | cut -d',' -f5)"
fi

echo ""
echo "🔧 OPTIMIZATION SUGGESTIONS:"
echo "============================"

# Проверяем мониторинг данные для последнего теста
LAST_MONITOR=$(ls -t "$RESULTS_DIR"/*_monitor.csv 2>/dev/null | head -1)
if [ -f "$LAST_MONITOR" ]; then
    MAX_MEMORY=$(awk -F',' 'NR>1 {if($3>max) max=$3} END {print max}' "$LAST_MONITOR")
    MAX_CPU=$(awk -F',' 'NR>1 {if($4>max) max=$4} END {print max}' "$LAST_MONITOR")

    echo "Resource utilization at peak:"
    echo "• Memory: ${MAX_MEMORY:-N/A} MB"
    echo "• CPU: ${MAX_CPU:-N/A}%"

    if [ "$MAX_MEMORY" -gt 1000 ] 2>/dev/null; then
        echo "• ⚠️  High memory usage detected - consider memory optimizations"
    fi

    if [ "$MAX_CPU" -gt 80 ] 2>/dev/null; then
        echo "• ⚠️  High CPU usage - consider CPU optimizations or more cores"
    fi
fi

echo ""
echo "💡 NEXT STEPS:"
echo "=============="
echo "1. Review detailed results in: $RESULTS_DIR/"
echo "2. Check monitoring data: *_monitor.csv files"
echo "3. If limits were hit, optimize system: ./optimize_system.sh"
echo "4. For production, use 50-70% of maximum stable load"
echo "5. Set up monitoring and alerting based on these baseline metrics"

echo ""
echo "📁 All results and monitoring data saved to: $RESULTS_DIR"
echo ""

# Создаем быстрый отчет
cat > "$RESULTS_DIR/quick_report.txt" << EOF
EXTREME LOAD TEST QUICK REPORT
==============================
Test Date: $(date)
Service: $SERVICE_URL

PEAK PERFORMANCE:
• Maximum RPS: $PEAK_RPS
• At connections: $PEAK_CONN
• Peak memory: ${MAX_MEMORY:-N/A} MB
• Peak CPU: ${MAX_CPU:-N/A}%

RECOMMENDED SETTINGS:
• Production load: $((LAST_CONN / 2)) concurrent connections
• Monitor latency threshold: 50ms (P99)
• Memory limit: $((MAX_MEMORY * 150 / 100)) MB (150% of peak)

FILES:
• Full results: extreme_summary.csv
• Monitoring data: *_monitor.csv
• Individual tests: *.txt
EOF

echo "📋 Quick report created: $RESULTS_DIR/quick_report.txt"
echo ""
echo "✅ Extreme load testing completed successfully!"