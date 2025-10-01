# Task 6 Deployment Diagram - Подробное объяснение

## Обзор диаграммы
Диаграмма развертывания (C4 Deployment) показывает физическое развертывание Task 6 в производственной среде, включая инфраструктуру, конфигурацию, ограничения ресурсов и интеграцию с внешними системами мониторинга.

## Связь с реализацией кода

### 🖥️ **Production Server Environment**

#### Boxo IPFS Node Container
**Архитектурное представление:**
```
Deployment_Node(boxo_container, "Boxo IPFS Node", "Docker Container") {
    Container(boxo_app, "Boxo Application", "Go Binary", "Main IPFS node with Task 6 integration")
    
    Container_Boundary(task6_modules, "Task 6 Modules") {
        Container(resource_monitor, "Resource Monitor", "Go Module", "CPU/Memory/Disk monitoring")
        Container(graceful_degradation, "Graceful Degradation", "Go Module", "Service quality management")
        Container(auto_scaler, "Auto Scaler", "Go Module", "Component scaling")
    }
}
```

**Реализация в коде:**
```go
// main.go - Основное приложение Boxo с интеграцией Task 6
package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "github.com/ipfs/boxo/monitoring"
)

func main() {
    // Инициализация логгера
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))

    // Создание контекста для graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Инициализация Task 6 модулей
    task6System, err := initializeTask6System(logger)
    if err != nil {
        logger.Error("Failed to initialize Task 6 system", slog.String("error", err.Error()))
        os.Exit(1)
    }

    // Запуск Task 6 системы
    if err := task6System.Start(ctx); err != nil {
        logger.Error("Failed to start Task 6 system", slog.String("error", err.Error()))
        os.Exit(1)
    }

    // Инициализация основного Boxo IPFS узла
    ipfsNode, err := initializeBoxoNode(ctx, task6System)
    if err != nil {
        logger.Error("Failed to initialize Boxo node", slog.String("error", err.Error()))
        os.Exit(1)
    }

    // Запуск IPFS узла
    if err := ipfsNode.Start(ctx); err != nil {
        logger.Error("Failed to start Boxo node", slog.String("error", err.Error()))
        os.Exit(1)
    }

    logger.Info("Boxo IPFS Node with Task 6 started successfully")

    // Graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    logger.Info("Shutting down...")
    
    // Остановка компонентов
    if err := ipfsNode.Stop(); err != nil {
        logger.Error("Error stopping Boxo node", slog.String("error", err.Error()))
    }
    
    if err := task6System.Stop(); err != nil {
        logger.Error("Error stopping Task 6 system", slog.String("error", err.Error()))
    }

    logger.Info("Shutdown complete")
}

// Инициализация системы Task 6
func initializeTask6System(logger *slog.Logger) (*Task6System, error) {
    // Чтение конфигурации из переменных окружения
    config := &Task6Config{
        MonitoringInterval:    getEnvDuration("TASK6_MONITORING_INTERVAL", 30*time.Second),
        CPUThreshold:         getEnvFloat("TASK6_CPU_THRESHOLD", 80.0),
        MemoryThreshold:      getEnvFloat("TASK6_MEMORY_THRESHOLD", 85.0),
        DegradationEnabled:   getEnvBool("TASK6_DEGRADATION_ENABLED", true),
        AutoScalingEnabled:   getEnvBool("TASK6_AUTOSCALING_ENABLED", true),
    }

    // Создание Performance Monitor
    performanceMonitor := monitoring.NewPerformanceMonitor()

    // Создание Resource Monitor
    resourceCollector := &monitoring.ResourceCollector{}
    thresholds := monitoring.ResourceMonitorThresholds{
        CPUWarning:        config.CPUThreshold,
        CPUCritical:       config.CPUThreshold + 10,
        MemoryWarning:     config.MemoryThreshold / 100,
        MemoryCritical:    (config.MemoryThreshold + 10) / 100,
        DiskWarning:       0.85,
        DiskCritical:      0.95,
        GoroutineWarning:  10000,
        GoroutineCritical: 50000,
    }
    
    resourceMonitor := monitoring.NewResourceMonitor(resourceCollector, thresholds)
    resourceMonitor.SetMonitorInterval(config.MonitoringInterval)

    // Создание Graceful Degradation Manager
    var degradationManager monitoring.GracefulDegradationManager
    if config.DegradationEnabled {
        degradationManager = monitoring.NewGracefulDegradationManager(
            performanceMonitor,
            resourceMonitor,
            logger,
        )
    }

    // Создание Auto Scaler
    var autoScaler monitoring.AutoScaler
    if config.AutoScalingEnabled {
        autoScaler = monitoring.NewAutoScaler(
            performanceMonitor,
            resourceMonitor,
            logger,
        )
    }

    return &Task6System{
        PerformanceMonitor: performanceMonitor,
        ResourceMonitor:    resourceMonitor,
        DegradationManager: degradationManager,
        AutoScaler:         autoScaler,
        Config:             config,
        Logger:             logger,
    }, nil
}

// Конфигурация Task 6 из переменных окружения
type Task6Config struct {
    MonitoringInterval  time.Duration
    CPUThreshold       float64
    MemoryThreshold    float64
    DegradationEnabled bool
    AutoScalingEnabled bool
}

// Система Task 6
type Task6System struct {
    PerformanceMonitor monitoring.PerformanceMonitor
    ResourceMonitor    *monitoring.ResourceMonitor
    DegradationManager monitoring.GracefulDegradationManager
    AutoScaler         monitoring.AutoScaler
    Config             *Task6Config
    Logger             *slog.Logger
}

func (t6 *Task6System) Start(ctx context.Context) error {
    // Запуск Performance Monitor
    if err := t6.PerformanceMonitor.StartCollection(ctx, t6.Config.MonitoringInterval); err != nil {
        return fmt.Errorf("failed to start performance monitor: %w", err)
    }

    // Запуск Resource Monitor
    if err := t6.ResourceMonitor.Start(ctx); err != nil {
        return fmt.Errorf("failed to start resource monitor: %w", err)
    }

    // Запуск Graceful Degradation Manager
    if t6.DegradationManager != nil {
        if err := t6.DegradationManager.Start(ctx); err != nil {
            return fmt.Errorf("failed to start degradation manager: %w", err)
        }
    }

    // Запуск Auto Scaler
    if t6.AutoScaler != nil {
        if err := t6.AutoScaler.Start(ctx); err != nil {
            return fmt.Errorf("failed to start auto scaler: %w", err)
        }
    }

    t6.Logger.Info("Task 6 system started successfully")
    return nil
}

func (t6 *Task6System) Stop() error {
    // Остановка компонентов в обратном порядке
    if t6.AutoScaler != nil {
        if err := t6.AutoScaler.Stop(); err != nil {
            t6.Logger.Error("Error stopping auto scaler", slog.String("error", err.Error()))
        }
    }

    if t6.DegradationManager != nil {
        if err := t6.DegradationManager.Stop(); err != nil {
            t6.Logger.Error("Error stopping degradation manager", slog.String("error", err.Error()))
        }
    }

    t6.ResourceMonitor.Stop()
    
    if err := t6.PerformanceMonitor.StopCollection(); err != nil {
        t6.Logger.Error("Error stopping performance monitor", slog.String("error", err.Error()))
    }

    t6.Logger.Info("Task 6 system stopped")
    return nil
}

// Вспомогательные функции для чтения переменных окружения
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
    if value := os.Getenv(key); value != "" {
        if duration, err := time.ParseDuration(value); err == nil {
            return duration
        }
    }
    return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
    if value := os.Getenv(key); value != "" {
        if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
            return floatValue
        }
    }
    return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
    if value := os.Getenv(key); value != "" {
        if boolValue, err := strconv.ParseBool(value); err == nil {
            return boolValue
        }
    }
    return defaultValue
}
```

