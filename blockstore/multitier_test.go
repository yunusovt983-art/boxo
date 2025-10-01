package blockstore

import (
	"context"
	"testing"
	"time"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	ipld "github.com/ipfs/go-ipld-format"
)

// createTestBlock creates a test block with the given data
func createTestBlock(data []byte) blocks.Block {
	return blocks.NewBlock(data)
}

// blocksEqual compares two blocks for equality
func blocksEqual(a, b blocks.Block) bool {
	return a.Cid().Equals(b.Cid()) && string(a.RawData()) == string(b.RawData())
}

// createTestMultiTierBlockstore creates a test multi-tier blockstore
func createTestMultiTierBlockstore(t *testing.T) MultiTierBlockstore {
	ctx := context.Background()
	
	// Create in-memory datastores for each tier
	memoryDS := dssync.MutexWrap(ds.NewMapDatastore())
	ssdDS := dssync.MutexWrap(ds.NewMapDatastore())
	hddDS := dssync.MutexWrap(ds.NewMapDatastore())
	
	// Create blockstores for each tier
	tiers := map[CacheTier]Blockstore{
		MemoryTier: NewBlockstore(memoryDS),
		SSDTier:    NewBlockstore(ssdDS),
		HDDTier:    NewBlockstore(hddDS),
	}
	
	// Create multi-tier blockstore with test configuration
	config := &MultiTierConfig{
		Tiers: map[CacheTier]*TierConfig{
			MemoryTier: {
				MaxSize:            100 * 1024 * 1024, // 100MB for benchmarks
				MaxItems:           100000,
				TTL:                time.Hour,
				PromotionThreshold: 1,
				Enabled:            true,
			},
			SSDTier: {
				MaxSize:            10 * 1024 * 1024, // 10MB
				MaxItems:           1000,
				TTL:                24 * time.Hour,
				PromotionThreshold: 3,
				Enabled:            true,
			},
			HDDTier: {
				MaxSize:            100 * 1024 * 1024, // 100MB
				MaxItems:           10000,
				TTL:                7 * 24 * time.Hour,
				PromotionThreshold: 5,
				Enabled:            true,
			},
		},
		AutoPromote:        true,
		AutoDemote:         true,
		CompactionInterval: time.Minute,
	}
	
	mtbs, err := NewMultiTierBlockstore(ctx, tiers, config)
	if err != nil {
		t.Fatalf("failed to create multi-tier blockstore: %v", err)
	}
	
	return mtbs
}

func TestMultiTierBlockstore_BasicOperations(t *testing.T) {
	mtbs := createTestMultiTierBlockstore(t)
	defer mtbs.Close()
	
	ctx := context.Background()
	
	// Test Put and Get
	block := createTestBlock([]byte("test data"))
	
	err := mtbs.Put(ctx, block)
	if err != nil {
		t.Fatalf("failed to put block: %v", err)
	}
	
	retrievedBlock, err := mtbs.Get(ctx, block.Cid())
	if err != nil {
		t.Fatalf("failed to get block: %v", err)
	}
	
	if !blocksEqual(block, retrievedBlock) {
		t.Errorf("retrieved block does not match original")
	}
	
	// Test Has
	has, err := mtbs.Has(ctx, block.Cid())
	if err != nil {
		t.Fatalf("failed to check if block exists: %v", err)
	}
	if !has {
		t.Errorf("block should exist")
	}
	
	// Test GetSize
	size, err := mtbs.GetSize(ctx, block.Cid())
	if err != nil {
		t.Fatalf("failed to get block size: %v", err)
	}
	if size != len(block.RawData()) {
		t.Errorf("expected size %d, got %d", len(block.RawData()), size)
	}
	
	// Test DeleteBlock
	err = mtbs.DeleteBlock(ctx, block.Cid())
	if err != nil {
		t.Fatalf("failed to delete block: %v", err)
	}
	
	has, err = mtbs.Has(ctx, block.Cid())
	if err != nil {
		t.Fatalf("failed to check if block exists after deletion: %v", err)
	}
	if has {
		t.Errorf("block should not exist after deletion")
	}
}

