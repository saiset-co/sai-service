#!/bin/bash

# ОПТИМИЗИРОВАННЫЕ ПАРАМЕТРЫ ДЛЯ ТЕСТИРОВАНИЯ ПРОИЗВОДИТЕЛЬНОСТИ
# Основаны на анализе ваших результатов

# =============================================================================
# ПРОБЛЕМЫ В ТЕКУЩИХ ТЕСТАХ:
# =============================================================================
# 1. run_complete_test.sh показывает ~8K RPS среднее
# 2. max_rps.sh показывает 79K RPS при 100 соединениях
# 3. Огромная разница указывает на проблемы в методологии тестирования

# =============================================================================
# ОПТИМИЗИРОВАННЫЕ ПАРАМЕТРЫ
# =============================================================================

# Основные настройки
SERVICE_URL="http://localhost:8081"
SERVICE_PORT=8081

# Параметры тестирования - ИСПРАВЛЕННЫЕ
declare -A TEST_CONFIGS=(
    # Формат: "test_name:duration:connections:qps:description"
    ["baseline"]="60:10:500:Базовая линия - низкая нагрузка"
    ["warmup"]="30:50:1000:Прогрев системы"
    ["normal_load"]="120:100:2000:Нормальная нагрузка"
    ["increased_load"]="120:200:5000:Увеличенная нагрузка"
    ["high_load"]="120:500:10000:Высокая нагрузка"
    ["stress_test"]="60:1000:0:Стресс тест (без ограничения RPS)"
    ["peak_capacity"]="180:2000:0:Пиковая нагрузка"
    ["endurance"]="600:500:3000:Тест выносливости (10 минут)"
)

# Эндпоинты для тестирования
ENDPOINTS=(
    "/ping"
    "/hello/testuser"
    "/data"
    "/health"
)

# Параметры мониторинга
MONITORING_INTERVAL=1  # секунды
CPU_THRESHOLD=80      # процент
MEMORY_THRESHOLD=80   # процент
LATENCY_THRESHOLD=50  # миллисекунды

# =============================================================================
# УЛУЧШЕННАЯ ФУНКЦИЯ ТЕСТИРОВАНИЯ
# =============================================================================

run_optimized_test() {
    local test_name=$1
    local duration=$2
    local connections=$3
    local qps=$4
    local endpoint=${5:-"/ping"}
    local description=$6

    echo "🚀 OPTIMIZED TEST: $test_name"
    echo "   Description: $description"
    echo "   Duration: ${duration}s"
    echo "   Connections: $connections"
    echo "   Target QPS: $([ $qps -eq 0 ] && echo "Unlimited" || echo $qps)"
    echo "   Endpoint: $endpoint"

    local output_file="$RESULTS_DIR/${test_name}_optimized.txt"
    local start_time=$(date +%s)

    # Строим команду hey с оптимизированными параметрами
    local hey_cmd="hey"
    hey_cmd+=" -z ${duration}s"           # Продолжительность
    hey_cmd+=" -c $connections"           # Соединения
    hey_cmd+=" -t 30"                     # Таймаут 30 секунд
    hey_cmd+=" -disable-compression"      # Отключаем сжатие для честного тестирования
    hey_cmd+=" -disable-keepalive"        # Отключаем keep-alive для реалистичной нагрузки

    # Добавляем QPS только если он задан
    if [ $qps -gt 0 ]; then
        hey_cmd+=" -q $qps"
    fi

    hey_cmd+=" \"$SERVICE_URL$endpoint\""

    echo "   Command: $hey_cmd"
    echo "   Starting test..."

    # Запускаем тест с таймаутом
    timeout $((duration + 60)) bash -c "$hey_cmd" > "$output_file" 2>&1
    local test_exit_code=$?

    local end_time=$(date +%s)
    local actual_duration=$((end_time - start_time))

    # Анализируем результаты
    if [ $test_exit_code -eq 0 ] && [ -f "$output_file" ]; then
        analyze_test_results "$output_file" "$test_name" "$qps"
    else
        echo "   ❌ TEST FAILED (exit code: $test_exit_code, duration: ${actual_duration}s)"
        echo "   📄 Checking output file..."
        if [ -f "$output_file" ]; then
            echo "   Last 10 lines of output:"
            tail -10 "$output_file" | sed 's/^/      /'
        fi
    fi

    echo ""
}

# =============================================================================
# АНАЛИЗ РЕЗУЛЬТАТОВ
# =============================================================================

