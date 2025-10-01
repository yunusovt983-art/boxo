package blockstore

import (
	"context"
	"errors"
	"sync"
	"time"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	metrics "github.com/ipfs/go-metrics-interface"
)

// CacheTier represents different storage tiers in the multi-tier cache
type CacheTier int

const (
	// MemoryTier represents the fastest, most expensive storage (RAM)
	MemoryTier CacheTier = iota
	// SSDTier represents medium speed, medium cost storage (SSD)
	SSDTier
	// HDDTier represents slower, cheaper storage (HDD)
	HDDTier
)

// String returns the string representation of the cache tier
func (t CacheTier) String() string {
	switch t {
	case MemoryTier:
		return "memory"
	case SSDTier:
		return "ssd"
	case HDDTier:
		return "hdd"
	default:
		return "unknown"
	}
}

// MultiTierBlockstore extends the standard Blockstore interface with multi-tier caching capabilities
type MultiTierBlockstore interface {
	Blockstore
	Viewer
	
	// GetWithTier attempts to retrieve a block from a specific tier
	GetWithTier(ctx context.Context, c cid.Cid, tier CacheTier) (blocks.Block, error)
	
	// PutWithTier stores a block in a specific tier
	PutWithTier(ctx context.Context, block blocks.Block, tier CacheTier) error
	
	// PromoteBlock moves a block to a higher tier based on access patterns
	PromoteBlock(ctx context.Context, c cid.Cid) error
	
	// DemoteBlock moves a block to a lower tier to free up space
	DemoteBlock(ctx context.Context, c cid.Cid) error
	
	// GetTierStats returns statistics for each tier
	GetTierStats() map[CacheTier]*TierStats
	
	// Compact performs maintenance operations on all tiers
	Compact(ctx context.Context) error
	
	// Close shuts down the multi-tier blockstore
	Close() error
}

// TierConfig holds configuration for a specific cache tier
type TierConfig struct {
	// MaxSize is the maximum size in bytes for this tier
	MaxSize int64
	
	// MaxItems is the maximum number of items for this tier
	MaxItems int
	
	// TTL is the time-to-live for items in this tier
	TTL time.Duration
	
	// PromotionThreshold is the access count needed for promotion
	PromotionThreshold int
	
	// Enabled indicates if this tier is active
	Enabled bool
}

// MultiTierConfig holds configuration for the entire multi-tier cache
type MultiTierConfig struct {
	// Tiers maps each tier to its configuration
	Tiers map[CacheTier]*TierConfig
	
	// AutoPromote enables automatic promotion based on access patterns
	AutoPromote bool
	
	// AutoDemote enables automatic demotion when tiers are full
	AutoDemote bool
	
	// CompactionInterval is how often to run maintenance operations
	CompactionInterval time.Duration
}

// TierStats holds statistics for a cache tier
type TierStats struct {
	// Size is the current size in bytes
	Size int64
	
	// Items is the current number of items
	Items int
	
	// Hits is the number of cache hits
	Hits int64
	
	// Misses is the number of cache misses
	Misses int64
	
	// Promotions is the number of blocks promoted from this tier
	Promotions int64
	
	// Demotions is the number of blocks demoted to this tier
	Demotions int64
	
	// LastAccess is the timestamp of the last access
	LastAccess time.Time
}

// AccessInfo tracks access patterns for blocks
type AccessInfo struct {
	// Count is the number of times this block has been accessed
	Count int
	
	// LastAccess is the timestamp of the last access
	LastAccess time.Time
	
	// FirstAccess is the timestamp of the first access
	FirstAccess time.Time
	
	// CurrentTier is the tier where this block is currently stored
	CurrentTier CacheTier
	
	// Size is the size of the block in bytes
	Size int
}

