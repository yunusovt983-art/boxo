# Подробное объяснение архитектурных диаграмм Task 5

## Введение: Мост между архитектурой и кодом

Каждая диаграмма в этой документации служит мостом между высокоуровневым архитектурным дизайном и конкретной реализацией кода. Диаграммы организованы по принципу "от общего к частному", позволяя разработчикам понять систему на разных уровнях абстракции и найти соответствующие элементы в коде.

---

## 1. Context Diagram (context-diagram.puml)

### 🎯 Назначение
Показывает систему мониторинга в контексте всей экосистемы IPFS/Boxo, определяя границы системы и внешние взаимодействия.

### 🔗 Связь с кодом
```go
// Соответствует основному интерфейсу системы мониторинга
// Файл: bitswap/monitoring/performance_monitor.go
type PerformanceMonitor interface {
    Start(ctx context.Context) error
    Stop() error
    CollectMetrics() (*MetricsSnapshot, error)
    ExportToPrometheus() error
}
```

### 📊 Ключевые элементы диаграммы и их реализация:

#### Системный администратор → Система мониторинга
```go
// HTTP API для администраторов
// Файл: bitswap/monitoring/api/admin_handler.go
type AdminHandler struct {
    monitor PerformanceMonitor
    analyzer BottleneckAnalyzer
}

func (h *AdminHandler) GetSystemStatus(w http.ResponseWriter, r *http.Request) {
    metrics, err := h.monitor.CollectMetrics()
    // Возвращает JSON с текущим состоянием системы
}
```

#### Система мониторинга → Boxo Core
```go
// Интерфейсы для сбора метрик из компонентов Boxo
// Файл: bitswap/monitoring/collectors/bitswap_collector.go
type BitswapCollector struct {
    bitswap *bitswap.Bitswap
}

func (c *BitswapCollector) CollectMetrics() *BitswapMetrics {
    return &BitswapMetrics{
        RequestsPerSecond: c.bitswap.GetRequestRate(),
        ActiveConnections: c.bitswap.GetConnectionCount(),
        // ... другие метрики
    }
}
```

#### Система мониторинга → Prometheus
```go
// Экспорт метрик в формате Prometheus
// Файл: bitswap/monitoring/exporters/prometheus_exporter.go
func (e *PrometheusExporter) ExportMetrics(metrics *MetricsSnapshot) error {
    // Регистрация метрик в Prometheus registry
    e.requestsPerSecond.Set(metrics.Bitswap.RequestsPerSecond)
    e.activeConnections.Set(float64(metrics.Bitswap.ActiveConnections))
    return nil
}
```

### 🎨 Архитектурные решения, отраженные в диаграмме:
- **Разделение ответственности**: Четкое разделение между системой мониторинга и мониторируемыми компонентами
- **Внешние интеграции**: Показаны все внешние системы, с которыми интегрируется мониторинг
- **Пользовательские роли**: Различные типы пользователей с разными потребностями

---

## 2. Container Diagram (container-diagram.puml)

### 🎯 Назначение
Детализирует внутреннюю архитектуру системы мониторинга, показывая основные контейнеры (сервисы) и их взаимодействие.

### 🔗 Связь с кодом
Каждый контейнер соответствует отдельному Go пакету или сервису:

```go
// Структура проекта
bitswap/
├── monitoring/
│   ├── performance/          // Performance Monitor контейнер
│   ├── analysis/            // Bottleneck Analyzer контейнер  
│   ├── alerts/              // Alert Manager контейнер
│   ├── tuning/              // Auto Tuner контейнер
│   ├── storage/             // Metrics Storage
│   └── web/                 // Web Dashboard
```

### 📊 Ключевые контейнеры и их реализация:

#### Performance Monitor
```go
// Файл: bitswap/monitoring/performance/monitor.go
type Monitor struct {
    collectors map[string]MetricsCollector
    storage    MetricsStorage
    exporter   PrometheusExporter
    ticker     *time.Ticker
}

func (m *Monitor) Start(ctx context.Context) error {
    go m.collectLoop(ctx)
    go m.exportLoop(ctx)
    return nil
}

func (m *Monitor) collectLoop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-m.ticker.C:
            m.collectAllMetrics()
        }
    }
}
```

