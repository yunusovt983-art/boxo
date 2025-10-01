# Task 6 Component Diagram - Подробное объяснение

## Обзор диаграммы
Диаграмма компонентов (C4 Level 3) детализирует внутреннюю структуру каждого контейнера Task 6, показывая основные компоненты и их взаимодействие. Это уровень, где архитектурные решения становятся видимыми в коде.

## Связь с реализацией кода

### 🔍 **Resource Monitor Container**

#### Resource Collector
**Архитектурное представление:**
```
Component(resource_collector, "Resource Collector", "Interface", "Collects system metrics (CPU, memory, disk)")
```

**Реализация в коде:**
```go
// monitoring/resource_monitor.go
type ResourceCollector struct {
    cpuUsage    float64
    memoryStats MemoryStats
    diskStats   DiskStats
}

// Интерфейс для сбора метрик
type MetricCollector interface {
    CollectMetrics() map[string]interface{}
    GetMetricNames() []string
}

// Основные методы сбора
func (rc *ResourceCollector) UpdateCPUUsage(usage float64)
func (rc *ResourceCollector) GetMemoryPressure() float64
func (rc *ResourceCollector) GetDiskPressure() float64
func (rc *ResourceCollector) ForceGC()
```

#### Resource History
**Архитектурное представление:**
```
Component(resource_history, "Resource History", "Component", "Stores and analyzes historical data")
```

**Реализация в коде:**
```go
// Хранение исторических данных
type ResourceHistory struct {
    Timestamps []time.Time `json:"timestamps"`
    Values     []float64   `json:"values"`
    MaxSize    int         `json:"max_size"`
}

// В ResourceMonitor
type ResourceMonitor struct {
    // Исторические данные для предиктивного анализа
    cpuHistory       *ResourceHistory
    memoryHistory    *ResourceHistory
    diskHistory      *ResourceHistory
    goroutineHistory *ResourceHistory
}

// Добавление данных в историю
func (rh *ResourceHistory) Add(timestamp time.Time, value float64) {
    rh.Timestamps = append(rh.Timestamps, timestamp)
    rh.Values = append(rh.Values, value)
    
    // Ограничение размера (последние 100 точек)
    if len(rh.Values) > rh.MaxSize {
        rh.Timestamps = rh.Timestamps[1:]
        rh.Values = rh.Values[1:]
    }
}
```

#### Predictive Analyzer
**Архитектурное представление:**
```
Component(predictive_analyzer, "Predictive Analyzer", "Component", "Analyzes trends and predicts resource usage")
```

**Реализация в коде:**
```go
// Анализ трендов с использованием линейной регрессии
func (rh *ResourceHistory) GetTrend() (direction string, slope float64) {
    if len(rh.Values) < 2 {
        return "stable", 0.0
    }
    
    // Простая линейная регрессия
    n := float64(len(rh.Values))
    sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
    
    for i, value := range rh.Values {
        x := float64(i)
        sumX += x
        sumY += value
        sumXY += x * value
        sumX2 += x * x
    }
    
    slope = (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
    
    if math.Abs(slope) < 0.001 {
        direction = "stable"
    } else if slope > 0 {
        direction = "increasing"
    } else {
        direction = "decreasing"
    }
    
    return direction, slope
}

// Предиктивный анализ
func (rm *ResourceMonitor) getPrediction(metricType string, currentValue, criticalThreshold float64) *ResourcePrediction {
    // Получение истории для метрики
    var history *ResourceHistory
    switch metricType {
    case "cpu":
        history = rm.cpuHistory
    case "memory":
        history = rm.memoryHistory
    // ...
    }
    
    direction, slope := history.GetTrend()
    
    prediction := &ResourcePrediction{
        TrendDirection: direction,
        Confidence:     rm.calculateConfidence(history),
    }
    
    // Расчет времени до критического порога
    if direction == "increasing" && slope > 0 {
        timeToThreshold := time.Duration((criticalThreshold-currentValue)/slope) * rm.monitorInterval
        prediction.TimeToThreshold = timeToThreshold
        
        if timeToThreshold < time.Hour {
            prediction.RecommendedAction = "Immediate action required"
        }
    }
    
    return prediction
}
```