// multiTierBlockstore implements MultiTierBlockstore
type multiTierBlockstore struct {
	// config holds the configuration for all tiers
	config *MultiTierConfig
	
	// tiers maps each tier to its blockstore implementation
	tiers map[CacheTier]Blockstore
	
	// accessInfo tracks access patterns for blocks
	accessInfo map[string]*AccessInfo
	accessMu   sync.RWMutex
	
	// stats holds statistics for each tier
	stats map[CacheTier]*TierStats
	statsMu sync.RWMutex
	
	// compactionTicker runs periodic maintenance
	compactionTicker *time.Ticker
	compactionStop   chan struct{}
	
	// metrics for monitoring
	metrics struct {
		hits       metrics.Counter
		misses     metrics.Counter
		promotions metrics.Counter
		demotions  metrics.Counter
	}
	
	// viewer support
	viewer Viewer
}

// DefaultMultiTierConfig returns a default configuration for multi-tier caching
func DefaultMultiTierConfig() *MultiTierConfig {
	return &MultiTierConfig{
		Tiers: map[CacheTier]*TierConfig{
			MemoryTier: {
				MaxSize:            2 << 30, // 2GB
				MaxItems:           100000,
				TTL:                time.Hour,
				PromotionThreshold: 1, // Already in top tier
				Enabled:            true,
			},
			SSDTier: {
				MaxSize:            50 << 30, // 50GB
				MaxItems:           1000000,
				TTL:                24 * time.Hour,
				PromotionThreshold: 3, // 3 accesses to promote to memory
				Enabled:            true,
			},
			HDDTier: {
				MaxSize:            500 << 30, // 500GB
				MaxItems:           10000000,
				TTL:                7 * 24 * time.Hour, // 1 week
				PromotionThreshold: 5,                  // 5 accesses to promote to SSD
				Enabled:            true,
			},
		},
		AutoPromote:        true,
		AutoDemote:         true,
		CompactionInterval: 30 * time.Minute,
	}
}

// NewMultiTierBlockstore creates a new multi-tier blockstore
func NewMultiTierBlockstore(ctx context.Context, tiers map[CacheTier]Blockstore, config *MultiTierConfig) (MultiTierBlockstore, error) {
	if config == nil {
		config = DefaultMultiTierConfig()
	}
	
	// Validate that we have at least one tier
	if len(tiers) == 0 {
		return nil, errors.New("at least one tier must be provided")
	}
	
	// Initialize stats for each tier
	stats := make(map[CacheTier]*TierStats)
	for tier := range tiers {
		stats[tier] = &TierStats{
			LastAccess: time.Now(),
		}
	}
	
	mtbs := &multiTierBlockstore{
		config:         config,
		tiers:          tiers,
		accessInfo:     make(map[string]*AccessInfo),
		stats:          stats,
		compactionStop: make(chan struct{}),
	}
	
	// Initialize metrics
	mtbs.metrics.hits = metrics.NewCtx(ctx, "multitier.hits_total", "Number of cache hits in multi-tier cache").Counter()
	mtbs.metrics.misses = metrics.NewCtx(ctx, "multitier.misses_total", "Number of cache misses in multi-tier cache").Counter()
	mtbs.metrics.promotions = metrics.NewCtx(ctx, "multitier.promotions_total", "Number of block promotions").Counter()
	mtbs.metrics.demotions = metrics.NewCtx(ctx, "multitier.demotions_total", "Number of block demotions").Counter()
	
	// Check if any tier supports Viewer interface
	for _, bs := range tiers {
		if v, ok := bs.(Viewer); ok {
			mtbs.viewer = v
			break
		}
	}
	
	// Start compaction routine if configured
	if config.CompactionInterval > 0 {
		mtbs.compactionTicker = time.NewTicker(config.CompactionInterval)
		go mtbs.compactionRoutine(ctx)
	}
	
	return mtbs, nil
}

// multiTierCacheKey generates a cache key from a CID for multi-tier use
func multiTierCacheKey(c cid.Cid) string {
	return string(c.Hash())
}

