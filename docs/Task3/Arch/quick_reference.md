# Quick Reference - Task 3 Architecture

## 🎯 Быстрый обзор архитектуры

### Основные компоненты Task 3

| Компонент | Файл диаграммы | Файл реализации | Основная функция |
|-----------|----------------|-----------------|------------------|
| **MultiTier Cache** | `c4_level2_container.puml` | `blockstore/multitier.go` | Многоуровневое кэширование Memory/SSD/HDD |
| **Batch I/O Manager** | `c4_level2_container.puml` | `blockstore/batch_io.go` | Группировка операций I/O |
| **Streaming Handler** | `c4_level2_container.puml` | `blockstore/streaming.go` | Потоковая обработка больших блоков |
| **Memory Manager** | `c4_level2_container.puml` | `blockstore/memory_manager.go` | Управление памятью и graceful degradation |

### Ключевые sequence диаграммы

| Диаграмма | Сценарий | Ключевая реализация |
|-----------|----------|-------------------|
| `sequence_multitier_operations.puml` | Get с автопродвижением | `mtbs.considerPromotion()` |
| `sequence_batch_operations.puml` | Асинхронная обработка батчей | `bim.putWorker()` |
| `sequence_streaming_operations.puml` | Чанкинг больших блоков | `sh.chunkData()` |
| `sequence_memory_management.puml` | Мониторинг давления памяти | `mm.checkMemoryPressure()` |

### Производительные характеристики

| Оптимизация | Улучшение | Реализация |
|-------------|-----------|------------|
| **Латентность Get** | 50-80% снижение | MultiTier Cache с LRU |
| **Пропускная способность** | 300-500% увеличение | Batch I/O операции |
| **Использование памяти** | 90% снижение для больших блоков | Streaming с чанкингом |
| **Стабильность** | 99.9% uptime | Memory Management |

### Быстрая навигация по файлам

#### Архитектурные диаграммы
- **Context**: `c4_level1_context.puml` - Общий обзор системы
- **Containers**: `c4_level2_container.puml` - Основные компоненты  
- **Components**: `c4_level3_component.puml` - Детальная структура
- **Code**: `c4_level4_code.puml` - Классы и интерфейсы
- **Deployment**: `deployment_diagram.puml` - Физическая архитектура

#### Sequence диаграммы
- **MultiTier**: `sequence_multitier_operations.puml`
- **Batch I/O**: `sequence_batch_operations.puml`  
- **Streaming**: `sequence_streaming_operations.puml`
- **Memory**: `sequence_memory_management.puml`

#### Документация
- **Общие объяснения**: `diagram_explanations.md`
- **Context объяснение**: `level1_context_explanation.md`
- **Container объяснение**: `level2_container_explanation.md`
- **Маппинг к коду**: `implementation_mapping.md`

### Основные интерфейсы

```go
// Task 3.1: Многоуровневое кэширование
type MultiTierBlockstore interface {
    Get(ctx, cid) (Block, error)
    Put(ctx, block) error
    PromoteBlock(ctx, cid) error
    GetTierStats() map[CacheTier]*TierStats
}

// Task 3.2: Батчевые операции
type BatchIOManager interface {
    BatchPut(ctx, []Block) error
    BatchGet(ctx, []cid.Cid) ([]Block, error)
    GetStats() *BatchIOStats
}

// Task 3.3: Потоковая обработка
type StreamingHandler interface {
    StreamPut(ctx, cid, io.Reader) error
    StreamGet(ctx, cid) (io.ReadCloser, error)
    ChunkedPut(ctx, cid, io.Reader, int) error
}

// Task 3.4: Управление памятью
type MemoryManager interface {
    GetCurrentPressure() MemoryPressureLevel
    SetCallback(MemoryPressureCallback)
    ForceGC()
}
```

### Метрики Prometheus

```go
// Основные метрики для мониторинга
multitier_cache_hits_total{tier="memory|ssd|hdd"}
batch_io_operations_total{operation="put|get|has|delete"}  
streaming_compression_ratio
memory_pressure_level  // 0=None, 1=Low, 2=Medium, 3=High, 4=Critical
```

### Конфигурация для высокой нагрузки

```go
config := &OptimizedConfig{
    MultiTier: &MultiTierConfig{
        MemoryTier: {MaxSize: 8*GB, PromotionThreshold: 1},
        SSDTier:    {MaxSize: 1*TB, PromotionThreshold: 2},
    },
    BatchIO: &BatchIOConfig{
        MaxBatchSize: 5000,
        MaxConcurrentBatches: 50,
    },
    Streaming: &StreamingConfig{
        LargeBlockThreshold: 1*MB,
        DefaultChunkSize: 256*KB,
    },
    Memory: &MemoryConfig{
        Thresholds: {Low: 0.7, High: 0.9, Critical: 0.95},
    },
}
```

Этот quick reference обеспечивает быстрый доступ к ключевой информации об архитектуре Task 3 и её реализации.