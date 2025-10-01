# Task 6 Container Diagram - Подробное объяснение

## Обзор диаграммы
Диаграмма контейнеров (C4 Level 2) разбивает Task 6 на основные модули и показывает их взаимодействие с компонентами Boxo и внешними системами. Каждый контейнер представляет отдельный исполняемый модуль или сервис.

## Связь с реализацией кода

### 🏗️ **Основные контейнеры Task 6**

#### Resource Monitor
**Архитектурное представление:**
```
Container(resource_monitor, "Resource Monitor", "Go Module", "Monitors CPU, memory, disk usage with predictive analysis")
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
}

// Основные функции мониторинга
func (rm *ResourceMonitor) Start(ctx context.Context) error
func (rm *ResourceMonitor) collectAndAnalyze()
func (rm *ResourceMonitor) getPrediction(metricType string, currentValue, criticalThreshold float64) *ResourcePrediction
```

**Ключевые особенности реализации:**
- **Предиктивный анализ** через `ResourceHistory` и тренд-анализ
- **Настраиваемые пороги** через `ResourceMonitorThresholds`
- **Callback система** для интеграции с внешними системами

#### Graceful Degradation Manager
**Архитектурное представление:**
```
Container(graceful_degradation, "Graceful Degradation Manager", "Go Module", "Manages service quality reduction during overload")
```

**Реализация в коде:**
```go
// monitoring/graceful_degradation.go
type gracefulDegradationManager struct {
    mu                 sync.RWMutex
    performanceMonitor PerformanceMonitor
    resourceMonitor    *ResourceMonitor
    
    // Состояние деградации
    currentLevel       DegradationLevel
    degradationStarted *time.Time
    activeRules        map[string]*DegradationRule
    activeActions      map[string]DegradationAction
    
    // Конфигурация
    rules            []DegradationRule
    actions          map[string]DegradationAction
    recoveryCallback RecoveryCallback
}

// 5 уровней деградации
const (
    DegradationNone DegradationLevel = iota
    DegradationLight
    DegradationModerate
    DegradationSevere
    DegradationCritical
)

// 13 типов действий деградации
func (gdm *gracefulDegradationManager) registerDefaultActions() {
    gdm.actions["reduce_worker_threads"] = &ReduceWorkerThreadsAction{}
    gdm.actions["increase_batch_timeout"] = &IncreaseBatchTimeoutAction{}
    gdm.actions["clear_caches"] = &ClearCachesAction{}
    // ... и другие действия
}
```

#### Auto Scaler
**Архитектурное представление:**
```
Container(auto_scaler, "Auto Scaler", "Go Module", "Dynamically scales components based on load")
```

**Реализация в коде:**
```go
// monitoring/auto_scaler.go
type autoScaler struct {
    mu                 sync.RWMutex
    performanceMonitor PerformanceMonitor
    resourceMonitor    ResourceMonitorInterface
    
    // Управление масштабированием
    rules           []ScalingRule
    components      map[string]ScalableComponent
    lastScalingTime map[string]time.Time
    scalingHistory  []ScalingEvent
    
    // Изоляция компонентов
    isolatedComponents map[string]*IsolatedComponent
    isolationTimeout   time.Duration
}

// Направления масштабирования
type ScalingDirection int
const (
    ScaleUp ScalingDirection = iota
    ScaleDown
    ScaleStable
)
```

#### Scalable Components
**Архитектурное представление:**
```
Container(scalable_components, "Scalable Components", "Go Module", "Worker pools, connection pools, batch processors")
```

**Реализация в коде:**
```go
// monitoring/scalable_components.go
// 5 типов масштабируемых компонентов:

// 1. Worker Pool
type WorkerPoolComponent struct {
    currentScale   int
    minScale       int
    maxScale       int
    activeWorkers  int
    queuedTasks    int64
}

// 2. Connection Pool
type ConnectionPoolComponent struct {
    currentScale      int
    activeConnections int
    idleConnections   int
    totalRequests     int64
}

// 3. Batch Processor
type BatchProcessorComponent struct {
    currentScale     int
    activeBatches    int
    queuedBatches    int
    averageBatchSize int
}

// 4. Cache Component
type CacheComponent struct {
    currentScale  int // Cache size in MB
    hitRate       float64
    memoryUsage   int64
}

// 5. Network Buffer
type NetworkBufferComponent struct {
    currentScale      int // Buffer size in KB
    bufferUtilization float64
    throughput        float64
}

// Общий интерфейс для всех компонентов
type ScalableComponent interface {
    GetCurrentScale() int
    SetScale(ctx context.Context, scale int) error
    GetScaleRange() (min, max int)
    GetComponentName() string
    IsHealthy() bool
    GetMetrics() map[string]interface{}
}
```

