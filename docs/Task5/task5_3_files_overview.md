# Task 5.3 - Полный обзор файлов "Добавить систему алертов и уведомлений"

## Обзор задачи
**Task 5.3**: "Добавить систему алертов и уведомлений" - создание комплексной системы мониторинга с автоматическими алертами, структурированным логированием и трассировкой запросов при превышении SLA.

**Требования**: 4.2, 4.3 - Система алертов и структурированное логирование с трассировкой

## Архитектура решения

### Основные компоненты:
1. **AlertManager** - центральная система управления алертами
2. **AlertNotifiers** - каналы уведомлений (Log, Webhook, Email, Slack)
3. **StructuredLogger** - система структурированного логирования с контекстом
4. **RequestTracer** - механизм трассировки запросов при превышении SLA
5. **Интеграционные тесты** - комплексное тестирование системы алертов

## Созданные файлы

### 1. monitoring/alert_manager.go
**Размер**: ~1200 строк  
**Назначение**: Основная реализация системы управления алертами

#### Ключевые интерфейсы:

##### AlertManager interface
```go
type AlertManager interface {
    StartMonitoring(ctx context.Context, interval time.Duration) error
    StopMonitoring() error
    RegisterNotifier(name string, notifier AlertNotifier) error
    UnregisterNotifier(name string) error
    SetAlertRules(rules []AlertRule) error
    GetActiveAlerts() []Alert
    GetAlertHistory(since time.Time) []Alert
    AcknowledgeAlert(alertID string, acknowledgedBy string) error
    ResolveAlert(alertID string, resolvedBy string) error
    GetAlertStatistics() AlertStatistics
}
```

##### AlertNotifier interface
```go
type AlertNotifier interface {
    SendAlert(ctx context.Context, alert Alert) error
    SendResolution(ctx context.Context, alert Alert) error
    GetName() string
    IsHealthy() bool
}
```

##### RequestTracer interface
```go
type RequestTracer interface {
    StartTrace(ctx context.Context, operation string) (TraceContext, error)
    EndTrace(traceCtx TraceContext, err error) error
    GetTrace(traceID string) (*TraceData, error)
    GetActiveTraces() []TraceData
}
```

#### Структуры данных:

##### Alert - Алерт производительности
```go
type Alert struct {
    ID              string                 `json:"id"`
    RuleID          string                 `json:"rule_id"`
    Title           string                 `json:"title"`
    Description     string                 `json:"description"`
    Severity        Severity               `json:"severity"`
    Component       string                 `json:"component"`
    Metric          string                 `json:"metric"`
    Value           interface{}            `json:"value"`
    Threshold       interface{}            `json:"threshold"`
    Labels          map[string]string      `json:"labels"`
    Annotations     map[string]string      `json:"annotations"`
    Context         map[string]interface{} `json:"context"`
    CreatedAt       time.Time              `json:"created_at"`
    UpdatedAt       time.Time              `json:"updated_at"`
    AcknowledgedAt  *time.Time             `json:"acknowledged_at,omitempty"`
    AcknowledgedBy  string                 `json:"acknowledged_by,omitempty"`
    ResolvedAt      *time.Time             `json:"resolved_at,omitempty"`
    ResolvedBy      string                 `json:"resolved_by,omitempty"`
    State           AlertState             `json:"state"`
    FireCount       int                    `json:"fire_count"`
    LastFiredAt     time.Time              `json:"last_fired_at"`
    TraceID         string                 `json:"trace_id,omitempty"`
    RequestID       string                 `json:"request_id,omitempty"`
}
```

##### AlertRule - Правило алерта
```go
type AlertRule struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Component   string                 `json:"component"`
    Metric      string                 `json:"metric"`
    Condition   AlertCondition         `json:"condition"`
    Threshold   interface{}            `json:"threshold"`
    Duration    time.Duration          `json:"duration"`
    Severity    Severity               `json:"severity"`
    Labels      map[string]string      `json:"labels"`
    Annotations map[string]string      `json:"annotations"`
    Enabled     bool                   `json:"enabled"`
    CreatedAt   time.Time              `json:"created_at"`
    UpdatedAt   time.Time              `json:"updated_at"`
}
```

