# Task 7.3 Fault Tolerance Component Diagram - Подробное объяснение

## Обзор диаграммы

**Файл**: `task7_c4_component_fault_tolerance.puml`  
**Тип**: C4 Component Diagram  
**Назначение**: Детализирует внутреннюю структуру системы тестирования отказоустойчивости

## Архитектурный дизайн → Реализация кода

### 🌪️ Ядро Chaos Engineering

#### ChaosEngine
**Архитектурная роль**: Главный движок chaos engineering

**Реализация в коде**:
```go
// fault_tolerance/chaos_engine.go
type ChaosEngine struct {
    orchestrator     *ChaosOrchestrator
    faultInjector    *FaultInjector
    scheduler        *ChaosScheduler
    safetyController *SafetyController
    
    // Управление экспериментами
    activeExperiments map[string]*ChaosExperiment
    experimentHistory []*ExperimentResult
    
    // Безопасность
    blastRadiusLimit  float64
    emergencyStop     chan struct{}
    
    // Синхронизация
    mu                sync.RWMutex
    running           bool
}

func NewChaosEngine(config *ChaosConfig) *ChaosEngine {
    return &ChaosEngine{
        orchestrator:      NewChaosOrchestrator(),
        faultInjector:     NewFaultInjector(),
        scheduler:         NewChaosScheduler(),
        safetyController:  NewSafetyController(),
        activeExperiments: make(map[string]*ChaosExperiment),
        experimentHistory: make([]*ExperimentResult, 0),
        blastRadiusLimit:  config.MaxBlastRadius,
        emergencyStop:     make(chan struct{}),
    }
}

func (ce *ChaosEngine) RunExperiment(experiment *ChaosExperiment) (*ExperimentResult, error) {
    ce.mu.Lock()
    if ce.running {
        ce.mu.Unlock()
        return nil, errors.New("another experiment is already running")
    }
    ce.running = true
    ce.mu.Unlock()
    
    defer func() {
        ce.mu.Lock()
        ce.running = false
        delete(ce.activeExperiments, experiment.ID)
        ce.mu.Unlock()
    }()
    
    // Проверка безопасности эксперимента
    if err := ce.safetyController.ValidateExperiment(experiment); err != nil {
        return nil, fmt.Errorf("experiment safety validation failed: %w", err)
    }
    
    // Проверка blast radius
    if experiment.BlastRadius > ce.blastRadiusLimit {
        return nil, fmt.Errorf("experiment blast radius %.2f exceeds limit %.2f", 
            experiment.BlastRadius, ce.blastRadiusLimit)
    }
    
    result := &ExperimentResult{
        ExperimentID: experiment.ID,
        StartTime:    time.Now(),
        Hypothesis:   experiment.Hypothesis,
    }
    
    ce.activeExperiments[experiment.ID] = experiment
    
    // Выполнение эксперимента через оркестратор
    if err := ce.orchestrator.ExecuteExperiment(experiment, result); err != nil {
        result.Success = false
        result.Error = err
        return result, err
    }
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    result.Success = true
    
    ce.experimentHistory = append(ce.experimentHistory, result)
    return result, nil
}

func (ce *ChaosEngine) EmergencyStop() error {
    ce.mu.Lock()
    defer ce.mu.Unlock()
    
    // Отправка сигнала экстренной остановки
    select {
    case ce.emergencyStop <- struct{}{}:
    default:
    }
    
    // Остановка всех активных экспериментов
    for _, experiment := range ce.activeExperiments {
        if err := ce.orchestrator.StopExperiment(experiment); err != nil {
            log.Printf("Failed to stop experiment %s: %v", experiment.ID, err)
        }
    }
    
    // Восстановление всех инжектированных сбоев
    return ce.faultInjector.RestoreAllFaults()
}
```

#### FaultInjector
**Архитектурная роль**: Инжектор сбоев в систему

