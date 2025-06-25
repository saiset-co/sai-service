#!/usr/bin/env python3
"""
Скрипт для визуализации результатов тестирования производительности
Использование: python3 visualize_results.py metrics_20240115_143022.csv
"""

import sys
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns
from datetime import datetime
import os
import glob

def load_metrics_data(csv_file):
    """Загрузка данных из CSV файла"""
    try:
        df = pd.read_csv(csv_file)
        df['timestamp'] = pd.to_datetime(df['timestamp'])
        return df
    except Exception as e:
        print(f"❌ Ошибка загрузки данных: {e}")
        return None

def create_performance_dashboard(df, output_dir="./charts"):
    """Создание дашборда с графиками производительности"""

    # Создаем директорию для графиков
    os.makedirs(output_dir, exist_ok=True)

    # Настройка стиля
    plt.style.use('seaborn-v0_8')
    sns.set_palette("husl")

    # 1. График RPS во времени
    plt.figure(figsize=(12, 6))
    plt.subplot(2, 2, 1)
    plt.plot(df['timestamp'], df['rps'], linewidth=2, color='#2E86AB')
    plt.title('🚀 Requests Per Second (RPS)', fontsize=14, fontweight='bold')
    plt.xlabel('Time')
    plt.ylabel('RPS')
    plt.grid(True, alpha=0.3)
    plt.xticks(rotation=45)

    # 2. График латентности
    plt.subplot(2, 2, 2)
    plt.plot(df['timestamp'], df['avg_latency_ms'], label='Average', linewidth=2, color='#A23B72')
    plt.plot(df['timestamp'], df['p99_latency_ms'], label='P99', linewidth=2, color='#F18F01')
    plt.title('⏱️ Response Latency', fontsize=14, fontweight='bold')
    plt.xlabel('Time')
    plt.ylabel('Latency (ms)')
    plt.legend()
    plt.grid(True, alpha=0.3)
    plt.xticks(rotation=45)

    # 3. График использования памяти
    plt.subplot(2, 2, 3)
    plt.plot(df['timestamp'], df['memory_mb'], linewidth=2, color='#C73E1D')
    plt.title('💾 Memory Usage', fontsize=14, fontweight='bold')
    plt.xlabel('Time')
    plt.ylabel('Memory (MB)')
    plt.grid(True, alpha=0.3)
    plt.xticks(rotation=45)

    # 4. График CPU и соединений
    plt.subplot(2, 2, 4)
    ax1 = plt.gca()
    line1 = ax1.plot(df['timestamp'], df['cpu_percent'], linewidth=2, color='#3F88C5', label='CPU %')
    ax1.set_xlabel('Time')
    ax1.set_ylabel('CPU %', color='#3F88C5')
    ax1.tick_params(axis='y', labelcolor='#3F88C5')

    ax2 = ax1.twinx()
    line2 = ax2.plot(df['timestamp'], df['connections'], linewidth=2, color='#D00000', label='Connections')
    ax2.set_ylabel('Connections', color='#D00000')
    ax2.tick_params(axis='y', labelcolor='#D00000')

    # Объединяем легенды
    lines = line1 + line2
    labels = [l.get_label() for l in lines]
    ax1.legend(lines, labels, loc='upper left')

    plt.title('💻 CPU & Connections', fontsize=14, fontweight='bold')
    plt.xticks(rotation=45)
    plt.grid(True, alpha=0.3)

    plt.tight_layout()
    plt.savefig(f"{output_dir}/performance_dashboard.png", dpi=300, bbox_inches='tight')
    plt.show()

