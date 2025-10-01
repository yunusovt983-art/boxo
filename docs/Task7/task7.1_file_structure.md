# Task 7.1 - Структура файлов и компонентов

## Полная структура файлов Task 7.1

### 📁 Основная директория: `bitswap/loadtest/`

```
bitswap/loadtest/
├── comprehensive_load_test.go      # Основной набор комплексных тестов
├── load_test_core.go              # Ядро системы нагрузочного тестирования
├── types.go                       # Типы данных и конфигурации
├── utils.go                       # Вспомогательные функции
├── test_runner.go                 # Оркестратор выполнения тестов
├── high_load_tests.go             # Специализированные тесты высокой нагрузки
├── performance_bench_test.go      # Go бенчмарки производительности
├── stress_test.go                 # Стресс-тесты и восстановление
├── run_load_tests.go              # CLI интерфейс для запуска тестов
├── simple_test_runner.go          # Упрощенный раннер для быстрых тестов
└── test_results/                  # Директория результатов тестов
    ├── load_test_summary.txt
    ├── benchmark_results.txt
    ├── cpu.prof
    ├── mem.prof
    ├── coverage.out
    ├── coverage.html
    ├── test_output.log
    ├── system_monitor.log
    └── report.md
```

### 📁 Конфигурационные файлы

```
bitswap/
├── bitswap_load_test_config.json  # Основная конфигурация тестов
├── Makefile.loadtest              # Автоматизация тестирования
└── LOAD_TESTING_README.md         # Документация пользователя
```

### 📁 Интеграционные файлы

```
bitswap/
├── performance_bench_test.go      # Интеграционные бенчмарки
├── stress_test.go                 # Интеграционные стресс-тесты
└── simple_test_runner.go          # Простой раннер тестов
```

## Детальное описание файлов

### 🔧 Ядро системы тестирования

#### `comprehensive_load_test.go` (1,200+ строк)
**Назначение**: Основной файл с полным набором нагрузочных тестов

**Основные тесты**:
```go
func TestBitswapComprehensiveLoadSuite(t *testing.T) {
    // Главный набор тестов с подтестами:
    
    t.Run("Test_10K_ConcurrentConnections", func(t *testing.T) {
        // Тестирование 10,000+ одновременных соединений
        // Валидация латентности < 100ms для 95% запросов
        // Проверка успешности соединений > 95%
    })
    
    t.Run("Test_100K_RequestsPerSecond", func(t *testing.T) {
        // Тестирование 100,000+ запросов в секунду
        // Проверка автомасштабирования
        // Валидация P95 латентности < 200ms
    })
    
    t.Run("Test_24Hour_Stability", func(t *testing.T) {
        // Длительные тесты стабильности (24+ часа)
        // Обнаружение утечек памяти
        // Мониторинг деградации производительности
    })
    
    t.Run("Test_Automated_Benchmarks", func(t *testing.T) {
        // Автоматизированные бенчмарки производительности
        // Масштабируемые тесты (small/medium/large/xlarge)
        // Измерение эффективности и ресурсов
    })
    
    t.Run("Test_Extreme_Load_Scenarios", func(t *testing.T) {
        // Экстремальные сценарии нагрузки
        // 50K+ соединений, 200K+ запросов/сек
        // Большие блоки, множество мелких блоков
    })
    
    t.Run("Test_Memory_Leak_Detection", func(t *testing.T) {
        // Обнаружение утечек памяти
        // Множественные циклы нагрузки
        // Мониторинг роста памяти
    })
    
    t.Run("Test_Network_Conditions", func(t *testing.T) {
        // Тестирование различных сетевых условий
        // Латентность от 0ms до 500ms
        // Симуляция различных типов сетей
    })
}
```

**Ключевые функции**:
- `setupLoadTestEnvironment()` - настройка тестового окружения
- `validatePerformanceMetrics()` - валидация метрик производительности
- `generateLoadTestReport()` - генерация отчетов
- `cleanupLoadTestResources()` - очистка ресурсов

