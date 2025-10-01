package monitoring

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Built-in degradation actions

// ReduceWorkerThreadsAction reduces the number of worker threads
type ReduceWorkerThreadsAction struct {
	mu              sync.RWMutex
	isActive        bool
	originalThreads int
	reducedThreads  int
	reductionFactor float64
}

func (a *ReduceWorkerThreadsAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isActive {
		return nil // Already active
	}

	// Simulate reducing worker threads based on degradation level
	a.originalThreads = 128 // Default worker count

	switch level {
	case DegradationLight:
		a.reductionFactor = 0.8 // Reduce to 80%
	case DegradationModerate:
		a.reductionFactor = 0.6 // Reduce to 60%
	case DegradationSevere:
		a.reductionFactor = 0.4 // Reduce to 40%
	case DegradationCritical:
		a.reductionFactor = 0.2 // Reduce to 20%
	default:
		a.reductionFactor = 1.0
	}

	a.reducedThreads = int(float64(a.originalThreads) * a.reductionFactor)
	a.isActive = true

	slog.Info("Reduced worker threads",
		slog.Int("original", a.originalThreads),
		slog.Int("reduced", a.reducedThreads),
		slog.String("level", level.String()),
	)

	return nil
}

func (a *ReduceWorkerThreadsAction) Recover(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isActive {
		return nil // Not active
	}

	a.isActive = false

	slog.Info("Restored worker threads",
		slog.Int("restored", a.originalThreads),
	)

	return nil
}

func (a *ReduceWorkerThreadsAction) GetName() string {
	return "reduce_worker_threads"
}

func (a *ReduceWorkerThreadsAction) IsActive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isActive
}

// IncreaseBatchTimeoutAction increases batch processing timeouts
type IncreaseBatchTimeoutAction struct {
	mu               sync.RWMutex
	isActive         bool
	originalTimeout  time.Duration
	increasedTimeout time.Duration
}

func (a *IncreaseBatchTimeoutAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isActive {
		return nil
	}

	a.originalTimeout = 10 * time.Millisecond // Default timeout

	switch level {
	case DegradationLight:
		a.increasedTimeout = 20 * time.Millisecond
	case DegradationModerate:
		a.increasedTimeout = 50 * time.Millisecond
	case DegradationSevere:
		a.increasedTimeout = 100 * time.Millisecond
	case DegradationCritical:
		a.increasedTimeout = 200 * time.Millisecond
	default:
		a.increasedTimeout = a.originalTimeout
	}

	a.isActive = true

	slog.Info("Increased batch timeout",
		slog.Duration("original", a.originalTimeout),
		slog.Duration("increased", a.increasedTimeout),
		slog.String("level", level.String()),
	)

	return nil
}

func (a *IncreaseBatchTimeoutAction) Recover(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isActive {
		return nil
	}

	a.isActive = false

	slog.Info("Restored batch timeout",
		slog.Duration("restored", a.originalTimeout),
	)

	return nil
}

func (a *IncreaseBatchTimeoutAction) GetName() string {
	return "increase_batch_timeout"
}

func (a *IncreaseBatchTimeoutAction) IsActive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isActive
}

// ClearCachesAction clears various caches to free memory
type ClearCachesAction struct {
	mu            sync.RWMutex
	isActive      bool
	clearedCaches []string
}

func (a *ClearCachesAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isActive {
		return nil
	}

	a.clearedCaches = []string{}

	switch level {
	case DegradationLight:
		a.clearedCaches = []string{"lru_cache"}
	case DegradationModerate:
		a.clearedCaches = []string{"lru_cache", "bloom_cache"}
	case DegradationSevere:
		a.clearedCaches = []string{"lru_cache", "bloom_cache", "memory_cache"}
	case DegradationCritical:
		a.clearedCaches = []string{"lru_cache", "bloom_cache", "memory_cache", "disk_cache"}
	}

	a.isActive = true

	slog.Info("Cleared caches",
		slog.Any("caches", a.clearedCaches),
		slog.String("level", level.String()),
	)

	return nil
}

func (a *ClearCachesAction) Recover(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isActive {
		return nil
	}

	a.isActive = false

	slog.Info("Cache clearing action recovered",
		slog.Any("previously_cleared", a.clearedCaches),
	)

	return nil
}

func (a *ClearCachesAction) GetName() string {
	return "clear_caches"
}

