package blockstore

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	metrics "github.com/ipfs/go-metrics-interface"
)

// StreamingHandler manages streaming operations for large blocks
type StreamingHandler interface {
	// StreamPut stores a large block using streaming
	StreamPut(ctx context.Context, c cid.Cid, data io.Reader) error
	
	// StreamGet retrieves a large block using streaming
	StreamGet(ctx context.Context, c cid.Cid) (io.ReadCloser, error)
	
	// ChunkedPut stores a large block by splitting it into chunks
	ChunkedPut(ctx context.Context, c cid.Cid, data io.Reader, chunkSize int) error
	
	// ChunkedGet retrieves a large block by reassembling chunks
	ChunkedGet(ctx context.Context, c cid.Cid) (io.ReadCloser, error)
	
	// CompressedPut stores a block with compression
	CompressedPut(ctx context.Context, c cid.Cid, data io.Reader) error
	
	// CompressedGet retrieves a compressed block
	CompressedGet(ctx context.Context, c cid.Cid) (io.ReadCloser, error)
	
	// GetStreamingStats returns streaming operation statistics
	GetStreamingStats() *StreamingStats
	
	// Close shuts down the streaming handler
	Close() error
}

// StreamingConfig holds configuration for streaming operations
type StreamingConfig struct {
	// LargeBlockThreshold is the size threshold for using streaming (default: 1MB)
	LargeBlockThreshold int64
	
	// DefaultChunkSize is the default chunk size for splitting large blocks
	DefaultChunkSize int
	
	// EnableCompression enables automatic compression for large blocks
	EnableCompression bool
	
	// CompressionLevel is the compression level (1-9, where 9 is best compression)
	CompressionLevel int
	
	// MaxConcurrentStreams is the maximum number of concurrent streaming operations
	MaxConcurrentStreams int
	
	// StreamTimeout is the timeout for streaming operations
	StreamTimeout time.Duration
	
	// BufferSize is the buffer size for streaming operations
	BufferSize int
}

// StreamingStats holds statistics for streaming operations
type StreamingStats struct {
	// TotalStreams is the total number of streaming operations
	TotalStreams int64
	
	// TotalBytesStreamed is the total number of bytes streamed
	TotalBytesStreamed int64
	
	// TotalChunks is the total number of chunks created
	TotalChunks int64
	
	// CompressionRatio is the average compression ratio achieved
	CompressionRatio float64
	
	// AverageStreamLatency is the average streaming operation latency
	AverageStreamLatency time.Duration
	
	// ActiveStreams is the current number of active streams
	ActiveStreams int64
	
	// FailedStreams is the number of failed streaming operations
	FailedStreams int64
}

// ChunkInfo represents information about a chunk
type ChunkInfo struct {
	Index      int
	Size       int64
	CID        cid.Cid
	Checksum   []byte
	Compressed bool // Whether this specific chunk is compressed
}

// ChunkMetadata represents metadata for a chunked block
type ChunkMetadata struct {
	OriginalCID   cid.Cid
	OriginalSize  int64
	ChunkSize     int
	TotalChunks   int
	Chunks        []ChunkInfo
	Compressed    bool
	CreatedAt     time.Time
}

// streamingHandler implements StreamingHandler
type streamingHandler struct {
	config     *StreamingConfig
	blockstore Blockstore
	
	// Chunk metadata storage
	chunkMetadata map[string]*ChunkMetadata
	metadataMu    sync.RWMutex
	
	// Stream management
	activeStreams map[string]context.CancelFunc
	streamsMu     sync.RWMutex
	
	// Statistics
	stats   *StreamingStats
	statsMu sync.RWMutex
	
	// Metrics
	metrics struct {
		streamsTotal     metrics.Counter
		bytesStreamed    metrics.Counter
		chunksCreated    metrics.Counter
		streamLatency    metrics.Histogram
		compressionRatio metrics.Gauge
	}
}

// DefaultStreamingConfig returns a default configuration for streaming operations
func DefaultStreamingConfig() *StreamingConfig {
	return &StreamingConfig{
		LargeBlockThreshold:  1024 * 1024, // 1MB
		DefaultChunkSize:     256 * 1024,  // 256KB
		EnableCompression:    true,
		CompressionLevel:     6, // Balanced compression
		MaxConcurrentStreams: 10,
		StreamTimeout:        30 * time.Second,
		BufferSize:           64 * 1024, // 64KB
	}
}