#### Alert System
**Архитектурное представление:**
```
Component(alert_system, "Alert System", "Component", "Generates warnings and critical alerts")
```

**Реализация в коде:**
```go
// Структура алерта
type ResourceAlert struct {
    Type       string              `json:"type"`
    Level      string              `json:"level"` // "warning" or "critical"
    Message    string              `json:"message"`
    Value      float64             `json:"value"`
    Threshold  float64             `json:"threshold"`
    Timestamp  time.Time           `json:"timestamp"`
    Prediction *ResourcePrediction `json:"prediction,omitempty"`
}

// Проверка порогов и генерация алертов
func (rm *ResourceMonitor) checkThreshold(metricType string, value, warningThreshold, criticalThreshold float64, unit string) {
    now := time.Now()
    
    // Проверка cooldown для предотвращения спама
    if lastAlert, exists := rm.lastAlerts[metricType]; exists {
        if now.Sub(lastAlert) < rm.alertCooldown {
            return
        }
    }
    
    var alert *ResourceAlert
    
    if value >= criticalThreshold {
        alert = &ResourceAlert{
            Type:      metricType,
            Level:     "critical",
            Message:   fmt.Sprintf("%s usage is critical: %.2f%s", metricType, value, unit),
            Value:     value,
            Threshold: criticalThreshold,
            Timestamp: now,
        }
    } else if value >= warningThreshold {
        alert = &ResourceAlert{
            Type:      metricType,
            Level:     "warning",
            Message:   fmt.Sprintf("%s usage is high: %.2f%s", metricType, value, unit),
            Value:     value,
            Threshold: warningThreshold,
            Timestamp: now,
        }
    }
    
    if alert != nil {
        // Добавление предиктивного анализа
        alert.Prediction = rm.getPrediction(metricType, value, criticalThreshold)
        
        // Отправка алерта
        if rm.alertCallback != nil {
            rm.alertCallback(*alert)
        }
        
        rm.lastAlerts[metricType] = now
    }
}
```

### 🛡️ **Graceful Degradation Container**

#### Degradation Manager
**Архитектурное представление:**
```
Component(degradation_manager, "Degradation Manager", "Interface", "Manages degradation levels and recovery")
```

**Реализация в коде:**
```go
// Интерфейс управления деградацией
type GracefulDegradationManager interface {
    Start(ctx context.Context) error
    Stop() error
    SetDegradationRules(rules []DegradationRule) error
    GetCurrentDegradationLevel() DegradationLevel
    ForceRecovery() error
}

// Реализация менеджера
type gracefulDegradationManager struct {
    mu                 sync.RWMutex
    performanceMonitor PerformanceMonitor
    resourceMonitor    *ResourceMonitor
    
    // Состояние деградации
    currentLevel       DegradationLevel
    degradationStarted *time.Time
    lastLevelChange    time.Time
    activeRules        map[string]*DegradationRule
    activeActions      map[string]DegradationAction
}

// 5 уровней деградации
type DegradationLevel int
const (
    DegradationNone DegradationLevel = iota
    DegradationLight
    DegradationModerate
    DegradationSevere
    DegradationCritical
)
```

#### Degradation Rules Engine
**Архитектурное представление:**
```
Component(degradation_rules, "Degradation Rules Engine", "Component", "Evaluates conditions and triggers actions")
```