#### Docker Configuration
**Архитектурное представление:**
```
note right of boxo_container
  **Resource Limits:**
  - CPU: 4 cores
  - Memory: 8GB
  - Disk: 100GB SSD
  
  **Environment Variables:**
  - TASK6_MONITORING_INTERVAL=30s
  - TASK6_CPU_THRESHOLD=80%
  - TASK6_MEMORY_THRESHOLD=85%
  - TASK6_DEGRADATION_ENABLED=true
  - TASK6_AUTOSCALING_ENABLED=true
end note
```

**Реализация в коде:**
```dockerfile
# Dockerfile для Boxo IPFS Node с Task 6
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Копирование исходного кода
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Сборка приложения
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o boxo-node ./cmd/boxo-node

FROM alpine:latest

# Установка необходимых пакетов
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Копирование бинарного файла
COPY --from=builder /app/boxo-node .

# Создание пользователя для безопасности
RUN adduser -D -s /bin/sh boxo
USER boxo

# Переменные окружения по умолчанию
ENV TASK6_MONITORING_INTERVAL=30s
ENV TASK6_CPU_THRESHOLD=80.0
ENV TASK6_MEMORY_THRESHOLD=85.0
ENV TASK6_DEGRADATION_ENABLED=true
ENV TASK6_AUTOSCALING_ENABLED=true

# Порты
EXPOSE 4001 5001 8080 9090

# Команда запуска
CMD ["./boxo-node"]
```