analyze_test_results() {
    local output_file=$1
    local test_name=$2
    local target_qps=$3

    # Извлекаем метрики с очисткой от переносов строк
    local actual_rps=$(grep "Requests/sec:" "$output_file" | awk '{print $2}' | head -1 | tr -d '\n\r\t ')
    local total_requests=$(grep "Total:" "$output_file" | awk '{print $2}' | head -1 | tr -d '\n\r\t ')
    local avg_latency=$(grep -A 1 "Average:" "$output_file" | head -1 | awk '{print $2}' | tr -d '\n\r\t ')
    local p95_latency=$(grep "95% in" "$output_file" | awk '{print $3}' | head -1 | tr -d '\n\r\t ')
    local p99_latency=$(grep "99% in" "$output_file" | awk '{print $3}' | head -1 | tr -d '\n\r\t ')

    # Считаем ошибки более точно
    local errors=0
    local error_lines=$(grep -E "Non-2xx|timeout|error|failed" "$output_file" 2>/dev/null | wc -l)
    if [ "$error_lines" -gt 0 ]; then
        errors=$error_lines
    fi

    # Получаем коды успешных ответов
    local success_codes=$(grep "\[200\]" "$output_file" | awk '{print $2}' | tr -d '\n\r\t ' || echo "0")

    # Валидация данных с принудительной очисткой
    if ! [[ "$actual_rps" =~ ^[0-9]+\.?[0-9]*$ ]] || [ -z "$actual_rps" ]; then
        actual_rps="0"
    fi
    if ! [[ "$errors" =~ ^[0-9]+$ ]] || [ -z "$errors" ]; then
        errors="0"
    fi
    if ! [[ "$success_codes" =~ ^[0-9]+$ ]] || [ -z "$success_codes" ]; then
        success_codes="0"
    fi

    # Рассчитываем эффективность
    local efficiency="N/A"
    if [ "$target_qps" -gt 0 ] && [[ "$actual_rps" =~ ^[0-9]+\.?[0-9]*$ ]]; then
        efficiency=$(awk "BEGIN {printf \"%.1f\", ($actual_rps / $target_qps) * 100}")
    fi

    # Выводим результаты
    echo "   📊 RESULTS:"
    echo "      Total Requests: ${total_requests:-N/A}"
    echo "      Actual RPS: ${actual_rps:-N/A}"
    echo "      Target Efficiency: ${efficiency}%"
    echo "      Average Latency: ${avg_latency:-N/A}"
    echo "      95th Percentile: ${p95_latency:-N/A}"
    echo "      99th Percentile: ${p99_latency:-N/A}"
    echo "      Success Responses: ${success_codes:-N/A}"
    echo "      Errors: ${errors:-0}"

    # УПРОЩЕННАЯ логика определения статуса
    local status="SUCCESS"  # По умолчанию успех

    # Простые проверки
    if [ "$actual_rps" = "0" ] || [ -z "$actual_rps" ]; then
        status="NO_DATA"
    elif ! [[ "$actual_rps" =~ ^[0-9]+\.?[0-9]*$ ]]; then
        status="NO_DATA"
    elif [ "$errors" -gt 100 ]; then
        status="HIGH_ERRORS"
    elif [[ "$efficiency" =~ ^[0-9]+\.?[0-9]*$ ]]; then
        # Если efficiency есть, проверяем её
        if awk "BEGIN {exit !($efficiency >= 80)}" 2>/dev/null; then
            status="SUCCESS"
        elif awk "BEGIN {exit !($efficiency >= 50)}" 2>/dev/null; then
            status="PARTIAL"
        else
            status="DEGRADED"
        fi
    fi

    case $status in
        "SUCCESS")    echo "      🎉 STATUS: SUCCESS" ;;
        "PARTIAL")    echo "      ⚠️  STATUS: PARTIAL SUCCESS" ;;
        "DEGRADED")   echo "      📉 STATUS: PERFORMANCE DEGRADED" ;;
        "HIGH_ERRORS") echo "      ❌ STATUS: TOO MANY ERRORS" ;;
        "LOW_RPS")    echo "      ❌ STATUS: RPS TOO LOW" ;;
        "NO_DATA")    echo "      ❌ STATUS: NO VALID DATA" ;;
        *)            echo "      ❓ STATUS: UNKNOWN" ;;
    esac

    # Записываем в CSV для дальнейшего анализа
    echo "$test_name,$target_qps,$actual_rps,$total_requests,$avg_latency,$p95_latency,$p99_latency,$errors,$success_codes,$efficiency,$status" >> "$RESULTS_DIR/optimized_results.csv"
}

