# Task 7.1 - Справочник команд нагрузочного тестирования

## Полный справочник команд Task 7.1

Этот документ содержит все команды, использованные при реализации Task 7.1 "Создать нагрузочные тесты для Bitswap", с подробными объяснениями и примерами использования.

## 🔗 Команды тестирования 10,000+ одновременных соединений

### Основные команды

```bash
# Базовый тест 10K соединений
go test -v -timeout=30m -run="TestBitswapComprehensiveLoadSuite/Test_10K_ConcurrentConnections" ./loadtest
```
**Назначение**: Тестирование системы с 10,000 одновременными соединениями
**Критерии успеха**: Латентность <100ms для 95% запросов, успешность >95%
**Продолжительность**: ~5-10 минут
**Ресурсы**: 4+ CPU ядра, 8GB+ RAM

```bash
# Автоматизированный запуск через Makefile
make -f Makefile.loadtest test-concurrent
```
**Назначение**: Упрощенный запуск тестов одновременных соединений
**Преимущества**: Предустановленные параметры, автоматическая настройка окружения
**Результаты**: Сохраняются в `test_results/`

```bash
# Тест с детекцией race conditions
go test -v -timeout=15m -race -run="TestBitswapComprehensiveLoadSuite/Test_10K_ConcurrentConnections" ./loadtest
```
**Назначение**: Проверка потокобезопасности при высокой конкурентности
**Важность**: Критично для многопоточных операций
**Замедление**: ~10x медленнее обычного теста

### Расширенные тесты соединений

```bash
# Тест 15K соединений
CONCURRENT_CONNECTIONS=15000 go test -v -timeout=25m -run="TestBitswapComprehensiveLoadSuite/Test_10K_ConcurrentConnections" ./loadtest
```

```bash
# Тест 25K соединений (экстремальная нагрузка)
CONCURRENT_CONNECTIONS=25000 go test -v -timeout=20m -run="TestBitswapComprehensiveLoadSuite/Test_10K_ConcurrentConnections" ./loadtest
```

```bash
# Тест 50K соединений (максимальная нагрузка)
CONCURRENT_CONNECTIONS=50000 go test -v -timeout=15m -run="TestBitswapComprehensiveLoadSuite/Test_Extreme_Load_Scenarios" ./loadtest
```

## 🚀 Команды тестирования 100,000+ запросов в секунду

### Основные команды

```bash
# Базовый тест 100K RPS
go test -v -timeout=20m -run="TestBitswapComprehensiveLoadSuite/Test_100K_RequestsPerSecond" ./loadtest
```
**Назначение**: Валидация высокой пропускной способности
**Целевые показатели**: 100K+ запросов/сек с достижением 60%+
**Критерии**: P95 латентность <200ms, автомасштабирование

```bash
# Автоматизированный запуск
make -f Makefile.loadtest test-throughput
```
**Назначение**: Упрощенный запуск тестов пропускной способности
**Конфигурация**: Автоматическая настройка параметров нагрузки

```bash
# Бенчмарк пропускной способности
go test -v -timeout=15m -bench="BenchmarkBitswap.*Throughput" -benchmem -run="^$" ./loadtest
```
**Назначение**: Измерение пропускной способности с профилированием памяти
**Результаты**: Детальные метрики производительности

### Градуированные тесты пропускной способности

```bash
# 50K RPS (достижение 80%)
TARGET_RPS=50000 go test -v -timeout=15m -run="TestBitswapComprehensiveLoadSuite/Test_100K_RequestsPerSecond" ./loadtest
```

```bash
# 75K RPS (достижение 70%)
TARGET_RPS=75000 go test -v -timeout=18m -run="TestBitswapComprehensiveLoadSuite/Test_100K_RequestsPerSecond" ./loadtest
```

```bash
# 150K RPS (достижение 40%)
TARGET_RPS=150000 go test -v -timeout=12m -run="TestBitswapComprehensiveLoadSuite/Test_100K_RequestsPerSecond" ./loadtest
```

```bash
# 200K RPS (экстремальная нагрузка)
TARGET_RPS=200000 go test -v -timeout=10m -run="TestBitswapComprehensiveLoadSuite/Test_Extreme_Load_Scenarios" ./loadtest
```

## ⏱️ Команды тестирования 24+ часовой стабильности

### Основные команды

