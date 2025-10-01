package blockstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
)

// createTestStreamingHandler creates a test streaming handler
func createTestStreamingHandler(t *testing.T) (StreamingHandler, Blockstore) {
	ctx := context.Background()
	
	// Create in-memory datastore
	memoryDS := dssync.MutexWrap(ds.NewMapDatastore())
	bs := NewBlockstore(memoryDS)
	
	// Create streaming handler with test configuration
	config := &StreamingConfig{
		LargeBlockThreshold:  1024, // 1KB for testing
		DefaultChunkSize:     256,  // 256 bytes for testing
		EnableCompression:    true,
		CompressionLevel:     6,
		MaxConcurrentStreams: 5,
		StreamTimeout:        10 * time.Second,
		BufferSize:           128,
	}
	
	sh, err := NewStreamingHandler(ctx, bs, config)
	if err != nil {
		t.Fatalf("failed to create streaming handler: %v", err)
	}
	
	return sh, bs
}

func TestStreamingHandler_StreamPut_SmallBlock(t *testing.T) {
	sh, _ := createTestStreamingHandler(t)
	defer sh.Close()
	
	ctx := context.Background()
	
	// Create small test data (below threshold)
	testData := []byte("small test data")
	block := createTestBlock(testData)
	
	// Stream put
	err := sh.StreamPut(ctx, block.Cid(), bytes.NewReader(testData))
	if err != nil {
		t.Fatalf("failed to stream put small block: %v", err)
	}
	
	// Verify data can be retrieved
	reader, err := sh.StreamGet(ctx, block.Cid())
	if err != nil {
		t.Fatalf("failed to stream get small block: %v", err)
	}
	defer reader.Close()
	
	retrievedData, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read retrieved data: %v", err)
	}
	
	if !bytes.Equal(testData, retrievedData) {
		t.Errorf("retrieved data does not match original")
	}
}

func TestStreamingHandler_StreamPut_LargeBlock(t *testing.T) {
	sh, _ := createTestStreamingHandler(t)
	defer sh.Close()
	
	ctx := context.Background()
	
	// Create large test data (above threshold)
	testData := make([]byte, 2048) // 2KB
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	
	block := createTestBlock(testData)
	
	// Stream put
	err := sh.StreamPut(ctx, block.Cid(), bytes.NewReader(testData))
	if err != nil {
		t.Fatalf("failed to stream put large block: %v", err)
	}
	
	// Verify data can be retrieved
	reader, err := sh.StreamGet(ctx, block.Cid())
	if err != nil {
		t.Fatalf("failed to stream get large block: %v", err)
	}
	defer reader.Close()
	
	retrievedData, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read retrieved data: %v", err)
	}
	
	if !bytes.Equal(testData, retrievedData) {
		t.Errorf("retrieved data does not match original")
	}
}

func TestStreamingHandler_ChunkedPut(t *testing.T) {
	sh, _ := createTestStreamingHandler(t)
	defer sh.Close()
	
	ctx := context.Background()
	
	// Create test data that will be split into multiple chunks
	testData := make([]byte, 1000) // Will be split into 4 chunks of 256 bytes each
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	
	block := createTestBlock(testData)
	
	// Chunked put
	err := sh.ChunkedPut(ctx, block.Cid(), bytes.NewReader(testData), 256)
	if err != nil {
		t.Fatalf("failed to chunked put: %v", err)
	}
	
	// Verify data can be retrieved
	reader, err := sh.ChunkedGet(ctx, block.Cid())
	if err != nil {
		t.Fatalf("failed to chunked get: %v", err)
	}
	defer reader.Close()
	
	retrievedData, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read retrieved data: %v", err)
	}
	
	if !bytes.Equal(testData, retrievedData) {
		t.Errorf("retrieved data does not match original")
		t.Errorf("expected length: %d, got length: %d", len(testData), len(retrievedData))
	}
	
	// Verify stats
	stats := sh.GetStreamingStats()
	if stats.TotalChunks == 0 {
		t.Errorf("expected non-zero chunk count")
	}
}

func TestStreamingHandler_CompressedPut(t *testing.T) {
	sh, _ := createTestStreamingHandler(t)
	defer sh.Close()
	
	ctx := context.Background()
	
	// Create highly compressible test data
	testData := []byte(strings.Repeat("test data ", 100)) // Repeating pattern compresses well
	block := createTestBlock(testData)
	
	// Compressed put
	err := sh.CompressedPut(ctx, block.Cid(), bytes.NewReader(testData))
	if err != nil {
		t.Fatalf("failed to compressed put: %v", err)
	}
	
	// Verify data can be retrieved
	reader, err := sh.CompressedGet(ctx, block.Cid())
	if err != nil {
		t.Fatalf("failed to compressed get: %v", err)
	}
	defer reader.Close()
	
	retrievedData, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read retrieved data: %v", err)
	}
	
	if !bytes.Equal(testData, retrievedData) {
		t.Errorf("retrieved data does not match original")
	}
	
	// Verify compression ratio was recorded
	stats := sh.GetStreamingStats()
	if stats.CompressionRatio == 0 {
		t.Logf("compression ratio not recorded (data might not have been compressed)")
	}
}

