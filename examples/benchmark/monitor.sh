#!/bin/bash

# Скрипт для мониторинга производительности микросервиса в реальном времени
# Использование: ./monitor.sh [service_url] [duration_seconds]

SERVICE_URL=${1:-"http://localhost:8080"}
DURATION=${2:-300}  # 5 минут по умолчанию
INTERVAL=5          # Интервал между замерами в секундах

echo "🔍 Real-time Performance Monitor"
echo "================================"
echo "Service: $SERVICE_URL"
echo "Duration: ${DURATION}s"
echo "Interval: ${INTERVAL}s"
echo "Press Ctrl+C to stop"
echo ""

# Создаем файл для логирования метрик
METRICS_LOG="./metrics_$(date +%Y%m%d_%H%M%S).log"
CSV_LOG="./metrics_$(date +%Y%m%d_%H%M%S).csv"

# Заголовки для CSV
echo "timestamp,rps,avg_latency_ms,p99_latency_ms,success_rate,memory_mb,cpu_percent,connections" > "$CSV_LOG"

# Функция для получения метрик сервиса
get_service_metrics() {
    local temp_file=$(mktemp)

    # Быстрый тест для получения актуальных метрик
    timeout 10 hey -n 100 -c 10 -t 5 "$SERVICE_URL/ping" > "$temp_file" 2>/dev/null

    if [ $? -eq 0 ]; then
        local rps=$(grep "Requests/sec:" "$temp_file" | awk '{print $2}' | sed 's/[^0-9.]//g')
        local avg_latency=$(grep "Average:" "$temp_file" | awk '{print $2}' | sed 's/[^0-9.]//g')
        local p99_latency=$(grep "99% in" "$temp_file" | awk '{print $3}' | sed 's/[^0-9.]//g')
        local success_rate=$(grep -c "200" "$temp_file")

        # Конвертируем латентность в миллисекунды если нужно
        if [[ $avg_latency == *"s"* ]]; then
            avg_latency=$(echo "$avg_latency * 1000" | bc -l 2>/dev/null | cut -d. -f1)
        fi
        if [[ $p99_latency == *"s"* ]]; then
            p99_latency=$(echo "$p99_latency * 1000" | bc -l 2>/dev/null | cut -d. -f1)
        fi

        echo "${rps:-0},${avg_latency:-0},${p99_latency:-0},${success_rate:-0}"
    else
        echo "0,0,0,0"
    fi

    rm -f "$temp_file"
}

# Функция для получения системных метрик
get_system_metrics() {
    local pid=$(pgrep -f "go run\\|perf-test-service" | head -1)

    if [ -n "$pid" ]; then
        # Получаем использование памяти в MB
        local memory_kb=$(ps -p "$pid" -o rss= 2>/dev/null | tr -d ' ')
        local memory_mb=$((memory_kb / 1024))

        # Получаем использование CPU
        local cpu_percent=$(ps -p "$pid" -o %cpu= 2>/dev/null | tr -d ' ')

        # Количество соединений на порту 8080
        local connections=$(netstat -an 2>/dev/null | grep ":8080" | wc -l)

        echo "${memory_mb:-0},${cpu_percent:-0},${connections:-0}"
    else
        echo "0,0,0"
    fi
}

# Функция отображения метрик в удобном формате
display_metrics() {
    local timestamp=$1
    local service_metrics=$2
    local system_metrics=$3

    IFS=',' read -r rps avg_latency p99_latency success_rate <<< "$service_metrics"
    IFS=',' read -r memory_mb cpu_percent connections <<< "$system_metrics"

    printf "⏰ %s\n" "$timestamp"
    printf "📊 Service: RPS=%-8s AvgLat=%-6sms P99=%-6sms Success=%-3s\n" "$rps" "$avg_latency" "$p99_latency" "$success_rate"
    printf "💻 System:  MEM=%-6sMB CPU=%-6s%% Conn=%-6s\n" "$memory_mb" "$cpu_percent" "$connections"
    printf "────────────────────────────────────────────────────────\n"
}

# Проверяем доступность сервиса
if ! curl -s "$SERVICE_URL/health" > /dev/null 2>&1; then
    echo "❌ Service is not available at $SERVICE_URL"
    echo "Please start the service first"
    exit 1
fi

echo "✅ Service is running, starting monitoring..."
echo ""

# Основной цикл мониторинга
start_time=$(date +%s)
end_time=$((start_time + DURATION))

while [ $(date +%s) -lt $end_time ]; do
    timestamp=$(date "+%Y-%m-%d %H:%M:%S")

    # Получаем метрики
    service_metrics=$(get_service_metrics)
    system_metrics=$(get_system_metrics)

    # Отображаем метрики
    display_metrics "$timestamp" "$service_metrics" "$system_metrics"

    # Сохраняем в CSV
    echo "$timestamp,$service_metrics,$system_metrics" >> "$CSV_LOG"

    # Сохраняем в лог
    echo "[$timestamp] Service: $service_metrics | System: $system_metrics" >> "$METRICS_LOG"

    sleep $INTERVAL
done

echo ""
echo "🏁 Monitoring completed!"
echo "📄 Metrics saved to:"
echo "   CSV: $CSV_LOG"
echo "   Log: $METRICS_LOG"
echo ""

# Генерируем сводку
echo "📈 Performance Summary:"
echo "======================="

# Анализируем CSV данные
if command -v awk >/dev/null 2>&1; then
    echo "RPS Statistics:"
    awk -F',' 'NR>1 {sum+=$2; if($2>max) max=$2; if(min=="" || $2<min) min=$2} END {
        print "  Average: " (sum/(NR-1))
        print "  Maximum: " max
        print "  Minimum: " min
    }' "$CSV_LOG"

    echo ""
    echo "Latency Statistics (ms):"
    awk -F',' 'NR>1 {sum+=$3; if($3>max) max=$3; if(min=="" || $3<min) min=$3} END {
        print "  Average: " (sum/(NR-1))
        print "  Maximum: " max
        print "  Minimum: " min
    }' "$CSV_LOG"

    echo ""
    echo "Memory Usage (MB):"
    awk -F',' 'NR>1 {sum+=$6; if($6>max) max=$6; if(min=="" || $6<min) min=$6} END {
        print "  Average: " (sum/(NR-1))
        print "  Maximum: " max
        print "  Minimum: " min
    }' "$CSV_LOG"
fi

echo ""
echo "💡 Analysis commands:"
echo "   View CSV: cat $CSV_LOG"
echo "   Plot data: python3 -c \"import pandas as pd; import matplotlib.pyplot as plt; df=pd.read_csv('$CSV_LOG'); df.plot(x='timestamp', y='rps'); plt.show()\""
echo "   Excel import: open $CSV_LOG in Excel/LibreOffice"