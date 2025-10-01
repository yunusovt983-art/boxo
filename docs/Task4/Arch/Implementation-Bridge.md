# Task 4: Мост между архитектурой и реализацией

Этот документ показывает, как архитектурные диаграммы Task 4 напрямую соответствуют реальному коду и могут использоваться разработчиками.

## 🎯 Как использовать диаграммы для разработки

### 1. От диаграммы к коду

**Шаг 1**: Изучите контекстную диаграмму для понимания общей картины
**Шаг 2**: Используйте диаграммы компонентов для определения интерфейсов
**Шаг 3**: Следуйте sequence диаграммам для реализации логики
**Шаг 4**: Применяйте deployment диаграмму для конфигурации

### 2. Практические примеры

#### Пример 1: Реализация адаптивного менеджера соединений

**Из диаграммы c4-level3-component-task41.puml:**
```plantuml
Component(conn_manager, "AdaptiveConnManager", "Go struct", "Главный менеджер с динамическими лимитами")
Component(load_monitor, "LoadMonitor", "Go struct", "Мониторинг системной нагрузки")
```

**Реализация в коде:**
```go
// bitswap/network/adaptive_conn_manager.go
type AdaptiveConnManager struct {
    config      *Config
    loadMonitor *LoadMonitor  // Соответствует диаграмме
    host        host.Host
    
    // Динамические лимиты из диаграммы
    currentHighWater int
    currentLowWater  int
    currentGracePeriod time.Duration
}

func NewAdaptiveConnManager(config *Config, host host.Host) *AdaptiveConnManager {
    return &AdaptiveConnManager{
        config:      config,
        loadMonitor: NewLoadMonitor(), // Создание компонента из диаграммы
        host:        host,
        currentHighWater: config.BaseHighWater,
        currentLowWater:  config.BaseLowWater,
    }
}
```

#### Пример 2: Sequence диаграмма → Реализация цикла адаптации

**Из sequence-adaptive-connection.puml:**
```plantuml
loop Каждые 30 секунд
    ACM -> LM: GetCurrentLoad()
    LM -> ACM: LoadMetrics(load=0.85)
    alt Load > HighThreshold (0.8)
        ACM -> ACM: AdaptToHighLoad()
```

**Реализация:**
```go
func (acm *AdaptiveConnManager) Start() error {
    // Запуск цикла адаптации согласно sequence диаграмме
    go acm.adaptationLoop()
    return nil
}

func (acm *AdaptiveConnManager) adaptationLoop() {
    ticker := time.NewTicker(30 * time.Second) // Из диаграммы: "Каждые 30 секунд"
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            // Следуем sequence диаграмме
            load := acm.loadMonitor.GetCurrentLoad() // ACM -> LM: GetCurrentLoad()
            
            if load > acm.config.HighLoadThreshold { // alt Load > HighThreshold (0.8)
                acm.adaptToHighLoad() // ACM -> ACM: AdaptToHighLoad()
            } else if load < acm.config.LowLoadThreshold {
                acm.adaptToLowLoad()
            }
        case <-acm.stopChan:
            return
        }
    }
}
```

### 3. Использование диаграмм для тестирования

#### Тестирование по component диаграмме

**Из c4-level3-component-task42.puml:**
```plantuml
Component(quality_scorer, "QualityScorer", "Go struct", "Алгоритм комплексной оценки качества")
note right of quality_scorer
  Алгоритм оценки:
  • Латентность: 40% веса
  • Пропускная способность: 30% веса  
  • Частота ошибок: 30% веса
end note
```

