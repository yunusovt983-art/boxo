# Task 7.2 Integration Testing Component Diagram - Подробное объяснение

## Обзор диаграммы

**Файл**: `task7_c4_component_integration_testing.puml`  
**Тип**: C4 Component Diagram  
**Назначение**: Детализирует внутреннюю структуру системы интеграционного тестирования кластера

## Архитектурный дизайн → Реализация кода

### 🏗️ Ядро кластерного тестирования

#### ClusterTestEngine
**Архитектурная роль**: Движок интеграционного тестирования кластера

**Реализация в коде**:
```go
// integration/cluster_test_engine.go
type ClusterTestEngine struct {
    orchestrator     *ClusterOrchestrator
    configManager    *ClusterConfigManager
    scenarioManager  *TestScenarioManager
    
    // Управление кластером
    clusterManager   *ClusterManager
    nodeManager      *NodeManager
    
    // Состояние тестирования
    activeTests      map[string]*ClusterTest
    testHistory      []*ClusterTestResult
    
    // Синхронизация
    mu               sync.RWMutex
    running          bool
}

func NewClusterTestEngine(config *ClusterTestConfig) *ClusterTestEngine {
    return &ClusterTestEngine{
        orchestrator:    NewClusterOrchestrator(),
        configManager:   NewClusterConfigManager(config),
        scenarioManager: NewTestScenarioManager(),
        clusterManager:  NewClusterManager(),
        nodeManager:     NewNodeManager(),
        activeTests:     make(map[string]*ClusterTest),
        testHistory:     make([]*ClusterTestResult, 0),
    }
}

func (cte *ClusterTestEngine) RunClusterTest(testType ClusterTestType, params *ClusterTestParams) (*ClusterTestResult, error) {
    cte.mu.Lock()
    if cte.running {
        cte.mu.Unlock()
        return nil, errors.New("cluster test already running")
    }
    cte.running = true
    cte.mu.Unlock()
    
    defer func() {
        cte.mu.Lock()
        cte.running = false
        cte.mu.Unlock()
    }()
    
    // Создание тестового кластера
    cluster, err := cte.clusterManager.CreateTestCluster(params.NodeCount)
    if err != nil {
        return nil, fmt.Errorf("failed to create test cluster: %w", err)
    }
    defer cluster.Cleanup()
    
    // Выполнение теста
    result := &ClusterTestResult{
        TestType:  testType,
        StartTime: time.Now(),
        ClusterID: cluster.ID(),
    }
    
    switch testType {
    case InterNodeCommunicationTest:
        err = cte.runInterNodeCommunicationTest(cluster, params, result)
    case ClusterMetricsTest:
        err = cte.runClusterMetricsTest(cluster, params, result)
    case ClusterFaultToleranceTest:
        err = cte.runClusterFaultToleranceTest(cluster, params, result)
    default:
        err = fmt.Errorf("unknown test type: %v", testType)
    }
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    result.Success = err == nil
    
    if err != nil {
        result.Error = err
    }
    
    cte.testHistory = append(cte.testHistory, result)
    return result, err
}
```

#### ClusterOrchestrator
**Архитектурная роль**: Оркестрация кластерных тестов

**Реализация в коде**:
```go
// integration/cluster_orchestrator.go
type ClusterOrchestrator struct {
    testExecutors    map[ClusterTestType]ClusterTestExecutor
    resourceManager  *ClusterResourceManager
    topologyManager  *NetworkTopologyManager
    
    // Координация тестов
    testQueue        chan *ClusterTestRequest
    resultCollector  *ClusterResultCollector
    
    // Мониторинг выполнения
    progressTracker  *ClusterProgressTracker
    healthMonitor    *ClusterHealthMonitor
}

func (co *ClusterOrchestrator) ExecuteClusterTest(cluster *TestCluster, testType ClusterTestType, params *ClusterTestParams) (*ClusterTestResult, error) {
    executor, exists := co.testExecutors[testType]
    if !exists {
        return nil, fmt.Errorf("no executor found for test type: %v", testType)
    }
    
    // Проверка готовности кластера
    if err := co.healthMonitor.CheckClusterHealth(cluster); err != nil {
        return nil, fmt.Errorf("cluster health check failed: %w", err)
    }
    
    // Запуск мониторинга прогресса
    co.progressTracker.StartTracking(cluster.ID(), testType)
    defer co.progressTracker.StopTracking(cluster.ID())
    
    // Выполнение теста
    result, err := executor.Execute(cluster, params)
    if err != nil {
        return nil, fmt.Errorf("test execution failed: %w", err)
    }
    
    // Сбор дополнительных метрик
    result.ClusterMetrics = co.collectClusterMetrics(cluster)
    result.TopologyInfo = co.topologyManager.GetTopologySnapshot(cluster)
    
    return result, nil
}
```