#### `load_test_core.go` (800+ строк)
**Назначение**: Ядро системы нагрузочного тестирования

**Основные компоненты**:
```go
type LoadTestEngine struct {
    config          *LoadTestConfig
    metrics         *PerformanceMetrics
    resourceMonitor *ResourceMonitor
    testInstances   []TestInstance
    resultCollector *ResultCollector
}

// Основные методы движка
func (lte *LoadTestEngine) RunConcurrentConnectionTest(connections int) (*TestResults, error)
func (lte *LoadTestEngine) RunThroughputTest(targetRPS int) (*TestResults, error)
func (lte *LoadTestEngine) RunStabilityTest(duration time.Duration) (*TestResults, error)
func (lte *LoadTestEngine) RunBenchmarkSuite() (*BenchmarkResults, error)
```

**Ключевые функции**:
- `NewLoadTestEngine()` - создание движка тестирования
- `ConcurrentConnectionTest()` - тестирование соединений
- `ThroughputTest()` - тестирование пропускной способности
- `StabilityTest()` - тестирование стабильности
- `BenchmarkRunner()` - запуск бенчмарков

#### `types.go` (400+ строк)
**Назначение**: Определение всех типов данных и структур

**Основные типы**:
```go
type LoadTestConfig struct {
    TestTimeout           time.Duration `json:"test_timeout"`
    StabilityDuration     time.Duration `json:"stability_duration"`
    MaxConcurrentConns    int          `json:"max_concurrent_connections"`
    MaxRequestRate        int          `json:"max_request_rate"`
    EnableLongRunning     bool         `json:"enable_long_running"`
    EnableStressTests     bool         `json:"enable_stress_tests"`
    EnableBenchmarks      bool         `json:"enable_benchmarks"`
    OutputDir            string        `json:"output_dir"`
    GenerateReports      bool         `json:"generate_reports"`
}

type TestResults struct {
    TestName           string                 `json:"test_name"`
    StartTime          time.Time             `json:"start_time"`
    EndTime            time.Time             `json:"end_time"`
    Duration           time.Duration         `json:"duration"`
    TotalRequests      int64                 `json:"total_requests"`
    SuccessfulRequests int64                 `json:"successful_requests"`
    FailedRequests     int64                 `json:"failed_requests"`
    RequestsPerSecond  float64               `json:"requests_per_second"`
    AverageLatency     time.Duration         `json:"average_latency"`
    P95Latency         time.Duration         `json:"p95_latency"`
    P99Latency         time.Duration         `json:"p99_latency"`
    MaxLatency         time.Duration         `json:"max_latency"`
    MemoryUsage        MemoryMetrics         `json:"memory_usage"`
    ConnectionMetrics  ConnectionMetrics     `json:"connection_metrics"`
    ErrorDetails       map[string]int        `json:"error_details"`
}

type PerformanceMetrics struct {
    RequestMetrics    RequestMetrics    `json:"request_metrics"`
    ConnectionMetrics ConnectionMetrics `json:"connection_metrics"`
    ResourceMetrics   ResourceMetrics   `json:"resource_metrics"`
    LatencyMetrics    LatencyMetrics    `json:"latency_metrics"`
}
```

#### `utils.go` (600+ строк)
**Назначение**: Вспомогательные функции и утилиты