**Тест на основе диаграммы:**
```go
func TestQualityScorer_CalculateScore(t *testing.T) {
    // Тест основан на весах из диаграммы
    scorer := NewQualityScorer()
    
    tests := []struct {
        name      string
        latency   time.Duration
        bandwidth float64
        errorRate float64
        expected  float64
    }{
        {
            name:      "perfect connection",
            latency:   10 * time.Millisecond,  // Отличная латентность
            bandwidth: 10.0,                   // 10 MB/s - максимум
            errorRate: 0.0,                    // Нет ошибок
            expected:  1.0,                    // Идеальная оценка
        },
        {
            name:      "poor connection", 
            latency:   200 * time.Millisecond, // Плохая латентность
            bandwidth: 0.1,                    // Низкая пропускная способность
            errorRate: 0.1,                    // 10% ошибок
            expected:  0.0,                    // Худшая оценка
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            score := scorer.CalculateQualityScore(tt.latency, tt.bandwidth, tt.errorRate)
            assert.InDelta(t, tt.expected, score, 0.1)
        })
    }
}
```

### 4. Конфигурация на основе deployment диаграммы

**Из deployment-diagram.puml:**
```plantuml
Container(boxo1, "Boxo с оптимизацией", "Go application", "Адаптивное управление соединениями")
note right of boxo1
  Оптимизации Task 4:
  • Адаптивные лимиты соединений
  • Мониторинг качества соединений
end note
```

**Docker конфигурация:**
```dockerfile
# Dockerfile основанный на deployment диаграмме
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .

# Сборка с оптимизациями Task 4
RUN go build -tags="task4_optimizations" -o boxo-optimized ./cmd/boxo

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/boxo-optimized .

# Порты из deployment диаграммы
EXPOSE 4001 8080 9090

# Переменные среды для Task 4 оптимизаций
ENV BOXO_ADAPTIVE_CONN_ENABLED=true
ENV BOXO_HIGH_LOAD_THRESHOLD=0.8
ENV BOXO_LOW_LOAD_THRESHOLD=0.3
ENV BOXO_BUFFER_ADAPTATION_ENABLED=true
ENV BOXO_KEEPALIVE_ENABLED=true

CMD ["./boxo-optimized"]
```

**docker-compose.yml для кластера:**
```yaml
version: '3.8'
services:
  # Конфигурация основана на deployment диаграмме
  boxo-node-1:
    build: .
    environment:
      - NODE_ID=node-1
      - CLUSTER_PEERS=node-2,node-3  # Кластерные соединения из диаграммы
      - BOXO_ADAPTIVE_CONN_ENABLED=true
      - BOXO_MAX_CONNECTIONS=3000
      - BOXO_GRACE_PERIOD=30s
    ports:
      - "4001:4001"  # libp2p из диаграммы
      - "8080:8080"  # HTTP API
      - "9090:9090"  # Prometheus metrics
    networks:
      - ipfs-cluster

  boxo-node-2:
    build: .
    environment:
      - NODE_ID=node-2
      - CLUSTER_PEERS=node-1,node-3
      - BOXO_ADAPTIVE_CONN_ENABLED=true
    ports:
      - "4002:4001"
      - "8081:8080"
      - "9091:9090"
    networks:
      - ipfs-cluster

  # Мониторинг из deployment диаграммы
  prometheus:
    image: prom/prometheus
    ports:
      - "9092:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    networks:
      - ipfs-cluster

networks:
  ipfs-cluster:
    driver: bridge
```

## 🔧 Практические инструменты

### 1. Генерация кода из диаграмм

**Скрипт для создания заглушек интерфейсов:**
```bash
#!/bin/bash
# generate_interfaces.sh - Создание Go интерфейсов из component диаграмм

echo "Generating interfaces from Task 4 component diagrams..."

# Из c4-level3-component-task41.puml
cat > bitswap/network/interfaces.go << 'EOF'
package network

import (
    "context"
    "time"
    "github.com/libp2p/go-libp2p/core/peer"
)

// Интерфейсы основаны на component диаграммах Task 4

// AdaptiveConnectionManager - из c4-level3-component-task41.puml
type AdaptiveConnectionManager interface {
    SetDynamicLimits(high, low int, grace time.Duration) error
    AddClusterPeer(peer.ID) error
    GetCurrentLoad() float64
    Start() error
    Stop() error
}

// QualityMonitor - из c4-level3-component-task42.puml  
type QualityMonitor interface {
    GetConnectionQuality(peer.ID) (*ConnectionQuality, error)
    StartMonitoring(peer.ID) error
    FindAlternativeRoutes(peer.ID) ([]peer.ID, error)
}

// BufferKeepAliveManager - из c4-level3-component-task43.puml
type BufferKeepAliveManager interface {
    OptimizeBuffers(peer.ID) error
    EnableKeepAlive(peer.ID, bool) error
    DetectSlowConnections() []peer.ID
}
EOF

echo "Interfaces generated in bitswap/network/interfaces.go"
```

