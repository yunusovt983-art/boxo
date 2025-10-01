# ML Algorithms Guide - Алгоритмы машинного обучения в AutoTuner

## 🧠 Обзор ML подсистемы

AutoTuner использует комбинацию различных алгоритмов машинного обучения для предсказания оптимальных параметров конфигурации Boxo.

### Архитектура ML подсистемы

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│ Feature         │    │ Model Ensemble   │    │ Prediction      │
│ Extraction      │───►│ (Multiple Models)│───►│ Aggregation     │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                        │                        │
         ▼                        ▼                        ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│ Data            │    │ Model Training   │    │ Confidence      │
│ Preprocessing   │    │ & Validation     │    │ Estimation      │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## 🌲 Random Forest (Основной алгоритм)

### Назначение
Основной алгоритм для предсказания оптимальных параметров конфигурации.

### Принцип работы
```go
type RandomForestPredictor struct {
    trees       []*DecisionTree
    nTrees      int
    maxDepth    int
    minSamples  int
    features    []string
}

func (rf *RandomForestPredictor) Predict(features []float64) ([]float64, float64, error) {
    predictions := make([][]float64, len(rf.trees))
    
    // Получение предсказаний от каждого дерева
    for i, tree := range rf.trees {
        pred, err := tree.Predict(features)
        if err != nil {
            return nil, 0, err
        }
        predictions[i] = pred
    }
    
    // Агрегация результатов (среднее)
    result := rf.aggregatePredictions(predictions)
    confidence := rf.calculateConfidence(predictions)
    
    return result, confidence, nil
}
```

### Преимущества
- Устойчивость к переобучению
- Хорошая интерпретируемость
- Работа с разнородными признаками
- Оценка важности признаков

### Параметры настройки
```yaml
random_forest:
  n_estimators: 100        # Количество деревьев
  max_depth: 10           # Максимальная глубина
  min_samples_split: 5    # Минимум образцов для разделения
  max_features: "sqrt"    # Количество признаков для рассмотрения
  bootstrap: true         # Использование bootstrap выборки
```

## 📈 Linear Regression (Трендовый анализ)

### Назначение
Анализ трендов производительности и прогнозирование развития метрик.

### Реализация
```go
type LinearRegressionAnalyzer struct {
    weights    []float64
    intercept  float64
    scaler     *StandardScaler
}

func (lr *LinearRegressionAnalyzer) AnalyzeTrend(timeSeries []MetricPoint) (*TrendAnalysis, error) {
    // Подготовка данных
    X, y := lr.prepareTimeSeriesData(timeSeries)
    
    // Нормализация
    X_scaled := lr.scaler.Transform(X)
    
    // Предсказание
    predictions := lr.predict(X_scaled)
    
    // Анализ тренда
    trend := lr.calculateTrendDirection(predictions)
    confidence := lr.calculateTrendConfidence(y, predictions)
    
    return &TrendAnalysis{
        Direction:  trend,
        Slope:      lr.weights[0],
        Confidence: confidence,
        Forecast:   lr.forecastNext(X_scaled, 5), // 5 точек вперед
    }, nil
}
```

### Применение
- Прогнозирование роста латентности
- Анализ тенденций использования ресурсов
- Предсказание деградации производительности
- Определение сезонных паттернов

## 🎯 K-Means Clustering (Группировка состояний)

### Назначение
Группировка схожих состояний системы для выявления паттернов поведения.

### Алгоритм
```go
type KMeansClusterer struct {
    k           int
    centroids   [][]float64
    maxIters    int
    tolerance   float64
}

func (km *KMeansClusterer) FitPredict(data [][]float64) ([]int, error) {
    // Инициализация центроидов
    km.initializeCentroids(data)
    
    for iter := 0; iter < km.maxIters; iter++ {
        // Назначение точек к кластерам
        assignments := km.assignToClusters(data)
        
        // Обновление центроидов
        newCentroids := km.updateCentroids(data, assignments)
        
        // Проверка сходимости
        if km.hasConverged(newCentroids) {
            break
        }
        
        km.centroids = newCentroids
    }
    
    return km.assignToClusters(data), nil
}
```

