# Troubleshooting Guide - Руководство по устранению неполадок AutoTuner

## 🔍 Общий подход к диагностике

### Пошаговая диагностика

1. **Проверка статуса системы**
2. **Анализ логов**
3. **Проверка метрик**
4. **Валидация конфигурации**
5. **Тестирование компонентов**
6. **Анализ производительности**

### Инструменты диагностики

```bash
# Проверка статуса AutoTuner
autotuner status

# Проверка здоровья системы
autotuner health-check

# Просмотр логов
autotuner logs --tail 100 --level error

# Проверка метрик
autotuner metrics --component ml

# Валидация конфигурации
autotuner config validate

# Диагностика ML модели
autotuner ml diagnose

# Тест безопасности
autotuner safety test
```

## 🚨 Распространенные проблемы

### 1. AutoTuner не запускается

#### Симптомы
- Процесс завершается сразу после запуска
- Ошибки в логах при инициализации
- Порты недоступны

#### Возможные причины и решения

**Проблема**: Неверная конфигурация
```bash
# Проверка конфигурации
autotuner config validate --file /etc/autotuner/config.yml

# Типичные ошибки:
# - Неверный формат YAML
# - Отсутствующие обязательные поля
# - Неверные типы данных
```

**Решение**:
```yaml
# Исправление типичных ошибок конфигурации
autotuner:
  enabled: true  # должно быть boolean, не строка
  tuning_interval: "15m"  # должно быть в кавычках
  ml:
    min_confidence: 0.8  # должно быть число, не строка
```

**Проблема**: Порт уже используется
```bash
# Проверка занятых портов
netstat -tulpn | grep :8080
lsof -i :8080

# Изменение порта в конфигурации
autotuner:
  server:
    port: 8081  # Использовать другой порт
```

**Проблема**: Недостаточно прав доступа
```bash
# Проверка прав на файлы
ls -la /etc/autotuner/
ls -la /var/lib/autotuner/

# Исправление прав
sudo chown -R autotuner:autotuner /var/lib/autotuner/
sudo chmod 755 /var/lib/autotuner/
sudo chmod 644 /etc/autotuner/config.yml
```

**Проблема**: Недоступна база данных
```bash
# Проверка подключения к PostgreSQL
psql -h localhost -U autotuner -d autotuner -c "SELECT 1;"

# Проверка SQLite файла
ls -la /var/lib/autotuner/autotuner.db
sqlite3 /var/lib/autotuner/autotuner.db ".tables"
```

### 2. ML модель не обучается

#### Симптомы
- Ошибки "insufficient training data"
- Низкая точность модели
- Модель не обновляется

#### Диагностика и решения

**Проблема**: Недостаточно исторических данных
```bash
# Проверка количества данных
autotuner ml data-stats

# Ожидаемый вывод:
# Training samples: 1000+ (минимум)
# Time range: 24h+ (минимум)
# Features: 15+ (рекомендуется)
```

**Решение**:
```yaml
# Уменьшение требований к данным для начального обучения
ml:
  training:
    min_samples: 100  # Уменьшить с 1000
    min_time_range: "6h"  # Уменьшить с 24h
```

**Проблема**: Плохое качество данных
```bash
# Анализ качества данных
autotuner ml data-quality

# Проверка на:
# - Пропущенные значения
# - Выбросы
# - Корреляции между признаками
```

**Решение**:
```go
// Настройка предобработки данных
type DataPreprocessor struct {
    fillMissingValues bool
    removeOutliers    bool
    normalizeFeatures bool
}

// В конфигурации:
ml:
  preprocessing:
    fill_missing_values: true
    remove_outliers: true
    normalize_features: true
    outlier_threshold: 3.0  # Z-score
```

**Проблема**: Переобучение модели
```bash
# Проверка метрик переобучения
autotuner ml model-stats

# Признаки переобучения:
# - Training accuracy > 0.95, Validation accuracy < 0.8
# - Большая разница между train/validation loss
```

**Решение**:
```yaml
ml:
  random_forest:
    max_depth: 5  # Уменьшить с 10
    min_samples_split: 10  # Увеличить с 5
    min_samples_leaf: 5    # Увеличить с 2
  training:
    early_stopping_patience: 5  # Раннее остановка
    regularization: 0.01  # L2 регуляризация
```

### 3. Canary deployment не работает