def create_summary_stats(df):
    """Создание сводной статистики"""

    print("\n📊 PERFORMANCE STATISTICS SUMMARY")
    print("=" * 50)

    # RPS статистика
    print(f"\n🚀 REQUESTS PER SECOND:")
    print(f"   Average: {df['rps'].mean():.2f}")
    print(f"   Maximum: {df['rps'].max():.2f}")
    print(f"   Minimum: {df['rps'].min():.2f}")
    print(f"   Std Dev: {df['rps'].std():.2f}")

    # Латентность
    print(f"\n⏱️  RESPONSE LATENCY (ms):")
    print(f"   Avg Latency - Mean: {df['avg_latency_ms'].mean():.2f}")
    print(f"   Avg Latency - Max:  {df['avg_latency_ms'].max():.2f}")
    print(f"   P99 Latency - Mean: {df['p99_latency_ms'].mean():.2f}")
    print(f"   P99 Latency - Max:  {df['p99_latency_ms'].max():.2f}")

    # Системные ресурсы
    print(f"\n💻 SYSTEM RESOURCES:")
    print(f"   Memory - Average: {df['memory_mb'].mean():.2f} MB")
    print(f"   Memory - Peak:    {df['memory_mb'].max():.2f} MB")
    print(f"   CPU - Average:    {df['cpu_percent'].mean():.2f}%")
    print(f"   CPU - Peak:       {df['cpu_percent'].max():.2f}%")
    print(f"   Connections - Avg: {df['connections'].mean():.0f}")
    print(f"   Connections - Max: {df['connections'].max():.0f}")

    # Производительность
    total_requests = df['rps'].sum() * 5  # 5 секунд интервал
    duration_minutes = len(df) * 5 / 60

    print(f"\n📈 OVERALL PERFORMANCE:")
    print(f"   Test Duration:     {duration_minutes:.1f} minutes")
    print(f"   Total Requests:    ~{total_requests:.0f}")
    print(f"   Success Rate:      {(df['success_rate'].mean() / 100 * 100):.1f}%")
    print(f"   Stability Score:   {(100 - df['rps'].std() / df['rps'].mean() * 100):.1f}%")

def create_percentile_analysis(df, output_dir="./charts"):
    """Создание анализа перцентилей"""

    plt.figure(figsize=(12, 8))

    # RPS распределение
    plt.subplot(2, 2, 1)
    plt.hist(df['rps'], bins=20, alpha=0.7, color='#2E86AB', edgecolor='black')
    plt.axvline(df['rps'].median(), color='red', linestyle='--', linewidth=2, label=f'Median: {df["rps"].median():.0f}')
    plt.title('RPS Distribution')
    plt.xlabel('RPS')
    plt.ylabel('Frequency')
    plt.legend()
    plt.grid(True, alpha=0.3)

    # Латентность распределение
    plt.subplot(2, 2, 2)
    plt.hist(df['avg_latency_ms'], bins=20, alpha=0.7, color='#A23B72', edgecolor='black')
    plt.axvline(df['avg_latency_ms'].median(), color='red', linestyle='--', linewidth=2, label=f'Median: {df["avg_latency_ms"].median():.1f}ms')
    plt.title('Average Latency Distribution')
    plt.xlabel('Latency (ms)')
    plt.ylabel('Frequency')
    plt.legend()
    plt.grid(True, alpha=0.3)

    # Box plots
    plt.subplot(2, 2, 3)
    box_data = [df['rps'], df['avg_latency_ms'], df['memory_mb']]
    plt.boxplot(box_data, labels=['RPS', 'Latency(ms)', 'Memory(MB)'])
    plt.title('Performance Metrics Box Plot')
    plt.ylabel('Values')
    plt.grid(True, alpha=0.3)

    # Correlation heatmap
    plt.subplot(2, 2, 4)
    correlation_matrix = df[['rps', 'avg_latency_ms', 'memory_mb', 'cpu_percent', 'connections']].corr()
    sns.heatmap(correlation_matrix, annot=True, cmap='coolwarm', center=0, square=True)
    plt.title('Metrics Correlation')

    plt.tight_layout()
    plt.savefig(f"{output_dir}/performance_analysis.png", dpi=300, bbox_inches='tight')
    plt.show()

