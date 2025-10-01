# Подробные объяснения архитектурных диаграмм Task 3

## Обзор

Данный документ содержит детальные объяснения каждой архитектурной диаграммы Task 3, показывая прямую связь между архитектурным дизайном и фактической реализацией кода. Каждая диаграмма служит мостом между высокоуровневыми концепциями и конкретными файлами Go кода.

---

## 1. Context Diagram (Level 1) - `c4_level1_context.puml`

### Архитектурная цель
Показать оптимизированное блочное хранилище в контексте всей экосистемы IPFS, определить границы системы и основных участников.

### Связь с реализацией

#### Основная система: "Оптимизированное блочное хранилище"
```go
// Файл: blockstore/blockstore.go
type Blockstore interface {
    DeleteBlock(context.Context, cid.Cid) error
    Has(context.Context, cid.Cid) (bool, error)
    Get(context.Context, cid.Cid) (blocks.Block, error)
    GetSize(context.Context, cid.Cid) (int, error)
    Put(context.Context, blocks.Block) error
    PutMany(context.Context, []blocks.Block) error
    AllKeysChan(context.Context) (<-chan cid.Cid, error)
    HashOnRead(enabled bool)
}
```

#### Участники системы

**Разработчик (Developer)**
- **Архитектурная роль**: Использует API блочного хранилища для операций с данными
- **Реализация в коде**: 
```go
// Пример использования в клиентском коде
func ExampleDeveloperUsage() {
    ctx := context.Background()
    
    // Создание оптимизированного blockstore
    mabs, err := NewMemoryAwareBlockstore(ctx, baseBlockstore, config)
    
    // Использование разработчиком
    block := blocks.NewBlock([]byte("developer data"))
    err = mabs.Put(ctx, block)
    
    retrievedBlock, err := mabs.Get(ctx, block.Cid())
}
```

**Администратор (Admin)**
- **Архитектурная роль**: Мониторит производительность и управляет ресурсами
- **Реализация в коде**:
```go
// Файл: blockstore/memory_manager.go
func (mm *MemoryMonitor) GetStats() *MemoryStats {
    // Предоставляет статистику для администратора
    return &MemoryStats{
        UsedMemory:     mm.getCurrentMemoryUsage(),
        TotalMemory:    mm.getTotalMemory(),
        UsageRatio:     mm.calculateUsageRatio(),
        PressureLevel:  mm.currentPressureLevel,
        GCCycles:       mm.gcCycles,
    }
}
```

#### Внешние системы

**Хранилища данных (Storage Backends)**
- **Архитектурная роль**: Физическое хранение данных на разных уровнях
- **Реализация в коде**:
```go
// Файл: blockstore/multitier.go
type multiTierBlockstore struct {
    tiers map[CacheTier]Blockstore  // Memory, SSD, HDD уровни
}

func (mtbs *multiTierBlockstore) initializeTiers() {
    mtbs.tiers = map[CacheTier]Blockstore{
        MemoryTier: NewBlockstore(memoryDatastore),  // RAM
        SSDTier:    NewBlockstore(ssdDatastore),     // SSD
        HDDTier:    NewBlockstore(hddDatastore),     // HDD
    }
}
```

**Система мониторинга (Monitoring)**
- **Архитектурная роль**: Сбор и визуализация метрик производительности
- **Реализация в коде**:
```go
// Файл: blockstore/metrics.go (концептуальный)
func (mtbs *multiTierBlockstore) exportPrometheusMetrics() {
    // Экспорт метрик для Prometheus
    prometheus.NewGaugeVec("multitier_cache_size_bytes", []string{"tier"})
    prometheus.NewCounterVec("multitier_operations_total", []string{"operation", "tier"})
}
```

### Ключевые потоки данных

1. **"Выполняет операции с блоками"** → Реализовано через стандартный Blockstore интерфейс
2. **"Хранит данные"** → Реализовано через делегирование к конкретным datastore'ам
3. **"Отправляет метрики"** → Реализовано через Prometheus экспортеры

---

## 2. Container Diagram (Level 2) - `c4_level2_container.puml`

### Архитектурная цель
Показать основные контейнеры (компоненты высокого уровня) и их взаимодействия внутри оптимизированного блочного хранилища.

### Связь с реализацией

#### MultiTier Cache
- **Архитектурный контейнер**: Многоуровневая система кэширования
- **Файл реализации**: `blockstore/multitier.go`
- **Основной интерфейс**:
```go
type MultiTierBlockstore interface {
    Blockstore
    Viewer
    GetWithTier(ctx context.Context, c cid.Cid, tier CacheTier) (blocks.Block, error)
    PutWithTier(ctx context.Context, block blocks.Block, tier CacheTier) error
    PromoteBlock(ctx context.Context, c cid.Cid) error
    DemoteBlock(ctx context.Context, c cid.Cid) error
    GetTierStats() map[CacheTier]*TierStats
}
```

