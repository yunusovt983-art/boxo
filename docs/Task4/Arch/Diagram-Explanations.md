# Task 4: Подробные объяснения PlantUML диаграмм

Этот документ содержит детальные объяснения каждой PlantUML диаграммы в папке Task4/Arch, служащие мостом между архитектурным дизайном и фактической реализацией кода.

## 📋 Обзор диаграмм

### 1. [architecture-overview.puml](#1-architecture-overviewpuml)
### 2. [c4-level1-context.puml](#2-c4-level1-contextpuml)  
### 3. [c4-level2-container.puml](#3-c4-level2-containerpuml)
### 4. [c4-level3-component-task41.puml](#4-c4-level3-component-task41puml)
### 5. [c4-level3-component-task42.puml](#5-c4-level3-component-task42puml)
### 6. [c4-level3-component-task43.puml](#6-c4-level3-component-task43puml)
### 7. [c4-level4-code-data-flow.puml](#7-c4-level4-code-data-flowpuml)
### 8. [deployment-diagram.puml](#8-deployment-diagrampuml)
### 9. [sequence-adaptive-connection.puml](#9-sequence-adaptive-connectionpuml)
### 10. [sequence-buffer-keepalive.puml](#10-sequence-buffer-keepalivepuml)

---

## 1. architecture-overview.puml

### 🎯 Назначение
Высокоуровневый обзор всей архитектуры Task 4 с указанием размеров кода и ключевых метрик производительности.

### 🔗 Связь с кодом
```go
// Соответствует структуре пакетов:
bitswap/network/adaptive_conn_manager.go     // 1,200+ строк
bitswap/network/connection_quality_monitor.go // 400+ строк  
bitswap/network/buffer_keepalive_manager.go  // 1,200+ строк
```###
 🏗️ Архитектурные компоненты

**Task 4.1 - Адаптивный менеджер соединений:**
- `AdaptiveConnManager`: Главный компонент с динамическими лимитами
- `LoadMonitor`: Отслеживание системной нагрузки
- `ClusterConnectionPool`: Управление кластерными соединениями

**Task 4.2 - Мониторинг качества:**
- `ConnectionQuality`: Структура метрик соединений
- `LatencyMonitor`: Измерение латентности через libp2p ping
- `RouteOptimizer`: Поиск альтернативных маршрутов

**Task 4.3 - Буферизация и Keep-Alive:**
- `BufferKeepAliveManager`: Центральный менеджер оптимизации
- `BandwidthMonitor`: Мониторинг пропускной способности
- `SlowConnectionDetector`: Обнаружение медленных соединений

### 📊 Производительные метрики
- **Адаптация**: ~1M ops/sec для обновления лимитов
- **Качество**: ~5M ops/sec для расчета метрик
- **Масштабируемость**: до 10K одновременных соединений

---

## 2. c4-level1-context.puml

### 🎯 Назначение
Контекстная диаграмма показывает Task 4 в экосистеме IPFS кластера с внешними системами и пользователями.

### 🔗 Связь с кодом
```go
// Интерфейсы для внешних систем:
type NetworkOptimizer interface {
    OptimizeConnections() error
    GetPerformanceMetrics() Metrics
    ConfigureAdaptiveSettings(config Config) error
}

// Реализация в:
bitswap/network/network_optimizer.go
```

### 👥 Участники системы

**Администратор кластера:**
- Мониторит производительность через Prometheus/Grafana
- Настраивает пороги адаптации и лимиты соединений
- Анализирует метрики качества соединений

**Разработчик приложения:**
- Использует IPFS API для обмена данными
- Получает преимущества от автоматической оптимизации
- Настраивает приоритеты для критических операций

**DevOps инженер:**
- Конфигурирует сетевые параметры кластера
- Настраивает мониторинг и алерты
- Оптимизирует развертывание для высоких нагрузок

### 🌐 Внешние системы

**Внешние IPFS узлы:**
- Источник и получатель данных через Bitswap протокол
- Различное качество соединений требует адаптации
- Автоматическое переключение на лучшие маршруты

