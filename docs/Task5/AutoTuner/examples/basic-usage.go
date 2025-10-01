// Package main демонстрирует базовое использование AutoTuner
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ipfs/boxo/bitswap/monitoring/tuning"
)

func main() {
	// Создание контекста с отменой
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Обработка сигналов для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Пример 1: Базовое использование с конфигурацией по умолчанию
	basicUsageExample(ctx)

	// Пример 2: Использование с кастомной конфигурацией
	customConfigExample(ctx)

	// Пример 3: Мониторинг состояния AutoTuner
	monitoringExample(ctx)

	// Пример 4: Ручное управление тюнингом
	manualControlExample(ctx)

	// Ожидание сигнала завершения
	<-sigChan
	log.Println("Получен сигнал завершения, останавливаем AutoTuner...")
}

// basicUsageExample демонстрирует базовое использование AutoTuner
func basicUsageExample(ctx context.Context) {
	log.Println("=== Пример 1: Базовое использование ===")

	// Создание AutoTuner с конфигурацией по умолчанию
	autoTuner, err := tuning.NewAutoTuner(nil) // nil = использовать конфигурацию по умолчанию
	if err != nil {
		log.Fatalf("Ошибка создания AutoTuner: %v", err)
	}

	// Запуск AutoTuner
	if err := autoTuner.Start(ctx); err != nil {
		log.Fatalf("Ошибка запуска AutoTuner: %v", err)
	}
	defer func() {
		if err := autoTuner.Stop(); err != nil {
			log.Printf("Ошибка остановки AutoTuner: %v", err)
		}
	}()

	log.Println("AutoTuner запущен с конфигурацией по умолчанию")

	// AutoTuner будет автоматически выполнять тюнинг каждые 15 минут (по умолчанию)
	// Для демонстрации подождем немного
	time.Sleep(5 * time.Second)

	// Получение текущего состояния
	state := autoTuner.GetCurrentState()
	log.Printf("Текущее состояние AutoTuner: %v", state)

	// Получение метрик
	metrics := autoTuner.GetMetrics()
	log.Printf("Метрики AutoTuner: успешных тюнингов=%d, неудачных=%d",
		metrics.SuccessfulTunings, metrics.FailedTunings)
}

// customConfigExample демонстрирует использование с кастомной конфигурацией
func customConfigExample(ctx context.Context) {
	log.Println("\n=== Пример 2: Кастомная конфигурация ===")

	// Создание кастомной конфигурации
	config := &tuning.AutoTunerConfig{
		Enabled:        true,
		TuningInterval: 5 * time.Minute, // Более частые тюнинги для демонстрации

		// ML настройки
		ML: tuning.MLConfig{
			Enabled:         true,
			ModelType:       "random_forest",
			MinConfidence:   0.85, // Более высокий порог уверенности
			RetrainInterval: 2 * time.Hour,
		},

		// Настройки безопасности
		Safety: tuning.SafetyConfig{
			Enabled: true,
			Canary: tuning.CanaryConfig{
				Enabled:           true,
				InitialPercentage: 10,
				MaxPercentage:     50,
				StepSize:          10,
				StepDuration:      3 * time.Minute,
				SuccessThreshold:  0.9,
			},
			RollbackTimeout:   2 * time.Minute,
			MaxChangesPerHour: 2,
		},

		// Параметры для тюнинга
		Parameters: map[string]*tuning.ParameterConfig{
			"bitswap_max_outstanding_bytes_per_peer": {
				Type:        "int",
				Min:         1024 * 1024,      // 1MB
				Max:         64 * 1024 * 1024, // 64MB
				Default:     16 * 1024 * 1024, // 16MB
				Step:        1024 * 1024,      // 1MB шаг
				Description: "Максимум байт на peer в Bitswap",
			},
			"bitswap_worker_pool_size": {
				Type:        "int",
				Min:         1,
				Max:         50,
				Default:     10,
				Step:        1,
				Description: "Размер пула воркеров Bitswap",
			},
		},
	}

	// Создание AutoTuner с кастомной конфигурацией
	autoTuner, err := tuning.NewAutoTuner(config)
	if err != nil {
		log.Fatalf("Ошибка создания AutoTuner: %v", err)
	}

	// Запуск
	if err := autoTuner.Start(ctx); err != nil {
		log.Fatalf("Ошибка запуска AutoTuner: %v", err)
	}
	defer autoTuner.Stop()

	log.Println("AutoTuner запущен с кастомной конфигурацией")
	log.Printf("Интервал тюнинга: %v", config.TuningInterval)
	log.Printf("ML минимальная уверенность: %.2f", config.ML.MinConfidence)
	log.Printf("Canary начальный процент: %d%%", config.Safety.Canary.InitialPercentage)

	// Ожидание первого цикла тюнинга
	time.Sleep(10 * time.Second)
}