#### Batch I/O Manager
- **Архитектурный контейнер**: Группировка и асинхронная обработка операций I/O
- **Файл реализации**: `blockstore/batch_io.go`
- **Основной интерфейс**:
```go
type BatchIOManager interface {
    BatchPut(ctx context.Context, blocks []blocks.Block) error
    BatchGet(ctx context.Context, cids []cid.Cid) ([]blocks.Block, error)
    BatchHas(ctx context.Context, cids []cid.Cid) ([]bool, error)
    BatchDelete(ctx context.Context, cids []cid.Cid) error
    Flush(ctx context.Context) error
    GetStats() *BatchIOStats
}
```

#### Streaming Handler
- **Архитектурный контейнер**: Потоковая обработка больших блоков
- **Файл реализации**: `blockstore/streaming.go`
- **Основной интерфейс**:
```go
type StreamingHandler interface {
    StreamPut(ctx context.Context, c cid.Cid, data io.Reader) error
    StreamGet(ctx context.Context, c cid.Cid) (io.ReadCloser, error)
    ChunkedPut(ctx context.Context, c cid.Cid, data io.Reader, chunkSize int) error
    CompressedPut(ctx context.Context, c cid.Cid, data io.Reader) error
}
```

#### Memory Manager
- **Архитектурный контейнер**: Мониторинг памяти и graceful degradation
- **Файлы реализации**: `blockstore/memory_manager.go`, `blockstore/memory_aware.go`
- **Основные интерфейсы**:
```go
type MemoryMonitor interface {
    Start(ctx context.Context) error
    GetStats() *MemoryStats
    SetCallback(callback MemoryPressureCallback)
    ForceGC()
}

type MemoryAwareBlockstore interface {
    Blockstore
    GetMemoryStats() *MemoryStats
    SetMemoryLimits(limits *MemoryLimits)
}
```

### Взаимодействия между контейнерами

#### "Проверяет давление памяти"
```go
// Файл: blockstore/multitier.go
func (mtbs *multiTierBlockstore) Put(ctx context.Context, block blocks.Block) error {
    // Проверка давления памяти перед операцией
    if mtbs.memoryManager.GetCurrentPressure() >= MemoryPressureHigh {
        return mtbs.handleHighMemoryPressure(ctx, block)
    }
    return mtbs.putToOptimalTier(ctx, block)
}
```

#### "Триггерит очистку кэша"
```go
// Файл: blockstore/memory_aware.go
func (mabs *MemoryAwareBlockstore) onMemoryPressure(level MemoryPressureLevel) {
    switch level {
    case MemoryPressureHigh:
        mabs.multiTierCache.TriggerCleanup(AggressivenessHigh)
    case MemoryPressureCritical:
        mabs.multiTierCache.TriggerCleanup(AggressivenessMaximum)
    }
}
```

---

## 3. Component Diagram (Level 3) - `c4_level3_component.puml`

### Архитектурная цель
Детализировать внутреннюю структуру каждого контейнера, показать компоненты и их специфические обязанности.

### Связь с реализацией

#### MultiTier Cache System Components

**Tier Manager**
- **Архитектурная роль**: Управление уровнями кэширования и их конфигурацией
- **Реализация в коде**:
```go
// Файл: blockstore/multitier.go
type tierManager struct {
    tiers   map[CacheTier]Blockstore
    configs map[CacheTier]*TierConfig
    stats   map[CacheTier]*TierStats
}

func (tm *tierManager) selectOptimalTier(blockSize int, accessPattern AccessPattern) CacheTier {
    // Логика выбора оптимального уровня
    if blockSize < tm.configs[MemoryTier].MaxBlockSize && 
       tm.stats[MemoryTier].AvailableSpace() > blockSize {
        return MemoryTier
    }
    return SSDTier
}
```

**LRU Engine**
- **Архитектурная роль**: LRU алгоритм с учетом частоты доступа
- **Реализация в коде**:
```go
// Файл: blockstore/multitier.go
type lruEngine struct {
    accessInfo map[string]*AccessInfo
    mu         sync.RWMutex
}

func (lru *lruEngine) updateAccess(key string, tier CacheTier) {
    lru.mu.Lock()
    defer lru.mu.Unlock()
    
    info := lru.accessInfo[key]
    if info == nil {
        info = &AccessInfo{FirstAccess: time.Now()}
        lru.accessInfo[key] = info
    }
    
    info.Count++
    info.LastAccess = time.Now()
    info.CurrentTier = tier
}
```

**Promotion Engine**
- **Архитектурная роль**: Автоматическое продвижение блоков между уровнями
- **Реализация в коде**:
```go
// Файл: blockstore/multitier.go
func (pe *promotionEngine) considerPromotion(ctx context.Context, c cid.Cid) error {
    accessInfo := pe.lruEngine.getAccessInfo(c.String())
    if accessInfo == nil {
        return nil
    }
    
    threshold := pe.tierManager.getPromotionThreshold(accessInfo.CurrentTier)
    if accessInfo.Count >= threshold {
        return pe.promoteToUpperTier(ctx, c, accessInfo.CurrentTier)
    }
    return nil
}
```