##### TraceContext - Контекст трассировки
```go
type TraceContext struct {
    TraceID     string                 `json:"trace_id"`
    SpanID      string                 `json:"span_id"`
    Operation   string                 `json:"operation"`
    StartTime   time.Time              `json:"start_time"`
    Labels      map[string]string      `json:"labels"`
    Attributes  map[string]interface{} `json:"attributes"`
}
```

##### AlertStatistics - Статистика алертов
```go
type AlertStatistics struct {
    TotalAlerts       int                    `json:"total_alerts"`
    ActiveAlerts      int                    `json:"active_alerts"`
    AcknowledgedAlerts int                   `json:"acknowledged_alerts"`
    ResolvedAlerts    int                    `json:"resolved_alerts"`
    AlertsByComponent map[string]int         `json:"alerts_by_component"`
    AlertsBySeverity  map[Severity]int       `json:"alerts_by_severity"`
    MTTR              time.Duration          `json:"mttr"` // Mean Time To Resolution
    AlertFrequency    map[string]float64     `json:"alert_frequency"`
    LastUpdated       time.Time              `json:"last_updated"`
}
```

#### Алгоритмы мониторинга:

##### 1. Непрерывный мониторинг
```go
func (am *alertManager) StartMonitoring(ctx context.Context, interval time.Duration) error
```
- **Функции**:
  - Периодический сбор метрик производительности
  - Оценка правил алертов против текущих метрик
  - Проверка нарушений SLA и запуск трассировки
  - Отправка уведомлений через зарегистрированные каналы

##### 2. Оценка правил алертов
```go
func (am *alertManager) evaluateRule(rule AlertRule, metrics *PerformanceMetrics) (bool, interface{})
```
- **Поддерживаемые условия**:
  - GreaterThan (>), LessThan (<)
  - GreaterOrEqual (>=), LessOrEqual (<=)
  - Equals (==), NotEquals (!=)
  - Contains, Regex

##### 3. Проверка нарушений SLA
```go
func (am *alertManager) checkSLAViolations(ctx context.Context, metrics *PerformanceMetrics)
```
- **SLA пороги по умолчанию**:
  - BitswapRequestSLA: 100ms
  - BlockstoreReadSLA: 50ms
  - BlockstoreWriteSLA: 100ms
  - NetworkLatencySLA: 200ms

#### Правила алертов по умолчанию:

##### 1. Bitswap High Latency
```go
{
    ID:          "bitswap-high-latency",
    Component:   "bitswap",
    Metric:      "p95_response_time",
    Condition:   ConditionGreaterThan,
    Threshold:   100 * time.Millisecond,
    Severity:    SeverityHigh,
}
```

##### 2. Bitswap Queue Depth High
```go
{
    ID:          "bitswap-queue-depth",
    Component:   "bitswap",
    Metric:      "queued_requests",
    Condition:   ConditionGreaterThan,
    Threshold:   int64(1000),
    Severity:    SeverityMedium,
}
```

##### 3. Blockstore Cache Hit Rate Low
```go
{
    ID:          "blockstore-cache-hit-rate-low",
    Component:   "blockstore",
    Metric:      "cache_hit_rate",
    Condition:   ConditionLessThan,
    Threshold:   0.80,
    Severity:    SeverityMedium,
}
```

##### 4. High CPU Usage
```go
{
    ID:          "resource-cpu-high",
    Component:   "resource",
    Metric:      "cpu_usage",
    Condition:   ConditionGreaterThan,
    Threshold:   80.0,
    Severity:    SeverityHigh,
}
```

##### 5. High Memory Usage
```go
{
    ID:          "resource-memory-high",
    Component:   "resource",
    Metric:      "memory_usage_percent",
    Condition:   ConditionGreaterThan,
    Threshold:   85.0,
    Severity:    SeverityCritical,
}
```

