# Task 6 Sequence Diagram - Подробное объяснение

## Обзор диаграммы
Диаграмма последовательности показывает временные взаимодействия между компонентами Task 6 в различных сценариях работы. Это динамическое представление архитектуры, которое демонстрирует, как компоненты взаимодействуют во времени.

## Связь с реализацией кода

### 🚀 **Initialization Phase (Фаза инициализации)**

#### Admin Configuration
**Архитектурное представление:**
```
Admin -> RM: Configure thresholds
Admin -> GD: Set degradation rules
Admin -> AS: Configure scaling rules
```

**Реализация в коде:**
```go
// Конфигурация порогов Resource Monitor
func main() {
    // Создание Resource Monitor
    collector := &ResourceCollector{}
    thresholds := ResourceMonitorThresholds{
        CPUWarning:        70.0,
        CPUCritical:       90.0,
        MemoryWarning:     0.8,
        MemoryCritical:    0.95,
        DiskWarning:       0.85,
        DiskCritical:      0.95,
        GoroutineWarning:  10000,
        GoroutineCritical: 50000,
    }
    
    resourceMonitor := NewResourceMonitor(collector, thresholds)
    
    // Конфигурация правил деградации
    degradationRules := []DegradationRule{
        {
            ID:        "cpu-overload-light",
            Component: "resource",
            Metric:    "cpu_usage",
            Condition: ConditionGreaterThan,
            Threshold: 75.0,
            Level:     DegradationLight,
            Actions:   []string{"reduce_worker_threads", "increase_batch_timeout"},
        },
        // ... другие правила
    }
    
    degradationManager := NewGracefulDegradationManager(performanceMonitor, resourceMonitor, logger)
    degradationManager.SetDegradationRules(degradationRules)
    
    // Конфигурация правил масштабирования
    scalingRules := []ScalingRule{
        {
            ID:          "worker-threads-scale-up",
            Component:   "worker_pool",
            Metric:      "cpu_usage",
            Condition:   ConditionGreaterThan,
            Threshold:   70.0,
            Direction:   ScaleUp,
            ScaleFactor: 1.5,
            MinValue:    64,
            MaxValue:    2048,
            Cooldown:    2 * time.Minute,
        },
        // ... другие правила
    }
    
    autoScaler := NewAutoScaler(performanceMonitor, resourceMonitor, logger)
    autoScaler.SetScalingRules(scalingRules)
}
```

#### Component Registration
**Архитектурное представление:**
```
RM -> PM: Register resource collector
GD -> PM: Register degradation monitor
AS -> PM: Register scaling monitor
AS -> SC: Register scalable components
```

**Реализация в коде:**
```go
// Регистрация коллекторов в PerformanceMonitor
func (pm *performanceMonitor) RegisterCollector(name string, collector MetricCollector) error {
    pm.mu.Lock()
    defer pm.mu.Unlock()
    
    pm.collectors[name] = collector
    return nil
}

// Инициализация системы
func initializeSystem() {
    // Создание PerformanceMonitor
    performanceMonitor := NewPerformanceMonitor()
    
    // Регистрация Resource Collector
    resourceCollector := &ResourceCollector{}
    performanceMonitor.RegisterCollector("resource", resourceCollector)
    
    // Создание и запуск компонентов
    resourceMonitor := NewResourceMonitor(resourceCollector, thresholds)
    degradationManager := NewGracefulDegradationManager(performanceMonitor, resourceMonitor, logger)
    autoScaler := NewAutoScaler(performanceMonitor, resourceMonitor, logger)
    
    // Регистрация масштабируемых компонентов
    workerPool := NewWorkerPoolComponent("worker_pool", 128, 64, 2048)
    connectionPool := NewConnectionPoolComponent("connection_pool", 1000, 500, 5000)
    batchProcessor := NewBatchProcessorComponent("batch_processor", 50, 10, 200)
    cache := NewCacheComponent("cache", 512, 256, 2048)
    networkBuffer := NewNetworkBufferComponent("network_buffer", 1024, 512, 4096)
    
    autoScaler.RegisterScalableComponent("worker_pool", workerPool)
    autoScaler.RegisterScalableComponent("connection_pool", connectionPool)
    autoScaler.RegisterScalableComponent("batch_processor", batchProcessor)
    autoScaler.RegisterScalableComponent("cache", cache)
    autoScaler.RegisterScalableComponent("network_buffer", networkBuffer)
}
```