**Система мониторинга:**
- Prometheus для сбора метрик производительности
- Grafana для визуализации и дашбордов
- AlertManager для уведомлений о проблемах

---

## 3. c4-level2-container.puml

### 🎯 Назначение
Диаграмма контейнеров детализирует основные компоненты Task 4 и их взаимодействия.

### 🔗 Связь с кодом
```go
// Основные интерфейсы взаимодействия:
type AdaptiveConnectionManager interface {
    SetDynamicLimits(high, low int, grace time.Duration) error
    GetConnectionQuality(peer.ID) (*ConnectionQuality, error)
    AddClusterPeer(peer.ID) error
}

type QualityMonitor interface {
    GetConnectionQuality(peer.ID) (*ConnectionQuality, error)
    StartMonitoring(peer.ID) error
    FindAlternativeRoutes(peer.ID) ([]peer.ID, error)
}

type BufferKeepAliveManager interface {
    OptimizeBuffers(peer.ID) error
    EnableKeepAlive(peer.ID, bool) error
    DetectSlowConnections() []peer.ID
}
```

### 🔄 Взаимодействия компонентов

**Адаптивный менеджер соединений ↔ Монитор качества:**
```go
// Получение метрик качества для адаптации лимитов
quality, err := qualityMonitor.GetConnectionQuality(peerID)
if quality.Score < 0.3 {
    connManager.ReduceLimitsForPeer(peerID)
}
```

**Монитор качества ↔ Менеджер буферизации:**
```go
// Передача данных о производительности для оптимизации буферов
bufferManager.UpdateQuality(peerID, quality.Latency, quality.Bandwidth)
```

**Все компоненты → Сборщик метрик:**
```go
// Экспорт метрик в Prometheus формате
metrics.RecordConnectionCount(activeConnections)
metrics.RecordQualityScore(peerID, quality.Score)
metrics.RecordBufferSize(peerID, bufferSize)
```

---

## 4. c4-level3-component-task41.puml

### 🎯 Назначение
Детальная архитектура Task 4.1 - Адаптивного менеджера соединений с компонентами и их ответственностями.

### 🔗 Связь с кодом
```go
// Главная структура AdaptiveConnManager:
type AdaptiveConnManager struct {
    config           *Config
    loadMonitor      *LoadMonitor
    clusterPool      *ClusterConnectionPool
    connectionTracker *ConnectionTracker
    adaptationEngine *AdaptationEngine
    configAdapter    *ConfigurationAdapter
    metricsReporter  *MetricsReporter
    
    // libp2p интеграция
    host             host.Host
    connManager      connmgr.ConnManager
}
```

### 🧩 Компоненты и их реализация

**AdaptiveConnManager** (`adaptive_conn_manager.go:50-150`):
```go
func NewAdaptiveConnManager(config *Config, host host.Host) *AdaptiveConnManager {
    acm := &AdaptiveConnManager{
        config:      config,
        host:        host,
        loadMonitor: NewLoadMonitor(),
        // ... инициализация других компонентов
    }
    return acm
}

func (acm *AdaptiveConnManager) SetDynamicLimits(high, low int, grace time.Duration) error {
    return acm.configAdapter.ApplyLimits(high, low, grace)
}
```

**LoadMonitor** (`load_monitor.go:30-120`):
```go
type LoadMonitor struct {
    activeConnections int32
    requestsPerSecond float64
    averageLatency    time.Duration
    errorRate         float64
}

func (lm *LoadMonitor) GetCurrentLoad() float64 {
    // Расчет нагрузки на основе метрик
    connLoad := float64(lm.activeConnections) / 2000.0
    latencyLoad := float64(lm.averageLatency) / float64(100*time.Millisecond)
    return (connLoad + latencyLoad) / 2.0
}
```