// NewStreamingHandler creates a new streaming handler
func NewStreamingHandler(ctx context.Context, bs Blockstore, config *StreamingConfig) (StreamingHandler, error) {
	if config == nil {
		config = DefaultStreamingConfig()
	}
	
	if bs == nil {
		return nil, errors.New("blockstore cannot be nil")
	}
	
	sh := &streamingHandler{
		config:        config,
		blockstore:    bs,
		chunkMetadata: make(map[string]*ChunkMetadata),
		activeStreams: make(map[string]context.CancelFunc),
		stats:         &StreamingStats{},
	}
	
	// Initialize metrics
	sh.metrics.streamsTotal = metrics.NewCtx(ctx, "streaming.streams_total", "Total number of streaming operations").Counter()
	sh.metrics.bytesStreamed = metrics.NewCtx(ctx, "streaming.bytes_streamed_total", "Total bytes streamed").Counter()
	sh.metrics.chunksCreated = metrics.NewCtx(ctx, "streaming.chunks_created_total", "Total chunks created").Counter()
	sh.metrics.streamLatency = metrics.NewCtx(ctx, "streaming.stream_latency_seconds", "Streaming operation latency").Histogram([]float64{0.1, 1, 10, 30, 60})
	sh.metrics.compressionRatio = metrics.NewCtx(ctx, "streaming.compression_ratio", "Compression ratio achieved").Gauge()
	
	return sh, nil
}

// StreamPut stores a large block using streaming
func (sh *streamingHandler) StreamPut(ctx context.Context, c cid.Cid, data io.Reader) error {
	if !c.Defined() {
		return errors.New("invalid CID")
	}
	
	start := time.Now()
	streamID := c.String()
	
	// Register active stream
	streamCtx, cancel := context.WithTimeout(ctx, sh.config.StreamTimeout)
	defer cancel()
	
	sh.streamsMu.Lock()
	sh.activeStreams[streamID] = cancel
	sh.streamsMu.Unlock()
	
	defer func() {
		sh.streamsMu.Lock()
		delete(sh.activeStreams, streamID)
		sh.streamsMu.Unlock()
	}()
	
	// Read all data into memory first to determine size
	buf := &bytes.Buffer{}
	bytesRead, err := io.Copy(buf, data)
	if err != nil {
		sh.updateStats(false, bytesRead, time.Since(start))
		return fmt.Errorf("failed to read stream data: %w", err)
	}
	
	// Decide on storage strategy based on size
	if bytesRead > sh.config.LargeBlockThreshold {
		// Use chunked storage for large blocks
		err = sh.ChunkedPut(streamCtx, c, bytes.NewReader(buf.Bytes()), sh.config.DefaultChunkSize)
	} else {
		// Use regular storage for smaller blocks
		block, blockErr := blocks.NewBlockWithCid(buf.Bytes(), c)
		if blockErr != nil {
			err = blockErr
		} else {
			err = sh.blockstore.Put(streamCtx, block)
		}
	}
	
	sh.updateStats(err == nil, bytesRead, time.Since(start))
	return err
}

// StreamGet retrieves a large block using streaming
func (sh *streamingHandler) StreamGet(ctx context.Context, c cid.Cid) (io.ReadCloser, error) {
	if !c.Defined() {
		return nil, errors.New("invalid CID")
	}
	
	start := time.Now()
	streamID := c.String()
	
	// Register active stream
	streamCtx, cancel := context.WithTimeout(ctx, sh.config.StreamTimeout)
	
	sh.streamsMu.Lock()
	sh.activeStreams[streamID] = cancel
	sh.streamsMu.Unlock()
	
	// Check if this is a chunked block
	key := string(c.Hash())
	sh.metadataMu.RLock()
	metadata, isChunked := sh.chunkMetadata[key]
	sh.metadataMu.RUnlock()
	
	if isChunked {
		// Use chunked retrieval
		reader, err := sh.ChunkedGet(streamCtx, c)
		if err != nil {
			cancel()
			sh.streamsMu.Lock()
			delete(sh.activeStreams, streamID)
			sh.streamsMu.Unlock()
			sh.updateStats(false, 0, time.Since(start))
			return nil, err
		}
		
		// Wrap reader to clean up on close
		return &streamReader{
			ReadCloser: reader,
			onClose: func() {
				cancel()
				sh.streamsMu.Lock()
				delete(sh.activeStreams, streamID)
				sh.streamsMu.Unlock()
				sh.updateStats(true, metadata.OriginalSize, time.Since(start))
			},
		}, nil
	}
	
	// Regular block retrieval
	block, err := sh.blockstore.Get(streamCtx, c)
	if err != nil {
		cancel()
		sh.streamsMu.Lock()
		delete(sh.activeStreams, streamID)
		sh.streamsMu.Unlock()
		sh.updateStats(false, 0, time.Since(start))
		return nil, err
	}
	
	reader := &streamReader{
		ReadCloser: io.NopCloser(bytes.NewReader(block.RawData())),
		onClose: func() {
			cancel()
			sh.streamsMu.Lock()
			delete(sh.activeStreams, streamID)
			sh.streamsMu.Unlock()
			sh.updateStats(true, int64(len(block.RawData())), time.Since(start))
		},
	}
	
	return reader, nil
}