### 🔄 **Normal Operation (Нормальная работа)**

#### Monitoring Loop
**Архитектурное представление:**
```
loop Every monitoring interval
    PM -> RM: Collect system metrics
    RM -> RM: Store in history
    RM -> RM: Analyze trends
    
    alt CPU usage > warning threshold
        RM -> Alert: Send warning alert
        Alert -> Prom: Export alert metric
    end
    
    PM -> GD: Provide performance metrics
    GD -> GD: Evaluate degradation rules
    
    PM -> AS: Provide performance metrics
    AS -> AS: Evaluate scaling rules
end
```

**Реализация в коде:**
```go
// Resource Monitor monitoring loop
func (rm *ResourceMonitor) monitorLoop(ctx context.Context) {
    ticker := time.NewTicker(rm.monitorInterval) // 30 секунд по умолчанию
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-rm.stopChan:
            return
        case <-ticker.C:
            rm.collectAndAnalyze() // Сбор и анализ метрик
        }
    }
}

// Сбор и анализ метрик
func (rm *ResourceMonitor) collectAndAnalyze() {
    now := time.Now()

    // Сбор текущих метрик
    metrics, err := rm.collector.CollectMetrics(context.Background())
    if err != nil {
        return
    }

    // Обновление исторических данных
    if cpuUsage, ok := metrics["cpu_usage"].(float64); ok {
        rm.cpuHistory.Add(now, cpuUsage) // Сохранение в истории
        rm.checkThreshold("cpu", cpuUsage, rm.thresholds.CPUWarning, rm.thresholds.CPUCritical, "%")
    }

    memoryPressure := rm.collector.GetMemoryPressure()
    rm.memoryHistory.Add(now, memoryPressure) // Сохранение в истории
    rm.checkThreshold("memory", memoryPressure*100, rm.thresholds.MemoryWarning*100, rm.thresholds.MemoryCritical*100, "%")

    // Аналогично для других метрик...
}

// Проверка порогов и отправка алертов
func (rm *ResourceMonitor) checkThreshold(metricType string, value, warningThreshold, criticalThreshold float64, unit string) {
    now := time.Now()

    // Проверка cooldown для предотвращения спама алертов
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
            rm.alertCallback(*alert) // Отправка в Alert System
        }

        rm.lastAlerts[metricType] = now
    }
}

// Graceful Degradation monitoring loop
func (gdm *gracefulDegradationManager) monitorLoop(ctx context.Context) {
    ticker := time.NewTicker(gdm.monitorInterval) // 10 секунд по умолчанию
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-gdm.stopChan:
            return
        case <-ticker.C:
            gdm.evaluateDegradation(ctx) // Оценка деградации
        }
    }
}

// Auto Scaler monitoring loop
func (as *autoScaler) monitorLoop(ctx context.Context) {
    ticker := time.NewTicker(as.monitorInterval) // 30 секунд по умолчанию
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-as.stopChan:
            return
        case <-ticker.C:
            as.evaluateScaling(ctx)           // Оценка масштабирования
            as.checkIsolatedComponents(ctx)   // Проверка изолированных компонентов
        }
    }
}
```

### 🔥 **High Load Scenario (Сценарий высокой нагрузки)**

#### Resource Alert and Degradation
**Архитектурное представление:**
```
PM -> RM: Collect metrics (CPU: 85%, Memory: 90%)
RM -> Alert: Send critical alert
Alert -> Admin: Notify high resource usage

PM -> GD: Performance metrics
GD -> GD: Evaluate: CPU > 80% → Light degradation
GD -> SC: Reduce worker threads (128 → 102)
GD -> SC: Increase batch timeout (10ms → 20ms)
GD -> Alert: Log degradation level change
```