**ClusterConnectionPool** (`cluster_pool.go:25-80`):
```go
type ClusterConnectionPool struct {
    clusterPeers map[peer.ID]bool
    host         host.Host
}

func (ccp *ClusterConnectionPool) AddClusterPeer(peerID peer.ID) error {
    ccp.clusterPeers[peerID] = true
    // Защита соединения от отключения
    ccp.host.ConnManager().Protect(peerID, "cluster-peer")
    return nil
}
```

### 🔄 Алгоритмы адаптации
```go
func (ae *AdaptationEngine) AdaptToLoad(load float64) {
    if load > ae.config.HighLoadThreshold {
        // Увеличить лимиты при высокой нагрузке
        newHigh := int(float64(ae.currentHighWater) * 1.5)
        newLow := int(float64(ae.currentLowWater) * 1.2)
        ae.configAdapter.SetLimits(newHigh, newLow)
    } else if load < ae.config.LowLoadThreshold {
        // Уменьшить лимиты при низкой нагрузке
        newHigh := int(float64(ae.currentHighWater) * 0.8)
        newLow := int(float64(ae.currentLowWater) * 0.9)
        ae.configAdapter.SetLimits(newHigh, newLow)
    }
}
```

---

## 5. c4-level3-component-task42.puml

### 🎯 Назначение
Архитектура Task 4.2 - Системы мониторинга качества соединений с алгоритмами оценки и оптимизации маршрутов.

### 🔗 Связь с кодом
```go
// Структура ConnectionQuality:
type ConnectionQuality struct {
    PeerID           peer.ID
    Latency          time.Duration
    Bandwidth        float64
    ErrorRate        float64
    AvgLatency       time.Duration
    AvgBandwidth     float64
    AvgErrorRate     float64
    QualityScore     float64 // 0.0-1.0
    TotalRequests    int64
    SuccessfulRequests int64
    AlternativeRoutes []peer.ID
    PreferredRoute   peer.ID
}
```

### 🧩 Компоненты и алгоритмы

**ConnectionQualityTracker** (`connection_quality_monitor.go:80-200`):
```go
func (cqt *ConnectionQualityTracker) UpdateQuality(peerID peer.ID) {
    latency := cqt.latencyMonitor.GetLatencyStats(peerID)
    bandwidth := cqt.bandwidthMonitor.GetBandwidthStats(peerID)
    errorRate := cqt.errorTracker.GetErrorRate(peerID)
    
    quality := cqt.qualityScorer.CalculateQualityScore(latency, bandwidth, errorRate)
    cqt.connectionQualities[peerID] = quality
}
```

**QualityScorer** - Алгоритм оценки (`quality_scorer.go:40-90`):
```go
func (qs *QualityScorer) CalculateQualityScore(latency time.Duration, bandwidth, errorRate float64) float64 {
    // Нормализация метрик (0.0-1.0)
    latencyScore := 1.0 - math.Min(float64(latency)/float64(200*time.Millisecond), 1.0)
    bandwidthScore := math.Min(bandwidth/10.0, 1.0) // 10 MB/s = 1.0
    errorScore := 1.0 - math.Min(errorRate/0.1, 1.0) // 10% error = 0.0
    
    // Взвешенная оценка
    return latencyScore*0.4 + bandwidthScore*0.3 + errorScore*0.3
}
```

**LatencyMonitor** (`latency_monitor.go:30-100`):
```go
func (lm *LatencyMonitor) MeasureLatency(peerID peer.ID) (time.Duration, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // Использование libp2p ping сервиса
    result := <-ping.Ping(ctx, lm.host, peerID)
    if result.Error != nil {
        // Fallback: измерение через время соединения
        return lm.measureConnectionTime(peerID), nil
    }
    return result.RTT, nil
}
```