// getOrderedTiers returns tiers in order from fastest to slowest
func (mtbs *multiTierBlockstore) getOrderedTiers() []CacheTier {
	var tiers []CacheTier
	
	// Add tiers in order of preference (fastest first)
	if _, exists := mtbs.tiers[MemoryTier]; exists && mtbs.config.Tiers[MemoryTier].Enabled {
		tiers = append(tiers, MemoryTier)
	}
	if _, exists := mtbs.tiers[SSDTier]; exists && mtbs.config.Tiers[SSDTier].Enabled {
		tiers = append(tiers, SSDTier)
	}
	if _, exists := mtbs.tiers[HDDTier]; exists && mtbs.config.Tiers[HDDTier].Enabled {
		tiers = append(tiers, HDDTier)
	}
	
	return tiers
}

// updateAccessInfo updates access tracking information for a block
func (mtbs *multiTierBlockstore) updateAccessInfo(key string, tier CacheTier, size int) {
	mtbs.accessMu.Lock()
	defer mtbs.accessMu.Unlock()
	
	now := time.Now()
	info, exists := mtbs.accessInfo[key]
	if !exists {
		info = &AccessInfo{
			Count:       1,
			FirstAccess: now,
			LastAccess:  now,
			CurrentTier: tier,
			Size:        size,
		}
		mtbs.accessInfo[key] = info
	} else {
		info.Count++
		info.LastAccess = now
		info.CurrentTier = tier
		if size > 0 {
			info.Size = size
		}
	}
}

// updateTierStats updates statistics for a specific tier
func (mtbs *multiTierBlockstore) updateTierStats(tier CacheTier, hit bool, size int64) {
	mtbs.statsMu.Lock()
	defer mtbs.statsMu.Unlock()
	
	stats := mtbs.stats[tier]
	if hit {
		stats.Hits++
		mtbs.metrics.hits.Inc()
	} else {
		stats.Misses++
		mtbs.metrics.misses.Inc()
	}
	stats.LastAccess = time.Now()
	
	if size > 0 {
		stats.Size += size
		stats.Items++
	}
}

// Get implements the Blockstore interface
func (mtbs *multiTierBlockstore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	if !c.Defined() {
		return nil, ipld.ErrNotFound{Cid: c}
	}
	
	key := multiTierCacheKey(c)
	tiers := mtbs.getOrderedTiers()
	
	// Try each tier in order (fastest first)
	for _, tier := range tiers {
		bs, exists := mtbs.tiers[tier]
		if !exists {
			continue
		}
		
		block, err := bs.Get(ctx, c)
		if err == nil {
			// Found the block, update access info and stats
			mtbs.updateAccessInfo(key, tier, len(block.RawData()))
			mtbs.updateTierStats(tier, true, int64(len(block.RawData())))
			
			// Auto-promote if configured and not already in top tier
			if mtbs.config.AutoPromote && tier != MemoryTier {
				go func() {
					if err := mtbs.considerPromotion(ctx, c, key); err != nil {
						logger.Debugf("failed to promote block %s: %v", c, err)
					}
				}()
			}
			
			return block, nil
		}
		
		// If not found in this tier, update miss stats
		if ipld.IsNotFound(err) {
			mtbs.updateTierStats(tier, false, 0)
			continue
		}
		
		// If it's a different error, return it
		return nil, err
	}
	
	// Block not found in any tier
	return nil, ipld.ErrNotFound{Cid: c}
}

// GetWithTier attempts to retrieve a block from a specific tier
func (mtbs *multiTierBlockstore) GetWithTier(ctx context.Context, c cid.Cid, tier CacheTier) (blocks.Block, error) {
	if !c.Defined() {
		return nil, ipld.ErrNotFound{Cid: c}
	}
	
	bs, exists := mtbs.tiers[tier]
	if !exists {
		return nil, errors.New("tier not available")
	}
	
	key := multiTierCacheKey(c)
	block, err := bs.Get(ctx, c)
	if err == nil {
		mtbs.updateAccessInfo(key, tier, len(block.RawData()))
		mtbs.updateTierStats(tier, true, int64(len(block.RawData())))
	} else if ipld.IsNotFound(err) {
		mtbs.updateTierStats(tier, false, 0)
	}
	
	return block, err
}

