# Task 6 Code Diagram - Подробное объяснение

## Обзор диаграммы
Диаграмма кода (C4 Level 4) показывает детальную структуру классов, интерфейсов и их взаимосвязи в Task 6. Это самый детальный уровень, который напрямую соответствует реализации в Go коде.

## Связь с реализацией кода

### 📊 **Resource Monitoring (Task 6.1)**

#### ResourceMonitorInterface
**Архитектурное представление:**
```
interface ResourceMonitorInterface {
    +GetThresholds() ResourceMonitorThresholds
    +GetCurrentStatus() map[string]interface{}
}
```

**Реализация в коде:**
```go
// monitoring/auto_scaler.go - интерфейс для AutoScaler
type ResourceMonitorInterface interface {
    GetThresholds() ResourceMonitorThresholds
    GetCurrentStatus() map[string]interface{}
}

// monitoring/resource_monitor.go - реализация
func (rm *ResourceMonitor) GetThresholds() ResourceMonitorThresholds {
    rm.mu.RLock()
    defer rm.mu.RUnlock()
    return rm.thresholds
}

func (rm *ResourceMonitor) GetCurrentStatus() map[string]interface{} {
    rm.mu.RLock()
    defer rm.mu.RUnlock()
    
    summary := rm.collector.GetResourceSummary()
    
    // Добавление трендовой информации
    cpuDirection, cpuSlope := rm.cpuHistory.GetTrend()
    memoryDirection, memorySlope := rm.memoryHistory.GetTrend()
    
    summary["cpu_trend"] = map[string]interface{}{
        "direction": cpuDirection,
        "slope":     cpuSlope,
    }
    summary["memory_trend"] = map[string]interface{}{
        "direction": memoryDirection,
        "slope":     memorySlope,
    }
    
    return summary
}
```

#### ResourceMonitor Class
**Архитектурное представление:**
```
class ResourceMonitor {
    -collector ResourceCollector
    -thresholds ResourceMonitorThresholds
    -history []ResourceSnapshot
    -alertCallback AlertCallback
    +Start(ctx context.Context) error
    +Stop() error
    +ForceCollection() error
    +GetResourceHistory() []ResourceSnapshot
    +PredictResourceUsage() ResourcePrediction
}
```

**Реализация в коде:**
```go
// monitoring/resource_monitor.go
type ResourceMonitor struct {
    mu            sync.RWMutex
    collector     *ResourceCollector
    thresholds    ResourceMonitorThresholds
    alertCallback func(ResourceAlert)

    // Исторические данные для предиктивного анализа
    cpuHistory       *ResourceHistory
    memoryHistory    *ResourceHistory
    diskHistory      *ResourceHistory
    goroutineHistory *ResourceHistory

    // Состояние мониторинга
    isRunning       bool
    stopChan        chan struct{}
    monitorInterval time.Duration

    // Состояние алертов
    lastAlerts    map[string]time.Time
    alertCooldown time.Duration
}

// Конструктор
func NewResourceMonitor(collector *ResourceCollector, thresholds ResourceMonitorThresholds) *ResourceMonitor {
    return &ResourceMonitor{
        collector:        collector,
        thresholds:       thresholds,
        cpuHistory:       NewResourceHistory(100), // Последние 100 точек данных
        memoryHistory:    NewResourceHistory(100),
        diskHistory:      NewResourceHistory(100),
        goroutineHistory: NewResourceHistory(100),
        lastAlerts:       make(map[string]time.Time),
        alertCooldown:    5 * time.Minute,
        monitorInterval:  30 * time.Second,
        stopChan:         make(chan struct{}),
    }
}

// Основные методы
func (rm *ResourceMonitor) Start(ctx context.Context) error {
    rm.mu.Lock()
    if rm.isRunning {
        rm.mu.Unlock()
        return fmt.Errorf("resource monitor is already running")
    }
    rm.isRunning = true
    rm.mu.Unlock()

    go rm.monitorLoop(ctx)
    return nil
}

func (rm *ResourceMonitor) Stop() {
    rm.mu.Lock()
    defer rm.mu.Unlock()

    if rm.isRunning {
        close(rm.stopChan)
        rm.isRunning = false
    }
}

func (rm *ResourceMonitor) ForceCollection() {
    rm.collectAndAnalyze()
}

func (rm *ResourceMonitor) GetResourceHistory(metricType string) *ResourceHistory {
    rm.mu.RLock()
    defer rm.mu.RUnlock()

    switch metricType {
    case "cpu":
        return rm.cpuHistory
    case "memory":
        return rm.memoryHistory
    case "disk":
        return rm.diskHistory
    case "goroutines":
        return rm.goroutineHistory
    default:
        return nil
    }
}

// Предиктивный анализ
func (rm *ResourceMonitor) PredictResourceUsage() ResourcePrediction {
    // Реализация предиктивного анализа
    // (в текущем коде это часть getPrediction метода)
}
```

