# Level 2 Container Diagram - Подробное объяснение

## Файл: `c4_level2_container.puml`

### Архитектурная цель
Контейнерная диаграмма показывает высокоуровневую архитектуру оптимизированного блочного хранилища, разбивая его на основные функциональные контейнеры. Каждый контейнер соответствует одной из подзадач Task 3 и имеет четко определенные обязанности.

## Связь архитектуры с реализацией

### Основные контейнеры системы

#### 1. MultiTier Cache (Task 3.1)

**Архитектурное представление:**
```plantuml
Container(multitier_cache, "MultiTier Cache", "Go", 
          "Многоуровневая система кэширования с автоматическим управлением данными")
```

**Реализация в коде:**
```go
// Файл: blockstore/multitier.go
type MultiTierCache struct {
    // Конфигурация уровней
    config *MultiTierConfig
    
    // Физические уровни хранения
    tiers map[CacheTier]Blockstore
    
    // Отслеживание доступа к блокам
    accessTracker *AccessTracker
    
    // LRU движок для каждого уровня
    lruEngines map[CacheTier]*LRUEngine
    
    // Статистика по уровням
    stats map[CacheTier]*TierStats
    
    // Синхронизация
    mu sync.RWMutex
}

// Основной интерфейс контейнера
type MultiTierCacheInterface interface {
    // Стандартные операции Blockstore
    Get(ctx context.Context, c cid.Cid) (blocks.Block, error)
    Put(ctx context.Context, block blocks.Block) error
    Has(ctx context.Context, c cid.Cid) (bool, error)
    
    // Специфичные для многоуровневого кэша
    GetWithTier(ctx context.Context, c cid.Cid, tier CacheTier) (blocks.Block, error)
    PutWithTier(ctx context.Context, block blocks.Block, tier CacheTier) error
    PromoteBlock(ctx context.Context, c cid.Cid) error
    DemoteBlock(ctx context.Context, c cid.Cid) error
    
    // Управление и статистика
    GetTierStats() map[CacheTier]*TierStats
    Compact(ctx context.Context) error
}

func NewMultiTierCache(config *MultiTierConfig) (*MultiTierCache, error) {
    mtc := &MultiTierCache{
        config:        config,
        tiers:         make(map[CacheTier]Blockstore),
        accessTracker: NewAccessTracker(),
        lruEngines:    make(map[CacheTier]*LRUEngine),
        stats:         make(map[CacheTier]*TierStats),
    }
    
    // Инициализация каждого уровня
    for tier, tierConfig := range config.Tiers {
        mtc.initializeTier(tier, tierConfig)
    }
    
    return mtc, nil
}
```

**Связи с хранилищами:**
```go
// Файл: blockstore/multitier.go
func (mtc *MultiTierCache) initializeTier(tier CacheTier, config *TierConfig) error {
    switch tier {
    case MemoryTier:
        // Связь с Memory Tier
        memDS := dssync.MutexWrap(ds.NewMapDatastore())
        mtc.tiers[MemoryTier] = NewBlockstore(memDS)
        
    case SSDTier:
        // Связь с SSD Tier
        ssdDS, err := leveldb.NewDatastore(config.Path, nil)
        if err != nil {
            return err
        }
        mtc.tiers[SSDTier] = NewBlockstore(ssdDS)
        
    case HDDTier:
        // Связь с HDD Tier
        hddDS, err := flatfs.CreateOrOpen(config.Path, flatfs.NextToLast(2), false)
        if err != nil {
            return err
        }
        mtc.tiers[HDDTier] = NewBlockstore(hddDS)
    }
    
    // Инициализация LRU движка для уровня
    mtc.lruEngines[tier] = NewLRUEngine(config.MaxItems)
    mtc.stats[tier] = NewTierStats()
    
    return nil
}
```

#### 2. Batch I/O Manager (Task 3.2)

**Архитектурное представление:**
```plantuml
Container(batch_io, "Batch I/O Manager", "Go", 
          "Группировка и асинхронная обработка операций I/O")
```