**Реализация в коде:**
```go
// Сценарий высокой нагрузки в Resource Monitor
func (rm *ResourceMonitor) collectAndAnalyze() {
    // Сбор метрик показывает высокую нагрузку
    metrics := map[string]interface{}{
        "cpu_usage":       85.0,  // 85% CPU
        "memory_pressure": 0.90,  // 90% Memory
        "goroutine_count": 15000,
    }

    // Проверка CPU порога (критический = 90%, предупреждение = 70%)
    rm.checkThreshold("cpu", 85.0, 70.0, 90.0, "%")
    // Это вызовет критический алерт, так как 85% > 70%

    // Проверка Memory порога
    rm.checkThreshold("memory", 90.0, 80.0, 95.0, "%")
    // Это вызовет критический алерт, так как 90% > 80%
}

// Graceful Degradation оценивает правила
func (gdm *gracefulDegradationManager) evaluateDegradation(ctx context.Context) {
    metrics := gdm.performanceMonitor.CollectMetrics()
    
    // Оценка правила "cpu-overload-light"
    rule := DegradationRule{
        ID:        "cpu-overload-light",
        Component: "resource",
        Metric:    "cpu_usage",
        Condition: ConditionGreaterThan,
        Threshold: 75.0,  // CPU > 75%
        Level:     DegradationLight,
        Actions:   []string{"reduce_worker_threads", "increase_batch_timeout"},
    }

    if gdm.evaluateRule(rule, metrics) { // CPU 85% > 75% = true
        // Применение деградации
        gdm.applyDegradation(ctx, DegradationLight, []*DegradationRule{&rule}, metrics)
    }
}

// Применение действий деградации
func (gdm *gracefulDegradationManager) applyDegradation(ctx context.Context, level DegradationLevel, rules []*DegradationRule, metrics *PerformanceMetrics) {
    // Выполнение действий
    for _, actionName := range []string{"reduce_worker_threads", "increase_batch_timeout"} {
        if action, exists := gdm.actions[actionName]; exists {
            if err := action.Execute(ctx, level, metrics); err != nil {
                gdm.logger.Error("Failed to execute degradation action", 
                    slog.String("action", actionName))
            } else {
                gdm.activeActions[actionName] = action
            }
        }
    }

    // Логирование изменения уровня деградации
    gdm.logger.Warn("Degradation level changed",
        slog.String("from_level", "none"),
        slog.String("to_level", "light"),
        slog.Any("executed_actions", []string{"reduce_worker_threads", "increase_batch_timeout"}))
}

// Пример реализации действия уменьшения воркеров
type ReduceWorkerThreadsAction struct {
    isActive      bool
    originalScale int
    reducedScale  int
}

func (r *ReduceWorkerThreadsAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
    // Получение текущего масштаба воркеров (например, из WorkerPoolComponent)
    r.originalScale = 128  // Текущий масштаб
    
    // Уменьшение на 20% для Light деградации
    reductionFactor := 0.8
    r.reducedScale = int(float64(r.originalScale) * reductionFactor) // 128 * 0.8 = 102
    
    // Применение нового масштаба
    // (в реальной реализации это бы взаимодействовало с WorkerPoolComponent)
    r.isActive = true
    
    return nil
}
```

#### Auto Scaling Response
**Архитектурное представление:**
```
PM -> AS: Performance metrics
AS -> AS: Evaluate: CPU > 70% → Scale up workers
AS -> AS: Check cooldown period
AS -> SC: Scale worker pool (102 → 153)
AS -> Alert: Log scaling event
```