def generate_report(df, csv_file, output_dir="./charts"):
    """Генерация HTML отчета"""

    html_content = f"""
    <!DOCTYPE html>
    <html>
    <head>
        <title>Performance Test Report</title>
        <style>
            body {{ font-family: Arial, sans-serif; margin: 40px; background-color: #f5f5f5; }}
            .container {{ max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }}
            h1 {{ color: #2E86AB; border-bottom: 3px solid #2E86AB; padding-bottom: 10px; }}
            h2 {{ color: #A23B72; margin-top: 30px; }}
            .metric {{ background: #f8f9fa; padding: 15px; margin: 10px 0; border-radius: 5px; border-left: 4px solid #2E86AB; }}
            .stats-grid {{ display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 20px; margin: 20px 0; }}
            .stat-card {{ background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 20px; border-radius: 8px; text-align: center; }}
            .stat-value {{ font-size: 2em; font-weight: bold; }}
            .stat-label {{ font-size: 0.9em; opacity: 0.9; }}
            img {{ max-width: 100%; height: auto; margin: 20px 0; border-radius: 8px; box-shadow: 0 2px 8px rgba(0,0,0,0.15); }}
            .footer {{ margin-top: 40px; padding-top: 20px; border-top: 1px solid #ddd; color: #666; text-align: center; }}
        </style>
    </head>
    <body>
        <div class="container">
            <h1>🚀 Performance Test Report</h1>

            <div class="metric">
                <strong>📅 Test Date:</strong> {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}<br>
                <strong>📁 Data Source:</strong> {csv_file}<br>
                <strong>⏱️ Test Duration:</strong> {len(df) * 5 / 60:.1f} minutes<br>
                <strong>🔢 Data Points:</strong> {len(df)}
            </div>

            <h2>📊 Key Performance Metrics</h2>
            <div class="stats-grid">
                <div class="stat-card">
                    <div class="stat-value">{df['rps'].mean():.0f}</div>
                    <div class="stat-label">Average RPS</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">{df['avg_latency_ms'].mean():.1f}ms</div>
                    <div class="stat-label">Average Latency</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">{df['memory_mb'].mean():.0f}MB</div>
                    <div class="stat-label">Average Memory</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">{df['cpu_percent'].mean():.1f}%</div>
                    <div class="stat-label">Average CPU</div>
                </div>
            </div>

            <h2>📈 Performance Dashboard</h2>
            <img src="performance_dashboard.png" alt="Performance Dashboard">

            <h2>📊 Statistical Analysis</h2>
            <img src="performance_analysis.png" alt="Performance Analysis">

            <h2>💡 Performance Insights</h2>
            <div class="metric">
                <strong>🎯 Peak Performance:</strong> {df['rps'].max():.0f} RPS at {df.loc[df['rps'].idxmax(), 'timestamp']}<br>
                <strong>⚡ Fastest Response:</strong> {df['avg_latency_ms'].min():.1f}ms<br>
                <strong>🏔️ Memory Peak:</strong> {df['memory_mb'].max():.0f}MB<br>
                <strong>🔄 Max Connections:</strong> {df['connections'].max():.0f}
            </div>

            <div class="footer">
                Generated by SAI-Service Performance Monitor | {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}
            </div>
        </div>
    </body>
    </html>
    """

    report_file = f"{output_dir}/performance_report.html"
    with open(report_file, 'w', encoding='utf-8') as f:
        f.write(html_content)

    print(f"\n📄 HTML Report generated: {report_file}")

def main():
    """Основная функция"""

    if len(sys.argv) < 2:
        # Ищем последний CSV файл автоматически
        csv_files = glob.glob("metrics_*.csv")
        if not csv_files:
            print("❌ No metrics CSV files found!")
            print("Usage: python3 visualize_results.py <csv_file>")
            print("Or run the monitoring script first: ./monitor.sh")
            sys.exit(1)

        # Берем самый новый файл
        csv_file = max(csv_files, key=os.path.getctime)
        print(f"📁 Auto-detected latest CSV file: {csv_file}")
    else:
        csv_file = sys.argv[1]

    if not os.path.exists(csv_file):
        print(f"❌ File not found: {csv_file}")
        sys.exit(1)

    print(f"📊 Loading performance data from {csv_file}...")

    # Загружаем данные
    df = load_metrics_data(csv_file)
    if df is None:
        sys.exit(1)

    print(f"✅ Loaded {len(df)} data points")
    print(f"📅 Time range: {df['timestamp'].min()} to {df['timestamp'].max()}")

    # Создаем директорию для результатов
    output_dir = "./performance_charts"
    os.makedirs(output_dir, exist_ok=True)

    try:
        # Генерируем визуализации
        print("\n🎨 Generating performance dashboard...")
        create_performance_dashboard(df, output_dir)

        print("📈 Generating statistical analysis...")
        create_percentile_analysis(df, output_dir)

        print("📄 Generating HTML report...")
        generate_report(df, csv_file, output_dir)

        # Выводим статистику
        create_summary_stats(df)

        print(f"\n🎉 Analysis completed successfully!")
        print(f"📁 All files saved to: {output_dir}/")
        print(f"🌐 Open performance_report.html in your browser to view the full report")

    except Exception as e:
        print(f"❌ Error during analysis: {e}")
        print("💡 Make sure you have the required packages installed:")
        print("   pip install pandas matplotlib seaborn")

def check_dependencies():
    """Проверка зависимостей"""
    required_packages = ['pandas', 'matplotlib', 'seaborn']
    missing = []

    for package in required_packages:
        try:
            __import__(package)
        except ImportError:
            missing.append(package)

    if missing:
        print("❌ Missing required packages:")
        for package in missing:
            print(f"   - {package}")
        print(f"\nInstall them with: pip install {' '.join(missing)}")
        return False

    return True

if __name__ == "__main__":
    print("🚀 SAI-Service Performance Visualizer")
    print("=====================================")

    # Проверяем зависимости
    if not check_dependencies():
        sys.exit(1)

    main()