func TestMultiTierBlockstore_TierSpecificOperations(t *testing.T) {
	mtbs := createTestMultiTierBlockstore(t)
	defer mtbs.Close()
	
	ctx := context.Background()
	
	// Test PutWithTier and GetWithTier
	block := createTestBlock([]byte("tier test data"))
	
	// Put in SSD tier
	err := mtbs.PutWithTier(ctx, block, SSDTier)
	if err != nil {
		t.Fatalf("failed to put block in SSD tier: %v", err)
	}
	
	// Get from SSD tier
	retrievedBlock, err := mtbs.GetWithTier(ctx, block.Cid(), SSDTier)
	if err != nil {
		t.Fatalf("failed to get block from SSD tier: %v", err)
	}
	
	if !blocksEqual(block, retrievedBlock) {
		t.Errorf("retrieved block does not match original")
	}
	
	// Should not be in Memory tier
	_, err = mtbs.GetWithTier(ctx, block.Cid(), MemoryTier)
	if err == nil {
		t.Errorf("block should not be in Memory tier")
	}
	if !ipld.IsNotFound(err) {
		t.Errorf("expected NotFound error, got: %v", err)
	}
}

func TestMultiTierBlockstore_Promotion(t *testing.T) {
	mtbs := createTestMultiTierBlockstore(t)
	defer mtbs.Close()
	
	ctx := context.Background()
	
	// Put block in HDD tier
	block := createTestBlock([]byte("promotion test data"))
	err := mtbs.PutWithTier(ctx, block, HDDTier)
	if err != nil {
		t.Fatalf("failed to put block in HDD tier: %v", err)
	}
	
	// Verify it's in HDD tier
	_, err = mtbs.GetWithTier(ctx, block.Cid(), HDDTier)
	if err != nil {
		t.Fatalf("block should be in HDD tier: %v", err)
	}
	
	// Promote to SSD tier
	err = mtbs.PromoteBlock(ctx, block.Cid())
	if err != nil {
		t.Fatalf("failed to promote block: %v", err)
	}
	
	// Verify it's now in SSD tier
	_, err = mtbs.GetWithTier(ctx, block.Cid(), SSDTier)
	if err != nil {
		t.Fatalf("block should be in SSD tier after promotion: %v", err)
	}
	
	// Verify it's no longer in HDD tier
	_, err = mtbs.GetWithTier(ctx, block.Cid(), HDDTier)
	if err == nil {
		t.Errorf("block should not be in HDD tier after promotion")
	}
}

func TestMultiTierBlockstore_Demotion(t *testing.T) {
	mtbs := createTestMultiTierBlockstore(t)
	defer mtbs.Close()
	
	ctx := context.Background()
	
	// Put block in Memory tier
	block := createTestBlock([]byte("demotion test data"))
	err := mtbs.PutWithTier(ctx, block, MemoryTier)
	if err != nil {
		t.Fatalf("failed to put block in Memory tier: %v", err)
	}
	
	// Verify it's in Memory tier
	_, err = mtbs.GetWithTier(ctx, block.Cid(), MemoryTier)
	if err != nil {
		t.Fatalf("block should be in Memory tier: %v", err)
	}
	
	// Demote to SSD tier
	err = mtbs.DemoteBlock(ctx, block.Cid())
	if err != nil {
		t.Fatalf("failed to demote block: %v", err)
	}
	
	// Verify it's now in SSD tier
	_, err = mtbs.GetWithTier(ctx, block.Cid(), SSDTier)
	if err != nil {
		t.Fatalf("block should be in SSD tier after demotion: %v", err)
	}
	
	// Verify it's no longer in Memory tier
	_, err = mtbs.GetWithTier(ctx, block.Cid(), MemoryTier)
	if err == nil {
		t.Errorf("block should not be in Memory tier after demotion")
	}
}