#### Batch I/O System Components

**Operation Queue**
- **Архитектурная роль**: Очереди для разных типов операций
- **Реализация в коде**:
```go
// Файл: blockstore/batch_io.go
type operationQueue struct {
    putQueue    chan *BatchOperation
    getQueue    chan *BatchOperation
    hasQueue    chan *BatchOperation
    deleteQueue chan *BatchOperation
}

func (oq *operationQueue) enqueuePut(op *BatchOperation) error {
    select {
    case oq.putQueue <- op:
        return nil
    case <-time.After(oq.timeout):
        return ErrQueueTimeout
    }
}
```

**Worker Pool**
- **Архитектурная роль**: Пул goroutines для асинхронной обработки
- **Реализация в коде**:
```go
// Файл: blockstore/batch_io.go
func (bim *batchIOManager) startWorkers(ctx context.Context) {
    for i := 0; i < bim.config.MaxConcurrentBatches; i++ {
        bim.workers.Add(4) // По одному для каждого типа операций
        
        go bim.putWorker(ctx)    // Worker для Put операций
        go bim.getWorker(ctx)    // Worker для Get операций
        go bim.hasWorker(ctx)    // Worker для Has операций
        go bim.deleteWorker(ctx) // Worker для Delete операций
    }
}
```

**Batch Processor**
- **Архитектурная роль**: Группировка и обработка батчей
- **Реализация в коде**:
```go
// Файл: blockstore/batch_io.go
func (bp *batchProcessor) processPendingPuts(ops []*BatchOperation) {
    start := time.Now()
    
    // Собираем все блоки из всех операций
    var allBlocks []blocks.Block
    for _, op := range ops {
        allBlocks = append(allBlocks, op.Blocks...)
    }
    
    // Выполняем батчевую запись
    err := bp.transactionManager.executeBatch(allBlocks)
    
    // Обновляем статистику
    bp.updateStats(len(ops), len(allBlocks), time.Since(start), err == nil)
}
```

#### Streaming System Components

**Chunk Manager**
- **Архитектурная роль**: Разбиение и сборка больших блоков
- **Реализация в коде**:
```go
// Файл: blockstore/streaming.go
type chunkManager struct {
    chunkSize     int
    metadata      map[string]*ChunkMetadata
    blockstore    Blockstore
}

func (cm *chunkManager) chunkData(data io.Reader, chunkSize int) ([]*Chunk, error) {
    var chunks []*Chunk
    buffer := make([]byte, chunkSize)
    
    for {
        n, err := data.Read(buffer)
        if n > 0 {
            chunk := &Chunk{
                Data:     buffer[:n],
                Index:    len(chunks),
                Checksum: calculateChecksum(buffer[:n]),
            }
            chunks = append(chunks, chunk)
        }
        if err == io.EOF {
            break
        }
    }
    return chunks, nil
}
```

**Compression Engine**
- **Архитектурная роль**: Адаптивное сжатие данных
- **Реализация в коде**:
```go
// Файл: blockstore/streaming.go
func (ce *compressionEngine) shouldCompress(data []byte) (bool, float64) {
    // Быстрая оценка сжимаемости
    entropy := ce.calculateEntropy(data)
    estimatedRatio := ce.estimateCompressionRatio(entropy)
    
    // Сжимаем только если ожидаемая экономия > 20%
    return estimatedRatio < 0.8, estimatedRatio
}

func (ce *compressionEngine) compressData(data []byte) ([]byte, error) {
    var buf bytes.Buffer
    writer := gzip.NewWriter(&buf)
    writer.Write(data)
    writer.Close()
    return buf.Bytes(), nil
}
```

#### Memory Management System Components

**Memory Monitor**
- **Архитектурная роль**: Мониторинг использования памяти в реальном времени
- **Реализация в коде**:
```go
// Файл: blockstore/memory_manager.go
func (mm *MemoryMonitor) startMonitoring(ctx context.Context) {
    ticker := time.NewTicker(mm.config.CheckInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            mm.checkMemoryPressure()
        case <-ctx.Done():
            return
        }
    }
}

func (mm *MemoryMonitor) checkMemoryPressure() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    usageRatio := float64(m.Alloc) / float64(m.Sys)
    newLevel := mm.calculatePressureLevel(usageRatio)
    
    if newLevel != mm.currentLevel {
        mm.notifyPressureChange(newLevel)
    }
}
```

