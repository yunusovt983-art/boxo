# Task 7 Container Diagram - Подробное объяснение

## Обзор диаграммы

**Файл**: `task7_c4_container.puml`  
**Тип**: C4 Container Diagram  
**Назначение**: Детализирует внутреннюю структуру Task 7 на уровне контейнеров

## Архитектурный дизайн → Реализация кода

### 🏗️ Container Boundary: 7.1 Нагрузочное тестирование Bitswap

#### Load Test Engine
**Архитектурная роль**: Ядро системы нагрузочного тестирования

**Реализация в коде**:
```go
// bitswap/loadtest/load_test_engine.go
type LoadTestEngine struct {
    config          *LoadTestConfig
    testSuites      map[string]TestSuite
    resourceMonitor *ResourceMonitor
    metricsCollector *MetricsCollector
    resultAggregator *ResultAggregator
    
    // Управление жизненным циклом
    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
    
    // Состояние
    mu      sync.RWMutex
    running bool
    results []*TestResult
}

func NewLoadTestEngine(config *LoadTestConfig) *LoadTestEngine {
    ctx, cancel := context.WithCancel(context.Background())
    
    return &LoadTestEngine{
        config: config,
        testSuites: make(map[string]TestSuite),
        resourceMonitor: NewResourceMonitor(),
        metricsCollector: NewMetricsCollector(),
        resultAggregator: NewResultAggregator(),
        ctx: ctx,
        cancel: cancel,
        results: make([]*TestResult, 0),
    }
}
```

#### Concurrent Connection Tests
**Архитектурная роль**: Тесты 10,000+ одновременных соединений

**Реализация в коде**:
```go
// bitswap/loadtest/concurrent_connection_tests.go
type ConcurrentConnectionTests struct {
    maxConnections   int
    connectionPool   *ConnectionPool
    latencyCollector *LatencyCollector
    errorTracker     *ErrorTracker
}

func (cct *ConcurrentConnectionTests) RunTest(targetConnections int) (*TestResult, error) {
    result := &TestResult{
        TestName:  fmt.Sprintf("ConcurrentConnections_%d", targetConnections),
        StartTime: time.Now(),
    }
    
    // Создание пула соединений
    pool, err := cct.connectionPool.CreatePool(targetConnections)
    if err != nil {
        return nil, fmt.Errorf("failed to create connection pool: %w", err)
    }
    defer pool.Close()
    
    // Параллельное установление соединений
    var wg sync.WaitGroup
    connectionResults := make(chan *ConnectionResult, targetConnections)
    
    for i := 0; i < targetConnections; i++ {
        wg.Add(1)
        go func(connID int) {
            defer wg.Done()
            cct.establishAndTestConnection(connID, connectionResults)
        }(i)
    }
    
    wg.Wait()
    close(connectionResults)
    
    // Агрегация результатов
    return cct.aggregateConnectionResults(connectionResults, result), nil
}
```

#### Throughput Tests
**Архитектурная роль**: Тесты 100,000+ запросов в секунду

**Реализация в коде**:
```go
// bitswap/loadtest/throughput_tests.go
type ThroughputTests struct {
    rateLimiter     *rate.Limiter
    requestGenerator *RequestGenerator
    responseTracker  *ResponseTracker
    throughputMeter  *ThroughputMeter
}

func (tt *ThroughputTests) RunTest(targetRPS int) (*TestResult, error) {
    // Настройка rate limiter
    tt.rateLimiter = rate.NewLimiter(rate.Limit(targetRPS), targetRPS/10)
    
    result := &TestResult{
        TestName:  fmt.Sprintf("Throughput_%d_RPS", targetRPS),
        StartTime: time.Now(),
    }
    
    // Запуск генераторов нагрузки
    numWorkers := runtime.NumCPU() * 4
    var wg sync.WaitGroup
    
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            tt.runThroughputWorker(workerID, targetRPS/numWorkers)
        }(i)
    }
    
    // Мониторинг в течение тестового периода
    testDuration := 60 * time.Second
    time.Sleep(testDuration)
    
    wg.Wait()
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    
    return tt.calculateThroughputMetrics(result), nil
}
```####
 Stability Tests
**Архитектурная роль**: Тесты стабильности 24+ часа

