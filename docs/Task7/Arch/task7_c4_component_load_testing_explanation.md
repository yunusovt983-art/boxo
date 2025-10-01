# Task 7.1 Load Testing Component Diagram - Подробное объяснение

## Обзор диаграммы

**Файл**: `task7_c4_component_load_testing.puml`  
**Тип**: C4 Component Diagram  
**Назначение**: Детализирует внутреннюю структуру системы нагрузочного тестирования

## Архитектурный дизайн → Реализация кода

### 🔧 Ядро системы тестирования

#### LoadTestEngine
**Архитектурная роль**: Главный движок нагрузочного тестирования

**Реализация в коде**:
```go
// bitswap/loadtest/load_test_engine.go
type LoadTestEngine struct {
    config           *LoadTestConfig
    orchestrator     *TestOrchestrator
    configManager    *TestConfigManager
    validator        *TestValidator
    
    // Состояние выполнения
    currentTest      TestInstance
    testHistory      []*TestResult
    
    // Синхронизация
    mu               sync.RWMutex
    running          bool
    
    // Каналы управления
    stopChan         chan struct{}
    resultChan       chan *TestResult
}

func NewLoadTestEngine(config *LoadTestConfig) *LoadTestEngine {
    return &LoadTestEngine{
        config:        config,
        orchestrator:  NewTestOrchestrator(),
        configManager: NewTestConfigManager(config),
        validator:     NewTestValidator(),
        testHistory:   make([]*TestResult, 0),
        stopChan:      make(chan struct{}),
        resultChan:    make(chan *TestResult, 100),
    }
}

func (lte *LoadTestEngine) ExecuteTest(testType TestType, params TestParameters) (*TestResult, error) {
    lte.mu.Lock()
    if lte.running {
        lte.mu.Unlock()
        return nil, errors.New("another test is already running")
    }
    lte.running = true
    lte.mu.Unlock()
    
    defer func() {
        lte.mu.Lock()
        lte.running = false
        lte.mu.Unlock()
    }()
    
    // Валидация параметров теста
    if err := lte.validator.ValidateTestParameters(testType, params); err != nil {
        return nil, fmt.Errorf("invalid test parameters: %w", err)
    }
    
    // Создание экземпляра теста
    testInstance, err := lte.orchestrator.CreateTestInstance(testType, params)
    if err != nil {
        return nil, fmt.Errorf("failed to create test instance: %w", err)
    }
    
    lte.currentTest = testInstance
    
    // Выполнение теста
    result, err := lte.orchestrator.ExecuteTest(testInstance)
    if err != nil {
        return nil, fmt.Errorf("test execution failed: %w", err)
    }
    
    // Валидация результатов
    if err := lte.validator.ValidateTestResult(result); err != nil {
        return nil, fmt.Errorf("test result validation failed: %w", err)
    }
    
    lte.testHistory = append(lte.testHistory, result)
    return result, nil
}
```

#### TestOrchestrator
**Архитектурная роль**: Оркестрация выполнения тестов

**Реализация в коде**:
```go
// bitswap/loadtest/test_orchestrator.go
type TestOrchestrator struct {
    testFactories    map[TestType]TestFactory
    resourceManager  *ResourceManager
    executionPlan    *ExecutionPlan
    
    // Управление ресурсами
    workerPool       *WorkerPool
    connectionPool   *ConnectionPool
    
    // Мониторинг выполнения
    progressTracker  *ProgressTracker
    metricsCollector *MetricsCollector
}

func (to *TestOrchestrator) ExecuteTest(testInstance TestInstance) (*TestResult, error) {
    // Подготовка ресурсов
    resources, err := to.resourceManager.AllocateResources(testInstance.GetResourceRequirements())
    if err != nil {
        return nil, fmt.Errorf("failed to allocate resources: %w", err)
    }
    defer to.resourceManager.ReleaseResources(resources)
    
    // Создание плана выполнения
    plan, err := to.createExecutionPlan(testInstance)
    if err != nil {
        return nil, fmt.Errorf("failed to create execution plan: %w", err)
    }
    
    // Запуск мониторинга прогресса
    to.progressTracker.StartTracking(testInstance.GetID())
    defer to.progressTracker.StopTracking(testInstance.GetID())
    
    // Выполнение теста по плану
    result := &TestResult{
        TestID:    testInstance.GetID(),
        StartTime: time.Now(),
    }
    
    for _, phase := range plan.Phases {
        phaseResult, err := to.executePhase(phase, resources)
        if err != nil {
            result.Error = err
            break
        }
        result.PhaseResults = append(result.PhaseResults, phaseResult)
    }
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    
    return result, nil
}
```

### 🧪 Типы тестов

