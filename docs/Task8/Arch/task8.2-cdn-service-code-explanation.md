# Task 8.2 CDN Service Code Diagram - Detailed Explanation

## Overview
The `task8.2-cdn-service-code.puml` diagram shows the detailed code-level architecture of the CDN Service Application, representing the deepest level of the C4 model with actual Go code structures and their interactions.

## Architectural Purpose
This diagram serves as the **implementation blueprint** at the code level, showing how high-level architectural concepts are translated into specific Go types, interfaces, and functions that deliver high-performance CDN functionality.

## Code Layer Analysis

### Configuration Layer

#### CDNConfig Struct
- **Purpose**: Configuration parameters for CDN optimization
- **Real Implementation**: Core configuration structure
- **Code Evidence**:
  ```go
  type CDNConfig struct {
      MaxOutstandingBytesPerPeer int64         `json:"max_outstanding_bytes_per_peer"`
      WorkerCount               int           `json:"worker_count"`
      BatchSize                 int           `json:"batch_size"`
      CacheSize                 int64         `json:"cache_size"`
      CompressionEnabled        bool          `json:"compression_enabled"`
      PrefetchEnabled           bool          `json:"prefetch_enabled"`
      MaxConcurrentRequests     int           `json:"max_concurrent_requests"`
      RequestTimeout            time.Duration `json:"request_timeout"`
  }
  ```
- **Architecture Bridge**: Direct implementation of Task 8.1 parameter documentation

#### ConfigValidator Interface
- **Purpose**: Validates configuration parameters
- **Real Implementation**: Validation logic with business rules
- **Code Evidence**:
  ```go
  type ConfigValidator interface {
      Validate(config *CDNConfig) error
      ValidateForEnvironment(config *CDNConfig, env string) error
  }
  
  type DefaultConfigValidator struct{}
  
  func (v *DefaultConfigValidator) Validate(config *CDNConfig) error {
      if config.MaxOutstandingBytesPerPeer < 10<<20 { // 10MB minimum
          return fmt.Errorf("max_outstanding_bytes_per_peer too low for CDN")
      }
      if config.WorkerCount < 512 {
          return fmt.Errorf("worker_count too low for high-load CDN")
      }
      return nil
  }
  ```

### Service Layer

#### CDNService Struct
- **Purpose**: Main service orchestrator
- **Real Implementation**: Central coordination point
- **Code Evidence**:
  ```go
  type CDNService struct {
      config      *CDNConfig
      clusterAPI  *client.Client
      cache       *lru.Cache
      metrics     *CDNMetrics
      blockstore  blockstore.Blockstore
      bitswap     *bitswap.Bitswap
      server      *http.Server
      ctx         context.Context
      cancel      context.CancelFunc
  }
  ```

#### ContentHandler
- **Purpose**: HTTP request processing and routing
- **Real Implementation**: High-performance HTTP handling
- **Code Evidence**:
  ```go
  func (cdn *CDNService) handleContent(w http.ResponseWriter, r *http.Request) {
      start := time.Now()
      defer func() {
          cdn.metrics.requestDuration.WithLabelValues(r.Method).Observe(time.Since(start).Seconds())
      }()
      
      contentPath := strings.TrimPrefix(r.URL.Path, "/content/")
      
      // Cache check
      if cached, found := cdn.cache.Get(contentPath); found {
          cdn.metrics.cacheHits.Inc()
          w.Header().Set("X-Cache", "HIT")
          w.Write(cached.([]byte))
          return
      }
      
      // Fetch from cluster
      content, err := cdn.fetchFromCluster(contentPath)
      if err != nil {
          cdn.metrics.errorRate.WithLabelValues("cluster_fetch").Inc()
          http.Error(w, err.Error(), http.StatusInternalServerError)
          return
      }
      
      cdn.cache.Add(contentPath, content)
      w.Header().Set("X-Cache", "MISS")
      w.Write(content)
  }
  ```

### Integration Layer

#### ClusterClient Interface
- **Purpose**: IPFS-cluster API client wrapper
- **Real Implementation**: Abstraction over cluster operations
- **Code Evidence**:
  ```go
  type ClusterClient interface {
      Pin(ctx context.Context, cid cid.Cid) error
      Unpin(ctx context.Context, cid cid.Cid) error
      Status(ctx context.Context, cid cid.Cid) (*api.GlobalPinInfo, error)
      Cat(ctx context.Context, cid cid.Cid) (io.ReadCloser, error)
      Add(ctx context.Context, reader io.Reader) (*api.AddedOutput, error)
  }
  ```

