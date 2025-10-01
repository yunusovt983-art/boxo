# 🚀 Быстрый старт с архитектурными диаграммами Task 5

## 📋 Что вы получите

После изучения этого руководства вы сможете:
- Читать и понимать архитектурные диаграммы Task 5
- Использовать диаграммы для разработки кода
- Валидировать код на соответствие архитектуре
- Интегрировать диаграммы в свой рабочий процесс

## ⚡ 5-минутный старт

### 1. Просмотр диаграмм (2 минуты)

**Откройте [index.html](index.html)** в браузере для интерактивной навигации по диаграммам.

**Или используйте PlantUML Online:**
1. Скопируйте содержимое любого `.puml` файла
2. Вставьте в [PlantUML Online Server](https://www.plantuml.com/plantuml/uml/)
3. Получите визуализацию диаграммы

### 2. Понимание структуры (2 минуты)

**Начните с Context Diagram:**
```
Пользователи → Система мониторинга → Boxo компоненты
```

**Затем изучите Container Diagram:**
```
Performance Monitor → Bottleneck Analyzer → Alert Manager → Auto Tuner
```

### 3. Связь с кодом (1 минута)

**Каждая диаграмма соответствует коду:**
- `PerformanceMonitor` интерфейс → `bitswap/monitoring/performance/monitor.go`
- `BitswapMetrics` структура → `bitswap/monitoring/types/metrics.go`
- `TunerStateMachine` → `bitswap/monitoring/tuning/state_machine.go`

## 🎯 Основные сценарии использования

### Сценарий 1: Я новый разработчик в команде

**Цель:** Быстро понять архитектуру системы мониторинга

**Действия:**
1. Изучите [Context Diagram](context-diagram.puml) - общее понимание системы
2. Прочитайте [Container Diagram](container-diagram.puml) - основные компоненты
3. Посмотрите [Sequence Diagram](sequence-diagram.puml) - как компоненты взаимодействуют
4. Изучите [Class Diagram](class-diagram.puml) - структуры данных

**Результат:** Понимание архитектуры за 15-20 минут

### Сценарий 2: Я добавляю новую функциональность

**Цель:** Понять, где и как добавить новый код

**Действия:**
1. Найдите соответствующий компонент в [Component Diagram](component-diagram.puml)
2. Изучите интерфейсы в [Code Diagram](code-diagram.puml)
3. Проверьте взаимодействия в [Sequence Diagram](sequence-diagram.puml)
4. Реализуйте код согласно диаграммам

**Пример:**
```go
// Добавление нового типа метрик согласно Class Diagram
type SecurityMetrics struct {
    FailedAuthAttempts   int64     `json:"failed_auth_attempts"`
    SuspiciousActivities int64     `json:"suspicious_activities"`
    Timestamp           time.Time  `json:"timestamp"`
}

// Интеграция в MetricsSnapshot согласно диаграмме
type MetricsSnapshot struct {
    BitswapMetrics    *BitswapMetrics    `json:"bitswap"`
    BlockstoreMetrics *BlockstoreMetrics `json:"blockstore"`
    NetworkMetrics    *NetworkMetrics    `json:"network"`
    SecurityMetrics   *SecurityMetrics   `json:"security"` // Новое поле
    Timestamp         time.Time          `json:"timestamp"`
    NodeID            string             `json:"node_id"`
}
```

### Сценарий 3: Я делаю code review

**Цель:** Проверить соответствие кода архитектуре

**Действия:**
1. Сравните новые интерфейсы с [Code Diagram](code-diagram.puml)
2. Проверьте зависимости согласно [Component Diagram](component-diagram.puml)
3. Убедитесь, что последовательность вызовов соответствует [Sequence Diagram](sequence-diagram.puml)

**Чек-лист:**
- [ ] Новые интерфейсы соответствуют Code Diagram
- [ ] Зависимости не нарушают Component Diagram
- [ ] Обработка ошибок соответствует Sequence Diagram
- [ ] Состояния Auto Tuner учтены согласно State Diagram

### Сценарий 4: Я отлаживаю проблему

**Цель:** Понять поток данных и найти источник проблемы

**Действия:**
1. Изучите [Sequence Diagram](sequence-diagram.puml) для понимания потока
2. Проверьте состояния в [State Diagram](state-diagram.puml)
3. Добавьте логирование в ключевых точках согласно диаграммам

**Пример отладки:**
```go
func (w *MonitoringWorkflow) ExecuteMonitoringCycle(ctx context.Context) error {
    log.Debug("Step 1: Starting metrics collection") // Sequence Diagram точка 1
    
    metrics, err := w.monitor.CollectMetrics()
    if err != nil {
        log.Error("Failed at Sequence Diagram step 1", "error", err)
        return err
    }
    
    log.Debug("Step 2: Starting analysis") // Sequence Diagram точка 2
    // ... продолжение согласно диаграмме
}
```

## 🛠️ Инструменты и настройка

### Локальная генерация диаграмм

```bash
# Установка PlantUML
make install-plantuml

# Генерация PNG диаграмм
make png

# Валидация синтаксиса
make validate

# Запуск локального веб-сервера
make serve
```

### Интеграция с IDE

**VS Code:**
1. Установите PlantUML Extension
2. Откройте любой `.puml` файл
3. Нажмите `Alt+D` для предварительного просмотра

**IntelliJ IDEA:**
1. Установите PlantUML Integration Plugin
2. Откройте `.puml` файл
3. Используйте встроенный просмотр

### Валидация архитектуры

```bash
# Проверка соответствия кода диаграммам
make arch-validate

# Генерация отчета о нарушениях
make arch-report

# Автоматическая проверка при коммите
make setup-hooks
```

## 📚 Дальнейшее изучение

### Обязательно к прочтению:
1. **[diagrams-explanation.md](diagrams-explanation.md)** - Подробное объяснение каждой диаграммы
2. **[practical-usage-guide.md](practical-usage-guide.md)** - Практические примеры использования

### Для продвинутого использования:
3. **[tooling-integration.md](tooling-integration.md)** - Интеграция с инструментами разработки
4. **[README.md](README.md)** - Полная документация архитектуры

## ❓ Часто задаваемые вопросы

### Q: Как понять, какую диаграмму использовать?

**A:** Используйте эту схему:
- **Общее понимание системы** → Context Diagram
- **Архитектура сервисов** → Container Diagram  
- **Внутренняя структура** → Component Diagram
- **Интерфейсы и классы** → Code Diagram
- **Поток выполнения** → Sequence Diagram
- **Жизненный цикл** → State Diagram
- **Развертывание** → Deployment Diagram
- **Структуры данных** → Class Diagram

### Q: Что делать, если код не соответствует диаграмме?

**A:** Есть два варианта:
1. **Обновить код** согласно диаграмме (если диаграмма правильная)
2. **Обновить диаграмму** согласно коду (если код правильный)

Главное - поддерживать синхронизацию!

### Q: Как часто нужно обновлять диаграммы?

**A:** 
- **При изменении архитектуры** - обязательно
- **При добавлении новых компонентов** - обязательно  
- **При рефакторинге** - желательно
- **Регулярный аудит** - раз в месяц

### Q: Можно ли автоматизировать проверку соответствия?

**A:** Да! Используйте:
- Pre-commit hooks для проверки при коммите
- CI/CD pipeline для проверки при PR
- IDE плагины для проверки в реальном времени
- Архитектурные тесты в test suite

## 🎉 Готово!

Теперь вы готовы эффективно использовать архитектурные диаграммы Task 5 в своей работе. 

**Следующие шаги:**
1. Изучите диаграммы для вашего текущего задания
2. Настройте инструменты в своей IDE
3. Интегрируйте валидацию в свой рабочий процесс
4. Поделитесь знаниями с командой

**Помните:** Диаграммы - это живой инструмент разработки, а не статическая документация. Используйте их активно!