#### ConcurrentConnectionTest
**Архитектурная роль**: Тесты 10,000+ одновременных соединений

**Реализация в коде**:
```go
// bitswap/loadtest/concurrent_connection_test.go
type ConcurrentConnectionTest struct {
    maxConnections    int
    connectionManager *ConnectionManager
    latencyTracker    *LatencyTracker
    throughputMeter   *ThroughputMeter
    
    // Конфигурация теста
    testConfig        *ConcurrentTestConfig
    
    // Состояние теста
    activeConnections map[string]*Connection
    connectionsMu     sync.RWMutex
}

func (cct *ConcurrentConnectionTest) Execute(params *ConcurrentTestParams) (*TestResult, error) {
    targetConnections := params.TargetConnections
    if targetConnections > cct.maxConnections {
        return nil, fmt.Errorf("target connections %d exceeds maximum %d", 
            targetConnections, cct.maxConnections)
    }
    
    result := &TestResult{
        TestName:  fmt.Sprintf("ConcurrentConnections_%d", targetConnections),
        StartTime: time.Now(),
        Metadata: map[string]interface{}{
            "target_connections": targetConnections,
            "test_duration":      params.Duration,
        },
    }
    
    // Создание пула соединений
    connectionPool := make(chan *Connection, targetConnections)
    errorChan := make(chan error, targetConnections)
    
    // Запуск горутин для установления соединений
    var wg sync.WaitGroup
    for i := 0; i < targetConnections; i++ {
        wg.Add(1)
        go func(connID int) {
            defer wg.Done()
            
            conn, err := cct.establishConnection(connID)
            if err != nil {
                errorChan <- fmt.Errorf("connection %d failed: %w", connID, err)
                return
            }
            
            connectionPool <- conn
            
            // Выполнение тестовых операций на соединении
            cct.performConnectionOperations(conn, params.Duration)
        }(i)
    }
    
    // Ожидание завершения всех соединений
    wg.Wait()
    close(connectionPool)
    close(errorChan)
    
    // Сбор результатов
    successfulConnections := len(connectionPool)
    errors := make([]error, 0)
    for err := range errorChan {
        errors = append(errors, err)
    }
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    result.SuccessfulOperations = int64(successfulConnections)
    result.FailedOperations = int64(len(errors))
    result.SuccessRate = float64(successfulConnections) / float64(targetConnections)
    
    // Сбор метрик латентности
    result.LatencyMetrics = cct.latencyTracker.GetMetrics()
    result.ThroughputMetrics = cct.throughputMeter.GetMetrics()
    
    return result, nil
}

func (cct *ConcurrentConnectionTest) establishConnection(connID int) (*Connection, error) {
    startTime := time.Now()
    
    conn, err := cct.connectionManager.CreateConnection(fmt.Sprintf("conn-%d", connID))
    if err != nil {
        return nil, err
    }
    
    establishmentTime := time.Since(startTime)
    cct.latencyTracker.RecordConnectionEstablishment(connID, establishmentTime)
    
    cct.connectionsMu.Lock()
    cct.activeConnections[conn.ID()] = conn
    cct.connectionsMu.Unlock()
    
    return conn, nil
}
```

#### ThroughputTest
**Архитектурная роль**: Тесты 100,000+ запросов в секунду