#### Bottleneck Analyzer
```go
// Файл: bitswap/monitoring/analysis/analyzer.go
type Analyzer struct {
    trendAnalyzer    *TrendAnalyzer
    anomalyDetector  *AnomalyDetector
    recommendationEngine *RecommendationEngine
    metricsStorage   MetricsStorage
}

func (a *Analyzer) AnalyzePerformance(ctx context.Context) ([]*PerformanceIssue, error) {
    metrics, err := a.metricsStorage.GetRecentMetrics(time.Hour)
    if err != nil {
        return nil, err
    }
    
    trends := a.trendAnalyzer.AnalyzeTrends(metrics)
    anomalies := a.anomalyDetector.DetectAnomalies(metrics)
    
    return append(trends, anomalies...), nil
}
```

#### Alert Manager
```go
// Файл: bitswap/monitoring/alerts/manager.go
type Manager struct {
    rules       []*AlertRule
    generator   *AlertGenerator
    sender      *NotificationSender
    logger      *StructuredLogger
    tracer      *RequestTracer
}

func (m *Manager) ProcessIssue(issue *PerformanceIssue) error {
    alert := m.generator.GenerateAlert(issue)
    if alert != nil {
        m.logger.LogAlert(alert)
        return m.sender.SendNotification(alert)
    }
    return nil
}
```

#### Auto Tuner
```go
// Файл: bitswap/monitoring/tuning/tuner.go
type Tuner struct {
    predictor   *MLPredictor
    optimizer   *ConfigOptimizer
    applier     *SafeApplier
    rollback    *RollbackManager
}

func (t *Tuner) OptimizeConfiguration(ctx context.Context) error {
    params, err := t.predictor.PredictOptimalParameters(ctx)
    if err != nil {
        return err
    }
    
    config, err := t.optimizer.OptimizeConfiguration(params)
    if err != nil {
        return err
    }
    
    return t.applier.ApplyConfiguration(config)
}
```

### 🎨 Архитектурные паттерны:
- **Microservices**: Каждый контейнер - независимый сервис
- **Event-driven**: Асинхронное взаимодействие через каналы
- **Separation of Concerns**: Четкое разделение ответственности

---

## 3. Component Diagram (component-diagram.puml)

### 🎯 Назначение
Показывает внутреннюю структуру каждого контейнера, детализируя компоненты и их взаимодействие.

### 🔗 Связь с кодом
Каждый компонент соответствует конкретному Go типу или интерфейсу:

#### Performance Monitor Components
```go
// Metrics Collector
// Файл: bitswap/monitoring/performance/collectors/metrics_collector.go
type MetricsCollector struct {
    bitswapMonitor    *BitswapMonitor
    blockstoreMonitor *BlockstoreMonitor
    networkMonitor    *NetworkMonitor
    aggregator        *MetricsAggregator
}

func (c *MetricsCollector) CollectAll() (*MetricsSnapshot, error) {
    bitswapMetrics := c.bitswapMonitor.Collect()
    blockstoreMetrics := c.blockstoreMonitor.Collect()
    networkMetrics := c.networkMonitor.Collect()
    
    return c.aggregator.Aggregate(bitswapMetrics, blockstoreMetrics, networkMetrics)
}

// Prometheus Exporter
// Файл: bitswap/monitoring/performance/exporters/prometheus_exporter.go
type PrometheusExporter struct {
    registry *prometheus.Registry
    server   *http.Server
    
    // Prometheus метрики
    requestsPerSecond   prometheus.Gauge
    activeConnections   prometheus.Gauge
    errorRate          prometheus.Gauge
}

func (e *PrometheusExporter) RegisterMetrics() {
    e.requestsPerSecond = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "bitswap_requests_per_second",
        Help: "Number of Bitswap requests per second",
    })
    e.registry.MustRegister(e.requestsPerSecond)
}
```