### 🔄 **Взаимодействие между контейнерами**

#### Resource Monitor → Graceful Degradation
**Архитектурное представление:**
```
Rel(resource_monitor, graceful_degradation, "Triggers degradation")
```

**Реализация в коде:**
```go
// ResourceMonitor отправляет алерты, которые могут триггерить деградацию
func (rm *ResourceMonitor) checkThreshold(metricType string, value, warningThreshold, criticalThreshold float64, unit string) {
    if alert != nil {
        if callback != nil {
            callback(*alert) // Может триггерить деградацию
        }
    }
}

// GracefulDegradationManager использует ResourceMonitor для получения метрик
func (gdm *gracefulDegradationManager) evaluateDegradation(ctx context.Context) {
    metrics := gdm.performanceMonitor.CollectMetrics()
    // Оценка правил деградации на основе метрик
}
```

#### Resource Monitor → Auto Scaler
**Архитектурное представление:**
```
Rel(resource_monitor, auto_scaler, "Provides metrics")
```

**Реализация в коде:**
```go
// AutoScaler использует ResourceMonitorInterface для получения данных
type ResourceMonitorInterface interface {
    GetThresholds() ResourceMonitorThresholds
    GetCurrentStatus() map[string]interface{}
}

// AutoScaler оценивает правила масштабирования
func (as *autoScaler) evaluateScaling(ctx context.Context) {
    metrics := as.performanceMonitor.CollectMetrics()
    // Использование метрик для принятия решений о масштабировании
}
```

#### Auto Scaler → Scalable Components
**Архитектурное представление:**
```
Rel(auto_scaler, scalable_components, "Scales components")
```

**Реализация в коде:**
```go
// AutoScaler управляет масштабированием компонентов
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
    }
    
    // Применение масштабирования
    if err := component.SetScale(ctx, newScale); err != nil {
        // Обработка ошибки
    }
}

// Регистрация компонентов
func (as *autoScaler) registerDefaultComponents() {
    as.components["worker_pool"] = &WorkerPoolComponent{}
    as.components["connection_pool"] = &ConnectionPoolComponent{}
    as.components["batch_processor"] = &BatchProcessorComponent{}
}
```

### 🗄️ **Хранилища данных**

#### Metrics Store
**Архитектурное представление:**
```
ContainerDb(metrics_store, "Metrics Store", "In-Memory", "Historical performance data")
```

**Реализация в коде:**
```go
// ResourceHistory для хранения исторических данных
type ResourceHistory struct {
    Timestamps []time.Time `json:"timestamps"`
    Values     []float64   `json:"values"`
    MaxSize    int         `json:"max_size"`
}

// Добавление данных в историю
func (rh *ResourceHistory) Add(timestamp time.Time, value float64) {
    rh.Timestamps = append(rh.Timestamps, timestamp)
    rh.Values = append(rh.Values, value)
    
    // Ограничение размера истории
    if len(rh.Values) > rh.MaxSize {
        rh.Timestamps = rh.Timestamps[1:]
        rh.Values = rh.Values[1:]
    }
}

// Анализ трендов
func (rh *ResourceHistory) GetTrend() (direction string, slope float64) {
    // Простая линейная регрессия для расчета тренда
}
```

#### Configuration Store
**Архитектурное представление:**
```
ContainerDb(config_store, "Configuration Store", "File/Memory", "Thresholds, rules, and settings")
```