```yaml
# docker-compose.yml для развертывания
version: '3.8'

services:
  boxo-node:
    build: .
    container_name: boxo-ipfs-node
    restart: unless-stopped
    
    # Ограничения ресурсов
    deploy:
      resources:
        limits:
          cpus: '4.0'
          memory: 8G
        reservations:
          cpus: '1.0'
          memory: 2G
    
    # Переменные окружения
    environment:
      - TASK6_MONITORING_INTERVAL=30s
      - TASK6_CPU_THRESHOLD=80.0
      - TASK6_MEMORY_THRESHOLD=85.0
      - TASK6_DEGRADATION_ENABLED=true
      - TASK6_AUTOSCALING_ENABLED=true
      - PROMETHEUS_ENDPOINT=http://prometheus:9090
      - LOG_LEVEL=info
    
    # Порты
    ports:
      - "4001:4001"     # IPFS Swarm
      - "5001:5001"     # IPFS API
      - "8080:8080"     # IPFS Gateway
      - "9090:9090"     # Metrics endpoint
    
    # Volumes для персистентности
    volumes:
      - boxo-data:/root/.ipfs
      - ./config:/root/config:ro
    
    # Healthcheck
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:5001/api/v0/id"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    
    # Зависимости
    depends_on:
      - prometheus
      - grafana
    
    # Сети
    networks:
      - monitoring
      - ipfs

volumes:
  boxo-data:
    driver: local

networks:
  monitoring:
    driver: bridge
  ipfs:
    driver: bridge
```

### 🗄️ **Data Storage**

#### Metrics Cache
**Архитектурное представление:**
```
ContainerDb(metrics_cache, "Metrics Cache", "In-Memory", "Recent performance data")
```

**Реализация в коде:**
```go
// Внутренняя структура для кеширования метрик
type MetricsCache struct {
    mu      sync.RWMutex
    data    map[string]*CachedMetric
    maxSize int
    ttl     time.Duration
}

type CachedMetric struct {
    Value     interface{}
    Timestamp time.Time
    TTL       time.Duration
}

func NewMetricsCache(maxSize int, ttl time.Duration) *MetricsCache {
    cache := &MetricsCache{
        data:    make(map[string]*CachedMetric),
        maxSize: maxSize,
        ttl:     ttl,
    }
    
    // Запуск очистки устаревших данных
    go cache.cleanupExpired()
    
    return cache
}

func (mc *MetricsCache) Set(key string, value interface{}) {
    mc.mu.Lock()
    defer mc.mu.Unlock()
    
    // Проверка размера кеша
    if len(mc.data) >= mc.maxSize {
        mc.evictOldest()
    }
    
    mc.data[key] = &CachedMetric{
        Value:     value,
        Timestamp: time.Now(),
        TTL:       mc.ttl,
    }
}

func (mc *MetricsCache) Get(key string) (interface{}, bool) {
    mc.mu.RLock()
    defer mc.mu.RUnlock()
    
    metric, exists := mc.data[key]
    if !exists {
        return nil, false
    }
    
    // Проверка TTL
    if time.Since(metric.Timestamp) > metric.TTL {
        delete(mc.data, key)
        return nil, false
    }
    
    return metric.Value, true
}

// Использование в PerformanceMonitor
type performanceMonitor struct {
    cache       *MetricsCache
    collectors  map[string]MetricCollector
    registry    *prometheus.Registry
}

func (pm *performanceMonitor) CollectMetrics() *PerformanceMetrics {
    // Проверка кеша
    if cached, exists := pm.cache.Get("current_metrics"); exists {
        if metrics, ok := cached.(*PerformanceMetrics); ok {
            return metrics
        }
    }
    
    // Сбор новых метрик
    metrics := &PerformanceMetrics{
        Timestamp: time.Now(),
    }
    
    // Сбор от всех коллекторов
    for name, collector := range pm.collectors {
        collectedMetrics := collector.CollectMetrics()
        // Обработка метрик...
    }
    
    // Кеширование результата
    pm.cache.Set("current_metrics", metrics)
    
    return metrics
}
```

