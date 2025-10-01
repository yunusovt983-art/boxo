# Task 7.1 - Руководство по реализации нагрузочных тестов

## Обзор реализации

Этот документ описывает детальную реализацию Task 7.1 "Создать нагрузочные тесты для Bitswap" с техническими деталями, архитектурными решениями и примерами кода.

## Архитектура системы нагрузочного тестирования

### Основные компоненты

```
┌─────────────────────────────────────────────────────────────┐
│                    Load Test System                        │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌──────────────┐ │
│  │ Test Controller │  │ Metrics         │  │ Report       │ │
│  │                 │  │ Collector       │  │ Generator    │ │
│  └─────────────────┘  └─────────────────┘  └──────────────┘ │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌──────────────┐ │
│  │ Load Test       │  │ Resource        │  │ Test         │ │
│  │ Engine          │  │ Monitor         │  │ Validator    │ │
│  └─────────────────┘  └─────────────────┘  └──────────────┘ │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌──────────────┐ │
│  │ Test Instances  │  │ Data Generator  │  │ Result       │ │
│  │ Manager         │  │                 │  │ Aggregator   │ │
│  └─────────────────┘  └─────────────────┘  └──────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Детальная реализация компонентов

### 1. LoadTestEngine - Ядро системы

```go
type LoadTestEngine struct {
    config          *LoadTestConfig
    metrics         *PerformanceMetrics
    resourceMonitor *ResourceMonitor
    testInstances   []TestInstance
    resultCollector *ResultCollector
    logger          *slog.Logger
    
    // Управление жизненным циклом
    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
    
    // Синхронизация
    mu       sync.RWMutex
    running  bool
    results  []*TestResults
}
```#
## Ключевые методы LoadTestEngine

```go
func NewLoadTestEngine(config *LoadTestConfig) *LoadTestEngine {
    ctx, cancel := context.WithCancel(context.Background())
    
    return &LoadTestEngine{
        config:          config,
        metrics:         NewPerformanceMetrics(),
        resourceMonitor: NewResourceMonitor(),
        resultCollector: NewResultCollector(),
        logger:          slog.Default(),
        ctx:             ctx,
        cancel:          cancel,
        testInstances:   make([]TestInstance, 0),
        results:         make([]*TestResults, 0),
    }
}

