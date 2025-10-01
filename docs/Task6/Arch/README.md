# Task 6 - Architecture Documentation

Эта папка содержит архитектурную документацию для Task 6 (Resource Management and Fault Tolerance) в формате C4 PlantUML диаграмм с подробными объяснениями связи между архитектурным дизайном и фактической реализацией кода.

## Обзор диаграмм

### 1. Context Diagram (`task6-context-diagram.puml`)
**Уровень 1 - Системный контекст**
- Показывает Task 6 как единую систему в контексте внешних пользователей и систем
- Отображает взаимодействие с администраторами, разработчиками и внешними системами мониторинга
- Демонстрирует интеграцию с Prometheus, Grafana, AlertManager

📖 **[Подробное объяснение Context Diagram](task6-context-diagram-explanation.md)**

### 2. Container Diagram (`task6-container-diagram.puml`)
**Уровень 2 - Контейнеры**
- Разбивает Task 6 на основные модули: Resource Monitor, Graceful Degradation, Auto Scaler
- Показывает взаимодействие с основными компонентами Boxo (Bitswap, Blockstore, Network)
- Отображает хранилища данных (метрики, конфигурация)

📖 **[Подробное объяснение Container Diagram](task6-container-diagram-explanation.md)**

### 3. Component Diagram (`task6-component-diagram.puml`)
**Уровень 3 - Компоненты**
- Детализирует внутреннюю структуру каждого контейнера
- Показывает основные компоненты и их взаимодействие:
  - Resource Monitor: сбор метрик, анализ трендов, система алертов
  - Graceful Degradation: управление деградацией, правила, восстановление
  - Auto Scaler: масштабирование, изоляция компонентов, история событий
  - Scalable Components: пулы воркеров, соединений, батч-процессоры

📖 **[Подробное объяснение Component Diagram](task6-component-diagram-explanation.md)**

### 4. Code Diagram (`task6-code-diagram.puml`)
**Уровень 4 - Код**
- Показывает детальную структуру классов и интерфейсов
- Отображает основные методы и поля для каждого компонента
- Демонстрирует отношения наследования и композиции
- Включает все три подсистемы Task 6 с их интерфейсами и реализациями

📖 **[Подробное объяснение Code Diagram](task6-code-diagram-explanation.md)**

### 5. Sequence Diagram (`task6-sequence-diagram.puml`)
**Диаграмма последовательности**
- Показывает временные взаимодействия между компонентами
- Демонстрирует различные сценарии:
  - Инициализация системы
  - Нормальная работа
  - Сценарий высокой нагрузки
  - Нехватка памяти
  - Отказ компонентов
  - Восстановление

📖 **[Подробное объяснение Sequence Diagram](task6-sequence-diagram-explanation.md)**

### 6. Deployment Diagram (`task6-deployment-diagram.puml`)
**Диаграмма развертывания**
- Показывает физическое развертывание системы
- Отображает Docker контейнеры и их конфигурацию
- Демонстрирует интеграцию с внешними системами мониторинга
- Включает конфигурационные параметры и ограничения ресурсов

📖 **[Подробное объяснение Deployment Diagram](task6-deployment-diagram-explanation.md)**

## Ключевые архитектурные решения

### Модульность
- Task 6 разделен на три независимых модуля с четкими интерфейсами
- Каждый модуль может работать автономно или в составе системы
- Общие компоненты (PerformanceMonitor, StructuredLogger) переиспользуются

### Масштабируемость
- Система поддерживает 5 типов масштабируемых компонентов
- Автоматическое масштабирование на основе метрик производительности
- Изоляция проблемных компонентов с автоматическим восстановлением

### Отказоустойчивость
- 13 типов действий для graceful degradation
- Многоуровневая система деградации (None → Light → Moderate → Severe → Critical)
- Автоматическое восстановление при нормализации метрик

### Мониторинг
- Интеграция с Prometheus для экспорта метрик
- Структурированное логирование с контекстом
- Множественные каналы уведомлений (Slack, Email, PagerDuty)

## Использование диаграмм

Для просмотра диаграмм используйте:
1. **PlantUML плагин** в IDE (VS Code, IntelliJ IDEA)
2. **Онлайн PlantUML сервер**: http://www.plantuml.com/plantuml/
3. **Локальный PlantUML**: установите Java и plantuml.jar

### Пример команды для генерации PNG:
```bash
java -jar plantuml.jar -tpng task6-context-diagram.puml
```

## Связь с реализацией

Диаграммы соответствуют реальной реализации в папке `monitoring/`:
- `resource_monitor.go` - Resource Monitor
- `graceful_degradation.go` - Graceful Degradation Manager  
- `auto_scaler.go` - Auto Scaler
- `scalable_components.go` - Scalable Components

Тесты находятся в соответствующих `*_test.go` файлах.