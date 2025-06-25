#!/bin/bash

# RPS-ориентированный тест производительности
# Находит максимальный RPS при оптимальном количестве соединений

SERVICE_URL="http://localhost:8081"
RESULTS_DIR="./rps_test_$(date +%Y%m%d_%H%M%S)"
OPTIMAL_CONNECTIONS=1000  # Начальное значение, будет оптимизировано

mkdir -p "$RESULTS_DIR"

echo "🚀 RPS-FOCUSED PERFORMANCE TESTING"
echo "================================="
echo "Service: $SERVICE_URL"
echo "Results: $RESULTS_DIR"
echo ""

# Проверяем доступность сервиса
if ! curl -s "$SERVICE_URL/ping" > /dev/null; then
    echo "❌ Service is not available"
    exit 1
fi

# Функция для запуска RPS-теста
run_rps_test() {
    local test_name=$1
    local target_rps=$2
    local connections=$3
    local duration=${4:-30}
    local endpoint=${5:-"/ping"}

    echo "🎯 RPS TEST: $test_name"
    echo "   Target RPS: $target_rps"
    echo "   Connections: $connections"
    echo "   Duration: ${duration}s"
    echo "   Endpoint: $endpoint"

    local output_file="$RESULTS_DIR/${test_name}.txt"
    local start_time=$(date +%s)

    # Используем -q для указания конкретного RPS
    timeout $((duration + 10)) hey -q $target_rps -z ${duration}s -c $connections -t 30 "$SERVICE_URL$endpoint" > "$output_file" 2>&1
    local test_exit_code=$?

    local end_time=$(date +%s)
    local actual_duration=$((end_time - start_time))

    if [ $test_exit_code -eq 0 ] && [ -f "$output_file" ]; then
        # Очищаем все переменные от переносов строк и пробелов
        local actual_rps=$(grep "Requests/sec:" "$output_file" | awk '{print $2}' | head -1 | tr -d '\n\r ' || echo "0")
        local avg_latency=$(grep "Average:" "$output_file" | awk '{print $2}' | head -1 | tr -d '\n\r ' || echo "0")
        local p99_latency=$(grep "99% in" "$output_file" | awk '{print $3}' | head -1 | tr -d '\n\r ' || echo "0")
        local error_rate=$(grep -c "Non-2xx\|timeout\|error" "$output_file" 2>/dev/null || echo "0")
        local success_rate=$(grep "Status code distribution:" "$output_file" -A 5 | grep "\[200\]" | awk '{print $2}' | tr -d '\n\r ' || echo "0")

        # Убеждаемся что все числовые переменные корректны
        if ! [[ "$actual_rps" =~ ^[0-9]+\.?[0-9]*$ ]]; then
            actual_rps="0"
        fi
        if ! [[ "$error_rate" =~ ^[0-9]+$ ]]; then
            error_rate="0"
        fi
        if ! [[ "$success_rate" =~ ^[0-9]+$ ]]; then
            success_rate="0"
        fi
        if ! [[ "$target_rps" =~ ^[0-9]+$ ]]; then
            target_rps="1"
        fi

        # Рассчитываем эффективность достижения цели - исправляем проблему с bc
        local rps_ratio
        if command -v bc >/dev/null 2>&1 && [ "$target_rps" -gt 0 ]; then
            rps_ratio=$(echo "scale=3; $actual_rps / $target_rps" | bc -l 2>/dev/null | tr -d '\n' || echo "0.0")
        else
            # Fallback: простое деление с помощью awk
            rps_ratio=$(awk "BEGIN {printf \"%.3f\", $actual_rps / $target_rps}" 2>/dev/null || echo "0.0")
        fi

        # Убеждаемся что ratio это число
        if ! [[ "$rps_ratio" =~ ^[0-9]+\.?[0-9]*$ ]]; then
            rps_ratio="0.0"
        fi

        local achievement_percent=$(awk "BEGIN {printf \"%.1f\", $rps_ratio * 100}" 2>/dev/null || echo "0.0")

        echo "   📊 Target RPS: $target_rps"
        echo "   ✅ Actual RPS: ${actual_rps:-N/A}"
        echo "   📈 Achievement: ${achievement_percent}%"
        echo "   ⏱️  Avg Latency: ${avg_latency:-N/A}"
        echo "   📊 P99 Latency: ${p99_latency:-N/A}"
        echo "   ✅ Success: ${success_rate:-0}"
        echo "   ❌ Errors: ${error_rate:-0}"

        # Записываем в сводку
        echo "$test_name,$target_rps,$actual_rps,$connections,$avg_latency,$p99_latency,$error_rate,$success_rate,$rps_ratio" >> "$RESULTS_DIR/rps_summary.csv"

        # Определяем успешность теста с простыми условиями
        if [ "$actual_rps" != "0" ]; then
            local rps_ratio_int=$(awk "BEGIN {printf \"%.0f\", ($actual_rps / $target_rps) * 100}")

            if [ "$rps_ratio_int" -ge 90 ] && [ "$error_rate" -lt 10 ]; then
                echo "   🎉 SUCCESS: Achieved target RPS!"
                return 0
            elif [ "$rps_ratio_int" -ge 70 ] && [ "$error_rate" -lt 100 ]; then
                echo "   ⚠️  PARTIAL: Close to target but not optimal"
                return 1
            else
                echo "   ❌ FAILED: Cannot achieve target RPS (${rps_ratio_int}% of target)"
                return 2
            fi
        else
            echo "   ❌ FAILED: No valid RPS data"
            return 2
        fi
    else
        echo "   ❌ TEST FAILED (exit code: $test_exit_code)"
        echo "$test_name,$target_rps,FAILED,$connections,FAILED,FAILED,FAILED,FAILED,0" >> "$RESULTS_DIR/rps_summary.csv"
        return 2
    fi

    echo ""
}

