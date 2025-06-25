#!/bin/bash

# Скрипт для оптимизации системы под высокие нагрузки
# ВНИМАНИЕ: Требует sudo прав!

echo "🔧 SYSTEM OPTIMIZATION FOR HIGH LOAD TESTING"
echo "============================================="
echo ""

# Проверяем права sudo
if [ "$EUID" -ne 0 ]; then
    echo "⚠️  This script requires sudo privileges for system optimization"
    echo "Run with: sudo ./optimize_system.sh"
    echo ""
    echo "Or run individual commands manually:"
    echo ""
fi

# Функция для безопасного выполнения команд
safe_execute() {
    local cmd="$1"
    local desc="$2"

    echo "🔧 $desc"
    echo "   Command: $cmd"

    if [ "$EUID" -eq 0 ]; then
        eval "$cmd"
        if [ $? -eq 0 ]; then
            echo "   ✅ Success"
        else
            echo "   ❌ Failed"
        fi
    else
        echo "   ⏭️  Skipped (requires sudo)"
    fi
    echo ""
}

# Показываем текущие лимиты
echo "📊 CURRENT SYSTEM LIMITS:"
echo "========================="
echo "File descriptors (soft): $(ulimit -Sn)"
echo "File descriptors (hard): $(ulimit -Hn)"
echo "Max processes: $(ulimit -u)"
echo "Max memory: $(ulimit -m)"
echo ""

# Показываем текущие сетевые настройки
echo "🌐 CURRENT NETWORK SETTINGS:"
echo "============================="
echo "somaxconn: $(cat /proc/sys/net/core/somaxconn 2>/dev/null || echo 'N/A')"
echo "tcp_max_syn_backlog: $(cat /proc/sys/net/ipv4/tcp_max_syn_backlog 2>/dev/null || echo 'N/A')"
echo "netdev_max_backlog: $(cat /proc/sys/net/core/netdev_max_backlog 2>/dev/null || echo 'N/A')"
echo "tcp_tw_reuse: $(cat /proc/sys/net/ipv4/tcp_tw_reuse 2>/dev/null || echo 'N/A')"
echo ""

echo "🚀 APPLYING OPTIMIZATIONS:"
echo "=========================="

# 1. Увеличиваем лимиты файловых дескрипторов
echo "1. File Descriptor Limits"
echo "-------------------------"

# Для текущей сессии
ulimit -n 1000000 2>/dev/null && echo "   ✅ Session ulimit updated" || echo "   ⚠️  Session ulimit update failed"

# Для системы (требует перезагрузки)
safe_execute "echo '* soft nofile 1000000' >> /etc/security/limits.conf" "Add soft nofile limit"
safe_execute "echo '* hard nofile 1000000' >> /etc/security/limits.conf" "Add hard nofile limit"
safe_execute "echo 'root soft nofile 1000000' >> /etc/security/limits.conf" "Add root soft nofile limit"
safe_execute "echo 'root hard nofile 1000000' >> /etc/security/limits.conf" "Add root hard nofile limit"

# 2. Оптимизация сетевых настроек
echo "2. Network Optimizations"
echo "------------------------"

safe_execute "echo 'net.core.somaxconn = 65536' >> /etc/sysctl.conf" "Increase socket listen backlog"
safe_execute "echo 'net.ipv4.tcp_max_syn_backlog = 30000' >> /etc/sysctl.conf" "Increase SYN backlog"
safe_execute "echo 'net.core.netdev_max_backlog = 30000' >> /etc/sysctl.conf" "Increase netdev backlog"
safe_execute "echo 'net.ipv4.tcp_tw_reuse = 1' >> /etc/sysctl.conf" "Enable TIME_WAIT reuse"
safe_execute "echo 'net.ipv4.tcp_fin_timeout = 15' >> /etc/sysctl.conf" "Reduce FIN timeout"
safe_execute "echo 'net.ipv4.tcp_keepalive_time = 300' >> /etc/sysctl.conf" "Reduce keepalive time"
safe_execute "echo 'net.ipv4.tcp_keepalive_probes = 5' >> /etc/sysctl.conf" "Reduce keepalive probes"
safe_execute "echo 'net.ipv4.tcp_keepalive_intvl = 15' >> /etc/sysctl.conf" "Reduce keepalive interval"

# 3. Применяем настройки
safe_execute "sysctl -p" "Apply sysctl changes"

