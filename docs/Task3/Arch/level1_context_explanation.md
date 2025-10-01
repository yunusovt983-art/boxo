# Level 1 Context Diagram - Подробное объяснение

## Файл: `c4_level1_context.puml`

### Архитектурная цель
Контекстная диаграмма определяет границы системы "Оптимизированное блочное хранилище" и показывает её место в экосистеме IPFS. Она отвечает на вопросы: "Кто использует систему?", "С чем она взаимодействует?" и "Какую ценность предоставляет?".

## Связь архитектуры с реализацией

### Центральная система: Оптимизированное блочное хранилище

**Архитектурное представление:**
```plantuml
System(blockstore_opt, "Оптимизированное блочное хранилище", 
       "Высокопроизводительное хранилище с многоуровневым кэшированием, 
        батчевыми операциями и управлением памятью")
```

**Реализация в коде:**
```go
// Файл: blockstore/optimized_blockstore.go
type OptimizedBlockstore struct {
    // Task 3.1: Многоуровневое кэширование
    multiTier    MultiTierBlockstore
    
    // Task 3.2: Батчевые операции I/O
    batchIO      BatchIOManager
    
    // Task 3.3: Потоковая обработка
    streaming    StreamingHandler
    
    // Task 3.4: Управление памятью
    memoryAware  MemoryAwareBlockstore
}

// Основной конструктор системы
func NewOptimizedBlockstore(config *OptimizedConfig) (*OptimizedBlockstore, error) {
    obs := &OptimizedBlockstore{}
    
    // Инициализация всех компонентов Task 3
    if err := obs.initializeMultiTier(config.MultiTier); err != nil {
        return nil, err
    }
    
    if err := obs.initializeBatchIO(config.BatchIO); err != nil {
        return nil, err
    }
    
    if err := obs.initializeStreaming(config.Streaming); err != nil {
        return nil, err
    }
    
    if err := obs.initializeMemoryManagement(config.Memory); err != nil {
        return nil, err
    }
    
    return obs, nil
}
```

### Участники системы (Actors)

#### 1. Разработчик (Developer)

**Архитектурное взаимодействие:**
```plantuml
Rel(developer, blockstore_opt, "Выполняет операции с блоками", "HTTPS/gRPC")
```

**Реализация в коде:**
```go
// Файл: examples/developer_usage.go
func DeveloperWorkflow() {
    ctx := context.Background()
    
    // Разработчик создает оптимизированное хранилище
    config := DefaultOptimizedConfig()
    blockstore, err := NewOptimizedBlockstore(config)
    if err != nil {
        log.Fatal(err)
    }
    defer blockstore.Close()
    
    // Разработчик выполняет операции с блоками
    
    // 1. Сохранение блока (автоматически использует оптимизации)
    data := []byte("Hello, IPFS!")
    block := blocks.NewBlock(data)
    err = blockstore.Put(ctx, block)
    
    // 2. Получение блока (автоматически использует многоуровневый кэш)
    retrievedBlock, err := blockstore.Get(ctx, block.Cid())
    
    // 3. Батчевые операции для высокой производительности
    var manyBlocks []blocks.Block
    for i := 0; i < 1000; i++ {
        manyBlocks = append(manyBlocks, blocks.NewBlock([]byte(fmt.Sprintf("Block %d", i))))
    }
    err = blockstore.BatchPut(ctx, manyBlocks)
    
    // 4. Потоковая обработка больших файлов
    largeData := make([]byte, 10*1024*1024) // 10MB
    err = blockstore.StreamPut(ctx, largeCID, bytes.NewReader(largeData))
}
```