#### Симптомы
- Canary развертывание зависает
- Все узлы получают изменения сразу
- Откат не срабатывает

#### Диагностика

```bash
# Проверка статуса canary
autotuner canary status

# Проверка health checks
autotuner health-check --nodes canary

# Просмотр логов canary
autotuner logs --component canary --level debug
```

**Проблема**: Health check не настроен
```yaml
# Настройка health checks
safety:
  health_checks:
    - name: "latency_check"
      type: "performance"
      threshold: 100  # ms
      weight: 0.4
    - name: "error_rate_check"
      type: "error_rate"
      threshold: 0.05  # 5%
      weight: 0.6
```

**Проблема**: Неправильный выбор узлов
```bash
# Проверка доступных узлов
autotuner nodes list

# Проверка селектора узлов
autotuner nodes select --percentage 10 --dry-run
```

**Решение**:
```yaml
safety:
  canary:
    node_selector:
      strategy: "random"  # random, hash, label
      exclude_labels:
        - "critical=true"
        - "maintenance=true"
```

### 4. Высокая частота откатов

#### Симптомы
- Rollback rate > 20%
- Частые алерты о откатах
- Система не улучшается

#### Анализ причин

```bash
# Анализ истории откатов
autotuner rollback history --last 24h

# Статистика откатов
autotuner rollback stats

# Анализ причин откатов
autotuner rollback analyze --group-by reason
```

**Проблема**: Слишком агрессивная оптимизация
```yaml
# Более консервативные настройки
optimization:
  genetic:
    mutation_rate: 0.05  # Уменьшить с 0.1
    population_size: 100  # Увеличить с 50
  
ml:
  min_confidence: 0.9  # Увеличить с 0.8

safety:
  canary:
    success_threshold: 0.95  # Увеличить с 0.9
    step_duration: "10m"     # Увеличить с 5m
```

**Проблема**: Неподходящие целевые функции
```yaml
# Пересмотр весов целевых функций
optimization:
  objectives:
    - name: "stability"  # Добавить стабильность
      weight: 0.3
      target: "maximize"
    - name: "latency"
      weight: 0.3        # Уменьшить с 0.4
      target: "minimize"
```

### 5. Низкая производительность ML

#### Симптомы
- Медленные предсказания (> 5 секунд)
- Высокое использование CPU/памяти
- Таймауты ML операций

#### Оптимизация производительности

**Проблема**: Большая модель
```bash
# Проверка размера модели
autotuner ml model-info

# Размер модели > 100MB может быть проблемой
```

**Решение**:
```yaml
ml:
  random_forest:
    n_estimators: 50  # Уменьшить с 100
    max_depth: 8      # Уменьшить с 10
  
  # Включить сжатие модели
  model_compression:
    enabled: true
    algorithm: "gzip"
    level: 6
```

**Проблема**: Слишком много признаков
```bash
# Анализ важности признаков
autotuner ml feature-importance

# Удаление неважных признаков
autotuner ml feature-selection --threshold 0.01
```

**Решение**:
```yaml
ml:
  features:
    max_features: 20  # Ограничить количество
    selection_method: "importance"  # Автоматический отбор
    importance_threshold: 0.01
```

### 6. Проблемы с мониторингом

#### Симптомы
- Метрики не обновляются
- Grafana показывает "No data"
- Алерты не срабатывают

#### Диагностика мониторинга

```bash
# Проверка Prometheus endpoint
curl http://localhost:9090/metrics | grep autotuner

# Проверка подключения к Prometheus
autotuner metrics test-connection

# Проверка алертов
autotuner alerts test --rule high_failure_rate
```

**Проблема**: Prometheus не собирает метрики
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'autotuner'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 30s
    metrics_path: /metrics
```

**Проблема**: Неправильные labels в метриках
```go
// Проверка labels в коде
autotunerSuccess.WithLabelValues(
    "prediction",  // operation
    "success",     // result
).Inc()

// Убедиться что labels соответствуют определению метрики
```

## 🔧 Диагностические команды

### Системная диагностика

```bash
# Полная диагностика системы
autotuner diagnose --full

# Проверка конкретного компонента
autotuner diagnose --component ml
autotuner diagnose --component safety
autotuner diagnose --component optimization