// Put implements the Blockstore interface
func (mtbs *multiTierBlockstore) Put(ctx context.Context, block blocks.Block) error {
	// Always try to put in the fastest available tier first
	tiers := mtbs.getOrderedTiers()
	if len(tiers) == 0 {
		return errors.New("no tiers available")
	}
	
	return mtbs.PutWithTier(ctx, block, tiers[0])
}

// PutWithTier stores a block in a specific tier
func (mtbs *multiTierBlockstore) PutWithTier(ctx context.Context, block blocks.Block, tier CacheTier) error {
	bs, exists := mtbs.tiers[tier]
	if !exists {
		return errors.New("tier not available")
	}
	
	// Check if tier has space
	if !mtbs.hasTierSpace(tier, len(block.RawData())) {
		if mtbs.config.AutoDemote {
			// Try to make space by demoting old blocks
			if err := mtbs.makeSpace(ctx, tier, len(block.RawData())); err != nil {
				return err
			}
		} else {
			return errors.New("tier is full")
		}
	}
	
	err := bs.Put(ctx, block)
	if err == nil {
		key := multiTierCacheKey(block.Cid())
		mtbs.updateAccessInfo(key, tier, len(block.RawData()))
		mtbs.updateTierStats(tier, true, int64(len(block.RawData())))
	}
	
	return err
}

// PutMany implements the Blockstore interface
func (mtbs *multiTierBlockstore) PutMany(ctx context.Context, blocks []blocks.Block) error {
	// Group blocks by target tier (start with fastest)
	tiers := mtbs.getOrderedTiers()
	if len(tiers) == 0 {
		return errors.New("no tiers available")
	}
	
	targetTier := tiers[0]
	bs, exists := mtbs.tiers[targetTier]
	if !exists {
		return errors.New("target tier not available")
	}
	
	// Check if we need to make space
	totalSize := 0
	for _, block := range blocks {
		totalSize += len(block.RawData())
	}
	
	if !mtbs.hasTierSpace(targetTier, totalSize) {
		if mtbs.config.AutoDemote {
			if err := mtbs.makeSpace(ctx, targetTier, totalSize); err != nil {
				return err
			}
		} else {
			return errors.New("tier is full")
		}
	}
	
	err := bs.PutMany(ctx, blocks)
	if err == nil {
		// Update access info for all blocks
		for _, block := range blocks {
			key := multiTierCacheKey(block.Cid())
			mtbs.updateAccessInfo(key, targetTier, len(block.RawData()))
		}
		mtbs.updateTierStats(targetTier, true, int64(totalSize))
	}
	
	return err
}

// Has implements the Blockstore interface
func (mtbs *multiTierBlockstore) Has(ctx context.Context, c cid.Cid) (bool, error) {
	if !c.Defined() {
		return false, nil
	}
	
	// Check each tier in order
	for _, tier := range mtbs.getOrderedTiers() {
		bs, exists := mtbs.tiers[tier]
		if !exists {
			continue
		}
		
		has, err := bs.Has(ctx, c)
		if err != nil {
			return false, err
		}
		if has {
			key := multiTierCacheKey(c)
			mtbs.updateAccessInfo(key, tier, 0) // Size unknown for Has operation
			mtbs.updateTierStats(tier, true, 0)
			return true, nil
		}
	}
	
	return false, nil
}

// GetSize implements the Blockstore interface
func (mtbs *multiTierBlockstore) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	if !c.Defined() {
		return -1, ipld.ErrNotFound{Cid: c}
	}
	
	// Check each tier in order
	for _, tier := range mtbs.getOrderedTiers() {
		bs, exists := mtbs.tiers[tier]
		if !exists {
			continue
		}
		
		size, err := bs.GetSize(ctx, c)
		if err == nil {
			key := multiTierCacheKey(c)
			mtbs.updateAccessInfo(key, tier, size)
			mtbs.updateTierStats(tier, true, 0) // Don't double-count size
			return size, nil
		}
		
		if !ipld.IsNotFound(err) {
			return -1, err
		}
	}
	
	return -1, ipld.ErrNotFound{Cid: c}
}

