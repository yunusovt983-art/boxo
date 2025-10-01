# AutoTuner - Автоматический тюнер параметров Boxo

## 🎯 Обзор

AutoTuner - это интеллектуальная система автоматической оптимизации конфигурации Boxo, использующая машинное обучение для предсказания оптимальных параметров и безопасные механизмы для их применения.

## 🏗️ Архитектура

### Основные компоненты

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   AutoTuner     │    │   ML Predictor   │    │  Safe Applier   │
│   (Coordinator) │◄──►│   (Intelligence) │◄──►│   (Safety)      │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                        │                        │
         ▼                        ▼                        ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  State Machine  │    │ Config Optimizer │    │ Rollback Manager│
│  (Lifecycle)    │    │ (Optimization)   │    │ (Recovery)      │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

### Поток данных

```
Metrics → Feature Extraction → ML Prediction → Optimization → Validation → Application → Monitoring
   ▲                                                                                        │
   └────────────────────────── Feedback Loop ◄─────────────────────────────────────────────┘
```

## 🤖 Машинное обучение

### Алгоритмы

#### 1. Random Forest (Основной)
- **Назначение**: Предсказание оптимальных параметров
- **Входы**: Метрики производительности, текущая конфигурация
- **Выходы**: Рекомендуемые параметры с уверенностью
- **Преимущества**: Устойчивость к переобучению, интерпретируемость

#### 2. Linear Regression (Трендовый анализ)
- **Назначение**: Анализ трендов производительности
- **Входы**: Временные ряды метрик
- **Выходы**: Прогноз развития трендов
- **Преимущества**: Простота, быстрота вычислений

#### 3. K-Means Clustering (Группировка)
- **Назначение**: Группировка схожих состояний системы
- **Входы**: Векторы состояний системы
- **Выходы**: Кластеры состояний
- **Преимущества**: Выявление паттернов поведения

#### 4. Reinforcement Learning (Долгосрочная оптимизация)
- **Назначение**: Обучение на основе долгосрочных результатов
- **Алгоритм**: Q-Learning с neural network
- **Награда**: Улучшение производительности
- **Преимущества**: Адаптация к изменяющимся условиям

### Процесс обучения

```python
# Псевдокод процесса обучения
def train_ml_model():
    # 1. Сбор данных
    historical_data = collect_historical_data(timespan="30d")
    
    # 2. Извлечение признаков
    features = extract_features(historical_data)
    labels = extract_performance_outcomes(historical_data)
    
    # 3. Подготовка данных
    X_train, X_test, y_train, y_test = split_data(features, labels)
    X_train_scaled = scale_features(X_train)
    
    # 4. Обучение модели
    model = RandomForestRegressor(n_estimators=100, max_depth=10)
    model.fit(X_train_scaled, y_train)
    
    # 5. Валидация
    predictions = model.predict(X_test)
    accuracy = calculate_accuracy(predictions, y_test)
    
    # 6. Сохранение модели
    if accuracy > 0.85:
        save_model(model, "production_model.pkl")
        
    return model, accuracy
```

## 🛡️ Механизмы безопасности

### 1. Валидация параметров

```go
type ParameterValidator struct {
    constraints map[string]Constraint
    rules       []ValidationRule
}

func (pv *ParameterValidator) Validate(params *TuningParameters) error {
    // Проверка диапазонов
    for name, value := range params.Values {
        constraint := pv.constraints[name]
        if value < constraint.Min || value > constraint.Max {
            return fmt.Errorf("parameter %s out of range: %v", name, value)
        }
    }
    
    // Проверка бизнес-правил
    for _, rule := range pv.rules {
        if err := rule.Check(params); err != nil {
            return fmt.Errorf("validation rule failed: %w", err)
        }
    }
    
    return nil
}
```

### 2. Canary Deployment

```go
type CanaryDeployment struct {
    percentage    int
    duration      time.Duration
    healthChecker HealthChecker
}

func (cd *CanaryDeployment) Deploy(config *Configuration) error {
    // Применение к части узлов
    canaryNodes := cd.selectCanaryNodes(cd.percentage)
    
    for _, node := range canaryNodes {
        if err := node.ApplyConfig(config); err != nil {
            return fmt.Errorf("canary deployment failed: %w", err)
        }
    }
    
    // Мониторинг здоровья
    if err := cd.monitorHealth(cd.duration); err != nil {
        cd.rollbackCanary(canaryNodes)
        return fmt.Errorf("canary health check failed: %w", err)
    }
    
    // Полное развертывание
    return cd.deployToAllNodes(config)
}
```

### 3. Автоматический откат