func (a *ClearCachesAction) IsActive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isActive
}

// ReduceBufferSizesAction reduces buffer sizes to conserve memory
type ReduceBufferSizesAction struct {
	mu            sync.RWMutex
	isActive      bool
	originalSizes map[string]int
	reducedSizes  map[string]int
}

func (a *ReduceBufferSizesAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isActive {
		return nil
	}

	a.originalSizes = map[string]int{
		"read_buffer":  64 * 1024, // 64KB
		"write_buffer": 64 * 1024, // 64KB
		"net_buffer":   32 * 1024, // 32KB
	}

	a.reducedSizes = make(map[string]int)

	var reductionFactor float64
	switch level {
	case DegradationLight:
		reductionFactor = 0.75
	case DegradationModerate:
		reductionFactor = 0.5
	case DegradationSevere:
		reductionFactor = 0.25
	case DegradationCritical:
		reductionFactor = 0.125
	default:
		reductionFactor = 1.0
	}

	for bufferType, originalSize := range a.originalSizes {
		a.reducedSizes[bufferType] = int(float64(originalSize) * reductionFactor)
	}

	a.isActive = true

	slog.Info("Reduced buffer sizes",
		slog.Any("original", a.originalSizes),
		slog.Any("reduced", a.reducedSizes),
		slog.String("level", level.String()),
	)

	return nil
}

func (a *ReduceBufferSizesAction) Recover(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isActive {
		return nil
	}

	a.isActive = false

	slog.Info("Restored buffer sizes",
		slog.Any("restored", a.originalSizes),
	)

	return nil
}

func (a *ReduceBufferSizesAction) GetName() string {
	return "reduce_buffer_sizes"
}

func (a *ReduceBufferSizesAction) IsActive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isActive
}

// LimitConcurrentOperationsAction limits the number of concurrent operations
type LimitConcurrentOperationsAction struct {
	mu             sync.RWMutex
	isActive       bool
	originalLimits map[string]int
	reducedLimits  map[string]int
}

func (a *LimitConcurrentOperationsAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isActive {
		return nil
	}

	a.originalLimits = map[string]int{
		"bitswap_requests":    1000,
		"blockstore_reads":    500,
		"blockstore_writes":   200,
		"network_connections": 2000,
	}

	a.reducedLimits = make(map[string]int)

	var reductionFactor float64
	switch level {
	case DegradationLight:
		reductionFactor = 0.8
	case DegradationModerate:
		reductionFactor = 0.6
	case DegradationSevere:
		reductionFactor = 0.4
	case DegradationCritical:
		reductionFactor = 0.2
	default:
		reductionFactor = 1.0
	}

	for operation, originalLimit := range a.originalLimits {
		a.reducedLimits[operation] = int(float64(originalLimit) * reductionFactor)
	}

	a.isActive = true

	slog.Info("Limited concurrent operations",
		slog.Any("original", a.originalLimits),
		slog.Any("reduced", a.reducedLimits),
		slog.String("level", level.String()),
	)

	return nil
}

func (a *LimitConcurrentOperationsAction) Recover(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isActive {
		return nil
	}

	a.isActive = false

	slog.Info("Restored concurrent operation limits",
		slog.Any("restored", a.originalLimits),
	)

	return nil
}

func (a *LimitConcurrentOperationsAction) GetName() string {
	return "limit_concurrent_operations"
}

func (a *LimitConcurrentOperationsAction) IsActive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isActive
}

// DisableNonCriticalFeaturesAction disables non-critical features
type DisableNonCriticalFeaturesAction struct {
	mu               sync.RWMutex
	isActive         bool
	disabledFeatures []string
}

func (a *DisableNonCriticalFeaturesAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isActive {
		return nil
	}

	switch level {
	case DegradationLight:
		a.disabledFeatures = []string{"metrics_collection", "debug_logging"}
	case DegradationModerate:
		a.disabledFeatures = []string{"metrics_collection", "debug_logging", "tracing"}
	case DegradationSevere:
		a.disabledFeatures = []string{"metrics_collection", "debug_logging", "tracing", "compression"}
	case DegradationCritical:
		a.disabledFeatures = []string{"metrics_collection", "debug_logging", "tracing", "compression", "background_tasks"}
	}

	a.isActive = true

	slog.Info("Disabled non-critical features",
		slog.Any("features", a.disabledFeatures),
		slog.String("level", level.String()),
	)

	return nil
}