**Pressure Detector**
- **Архитектурная роль**: Определение уровней давления памяти
- **Реализация в коде**:
```go
// Файл: blockstore/memory_manager.go
func (pd *pressureDetector) calculatePressureLevel(usageRatio float64) MemoryPressureLevel {
    switch {
    case usageRatio >= pd.thresholds.Critical:
        return MemoryPressureCritical
    case usageRatio >= pd.thresholds.High:
        return MemoryPressureHigh
    case usageRatio >= pd.thresholds.Medium:
        return MemoryPressureMedium
    case usageRatio >= pd.thresholds.Low:
        return MemoryPressureLow
    default:
        return MemoryPressureNone
    }
}
```

---

## 4. Code Diagram (Level 4) - `c4_level4_code.puml`

### Архитектурная цель
Показать детальную структуру кода: интерфейсы, классы, их методы и взаимосвязи.

### Связь с реализацией

#### Интерфейсы и их реализации

**MultiTierBlockstore Interface → multiTierBlockstore Implementation**
```go
// Файл: blockstore/multitier.go

// Интерфейс (показан в диаграмме)
type MultiTierBlockstore interface {
    Get(ctx context.Context, cid cid.Cid) (blocks.Block, error)
    Put(ctx context.Context, block blocks.Block) error
    GetWithTier(ctx context.Context, c cid.Cid, tier CacheTier) (blocks.Block, error)
    PutWithTier(ctx context.Context, block blocks.Block, tier CacheTier) error
    PromoteBlock(ctx context.Context, c cid.Cid) error
    DemoteBlock(ctx context.Context, c cid.Cid) error
    GetTierStats() map[CacheTier]*TierStats
    Compact(ctx context.Context) error
}

// Реализация (показана в диаграмме)
type multiTierBlockstore struct {
    config     *MultiTierConfig
    tiers      map[CacheTier]Blockstore
    accessInfo map[string]*AccessInfo
    stats      map[CacheTier]*TierStats
    mu         sync.RWMutex
    compactionTicker *time.Ticker
}

// Конструктор (показан в диаграмме)
func NewMultiTierBlockstore(ctx context.Context, tiers map[CacheTier]Blockstore, config *MultiTierConfig) (*multiTierBlockstore, error) {
    mtbs := &multiTierBlockstore{
        config:     config,
        tiers:      tiers,
        accessInfo: make(map[string]*AccessInfo),
        stats:      make(map[CacheTier]*TierStats),
    }
    
    mtbs.startCompactionLoop(ctx)
    return mtbs, nil
}
```

#### Структуры данных

**TierConfig (показана в диаграмме)**
```go
// Файл: blockstore/multitier.go
type TierConfig struct {
    MaxSize            int64         `json:"max_size"`
    MaxItems           int           `json:"max_items"`
    TTL                time.Duration `json:"ttl"`
    PromotionThreshold int           `json:"promotion_threshold"`
    Enabled            bool          `json:"enabled"`
}
```

**AccessInfo (показана в диаграмме)**
```go
// Файл: blockstore/multitier.go
type AccessInfo struct {
    Count       int           `json:"count"`
    LastAccess  time.Time     `json:"last_access"`
    FirstAccess time.Time     `json:"first_access"`
    CurrentTier CacheTier     `json:"current_tier"`
    Size        int           `json:"size"`
}
```

**BatchOperation (показана в диаграмме)**
```go
// Файл: blockstore/batch_io.go
type BatchOperation struct {
    Type      BatchOpType           `json:"type"`
    Blocks    []blocks.Block        `json:"blocks,omitempty"`
    CIDs      []cid.Cid            `json:"cids,omitempty"`
    Results   chan BatchResult      `json:"-"`
    Context   context.Context       `json:"-"`
    Timestamp time.Time            `json:"timestamp"`
}
```

#### Методы и их реализация

**updateAccessInfo (показан в диаграмме)**
```go
// Файл: blockstore/multitier.go
func (mtbs *multiTierBlockstore) updateAccessInfo(key string, tier CacheTier, size int) {
    mtbs.mu.Lock()
    defer mtbs.mu.Unlock()
    
    info := mtbs.accessInfo[key]
    if info == nil {
        info = &AccessInfo{
            FirstAccess: time.Now(),
            CurrentTier: tier,
            Size:        size,
        }
        mtbs.accessInfo[key] = info
    }
    
    info.Count++
    info.LastAccess = time.Now()
    
    // Обновляем статистику уровня
    mtbs.stats[tier].Hits++
}
```

**processPendingPuts (показан в диаграмме)**
```go
// Файл: blockstore/batch_io.go
func (bim *batchIOManager) processPendingPuts(ops []*BatchOperation) {
    start := time.Now()
    
    // Собираем все блоки из всех операций
    var allBlocks []blocks.Block
    for _, op := range ops {
        allBlocks = append(allBlocks, op.Blocks...)
    }
    
    // Выполняем транзакционную запись
    var err error
    for attempt := 0; attempt <= bim.config.RetryAttempts; attempt++ {
        if attempt > 0 {
            time.Sleep(bim.config.RetryDelay)
        }
        
        err = bim.transactionalPutMany(context.Background(), allBlocks)
        if err == nil {
            break
        }
    }
    
    // Обновляем статистику
    bim.updateStats(len(ops), len(allBlocks), time.Since(start), err == nil)
    
    // Отправляем результаты
    result := BatchResult{Error: err}
    for _, op := range ops {
        select {
        case op.Results <- result:
        default:
        }
    }
}
```