**Реализация в коде:**
```go
// Файл: blockstore/batch_io.go
type BatchIOManager struct {
    // Конфигурация батчевых операций
    config *BatchIOConfig
    
    // Базовое хранилище для делегирования
    blockstore Blockstore
    
    // Очереди для разных типов операций
    putQueue    chan *BatchOperation
    getQueue    chan *BatchOperation
    hasQueue    chan *BatchOperation
    deleteQueue chan *BatchOperation
    
    // Управление worker'ами
    workers    sync.WaitGroup
    stopChan   chan struct{}
    flushChan  chan chan error
    
    // Статистика и метрики
    stats   *BatchIOStats
    statsMu sync.RWMutex
}

type BatchIOManagerInterface interface {
    // Батчевые операции
    BatchPut(ctx context.Context, blocks []blocks.Block) error
    BatchGet(ctx context.Context, cids []cid.Cid) ([]blocks.Block, error)
    BatchHas(ctx context.Context, cids []cid.Cid) ([]bool, error)
    BatchDelete(ctx context.Context, cids []cid.Cid) error
    
    // Управление
    Flush(ctx context.Context) error
    Close() error
    GetStats() *BatchIOStats
}

func NewBatchIOManager(ctx context.Context, bs Blockstore, config *BatchIOConfig) (*BatchIOManager, error) {
    bim := &BatchIOManager{
        config:      config,
        blockstore:  bs,
        putQueue:    make(chan *BatchOperation, config.QueueSize),
        getQueue:    make(chan *BatchOperation, config.QueueSize),
        hasQueue:    make(chan *BatchOperation, config.QueueSize),
        deleteQueue: make(chan *BatchOperation, config.QueueSize),
        stopChan:    make(chan struct{}),
        flushChan:   make(chan chan error),
        stats:       NewBatchIOStats(),
    }
    
    // Запуск worker'ов для асинхронной обработки
    bim.startWorkers(ctx)
    
    return bim, nil
}
```

**Асинхронная обработка с goroutines:**
```go
// Файл: blockstore/batch_io.go
func (bim *BatchIOManager) startWorkers(ctx context.Context) {
    for i := 0; i < bim.config.MaxConcurrentBatches; i++ {
        bim.workers.Add(4) // По одному для каждого типа операций
        
        go bim.putWorker(ctx)    // Worker для Put операций
        go bim.getWorker(ctx)    // Worker для Get операций
        go bim.hasWorker(ctx)    // Worker для Has операций
        go bim.deleteWorker(ctx) // Worker для Delete операций
    }
    
    // Координатор flush операций
    bim.workers.Add(1)
    go bim.flushCoordinator(ctx)
}

func (bim *BatchIOManager) putWorker(ctx context.Context) {
    defer bim.workers.Done()
    
    var pendingOps []*BatchOperation
    timer := time.NewTimer(bim.config.BatchTimeout)
    timer.Stop()
    
    for {
        select {
        case op := <-bim.putQueue:
            pendingOps = append(pendingOps, op)
            
            // Запускаем таймер для первой операции
            if len(pendingOps) == 1 {
                timer.Reset(bim.config.BatchTimeout)
            }
            
            // Обрабатываем если батч заполнен
            if len(pendingOps) >= bim.config.MaxBatchSize {
                timer.Stop()
                bim.processPendingPuts(pendingOps)
                pendingOps = nil
            }
            
        case <-timer.C:
            // Обрабатываем по таймауту
            if len(pendingOps) > 0 {
                bim.processPendingPuts(pendingOps)
                pendingOps = nil
            }
            
        case <-ctx.Done():
            return
        }
    }
}
```

#### 3. Streaming Handler (Task 3.3)

**Архитектурное представление:**
```plantuml
Container(streaming, "Streaming Handler", "Go", 
          "Потоковая обработка больших блоков с сжатием и чанкингом")
```