```bash
# Полный 24-часовой тест стабильности
BITSWAP_STABILITY_DURATION=24h go test -v -timeout=25h -run="TestBitswapComprehensiveLoadSuite/Test_24Hour_Stability" ./loadtest
```
**Назначение**: Проверка долгосрочной стабильности системы
**Критерии**: Рост памяти <5%, успешность >98%, отсутствие утечек
**Ресурсы**: 16GB+ RAM, выделенная среда

```bash
# Автоматизированный запуск
make -f Makefile.loadtest test-stability
```
**Назначение**: Запуск с настройками по умолчанию (30 минут для разработки)
**Конфигурация**: Через переменную `BITSWAP_STABILITY_DURATION`

### Сокращенные тесты стабильности

```bash
# 30-минутный тест (CI/CD)
BITSWAP_STABILITY_DURATION=30m go test -v -timeout=35m -run="TestBitswapComprehensiveLoadSuite/Test_24Hour_Stability" ./loadtest
```
**Назначение**: CI-дружественная версия теста стабильности
**Применение**: Автоматизированное тестирование, smoke tests

```bash
# 1-часовой тест (промежуточная проверка)
BITSWAP_STABILITY_DURATION=1h go test -v -timeout=65m -run="TestBitswapComprehensiveLoadSuite/Test_24Hour_Stability" ./loadtest
```
**Назначение**: Проверка основных показателей стабильности
**Баланс**: Между скоростью выполнения и полнотой проверки

```bash
# 4-часовой тест (расширенная проверка)
BITSWAP_STABILITY_DURATION=4h go test -v -timeout=4h30m -run="TestBitswapComprehensiveLoadSuite/Test_24Hour_Stability" ./loadtest
```
**Назначение**: Детальная проверка стабильности без полного 24h цикла

### Тесты различных уровней нагрузки

```bash
# Легкая нагрузка (500 RPS)
STABILITY_LOAD=light BITSWAP_STABILITY_DURATION=2h go test -v -timeout=2h30m -run="TestBitswapComprehensiveLoadSuite/Test_24Hour_Stability" ./loadtest
```

```bash
# Средняя нагрузка (1000 RPS)
STABILITY_LOAD=medium BITSWAP_STABILITY_DURATION=2h go test -v -timeout=2h30m -run="TestBitswapComprehensiveLoadSuite/Test_24Hour_Stability" ./loadtest
```

```bash
# Высокая нагрузка (2000 RPS)
STABILITY_LOAD=high BITSWAP_STABILITY_DURATION=2h go test -v -timeout=2h30m -run="TestBitswapComprehensiveLoadSuite/Test_24Hour_Stability" ./loadtest
```

## 📈 Команды автоматизированных бенчмарков

### Основные команды

```bash
# Полный набор автоматизированных бенчмарков
go test -v -timeout=15m -run="TestBitswapComprehensiveLoadSuite/Test_Automated_Benchmarks" ./loadtest
```
**Назначение**: Систематическая оценка производительности на различных масштабах
**Масштабы**: Small (10 узлов) → XLarge (100 узлов)
**Метрики**: RPS, эффективность, использование ресурсов

```bash
# Автоматизированный запуск
make -f Makefile.loadtest test-benchmarks
```
**Назначение**: Упрощенный запуск всех бенчмарков производительности

```bash
# Все Go бенчмарки
go test -v -timeout=30m -bench=. -benchmem -run="^$" ./loadtest > test_results/benchmark_results.txt
```
**Назначение**: Запуск всех Go бенчмарков с сохранением результатов
**Профилирование**: Включает измерение использования памяти

```bash
# Автоматизированный запуск Go бенчмарков
make -f Makefile.loadtest bench
```

### Специализированные бенчмарки

```bash
# Бенчмарки масштабируемости
go test -v -timeout=20m -bench="BenchmarkBitswap.*Scaling" -benchmem -run="^$" ./loadtest
```
**Назначение**: Проверка поведения системы при увеличении количества узлов
**Масштабы**: 10, 25, 50, 100, 200 узлов

```bash
# Автоматизированные бенчмарки масштабируемости
make -f Makefile.loadtest bench-scaling
```

```bash
# Бенчмарки латентности
go test -v -timeout=15m -bench="BenchmarkBitswap.*Latency" -benchmem -run="^$" ./loadtest
```
**Назначение**: Измерение времени отклика при различных сетевых условиях
**Условия**: 0ms (идеальная сеть) → 500ms (очень плохая сеть)

```bash
# Автоматизированные бенчмарки латентности
make -f Makefile.loadtest bench-latency
```

### Бенчмарки по масштабам