# =============================================================================
# ПРОГРЕССИВНЫЙ ПОИСК МАКСИМАЛЬНОГО RPS
# =============================================================================

find_max_sustainable_rps() {
    echo "🔍 FINDING MAXIMUM SUSTAINABLE RPS"
    echo "=================================="

    local connections=500  # Оптимальное количество соединений
    local duration=60      # Продолжительность каждого теста

    # Стартовые значения для бинарного поиска (основано на ваших результатах)
    local min_rps=5000      # Сервис легко выдает 20K+, начнем с 5K
    local max_rps=50000     # Разумный верхний предел
    local sustainable_rps=0

    echo "Starting binary search between $min_rps and $max_rps RPS..."

    while [ $((max_rps - min_rps)) -gt 1000 ]; do
        local test_rps=$(( (min_rps + max_rps) / 2 ))

        echo ""
        echo "🎯 Testing RPS: $test_rps (range: $min_rps - $max_rps)"

        run_optimized_test "binary_search_${test_rps}" $duration $connections $test_rps "/ping" "Binary search for max RPS"

        # Проверяем результат последнего теста - упрощенно
        sleep 1  # Даем время записаться CSV

        local last_status=$(tail -1 "$RESULTS_DIR/optimized_results.csv" | cut -d',' -f11)
        local last_actual_rps=$(tail -1 "$RESULTS_DIR/optimized_results.csv" | cut -d',' -f3)

        echo "   📊 Result: Status='$last_status', Actual RPS='$last_actual_rps'"

        if [ "$last_status" = "SUCCESS" ] || [ "$last_status" = "PARTIAL" ]; then
            # Успешный тест
            sustainable_rps=$test_rps
            min_rps=$test_rps
            echo "   ✅ $test_rps RPS is sustainable, testing higher..."
        else
            # Неуспешный тест
            max_rps=$test_rps
            echo "   ❌ $test_rps RPS not sustainable, testing lower..."
        fi

        sleep 5  # Пауза между тестами
    done

    echo ""
    echo "🏆 MAXIMUM SUSTAINABLE RPS: $sustainable_rps"

    # Подтверждающий тест на максимальном RPS
    if [ $sustainable_rps -gt 0 ]; then
        echo ""
        echo "🔄 Confirmation test at maximum sustainable RPS..."
        run_optimized_test "max_sustainable_confirmation" 300 $connections $sustainable_rps "/ping" "Confirmation test - 5 minutes at max RPS"
    fi

    echo $sustainable_rps
}

# =============================================================================
# ОСНОВНОЙ СЦЕНАРИЙ ТЕСТИРОВАНИЯ
# =============================================================================

main_test_suite() {
    echo "🚀 STARTING OPTIMIZED PERFORMANCE TEST SUITE"
    echo "============================================"

    # Создаем заголовок CSV
    echo "test_name,target_qps,actual_rps,total_requests,avg_latency,p95_latency,p99_latency,errors,success_codes,efficiency,status" > "$RESULTS_DIR/optimized_results.csv"

    # 1. Прогрев системы
    echo "=== PHASE 1: SYSTEM WARMUP ==="
    run_optimized_test "warmup" 30 50 1000 "/ping" "System warmup"

    # 2. Базовые тесты
    echo "=== PHASE 2: BASELINE TESTS ==="
    for config_key in baseline normal_load increased_load; do
        IFS=':' read -r duration connections qps description <<< "${TEST_CONFIGS[$config_key]}"
        run_optimized_test "$config_key" "$duration" "$connections" "$qps" "/ping" "$description"
    done

    # 3. Поиск максимального RPS
    echo "=== PHASE 3: FINDING MAXIMUM RPS ==="
    MAX_RPS=$(find_max_sustainable_rps)

    # 4. Тесты различных эндпоинтов
    echo "=== PHASE 4: ENDPOINT COMPARISON ==="
    local test_rps=$((MAX_RPS / 2))  # Используем 50% от максимального

    for endpoint in "${ENDPOINTS[@]}"; do
        run_optimized_test "endpoint_$(echo $endpoint | tr '/' '_')" 120 500 $test_rps "$endpoint" "Endpoint comparison test"
    done

    # 5. Длительный тест стабильности
    echo "=== PHASE 5: STABILITY TEST ==="
    local stable_rps=$((MAX_RPS * 80 / 100))  # 80% от максимального
    run_optimized_test "stability" 600 500 $stable_rps "/ping" "10-minute stability test"

    echo ""
    echo "🎉 OPTIMIZED TEST SUITE COMPLETED!"
    echo "================================="
}