### 2. monitoring/alert_notifiers.go
**Размер**: ~800 строк  
**Назначение**: Реализация различных каналов уведомлений

#### Типы уведомителей:

##### 1. LogNotifier - Логирование алертов
```go
type LogNotifier struct {
    logger StructuredLogger
    name   string
}
```
- **Функции**:
  - Отправка алертов в структурированные логи
  - Автоматическое определение уровня логирования по серьезности
  - Всегда здоровый (IsHealthy() = true)

##### 2. WebhookNotifier - HTTP Webhook уведомления
```go
type WebhookNotifier struct {
    name       string
    url        string
    headers    map[string]string
    timeout    time.Duration
    client     *http.Client
    logger     StructuredLogger
    healthy    bool
    lastError  error
}
```
- **Функции**:
  - Отправка JSON payload на HTTP endpoint
  - Настраиваемые заголовки и таймауты
  - Мониторинг здоровья с автоматическим восстановлением
  - Retry логика при ошибках

##### WebhookPayload структура:
```go
type WebhookPayload struct {
    Type        string                 `json:"type"` // "alert" или "resolution"
    Alert       Alert                  `json:"alert"`
    Timestamp   time.Time              `json:"timestamp"`
    Source      string                 `json:"source"`
    Environment string                 `json:"environment,omitempty"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