**Реализация в коде**:
```go
// bitswap/loadtest/stability_tests.go
type StabilityTests struct {
    memoryMonitor    *MemoryMonitor
    leakDetector     *MemoryLeakDetector
    stabilityTracker *StabilityTracker
    longRunningLoad  *LongRunningLoadGenerator
}

func (st *StabilityTests) RunTest(duration time.Duration) (*TestResult, error) {
    result := &TestResult{
        TestName:  fmt.Sprintf("Stability_%s", duration.String()),
        StartTime: time.Now(),
    }
    
    // Запуск мониторинга памяти
    st.memoryMonitor.Start(30 * time.Second) // Каждые 30 секунд
    defer st.memoryMonitor.Stop()
    
    // Запуск детектора утечек памяти
    st.leakDetector.Start()
    defer st.leakDetector.Stop()
    
    // Постоянная нагрузка
    constantRPS := 1000
    ctx, cancel := context.WithTimeout(context.Background(), duration)
    defer cancel()
    
    err := st.longRunningLoad.GenerateLoad(ctx, constantRPS)
    if err != nil {
        return nil, fmt.Errorf("stability test failed: %w", err)
    }
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    
    // Анализ стабильности
    return st.analyzeStabilityResults(result), nil
}
```

#### Benchmark Suite
**Архитектурная роль**: Автоматизированные бенчмарки производительности

**Реализация в коде**:
```go
// bitswap/loadtest/benchmark_suite.go
type BenchmarkSuite struct {
    benchmarks      map[string]Benchmark
    scaleFactors    []int
    resultCollector *BenchmarkResultCollector
}

func (bs *BenchmarkSuite) RunAllBenchmarks() (*BenchmarkResults, error) {
    results := &BenchmarkResults{
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
        result, err := bs.runScaleBenchmark(scale)
        if err != nil {
            log.Printf("Benchmark %s failed: %v", scale.Name, err)
            continue
        }
        results.Benchmarks[scale.Name] = result
    }
    
    results.EndTime = time.Now()
    results.Duration = results.EndTime.Sub(results.StartTime)
    
    return results, nil
}
```

### 🏗️ Container Boundary: 7.2 Интеграционное тестирование кластера

#### Cluster Integration Tests
**Архитектурная роль**: Тесты взаимодействия узлов кластера

**Реализация в коде**:
```go
// integration/cluster_integration_tests.go
type ClusterIntegrationTests struct {
    clusterManager   *ClusterManager
    nodeManager      *NodeManager
    communicationTester *InterNodeCommunicationTester
    dataConsistencyTester *DataConsistencyTester
}

func (cit *ClusterIntegrationTests) RunInterNodeTests() (*IntegrationResult, error) {
    // Настройка тестового кластера
    cluster, err := cit.clusterManager.SetupTestCluster(5)
    if err != nil {
        return nil, fmt.Errorf("failed to setup cluster: %w", err)
    }
    defer cluster.Cleanup()
    
    result := &IntegrationResult{
        TestName:  "InterNodeCommunication",
        StartTime: time.Now(),
    }
    
    // Тестирование связности
    connectivityResult, err := cit.communicationTester.TestConnectivity(cluster)
    if err != nil {
        return nil, err
    }
    result.ConnectivityResult = connectivityResult
    
    // Тестирование обмена данными
    dataExchangeResult, err := cit.communicationTester.TestDataExchange(cluster)
    if err != nil {
        return nil, err
    }
    result.DataExchangeResult = dataExchangeResult
    
    result.EndTime = time.Now()
    return result, nil
}
```

#### Metrics Validation
**Архитектурная роль**: Валидация метрик и алертов

**Реализация в коде**:
```go
// integration/metrics_validation.go
type MetricsValidation struct {
    prometheusClient *prometheus.Client
    metricsValidator *MetricsValidator
    alertValidator   *AlertValidator
    thresholdChecker *ThresholdChecker
}

func (mv *MetricsValidation) ValidateClusterMetrics() (*ValidationResult, error) {
    result := &ValidationResult{
        StartTime: time.Now(),
        Validations: make(map[string]*ValidationCheck),
    }
    
    // Сбор метрик кластера
    metrics, err := mv.prometheusClient.QueryClusterMetrics()
    if err != nil {
        return nil, fmt.Errorf("failed to query metrics: %w", err)
    }
    
    // Валидация консистентности метрик
    consistencyCheck := mv.metricsValidator.CheckConsistency(metrics)
    result.Validations["consistency"] = consistencyCheck
    
    // Проверка пороговых значений
    thresholdCheck := mv.thresholdChecker.CheckThresholds(metrics)
    result.Validations["thresholds"] = thresholdCheck
    
    // Валидация алертов
    alertCheck := mv.alertValidator.ValidateAlerts(metrics)
    result.Validations["alerts"] = alertCheck
    
    result.EndTime = time.Now()
    return result, nil
}
```