#### Bottleneck Analyzer Components
```go
// Trend Analyzer
// Файл: bitswap/monitoring/analysis/trend_analyzer.go
type TrendAnalyzer struct {
    windowSize  int
    thresholds  *AnalysisThresholds
    calculator  *TrendCalculator
}

func (ta *TrendAnalyzer) AnalyzeTrends(metrics []*MetricsSnapshot) []*PerformanceIssue {
    issues := make([]*PerformanceIssue, 0)
    
    for _, metricType := range []string{"latency", "throughput", "error_rate"} {
        trend := ta.calculator.CalculateTrend(metrics, metricType)
        if ta.isDegradation(trend) {
            issue := &PerformanceIssue{
                Type:        IssueTypeTrendDegradation,
                MetricName:  metricType,
                Trend:       trend,
                DetectedAt:  time.Now(),
            }
            issues = append(issues, issue)
        }
    }
    
    return issues
}

// Anomaly Detector
// Файл: bitswap/monitoring/analysis/anomaly_detector.go
type AnomalyDetector struct {
    model       *StatisticalModel
    sensitivity float64
    zscore      *ZScoreCalculator
}

func (ad *AnomalyDetector) DetectAnomalies(metrics []*MetricsSnapshot) []*PerformanceIssue {
    issues := make([]*PerformanceIssue, 0)
    
    for _, snapshot := range metrics {
        if ad.isAnomaly(snapshot.Bitswap.RequestsPerSecond, "requests_per_second") {
            issue := &PerformanceIssue{
                Type:         IssueTypeAnomaly,
                MetricName:   "requests_per_second",
                CurrentValue: snapshot.Bitswap.RequestsPerSecond,
                DetectedAt:   snapshot.Timestamp,
            }
            issues = append(issues, issue)
        }
    }
    
    return issues
}
```

### 🎨 Компонентные паттерны:
- **Strategy Pattern**: Различные алгоритмы анализа
- **Observer Pattern**: Уведомления о событиях
- **Factory Pattern**: Создание различных типов анализаторов

---

## 4. Code Diagram (code-diagram.puml)

### 🎯 Назначение
Показывает ключевые интерфейсы, структуры данных и их взаимосвязи на уровне кода.

### 🔗 Прямое соответствие коду
Диаграмма напрямую отражает Go интерфейсы и структуры:

```go
// Файл: bitswap/monitoring/interfaces.go

// Основные интерфейсы системы мониторинга
type PerformanceMonitor interface {
    CollectMetrics() (*MetricsSnapshot, error)
    ExportToPrometheus() error
    Start(ctx context.Context) error
    Stop() error
}

type BottleneckAnalyzer interface {
    AnalyzeTrends(metrics []*MetricsSnapshot) []*PerformanceIssue
    DetectAnomalies(metrics []*MetricsSnapshot) []*PerformanceIssue
    GenerateRecommendations(issues []*PerformanceIssue) []*OptimizationRecommendation
}

type AlertManager interface {
    GenerateAlert(issue *PerformanceIssue) *AlertEvent
    SendNotification(alert *AlertEvent) error
    ConfigureRules(rules []*AlertRule) error
}

type AutoTuner interface {
    PredictOptimalParameters(metrics []*MetricsSnapshot) *TuningParameters
    OptimizeConfiguration(params *TuningParameters) *OptimizationResult
    ApplyConfiguration(config *Configuration) error
    RollbackConfiguration() error
}

// Структуры данных
type MetricsSnapshot struct {
    BitswapMetrics    *BitswapMetrics    `json:"bitswap"`
    BlockstoreMetrics *BlockstoreMetrics `json:"blockstore"`
    NetworkMetrics    *NetworkMetrics    `json:"network"`
    Timestamp         time.Time          `json:"timestamp"`
    NodeID            string             `json:"node_id"`
}

type BitswapMetrics struct {
    RequestsPerSecond   float64       `json:"requests_per_second"`
    AverageLatency      time.Duration `json:"average_latency"`
    ErrorRate           float64       `json:"error_rate"`
    ActiveConnections   int64         `json:"active_connections"`
    BytesTransferred    int64         `json:"bytes_transferred"`
    PeerCount           int64         `json:"peer_count"`
    Timestamp           time.Time     `json:"timestamp"`
}

type PerformanceIssue struct {
    ID              string            `json:"id"`
    Type            IssueType         `json:"type"`
    Severity        Severity          `json:"severity"`
    Component       string            `json:"component"`
    Description     string            `json:"description"`
    MetricName      string            `json:"metric_name"`
    CurrentValue    float64           `json:"current_value"`
    ExpectedValue   float64           `json:"expected_value"`
    DetectedAt      time.Time         `json:"detected_at"`
    AffectedNodes   []string          `json:"affected_nodes"`
    Metadata        map[string]interface{} `json:"metadata"`
}

// Перечисления
type IssueType int

const (
    IssueTypeLatencyDegradation IssueType = iota
    IssueTypeThroughputDecline
    IssueTypeMemoryLeak
    IssueTypeCPUSpike
    IssueTypeDiskIOBottleneck
    IssueTypeNetworkCongestion
)

type Severity int

const (
    SeverityLow Severity = iota
    SeverityMedium
    SeverityHigh
    SeverityCritical
)
```