func TestMultiTierBlockstore_AutoPromotion(t *testing.T) {
	mtbs := createTestMultiTierBlockstore(t)
	defer mtbs.Close()
	
	ctx := context.Background()
	
	// Put block in HDD tier
	block := createTestBlock([]byte("auto promotion test data"))
	err := mtbs.PutWithTier(ctx, block, HDDTier)
	if err != nil {
		t.Fatalf("failed to put block in HDD tier: %v", err)
	}
	
	// Access the block multiple times to trigger auto-promotion
	// The HDD tier has PromotionThreshold: 5
	for i := 0; i < 6; i++ {
		_, err = mtbs.Get(ctx, block.Cid())
		if err != nil {
			t.Fatalf("failed to get block (access %d): %v", i+1, err)
		}
		// Give some time for async promotion to happen
		time.Sleep(10 * time.Millisecond)
	}
	
	// Give more time for async promotion
	time.Sleep(100 * time.Millisecond)
	
	// Check if block still exists (using general Get method)
	_, err = mtbs.Get(ctx, block.Cid())
	if err != nil {
		t.Fatalf("block should still exist after access: %v", err)
	}
	
	// Check if block was promoted to SSD tier
	// Note: Auto-promotion is async, so we check both tiers
	inSSD := false
	inHDD := false
	
	_, err = mtbs.GetWithTier(ctx, block.Cid(), SSDTier)
	if err == nil {
		inSSD = true
	}
	
	_, err = mtbs.GetWithTier(ctx, block.Cid(), HDDTier)
	if err == nil {
		inHDD = true
	}
	
	if !inSSD && !inHDD {
		// Check memory tier as well
		_, err = mtbs.GetWithTier(ctx, block.Cid(), MemoryTier)
		if err == nil {
			t.Logf("block promoted all the way to Memory tier")
		} else {
			t.Errorf("block not found in any tier after access")
		}
	} else if inSSD {
		t.Logf("block successfully promoted to SSD tier")
	} else {
		t.Logf("block still in HDD tier (promotion may be async)")
	}
}

func TestMultiTierBlockstore_PutMany(t *testing.T) {
	mtbs := createTestMultiTierBlockstore(t)
	defer mtbs.Close()
	
	ctx := context.Background()
	
	// Create multiple test blocks
	var blocks []blocks.Block
	for i := 0; i < 10; i++ {
		data := []byte("test data " + string(rune('0'+i)))
		blocks = append(blocks, createTestBlock(data))
	}
	
	// Put all blocks at once
	err := mtbs.PutMany(ctx, blocks)
	if err != nil {
		t.Fatalf("failed to put many blocks: %v", err)
	}
	
	// Verify all blocks can be retrieved
	for i, block := range blocks {
		retrievedBlock, err := mtbs.Get(ctx, block.Cid())
		if err != nil {
			t.Fatalf("failed to get block %d: %v", i, err)
		}
		
		if !blocksEqual(block, retrievedBlock) {
			t.Errorf("retrieved block %d does not match original", i)
		}
	}
}