### Применение в AutoTuner
```go
type SystemStateClusterer struct {
    clusterer *KMeansClusterer
    clusters  map[int]*ClusterProfile
}

func (ssc *SystemStateClusterer) AnalyzeSystemStates(metrics []*MetricsSnapshot) error {
    // Извлечение признаков состояния
    features := ssc.extractStateFeatures(metrics)
    
    // Кластеризация
    assignments, err := ssc.clusterer.FitPredict(features)
    if err != nil {
        return err
    }
    
    // Создание профилей кластеров
    ssc.createClusterProfiles(metrics, assignments)
    
    return nil
}

func (ssc *SystemStateClusterer) GetOptimalConfigForState(currentMetrics *MetricsSnapshot) (*Configuration, error) {
    // Определение кластера текущего состояния
    clusterID := ssc.predictCluster(currentMetrics)
    
    // Получение оптимальной конфигурации для кластера
    profile := ssc.clusters[clusterID]
    return profile.OptimalConfig, nil
}
```

## 🎮 Reinforcement Learning (Q-Learning)

### Назначение
Долгосрочная оптимизация с обучением на основе результатов применения конфигураций.

### Архитектура
```go
type QLearningAgent struct {
    qTable      map[string]map[string]float64  // state -> action -> Q-value
    learningRate float64
    discountFactor float64
    epsilon     float64  // exploration rate
    
    stateEncoder   StateEncoder
    actionEncoder  ActionEncoder
    rewardFunction RewardFunction
}

func (qla *QLearningAgent) SelectAction(state *SystemState) (*TuningAction, error) {
    stateKey := qla.stateEncoder.Encode(state)
    
    // Epsilon-greedy стратегия
    if rand.Float64() < qla.epsilon {
        // Исследование: случайное действие
        return qla.getRandomAction(), nil
    } else {
        // Эксплуатация: лучшее известное действие
        return qla.getBestAction(stateKey), nil
    }
}

func (qla *QLearningAgent) UpdateQValue(state *SystemState, action *TuningAction, reward float64, nextState *SystemState) {
    stateKey := qla.stateEncoder.Encode(state)
    actionKey := qla.actionEncoder.Encode(action)
    nextStateKey := qla.stateEncoder.Encode(nextState)
    
    // Текущее Q-значение
    currentQ := qla.qTable[stateKey][actionKey]
    
    // Максимальное Q-значение для следующего состояния
    maxNextQ := qla.getMaxQValue(nextStateKey)
    
    // Обновление Q-значения
    newQ := currentQ + qla.learningRate*(reward + qla.discountFactor*maxNextQ - currentQ)
    qla.qTable[stateKey][actionKey] = newQ
}
```

### Функция награды
```go
type PerformanceRewardFunction struct {
    baselineMetrics *Metrics
    weights         map[string]float64
}

func (prf *PerformanceRewardFunction) Calculate(beforeMetrics, afterMetrics *Metrics) float64 {
    reward := 0.0
    
    // Награда за улучшение латентности
    latencyImprovement := (beforeMetrics.AverageLatency - afterMetrics.AverageLatency).Seconds()
    reward += prf.weights["latency"] * latencyImprovement / beforeMetrics.AverageLatency.Seconds()
    
    // Награда за увеличение пропускной способности
    throughputImprovement := (afterMetrics.RequestsPerSecond - beforeMetrics.RequestsPerSecond)
    reward += prf.weights["throughput"] * throughputImprovement / beforeMetrics.RequestsPerSecond
    
    // Штраф за увеличение использования ресурсов
    resourcePenalty := (afterMetrics.ResourceUsage - beforeMetrics.ResourceUsage)
    reward -= prf.weights["resources"] * resourcePenalty / beforeMetrics.ResourceUsage
    
    // Штраф за нестабильность
    stabilityPenalty := math.Abs(afterMetrics.ErrorRate - beforeMetrics.ErrorRate)
    reward -= prf.weights["stability"] * stabilityPenalty
    
    return reward
}
```

## 🔧 Feature Engineering (Извлечение признаков)

