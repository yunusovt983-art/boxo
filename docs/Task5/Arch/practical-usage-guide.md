# Практическое руководство по использованию архитектурных диаграмм

## 🎯 Как диаграммы помогают в ежедневной разработке

### 1. Планирование новых функций

#### Сценарий: Добавление нового типа метрик
**Используемые диаграммы**: Component Diagram + Class Diagram

**Процесс**:
1. Изучите Component Diagram для понимания, где добавить новый компонент
2. Обновите Class Diagram, добавив новые структуры данных
3. Реализуйте код согласно обновленным диаграммам

**Пример**:
```go
// 1. Обновляем диаграмму классов, добавляя новую структуру
type SecurityMetrics struct {
    FailedAuthAttempts    int64     `json:"failed_auth_attempts"`
    SuspiciousActivities  int64     `json:"suspicious_activities"`
    BlockedIPs           []string   `json:"blocked_ips"`
    Timestamp            time.Time  `json:"timestamp"`
}

// 2. Обновляем MetricsSnapshot в соответствии с диаграммой
type MetricsSnapshot struct {
    BitswapMetrics    *BitswapMetrics    `json:"bitswap"`
    BlockstoreMetrics *BlockstoreMetrics `json:"blockstore"`
    NetworkMetrics    *NetworkMetrics    `json:"network"`
    SecurityMetrics   *SecurityMetrics   `json:"security"` // Новое поле
    Timestamp         time.Time          `json:"timestamp"`
    NodeID            string             `json:"node_id"`
}

// 3. Добавляем новый монитор в Component Diagram
type SecurityMonitor struct {
    authLogger    *AuthLogger
    ipTracker     *IPTracker
    alertThreshold int64
}

func (sm *SecurityMonitor) CollectMetrics() *SecurityMetrics {
    return &SecurityMetrics{
        FailedAuthAttempts:   sm.authLogger.GetFailedAttempts(),
        SuspiciousActivities: sm.ipTracker.GetSuspiciousCount(),
        BlockedIPs:          sm.ipTracker.GetBlockedIPs(),
        Timestamp:           time.Now(),
    }
}
```

### 2. Code Review с использованием диаграмм

#### Чек-лист для ревьюера:
- [ ] Соответствует ли новый код Component Diagram?
- [ ] Правильно ли реализованы интерфейсы из Code Diagram?
- [ ] Следует ли код паттернам из Sequence Diagram?
- [ ] Учтены ли состояния из State Diagram?

**Пример комментария в PR**:
```markdown
## Архитектурный анализ

### ✅ Соответствие диаграммам:
- Новый `SecurityMonitor` правильно интегрирован в `MetricsCollector` (Component Diagram)
- Интерфейс `Monitor` реализован корректно (Code Diagram)

### ⚠️ Замечания:
- Отсутствует обработка ошибок в соответствии с Sequence Diagram
- Нужно добавить переход в состояние `StateAnalyzing` при обнаружении угроз (State Diagram)

### 🔧 Предложения:
```go
func (sm *SecurityMonitor) CollectMetrics() (*SecurityMetrics, error) {
    // Добавить обработку ошибок как показано в Sequence Diagram
    failedAttempts, err := sm.authLogger.GetFailedAttempts()
    if err != nil {
        return nil, fmt.Errorf("failed to get auth attempts: %w", err)
    }
    // ...
}
```

### 3. Отладка и диагностика проблем

#### Сценарий: Система не генерирует алерты
**Используемые диаграммы**: Sequence Diagram + State Diagram

**Процесс диагностики**:
1. Проверьте Sequence Diagram для понимания потока данных
2. Используйте State Diagram для проверки состояний Auto Tuner
3. Добавьте логирование в ключевых точках

**Диагностический код**:
```go
// Добавляем трассировку согласно Sequence Diagram
func (w *MonitoringWorkflow) ExecuteMonitoringCycle(ctx context.Context) error {
    log.Debug("Starting monitoring cycle") // Точка 1 на Sequence Diagram
    
    // 1. Сбор метрик
    metrics, err := w.monitor.CollectMetrics()
    if err != nil {
        log.Error("Failed at step 1 (CollectMetrics) in Sequence Diagram", "error", err)
        return err
    }
    log.Debug("Metrics collected successfully", "metrics_count", len(metrics))
    
    // 2. Анализ производительности  
    issues, err := w.analyzer.AnalyzeTrends([]*MetricsSnapshot{metrics})
    if err != nil {
        log.Error("Failed at step 2 (AnalyzeTrends) in Sequence Diagram", "error", err)
        return err
    }
    log.Debug("Analysis completed", "issues_found", len(issues))
    
    // 3. Проверка состояния Auto Tuner согласно State Diagram
    currentState := w.tuner.GetCurrentState()
    log.Debug("Auto Tuner state", "state", currentState)
    
    if currentState != StateIdle {
        log.Warn("Auto Tuner not in idle state, skipping optimization", "state", currentState)
        return nil
    }
    
    // Продолжение согласно диаграммам...
}
```

### 4. Рефакторинг с учетом архитектуры

#### Сценарий: Выделение общего интерфейса для мониторов
**Используемые диаграммы**: Component Diagram + Code Diagram

**До рефакторинга** (нарушает принципы диаграмм):
```go
// Дублирование кода в разных мониторах
type BitswapMonitor struct {
    // специфичные поля
}