#### Configuration Store
**Архитектурное представление:**
```
ContainerDb(config_files, "Configuration", "JSON/YAML Files", "Thresholds, rules, and settings")
```

**Реализация в коде:**
```go
// config/task6.yaml - Конфигурационный файл
monitoring:
  interval: 30s
  
resource_thresholds:
  cpu:
    warning: 70.0
    critical: 90.0
  memory:
    warning: 80.0
    critical: 95.0
  disk:
    warning: 85.0
    critical: 95.0
  goroutines:
    warning: 10000
    critical: 50000

degradation:
  enabled: true
  rules:
    - id: "cpu-overload-light"
      name: "CPU Overload Light"
      component: "resource"
      metric: "cpu_usage"
      condition: "greater_than"
      threshold: 75.0
      level: "light"
      actions: ["reduce_worker_threads", "increase_batch_timeout"]
      priority: 100
      enabled: true
    
    - id: "memory-pressure-critical"
      name: "Memory Pressure Critical"
      component: "resource"
      metric: "memory_usage_percent"
      condition: "greater_than"
      threshold: 95.0
      level: "critical"
      actions: ["emergency_cache_clear", "disable_non_essential_services"]
      priority: 300
      enabled: true

scaling:
  enabled: true
  rules:
    - id: "worker-threads-scale-up"
      name: "Worker Threads Scale Up"
      component: "worker_pool"
      metric: "cpu_usage"
      condition: "greater_than"
      threshold: 70.0
      direction: "up"
      scale_factor: 1.5
      min_value: 64
      max_value: 2048
      cooldown: "2m"
      priority: 100
      enabled: true

// Загрузка конфигурации
type ConfigLoader struct {
    configPath string
}

func NewConfigLoader(configPath string) *ConfigLoader {
    return &ConfigLoader{configPath: configPath}
}

func (cl *ConfigLoader) LoadTask6Config() (*Task6Config, error) {
    data, err := os.ReadFile(cl.configPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %w", err)
    }
    
    var config Task6Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }
    
    return &config, nil
}

// Структуры конфигурации
type Task6Config struct {
    Monitoring   MonitoringConfig   `yaml:"monitoring"`
    Thresholds   ThresholdsConfig   `yaml:"resource_thresholds"`
    Degradation  DegradationConfig  `yaml:"degradation"`
    Scaling      ScalingConfig      `yaml:"scaling"`
}

type MonitoringConfig struct {
    Interval time.Duration `yaml:"interval"`
}

type ThresholdsConfig struct {
    CPU        ThresholdPair `yaml:"cpu"`
    Memory     ThresholdPair `yaml:"memory"`
    Disk       ThresholdPair `yaml:"disk"`
    Goroutines struct {
        Warning  int `yaml:"warning"`
        Critical int `yaml:"critical"`
    } `yaml:"goroutines"`
}

type ThresholdPair struct {
    Warning  float64 `yaml:"warning"`
    Critical float64 `yaml:"critical"`
}
```

