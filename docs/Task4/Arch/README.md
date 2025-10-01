# Task 4: Архитектурная документация - Сетевая оптимизация для кластерной среды

Эта папка содержит полную архитектурную документацию для Task 4 в формате C4 PlantUML, служащую мостом между архитектурным дизайном и фактической реализацией кода.

## 📁 Структура документации

### 📋 Основная документация
- [`Diagram-Explanations.md`](./Diagram-Explanations.md) - **Подробные объяснения каждой диаграммы с привязкой к коду**
- [`Implementation-Bridge.md`](./Implementation-Bridge.md) - **Практические примеры использования диаграмм для разработки**

### 🏗️ C4 архитектурные диаграммы

#### 📊 Обзорные диаграммы
- [`architecture-overview.puml`](./architecture-overview.puml) - Общая архитектура Task 4 с размерами кода и метриками
- [`c4-level1-context.puml`](./c4-level1-context.puml) - Контекстная диаграмма в экосистеме IPFS кластера

#### 🏢 Контейнерный уровень
- [`c4-level2-container.puml`](./c4-level2-container.puml) - Основные контейнеры и их взаимодействия

#### 🔧 Компонентный уровень
- [`c4-level3-component-task41.puml`](./c4-level3-component-task41.puml) - **Task 4.1**: Адаптивный менеджер соединений
- [`c4-level3-component-task42.puml`](./c4-level3-component-task42.puml) - **Task 4.2**: Мониторинг качества соединений  
- [`c4-level3-component-task43.puml`](./c4-level3-component-task43.puml) - **Task 4.3**: Буферизация и Keep-Alive

#### 💻 Кодовый уровень
- [`c4-level4-code-data-flow.puml`](./c4-level4-code-data-flow.puml) - Детальный поток данных между компонентами

#### 🚀 Развертывание
- [`deployment-diagram.puml`](./deployment-diagram.puml) - Диаграмма развертывания для кластерной среды

#### ⏱️ Последовательности
- [`sequence-adaptive-connection.puml`](./sequence-adaptive-connection.puml) - Последовательность адаптивного управления соединениями
- [`sequence-buffer-keepalive.puml`](./sequence-buffer-keepalive.puml) - Последовательность буферизации и Keep-Alive

## 🎯 Как использовать эту документацию

### Для разработчиков
1. **Начните с [`Diagram-Explanations.md`](./Diagram-Explanations.md)** - понимание каждой диаграммы
2. **Изучите [`Implementation-Bridge.md`](./Implementation-Bridge.md)** - практические примеры
3. **Используйте component диаграммы** для определения интерфейсов
4. **Следуйте sequence диаграммам** для реализации логики

### Для архитекторов
1. **Context диаграмма** - понимание места в экосистеме
2. **Container диаграмма** - высокоуровневая архитектура
3. **Component диаграммы** - детальная структура каждой подзадачи
4. **Deployment диаграмма** - стратегия развертывания

### Для DevOps
1. **Deployment диаграмма** - конфигурация кластера
2. **Sequence диаграммы** - понимание временных циклов
3. **Implementation Bridge** - готовые конфигурации Docker/K8s

## 🏗️ Архитектурные компоненты Task 4

### Task 4.1: Адаптивный менеджер соединений
```go
// Основные компоненты из диаграммы:
type AdaptiveConnManager struct {
    loadMonitor      *LoadMonitor           // Мониторинг нагрузки
    clusterPool      *ClusterConnectionPool // Кластерные соединения  
    connectionTracker *ConnectionTracker     // Отслеживание соединений
    adaptationEngine *AdaptationEngine      // Алгоритмы адаптации
}
```

**Ключевые возможности:**
- Динамические лимиты HighWater/LowWater (1000-3000)
- Автоматическая настройка GracePeriod (15s-45s)
- Пулы постоянных соединений для кластерных узлов
- Адаптация к нагрузке каждые 30 секунд

### Task 4.2: Мониторинг качества соединений
```go
// Структура ConnectionQuality из диаграммы:
type ConnectionQuality struct {
    Latency      time.Duration // Текущая латентность
    Bandwidth    float64       // Пропускная способность  
    ErrorRate    float64       // Частота ошибок
    QualityScore float64       // Комплексная оценка (0-1)
}
```