```bash
# Small Scale (10 узлов, 500 блоков, 100 запросов)
BENCHMARK_SCALE=small go test -v -timeout=5m -run="TestBitswapComprehensiveLoadSuite/Test_Automated_Benchmarks/Benchmark_Small_Scale" ./loadtest
```

```bash
# Medium Scale (25 узлов, 1000 блоков, 500 запросов)
BENCHMARK_SCALE=medium go test -v -timeout=8m -run="TestBitswapComprehensiveLoadSuite/Test_Automated_Benchmarks/Benchmark_Medium_Scale" ./loadtest
```

```bash
# Large Scale (50 узлов, 2000 блоков, 1000 запросов)
BENCHMARK_SCALE=large go test -v -timeout=12m -run="TestBitswapComprehensiveLoadSuite/Test_Automated_Benchmarks/Benchmark_Large_Scale" ./loadtest
```

```bash
# XLarge Scale (100 узлов, 5000 блоков, 2000 запросов)
BENCHMARK_SCALE=xlarge go test -v -timeout=20m -run="TestBitswapComprehensiveLoadSuite/Test_Automated_Benchmarks/Benchmark_XLarge_Scale" ./loadtest
```

## 🔥 Команды комплексного тестирования

### Полные наборы тестов

```bash
# Полный комплексный набор (может занять 26+ часов)
go test -v -timeout=26h -run="TestBitswapComprehensiveLoadSuite" ./loadtest
```
**Назначение**: Запуск всех категорий нагрузочных тестов
**Включает**: 10K+ соединения, 100K+ RPS, 24h стабильность, бенчмарки, экстремальные сценарии
**Требования**: Выделенная среда, 32GB+ RAM, 16+ CPU ядер

```bash
# Автоматизированный полный запуск
make -f Makefile.loadtest test-all
```
**Назначение**: Полная автоматизация всего набора тестов
**Дополнительно**: Включает стресс-тесты и генерацию отчетов

```bash
# Стресс-тесты
go test -v -timeout=2h -run="TestBitswapStress" ./loadtest
```
**Назначение**: Тестирование поведения в экстремальных условиях
**Сценарии**: Нехватка ресурсов, восстановление после сбоев

```bash
# Автоматизированные стресс-тесты
make -f Makefile.loadtest test-stress
```

### CI/CD оптимизированные команды

```bash
# CI-дружественный набор тестов
BITSWAP_STABILITY_DURATION=5m go test -v -timeout=45m -run="TestBitswapComprehensiveLoadSuite" ./loadtest
```
**Назначение**: Сокращенная версия для автоматизированного тестирования
**Оптимизация**: Уменьшенная продолжительность, сохранение покрытия

```bash
# Автоматизированный CI запуск
make -f Makefile.loadtest test-ci
```

```bash
# Smoke test (быстрая проверка инфраструктуры)
go test -v -timeout=2m -run="TestBitswapComprehensiveLoadSuite/Test_Automated_Benchmarks/Benchmark_Small_Scale" ./loadtest
```
**Назначение**: Быстрая проверка работоспособности системы тестирования
**Время выполнения**: ~1-2 минуты

```bash
# Автоматизированный smoke test
make -f Makefile.loadtest test-smoke
```

## 🔬 Команды расширенного тестирования

### Экстремальные сценарии

```bash
# Экстремальные сценарии нагрузки
go test -v -timeout=15m -run="TestBitswapComprehensiveLoadSuite/Test_Extreme_Load_Scenarios" ./loadtest
```
**Назначение**: Тестирование предельных возможностей системы
**Сценарии**: 50K+ соединений, 200K+ RPS, большие блоки, множество мелких блоков

```bash
# Автоматизированные экстремальные тесты
make -f Makefile.loadtest test-extreme
```

### Тесты обнаружения утечек памяти

```bash
# Обнаружение утечек памяти
go test -v -timeout=45m -run="TestBitswapComprehensiveLoadSuite/Test_Memory_Leak_Detection" ./loadtest
```
**Назначение**: Выявление утечек памяти через множественные циклы
**Циклы**: Короткие (15 мин × 10) и длинные (30 мин × 5)
**Критерии**: Общий рост памяти <10%, рост за цикл <5%

```bash
# Автоматизированные тесты утечек памяти
make -f Makefile.loadtest test-memory
```

### Тесты сетевых условий

```bash
# Тестирование различных сетевых условий
go test -v -timeout=15m -run="TestBitswapComprehensiveLoadSuite/Test_Network_Conditions" ./loadtest
```
**Назначение**: Проверка производительности при различной латентности сети
**Условия**: Perfect (0ms), LAN (1ms), Fast WAN (10ms), Slow WAN (50ms), Satellite (200ms), Very Poor (500ms)