**RouteOptimizer** (`route_optimizer.go:50-150`):
```go
func (ro *RouteOptimizer) FindAlternativeRoutes(peerID peer.ID) ([]peer.ID, error) {
    currentQuality := ro.qualityTracker.GetConnectionQuality(peerID)
    if currentQuality.QualityScore > 0.3 {
        return nil, nil // Качество приемлемое
    }
    
    // Поиск альтернативных пиров с лучшим качеством
    alternatives := ro.findBetterPeers(currentQuality.QualityScore)
    return alternatives, nil
}

func (ro *RouteOptimizer) findBetterPeers(currentScore float64) []peer.ID {
    var betterPeers []peer.ID
    for peerID, quality := range ro.qualityTracker.GetAllQualities() {
        if quality.QualityScore > currentScore+0.1 { // Улучшение на 10%+
            betterPeers = append(betterPeers, peerID)
        }
    }
    return betterPeers
}
```

---

## 6. c4-level3-component-task43.puml

### 🎯 Назначение
Архитектура Task 4.3 - Системы буферизации и Keep-Alive с BDP алгоритмами и обнаружением медленных соединений.

### 🔗 Связь с кодом
```go
// Главная структура BufferKeepAliveManager:
type BufferKeepAliveManager struct {
    adaptiveBuffer    *AdaptiveBufferSizer
    bandwidthMonitor  *BandwidthMonitor
    keepaliveManager  *KeepAliveManager
    slowDetector      *SlowConnectionDetector
    connectionBuffers map[peer.ID]*ConnectionBuffer
    performanceBench  *PerformanceBenchmarker
}
```

### 🧩 Компоненты и алгоритмы

**AdaptiveBufferSizer** - BDP алгоритм (`adaptive_buffer.go:60-140`):
```go
func (abs *AdaptiveBufferSizer) AdaptBufferSize(peerID peer.ID, bandwidth float64, latency time.Duration) {
    bdp := bandwidth * float64(latency.Nanoseconds()) / 1e9 // bytes
    
    currentSize := abs.connectionBuffers[peerID].GetSize()
    
    if bandwidth < abs.config.LowBandwidthThreshold {
        // Сжатие буфера при низкой пропускной способности
        newSize := int(float64(currentSize) * 0.75)
        abs.setBufferSize(peerID, newSize)
    } else if bandwidth > abs.config.HighBandwidthThreshold {
        // Рост буфера при высокой пропускной способности
        newSize := int(float64(currentSize) * 1.5)
        abs.setBufferSize(peerID, newSize)
    } else {
        // Оптимальный размер на основе BDP
        abs.setBufferSize(peerID, int(bdp))
    }
}

func (abs *AdaptiveBufferSizer) setBufferSize(peerID peer.ID, size int) {
    // Применение лимитов: 4KB <= size <= 1MB
    size = max(4*1024, min(size, 1024*1024))
    abs.connectionBuffers[peerID].SetSize(size)
}
```

**KeepAliveManager** - Интеллектуальное управление (`keepalive_manager.go:80-180`):
```go
type KeepAliveState struct {
    Enabled           bool
    AdaptiveInterval  time.Duration
    ImportanceScore   float64 // 0.5-1.0
    ConsecutiveFailures int
    LastProbeTime     time.Time
    RecentActivity    time.Time
}

func (kam *KeepAliveManager) EnableKeepAlive(peerID peer.ID, isImportant bool) {
    state := &KeepAliveState{
        Enabled:          true,
        AdaptiveInterval: kam.config.BaseKeepAliveInterval,
        ImportanceScore:  0.5,
    }
    
    if isImportant {
        state.ImportanceScore = 1.0
        state.AdaptiveInterval = kam.config.BaseKeepAliveInterval / 2
    }
    
    kam.keepAliveStates[peerID] = state
    go kam.keepAliveLoop(peerID)
}

func (kam *KeepAliveManager) keepAliveLoop(peerID peer.ID) {
    for {
        state := kam.keepAliveStates[peerID]
        if !state.Enabled {
            return
        }
        
        time.Sleep(state.AdaptiveInterval)
        
        if kam.host.Network().Connectedness(peerID) == network.Connected {
            kam.recordActivity(peerID)
            kam.adaptInterval(peerID, true) // Увеличить интервал
        } else {
            kam.sendKeepAlive(peerID)
        }
    }
}
```