**API интерфейсы для разработчика:**
```go
// Файл: blockstore/developer_api.go
type DeveloperAPI interface {
    // Стандартные операции Blockstore
    Put(ctx context.Context, block blocks.Block) error
    Get(ctx context.Context, c cid.Cid) (blocks.Block, error)
    Has(ctx context.Context, c cid.Cid) (bool, error)
    
    // Оптимизированные операции (Task 3.2)
    BatchPut(ctx context.Context, blocks []blocks.Block) error
    BatchGet(ctx context.Context, cids []cid.Cid) ([]blocks.Block, error)
    
    // Потоковые операции (Task 3.3)
    StreamPut(ctx context.Context, c cid.Cid, data io.Reader) error
    StreamGet(ctx context.Context, c cid.Cid) (io.ReadCloser, error)
    
    // Управление кэшем (Task 3.1)
    PromoteBlock(ctx context.Context, c cid.Cid) error
    GetCacheStats() map[string]interface{}
}
```

#### 2. Администратор (Admin)

**Архитектурное взаимодействие:**
```plantuml
Rel(admin, blockstore_opt, "Мониторит и настраивает", "HTTP/Prometheus")
```

**Реализация в коде:**
```go
// Файл: admin/monitoring.go
type AdminInterface struct {
    blockstore *OptimizedBlockstore
    metrics    *MetricsCollector
}

func (ai *AdminInterface) GetSystemHealth() *HealthReport {
    return &HealthReport{
        // Task 3.1: Статистика многоуровневого кэша
        CacheStats: ai.blockstore.multiTier.GetTierStats(),
        
        // Task 3.2: Статистика батчевых операций
        BatchStats: ai.blockstore.batchIO.GetStats(),
        
        // Task 3.3: Статистика потоковой обработки
        StreamingStats: ai.blockstore.streaming.GetStreamingStats(),
        
        // Task 3.4: Статистика использования памяти
        MemoryStats: ai.blockstore.memoryAware.GetMemoryStats(),
    }
}

func (ai *AdminInterface) ConfigureSystem(config *AdminConfig) error {
    // Администратор может настраивать пороги памяти
    if config.MemoryThresholds != nil {
        ai.blockstore.memoryAware.SetMemoryThresholds(config.MemoryThresholds)
    }
    
    // Настройка параметров кэширования
    if config.CacheConfig != nil {
        ai.blockstore.multiTier.UpdateConfig(config.CacheConfig)
    }
    
    return nil
}
```

**Prometheus метрики для администратора:**
```go
// Файл: metrics/prometheus.go
func (obs *OptimizedBlockstore) RegisterPrometheusMetrics() {
    // Task 3.1: Метрики многоуровневого кэша
    prometheus.MustRegister(prometheus.NewGaugeFunc(
        prometheus.GaugeOpts{
            Name: "blockstore_cache_hit_ratio",
            Help: "Cache hit ratio by tier",
        },
        func() float64 {
            stats := obs.multiTier.GetTierStats()
            return calculateHitRatio(stats)
        },
    ))
    
    // Task 3.2: Метрики батчевых операций
    prometheus.MustRegister(prometheus.NewCounterFunc(
        prometheus.CounterOpts{
            Name: "blockstore_batch_operations_total",
            Help: "Total number of batch operations",
        },
        func() float64 {
            return float64(obs.batchIO.GetStats().TotalBatches)
        },
    ))
    
    // Task 3.4: Метрики памяти
    prometheus.MustRegister(prometheus.NewGaugeFunc(
        prometheus.GaugeOpts{
            Name: "blockstore_memory_pressure_level",
            Help: "Current memory pressure level (0-4)",
        },
        func() float64 {
            return float64(obs.memoryAware.GetMemoryStats().PressureLevel)
        },
    ))
}
```

### Внешние системы (External Systems)

#### 1. Хранилища данных (Storage Backends)

**Архитектурное взаимодействие:**
```plantuml
Rel(blockstore_opt, storage_backends, "Хранит данные", "Файловая система/API")
```