**Основные функции**:
```go
// Генерация тестовых данных
func GenerateTestBlocks(count int, sizeRange [2]int) []blocks.Block
func GenerateTestPeers(count int) []peer.ID
func GenerateTestRequests(count int, blocks []blocks.Block) []TestRequest

// Валидация результатов
func ValidatePerformanceResults(results *TestResults, criteria *PerformanceCriteria) error
func ValidateLatencyRequirements(latency LatencyMetrics, requirements LatencyRequirements) error
func ValidateMemoryUsage(memory MemoryMetrics, limits MemoryLimits) error

// Анализ производительности
func AnalyzePerformanceTrends(results []*TestResults) *PerformanceAnalysis
func CalculateEfficiencyMetrics(results *TestResults, nodeCount int) *EfficiencyMetrics
func DetectPerformanceRegressions(current, baseline *TestResults) *RegressionAnalysis

// Генерация отчетов
func GenerateLoadTestReport(results []*TestResults, config *LoadTestConfig) (*LoadTestReport, error)
func GenerateHTMLReport(report *LoadTestReport, outputPath string) error
func GenerateJSONReport(report *LoadTestReport, outputPath string) error

// Мониторинг ресурсов
func StartResourceMonitoring(interval time.Duration) *ResourceMonitor
func CollectSystemMetrics() *SystemMetrics
func MonitorMemoryLeaks(duration time.Duration) *MemoryLeakReport
```

### 🛠️ Специализированные тесты

#### `high_load_tests.go` (500+ строк)
**Назначение**: Тесты экстремально высокой нагрузки

**Тестовые сценарии**:
```go
func TestExtremeConnectionLoad(t *testing.T) {
    // Тестирование 50,000+ одновременных соединений
    testCases := []struct {
        name        string
        connections int
        duration    time.Duration
        criteria    PerformanceCriteria
    }{
        {"50K_Connections", 50000, 3*time.Minute, extremeCriteria},
        {"75K_Connections", 75000, 2*time.Minute, extremeCriteria},
        {"100K_Connections", 100000, 1*time.Minute, extremeCriteria},
    }
}

func TestExtremeThroughputLoad(t *testing.T) {
    // Тестирование 200,000+ запросов в секунду
    testCases := []struct {
        name      string
        targetRPS int
        duration  time.Duration
        minAchievement float64
    }{
        {"200K_RPS", 200000, 30*time.Second, 0.3},
        {"250K_RPS", 250000, 20*time.Second, 0.2},
        {"300K_RPS", 300000, 10*time.Second, 0.1},
    }
}

func TestLargeBlockHandling(t *testing.T) {
    // Тестирование больших блоков (10MB+)
    blockSizes := []int{
        10 * 1024 * 1024,  // 10MB
        50 * 1024 * 1024,  // 50MB
        100 * 1024 * 1024, // 100MB
    }
}

func TestManySmallBlocks(t *testing.T) {
    // Тестирование множества мелких блоков (100K+)
    blockCounts := []int{100000, 250000, 500000, 1000000}
}
```

#### `stress_test.go` (400+ строк)
**Назначение**: Стресс-тестирование и восстановление

**Стресс-сценарии**:
```go
func TestResourceExhaustionStress(t *testing.T) {
    // Тестирование при нехватке ресурсов
    // Ограничение памяти, CPU, файловых дескрипторов
}

func TestFailureRecoveryStress(t *testing.T) {
    // Тестирование восстановления после сбоев
    // Симуляция сбоев узлов, сети, дисков
}

func TestGracefulDegradationStress(t *testing.T) {
    // Тестирование graceful degradation
    // Постепенное увеличение нагрузки до критической
}

func TestConcurrentStressScenarios(t *testing.T) {
    // Множественные стресс-сценарии одновременно
    // Комбинированная нагрузка различных типов
}
```

#### `performance_bench_test.go` (300+ строк)
**Назначение**: Go бенчмарки производительности

**Бенчмарки**:
```go
func BenchmarkBitswapScaling(b *testing.B) {
    // Бенчмарки масштабируемости
    scales := []int{10, 25, 50, 100, 200}
    for _, scale := range scales {
        b.Run(fmt.Sprintf("Scale_%d", scale), func(b *testing.B) {
            // Тестирование с различным количеством узлов
        })
    }
}

func BenchmarkBitswapThroughput(b *testing.B) {
    // Бенчмарки пропускной способности
    throughputs := []int{1000, 5000, 10000, 25000, 50000}
    for _, rps := range throughputs {
        b.Run(fmt.Sprintf("RPS_%d", rps), func(b *testing.B) {
            // Тестирование различных уровней RPS
        })
    }
}

func BenchmarkBitswapLatency(b *testing.B) {
    // Бенчмарки латентности
    latencies := []time.Duration{
        0 * time.Millisecond,    // Perfect network
        1 * time.Millisecond,    // LAN
        10 * time.Millisecond,   // Fast WAN
        50 * time.Millisecond,   // Slow WAN
        200 * time.Millisecond,  // Satellite
        500 * time.Millisecond,  // Very poor
    }
}

func BenchmarkBitswapMemoryEfficiency(b *testing.B) {
    // Бенчмарки эффективности памяти
    // Измерение req/sec на MB памяти
}
```