### 🔗 Кластерные тесты

#### InterNodeCommTest
**Архитектурная роль**: Тесты межузлового взаимодействия

**Реализация в коде**:
```go
// integration/inter_node_comm_test.go
type InterNodeCommTest struct {
    connectivityTester *ConnectivityTester
    latencyMeasurer    *LatencyMeasurer
    bandwidthTester    *BandwidthTester
    dataExchangeTester *DataExchangeTester
}

func (inct *InterNodeCommTest) Execute(cluster *TestCluster, params *ClusterTestParams) (*ClusterTestResult, error) {
    result := &ClusterTestResult{
        TestName:  "InterNodeCommunication",
        StartTime: time.Now(),
    }
    
    nodes := cluster.GetNodes()
    if len(nodes) < 2 {
        return nil, errors.New("at least 2 nodes required for inter-node communication test")
    }
    
    // Тестирование связности между всеми парами узлов
    connectivityResults := make(map[string]*ConnectivityResult)
    for i, nodeA := range nodes {
        for j, nodeB := range nodes[i+1:] {
            pairKey := fmt.Sprintf("%s-%s", nodeA.ID(), nodeB.ID())
            
            // Тест связности
            connResult, err := inct.testNodePairConnectivity(nodeA, nodeB)
            if err != nil {
                return nil, fmt.Errorf("connectivity test failed for %s: %w", pairKey, err)
            }
            connectivityResults[pairKey] = connResult
        }
    }
    
    result.ConnectivityResults = connectivityResults
    
    // Тестирование обмена данными
    dataExchangeResults, err := inct.testDataExchange(nodes)
    if err != nil {
        return nil, fmt.Errorf("data exchange test failed: %w", err)
    }
    result.DataExchangeResults = dataExchangeResults
    
    // Тестирование производительности сети
    networkPerfResults, err := inct.testNetworkPerformance(nodes)
    if err != nil {
        return nil, fmt.Errorf("network performance test failed: %w", err)
    }
    result.NetworkPerformanceResults = networkPerfResults
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    
    return result, nil
}

func (inct *InterNodeCommTest) testNodePairConnectivity(nodeA, nodeB *ClusterNode) (*ConnectivityResult, error) {
    result := &ConnectivityResult{
        NodeA: nodeA.ID(),
        NodeB: nodeB.ID(),
    }
    
    // Измерение латентности
    latency, err := inct.latencyMeasurer.MeasureLatency(nodeA, nodeB)
    if err != nil {
        return nil, fmt.Errorf("latency measurement failed: %w", err)
    }
    result.Latency = latency
    
    // Тест пропускной способности
    bandwidth, err := inct.bandwidthTester.MeasureBandwidth(nodeA, nodeB)
    if err != nil {
        return nil, fmt.Errorf("bandwidth measurement failed: %w", err)
    }
    result.Bandwidth = bandwidth
    
    // Проверка стабильности соединения
    stability, err := inct.connectivityTester.TestConnectionStability(nodeA, nodeB, 30*time.Second)
    if err != nil {
        return nil, fmt.Errorf("connection stability test failed: %w", err)
    }
    result.Stability = stability
    
    return result, nil
}
```

#### ClusterMetricsTest
**Архитектурная роль**: Тесты метрик кластера