**Реализация в коде:**
```go
// Auto Scaler оценивает правила масштабирования
func (as *autoScaler) evaluateScaling(ctx context.Context) {
    metrics := as.performanceMonitor.CollectMetrics()
    
    // Оценка правила масштабирования вверх
    rule := ScalingRule{
        ID:          "worker-threads-scale-up",
        Component:   "worker_pool",
        Metric:      "cpu_usage",
        Condition:   ConditionGreaterThan,
        Threshold:   70.0,  // CPU > 70%
        Direction:   ScaleUp,
        ScaleFactor: 1.5,   // Увеличить в 1.5 раза
        MinValue:    64,
        MaxValue:    2048,
        Cooldown:    2 * time.Minute,
    }

    // Проверка изоляции и cooldown
    if !as.isComponentIsolated("worker_pool") && !as.isInCooldown("worker_pool", rule.Cooldown) {
        if as.evaluateScalingRule(rule, metrics) { // CPU 85% > 70% = true
            as.applyScaling(ctx, rule, metrics)
        }
    }
}

// Применение масштабирования
func (as *autoScaler) applyScaling(ctx context.Context, rule ScalingRule, metrics *PerformanceMetrics) {
    component := as.components["worker_pool"] // WorkerPoolComponent
    currentScale := component.GetCurrentScale() // 102 (после деградации)
    
    // Расчет нового масштаба
    newScale := int(math.Ceil(float64(currentScale) * rule.ScaleFactor))
    // newScale = ceil(102 * 1.5) = ceil(153) = 153
    
    // Применение ограничений
    if newScale > rule.MaxValue {
        newScale = rule.MaxValue // 2048
    }
    
    // Создание события масштабирования
    event := ScalingEvent{
        Timestamp:   time.Now(),
        Component:   "worker_pool",
        Direction:   ScaleUp,
        FromScale:   currentScale, // 102
        ToScale:     newScale,     // 153
        ScaleFactor: rule.ScaleFactor, // 1.5
        Trigger:     fmt.Sprintf("rule: %s", rule.ID),
        Metrics: map[string]interface{}{
            "cpu_usage":     metrics.ResourceMetrics.CPUUsage,
            "memory_usage":  metrics.ResourceMetrics.MemoryUsage,
            "response_time": metrics.BitswapMetrics.P95ResponseTime,
        },
    }

    // Применение масштабирования к компоненту
    if err := component.SetScale(ctx, newScale); err != nil {
        event.Success = false
        event.Error = err.Error()
    } else {
        event.Success = true
    }

    // Запись события и обновление времени последнего масштабирования
    as.addToHistory(event)
    as.lastScalingTime["worker_pool"] = time.Now()

    // Логирование
    as.logger.Info("Component scaled successfully",
        slog.String("component", "worker_pool"),
        slog.String("direction", "up"),
        slog.Int("from_scale", currentScale),
        slog.Int("to_scale", newScale))

    // Вызов callback если установлен
    if as.scalingCallback != nil {
        go as.scalingCallback(event)
    }
}
```

### 💾 **Memory Pressure Scenario (Сценарий нехватки памяти)**

**Архитектурное представление:**
```
PM -> RM: Collect metrics (Memory: 95%)
RM -> Alert: Send critical memory alert

PM -> GD: Performance metrics
GD -> GD: Evaluate: Memory > 95% → Critical degradation
GD -> SC: Emergency cache clear
GD -> SC: Disable non-essential services
GD -> SC: Reduce connection limits
GD -> Alert: Log critical degradation
```

**Реализация в коде:**
```go
// Критическая нехватка памяти
func (gdm *gracefulDegradationManager) evaluateDegradation(ctx context.Context) {
    metrics := gdm.performanceMonitor.CollectMetrics()
    
    // Правило критической деградации при нехватке памяти
    rule := DegradationRule{
        ID:        "memory-pressure-critical",
        Component: "resource",
        Metric:    "memory_usage_percent",
        Condition: ConditionGreaterThan,
        Threshold: 95.0,  // Memory > 95%
        Level:     DegradationCritical,
        Actions:   []string{"emergency_cache_clear", "disable_non_essential_services", "reduce_connection_limits"},
        Priority:  300,   // Высокий приоритет
    }

    if gdm.evaluateRule(rule, metrics) { // Memory 95% > 95% = true
        gdm.applyDegradation(ctx, DegradationCritical, []*DegradationRule{&rule}, metrics)
    }
}

// Реализация критических действий деградации
type EmergencyCacheClearAction struct {
    isActive bool
}

func (e *EmergencyCacheClearAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
    // Экстренная очистка всех кешей
    // В реальной реализации это бы взаимодействовало с CacheComponent
    e.isActive = true
    
    // Логирование критического действия
    slog.Error("Emergency cache clear executed due to critical memory pressure",
        slog.Float64("memory_usage_percent", 95.0),
        slog.String("degradation_level", "critical"))
    
    return nil
}

type DisableNonEssentialServicesAction struct {
    isActive         bool
    disabledServices []string
}

func (d *DisableNonEssentialServicesAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
    // Отключение неосновных сервисов
    d.disabledServices = []string{"metrics_export", "debug_endpoints", "optional_features"}
    d.isActive = true
    
    slog.Warn("Non-essential services disabled",
        slog.Any("disabled_services", d.disabledServices),
        slog.String("reason", "critical_memory_pressure"))
    
    return nil
}
```