```bash
# Автоматизированные тесты сетевых условий
make -f Makefile.loadtest test-network
```

## 🔍 Команды профилирования и анализа

### CPU профилирование

```bash
# CPU профилирование во время нагрузочных тестов
go test -v -timeout=10m -cpuprofile=test_results/cpu.prof -run="TestBitswapComprehensiveLoadSuite/Test_Automated_Benchmarks" ./loadtest
```
**Назначение**: Анализ использования CPU для выявления узких мест
**Результат**: Файл `cpu.prof` для анализа с `go tool pprof`

```bash
# Автоматизированное CPU профилирование
make -f Makefile.loadtest profile-cpu
```

```bash
# Анализ CPU профиля
go tool pprof test_results/cpu.prof
```
**Команды в pprof**: `top`, `list`, `web`, `svg`

### Профилирование памяти

```bash
# Профилирование памяти
go test -v -timeout=10m -memprofile=test_results/mem.prof -run="TestBitswapComprehensiveLoadSuite/Test_Memory_Leak_Detection" ./loadtest
```
**Назначение**: Анализ использования памяти и выявление утечек
**Результат**: Файл `mem.prof` для детального анализа

```bash
# Автоматизированное профилирование памяти
make -f Makefile.loadtest profile-memory
```

```bash
# Анализ профиля памяти
go tool pprof test_results/mem.prof
```
**Команды в pprof**: `top`, `list`, `web`, `alloc_space`, `inuse_space`

### Анализ покрытия кода

```bash
# Анализ покрытия кода нагрузочными тестами
go test -v -timeout=15m -coverprofile=test_results/coverage.out -run="TestBitswapComprehensiveLoadSuite/Test_Automated_Benchmarks" ./loadtest
```
**Назначение**: Проверка полноты покрытия кода тестами

```bash
# Генерация HTML отчета покрытия
go tool cover -html=test_results/coverage.out -o test_results/coverage.html
```
**Результат**: Визуальный HTML отчет покрытия кода

```bash
# Автоматизированный анализ покрытия
make -f Makefile.loadtest test-coverage
```

### Race condition detection

```bash
# Обнаружение race conditions
go test -v -timeout=15m -race -run="TestBitswapComprehensiveLoadSuite/Test_10K_ConcurrentConnections" ./loadtest
```
**Назначение**: Выявление состояний гонки в многопоточном коде
**Важность**: Критично для высококонкурентных систем
**Замедление**: Значительное (5-10x)

```bash
# Автоматизированная проверка race conditions
make -f Makefile.loadtest test-race
```

## 📊 Команды мониторинга и логирования

### Подробное логирование

```bash
# Запуск с подробным логированием
go test -v -timeout=30m -run="TestBitswapComprehensiveLoadSuite" ./loadtest 2>&1 | tee test_results/test_output.log
```
**Назначение**: Сохранение всего вывода тестов для последующего анализа
**Результат**: Файл `test_output.log` с полными логами

```bash
# Автоматизированное подробное логирование
make -f Makefile.loadtest test-verbose
```

### Мониторинг системных ресурсов

```bash
# Мониторинг ресурсов во время тестов
make -f Makefile.loadtest monitor
```
**Назначение**: Отслеживание CPU, памяти, горутин во время выполнения тестов
**Интервал**: Каждые 5 секунд
**Результат**: Файл `system_monitor.log`

**Пример вывода**:
```
2024-01-01 12:00:00: CPU: 45.2%, Memory: 67.8%, Goroutines: 15432
2024-01-01 12:00:05: CPU: 52.1%, Memory: 71.3%, Goroutines: 18765
```

## 🛠️ Команды валидации и настройки

### Валидация окружения

```bash
# Проверка готовности системы для нагрузочного тестирования
make -f Makefile.loadtest validate-env
```
**Проверки**:
- Версия Go
- Доступная память
- Количество CPU ядер
- Лимиты файловых дескрипторов
- Свободное место на диске

### Системные требования

```bash
# Отображение системных требований
make -f Makefile.loadtest requirements
```
**Информация**:
- Минимальные требования
- Рекомендуемые требования
- Текущие системные характеристики
- Сравнение с требованиями

### Очистка результатов

```bash
# Очистка всех результатов тестов
make -f Makefile.loadtest clean
```
**Удаляет**:
- Директорию `test_results/`
- Файлы профилирования (*.prof)
- Временные исполняемые файлы (*.test)
- Логи (*.log)