**Реализация в коде**:
```go
// integration/cluster_metrics_test.go
type ClusterMetricsTest struct {
    metricsCollector *ClusterMetricsCollector
    validator        *MetricsValidator
    alertTester      *AlertTester
    consistencyChecker *ConsistencyChecker
}

func (cmt *ClusterMetricsTest) Execute(cluster *TestCluster, params *ClusterTestParams) (*ClusterTestResult, error) {
    result := &ClusterTestResult{
        TestName:  "ClusterMetrics",
        StartTime: time.Now(),
    }
    
    // Сбор базовых метрик кластера
    baselineMetrics, err := cmt.metricsCollector.CollectBaselineMetrics(cluster)
    if err != nil {
        return nil, fmt.Errorf("failed to collect baseline metrics: %w", err)
    }
    result.BaselineMetrics = baselineMetrics
    
    // Генерация нагрузки для получения метрик
    loadGenerator := NewClusterLoadGenerator()
    if err := loadGenerator.GenerateLoad(cluster, params.LoadDuration); err != nil {
        return nil, fmt.Errorf("failed to generate load: %w", err)
    }
    
    // Сбор метрик под нагрузкой
    loadMetrics, err := cmt.metricsCollector.CollectLoadMetrics(cluster)
    if err != nil {
        return nil, fmt.Errorf("failed to collect load metrics: %w", err)
    }
    result.LoadMetrics = loadMetrics
    
    // Валидация метрик
    validationResults := make(map[string]*ValidationResult)
    
    // Проверка консистентности метрик между узлами
    consistencyResult, err := cmt.consistencyChecker.CheckMetricsConsistency(cluster, loadMetrics)
    if err != nil {
        return nil, fmt.Errorf("metrics consistency check failed: %w", err)
    }
    validationResults["consistency"] = consistencyResult
    
    // Валидация пороговых значений
    thresholdResult, err := cmt.validator.ValidateThresholds(loadMetrics)
    if err != nil {
        return nil, fmt.Errorf("threshold validation failed: %w", err)
    }
    validationResults["thresholds"] = thresholdResult
    
    // Тестирование алертов
    alertResult, err := cmt.alertTester.TestAlerts(cluster, loadMetrics)
    if err != nil {
        return nil, fmt.Errorf("alert testing failed: %w", err)
    }
    validationResults["alerts"] = alertResult
    
    result.ValidationResults = validationResults
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    
    return result, nil
}
```

### 📊 Валидация метрик

#### MetricsValidator
**Архитектурная роль**: Валидатор метрик кластера

**Реализация в коде**:
```go
// integration/metrics_validator.go
type MetricsValidator struct {
    thresholds       map[string]*MetricThreshold
    validationRules  []*ValidationRule
    anomalyDetector  *AnomalyDetector
}

func (mv *MetricsValidator) ValidateClusterMetrics(metrics *ClusterMetrics) (*ValidationResult, error) {
    result := &ValidationResult{
        StartTime:   time.Now(),
        Validations: make(map[string]*ValidationCheck),
    }
    
    // Валидация каждой метрики по пороговым значениям
    for metricName, metricValue := range metrics.Metrics {
        threshold, exists := mv.thresholds[metricName]
        if !exists {
            continue
        }
        
        check := &ValidationCheck{
            MetricName: metricName,
            Value:      metricValue,
            Threshold:  threshold,
        }
        
        if mv.isWithinThreshold(metricValue, threshold) {
            check.Status = ValidationPassed
        } else {
            check.Status = ValidationFailed
            check.Message = fmt.Sprintf("Metric %s value %v exceeds threshold %v", 
                metricName, metricValue, threshold)
        }
        
        result.Validations[metricName] = check
    }
    
    // Применение пользовательских правил валидации
    for _, rule := range mv.validationRules {
        ruleResult, err := rule.Apply(metrics)
        if err != nil {
            return nil, fmt.Errorf("validation rule %s failed: %w", rule.Name, err)
        }
        result.Validations[rule.Name] = ruleResult
    }
    
    // Обнаружение аномалий
    anomalies, err := mv.anomalyDetector.DetectAnomalies(metrics)
    if err != nil {
        return nil, fmt.Errorf("anomaly detection failed: %w", err)
    }
    result.Anomalies = anomalies
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    
    // Определение общего статуса валидации
    result.OverallStatus = mv.calculateOverallStatus(result.Validations)
    
    return result, nil
}

func (mv *MetricsValidator) isWithinThreshold(value interface{}, threshold *MetricThreshold) bool {
    switch v := value.(type) {
    case float64:
        return v >= threshold.Min && v <= threshold.Max
    case int64:
        return float64(v) >= threshold.Min && float64(v) <= threshold.Max
    case time.Duration:
        return v >= threshold.MinDuration && v <= threshold.MaxDuration
    default:
        return false
    }
}
```