**Реализация в коде**:
```go
// fault_tolerance/fault_injector.go
type FaultInjector struct {
    simulators       map[FaultType]FaultSimulator
    activeFaults     map[string]*InjectedFault
    faultHistory     []*FaultRecord
    
    // Управление состоянием
    mu               sync.RWMutex
    injectionActive  bool
}

func NewFaultInjector() *FaultInjector {
    return &FaultInjector{
        simulators: map[FaultType]FaultSimulator{
            NetworkFaultType:    NewNetworkFailureSimulator(),
            NodeFaultType:       NewNodeFailureSimulator(),
            DiskFaultType:       NewDiskFailureSimulator(),
            MemoryFaultType:     NewMemoryPressureSimulator(),
            CPUFaultType:        NewCPUStressSimulator(),
            PartitionFaultType:  NewNetworkPartitionSimulator(),
        },
        activeFaults: make(map[string]*InjectedFault),
        faultHistory: make([]*FaultRecord, 0),
    }
}

func (fi *FaultInjector) InjectFault(faultSpec *FaultSpec) (*InjectedFault, error) {
    fi.mu.Lock()
    defer fi.mu.Unlock()
    
    simulator, exists := fi.simulators[faultSpec.Type]
    if !exists {
        return nil, fmt.Errorf("no simulator found for fault type: %v", faultSpec.Type)
    }
    
    // Создание записи об инжектированном сбое
    fault := &InjectedFault{
        ID:        generateFaultID(),
        Spec:      faultSpec,
        StartTime: time.Now(),
        Status:    FaultStatusActive,
    }
    
    // Инжекция сбоя через соответствующий симулятор
    if err := simulator.InjectFault(faultSpec); err != nil {
        return nil, fmt.Errorf("fault injection failed: %w", err)
    }
    
    fi.activeFaults[fault.ID] = fault
    
    // Планирование автоматического восстановления
    if faultSpec.Duration > 0 {
        go fi.scheduleRecovery(fault.ID, faultSpec.Duration)
    }
    
    // Запись в историю
    record := &FaultRecord{
        FaultID:     fault.ID,
        Type:        faultSpec.Type,
        Target:      faultSpec.Target,
        InjectedAt:  fault.StartTime,
        Duration:    faultSpec.Duration,
    }
    fi.faultHistory = append(fi.faultHistory, record)
    
    return fault, nil
}

func (fi *FaultInjector) RecoverFault(faultID string) error {
    fi.mu.Lock()
    defer fi.mu.Unlock()
    
    fault, exists := fi.activeFaults[faultID]
    if !exists {
        return fmt.Errorf("fault %s not found or already recovered", faultID)
    }
    
    simulator := fi.simulators[fault.Spec.Type]
    if err := simulator.RecoverFault(fault.Spec); err != nil {
        return fmt.Errorf("fault recovery failed: %w", err)
    }
    
    fault.EndTime = time.Now()
    fault.Duration = fault.EndTime.Sub(fault.StartTime)
    fault.Status = FaultStatusRecovered
    
    delete(fi.activeFaults, faultID)
    
    return nil
}
```

### 💥 Симуляция сбоев

#### NetworkFailureSimulator
**Архитектурная роль**: Симулятор сбоев сети