### 📊 **Monitoring Stack**

#### Prometheus Integration
**Архитектурное представление:**
```
Container(prometheus, "Prometheus", "Time-series DB", "Metrics collection and storage")
Rel(resource_monitor, prometheus, "Exports metrics", "HTTP :9090")
```

**Реализация в коде:**
```go
// Интеграция с Prometheus
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus метрики для Task 6
type PrometheusMetrics struct {
    // Resource Monitor метрики
    CPUUsage        prometheus.GaugeVec
    MemoryUsage     prometheus.GaugeVec
    DiskUsage       prometheus.GaugeVec
    GoroutineCount  prometheus.Gauge
    
    // Degradation метрики
    DegradationLevel    prometheus.Gauge
    ActiveDegradations  prometheus.GaugeVec
    DegradationEvents   prometheus.CounterVec
    
    // Scaling метрики
    ComponentScales     prometheus.GaugeVec
    ScalingEvents       prometheus.CounterVec
    IsolatedComponents  prometheus.Gauge
}

func NewPrometheusMetrics() *PrometheusMetrics {
    return &PrometheusMetrics{
        CPUUsage: *prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "task6_cpu_usage_percent",
                Help: "Current CPU usage percentage",
            },
            []string{"instance"},
        ),
        
        MemoryUsage: *prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "task6_memory_usage_bytes",
                Help: "Current memory usage in bytes",
            },
            []string{"instance", "type"},
        ),
        
        DegradationLevel: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "task6_degradation_level",
                Help: "Current degradation level (0=none, 1=light, 2=moderate, 3=severe, 4=critical)",
            },
        ),
        
        ComponentScales: *prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "task6_component_scale",
                Help: "Current scale of components",
            },
            []string{"component"},
        ),
        
        ScalingEvents: *prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "task6_scaling_events_total",
                Help: "Total number of scaling events",
            },
            []string{"component", "direction", "success"},
        ),
    }
}

// Регистрация метрик
func (pm *PrometheusMetrics) Register(registry *prometheus.Registry) {
    registry.MustRegister(
        pm.CPUUsage,
        pm.MemoryUsage,
        pm.DegradationLevel,
        pm.ComponentScales,
        pm.ScalingEvents,
    )
}

// Обновление метрик в Resource Monitor
func (rm *ResourceMonitor) updatePrometheusMetrics() {
    status := rm.GetCurrentStatus()
    
    // Обновление CPU метрик
    if cpuUsage, ok := status["cpu_usage_percent"].(float64); ok {
        rm.prometheusMetrics.CPUUsage.WithLabelValues("main").Set(cpuUsage)
    }
    
    // Обновление Memory метрик
    if memoryUsage, ok := status["memory_usage"].(uint64); ok {
        rm.prometheusMetrics.MemoryUsage.WithLabelValues("main", "used").Set(float64(memoryUsage))
    }
    
    // Обновление Goroutine метрик
    if goroutines, ok := status["goroutines"].(int); ok {
        rm.prometheusMetrics.GoroutineCount.Set(float64(goroutines))
    }
}

// HTTP endpoint для метрик
func (t6 *Task6System) StartMetricsServer(port int) error {
    http.Handle("/metrics", promhttp.HandlerFor(
        t6.PerformanceMonitor.GetPrometheusRegistry(),
        promhttp.HandlerOpts{},
    ))
    
    server := &http.Server{
        Addr:    fmt.Sprintf(":%d", port),
        Handler: nil,
    }
    
    go func() {
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            t6.Logger.Error("Metrics server error", slog.String("error", err.Error()))
        }
    }()
    
    t6.Logger.Info("Metrics server started", slog.Int("port", port))
    return nil
}
```

