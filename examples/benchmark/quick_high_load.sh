#!/bin/bash

# Быстрые команды для высоконагруженного тестирования SAI-Service
# Использование: ./quick_high_load.sh [connections] [duration] [endpoint]

SERVICE_URL="http://localhost:8081"
CONNECTIONS=${1:-5000}      # По умолчанию 5000 соединений
DURATION=${2:-60}           # По умолчанию 60 секунд
ENDPOINT=${3:-"/ping"}      # По умолчанию /ping

echo "⚡ QUICK HIGH LOAD TEST"
echo "====================="
echo "Service: $SERVICE_URL"
echo "Connections: $CONNECTIONS"
echo "Duration: ${DURATION}s"
echo "Endpoint: $ENDPOINT"
echo ""

# Проверяем доступность сервиса
if ! curl -s "$SERVICE_URL/health" > /dev/null; then
    echo "❌ Service is not available at $SERVICE_URL"
    echo "Please start the service first"
    exit 1
fi

# Проверяем лимиты
current_limit=$(ulimit -n)
if [ "$current_limit" -lt $((CONNECTIONS + 1000)) ]; then
    echo "⚠️  WARNING: File descriptor limit ($current_limit) may be too low for $CONNECTIONS connections"
    echo "Try: ulimit -n $((CONNECTIONS * 2))"
    echo ""
fi

# Показываем системную информацию
echo "📊 SYSTEM INFO:"
echo "==============="
echo "CPU cores: $(nproc)"
echo "Available memory: $(free -h | grep Mem | awk '{print $7}')"
echo "File descriptor limit: $(ulimit -n)"
echo "Current connections: $(ss -t | wc -l)"
echo ""

# Функция для мониторинга в фоне
start_monitor() {
    {
        echo "time,rps_sample,memory_mb,cpu_percent,connections,load_avg"
        while [ -f /tmp/quick_monitor ]; do
            local time=$(date "+%H:%M:%S")

            # Быстрая проба RPS (100 запросов)
            local rps_sample=$(timeout 5 hey -n 100 -c 10 "$SERVICE_URL/ping" 2>/dev/null | grep "Requests/sec:" | awk '{print $2}' || echo "0")

            # Системные метрики
            local memory=$(ps aux | grep -E "(go run|perf-test)" | awk '{sum += $6} END {print sum/1024}')
            local cpu=$(ps aux | grep -E "(go run|perf-test)" | awk '{sum += $3} END {print sum}')
            local connections=$(ss -t | wc -l)
            local load_avg=$(cat /proc/loadavg | awk '{print $1}')

            echo "$time,${rps_sample:-0},${memory:-0},${cpu:-0},$connections,$load_avg"
            sleep 3
        done
    } > "quick_monitor_$(date +%H%M%S).csv" &

    echo $! > /tmp/monitor_pid
}

# Функция остановки мониторинга
stop_monitor() {
    rm -f /tmp/quick_monitor
    if [ -f /tmp/monitor_pid ]; then
        kill $(cat /tmp/monitor_pid) 2>/dev/null || true
        rm -f /tmp/monitor_pid
    fi
}

# Обработчик сигналов
cleanup() {
    echo ""
    echo "🛑 Stopping test..."
    stop_monitor
    exit 0
}
trap cleanup INT TERM

echo "🚀 Starting high load test..."
echo "Press Ctrl+C to stop"
echo ""

# Запускаем мониторинг
touch /tmp/quick_monitor
start_monitor

# Запускаем основной тест
echo "🔥 Testing with $CONNECTIONS connections for ${DURATION}s..."
start_time=$(date +%s)

hey -z ${DURATION}s -c $CONNECTIONS -t 30 "$SERVICE_URL$ENDPOINT" | tee "quick_test_$(date +%H%M%S).txt"

end_time=$(date +%s)
actual_duration=$((end_time - start_time))

# Останавливаем мониторинг
stop_monitor

echo ""
echo "✅ Test completed in ${actual_duration}s"
echo ""

# Быстрый анализ результатов
if [ -f "quick_test_"*.txt ]; then
    RESULT_FILE=$(ls -t quick_test_*.txt | head -1)

    echo "📊 QUICK ANALYSIS:"
    echo "=================="

    RPS=$(grep "Requests/sec:" "$RESULT_FILE" | awk '{print $2}')
    AVG_LATENCY=$(grep "Average:" "$RESULT_FILE" | awk '{print $2}')
    P99_LATENCY=$(grep "99% in" "$RESULT_FILE" | awk '{print $3}')
    TOTAL_REQ=$(grep "Total:" "$RESULT_FILE" | head -1 | awk '{print $2}')
    ERRORS=$(grep -c "Non-2xx" "$RESULT_FILE" 2>/dev/null || echo "0")

    echo "🎯 Performance:"
    echo "   RPS: ${RPS:-N/A}"
    echo "   Average Latency: ${AVG_LATENCY:-N/A}"
    echo "   P99 Latency: ${P99_LATENCY:-N/A}"
    echo "   Total Requests: ${TOTAL_REQ:-N/A}"
    echo "   Errors: ${ERRORS:-0}"

    # Оценка производительности
    if [ -n "$RPS" ] && [ "$RPS" != "N/A" ]; then
        if (( $(echo "$RPS > 10000" | bc -l 2>/dev/null || echo "0") )); then
            echo "   🏆 Excellent performance!"
        elif (( $(echo "$RPS > 5000" | bc -l 2>/dev/null || echo "0") )); then
            echo "   ✅ Good performance"
        elif (( $(echo "$RPS > 1000" | bc -l 2>/dev/null || echo "0") )); then
            echo "   ⚠️  Moderate performance"
        else
            echo "   ❌ Low performance - check for bottlenecks"
        fi
    fi

    echo ""
    echo "💡 Quick recommendations:"
    if [ "$ERRORS" -gt 0 ]; then
        echo "   ⚠️  Errors detected ($ERRORS) - reduce load or check service"
    fi

    # Анализ латентности
    if [ -n "$P99_LATENCY" ] && [ "$P99_LATENCY" != "N/A" ]; then
        P99_MS=$(echo "$P99_LATENCY" | sed 's/[^0-9.]//g')
        if (( $(echo "$P99_MS > 100" | bc -l 2>/dev/null || echo "0") )); then
            echo "   ⚠️  High P99 latency (${P99_LATENCY}) - consider optimization"
        fi
    fi
fi

echo ""
echo "📁 Files created:"
ls -la quick_test_*.txt quick_monitor_*.csv 2>/dev/null || echo "   No files created"

echo ""
echo "🔄 TO RUN DIFFERENT TESTS:"
echo "=========================="
echo "# Higher load:"
echo "./quick_high_load.sh 10000 60 /ping"
echo ""
echo "# Different endpoint:"
echo "./quick_high_load.sh 5000 60 /hello/test"
echo ""
echo "# Longer duration:"
echo "./quick_high_load.sh 5000 300 /ping"
echo ""
echo "# Quick stress test:"
echo "./quick_high_load.sh 20000 30 /ping"

echo ""
echo "🎯 Next steps based on your baseline (16K RPS):"
echo "=============================================="
echo "• Your service performed excellently at 16K RPS with light load"
echo "• Try: ./quick_high_load.sh 10000 120 /ping  # Find sustained max"
echo "• Try: ./quick_high_load.sh 20000 60 /ping   # Find breaking point"
echo "• Monitor memory and CPU during tests"
echo "• Compare different endpoints performance"

echo ""
echo "✅ Quick high load test completed!"