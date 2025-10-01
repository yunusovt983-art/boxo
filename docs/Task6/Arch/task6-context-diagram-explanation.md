# Task 6 Context Diagram - Подробное объяснение

## Обзор диаграммы
Контекстная диаграмма (C4 Level 1) показывает Task 6 как единую систему в контексте внешних пользователей и систем. Это самый высокий уровень абстракции, который помогает понять место системы в общей экосистеме.

## Связь с реализацией кода

### 🎯 **Центральная система: Task 6 - Resource Management**
**Архитектурное представление:**
```
System(task6, "Task 6 - Resource Management", "Comprehensive resource monitoring, graceful degradation, and auto-scaling system")
```

**Реализация в коде:**
- **Файл:** `monitoring/resource_monitor.go` - основной модуль мониторинга ресурсов
- **Файл:** `monitoring/graceful_degradation.go` - управление деградацией сервиса
- **Файл:** `monitoring/auto_scaler.go` - автоматическое масштабирование
- **Файл:** `monitoring/scalable_components.go` - масштабируемые компоненты

### 👥 **Пользователи системы**

#### System Administrator
**Архитектурное представление:**
```
Person(admin, "System Administrator", "Monitors and configures resource management")
Rel(admin, task6, "Configures thresholds and rules")
```

**Реализация в коде:**
```go
// ResourceMonitor - конфигурация порогов
func (rm *ResourceMonitor) UpdateThresholds(thresholds ResourceMonitorThresholds) {
    rm.mu.Lock()
    defer rm.mu.Unlock()
    rm.thresholds = thresholds
}

// GracefulDegradationManager - настройка правил деградации
func (gdm *gracefulDegradationManager) SetDegradationRules(rules []DegradationRule) error {
    // Валидация и установка правил
    gdm.rules = rules
    return nil
}

// AutoScaler - конфигурация правил масштабирования
func (as *autoScaler) SetScalingRules(rules []ScalingRule) error {
    // Валидация и установка правил масштабирования
    as.rules = rules
    return nil
}
```

#### Developer
**Архитектурное представление:**
```
Person(developer, "Developer", "Integrates with monitoring APIs")
Rel(developer, task6, "Uses monitoring APIs")
```

**Реализация в коде:**
```go
// Публичные интерфейсы для разработчиков
type ResourceMonitor interface {
    GetCurrentStatus() map[string]interface{}
    GetResourceHistory(metricType string) *ResourceHistory
    GetHealthStatus() map[string]interface{}
}

type GracefulDegradationManager interface {
    GetCurrentDegradationLevel() DegradationLevel
    GetDegradationStatus() DegradationStatus
    ForceRecovery() error
}

type AutoScaler interface {
    GetScalingStatus() ScalingStatus
    ForceScale(component string, direction ScalingDirection, factor float64) error
    RegisterScalableComponent(name string, component ScalableComponent) error
}
```

### 🔗 **Внешние системы**

#### Prometheus
**Архитектурное представление:**
```
System_Ext(prometheus, "Prometheus", "Metrics collection and alerting")
Rel(task6, prometheus, "Exports metrics")
```

**Реализация в коде:**
```go
// PerformanceMonitor интерфейс для экспорта метрик
type PerformanceMonitor interface {
    GetPrometheusRegistry() *prometheus.Registry
    CollectMetrics() *PerformanceMetrics
}

// Пример экспорта метрик в ResourceMonitor
func (rm *ResourceMonitor) GetCurrentStatus() map[string]interface{} {
    // Сбор текущих метрик для экспорта в Prometheus
    summary := rm.collector.GetResourceSummary()
    // Добавление трендовой информации
    cpuDirection, cpuSlope := rm.cpuHistory.GetTrend()
    summary["cpu_trend"] = map[string]interface{}{
        "direction": cpuDirection,
        "slope":     cpuSlope,
    }
    return summary
}
```

#### AlertManager
**Архитектурное представление:**
```
System_Ext(alertmanager, "AlertManager", "Alert routing and notifications")
Rel(task6, alertmanager, "Sends alerts")
```