// ChunkedPut stores a large block by splitting it into chunks
func (sh *streamingHandler) ChunkedPut(ctx context.Context, c cid.Cid, data io.Reader, chunkSize int) error {
	if chunkSize <= 0 {
		chunkSize = sh.config.DefaultChunkSize
	}
	
	var chunks []ChunkInfo
	var totalSize int64
	chunkIndex := 0
	
	// Create a buffer for reading chunks
	buffer := make([]byte, chunkSize)
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		// Read next chunk
		n, err := io.ReadFull(data, buffer)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("failed to read chunk %d: %w", chunkIndex, err)
		}
		
		if n == 0 {
			break // End of data
		}
		
		// Create a copy of the chunk data to avoid buffer reuse issues
		chunkData := make([]byte, n)
		copy(chunkData, buffer[:n])
		totalSize += int64(n)
		
		// Store original data for checksum calculation
		originalChunkData := make([]byte, n)
		copy(originalChunkData, chunkData)
		
		// Track whether this chunk is compressed
		chunkCompressed := false
		
		// Compress chunk if enabled
		if sh.config.EnableCompression {
			compressed, err := sh.compressData(chunkData)
			if err != nil {
				return fmt.Errorf("failed to compress chunk %d: %w", chunkIndex, err)
			}
			
			// Use compressed data if it's smaller
			if len(compressed) < len(chunkData) {
				chunkData = compressed
				chunkCompressed = true
			}
		}
		
		// Create chunk block
		chunkBlock := blocks.NewBlock(chunkData)
		
		// Store chunk
		if err := sh.blockstore.Put(ctx, chunkBlock); err != nil {
			return fmt.Errorf("failed to store chunk %d: %w", chunkIndex, err)
		}
		
		// Record chunk info
		chunks = append(chunks, ChunkInfo{
			Index:      chunkIndex,
			Size:       int64(n),
			CID:        chunkBlock.Cid(),
			Checksum:   sh.calculateChecksum(originalChunkData),
			Compressed: chunkCompressed,
		})
		

		
		chunkIndex++
		
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
	}
	
	// Create and store metadata
	metadata := &ChunkMetadata{
		OriginalCID:  c,
		OriginalSize: totalSize,
		ChunkSize:    chunkSize,
		TotalChunks:  len(chunks),
		Chunks:       chunks,
		Compressed:   sh.config.EnableCompression,
		CreatedAt:    time.Now(),
	}
	
	// Store metadata
	key := string(c.Hash())
	sh.metadataMu.Lock()
	sh.chunkMetadata[key] = metadata
	sh.metadataMu.Unlock()
	
	// Update statistics
	sh.statsMu.Lock()
	sh.stats.TotalChunks += int64(len(chunks))
	sh.statsMu.Unlock()
	
	sh.metrics.chunksCreated.Add(float64(len(chunks)))
	
	return nil
}

// ChunkedGet retrieves a large block by reassembling chunks
func (sh *streamingHandler) ChunkedGet(ctx context.Context, c cid.Cid) (io.ReadCloser, error) {
	key := string(c.Hash())
	
	// Get metadata
	sh.metadataMu.RLock()
	metadata, exists := sh.chunkMetadata[key]
	sh.metadataMu.RUnlock()
	
	if !exists {
		return nil, ipld.ErrNotFound{Cid: c}
	}
	
	// Sort chunks by index to ensure correct order
	chunks := make([]ChunkInfo, len(metadata.Chunks))
	copy(chunks, metadata.Chunks)
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Index < chunks[j].Index
	})
	
	// Create a pipe for streaming the reassembled data
	pr, pw := io.Pipe()
	
	go func() {
		defer pw.Close()
		
		for _, chunkInfo := range chunks {
			select {
			case <-ctx.Done():
				pw.CloseWithError(ctx.Err())
				return
			default:
			}
			
			// Retrieve chunk
			chunkBlock, err := sh.blockstore.Get(ctx, chunkInfo.CID)
			if err != nil {
				pw.CloseWithError(fmt.Errorf("failed to retrieve chunk %d: %w", chunkInfo.Index, err))
				return
			}
			
			chunkData := chunkBlock.RawData()
			

			
			// Decompress if this specific chunk is compressed
			if chunkInfo.Compressed {
				decompressed, err := sh.decompressData(chunkData)
				if err != nil {
					// If decompression fails, use original data (might not be compressed)
					decompressed = chunkData
				}
				chunkData = decompressed
			}
			
			// Verify checksum
			if !bytes.Equal(sh.calculateChecksum(chunkData), chunkInfo.Checksum) {
				pw.CloseWithError(fmt.Errorf("checksum mismatch for chunk %d", chunkInfo.Index))
				return
			}
			

			
			// Write chunk data to pipe
			if _, err := pw.Write(chunkData); err != nil {
				pw.CloseWithError(err)
				return
			}
		}
	}()
	
	return pr, nil
}