### 🔧 **Component Failure Scenario (Сценарий отказа компонента)**

**Архитектурное представление:**
```
SC -> AS: Component health check failed
AS -> AS: Evaluate component health
AS -> AS: Isolate problematic component
AS -> SC: Scale component to minimum
AS -> Alert: Log component isolation
```

**Реализация в коде:**
```go
// Проверка здоровья компонентов в Auto Scaler
func (as *autoScaler) updateComponentHealth(metrics *PerformanceMetrics) {
    as.mu.Lock()
    defer as.mu.Unlock()

    for name, component := range as.components {
        health := &ComponentHealth{
            IsHealthy:    component.IsHealthy(), // Проверка здоровья
            CurrentScale: component.GetCurrentScale(),
            Metrics:      component.GetMetrics(),
        }

        // Если компонент нездоров, изолировать его
        if !health.IsHealthy {
            as.isolateUnhealthyComponent(name, "health_check_failed")
        }

        as.componentHealth[name] = health
    }
}

// Изоляция нездорового компонента
func (as *autoScaler) isolateUnhealthyComponent(componentName, reason string) {
    // Проверка, не изолирован ли уже компонент
    if _, isolated := as.isolatedComponents[componentName]; isolated {
        return
    }

    component := as.components[componentName]
    originalScale := component.GetCurrentScale()
    minScale, _ := component.GetScaleRange()

    // Создание записи об изоляции
    isolatedComponent := &IsolatedComponent{
        Name:          componentName,
        Reason:        reason,
        IsolatedAt:    time.Now(),
        OriginalScale: originalScale,
        IsolatedScale: minScale,
        AutoRestore:   true,
        RestoreAfter:  5 * time.Minute, // Автовосстановление через 5 минут
    }

    as.isolatedComponents[componentName] = isolatedComponent

    // Масштабирование до минимума
    ctx := context.Background()
    if err := component.SetScale(ctx, minScale); err != nil {
        as.logger.Error("Failed to isolate component",
            slog.String("component", componentName),
            slog.String("error", err.Error()))
        delete(as.isolatedComponents, componentName)
        return
    }

    as.logger.Warn("Component isolated",
        slog.String("component", componentName),
        slog.String("reason", reason),
        slog.Int("original_scale", originalScale),
        slog.Int("isolated_scale", minScale),
        slog.Duration("auto_restore_after", 5*time.Minute))
}

// Пример нездорового компонента
func (c *CacheComponent) IsHealthy() bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    // Кеш считается нездоровым, если hit rate < 50%
    return c.isHealthy && c.hitRate > 0.5
}
```

### 🔄 **Recovery Phase (Фаза восстановления)**

**Архитектурное представление:**
```
PM -> RM: Collect metrics (CPU: 45%, Memory: 60%)
RM -> Alert: Resource usage normalized

PM -> GD: Performance metrics
GD -> GD: Evaluate: Metrics stable for 60s
GD -> GD: Initiate recovery
GD -> SC: Restore worker threads (102 → 128)
GD -> SC: Restore batch timeout (20ms → 10ms)
GD -> Alert: Log recovery completion

AS -> AS: Check isolated components
AS -> AS: Auto-restore timeout reached
AS -> SC: Restore component scale
AS -> Alert: Log component restoration
```