### 🎨 Паттерны проектирования в коде:
- **Interface Segregation**: Разделение интерфейсов по ответственности
- **Dependency Injection**: Внедрение зависимостей через интерфейсы
- **Value Objects**: Неизменяемые структуры данных для метрик

---

## 5. Sequence Diagram (sequence-diagram.puml)

### 🎯 Назначение
Показывает временную последовательность взаимодействий между компонентами в различных сценариях.

### 🔗 Связь с кодом
Каждое взаимодействие соответствует вызову метода или отправке сообщения:

#### Сценарий: Сбор и анализ метрик
```go
// Файл: bitswap/monitoring/workflows/monitoring_workflow.go
type MonitoringWorkflow struct {
    monitor  PerformanceMonitor
    analyzer BottleneckAnalyzer
    alertMgr AlertManager
    tuner    AutoTuner
}

func (w *MonitoringWorkflow) ExecuteMonitoringCycle(ctx context.Context) error {
    // 1. Сбор метрик (соответствует стрелке на диаграмме)
    metrics, err := w.monitor.CollectMetrics()
    if err != nil {
        return fmt.Errorf("failed to collect metrics: %w", err)
    }
    
    // 2. Анализ производительности
    issues, err := w.analyzer.AnalyzeTrends([]*MetricsSnapshot{metrics})
    if err != nil {
        return fmt.Errorf("failed to analyze trends: %w", err)
    }
    
    // 3. Генерация алертов при обнаружении проблем
    for _, issue := range issues {
        alert := w.alertMgr.GenerateAlert(issue)
        if alert != nil {
            if err := w.alertMgr.SendNotification(alert); err != nil {
                log.Errorf("Failed to send alert: %v", err)
            }
        }
    }
    
    // 4. Автоматическая оптимизация
    if len(issues) > 0 {
        params, err := w.tuner.PredictOptimalParameters([]*MetricsSnapshot{metrics})
        if err != nil {
            return fmt.Errorf("failed to predict parameters: %w", err)
        }
        
        result, err := w.tuner.OptimizeConfiguration(params)
        if err != nil {
            return fmt.Errorf("failed to optimize configuration: %w", err)
        }
        
        if !result.Success {
            log.Warnf("Optimization failed, rolling back: %v", result.Errors)
            return w.tuner.RollbackConfiguration()
        }
    }
    
    return nil
}
```

#### Сценарий: Обработка алертов
```go
// Файл: bitswap/monitoring/alerts/alert_processor.go
func (ap *AlertProcessor) ProcessAlert(ctx context.Context, issue *PerformanceIssue) error {
    // Генерация алерта
    alert := ap.generator.GenerateAlert(issue)
    if alert == nil {
        return nil // Алерт не требуется
    }
    
    // Логирование
    ap.logger.LogAlert(alert)
    
    // Отправка уведомлений
    for _, channel := range alert.NotificationChannels {
        if err := ap.sender.SendToChannel(alert, channel); err != nil {
            log.Errorf("Failed to send to channel %s: %v", channel, err)
        }
    }
    
    return nil
}
```

### 🎨 Временные паттерны:
- **Request-Response**: Синхронные вызовы методов
- **Fire-and-Forget**: Асинхронные уведомления
- **Timeout Handling**: Обработка таймаутов в длительных операциях

---

## 6. Deployment Diagram (deployment-diagram.puml)

### 🎯 Назначение
Показывает физическое развертывание системы мониторинга в инфраструктуре.

### 🔗 Связь с кодом и конфигурацией
Диаграмма соответствует конфигурационным файлам и скриптам развертывания:

#### Docker Compose конфигурация
```yaml
# Файл: deployments/docker-compose.yml
version: '3.8'

services:
  performance-monitor:
    build: ./bitswap/monitoring/performance
    environment:
      - METRICS_COLLECTION_INTERVAL=30s
      - PROMETHEUS_PORT=9090
    ports:
      - "9090:9090"
    depends_on:
      - redis
      - prometheus
    
  bottleneck-analyzer:
    build: ./bitswap/monitoring/analysis
    environment:
      - ANALYSIS_WINDOW_SIZE=300
      - ANOMALY_SENSITIVITY=0.95
    depends_on:
      - redis
      - performance-monitor
    
  alert-manager:
    build: ./bitswap/monitoring/alerts
    environment:
      - SLACK_WEBHOOK_URL=${SLACK_WEBHOOK_URL}
      - EMAIL_SMTP_SERVER=${EMAIL_SMTP_SERVER}
    volumes:
      - ./config/alert-rules.yml:/etc/alert-rules.yml
    
  auto-tuner:
    build: ./bitswap/monitoring/tuning
    environment:
      - ML_MODEL_PATH=/models/tuning-model.pkl
      - ROLLBACK_TIMEOUT=300s
    volumes:
      - ./models:/models
      - ./config:/config
    
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9091:9090"
    volumes:
      - ./config/prometheus.yml:/etc/prometheus/prometheus.yml
```

#### Kubernetes манифесты
```yaml
# Файл: deployments/k8s/performance-monitor-deployment.yml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: performance-monitor
  namespace: boxo-monitoring
spec:
  replicas: 3
  selector:
    matchLabels:
      app: performance-monitor
  template:
    metadata:
      labels:
        app: performance-monitor
    spec:
      containers:
      - name: performance-monitor
        image: boxo/performance-monitor:latest
        ports:
        - containerPort: 8080
        - containerPort: 9090  # Prometheus metrics
        env:
        - name: REDIS_URL
          value: "redis://redis-service:6379"
        - name: COLLECTION_INTERVAL
          value: "30s"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

#### Конфигурация мониторинга
```go
// Файл: bitswap/monitoring/config/deployment_config.go
type DeploymentConfig struct {
    Environment     string            `yaml:"environment"`
    Replicas        int               `yaml:"replicas"`
    Resources       ResourceLimits    `yaml:"resources"`
    Networking      NetworkConfig     `yaml:"networking"`
    Storage         StorageConfig     `yaml:"storage"`
    Monitoring      MonitoringConfig  `yaml:"monitoring"`
    Security        SecurityConfig    `yaml:"security"`
}

type ResourceLimits struct {
    CPU    string `yaml:"cpu"`
    Memory string `yaml:"memory"`
    Disk   string `yaml:"disk"`
}

func LoadDeploymentConfig(env string) (*DeploymentConfig, error) {
    configPath := fmt.Sprintf("config/deployment-%s.yml", env)
    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, err
    }
    
    var config DeploymentConfig
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, err
    }
    
    return &config, nil
}
```

### 🎨 Инфраструктурные паттерны:
- **High Availability**: Репликация критических компонентов
- **Load Balancing**: Распределение нагрузки между экземплярами
- **Service Discovery**: Автоматическое обнаружение сервисов

---

## 7. State Diagram (state-diagram.puml)

### 🎯 Назначение
Показывает жизненный цикл Auto Tuner и переходы между состояниями оптимизации.

### 🔗 Связь с кодом
Диаграмма состояний напрямую реализована через State Machine:

```go
// Файл: bitswap/monitoring/tuning/state_machine.go
type TunerState int

const (
    StateIdle TunerState = iota
    StateAnalyzing
    StatePredicting
    StateOptimizing
    StateApplying
    StateMonitoring
    StateRollingBack
    StateSuccess
)

type TunerStateMachine struct {
    currentState TunerState
    context      *TuningContext
    transitions  map[TunerState][]TunerState
    handlers     map[TunerState]StateHandler
    mutex        sync.RWMutex
}

type StateHandler func(ctx context.Context, context *TuningContext) (TunerState, error)

func NewTunerStateMachine() *TunerStateMachine {
    sm := &TunerStateMachine{
        currentState: StateIdle,
        transitions:  make(map[TunerState][]TunerState),
        handlers:     make(map[TunerState]StateHandler),
    }
    
    // Определение допустимых переходов (соответствует диаграмме)
    sm.transitions[StateIdle] = []TunerState{StateAnalyzing}
    sm.transitions[StateAnalyzing] = []TunerState{StatePredicting, StateIdle}
    sm.transitions[StatePredicting] = []TunerState{StateOptimizing, StateIdle}
    sm.transitions[StateOptimizing] = []TunerState{StateApplying, StateIdle}
    sm.transitions[StateApplying] = []TunerState{StateMonitoring, StateRollingBack}
    sm.transitions[StateMonitoring] = []TunerState{StateSuccess, StateRollingBack}
    sm.transitions[StateRollingBack] = []TunerState{StateIdle}
    sm.transitions[StateSuccess] = []TunerState{StateIdle}
    
    // Регистрация обработчиков состояний
    sm.handlers[StateIdle] = sm.handleIdle
    sm.handlers[StateAnalyzing] = sm.handleAnalyzing
    sm.handlers[StatePredicting] = sm.handlePredicting
    sm.handlers[StateOptimizing] = sm.handleOptimizing
    sm.handlers[StateApplying] = sm.handleApplying
    sm.handlers[StateMonitoring] = sm.handleMonitoring
    sm.handlers[StateRollingBack] = sm.handleRollingBack
    sm.handlers[StateSuccess] = sm.handleSuccess
    
    return sm
}