### 🔍 Мониторинг кластера

#### ClusterMetricsCollector
**Архитектурная роль**: Сборщик метрик кластера

**Реализация в коде**:
```go
// integration/cluster_metrics_collector.go
type ClusterMetricsCollector struct {
    nodeCollectors   map[string]*NodeMetricsCollector
    aggregator       *MetricsAggregator
    prometheusClient *prometheus.Client
    
    // Конфигурация сбора
    collectionInterval time.Duration
    metricsBuffer     *MetricsBuffer
}

func (cmc *ClusterMetricsCollector) CollectClusterMetrics(cluster *TestCluster) (*ClusterMetrics, error) {
    metrics := &ClusterMetrics{
        ClusterID:   cluster.ID(),
        Timestamp:   time.Now(),
        NodeMetrics: make(map[string]*NodeMetrics),
    }
    
    // Сбор метрик с каждого узла
    var wg sync.WaitGroup
    nodeMetricsChan := make(chan *NodeMetricsResult, len(cluster.GetNodes()))
    
    for _, node := range cluster.GetNodes() {
        wg.Add(1)
        go func(n *ClusterNode) {
            defer wg.Done()
            
            collector, exists := cmc.nodeCollectors[n.ID()]
            if !exists {
                collector = NewNodeMetricsCollector(n)
                cmc.nodeCollectors[n.ID()] = collector
            }
            
            nodeMetrics, err := collector.CollectMetrics()
            nodeMetricsChan <- &NodeMetricsResult{
                NodeID:  n.ID(),
                Metrics: nodeMetrics,
                Error:   err,
            }
        }(node)
    }
    
    wg.Wait()
    close(nodeMetricsChan)
    
    // Обработка результатов сбора метрик
    for result := range nodeMetricsChan {
        if result.Error != nil {
            log.Printf("Failed to collect metrics from node %s: %v", result.NodeID, result.Error)
            continue
        }
        metrics.NodeMetrics[result.NodeID] = result.Metrics
    }
    
    // Агрегация метрик кластера
    aggregatedMetrics, err := cmc.aggregator.AggregateNodeMetrics(metrics.NodeMetrics)
    if err != nil {
        return nil, fmt.Errorf("failed to aggregate metrics: %w", err)
    }
    metrics.AggregatedMetrics = aggregatedMetrics
    
    // Расчет метрик кластера
    metrics.ClusterMetrics = cmc.calculateClusterMetrics(metrics.NodeMetrics)
    
    return metrics, nil
}

func (cmc *ClusterMetricsCollector) calculateClusterMetrics(nodeMetrics map[string]*NodeMetrics) *ClusterLevelMetrics {
    clusterMetrics := &ClusterLevelMetrics{
        TotalNodes:    len(nodeMetrics),
        ActiveNodes:   0,
        TotalRequests: 0,
        TotalErrors:   0,
    }
    
    var totalLatency time.Duration
    var latencyCount int
    
    for _, nodeMetric := range nodeMetrics {
        if nodeMetric.Status == NodeStatusActive {
            clusterMetrics.ActiveNodes++
        }
        
        clusterMetrics.TotalRequests += nodeMetric.RequestCount
        clusterMetrics.TotalErrors += nodeMetric.ErrorCount
        
        if nodeMetric.AverageLatency > 0 {
            totalLatency += nodeMetric.AverageLatency
            latencyCount++
        }
    }
    
    if latencyCount > 0 {
        clusterMetrics.AverageLatency = totalLatency / time.Duration(latencyCount)
    }
    
    if clusterMetrics.TotalRequests > 0 {
        clusterMetrics.ErrorRate = float64(clusterMetrics.TotalErrors) / float64(clusterMetrics.TotalRequests)
    }
    
    clusterMetrics.ClusterHealth = cmc.calculateClusterHealth(clusterMetrics)
    
    return clusterMetrics
}
```