# Экспорт диагностической информации
autotuner diagnose --export /tmp/autotuner-diag.tar.gz
```

### Тестирование компонентов

```bash
# Тест ML модели
autotuner test ml --sample-data /path/to/test-data.json

# Тест safety механизмов
autotuner test safety --simulate-failure

# Тест оптимизации
autotuner test optimization --dry-run

# Интеграционный тест
autotuner test integration --duration 5m
```

### Профилирование производительности

```bash
# CPU профилирование
autotuner profile cpu --duration 30s --output cpu.prof

# Memory профилирование
autotuner profile memory --output memory.prof

# Анализ профилей
go tool pprof cpu.prof
go tool pprof memory.prof
```

## 📊 Мониторинг здоровья системы

### Ключевые метрики для мониторинга

```yaml
# Алерты для мониторинга здоровья
alerts:
  - name: "autotuner_down"
    condition: "up == 0"
    for: "1m"
    severity: "critical"
    
  - name: "high_ml_prediction_latency"
    condition: "autotuner_ml_prediction_duration_seconds > 5"
    for: "2m"
    severity: "warning"
    
  - name: "high_rollback_rate"
    condition: "rate(autotuner_rollbacks_total[5m]) > 0.1"
    for: "1m"
    severity: "error"
    
  - name: "low_ml_accuracy"
    condition: "autotuner_ml_model_accuracy < 0.7"
    for: "5m"
    severity: "warning"
    
  - name: "config_validation_errors"
    condition: "rate(autotuner_config_validation_errors_total[5m]) > 0"
    for: "1m"
    severity: "error"
```

### Health Check endpoint

```bash
# Проверка здоровья через HTTP
curl http://localhost:8080/health

# Ожидаемый ответ:
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "components": {
    "ml_model": "healthy",
    "database": "healthy",
    "safety_system": "healthy"
  },
  "metrics": {
    "uptime": "24h30m",
    "last_successful_tuning": "2024-01-15T10:15:00Z",
    "ml_model_accuracy": 0.87
  }
}
```

## 🆘 Экстренные процедуры

### Полное отключение AutoTuner

```bash
# Экстренная остановка
autotuner emergency-stop

# Отключение через конфигурацию
autotuner config set enabled false

# Остановка через systemd
sudo systemctl stop autotuner
sudo systemctl disable autotuner
```

### Откат к последней рабочей конфигурации

```bash
# Просмотр доступных backup'ов
autotuner backup list

# Откат к конкретному backup'у
autotuner rollback --backup-id backup-20240115-103000

# Откат к последнему рабочему состоянию
autotuner rollback --last-good
```

### Сброс ML модели

```bash
# Удаление текущей модели
autotuner ml reset-model

# Переобучение с нуля
autotuner ml retrain --force

# Загрузка backup модели
autotuner ml restore-model --backup-id model-backup-20240115
```

## 📝 Сбор информации для поддержки

### Создание отчета о проблеме

```bash
# Создание полного отчета
autotuner support-bundle --output /tmp/autotuner-support.tar.gz

# Отчет включает:
# - Логи за последние 24 часа
# - Текущую конфигурацию
# - Метрики системы
# - Состояние ML модели
# - История изменений
# - Системную информацию
```

### Включение debug логирования

```yaml
# Временное включение debug режима
autotuner:
  logging:
    level: "debug"
    components:
      ml: "trace"
      safety: "debug"
      optimization: "debug"
```

```bash
# Через CLI
autotuner config set logging.level debug
autotuner restart
```

### Экспорт метрик для анализа

```bash
# Экспорт метрик в CSV
autotuner metrics export --format csv --duration 24h --output metrics.csv

# Экспорт в JSON для анализа
autotuner metrics export --format json --duration 7d --output metrics.json
```

## 🔄 Восстановление после сбоев

### Восстановление базы данных

```bash
# Проверка целостности SQLite
sqlite3 /var/lib/autotuner/autotuner.db "PRAGMA integrity_check;"

# Восстановление из backup'а
autotuner db restore --backup /var/backups/autotuner-db-20240115.sql

# Пересоздание схемы
autotuner db migrate --force
```

### Восстановление ML модели

```bash
# Проверка модели
autotuner ml validate-model

# Переобучение при повреждении
autotuner ml retrain --emergency

# Использование fallback модели
autotuner ml use-fallback
```

Это руководство поможет быстро диагностировать и решить большинство проблем с AutoTuner.