### 2. Валидация реализации против диаграмм

**Тест соответствия архитектуре:**
```go
func TestArchitectureCompliance(t *testing.T) {
    // Проверка что реализация соответствует component диаграммам
    
    t.Run("AdaptiveConnManager components", func(t *testing.T) {
        acm := NewAdaptiveConnManager(testConfig, testHost)
        
        // Проверка наличия компонентов из диаграммы
        assert.NotNil(t, acm.loadMonitor, "LoadMonitor component missing")
        assert.NotNil(t, acm.clusterPool, "ClusterConnectionPool component missing")
        assert.NotNil(t, acm.connectionTracker, "ConnectionTracker component missing")
        assert.NotNil(t, acm.adaptationEngine, "AdaptationEngine component missing")
    })
    
    t.Run("QualityMonitor components", func(t *testing.T) {
        qm := NewConnectionQualityMonitor(testConfig)
        
        // Компоненты из c4-level3-component-task42.puml
        assert.NotNil(t, qm.qualityTracker, "ConnectionQualityTracker missing")
        assert.NotNil(t, qm.latencyMonitor, "LatencyMonitor missing")
        assert.NotNil(t, qm.routeOptimizer, "RouteOptimizer missing")
        assert.NotNil(t, qm.qualityScorer, "QualityScorer missing")
    })
}
```

### 3. Мониторинг соответствия архитектуре

**Prometheus метрики для валидации архитектуры:**
```go
// Метрики основанные на архитектурных диаграммах
var (
    // Из architecture-overview.puml - производительные метрики
    adaptationOpsPerSecond = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "boxo_adaptation_ops_per_second",
        Help: "Adaptation operations per second (target: 1M ops/sec)",
    })
    
    qualityOpsPerSecond = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "boxo_quality_ops_per_second", 
        Help: "Quality calculation operations per second (target: 5M ops/sec)",
    })
    
    maxConnections = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "boxo_max_connections_supported",
        Help: "Maximum connections supported (target: 10K)",
    })
    
    // Из component диаграмм - компонентные метрики
    componentHealth = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "boxo_component_health",
            Help: "Health status of architectural components (1=healthy, 0=unhealthy)",
        },
        []string{"component", "task"},
    )
)

func RecordArchitecturalMetrics() {
    // Task 4.1 компоненты
    componentHealth.WithLabelValues("AdaptiveConnManager", "4.1").Set(1)
    componentHealth.WithLabelValues("LoadMonitor", "4.1").Set(1)
    componentHealth.WithLabelValues("ClusterConnectionPool", "4.1").Set(1)
    
    // Task 4.2 компоненты  
    componentHealth.WithLabelValues("ConnectionQualityTracker", "4.2").Set(1)
    componentHealth.WithLabelValues("LatencyMonitor", "4.2").Set(1)
    componentHealth.WithLabelValues("RouteOptimizer", "4.2").Set(1)
    
    // Task 4.3 компоненты
    componentHealth.WithLabelValues("BufferKeepAliveManager", "4.3").Set(1)
    componentHealth.WithLabelValues("AdaptiveBufferSizer", "4.3").Set(1)
    componentHealth.WithLabelValues("SlowConnectionDetector", "4.3").Set(1)
}
```

## 📚 Заключение

Диаграммы Task 4 служат живой документацией, которая:

1. **Направляет разработку**: Четкие интерфейсы и компоненты
2. **Упрощает тестирование**: Понятные границы и ответственности  
3. **Облегчает развертывание**: Готовые конфигурации и топологии
4. **Обеспечивает соответствие**: Валидация реализации против архитектуры

Используйте эти диаграммы как основу для разработки, тестирования и развертывания Task 4 компонентов.