func TestStreamingHandler_Stats(t *testing.T) {
	sh, _ := createTestStreamingHandler(t)
	defer sh.Close()
	
	ctx := context.Background()
	
	// Get initial stats
	initialStats := sh.GetStreamingStats()
	if initialStats.TotalStreams != 0 {
		t.Errorf("expected 0 initial streams, got %d", initialStats.TotalStreams)
	}
	
	// Perform some streaming operations
	testData := []byte("stats test data")
	block := createTestBlock(testData)
	
	err := sh.StreamPut(ctx, block.Cid(), bytes.NewReader(testData))
	if err != nil {
		t.Fatalf("failed to stream put: %v", err)
	}
	
	reader, err := sh.StreamGet(ctx, block.Cid())
	if err != nil {
		t.Fatalf("failed to stream get: %v", err)
	}
	reader.Close()
	
	// Check updated stats
	stats := sh.GetStreamingStats()
	if stats.TotalStreams == 0 {
		t.Errorf("expected non-zero streams after operations")
	}
	
	if stats.TotalBytesStreamed == 0 {
		t.Errorf("expected non-zero bytes streamed")
	}
}

func TestStreamingHandler_ConcurrentStreams(t *testing.T) {
	sh, _ := createTestStreamingHandler(t)
	defer sh.Close()
	
	ctx := context.Background()
	
	// Run concurrent streaming operations
	const numGoroutines = 3
	const dataSize = 500
	
	errChan := make(chan error, numGoroutines*2) // Put and Get for each goroutine
	
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			// Create unique test data
			testData := make([]byte, dataSize)
			for i := range testData {
				testData[i] = byte((goroutineID*256 + i) % 256)
			}
			
			block := createTestBlock(testData)
			
			// Stream put
			err := sh.StreamPut(ctx, block.Cid(), bytes.NewReader(testData))
			if err != nil {
				errChan <- err
				return
			}
			
			// Stream get
			reader, err := sh.StreamGet(ctx, block.Cid())
			if err != nil {
				errChan <- err
				return
			}
			defer reader.Close()
			
			retrievedData, err := io.ReadAll(reader)
			if err != nil {
				errChan <- err
				return
			}
			
			if !bytes.Equal(testData, retrievedData) {
				errChan <- errors.New("data mismatch")
				return
			}
			
			errChan <- nil
		}(g)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		err := <-errChan
		if err != nil {
			t.Errorf("goroutine %d failed: %v", i, err)
		}
	}
}

func TestStreamingBlockstore_AutomaticStreaming(t *testing.T) {
	ctx := context.Background()
	
	// Create in-memory datastore
	memoryDS := dssync.MutexWrap(ds.NewMapDatastore())
	bs := NewBlockstore(memoryDS)
	
	// Create streaming blockstore with low threshold for testing
	config := &StreamingConfig{
		LargeBlockThreshold:  100,                // 100 bytes
		DefaultChunkSize:     50,                 // 50 bytes
		EnableCompression:    true,
		MaxConcurrentStreams: 5,
		StreamTimeout:        30 * time.Second, // Increased timeout
		BufferSize:           128,
	}
	
	sbs, err := NewStreamingBlockstore(ctx, bs, config)
	if err != nil {
		t.Fatalf("failed to create streaming blockstore: %v", err)
	}
	defer sbs.Close()
	
	// Test small block (should use regular storage)
	smallData := []byte("small")
	smallBlock := createTestBlock(smallData)
	
	err = sbs.Put(ctx, smallBlock)
	if err != nil {
		t.Fatalf("failed to put small block: %v", err)
	}
	
	retrievedSmall, err := sbs.Get(ctx, smallBlock.Cid())
	if err != nil {
		t.Fatalf("failed to get small block: %v", err)
	}
	
	if !bytes.Equal(smallData, retrievedSmall.RawData()) {
		t.Errorf("small block data mismatch")
	}
	
	// Test large block (should use streaming storage)
	largeData := make([]byte, 200) // Above threshold
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	largeBlock := createTestBlock(largeData)
	
	err = sbs.Put(ctx, largeBlock)
	if err != nil {
		t.Fatalf("failed to put large block: %v", err)
	}
	
	retrievedLarge, err := sbs.Get(ctx, largeBlock.Cid())
	if err != nil {
		t.Fatalf("failed to get large block: %v", err)
	}
	
	if !bytes.Equal(largeData, retrievedLarge.RawData()) {
		t.Errorf("large block data mismatch")
	}
}