**Реализация в коде:**
```go
// Файл: blockstore/streaming.go
type StreamingHandler struct {
    // Конфигурация потоковой обработки
    config *StreamingConfig
    
    // Базовое хранилище
    blockstore Blockstore
    
    // Управление активными потоками
    activeStreams map[string]*StreamInfo
    streamsMu     sync.RWMutex
    
    // Метаданные чанков
    chunkMetadata map[string]*ChunkMetadata
    metadataMu    sync.RWMutex
    
    // Статистика сжатия
    compressionStats *CompressionStats
}

type StreamingHandlerInterface interface {
    // Потоковые операции
    StreamPut(ctx context.Context, c cid.Cid, data io.Reader) error
    StreamGet(ctx context.Context, c cid.Cid) (io.ReadCloser, error)
    
    // Чанкинг больших файлов
    ChunkedPut(ctx context.Context, c cid.Cid, data io.Reader, chunkSize int) error
    ChunkedGet(ctx context.Context, c cid.Cid) (io.ReadCloser, error)
    
    // Сжатие данных
    CompressedPut(ctx context.Context, c cid.Cid, data io.Reader) error
    CompressedGet(ctx context.Context, c cid.Cid) (io.ReadCloser, error)
    
    // Статистика и управление
    GetStreamingStats() *StreamingStats
    Close() error
}

func NewStreamingHandler(ctx context.Context, bs Blockstore, config *StreamingConfig) (*StreamingHandler, error) {
    sh := &StreamingHandler{
        config:           config,
        blockstore:       bs,
        activeStreams:    make(map[string]*StreamInfo),
        chunkMetadata:    make(map[string]*ChunkMetadata),
        compressionStats: NewCompressionStats(),
    }
    
    return sh, nil
}
```

**Потоковая обработка больших блоков:**
```go
// Файл: blockstore/streaming.go
func (sh *StreamingHandler) StreamPut(ctx context.Context, c cid.Cid, data io.Reader) error {
    // Определяем размер для выбора стратегии
    size := sh.estimateSize(data)
    
    if !sh.shouldUseStreaming(size) {
        // Для маленьких блоков используем обычный Put
        return sh.regularPut(ctx, c, data)
    }
    
    // Регистрируем активный поток
    streamInfo := &StreamInfo{
        CID:       c,
        StartTime: time.Now(),
        Size:      size,
    }
    sh.registerActiveStream(c.String(), streamInfo)
    defer sh.unregisterActiveStream(c.String())
    
    // Чанкинг больших данных
    return sh.processLargeBlock(ctx, c, data, size)
}

func (sh *StreamingHandler) shouldUseStreaming(size int64) bool {
    return size > sh.config.LargeBlockThreshold // По умолчанию 1MB
}

func (sh *StreamingHandler) processLargeBlock(ctx context.Context, c cid.Cid, data io.Reader, size int64) error {
    // Разбиваем на чанки
    chunks, err := sh.chunkData(data, sh.config.DefaultChunkSize)
    if err != nil {
        return err
    }
    
    var chunkCIDs []cid.Cid
    for i, chunk := range chunks {
        // Проверяем необходимость сжатия
        shouldCompress, ratio := sh.shouldCompress(chunk.Data)
        
        var finalData []byte
        compressed := false
        
        if shouldCompress {
            finalData, err = sh.compressData(chunk.Data)
            if err == nil && len(finalData) < len(chunk.Data) {
                compressed = true
                sh.compressionStats.RecordCompression(ratio)
            } else {
                finalData = chunk.Data
            }
        } else {
            finalData = chunk.Data
        }
        
        // Сохраняем чанк
        chunkCID := sh.generateChunkCID(c, i, finalData)
        chunkCIDs = append(chunkCIDs, chunkCID)
        
        chunkBlock := blocks.NewBlock(finalData)
        if err := sh.blockstore.Put(ctx, chunkBlock); err != nil {
            return err
        }
        
        // Сохраняем метаданные чанка
        sh.saveChunkMetadata(chunkCID, &ChunkMetadata{
            Index:      i,
            Size:       len(chunk.Data),
            Compressed: compressed,
            Checksum:   chunk.Checksum,
        })
    }
    
    // Сохраняем метаданные блока
    return sh.saveBlockMetadata(ctx, c, &BlockMetadata{
        TotalChunks: len(chunks),
        ChunkSize:   sh.config.DefaultChunkSize,
        TotalSize:   size,
        ChunkCIDs:   chunkCIDs,
    })
}
```