```

##### 3. EmailNotifier - Email уведомления
```go
type EmailNotifier struct {
    name       string
    smtpHost   string
    smtpPort   int
    username   string
    password   string
    from       string
    to         []string
    logger     StructuredLogger
    healthy    bool
    lastError  error
}
```
- **Функции**:
  - Отправка email через SMTP
  - Форматирование алертов в читаемый текст
  - Поддержка множественных получателей
  - Автоматическое форматирование темы по серьезности

##### 4. SlackNotifier - Slack уведомления
```go
type SlackNotifier struct {
    name      string
    webhookURL string
    channel   string
    username  string
    iconEmoji string
    client    *http.Client
    logger    StructuredLogger
    healthy   bool
    lastError error
}
```
- **Функции**:
  - Отправка богато форматированных сообщений в Slack
  - Цветовое кодирование по серьезности
  - Структурированные поля с метриками
  - Поддержка attachments и эмодзи

##### SlackMessage структура:
```go
type SlackMessage struct {
    Channel     string            `json:"channel,omitempty"`
    Username    string            `json:"username,omitempty"`
    IconEmoji   string            `json:"icon_emoji,omitempty"`
    Text        string            `json:"text"`
    Attachments []SlackAttachment `json:"attachments,omitempty"`
}
```

#### Цветовое кодирование серьезности:
- **Critical**: "danger" (красный)
- **High**: "warning" (оранжевый)
- **Medium**: "#ff9900" (желтый)
- **Low**: "#36a64f" (зеленый)

### 3. monitoring/structured_logger.go
**Размер**: ~900 строк  
**Назначение**: Система структурированного логирования с контекстом

#### Ключевые интерфейсы:

##### StructuredLogger interface
```go
type StructuredLogger interface {
    // Context-aware logging methods
    LogWithContext(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) error
    InfoWithContext(ctx context.Context, msg string, attrs ...slog.Attr)
    WarnWithContext(ctx context.Context, msg string, attrs ...slog.Attr)
    ErrorWithContext(ctx context.Context, msg string, attrs ...slog.Attr)
    DebugWithContext(ctx context.Context, msg string, attrs ...slog.Attr)
    
    // Performance logging
    LogPerformanceMetrics(ctx context.Context, component string, metrics map[string]interface{})
    LogSlowOperation(ctx context.Context, operation string, duration time.Duration, threshold time.Duration)
    LogBottleneck(ctx context.Context, bottleneck Bottleneck)
    LogAlert(ctx context.Context, alert Alert)
    
    // Request tracing
    LogRequestStart(ctx context.Context, operation string, params map[string]interface{}) context.Context
    LogRequestEnd(ctx context.Context, operation string, duration time.Duration, err error)
    
    // Configuration
    SetLevel(level slog.Level)
    SetOutput(output LogOutput)
    AddHook(hook LogHook) error
    RemoveHook(hookName string) error
    
    // Utility methods
    WithFields(fields map[string]interface{}) StructuredLogger
    WithComponent(component string) StructuredLogger
    Flush() error
}
```

##### LogOutput interface
```go
type LogOutput interface {
    Write(entry LogEntry) error
    Flush() error
    Close() error
    GetName() string
}
```

##### LogHook interface
```go
type LogHook interface {
    Process(entry LogEntry) (LogEntry, error)
    GetName() string
    ShouldProcess(level slog.Level) bool
}
```

#### Структуры данных:

##### LogEntry - Запись лога
```go
type LogEntry struct {
    Timestamp   time.Time              `json:"timestamp"`
    Level       slog.Level             `json:"level"`
    Message     string                 `json:"message"`
    Component   string                 `json:"component,omitempty"`
    Operation   string                 `json:"operation,omitempty"`
    TraceID     string                 `json:"trace_id,omitempty"`
    SpanID      string                 `json:"span_id,omitempty"`
    RequestID   string                 `json:"request_id,omitempty"`
    UserID      string                 `json:"user_id,omitempty"`
    Fields      map[string]interface{} `json:"fields,omitempty"`
    Error       string                 `json:"error,omitempty"`
    Stack       string                 `json:"stack,omitempty"`
    Duration    *time.Duration         `json:"duration,omitempty"`
    Caller      string                 `json:"caller,omitempty"`
    Hostname    string                 `json:"hostname,omitempty"`
    PID         int                    `json:"pid,omitempty"`
    Goroutine   int64                  `json:"goroutine,omitempty"`
}
```

##### LogContext - Контекст логирования
```go
type LogContext struct {
    TraceID     string                 `json:"trace_id,omitempty"`
    SpanID      string                 `json:"span_id,omitempty"`
    RequestID   string                 `json:"request_id,omitempty"`
    UserID      string                 `json:"user_id,omitempty"`
    Component   string                 `json:"component,omitempty"`
    Operation   string                 `json:"operation,omitempty"`
    StartTime   time.Time              `json:"start_time,omitempty"`
    Fields      map[string]interface{} `json:"fields,omitempty"`
}
```

#### Функции логирования:

##### 1. Контекстное логирование
```go
func (sl *structuredLogger) LogWithContext(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) error
```
- **Автоматическое извлечение контекста**:
  - TraceID, SpanID, RequestID, UserID
  - Компонент и операция
  - Время начала для расчета длительности
  - Пользовательские поля

##### 2. Логирование производительности
```go
func (sl *structuredLogger) LogPerformanceMetrics(ctx context.Context, component string, metrics map[string]interface{})
func (sl *structuredLogger) LogSlowOperation(ctx context.Context, operation string, duration time.Duration, threshold time.Duration)
func (sl *structuredLogger) LogBottleneck(ctx context.Context, bottleneck Bottleneck)
func (sl *structuredLogger) LogAlert(ctx context.Context, alert Alert)
```

##### 3. Трассировка запросов
```go
func (sl *structuredLogger) LogRequestStart(ctx context.Context, operation string, params map[string]interface{}) context.Context
func (sl *structuredLogger) LogRequestEnd(ctx context.Context, operation string, duration time.Duration, err error)
```
- **Автоматическая генерация ID**:
  - RequestID: "req_{timestamp}_{goroutine}"
  - TraceID: "trace_{timestamp}_{goroutine}"

#### Контекстные помощники:

##### Context управление
```go
func WithLogContext(ctx context.Context, logCtx *LogContext) context.Context
func GetLogContext(ctx context.Context) *LogContext
func WithTraceID(ctx context.Context, traceID string) context.Context
func WithRequestID(ctx context.Context, requestID string) context.Context
func WithUserID(ctx context.Context, userID string) context.Context
```

#### Опции конфигурации:

##### LoggerOption функции
```go
func WithLevel(level slog.Level) LoggerOption
func WithComponent(component string) LoggerOption
func WithFields(fields map[string]interface{}) LoggerOption
func WithOutput(name string, output LogOutput) LoggerOption
func WithHook(hook LogHook) LoggerOption
```

### 4. monitoring/alert_manager_test.go
**Размер**: ~600 строк  
**Назначение**: Интеграционные тесты для AlertManager

#### Mock реализации:

##### mockPerformanceMonitor
```go
type mockPerformanceMonitor struct {
    mu      sync.RWMutex
    metrics *PerformanceMetrics
}
```
- **Функции**: Симуляция сбора метрик производительности
- **Настройка**: Возможность установки различных значений метрик

##### mockBottleneckAnalyzer
```go
type mockBottleneckAnalyzer struct {
    bottlenecks []Bottleneck
}
```
- **Функции**: Симуляция анализа узких мест
- **Настройка**: Возможность установки обнаруженных узких мест

##### mockRequestTracer
```go
type mockRequestTracer struct {
    traces map[string]*TraceData
}
```
- **Функции**: Симуляция трассировки запросов
- **Хранение**: В памяти карта активных трассировок

##### mockAlertNotifier
```go
type mockAlertNotifier struct {
    name         string
    alerts       []Alert
    resolutions  []Alert
    healthy      bool
}
```
- **Функции**: Симуляция отправки уведомлений
- **Проверка**: Сохранение отправленных алертов и разрешений

#### Тестовые сценарии:

##### 1. TestAlertManager_BasicFunctionality
- **Проверки**: Базовые операции (GetActiveAlerts, GetAlertStatistics)
- **Валидация**: Корректность начального состояния

##### 2. TestAlertManager_AlertRules
- **Проверки**: Установка валидных и невалидных правил
- **Валидация**: Правильность валидации правил

##### 3. TestAlertManager_AlertFiring
- **Сценарий**: Настройка метрик, превышающих пороги
- **Проверки**: Срабатывание алертов, отправка уведомлений
- **Валидация**: Корректность состояния алертов

##### 4. TestAlertManager_AlertResolution
- **Сценарий**: Срабатывание алерта, затем нормализация метрик
- **Проверки**: Автоматическое разрешение алертов
- **Валидация**: Отправка уведомлений о разрешении

##### 5. TestAlertManager_ManualOperations
- **Проверки**: Ручное подтверждение и разрешение алертов
- **Валидация**: Корректность изменения состояний

##### 6. TestAlertManager_SLAViolations
- **Сценарий**: Метрики, нарушающие SLA
- **Проверки**: Запуск трассировки при нарушениях
- **Валидация**: Создание активных трассировок

### 5. monitoring/alert_notifiers_test.go
**Размер**: ~700 строк  
**Назначение**: Тесты для всех типов уведомителей

#### Тестовые сценарии по типам:

##### TestLogNotifier
- **SendAlert**: Проверка логирования алертов с правильным уровнем
- **SendResolution**: Проверка логирования разрешений
- **Валидация**: Корректность полей в логах

##### TestWebhookNotifier
- **SendAlert**: Отправка HTTP POST с JSON payload
- **SendResolution**: Отправка уведомления о разрешении
- **ErrorHandling**: Обработка ошибок сервера, сети, таймаутов
- **Валидация**: Корректность payload и заголовков

##### TestEmailNotifier
- **FormatAlertEmail**: Форматирование алертов в текст
- **FormatResolutionEmail**: Форматирование разрешений
- **Валидация**: Наличие всех необходимых полей

##### TestSlackNotifier
- **SendAlert**: Отправка форматированных сообщений
- **SendResolution**: Отправка уведомлений о разрешении
- **GetSeverityColor**: Правильность цветового кодирования
- **Валидация**: Корректность Slack attachments

#### Mock HTTP серверы:
- **Успешные ответы**: Тестирование нормальной работы
- **Ошибки сервера**: Тестирование обработки HTTP 500
- **Сетевые ошибки**: Тестирование недоступности сервера
- **Таймауты**: Тестирование превышения времени ожидания

### 6. monitoring/structured_logger_test.go
**Размер**: ~800 строк  
**Назначение**: Тесты для структурированного логирования

#### Mock реализации:

##### mockLogOutput
```go
type mockLogOutput struct {
    name    string
    entries []LogEntry
    mu      sync.RWMutex
}
```
- **Функции**: Сохранение записей логов в памяти
- **Проверка**: Возможность извлечения и анализа записей

##### mockLogHook
```go
type mockLogHook struct {
    name      string
    processed []LogEntry
    mu        sync.RWMutex
}
```
- **Функции**: Обработка записей логов с добавлением меток
- **Проверка**: Отслеживание обработанных записей

#### Тестовые сценарии:

##### TestStructuredLogger_BasicLogging
- **InfoLogging**: Базовое логирование с атрибутами
- **ErrorLogging**: Логирование ошибок со стеком
- **LevelFiltering**: Фильтрация по уровню логирования

##### TestStructuredLogger_ContextHandling
- **LogContext**: Извлечение контекста из Context
- **RequestTracing**: Автоматическая трассировка запросов
- **Валидация**: Корректность TraceID, RequestID, длительности

##### TestStructuredLogger_Hooks
- **HookProcessing**: Обработка записей через хуки
- **AddRemoveHook**: Динамическое управление хуками

##### TestStructuredLogger_PerformanceLogging
- **PerformanceMetrics**: Логирование метрик производительности
- **SlowOperation**: Логирование медленных операций
- **BottleneckLogging**: Логирование узких мест
- **AlertLogging**: Логирование алертов с правильным уровнем

##### TestStructuredLogger_WithMethods
- **WithFields**: Создание логгера с дополнительными полями
- **WithComponent**: Создание логгера для конкретного компонента

#### Benchmark тесты:
- **BenchmarkStructuredLogger_InfoLogging**: Производительность базового логирования
- **BenchmarkStructuredLogger_WithContext**: Производительность с контекстом
- **BenchmarkStructuredLogger_JSONSerialization**: Производительность сериализации

## Алгоритмы и методы

### 1. Система алертов

#### Жизненный цикл алерта:
```
Firing → Acknowledged → Resolved
   ↓           ↓           ↓
