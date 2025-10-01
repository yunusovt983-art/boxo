# Configuration Guide - Руководство по конфигурации AutoTuner

## 📋 Обзор конфигурации

AutoTuner использует многоуровневую систему конфигурации, позволяющую гибко настраивать все аспекты автоматического тюнинга параметров.

### Иерархия конфигурации

```
1. Значения по умолчанию (встроенные в код)
2. Основной конфигурационный файл (config.yml)
3. Переменные окружения
4. Параметры командной строки
5. Runtime конфигурация (через API)
```

## 🔧 Основная конфигурация

### Полный пример конфигурации

```yaml
# config.yml - Основная конфигурация AutoTuner
autotuner:
  # Общие настройки
  enabled: true
  log_level: "info"
  tuning_interval: "15m"
  max_concurrent_tunings: 1
  
  # Настройки сервера
  server:
    host: "0.0.0.0"
    port: 8080
    metrics_port: 9090
    health_check_path: "/health"
    metrics_path: "/metrics"
    
  # Настройки базы данных
  database:
    type: "sqlite"  # sqlite, postgres, mysql
    connection_string: "/var/lib/autotuner/autotuner.db"
    max_connections: 10
    connection_timeout: "30s"
    
  # ML настройки
  ml:
    enabled: true
    model_type: "random_forest"  # random_forest, linear_regression, ensemble
    model_path: "/var/lib/autotuner/models"
    training_window: "24h"
    retrain_interval: "6h"
    min_confidence: 0.8
    feature_window: "1h"
    
    # Настройки Random Forest
    random_forest:
      n_estimators: 100
      max_depth: 10
      min_samples_split: 5
      min_samples_leaf: 2
      max_features: "sqrt"
      bootstrap: true
      random_state: 42
      
    # Настройки обучения
    training:
      test_size: 0.2
      validation_size: 0.1
      cross_validation_folds: 5
      early_stopping_patience: 10
      
    # Настройки признаков
    features:
      temporal_window: "1h"
      aggregation_functions: ["mean", "std", "min", "max", "p95"]
      lag_features: [1, 5, 15, 30]  # минуты
      
  # Настройки безопасности
  safety:
    enabled: true
    backup_retention: "7d"
    max_backup_size: "1GB"
    
    # Canary deployment
    canary:
      enabled: true
      initial_percentage: 10
      max_percentage: 50
      step_size: 10
      step_duration: "5m"
      success_threshold: 0.9
      failure_threshold: 0.7
      
    # Rollback настройки
    rollback:
      enabled: true
      timeout: "5m"
      max_attempts: 3
      health_check_interval: "30s"
      
    # Rate limiting
    rate_limiting:
      max_changes_per_hour: 3
      max_changes_per_day: 10
      cooldown_period: "10m"
      
  # Настройки оптимизации
  optimization:
    algorithm: "genetic"  # genetic, simulated_annealing, bayesian
    
    # Genetic Algorithm
    genetic:
      population_size: 50
      generations: 100
      mutation_rate: 0.1
      crossover_rate: 0.8
      selection_method: "tournament"
      tournament_size: 3
      elitism_rate: 0.1
      
    # Simulated Annealing
    simulated_annealing:
      initial_temperature: 1000
      cooling_rate: 0.95
      min_temperature: 0.01
      max_iterations: 1000
      
    # Bayesian Optimization
    bayesian:
      acquisition_function: "expected_improvement"
      n_initial_points: 10
      n_calls: 100
      
    # Целевые функции
    objectives:
      - name: "latency"
        weight: 0.4
        target: "minimize"
        threshold: 100  # ms
      - name: "throughput"
        weight: 0.3
        target: "maximize"
        threshold: 1000  # rps
      - name: "resource_usage"
        weight: 0.2
        target: "minimize"
        threshold: 0.8  # 80%
      - name: "error_rate"
        weight: 0.1
        target: "minimize"
        threshold: 0.01  # 1%
        
  # Параметры для тюнинга
  parameters:
    # Bitswap параметры
    bitswap_max_outstanding_bytes_per_peer:
      type: "int"
      min: 1048576      # 1MB
      max: 134217728    # 128MB
      default: 16777216 # 16MB
      step: 1048576     # 1MB
      description: "Maximum outstanding bytes per peer in Bitswap"
      
    bitswap_worker_pool_size:
      type: "int"
      min: 1
      max: 100
      default: 10
      step: 1
      description: "Size of Bitswap worker pool"
      
    bitswap_request_timeout:
      type: "duration"
      min: "1s"
      max: "300s"
      default: "30s"
      step: "1s"
      description: "Timeout for Bitswap requests"
      
    # Blockstore параметры
    blockstore_cache_size:
      type: "int"
      min: 67108864     # 64MB
      max: 2147483648   # 2GB
      default: 268435456 # 256MB
      step: 67108864    # 64MB
      description: "Size of blockstore cache"
      
    blockstore_bloom_filter_size:
      type: "int"
      min: 1000000      # 1M
      max: 100000000    # 100M
      default: 10000000 # 10M
      step: 1000000     # 1M
      description: "Size of blockstore bloom filter"
      
    # Network параметры
    network_connection_limit:
      type: "int"
      min: 100
      max: 10000
      default: 2000
      step: 100
      description: "Maximum number of network connections"
      
    network_buffer_size:
      type: "int"
      min: 4096         # 4KB
      max: 1048576      # 1MB
      default: 65536    # 64KB
      step: 4096        # 4KB
      description: "Network buffer size"
      
  # Мониторинг и метрики
  monitoring:
    enabled: true
    metrics_interval: "30s"
    health_check_interval: "10s"
    
    # Prometheus настройки
    prometheus:
      enabled: true
      namespace: "autotuner"
      subsystem: ""
      
    # Логирование
    logging:
      level: "info"
      format: "json"  # json, text
      output: "stdout"  # stdout, file
      file_path: "/var/log/autotuner/autotuner.log"
      max_size: "100MB"
      max_backups: 5
      max_age: "30d"
      compress: true
      
  # Алерты
  alerting:
    enabled: true
    
    # Slack интеграция
    slack:
      enabled: true
      webhook_url: "${SLACK_WEBHOOK_URL}"
      channel: "#autotuner-alerts"
      username: "AutoTuner"
      
    # Email интеграция
    email:
      enabled: false
      smtp_server: "smtp.example.com"
      smtp_port: 587
      username: "${EMAIL_USERNAME}"
      password: "${EMAIL_PASSWORD}"
      from: "autotuner@example.com"
      to: ["admin@example.com"]
      
    # PagerDuty интеграция
    pagerduty:
      enabled: false
      integration_key: "${PAGERDUTY_INTEGRATION_KEY}"
      
    # Правила алертов
    rules:
      - name: "high_failure_rate"
        condition: "failure_rate > 0.2"
        severity: "warning"
        cooldown: "5m"
        
      - name: "rollback_executed"
        condition: "rollback_count > 0"
        severity: "error"
        cooldown: "1m"
        
      - name: "ml_model_accuracy_low"
        condition: "ml_accuracy < 0.7"
        severity: "warning"
        cooldown: "1h"
```