func (a *DisableNonCriticalFeaturesAction) Recover(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isActive {
		return nil
	}

	a.isActive = false

	slog.Info("Re-enabled features",
		slog.Any("features", a.disabledFeatures),
	)

	return nil
}

func (a *DisableNonCriticalFeaturesAction) GetName() string {
	return "disable_non_critical_features"
}

func (a *DisableNonCriticalFeaturesAction) IsActive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isActive
}

// ReduceConnectionLimitsAction reduces connection limits
type ReduceConnectionLimitsAction struct {
	mu             sync.RWMutex
	isActive       bool
	originalLimits map[string]int
	reducedLimits  map[string]int
}

func (a *ReduceConnectionLimitsAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isActive {
		return nil
	}

	a.originalLimits = map[string]int{
		"high_water": 2000,
		"low_water":  1000,
		"max_peers":  500,
	}

	a.reducedLimits = make(map[string]int)

	var reductionFactor float64
	switch level {
	case DegradationLight:
		reductionFactor = 0.8
	case DegradationModerate:
		reductionFactor = 0.6
	case DegradationSevere:
		reductionFactor = 0.4
	case DegradationCritical:
		reductionFactor = 0.2
	default:
		reductionFactor = 1.0
	}

	for limitType, originalLimit := range a.originalLimits {
		a.reducedLimits[limitType] = int(float64(originalLimit) * reductionFactor)
	}

	a.isActive = true

	slog.Info("Reduced connection limits",
		slog.Any("original", a.originalLimits),
		slog.Any("reduced", a.reducedLimits),
		slog.String("level", level.String()),
	)

	return nil
}

func (a *ReduceConnectionLimitsAction) Recover(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isActive {
		return nil
	}

	a.isActive = false

	slog.Info("Restored connection limits",
		slog.Any("restored", a.originalLimits),
	)

	return nil
}

func (a *ReduceConnectionLimitsAction) GetName() string {
	return "reduce_connection_limits"
}

func (a *ReduceConnectionLimitsAction) IsActive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isActive
}

// EmergencyCacheClearAction performs emergency cache clearing
type EmergencyCacheClearAction struct {
	mu       sync.RWMutex
	isActive bool
}

func (a *EmergencyCacheClearAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isActive {
		return nil
	}

	a.isActive = true

	slog.Warn("Emergency cache clear executed",
		slog.String("level", level.String()),
		slog.Float64("memory_usage", float64(metrics.ResourceMetrics.MemoryUsage)/float64(metrics.ResourceMetrics.MemoryTotal)*100),
	)

	return nil
}

func (a *EmergencyCacheClearAction) Recover(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isActive {
		return nil
	}

	a.isActive = false

	slog.Info("Emergency cache clear action recovered")

	return nil
}

func (a *EmergencyCacheClearAction) GetName() string {
	return "emergency_cache_clear"
}

func (a *EmergencyCacheClearAction) IsActive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isActive
}

// DisableNonEssentialServicesAction disables non-essential services
type DisableNonEssentialServicesAction struct {
	mu               sync.RWMutex
	isActive         bool
	disabledServices []string
}

func (a *DisableNonEssentialServicesAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isActive {
		return nil
	}

	a.disabledServices = []string{
		"metrics_server",
		"debug_server",
		"profiling_server",
		"health_check_server",
		"admin_api",
	}

	a.isActive = true

	slog.Warn("Disabled non-essential services",
		slog.Any("services", a.disabledServices),
		slog.String("level", level.String()),
	)

	return nil
}

func (a *DisableNonEssentialServicesAction) Recover(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isActive {
		return nil
	}

	a.isActive = false

	slog.Info("Re-enabled non-essential services",
		slog.Any("services", a.disabledServices),
	)

	return nil
}

func (a *DisableNonEssentialServicesAction) GetName() string {
	return "disable_non_essential_services"
}

func (a *DisableNonEssentialServicesAction) IsActive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isActive
}

// IncreaseBatchSizeAction increases batch sizes for better throughput
type IncreaseBatchSizeAction struct {
	mu             sync.RWMutex
	isActive       bool
	originalSizes  map[string]int
	increasedSizes map[string]int
}