#### 4. Memory Manager (Task 3.4)

**Архитектурное представление:**
```plantuml
Container(memory_mgr, "Memory Manager", "Go", 
          "Мониторинг памяти и graceful degradation")
```

**Реализация в коде:**
```go
// Файл: blockstore/memory_manager.go
type MemoryManager struct {
    // Конфигурация мониторинга
    config *MemoryConfig
    
    // Пороговые значения
    thresholds *MemoryThresholds
    
    // Текущее состояние
    currentStats  *MemoryStats
    currentLevel  MemoryPressureLevel
    
    // Колбэки для уведомлений
    callbacks []MemoryPressureCallback
    
    // Управление мониторингом
    ticker   *time.Ticker
    stopChan chan struct{}
    
    // Синхронизация
    mu sync.RWMutex
}

type MemoryManagerInterface interface {
    // Мониторинг
    Start(ctx context.Context) error
    Stop() error
    GetStats() *MemoryStats
    GetCurrentPressure() MemoryPressureLevel
    
    // Управление
    SetCallback(callback MemoryPressureCallback)
    ForceGC()
    SetThresholds(thresholds *MemoryThresholds)
}

func NewMemoryManager(config *MemoryConfig) (*MemoryManager, error) {
    mm := &MemoryManager{
        config:       config,
        thresholds:   config.Thresholds,
        currentStats: NewMemoryStats(),
        currentLevel: MemoryPressureNone,
        stopChan:     make(chan struct{}),
    }
    
    return mm, nil
}
```

**Мониторинг в реальном времени:**
```go
// Файл: blockstore/memory_manager.go
func (mm *MemoryManager) Start(ctx context.Context) error {
    mm.ticker = time.NewTicker(mm.config.CheckInterval)
    
    go mm.monitoringLoop(ctx)
    
    return nil
}

func (mm *MemoryManager) monitoringLoop(ctx context.Context) {
    for {
        select {
        case <-mm.ticker.C:
            mm.checkMemoryPressure()
            
        case <-mm.stopChan:
            return
            
        case <-ctx.Done():
            return
        }
    }
}

func (mm *MemoryManager) checkMemoryPressure() {
    // Получаем текущую статистику памяти
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    // Обновляем статистику
    mm.mu.Lock()
    mm.currentStats.UsedMemory = int64(m.Alloc)
    mm.currentStats.TotalMemory = int64(m.Sys)
    mm.currentStats.UsageRatio = float64(m.Alloc) / float64(m.Sys)
    mm.currentStats.GCCycles = int64(m.NumGC)
    mm.mu.Unlock()
    
    // Определяем новый уровень давления
    newLevel := mm.calculatePressureLevel(mm.currentStats.UsageRatio)
    
    if newLevel != mm.currentLevel {
        mm.handlePressureChange(newLevel)
        mm.currentLevel = newLevel
    }
}

func (mm *MemoryManager) calculatePressureLevel(usageRatio float64) MemoryPressureLevel {
    switch {
    case usageRatio >= mm.thresholds.Critical:
        return MemoryPressureCritical
    case usageRatio >= mm.thresholds.High:
        return MemoryPressureHigh
    case usageRatio >= mm.thresholds.Medium:
        return MemoryPressureMedium
    case usageRatio >= mm.thresholds.Low:
        return MemoryPressureLow
    default:
        return MemoryPressureNone
    }
}
```

### Взаимодействия между контейнерами

#### 1. Memory Manager → MultiTier Cache

**Архитектурное взаимодействие:**
```plantuml
Rel(memory_mgr, multitier_cache, "Триггерит очистку кэша", "Callbacks")
```