// monitoringExample демонстрирует мониторинг состояния AutoTuner
func monitoringExample(ctx context.Context) {
	log.Println("\n=== Пример 3: Мониторинг состояния ===")

	// Создание AutoTuner
	autoTuner, err := tuning.NewAutoTuner(nil)
	if err != nil {
		log.Fatalf("Ошибка создания AutoTuner: %v", err)
	}

	if err := autoTuner.Start(ctx); err != nil {
		log.Fatalf("Ошибка запуска AutoTuner: %v", err)
	}
	defer autoTuner.Stop()

	// Запуск мониторинга в отдельной горутине
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Получение текущего состояния
				state := autoTuner.GetCurrentState()

				// Получение метрик
				metrics := autoTuner.GetMetrics()

				// Получение информации о ML модели
				mlInfo := autoTuner.GetMLModelInfo()

				log.Printf("--- Статус мониторинга ---")
				log.Printf("Состояние: %v", state)
				log.Printf("Время в текущем состоянии: %v", time.Since(metrics.StateStartTime))
				log.Printf("Успешных тюнингов: %d", metrics.SuccessfulTunings)
				log.Printf("Неудачных тюнингов: %d", metrics.FailedTunings)
				log.Printf("Откатов: %d", metrics.RollbackCount)
				log.Printf("Точность ML модели: %.2f%%", mlInfo.Accuracy*100)
				log.Printf("Возраст ML модели: %v", mlInfo.Age)
				log.Printf("Последний успешный тюнинг: %v", metrics.LastSuccessfulTuning)

				// Проверка здоровья системы
				if err := autoTuner.HealthCheck(); err != nil {
					log.Printf("⚠️  Проблема со здоровьем системы: %v", err)
				} else {
					log.Printf("✅ Система здорова")
				}
			}
		}
	}()

	// Ожидание для демонстрации мониторинга
	time.Sleep(30 * time.Second)
}

// manualControlExample демонстрирует ручное управление тюнингом
func manualControlExample(ctx context.Context) {
	log.Println("\n=== Пример 4: Ручное управление ===")

	// Создание AutoTuner с отключенным автоматическим тюнингом
	config := tuning.DefaultAutoTunerConfig()
	config.AutomaticTuning = false // Отключаем автоматический тюнинг

	autoTuner, err := tuning.NewAutoTuner(config)
	if err != nil {
		log.Fatalf("Ошибка создания AutoTuner: %v", err)
	}

	if err := autoTuner.Start(ctx); err != nil {
		log.Fatalf("Ошибка запуска AutoTuner: %v", err)
	}
	defer autoTuner.Stop()

	log.Println("AutoTuner запущен в ручном режиме")

	// Ручной запуск тюнинга
	log.Println("Запуск ручного тюнинга...")
	result, err := autoTuner.TuneConfiguration(ctx)
	if err != nil {
		log.Printf("Ошибка ручного тюнинга: %v", err)
	} else {
		log.Printf("Результат ручного тюнинга:")
		log.Printf("  Успех: %v", result.Success)
		log.Printf("  Улучшение: %.2f%%", result.Improvement*100)
		log.Printf("  Уверенность: %.2f%%", result.Confidence*100)
		log.Printf("  Примененные параметры: %+v", result.AppliedParameters)
		log.Printf("  Время выполнения: %v", result.ExecutionTime)
	}

	// Получение рекомендаций без применения
	log.Println("\nПолучение рекомендаций без применения...")
	recommendations, err := autoTuner.GetRecommendations(ctx)
	if err != nil {
		log.Printf("Ошибка получения рекомендаций: %v", err)
	} else {
		log.Printf("Получено %d рекомендаций:", len(recommendations))
		for i, rec := range recommendations {
			log.Printf("  %d. %s (приоритет: %v, ожидаемое улучшение: %.2f%%)",
				i+1, rec.Description, rec.Priority, rec.ExpectedImprovement*100)
		}
	}

	// Применение конкретной рекомендации
	if len(recommendations) > 0 {
		log.Printf("\nПрименение первой рекомендации...")
		err := autoTuner.ApplyRecommendation(ctx, recommendations[0])
		if err != nil {
			log.Printf("Ошибка применения рекомендации: %v", err)
		} else {
			log.Printf("Рекомендация успешно применена")
		}
	}

	// Ручной откат (если необходимо)
	log.Println("\nДемонстрация ручного отката...")
	backups, err := autoTuner.GetAvailableBackups()
	if err != nil {
		log.Printf("Ошибка получения списка backup'ов: %v", err)
	} else if len(backups) > 0 {
		log.Printf("Доступно %d backup'ов", len(backups))
		log.Printf("Последний backup: %s (создан: %v)",
			backups[0].ID, backups[0].CreatedAt)

		// Откат к последнему backup'у (для демонстрации)
		// В реальном использовании это нужно делать осторожно
		log.Printf("Выполнение отката к backup'у %s...", backups[0].ID)
		err := autoTuner.RollbackToBackup(ctx, backups[0].ID)
		if err != nil {
			log.Printf("Ошибка отката: %v", err)
		} else {
			log.Printf("Откат выполнен успешно")
		}
	}
}

