package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PerformanceMonitor defines the interface for collecting and analyzing performance metrics
type PerformanceMonitor interface {
	// CollectMetrics gathers current performance metrics from all components
	CollectMetrics() *PerformanceMetrics
	
	// StartCollection begins continuous metric collection
	StartCollection(ctx context.Context, interval time.Duration) error
	
	// StopCollection stops metric collection
	StopCollection() error
	
	// RegisterCollector adds a custom metric collector
	RegisterCollector(name string, collector MetricCollector) error
	
	// GetPrometheusRegistry returns the Prometheus registry for external use
	GetPrometheusRegistry() *prometheus.Registry
}

// MetricCollector defines interface for component-specific metric collectors
type MetricCollector interface {
	CollectMetrics(ctx context.Context) (map[string]interface{}, error)
	GetMetricNames() []string
}

// PerformanceMetrics contains all collected performance metrics
type PerformanceMetrics struct {
	Timestamp         time.Time         `json:"timestamp"`
	BitswapMetrics    BitswapStats      `json:"bitswap_metrics"`
	BlockstoreMetrics BlockstoreStats   `json:"blockstore_metrics"`
	NetworkMetrics    NetworkStats      `json:"network_metrics"`
	ResourceMetrics   ResourceStats     `json:"resource_metrics"`
	CustomMetrics     map[string]interface{} `json:"custom_metrics"`
}

// BitswapStats contains Bitswap-specific performance metrics
type BitswapStats struct {
	RequestsPerSecond     float64       `json:"requests_per_second"`
	AverageResponseTime   time.Duration `json:"average_response_time"`
	P95ResponseTime       time.Duration `json:"p95_response_time"`
	P99ResponseTime       time.Duration `json:"p99_response_time"`
	OutstandingRequests   int64         `json:"outstanding_requests"`
	ActiveConnections     int           `json:"active_connections"`
	QueuedRequests        int64         `json:"queued_requests"`
	BlocksReceived        int64         `json:"blocks_received"`
	BlocksSent            int64         `json:"blocks_sent"`
	DuplicateBlocks       int64         `json:"duplicate_blocks"`
	ErrorRate             float64       `json:"error_rate"`
}

// BlockstoreStats contains blockstore performance metrics
type BlockstoreStats struct {
	CacheHitRate          float64       `json:"cache_hit_rate"`
	AverageReadLatency    time.Duration `json:"average_read_latency"`
	AverageWriteLatency   time.Duration `json:"average_write_latency"`
	BatchOperationsPerSec float64       `json:"batch_operations_per_sec"`
	CompressionRatio      float64       `json:"compression_ratio"`
	TotalBlocks           int64         `json:"total_blocks"`
	CacheSize             int64         `json:"cache_size"`
	DiskUsage             int64         `json:"disk_usage"`
}

// NetworkStats contains network layer performance metrics
type NetworkStats struct {
	ActiveConnections     int                    `json:"active_connections"`
	TotalBandwidth        int64                  `json:"total_bandwidth"`
	AverageLatency        time.Duration          `json:"average_latency"`
	PacketLossRate        float64                `json:"packet_loss_rate"`
	ConnectionErrors      int64                  `json:"connection_errors"`
	BytesSent             int64                  `json:"bytes_sent"`
	BytesReceived         int64                  `json:"bytes_received"`
	PeerConnections       map[string]PeerMetrics `json:"peer_connections"`
}

// PeerMetrics contains per-peer connection metrics
type PeerMetrics struct {
	Latency       time.Duration `json:"latency"`
	Bandwidth     int64         `json:"bandwidth"`
	ErrorRate     float64       `json:"error_rate"`
	LastSeen      time.Time     `json:"last_seen"`
	BytesSent     int64         `json:"bytes_sent"`
	BytesReceived int64         `json:"bytes_received"`
}

// ResourceStats contains system resource usage metrics
type ResourceStats struct {
	CPUUsage      float64 `json:"cpu_usage"`
	MemoryUsage   int64   `json:"memory_usage"`
	MemoryTotal   int64   `json:"memory_total"`
	DiskUsage     int64   `json:"disk_usage"`
	DiskTotal     int64   `json:"disk_total"`
	GoroutineCount int    `json:"goroutine_count"`
	GCPauseTime   time.Duration `json:"gc_pause_time"`
}

// performanceMonitor implements the PerformanceMonitor interface
type performanceMonitor struct {
	mu                sync.RWMutex
	registry          *prometheus.Registry
	collectors        map[string]MetricCollector
	isCollecting      bool
	stopChan          chan struct{}
	
	// Prometheus metrics
	bitswapRequestsTotal     prometheus.Counter
	bitswapResponseTime      prometheus.Histogram
	bitswapActiveConnections prometheus.Gauge
	bitswapQueuedRequests    prometheus.Gauge
	
	blockstoreCacheHitRate   prometheus.Gauge
	blockstoreReadLatency    prometheus.Histogram
	blockstoreWriteLatency   prometheus.Histogram
	blockstoreTotalBlocks    prometheus.Gauge
	
	networkActiveConnections prometheus.Gauge
	networkBandwidth         prometheus.Gauge
	networkLatency           prometheus.Histogram
	networkPacketLoss        prometheus.Gauge
	
	resourceCPUUsage         prometheus.Gauge
	resourceMemoryUsage      prometheus.Gauge
	resourceDiskUsage        prometheus.Gauge
	resourceGoroutines       prometheus.Gauge
}