#### MetricCollector Interface
**Архитектурное представление:**
```
interface MetricCollector {
    +CollectMetrics() map[string]interface{}
    +GetMetricNames() []string
}
```

**Реализация в коде:**
```go
// monitoring/resource_monitor.go
type MetricCollector interface {
    CollectMetrics() map[string]interface{}
    GetMetricNames() []string
}

// ResourceCollector реализует MetricCollector
type ResourceCollector struct {
    cpuUsage    float64
    memoryStats MemoryStats
    diskStats   DiskStats
}

func (rc *ResourceCollector) CollectMetrics() map[string]interface{} {
    // Реализация сбора метрик
    return map[string]interface{}{
        "cpu_usage":       rc.cpuUsage,
        "memory_pressure": rc.GetMemoryPressure(),
        "disk_pressure":   rc.GetDiskPressure(),
        "goroutine_count": runtime.NumGoroutine(),
    }
}

func (rc *ResourceCollector) GetMetricNames() []string {
    return []string{"cpu_usage", "memory_pressure", "disk_pressure", "goroutine_count"}
}
```

#### ResourceSnapshot Struct
**Архитектурное представление:**
```
struct ResourceSnapshot {
    +Timestamp time.Time
    +CPUUsage float64
    +MemoryUsage uint64
    +DiskUsage uint64
    +GoroutineCount int
}
```

**Реализация в коде:**
```go
// В текущей реализации используется ResourceHistory
// ResourceSnapshot можно добавить как:
type ResourceSnapshot struct {
    Timestamp       time.Time `json:"timestamp"`
    CPUUsage        float64   `json:"cpu_usage"`
    MemoryUsage     uint64    `json:"memory_usage"`
    DiskUsage       uint64    `json:"disk_usage"`
    GoroutineCount  int       `json:"goroutine_count"`
}

// Метод для получения снимков из истории
func (rm *ResourceMonitor) GetResourceSnapshots() []ResourceSnapshot {
    snapshots := make([]ResourceSnapshot, 0)
    
    cpuHistory := rm.GetResourceHistory("cpu")
    memoryHistory := rm.GetResourceHistory("memory")
    
    // Объединение данных из разных историй в снимки
    for i := 0; i < len(cpuHistory.Timestamps); i++ {
        snapshot := ResourceSnapshot{
            Timestamp:   cpuHistory.Timestamps[i],
            CPUUsage:    cpuHistory.Values[i],
            // Добавление других метрик...
        }
        snapshots = append(snapshots, snapshot)
    }
    
    return snapshots
}
```

### 🛡️ **Graceful Degradation (Task 6.2)**

#### GracefulDegradationManager Interface
**Архитектурное представление:**
```
interface GracefulDegradationManager {
    +Start(ctx context.Context) error
    +Stop() error
    +SetDegradationRules(rules []DegradationRule) error
    +GetCurrentDegradationLevel() DegradationLevel
    +ForceRecovery() error
    +SetRecoveryCallback(callback RecoveryCallback) error
}
```

**Реализация в коде:**
```go
// monitoring/graceful_degradation.go
type GracefulDegradationManager interface {
    Start(ctx context.Context) error
    Stop() error
    SetDegradationRules(rules []DegradationRule) error
    GetCurrentDegradationLevel() DegradationLevel
    GetDegradationStatus() DegradationStatus
    ForceRecovery() error
    RegisterDegradationAction(name string, action DegradationAction) error
    SetRecoveryCallback(callback RecoveryCallback) error
}
```