// CompressedPut stores a block with compression
func (sh *streamingHandler) CompressedPut(ctx context.Context, c cid.Cid, data io.Reader) error {
	// Read all data
	buf := &bytes.Buffer{}
	originalSize, err := io.Copy(buf, data)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}
	
	// Compress data
	compressed, err := sh.compressData(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}
	
	// Use compressed data if it's smaller
	var finalData []byte
	if len(compressed) < int(originalSize) {
		finalData = compressed
		
		// Update compression ratio
		ratio := float64(len(compressed)) / float64(originalSize)
		sh.metrics.compressionRatio.Set(ratio)
		
		sh.statsMu.Lock()
		if sh.stats.CompressionRatio == 0 {
			sh.stats.CompressionRatio = ratio
		} else {
			// Moving average
			sh.stats.CompressionRatio = (sh.stats.CompressionRatio + ratio) / 2
		}
		sh.statsMu.Unlock()
	} else {
		finalData = buf.Bytes()
	}
	
	// Store block
	block, err := blocks.NewBlockWithCid(finalData, c)
	if err != nil {
		return err
	}
	return sh.blockstore.Put(ctx, block)
}

// CompressedGet retrieves a compressed block
func (sh *streamingHandler) CompressedGet(ctx context.Context, c cid.Cid) (io.ReadCloser, error) {
	block, err := sh.blockstore.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	
	// Try to decompress
	decompressed, err := sh.decompressData(block.RawData())
	if err != nil {
		// If decompression fails, return original data
		return io.NopCloser(bytes.NewReader(block.RawData())), nil
	}
	
	return io.NopCloser(bytes.NewReader(decompressed)), nil
}

// compressData compresses data using gzip
func (sh *streamingHandler) compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	
	writer, err := gzip.NewWriterLevel(&buf, sh.config.CompressionLevel)
	if err != nil {
		return nil, err
	}
	
	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return nil, err
	}
	
	if err := writer.Close(); err != nil {
		return nil, err
	}
	
	return buf.Bytes(), nil
}

// decompressData decompresses gzip-compressed data
func (sh *streamingHandler) decompressData(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return nil, err
	}
	
	return buf.Bytes(), nil
}

// calculateChecksum calculates a simple checksum for data integrity
func (sh *streamingHandler) calculateChecksum(data []byte) []byte {
	// Simple XOR checksum for demonstration
	// In production, you might want to use a more robust checksum like CRC32 or SHA256
	checksum := byte(0)
	for _, b := range data {
		checksum ^= b
	}
	return []byte{checksum}
}

// GetStreamingStats returns streaming operation statistics
func (sh *streamingHandler) GetStreamingStats() *StreamingStats {
	sh.statsMu.RLock()
	defer sh.statsMu.RUnlock()
	
	// Return a copy to avoid race conditions
	stats := *sh.stats
	
	// Add current active streams
	sh.streamsMu.RLock()
	stats.ActiveStreams = int64(len(sh.activeStreams))
	sh.streamsMu.RUnlock()
	
	return &stats
}

// Close shuts down the streaming handler
func (sh *streamingHandler) Close() error {
	// Cancel all active streams
	sh.streamsMu.Lock()
	for _, cancel := range sh.activeStreams {
		cancel()
	}
	sh.activeStreams = make(map[string]context.CancelFunc)
	sh.streamsMu.Unlock()
	
	// Clear metadata
	sh.metadataMu.Lock()
	sh.chunkMetadata = make(map[string]*ChunkMetadata)
	sh.metadataMu.Unlock()
	
	return nil
}