func (lte *LoadTestEngine) RunConcurrentConnectionTest(connections int) (*TestResults, error) {
    lte.mu.Lock()
    defer lte.mu.Unlock()
    
    if lte.running {
        return nil, errors.New("test already running")
    }
    
    lte.running = true
    defer func() { lte.running = false }()
    
    // Настройка тестового окружения
    testEnv, err := lte.setupTestEnvironment(connections)
    if err != nil {
        return nil, fmt.Errorf("failed to setup test environment: %w", err)
    }
    defer testEnv.Cleanup()
    
    // Запуск мониторинга ресурсов
    lte.resourceMonitor.Start(lte.ctx)
    defer lte.resourceMonitor.Stop()
    
    // Выполнение теста
    results := &TestResults{
        TestName:  fmt.Sprintf("ConcurrentConnections_%d", connections),
        StartTime: time.Now(),
    }
    
    err = lte.executeConcurrentConnectionTest(testEnv, connections, results)
    results.EndTime = time.Now()
    results.Duration = results.EndTime.Sub(results.StartTime)
    
    if err != nil {
        return results, fmt.Errorf("test execution failed: %w", err)
    }
    
    // Валидация результатов
    if err := lte.validateResults(results); err != nil {
        return results, fmt.Errorf("result validation failed: %w", err)
    }
    
    lte.results = append(lte.results, results)
    return results, nil
}
```

### 2. Реализация тестов 10,000+ соединений

```go
func (lte *LoadTestEngine) executeConcurrentConnectionTest(
    testEnv *TestEnvironment, 
    connections int, 
    results *TestResults,
) error {
    // Создание пула воркеров
    workerPool := make(chan struct{}, connections)
    for i := 0; i < connections; i++ {
        workerPool <- struct{}{}
    }
    
    var wg sync.WaitGroup
    var totalRequests, successfulRequests int64
    latencies := make([]time.Duration, 0, connections)
    latencyMu := sync.Mutex{}
    
    // Запуск конкурентных соединений
    for i := 0; i < connections; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            <-workerPool // Получить разрешение на выполнение
            
            atomic.AddInt64(&totalRequests, 1)
            
            start := time.Now()
            err := lte.performSingleRequest(testEnv, workerID)
            latency := time.Since(start)
            
            latencyMu.Lock()
            latencies = append(latencies, latency)
            latencyMu.Unlock()
            
            if err == nil {
                atomic.AddInt64(&successfulRequests, 1)
            } else {
                lte.logger.Debug("Request failed", 
                    "worker_id", workerID, 
                    "error", err)
            }
            
            workerPool <- struct{}{} // Вернуть разрешение
        }(i)
    }
    
    wg.Wait()
    
    // Агрегация результатов
    results.TotalRequests = totalRequests
    results.SuccessfulRequests = successfulRequests
    results.FailedRequests = totalRequests - successfulRequests
    
    // Расчет метрик латентности
    sort.Slice(latencies, func(i, j int) bool {
        return latencies[i] < latencies[j]
    })
    
    if len(latencies) > 0 {
        results.AverageLatency = calculateAverage(latencies)
        results.P95Latency = calculatePercentile(latencies, 0.95)
        results.P99Latency = calculatePercentile(latencies, 0.99)
        results.MaxLatency = latencies[len(latencies)-1]
    }
    
    // Расчет RPS
    if results.Duration > 0 {
        results.RequestsPerSecond = float64(totalRequests) / results.Duration.Seconds()
    }
    
    return nil
}
```

### 3. Реализация тестов 100,000+ RPS

```go
func (lte *LoadTestEngine) RunThroughputTest(targetRPS int) (*TestResults, error) {
    results := &TestResults{
        TestName:  fmt.Sprintf("Throughput_%d_RPS", targetRPS),
        StartTime: time.Now(),
    }
    
    // Настройка rate limiter
    rateLimiter := rate.NewLimiter(rate.Limit(targetRPS), targetRPS/10)
    
    // Настройка тестового окружения
    testEnv, err := lte.setupTestEnvironment(targetRPS / 100) // Меньше соединений, больше RPS
    if err != nil {
        return results, fmt.Errorf("failed to setup test environment: %w", err)
    }
    defer testEnv.Cleanup()
    
    // Запуск теста на определенное время
    testDuration := 60 * time.Second
    testCtx, testCancel := context.WithTimeout(lte.ctx, testDuration)
    defer testCancel()
    
    var wg sync.WaitGroup
    var totalRequests, successfulRequests int64
    
    // Воркеры для генерации нагрузки
    numWorkers := runtime.NumCPU() * 4
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            for {
                select {
                case <-testCtx.Done():
                    return
                default:
                    // Ожидание разрешения от rate limiter
                    if err := rateLimiter.Wait(testCtx); err != nil {
                        return
                    }
                    
                    atomic.AddInt64(&totalRequests, 1)
                    
                    // Выполнение запроса
                    if err := lte.performSingleRequest(testEnv, workerID); err == nil {
                        atomic.AddInt64(&successfulRequests, 1)
                    }
                }
            }
        }(i)
    }
    
    wg.Wait()
    
    results.EndTime = time.Now()
    results.Duration = results.EndTime.Sub(results.StartTime)
    results.TotalRequests = totalRequests
    results.SuccessfulRequests = successfulRequests
    results.FailedRequests = totalRequests - successfulRequests
    
    if results.Duration > 0 {
        results.RequestsPerSecond = float64(totalRequests) / results.Duration.Seconds()
    }
    
    // Расчет достижения целевого RPS
    achievementRate := results.RequestsPerSecond / float64(targetRPS)
    results.Metadata = map[string]interface{}{
        "target_rps":        targetRPS,
        "actual_rps":        results.RequestsPerSecond,
        "achievement_rate":  achievementRate,
    }
    
    return results, nil
}
```

### 4. Реализация тестов 24+ часовой стабильности

```go
func (lte *LoadTestEngine) RunStabilityTest(duration time.Duration) (*TestResults, error) {
    results := &TestResults{
        TestName:  fmt.Sprintf("Stability_%s", duration.String()),
        StartTime: time.Now(),
    }
    
    // Настройка длительного тестового окружения
    testEnv, err := lte.setupLongRunningTestEnvironment()
    if err != nil {
        return results, fmt.Errorf("failed to setup long-running test environment: %w", err)
    }
    defer testEnv.Cleanup()
    
    // Контекст для длительного теста
    testCtx, testCancel := context.WithTimeout(lte.ctx, duration)
    defer testCancel()
    
    // Мониторинг памяти
    memoryMonitor := NewMemoryMonitor()
    memoryMonitor.Start(testCtx, 30*time.Second) // Каждые 30 секунд
    defer memoryMonitor.Stop()
    
    // Постоянная нагрузка
    constantLoad := 1000 // 1000 RPS постоянной нагрузки
    rateLimiter := rate.NewLimiter(rate.Limit(constantLoad), constantLoad/10)
    
    var wg sync.WaitGroup
    var totalRequests, successfulRequests int64
    
    // Воркеры для постоянной нагрузки
    numWorkers := 20
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            for {
                select {
                case <-testCtx.Done():
                    return
                default:
                    if err := rateLimiter.Wait(testCtx); err != nil {
                        return
                    }
                    
                    atomic.AddInt64(&totalRequests, 1)
                    
                    if err := lte.performSingleRequest(testEnv, workerID); err == nil {
                        atomic.AddInt64(&successfulRequests, 1)
                    }
                }
            }
        }(i)
    }
    
    wg.Wait()
    
    results.EndTime = time.Now()
    results.Duration = results.EndTime.Sub(results.StartTime)
    results.TotalRequests = totalRequests
    results.SuccessfulRequests = successfulRequests
    results.FailedRequests = totalRequests - successfulRequests
    
    // Анализ стабильности памяти
    memoryReport := memoryMonitor.GenerateReport()
    results.MemoryUsage = MemoryMetrics{
        InitialMB:     memoryReport.InitialMemoryMB,
        PeakMB:        memoryReport.PeakMemoryMB,
        FinalMB:       memoryReport.FinalMemoryMB,
        GrowthPercent: memoryReport.GrowthPercent,
    }
    
    // Проверка критериев стабильности
    if memoryReport.GrowthPercent > 5.0 {
        return results, fmt.Errorf("memory growth %.2f%% exceeds 5%% threshold", 
            memoryReport.GrowthPercent)
    }
    
    successRate := float64(successfulRequests) / float64(totalRequests)
    if successRate < 0.98 {
        return results, fmt.Errorf("success rate %.2f%% below 98%% threshold", 
            successRate*100)
    }
    
    return results, nil
}
```

### 5. Реализация автоматизированных бенчмарков

```go
func (lte *LoadTestEngine) RunBenchmarkSuite() (*BenchmarkResults, error) {
    benchmarkResults := &BenchmarkResults{
        StartTime: time.Now(),
        Benchmarks: make(map[string]*BenchmarkResult),
    }
    
    // Определение масштабов тестирования
    scales := []BenchmarkScale{
        {"Small", 10, 500, 100},
        {"Medium", 25, 1000, 500},
        {"Large", 50, 2000, 1000},
        {"XLarge", 100, 5000, 2000},
    }
    
    for _, scale := range scales {
        lte.logger.Info("Running benchmark", "scale", scale.Name)
        
        result, err := lte.runSingleBenchmark(scale)
        if err != nil {
            lte.logger.Error("Benchmark failed", "scale", scale.Name, "error", err)
            continue
        }
        
        benchmarkResults.Benchmarks[scale.Name] = result
    }
    
    benchmarkResults.EndTime = time.Now()
    benchmarkResults.Duration = benchmarkResults.EndTime.Sub(benchmarkResults.StartTime)
    
    // Анализ эффективности масштабирования
    lte.analyzeBenchmarkEfficiency(benchmarkResults)
    
    return benchmarkResults, nil
}

