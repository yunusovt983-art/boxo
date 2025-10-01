package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/config/adaptive"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// CDNService represents a high-performance CDN service using Boxo optimizations
type CDNService struct {
	config     *adaptive.HighLoadConfig
	blockstore blockstore.Blockstore
	bitswap    *bitswap.Bitswap
	metrics    *CDNMetrics
}

// CDNMetrics contains Prometheus metrics for the CDN service
type CDNMetrics struct {
	requestsTotal     prometheus.Counter
	requestDuration   prometheus.Histogram
	cacheHitRate      prometheus.Gauge
	activeConnections prometheus.Gauge
	errorRate         prometheus.Counter
}

func newCDNMetrics() *CDNMetrics {
	return &CDNMetrics{
		requestsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cdn_requests_total",
			Help: "Total number of CDN requests",
		}),
		requestDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "cdn_request_duration_seconds",
			Help:    "CDN request duration in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		cacheHitRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "cdn_cache_hit_rate",
			Help: "CDN cache hit rate",
		}),
		activeConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "cdn_active_connections",
			Help: "Number of active CDN connections",
		}),
		errorRate: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cdn_errors_total",
			Help: "Total number of CDN errors",
		}),
	}
}

func (m *CDNMetrics) register() {
	prometheus.MustRegister(m.requestsTotal)
	prometheus.MustRegister(m.requestDuration)
	prometheus.MustRegister(m.cacheHitRate)
	prometheus.MustRegister(m.activeConnections)
	prometheus.MustRegister(m.errorRate)
}

// NewCDNService creates a new CDN service with high-load optimizations
func NewCDNService() (*CDNService, error) {
	// Configure for CDN workload - high throughput, large files
	config := &adaptive.HighLoadConfig{
		Bitswap: adaptive.AdaptiveBitswapConfig{
			MaxOutstandingBytesPerPeer: 200 << 20, // 200MB for large file transfers
			MinOutstandingBytesPerPeer: 10 << 20,  // 10MB minimum
			MinWorkerCount:             512,
			MaxWorkerCount:             4096,
			BatchSize:                  2000, // Large batches for efficiency
			BatchTimeout:               time.Millisecond * 5,
			HighPriorityThreshold:      time.Millisecond * 20,
			CriticalPriorityThreshold:  time.Millisecond * 5,
		},
		Blockstore: adaptive.BlockstoreConfig{
			MemoryCacheSize:    16 << 30, // 16GB memory cache for hot content
			SSDCacheSize:       1 << 40,  // 1TB SSD cache
			BatchSize:          5000,
			BatchTimeout:       time.Millisecond * 10,
			CompressionEnabled: true,
			CompressionLevel:   9, // Maximum compression for storage efficiency
		},
		Network: adaptive.NetworkConfig{
			HighWater:          10000, // High connection limits for CDN
			LowWater:           5000,
			GracePeriod:        time.Second * 10,
			LatencyThreshold:   time.Millisecond * 50,
			BandwidthThreshold: 100 << 20, // 100MB/s
			ErrorRateThreshold: 0.001,     // 0.1% error tolerance
			KeepAliveInterval:  time.Second * 10,
			KeepAliveTimeout:   time.Second * 3,
		},
		Monitoring: adaptive.MonitoringConfig{
			MetricsInterval: time.Second * 1, // High-frequency monitoring
			AlertThresholds: adaptive.AlertThresholds{
				ResponseTimeP95: time.Millisecond * 25,
				ErrorRate:       0.0005,
				MemoryUsage:     0.95,
				CPUUsage:        0.9,
			},
			AutoTuningEnabled:  true,
			AutoTuningInterval: time.Minute * 1,
			PredictiveScaling:  true,
		},
	}

	metrics := newCDNMetrics()
	metrics.register()

	// Initialize blockstore with multi-tier caching
	bs, err := blockstore.NewMultiTierBlockstore(config.Blockstore)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockstore: %w", err)
	}

	// Initialize Bitswap with adaptive configuration
	bswap, err := bitswap.NewBitswapWithConfig(config.Bitswap, bs)
	if err != nil {
		return nil, fmt.Errorf("failed to create bitswap: %w", err)
	}

	return &CDNService{
		config:     config,
		blockstore: bs,
		bitswap:    bswap,
		metrics:    metrics,
	}, nil
}

// ServeContent handles content delivery requests
func (cdn *CDNService) ServeContent(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	cdn.metrics.requestsTotal.Inc()
	cdn.metrics.activeConnections.Inc()
	defer cdn.metrics.activeConnections.Dec()

	vars := mux.Vars(r)
	cid := vars["cid"]

	if cid == "" {
		cdn.metrics.errorRate.Inc()
		http.Error(w, "CID required", http.StatusBadRequest)
		return
	}

	// Retrieve content from IPFS network
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	block, err := cdn.blockstore.Get(ctx, cid)
	if err != nil {
		cdn.metrics.errorRate.Inc()
		http.Error(w, fmt.Sprintf("Content not found: %v", err), http.StatusNotFound)
		return
	}

	// Set appropriate headers for CDN
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("ETag", cid)

	// Stream content to client
	if _, err := w.Write(block.RawData()); err != nil {
		cdn.metrics.errorRate.Inc()
		log.Printf("Error writing response: %v", err)
		return
	}

	// Record metrics
	duration := time.Since(start)
	cdn.metrics.requestDuration.Observe(duration.Seconds())

	log.Printf("Served content %s in %v", cid, duration)
}

// HealthCheck provides health status for load balancers
func (cdn *CDNService) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
		"config": map[string]interface{}{
			"max_workers":     cdn.config.Bitswap.MaxWorkerCount,
			"memory_cache_gb": cdn.config.Blockstore.MemoryCacheSize >> 30,
			"max_connections": cdn.config.Network.HighWater,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status": "healthy", "timestamp": %d}`, time.Now().Unix())
}

// MetricsHandler exposes Prometheus metrics
func (cdn *CDNService) MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// Start starts the CDN service
func (cdn *CDNService) Start(port string) error {
	router := mux.NewRouter()

	// Content delivery endpoints
	router.HandleFunc("/content/{cid}", cdn.ServeContent).Methods("GET")
	router.HandleFunc("/health", cdn.HealthCheck).Methods("GET")
	router.Handle("/metrics", cdn.MetricsHandler()).Methods("GET")

	// Static file serving for web interface
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./static/")))

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting CDN service on port %s", port)
	log.Printf("Configuration: Workers=%d, MemCache=%dGB, MaxConn=%d",
		cdn.config.Bitswap.MaxWorkerCount,
		cdn.config.Blockstore.MemoryCacheSize>>30,
		cdn.config.Network.HighWater)

	return server.ListenAndServe()
}

func main() {
	// Create CDN service
	cdn, err := NewCDNService()
	if err != nil {
		log.Fatalf("Failed to create CDN service: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down CDN service...")
		os.Exit(0)
	}()

	// Start service
	port := os.Getenv("CDN_PORT")
	if port == "" {
		port = "8080"
	}

	if err := cdn.Start(port); err != nil {
		log.Fatalf("CDN service failed: %v", err)
	}
}