#### Композиция и агрегация (показаны в диаграмме)

**MemoryAwareBlockstore wraps MultiTierBlockstore**
```go
// Файл: blockstore/memory_aware.go
type MemoryAwareBlockstore struct {
    blockstore Blockstore           // Может быть MultiTierBlockstore
    monitor    *MemoryMonitor       // Композиция
    config     *MemoryAwareConfig   // Композиция
    degradationLevel int
    lastCleanup time.Time
}

func (mabs *MemoryAwareBlockstore) Put(ctx context.Context, block blocks.Block) error {
    // Проверяем давление памяти
    if mabs.shouldRejectOperation() {
        return ErrMemoryPressure
    }
    
    // Делегируем к обернутому blockstore (может быть MultiTierBlockstore)
    return mabs.blockstore.Put(ctx, block)
}
```

---

## 5. Sequence Diagrams - Детальные объяснения

### 5.1 MultiTier Operations - `sequence_multitier_operations.puml`

#### Архитектурная цель
Показать временную последовательность операций многоуровневого кэширования с автоматическим продвижением и управлением памятью.

#### Сценарий 1: "Операция Get с автоматическим продвижением"

**Архитектурный поток**:
1. Client → MultiTierBlockstore: Get(ctx, cid)
2. Поиск по уровням: Memory → SSD → HDD
3. Обновление статистики доступа
4. Проверка необходимости продвижения
5. Автоматическое продвижение на SSD уровень

**Реализация в коде**:
```go
// Файл: blockstore/multitier.go
func (mtbs *multiTierBlockstore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
    key := c.String()
    
    // 1. Проверяем Memory Tier
    if block, err := mtbs.tiers[MemoryTier].Get(ctx, c); err == nil {
        mtbs.updateAccessInfo(key, MemoryTier, len(block.RawData()))
        return block, nil
    }
    
    // 2. Проверяем SSD Tier
    if block, err := mtbs.tiers[SSDTier].Get(ctx, c); err == nil {
        mtbs.updateAccessInfo(key, SSDTier, len(block.RawData()))
        mtbs.considerPromotion(ctx, c, key) // Асинхронно
        return block, nil
    }
    
    // 3. Проверяем HDD Tier
    if block, err := mtbs.tiers[HDDTier].Get(ctx, c); err == nil {
        mtbs.updateAccessInfo(key, HDDTier, len(block.RawData()))
        
        // Продвигаем на SSD если часто используется
        go mtbs.considerPromotion(ctx, c, key)
        
        return block, nil
    }
    
    return nil, ErrNotFound
}

func (mtbs *multiTierBlockstore) considerPromotion(ctx context.Context, c cid.Cid, key string) error {
    mtbs.mu.RLock()
    accessInfo := mtbs.accessInfo[key]
    mtbs.mu.RUnlock()
    
    if accessInfo == nil {
        return nil
    }
    
    // Проверяем порог для продвижения
    threshold := mtbs.config.Tiers[accessInfo.CurrentTier].PromotionThreshold
    if accessInfo.Count >= threshold {
        return mtbs.PromoteBlock(ctx, c)
    }
    
    return nil
}
```

#### Сценарий 2: "Автоматическая очистка при высоком давлении памяти"

**Архитектурный поток**:
1. MemoryMonitor обнаруживает высокое давление
2. Уведомляет MultiTierBlockstore
3. Поиск кандидатов для понижения (LRU)
4. Перемещение блоков с Memory на SSD уровень

**Реализация в коде**:
```go
// Файл: blockstore/memory_aware.go
func (mabs *MemoryAwareBlockstore) handleMemoryPressure(level MemoryPressureLevel) {
    switch level {
    case MemoryPressureHigh:
        // Триггерим агрессивную очистку кэша
        if mtbs, ok := mabs.blockstore.(*multiTierBlockstore); ok {
            mtbs.makeSpace(context.Background(), MemoryTier, 0.3) // Освобождаем 30%
        }
    }
}

// Файл: blockstore/multitier.go
func (mtbs *multiTierBlockstore) makeSpace(ctx context.Context, tier CacheTier, ratio float64) error {
    // Получаем кандидатов для понижения (LRU)
    candidates := mtbs.getLRUCandidates(tier, ratio)
    
    for _, candidate := range candidates {
        // Понижаем блок на следующий уровень
        if err := mtbs.DemoteBlock(ctx, candidate.CID); err != nil {
            continue // Продолжаем с другими кандидатами
        }
    }
    
    return nil
}
```