**Реализация в коде**:
```go
// fault_tolerance/network_failure_simulator.go
type NetworkFailureSimulator struct {
    networkController *NetworkController
    trafficShaper     *TrafficShaper
    firewallManager   *FirewallManager
    
    // Активные сетевые сбои
    activeFailures    map[string]*NetworkFailure
    mu                sync.RWMutex
}

func (nfs *NetworkFailureSimulator) InjectFault(faultSpec *FaultSpec) error {
    switch faultSpec.NetworkFaultType {
    case NetworkPartition:
        return nfs.createNetworkPartition(faultSpec)
    case PacketLoss:
        return nfs.injectPacketLoss(faultSpec)
    case LatencyInjection:
        return nfs.injectLatency(faultSpec)
    case BandwidthLimitation:
        return nfs.limitBandwidth(faultSpec)
    case ConnectionDrop:
        return nfs.dropConnections(faultSpec)
    default:
        return fmt.Errorf("unknown network fault type: %v", faultSpec.NetworkFaultType)
    }
}

func (nfs *NetworkFailureSimulator) createNetworkPartition(faultSpec *FaultSpec) error {
    partition := &NetworkPartition{
        ID:       faultSpec.ID,
        GroupA:   faultSpec.PartitionGroups[0],
        GroupB:   faultSpec.PartitionGroups[1],
        Duration: faultSpec.Duration,
    }
    
    // Блокировка трафика между группами
    for _, nodeA := range partition.GroupA {
        for _, nodeB := range partition.GroupB {
            rule := &FirewallRule{
                Source:      nodeA,
                Destination: nodeB,
                Action:      FirewallActionDrop,
                Direction:   FirewallDirectionBoth,
            }
            
            if err := nfs.firewallManager.AddRule(rule); err != nil {
                return fmt.Errorf("failed to add firewall rule: %w", err)
            }
        }
    }
    
    nfs.mu.Lock()
    nfs.activeFailures[partition.ID] = &NetworkFailure{
        Type:      NetworkPartition,
        Partition: partition,
        StartTime: time.Now(),
    }
    nfs.mu.Unlock()
    
    log.Printf("Network partition created: %v <-> %v", partition.GroupA, partition.GroupB)
    return nil
}

func (nfs *NetworkFailureSimulator) injectLatency(faultSpec *FaultSpec) error {
    latencyConfig := &LatencyConfig{
        Target:    faultSpec.Target,
        Latency:   faultSpec.LatencyAmount,
        Jitter:    faultSpec.LatencyJitter,
        Duration:  faultSpec.Duration,
    }
    
    if err := nfs.trafficShaper.AddLatency(latencyConfig); err != nil {
        return fmt.Errorf("failed to inject latency: %w", err)
    }
    
    nfs.mu.Lock()
    nfs.activeFailures[faultSpec.ID] = &NetworkFailure{
        Type:          LatencyInjection,
        LatencyConfig: latencyConfig,
        StartTime:     time.Now(),
    }
    nfs.mu.Unlock()
    
    log.Printf("Latency injection started: %v latency on %s", latencyConfig.Latency, latencyConfig.Target)
    return nil
}
```

#### NodeFailureSimulator
**Архитектурная роль**: Симулятор сбоев узлов

**Реализация в коде**:
```go
// fault_tolerance/node_failure_simulator.go
type NodeFailureSimulator struct {
    nodeManager       *NodeManager
    processManager    *ProcessManager
    resourceController *ResourceController
    
    // Активные сбои узлов
    activeNodeFailures map[string]*NodeFailure
    mu                 sync.RWMutex
}

func (nfs *NodeFailureSimulator) InjectFault(faultSpec *FaultSpec) error {
    switch faultSpec.NodeFaultType {
    case NodeCrash:
        return nfs.crashNode(faultSpec)
    case ProcessKill:
        return nfs.killProcess(faultSpec)
    case ResourceExhaustion:
        return nfs.exhaustResources(faultSpec)
    case NodeFreeze:
        return nfs.freezeNode(faultSpec)
    default:
        return fmt.Errorf("unknown node fault type: %v", faultSpec.NodeFaultType)
    }
}

func (nfs *NodeFailureSimulator) crashNode(faultSpec *FaultSpec) error {
    nodeID := faultSpec.Target
    
    // Получение информации об узле
    node, err := nfs.nodeManager.GetNode(nodeID)
    if err != nil {
        return fmt.Errorf("failed to get node %s: %w", nodeID, err)
    }
    
    // Сохранение состояния узла для восстановления
    nodeState, err := nfs.nodeManager.CaptureNodeState(node)
    if err != nil {
        return fmt.Errorf("failed to capture node state: %w", err)
    }
    
    // Остановка всех процессов на узле
    if err := nfs.processManager.StopAllProcesses(node); err != nil {
        return fmt.Errorf("failed to stop processes on node %s: %w", nodeID, err)
    }
    
    // Симуляция краха узла
    if err := nfs.nodeManager.SimulateCrash(node); err != nil {
        return fmt.Errorf("failed to simulate node crash: %w", err)
    }
    
    nfs.mu.Lock()
    nfs.activeNodeFailures[faultSpec.ID] = &NodeFailure{
        Type:         NodeCrash,
        NodeID:       nodeID,
        StartTime:    time.Now(),
        SavedState:   nodeState,
        RecoveryPlan: nfs.createRecoveryPlan(node, nodeState),
    }
    nfs.mu.Unlock()
    
    log.Printf("Node %s crashed", nodeID)
    return nil
}

func (nfs *NodeFailureSimulator) RecoverFault(faultSpec *FaultSpec) error {
    nfs.mu.Lock()
    failure, exists := nfs.activeNodeFailures[faultSpec.ID]
    nfs.mu.Unlock()
    
    if !exists {
        return fmt.Errorf("node failure %s not found", faultSpec.ID)
    }
    
    switch failure.Type {
    case NodeCrash:
        return nfs.recoverFromCrash(failure)
    case ProcessKill:
        return nfs.restartProcess(failure)
    case ResourceExhaustion:
        return nfs.releaseResources(failure)
    case NodeFreeze:
        return nfs.unfreezeNode(failure)
    default:
        return fmt.Errorf("unknown failure type: %v", failure.Type)
    }
}
```