**Алгоритм оценки качества:**
- Латентность: 40% веса (0-200ms → 1.0-0.0)
- Пропускная способность: 30% веса (0-10MB/s → 0.0-1.0)  
- Частота ошибок: 30% веса (0-10% → 1.0-0.0)

### Task 4.3: Буферизация и Keep-Alive
```go
// BDP алгоритм из диаграммы:
func (abs *AdaptiveBufferSizer) CalculateBDP(bandwidth float64, latency time.Duration) int {
    bdp := bandwidth * float64(latency.Nanoseconds()) / 1e9
    return clamp(int(bdp), 4*1024, 1024*1024) // 4KB-1MB лимиты
}
```

**Интеллектуальное Keep-Alive:**
- Адаптивные интервалы на основе важности (0.5-1.0)
- Кластерные пиры: интервал/2 для приоритета
- Прогрессивное обнаружение сбоев (3 попытки)

## 📊 Производительные характеристики

### Целевые метрики (из architecture-overview.puml)
- **Адаптация**: ~1M ops/sec для обновления лимитов
- **Качество**: ~5M ops/sec для расчета метрик  
- **Масштабируемость**: до 10K одновременных соединений
- **Латентность**: P95 < 100ms для адаптации

### BDP оптимизация (из Task 4.3)
- **Алгоритм**: `optimal_buffer = bandwidth × latency`
- **Лимиты**: 4KB ≤ buffer ≤ 1MB
- **Адаптация**: Низкая ПС (×0.75), Высокая ПС (×1.5)

### Обнаружение медленных соединений
- **Порог**: 500ms латентность
- **Выборка**: 5 измерений для статистики
- **Подтверждение**: 3 обнаружения для активации обхода

## 🔗 Связь диаграмм с кодом

### Файловая структура
```
bitswap/network/
├── adaptive_conn_manager.go      # Task 4.1 - 1,200+ строк
├── connection_quality_monitor.go # Task 4.2 - 400+ строк  
├── buffer_keepalive_manager.go   # Task 4.3 - 1,200+ строк
├── load_monitor.go               # Мониторинг нагрузки
├── cluster_pool.go               # Кластерные соединения
└── interfaces.go                 # Интерфейсы из диаграмм
```

### Тестирование
```
bitswap/network/
├── adaptive_conn_manager_test.go      # Unit тесты - 800+ строк
├── connection_quality_monitor_test.go # Unit тесты - 600+ строк
├── buffer_keepalive_manager_test.go   # Unit тесты - 900+ строк
├── integration_test.go                # Интеграционные тесты
└── benchmark_test.go                  # Бенчмарки производительности
```

## 🛠️ Инструменты для работы с диаграммами

### Просмотр диаграмм
- [PlantUML Online Server](http://www.plantuml.com/plantuml/uml/)
- VS Code с расширением PlantUML
- IntelliJ IDEA с плагином PlantUML

### Генерация документации
```bash
# Генерация PNG из PlantUML файлов
plantuml -tpng docs/Task4/Arch/*.puml

# Генерация SVG для веб-документации  
plantuml -tsvg docs/Task4/Arch/*.puml
```

### Валидация архитектуры
```bash
# Запуск тестов соответствия архитектуре
go test -v ./bitswap/network/... -run TestArchitectureCompliance

# Проверка производительных метрик
go test -v ./bitswap/network/... -bench=BenchmarkTask4
```

## 📚 Связанная документация

- **Исходный код**: [`../../bitswap/network/`](../../bitswap/network/) - Реализация Task 4
- **Тесты**: [`../../bitswap/network/*_test.go`](../../bitswap/network/) - Unit и интеграционные тесты  
- **Бенчмарки**: [`../../bitswap/network/*_bench_test.go`](../../bitswap/network/) - Тесты производительности
- **Конфигурация**: [`../../examples/task4/`](../../examples/task4/) - Примеры конфигурации

## 🎯 Быстрый старт

1. **Изучите архитектуру**: Начните с [`Diagram-Explanations.md`](./Diagram-Explanations.md)
2. **Понимайте реализацию**: Читайте [`Implementation-Bridge.md`](./Implementation-Bridge.md)  
3. **Запустите тесты**: `go test -v ./bitswap/network/...`
4. **Развертывайте кластер**: Используйте конфигурации из deployment диаграммы

Эти диаграммы - не просто документация, а живой мост между архитектурным дизайном и работающим кодом Task 4!