func (sm *TunerStateMachine) TransitionTo(newState TunerState) error {
    sm.mutex.Lock()
    defer sm.mutex.Unlock()
    
    // Проверка допустимости перехода
    validTransitions := sm.transitions[sm.currentState]
    for _, validState := range validTransitions {
        if validState == newState {
            log.Infof("Transitioning from %v to %v", sm.currentState, newState)
            sm.currentState = newState
            return nil
        }
    }
    
    return fmt.Errorf("invalid transition from %v to %v", sm.currentState, newState)
}

// Обработчики состояний
func (sm *TunerStateMachine) handleAnalyzing(ctx context.Context, tuningCtx *TuningContext) (TunerState, error) {
    log.Info("Analyzing current metrics...")
    
    metrics, err := tuningCtx.Monitor.CollectMetrics()
    if err != nil {
        return StateIdle, fmt.Errorf("failed to collect metrics: %w", err)
    }
    
    issues, err := tuningCtx.Analyzer.AnalyzeTrends([]*MetricsSnapshot{metrics})
    if err != nil {
        return StateIdle, fmt.Errorf("failed to analyze trends: %w", err)
    }
    
    if len(issues) == 0 {
        log.Info("No optimization needed")
        return StateIdle, nil
    }
    
    tuningCtx.Issues = issues
    return StatePredicting, nil
}

func (sm *TunerStateMachine) handlePredicting(ctx context.Context, tuningCtx *TuningContext) (TunerState, error) {
    log.Info("Predicting optimal parameters...")
    
    params, err := tuningCtx.Predictor.PredictOptimalParameters(tuningCtx.Metrics)
    if err != nil {
        return StateIdle, fmt.Errorf("failed to predict parameters: %w", err)
    }
    
    if params.Confidence < 0.8 {
        log.Warnf("Low confidence in predictions: %.2f", params.Confidence)
        return StateIdle, nil
    }
    
    tuningCtx.Parameters = params
    return StateOptimizing, nil
}

func (sm *TunerStateMachine) handleApplying(ctx context.Context, tuningCtx *TuningContext) (TunerState, error) {
    log.Info("Applying configuration changes...")
    
    // Создание backup
    if err := tuningCtx.Applier.CreateBackup(); err != nil {
        return StateRollingBack, fmt.Errorf("failed to create backup: %w", err)
    }
    
    // Применение конфигурации
    if err := tuningCtx.Applier.ApplyConfiguration(tuningCtx.Configuration); err != nil {
        return StateRollingBack, fmt.Errorf("failed to apply configuration: %w", err)
    }
    
    // Запуск мониторинга результатов
    tuningCtx.MonitoringStartTime = time.Now()
    return StateMonitoring, nil
}

func (sm *TunerStateMachine) handleMonitoring(ctx context.Context, tuningCtx *TuningContext) (TunerState, error) {
    log.Info("Monitoring optimization results...")
    
    // Сбор метрик после применения изменений
    currentMetrics, err := tuningCtx.Monitor.CollectMetrics()
    if err != nil {
        return StateRollingBack, fmt.Errorf("failed to collect metrics: %w", err)
    }
    
    // Сравнение с базовыми метриками
    improvement := calculateImprovement(tuningCtx.BaselineMetrics, currentMetrics)
    
    if improvement < 0 {
        log.Warnf("Performance degradation detected: %.2f%%", improvement*100)
        return StateRollingBack, nil
    }
    
    if improvement > tuningCtx.Parameters.PredictedImprovement*0.8 {
        log.Infof("Optimization successful: %.2f%% improvement", improvement*100)
        return StateSuccess, nil
    }
    
    // Продолжение мониторинга, если результаты неопределенные
    if time.Since(tuningCtx.MonitoringStartTime) > 5*time.Minute {
        log.Warn("Monitoring timeout, rolling back")
        return StateRollingBack, nil
    }
    
    return StateMonitoring, nil
}
```

### 🎨 Паттерны состояний:
- **State Machine**: Четкое управление состояниями
- **Guard Conditions**: Проверки перед переходами
- **Entry/Exit Actions**: Действия при входе/выходе из состояний

---

## 8. Class Diagram (class-diagram.puml)

### 🎯 Назначение
Показывает детальную структуру классов, интерфейсов и их взаимосвязи.

### 🔗 Прямое соответствие Go структурам
Каждый класс на диаграмме соответствует Go структуре или интерфейсу:

```go
// Файл: bitswap/monitoring/types/metrics.go