# Функция для поиска оптимального количества соединений для заданного RPS
find_optimal_connections() {
    local target_rps=$1
    local test_duration=${2:-20}

    echo "🔍 Finding optimal connections for $target_rps RPS..."

    # Тестируем разные количества соединений
    local connections_list=(50 100 200 500 1000 2000 5000)
    local best_connections=100
    local best_ratio=0

    for conn in "${connections_list[@]}"; do
        echo "   Testing $conn connections..."

        if run_rps_test "optimize_${target_rps}_${conn}" $target_rps $conn $test_duration "/ping"; then
            # Получаем ratio из последней записи, очищаем от мусора
            local current_ratio=$(tail -1 "$RESULTS_DIR/rps_summary.csv" | cut -d',' -f9 | tr -d '\n\r ')

            # Проверяем что ratio корректный
            if [[ "$current_ratio" =~ ^[0-9]+\.?[0-9]*$ ]]; then
                if [ "$(awk "BEGIN {print ($current_ratio > $best_ratio) ? 1 : 0}")" = "1" ]; then
                    best_ratio=$current_ratio
                    best_connections=$conn
                fi
            fi
        fi

        sleep 2  # Пауза между тестами
    done

    echo "🏆 Optimal connections for $target_rps RPS: $best_connections (efficiency: $(awk "BEGIN {printf \"%.1f\", $best_ratio * 100}")%)"
    OPTIMAL_CONNECTIONS=$best_connections
}

# Создаем заголовок для CSV
echo "test_name,target_rps,actual_rps,connections,avg_latency,p99_latency,errors,success_requests,efficiency_ratio" > "$RESULTS_DIR/rps_summary.csv"

echo "🚀 Starting RPS-focused testing..."
echo ""

# PHASE 1: Находим оптимальное количество соединений для среднего RPS
echo "=== PHASE 1: OPTIMIZING CONNECTIONS ==="
find_optimal_connections 10000 15

echo ""
echo "=== PHASE 2: PROGRESSIVE RPS INCREASE ==="

echo "Using optimal connections: $OPTIMAL_CONNECTIONS"
echo ""

# Прогрессивно увеличиваем RPS
TARGET_RPS_LIST=(
    1000
    5000
    10000
    15000
    20000
    30000
    40000
    50000
    60000
    70000
    80000
    100000
    120000
    150000
    200000
)

MAX_ACHIEVED_RPS=0
MAX_STABLE_RPS=0

for target_rps in "${TARGET_RPS_LIST[@]}"; do
    echo "🎯 Testing target RPS: $target_rps"

    result=$(run_rps_test "progressive_${target_rps}" $target_rps $OPTIMAL_CONNECTIONS 30 "/ping")
    test_result=$?

    # Получаем фактический RPS из результата
    actual_rps=$(tail -1 "$RESULTS_DIR/rps_summary.csv" | cut -d',' -f3)

    # Проверяем что actual_rps это число и сравниваем безопасно
    if [[ "$actual_rps" =~ ^[0-9]+\.?[0-9]*$ ]]; then
        # Используем awk для сравнения чисел с плавающей точкой
        if [ "$(awk "BEGIN {print ($actual_rps > $MAX_ACHIEVED_RPS) ? 1 : 0}")" = "1" ]; then
            MAX_ACHIEVED_RPS=$(printf "%.2f" "$actual_rps")
        fi

        if [ $test_result -eq 0 ] && [ "$(awk "BEGIN {print ($actual_rps > $MAX_STABLE_RPS) ? 1 : 0}")" = "1" ]; then
            MAX_STABLE_RPS=$(printf "%.2f" "$actual_rps")
        fi
    fi

    # Если тест провалился дважды подряд, прекращаем
    if [ $test_result -eq 2 ]; then
        CONSECUTIVE_FAILURES=$((CONSECUTIVE_FAILURES + 1))
        if [ $CONSECUTIVE_FAILURES -ge 2 ]; then
            echo "⚠️  Two consecutive failures - stopping RPS increase"
            break
        fi
    else
        CONSECUTIVE_FAILURES=0
    fi

    sleep 3