**SlowConnectionDetector** (`slow_connection_detector.go:40-120`):
```go
type SlowConnectionInfo struct {
    LatencySamples    []time.Duration
    ConfirmationCount int
    IsSlow           bool
    BypassRoutes     []peer.ID
}

func (scd *SlowConnectionDetector) RecordLatency(peerID peer.ID, latency time.Duration) {
    info := scd.getOrCreateInfo(peerID)
    
    // Добавление в циркулярный буфер
    if len(info.LatencySamples) >= scd.config.SlowConnectionSamples {
        info.LatencySamples = info.LatencySamples[1:]
    }
    info.LatencySamples = append(info.LatencySamples, latency)
    
    // Проверка на медленное соединение
    if len(info.LatencySamples) >= scd.config.SlowConnectionSamples {
        avgLatency := scd.calculateAverageLatency(info.LatencySamples)
        
        if avgLatency > scd.config.SlowConnectionThreshold {
            info.ConfirmationCount++
            if info.ConfirmationCount >= 3 && !info.IsSlow {
                scd.markAsSlow(peerID, info)
            }
        } else {
            info.ConfirmationCount = max(0, info.ConfirmationCount-1)
            if info.ConfirmationCount == 0 && info.IsSlow {
                scd.unmarkAsSlow(peerID, info)
            }
        }
    }
}
```

### 📊 Бенчмарки производительности
```go
// Бенчмарки в performance_benchmarker.go:
func BenchmarkBufferAdaptation(b *testing.B) {
    // Цель: 1M ops/sec для адаптации буферов
    for i := 0; i < b.N; i++ {
        bufferSizer.AdaptBufferSize(testPeerID, 1000000, 50*time.Millisecond)
    }
}

func BenchmarkKeepAliveManagement(b *testing.B) {
    // Тест масштабируемости: 100-10K пиров
    for peerCount := 100; peerCount <= 10000; peerCount *= 10 {
        b.Run(fmt.Sprintf("peers-%d", peerCount), func(b *testing.B) {
            // Тестирование с различным количеством пиров
        })
    }
}
```

---

## 7. c4-level4-code-data-flow.puml

### 🎯 Назначение
Детальный поток данных между компонентами Task 4 с конкретными вызовами методов и обработкой событий.

### 🔗 Связь с кодом
Диаграмма показывает реальную последовательность вызовов методов:

```go
// Установка соединения:
func (acm *AdaptiveConnManager) handleConnection(peerID peer.ID) {
    // 1. Создание информации о соединении
    cq.CreateConnectionInfo(peerID)
    
    // 2. Инициализация буферов
    bkm.InitializeBuffers(peerID)
    
    // 3. Запуск мониторинга
    bm.StartMonitoring(peerID)
    
    // 4. Включение keep-alive
    kam.EnableKeepAlive(peerID, acm.isClusterPeer(peerID))
}
```

### 🔄 Циклы мониторинга и адаптации

**Мониторинг качества (каждые 30 секунд):**
```go
func (cq *ConnectionQuality) monitoringLoop() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        for peerID := range cq.trackedPeers {
            latency := cq.measureLatency(peerID)
            bandwidth := cq.getBandwidth(peerID)
            quality := cq.calculateQualityScore(latency, bandwidth)
            
            if quality < 0.3 {
                alternatives := cq.findAlternativeRoutes(peerID)
                cq.setAlternativeRoutes(peerID, alternatives)
            }
        }
    }
}
```

**Адаптация к нагрузке (каждые 10 секунд):**
```go
func (acm *AdaptiveConnManager) adaptationLoop() {
    ticker := time.NewTicker(10 * time.Second)
    for range ticker.C {
        load := acm.loadMonitor.GetCurrentLoad()
        
        if load > acm.config.HighLoadThreshold {
            acm.increaseConnectionLimits()
            acm.decreaseGracePeriod()
        } else if load < acm.config.LowLoadThreshold {
            acm.decreaseConnectionLimits()
            acm.increaseGracePeriod()
        }
        
        acm.applyNewLimits()
    }
}
```