#### gracefulDegradationManager Class
**Архитектурное представление:**
```
class gracefulDegradationManager {
    -performanceMonitor PerformanceMonitor
    -resourceMonitor *ResourceMonitor
    -currentLevel DegradationLevel
    -rules []DegradationRule
    -actions map[string]DegradationAction
    -isRunning bool
    +evaluateDegradation(ctx context.Context)
    +applyDegradationActions(level DegradationLevel)
    +recoverFromDegradation()
}
```

**Реализация в коде:**
```go
type gracefulDegradationManager struct {
    mu                 sync.RWMutex
    performanceMonitor PerformanceMonitor
    resourceMonitor    *ResourceMonitor
    logger             *slog.Logger

    // Состояние деградации
    currentLevel       DegradationLevel
    degradationStarted *time.Time
    lastLevelChange    time.Time
    activeRules        map[string]*DegradationRule
    activeActions      map[string]DegradationAction

    // Конфигурация
    rules            []DegradationRule
    actions          map[string]DegradationAction
    recoveryCallback RecoveryCallback

    // Мониторинг
    isRunning       bool
    stopChan        chan struct{}
    monitorInterval time.Duration

    // Управление восстановлением
    recoveryThresholds map[DegradationLevel]RecoveryThreshold
    recoveryProgress   float64

    // История
    eventHistory   []DegradationEvent
    maxHistorySize int
}

// Основные методы
func (gdm *gracefulDegradationManager) evaluateDegradation(ctx context.Context) {
    metrics := gdm.performanceMonitor.CollectMetrics()
    if metrics == nil {
        return
    }

    // Оценка восстановления в первую очередь
    if gdm.currentLevel != DegradationNone {
        if gdm.shouldRecover(metrics) {
            gdm.attemptRecovery(ctx, metrics)
            return
        }
    }

    // Оценка правил деградации
    highestLevel := DegradationNone
    var triggeredRules []*DegradationRule

    for _, rule := range gdm.rules {
        if !rule.Enabled {
            continue
        }

        if gdm.evaluateRule(rule, metrics) {
            triggeredRules = append(triggeredRules, &rule)
            if rule.Level > highestLevel {
                highestLevel = rule.Level
            }
        }
    }

    // Применение деградации при необходимости
    if highestLevel > gdm.currentLevel {
        gdm.applyDegradation(ctx, highestLevel, triggeredRules, metrics)
    }
}

func (gdm *gracefulDegradationManager) applyDegradationActions(level DegradationLevel) {
    // Реализация применения действий деградации
    // (в коде это часть applyDegradation метода)
}

func (gdm *gracefulDegradationManager) recoverFromDegradation() {
    // Реализация восстановления
    // (в коде это recoverToLevel метод)
}
```

#### DegradationAction Interface
**Архитектурное представление:**
```
interface DegradationAction {
    +Execute(level DegradationLevel, params map[string]interface{}) error
    +Recover() error
    +GetActionName() string
    +IsReversible() bool
}
```

**Реализация в коде:**
```go
type DegradationAction interface {
    Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error
    Recover(ctx context.Context) error
    GetName() string
    IsActive() bool
}

// Пример реализации действия
type ReduceWorkerThreadsAction struct {
    isActive      bool
    originalScale int
}

func (r *ReduceWorkerThreadsAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
    // Реализация уменьшения количества воркеров
    r.isActive = true
    return nil
}

func (r *ReduceWorkerThreadsAction) Recover(ctx context.Context) error {
    // Восстановление исходного количества воркеров
    r.isActive = false
    return nil
}

func (r *ReduceWorkerThreadsAction) GetName() string {
    return "reduce_worker_threads"
}

func (r *ReduceWorkerThreadsAction) IsActive() bool {
    return r.isActive
}
```

#### DegradationLevel Enum
**Архитектурное представление:**
```
enum DegradationLevel {
    DegradationNone
    DegradationLight
    DegradationModerate
    DegradationSevere
    DegradationCritical
}
```

**Реализация в коде:**
```go
type DegradationLevel int

const (
    DegradationNone DegradationLevel = iota
    DegradationLight
    DegradationModerate
    DegradationSevere
    DegradationCritical
)

func (dl DegradationLevel) String() string {
    switch dl {
    case DegradationNone:
        return "none"
    case DegradationLight:
        return "light"
    case DegradationModerate:
        return "moderate"
    case DegradationSevere:
        return "severe"
    case DegradationCritical:
        return "critical"
    default:
        return "unknown"
    }
}
```