func (lte *LoadTestEngine) runSingleBenchmark(scale BenchmarkScale) (*BenchmarkResult, error) {
    // Настройка тестового окружения для конкретного масштаба
    testEnv, err := lte.setupBenchmarkEnvironment(scale.Nodes, scale.Blocks)
    if err != nil {
        return nil, fmt.Errorf("failed to setup benchmark environment: %w", err)
    }
    defer testEnv.Cleanup()
    
    result := &BenchmarkResult{
        Scale:     scale,
        StartTime: time.Now(),
    }
    
    // Прогрев системы
    lte.warmupSystem(testEnv, scale.Requests/10)
    
    // Выполнение бенчмарка
    var wg sync.WaitGroup
    var totalRequests, successfulRequests int64
    var totalLatency int64
    
    startTime := time.Now()
    
    for i := 0; i < scale.Requests; i++ {
        wg.Add(1)
        go func(requestID int) {
            defer wg.Done()
            
            atomic.AddInt64(&totalRequests, 1)
            
            reqStart := time.Now()
            err := lte.performBenchmarkRequest(testEnv, requestID)
            latency := time.Since(reqStart)
            
            atomic.AddInt64(&totalLatency, int64(latency))
            
            if err == nil {
                atomic.AddInt64(&successfulRequests, 1)
            }
        }(i)
    }
    
    wg.Wait()
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(startTime)
    result.TotalRequests = totalRequests
    result.SuccessfulRequests = successfulRequests
    result.RequestsPerSecond = float64(totalRequests) / result.Duration.Seconds()
    result.AverageLatency = time.Duration(totalLatency / totalRequests)
    
    // Расчет эффективности
    result.Efficiency = result.RequestsPerSecond / float64(scale.Nodes)
    
    // Сбор метрик использования ресурсов
    result.ResourceMetrics = lte.collectResourceMetrics()
    
    return result, nil
}
```

### 6. Система мониторинга ресурсов

```go
type ResourceMonitor struct {
    ctx        context.Context
    cancel     context.CancelFunc
    interval   time.Duration
    metrics    []ResourceSnapshot
    mu         sync.RWMutex
    running    bool
}

