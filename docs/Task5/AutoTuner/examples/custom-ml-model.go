// Package main демонстрирует создание и использование кастомной ML модели в AutoTuner
package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/ipfs/boxo/bitswap/monitoring/tuning"
	"github.com/ipfs/boxo/bitswap/monitoring/tuning/ml"
)

func main() {
	// Пример 1: Простая кастомная модель
	simpleCustomModelExample()

	// Пример 2: Продвинутая модель с обучением
	advancedCustomModelExample()

	// Пример 3: Ансамбль моделей
	ensembleModelExample()

	// Пример 4: Модель с внешним API
	externalAPIModelExample()
}

// simpleCustomModelExample демонстрирует простую кастомную модель
func simpleCustomModelExample() {
	log.Println("=== Пример 1: Простая кастомная модель ===")

	// Создание простой модели на основе правил
	customModel := &SimpleRuleBasedModel{
		rules: []Rule{
			{
				Condition: func(features []float64) bool {
					// Если латентность высокая (feature[0] > 100ms)
					return features[0] > 0.1 // 100ms в секундах
				},
				Action: func(features []float64) []float64 {
					// Увеличить размер пула воркеров
					return []float64{
						features[1] * 1.2, // worker_pool_size * 1.2
						features[2],       // оставить остальные параметры без изменений
					}
				},
				Confidence: 0.8,
			},
			{
				Condition: func(features []float64) bool {
					// Если пропускная способность низкая (feature[1] < 500 RPS)
					return len(features) > 3 && features[3] < 500
				},
				Action: func(features []float64) []float64 {
					// Увеличить буферы
					return []float64{
						features[1],       // worker_pool_size без изменений
						features[2] * 1.5, // buffer_size * 1.5
					}
				},
				Confidence: 0.7,
			},
		},
	}

	// Создание конфигурации с кастомной моделью
	config := tuning.DefaultAutoTunerConfig()
	config.ML.CustomModel = customModel

	// Создание AutoTuner
	autoTuner, err := tuning.NewAutoTuner(config)
	if err != nil {
		log.Fatalf("Ошибка создания AutoTuner: %v", err)
	}

	ctx := context.Background()
	if err := autoTuner.Start(ctx); err != nil {
		log.Fatalf("Ошибка запуска AutoTuner: %v", err)
	}
	defer autoTuner.Stop()

	log.Println("AutoTuner с простой кастомной моделью запущен")

	// Тестирование модели
	testFeatures := []float64{0.15, 10, 65536, 300} // высокая латентность, низкая пропускная способность
	predictions, confidence, err := customModel.Predict(testFeatures)
	if err != nil {
		log.Printf("Ошибка предсказания: %v", err)
	} else {
		log.Printf("Предсказания: %v, уверенность: %.2f", predictions, confidence)
	}

	time.Sleep(5 * time.Second)
}

// advancedCustomModelExample демонстрирует продвинутую модель с обучением
func advancedCustomModelExample() {
	log.Println("\n=== Пример 2: Продвинутая модель с обучением ===")

	// Создание нейронной сети (упрощенная версия)
	neuralModel := &SimpleNeuralNetwork{
		inputSize:    4, // латентность, пропускная способность, использование CPU, частота ошибок
		hiddenSize:   8,
		outputSize:   2, // worker_pool_size, buffer_size
		learningRate: 0.01,
		weights1:     make([][]float64, 4),
		weights2:     make([][]float64, 8),
		bias1:        make([]float64, 8),
		bias2:        make([]float64, 2),
	}

	// Инициализация весов
	neuralModel.initializeWeights()

	// Генерация тренировочных данных
	trainingData := generateTrainingData(1000)

	// Обучение модели
	log.Println("Обучение нейронной сети...")
	err := neuralModel.Train(trainingData)
	if err != nil {
		log.Fatalf("Ошибка обучения: %v", err)
	}

	// Создание конфигурации
	config := tuning.DefaultAutoTunerConfig()
	config.ML.CustomModel = neuralModel

	autoTuner, err := tuning.NewAutoTuner(config)
	if err != nil {
		log.Fatalf("Ошибка создания AutoTuner: %v", err)
	}

	ctx := context.Background()
	if err := autoTuner.Start(ctx); err != nil {
		log.Fatalf("Ошибка запуска AutoTuner: %v", err)
	}
	defer autoTuner.Stop()

	log.Println("AutoTuner с нейронной сетью запущен")

	// Тестирование обученной модели
	testFeatures := []float64{0.08, 800, 0.6, 0.02} // нормальные показатели
	predictions, confidence, err := neuralModel.Predict(testFeatures)
	if err != nil {
		log.Printf("Ошибка предсказания: %v", err)
	} else {
		log.Printf("Предсказания нейронной сети: %v, уверенность: %.2f", predictions, confidence)
	}

	time.Sleep(5 * time.Second)
}