done

echo ""
echo "=== PHASE 3: FINE-TUNING MAXIMUM RPS ==="

if [ -n "$MAX_STABLE_RPS" ] && [ "$MAX_STABLE_RPS" -gt 0 ]; then
    echo "🔧 Fine-tuning around maximum stable RPS: $MAX_STABLE_RPS"

    # Тестируем значения вокруг максимального стабильного RPS
    FINE_TUNE_START=$(awk "BEGIN {printf \"%.0f\", $MAX_STABLE_RPS * 0.9}")  # -10%
    FINE_TUNE_END=$(awk "BEGIN {printf \"%.0f\", $MAX_STABLE_RPS * 1.2}")     # +20%
    FINE_TUNE_STEP=$(awk "BEGIN {printf \"%.0f\", $MAX_STABLE_RPS * 0.05}")   # шаг 5%

    # Убеждаемся что step не равен 0
    if [ "$FINE_TUNE_STEP" -eq 0 ]; then
        FINE_TUNE_STEP=100
    fi

    for rps in $(seq $FINE_TUNE_START $FINE_TUNE_STEP $FINE_TUNE_END); do
        echo "🎯 Fine-tuning RPS: $rps"
        run_rps_test "finetune_${rps}" $rps $OPTIMAL_CONNECTIONS 45 "/ping"
        sleep 2
    done
fi

echo ""
echo "=== PHASE 4: SUSTAINED MAXIMUM LOAD ==="

# Длительный тест на максимальном стабильном RPS
if [ -n "$MAX_STABLE_RPS" ] && [ "$MAX_STABLE_RPS" != "0" ] && [ "$MAX_STABLE_RPS" != "0.0" ]; then
    # Конвертируем в целое для безопасного сравнения
    MAX_STABLE_INT=$(awk "BEGIN {printf \"%.0f\", $MAX_STABLE_RPS}")
    if [ "$MAX_STABLE_INT" -gt 0 ]; then
        echo "🔥 Testing sustained load at maximum stable RPS: $MAX_STABLE_RPS for 5 minutes..."
        run_rps_test "sustained_max" "$MAX_STABLE_INT" $OPTIMAL_CONNECTIONS 300 "/ping"
    fi
fi

echo ""
echo "=== PHASE 5: DIFFERENT ENDPOINTS AT MAX RPS ==="

if [ -n "$MAX_STABLE_RPS" ] && [ "$MAX_STABLE_RPS" != "0" ] && [ "$MAX_STABLE_RPS" != "0.0" ]; then
    # Конвертируем в целое для безопасного использования
    MAX_STABLE_INT=$(awk "BEGIN {printf \"%.0f\", $MAX_STABLE_RPS}")
    HALF_STABLE_INT=$(awk "BEGIN {printf \"%.0f\", $MAX_STABLE_RPS / 2}")

    if [ "$MAX_STABLE_INT" -gt 0 ]; then
        # Тестируем разные эндпоинты на максимальном RPS
        run_rps_test "max_hello" "$MAX_STABLE_INT" $OPTIMAL_CONNECTIONS 60 "/hello/maxrps"
        run_rps_test "max_data" "$HALF_STABLE_INT" $OPTIMAL_CONNECTIONS 60 "/data"  # Более тяжелый эндпоинт
    fi
fi

echo ""
echo "🎉 RPS-FOCUSED TESTING COMPLETED!"
echo "================================="

# Анализируем результаты
echo ""
echo "📊 RPS PERFORMANCE ANALYSIS:"
echo "============================"

echo "Results Summary:"
echo "---------------"
cat "$RESULTS_DIR/rps_summary.csv" | column -t -s ','

echo ""
echo "🏆 PEAK PERFORMANCE METRICS:"
echo "============================"

ABSOLUTE_MAX=$(grep -v "FAILED\|actual_rps" "$RESULTS_DIR/rps_summary.csv" | awk -F',' '{print $3}' | sort -nr | head -1)
STABLE_MAX=$(grep -v "FAILED\|actual_rps" "$RESULTS_DIR/rps_summary.csv" | awk -F',' '$9 >= 0.9 {print $3}' | sort -nr | head -1)

echo "• Absolute Maximum RPS: ${ABSOLUTE_MAX:-N/A}"
echo "• Maximum Stable RPS (>90% efficiency): ${STABLE_MAX:-N/A}"
echo "• Optimal Connections: $OPTIMAL_CONNECTIONS"