// Основные структуры метрик (соответствуют классам на диаграмме)
type MetricsSnapshot struct {
    BitswapMetrics    *BitswapMetrics    `json:"bitswap" validate:"required"`
    BlockstoreMetrics *BlockstoreMetrics `json:"blockstore" validate:"required"`
    NetworkMetrics    *NetworkMetrics    `json:"network" validate:"required"`
    Timestamp         time.Time          `json:"timestamp" validate:"required"`
    NodeID            string             `json:"node_id" validate:"required,min=1"`
}

func (ms *MetricsSnapshot) Validate() error {
    validate := validator.New()
    return validate.Struct(ms)
}

func (ms *MetricsSnapshot) Age() time.Duration {
    return time.Since(ms.Timestamp)
}

func (ms *MetricsSnapshot) IsStale(maxAge time.Duration) bool {
    return ms.Age() > maxAge
}

type BitswapMetrics struct {
    RequestsPerSecond   float64       `json:"requests_per_second" validate:"min=0"`
    AverageLatency      time.Duration `json:"average_latency" validate:"min=0"`
    ErrorRate           float64       `json:"error_rate" validate:"min=0,max=1"`
    ActiveConnections   int64         `json:"active_connections" validate:"min=0"`
    BytesTransferred    int64         `json:"bytes_transferred" validate:"min=0"`
    PeerCount           int64         `json:"peer_count" validate:"min=0"`
    Timestamp           time.Time     `json:"timestamp" validate:"required"`
    
    // Дополнительные вычисляемые поля
    ThroughputMBps      float64       `json:"throughput_mbps"`
    LatencyP95          time.Duration `json:"latency_p95"`
    LatencyP99          time.Duration `json:"latency_p99"`
}

func (bm *BitswapMetrics) CalculateThroughput() {
    bm.ThroughputMBps = float64(bm.BytesTransferred) / (1024 * 1024)
}

func (bm *BitswapMetrics) IsHealthy() bool {
    return bm.ErrorRate < 0.05 && // Менее 5% ошибок
           bm.AverageLatency < 100*time.Millisecond && // Латентность менее 100мс
           bm.RequestsPerSecond > 0 // Есть активность
}

// Файл: bitswap/monitoring/types/issues.go
type PerformanceIssue struct {
    ID              string                 `json:"id" validate:"required"`
    Type            IssueType              `json:"type" validate:"required"`
    Severity        Severity               `json:"severity" validate:"required"`
    Component       string                 `json:"component" validate:"required"`
    Description     string                 `json:"description" validate:"required"`
    MetricName      string                 `json:"metric_name" validate:"required"`
    CurrentValue    float64                `json:"current_value"`
    ExpectedValue   float64                `json:"expected_value"`
    DetectedAt      time.Time              `json:"detected_at" validate:"required"`
    AffectedNodes   []string               `json:"affected_nodes"`
    Metadata        map[string]interface{} `json:"metadata"`
    
    // Методы для работы с проблемой
    resolvedAt      *time.Time
    escalationLevel int
}

func (pi *PerformanceIssue) IsResolved() bool {
    return pi.resolvedAt != nil
}

func (pi *PerformanceIssue) Resolve() {
    now := time.Now()
    pi.resolvedAt = &now
}

func (pi *PerformanceIssue) Duration() time.Duration {
    if pi.resolvedAt != nil {
        return pi.resolvedAt.Sub(pi.DetectedAt)
    }
    return time.Since(pi.DetectedAt)
}