// DeleteBlock implements the Blockstore interface
func (mtbs *multiTierBlockstore) DeleteBlock(ctx context.Context, c cid.Cid) error {
	if !c.Defined() {
		return nil
	}
	
	key := multiTierCacheKey(c)
	var lastErr error
	deleted := false
	
	// Delete from all tiers
	for _, tier := range mtbs.getOrderedTiers() {
		bs, exists := mtbs.tiers[tier]
		if !exists {
			continue
		}
		
		err := bs.DeleteBlock(ctx, c)
		if err == nil {
			deleted = true
			// Update stats to reflect deletion
			mtbs.accessMu.Lock()
			if info, exists := mtbs.accessInfo[key]; exists {
				mtbs.statsMu.Lock()
				if stats, exists := mtbs.stats[tier]; exists {
					stats.Size -= int64(info.Size)
					stats.Items--
				}
				mtbs.statsMu.Unlock()
			}
			mtbs.accessMu.Unlock()
		} else if !ipld.IsNotFound(err) {
			lastErr = err
		}
	}
	
	if deleted {
		// Remove access info
		mtbs.accessMu.Lock()
		delete(mtbs.accessInfo, key)
		mtbs.accessMu.Unlock()
		return nil
	}
	
	if lastErr != nil {
		return lastErr
	}
	
	return nil // Block wasn't found, but that's not an error for deletion
}

// AllKeysChan implements the Blockstore interface
func (mtbs *multiTierBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	// Collect keys from all tiers and deduplicate
	output := make(chan cid.Cid, 100)
	seen := make(map[string]bool)
	
	go func() {
		defer close(output)
		
		for _, tier := range mtbs.getOrderedTiers() {
			bs, exists := mtbs.tiers[tier]
			if !exists {
				continue
			}
			
			ch, err := bs.AllKeysChan(ctx)
			if err != nil {
				logger.Errorf("failed to get keys from tier %s: %v", tier, err)
				continue
			}
			
			for {
				select {
				case <-ctx.Done():
					return
				case c, ok := <-ch:
					if !ok {
						goto nextTier
					}
					
					key := multiTierCacheKey(c)
					if !seen[key] {
						seen[key] = true
						select {
						case <-ctx.Done():
							return
						case output <- c:
						}
					}
				}
			}
			nextTier:
		}
	}()
	
	return output, nil
}

// View implements the Viewer interface if supported by underlying tiers
func (mtbs *multiTierBlockstore) View(ctx context.Context, c cid.Cid, callback func([]byte) error) error {
	if mtbs.viewer == nil {
		// Fallback to Get if no tier supports View
		block, err := mtbs.Get(ctx, c)
		if err != nil {
			return err
		}
		return callback(block.RawData())
	}
	
	if !c.Defined() {
		return ipld.ErrNotFound{Cid: c}
	}
	
	// Try each tier in order
	for _, tier := range mtbs.getOrderedTiers() {
		bs, exists := mtbs.tiers[tier]
		if !exists {
			continue
		}
		
		if viewer, ok := bs.(Viewer); ok {
			err := viewer.View(ctx, c, callback)
			if err == nil {
				key := multiTierCacheKey(c)
				mtbs.updateAccessInfo(key, tier, 0) // Size will be determined in callback
				mtbs.updateTierStats(tier, true, 0)
				return nil
			}
			
			if !ipld.IsNotFound(err) {
				return err
			}
		}
	}
	
	return ipld.ErrNotFound{Cid: c}
}