#### DegradationRule Struct
**Архитектурное представление:**
```
struct DegradationRule {
    +ID string
    +Name string
    +Condition AlertCondition
    +Threshold interface{}
    +Level DegradationLevel
    +Actions []string
    +Priority int
}
```

**Реализация в коде:**
```go
type DegradationRule struct {
    ID          string           `json:"id"`
    Name        string           `json:"name"`
    Description string           `json:"description"`
    Component   string           `json:"component"`
    Metric      string           `json:"metric"`
    Condition   AlertCondition   `json:"condition"`
    Threshold   interface{}      `json:"threshold"`
    Level       DegradationLevel `json:"level"`
    Actions     []string         `json:"actions"`
    Duration    time.Duration    `json:"duration"`
    Priority    int              `json:"priority"`
    Enabled     bool             `json:"enabled"`
    CreatedAt   time.Time        `json:"created_at"`
    UpdatedAt   time.Time        `json:"updated_at"`
}
```

### ⚖️ **Auto Scaling (Task 6.3)**

#### AutoScaler Interface
**Архитектурное представление:**
```
interface AutoScaler {
    +Start(ctx context.Context) error
    +Stop() error
    +SetScalingRules(rules []ScalingRule) error
    +RegisterScalableComponent(name string, component ScalableComponent) error
    +ForceScale(component string, direction ScalingDirection, factor float64) error
    +IsolateComponent(component string, reason string) error
    +RestoreComponent(component string) error
    +GetScalingStatus() ScalingStatus
}
```

**Реализация в коде:**
```go
// monitoring/auto_scaler.go
type AutoScaler interface {
    Start(ctx context.Context) error
    Stop() error
    SetScalingRules(rules []ScalingRule) error
    GetScalingStatus() ScalingStatus
    ForceScale(component string, direction ScalingDirection, factor float64) error
    RegisterScalableComponent(name string, component ScalableComponent) error
    IsolateComponent(component string, reason string) error
    RestoreComponent(component string) error
    GetIsolatedComponents() []IsolatedComponent
    SetScalingCallback(callback ScalingCallback) error
}
```

#### autoScaler Class
**Архитектурное представление:**
```
class autoScaler {
    -performanceMonitor PerformanceMonitor
    -resourceMonitor ResourceMonitorInterface
    -rules []ScalingRule
    -components map[string]ScalableComponent
    -isolatedComponents map[string]*IsolatedComponent
    -scalingHistory []ScalingEvent
    -lastScalingTime map[string]time.Time
    +evaluateScaling(ctx context.Context)
    +applyScaling(ctx context.Context, rule ScalingRule, metrics *PerformanceMetrics)
    +isInCooldown(component string, cooldown time.Duration) bool
}
```

**Реализация в коде:**
```go
type autoScaler struct {
    mu                 sync.RWMutex
    performanceMonitor PerformanceMonitor
    resourceMonitor    ResourceMonitorInterface
    logger             *slog.Logger

    // Состояние масштабирования
    isRunning       bool
    stopChan        chan struct{}
    monitorInterval time.Duration

    // Конфигурация
    rules           []ScalingRule
    components      map[string]ScalableComponent
    scalingCallback ScalingCallback

    // Управление масштабированием
    lastScalingTime map[string]time.Time
    scalingHistory  []ScalingEvent
    maxHistorySize  int

    // Изоляция компонентов
    isolatedComponents map[string]*IsolatedComponent
    isolationTimeout   time.Duration

    // Мониторинг здоровья
    componentHealth map[string]*ComponentHealth
}

// Основные методы
func (as *autoScaler) evaluateScaling(ctx context.Context) {
    metrics := as.performanceMonitor.CollectMetrics()
    if metrics == nil {
        return
    }

    // Обновление здоровья компонентов
    as.updateComponentHealth(metrics)

    // Оценка правил масштабирования
    for _, rule := range as.rules {
        if !rule.Enabled {
            continue
        }

        if as.isComponentIsolated(rule.Component) {
            continue
        }

        if as.isInCooldown(rule.Component, rule.Cooldown) {
            continue
        }

        if as.evaluateScalingRule(rule, metrics) {
            as.applyScaling(ctx, rule, metrics)
        }
    }
}

func (as *autoScaler) applyScaling(ctx context.Context, rule ScalingRule, metrics *PerformanceMetrics) {
    component, exists := as.components[rule.Component]
    if !exists {
        return
    }

    currentScale := component.GetCurrentScale()
    var newScale int

    switch rule.Direction {
    case ScaleUp:
        newScale = int(math.Ceil(float64(currentScale) * rule.ScaleFactor))
    case ScaleDown:
        newScale = int(math.Floor(float64(currentScale) * rule.ScaleFactor))
    default:
        return
    }

    // Применение ограничений
    if newScale < rule.MinValue {
        newScale = rule.MinValue
    }
    if newScale > rule.MaxValue {
        newScale = rule.MaxValue
    }

    if newScale == currentScale {
        return
    }

    // Применение масштабирования
    if err := component.SetScale(ctx, newScale); err != nil {
        // Обработка ошибки
    }
}

func (as *autoScaler) isInCooldown(component string, cooldown time.Duration) bool {
    as.mu.RLock()
    defer as.mu.RUnlock()

    lastScaling, exists := as.lastScalingTime[component]
    if !exists {
        return false
    }

    return time.Since(lastScaling) < cooldown
}
```