**Реализация в коде:**
```go
// Структура правила деградации
type DegradationRule struct {
    ID          string           `json:"id"`
    Name        string           `json:"name"`
    Component   string           `json:"component"`
    Metric      string           `json:"metric"`
    Condition   AlertCondition   `json:"condition"`
    Threshold   interface{}      `json:"threshold"`
    Level       DegradationLevel `json:"level"`
    Actions     []string         `json:"actions"`
    Priority    int              `json:"priority"`
    Enabled     bool             `json:"enabled"`
}

// Оценка правил деградации
func (gdm *gracefulDegradationManager) evaluateDegradation(ctx context.Context) {
    metrics := gdm.performanceMonitor.CollectMetrics()
    
    // Сортировка правил по приоритету
    sortedRules := make([]DegradationRule, len(gdm.rules))
    copy(sortedRules, gdm.rules)
    
    // Сортировка по приоритету (высший первый)
    for i := 0; i < len(sortedRules)-1; i++ {
        for j := i + 1; j < len(sortedRules); j++ {
            if sortedRules[i].Priority < sortedRules[j].Priority {
                sortedRules[i], sortedRules[j] = sortedRules[j], sortedRules[i]
            }
        }
    }
    
    // Оценка каждого правила
    for _, rule := range sortedRules {
        if !rule.Enabled {
            continue
        }
        
        if gdm.evaluateRule(rule, metrics) {
            // Правило сработало - применить деградацию
        }
    }
}

// Оценка отдельного правила
func (gdm *gracefulDegradationManager) evaluateRule(rule DegradationRule, metrics *PerformanceMetrics) bool {
    value := gdm.extractMetricValue(rule.Component, rule.Metric, metrics)
    if value == nil {
        return false
    }
    
    return gdm.compareValues(value, rule.Condition, rule.Threshold)
}
```

#### Degradation Actions
**Архитектурное представление:**
```
Component(degradation_actions, "Degradation Actions", "Component", "Executes 13 types of degradation actions")
```

**Реализация в коде:**
```go
// Интерфейс действия деградации
type DegradationAction interface {
    Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error
    Recover(ctx context.Context) error
    GetName() string
    IsActive() bool
}

// 13 типов действий деградации
func (gdm *gracefulDegradationManager) registerDefaultActions() {
    gdm.actions["reduce_worker_threads"] = &ReduceWorkerThreadsAction{}
    gdm.actions["increase_batch_timeout"] = &IncreaseBatchTimeoutAction{}
    gdm.actions["clear_caches"] = &ClearCachesAction{}
    gdm.actions["reduce_buffer_sizes"] = &ReduceBufferSizesAction{}
    gdm.actions["limit_concurrent_operations"] = &LimitConcurrentOperationsAction{}
    gdm.actions["disable_non_critical_features"] = &DisableNonCriticalFeaturesAction{}
    gdm.actions["reduce_connection_limits"] = &ReduceConnectionLimitsAction{}
    gdm.actions["emergency_cache_clear"] = &EmergencyCacheClearAction{}
    gdm.actions["disable_non_essential_services"] = &DisableNonEssentialServicesAction{}
    gdm.actions["increase_batch_size"] = &IncreaseBatchSizeAction{}
    gdm.actions["prioritize_critical_requests"] = &PrioritizeCriticalRequestsAction{}
    gdm.actions["enable_compression"] = &EnableCompressionAction{}
    gdm.actions["prioritize_critical_traffic"] = &PrioritizeCriticalTrafficAction{}
}

// Применение действий деградации
func (gdm *gracefulDegradationManager) applyDegradation(ctx context.Context, level DegradationLevel, rules []*DegradationRule, metrics *PerformanceMetrics) {
    // Сбор всех действий для выполнения
    actionsToExecute := make(map[string]bool)
    for _, rule := range rules {
        for _, actionName := range rule.Actions {
            actionsToExecute[actionName] = true
        }
    }
    
    // Выполнение действий деградации
    for actionName := range actionsToExecute {
        if action, exists := gdm.actions[actionName]; exists {
            if err := action.Execute(ctx, level, metrics); err != nil {
                gdm.logger.Error("Failed to execute degradation action", 
                    slog.String("action", actionName),
                    slog.String("error", err.Error()))
            } else {
                gdm.activeActions[actionName] = action
            }
        }
    }
}
```

#### Recovery Manager
**Архитектурное представление:**
```
Component(recovery_manager, "Recovery Manager", "Component", "Handles automatic and manual recovery")
```