func NewResourceMonitor() *ResourceMonitor {
    return &ResourceMonitor{
        interval: 5 * time.Second,
        metrics:  make([]ResourceSnapshot, 0),
    }
}

func (rm *ResourceMonitor) Start(ctx context.Context) {
    rm.mu.Lock()
    defer rm.mu.Unlock()
    
    if rm.running {
        return
    }
    
    rm.ctx, rm.cancel = context.WithCancel(ctx)
    rm.running = true
    
    go rm.monitorLoop()
}

func (rm *ResourceMonitor) monitorLoop() {
    ticker := time.NewTicker(rm.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-rm.ctx.Done():
            return
        case <-ticker.C:
            snapshot := rm.collectSnapshot()
            
            rm.mu.Lock()
            rm.metrics = append(rm.metrics, snapshot)
            rm.mu.Unlock()
        }
    }
}

func (rm *ResourceMonitor) collectSnapshot() ResourceSnapshot {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    return ResourceSnapshot{
        Timestamp:    time.Now(),
        MemoryMB:     float64(m.Alloc) / 1024 / 1024,
        GoroutineCount: runtime.NumGoroutine(),
        HeapObjects:  m.HeapObjects,
        GCPauses:     time.Duration(m.PauseNs[(m.NumGC+255)%256]),
    }
}
```

### 7. Валидация результатов

```go
type ResultValidator struct {
    criteria map[string]PerformanceCriteria
}

func NewResultValidator() *ResultValidator {
    return &ResultValidator{
        criteria: map[string]PerformanceCriteria{
            "concurrent_connections": {
                MinSuccessRate:   0.95,
                MaxAvgLatency:    100 * time.Millisecond,
                MaxP95Latency:    100 * time.Millisecond,
            },
            "throughput": {
                MinAchievementRate: 0.6,
                MaxP95Latency:      200 * time.Millisecond,
            },
            "stability": {
                MaxMemoryGrowth: 0.05,
                MinSuccessRate:  0.98,
            },
        },
    }
}

func (rv *ResultValidator) ValidateResults(results *TestResults) error {
    testType := rv.determineTestType(results.TestName)
    criteria, exists := rv.criteria[testType]
    if !exists {
        return fmt.Errorf("no validation criteria for test type: %s", testType)
    }
    
    // Проверка успешности
    successRate := float64(results.SuccessfulRequests) / float64(results.TotalRequests)
    if successRate < criteria.MinSuccessRate {
        return fmt.Errorf("success rate %.2f%% below threshold %.2f%%", 
            successRate*100, criteria.MinSuccessRate*100)
    }
    
    // Проверка латентности
    if criteria.MaxAvgLatency > 0 && results.AverageLatency > criteria.MaxAvgLatency {
        return fmt.Errorf("average latency %v exceeds threshold %v", 
            results.AverageLatency, criteria.MaxAvgLatency)
    }
    
    if criteria.MaxP95Latency > 0 && results.P95Latency > criteria.MaxP95Latency {
        return fmt.Errorf("P95 latency %v exceeds threshold %v", 
            results.P95Latency, criteria.MaxP95Latency)
    }
    
    // Проверка роста памяти (для тестов стабильности)
    if criteria.MaxMemoryGrowth > 0 && results.MemoryUsage.GrowthPercent > criteria.MaxMemoryGrowth {
        return fmt.Errorf("memory growth %.2f%% exceeds threshold %.2f%%", 
            results.MemoryUsage.GrowthPercent*100, criteria.MaxMemoryGrowth*100)
    }
    
    return nil
}
```

### 8. Генерация отчетов

```go
type ReportGenerator struct {
    templates map[string]*template.Template
}

