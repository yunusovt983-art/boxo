package blockstore

import (
	"context"
	"testing"
	"time"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
)

// createTestBatchIOManager creates a test batch I/O manager
func createTestBatchIOManager(t *testing.T) (BatchIOManager, Blockstore) {
	ctx := context.Background()
	
	// Create in-memory datastore
	memoryDS := dssync.MutexWrap(ds.NewMapDatastore())
	bs := NewBlockstore(memoryDS)
	
	// Create batch I/O manager with test configuration
	config := &BatchIOConfig{
		MaxBatchSize:         10,
		BatchTimeout:         50 * time.Millisecond,
		MaxConcurrentBatches: 2,
		EnableTransactions:   true,
		RetryAttempts:        2,
		RetryDelay:           5 * time.Millisecond,
	}
	
	bim, err := NewBatchIOManager(ctx, bs, config)
	if err != nil {
		t.Fatalf("failed to create batch I/O manager: %v", err)
	}
	
	return bim, bs
}

func TestBatchIOManager_BatchPut(t *testing.T) {
	bim, _ := createTestBatchIOManager(t)
	defer bim.Close()
	
	ctx := context.Background()
	
	// Create test blocks
	var blocks []blocks.Block
	for i := 0; i < 5; i++ {
		data := []byte("test data " + string(rune('0'+i)))
		blocks = append(blocks, createTestBlock(data))
	}
	
	// Batch put
	err := bim.BatchPut(ctx, blocks)
	if err != nil {
		t.Fatalf("failed to batch put blocks: %v", err)
	}
	
	// Verify blocks were stored
	retrievedBlocks, err := bim.BatchGet(ctx, []cid.Cid{
		blocks[0].Cid(),
		blocks[1].Cid(),
		blocks[2].Cid(),
	})
	if err != nil {
		t.Fatalf("failed to batch get blocks: %v", err)
	}
	
	if len(retrievedBlocks) != 3 {
		t.Errorf("expected 3 blocks, got %d", len(retrievedBlocks))
	}
	
	// Verify block contents
	for i, block := range retrievedBlocks {
		if !blocksEqual(blocks[i], block) {
			t.Errorf("retrieved block %d does not match original", i)
		}
	}
}

func TestBatchIOManager_BatchGet(t *testing.T) {
	bim, bs := createTestBatchIOManager(t)
	defer bim.Close()
	
	ctx := context.Background()
	
	// Create and store test blocks directly in blockstore
	var blocks []blocks.Block
	var cids []cid.Cid
	
	for i := 0; i < 5; i++ {
		data := []byte("test data " + string(rune('0'+i)))
		block := createTestBlock(data)
		blocks = append(blocks, block)
		cids = append(cids, block.Cid())
		
		err := bs.Put(ctx, block)
		if err != nil {
			t.Fatalf("failed to put block %d: %v", i, err)
		}
	}
	
	// Batch get
	retrievedBlocks, err := bim.BatchGet(ctx, cids)
	if err != nil {
		t.Fatalf("failed to batch get blocks: %v", err)
	}
	
	if len(retrievedBlocks) != len(blocks) {
		t.Errorf("expected %d blocks, got %d", len(blocks), len(retrievedBlocks))
	}
	
	// Verify block contents
	for i, block := range retrievedBlocks {
		if !blocksEqual(blocks[i], block) {
			t.Errorf("retrieved block %d does not match original", i)
		}
	}
}

func TestBatchIOManager_BatchHas(t *testing.T) {
	bim, bs := createTestBatchIOManager(t)
	defer bim.Close()
	
	ctx := context.Background()
	
	// Create and store some test blocks
	var existingCIDs []cid.Cid
	var nonExistentCIDs []cid.Cid
	
	// Store some blocks
	for i := 0; i < 3; i++ {
		data := []byte("existing data " + string(rune('0'+i)))
		block := createTestBlock(data)
		existingCIDs = append(existingCIDs, block.Cid())
		
		err := bs.Put(ctx, block)
		if err != nil {
			t.Fatalf("failed to put block %d: %v", i, err)
		}
	}
	
	// Create some non-existent CIDs
	for i := 0; i < 2; i++ {
		data := []byte("non-existent data " + string(rune('0'+i)))
		block := createTestBlock(data)
		nonExistentCIDs = append(nonExistentCIDs, block.Cid())
	}
	
	// Combine CIDs for testing
	allCIDs := append(existingCIDs, nonExistentCIDs...)
	
	// Batch has
	results, err := bim.BatchHas(ctx, allCIDs)
	if err != nil {
		t.Fatalf("failed to batch has blocks: %v", err)
	}
	
	if len(results) != len(allCIDs) {
		t.Errorf("expected %d results, got %d", len(allCIDs), len(results))
	}
	
	// Verify results
	for i := 0; i < len(existingCIDs); i++ {
		if !results[i] {
			t.Errorf("existing block %d should be found", i)
		}
	}
	
	for i := len(existingCIDs); i < len(allCIDs); i++ {
		if results[i] {
			t.Errorf("non-existent block %d should not be found", i-len(existingCIDs))
		}
	}
}