### 🔧 Оркестрация и управление

#### `test_runner.go` (350+ строк)
**Назначение**: Оркестратор выполнения тестов

**Основные компоненты**:
```go
type TestSuiteRunner struct {
    config      *LoadTestConfig
    testSuites  []TestSuite
    results     []*TestResults
    reporter    *TestReporter
    monitor     *ResourceMonitor
}

func (tsr *TestSuiteRunner) RunAllTests() error
func (tsr *TestSuiteRunner) RunTestSuite(suite TestSuite) (*TestResults, error)
func (tsr *TestSuiteRunner) RunParallelTests(suites []TestSuite) ([]*TestResults, error)
func (tsr *TestSuiteRunner) GenerateReport() (*TestReport, error)
```

#### `run_load_tests.go` (200+ строк)
**Назначение**: CLI интерфейс для запуска тестов

**CLI команды**:
```go
func main() {
    // Поддержка команд:
    // --test-type: concurrent|throughput|stability|benchmarks|all
    // --duration: продолжительность тестов
    // --connections: количество соединений
    // --rps: целевой RPS
    // --output-dir: директория результатов
    // --config: файл конфигурации
}
```

#### `simple_test_runner.go` (150+ строк)
**Назначение**: Упрощенный раннер для быстрых тестов

**Функции**:
```go
func RunSmokeTest() error
func RunQuickPerformanceTest() error
func RunCIFriendlyTests() error
func ValidateTestInfrastructure() error
```

### 📊 Автоматизация

#### `Makefile.loadtest` (400+ строк)
**Назначение**: Полная автоматизация нагрузочного тестирования

**Основные команды**:
```makefile
# Основные тесты
test-concurrent:     # Тесты 10K+ соединений
test-throughput:     # Тесты 100K+ req/sec
test-stability:      # Тесты 24h стабильности
test-benchmarks:     # Автоматизированные бенчмарки

# Комплексные тесты
test-all:           # Полный набор тестов
test-ci:            # CI-дружественные тесты
test-stress:        # Стресс-тесты
test-extreme:       # Экстремальные сценарии

# Специализированные тесты
test-memory:        # Тесты утечек памяти
test-network:       # Тесты сетевых условий
test-smoke:         # Smoke tests

# Бенчмарки
bench:              # Все бенчмарки
bench-scaling:      # Бенчмарки масштабируемости
bench-throughput:   # Бенчмарки пропускной способности
bench-latency:      # Бенчмарки латентности

# Профилирование
profile-cpu:        # CPU профилирование
profile-memory:     # Профилирование памяти
test-race:          # Race detection
test-coverage:      # Анализ покрытия

# Утилиты
validate-env:       # Валидация окружения
requirements:       # Системные требования
monitor:           # Мониторинг ресурсов
clean:             # Очистка результатов
```

### 📋 Конфигурация

#### `bitswap_load_test_config.json`
**Назначение**: Основная конфигурация всех тестов