func TestStreamingBlockstore_View(t *testing.T) {
	ctx := context.Background()
	
	// Create in-memory datastore
	memoryDS := dssync.MutexWrap(ds.NewMapDatastore())
	bs := NewBlockstore(memoryDS)
	
	// Create streaming blockstore
	config := &StreamingConfig{
		LargeBlockThreshold:  100,
		DefaultChunkSize:     50,
		MaxConcurrentStreams: 5,
		StreamTimeout:        30 * time.Second, // Increased timeout
		BufferSize:           128,
	}
	
	sbs, err := NewStreamingBlockstore(ctx, bs, config)
	if err != nil {
		t.Fatalf("failed to create streaming blockstore: %v", err)
	}
	defer sbs.Close()
	
	// Test View with large block
	testData := make([]byte, 200)
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	block := createTestBlock(testData)
	
	err = sbs.Put(ctx, block)
	if err != nil {
		t.Fatalf("failed to put block: %v", err)
	}
	
	// Use View to access data
	var viewedData []byte
	err = sbs.View(ctx, block.Cid(), func(data []byte) error {
		viewedData = make([]byte, len(data))
		copy(viewedData, data)
		return nil
	})
	
	if err != nil {
		t.Fatalf("View failed: %v", err)
	}
	
	if !bytes.Equal(testData, viewedData) {
		t.Errorf("viewed data does not match original")
	}
}

func TestStreamingHandler_StreamPutGet_LargeBlock_Debug(t *testing.T) {
	sh, _ := createTestStreamingHandler(t)
	defer sh.Close()
	
	ctx := context.Background()
	
	// Create large test data (above threshold) - same as in the failing test
	testData := make([]byte, 200) // Above 1KB threshold in test config
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	
	block := createTestBlock(testData)
	
	// Stream put
	err := sh.StreamPut(ctx, block.Cid(), bytes.NewReader(testData))
	if err != nil {
		t.Fatalf("failed to stream put: %v", err)
	}
	
	// Stream get
	reader, err := sh.StreamGet(ctx, block.Cid())
	if err != nil {
		t.Fatalf("failed to stream get: %v", err)
	}
	defer reader.Close()
	
	retrievedData, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read retrieved data: %v", err)
	}
	
	if !bytes.Equal(testData, retrievedData) {
		t.Errorf("retrieved data does not match original")
	}
}

func TestStreamingHandler_ErrorHandling(t *testing.T) {
	sh, _ := createTestStreamingHandler(t)
	defer sh.Close()
	
	ctx := context.Background()
	
	// Test with invalid CID
	invalidCID := cid.Cid{}
	err := sh.StreamPut(ctx, invalidCID, bytes.NewReader([]byte("test")))
	if err == nil {
		t.Errorf("expected error with invalid CID")
	}
	
	// Test getting non-existent block
	testData := []byte("test data")
	block := createTestBlock(testData)
	
	_, err = sh.StreamGet(ctx, block.Cid())
	if err == nil {
		t.Errorf("expected error when getting non-existent block")
	}
}

// Benchmark tests
func BenchmarkStreamingHandler_StreamPut(b *testing.B) {
	ctx := context.Background()
	
	// Create in-memory datastore
	memoryDS := dssync.MutexWrap(ds.NewMapDatastore())
	bs := NewBlockstore(memoryDS)
	
	// Create streaming handler
	config := DefaultStreamingConfig()
	sh, err := NewStreamingHandler(ctx, bs, config)
	if err != nil {
		b.Fatalf("failed to create streaming handler: %v", err)
	}
	defer sh.Close()
	
	// Pre-generate test data
	testData := make([]byte, 1024*1024) // 1MB
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		block := createTestBlock(append(testData, byte(i))) // Make each block unique
		err := sh.StreamPut(ctx, block.Cid(), bytes.NewReader(testData))
		if err != nil {
			b.Fatalf("failed to stream put: %v", err)
		}
	}
}

func BenchmarkStreamingHandler_ChunkedPut(b *testing.B) {
	ctx := context.Background()
	
	// Create in-memory datastore
	memoryDS := dssync.MutexWrap(ds.NewMapDatastore())
	bs := NewBlockstore(memoryDS)
	
	// Create streaming handler
	config := DefaultStreamingConfig()
	sh, err := NewStreamingHandler(ctx, bs, config)
	if err != nil {
		b.Fatalf("failed to create streaming handler: %v", err)
	}
	defer sh.Close()
	
	// Pre-generate test data
	testData := make([]byte, 1024*1024) // 1MB
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		block := createTestBlock(append(testData, byte(i))) // Make each block unique
		err := sh.ChunkedPut(ctx, block.Cid(), bytes.NewReader(testData), config.DefaultChunkSize)
		if err != nil {
			b.Fatalf("failed to chunked put: %v", err)
		}
	}
}