```go
type RollbackManager struct {
    backups       map[string]*ConfigBackup
    healthChecker HealthChecker
    rollbackTimer *time.Timer
}

func (rm *RollbackManager) MonitorAndRollback(configID string) {
    rm.rollbackTimer = time.AfterFunc(5*time.Minute, func() {
        if !rm.healthChecker.IsHealthy() {
            log.Warn("System unhealthy, initiating rollback")
            rm.Rollback(configID)
        }
    })
}

func (rm *RollbackManager) Rollback(configID string) error {
    backup := rm.backups[configID]
    if backup == nil {
        return fmt.Errorf("no backup found for config %s", configID)
    }
    
    log.Infof("Rolling back to configuration: %s", backup.ID)
    return backup.Restore()
}
```

## 🔄 Машина состояний

### Состояния

```go
type TunerState int

const (
    StateIdle TunerState = iota
    StateAnalyzing
    StatePredicting
    StateOptimizing
    StateApplying
    StateMonitoring
    StateRollingBack
    StateSuccess
    StateError
)
```

### Переходы

```go
var stateTransitions = map[TunerState][]TunerState{
    StateIdle:        {StateAnalyzing},
    StateAnalyzing:   {StatePredicting, StateIdle, StateError},
    StatePredicting:  {StateOptimizing, StateIdle, StateError},
    StateOptimizing:  {StateApplying, StateError},
    StateApplying:    {StateMonitoring, StateRollingBack, StateError},
    StateMonitoring:  {StateSuccess, StateRollingBack, StateError},
    StateRollingBack: {StateIdle, StateError},
    StateSuccess:     {StateIdle},
    StateError:       {StateIdle},
}
```

### Обработчики состояний

```go
func (sm *StateMachine) handleAnalyzing(ctx context.Context) (TunerState, error) {
    log.Info("Analyzing current system metrics...")
    
    // Сбор метрик
    metrics, err := sm.metricsCollector.Collect()
    if err != nil {
        return StateError, fmt.Errorf("failed to collect metrics: %w", err)
    }
    
    // Анализ необходимости оптимизации
    needsOptimization, err := sm.analyzer.NeedsOptimization(metrics)
    if err != nil {
        return StateError, fmt.Errorf("analysis failed: %w", err)
    }
    
    if !needsOptimization {
        log.Info("No optimization needed")
        return StateIdle, nil
    }
    
    sm.context.Metrics = metrics
    return StatePredicting, nil
}
```

## 📊 Оптимизация конфигурации

### Целевые функции

```go
type ObjectiveFunction interface {
    Evaluate(config *Configuration, metrics *Metrics) float64
    Weight() float64
}

// Минимизация латентности
type LatencyObjective struct{}

func (lo *LatencyObjective) Evaluate(config *Configuration, metrics *Metrics) float64 {
    return 1.0 / (1.0 + metrics.AverageLatency.Seconds())
}

func (lo *LatencyObjective) Weight() float64 { return 0.4 }

// Максимизация пропускной способности
type ThroughputObjective struct{}

func (to *ThroughputObjective) Evaluate(config *Configuration, metrics *Metrics) float64 {
    return metrics.RequestsPerSecond / 1000.0 // Нормализация
}

func (to *ThroughputObjective) Weight() float64 { return 0.3 }
```

### Алгоритм оптимизации

```go
type GeneticOptimizer struct {
    populationSize int
    generations    int
    mutationRate   float64
    objectives     []ObjectiveFunction
}

func (go *GeneticOptimizer) Optimize(currentConfig *Configuration) (*Configuration, error) {
    // Инициализация популяции
    population := go.initializePopulation(currentConfig)
    
    for generation := 0; generation < go.generations; generation++ {
        // Оценка приспособленности
        fitness := go.evaluateFitness(population)
        
        // Селекция
        parents := go.selection(population, fitness)
        
        // Скрещивание
        offspring := go.crossover(parents)
        
        // Мутация
        go.mutate(offspring)
        
        // Новое поколение
        population = go.nextGeneration(population, offspring, fitness)
    }
    
    // Лучшая конфигурация
    best := go.getBest(population)
    return best, nil
}
```

## 📈 Метрики и мониторинг

### Ключевые метрики

```go
type TunerMetrics struct {
    // Производительность
    PredictionDuration    time.Duration `json:"prediction_duration"`
    OptimizationDuration  time.Duration `json:"optimization_duration"`
    ApplicationDuration   time.Duration `json:"application_duration"`
    
    // Качество
    PredictionAccuracy    float64       `json:"prediction_accuracy"`
    SuccessfulTunings     int64         `json:"successful_tunings"`
    FailedTunings         int64         `json:"failed_tunings"`
    RollbackCount         int64         `json:"rollback_count"`
    
    // ML модель
    ModelAge              time.Duration `json:"model_age"`
    ModelAccuracy         float64       `json:"model_accuracy"`
    TrainingDataSize      int64         `json:"training_data_size"`
    
    // Состояние
    CurrentState          TunerState    `json:"current_state"`
    StateStartTime        time.Time     `json:"state_start_time"`
    LastSuccessfulTuning  time.Time     `json:"last_successful_tuning"`
}
```