func (bm *BitswapMonitor) Collect() *BitswapMetrics {
    // логика сбора
}

type NetworkMonitor struct {
    // специфичные поля  
}

func (nm *NetworkMonitor) Collect() *NetworkMetrics {
    // похожая логика сбора
}
```

**После рефакторинга** (соответствует Code Diagram):
```go
// Общий интерфейс согласно Code Diagram
type MetricsCollector interface {
    CollectMetrics() (interface{}, error)
    GetCollectorType() string
    IsHealthy() bool
}

// Базовая структура для всех мониторов
type BaseMonitor struct {
    name         string
    interval     time.Duration
    lastCollected time.Time
    errorCount   int64
}

func (bm *BaseMonitor) IsHealthy() bool {
    return bm.errorCount < 5 && time.Since(bm.lastCollected) < bm.interval*2
}

// Специализированные мониторы
type BitswapMonitor struct {
    BaseMonitor
    bitswap *bitswap.Bitswap
}

func (bm *BitswapMonitor) CollectMetrics() (interface{}, error) {
    defer func() { bm.lastCollected = time.Now() }()
    
    metrics := &BitswapMetrics{
        RequestsPerSecond: bm.bitswap.GetRequestRate(),
        // ...
    }
    
    return metrics, nil
}

func (bm *BitswapMonitor) GetCollectorType() string {
    return "bitswap"
}
```

### 5. Тестирование на основе диаграмм

#### Unit тесты на основе Class Diagram
```go
// Тестируем каждый класс из Class Diagram
func TestBitswapMetrics_IsHealthy(t *testing.T) {
    tests := []struct {
        name     string
        metrics  *BitswapMetrics
        expected bool
    }{
        {
            name: "healthy metrics",
            metrics: &BitswapMetrics{
                ErrorRate:       0.02, // 2% - здоровый уровень
                AverageLatency:  50 * time.Millisecond,
                RequestsPerSecond: 100,
            },
            expected: true,
        },
        {
            name: "high error rate",
            metrics: &BitswapMetrics{
                ErrorRate:       0.10, // 10% - нездоровый уровень
                AverageLatency:  50 * time.Millisecond,
                RequestsPerSecond: 100,
            },
            expected: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := tt.metrics.IsHealthy()
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

#### Integration тесты на основе Sequence Diagram
```go
func TestMonitoringWorkflow_FullCycle(t *testing.T) {
    // Настройка моков согласно Component Diagram
    mockMonitor := &MockPerformanceMonitor{}
    mockAnalyzer := &MockBottleneckAnalyzer{}
    mockAlertMgr := &MockAlertManager{}
    mockTuner := &MockAutoTuner{}
    
    workflow := &MonitoringWorkflow{
        monitor:  mockMonitor,
        analyzer: mockAnalyzer,
        alertMgr: mockAlertMgr,
        tuner:    mockTuner,
    }
    
    // Тестируем последовательность согласно Sequence Diagram
    t.Run("successful monitoring cycle", func(t *testing.T) {
        // 1. Настройка ожиданий согласно Sequence Diagram
        mockMonitor.On("CollectMetrics").Return(&MetricsSnapshot{}, nil)
        mockAnalyzer.On("AnalyzeTrends", mock.Any).Return([]*PerformanceIssue{}, nil)
        
        // 2. Выполнение цикла
        err := workflow.ExecuteMonitoringCycle(context.Background())
        
        // 3. Проверка вызовов в правильном порядке (Sequence Diagram)
        assert.NoError(t, err)
        mockMonitor.AssertCalled(t, "CollectMetrics")
        mockAnalyzer.AssertCalled(t, "AnalyzeTrends", mock.Any)
    })
}
```

#### State Machine тесты на основе State Diagram
```go
func TestTunerStateMachine_StateTransitions(t *testing.T) {
    sm := NewTunerStateMachine()
    
    // Тестируем переходы согласно State Diagram
    tests := []struct {
        name        string
        fromState   TunerState
        toState     TunerState
        shouldError bool
    }{
        {"idle to analyzing", StateIdle, StateAnalyzing, false},
        {"analyzing to predicting", StateAnalyzing, StatePredicting, false},
        {"analyzing to idle", StateAnalyzing, StateIdle, false},
        {"idle to applying", StateIdle, StateApplying, true}, // Недопустимый переход
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            sm.currentState = tt.fromState
            err := sm.TransitionTo(tt.toState)
            
            if tt.shouldError {
                assert.Error(t, err)
                assert.Equal(t, tt.fromState, sm.currentState) // Состояние не изменилось
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.toState, sm.currentState)
            }
        })
    }
}
```

### 6. Документирование API на основе диаграмм

#### OpenAPI спецификация, основанная на Class Diagram
```yaml
# api-spec.yml - генерируется на основе Class Diagram
openapi: 3.0.0
info:
  title: Boxo Monitoring API
  version: 1.0.0
  description: API для системы мониторинга производительности Boxo

paths:
  /metrics/snapshot:
    get:
      summary: Получить снимок метрик
      responses:
        '200':
          description: Успешный ответ
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MetricsSnapshot'

components:
  schemas:
    # Схемы основаны на структурах из Class Diagram
    MetricsSnapshot:
      type: object
      required:
        - bitswap
        - blockstore
        - network
        - timestamp
        - node_id
      properties:
        bitswap:
          $ref: '#/components/schemas/BitswapMetrics'
        blockstore:
          $ref: '#/components/schemas/BlockstoreMetrics'
        network:
          $ref: '#/components/schemas/NetworkMetrics'
        timestamp:
          type: string
          format: date-time
        node_id:
          type: string
          minLength: 1
    
    BitswapMetrics:
      type: object
      properties:
        requests_per_second:
          type: number
          minimum: 0
        average_latency:
          type: string
          description: Duration in nanoseconds
        error_rate:
          type: number
          minimum: 0
          maximum: 1
        active_connections:
          type: integer
          minimum: 0
```

### 7. Мониторинг соответствия архитектуре

#### Автоматическая проверка соответствия диаграммам
```go
// tools/arch-validator/main.go
// Инструмент для проверки соответствия кода диаграммам

type ArchitectureValidator struct {
    codeAnalyzer    *CodeAnalyzer
    diagramParser   *DiagramParser
    violations      []Violation
}

func (av *ArchitectureValidator) ValidateInterfaces() error {
    // Парсим Code Diagram для получения ожидаемых интерфейсов
    expectedInterfaces := av.diagramParser.ParseInterfaces("code-diagram.puml")
    
    // Анализируем код для поиска реальных интерфейсов
    actualInterfaces := av.codeAnalyzer.FindInterfaces("./bitswap/monitoring/")
    
    // Сравниваем
    for _, expected := range expectedInterfaces {
        if actual, found := actualInterfaces[expected.Name]; !found {
            av.violations = append(av.violations, Violation{
                Type:        "missing_interface",
                Expected:    expected.Name,
                Description: fmt.Sprintf("Interface %s from diagram not found in code", expected.Name),
            })
        } else if !av.interfacesMatch(expected, actual) {
            av.violations = append(av.violations, Violation{
                Type:        "interface_mismatch",
                Expected:    expected.Name,
                Description: "Interface signature doesn't match diagram",
            })
        }
    }
    
    return nil
}

func (av *ArchitectureValidator) ValidateComponentDependencies() error {
    // Парсим Component Diagram для получения ожидаемых зависимостей
    expectedDeps := av.diagramParser.ParseDependencies("component-diagram.puml")
    
    // Анализируем импорты в коде
    actualDeps := av.codeAnalyzer.FindDependencies("./bitswap/monitoring/")
    
    // Проверяем на нарушения архитектуры
    for _, actualDep := range actualDeps {
        if !av.isDependencyAllowed(actualDep, expectedDeps) {
            av.violations = append(av.violations, Violation{
                Type:        "forbidden_dependency",
                Actual:      actualDep.String(),
                Description: "Dependency not allowed by Component Diagram",
            })
        }
    }
    
    return nil
}
```

### 8. Генерация кода на основе диаграмм

#### Генератор интерфейсов из Code Diagram
```go
// tools/code-generator/interface_generator.go
type InterfaceGenerator struct {
    diagramParser *DiagramParser
    codeWriter    *CodeWriter
}

func (ig *InterfaceGenerator) GenerateFromDiagram(diagramPath string) error {
    interfaces := ig.diagramParser.ParseInterfaces(diagramPath)
    
    for _, iface := range interfaces {
        code := ig.generateInterfaceCode(iface)
        filename := fmt.Sprintf("%s.go", strings.ToLower(iface.Name))
        
        if err := ig.codeWriter.WriteFile(filename, code); err != nil {
            return fmt.Errorf("failed to write %s: %w", filename, err)
        }
    }
    
    return nil
}

func (ig *InterfaceGenerator) generateInterfaceCode(iface *Interface) string {
    var builder strings.Builder
    
    builder.WriteString(fmt.Sprintf("// Code generated from %s. DO NOT EDIT.\n\n", iface.Source))
    builder.WriteString("package monitoring\n\n")
    builder.WriteString(fmt.Sprintf("// %s %s\n", iface.Name, iface.Description))
    builder.WriteString(fmt.Sprintf("type %s interface {\n", iface.Name))
    
    for _, method := range iface.Methods {
        builder.WriteString(fmt.Sprintf("\t%s\n", method.Signature))
    }
    
    builder.WriteString("}\n")
    
    return builder.String()
}
```

## 🔄 Процесс синхронизации диаграмм и кода

### 1. При изменении архитектуры:
1. Обновите соответствующие диаграммы
2. Запустите генерацию кода (если используется)
3. Обновите реализацию
4. Запустите валидацию архитектуры
5. Обновите тесты

### 2. При изменении кода:
1. Проверьте соответствие диаграммам
2. При необходимости обновите диаграммы
3. Запустите валидацию архитектуры
4. Обновите документацию

### 3. Автоматизация в CI/CD:
```yaml
# .github/workflows/architecture-validation.yml
name: Architecture Validation

on: [push, pull_request]

jobs:
  validate-architecture:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v2
    
    - name: Setup Go
      uses: actions/setup-go@v2
      with:
        go-version: 1.21
    
    - name: Install PlantUML
      run: |
        wget https://github.com/plantuml/plantuml/releases/latest/download/plantuml-1.2024.0.jar
        sudo mv plantuml-1.2024.0.jar /usr/local/bin/plantuml.jar
    
    - name: Validate diagrams syntax
      run: |
        cd docs/Task5/Arch
        make validate
    
    - name: Run architecture validation
      run: |
        go run tools/arch-validator/main.go \
          --diagrams-path docs/Task5/Arch \
          --code-path bitswap/monitoring \
          --output-format json
    
    - name: Check for architecture violations
      run: |
        if [ -s architecture-violations.json ]; then
          echo "Architecture violations found:"
          cat architecture-violations.json
          exit 1
        fi
```

Эта система обеспечивает постоянную синхронизацию между архитектурными диаграммами и реальным кодом, делая документацию живой и полезной для всей команды разработки.