```json
{
  "test_timeout": "30m",
  "stability_duration": "24h",
  "max_concurrent_connections": 50000,
  "max_request_rate": 200000,
  "enable_long_running": true,
  "enable_stress_tests": true,
  "enable_benchmarks": true,
  "output_dir": "test_results",
  "generate_reports": true,
  
  "performance_criteria": {
    "concurrent_connections": {
      "min_success_rate": 0.95,
      "max_avg_latency": "100ms",
      "max_p95_latency": "100ms"
    },
    "throughput": {
      "min_achievement_rate": 0.6,
      "max_p95_latency": "200ms"
    },
    "stability": {
      "max_memory_growth": 0.05,
      "min_success_rate": 0.98
    }
  },
  
  "test_scenarios": {
    "concurrent_connections": [10000, 15000, 20000, 25000, 50000],
    "throughput_targets": [50000, 75000, 100000, 125000, 150000, 200000],
    "stability_loads": [500, 1000, 2000],
    "benchmark_scales": [10, 25, 50, 100]
  },
  
  "resource_limits": {
    "max_memory_mb": 32768,
    "max_cpu_cores": 16,
    "max_file_descriptors": 65536,
    "max_goroutines": 100000
  }
}
```

## Взаимодействие компонентов

### Поток выполнения тестов

```
1. Makefile.loadtest
   ↓ (запускает)
2. go test ./loadtest
   ↓ (загружает)
3. comprehensive_load_test.go
   ↓ (использует)
4. load_test_core.go (LoadTestEngine)
   ↓ (конфигурируется через)
5. types.go (LoadTestConfig)
   ↓ (использует утилиты)
6. utils.go (генерация данных, валидация)
   ↓ (запускает специализированные тесты)
7. high_load_tests.go / stress_test.go
   ↓ (собирает результаты)
8. test_runner.go (TestSuiteRunner)
   ↓ (генерирует отчеты)
9. test_results/ (файлы результатов)
```

### Зависимости между файлами

```
comprehensive_load_test.go
├── load_test_core.go (LoadTestEngine)
├── types.go (конфигурации и типы)
├── utils.go (утилиты и валидация)
├── test_runner.go (оркестрация)
├── high_load_tests.go (экстремальные тесты)
├── stress_test.go (стресс-тесты)
└── performance_bench_test.go (бенчмарки)

Makefile.loadtest
├── bitswap_load_test_config.json (конфигурация)
├── run_load_tests.go (CLI интерфейс)
├── simple_test_runner.go (быстрые тесты)
└── test_results/ (результаты)
```

## Метрики и результаты

### Собираемые метрики

**Производительность**:
- Запросы в секунду (RPS)
- Латентность (avg, P95, P99, max)
- Успешность запросов (%)
- Пропускная способность (bytes/sec)

**Ресурсы**:
- Использование CPU (%)
- Использование памяти (MB)
- Количество горутин
- Файловые дескрипторы
- Сетевые соединения

**Стабильность**:
- Рост памяти за период (%)
- Количество утечек памяти
- Деградация производительности
- Частота ошибок

### Форматы результатов

**JSON** (программная обработка):
```json
{
  "test_name": "Test_10K_ConcurrentConnections",
  "start_time": "2024-01-01T00:00:00Z",
  "duration": "5m30s",
  "total_requests": 500000,
  "successful_requests": 485000,
  "requests_per_second": 1515.15,
  "average_latency": "65ms",
  "p95_latency": "95ms",
  "memory_usage": {
    "initial_mb": 256,
    "peak_mb": 1024,
    "final_mb": 280,
    "growth_percent": 9.375
  }
}
```

**Текстовый отчет** (человеко-читаемый):
```
Load Test Summary
================
Test: 10K Concurrent Connections
Duration: 5m30s
Total Requests: 500,000
Successful: 485,000 (97.0%)
Failed: 15,000 (3.0%)
RPS: 1,515.15
Latency: avg=65ms, P95=95ms, P99=120ms, max=250ms
Memory: 256MB → 1024MB → 280MB (9.4% growth)
Status: ✅ PASSED (success rate > 95%)
```

**HTML отчет** (визуальный):
- Графики производительности
- Диаграммы использования ресурсов
- Таблицы детальных метрик
- Сравнение с предыдущими результатами

Эта структура обеспечивает полное покрытие всех требований Task 7.1 и предоставляет мощную, масштабируемую систему нагрузочного тестирования для Bitswap.