# 4. Оптимизации для производительности
echo "3. Performance Optimizations"
echo "----------------------------"

safe_execute "echo 'net.core.rmem_default = 262144' >> /etc/sysctl.conf" "Increase default receive buffer"
safe_execute "echo 'net.core.rmem_max = 16777216' >> /etc/sysctl.conf" "Increase max receive buffer"
safe_execute "echo 'net.core.wmem_default = 262144' >> /etc/sysctl.conf" "Increase default send buffer"
safe_execute "echo 'net.core.wmem_max = 16777216' >> /etc/sysctl.conf" "Increase max send buffer"
safe_execute "echo 'net.ipv4.tcp_rmem = 4096 87380 16777216' >> /etc/sysctl.conf" "Optimize TCP receive memory"
safe_execute "echo 'net.ipv4.tcp_wmem = 4096 87380 16777216' >> /etc/sysctl.conf" "Optimize TCP send memory"

# 5. Настройки для высокой нагрузки
echo "4. High Load Settings"
echo "--------------------"

safe_execute "echo 'fs.file-max = 2000000' >> /etc/sysctl.conf" "Increase max open files"
safe_execute "echo 'net.ipv4.ip_local_port_range = 1024 65535' >> /etc/sysctl.conf" "Expand port range"
safe_execute "echo 'net.netfilter.nf_conntrack_max = 1000000' >> /etc/sysctl.conf" "Increase connection tracking" 2>/dev/null || true

# Применяем все изменения
safe_execute "sysctl -p" "Apply all sysctl changes"

echo ""
echo "💻 GO RUNTIME OPTIMIZATIONS:"
echo "============================="

cat << 'EOF'
Add these environment variables before running your service:

# CPU optimization
export GOMAXPROCS=$(nproc)

# Memory optimization
export GOGC=50          # More aggressive garbage collection
export GOMEMLIMIT=8GiB  # Set memory limit (adjust for your system)

# Network optimization
export GODEBUG=netdns=go+1

# Example startup command:
GOMAXPROCS=$(nproc) GOGC=50 ./your-service

# Or in systemd service file:
[Service]
Environment=GOMAXPROCS=auto
Environment=GOGC=50
Environment=GOMEMLIMIT=8GiB
EOF

echo ""
echo "🔍 VERIFICATION COMMANDS:"
echo "========================="

cat << 'EOF'
After reboot, verify the changes:

# Check file descriptor limits
ulimit -n

# Check network settings
sysctl net.core.somaxconn
sysctl net.ipv4.tcp_max_syn_backlog
sysctl net.core.netdev_max_backlog

# Monitor during tests
watch -n 1 'ss -tuln | wc -l; echo "Active connections"'
watch -n 1 'cat /proc/sys/fs/file-nr'
EOF

echo ""
echo "⚠️  IMPORTANT NOTES:"
echo "==================="
echo "• Some changes require a system reboot to take effect"
echo "• File descriptor limits need a new shell session"
echo "• Monitor system resources during high load tests"
echo "• These settings are optimized for testing, adjust for production"
echo ""

# Создаем скрипт для быстрой проверки
cat > check_limits.sh << 'EOF'
#!/bin/bash
echo "=== CURRENT SYSTEM LIMITS ==="
echo "File descriptors: $(ulimit -n)"
echo "Max processes: $(ulimit -u)"
echo ""
echo "=== NETWORK SETTINGS ==="
echo "somaxconn: $(cat /proc/sys/net/core/somaxconn)"
echo "tcp_max_syn_backlog: $(cat /proc/sys/net/ipv4/tcp_max_syn_backlog)"
echo "netdev_max_backlog: $(cat /proc/sys/net/core/netdev_max_backlog)"
echo ""
echo "=== CURRENT CONNECTIONS ==="
echo "TCP connections: $(ss -t | wc -l)"
echo "Listening sockets: $(ss -tln | wc -l)"
echo "File descriptors in use: $(lsof | wc -l)"
EOF

chmod +x check_limits.sh

echo "📋 Created check_limits.sh script for monitoring"
echo ""
echo "🔄 TO APPLY ALL CHANGES:"
echo "========================"
echo "1. Run: sudo ./optimize_system.sh"
echo "2. Restart your shell session (or reboot)"
echo "3. Verify with: ./check_limits.sh"
echo "4. Run high load tests: ./high_load_test.sh"
echo ""
echo "✅ System optimization script completed!"