// updateStats updates streaming operation statistics
func (sh *streamingHandler) updateStats(success bool, bytesProcessed int64, latency time.Duration) {
	sh.statsMu.Lock()
	defer sh.statsMu.Unlock()
	
	sh.stats.TotalStreams++
	sh.stats.TotalBytesStreamed += bytesProcessed
	
	if !success {
		sh.stats.FailedStreams++
	}
	
	// Update average latency (simple moving average)
	if sh.stats.TotalStreams == 1 {
		sh.stats.AverageStreamLatency = latency
	} else {
		// Weighted average
		weight := 0.1 // Give more weight to recent measurements
		sh.stats.AverageStreamLatency = time.Duration(
			float64(sh.stats.AverageStreamLatency)*(1-weight) + float64(latency)*weight,
		)
	}
	
	// Update metrics
	sh.metrics.streamsTotal.Inc()
	sh.metrics.bytesStreamed.Add(float64(bytesProcessed))
	sh.metrics.streamLatency.Observe(latency.Seconds())
}

// streamReader wraps an io.ReadCloser with a custom close function
type streamReader struct {
	io.ReadCloser
	onClose func()
	closed  bool
	mu      sync.Mutex
}

func (sr *streamReader) Close() error {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	if sr.closed {
		return nil
	}
	
	sr.closed = true
	
	if sr.onClose != nil {
		sr.onClose()
	}
	
	return sr.ReadCloser.Close()
}

// StreamingBlockstore wraps a regular blockstore with streaming capabilities
type StreamingBlockstore struct {
	Blockstore
	StreamingHandler
}

// NewStreamingBlockstore creates a new blockstore with streaming capabilities
func NewStreamingBlockstore(ctx context.Context, bs Blockstore, config *StreamingConfig) (*StreamingBlockstore, error) {
	sh, err := NewStreamingHandler(ctx, bs, config)
	if err != nil {
		return nil, err
	}
	
	return &StreamingBlockstore{
		Blockstore:       bs,
		StreamingHandler: sh,
	}, nil
}

// Put automatically uses streaming for large blocks
func (sbs *StreamingBlockstore) Put(ctx context.Context, block blocks.Block) error {
	data := block.RawData()
	
	// Use streaming for large blocks
	if int64(len(data)) > sbs.StreamingHandler.(*streamingHandler).config.LargeBlockThreshold {
		return sbs.StreamPut(ctx, block.Cid(), bytes.NewReader(data))
	}
	
	// Use regular put for smaller blocks
	return sbs.Blockstore.Put(ctx, block)
}

// Get automatically uses streaming for large blocks
func (sbs *StreamingBlockstore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	// Check if this is a chunked block
	sh := sbs.StreamingHandler.(*streamingHandler)
	key := string(c.Hash())
	
	sh.metadataMu.RLock()
	metadata, isChunked := sh.chunkMetadata[key]
	sh.metadataMu.RUnlock()
	
	if isChunked {
		// Use streaming get for chunked blocks
		reader, err := sbs.StreamGet(ctx, c)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		
		// Read all data
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		
		// Verify the data length matches expected
		if int64(len(data)) != metadata.OriginalSize {
			return nil, fmt.Errorf("data length mismatch: expected %d, got %d", metadata.OriginalSize, len(data))
		}
		
		block, err := blocks.NewBlockWithCid(data, c)
		if err != nil {
			return nil, err
		}
		return block, nil
	}
	
	// Use regular get for normal blocks
	return sbs.Blockstore.Get(ctx, c)
}

// View implements the Viewer interface for streaming blocks
func (sbs *StreamingBlockstore) View(ctx context.Context, c cid.Cid, callback func([]byte) error) error {
	// Check if this is a chunked block
	sh := sbs.StreamingHandler.(*streamingHandler)
	key := string(c.Hash())
	
	sh.metadataMu.RLock()
	_, isChunked := sh.chunkMetadata[key]
	sh.metadataMu.RUnlock()
	
	if isChunked {
		// Use streaming get for chunked blocks
		reader, err := sbs.StreamGet(ctx, c)
		if err != nil {
			return err
		}
		defer reader.Close()
		
		// Read all data
		data, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		
		return callback(data)
	}
	
	// Use regular view for normal blocks
	if viewer, ok := sbs.Blockstore.(Viewer); ok {
		return viewer.View(ctx, c, callback)
	}
	
	// Fallback to Get
	block, err := sbs.Blockstore.Get(ctx, c)
	if err != nil {
		return err
	}
	
	return callback(block.RawData())
}

// Close shuts down the streaming blockstore
func (sbs *StreamingBlockstore) Close() error {
	if err := sbs.StreamingHandler.Close(); err != nil {
		return err
	}
	
	// Close underlying blockstore if it supports it
	if closer, ok := sbs.Blockstore.(interface{ Close() error }); ok {
		return closer.Close()
	}
	
	return nil
}