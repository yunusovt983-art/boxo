# Task 7 Context Diagram - Подробное объяснение

## Обзор диаграммы

**Файл**: `task7_c4_context.puml`  
**Тип**: C4 Context Diagram  
**Назначение**: Показывает Task 7 в контексте всей системы и взаимодействие с внешними акторами

## Архитектурный дизайн → Реализация кода

### 🎭 Акторы системы

#### 1. Разработчик (Developer)
**Архитектурная роль**: Основной пользователь системы нагрузочного тестирования

**Реализация в коде**:
```go
// bitswap/loadtest/cli_interface.go
type DeveloperInterface struct {
    loadTestEngine *LoadTestEngine
    makefileRunner *MakefileRunner
    testRunner     *TestRunner
}

func (di *DeveloperInterface) RunLoadTests(testType string) error {
    switch testType {
    case "concurrent":
        return di.loadTestEngine.RunConcurrentConnectionTest(10000)
    case "throughput":
        return di.loadTestEngine.RunThroughputTest(100000)
    case "stability":
        return di.loadTestEngine.RunStabilityTest(24 * time.Hour)
    }
}
```

**Взаимодействие через**:
- CLI команды: `go test -run TestBitswapComprehensiveLoadSuite`
- Makefile: `make test-concurrent`, `make test-throughput`
- Прямые вызовы API тестирования

#### 2. DevOps инженер (DevOps Engineer)
**Архитектурная роль**: Автоматизация и мониторинг производительности

**Реализация в коде**:
```go
// monitoring/devops_interface.go
type DevOpsInterface struct {
    ciIntegration     *CICDIntegration
    metricsCollector  *MetricsCollector
    reportGenerator   *ReportGenerator
    alertManager      *AlertManager
}

func (doi *DevOpsInterface) SetupAutomation(config *AutomationConfig) error {
    // Настройка GitHub Actions workflow
    workflow := &GitHubWorkflow{
        Name: "Load Testing Automation",
        Triggers: []string{"push", "pull_request", "schedule"},
        Jobs: []Job{
            {Name: "concurrent-tests", Command: "make test-concurrent"},
            {Name: "throughput-tests", Command: "make test-throughput"},
        },
    }
    return doi.ciIntegration.DeployWorkflow(workflow)
}
```

#### 3. SRE (Site Reliability Engineer)
**Архитектурная роль**: Обеспечение надежности и анализ отказоустойчивости

**Реализация в коде**:
```go
// fault_tolerance/sre_interface.go
type SREInterface struct {
    chaosEngine       *ChaosEngine
    incidentManager   *IncidentManager
    slaMonitor        *SLAMonitor
    postmortemGen     *PostmortemGenerator
}

func (si *SREInterface) AnalyzeSystemResilience() (*ResilienceReport, error) {
    // Запуск chaos engineering экспериментов
    experiments := []ChaosExperiment{
        {Type: "network-partition", Duration: 5 * time.Minute},
        {Type: "node-failure", Duration: 2 * time.Minute},
        {Type: "memory-pressure", Duration: 10 * time.Minute},
    }
    
    results := make([]*ExperimentResult, 0)
    for _, exp := range experiments {
        result, err := si.chaosEngine.RunExperiment(exp)
        if err != nil {
            return nil, err
        }
        results = append(results, result)
    }
    
    return si.generateResilienceReport(results), nil
}
```

### 🏗️ Системы Task 7

#### 1. Система нагрузочного тестирования (Load Testing System)
**Архитектурная роль**: Комплексное тестирование Bitswap под высокой нагрузкой