#### Prometheus Configuration
```yaml
# prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "task6_alerts.yml"

scrape_configs:
  - job_name: 'boxo-task6'
    static_configs:
      - targets: ['boxo-node:9090']
    scrape_interval: 15s
    metrics_path: /metrics
    
alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093

# task6_alerts.yml - Правила алертов
groups:
  - name: task6_alerts
    rules:
      - alert: HighCPUUsage
        expr: task6_cpu_usage_percent > 85
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High CPU usage detected"
          description: "CPU usage is {{ $value }}% for more than 2 minutes"
      
      - alert: CriticalMemoryUsage
        expr: (task6_memory_usage_bytes / task6_memory_total_bytes) * 100 > 90
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Critical memory usage"
          description: "Memory usage is {{ $value }}% for more than 1 minute"
      
      - alert: DegradationActive
        expr: task6_degradation_level > 0
        for: 0s
        labels:
          severity: warning
        annotations:
          summary: "Service degradation active"
          description: "Degradation level is {{ $value }}"
```

### 🔗 **External Services Integration**

#### Slack Notifications
**Архитектурное представление:**
```
Container(slack, "Slack", "Notification Service", "Team notifications")
Rel(alertmanager, slack, "Notifications", "Webhook")
```

**Реализация в коде:**
```go
// Slack интеграция для уведомлений
type SlackNotifier struct {
    webhookURL string
    channel    string
    username   string
}

func NewSlackNotifier(webhookURL, channel, username string) *SlackNotifier {
    return &SlackNotifier{
        webhookURL: webhookURL,
        channel:    channel,
        username:   username,
    }
}

type SlackMessage struct {
    Channel   string       `json:"channel"`
    Username  string       `json:"username"`
    Text      string       `json:"text"`
    Attachments []SlackAttachment `json:"attachments,omitempty"`
}

type SlackAttachment struct {
    Color     string `json:"color"`
    Title     string `json:"title"`
    Text      string `json:"text"`
    Timestamp int64  `json:"ts"`
}

func (sn *SlackNotifier) SendAlert(alert ResourceAlert) error {
    color := "warning"
    if alert.Level == "critical" {
        color = "danger"
    }
    
    message := SlackMessage{
        Channel:  sn.channel,
        Username: sn.username,
        Text:     "Task 6 Alert",
        Attachments: []SlackAttachment{
            {
                Color:     color,
                Title:     fmt.Sprintf("%s Alert: %s", strings.Title(alert.Level), alert.Type),
                Text:      alert.Message,
                Timestamp: alert.Timestamp.Unix(),
            },
        },
    }
    
    jsonData, err := json.Marshal(message)
    if err != nil {
        return fmt.Errorf("failed to marshal slack message: %w", err)
    }
    
    resp, err := http.Post(sn.webhookURL, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("failed to send slack notification: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("slack API returned status %d", resp.StatusCode)
    }
    
    return nil
}

// Интеграция с Resource Monitor
func (rm *ResourceMonitor) SetupSlackNotifications(webhookURL, channel string) {
    slackNotifier := NewSlackNotifier(webhookURL, channel, "Task6-Bot")
    
    rm.SetAlertCallback(func(alert ResourceAlert) {
        if err := slackNotifier.SendAlert(alert); err != nil {
            rm.logger.Error("Failed to send Slack notification",
                slog.String("error", err.Error()),
                slog.String("alert_type", alert.Type))
        }
    })
}
```

#### Log Aggregation
**Архитектурное представление:**
```
Container(log_aggregator, "Log Aggregator", "ELK/Splunk", "Centralized logging")
Rel(boxo_app, log_aggregator, "Structured logs", "TCP/UDP")
```