**Реализация в коде:**
```go
// Конфигурационные структуры
type ResourceMonitorThresholds struct {
    CPUWarning        float64
    CPUCritical       float64
    MemoryWarning     float64
    MemoryCritical    float64
    DiskWarning       float64
    DiskCritical      float64
    GoroutineWarning  int
    GoroutineCritical int
}

type DegradationRule struct {
    ID          string           `json:"id"`
    Component   string           `json:"component"`
    Metric      string           `json:"metric"`
    Condition   AlertCondition   `json:"condition"`
    Threshold   interface{}      `json:"threshold"`
    Level       DegradationLevel `json:"level"`
    Actions     []string         `json:"actions"`
}

type ScalingRule struct {
    ID          string           `json:"id"`
    Component   string           `json:"component"`
    Metric      string           `json:"metric"`
    Direction   ScalingDirection `json:"direction"`
    ScaleFactor float64          `json:"scale_factor"`
    MinValue    int              `json:"min_value"`
    MaxValue    int              `json:"max_value"`
    Cooldown    time.Duration    `json:"cooldown"`
}
```

### 🔗 **Интеграция с Boxo Core Systems**

#### Bitswap Integration
**Архитектурное представление:**
```
Rel(scalable_components, bitswap, "Scales bitswap workers")
```

**Реализация в коде:**
```go
// WorkerPoolComponent может масштабировать Bitswap воркеры
func (w *WorkerPoolComponent) SetScale(ctx context.Context, scale int) error {
    // Масштабирование воркеров для Bitswap
    if scale > oldScale {
        // Добавление воркеров
        w.activeWorkers = scale
    } else if scale < oldScale {
        // Удаление воркеров
        w.activeWorkers = scale
    }
    return nil
}
```

#### Network Layer Integration
**Архитектурное представление:**
```
Rel(scalable_components, network, "Scales network connections")
```

**Реализация в коде:**
```go
// ConnectionPoolComponent управляет сетевыми соединениями
func (c *ConnectionPoolComponent) SetScale(ctx context.Context, scale int) error {
    if scale > oldScale {
        // Добавление соединений
        additionalConnections := scale - oldScale
        c.idleConnections += additionalConnections
    } else if scale < oldScale {
        // Удаление соединений
        removedConnections := oldScale - scale
        if c.idleConnections >= removedConnections {
            c.idleConnections -= removedConnections
        }
    }
    return nil
}
```

### 📊 **Мониторинг и метрики**

#### Prometheus Integration
**Архитектурное представление:**
```
Rel(monitoring_api, prometheus, "Exports metrics")
```

**Реализация в коде:**
```go
// PerformanceMonitor интерфейс для экспорта метрик
type PerformanceMonitor interface {
    CollectMetrics() *PerformanceMetrics
    GetPrometheusRegistry() *prometheus.Registry
}

// Структура метрик для экспорта
type PerformanceMetrics struct {
    ResourceMetrics   ResourceStats   `json:"resource_metrics"`
    BitswapMetrics    BitswapStats    `json:"bitswap_metrics"`
    NetworkMetrics    NetworkStats    `json:"network_metrics"`
    Timestamp         time.Time       `json:"timestamp"`
}
```

## Ключевые архитектурные решения

### 1. **Модульная архитектура**
Каждый контейнер - это отдельный Go модуль с четко определенными интерфейсами:
- Легкое тестирование
- Независимое развитие
- Возможность замены реализации

### 2. **Асинхронное взаимодействие**
Использование каналов Go и callback'ов для неблокирующего взаимодействия:
```go
// Асинхронный мониторинг
func (rm *ResourceMonitor) monitorLoop(ctx context.Context) {
    ticker := time.NewTicker(rm.monitorInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-rm.stopChan:
            return
        case <-ticker.C:
            rm.collectAndAnalyze()
        }
    }
}
```

### 3. **Конфигурируемость**
Все пороги, правила и настройки вынесены в конфигурационные структуры:
- Легкая настройка под разные окружения
- Возможность динамического изменения конфигурации
- Валидация конфигурации

### 4. **Наблюдаемость**
Интеграция с стандартными инструментами мониторинга:
- Prometheus для метрик
- Структурированное логирование
- Трассировка событий

Эта диаграмма контейнеров показывает, как Task 6 разделен на логические модули и как они взаимодействуют друг с другом и с внешними системами, обеспечивая комплексное управление ресурсами в Boxo IPFS узле.