---

## 8. deployment-diagram.puml

### 🎯 Назначение
Диаграмма развертывания показывает, как Task 4 компоненты развертываются в реальной кластерной среде.

### 🔗 Связь с кодом
```yaml
# docker-compose.yml для кластера:
version: '3.8'
services:
  ipfs-node-1:
    image: boxo-optimized:latest
    environment:
      - BOXO_ADAPTIVE_CONN_ENABLED=true
      - BOXO_CLUSTER_PEERS=node-2,node-3
      - BOXO_HIGH_LOAD_THRESHOLD=0.8
    ports:
      - "4001:4001"  # libp2p
      - "8080:8080"  # HTTP API
      - "9090:9090"  # Metrics
```

### 🏗️ Конфигурация кластера

**Узел кластера:**
```go
// Конфигурация для кластерного развертывания:
type ClusterConfig struct {
    NodeID           string
    ClusterPeers     []peer.ID
    AdaptiveSettings AdaptiveConfig
    MetricsPort      int
    
    // Сетевые оптимизации
    MaxConnections   int    `default:"3000"`
    GracePeriod      time.Duration `default:"30s"`
    KeepAliveEnabled bool   `default:"true"`
}

func NewClusterNode(config ClusterConfig) *ClusterNode {
    node := &ClusterNode{
        adaptiveConnMgr: NewAdaptiveConnManager(config.AdaptiveSettings),
        qualityMonitor:  NewConnectionQualityMonitor(),
        bufferManager:   NewBufferKeepAliveManager(),
    }
    
    // Настройка кластерных соединений
    for _, peerID := range config.ClusterPeers {
        node.adaptiveConnMgr.AddClusterPeer(peerID)
    }
    
    return node
}
```

### 📊 Мониторинг в кластере

**Prometheus метрики:**
```go
// Метрики для кластерного мониторинга:
var (
    connectionCount = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "boxo_connections_total",
            Help: "Total number of connections",
        },
        []string{"node_id", "connection_type"},
    )
    
    connectionQuality = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "boxo_connection_quality_score",
            Help: "Connection quality score (0-1)",
        },
        []string{"node_id", "peer_id"},
    )
    
    bufferSize = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "boxo_buffer_size_bytes",
            Help: "Current buffer size in bytes",
        },
        []string{"node_id", "peer_id"},
    )
)
```

---

## 9. sequence-adaptive-connection.puml

### 🎯 Назначение
Последовательная диаграмма показывает временную последовательность адаптивного управления соединениями.

### 🔗 Связь с кодом
```go
// Обработчик событий соединения:
func (acm *AdaptiveConnManager) handleNetworkEvents() {
    acm.host.Network().Notify(&network.NotifyBundle{
        ConnectedF: func(n network.Network, conn network.Conn) {
            peerID := conn.RemotePeer()
            acm.onPeerConnected(peerID)
        },
        DisconnectedF: func(n network.Network, conn network.Conn) {
            peerID := conn.RemotePeer()
            acm.onPeerDisconnected(peerID)
        },
    })
}

func (acm *AdaptiveConnManager) onPeerConnected(peerID peer.ID) {
    // Создание информации о соединении
    acm.connectionTracker.TrackConnection(peerID)
    
    // Запись в мониторе нагрузки
    acm.loadMonitor.RecordConnection()
    
    // Инициализация буферов если это кластерный пир
    if acm.isClusterPeer(peerID) {
        acm.bufferManager.InitializeClusterBuffers(peerID)
    }
}
```

### ⏱️ Временные циклы

**Адаптация каждые 30 секунд:**
```go
func (acm *AdaptiveConnManager) startAdaptationLoop() {
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                load := acm.loadMonitor.GetCurrentLoad()
                acm.adaptToLoad(load)
            case <-acm.stopChan:
                return
            }
        }
    }()
}
```