**Реализация в коде**:
```go
// bitswap/loadtest/throughput_test.go
type ThroughputTest struct {
    rateLimiter      *rate.Limiter
    requestGenerator *RequestGenerator
    responseTracker  *ResponseTracker
    loadBalancer     *LoadBalancer
    
    // Метрики
    requestCounter   *atomic.Int64
    responseCounter  *atomic.Int64
    errorCounter     *atomic.Int64
}

func (tt *ThroughputTest) Execute(params *ThroughputTestParams) (*TestResult, error) {
    targetRPS := params.TargetRPS
    testDuration := params.Duration
    
    // Настройка rate limiter
    tt.rateLimiter = rate.NewLimiter(rate.Limit(targetRPS), targetRPS/10)
    
    result := &TestResult{
        TestName:  fmt.Sprintf("Throughput_%d_RPS", targetRPS),
        StartTime: time.Now(),
        Metadata: map[string]interface{}{
            "target_rps":     targetRPS,
            "test_duration":  testDuration,
        },
    }
    
    // Сброс счетчиков
    tt.requestCounter.Store(0)
    tt.responseCounter.Store(0)
    tt.errorCounter.Store(0)
    
    // Контекст для управления временем выполнения
    ctx, cancel := context.WithTimeout(context.Background(), testDuration)
    defer cancel()
    
    // Запуск воркеров для генерации нагрузки
    numWorkers := runtime.NumCPU() * 4
    var wg sync.WaitGroup
    
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            tt.runThroughputWorker(ctx, workerID, targetRPS/numWorkers)
        }(i)
    }
    
    // Мониторинг прогресса
    go tt.monitorThroughputProgress(ctx, result)
    
    // Ожидание завершения всех воркеров
    wg.Wait()
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    
    // Расчет финальных метрик
    totalRequests := tt.requestCounter.Load()
    totalResponses := tt.responseCounter.Load()
    totalErrors := tt.errorCounter.Load()
    
    result.TotalRequests = totalRequests
    result.SuccessfulOperations = totalResponses
    result.FailedOperations = totalErrors
    result.ActualRPS = float64(totalRequests) / result.Duration.Seconds()
    result.AchievementRate = result.ActualRPS / float64(targetRPS)
    
    return result, nil
}

func (tt *ThroughputTest) runThroughputWorker(ctx context.Context, workerID int, workerRPS int) {
    workerRateLimiter := rate.NewLimiter(rate.Limit(workerRPS), workerRPS/10)
    
    for {
        select {
        case <-ctx.Done():
            return
        default:
            // Ожидание разрешения от rate limiter
            if err := workerRateLimiter.Wait(ctx); err != nil {
                return
            }
            
            // Генерация и отправка запроса
            request := tt.requestGenerator.GenerateRequest(workerID)
            tt.requestCounter.Add(1)
            
            // Асинхронная отправка запроса
            go func(req *Request) {
                response, err := tt.sendRequest(req)
                if err != nil {
                    tt.errorCounter.Add(1)
                    return
                }
                
                tt.responseCounter.Add(1)
                tt.responseTracker.RecordResponse(response)
            }(request)
        }
    }
}
```

### 📊 Бенчмарки производительности

#### ScalingBenchmark
**Архитектурная роль**: Бенчмарки масштабируемости

**Реализация в коде**:
```go
// bitswap/loadtest/scaling_benchmark.go
type ScalingBenchmark struct {
    scaleFactors     []int
    baselineMetrics  *BaselineMetrics
    scalingAnalyzer  *ScalingAnalyzer
}

func (sb *ScalingBenchmark) RunBenchmark() (*BenchmarkResult, error) {
    result := &BenchmarkResult{
        BenchmarkName: "ScalingBenchmark",
        StartTime:     time.Now(),
        ScaleResults:  make(map[int]*ScaleResult),
    }
    
    // Установление базовой линии
    baseline, err := sb.establishBaseline()
    if err != nil {
        return nil, fmt.Errorf("failed to establish baseline: %w", err)
    }
    result.Baseline = baseline
    
    // Тестирование различных масштабов
    for _, scale := range sb.scaleFactors {
        scaleResult, err := sb.testScale(scale)
        if err != nil {
            log.Printf("Scale %d test failed: %v", scale, err)
            continue
        }
        result.ScaleResults[scale] = scaleResult
    }
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    
    // Анализ масштабируемости
    result.ScalingAnalysis = sb.scalingAnalyzer.AnalyzeScaling(result.ScaleResults)
    
    return result, nil
}

func (sb *ScalingBenchmark) testScale(nodeCount int) (*ScaleResult, error) {
    scaleResult := &ScaleResult{
        NodeCount: nodeCount,
        StartTime: time.Now(),
    }
    
    // Настройка тестового окружения для данного масштаба
    testEnv, err := sb.setupScaleEnvironment(nodeCount)
    if err != nil {
        return nil, fmt.Errorf("failed to setup scale environment: %w", err)
    }
    defer testEnv.Cleanup()
    
    // Выполнение тестов масштабируемости
    throughputResult, err := sb.measureThroughputAtScale(testEnv)
    if err != nil {
        return nil, err
    }
    scaleResult.ThroughputMetrics = throughputResult
    
    latencyResult, err := sb.measureLatencyAtScale(testEnv)
    if err != nil {
        return nil, err
    }
    scaleResult.LatencyMetrics = latencyResult
    
    resourceResult, err := sb.measureResourceUsageAtScale(testEnv)
    if err != nil {
        return nil, err
    }
    scaleResult.ResourceMetrics = resourceResult
    
    scaleResult.EndTime = time.Now()
    scaleResult.Duration = scaleResult.EndTime.Sub(scaleResult.StartTime)
    
    // Расчет эффективности масштабирования
    scaleResult.ScalingEfficiency = sb.calculateScalingEfficiency(scaleResult)
    
    return scaleResult, nil
}
```

### 📈 Мониторинг и метрики

#### PerformanceMonitor
**Архитектурная роль**: Мониторинг производительности в реальном времени