### Prometheus метрики

```go
var (
    tuningDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "autotuner_tuning_duration_seconds",
            Help: "Duration of tuning operations",
        },
        []string{"operation", "result"},
    )
    
    tuningSuccess = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "autotuner_tuning_operations_total",
            Help: "Total number of tuning operations",
        },
        []string{"result"},
    )
    
    modelAccuracy = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "autotuner_model_accuracy_ratio",
            Help: "Current ML model accuracy",
        },
    )
)
```

## 🔧 Конфигурация

### Основная конфигурация

```yaml
autotuner:
  enabled: true
  
  # Общие настройки
  tuning_interval: "15m"
  max_concurrent_tunings: 1
  
  # ML настройки
  ml:
    model_type: "random_forest"
    model_path: "/var/lib/autotuner/models"
    training_window: "24h"
    retrain_interval: "6h"
    min_confidence: 0.8
    feature_window: "1h"
    
    # Параметры модели
    random_forest:
      n_estimators: 100
      max_depth: 10
      min_samples_split: 5
      
  # Безопасность
  safety:
    backup_retention: "7d"
    canary_percentage: 10
    canary_duration: "5m"
    rollback_timeout: "5m"
    max_changes_per_hour: 3
    health_check_interval: "30s"
    
  # Оптимизация
  optimization:
    algorithm: "genetic"
    
    genetic:
      population_size: 50
      generations: 100
      mutation_rate: 0.1
      crossover_rate: 0.8
      
    objectives:
      - name: "latency"
        weight: 0.4
        target: "minimize"
      - name: "throughput"
        weight: 0.3
        target: "maximize"
      - name: "resource_usage"
        weight: 0.3
        target: "minimize"
        
  # Параметры для тюнинга
  parameters:
    bitswap_max_outstanding_bytes_per_peer:
      min: 1048576      # 1MB
      max: 134217728    # 128MB
      default: 16777216 # 16MB
      
    bitswap_worker_pool_size:
      min: 1
      max: 100
      default: 10
      
    blockstore_cache_size:
      min: 67108864     # 64MB
      max: 2147483648   # 2GB
      default: 268435456 # 256MB
```

### Конфигурация для разработки

```yaml
autotuner:
  enabled: true
  tuning_interval: "5m"  # Более частые тюнинги для тестирования
  
  ml:
    min_confidence: 0.6  # Более низкий порог для экспериментов
    retrain_interval: "1h"
    
  safety:
    canary_percentage: 50  # Больший процент для быстрого тестирования
    canary_duration: "1m"
    max_changes_per_hour: 10
    
  optimization:
    genetic:
      population_size: 20  # Меньше для быстроты
      generations: 50
```

## 🚀 Использование

### Базовое использование

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/ipfs/boxo/bitswap/monitoring/tuning"
)

func main() {
    // Загрузка конфигурации
    config, err := tuning.LoadConfig("config.yml")
    if err != nil {
        log.Fatal(err)
    }
    
    // Создание AutoTuner
    autoTuner, err := tuning.NewAutoTuner(config)
    if err != nil {
        log.Fatal(err)
    }
    
    // Запуск
    ctx := context.Background()
    if err := autoTuner.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer autoTuner.Stop()
    
    // Мониторинг состояния
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                state := autoTuner.GetCurrentState()
                metrics := autoTuner.GetMetrics()
                
                log.Printf("State: %v, Success Rate: %.2f%%", 
                    state, 
                    float64(metrics.SuccessfulTunings)/float64(metrics.SuccessfulTunings+metrics.FailedTunings)*100)
            case <-ctx.Done():
                return
            }
        }
    }()
    
    // Основной цикл
    select {}
}
```

### Продвинутое использование

```go
// Кастомная целевая функция
type CustomObjective struct {
    weight float64
}

func (co *CustomObjective) Evaluate(config *Configuration, metrics *Metrics) float64 {
    // Ваша логика оценки
    return customScore
}

func (co *CustomObjective) Weight() float64 {
    return co.weight
}

// Кастомный валидатор
type CustomValidator struct{}

func (cv *CustomValidator) Validate(params *TuningParameters) error {
    // Ваша логика валидации
    return nil
}

// Использование кастомных компонентов
config := tuning.DefaultConfig()
config.CustomObjectives = []tuning.ObjectiveFunction{&CustomObjective{weight: 0.2}}
config.CustomValidators = []tuning.Validator{&CustomValidator{}}