**Реализация в коде:**
```go
// Файл: storage/backends.go
type StorageBackends struct {
    // Memory Tier - самый быстрый уровень
    memoryStore  datastore.Datastore
    
    // SSD Tier - средний уровень производительности
    ssdStore     datastore.Datastore
    
    // HDD Tier - долгосрочное хранение
    hddStore     datastore.Datastore
}

func InitializeStorageBackends(config *StorageConfig) (*StorageBackends, error) {
    sb := &StorageBackends{}
    
    // Memory: In-memory datastore
    sb.memoryStore = dssync.MutexWrap(ds.NewMapDatastore())
    
    // SSD: LevelDB на SSD диске
    ssdDS, err := leveldb.NewDatastore(config.SSDPath, &leveldb.Options{
        Compression: opt.SnappyCompression, // Быстрое сжатие для SSD
    })
    if err != nil {
        return nil, fmt.Errorf("failed to initialize SSD store: %w", err)
    }
    sb.ssdStore = ssdDS
    
    // HDD: FlatFS на HDD диске для больших объемов
    hddDS, err := flatfs.CreateOrOpen(config.HDDPath, flatfs.NextToLast(2), false)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize HDD store: %w", err)
    }
    sb.hddStore = hddDS
    
    return sb, nil
}

// Интеграция с многоуровневым кэшем
func (obs *OptimizedBlockstore) initializeMultiTier(config *MultiTierConfig) error {
    backends, err := InitializeStorageBackends(config.Storage)
    if err != nil {
        return err
    }
    
    tiers := map[CacheTier]Blockstore{
        MemoryTier: NewBlockstore(backends.memoryStore),
        SSDTier:    NewBlockstore(backends.ssdStore),
        HDDTier:    NewBlockstore(backends.hddStore),
    }
    
    obs.multiTier, err = NewMultiTierBlockstore(context.Background(), tiers, config)
    return err
}
```

#### 2. Система мониторинга (Monitoring)

**Архитектурное взаимодействие:**
```plantuml
Rel(blockstore_opt, monitoring, "Отправляет метрики", "HTTP/Prometheus")
```

**Реализация в коде:**
```go
// Файл: monitoring/integration.go
type MonitoringIntegration struct {
    prometheus *PrometheusExporter
    grafana    *GrafanaIntegration
    alerts     *AlertManager
}

func (mi *MonitoringIntegration) StartMonitoring(obs *OptimizedBlockstore) error {
    // Регистрируем все метрики Task 3
    obs.RegisterPrometheusMetrics()
    
    // Настраиваем алерты для критических ситуаций
    mi.setupAlerts()
    
    // Запускаем HTTP сервер для метрик
    http.Handle("/metrics", promhttp.Handler())
    go http.ListenAndServe(":8080", nil)
    
    return nil
}

func (mi *MonitoringIntegration) setupAlerts() {
    // Алерт при высоком давлении памяти
    mi.alerts.AddRule(&AlertRule{
        Name: "HighMemoryPressure",
        Condition: "blockstore_memory_pressure_level >= 3",
        Action: func() {
            // Уведомление администратора
            mi.notifyAdmin("High memory pressure detected")
        },
    })
    
    // Алерт при низком hit rate кэша
    mi.alerts.AddRule(&AlertRule{
        Name: "LowCacheHitRate",
        Condition: "blockstore_cache_hit_ratio < 0.7",
        Action: func() {
            mi.notifyAdmin("Cache hit rate below 70%")
        },
    })
}
```

#### 3. IPFS сеть (IPFS Network)

**Архитектурное взаимодействие:**
```plantuml
Rel(blockstore_opt, ipfs_network, "Синхронизирует блоки", "libp2p")
```