**Реализация в коде:**
```go
// Структурированное логирование с отправкой в внешние системы
import (
    "log/slog"
    "net"
    "encoding/json"
)

type LogAggregatorHandler struct {
    conn   net.Conn
    local  slog.Handler
}

func NewLogAggregatorHandler(address string, localHandler slog.Handler) (*LogAggregatorHandler, error) {
    conn, err := net.Dial("tcp", address)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to log aggregator: %w", err)
    }
    
    return &LogAggregatorHandler{
        conn:  conn,
        local: localHandler,
    }, nil
}

func (lah *LogAggregatorHandler) Handle(ctx context.Context, record slog.Record) error {
    // Отправка в локальный handler
    if err := lah.local.Handle(ctx, record); err != nil {
        return err
    }
    
    // Подготовка для отправки в агрегатор
    logEntry := map[string]interface{}{
        "timestamp": record.Time.Format(time.RFC3339),
        "level":     record.Level.String(),
        "message":   record.Message,
        "service":   "boxo-task6",
    }
    
    // Добавление атрибутов
    record.Attrs(func(attr slog.Attr) bool {
        logEntry[attr.Key] = attr.Value.Any()
        return true
    })
    
    // Отправка в агрегатор
    jsonData, err := json.Marshal(logEntry)
    if err != nil {
        return err
    }
    
    _, err = lah.conn.Write(append(jsonData, '\n'))
    return err
}

// Настройка логирования с агрегацией
func setupLogging(logAggregatorAddress string) *slog.Logger {
    // Локальный handler
    localHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })
    
    // Handler с агрегацией
    if logAggregatorAddress != "" {
        if aggregatorHandler, err := NewLogAggregatorHandler(logAggregatorAddress, localHandler); err == nil {
            return slog.New(aggregatorHandler)
        }
    }
    
    return slog.New(localHandler)
}
```

### 🔧 **Health Checks and Monitoring**

**Архитектурное представление:**
```
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:5001/api/v0/id"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 40s
```

**Реализация в коде:**
```go
// Health check endpoint
func (t6 *Task6System) SetupHealthChecks() {
    http.HandleFunc("/health", t6.healthCheckHandler)
    http.HandleFunc("/ready", t6.readinessCheckHandler)
}

func (t6 *Task6System) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
    health := map[string]interface{}{
        "status":    "healthy",
        "timestamp": time.Now().Format(time.RFC3339),
        "components": map[string]interface{}{
            "resource_monitor":    t6.ResourceMonitor != nil,
            "degradation_manager": t6.DegradationManager != nil,
            "auto_scaler":        t6.AutoScaler != nil,
        },
    }
    
    // Проверка состояния компонентов
    if t6.ResourceMonitor != nil {
        resourceHealth := t6.ResourceMonitor.GetHealthStatus()
        health["resource_health"] = resourceHealth["health"]
        
        if resourceHealth["health"] == "critical" {
            health["status"] = "unhealthy"
            w.WriteHeader(http.StatusServiceUnavailable)
        }
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(health)
}

func (t6 *Task6System) readinessCheckHandler(w http.ResponseWriter, r *http.Request) {
    ready := map[string]interface{}{
        "status":    "ready",
        "timestamp": time.Now().Format(time.RFC3339),
    }
    
    // Проверка готовности всех компонентов
    allReady := true
    
    if t6.DegradationManager != nil {
        degradationStatus := t6.DegradationManager.GetDegradationStatus()
        if degradationStatus.Level >= DegradationSevere {
            allReady = false
            ready["degradation_level"] = degradationStatus.Level.String()
        }
    }
    
    if !allReady {
        ready["status"] = "not_ready"
        w.WriteHeader(http.StatusServiceUnavailable)
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(ready)
}
```

## Ключевые особенности развертывания

### 1. **Контейнеризация**
- Docker контейнер с ограничениями ресурсов
- Переменные окружения для конфигурации
- Health checks для мониторинга состояния
- Graceful shutdown для корректной остановки

### 2. **Конфигурируемость**
- Внешние конфигурационные файлы
- Переменные окружения для runtime настроек
- Динамическая перезагрузка конфигурации
- Валидация конфигурации при запуске

### 3. **Наблюдаемость**
- Экспорт метрик в Prometheus
- Структурированное логирование
- Health check endpoints
- Интеграция с внешними системами мониторинга

### 4. **Отказоустойчивость**
- Автоматический restart при сбоях
- Graceful degradation при перегрузке
- Изоляция проблемных компонентов
- Резервирование ресурсов

### 5. **Безопасность**
- Запуск от непривилегированного пользователя
- Ограничения ресурсов контейнера
- Сетевая изоляция
- Минимальный базовый образ

Эта диаграмма развертывания показывает, как Task 6 интегрируется в производственную среду, обеспечивая надежную работу, мониторинг и управление ресурсами Boxo IPFS узла.