### 🔄 Тестирование восстановления

#### RecoveryTestEngine
**Архитектурная роль**: Движок тестов восстановления

**Реализация в коде**:
```go
// fault_tolerance/recovery_test_engine.go
type RecoveryTestEngine struct {
    recoveryStrategies map[RecoveryType]*RecoveryStrategy
    healthChecker      *HealthChecker
    recoveryTracker    *RecoveryTracker
    slaMonitor         *SLAMonitor
}

func (rte *RecoveryTestEngine) TestRecovery(faultType FaultType, recoveryType RecoveryType) (*RecoveryTestResult, error) {
    result := &RecoveryTestResult{
        FaultType:    faultType,
        RecoveryType: recoveryType,
        StartTime:    time.Now(),
    }
    
    // Получение базовой линии здоровья системы
    baselineHealth, err := rte.healthChecker.GetSystemHealth()
    if err != nil {
        return nil, fmt.Errorf("failed to get baseline health: %w", err)
    }
    result.BaselineHealth = baselineHealth
    
    // Инжекция сбоя
    faultSpec := &FaultSpec{
        Type:     faultType,
        Duration: 2 * time.Minute, // Фиксированная длительность для тестов
    }
    
    injectedFault, err := rte.injectTestFault(faultSpec)
    if err != nil {
        return nil, fmt.Errorf("fault injection failed: %w", err)
    }
    result.InjectedFault = injectedFault
    
    // Мониторинг деградации системы
    degradationMetrics, err := rte.monitorDegradation(faultSpec.Duration)
    if err != nil {
        return nil, fmt.Errorf("degradation monitoring failed: %w", err)
    }
    result.DegradationMetrics = degradationMetrics
    
    // Запуск процесса восстановления
    recoveryStartTime := time.Now()
    strategy := rte.recoveryStrategies[recoveryType]
    
    recoveryResult, err := strategy.ExecuteRecovery(injectedFault)
    if err != nil {
        return nil, fmt.Errorf("recovery execution failed: %w", err)
    }
    
    result.RecoveryDuration = time.Since(recoveryStartTime)
    result.RecoverySteps = recoveryResult.Steps
    
    // Валидация восстановления
    recoveredHealth, err := rte.healthChecker.GetSystemHealth()
    if err != nil {
        return nil, fmt.Errorf("failed to get recovered health: %w", err)
    }
    result.RecoveredHealth = recoveredHealth
    
    // Анализ качества восстановления
    result.RecoveryQuality = rte.analyzeRecoveryQuality(baselineHealth, recoveredHealth)
    result.SLACompliance = rte.slaMonitor.CheckCompliance(result.RecoveryDuration)
    
    result.EndTime = time.Now()
    result.Success = result.RecoveryQuality.Score >= 0.8 && result.SLACompliance
    
    return result, nil
}
```

### ⚡ Circuit Breaker тестирование

#### CircuitBreakerValidator
**Архитектурная роль**: Валидатор circuit breaker логики

