# Task 2: Архитектурная документация

Эта папка содержит архитектурную документацию для Task 2 - "Оптимизация Bitswap для высокой пропускной способности" в формате C4 PlantUML.

## Структура документации

### 📋 Обзорная документация
- [`Architecture-Overview.md`](./Architecture-Overview.md) - Полный архитектурный обзор Task 2

### 🏗️ C4 диаграммы

#### Level 1: Context Diagram
- [`C4-Level1-Context.puml`](./C4-Level1-Context.puml) - Контекстная диаграмма системы

#### Level 2: Container Diagram  
- [`C4-Level2-Container.puml`](./C4-Level2-Container.puml) - Диаграмма контейнеров и их взаимодействий

#### Level 3: Component Diagrams
- [`C4-Level3-Component-AdaptiveConfig.puml`](./C4-Level3-Component-AdaptiveConfig.puml) - Task 2.1: Adaptive Configuration Management
- [`C4-Level3-Component-PrioritySystem.puml`](./C4-Level3-Component-PrioritySystem.puml) - Task 2.2: Request Prioritization System
- [`C4-Level3-Component-BatchProcessor.puml`](./C4-Level3-Component-BatchProcessor.puml) - Task 2.3: Batch Request Processing

#### Level 4: Code Level
- [`C4-Level4-Code-DataFlow.puml`](./C4-Level4-Code-DataFlow.puml) - Детальный поток данных и взаимодействие компонентов

#### Deployment
- [`C4-Deployment-Diagram.puml`](./C4-Deployment-Diagram.puml) - Диаграмма развертывания для высоконагруженной среды

## Ключевые архитектурные компоненты

### Task 2.1: Adaptive Configuration Management
- **AdaptiveBitswapConfig**: Центральная конфигурация с автоматической адаптацией
- **AdaptiveConnectionManager**: Управление соединениями с per-peer лимитами
- **Adaptation Engine**: Алгоритмы масштабирования на основе нагрузки

### Task 2.2: Request Prioritization System
- **PriorityRequestManager**: Менеджер приоритизации запросов
- **Request Classifier**: Классификация по критичности (Critical/High/Normal/Low)
- **Priority Queues**: Heap-based очереди для каждого приоритета
- **Resource Pool**: Управление ресурсами и лимитами

### Task 2.3: Batch Request Processing
- **BatchRequestProcessor**: Батчевая обработка для повышения пропускной способности
- **BatchWorkerPool**: Пулы воркеров с распределением по приоритетам
- **Batch Accumulator**: Накопление запросов с адаптивными размерами батчей

### Task 2.4: Circuit Breaker Protection
- **Circuit Breaker**: Защита от каскадных сбоев
- **Health Monitoring**: Мониторинг состояния системы
- **Graceful Recovery**: Постепенное восстановление после сбоев

## Производительные характеристики

### Целевые показатели
- **Пропускная способность**: >1000 одновременных запросов
- **Латентность**: P95 < 100ms
- **Масштабируемость**: Линейное масштабирование до 10K соединений
- **Адаптивность**: Автоматическая настройка каждые 30 секунд

### Адаптивные параметры
- **MaxOutstandingBytesPerPeer**: 1MB - 100MB (динамический)
- **WorkerCount**: 128 - 2048 (автомасштабирование)
- **BatchSize**: 100 - 1000 (адаптивный)
- **Priority Thresholds**: 10ms (Critical), 50ms (High)

## Мониторинг и метрики

### Load Monitor
- RPS, Average/P95 Response Time
- Outstanding Requests, Active Connections
- Циркулярный буфер на 1000 образцов для P95

### Экспортируемые метрики
- Конфигурационные параметры
- Метрики производительности
- Статистика приоритизации
- Метрики батчевой обработки

## Как использовать диаграммы

1. **Для понимания общей архитектуры**: начните с Context и Container диаграмм
2. **Для изучения конкретных компонентов**: используйте Component диаграммы
3. **Для понимания потоков данных**: изучите Code DataFlow диаграмму
4. **Для планирования развертывания**: используйте Deployment диаграмму

## Инструменты для просмотра

Диаграммы созданы в формате PlantUML и могут быть просмотрены с помощью:
- [PlantUML Online Server](http://www.plantuml.com/plantuml/uml/)
- VS Code с расширением PlantUML
- IntelliJ IDEA с плагином PlantUML
- Любой редактор с поддержкой PlantUML

## Связанная документация

- [Task 2 Implementation Files](../../bitswap/adaptive/) - Исходный код реализации
- [Task 2 Tests](../../bitswap/adaptive/*_test.go) - Unit и интеграционные тесты
- [Task 2 Benchmarks](../../bitswap/adaptive/*_bench_test.go) - Бенчмарки производительности