**Реализация в коде:**
```go
// Пороги восстановления для каждого уровня деградации
type RecoveryThreshold struct {
    StableMetricsDuration time.Duration          `json:"stable_metrics_duration"`
    RequiredMetrics       map[string]interface{} `json:"required_metrics"`
    GradualRecovery       bool                   `json:"gradual_recovery"`
    RecoverySteps         int                    `json:"recovery_steps"`
}

// Проверка возможности восстановления
func (gdm *gracefulDegradationManager) shouldRecover(metrics *PerformanceMetrics) bool {
    threshold, exists := gdm.recoveryThresholds[gdm.currentLevel]
    if !exists {
        return false
    }
    
    // Проверка соответствия метрик требованиям восстановления
    for metricName, requiredValue := range threshold.RequiredMetrics {
        currentValue := gdm.extractMetricValue("resource", metricName, metrics)
        if currentValue == nil {
            continue
        }
        
        // Для восстановления метрики должны быть НИЖЕ порогов
        if !gdm.compareNumeric(currentValue, requiredValue, func(a, b float64) bool { return a <= b }) {
            return false
        }
    }
    
    // Проверка стабильности метрик
    timeSinceLastChange := time.Since(gdm.lastLevelChange)
    return timeSinceLastChange >= threshold.StableMetricsDuration
}

// Восстановление до указанного уровня
func (gdm *gracefulDegradationManager) recoverToLevel(ctx context.Context, targetLevel DegradationLevel) {
    previousLevel := gdm.currentLevel
    gdm.currentLevel = targetLevel
    gdm.lastLevelChange = time.Now()
    
    // Восстановление от действий
    actionsToRecover := make([]string, 0)
    for actionName, action := range gdm.activeActions {
        if err := action.Recover(ctx); err != nil {
            gdm.logger.Error("Failed to recover from degradation action",
                slog.String("action", actionName),
                slog.String("error", err.Error()))
        } else {
            actionsToRecover = append(actionsToRecover, actionName)
        }
    }
    
    // Очистка восстановленных действий
    for _, actionName := range actionsToRecover {
        delete(gdm.activeActions, actionName)
    }
}
```

### ⚖️ **Auto Scaler Container**

#### Scaling Engine
**Архитектурное представление:**
```
Component(scaling_engine, "Scaling Engine", "Interface", "Core scaling logic and rule evaluation")
```

**Реализация в коде:**
```go
// Интерфейс автомасштабирования
type AutoScaler interface {
    Start(ctx context.Context) error
    Stop() error
    SetScalingRules(rules []ScalingRule) error
    ForceScale(component string, direction ScalingDirection, factor float64) error
    RegisterScalableComponent(name string, component ScalableComponent) error
}

// Основная логика масштабирования
func (as *autoScaler) evaluateScaling(ctx context.Context) {
    metrics := as.performanceMonitor.CollectMetrics()
    
    // Обновление здоровья компонентов
    as.updateComponentHealth(metrics)
    
    // Сортировка правил по приоритету
    sortedRules := make([]ScalingRule, len(as.rules))
    copy(sortedRules, as.rules)
    
    // Оценка правил масштабирования
    for _, rule := range sortedRules {
        if !rule.Enabled {
            continue
        }
        
        // Проверка изоляции компонента
        if as.isComponentIsolated(rule.Component) {
            continue
        }
        
        // Проверка cooldown периода
        if as.isInCooldown(rule.Component, rule.Cooldown) {
            continue
        }
        
        if as.evaluateScalingRule(rule, metrics) {
            as.applyScaling(ctx, rule, metrics)
        }
    }
}
```

#### Component Registry
**Архитектурное представление:**
```
Component(component_registry, "Component Registry", "Component", "Registers and manages scalable components")
```