func NewReportGenerator() *ReportGenerator {
    return &ReportGenerator{
        templates: make(map[string]*template.Template),
    }
}

func (rg *ReportGenerator) GenerateLoadTestReport(
    results []*TestResults, 
    config *LoadTestConfig,
) (*LoadTestReport, error) {
    report := &LoadTestReport{
        GeneratedAt: time.Now(),
        Config:      config,
        Results:     results,
        Summary:     rg.generateSummary(results),
    }
    
    // Анализ трендов производительности
    report.PerformanceAnalysis = rg.analyzePerformanceTrends(results)
    
    // Выявление регрессий
    report.RegressionAnalysis = rg.detectRegressions(results)
    
    // Рекомендации по оптимизации
    report.Recommendations = rg.generateRecommendations(results)
    
    return report, nil
}

func (rg *ReportGenerator) GenerateHTMLReport(
    report *LoadTestReport, 
    outputPath string,
) error {
    tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Load Test Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .summary { background: #f5f5f5; padding: 15px; border-radius: 5px; }
        .test-result { margin: 20px 0; padding: 15px; border: 1px solid #ddd; }
        .success { border-left: 5px solid #4CAF50; }
        .failure { border-left: 5px solid #f44336; }
        .metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 10px; }
        .metric { background: #fff; padding: 10px; border: 1px solid #eee; text-align: center; }
    </style>
</head>
<body>
    <h1>Load Test Report</h1>
    <div class="summary">
        <h2>Summary</h2>
        <p>Generated: {{.GeneratedAt.Format "2006-01-02 15:04:05"}}</p>
        <p>Total Tests: {{len .Results}}</p>
        <p>Overall Success Rate: {{.Summary.OverallSuccessRate | printf "%.2f"}}%</p>
    </div>
    
    {{range .Results}}
    <div class="test-result {{if gt .SuccessRate 0.95}}success{{else}}failure{{end}}">
        <h3>{{.TestName}}</h3>
        <div class="metrics">
            <div class="metric">
                <strong>{{.TotalRequests}}</strong><br>
                Total Requests
            </div>
            <div class="metric">
                <strong>{{.SuccessfulRequests}}</strong><br>
                Successful
            </div>
            <div class="metric">
                <strong>{{.RequestsPerSecond | printf "%.2f"}}</strong><br>
                RPS
            </div>
            <div class="metric">
                <strong>{{.AverageLatency}}</strong><br>
                Avg Latency
            </div>
        </div>
    </div>
    {{end}}
</body>
</html>
    `
    
    t, err := template.New("report").Parse(tmpl)
    if err != nil {
        return fmt.Errorf("failed to parse template: %w", err)
    }
    
    file, err := os.Create(outputPath)
    if err != nil {
        return fmt.Errorf("failed to create report file: %w", err)
    }
    defer file.Close()
    
    return t.Execute(file, report)
}
```

## Интеграция с Makefile

Система автоматизации через Makefile обеспечивает простое использование всех компонентов:

```makefile
test-concurrent:
	@echo "Running 10,000+ concurrent connections test..."
	@mkdir -p test_results
	go test -v -timeout=30m -run="TestBitswapComprehensiveLoadSuite/Test_10K_ConcurrentConnections" ./loadtest

test-throughput:
	@echo "Running 100,000+ requests per second test..."
	@mkdir -p test_results
	go test -v -timeout=20m -run="TestBitswapComprehensiveLoadSuite/Test_100K_RequestsPerSecond" ./loadtest

test-stability:
	@echo "Running stability test (duration: $(or $(BITSWAP_STABILITY_DURATION),30m))..."
	@mkdir -p test_results
	BITSWAP_STABILITY_DURATION=$(or $(BITSWAP_STABILITY_DURATION),30m) \
	go test -v -timeout=25h -run="TestBitswapComprehensiveLoadSuite/Test_24Hour_Stability" ./loadtest
```

## Заключение

Эта реализация обеспечивает:

- **Полное покрытие требований** Task 7.1
- **Масштабируемую архитектуру** для различных типов нагрузочных тестов
- **Детальный мониторинг** производительности и ресурсов
- **Автоматизированную валидацию** результатов
- **Комплексную отчетность** с анализом трендов
- **Простое использование** через Makefile автоматизацию

Система готова к использованию в продакшн среде и обеспечивает надежную валидацию производительности Bitswap под высокой нагрузкой.