// ensembleModelExample демонстрирует ансамбль моделей
func ensembleModelExample() {
	log.Println("\n=== Пример 3: Ансамбль моделей ===")

	// Создание нескольких моделей
	ruleModel := &SimpleRuleBasedModel{
		rules: []Rule{
			{
				Condition: func(features []float64) bool { return features[0] > 0.1 },
				Action: func(features []float64) []float64 {
					return []float64{features[1] * 1.1, features[2]}
				},
				Confidence: 0.7,
			},
		},
	}

	linearModel := &SimpleLinearModel{
		weights: []float64{-10, 5, 0.001, -100}, // коэффициенты для латентности, RPS, CPU, ошибок
		bias:    []float64{15, 32768},           // базовые значения для worker_pool_size, buffer_size
	}

	// Создание ансамбля
	ensemble := &ModelEnsemble{
		models:   []ml.MLModel{ruleModel, linearModel},
		weights:  []float64{0.6, 0.4}, // веса моделей
		strategy: "weighted_average",
	}

	// Создание конфигурации
	config := tuning.DefaultAutoTunerConfig()
	config.ML.CustomModel = ensemble

	autoTuner, err := tuning.NewAutoTuner(config)
	if err != nil {
		log.Fatalf("Ошибка создания AutoTuner: %v", err)
	}

	ctx := context.Background()
	if err := autoTuner.Start(ctx); err != nil {
		log.Fatalf("Ошибка запуска AutoTuner: %v", err)
	}
	defer autoTuner.Stop()

	log.Println("AutoTuner с ансамблем моделей запущен")

	// Тестирование ансамбля
	testFeatures := []float64{0.12, 600, 0.7, 0.03}
	predictions, confidence, err := ensemble.Predict(testFeatures)
	if err != nil {
		log.Printf("Ошибка предсказания: %v", err)
	} else {
		log.Printf("Предсказания ансамбля: %v, уверенность: %.2f", predictions, confidence)
	}

	time.Sleep(5 * time.Second)
}

// externalAPIModelExample демонстрирует модель с внешним API
func externalAPIModelExample() {
	log.Println("\n=== Пример 4: Модель с внешним API ===")

	// Создание модели, использующей внешний ML сервис
	apiModel := &ExternalAPIModel{
		endpoint: "http://ml-service.example.com/predict",
		apiKey:   "your-api-key",
		timeout:  5 * time.Second,
	}

	// Создание конфигурации
	config := tuning.DefaultAutoTunerConfig()
	config.ML.CustomModel = apiModel

	autoTuner, err := tuning.NewAutoTuner(config)
	if err != nil {
		log.Fatalf("Ошибка создания AutoTuner: %v", err)
	}

	ctx := context.Background()
	if err := autoTuner.Start(ctx); err != nil {
		log.Fatalf("Ошибка запуска AutoTuner: %v", err)
	}
	defer autoTuner.Stop()

	log.Println("AutoTuner с внешним API запущен")

	// Примечание: в реальном примере здесь был бы вызов внешнего API
	log.Println("Модель настроена для использования внешнего ML API")

	time.Sleep(5 * time.Second)
}

// Реализации кастомных моделей