**Реализация в коде:**
```go
// Реестр масштабируемых компонентов
type autoScaler struct {
    components map[string]ScalableComponent
    // ...
}

// Регистрация компонента
func (as *autoScaler) RegisterScalableComponent(name string, component ScalableComponent) error {
    as.mu.Lock()
    defer as.mu.Unlock()
    
    as.components[name] = component
    
    as.logger.Info("Scalable component registered",
        slog.String("name", name),
        slog.String("component_type", component.GetComponentName()),
        slog.Int("current_scale", component.GetCurrentScale()))
    
    return nil
}

// Регистрация компонентов по умолчанию
func (as *autoScaler) registerDefaultComponents() {
    as.components["worker_pool"] = &WorkerPoolComponent{}
    as.components["connection_pool"] = &ConnectionPoolComponent{}
    as.components["batch_processor"] = &BatchProcessorComponent{}
}
```

#### Isolation Manager
**Архитектурное представление:**
```
Component(isolation_manager, "Isolation Manager", "Component", "Isolates and restores problematic components")
```

**Реализация в коде:**
```go
// Изолированный компонент
type IsolatedComponent struct {
    Name          string        `json:"name"`
    Reason        string        `json:"reason"`
    IsolatedAt    time.Time     `json:"isolated_at"`
    OriginalScale int           `json:"original_scale"`
    IsolatedScale int           `json:"isolated_scale"`
    AutoRestore   bool          `json:"auto_restore"`
    RestoreAfter  time.Duration `json:"restore_after"`
}

// Изоляция компонента
func (as *autoScaler) IsolateComponent(component string, reason string) error {
    comp, exists := as.components[component]
    if !exists {
        return fmt.Errorf("component %s not found", component)
    }
    
    originalScale := comp.GetCurrentScale()
    minScale, _ := comp.GetScaleRange()
    
    // Создание записи об изоляции
    isolatedComponent := &IsolatedComponent{
        Name:          component,
        Reason:        reason,
        IsolatedAt:    time.Now(),
        OriginalScale: originalScale,
        IsolatedScale: minScale,
        AutoRestore:   true,
        RestoreAfter:  as.isolationTimeout,
    }
    
    as.isolatedComponents[component] = isolatedComponent
    
    // Масштабирование до минимума
    if err := comp.SetScale(context.Background(), minScale); err != nil {
        delete(as.isolatedComponents, component)
        return fmt.Errorf("failed to isolate component %s: %w", component, err)
    }
    
    return nil
}

// Восстановление компонента
func (as *autoScaler) RestoreComponent(component string) error {
    isolated, exists := as.isolatedComponents[component]
    if !exists {
        return fmt.Errorf("component %s is not isolated", component)
    }
    
    comp, compExists := as.components[component]
    if !compExists {
        return fmt.Errorf("component %s not found", component)
    }
    
    delete(as.isolatedComponents, component)
    
    // Восстановление до исходного масштаба
    if err := comp.SetScale(context.Background(), isolated.OriginalScale); err != nil {
        // Возврат в изолированные при ошибке
        as.isolatedComponents[component] = isolated
        return fmt.Errorf("failed to restore component %s: %w", component, err)
    }
    
    return nil
}
```

### 🔧 **Scalable Components Container**

#### Scalable Component Interface
**Архитектурное представление:**
```
Component(scalable_interface, "Scalable Component Interface", "Interface", "Common interface for all scalable components")
```

**Реализация в коде:**
```go
// Общий интерфейс для всех масштабируемых компонентов
type ScalableComponent interface {
    GetCurrentScale() int
    SetScale(ctx context.Context, scale int) error
    GetScaleRange() (min, max int)
    GetComponentName() string
    IsHealthy() bool
    GetMetrics() map[string]interface{}
}
```

#### Worker Pool Component
**Архитектурное представление:**
```
Component(worker_pool, "Worker Pool Component", "Component", "Scalable worker thread management")
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
    
    // Метрики
    activeWorkers  int
    queuedTasks    int64
    processedTasks int64
    averageLatency time.Duration
}

func (w *WorkerPoolComponent) SetScale(ctx context.Context, scale int) error {
    w.mu.Lock()
    defer w.mu.Unlock()
    
    if scale < w.minScale || scale > w.maxScale {
        return fmt.Errorf("scale %d is outside valid range [%d, %d]", scale, w.minScale, w.maxScale)
    }
    
    oldScale := w.currentScale
    w.currentScale = scale
    
    // Симуляция масштабирования пула воркеров
    if scale > oldScale {
        // Масштабирование вверх - добавление воркеров
        w.activeWorkers = scale
    } else if scale < oldScale {
        // Масштабирование вниз - удаление воркеров
        w.activeWorkers = scale
    }
    
    return nil
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
```