**Восстановление кластерных соединений:**
```go
func (acm *AdaptiveConnManager) maintainClusterConnections() {
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()
        
        for range ticker.C {
            for peerID := range acm.clusterPeers {
                if acm.host.Network().Connectedness(peerID) != network.Connected {
                    acm.reconnectClusterPeer(peerID)
                }
            }
        }
    }()
}
```

---

## 10. sequence-buffer-keepalive.puml

### 🎯 Назначение
Детальная последовательность оптимизации буферов и управления Keep-Alive соединениями.

### 🔗 Связь с кодом
```go
// Главный цикл BufferKeepAliveManager:
func (bkm *BufferKeepAliveManager) Start() error {
    // Запуск адаптации буферов
    go bkm.bufferAdaptationLoop()
    
    // Запуск keep-alive управления
    go bkm.keepAliveLoop()
    
    // Запуск обнаружения медленных соединений
    go bkm.slowConnectionDetectionLoop()
    
    return nil
}
```

### 🔄 Адаптация буферов (каждые 10 секунд)

```go
func (bkm *BufferKeepAliveManager) bufferAdaptationLoop() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        for peerID := range bkm.trackedPeers {
            bandwidth := bkm.bandwidthMonitor.GetPeerBandwidth(peerID)
            latency := bkm.estimateLatency(peerID)
            
            bkm.adaptBufferSize(peerID, bandwidth, latency)
        }
    }
}

func (bkm *BufferKeepAliveManager) adaptBufferSize(peerID peer.ID, bandwidth float64, latency time.Duration) {
    bdp := bandwidth * float64(latency.Nanoseconds()) / 1e9
    
    currentBuffer := bkm.connectionBuffers[peerID]
    
    if bandwidth < bkm.config.LowBandwidthThreshold {
        newSize := int(float64(currentBuffer.Size) * 0.75)
        bkm.setBufferSize(peerID, newSize)
    } else if bandwidth > bkm.config.HighBandwidthThreshold {
        newSize := int(float64(currentBuffer.Size) * 1.5)
        bkm.setBufferSize(peerID, newSize)
    } else {
        bkm.setBufferSize(peerID, int(bdp))
    }
}
```

### 💓 Keep-Alive управление

```go
func (bkm *BufferKeepAliveManager) keepAliveLoop() {
    for peerID, state := range bkm.keepAliveStates {
        if !state.Enabled {
            continue
        }
        
        go func(peerID peer.ID, state *KeepAliveState) {
            for {
                time.Sleep(state.AdaptiveInterval)
                
                if bkm.host.Network().Connectedness(peerID) == network.Connected {
                    bkm.recordActivity(peerID)
                    bkm.adaptInterval(peerID, true) // Увеличить интервал
                } else {
                    bkm.sendKeepAlive(peerID)
                    if bkm.keepAliveStates[peerID].ConsecutiveFailures >= 3 {
                        bkm.closeConnection(peerID)
                        return
                    }
                }
            }
        }(peerID, state)
    }
}
```

### 🐌 Обнаружение медленных соединений

```go
func (bkm *BufferKeepAliveManager) slowConnectionDetectionLoop() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        for peerID := range bkm.trackedPeers {
            latency := bkm.measureLatency(peerID)
            bkm.slowDetector.RecordLatency(peerID, latency)
            
            if bkm.slowDetector.IsSlowConnection(peerID) {
                alternatives := bkm.findBypassRoutes(peerID)
                bkm.slowDetector.SetBypassRoutes(peerID, alternatives)
            }
        }
    }
}
```

---

## 🔗 Заключение

Эти диаграммы служат мостом между архитектурным дизайном и реальной реализацией кода Task 4, обеспечивая:

1. **Понимание структуры**: Как компоненты организованы и взаимодействуют
2. **Детали реализации**: Конкретные методы, алгоритмы и структуры данных
3. **Временные аспекты**: Последовательность операций и циклы мониторинга
4. **Развертывание**: Практические аспекты использования в кластерной среде

Каждая диаграмма содержит ссылки на конкретные файлы кода, методы и структуры, что позволяет разработчикам легко переходить от архитектурного понимания к практической реализации.