// SimpleRuleBasedModel - простая модель на основе правил
type SimpleRuleBasedModel struct {
	rules []Rule
}

type Rule struct {
	Condition  func([]float64) bool
	Action     func([]float64) []float64
	Confidence float64
}

func (srbm *SimpleRuleBasedModel) Predict(features []float64) ([]float64, float64, error) {
	for _, rule := range srbm.rules {
		if rule.Condition(features) {
			predictions := rule.Action(features)
			return predictions, rule.Confidence, nil
		}
	}

	// Если ни одно правило не сработало, возвращаем текущие значения
	return features[1:], 0.5, nil
}

func (srbm *SimpleRuleBasedModel) Train(data *ml.TrainingData) error {
	// Модель на основе правил не требует обучения
	return nil
}

func (srbm *SimpleRuleBasedModel) Save(path string) error {
	// Сохранение правил (упрощенная реализация)
	log.Printf("Сохранение модели правил в %s", path)
	return nil
}

func (srbm *SimpleRuleBasedModel) Load(path string) error {
	// Загрузка правил (упрощенная реализация)
	log.Printf("Загрузка модели правил из %s", path)
	return nil
}

// SimpleNeuralNetwork - упрощенная нейронная сеть
type SimpleNeuralNetwork struct {
	inputSize    int
	hiddenSize   int
	outputSize   int
	learningRate float64

	weights1 [][]float64
	weights2 [][]float64
	bias1    []float64
	bias2    []float64
}

func (snn *SimpleNeuralNetwork) initializeWeights() {
	// Инициализация весов случайными значениями
	for i := 0; i < snn.inputSize; i++ {
		snn.weights1[i] = make([]float64, snn.hiddenSize)
		for j := 0; j < snn.hiddenSize; j++ {
			snn.weights1[i][j] = rand.Float64()*2 - 1 // [-1, 1]
		}
	}

	for i := 0; i < snn.hiddenSize; i++ {
		snn.weights2[i] = make([]float64, snn.outputSize)
		for j := 0; j < snn.outputSize; j++ {
			snn.weights2[i][j] = rand.Float64()*2 - 1
		}
	}

	for i := 0; i < snn.hiddenSize; i++ {
		snn.bias1[i] = rand.Float64()*2 - 1
	}

	for i := 0; i < snn.outputSize; i++ {
		snn.bias2[i] = rand.Float64()*2 - 1
	}
}

func (snn *SimpleNeuralNetwork) sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

func (snn *SimpleNeuralNetwork) forward(input []float64) ([]float64, []float64) {
	// Скрытый слой
	hidden := make([]float64, snn.hiddenSize)
	for j := 0; j < snn.hiddenSize; j++ {
		sum := snn.bias1[j]
		for i := 0; i < snn.inputSize; i++ {
			sum += input[i] * snn.weights1[i][j]
		}
		hidden[j] = snn.sigmoid(sum)
	}

	// Выходной слой
	output := make([]float64, snn.outputSize)
	for j := 0; j < snn.outputSize; j++ {
		sum := snn.bias2[j]
		for i := 0; i < snn.hiddenSize; i++ {
			sum += hidden[i] * snn.weights2[i][j]
		}
		output[j] = sum // Линейная активация для регрессии
	}

	return hidden, output
}

func (snn *SimpleNeuralNetwork) Predict(features []float64) ([]float64, float64, error) {
	if len(features) != snn.inputSize {
		return nil, 0, fmt.Errorf("неверный размер входных данных: ожидается %d, получено %d",
			snn.inputSize, len(features))
	}

	_, output := snn.forward(features)

	// Простая оценка уверенности на основе стабильности выходов
	confidence := 0.8 // Упрощенная оценка

	return output, confidence, nil
}