#### ScalableComponent Interface
**Архитектурное представление:**
```
interface ScalableComponent {
    +GetCurrentScale() int
    +SetScale(ctx context.Context, scale int) error
    +GetScaleRange() (min, max int)
    +GetComponentName() string
    +IsHealthy() bool
    +GetMetrics() map[string]interface{}
}
```

**Реализация в коде:**
```go
// monitoring/auto_scaler.go
type ScalableComponent interface {
    GetCurrentScale() int
    SetScale(ctx context.Context, scale int) error
    GetScaleRange() (min, max int)
    GetComponentName() string
    IsHealthy() bool
    GetMetrics() map[string]interface{}
}
```

#### Concrete Component Classes
**Архитектурное представление:**
```
class WorkerPoolComponent {
    -currentScale int
    -minScale int
    -maxScale int
    -activeWorkers int
    -queuedTasks int64
    +UpdateMetrics(queuedTasks, processedTasks int64, avgLatency time.Duration)
}
```

**Реализация в коде:**
```go
// monitoring/scalable_components.go
type WorkerPoolComponent struct {
    mu           sync.RWMutex
    name         string
    currentScale int
    minScale     int
    maxScale     int
    isHealthy    bool
    logger       *slog.Logger

    // Метрики
    activeWorkers  int
    queuedTasks    int64
    processedTasks int64
    averageLatency time.Duration
    lastScaleTime  time.Time
}

func (w *WorkerPoolComponent) GetCurrentScale() int {
    w.mu.RLock()
    defer w.mu.RUnlock()
    return w.currentScale
}

func (w *WorkerPoolComponent) SetScale(ctx context.Context, scale int) error {
    w.mu.Lock()
    defer w.mu.Unlock()

    if scale < w.minScale || scale > w.maxScale {
        return fmt.Errorf("scale %d is outside valid range [%d, %d]", scale, w.minScale, w.maxScale)
    }

    oldScale := w.currentScale
    w.currentScale = scale
    w.lastScaleTime = time.Now()

    // Симуляция масштабирования пула воркеров
    if scale > oldScale {
        w.activeWorkers = scale
    } else if scale < oldScale {
        w.activeWorkers = scale
    }

    return nil
}

func (w *WorkerPoolComponent) GetScaleRange() (min, max int) {
    w.mu.RLock()
    defer w.mu.RUnlock()
    return w.minScale, w.maxScale
}

func (w *WorkerPoolComponent) GetComponentName() string {
    return w.name
}

func (w *WorkerPoolComponent) IsHealthy() bool {
    w.mu.RLock()
    defer w.mu.RUnlock()
    return w.isHealthy
}

func (w *WorkerPoolComponent) GetMetrics() map[string]interface{} {
    w.mu.RLock()
    defer w.mu.RUnlock()

    return map[string]interface{}{
        "current_scale":   w.currentScale,
        "active_workers":  w.activeWorkers,
        "queued_tasks":    w.queuedTasks,
        "processed_tasks": w.processedTasks,
        "average_latency": w.averageLatency,
        "utilization":     float64(w.activeWorkers) / float64(w.currentScale),
    }
}

func (w *WorkerPoolComponent) UpdateMetrics(queuedTasks, processedTasks int64, avgLatency time.Duration) {
    w.mu.Lock()
    defer w.mu.Unlock()
    w.queuedTasks = queuedTasks
    w.processedTasks = processedTasks
    w.averageLatency = avgLatency
}
```