// PromoteBlock moves a block to a higher tier based on access patterns
func (mtbs *multiTierBlockstore) PromoteBlock(ctx context.Context, c cid.Cid) error {
	if !c.Defined() {
		return errors.New("invalid CID")
	}
	
	key := multiTierCacheKey(c)
	mtbs.accessMu.RLock()
	info, exists := mtbs.accessInfo[key]
	mtbs.accessMu.RUnlock()
	
	if !exists {
		return errors.New("block not found in access info")
	}
	
	// Determine target tier for promotion
	var targetTier CacheTier
	switch info.CurrentTier {
	case HDDTier:
		targetTier = SSDTier
	case SSDTier:
		targetTier = MemoryTier
	case MemoryTier:
		return nil // Already in top tier
	default:
		return errors.New("unknown current tier")
	}
	
	// Check if target tier exists and is enabled
	targetBS, exists := mtbs.tiers[targetTier]
	if !exists || !mtbs.config.Tiers[targetTier].Enabled {
		return errors.New("target tier not available")
	}
	
	// Get the block from current tier
	currentBS := mtbs.tiers[info.CurrentTier]
	block, err := currentBS.Get(ctx, c)
	if err != nil {
		return err
	}
	
	// Check if target tier has space
	if !mtbs.hasTierSpace(targetTier, len(block.RawData())) {
		if mtbs.config.AutoDemote {
			if err := mtbs.makeSpace(ctx, targetTier, len(block.RawData())); err != nil {
				return err
			}
		} else {
			return errors.New("target tier is full")
		}
	}
	
	// Put block in target tier
	if err := targetBS.Put(ctx, block); err != nil {
		return err
	}
	
	// Remove from current tier
	if err := currentBS.DeleteBlock(ctx, c); err != nil {
		// If deletion fails, try to remove from target tier to maintain consistency
		targetBS.DeleteBlock(ctx, c)
		return err
	}
	
	// Update access info and stats
	mtbs.updateAccessInfo(key, targetTier, len(block.RawData()))
	
	mtbs.statsMu.Lock()
	if stats, exists := mtbs.stats[info.CurrentTier]; exists {
		stats.Promotions++
	}
	mtbs.statsMu.Unlock()
	
	mtbs.metrics.promotions.Inc()
	
	logger.Debugf("promoted block %s from %s to %s", c, info.CurrentTier, targetTier)
	return nil
}

// DemoteBlock moves a block to a lower tier to free up space
func (mtbs *multiTierBlockstore) DemoteBlock(ctx context.Context, c cid.Cid) error {
	if !c.Defined() {
		return errors.New("invalid CID")
	}
	
	key := multiTierCacheKey(c)
	mtbs.accessMu.RLock()
	info, exists := mtbs.accessInfo[key]
	mtbs.accessMu.RUnlock()
	
	if !exists {
		return errors.New("block not found in access info")
	}
	
	// Determine target tier for demotion
	var targetTier CacheTier
	switch info.CurrentTier {
	case MemoryTier:
		targetTier = SSDTier
	case SSDTier:
		targetTier = HDDTier
	case HDDTier:
		return nil // Already in lowest tier
	default:
		return errors.New("unknown current tier")
	}
	
	// Check if target tier exists and is enabled
	targetBS, exists := mtbs.tiers[targetTier]
	if !exists || !mtbs.config.Tiers[targetTier].Enabled {
		return errors.New("target tier not available")
	}
	
	// Get the block from current tier
	currentBS := mtbs.tiers[info.CurrentTier]
	block, err := currentBS.Get(ctx, c)
	if err != nil {
		return err
	}
	
	// Put block in target tier
	if err := targetBS.Put(ctx, block); err != nil {
		return err
	}
	
	// Remove from current tier
	if err := currentBS.DeleteBlock(ctx, c); err != nil {
		// If deletion fails, try to remove from target tier to maintain consistency
		targetBS.DeleteBlock(ctx, c)
		return err
	}
	
	// Update access info and stats
	mtbs.updateAccessInfo(key, targetTier, len(block.RawData()))
	
	mtbs.statsMu.Lock()
	if stats, exists := mtbs.stats[targetTier]; exists {
		stats.Demotions++
	}
	mtbs.statsMu.Unlock()
	
	mtbs.metrics.demotions.Inc()
	
	logger.Debugf("demoted block %s from %s to %s", c, info.CurrentTier, targetTier)
	return nil
}