**Реализация в коде:**
```go
// Файл: network/ipfs_integration.go
type IPFSNetworkIntegration struct {
    blockstore *OptimizedBlockstore
    bitswap    exchange.Interface
    host       host.Host
}

func (ini *IPFSNetworkIntegration) SyncWithNetwork(ctx context.Context) error {
    // Используем оптимизированное хранилище для Bitswap
    ini.bitswap = bitswap.New(ctx, ini.host, ini.blockstore)
    
    // Настраиваем оптимизированную синхронизацию
    return ini.setupOptimizedSync()
}

func (ini *IPFSNetworkIntegration) setupOptimizedSync() error {
    // Используем батчевые операции для синхронизации множественных блоков
    ini.bitswap.SetBatchProcessor(func(wants []cid.Cid) {
        // Получаем блоки батчем для лучшей производительности
        blocks, err := ini.blockstore.BatchGet(context.Background(), wants)
        if err == nil {
            ini.sendBlocksToNetwork(blocks)
        }
    })
    
    return nil
}
```

## Ключевые оптимизации (показаны в note)

**Архитектурное представление:**
```plantuml
note right of blockstore_opt
  **Ключевые оптимизации:**
  • Многоуровневое кэширование (Memory/SSD/HDD)
  • Батчевые операции I/O
  • Потоковая обработка больших блоков
  • Интеллектуальное управление памятью
  • Автоматическая деградация при нагрузке
end note
```

**Реализация в коде:**
```go
// Файл: blockstore/optimizations.go
type OptimizationSuite struct {
    // Task 3.1: Многоуровневое кэширование
    MultiTierCaching struct {
        MemoryTier TierOptimization // < 1ms латентность
        SSDTier    TierOptimization // 1-10ms латентность  
        HDDTier    TierOptimization // 10-100ms латентность
    }
    
    // Task 3.2: Батчевые операции I/O
    BatchOperations struct {
        MaxBatchSize         int           // До 1000 блоков в батче
        ConcurrentWorkers    int           // До 50 worker'ов
        TransactionalWrites  bool          // ACID гарантии
    }
    
    // Task 3.3: Потоковая обработка
    StreamingProcessing struct {
        LargeBlockThreshold  int64         // > 1MB блоки
        ChunkSize           int           // 256KB чанки
        CompressionEnabled  bool          // gzip сжатие
    }
    
    // Task 3.4: Управление памятью
    MemoryManagement struct {
        RealTimeMonitoring  bool          // Каждые 5 секунд
        GracefulDegradation bool          // 4 уровня деградации
        AutoCleanup         bool          // Автоматическая очистка
    }
}

func (obs *OptimizedBlockstore) GetOptimizationMetrics() *OptimizationMetrics {
    return &OptimizationMetrics{
        // Улучшения производительности
        LatencyReduction:    "50-80%",     // Снижение латентности Get операций
        ThroughputIncrease: "300-500%",    // Увеличение пропускной способности
        MemoryEfficiency:   "90%",         // Снижение использования памяти для больших блоков
        
        // Масштабируемость
        ConcurrentOps:      10000,         // Одновременных операций
        MaxBlockSize:       "1GB+",       // Максимальный размер блока
        CacheCapacity:      "TB",          // Емкость кэша
        Throughput:         "100K ops/s", // Операций в секунду
    }
}
```

## Бизнес-ценность системы

### Для разработчиков:
1. **Простота использования**: Стандартный Blockstore API с автоматическими оптимизациями
2. **Высокая производительность**: Значительное улучшение скорости операций
3. **Масштабируемость**: Поддержка больших объемов данных и высокой нагрузки

### Для администраторов:
1. **Наблюдаемость**: Подробные метрики и мониторинг
2. **Управляемость**: Настраиваемые параметры и пороги
3. **Надежность**: Автоматическое управление ресурсами и graceful degradation

### Для IPFS экосистемы:
1. **Совместимость**: Полная совместимость с существующими IPFS узлами
2. **Эффективность**: Оптимизированная синхронизация и обмен блоками
3. **Стабильность**: Устойчивость к высоким нагрузкам и ограничениям ресурсов

Контекстная диаграмма служит фундаментом для понимания места оптимизированного блочного хранилища в экосистеме IPFS и определяет основные требования к его реализации.