func TestBatchIOManager_BatchDelete(t *testing.T) {
	bim, bs := createTestBatchIOManager(t)
	defer bim.Close()
	
	ctx := context.Background()
	
	// Create and store test blocks
	var blocks []blocks.Block
	var cids []cid.Cid
	
	for i := 0; i < 5; i++ {
		data := []byte("test data " + string(rune('0'+i)))
		block := createTestBlock(data)
		blocks = append(blocks, block)
		cids = append(cids, block.Cid())
		
		err := bs.Put(ctx, block)
		if err != nil {
			t.Fatalf("failed to put block %d: %v", i, err)
		}
	}
	
	// Verify blocks exist
	for i, c := range cids {
		has, err := bs.Has(ctx, c)
		if err != nil {
			t.Fatalf("failed to check block %d: %v", i, err)
		}
		if !has {
			t.Errorf("block %d should exist before deletion", i)
		}
	}
	
	// Batch delete first 3 blocks
	deleteList := cids[:3]
	err := bim.BatchDelete(ctx, deleteList)
	if err != nil {
		t.Fatalf("failed to batch delete blocks: %v", err)
	}
	
	// Verify deleted blocks don't exist
	for i, c := range deleteList {
		has, err := bs.Has(ctx, c)
		if err != nil {
			t.Fatalf("failed to check deleted block %d: %v", i, err)
		}
		if has {
			t.Errorf("deleted block %d should not exist", i)
		}
	}
	
	// Verify remaining blocks still exist
	for i := 3; i < len(cids); i++ {
		has, err := bs.Has(ctx, cids[i])
		if err != nil {
			t.Fatalf("failed to check remaining block %d: %v", i, err)
		}
		if !has {
			t.Errorf("remaining block %d should still exist", i)
		}
	}
}

func TestBatchIOManager_LargeBatch(t *testing.T) {
	bim, _ := createTestBatchIOManager(t)
	defer bim.Close()
	
	ctx := context.Background()
	
	// Create a large number of blocks (more than MaxBatchSize)
	var blocks []blocks.Block
	for i := 0; i < 25; i++ {
		data := []byte("large batch test data " + string(rune('0'+i%10)))
		blocks = append(blocks, createTestBlock(data))
	}
	
	// Batch put (should be split into multiple batches)
	err := bim.BatchPut(ctx, blocks)
	if err != nil {
		t.Fatalf("failed to batch put large number of blocks: %v", err)
	}
	
	// Verify all blocks were stored
	var cids []cid.Cid
	for _, block := range blocks {
		cids = append(cids, block.Cid())
	}
	
	retrievedBlocks, err := bim.BatchGet(ctx, cids)
	if err != nil {
		t.Fatalf("failed to batch get large number of blocks: %v", err)
	}
	
	if len(retrievedBlocks) != len(blocks) {
		t.Errorf("expected %d blocks, got %d", len(blocks), len(retrievedBlocks))
	}
}

func TestBatchIOManager_Flush(t *testing.T) {
	bim, _ := createTestBatchIOManager(t)
	defer bim.Close()
	
	ctx := context.Background()
	
	// Create test blocks
	var blocks []blocks.Block
	for i := 0; i < 3; i++ {
		data := []byte("flush test data " + string(rune('0'+i)))
		blocks = append(blocks, createTestBlock(data))
	}
	
	// Batch put
	err := bim.BatchPut(ctx, blocks)
	if err != nil {
		t.Fatalf("failed to batch put blocks: %v", err)
	}
	
	// Flush
	err = bim.Flush(ctx)
	if err != nil {
		t.Fatalf("failed to flush: %v", err)
	}
	
	// Verify blocks are accessible after flush
	var cids []cid.Cid
	for _, block := range blocks {
		cids = append(cids, block.Cid())
	}
	
	retrievedBlocks, err := bim.BatchGet(ctx, cids)
	if err != nil {
		t.Fatalf("failed to get blocks after flush: %v", err)
	}
	
	if len(retrievedBlocks) != len(blocks) {
		t.Errorf("expected %d blocks after flush, got %d", len(blocks), len(retrievedBlocks))
	}
}

func TestBatchIOManager_Stats(t *testing.T) {
	bim, _ := createTestBatchIOManager(t)
	defer bim.Close()
	
	ctx := context.Background()
	
	// Get initial stats
	initialStats := bim.GetStats()
	if initialStats.TotalBatches != 0 {
		t.Errorf("expected 0 initial batches, got %d", initialStats.TotalBatches)
	}
	
	// Create and put some blocks
	var blocks []blocks.Block
	for i := 0; i < 5; i++ {
		data := []byte("stats test data " + string(rune('0'+i)))
		blocks = append(blocks, createTestBlock(data))
	}
	
	err := bim.BatchPut(ctx, blocks)
	if err != nil {
		t.Fatalf("failed to batch put blocks: %v", err)
	}
	
	// Get updated stats
	stats := bim.GetStats()
	if stats.TotalBatches == 0 {
		t.Errorf("expected non-zero batches after operations")
	}
	
	if stats.TotalItems != int64(len(blocks)) {
		t.Errorf("expected %d total items, got %d", len(blocks), stats.TotalItems)
	}
	
	if stats.SuccessfulBatches == 0 {
		t.Errorf("expected non-zero successful batches")
	}
	
	if stats.AverageBatchSize == 0 {
		t.Errorf("expected non-zero average batch size")
	}
}