func (snn *SimpleNeuralNetwork) Train(data *ml.TrainingData) error {
	epochs := 100

	for epoch := 0; epoch < epochs; epoch++ {
		totalLoss := 0.0

		for i, sample := range data.Samples {
			if len(sample.Features) != snn.inputSize || len(sample.Labels) != snn.outputSize {
				continue
			}

			// Прямой проход
			hidden, output := snn.forward(sample.Features)

			// Вычисление ошибки
			outputError := make([]float64, snn.outputSize)
			for j := 0; j < snn.outputSize; j++ {
				outputError[j] = sample.Labels[j] - output[j]
				totalLoss += outputError[j] * outputError[j]
			}

			// Обратное распространение (упрощенная версия)
			// Обновление весов выходного слоя
			for i := 0; i < snn.hiddenSize; i++ {
				for j := 0; j < snn.outputSize; j++ {
					snn.weights2[i][j] += snn.learningRate * outputError[j] * hidden[i]
				}
			}

			// Обновление bias выходного слоя
			for j := 0; j < snn.outputSize; j++ {
				snn.bias2[j] += snn.learningRate * outputError[j]
			}
		}

		if epoch%20 == 0 {
			avgLoss := totalLoss / float64(len(data.Samples))
			log.Printf("Эпоха %d, средняя ошибка: %.4f", epoch, avgLoss)
		}
	}

	return nil
}

func (snn *SimpleNeuralNetwork) Save(path string) error {
	log.Printf("Сохранение нейронной сети в %s", path)
	return nil
}

func (snn *SimpleNeuralNetwork) Load(path string) error {
	log.Printf("Загрузка нейронной сети из %s", path)
	return nil
}

// SimpleLinearModel - простая линейная модель
type SimpleLinearModel struct {
	weights []float64
	bias    []float64
}

func (slm *SimpleLinearModel) Predict(features []float64) ([]float64, float64, error) {
	if len(features) != len(slm.weights) {
		return nil, 0, fmt.Errorf("неверный размер входных данных")
	}

	predictions := make([]float64, len(slm.bias))
	for i := range predictions {
		predictions[i] = slm.bias[i]
		for j, feature := range features {
			predictions[i] += slm.weights[j] * feature
		}

		// Применение ограничений
		if i == 0 { // worker_pool_size
			predictions[i] = math.Max(1, math.Min(100, predictions[i]))
		} else if i == 1 { // buffer_size
			predictions[i] = math.Max(4096, math.Min(1048576, predictions[i]))
		}
	}

	return predictions, 0.75, nil
}

func (slm *SimpleLinearModel) Train(data *ml.TrainingData) error {
	// Простая линейная регрессия (упрощенная реализация)
	log.Println("Обучение линейной модели...")
	return nil
}

func (slm *SimpleLinearModel) Save(path string) error {
	log.Printf("Сохранение линейной модели в %s", path)
	return nil
}

func (slm *SimpleLinearModel) Load(path string) error {
	log.Printf("Загрузка линейной модели из %s", path)
	return nil
}

// ModelEnsemble - ансамбль моделей
type ModelEnsemble struct {
	models   []ml.MLModel
	weights  []float64
	strategy string
}

func (me *ModelEnsemble) Predict(features []float64) ([]float64, float64, error) {
	if len(me.models) != len(me.weights) {
		return nil, 0, fmt.Errorf("количество моделей не соответствует количеству весов")
	}

	allPredictions := make([][]float64, len(me.models))
	allConfidences := make([]float64, len(me.models))

	// Получение предсказаний от всех моделей
	for i, model := range me.models {
		pred, conf, err := model.Predict(features)
		if err != nil {
			return nil, 0, fmt.Errorf("ошибка модели %d: %w", i, err)
		}
		allPredictions[i] = pred
		allConfidences[i] = conf
	}

	// Агрегация предсказаний
	if len(allPredictions) == 0 {
		return nil, 0, fmt.Errorf("нет предсказаний")
	}

	outputSize := len(allPredictions[0])
	finalPredictions := make([]float64, outputSize)

	switch me.strategy {
	case "weighted_average":
		totalWeight := 0.0
		for i := range me.weights {
			totalWeight += me.weights[i]
		}

		for j := 0; j < outputSize; j++ {
			weightedSum := 0.0
			for i, predictions := range allPredictions {
				weightedSum += predictions[j] * me.weights[i]
			}
			finalPredictions[j] = weightedSum / totalWeight
		}

	case "median":
		for j := 0; j < outputSize; j++ {
			values := make([]float64, len(allPredictions))
			for i, predictions := range allPredictions {
				values[i] = predictions[j]
			}
			// Простая медиана (без сортировки для упрощения)
			finalPredictions[j] = values[len(values)/2]
		}

	default:
		return nil, 0, fmt.Errorf("неизвестная стратегия ансамбля: %s", me.strategy)
	}

	// Агрегация уверенности
	totalConfidence := 0.0
	for i, conf := range allConfidences {
		totalConfidence += conf * me.weights[i]
	}
	finalConfidence := totalConfidence / len(allConfidences)

	return finalPredictions, finalConfidence, nil
}