**Реализация в коде**:
```go
// bitswap/loadtest/load_testing_system.go
type LoadTestingSystem struct {
    engine          *LoadTestEngine
    testSuites      map[string]TestSuite
    resourceMonitor *ResourceMonitor
    resultCollector *ResultCollector
}

func NewLoadTestingSystem(config *LoadTestConfig) *LoadTestingSystem {
    return &LoadTestingSystem{
        engine: NewLoadTestEngine(config),
        testSuites: map[string]TestSuite{
            "concurrent":  NewConcurrentConnectionTestSuite(),
            "throughput":  NewThroughputTestSuite(),
            "stability":   NewStabilityTestSuite(),
            "benchmarks":  NewBenchmarkTestSuite(),
        },
        resourceMonitor: NewResourceMonitor(),
        resultCollector: NewResultCollector(),
    }
}

func (lts *LoadTestingSystem) ExecuteComprehensiveTests() (*ComprehensiveResults, error) {
    results := &ComprehensiveResults{
        StartTime: time.Now(),
        TestResults: make(map[string]*TestResult),
    }
    
    // Выполнение всех типов тестов
    for name, suite := range lts.testSuites {
        result, err := lts.engine.RunTestSuite(suite)
        if err != nil {
            return nil, fmt.Errorf("failed to run %s tests: %w", name, err)
        }
        results.TestResults[name] = result
    }
    
    results.EndTime = time.Now()
    results.Duration = results.EndTime.Sub(results.StartTime)
    
    return results, nil
}
```

#### 2. Система интеграционного тестирования (Integration Testing System)
**Архитектурная роль**: Тестирование взаимодействия компонентов в кластерной среде

**Реализация в коде**:
```go
// integration/cluster_integration_system.go
type IntegrationTestingSystem struct {
    clusterManager    *ClusterManager
    testOrchestrator  *TestOrchestrator
    metricsValidator  *MetricsValidator
    ciIntegration     *CICDIntegration
}

func (its *IntegrationTestingSystem) RunClusterIntegrationTests() error {
    // Настройка кластерного окружения
    cluster, err := its.clusterManager.SetupTestCluster(5) // 5 узлов
    if err != nil {
        return fmt.Errorf("failed to setup cluster: %w", err)
    }
    defer cluster.Cleanup()
    
    // Выполнение интеграционных тестов
    testSuites := []IntegrationTestSuite{
        NewInterNodeCommunicationTest(),
        NewClusterMetricsTest(),
        NewClusterFaultToleranceTest(),
    }
    
    for _, suite := range testSuites {
        if err := its.testOrchestrator.RunSuite(suite, cluster); err != nil {
            return fmt.Errorf("integration test failed: %w", err)
        }
    }
    
    return nil
}
```

#### 3. Система тестирования отказоустойчивости (Fault Tolerance System)
**Архитектурная роль**: Chaos engineering и тестирование восстановления

**Реализация в коде**:
```go
// fault_tolerance/fault_tolerance_system.go
type FaultToleranceSystem struct {
    chaosEngine      *ChaosEngine
    faultSimulators  map[string]FaultSimulator
    recoveryTester   *RecoveryTester
    circuitBreaker   *CircuitBreakerValidator
}

func (fts *FaultToleranceSystem) RunChaosExperiments() (*ChaosResults, error) {
    experiments := []ChaosExperiment{
        {
            Name: "network-partition",
            Simulator: fts.faultSimulators["network"],
            Duration: 5 * time.Minute,
            BlastRadius: 0.3, // Затронуть 30% узлов
        },
        {
            Name: "node-failure",
            Simulator: fts.faultSimulators["node"],
            Duration: 2 * time.Minute,
            BlastRadius: 0.2, // Затронуть 20% узлов
        },
    }
    
    results := &ChaosResults{
        Experiments: make([]*ExperimentResult, 0),
    }
    
    for _, exp := range experiments {
        result, err := fts.chaosEngine.RunExperiment(exp)
        if err != nil {
            return nil, err
        }
        results.Experiments = append(results.Experiments, result)
    }
    
    return results, nil
}
```

### 🔗 Внешние системы

#### 1. Bitswap System
**Архитектурная роль**: Тестируемая система обмена блоками