func TestBatchIOManager_ConcurrentOperations(t *testing.T) {
	bim, _ := createTestBatchIOManager(t)
	defer bim.Close()
	
	ctx := context.Background()
	
	// Run concurrent batch operations
	const numGoroutines = 5
	const blocksPerGoroutine = 10
	
	errChan := make(chan error, numGoroutines)
	
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			var blocks []blocks.Block
			for i := 0; i < blocksPerGoroutine; i++ {
				data := []byte("concurrent test data " + string(rune('0'+goroutineID)) + string(rune('0'+i)))
				blocks = append(blocks, createTestBlock(data))
			}
			
			err := bim.BatchPut(ctx, blocks)
			errChan <- err
		}(g)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		err := <-errChan
		if err != nil {
			t.Errorf("goroutine %d failed: %v", i, err)
		}
	}
	
	// Verify stats
	stats := bim.GetStats()
	expectedItems := int64(numGoroutines * blocksPerGoroutine)
	if stats.TotalItems != expectedItems {
		t.Errorf("expected %d total items, got %d", expectedItems, stats.TotalItems)
	}
}

func TestBatchIOManager_EmptyOperations(t *testing.T) {
	bim, _ := createTestBatchIOManager(t)
	defer bim.Close()
	
	ctx := context.Background()
	
	// Test empty batch put
	err := bim.BatchPut(ctx, nil)
	if err != nil {
		t.Errorf("empty batch put should not fail: %v", err)
	}
	
	// Test empty batch get
	blocks, err := bim.BatchGet(ctx, nil)
	if err != nil {
		t.Errorf("empty batch get should not fail: %v", err)
	}
	if len(blocks) != 0 {
		t.Errorf("empty batch get should return empty slice")
	}
	
	// Test empty batch has
	results, err := bim.BatchHas(ctx, nil)
	if err != nil {
		t.Errorf("empty batch has should not fail: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("empty batch has should return empty slice")
	}
	
	// Test empty batch delete
	err = bim.BatchDelete(ctx, nil)
	if err != nil {
		t.Errorf("empty batch delete should not fail: %v", err)
	}
}

// Benchmark tests
func BenchmarkBatchIOManager_BatchPut(b *testing.B) {
	ctx := context.Background()
	
	// Create in-memory datastore
	memoryDS := dssync.MutexWrap(ds.NewMapDatastore())
	bs := NewBlockstore(memoryDS)
	
	// Create batch I/O manager with no limits for benchmarking
	config := &BatchIOConfig{
		MaxBatchSize:         1000,
		BatchTimeout:         100 * time.Millisecond,
		MaxConcurrentBatches: 10,
		EnableTransactions:   true,
		RetryAttempts:        0, // No retries for benchmarking
		RetryDelay:           0,
	}
	
	bim, err := NewBatchIOManager(ctx, bs, config)
	if err != nil {
		b.Fatalf("failed to create batch I/O manager: %v", err)
	}
	defer bim.Close()
	
	// Pre-generate blocks
	var blocks []blocks.Block
	for i := 0; i < b.N; i++ {
		data := []byte("benchmark test data " + string(rune('0'+i%10)))
		blocks = append(blocks, createTestBlock(data))
	}
	
	b.ResetTimer()
	
	// Batch put all blocks at once
	err = bim.BatchPut(ctx, blocks)
	if err != nil {
		b.Fatalf("failed to batch put blocks: %v", err)
	}
}

func BenchmarkBatchIOManager_BatchGet(b *testing.B) {
	ctx := context.Background()
	
	// Create in-memory datastore
	memoryDS := dssync.MutexWrap(ds.NewMapDatastore())
	bs := NewBlockstore(memoryDS)
	
	// Create batch I/O manager
	config := DefaultBatchIOConfig()
	config.RetryAttempts = 0 // No retries for benchmarking
	
	bim, err := NewBatchIOManager(ctx, bs, config)
	if err != nil {
		b.Fatalf("failed to create batch I/O manager: %v", err)
	}
	defer bim.Close()
	
	// Pre-generate and store blocks
	var cids []cid.Cid
	for i := 0; i < b.N; i++ {
		data := []byte("benchmark test data " + string(rune('0'+i%10)))
		block := createTestBlock(data)
		cids = append(cids, block.Cid())
		
		err := bs.Put(ctx, block)
		if err != nil {
			b.Fatalf("failed to put block: %v", err)
		}
	}
	
	b.ResetTimer()
	
	// Batch get all blocks at once
	_, err = bim.BatchGet(ctx, cids)
	if err != nil {
		b.Fatalf("failed to batch get blocks: %v", err)
	}
}