### 5.2 Batch Operations - `sequence_batch_operations.puml`

#### Архитектурная цель
Показать асинхронную обработку батчевых операций с группировкой, retry механизмами и конкурентной обработкой.

#### Сценарий: "Батчевая операция Put с разбиением"

**Архитектурный поток**:
1. Client отправляет 1000 блоков
2. BatchIOManager разбивает на батчи по 100 блоков
3. Операции ставятся в очередь
4. Worker асинхронно обрабатывает батчи
5. Группировка по таймауту
6. Транзакционное выполнение с retry

**Реализация в коде**:
```go
// Файл: blockstore/batch_io.go
func (bim *batchIOManager) BatchPut(ctx context.Context, blocks []blocks.Block) error {
    // Разбиваем на батчи если превышен MaxBatchSize
    for i := 0; i < len(blocks); i += bim.config.MaxBatchSize {
        end := i + bim.config.MaxBatchSize
        if end > len(blocks) {
            end = len(blocks)
        }
        
        batch := blocks[i:end]
        op := &BatchOperation{
            Type:      BatchOpPut,
            Blocks:    batch,
            Results:   make(chan BatchResult, 1),
            Context:   ctx,
            Timestamp: time.Now(),
        }
        
        // Отправляем в очередь worker'у
        select {
        case bim.putQueue <- op:
        case <-ctx.Done():
            return ctx.Err()
        }
        
        // Ожидаем результат
        select {
        case result := <-op.Results:
            if result.Error != nil {
                return result.Error
            }
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    return nil
}

// Worker для обработки Put операций
func (bim *batchIOManager) putWorker(ctx context.Context) {
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

### 5.3 Streaming Operations - `sequence_streaming_operations.puml`

#### Архитектурная цель
Показать потоковую обработку больших блоков с чанкингом, сжатием и восстановлением.

#### Сценарий: "Потоковая запись большого блока"

**Архитектурный поток**:
1. Client отправляет 5MB блок
2. StreamingHandler определяет необходимость потоковой обработки
3. ChunkManager разбивает на чанки по 256KB
4. CompressionEngine анализирует и сжимает каждый чанк
5. Сохранение чанков в Blockstore
6. MetadataTracker записывает метаданные

**Реализация в коде**:
```go
// Файл: blockstore/streaming.go
func (sh *streamingHandler) StreamPut(ctx context.Context, c cid.Cid, data io.Reader) error {
    // Определяем размер данных
    size := sh.estimateSize(data)
    
    if !sh.shouldUseStreaming(size) {
        // Для маленьких блоков используем обычный Put
        return sh.regularPut(ctx, c, data)
    }
    
    // Чанкинг больших данных
    chunks, err := sh.chunkManager.chunkData(data, sh.config.DefaultChunkSize)
    if err != nil {
        return err
    }
    
    var chunkCIDs []cid.Cid
    for i, chunk := range chunks {
        // Проверяем необходимость сжатия
        shouldCompress, ratio := sh.compressionEngine.shouldCompress(chunk.Data)
        
        var finalData []byte
        compressed := false
        
        if shouldCompress {
            finalData, err = sh.compressionEngine.compressData(chunk.Data)
            if err == nil && len(finalData) < len(chunk.Data) {
                compressed = true
            } else {
                finalData = chunk.Data // Используем оригинал если сжатие неэффективно
            }
        } else {
            finalData = chunk.Data
        }
        
        // Создаем CID для чанка
        chunkCID := sh.generateChunkCID(c, i, finalData)
        chunkCIDs = append(chunkCIDs, chunkCID)
        
        // Сохраняем чанк
        chunkBlock := blocks.NewBlock(finalData)
        if err := sh.blockstore.Put(ctx, chunkBlock); err != nil {
            return err
        }
        
        // Записываем метаданные чанка
        chunkMeta := &ChunkMetadata{
            Index:      i,
            Size:       len(chunk.Data),
            Compressed: compressed,
            Checksum:   chunk.Checksum,
        }
        sh.metadataTracker.recordChunk(chunkCID, chunkMeta)
    }
    
    // Создаем и сохраняем метаданные блока
    blockMeta := &BlockMetadata{
        OriginalCID:  c,
        TotalChunks:  len(chunks),
        ChunkSize:    sh.config.DefaultChunkSize,
        TotalSize:    size,
        ChunkCIDs:    chunkCIDs,
    }
    
    return sh.metadataTracker.saveBlockMetadata(ctx, c, blockMeta)
}