// NewPerformanceMonitor creates a new performance monitor instance
func NewPerformanceMonitor() PerformanceMonitor {
	registry := prometheus.NewRegistry()
	
	pm := &performanceMonitor{
		registry:   registry,
		collectors: make(map[string]MetricCollector),
		stopChan:   make(chan struct{}),
	}
	
	pm.initPrometheusMetrics()
	return pm
}

// initPrometheusMetrics initializes all Prometheus metrics
func (pm *performanceMonitor) initPrometheusMetrics() {
	// Bitswap metrics
	pm.bitswapRequestsTotal = promauto.With(pm.registry).NewCounter(prometheus.CounterOpts{
		Namespace: "boxo",
		Subsystem: "bitswap",
		Name:      "requests_total",
		Help:      "Total number of Bitswap requests processed",
	})
	
	pm.bitswapResponseTime = promauto.With(pm.registry).NewHistogram(prometheus.HistogramOpts{
		Namespace: "boxo",
		Subsystem: "bitswap",
		Name:      "response_time_seconds",
		Help:      "Bitswap request response time in seconds",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	})
	
	pm.bitswapActiveConnections = promauto.With(pm.registry).NewGauge(prometheus.GaugeOpts{
		Namespace: "boxo",
		Subsystem: "bitswap",
		Name:      "active_connections",
		Help:      "Number of active Bitswap connections",
	})
	
	pm.bitswapQueuedRequests = promauto.With(pm.registry).NewGauge(prometheus.GaugeOpts{
		Namespace: "boxo",
		Subsystem: "bitswap",
		Name:      "queued_requests",
		Help:      "Number of queued Bitswap requests",
	})
	
	// Blockstore metrics
	pm.blockstoreCacheHitRate = promauto.With(pm.registry).NewGauge(prometheus.GaugeOpts{
		Namespace: "boxo",
		Subsystem: "blockstore",
		Name:      "cache_hit_rate",
		Help:      "Blockstore cache hit rate (0-1)",
	})
	
	pm.blockstoreReadLatency = promauto.With(pm.registry).NewHistogram(prometheus.HistogramOpts{
		Namespace: "boxo",
		Subsystem: "blockstore",
		Name:      "read_latency_seconds",
		Help:      "Blockstore read operation latency in seconds",
		Buckets:   []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
	})
	
	pm.blockstoreWriteLatency = promauto.With(pm.registry).NewHistogram(prometheus.HistogramOpts{
		Namespace: "boxo",
		Subsystem: "blockstore",
		Name:      "write_latency_seconds",
		Help:      "Blockstore write operation latency in seconds",
		Buckets:   []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
	})
	
	pm.blockstoreTotalBlocks = promauto.With(pm.registry).NewGauge(prometheus.GaugeOpts{
		Namespace: "boxo",
		Subsystem: "blockstore",
		Name:      "total_blocks",
		Help:      "Total number of blocks in blockstore",
	})
	
	// Network metrics
	pm.networkActiveConnections = promauto.With(pm.registry).NewGauge(prometheus.GaugeOpts{
		Namespace: "boxo",
		Subsystem: "network",
		Name:      "active_connections",
		Help:      "Number of active network connections",
	})
	
	pm.networkBandwidth = promauto.With(pm.registry).NewGauge(prometheus.GaugeOpts{
		Namespace: "boxo",
		Subsystem: "network",
		Name:      "bandwidth_bytes_per_second",
		Help:      "Current network bandwidth in bytes per second",
	})
	
	pm.networkLatency = promauto.With(pm.registry).NewHistogram(prometheus.HistogramOpts{
		Namespace: "boxo",
		Subsystem: "network",
		Name:      "latency_seconds",
		Help:      "Network latency in seconds",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	})
	
	pm.networkPacketLoss = promauto.With(pm.registry).NewGauge(prometheus.GaugeOpts{
		Namespace: "boxo",
		Subsystem: "network",
		Name:      "packet_loss_rate",
		Help:      "Network packet loss rate (0-1)",
	})
	
	// Resource metrics
	pm.resourceCPUUsage = promauto.With(pm.registry).NewGauge(prometheus.GaugeOpts{
		Namespace: "boxo",
		Subsystem: "resource",
		Name:      "cpu_usage_percent",
		Help:      "CPU usage percentage (0-100)",
	})
	
	pm.resourceMemoryUsage = promauto.With(pm.registry).NewGauge(prometheus.GaugeOpts{
		Namespace: "boxo",
		Subsystem: "resource",
		Name:      "memory_usage_bytes",
		Help:      "Memory usage in bytes",
	})
	
	pm.resourceDiskUsage = promauto.With(pm.registry).NewGauge(prometheus.GaugeOpts{
		Namespace: "boxo",
		Subsystem: "resource",
		Name:      "disk_usage_bytes",
		Help:      "Disk usage in bytes",
	})
	
	pm.resourceGoroutines = promauto.With(pm.registry).NewGauge(prometheus.GaugeOpts{
		Namespace: "boxo",
		Subsystem: "resource",
		Name:      "goroutines_total",
		Help:      "Total number of goroutines",
	})
}