# Находим точку деградации RPS
echo ""
echo "📉 RPS DEGRADATION ANALYSIS:"
echo "============================"

python3 << EOF 2>/dev/null || echo "Python analysis skipped"
import csv

try:
    with open('$RESULTS_DIR/rps_summary.csv', 'r') as f:
        reader = csv.DictReader(f)
        progressive_tests = [row for row in reader if row['test_name'].startswith('progressive_') and row['actual_rps'] != 'FAILED']

    if len(progressive_tests) > 0:
        print("RPS Achievement Analysis:")
        print("Target RPS -> Actual RPS (Efficiency)")
        print("------------------------------------")

        for row in progressive_tests:
            target = int(row['target_rps'])
            actual = float(row['actual_rps'])
            efficiency = float(row['efficiency_ratio']) * 100
            latency = row['avg_latency']

            status = "✅" if efficiency >= 90 else "⚠️" if efficiency >= 70 else "❌"
            print(f"{status} {target:6d} -> {actual:8.0f} ({efficiency:5.1f}%) | Latency: {latency}")

            # Определяем точку деградации
            if efficiency < 80:
                print(f"\n⚠️  RPS degradation starts around {target} target RPS")
                print(f"   System can only achieve {actual:.0f} RPS ({efficiency:.1f}% of target)")
                break

except Exception as e:
    print(f"Analysis error: {e}")
EOF

echo ""
echo "💡 RPS OPTIMIZATION RECOMMENDATIONS:"
echo "==================================="

# Проверяем STABLE_MAX и конвертируем в целое число для сравнения
if [ -n "$STABLE_MAX" ] && [ "$STABLE_MAX" != "N/A" ]; then
    # Конвертируем в целое число для сравнения
    STABLE_MAX_INT=$(printf "%.0f" "$STABLE_MAX" 2>/dev/null || echo "0")

    if [ "$STABLE_MAX_INT" -gt 0 ]; then
        RECOMMENDED_RPS=$(awk "BEGIN {printf \"%.0f\", $STABLE_MAX * 0.8}")
        echo "Production RPS Settings:"
        echo "• Recommended max RPS: $RECOMMENDED_RPS (80% of stable max)"
        echo "• Burst capacity: $STABLE_MAX RPS"
        echo "• Optimal connections: $OPTIMAL_CONNECTIONS"
        echo "• Monitor latency threshold: 50ms average, 200ms P99"
    else
        echo "⚠️  Could not determine stable maximum RPS"
        echo "• Review test results for system bottlenecks"
        echo "• Consider system optimization"
    fi
else
    echo "⚠️  Could not determine stable maximum RPS"
    echo "• Review test results for system bottlenecks"
    echo "• Consider system optimization"
fi

echo ""
echo "🔧 SYSTEM TUNING FOR HIGHER RPS:"
echo "==============================="
echo "1. Connection pooling: Use $OPTIMAL_CONNECTIONS connections"
echo "2. Keep-alive: Enable HTTP keep-alive"
echo "3. Buffer sizes: Increase network buffers"
echo "4. Worker threads: Match to CPU cores"
echo "5. Memory: Pre-allocate response buffers"
echo "6. OS limits: Increase file descriptors and network limits"

# Создаем финальный отчет
cat > "$RESULTS_DIR/rps_report.txt" << EOF
RPS-FOCUSED LOAD TEST REPORT
============================
Test Date: $(date)
Service: $SERVICE_URL

MAXIMUM RPS ACHIEVED:
• Peak RPS: ${ABSOLUTE_MAX:-N/A}
• Stable RPS: ${STABLE_MAX:-N/A}
• Optimal Connections: $OPTIMAL_CONNECTIONS

PRODUCTION RECOMMENDATIONS:
• Target RPS: $(awk "BEGIN {printf \"%.0f\", $STABLE_MAX * 0.8}" 2>/dev/null || echo "N/A")
• Max burst RPS: $STABLE_MAX
• Connection pool size: $OPTIMAL_CONNECTIONS
• Latency SLA: <50ms average, <200ms P99

OPTIMIZATION PRIORITY:
1. Maintain connection count at $OPTIMAL_CONNECTIONS
2. Focus on reducing latency rather than increasing connections
3. Monitor RPS achievement ratio (target vs actual)
4. Set alerts at 80% of maximum stable RPS

FILES:
• Detailed results: rps_summary.csv
• Individual tests: *.txt
EOF

echo ""
echo "📁 All results saved to: $RESULTS_DIR"
echo "📋 RPS report: $RESULTS_DIR/rps_report.txt"
echo ""
echo "✅ RPS-focused testing completed successfully!"