func (me *ModelEnsemble) Train(data *ml.TrainingData) error {
	// Обучение всех моделей в ансамбле
	for i, model := range me.models {
		log.Printf("Обучение модели %d в ансамбле...", i)
		if err := model.Train(data); err != nil {
			return fmt.Errorf("ошибка обучения модели %d: %w", i, err)
		}
	}
	return nil
}

func (me *ModelEnsemble) Save(path string) error {
	log.Printf("Сохранение ансамбля моделей в %s", path)
	return nil
}

func (me *ModelEnsemble) Load(path string) error {
	log.Printf("Загрузка ансамбля моделей из %s", path)
	return nil
}

// ExternalAPIModel - модель, использующая внешний API
type ExternalAPIModel struct {
	endpoint string
	apiKey   string
	timeout  time.Duration
}

func (eam *ExternalAPIModel) Predict(features []float64) ([]float64, float64, error) {
	// В реальной реализации здесь был бы HTTP запрос к внешнему API
	log.Printf("Вызов внешнего API: %s", eam.endpoint)

	// Имитация задержки API
	time.Sleep(100 * time.Millisecond)

	// Имитация ответа API
	predictions := []float64{
		10 + rand.Float64()*10,       // worker_pool_size: 10-20
		32768 + rand.Float64()*32768, // buffer_size: 32KB-64KB
	}

	confidence := 0.85

	return predictions, confidence, nil
}

func (eam *ExternalAPIModel) Train(data *ml.TrainingData) error {
	// Внешняя модель обучается на стороне API
	log.Printf("Отправка данных для обучения на внешний API: %s", eam.endpoint)
	return nil
}

func (eam *ExternalAPIModel) Save(path string) error {
	// Внешняя модель сохраняется на стороне API
	return nil
}

func (eam *ExternalAPIModel) Load(path string) error {
	// Внешняя модель загружается с API
	return nil
}

// Вспомогательные функции

// generateTrainingData генерирует синтетические тренировочные данные
func generateTrainingData(samples int) *ml.TrainingData {
	data := &ml.TrainingData{
		Samples: make([]*ml.TrainingSample, samples),
	}

	for i := 0; i < samples; i++ {
		// Генерация случайных признаков
		latency := rand.Float64() * 0.2     // 0-200ms
		throughput := rand.Float64() * 2000 // 0-2000 RPS
		cpuUsage := rand.Float64()          // 0-100%
		errorRate := rand.Float64() * 0.1   // 0-10%

		features := []float64{latency, throughput, cpuUsage, errorRate}

		// Генерация целевых значений на основе простых правил
		workerPoolSize := 5.0
		bufferSize := 16384.0

		if latency > 0.1 { // Высокая латентность
			workerPoolSize += 5
		}
		if throughput < 500 { // Низкая пропускная способность
			bufferSize *= 2
		}
		if cpuUsage > 0.8 { // Высокая нагрузка CPU
			workerPoolSize -= 2
		}

		labels := []float64{workerPoolSize, bufferSize}

		data.Samples[i] = &ml.TrainingSample{
			Features: features,
			Labels:   labels,
		}
	}

	return data
}

func init() {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetPrefix("[Custom ML Model] ")
}