## 🌍 Переменные окружения

### Основные переменные

```bash
# Основные настройки
AUTOTUNER_ENABLED=true
AUTOTUNER_LOG_LEVEL=info
AUTOTUNER_CONFIG_PATH=/etc/autotuner/config.yml

# Сервер
AUTOTUNER_HOST=0.0.0.0
AUTOTUNER_PORT=8080
AUTOTUNER_METRICS_PORT=9090

# База данных
AUTOTUNER_DB_TYPE=postgres
AUTOTUNER_DB_CONNECTION_STRING=postgres://user:pass@localhost/autotuner

# ML настройки
AUTOTUNER_ML_MODEL_PATH=/var/lib/autotuner/models
AUTOTUNER_ML_MIN_CONFIDENCE=0.8

# Безопасность
AUTOTUNER_SAFETY_ENABLED=true
AUTOTUNER_CANARY_ENABLED=true
AUTOTUNER_ROLLBACK_TIMEOUT=5m

# Алерты
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/...
EMAIL_USERNAME=autotuner@example.com
EMAIL_PASSWORD=secret
PAGERDUTY_INTEGRATION_KEY=abc123...

# Мониторинг
AUTOTUNER_METRICS_ENABLED=true
AUTOTUNER_PROMETHEUS_ENABLED=true
```

## 🏭 Конфигурации для разных сред

### Development конфигурация

```yaml
# config-development.yml
autotuner:
  enabled: true
  log_level: "debug"
  tuning_interval: "5m"  # Более частые тюнинги для тестирования
  
  ml:
    min_confidence: 0.6  # Более низкий порог для экспериментов
    retrain_interval: "1h"
    training_window: "6h"
    
  safety:
    canary:
      initial_percentage: 50  # Больший процент для быстрого тестирования
      step_duration: "1m"
    rate_limiting:
      max_changes_per_hour: 10
      
  optimization:
    genetic:
      population_size: 20  # Меньше для быстроты
      generations: 50
      
  monitoring:
    metrics_interval: "10s"
    health_check_interval: "5s"
```

### Production конфигурация