**Реализация в коде**:
```go
// fault_tolerance/circuit_breaker_validator.go
type CircuitBreakerValidator struct {
    circuitBreakers   map[string]*CircuitBreaker
    thresholdTester   *ThresholdTester
    stateTracker      *StateTracker
    transitionLogger  *TransitionLogger
}

func (cbv *CircuitBreakerValidator) ValidateCircuitBreaker(cbName string) (*CircuitBreakerValidationResult, error) {
    cb, exists := cbv.circuitBreakers[cbName]
    if !exists {
        return nil, fmt.Errorf("circuit breaker %s not found", cbName)
    }
    
    result := &CircuitBreakerValidationResult{
        CircuitBreakerName: cbName,
        StartTime:          time.Now(),
        TestResults:        make(map[string]*TestResult),
    }
    
    // Тест 1: Валидация порогов срабатывания
    thresholdResult, err := cbv.testThresholds(cb)
    if err != nil {
        return nil, fmt.Errorf("threshold test failed: %w", err)
    }
    result.TestResults["thresholds"] = thresholdResult
    
    // Тест 2: Валидация переходов состояний
    stateTransitionResult, err := cbv.testStateTransitions(cb)
    if err != nil {
        return nil, fmt.Errorf("state transition test failed: %w", err)
    }
    result.TestResults["state_transitions"] = stateTransitionResult
    
    // Тест 3: Валидация окна восстановления
    recoveryWindowResult, err := cbv.testRecoveryWindow(cb)
    if err != nil {
        return nil, fmt.Errorf("recovery window test failed: %w", err)
    }
    result.TestResults["recovery_window"] = recoveryWindowResult
    
    // Тест 4: Тест под нагрузкой
    loadTestResult, err := cbv.testUnderLoad(cb)
    if err != nil {
        return nil, fmt.Errorf("load test failed: %w", err)
    }
    result.TestResults["load_test"] = loadTestResult
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    result.OverallSuccess = cbv.calculateOverallSuccess(result.TestResults)
    
    return result, nil
}

func (cbv *CircuitBreakerValidator) testStateTransitions(cb *CircuitBreaker) (*TestResult, error) {
    result := &TestResult{
        TestName:  "StateTransitions",
        StartTime: time.Now(),
    }
    
    // Начальное состояние должно быть Closed
    if cb.GetState() != CircuitBreakerStateClosed {
        return nil, fmt.Errorf("initial state should be Closed, got %v", cb.GetState())
    }
    
    // Симуляция ошибок для перехода в Open состояние
    errorCount := 0
    for i := 0; i < cb.GetFailureThreshold()+1; i++ {
        err := cb.Call(func() error {
            return errors.New("simulated error")
        })
        if err != nil {
            errorCount++
        }
    }
    
    // Проверка перехода в Open состояние
    if cb.GetState() != CircuitBreakerStateOpen {
        return nil, fmt.Errorf("expected Open state after %d failures, got %v", 
            errorCount, cb.GetState())
    }
    
    // Ожидание перехода в Half-Open состояние
    time.Sleep(cb.GetTimeout() + 100*time.Millisecond)
    
    // Попытка вызова для перехода в Half-Open
    cb.Call(func() error {
        return nil // Успешный вызов
    })
    
    if cb.GetState() != CircuitBreakerStateHalfOpen {
        return nil, fmt.Errorf("expected Half-Open state, got %v", cb.GetState())
    }
    
    // Несколько успешных вызовов для перехода обратно в Closed
    for i := 0; i < cb.GetSuccessThreshold(); i++ {
        cb.Call(func() error {
            return nil
        })
    }
    
    if cb.GetState() != CircuitBreakerStateClosed {
        return nil, fmt.Errorf("expected Closed state after successful calls, got %v", cb.GetState())
    }
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    result.Success = true
    
    return result, nil
}
```

### 🛡️ Паттерны устойчивости

#### RetryPatternTest
**Архитектурная роль**: Тесты паттерна Retry