**Реализация в коде:**
```go
// Graceful Degradation проверяет возможность восстановления
func (gdm *gracefulDegradationManager) evaluateDegradation(ctx context.Context) {
    metrics := gdm.performanceMonitor.CollectMetrics()
    
    // Проверка восстановления в первую очередь
    if gdm.currentLevel != DegradationNone {
        if gdm.shouldRecover(metrics) {
            gdm.attemptRecovery(ctx, metrics)
            return
        }
    }
    // ... остальная логика оценки деградации
}

// Проверка условий восстановления
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
        // CPU: 45% <= 70% ✓, Memory: 60% <= 75% ✓
        if !gdm.compareNumeric(currentValue, requiredValue, func(a, b float64) bool { return a <= b }) {
            return false
        }
    }

    // Проверка стабильности метрик (60 секунд для Light деградации)
    timeSinceLastChange := time.Since(gdm.lastLevelChange)
    return timeSinceLastChange >= threshold.StableMetricsDuration // 60s
}

// Попытка восстановления
func (gdm *gracefulDegradationManager) attemptRecovery(ctx context.Context, metrics *PerformanceMetrics) {
    threshold := gdm.recoveryThresholds[gdm.currentLevel]

    var targetLevel DegradationLevel
    if threshold.GradualRecovery {
        // Постепенное восстановление - уменьшение на один уровень
        targetLevel = gdm.currentLevel - 1
        if targetLevel < DegradationNone {
            targetLevel = DegradationNone
        }
    } else {
        // Полное восстановление
        targetLevel = DegradationNone
    }

    gdm.recoverToLevel(ctx, targetLevel)

    // Вызов callback восстановления
    if gdm.recoveryCallback != nil {
        eventType := RecoveryEventLevelReduced
        if targetLevel == DegradationNone {
            eventType = RecoveryEventFullRecovery
        }

        event := RecoveryEvent{
            Type:      eventType,
            FromLevel: gdm.currentLevel,
            ToLevel:   targetLevel,
            Timestamp: time.Now(),
            Duration:  time.Since(gdm.lastLevelChange),
            Metrics:   metrics,
        }

        go gdm.recoveryCallback(event)
    }
}

// Восстановление до указанного уровня
func (gdm *gracefulDegradationManager) recoverToLevel(ctx context.Context, targetLevel DegradationLevel) {
    previousLevel := gdm.currentLevel
    gdm.currentLevel = targetLevel
    gdm.lastLevelChange = time.Now()

    if targetLevel == DegradationNone {
        gdm.degradationStarted = nil
        gdm.recoveryProgress = 1.0
    }

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

    gdm.logger.Info("Recovery completed",
        slog.String("from_level", previousLevel.String()),
        slog.String("to_level", targetLevel.String()),
        slog.Any("recovered_actions", actionsToRecover))
}

// Auto Scaler проверяет изолированные компоненты
func (as *autoScaler) checkIsolatedComponents(ctx context.Context) {
    as.mu.Lock()
    componentsToRestore := make([]string, 0)

    for name, isolated := range as.isolatedComponents {
        // Проверка времени автовосстановления
        if isolated.AutoRestore && time.Since(isolated.IsolatedAt) >= isolated.RestoreAfter {
            componentsToRestore = append(componentsToRestore, name)
        }
    }
    as.mu.Unlock()

    // Восстановление компонентов вне блокировки
    for _, name := range componentsToRestore {
        if err := as.RestoreComponent(name); err != nil {
            as.logger.Error("Failed to auto-restore isolated component",
                slog.String("component", name),
                slog.String("error", err.Error()))
        } else {
            as.logger.Info("Component auto-restored from isolation",
                slog.String("component", name))
        }
    }
}
```

### 📊 **Monitoring and Reporting (Мониторинг и отчетность)**