**Интеграция в коде**:
```go
// integration/bitswap_integration.go
type BitswapSystemInterface struct {
    bitswapInstances []*bitswap.Bitswap
    networkLayer     network.BitSwapNetwork
    blockstore       blockstore.Blockstore
}

func (bsi *BitswapSystemInterface) CreateTestInstance(config *BitswapConfig) (*bitswap.Bitswap, error) {
    // Создание экземпляра Bitswap для тестирования
    bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
    network := mocknet.New()
    
    instance, err := bitswap.New(context.Background(), network, bs)
    if err != nil {
        return nil, err
    }
    
    bsi.bitswapInstances = append(bsi.bitswapInstances, instance)
    return instance, nil
}

func (bsi *BitswapSystemInterface) SimulateHighLoad(connections int, rps int) error {
    // Симуляция высокой нагрузки на Bitswap
    for i := 0; i < connections; i++ {
        go func(connID int) {
            for {
                // Генерация запросов с заданным RPS
                time.Sleep(time.Second / time.Duration(rps/connections))
                bsi.sendTestRequest(connID)
            }
        }(i)
    }
    return nil
}
```

#### 2. IPFS Cluster
**Архитектурная роль**: Кластерная среда для интеграционного тестирования

**Интеграция в коде**:
```go
// integration/ipfs_cluster_integration.go
type IPFSClusterInterface struct {
    clusterNodes []*cluster.Cluster
    ipfsNodes    []*core.IpfsNode
    network      mocknet.Mocknet
}

func (ici *IPFSClusterInterface) SetupTestCluster(nodeCount int) error {
    ici.network = mocknet.New()
    
    for i := 0; i < nodeCount; i++ {
        // Создание IPFS узла
        ipfsNode, err := ici.createIPFSNode(i)
        if err != nil {
            return err
        }
        ici.ipfsNodes = append(ici.ipfsNodes, ipfsNode)
        
        // Создание Cluster узла
        clusterNode, err := ici.createClusterNode(i, ipfsNode)
        if err != nil {
            return err
        }
        ici.clusterNodes = append(ici.clusterNodes, clusterNode)
    }
    
    // Соединение всех узлов
    return ici.connectAllNodes()
}
```

#### 3. Monitoring System (Prometheus)
**Архитектурная роль**: Система сбора и хранения метрик

**Интеграция в коде**:
```go
// monitoring/prometheus_integration.go
type PrometheusIntegration struct {
    registry   *prometheus.Registry
    collectors map[string]prometheus.Collector
    pusher     *push.Pusher
}

func (pi *PrometheusIntegration) RegisterLoadTestMetrics() {
    // Регистрация метрик нагрузочного тестирования
    pi.collectors["concurrent_connections"] = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "bitswap_concurrent_connections_total",
            Help: "Total number of concurrent connections in load test",
        },
        []string{"test_type", "node_id"},
    )
    
    pi.collectors["requests_per_second"] = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "bitswap_requests_per_second",
            Help: "Requests per second during load test",
        },
        []string{"test_type"},
    )
    
    pi.collectors["response_time"] = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "bitswap_response_time_seconds",
            Help: "Response time distribution",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
        },
        []string{"test_type", "percentile"},
    )
    
    for _, collector := range pi.collectors {
        pi.registry.MustRegister(collector)
    }
}
```

#### 4. CI/CD Pipeline (GitHub Actions)
**Архитектурная роль**: Автоматизированное тестирование и развертывание

**Интеграция в коде**:
```go
// automation/github_actions_integration.go
type GitHubActionsIntegration struct {
    workflowTemplates map[string]*WorkflowTemplate
    secretsManager    *SecretsManager
    artifactsManager  *ArtifactsManager
}

func (gai *GitHubActionsIntegration) GenerateLoadTestWorkflow() *WorkflowTemplate {
    return &WorkflowTemplate{
        Name: "Load Testing Workflow",
        On: WorkflowTriggers{
            Push: &PushTrigger{Branches: []string{"main", "develop"}},
            PullRequest: &PullRequestTrigger{Branches: []string{"main"}},
            Schedule: &ScheduleTrigger{Cron: "0 2 * * *"}, // Ежедневно в 2:00
        },
        Jobs: map[string]*Job{
            "load-tests": {
                RunsOn: "ubuntu-latest",
                Steps: []Step{
                    {Name: "Checkout", Uses: "actions/checkout@v3"},
                    {Name: "Setup Go", Uses: "actions/setup-go@v3", With: map[string]string{"go-version": "1.21"}},
                    {Name: "Run Concurrent Tests", Run: "make test-concurrent"},
                    {Name: "Run Throughput Tests", Run: "make test-throughput"},
                    {Name: "Upload Results", Uses: "actions/upload-artifact@v3"},
                },
            },
        },
    }
}
```