**Реализация в коде**:
```go
// fault_tolerance/retry_pattern_test.go
type RetryPatternTest struct {
    retryPolicies    map[string]*RetryPolicy
    backoffStrategies map[string]BackoffStrategy
    failureSimulator *FailureSimulator
}

func (rpt *RetryPatternTest) TestRetryPattern(policyName string) (*RetryTestResult, error) {
    policy, exists := rpt.retryPolicies[policyName]
    if !exists {
        return nil, fmt.Errorf("retry policy %s not found", policyName)
    }
    
    result := &RetryTestResult{
        PolicyName: policyName,
        StartTime:  time.Now(),
        Attempts:   make([]*AttemptResult, 0),
    }
    
    // Тест 1: Успешное выполнение без повторов
    successResult, err := rpt.testSuccessfulExecution(policy)
    if err != nil {
        return nil, fmt.Errorf("successful execution test failed: %w", err)
    }
    result.SuccessfulExecutionResult = successResult
    
    // Тест 2: Выполнение с временными сбоями
    transientFailureResult, err := rpt.testTransientFailures(policy)
    if err != nil {
        return nil, fmt.Errorf("transient failure test failed: %w", err)
    }
    result.TransientFailureResult = transientFailureResult
    
    // Тест 3: Выполнение с постоянными сбоями
    permanentFailureResult, err := rpt.testPermanentFailures(policy)
    if err != nil {
        return nil, fmt.Errorf("permanent failure test failed: %w", err)
    }
    result.PermanentFailureResult = permanentFailureResult
    
    // Тест 4: Валидация backoff стратегии
    backoffResult, err := rpt.testBackoffStrategy(policy)
    if err != nil {
        return nil, fmt.Errorf("backoff strategy test failed: %w", err)
    }
    result.BackoffStrategyResult = backoffResult
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    result.OverallSuccess = rpt.calculateOverallSuccess(result)
    
    return result, nil
}

func (rpt *RetryPatternTest) testTransientFailures(policy *RetryPolicy) (*TransientFailureTestResult, error) {
    result := &TransientFailureTestResult{
        StartTime: time.Now(),
    }
    
    // Симуляция временных сбоев
    failureCount := 0
    maxFailures := policy.MaxAttempts - 1 // Последняя попытка должна быть успешной
    
    err := policy.Execute(func() error {
        if failureCount < maxFailures {
            failureCount++
            return &TransientError{Message: fmt.Sprintf("Transient failure #%d", failureCount)}
        }
        return nil // Успешное выполнение
    })
    
    if err != nil {
        return nil, fmt.Errorf("retry should have succeeded after %d transient failures", maxFailures)
    }
    
    result.FailureCount = failureCount
    result.Success = true
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    
    return result, nil
}
```

## Взаимодействия между компонентами

### ChaosEngine → Fault Simulators
```go
func (ce *ChaosEngine) executeExperiment(experiment *ChaosExperiment) error {
    for _, faultSpec := range experiment.FaultSpecs {
        fault, err := ce.faultInjector.InjectFault(faultSpec)
        if err != nil {
            return fmt.Errorf("failed to inject fault %s: %w", faultSpec.ID, err)
        }
        
        // Мониторинг эффектов сбоя
        go ce.monitorFaultEffects(fault)
    }
    
    // Ожидание завершения эксперимента
    time.Sleep(experiment.Duration)
    
    // Восстановление всех сбоев
    return ce.faultInjector.RestoreAllFaults()
}
```

### Recovery Tests → Circuit Breaker Validation
```go
func (rte *RecoveryTestEngine) validateCircuitBreakerDuringRecovery(cb *CircuitBreaker, recovery *RecoveryProcess) error {
    // Проверка активации circuit breaker во время сбоя
    if cb.GetState() != CircuitBreakerStateOpen {
        return fmt.Errorf("circuit breaker should be open during fault")
    }
    
    // Мониторинг перехода в half-open во время восстановления
    go func() {
        for recovery.IsActive() {
            if cb.GetState() == CircuitBreakerStateHalfOpen {
                log.Printf("Circuit breaker transitioned to half-open during recovery")
                break
            }
            time.Sleep(100 * time.Millisecond)
        }
    }()
    
    return nil
}
```

Эта диаграмма служит детальным руководством для реализации всех компонентов системы тестирования отказоустойчивости Task 7.3.