func (a *IncreaseBatchSizeAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isActive {
		return nil
	}

	a.originalSizes = map[string]int{
		"bitswap_batch":    100,
		"blockstore_batch": 50,
		"network_batch":    25,
	}

	a.increasedSizes = make(map[string]int)

	var increaseFactor float64
	switch level {
	case DegradationLight:
		increaseFactor = 1.5
	case DegradationModerate:
		increaseFactor = 2.0
	case DegradationSevere:
		increaseFactor = 3.0
	case DegradationCritical:
		increaseFactor = 4.0
	default:
		increaseFactor = 1.0
	}

	for batchType, originalSize := range a.originalSizes {
		a.increasedSizes[batchType] = int(float64(originalSize) * increaseFactor)
	}

	a.isActive = true

	slog.Info("Increased batch sizes",
		slog.Any("original", a.originalSizes),
		slog.Any("increased", a.increasedSizes),
		slog.String("level", level.String()),
	)

	return nil
}

func (a *IncreaseBatchSizeAction) Recover(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isActive {
		return nil
	}

	a.isActive = false

	slog.Info("Restored batch sizes",
		slog.Any("restored", a.originalSizes),
	)

	return nil
}

func (a *IncreaseBatchSizeAction) GetName() string {
	return "increase_batch_size"
}

func (a *IncreaseBatchSizeAction) IsActive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isActive
}

// PrioritizeCriticalRequestsAction prioritizes critical requests
type PrioritizeCriticalRequestsAction struct {
	mu         sync.RWMutex
	isActive   bool
	priorities map[string]int
}

func (a *PrioritizeCriticalRequestsAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isActive {
		return nil
	}

	a.priorities = map[string]int{
		"critical":   1,
		"high":       2,
		"normal":     3,
		"low":        4,
		"background": 5,
	}

	a.isActive = true

	slog.Info("Enabled critical request prioritization",
		slog.Any("priorities", a.priorities),
		slog.String("level", level.String()),
	)

	return nil
}

func (a *PrioritizeCriticalRequestsAction) Recover(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isActive {
		return nil
	}

	a.isActive = false

	slog.Info("Disabled critical request prioritization")

	return nil
}

func (a *PrioritizeCriticalRequestsAction) GetName() string {
	return "prioritize_critical_requests"
}

func (a *PrioritizeCriticalRequestsAction) IsActive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isActive
}

// EnableCompressionAction enables compression to reduce bandwidth usage
type EnableCompressionAction struct {
	mu               sync.RWMutex
	isActive         bool
	compressionLevel int
}

func (a *EnableCompressionAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isActive {
		return nil
	}

	switch level {
	case DegradationLight:
		a.compressionLevel = 3
	case DegradationModerate:
		a.compressionLevel = 6
	case DegradationSevere:
		a.compressionLevel = 9
	case DegradationCritical:
		a.compressionLevel = 9
	default:
		a.compressionLevel = 1
	}

	a.isActive = true

	slog.Info("Enabled compression",
		slog.Int("level", a.compressionLevel),
		slog.String("degradation_level", level.String()),
	)

	return nil
}

func (a *EnableCompressionAction) Recover(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isActive {
		return nil
	}

	a.isActive = false

	slog.Info("Disabled compression")

	return nil
}

func (a *EnableCompressionAction) GetName() string {
	return "enable_compression"
}

func (a *EnableCompressionAction) IsActive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isActive
}

// PrioritizeCriticalTrafficAction prioritizes critical network traffic
type PrioritizeCriticalTrafficAction struct {
	mu       sync.RWMutex
	isActive bool
	qosRules map[string]int
}

func (a *PrioritizeCriticalTrafficAction) Execute(ctx context.Context, level DegradationLevel, metrics *PerformanceMetrics) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isActive {
		return nil
	}

	a.qosRules = map[string]int{
		"bitswap_critical": 1,
		"bitswap_normal":   2,
		"blockstore":       3,
		"discovery":        4,
		"maintenance":      5,
	}

	a.isActive = true

	slog.Info("Enabled critical traffic prioritization",
		slog.Any("qos_rules", a.qosRules),
		slog.String("level", level.String()),
	)

	return nil
}

func (a *PrioritizeCriticalTrafficAction) Recover(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isActive {
		return nil
	}

	a.isActive = false

	slog.Info("Disabled critical traffic prioritization")

	return nil
}

func (a *PrioritizeCriticalTrafficAction) GetName() string {
	return "prioritize_critical_traffic"
}

func (a *PrioritizeCriticalTrafficAction) IsActive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isActive
}