## Взаимодействия между компонентами

### 1. Developer → Load Testing System
**Архитектурное взаимодействие**: Запуск нагрузочных тестов через CLI и Makefile

**Реализация**:
```go
// cmd/load_test_cli.go
func main() {
    var (
        testType = flag.String("type", "all", "Type of test to run")
        config   = flag.String("config", "default", "Test configuration")
    )
    flag.Parse()
    
    loadTestSystem := loadtest.NewLoadTestingSystem(loadtest.LoadConfig(*config))
    
    switch *testType {
    case "concurrent":
        result, err := loadTestSystem.RunConcurrentTests()
        handleResult(result, err)
    case "throughput":
        result, err := loadTestSystem.RunThroughputTests()
        handleResult(result, err)
    case "all":
        result, err := loadTestSystem.ExecuteComprehensiveTests()
        handleResult(result, err)
    }
}
```

### 2. Load Testing System → Bitswap System
**Архитектурное взаимодействие**: Тестирование под нагрузкой 10K+ соединений, 100K+ RPS

**Реализация**:
```go
// bitswap/loadtest/bitswap_load_tester.go
type BitswapLoadTester struct {
    bitswapInstances []*bitswap.Bitswap
    loadGenerators   []*LoadGenerator
    metricsCollector *MetricsCollector
}

func (blt *BitswapLoadTester) TestConcurrentConnections(count int) (*TestResult, error) {
    // Создание множественных соединений
    connections := make([]*Connection, count)
    
    var wg sync.WaitGroup
    for i := 0; i < count; i++ {
        wg.Add(1)
        go func(connID int) {
            defer wg.Done()
            
            conn, err := blt.establishConnection(connID)
            if err != nil {
                blt.metricsCollector.RecordConnectionError(connID, err)
                return
            }
            connections[connID] = conn
            
            // Выполнение тестовых запросов
            blt.performTestRequests(conn)
        }(i)
    }
    
    wg.Wait()
    return blt.analyzeResults(connections), nil
}
```

### 3. Integration Testing → IPFS Cluster
**Архитектурное взаимодействие**: Тестирование межузлового взаимодействия

**Реализация**:
```go
// integration/inter_node_tester.go
func (int *InterNodeTester) TestClusterCommunication(cluster *IPFSCluster) error {
    nodes := cluster.GetNodes()
    
    // Тестирование связности между всеми парами узлов
    for i, nodeA := range nodes {
        for j, nodeB := range nodes[i+1:] {
            latency, err := int.measureLatency(nodeA, nodeB)
            if err != nil {
                return fmt.Errorf("failed to measure latency between %s and %s: %w", 
                    nodeA.ID(), nodeB.ID(), err)
            }
            
            int.metricsCollector.RecordInterNodeLatency(nodeA.ID(), nodeB.ID(), latency)
            
            // Тест обмена блоками
            if err := int.testBlockExchange(nodeA, nodeB); err != nil {
                return fmt.Errorf("block exchange failed: %w", err)
            }
        }
    }
    
    return nil
}
```

## Практическое применение

### Использование диаграммы для разработки

1. **Определение интерфейсов**: Каждая связь на диаграмме соответствует интерфейсу в коде
2. **Планирование тестов**: Взаимодействия показывают, какие интеграционные тесты нужны
3. **Мониторинг**: Связи с внешними системами определяют точки сбора метрик
4. **Автоматизация**: Взаимодействия с CI/CD показывают точки автоматизации

### Соответствие требованиям Task 7

- **7.1**: Load Testing System реализует все требования нагрузочного тестирования
- **7.2**: Integration Testing System покрывает кластерное тестирование
- **7.3**: Fault Tolerance System обеспечивает chaos engineering

Эта диаграмма служит основой для понимания общей архитектуры и планирования детальной реализации всех компонентов Task 7.