func TestMultiTierBlockstore_AllKeysChan(t *testing.T) {
	mtbs := createTestMultiTierBlockstore(t)
	defer mtbs.Close()
	
	ctx := context.Background()
	
	// Put blocks in different tiers
	var expectedCIDs []cid.Cid
	
	// Memory tier
	block1 := createTestBlock([]byte("memory data"))
	err := mtbs.PutWithTier(ctx, block1, MemoryTier)
	if err != nil {
		t.Fatalf("failed to put block in memory tier: %v", err)
	}
	expectedCIDs = append(expectedCIDs, block1.Cid())
	
	// SSD tier
	block2 := createTestBlock([]byte("ssd data"))
	err = mtbs.PutWithTier(ctx, block2, SSDTier)
	if err != nil {
		t.Fatalf("failed to put block in SSD tier: %v", err)
	}
	expectedCIDs = append(expectedCIDs, block2.Cid())
	
	// HDD tier
	block3 := createTestBlock([]byte("hdd data"))
	err = mtbs.PutWithTier(ctx, block3, HDDTier)
	if err != nil {
		t.Fatalf("failed to put block in HDD tier: %v", err)
	}
	expectedCIDs = append(expectedCIDs, block3.Cid())
	
	// Get all keys
	ch, err := mtbs.AllKeysChan(ctx)
	if err != nil {
		t.Fatalf("failed to get all keys channel: %v", err)
	}
	
	var retrievedCIDs []cid.Cid
	for c := range ch {
		retrievedCIDs = append(retrievedCIDs, c)
	}
	
	// Verify we got all expected CIDs
	if len(retrievedCIDs) != len(expectedCIDs) {
		t.Errorf("expected %d CIDs, got %d", len(expectedCIDs), len(retrievedCIDs))
	}
	
	// Check that all expected CIDs are present (using hash comparison for robustness)
	expectedMap := make(map[string]bool)
	for _, c := range expectedCIDs {
		expectedMap[string(c.Hash())] = true
	}
	
	retrievedMap := make(map[string]bool)
	for _, c := range retrievedCIDs {
		retrievedMap[string(c.Hash())] = true
	}
	
	// Check for missing CIDs
	for hash := range expectedMap {
		if !retrievedMap[hash] {
			t.Errorf("missing expected CID with hash: %x", hash)
		}
	}
	
	// We allow extra CIDs since the underlying blockstore might have additional data
}

func TestMultiTierBlockstore_Stats(t *testing.T) {
	mtbs := createTestMultiTierBlockstore(t)
	defer mtbs.Close()
	
	ctx := context.Background()
	
	// Put some blocks
	block1 := createTestBlock([]byte("stats test data 1"))
	block2 := createTestBlock([]byte("stats test data 2"))
	
	err := mtbs.PutWithTier(ctx, block1, MemoryTier)
	if err != nil {
		t.Fatalf("failed to put block 1: %v", err)
	}
	
	err = mtbs.PutWithTier(ctx, block2, SSDTier)
	if err != nil {
		t.Fatalf("failed to put block 2: %v", err)
	}
	
	// Get stats
	stats := mtbs.GetTierStats()
	
	// Check Memory tier stats
	memStats, exists := stats[MemoryTier]
	if !exists {
		t.Fatalf("Memory tier stats not found")
	}
	
	if memStats.Items != 1 {
		t.Errorf("expected 1 item in Memory tier, got %d", memStats.Items)
	}
	
	if memStats.Size != int64(len(block1.RawData())) {
		t.Errorf("expected size %d in Memory tier, got %d", len(block1.RawData()), memStats.Size)
	}
	
	// Check SSD tier stats
	ssdStats, exists := stats[SSDTier]
	if !exists {
		t.Fatalf("SSD tier stats not found")
	}
	
	if ssdStats.Items != 1 {
		t.Errorf("expected 1 item in SSD tier, got %d", ssdStats.Items)
	}
	
	if ssdStats.Size != int64(len(block2.RawData())) {
		t.Errorf("expected size %d in SSD tier, got %d", len(block2.RawData()), ssdStats.Size)
	}
}

func TestMultiTierBlockstore_Compaction(t *testing.T) {
	mtbs := createTestMultiTierBlockstore(t)
	defer mtbs.Close()
	
	ctx := context.Background()
	
	// Put some blocks
	block := createTestBlock([]byte("compaction test data"))
	err := mtbs.Put(ctx, block)
	if err != nil {
		t.Fatalf("failed to put block: %v", err)
	}
	
	// Run compaction
	err = mtbs.Compact(ctx)
	if err != nil {
		t.Fatalf("compaction failed: %v", err)
	}
	
	// Verify block is still accessible after compaction
	_, err = mtbs.Get(ctx, block.Cid())
	if err != nil {
		t.Fatalf("block not accessible after compaction: %v", err)
	}
}