autoTuner, err := tuning.NewAutoTuner(config)
```

## 🧪 Тестирование

### Unit тесты

```go
func TestAutoTuner_PredictOptimalParameters(t *testing.T) {
    // Подготовка
    config := tuning.TestConfig()
    tuner := tuning.NewAutoTuner(config)
    
    metrics := &tuning.Metrics{
        AverageLatency:    100 * time.Millisecond,
        RequestsPerSecond: 500,
        ErrorRate:         0.01,
    }
    
    // Выполнение
    params, err := tuner.PredictOptimalParameters(context.Background(), metrics)
    
    // Проверка
    assert.NoError(t, err)
    assert.NotNil(t, params)
    assert.True(t, params.Confidence > 0.8)
    assert.True(t, params.BitswapMaxOutstandingBytesPerPeer > 0)
}

func TestSafeApplier_CanaryDeployment(t *testing.T) {
    applier := tuning.NewSafeApplier(tuning.TestSafetyConfig())
    
    config := &tuning.Configuration{
        BitswapMaxOutstandingBytesPerPeer: 32 * 1024 * 1024,
    }
    
    err := applier.ApplyWithCanary(context.Background(), config)
    assert.NoError(t, err)
}
```

### Integration тесты

```go
func TestAutoTuner_FullCycle(t *testing.T) {
    // Настройка тестовой среды
    testEnv := tuning.NewTestEnvironment()
    defer testEnv.Cleanup()
    
    // Создание AutoTuner
    config := tuning.TestConfig()
    config.TuningInterval = 1 * time.Second
    
    tuner := tuning.NewAutoTuner(config)
    
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    // Запуск
    err := tuner.Start(ctx)
    require.NoError(t, err)
    defer tuner.Stop()
    
    // Ожидание завершения цикла тюнинга
    time.Sleep(10 * time.Second)
    
    // Проверка результатов
    metrics := tuner.GetMetrics()
    assert.True(t, metrics.SuccessfulTunings > 0)
}
```

### Benchmark тесты

```go
func BenchmarkMLPredictor_Predict(b *testing.B) {
    predictor := tuning.NewMLPredictor(tuning.TestMLConfig())
    features := generateTestFeatures()
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        _, _, err := predictor.Predict(features)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkGeneticOptimizer_Optimize(b *testing.B) {
    optimizer := tuning.NewGeneticOptimizer(tuning.TestOptimizationConfig())
    baseConfig := tuning.DefaultConfiguration()
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        _, err := optimizer.Optimize(baseConfig)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

## 🔍 Отладка и диагностика

### Логирование

```go
// Структурированное логирование
log.WithFields(log.Fields{
    "component": "autotuner",
    "operation": "predict",
    "confidence": params.Confidence,
    "duration": duration,
}).Info("ML prediction completed")

// Трассировка состояний
log.WithFields(log.Fields{
    "from_state": oldState,
    "to_state": newState,
    "trigger": trigger,
    "duration_in_state": timeInState,
}).Info("State transition")
```

### Метрики для отладки

```go
// Детальные метрики производительности
var (
    mlPredictionLatency = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name: "autotuner_ml_prediction_duration_seconds",
            Help: "ML prediction latency",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
        },
    )
    
    stateTransitionDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "autotuner_state_duration_seconds",
            Help: "Time spent in each state",
        },
        []string{"state"},
    )
)
```

### Health Checks

```go
func (at *AutoTuner) HealthCheck() error {
    // Проверка состояния ML модели
    if at.mlPredictor.ModelAge() > 24*time.Hour {
        return fmt.Errorf("ML model is too old")
    }
    
    // Проверка последнего успешного тюнинга
    if time.Since(at.lastSuccessfulTuning) > 6*time.Hour {
        return fmt.Errorf("no successful tuning in last 6 hours")
    }
    
    // Проверка состояния системы
    if at.currentState == StateError {
        return fmt.Errorf("tuner is in error state")
    }
    
    return nil
}
```

## 📚 Дополнительные ресурсы

- [ML Algorithms Guide](ml-algorithms.md) - Подробное описание ML алгоритмов
- [Safety Mechanisms](safety-mechanisms.md) - Механизмы безопасности
- [Configuration Guide](configuration-guide.md) - Руководство по конфигурации
- [Troubleshooting](troubleshooting.md) - Решение проблем
- [API Reference](api/) - Справочник по API

## 🤝 Вклад в развитие

Мы приветствуем вклад в развитие AutoTuner! Пожалуйста, ознакомьтесь с нашими [guidelines](CONTRIBUTING.md) перед отправкой pull request'ов.

## 📄 Лицензия

Этот проект лицензирован под [MIT License](LICENSE).