**Реализация в коде:**
```go
// Файл: blockstore/memory_integration.go
func (mm *MemoryManager) handlePressureChange(level MemoryPressureLevel) {
    // Уведомляем все зарегистрированные колбэки
    for _, callback := range mm.callbacks {
        go callback(level) // Асинхронно чтобы не блокировать мониторинг
    }
}

// Регистрация колбэка от MultiTier Cache
func (mtc *MultiTierCache) RegisterMemoryCallback(mm *MemoryManager) {
    mm.SetCallback(func(level MemoryPressureLevel) {
        switch level {
        case MemoryPressureHigh:
            // Агрессивная очистка Memory Tier
            mtc.cleanupTier(MemoryTier, 0.3) // Освобождаем 30%
            
        case MemoryPressureCritical:
            // Экстренная очистка всех уровней
            mtc.emergencyCleanup()
        }
    })
}

func (mtc *MultiTierCache) cleanupTier(tier CacheTier, ratio float64) {
    // Получаем кандидатов для удаления (LRU)
    candidates := mtc.lruEngines[tier].GetLRUCandidates(ratio)
    
    for _, candidate := range candidates {
        // Понижаем блок на следующий уровень или удаляем
        mtc.demoteOrEvict(candidate)
    }
    
    // Обновляем статистику
    mtc.stats[tier].CleanupOperations++
}
```

#### 2. Batch I/O Manager → Core Blockstore

**Архитектурное взаимодействие:**
```plantuml
Rel(batch_io, core_blockstore, "Выполняет батчи", "Go interface")
```

**Реализация в коде:**
```go
// Файл: blockstore/batch_integration.go
func (bim *BatchIOManager) processPendingPuts(ops []*BatchOperation) {
    start := time.Now()
    
    // Собираем все блоки из всех операций
    var allBlocks []blocks.Block
    for _, op := range ops {
        allBlocks = append(allBlocks, op.Blocks...)
    }
    
    // Выполняем батчевую запись через базовое хранилище
    var err error
    for attempt := 0; attempt <= bim.config.RetryAttempts; attempt++ {
        if attempt > 0 {
            time.Sleep(bim.config.RetryDelay)
        }
        
        // Используем транзакционную запись если поддерживается
        if batcher, ok := bim.blockstore.(BatchPutter); ok {
            err = batcher.PutMany(context.Background(), allBlocks)
        } else {
            // Fallback к индивидуальным Put операциям
            err = bim.putManyIndividually(allBlocks)
        }
        
        if err == nil {
            break
        }
    }
    
    // Обновляем статистику
    bim.updateStats(len(ops), len(allBlocks), time.Since(start), err == nil)
    
    // Отправляем результаты всем операциям
    result := BatchResult{Error: err}
    for _, op := range ops {
        select {
        case op.Results <- result:
        default:
        }
    }
}

// Интерфейс для оптимизированных батчевых операций
type BatchPutter interface {
    PutMany(ctx context.Context, blocks []blocks.Block) error
}
```

### Связи с физическими хранилищами

#### Memory Tier
```go
// Файл: storage/memory_tier.go
type MemoryTier struct {
    datastore datastore.Datastore
    cache     *lru.Cache
    stats     *TierStats
}

func NewMemoryTier(config *MemoryTierConfig) *MemoryTier {
    // In-memory datastore для максимальной скорости
    ds := dssync.MutexWrap(ds.NewMapDatastore())
    
    // LRU кэш для управления размером
    cache, _ := lru.New(config.MaxItems)
    
    return &MemoryTier{
        datastore: ds,
        cache:     cache,
        stats:     NewTierStats(),
    }
}

// Характеристики: < 1ms латентность, > 10 GB/s пропускная способность
```

#### SSD Tier
```go
// Файл: storage/ssd_tier.go
type SSDTier struct {
    datastore datastore.Datastore
    leveldb   *leveldb.DB
    stats     *TierStats
}

func NewSSDTier(config *SSDTierConfig) (*SSDTier, error) {
    // LevelDB на SSD для быстрого доступа
    db, err := leveldb.OpenFile(config.Path, &opt.Options{
        BlockCacheCapacity:     config.BlockCacheSize,
        WriteBuffer:           config.WriteBufferSize,
        CompactionTableSize:   config.CompactionTableSize,
        Compression:           opt.SnappyCompression,
    })
    if err != nil {
        return nil, err
    }
    
    ds := datastore.NewMapDatastore()
    
    return &SSDTier{
        datastore: ds,
        leveldb:   db,
        stats:     NewTierStats(),
    }, nil
}

// Характеристики: 1-10ms латентность, 1-3 GB/s пропускная способность
```