### 🔄 **Cross-package relationships**

#### Relationships Implementation
**Архитектурное представление:**
```
gracefulDegradationManager --> ResourceMonitorInterface
gracefulDegradationManager --> PerformanceMonitor
autoScaler --> ResourceMonitorInterface
autoScaler --> PerformanceMonitor
```

**Реализация в коде:**
```go
// Graceful Degradation использует ResourceMonitor
func NewGracefulDegradationManager(
    performanceMonitor PerformanceMonitor,
    resourceMonitor *ResourceMonitor,
    logger *slog.Logger,
) GracefulDegradationManager {
    return &gracefulDegradationManager{
        performanceMonitor: performanceMonitor,
        resourceMonitor:    resourceMonitor,
        // ...
    }
}

// AutoScaler использует ResourceMonitorInterface
func NewAutoScaler(
    performanceMonitor PerformanceMonitor,
    resourceMonitor ResourceMonitorInterface,
    logger *slog.Logger,
) AutoScaler {
    return &autoScaler{
        performanceMonitor: performanceMonitor,
        resourceMonitor:    resourceMonitor,
        // ...
    }
}

// Общий интерфейс PerformanceMonitor
type PerformanceMonitor interface {
    CollectMetrics() *PerformanceMetrics
    StartCollection(ctx context.Context, interval time.Duration) error
    StopCollection() error
    RegisterCollector(name string, collector MetricCollector) error
    GetPrometheusRegistry() *prometheus.Registry
}
```

## Ключевые архитектурные решения на уровне кода

### 1. **Интерфейсно-ориентированный дизайн**
Все основные компоненты определены как интерфейсы:
- Легкое тестирование с mock объектами
- Возможность замены реализации
- Слабая связанность между модулями

### 2. **Композиция над наследованием**
Go не поддерживает наследование, поэтому используется композиция:
```go
type gracefulDegradationManager struct {
    performanceMonitor PerformanceMonitor  // Композиция
    resourceMonitor    *ResourceMonitor    // Композиция
    // ...
}
```

### 3. **Потокобезопасность**
Все структуры используют мьютексы для безопасного доступа:
```go
type ResourceMonitor struct {
    mu sync.RWMutex  // Читатель-писатель мьютекс
    // ...
}

func (rm *ResourceMonitor) GetCurrentStatus() map[string]interface{} {
    rm.mu.RLock()         // Блокировка для чтения
    defer rm.mu.RUnlock() // Автоматическая разблокировка
    // ...
}
```

### 4. **Конфигурируемость через структуры**
Все настройки вынесены в отдельные структуры:
```go
type ResourceMonitorThresholds struct {
    CPUWarning        float64
    CPUCritical       float64
    MemoryWarning     float64
    MemoryCritical    float64
    // ...
}

type DegradationRule struct {
    ID          string
    Component   string
    Metric      string
    Condition   AlertCondition
    Threshold   interface{}
    Level       DegradationLevel
    Actions     []string
    // ...
}
```

### 5. **Обработка ошибок**
Все методы возвращают ошибки для правильной обработки:
```go
func (rm *ResourceMonitor) Start(ctx context.Context) error {
    if rm.isRunning {
        return fmt.Errorf("resource monitor is already running")
    }
    // ...
    return nil
}
```

### 6. **Контекстное управление**
Использование context.Context для управления жизненным циклом:
```go
func (rm *ResourceMonitor) monitorLoop(ctx context.Context) {
    ticker := time.NewTicker(rm.monitorInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return  // Graceful shutdown
        case <-rm.stopChan:
            return
        case <-ticker.C:
            rm.collectAndAnalyze()
        }
    }
}
```

Эта диаграмма кода показывает точное соответствие между архитектурным дизайном и реализацией в Go, демонстрируя, как высокоуровневые архитектурные решения воплощаются в конкретном коде.