```yaml
# config-production.yml
autotuner:
  enabled: true
  log_level: "warn"
  tuning_interval: "30m"  # Более консервативные интервалы
  
  ml:
    min_confidence: 0.9  # Высокий порог для продакшна
    retrain_interval: "12h"
    training_window: "7d"
    
  safety:
    canary:
      initial_percentage: 5   # Очень осторожное развертывание
      step_size: 5
      step_duration: "10m"
    rate_limiting:
      max_changes_per_hour: 1
      max_changes_per_day: 3
      
  optimization:
    genetic:
      population_size: 100  # Больше для лучшего качества
      generations: 200
      
  monitoring:
    metrics_interval: "60s"
    health_check_interval: "30s"
    
  alerting:
    enabled: true
    slack:
      enabled: true
    pagerduty:
      enabled: true
```

### Testing конфигурация

```yaml
# config-testing.yml
autotuner:
  enabled: true
  log_level: "debug"
  tuning_interval: "1m"
  
  database:
    type: "sqlite"
    connection_string: ":memory:"
    
  ml:
    enabled: false  # Отключаем ML для unit тестов
    
  safety:
    enabled: false  # Отключаем safety для быстрых тестов
    
  monitoring:
    enabled: false
    
  alerting:
    enabled: false
```

## 🎛️ Runtime конфигурация

### API для изменения конфигурации

```go
// REST API для управления конфигурацией
type ConfigAPI struct {
    configManager *ConfigManager
    validator     *ConfigValidator
}

// GET /api/v1/config
func (ca *ConfigAPI) GetConfig(w http.ResponseWriter, r *http.Request) {
    config := ca.configManager.GetCurrentConfig()
    json.NewEncoder(w).Encode(config)
}

// PUT /api/v1/config
func (ca *ConfigAPI) UpdateConfig(w http.ResponseWriter, r *http.Request) {
    var newConfig Config
    if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Валидация
    if err := ca.validator.Validate(&newConfig); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Применение
    if err := ca.configManager.UpdateConfig(&newConfig); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.WriteHeader(http.StatusOK)
}

// PATCH /api/v1/config/parameters/{name}
func (ca *ConfigAPI) UpdateParameter(w http.ResponseWriter, r *http.Request) {
    paramName := mux.Vars(r)["name"]
    
    var paramUpdate ParameterUpdate
    if err := json.NewDecoder(r.Body).Decode(&paramUpdate); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    if err := ca.configManager.UpdateParameter(paramName, &paramUpdate); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.WriteHeader(http.StatusOK)
}
```

### CLI для управления конфигурацией

```bash
# Просмотр текущей конфигурации
autotuner config show

# Обновление конфигурации из файла
autotuner config update --file config.yml

# Изменение отдельного параметра
autotuner config set ml.min_confidence 0.85

# Валидация конфигурации
autotuner config validate --file config.yml

# Экспорт конфигурации
autotuner config export --format yaml > current-config.yml

# Сброс к значениям по умолчанию
autotuner config reset

# Просмотр истории изменений
autotuner config history

# Откат к предыдущей конфигурации
autotuner config rollback --version 5
```

## 🔍 Валидация конфигурации

### Валидатор конфигурации

```go
type ConfigValidator struct {
    schema       *jsonschema.Schema
    customRules  []ValidationRule
}

func (cv *ConfigValidator) Validate(config *Config) error {
    // JSON Schema валидация
    if err := cv.schema.Validate(config); err != nil {
        return fmt.Errorf("schema validation failed: %w", err)
    }
    
    // Кастомные правила валидации
    for _, rule := range cv.customRules {
        if err := rule.Validate(config); err != nil {
            return fmt.Errorf("rule '%s' validation failed: %w", rule.Name(), err)
        }
    }
    
    return nil
}

// Правило: ML confidence должен быть разумным
type MLConfidenceRule struct{}

func (mcr *MLConfidenceRule) Name() string {
    return "ml_confidence_range"
}

func (mcr *MLConfidenceRule) Validate(config *Config) error {
    confidence := config.ML.MinConfidence
    if confidence < 0.5 || confidence > 1.0 {
        return fmt.Errorf("ML confidence must be between 0.5 and 1.0, got %.2f", confidence)
    }
    return nil
}

// Правило: Canary percentage должен быть разумным
type CanaryPercentageRule struct{}

func (cpr *CanaryPercentageRule) Name() string {
    return "canary_percentage_range"
}

func (cpr *CanaryPercentageRule) Validate(config *Config) error {
    initial := config.Safety.Canary.InitialPercentage
    max := config.Safety.Canary.MaxPercentage
    
    if initial < 1 || initial > 50 {
        return fmt.Errorf("canary initial percentage must be between 1 and 50, got %d", initial)
    }
    
    if max < initial || max > 100 {
        return fmt.Errorf("canary max percentage must be between initial (%d) and 100, got %d", initial, max)
    }
    
    return nil
}
```