### Временные признаки
```go
type TemporalFeatureExtractor struct {
    windowSize time.Duration
    aggregators []Aggregator
}

func (tfe *TemporalFeatureExtractor) Extract(metrics []*MetricsSnapshot) []float64 {
    features := make([]float64, 0)
    
    // Статистические признаки
    features = append(features, tfe.calculateMean(metrics)...)
    features = append(features, tfe.calculateStdDev(metrics)...)
    features = append(features, tfe.calculatePercentiles(metrics)...)
    
    // Трендовые признаки
    features = append(features, tfe.calculateTrend(metrics)...)
    features = append(features, tfe.calculateSeasonality(metrics)...)
    
    // Признаки изменчивости
    features = append(features, tfe.calculateVolatility(metrics)...)
    features = append(features, tfe.calculateAutocorrelation(metrics)...)
    
    return features
}
```

### Системные признаки
```go
type SystemFeatureExtractor struct {
    configEncoder ConfigurationEncoder
}

func (sfe *SystemFeatureExtractor) Extract(config *Configuration, metrics *MetricsSnapshot) []float64 {
    features := make([]float64, 0)
    
    // Признаки конфигурации
    features = append(features, sfe.configEncoder.Encode(config)...)
    
    // Признаки нагрузки
    features = append(features, 
        metrics.RequestsPerSecond,
        float64(metrics.ActiveConnections),
        metrics.ErrorRate,
    )
    
    // Признаки ресурсов
    features = append(features,
        metrics.CPUUsage,
        metrics.MemoryUsage,
        metrics.DiskUsage,
    )
    
    // Признаки производительности
    features = append(features,
        metrics.AverageLatency.Seconds(),
        metrics.P95Latency.Seconds(),
        metrics.P99Latency.Seconds(),
    )
    
    return features
}
```

## 📊 Model Ensemble (Ансамбль моделей)

### Архитектура ансамбля
```go
type ModelEnsemble struct {
    models  []MLModel
    weights []float64
    voting  VotingStrategy
}

func (me *ModelEnsemble) Predict(features []float64) ([]float64, float64, error) {
    predictions := make([][]float64, len(me.models))
    confidences := make([]float64, len(me.models))
    
    // Получение предсказаний от всех моделей
    for i, model := range me.models {
        pred, conf, err := model.Predict(features)
        if err != nil {
            return nil, 0, fmt.Errorf("model %d prediction failed: %w", i, err)
        }
        predictions[i] = pred
        confidences[i] = conf
    }
    
    // Агрегация предсказаний
    finalPrediction := me.voting.Aggregate(predictions, me.weights)
    finalConfidence := me.calculateEnsembleConfidence(confidences, me.weights)
    
    return finalPrediction, finalConfidence, nil
}
```

### Стратегии голосования
```go
// Взвешенное среднее
type WeightedAverageVoting struct{}

func (wav *WeightedAverageVoting) Aggregate(predictions [][]float64, weights []float64) []float64 {
    result := make([]float64, len(predictions[0]))
    
    for i := range result {
        weightedSum := 0.0
        totalWeight := 0.0
        
        for j, pred := range predictions {
            weightedSum += pred[i] * weights[j]
            totalWeight += weights[j]
        }
        
        result[i] = weightedSum / totalWeight
    }
    
    return result
}

// Медианное голосование
type MedianVoting struct{}

func (mv *MedianVoting) Aggregate(predictions [][]float64, weights []float64) []float64 {
    result := make([]float64, len(predictions[0]))
    
    for i := range result {
        values := make([]float64, len(predictions))
        for j, pred := range predictions {
            values[j] = pred[i]
        }
        
        sort.Float64s(values)
        result[i] = values[len(values)/2]
    }
    
    return result
}
```

## 🎯 Model Training Pipeline