```bash
# Ручная очистка
rm -rf test_results/
rm -f *.prof *.test cpu.out mem.out
```

## 🐳 Команды Docker-изолированного тестирования

### Docker тестирование

```bash
# Запуск тестов в Docker контейнере
make -f Makefile.loadtest docker-test
```
**Преимущества**: Изолированная среда, воспроизводимые результаты
**Конфигурация**: Автоматическое монтирование рабочей директории

```bash
# Прямой запуск в Docker
docker run --rm -v $(pwd):/workspace -w /workspace/bitswap \
  -e OUTPUT_DIR=/workspace/test_results \
  golang:1.21 make -f Makefile.loadtest test-ci
```
**Назначение**: Ручной контроль Docker окружения
**Переменные**: Настройка через `-e`

## 📋 Команды генерации отчетов

### Комплексные отчеты

```bash
# Генерация полного отчета по результатам
make -f Makefile.loadtest report
```
**Содержание**: Агрегация всех JSON результатов в markdown отчет
**Результат**: Файл `test_results/report.md`

```bash
# Отображение сводки результатов
make -f Makefile.loadtest results
```
**Назначение**: Быстрый просмотр основных результатов
**Источник**: Файл `load_test_summary.txt`

### Примеры конфигураций

```bash
# Создание примеров конфигурационных файлов
make -f Makefile.loadtest config-examples
```
**Создает**:
- `config_production.json` - полное продакшн тестирование
- `config_ci.json` - CI/CD дружественное тестирование

## 📈 Команды с переменными окружения

### Основные переменные

```bash
# Продолжительность тестов стабильности
export BITSWAP_STABILITY_DURATION=24h

# Директория результатов
export OUTPUT_DIR=./my_test_results

# Количество CPU ядер
export GOMAXPROCS=16

# Включение экстремальных тестов
export BITSWAP_EXTREME_DURATION=true
```

### Примеры использования переменных

```bash
# Кастомная конфигурация для продакшн тестирования
BITSWAP_STABILITY_DURATION=24h \
GOMAXPROCS=32 \
OUTPUT_DIR=production_results \
make -f Makefile.loadtest test-all
```

```bash
# Быстрое тестирование для разработки
BITSWAP_STABILITY_DURATION=5m \
GOMAXPROCS=8 \
OUTPUT_DIR=dev_results \
make -f Makefile.loadtest test-ci
```

```bash
# Экстремальное тестирование
BITSWAP_EXTREME_DURATION=true \
CONCURRENT_CONNECTIONS=50000 \
TARGET_RPS=200000 \
GOMAXPROCS=32 \
make -f Makefile.loadtest test-extreme
```

## 🎯 Команды для специфических сценариев

### Отладка производительности

```bash
# Комбинированное профилирование
go test -v -timeout=10m \
  -cpuprofile=cpu.prof \
  -memprofile=mem.prof \
  -blockprofile=block.prof \
  -run="TestBitswapComprehensiveLoadSuite/Test_Automated_Benchmarks" ./loadtest
```

### Непрерывное тестирование

```bash
# Циклическое выполнение тестов
while true; do
  make -f Makefile.loadtest test-smoke
  sleep 300  # 5 минут между циклами
done
```

### Параллельное тестирование

```bash
# Параллельный запуск различных типов тестов
make -f Makefile.loadtest test-concurrent &
make -f Makefile.loadtest test-throughput &
make -f Makefile.loadtest test-benchmarks &
wait
```

## 📊 Интерпретация результатов команд

### Критерии успеха

**Тесты 10K+ соединений**:
- Успешность > 95%
- Средняя латентность < 100ms
- P95 латентность < 100ms
- Отсутствие утечек памяти

**Тесты 100K+ RPS**:
- Достижение целевого RPS > 60%
- P95 латентность < 200ms
- Автомасштабирование работает
- Стабильная производительность

**Тесты 24h стабильности**:
- Рост памяти < 5%
- Успешность > 98%
- Отсутствие деградации производительности
- Стабильные метрики в течение времени

**Автоматизированные бенчмарки**:
- Линейная масштабируемость
- Эффективность > 80% от теоретического максимума
- Стабильные результаты между запусками
- Отсутствие регрессий производительности

Этот справочник команд обеспечивает полное покрытие всех аспектов нагрузочного тестирования Task 7.1 и служит исчерпывающим руководством для использования системы тестирования.