### Optimization Layer

#### BitswapConfig Struct
- **Purpose**: Optimized Bitswap parameters
- **Real Implementation**: Boxo-specific optimizations
- **Code Evidence**:
  ```go
  type BitswapConfig struct {
      MaxOutstandingBytesPerPeer int64
      WorkerCount               int
      BatchSize                 int
      BatchTimeout              time.Duration
      ProvideEnabled            bool
      ProviderSearchDelay       time.Duration
  }
  
  func (bc *BitswapConfig) ApplyOptimizations() *bitswap.Config {
      return &bitswap.Config{
          MaxOutstandingBytesPerPeer: bc.MaxOutstandingBytesPerPeer,
          TaskWorkerCount:           bc.WorkerCount,
          EngineBlockstoreWorkerCount: bc.WorkerCount / 4,
          EngineTaskWorkerCount:     bc.WorkerCount / 2,
          MaxCidSize:               100,
      }
  }
  ```

### Caching Layer

#### LRUCache Interface
- **Purpose**: Least Recently Used cache implementation
- **Real Implementation**: High-performance caching with compression
- **Code Evidence**:
  ```go
  type LRUCache interface {
      Get(key string) (interface{}, bool)
      Add(key string, value interface{})
      Remove(key string) bool
      Len() int
      Purge()
  }
  
  type CompressedLRUCache struct {
      cache       *lru.Cache
      compression bool
      stats       *CacheStats
  }
  
  func (c *CompressedLRUCache) Get(key string) (interface{}, bool) {
      value, found := c.cache.Get(key)
      if !found {
          atomic.AddInt64(&c.stats.Misses, 1)
          return nil, false
      }
      
      atomic.AddInt64(&c.stats.Hits, 1)
      
      if c.compression {
          if compressed, ok := value.(CompressedContent); ok {
              return compressed.Decompress()
          }
      }
      
      return value, true
  }
  ```

### Monitoring Layer

#### PrometheusMetrics Struct
- **Purpose**: Prometheus metric collectors
- **Real Implementation**: Comprehensive metrics collection
- **Code Evidence**:
  ```go
  type PrometheusMetrics struct {
      requestsTotal     *prometheus.CounterVec
      requestDuration   *prometheus.HistogramVec
      cacheHits         prometheus.Counter
      cacheMisses       prometheus.Counter
      activeConnections prometheus.Gauge
      errorRate         *prometheus.CounterVec
  }
  
  func NewPrometheusMetrics() *PrometheusMetrics {
      return &PrometheusMetrics{
          requestsTotal: prometheus.NewCounterVec(
              prometheus.CounterOpts{
                  Name: "cdn_requests_total",
                  Help: "Total number of CDN requests",
              },
              []string{"method", "status"},
          ),
          requestDuration: prometheus.NewHistogramVec(
              prometheus.HistogramOpts{
                  Name:    "cdn_request_duration_seconds",
                  Help:    "CDN request duration in seconds",
                  Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
              },
              []string{"method"},
          ),
      }
  }
  ```

## Code Flow and Interactions

### 1. Request Processing Flow
```
HTTP Request → ContentHandler → ClusterClient → LRUCache → HTTP Response
```

### 2. Configuration Flow
```
ConfigLoader → CDNConfig → ConfigValidator → CDNService
```

### 3. Monitoring Flow
```
Service Operations → PrometheusMetrics → Metrics Endpoint → Prometheus Server
```

## Architecture-to-Code Mapping

### 1. Task 8.1 Parameters → CDNConfig Fields
- Documentation parameter → Struct field → Runtime value

### 2. Performance Requirements → Validation Logic
- Documented thresholds → Validation rules → Runtime checks

### 3. Operational Procedures → Service Methods
- Documented procedures → Go functions → Executable code

## Key Implementation Patterns

### 1. Dependency Injection
- Clean separation of concerns
- Testable components
- Configurable behavior

### 2. Interface-Based Design
- Mockable dependencies
- Pluggable implementations
- Clear contracts

### 3. Metrics-Driven Development
- Observable operations
- Performance tracking
- Operational insights

## Conclusion

This code-level diagram shows the **final implementation** of architectural concepts, providing:

1. **Concrete implementations** of abstract designs
2. **Measurable performance** through metrics
3. **Maintainable code** through clean architecture
4. **Operational readiness** through comprehensive monitoring

The code directly implements the concepts documented in Task 8.1 and provides the foundation for the deployments shown in other Task 8.2 components.