### 🏗️ Container Boundary: 7.3 Тестирование отказоустойчивости

#### Network Failure Simulator
**Архитектурная роль**: Симуляция сбоев сети

**Реализация в коде**:
```go
// fault_tolerance/network_failure_simulator.go
type NetworkFailureSimulator struct {
    networkController *NetworkController
    partitionManager  *PartitionManager
    latencyInjector   *LatencyInjector
    packetLossInjector *PacketLossInjector
}

func (nfs *NetworkFailureSimulator) SimulateNetworkPartition(duration time.Duration) error {
    // Создание сетевого разделения
    partition := &NetworkPartition{
        GroupA: []string{"node-1", "node-2"},
        GroupB: []string{"node-3", "node-4", "node-5"},
        Duration: duration,
    }
    
    if err := nfs.partitionManager.CreatePartition(partition); err != nil {
        return fmt.Errorf("failed to create partition: %w", err)
    }
    
    // Мониторинг во время разделения
    go nfs.monitorPartitionEffects(partition)
    
    // Ожидание завершения
    time.Sleep(duration)
    
    // Восстановление связности
    return nfs.partitionManager.RestoreConnectivity(partition)
}
```

#### Chaos Engineering
**Архитектурная роль**: Chaos engineering тесты

**Реализация в коде**:
```go
// fault_tolerance/chaos_engineering.go
type ChaosEngineering struct {
    chaosEngine      *ChaosEngine
    experimentRunner *ExperimentRunner
    safetyChecker    *SafetyChecker
    blastRadiusController *BlastRadiusController
}

func (ce *ChaosEngineering) RunChaosExperiment(experiment *ChaosExperiment) (*ExperimentResult, error) {
    // Проверка безопасности эксперимента
    if err := ce.safetyChecker.ValidateExperiment(experiment); err != nil {
        return nil, fmt.Errorf("experiment safety check failed: %w", err)
    }
    
    // Контроль радиуса воздействия
    if err := ce.blastRadiusController.SetBlastRadius(experiment.BlastRadius); err != nil {
        return nil, fmt.Errorf("failed to set blast radius: %w", err)
    }
    
    result := &ExperimentResult{
        ExperimentName: experiment.Name,
        StartTime:      time.Now(),
    }
    
    // Выполнение эксперимента
    if err := ce.experimentRunner.RunExperiment(experiment); err != nil {
        result.Error = err
        return result, err
    }
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    
    return result, nil
}
```

### 🔧 Общие компоненты

#### Performance Monitor
**Архитектурная роль**: Мониторинг производительности в реальном времени

**Реализация в коде**:
```go
// monitoring/performance_monitor.go
type PerformanceMonitor struct {
    collectors       map[string]MetricsCollector
    prometheusRegistry *prometheus.Registry
    metricsBuffer    *MetricsBuffer
    alertManager     *AlertManager
}

func (pm *PerformanceMonitor) StartMonitoring(interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            pm.collectAllMetrics()
        case <-pm.ctx.Done():
            return
        }
    }
}

func (pm *PerformanceMonitor) collectAllMetrics() {
    for name, collector := range pm.collectors {
        metrics, err := collector.Collect()
        if err != nil {
            log.Printf("Failed to collect %s metrics: %v", name, err)
            continue
        }
        
        pm.metricsBuffer.Store(name, metrics)
        pm.checkAlertConditions(name, metrics)
    }
}
```

## Взаимодействия между контейнерами

### Load Test Engine ↔ Shared Components
```go
// Интеграция с мониторингом производительности
func (lte *LoadTestEngine) integrateWithMonitoring() {
    lte.performanceMonitor.RegisterCollector("load_test", &LoadTestMetricsCollector{
        engine: lte,
    })
    
    lte.resourceMonitor.RegisterCallback("memory_pressure", func(usage float64) {
        if usage > 0.8 {
            lte.throttleTestExecution()
        }
    })
}
```

### Integration Tests ↔ External Systems
```go
// Интеграция с IPFS Cluster
func (cit *ClusterIntegrationTests) integrateWithIPFSCluster() {
    cit.clusterManager.SetupClusterHooks(&ClusterHooks{
        OnNodeJoin: func(nodeID string) {
            cit.metricsCollector.RecordNodeJoin(nodeID)
        },
        OnNodeLeave: func(nodeID string) {
            cit.metricsCollector.RecordNodeLeave(nodeID)
        },
    })
}
```

Эта диаграмма служит основой для понимания архитектуры на уровне контейнеров и планирования детальной реализации компонентов Task 7.