**Реализация в коде:**
```go
// ResourceAlert структура для отправки алертов
type ResourceAlert struct {
    Type       string              `json:"type"`
    Level      string              `json:"level"` // "warning" or "critical"
    Message    string              `json:"message"`
    Value      float64             `json:"value"`
    Threshold  float64             `json:"threshold"`
    Timestamp  time.Time           `json:"timestamp"`
    Prediction *ResourcePrediction `json:"prediction,omitempty"`
}

// Callback для отправки алертов
func (rm *ResourceMonitor) SetAlertCallback(callback func(ResourceAlert)) {
    rm.mu.Lock()
    defer rm.mu.Unlock()
    rm.alertCallback = callback
}

// Проверка порогов и отправка алертов
func (rm *ResourceMonitor) checkThreshold(metricType string, value, warningThreshold, criticalThreshold float64, unit string) {
    // Логика проверки порогов и создания алертов
    if alert != nil {
        if callback != nil {
            callback(*alert) // Отправка в AlertManager
        }
    }
}
```

#### Grafana
**Архитектурное представление:**
```
System_Ext(grafana, "Grafana", "Monitoring dashboards")
Rel(prometheus, grafana, "Provides data")
```

**Реализация в коде:**
Grafana получает данные через Prometheus, который собирает метрики из Task 6:
```go
// Метрики, доступные для визуализации в Grafana
type PerformanceMetrics struct {
    ResourceMetrics   ResourceStats   `json:"resource_metrics"`
    BitswapMetrics    BitswapStats    `json:"bitswap_metrics"`
    NetworkMetrics    NetworkStats    `json:"network_metrics"`
    Timestamp         time.Time       `json:"timestamp"`
}
```

#### External Systems
**Архитектурное представление:**
```
System_Ext(external_systems, "External Systems", "Third-party monitoring and logging")
Rel(task6, external_systems, "Integrates via callbacks")
```

**Реализация в коде:**
```go
// Callback интерфейсы для интеграции с внешними системами
type ScalingCallback func(event ScalingEvent)
type RecoveryCallback func(event RecoveryEvent)

// Регистрация callback'ов
func (as *autoScaler) SetScalingCallback(callback ScalingCallback) error {
    as.scalingCallback = callback
    return nil
}

func (gdm *gracefulDegradationManager) SetRecoveryCallback(callback RecoveryCallback) error {
    gdm.recoveryCallback = callback
    return nil
}
```

## Ключевые архитектурные решения

### 1. **Единая точка входа**
Task 6 представлен как единая система, но внутри содержит три основных модуля:
- Resource Monitor (мониторинг ресурсов)
- Graceful Degradation (управление деградацией)
- Auto Scaler (автоматическое масштабирование)

### 2. **Интеграция через стандартные протоколы**
- **Prometheus metrics** - стандартный формат для мониторинга
- **HTTP API** - для конфигурации и управления
- **Callback interfaces** - для интеграции с внешними системами

### 3. **Разделение ответственности**
- **Администраторы** - конфигурация и мониторинг
- **Разработчики** - интеграция и использование API
- **Внешние системы** - сбор метрик и уведомления

## Практическое применение

### Для администраторов:
```go
// Настройка порогов мониторинга
thresholds := ResourceMonitorThresholds{
    CPUWarning:    70.0,
    CPUCritical:   90.0,
    MemoryWarning: 0.8,
    MemoryCritical: 0.95,
}
resourceMonitor.UpdateThresholds(thresholds)

// Настройка правил деградации
rules := []DegradationRule{
    {
        ID: "cpu-overload-light",
        Metric: "cpu_usage",
        Threshold: 75.0,
        Level: DegradationLight,
        Actions: []string{"reduce_worker_threads"},
    },
}
degradationManager.SetDegradationRules(rules)
```

### Для разработчиков:
```go
// Получение статуса системы
status := resourceMonitor.GetCurrentStatus()
degradationLevel := degradationManager.GetCurrentDegradationLevel()
scalingStatus := autoScaler.GetScalingStatus()

// Регистрация собственного масштабируемого компонента
component := &MyCustomComponent{}
autoScaler.RegisterScalableComponent("my_component", component)
```

Эта контекстная диаграмма служит отправной точкой для понимания всей системы Task 6 и показывает, как она интегрируется в более широкую экосистему мониторинга и управления ресурсами.