Notifications → Manual → Auto/Manual Resolution
```

#### Состояния алертов:
- **Firing**: Активный алерт, требующий внимания
- **Acknowledged**: Подтвержденный алерт, работа ведется
- **Resolved**: Разрешенный алерт, проблема устранена
- **Suppressed**: Подавленный алерт (будущая функция)

#### Алгоритм оценки правил:
```go
func (am *alertManager) evaluateRule(rule AlertRule, metrics *PerformanceMetrics) (bool, interface{}) {
    value := am.extractMetricValue(rule.Component, rule.Metric, metrics)
    return am.compareValues(value, rule.Condition, rule.Threshold), value
}
```

#### Поддерживаемые метрики по компонентам:

##### Bitswap:
- p95_response_time, average_response_time
- queued_requests, active_connections
- error_rate, requests_per_second

##### Blockstore:
- cache_hit_rate, average_read_latency
- average_write_latency, batch_operations_per_sec

##### Network:
- active_connections, average_latency
- total_bandwidth, packet_loss_rate

##### Resource:
- cpu_usage, memory_usage, memory_usage_percent
- disk_usage, goroutine_count

### 2. Система уведомлений

#### Архитектура уведомлений:
```
AlertManager → [Notifier1, Notifier2, ...] → External Systems
```

#### Обработка ошибок:
- **Здоровье уведомителей**: Автоматическое отслеживание состояния
- **Retry логика**: Повторные попытки при временных ошибках
- **Graceful degradation**: Продолжение работы при отказе части уведомителей

#### Форматирование сообщений:
- **Log**: Структурированные записи с контекстом
- **Webhook**: JSON payload с метаданными
- **Email**: Текстовое форматирование с деталями
- **Slack**: Rich formatting с цветовым кодированием

### 3. Структурированное логирование

#### Архитектура логирования:
```
Logger → [Hook1, Hook2, ...] → [Output1, Output2, ...]
```

#### Контекстная информация:
- **Трассировка**: TraceID, SpanID для связи записей
- **Запросы**: RequestID для отслеживания операций
- **Пользователи**: UserID для аудита
- **Компоненты**: Component для фильтрации
- **Операции**: Operation для группировки

#### Автоматические поля:
- **Timestamp**: Время записи
- **Caller**: Файл и строка кода
- **Hostname**: Имя хоста
- **PID**: ID процесса
- **Goroutine**: ID горутины
- **Stack**: Стек вызовов для ошибок

### 4. Трассировка запросов

#### SLA мониторинг:
```go
func (am *alertManager) checkSLAViolations(ctx context.Context, metrics *PerformanceMetrics) {
    if metrics.BitswapMetrics.P95ResponseTime > am.slaThresholds.BitswapRequestSLA {
        traceCtx, _ := am.requestTracer.StartTrace(ctx, "bitswap_sla_violation")
        // Логирование нарушения SLA с TraceID
    }
}
```

#### Структура трассировки:
- **TraceData**: Полная информация о трассировке
- **SpanData**: Информация о конкретных операциях
- **Attributes**: Дополнительные метаданные
- **Success/Error**: Результат выполнения

## Особенности реализации

### 1. Потокобезопасность
- **AlertManager**: RWMutex для защиты состояния алертов
- **Notifiers**: Индивидуальная защита состояния здоровья
- **Logger**: Потокобезопасные операции с выводами и хуками

### 2. Производительность
- **Асинхронные уведомления**: Горутины для каждого уведомителя
- **Эффективная сериализация**: Оптимизированный JSON
- **Минимальные аллокации**: Переиспользование структур

### 3. Надежность
- **Graceful shutdown**: Корректная остановка мониторинга
- **Error recovery**: Восстановление после ошибок
- **Health monitoring**: Отслеживание состояния компонентов

### 4. Расширяемость
- **Pluggable notifiers**: Легкое добавление новых каналов
- **Configurable rules**: Гибкая настройка правил алертов
- **Hook system**: Расширяемая обработка логов

### 5. Наблюдаемость
- **Self-monitoring**: Логирование работы системы алертов
- **Metrics**: Статистика алертов и производительности
- **Tracing**: Трассировка собственных операций

## Интеграция компонентов

### Взаимодействие AlertManager ↔ StructuredLogger:
```go
// AlertManager использует StructuredLogger для логирования
am.logger.Warn("SLA violation detected, starting trace",
    slog.String("component", "bitswap"),
    slog.Duration("response_time", metrics.BitswapMetrics.P95ResponseTime),
    slog.String("trace_id", traceCtx.TraceID),
)
```

### Взаимодействие AlertManager ↔ RequestTracer:
```go
// При нарушении SLA запускается трассировка
if metrics.BitswapMetrics.P95ResponseTime > am.slaThresholds.BitswapRequestSLA {
    traceCtx, err := am.requestTracer.StartTrace(ctx, "bitswap_sla_violation")
    // TraceID добавляется к алерту для корреляции
    alert.TraceID = traceCtx.TraceID
}
```

### Взаимодействие AlertNotifiers ↔ StructuredLogger:
```go
// Уведомители используют StructuredLogger для отладки
wn.logger.DebugWithContext(ctx, "Webhook notification sent successfully",
    slog.String("notifier", wn.name),
    slog.String("url", wn.url),
    slog.String("alert_id", payload.Alert.ID),
)
```

## Соответствие требованиям

✅ **Требование 4.2**: Реализована система анализа узких мест с автоматическими рекомендациями  
✅ **Требование 4.3**: Добавлено структурированное логирование с контекстом для отладки  
✅ **AlertManager**: Создан для генерации алертов при деградации производительности  
✅ **Структурированное логирование**: Добавлено с контекстом для отладки  
✅ **Трассировка запросов**: Создан механизм при превышении SLA  
✅ **Интеграционные тесты**: Написаны для проверки системы алертов  

### Детальное соответствие задачам:

#### "Реализовать AlertManager для генерации алертов при деградации производительности"
- ✅ **AlertManager interface**: 10 методов для полного управления алертами
- ✅ **Автоматическая генерация**: Непрерывный мониторинг с настраиваемым интервалом
- ✅ **Правила алертов**: 5 правил по умолчанию для всех компонентов
- ✅ **Жизненный цикл**: Firing → Acknowledged → Resolved
- ✅ **Статистика**: MTTR, частота алертов, распределение по компонентам

#### "Добавить структурированное логирование с контекстом для отладки"
- ✅ **StructuredLogger interface**: 15+ методов для контекстного логирования
- ✅ **Контекстная информация**: TraceID, SpanID, RequestID, UserID
- ✅ **Автоматические поля**: Timestamp, Caller, Hostname, PID, Goroutine
- ✅ **Производительное логирование**: Специальные методы для метрик, узких мест, алертов
- ✅ **Расширяемость**: Hook система и множественные выводы

#### "Создать механизм трассировки запросов при превышении SLA"
- ✅ **RequestTracer interface**: Полный жизненный цикл трассировки
- ✅ **SLA мониторинг**: Автоматическая проверка 4 типов SLA
- ✅ **Интеграция с алертами**: TraceID добавляется к алертам
- ✅ **Контекстное логирование**: Автоматическое логирование нарушений SLA

#### "Написать интеграционные тесты для проверки системы алертов"
- ✅ **AlertManager тесты**: 6 интеграционных тестов с mock компонентами
- ✅ **Notifiers тесты**: Тесты для всех 4 типов уведомителей
- ✅ **StructuredLogger тесты**: 8 тестовых сценариев с mock выводами
- ✅ **End-to-end тесты**: Полные сценарии от метрик до уведомлений

## Итоговая статистика

### Файлы и размеры:
1. `alert_manager.go` - ~1200 строк (система управления алертами)
2. `alert_notifiers.go` - ~800 строк (каналы уведомлений)
3. `structured_logger.go` - ~900 строк (структурированное логирование)
4. `alert_manager_test.go` - ~600 строк (интеграционные тесты AlertManager)
5. `alert_notifiers_test.go` - ~700 строк (тесты уведомителей)
6. `structured_logger_test.go` - ~800 строк (тесты логирования)

**Общий объем**: ~5000 строк высококачественного Go кода

### Функциональность:
- **Типы алертов**: 5 правил по умолчанию + настраиваемые
- **Каналы уведомлений**: 4 типа (Log, Webhook, Email, Slack)
- **Контекстное логирование**: 15+ методов с автоматическим контекстом
- **SLA мониторинг**: 4 типа SLA с автоматической трассировкой
- **Интеграционные тесты**: 20+ тестовых сценариев

### Качественные показатели:
- **Надежность**: Graceful error handling и recovery
- **Производительность**: Асинхронные уведомления и эффективное логирование
- **Наблюдаемость**: Self-monitoring и rich context
- **Расширяемость**: Pluggable архитектура для всех компонентов
- **Интеграция**: Seamless взаимодействие между всеми компонентами

### Практическая ценность:
- **Проактивный мониторинг**: Автоматическое обнаружение проблем
- **Быстрое реагирование**: Мгновенные уведомления через множественные каналы
- **Эффективная отладка**: Структурированные логи с полным контекстом
- **SLA соблюдение**: Автоматическая трассировка при нарушениях
- **Операционная эффективность**: Статистика и аналитика для улучшения процессов

Комплексная система алертов и уведомлений обеспечивает полную наблюдаемость и быстрое реагирование на проблемы производительности в системе Boxo, значительно улучшая операционную эффективность и надежность.