// considerPromotion checks if a block should be promoted based on access patterns
func (mtbs *multiTierBlockstore) considerPromotion(ctx context.Context, c cid.Cid, key string) error {
	mtbs.accessMu.RLock()
	info, exists := mtbs.accessInfo[key]
	mtbs.accessMu.RUnlock()
	
	if !exists {
		return nil
	}
	
	// Check if block meets promotion threshold
	tierConfig := mtbs.config.Tiers[info.CurrentTier]
	if tierConfig == nil || info.Count < tierConfig.PromotionThreshold {
		return nil
	}
	
	// Promote the block
	return mtbs.PromoteBlock(ctx, c)
}

// hasTierSpace checks if a tier has enough space for additional data
func (mtbs *multiTierBlockstore) hasTierSpace(tier CacheTier, additionalSize int) bool {
	config := mtbs.config.Tiers[tier]
	if config == nil {
		return false
	}
	
	mtbs.statsMu.RLock()
	stats := mtbs.stats[tier]
	mtbs.statsMu.RUnlock()
	
	// Check size limit
	if config.MaxSize > 0 && stats.Size+int64(additionalSize) > config.MaxSize {
		return false
	}
	
	// Check item count limit
	if config.MaxItems > 0 && stats.Items >= config.MaxItems {
		return false
	}
	
	return true
}

// makeSpace attempts to free up space in a tier by demoting old blocks
func (mtbs *multiTierBlockstore) makeSpace(ctx context.Context, tier CacheTier, neededSize int) error {
	// Find candidates for demotion (least recently used blocks)
	candidates := mtbs.findDemotionCandidates(tier, neededSize)
	
	for _, candidate := range candidates {
		c, err := cid.Parse(candidate)
		if err != nil {
			continue
		}
		
		if err := mtbs.DemoteBlock(ctx, c); err != nil {
			logger.Debugf("failed to demote block %s: %v", c, err)
			continue
		}
		
		// Check if we've freed enough space
		if mtbs.hasTierSpace(tier, neededSize) {
			return nil
		}
	}
	
	return errors.New("unable to free sufficient space")
}