#### Connection Pool Component
**Архитектурное представление:**
```
Component(connection_pool, "Connection Pool Component", "Component", "Scalable network connection management")
```

**Реализация в коде:**
```go
type ConnectionPoolComponent struct {
    mu           sync.RWMutex
    name         string
    currentScale int
    minScale     int
    maxScale     int
    
    // Метрики
    activeConnections int
    idleConnections   int
    totalRequests     int64
    failedRequests    int64
    averageLatency    time.Duration
}

func (c *ConnectionPoolComponent) SetScale(ctx context.Context, scale int) error {
    // Масштабирование пула соединений
    if scale > oldScale {
        // Добавление соединений
        additionalConnections := scale - oldScale
        c.idleConnections += additionalConnections
    } else if scale < oldScale {
        // Удаление соединений
        removedConnections := oldScale - scale
        if c.idleConnections >= removedConnections {
            c.idleConnections -= removedConnections
        } else {
            // Закрытие активных соединений
            c.activeConnections -= (removedConnections - c.idleConnections)
            c.idleConnections = 0
        }
    }
    
    return nil
}
```

### 🔄 **Взаимодействие компонентов**

#### Cross-container relationships
**Архитектурное представление:**
```
Rel(resource_api, performance_monitor, "Provides metrics")
Rel(degradation_manager, performance_monitor, "Monitors performance")
Rel(scaling_engine, performance_monitor, "Uses metrics")
```

**Реализация в коде:**
```go
// Общий интерфейс мониторинга производительности
type PerformanceMonitor interface {
    CollectMetrics() *PerformanceMetrics
    StartCollection(ctx context.Context, interval time.Duration) error
    StopCollection() error
    RegisterCollector(name string, collector MetricCollector) error
    GetPrometheusRegistry() *prometheus.Registry
}

// Структура метрик производительности
type PerformanceMetrics struct {
    ResourceMetrics   ResourceStats   `json:"resource_metrics"`
    BitswapMetrics    BitswapStats    `json:"bitswap_metrics"`
    NetworkMetrics    NetworkStats    `json:"network_metrics"`
    Timestamp         time.Time       `json:"timestamp"`
}

// Использование в разных компонентах
func (gdm *gracefulDegradationManager) evaluateDegradation(ctx context.Context) {
    metrics := gdm.performanceMonitor.CollectMetrics()
    // Использование метрик для оценки деградации
}

func (as *autoScaler) evaluateScaling(ctx context.Context) {
    metrics := as.performanceMonitor.CollectMetrics()
    // Использование метрик для принятия решений о масштабировании
}
```

## Ключевые архитектурные решения

### 1. **Разделение ответственности**
Каждый компонент имеет четко определенную ответственность:
- **Resource Collector** - только сбор метрик
- **Predictive Analyzer** - только анализ трендов
- **Alert System** - только генерация алертов
- **Degradation Rules Engine** - только оценка правил

### 2. **Интерфейсно-ориентированный дизайн**
Все взаимодействия происходят через интерфейсы:
- Легкое тестирование с mock объектами
- Возможность замены реализации
- Слабая связанность компонентов

### 3. **Конфигурируемость и расширяемость**
- Правила деградации и масштабирования настраиваются извне
- Новые действия деградации можно добавлять через интерфейс
- Новые масштабируемые компоненты регистрируются динамически

### 4. **Наблюдаемость**
- Все компоненты предоставляют метрики
- Структурированное логирование с контекстом
- История событий для анализа

Эта диаграмма компонентов показывает детальную внутреннюю структуру Task 6 и демонстрирует, как архитектурные решения реализованы в коде, обеспечивая модульность, расширяемость и наблюдаемость системы.