**Реализация в коде**:
```go
// monitoring/performance_monitor.go
type PerformanceMonitor struct {
    collectors       map[string]MetricsCollector
    registry         *prometheus.Registry
    metricsBuffer    *CircularBuffer
    alertThresholds  map[string]AlertThreshold
    
    // Состояние мониторинга
    monitoring       bool
    monitoringMu     sync.RWMutex
    
    // Каналы для метрик
    metricsChan      chan *MetricsSnapshot
    alertsChan       chan *Alert
}

func (pm *PerformanceMonitor) StartMonitoring(interval time.Duration) error {
    pm.monitoringMu.Lock()
    if pm.monitoring {
        pm.monitoringMu.Unlock()
        return errors.New("monitoring already started")
    }
    pm.monitoring = true
    pm.monitoringMu.Unlock()
    
    // Запуск сборщика метрик
    go pm.metricsCollectionLoop(interval)
    
    // Запуск анализатора алертов
    go pm.alertAnalysisLoop()
    
    return nil
}

func (pm *PerformanceMonitor) metricsCollectionLoop(interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            snapshot := pm.collectMetricsSnapshot()
            pm.metricsChan <- snapshot
            pm.metricsBuffer.Add(snapshot)
            
            // Проверка пороговых значений
            pm.checkAlertThresholds(snapshot)
            
        case <-pm.ctx.Done():
            return
        }
    }
}

func (pm *PerformanceMonitor) collectMetricsSnapshot() *MetricsSnapshot {
    snapshot := &MetricsSnapshot{
        Timestamp: time.Now(),
        Metrics:   make(map[string]interface{}),
    }
    
    // Сбор метрик от всех коллекторов
    for name, collector := range pm.collectors {
        metrics, err := collector.Collect()
        if err != nil {
            log.Printf("Failed to collect %s metrics: %v", name, err)
            continue
        }
        snapshot.Metrics[name] = metrics
    }
    
    return snapshot
}
```

### 🔄 Автоматизация

#### MakefileRunner
**Архитектурная роль**: Автоматизация через Makefile

**Реализация в коде**:
```go
// automation/makefile_runner.go
type MakefileRunner struct {
    makefilePath     string
    environment      map[string]string
    commandExecutor  *CommandExecutor
    outputCapture    *OutputCapture
}

func (mr *MakefileRunner) RunTarget(target string, args map[string]string) (*ExecutionResult, error) {
    // Подготовка команды
    cmd := exec.Command("make", "-f", mr.makefilePath, target)
    
    // Установка переменных окружения
    env := os.Environ()
    for key, value := range mr.environment {
        env = append(env, fmt.Sprintf("%s=%s", key, value))
    }
    for key, value := range args {
        env = append(env, fmt.Sprintf("%s=%s", key, value))
    }
    cmd.Env = env
    
    // Захват вывода
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
    }
    
    stderr, err := cmd.StderrPipe()
    if err != nil {
        return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
    }
    
    // Запуск команды
    startTime := time.Now()
    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("failed to start command: %w", err)
    }
    
    // Захват вывода
    stdoutOutput, _ := mr.outputCapture.CaptureOutput(stdout)
    stderrOutput, _ := mr.outputCapture.CaptureOutput(stderr)
    
    // Ожидание завершения
    err = cmd.Wait()
    endTime := time.Now()
    
    result := &ExecutionResult{
        Target:    target,
        StartTime: startTime,
        EndTime:   endTime,
        Duration:  endTime.Sub(startTime),
        ExitCode:  cmd.ProcessState.ExitCode(),
        Stdout:    stdoutOutput,
        Stderr:    stderrOutput,
        Success:   err == nil,
    }
    
    if err != nil {
        result.Error = err
    }
    
    return result, nil
}
```

## Взаимодействия между компонентами

### LoadTestEngine → Test Types
```go
func (lte *LoadTestEngine) executeTestType(testType TestType, params TestParameters) (*TestResult, error) {
    switch testType {
    case ConcurrentConnectionTestType:
        test := &ConcurrentConnectionTest{
            maxConnections: lte.config.MaxConnections,
            connectionManager: lte.connectionManager,
        }
        return test.Execute(params.(*ConcurrentTestParams))
        
    case ThroughputTestType:
        test := &ThroughputTest{
            rateLimiter: rate.NewLimiter(rate.Limit(params.(*ThroughputTestParams).TargetRPS), 100),
        }
        return test.Execute(params.(*ThroughputTestParams))
        
    case StabilityTestType:
        test := &StabilityTest{
            memoryMonitor: lte.memoryMonitor,
            leakDetector: lte.leakDetector,
        }
        return test.Execute(params.(*StabilityTestParams))
    }
}
```

Эта диаграмма служит детальным руководством для реализации всех компонентов системы нагрузочного тестирования Task 7.1.