### Процесс обучения
```go
type TrainingPipeline struct {
    dataCollector   DataCollector
    preprocessor    DataPreprocessor
    validator       ModelValidator
    modelRegistry   ModelRegistry
}

func (tp *TrainingPipeline) TrainModel(modelType string) error {
    // 1. Сбор данных
    trainingData, err := tp.dataCollector.CollectTrainingData(30 * 24 * time.Hour)
    if err != nil {
        return fmt.Errorf("data collection failed: %w", err)
    }
    
    // 2. Предобработка
    processedData, err := tp.preprocessor.Process(trainingData)
    if err != nil {
        return fmt.Errorf("preprocessing failed: %w", err)
    }
    
    // 3. Разделение на train/validation/test
    trainSet, valSet, testSet := tp.splitData(processedData, 0.7, 0.15, 0.15)
    
    // 4. Обучение модели
    model := tp.createModel(modelType)
    if err := model.Train(trainSet); err != nil {
        return fmt.Errorf("training failed: %w", err)
    }
    
    // 5. Валидация
    valMetrics, err := tp.validator.Validate(model, valSet)
    if err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    
    // 6. Тестирование
    testMetrics, err := tp.validator.Test(model, testSet)
    if err != nil {
        return fmt.Errorf("testing failed: %w", err)
    }
    
    // 7. Сохранение модели
    if testMetrics.Accuracy > 0.85 {
        return tp.modelRegistry.SaveModel(model, testMetrics)
    }
    
    return fmt.Errorf("model accuracy too low: %.2f", testMetrics.Accuracy)
}
```

### Автоматическое переобучение
```go
type AutoRetrainer struct {
    pipeline        *TrainingPipeline
    scheduler       *cron.Cron
    performanceMonitor *ModelPerformanceMonitor
}

func (ar *AutoRetrainer) Start() {
    // Периодическое переобучение
    ar.scheduler.AddFunc("0 2 * * *", func() { // Каждый день в 2:00
        ar.retrainIfNeeded()
    })
    
    // Переобучение при деградации производительности
    go ar.monitorPerformance()
    
    ar.scheduler.Start()
}

func (ar *AutoRetrainer) retrainIfNeeded() {
    currentModel := ar.getCurrentModel()
    
    // Проверка возраста модели
    if time.Since(currentModel.TrainedAt) > 7*24*time.Hour {
        log.Info("Model is old, retraining...")
        ar.pipeline.TrainModel(currentModel.Type)
        return
    }
    
    // Проверка производительности
    recentAccuracy := ar.performanceMonitor.GetRecentAccuracy(24 * time.Hour)
    if recentAccuracy < 0.8 {
        log.Info("Model performance degraded, retraining...")
        ar.pipeline.TrainModel(currentModel.Type)
    }
}
```

## 📈 Performance Monitoring

### Метрики качества модели
```go
type ModelMetrics struct {
    Accuracy          float64   `json:"accuracy"`
    Precision         float64   `json:"precision"`
    Recall            float64   `json:"recall"`
    F1Score           float64   `json:"f1_score"`
    MeanAbsoluteError float64   `json:"mae"`
    RootMeanSquareError float64 `json:"rmse"`
    
    // Специфичные для AutoTuner метрики
    PredictionAccuracy    float64 `json:"prediction_accuracy"`
    ImprovementPrediction float64 `json:"improvement_prediction"`
    ConfigurationSuccess  float64 `json:"configuration_success"`
}

func (mm *ModelMetrics) CalculateOverallScore() float64 {
    return (mm.Accuracy*0.3 + 
            mm.F1Score*0.2 + 
            mm.PredictionAccuracy*0.3 + 
            mm.ConfigurationSuccess*0.2)
}
```

### Мониторинг дрифта данных
```go
type DataDriftDetector struct {
    referenceDistribution map[string]*Distribution
    threshold             float64
}

func (ddd *DataDriftDetector) DetectDrift(currentData [][]float64) (bool, map[string]float64) {
    driftScores := make(map[string]float64)
    hasDrift := false
    
    for featureIdx, refDist := range ddd.referenceDistribution {
        currentDist := ddd.calculateDistribution(currentData, featureIdx)
        
        // Расчет KL-дивергенции
        klDiv := ddd.calculateKLDivergence(refDist, currentDist)
        driftScores[featureIdx] = klDiv
        
        if klDiv > ddd.threshold {
            hasDrift = true
        }
    }
    
    return hasDrift, driftScores
}
```

Этот документ описывает все ML алгоритмы, используемые в AutoTuner, их реализацию и применение для оптимизации конфигурации Boxo.