#### HDD Tier
```go
// Файл: storage/hdd_tier.go
type HDDTier struct {
    datastore datastore.Datastore
    flatfs    *flatfs.Datastore
    stats     *TierStats
}

func NewHDDTier(config *HDDTierConfig) (*HDDTier, error) {
    // FlatFS для эффективного хранения больших объемов
    fs, err := flatfs.CreateOrOpen(config.Path, flatfs.NextToLast(2), false)
    if err != nil {
        return nil, err
    }
    
    return &HDDTier{
        datastore: fs,
        flatfs:    fs,
        stats:     NewTierStats(),
    }, nil
}

// Характеристики: 10-100ms латентность, 100-200 MB/s пропускная способность
```

### Metrics Collector

**Архитектурное представление:**
```plantuml
Container(metrics, "Metrics Collector", "Go", "Сбор и экспорт метрик производительности")
```

**Реализация в коде:**
```go
// Файл: metrics/collector.go
type MetricsCollector struct {
    // Источники метрик
    multiTier *MultiTierCache
    batchIO   *BatchIOManager
    streaming *StreamingHandler
    memory    *MemoryManager
    
    // Prometheus метрики
    registry *prometheus.Registry
    
    // HTTP сервер для экспорта
    server *http.Server
}

func NewMetricsCollector(components *SystemComponents) *MetricsCollector {
    mc := &MetricsCollector{
        multiTier: components.MultiTier,
        batchIO:   components.BatchIO,
        streaming: components.Streaming,
        memory:    components.Memory,
        registry:  prometheus.NewRegistry(),
    }
    
    mc.registerMetrics()
    mc.startHTTPServer()
    
    return mc
}

func (mc *MetricsCollector) registerMetrics() {
    // Task 3.1: MultiTier Cache метрики
    mc.registry.MustRegister(prometheus.NewGaugeFunc(
        prometheus.GaugeOpts{
            Name: "multitier_cache_size_bytes",
            Help: "Current cache size in bytes by tier",
        },
        func() float64 {
            stats := mc.multiTier.GetTierStats()
            return float64(stats[MemoryTier].Size)
        },
    ))
    
    // Task 3.2: Batch I/O метрики
    mc.registry.MustRegister(prometheus.NewCounterFunc(
        prometheus.CounterOpts{
            Name: "batch_io_operations_total",
            Help: "Total number of batch I/O operations",
        },
        func() float64 {
            stats := mc.batchIO.GetStats()
            return float64(stats.TotalBatches)
        },
    ))
    
    // Task 3.3: Streaming метрики
    mc.registry.MustRegister(prometheus.NewGaugeFunc(
        prometheus.GaugeOpts{
            Name: "streaming_compression_ratio",
            Help: "Current compression ratio for streaming operations",
        },
        func() float64 {
            stats := mc.streaming.GetStreamingStats()
            return stats.CompressionRatio
        },
    ))
    
    // Task 3.4: Memory метрики
    mc.registry.MustRegister(prometheus.NewGaugeFunc(
        prometheus.GaugeOpts{
            Name: "memory_pressure_level",
            Help: "Current memory pressure level (0-4)",
        },
        func() float64 {
            return float64(mc.memory.GetCurrentPressure())
        },
    ))
}
```

## Заключение

Контейнерная диаграмма Level 2 показывает, как Task 3 разбивается на четыре основных функциональных контейнера, каждый из которых решает конкретную задачу оптимизации:

1. **MultiTier Cache** - обеспечивает интеллектуальное кэширование на трех уровнях
2. **Batch I/O Manager** - повышает пропускную способность через группировку операций
3. **Streaming Handler** - эффективно обрабатывает большие блоки
4. **Memory Manager** - обеспечивает стабильность через управление памятью

Все контейнеры тесно интегрированы и работают совместно для достижения максимальной производительности оптимизированного блочного хранилища.