func (pi *PerformanceIssue) ShouldEscalate(maxDuration time.Duration) bool {
    return !pi.IsResolved() && pi.Duration() > maxDuration
}

// Перечисления с методами
type IssueType int

const (
    IssueTypeLatencyDegradation IssueType = iota
    IssueTypeThroughputDecline
    IssueTypeMemoryLeak
    IssueTypeCPUSpike
    IssueTypeDiskIOBottleneck
    IssueTypeNetworkCongestion
)

func (it IssueType) String() string {
    switch it {
    case IssueTypeLatencyDegradation:
        return "latency_degradation"
    case IssueTypeThroughputDecline:
        return "throughput_decline"
    case IssueTypeMemoryLeak:
        return "memory_leak"
    case IssueTypeCPUSpike:
        return "cpu_spike"
    case IssueTypeDiskIOBottleneck:
        return "disk_io_bottleneck"
    case IssueTypeNetworkCongestion:
        return "network_congestion"
    default:
        return "unknown"
    }
}

func (it IssueType) Priority() int {
    switch it {
    case IssueTypeMemoryLeak:
        return 10 // Высший приоритет
    case IssueTypeCPUSpike:
        return 8
    case IssueTypeLatencyDegradation:
        return 6
    case IssueTypeThroughputDecline:
        return 5
    case IssueTypeNetworkCongestion:
        return 4
    case IssueTypeDiskIOBottleneck:
        return 3
    default:
        return 1
    }
}

// Файл: bitswap/monitoring/types/recommendations.go
type OptimizationRecommendation struct {
    ID                  string                 `json:"id" validate:"required"`
    IssueID             string                 `json:"issue_id" validate:"required"`
    Type                RecommendationType     `json:"type" validate:"required"`
    Priority            Priority               `json:"priority" validate:"required"`
    Description         string                 `json:"description" validate:"required"`
    Parameters          map[string]interface{} `json:"parameters" validate:"required"`
    ExpectedImprovement float64                `json:"expected_improvement" validate:"min=0,max=1"`
    RiskLevel           RiskLevel              `json:"risk_level" validate:"required"`
    CreatedAt           time.Time              `json:"created_at" validate:"required"`
    
    // Дополнительные поля для отслеживания
    appliedAt    *time.Time
    rollbackAt   *time.Time
    actualResult *OptimizationResult
}

func (or *OptimizationRecommendation) IsApplied() bool {
    return or.appliedAt != nil
}

func (or *OptimizationRecommendation) IsRolledBack() bool {
    return or.rollbackAt != nil
}

func (or *OptimizationRecommendation) CanApply() bool {
    return !or.IsApplied() && or.RiskLevel <= RiskLevelMedium
}

func (or *OptimizationRecommendation) Apply() error {
    if !or.CanApply() {
        return fmt.Errorf("recommendation cannot be applied")
    }
    
    now := time.Now()
    or.appliedAt = &now
    return nil
}
```

### 🎨 ООП паттерны в Go:
- **Composition over Inheritance**: Встраивание структур
- **Interface Segregation**: Маленькие, специализированные интерфейсы
- **Method Sets**: Методы, привязанные к типам
- **Value vs Pointer Receivers**: Правильное использование получателей

---

## Заключение: Единство архитектуры и кода

Каждая диаграмма в этой документации не просто иллюстрация - это **живая карта кода**, которая:

### 🔄 Обеспечивает двустороннюю связь:
- **От архитектуры к коду**: Диаграммы помогают разработчикам понять, какие структуры и интерфейсы нужно реализовать
- **От кода к архитектуре**: Изменения в коде должны отражаться в обновлении диаграмм

### 📋 Служит инструментом коммуникации:
- **Для новых разработчиков**: Быстрое понимание архитектуры системы
- **Для code review**: Проверка соответствия реализации архитектурным решениям
- **Для документации**: Актуальное описание системы на разных уровнях абстракции

### 🎯 Поддерживает качество кода:
- **Consistency**: Единообразие в именовании и структуре
- **Maintainability**: Легкость внесения изменений
- **Testability**: Четкое разделение ответственности для тестирования

### 🚀 Ускоряет разработку:
- **Code Generation**: Возможность генерации заготовок кода из диаграмм
- **Validation**: Проверка соответствия кода архитектуре
- **Refactoring**: Планирование изменений на уровне архитектуры

Эта документация должна обновляться вместе с кодом, обеспечивая актуальность и полезность для всей команды разработки.