func (sh *streamingHandler) shouldUseStreaming(size int64) bool {
    return size > sh.config.LargeBlockThreshold // По умолчанию 1MB
}
```

### 5.4 Memory Management - `sequence_memory_management.puml`

#### Архитектурная цель
Показать полный цикл управления памятью: мониторинг, обнаружение давления, graceful degradation и восстановление.

#### Сценарий: "Обнаружение и обработка критического давления памяти"

**Архитектурный поток**:
1. MemoryMonitor периодически проверяет использование памяти
2. PressureDetector определяет критический уровень (97%)
3. GCCoordinator принудительно запускает сборку мусора
4. DegradationController активирует аварийный режим
5. CleanupScheduler очищает все кэши
6. Восстановление при снижении давления

**Реализация в коде**:
```go
// Файл: blockstore/memory_manager.go
func (mm *MemoryMonitor) checkMemoryPressure() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    // Вычисляем коэффициент использования памяти
    usageRatio := float64(m.Alloc) / float64(m.Sys)
    newLevel := mm.pressureDetector.calculatePressureLevel(usageRatio)
    
    if newLevel != mm.currentLevel {
        mm.handlePressureChange(newLevel)
        mm.currentLevel = newLevel
    }
}

func (mm *MemoryMonitor) handlePressureChange(level MemoryPressureLevel) {
    switch level {
    case MemoryPressureCritical:
        // Принудительная сборка мусора
        mm.gcCoordinator.forceGC()
        
        // Активируем аварийный режим
        mm.degradationController.activateEmergencyMode()
        
        // Очищаем все кэши
        mm.cleanupScheduler.emergencyCleanup()
        
    case MemoryPressureHigh:
        // Агрессивная очистка кэша
        mm.cleanupScheduler.scheduleAggressiveCleanup()
        
    case MemoryPressureNone:
        // Восстанавливаем нормальную работу
        mm.degradationController.restoreNormalOperations()
    }
    
    // Уведомляем всех подписчиков
    mm.notifyCallbacks(level)
}

// Файл: blockstore/memory_aware.go
func (mabs *MemoryAwareBlockstore) onMemoryPressure(level MemoryPressureLevel) {
    switch level {
    case MemoryPressureCritical:
        mabs.degradationLevel = 4 // Максимальная деградация
        mabs.rejectAllWrites = true
        
    case MemoryPressureHigh:
        mabs.degradationLevel = 3
        mabs.disableCache = true
        
    case MemoryPressureNone:
        mabs.degradationLevel = 0
        mabs.rejectAllWrites = false
        mabs.disableCache = false
    }
}

func (mabs *MemoryAwareBlockstore) shouldRejectOperation() bool {
    return mabs.rejectAllWrites && mabs.degradationLevel >= 4
}
```

---

## 6. Deployment Diagram - `deployment_diagram.puml`

### Архитектурная цель
Показать физическое размещение компонентов системы, их взаимодействие с инфраструктурой и производительные характеристики.

### Связь с реализацией

#### Физическая архитектура

**Memory Tier (RAM)**
- **Архитектурные характеристики**: Размер 1-4GB, латентность <1ms, пропускная способность >10GB/s
- **Реализация в коде**:
```go
// Файл: blockstore/multitier.go
func (mtbs *multiTierBlockstore) initializeMemoryTier() {
    // Используем in-memory datastore для Memory Tier
    memoryDS := dssync.MutexWrap(ds.NewMapDatastore())
    
    mtbs.tiers[MemoryTier] = &tieredBlockstore{
        datastore: memoryDS,
        config: &TierConfig{
            MaxSize:            4 * 1024 * 1024 * 1024, // 4GB
            MaxItems:           1000000,                  // 1M блоков
            TTL:                time.Hour,                // 1 час TTL
            PromotionThreshold: 1,                        // Продвигаем после первого доступа
        },
    }
}
```

**SSD Tier**
- **Архитектурные характеристики**: Размер 100-500GB, латентность 1-10ms, пропускная способность 1-3GB/s
- **Реализация в коде**:
```go
// Файл: blockstore/multitier.go
func (mtbs *multiTierBlockstore) initializeSSDTier(ssdPath string) {
    // Используем файловую систему на SSD для SSD Tier
    ssdDS, err := leveldb.NewDatastore(ssdPath, nil)
    if err != nil {
        panic(err)
    }
    
    mtbs.tiers[SSDTier] = &tieredBlockstore{
        datastore: ssdDS,
        config: &TierConfig{
            MaxSize:            500 * 1024 * 1024 * 1024, // 500GB
            MaxItems:           10000000,                   // 10M блоков
            TTL:                24 * time.Hour,             // 24 часа TTL
            PromotionThreshold: 3,                          // Продвигаем после 3 доступов
        },
    }
}
```

**HDD Tier**
- **Архитектурные характеристики**: Размер 1-10TB, латентность 10-100ms, пропускная способность 100-200MB/s
- **Реализация в коде**:
```go
// Файл: blockstore/multitier.go
func (mtbs *multiTierBlockstore) initializeHDDTier(hddPath string) {
    // Используем файловую систему на HDD для долгосрочного хранения
    hddDS, err := flatfs.CreateOrOpen(hddPath, flatfs.NextToLast(2), false)
    if err != nil {
        panic(err)
    }
    
    mtbs.tiers[HDDTier] = &tieredBlockstore{
        datastore: hddDS,
        config: &TierConfig{
            MaxSize:            10 * 1024 * 1024 * 1024 * 1024, // 10TB
            MaxItems:           100000000,                        // 100M блоков
            TTL:                7 * 24 * time.Hour,               // 7 дней TTL
            PromotionThreshold: 5,                                // Продвигаем после 5 доступов
        },
    }
}
```

#### Мониторинг и метрики

**Prometheus Integration**
- **Архитектурная роль**: Сбор и хранение метрик производительности
- **Реализация в коде**:
```go
// Файл: blockstore/metrics.go
type Metrics struct {
    // MultiTier метрики
    CacheHits    *prometheus.CounterVec
    CacheMisses  *prometheus.CounterVec
    Promotions   *prometheus.CounterVec
    Demotions    *prometheus.CounterVec
    
    // Batch I/O метрики
    BatchesProcessed *prometheus.CounterVec
    BatchLatency     *prometheus.HistogramVec
    
    // Memory метрики
    MemoryUsage      *prometheus.GaugeVec
    PressureLevel    *prometheus.GaugeVec
}