// Дополнительные вспомогательные функции

// setupLogging настраивает логирование для примеров
func setupLogging() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetPrefix("[AutoTuner Example] ")
}

// waitForHealthy ожидает пока AutoTuner станет здоровым
func waitForHealthy(autoTuner *tuning.AutoTuner, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := autoTuner.HealthCheck(); err == nil {
				return nil
			}
		}
	}
}

// printConfiguration выводит текущую конфигурацию
func printConfiguration(autoTuner *tuning.AutoTuner) {
	config := autoTuner.GetCurrentConfiguration()
	log.Println("Текущая конфигурация:")
	for name, value := range config.Parameters {
		log.Printf("  %s: %v", name, value)
	}
}

// Пример использования с обработкой ошибок
func robustExample(ctx context.Context) {
	log.Println("\n=== Пример: Устойчивое использование ===")

	// Создание AutoTuner с retry логикой
	var autoTuner *tuning.AutoTuner
	var err error

	// Retry создания AutoTuner
	for attempts := 0; attempts < 3; attempts++ {
		autoTuner, err = tuning.NewAutoTuner(nil)
		if err == nil {
			break
		}
		log.Printf("Попытка %d создания AutoTuner неудачна: %v", attempts+1, err)
		time.Sleep(time.Duration(attempts+1) * time.Second)
	}

	if err != nil {
		log.Fatalf("Не удалось создать AutoTuner после 3 попыток: %v", err)
	}

	// Запуск с обработкой ошибок
	if err := autoTuner.Start(ctx); err != nil {
		log.Fatalf("Ошибка запуска AutoTuner: %v", err)
	}

	// Graceful shutdown
	defer func() {
		log.Println("Выполнение graceful shutdown...")

		// Ожидание завершения текущих операций
		if autoTuner.GetCurrentState() != tuning.StateIdle {
			log.Println("Ожидание завершения текущих операций...")
			for i := 0; i < 30; i++ { // Максимум 30 секунд
				if autoTuner.GetCurrentState() == tuning.StateIdle {
					break
				}
				time.Sleep(1 * time.Second)
			}
		}

		// Остановка AutoTuner
		if err := autoTuner.Stop(); err != nil {
			log.Printf("Ошибка остановки AutoTuner: %v", err)
		} else {
			log.Println("AutoTuner успешно остановлен")
		}
	}()

	// Ожидание готовности системы
	log.Println("Ожидание готовности системы...")
	if err := waitForHealthy(autoTuner, 30*time.Second); err != nil {
		log.Fatalf("Система не стала здоровой в течение 30 секунд: %v", err)
	}

	log.Println("Система готова к работе")

	// Основная логика работы
	time.Sleep(10 * time.Second)
}

func init() {
	setupLogging()
}