// findDemotionCandidates finds blocks that are good candidates for demotion
func (mtbs *multiTierBlockstore) findDemotionCandidates(tier CacheTier, neededSize int) []string {
	mtbs.accessMu.RLock()
	defer mtbs.accessMu.RUnlock()
	
	type candidate struct {
		key        string
		lastAccess time.Time
		size       int
	}
	
	var candidates []candidate
	
	// Find blocks in the specified tier
	for key, info := range mtbs.accessInfo {
		if info.CurrentTier == tier {
			candidates = append(candidates, candidate{
				key:        key,
				lastAccess: info.LastAccess,
				size:       info.Size,
			})
		}
	}
	
	// Sort by last access time (oldest first)
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[i].lastAccess.After(candidates[j].lastAccess) {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	
	// Return keys of candidates that would free enough space
	var result []string
	freedSize := 0
	for _, candidate := range candidates {
		result = append(result, candidate.key)
		freedSize += candidate.size
		if freedSize >= neededSize {
			break
		}
	}
	
	return result
}

// GetTierStats returns statistics for each tier
func (mtbs *multiTierBlockstore) GetTierStats() map[CacheTier]*TierStats {
	mtbs.statsMu.RLock()
	defer mtbs.statsMu.RUnlock()
	
	// Return a copy to avoid race conditions
	result := make(map[CacheTier]*TierStats)
	for tier, stats := range mtbs.stats {
		result[tier] = &TierStats{
			Size:       stats.Size,
			Items:      stats.Items,
			Hits:       stats.Hits,
			Misses:     stats.Misses,
			Promotions: stats.Promotions,
			Demotions:  stats.Demotions,
			LastAccess: stats.LastAccess,
		}
	}
	
	return result
}

// Compact performs maintenance operations on all tiers
func (mtbs *multiTierBlockstore) Compact(ctx context.Context) error {
	logger.Debug("starting multi-tier blockstore compaction")
	
	// Clean up expired access info
	mtbs.cleanupExpiredAccessInfo()
	
	// Perform tier-specific maintenance
	for tier, bs := range mtbs.tiers {
		if compactor, ok := bs.(interface{ Compact(context.Context) error }); ok {
			if err := compactor.Compact(ctx); err != nil {
				logger.Debugf("compaction failed for tier %s: %v", tier, err)
			}
		}
	}
	
	logger.Debug("multi-tier blockstore compaction completed")
	return nil
}

// cleanupExpiredAccessInfo removes old access information based on TTL
func (mtbs *multiTierBlockstore) cleanupExpiredAccessInfo() {
	mtbs.accessMu.Lock()
	defer mtbs.accessMu.Unlock()
	
	now := time.Now()
	
	for key, info := range mtbs.accessInfo {
		tierConfig := mtbs.config.Tiers[info.CurrentTier]
		if tierConfig != nil && tierConfig.TTL > 0 {
			if now.Sub(info.LastAccess) > tierConfig.TTL {
				delete(mtbs.accessInfo, key)
			}
		}
	}
}

// compactionRoutine runs periodic maintenance operations
func (mtbs *multiTierBlockstore) compactionRoutine(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-mtbs.compactionStop:
			return
		case <-mtbs.compactionTicker.C:
			if err := mtbs.Compact(ctx); err != nil {
				logger.Debugf("compaction routine error: %v", err)
			}
		}
	}
}

// Close shuts down the multi-tier blockstore
func (mtbs *multiTierBlockstore) Close() error {
	if mtbs.compactionTicker != nil {
		mtbs.compactionTicker.Stop()
	}
	
	close(mtbs.compactionStop)
	
	// Close underlying blockstores if they support it
	for tier, bs := range mtbs.tiers {
		if closer, ok := bs.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				logger.Debugf("failed to close tier %s: %v", tier, err)
			}
		}
	}
	
	return nil
}

// GC-related methods for compatibility with GCBlockstore interface
func (mtbs *multiTierBlockstore) GCLock(ctx context.Context) Unlocker {
	// Lock all tiers for GC
	var unlockers []Unlocker
	
	for _, bs := range mtbs.tiers {
		if gcbs, ok := bs.(GCBlockstore); ok {
			unlockers = append(unlockers, gcbs.GCLock(ctx))
		}
	}
	
	return &multiTierUnlocker{unlockers: unlockers}
}

func (mtbs *multiTierBlockstore) PinLock(ctx context.Context) Unlocker {
	// Lock all tiers for pinning
	var unlockers []Unlocker
	
	for _, bs := range mtbs.tiers {
		if gcbs, ok := bs.(GCBlockstore); ok {
			unlockers = append(unlockers, gcbs.PinLock(ctx))
		}
	}
	
	return &multiTierUnlocker{unlockers: unlockers}
}

func (mtbs *multiTierBlockstore) GCRequested(ctx context.Context) bool {
	// Check if any tier has GC requested
	for _, bs := range mtbs.tiers {
		if gcbs, ok := bs.(GCBlockstore); ok {
			if gcbs.GCRequested(ctx) {
				return true
			}
		}
	}
	return false
}

// multiTierUnlocker unlocks multiple tiers
type multiTierUnlocker struct {
	unlockers []Unlocker
}

func (mtu *multiTierUnlocker) Unlock(ctx context.Context) {
	for _, unlocker := range mtu.unlockers {
		unlocker.Unlock(ctx)
	}
}