func (mtbs *multiTierBlockstore) initMetrics() {
    mtbs.metrics = &Metrics{
        CacheHits: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "multitier_cache_hits_total",
                Help: "Total number of cache hits by tier",
            },
            []string{"tier"},
        ),
        // ... другие метрики
    }
    
    // Регистрируем метрики в Prometheus
    prometheus.MustRegister(mtbs.metrics.CacheHits)
}

func (mtbs *multiTierBlockstore) recordCacheHit(tier CacheTier) {
    mtbs.metrics.CacheHits.WithLabelValues(tier.String()).Inc()
}
```

#### Конфигурация для различных сценариев

**Высокая нагрузка (High Load Configuration)**
```go
// Файл: config/high_load.go
func HighLoadConfig() *OptimizedBlockstoreConfig {
    return &OptimizedBlockstoreConfig{
        MultiTier: &MultiTierConfig{
            Tiers: map[CacheTier]*TierConfig{
                MemoryTier: {
                    MaxSize:            8 * 1024 * 1024 * 1024, // 8GB для высокой нагрузки
                    MaxItems:           2000000,
                    PromotionThreshold: 1, // Агрессивное продвижение
                },
                SSDTier: {
                    MaxSize:            1024 * 1024 * 1024 * 1024, // 1TB SSD
                    MaxItems:           20000000,
                    PromotionThreshold: 2,
                },
            },
        },
        BatchIO: &BatchIOConfig{
            MaxBatchSize:         5000,              // Большие батчи
            BatchTimeout:         20 * time.Millisecond, // Быстрая обработка
            MaxConcurrentBatches: 50,                // Высокая конкурентность
        },
        Streaming: &StreamingConfig{
            LargeBlockThreshold: 512 * 1024,        // 512KB порог
            DefaultChunkSize:    128 * 1024,        // 128KB чанки
            EnableCompression:   true,
            CompressionLevel:    6,                  // Балансированное сжатие
        },
        Memory: &MemoryAwareConfig{
            MaxMemoryBytes: 16 * 1024 * 1024 * 1024, // 16GB лимит
            Thresholds: &MemoryThresholds{
                Low:      0.70, // 70%
                Medium:   0.80, // 80%
                High:     0.90, // 90%
                Critical: 0.95, // 95%
            },
        },
    }
}
```

---

## Заключение

Каждая диаграмма PlantUML в архитектуре Task 3 служит прямым мостом между архитектурным дизайном и фактической реализацией кода:

### 1. **Трассируемость от архитектуры к коду**
- Каждый компонент в диаграммах соответствует конкретному файлу Go
- Каждое взаимодействие реализовано через определенные методы и интерфейсы
- Каждая структура данных имеет прямое отображение в код

### 2. **Валидация архитектурных решений**
- Диаграммы позволяют проверить корректность архитектурных решений
- Sequence диаграммы показывают реальные потоки выполнения
- Deployment диаграмма отражает фактические требования к инфраструктуре

### 3. **Документация для разработчиков**
- Диаграммы служат живой документацией кода
- Новые разработчики могут быстро понять архитектуру
- Изменения в коде должны отражаться в диаграммах

### 4. **Основа для тестирования**
- Sequence диаграммы определяют сценарии для интеграционных тестов
- Component диаграммы показывают границы для unit тестов
- Deployment диаграмма определяет требования к производительным тестам

Эта архитектурная документация обеспечивает полную трассируемость от высокоуровневых требований до конкретной реализации кода, что критично для поддержания качества и эволюции системы.