# =============================================================================
# =============================================================================
# ИСПОЛНИТЕЛЬНАЯ ЧАСТЬ СКРИПТА
# =============================================================================

# Создаем директорию для результатов
RESULTS_DIR="./optimized_test_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RESULTS_DIR"

# Проверяем аргументы командной строки
if [ $# -eq 0 ]; then
    echo "🚀 OPTIMIZED PERFORMANCE TEST SCRIPT"
    echo "===================================="
    echo ""
    echo "USAGE:"
    echo "  $0 <command> [arguments]"
    echo ""
    echo "COMMANDS:"
    echo "  find_max_rps           - Найти максимальный устойчивый RPS"
    echo "  full_suite             - Запустить полный набор тестов"
    echo "  custom <duration> <connections> <qps> [endpoint]"
    echo "                         - Запустить кастомный тест"
    echo "  baseline               - Запустить только базовые тесты"
    echo ""
    echo "EXAMPLES:"
    echo "  $0 find_max_rps"
    echo "  $0 full_suite"
    echo "  $0 custom 60 200 5000 /ping"
    echo "  $0 baseline"
    echo ""
    exit 0
fi

# Проверяем доступность сервиса
echo "🔍 Checking service availability..."
if ! curl -s "$SERVICE_URL/ping" > /dev/null 2>&1; then
    echo "❌ Service is not available at $SERVICE_URL"
    echo "Please make sure your service is running on port $SERVICE_PORT"
    exit 1
fi
echo "✅ Service is available"

# Проверяем зависимости
if ! command -v hey >/dev/null 2>&1; then
    echo "❌ 'hey' tool is not installed"
    echo "Install with: go install github.com/rakyll/hey@latest"
    exit 1
fi

if ! command -v bc >/dev/null 2>&1; then
    echo "⚠️  'bc' is not installed, some calculations may not work"
fi

# Обработка команд
case "$1" in
    "find_max_rps")
        echo "🎯 Starting maximum RPS discovery..."
        MAX_RPS=$(find_max_sustainable_rps)
        echo ""
        echo "🏆 FINAL RESULT: Maximum sustainable RPS = $MAX_RPS"
        ;;

    "full_suite")
        echo "🚀 Starting full test suite..."
        main_test_suite
        ;;

    "custom")
        if [ $# -lt 4 ]; then
            echo "❌ Custom test requires: duration connections qps [endpoint]"
            echo "Example: $0 custom 60 200 5000 /ping"
            exit 1
        fi

        DURATION=$2
        CONNECTIONS=$3
        QPS=$4
        ENDPOINT=${5:-"/ping"}

        echo "🎯 Starting custom test..."
        echo "test_name,target_qps,actual_rps,total_requests,avg_latency,p95_latency,p99_latency,errors,success_codes,efficiency,status" > "$RESULTS_DIR/optimized_results.csv"
        run_optimized_test "custom" "$DURATION" "$CONNECTIONS" "$QPS" "$ENDPOINT" "Custom user test"
        ;;

    "baseline")
        echo "📊 Starting baseline tests..."
        echo "test_name,target_qps,actual_rps,total_requests,avg_latency,p95_latency,p99_latency,errors,success_codes,efficiency,status" > "$RESULTS_DIR/optimized_results.csv"

        run_optimized_test "warmup" 30 50 1000 "/ping" "System warmup"
        run_optimized_test "baseline" 60 100 2000 "/ping" "Baseline test"
        run_optimized_test "normal_load" 120 200 5000 "/ping" "Normal load test"

        echo ""
        echo "📈 BASELINE RESULTS:"
        echo "==================="
        column -t -s ',' "$RESULTS_DIR/optimized_results.csv"
        ;;

    *)
        echo "❌ Unknown command: $1"
        echo "Run '$0' without arguments to see usage"
        exit 1
        ;;
esac

echo ""
echo "📁 Results saved to: $RESULTS_DIR"
echo "📋 Detailed results: $RESULTS_DIR/optimized_results.csv"