### JSON Schema для конфигурации

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "AutoTuner Configuration",
  "type": "object",
  "properties": {
    "autotuner": {
      "type": "object",
      "properties": {
        "enabled": {
          "type": "boolean",
          "default": true
        },
        "log_level": {
          "type": "string",
          "enum": ["debug", "info", "warn", "error"],
          "default": "info"
        },
        "tuning_interval": {
          "type": "string",
          "pattern": "^[0-9]+(ns|us|µs|ms|s|m|h)$",
          "default": "15m"
        },
        "ml": {
          "type": "object",
          "properties": {
            "enabled": {
              "type": "boolean",
              "default": true
            },
            "model_type": {
              "type": "string",
              "enum": ["random_forest", "linear_regression", "ensemble"],
              "default": "random_forest"
            },
            "min_confidence": {
              "type": "number",
              "minimum": 0.0,
              "maximum": 1.0,
              "default": 0.8
            }
          },
          "required": ["enabled", "model_type", "min_confidence"]
        },
        "parameters": {
          "type": "object",
          "patternProperties": {
            "^[a-z_]+$": {
              "type": "object",
              "properties": {
                "type": {
                  "type": "string",
                  "enum": ["int", "float", "duration", "string", "bool"]
                },
                "min": {},
                "max": {},
                "default": {},
                "description": {
                  "type": "string"
                }
              },
              "required": ["type", "default"]
            }
          }
        }
      },
      "required": ["enabled", "ml", "parameters"]
    }
  },
  "required": ["autotuner"]
}
```

## 🔧 Настройка параметров

### Добавление нового параметра

```yaml
# Добавление нового параметра в конфигурацию
parameters:
  # Новый параметр
  bitswap_concurrent_requests:
    type: "int"
    min: 1
    max: 1000
    default: 100
    step: 10
    description: "Maximum concurrent requests in Bitswap"
    category: "performance"
    impact: "high"
    
    # Зависимости
    dependencies:
      - "bitswap_worker_pool_size"
      
    # Ограничения
    constraints:
      - condition: "value <= bitswap_worker_pool_size * 10"
        message: "Concurrent requests should not exceed 10x worker pool size"
        
    # Метаданные для ML
    ml_metadata:
      feature_importance: 0.7
      correlation_with: ["latency", "throughput"]
      optimization_priority: "high"
```

### Группировка параметров

```yaml
parameter_groups:
  performance:
    description: "Parameters affecting system performance"
    parameters:
      - "bitswap_max_outstanding_bytes_per_peer"
      - "bitswap_worker_pool_size"
      - "bitswap_concurrent_requests"
    optimization_weight: 0.8
    
  resource_usage:
    description: "Parameters affecting resource consumption"
    parameters:
      - "blockstore_cache_size"
      - "network_buffer_size"
    optimization_weight: 0.6
    
  stability:
    description: "Parameters affecting system stability"
    parameters:
      - "bitswap_request_timeout"
      - "network_connection_limit"
    optimization_weight: 0.9
```

## 📊 Мониторинг конфигурации

### Метрики конфигурации

```go
var (
    configChanges = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "autotuner_config_changes_total",
            Help: "Total number of configuration changes",
        },
        []string{"parameter", "source"},
    )
    
    configValidationErrors = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "autotuner_config_validation_errors_total",
            Help: "Total number of configuration validation errors",
        },
        []string{"rule", "parameter"},
    )
    
    activeParameterValues = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "autotuner_active_parameter_values",
            Help: "Current values of tuned parameters",
        },
        []string{"parameter"},
    )
)
```

### Аудит изменений

```go
type ConfigAudit struct {
    ID          string                 `json:"id"`
    Timestamp   time.Time             `json:"timestamp"`
    User        string                `json:"user"`
    Source      string                `json:"source"`  // api, cli, auto
    Action      string                `json:"action"`  // create, update, delete
    Parameter   string                `json:"parameter"`
    OldValue    interface{}           `json:"old_value"`
    NewValue    interface{}           `json:"new_value"`
    Reason      string                `json:"reason"`
    Metadata    map[string]interface{} `json:"metadata"`
}

func (cm *ConfigManager) auditChange(change *ConfigChange) {
    audit := &ConfigAudit{
        ID:        generateAuditID(),
        Timestamp: time.Now(),
        User:      change.User,
        Source:    change.Source,
        Action:    change.Action,
        Parameter: change.Parameter,
        OldValue:  change.OldValue,
        NewValue:  change.NewValue,
        Reason:    change.Reason,
        Metadata:  change.Metadata,
    }
    
    cm.auditLog.Record(audit)
    
    // Метрика
    configChanges.WithLabelValues(change.Parameter, change.Source).Inc()
}
```

Это руководство покрывает все аспекты конфигурации AutoTuner, от базовых настроек до продвинутых сценариев использования.