// CollectMetrics gathers current performance metrics from all components
func (pm *performanceMonitor) CollectMetrics() *PerformanceMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	metrics := &PerformanceMetrics{
		Timestamp:     time.Now(),
		CustomMetrics: make(map[string]interface{}),
	}
	
	// Collect metrics from registered collectors
	for name, collector := range pm.collectors {
		if collectedMetrics, err := collector.CollectMetrics(context.Background()); err == nil {
			metrics.CustomMetrics[name] = collectedMetrics
		}
	}
	
	// Update Prometheus metrics based on collected data
	pm.updatePrometheusMetrics(metrics)
	
	return metrics
}

// updatePrometheusMetrics updates Prometheus metrics with collected data
func (pm *performanceMonitor) updatePrometheusMetrics(metrics *PerformanceMetrics) {
	// Update Bitswap metrics from custom metrics
	if bitswapMetrics, ok := metrics.CustomMetrics["bitswap"].(map[string]interface{}); ok {
		if activeConns, ok := bitswapMetrics["active_connections"].(int); ok {
			pm.bitswapActiveConnections.Set(float64(activeConns))
		}
		if queuedReqs, ok := bitswapMetrics["queued_requests"].(int64); ok {
			pm.bitswapQueuedRequests.Set(float64(queuedReqs))
		}
	}
	
	// Update Blockstore metrics from custom metrics
	if blockstoreMetrics, ok := metrics.CustomMetrics["blockstore"].(map[string]interface{}); ok {
		if cacheHitRate, ok := blockstoreMetrics["cache_hit_rate"].(float64); ok {
			pm.blockstoreCacheHitRate.Set(cacheHitRate)
		}
		if totalBlocks, ok := blockstoreMetrics["total_blocks"].(int64); ok {
			pm.blockstoreTotalBlocks.Set(float64(totalBlocks))
		}
	}
	
	// Update Network metrics from custom metrics
	if networkMetrics, ok := metrics.CustomMetrics["network"].(map[string]interface{}); ok {
		if activeConns, ok := networkMetrics["active_connections"].(int); ok {
			pm.networkActiveConnections.Set(float64(activeConns))
		}
		if bandwidth, ok := networkMetrics["total_bandwidth"].(int64); ok {
			pm.networkBandwidth.Set(float64(bandwidth))
		}
		if packetLoss, ok := networkMetrics["packet_loss_rate"].(float64); ok {
			pm.networkPacketLoss.Set(packetLoss)
		}
	}
	
	// Update Resource metrics from custom metrics
	if resourceMetrics, ok := metrics.CustomMetrics["resource"].(map[string]interface{}); ok {
		if cpuUsage, ok := resourceMetrics["cpu_usage"].(float64); ok {
			pm.resourceCPUUsage.Set(cpuUsage)
		}
		if memUsage, ok := resourceMetrics["memory_usage"].(int64); ok {
			pm.resourceMemoryUsage.Set(float64(memUsage))
		}
		if diskUsage, ok := resourceMetrics["disk_usage"].(int64); ok {
			pm.resourceDiskUsage.Set(float64(diskUsage))
		}
		if goroutines, ok := resourceMetrics["goroutine_count"].(int); ok {
			pm.resourceGoroutines.Set(float64(goroutines))
		}
	}
}

// StartCollection begins continuous metric collection
func (pm *performanceMonitor) StartCollection(ctx context.Context, interval time.Duration) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	if pm.isCollecting {
		return nil // Already collecting
	}
	
	pm.isCollecting = true
	
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-pm.stopChan:
				return
			case <-ticker.C:
				pm.CollectMetrics()
			}
		}
	}()
	
	return nil
}

// StopCollection stops metric collection
func (pm *performanceMonitor) StopCollection() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	if !pm.isCollecting {
		return nil // Not collecting
	}
	
	pm.isCollecting = false
	close(pm.stopChan)
	pm.stopChan = make(chan struct{})
	
	return nil
}

// RegisterCollector adds a custom metric collector
func (pm *performanceMonitor) RegisterCollector(name string, collector MetricCollector) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.collectors[name] = collector
	return nil
}

// GetPrometheusRegistry returns the Prometheus registry for external use
func (pm *performanceMonitor) GetPrometheusRegistry() *prometheus.Registry {
	return pm.registry
}