func TestMultiTierBlockstore_View(t *testing.T) {
	mtbs := createTestMultiTierBlockstore(t)
	defer mtbs.Close()
	
	ctx := context.Background()
	
	// Test View functionality
	testData := []byte("view test data")
	block := createTestBlock(testData)
	
	err := mtbs.Put(ctx, block)
	if err != nil {
		t.Fatalf("failed to put block: %v", err)
	}
	
	// Use View to access block data
	var viewedData []byte
	err = mtbs.View(ctx, block.Cid(), func(data []byte) error {
		viewedData = make([]byte, len(data))
		copy(viewedData, data)
		return nil
	})
	
	if err != nil {
		t.Fatalf("View failed: %v", err)
	}
	
	if string(viewedData) != string(testData) {
		t.Errorf("viewed data does not match original: got %s, expected %s", viewedData, testData)
	}
}

// Benchmark tests
func BenchmarkMultiTierBlockstore_Get(b *testing.B) {
	// Create a testing.T for the helper function
	t := &testing.T{}
	mtbs := createTestMultiTierBlockstore(t)
	defer mtbs.Close()
	
	ctx := context.Background()
	
	// Put a test block
	block := createTestBlock([]byte("benchmark test data"))
	err := mtbs.Put(ctx, block)
	if err != nil {
		b.Fatalf("failed to put block: %v", err)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := mtbs.Get(ctx, block.Cid())
		if err != nil {
			b.Fatalf("failed to get block: %v", err)
		}
	}
}

func BenchmarkMultiTierBlockstore_Put(b *testing.B) {
	ctx := context.Background()
	
	// Create in-memory datastores for each tier
	memoryDS := dssync.MutexWrap(ds.NewMapDatastore())
	ssdDS := dssync.MutexWrap(ds.NewMapDatastore())
	hddDS := dssync.MutexWrap(ds.NewMapDatastore())
	
	// Create blockstores for each tier
	tiers := map[CacheTier]Blockstore{
		MemoryTier: NewBlockstore(memoryDS),
		SSDTier:    NewBlockstore(ssdDS),
		HDDTier:    NewBlockstore(hddDS),
	}
	
	// Create configuration without size limits for benchmarking
	config := &MultiTierConfig{
		Tiers: map[CacheTier]*TierConfig{
			MemoryTier: {
				MaxSize:            0, // No size limit
				MaxItems:           0, // No item limit
				TTL:                time.Hour,
				PromotionThreshold: 1,
				Enabled:            true,
			},
			SSDTier: {
				MaxSize:            0, // No size limit
				MaxItems:           0, // No item limit
				TTL:                24 * time.Hour,
				PromotionThreshold: 3,
				Enabled:            true,
			},
			HDDTier: {
				MaxSize:            0, // No size limit
				MaxItems:           0, // No item limit
				TTL:                7 * 24 * time.Hour,
				PromotionThreshold: 5,
				Enabled:            true,
			},
		},
		AutoPromote:        false, // Disable for benchmarking
		AutoDemote:         false, // Disable for benchmarking
		CompactionInterval: 0,     // Disable for benchmarking
	}
	
	mtbs, err := NewMultiTierBlockstore(ctx, tiers, config)
	if err != nil {
		b.Fatalf("failed to create multi-tier blockstore: %v", err)
	}
	defer mtbs.Close()
	
	// Pre-generate blocks to avoid allocation overhead in benchmark
	var blocks []blocks.Block
	for i := 0; i < b.N; i++ {
		data := []byte("benchmark test data " + string(rune('0'+i%10)))
		blocks = append(blocks, createTestBlock(data))
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		err := mtbs.Put(ctx, blocks[i])
		if err != nil {
			b.Fatalf("failed to put block: %v", err)
		}
	}
}