**Архитектурное представление:**
```
loop Continuous
    RM -> Prom: Export resource metrics
    GD -> Prom: Export degradation metrics
    AS -> Prom: Export scaling metrics
    SC -> Prom: Export component metrics
end

Admin -> RM: Get resource history
RM -> Admin: Historical data with trends

Admin -> GD: Get degradation status
GD -> Admin: Current level and active actions

Admin -> AS: Get scaling status
AS -> Admin: Component scales and history
```

**Реализация в коде:**
```go
// Непрерывный экспорт метрик в Prometheus
type PerformanceMonitor interface {
    GetPrometheusRegistry() *prometheus.Registry
    CollectMetrics() *PerformanceMetrics
}

// Экспорт метрик Resource Monitor
func (rm *ResourceMonitor) GetCurrentStatus() map[string]interface{} {
    // Эти данные экспортируются в Prometheus
    summary := rm.collector.GetResourceSummary()
    
    // Добавление трендовой информации
    cpuDirection, cpuSlope := rm.cpuHistory.GetTrend()
    summary["cpu_trend"] = map[string]interface{}{
        "direction": cpuDirection,
        "slope":     cpuSlope,
    }
    
    return summary
}

// API для администраторов - получение истории ресурсов
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

// API для получения статуса деградации
func (gdm *gracefulDegradationManager) GetDegradationStatus() DegradationStatus {
    gdm.mu.RLock()
    defer gdm.mu.RUnlock()

    activeRules := make([]string, 0, len(gdm.activeRules))
    for ruleID := range gdm.activeRules {
        activeRules = append(activeRules, ruleID)
    }

    activeActions := make([]string, 0, len(gdm.activeActions))
    for actionName := range gdm.activeActions {
        activeActions = append(activeActions, actionName)
    }

    return DegradationStatus{
        Level:              gdm.currentLevel,
        ActiveRules:        activeRules,
        ActiveActions:      activeActions,
        DegradationStarted: gdm.degradationStarted,
        LastLevelChange:    gdm.lastLevelChange,
        RecoveryProgress:   gdm.recoveryProgress,
        Metrics:            gdm.performanceMonitor.CollectMetrics(),
        History:            gdm.eventHistory[len(gdm.eventHistory)-10:], // Последние 10 событий
    }
}

// API для получения статуса масштабирования
func (as *autoScaler) GetScalingStatus() ScalingStatus {
    as.mu.RLock()
    defer as.mu.RUnlock()

    activeRules := make([]string, 0)
    for _, rule := range as.rules {
        if rule.Enabled {
            activeRules = append(activeRules, rule.ID)
        }
    }

    componentScales := make(map[string]int)
    for name, component := range as.components {
        componentScales[name] = component.GetCurrentScale()
    }

    return ScalingStatus{
        IsActive:         as.isRunning,
        ActiveRules:      activeRules,
        ComponentScales:  componentScales,
        IsolatedCount:    len(as.isolatedComponents),
        LastScalingEvent: as.getLastScalingEvent(),
        ScalingHistory:   as.getRecentHistory(10),
        ComponentHealth:  as.getComponentHealthCopy(),
        Metrics:          as.performanceMonitor.CollectMetrics(),
    }
}
```

## Ключевые особенности временных взаимодействий

### 1. **Асинхронность**
Все компоненты работают в отдельных горутинах с собственными циклами мониторинга:
- Resource Monitor: 30 секунд
- Graceful Degradation: 10 секунд  
- Auto Scaler: 30 секунд

### 2. **Приоритизация**
- Восстановление проверяется перед деградацией
- Правила сортируются по приоритету
- Критические действия имеют высший приоритет

### 3. **Защита от спама**
- Cooldown периоды для алертов (5 минут)
- Cooldown периоды для масштабирования (2-5 минут)
- Стабилизационные периоды для восстановления (30-300 секунд)

### 4. **Graceful операции**
- Постепенное восстановление по уровням
- Автоматическое восстановление изолированных компонентов
- Откат действий деградации при восстановлении

Эта диаграмма последовательности демонстрирует, как Task 6 обеспечивает динамическое управление ресурсами через координированное взаимодействие всех компонентов в различных сценариях нагрузки.