### 🤖 Автоматизация тестирования

#### CICDIntegration
**Архитектурная роль**: Интеграция с CI/CD pipeline

**Реализация в коде**:
```go
// integration/cicd_integration.go
type CICDIntegration struct {
    pipelineConfig   *PipelineConfig
    testScheduler    *TestScheduler
    resultPublisher  *ResultPublisher
    artifactManager  *ArtifactManager
}

func (ci *CICDIntegration) SetupIntegrationPipeline() error {
    // Создание GitHub Actions workflow
    workflow := &GitHubWorkflow{
        Name: "Cluster Integration Tests",
        On: WorkflowTriggers{
            Push: &PushTrigger{
                Branches: []string{"main", "develop"},
            },
            PullRequest: &PullRequestTrigger{
                Branches: []string{"main"},
            },
            Schedule: &ScheduleTrigger{
                Cron: "0 */6 * * *", // Каждые 6 часов
            },
        },
        Jobs: map[string]*Job{
            "cluster-integration-tests": {
                RunsOn: "ubuntu-latest",
                Strategy: &JobStrategy{
                    Matrix: map[string][]string{
                        "cluster-size": {"3", "5", "7"},
                        "test-type":    {"connectivity", "metrics", "fault-tolerance"},
                    },
                },
                Steps: []Step{
                    {Name: "Checkout", Uses: "actions/checkout@v3"},
                    {Name: "Setup Go", Uses: "actions/setup-go@v3"},
                    {Name: "Setup Test Cluster", Run: "make setup-test-cluster NODES=${{ matrix.cluster-size }}"},
                    {Name: "Run Integration Tests", Run: "make test-integration TYPE=${{ matrix.test-type }}"},
                    {Name: "Collect Results", Run: "make collect-test-results"},
                    {Name: "Upload Artifacts", Uses: "actions/upload-artifact@v3"},
                },
            },
        },
    }
    
    return ci.deployWorkflow(workflow)
}

func (ci *CICDIntegration) RunScheduledTests() error {
    // Получение списка запланированных тестов
    scheduledTests, err := ci.testScheduler.GetScheduledTests()
    if err != nil {
        return fmt.Errorf("failed to get scheduled tests: %w", err)
    }
    
    results := make([]*ClusterTestResult, 0)
    
    for _, test := range scheduledTests {
        result, err := ci.executeScheduledTest(test)
        if err != nil {
            log.Printf("Scheduled test %s failed: %v", test.Name, err)
            continue
        }
        results = append(results, result)
    }
    
    // Публикация результатов
    if err := ci.resultPublisher.PublishResults(results); err != nil {
        return fmt.Errorf("failed to publish results: %w", err)
    }
    
    // Сохранение артефактов
    if err := ci.artifactManager.SaveArtifacts(results); err != nil {
        return fmt.Errorf("failed to save artifacts: %w", err)
    }
    
    return nil
}
```

## Взаимодействия между компонентами

### ClusterTestEngine → Test Types
```go
func (cte *ClusterTestEngine) executeTestType(testType ClusterTestType, cluster *TestCluster, params *ClusterTestParams) (*ClusterTestResult, error) {
    switch testType {
    case InterNodeCommunicationTest:
        test := &InterNodeCommTest{
            connectivityTester: cte.connectivityTester,
            latencyMeasurer:    cte.latencyMeasurer,
        }
        return test.Execute(cluster, params)
        
    case ClusterMetricsTest:
        test := &ClusterMetricsTest{
            metricsCollector: cte.metricsCollector,
            validator:        cte.metricsValidator,
        }
        return test.Execute(cluster, params)
        
    case ClusterFaultToleranceTest:
        test := &ClusterFaultToleranceTest{
            faultInjector:    cte.faultInjector,
            recoveryTester:   cte.recoveryTester,
        }
        return test.Execute(cluster, params)
    }
}
```

Эта диаграмма служит детальным руководством для реализации всех компонентов системы интеграционного тестирования кластера Task 7.2.