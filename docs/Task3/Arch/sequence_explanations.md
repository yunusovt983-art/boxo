# Sequence Diagrams - Подробные объяснения

## Обзор

Sequence диаграммы показывают временную последовательность взаимодействий между компонентами системы. Каждая диаграмма демонстрирует конкретные сценарии использования и их реализацию в коде.

## 1. MultiTier Operations (`sequence_multitier_operations.puml`)

### Архитектурная цель
Показать как работает многоуровневое кэширование с автоматическим продвижением блоков и реакцией на давление памяти.

### Ключевые сценарии

#### Сценарий 1: Get с автопродвижением
**Поток выполнения:**
1. Client → MultiTierBlockstore: Get(ctx, cid)
2. Поиск по уровням: Memory → SSD → HDD  
3. Блок найден на HDD
4. Обновление статистики доступа
5. Проверка порога продвижения
6. Автоматическое продвижение на SSD

**Реализация:**
```go
// blockstore/multitier.go
func (mtbs *multiTierBlockstore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
    key := c.String()
    
    // Поиск по уровням от быстрого к медленному
    for _, tier := range []CacheTier{MemoryTier, SSDTier, HDDTier} {
        if block, err := mtbs.tiers[tier].Get(ctx, c); err == nil {
            mtbs.updateAccessInfo(key, tier, len(block.RawData()))
            
            // Асинхронное продвижение если нужно
            if tier != MemoryTier {
                go mtbs.considerPromotion(ctx, c, key)
            }
            
            return block, nil
        }
    }
    return nil, ErrNotFound
}
```

## 2. Batch Operations (`sequence_batch_operations.puml`)

### Архитектурная цель  
Демонстрировать асинхронную обработку батчевых операций с группировкой и retry механизмами.

### Ключевые сценарии

#### Сценарий: Батчевая операция Put
**Поток выполнения:**
1. Client отправляет 1000 блоков
2. Разбиение на батчи по 100 блоков
3. Постановка в очередь worker'у
4. Группировка по таймауту
5. Транзакционное выполнение
6. Retry при ошибках

**Реализация:**
```go
// blockstore/batch_io.go  
func (bim *batchIOManager) BatchPut(ctx context.Context, blocks []blocks.Block) error {
    for i := 0; i < len(blocks); i += bim.config.MaxBatchSize {
        batch := blocks[i:min(i+bim.config.MaxBatchSize, len(blocks))]
        
        op := &BatchOperation{
            Type: BatchOpPut,
            Blocks: batch,
            Results: make(chan BatchResult, 1),
        }
        
        bim.putQueue <- op
        result := <-op.Results
        if result.Error != nil {
            return result.Error
        }
    }
    return nil
}
```

## 3. Streaming Operations (`sequence_streaming_operations.puml`)

### Архитектурная цель
Показать потоковую обработку больших блоков с чанкингом и сжатием.

## 4. Memory Management (`sequence_memory_management.puml`)

### Архитектурная цель
Демонстрировать полный цикл управления памятью от мониторинга до восстановления.