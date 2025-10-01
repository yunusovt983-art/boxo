# Cluster Deployment Examples

This document provides configuration examples for typical IPFS-cluster deployments with Boxo high-load optimizations.

## Example 1: Content Distribution Network (CDN)

**Scenario**: High-throughput content delivery with read-heavy workload

### Characteristics
- 10+ nodes across multiple regions
- 50,000+ concurrent users
- 95% read operations, 5% write operations
- Large file sizes (1MB - 1GB)

### Configuration

```go
package main

import (
    "context"
    "time"
    "github.com/ipfs/boxo/config/adaptive"
    "github.com/ipfs/boxo/bitswap"
    "github.com/ipfs/boxo/blockstore"
)

func setupCDNCluster() *adaptive.HighLoadConfig {
    return &adaptive.HighLoadConfig{
        Bitswap: adaptive.AdaptiveBitswapConfig{
            // Optimize for large file transfers
            MaxOutstandingBytesPerPeer: 200 << 20, // 200MB
            MinOutstandingBytesPerPeer: 10 << 20,  // 10MB
            MinWorkerCount:             512,
            MaxWorkerCount:             4096,
            
            // Larger batches for efficiency
            BatchSize:    2000,
            BatchTimeout: time.Millisecond * 5,
            
            // Prioritize large block requests
            HighPriorityThreshold:     time.Millisecond * 20,
            CriticalPriorityThreshold: time.Millisecond * 5,
        },
        
        Blockstore: adaptive.BlockstoreConfig{
            // Large memory cache for hot content
            MemoryCacheSize: 16 << 30, // 16GB
            SSDCacheSize:    1 << 40,  // 1TB
            
            // Optimize for large blocks
            BatchSize:          5000,
            BatchTimeout:       time.Millisecond * 10,
            CompressionEnabled: true,
            CompressionLevel:   9, // Maximum compression for storage efficiency
        },
        
        Network: adaptive.NetworkConfig{
            // High connection limits for CDN
            HighWater:   10000,
            LowWater:    5000,
            GracePeriod: time.Second * 10,
            
            // Strict quality requirements
            LatencyThreshold:   time.Millisecond * 50,
            BandwidthThreshold: 100 << 20, // 100MB/s
            ErrorRateThreshold: 0.001,     // 0.1%
            
            KeepAliveInterval: time.Second * 10,
            KeepAliveTimeout:  time.Second * 3,
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
}
```

### Deployment Script

```bash
#!/bin/bash
# CDN Cluster Deployment Script

# Set environment variables
export BOXO_CONFIG_TYPE="cdn"
export BOXO_LOG_LEVEL="info"
export BOXO_METRICS_ENABLED="true"
export BOXO_PROMETHEUS_PORT="9090"

# Configure system limits
echo "* soft nofile 65536" >> /etc/security/limits.conf
echo "* hard nofile 65536" >> /etc/security/limits.conf

# Start IPFS cluster with optimized configuration
ipfs-cluster-service daemon \
    --config /etc/ipfs-cluster/service.json \
    --bootstrap /dns4/cluster-bootstrap.example.com/tcp/9096/p2p/QmBootstrapPeer \
    --enable-metrics \
    --metrics-prometheus
```

## Example 2: Collaborative Development Platform

**Scenario**: Version control and collaborative editing with balanced read/write operations

### Characteristics
- 5-15 nodes
- 1,000-5,000 concurrent users
- 60% read, 40% write operations
- Small to medium file sizes (1KB - 10MB)
- Frequent updates and synchronization

### Configuration

```go
func setupDevPlatform() *adaptive.HighLoadConfig {
    return &adaptive.HighLoadConfig{
        Bitswap: adaptive.AdaptiveBitswapConfig{
            // Balanced for read/write workload
            MaxOutstandingBytesPerPeer: 50 << 20, // 50MB
            MinOutstandingBytesPerPeer: 5 << 20,  // 5MB
            MinWorkerCount:             256,
            MaxWorkerCount:             1024,
            
            // Smaller batches for responsiveness
            BatchSize:    500,
            BatchTimeout: time.Millisecond * 15,
            
            // Balanced priority thresholds
            HighPriorityThreshold:     time.Millisecond * 50,
            CriticalPriorityThreshold: time.Millisecond * 10,
        },
        
        Blockstore: adaptive.BlockstoreConfig{
            // Moderate cache sizes
            MemoryCacheSize: 4 << 30,  // 4GB
            SSDCacheSize:    100 << 30, // 100GB
            
            // Optimized for frequent updates
            BatchSize:          1000,
            BatchTimeout:       time.Millisecond * 25,
            CompressionEnabled: true,
            CompressionLevel:   5, // Balanced compression
        },
        
        Network: adaptive.NetworkConfig{
            // Moderate connection limits
            HighWater:   3000,
            LowWater:    1500,
            GracePeriod: time.Second * 20,
            
            // Reasonable quality requirements
            LatencyThreshold:   time.Millisecond * 100,
            BandwidthThreshold: 10 << 20, // 10MB/s
            ErrorRateThreshold: 0.02,     // 2%
            
            KeepAliveInterval: time.Second * 20,
            KeepAliveTimeout:  time.Second * 7,
        },
        
        Monitoring: adaptive.MonitoringConfig{
            MetricsInterval: time.Second * 10,
            AlertThresholds: adaptive.AlertThresholds{
                ResponseTimeP95: time.Millisecond * 75,
                ErrorRate:       0.01,
                MemoryUsage:     0.85,
                CPUUsage:        0.8,
            },
            AutoTuningEnabled:  true,
            AutoTuningInterval: time.Minute * 3,
        },
    }
}
```

## Example 3: IoT Data Collection Hub

**Scenario**: High-frequency data ingestion from IoT devices

### Characteristics
- 3-10 nodes
- 100,000+ IoT devices
- 90% write operations, 10% read operations
- Small data packets (100B - 10KB)
- High frequency updates (every few seconds)

### Configuration

```go
func setupIoTHub() *adaptive.HighLoadConfig {
    return &adaptive.HighLoadConfig{
        Bitswap: adaptive.AdaptiveBitswapConfig{
            // Optimized for small, frequent writes
            MaxOutstandingBytesPerPeer: 10 << 20, // 10MB
            MinOutstandingBytesPerPeer: 1 << 20,  // 1MB
            MinWorkerCount:             128,
            MaxWorkerCount:             512,
            
            // Small batches for low latency
            BatchSize:    100,
            BatchTimeout: time.Millisecond * 5,
            
            // Fast response requirements
            HighPriorityThreshold:     time.Millisecond * 10,
            CriticalPriorityThreshold: time.Millisecond * 2,
        },
        
        Blockstore: adaptive.BlockstoreConfig{
            // Smaller cache, optimized for writes
            MemoryCacheSize: 1 << 30, // 1GB
            SSDCacheSize:    20 << 30, // 20GB
            
            // High-frequency batch operations
            BatchSize:          2000,
            BatchTimeout:       time.Millisecond * 5,
            CompressionEnabled: true,
            CompressionLevel:   3, // Fast compression
        },
        
        Network: adaptive.NetworkConfig{
            // High connection count for IoT devices
            HighWater:   15000,
            LowWater:    7500,
            GracePeriod: time.Second * 5,
            
            // Relaxed quality for IoT networks
            LatencyThreshold:   time.Millisecond * 500,
            BandwidthThreshold: 1 << 20, // 1MB/s
            ErrorRateThreshold: 0.05,    // 5%
            
            KeepAliveInterval: time.Second * 60,
            KeepAliveTimeout:  time.Second * 20,
        },
        
        Monitoring: adaptive.MonitoringConfig{
            MetricsInterval: time.Second * 5,
            AlertThresholds: adaptive.AlertThresholds{
                ResponseTimeP95: time.Millisecond * 100,
                ErrorRate:       0.02,
                MemoryUsage:     0.8,
                CPUUsage:        0.75,
            },
            AutoTuningEnabled:  true,
            AutoTuningInterval: time.Minute * 5,
        },
    }
}
```

## Example 4: Media Streaming Platform

**Scenario**: Real-time media streaming with quality adaptation

### Characteristics
- 20+ nodes globally distributed
- 100,000+ concurrent streams
- Large media files (10MB - 10GB)
- Quality adaptation based on bandwidth
- Low latency requirements

### Configuration

```go
func setupMediaStreaming() *adaptive.HighLoadConfig {
    return &adaptive.HighLoadConfig{
        Bitswap: adaptive.AdaptiveBitswapConfig{
            // Optimized for streaming workload
            MaxOutstandingBytesPerPeer: 500 << 20, // 500MB
            MinOutstandingBytesPerPeer: 50 << 20,  // 50MB
            MinWorkerCount:             1024,
            MaxWorkerCount:             8192,
            
            // Large batches for streaming efficiency
            BatchSize:    5000,
            BatchTimeout: time.Millisecond * 2,
            
            // Ultra-low latency requirements
            HighPriorityThreshold:     time.Millisecond * 5,
            CriticalPriorityThreshold: time.Millisecond * 1,
        },
        
        Blockstore: adaptive.BlockstoreConfig{
            // Massive cache for media content
            MemoryCacheSize: 32 << 30, // 32GB
            SSDCacheSize:    5 << 40,  // 5TB
            
            // Streaming-optimized batching
            BatchSize:          10000,
            BatchTimeout:       time.Millisecond * 1,
            CompressionEnabled: false, // Disable for media files
            CompressionLevel:   0,
        },
        
        Network: adaptive.NetworkConfig{
            // Maximum connection capacity
            HighWater:   50000,
            LowWater:    25000,
            GracePeriod: time.Second * 2,
            
            // Strict streaming requirements
            LatencyThreshold:   time.Millisecond * 20,
            BandwidthThreshold: 1 << 30, // 1GB/s
            ErrorRateThreshold: 0.0001,  // 0.01%
            
            KeepAliveInterval: time.Second * 5,
            KeepAliveTimeout:  time.Second * 1,
        },
        
        Monitoring: adaptive.MonitoringConfig{
            MetricsInterval: time.Millisecond * 500, // Ultra-high frequency
            AlertThresholds: adaptive.AlertThresholds{
                ResponseTimeP95: time.Millisecond * 10,
                ErrorRate:       0.0001,
                MemoryUsage:     0.98,
                CPUUsage:        0.95,
            },
            AutoTuningEnabled:  true,
            AutoTuningInterval: time.Second * 30,
            PredictiveScaling:  true,
        },
    }
}
```

## Docker Compose Examples

### Basic High-Load Cluster

```yaml
version: '3.8'

services:
  ipfs-cluster-0:
    image: ipfs/ipfs-cluster:latest
    environment:
      - CLUSTER_PEERNAME=cluster-0
      - CLUSTER_SECRET=${CLUSTER_SECRET}
      - BOXO_CONFIG_TYPE=medium
    volumes:
      - ./cluster-0:/data/ipfs-cluster
      - ./configs/medium-cluster.json:/data/ipfs-cluster/service.json
    ports:
      - "9094:9094"
      - "9095:9095"
      - "9096:9096"
    networks:
      - cluster-net

  ipfs-cluster-1:
    image: ipfs/ipfs-cluster:latest
    environment:
      - CLUSTER_PEERNAME=cluster-1
      - CLUSTER_SECRET=${CLUSTER_SECRET}
      - BOXO_CONFIG_TYPE=medium
    volumes:
      - ./cluster-1:/data/ipfs-cluster
      - ./configs/medium-cluster.json:/data/ipfs-cluster/service.json
    ports:
      - "9097:9094"
      - "9098:9095"
      - "9099:9096"
    networks:
      - cluster-net
    depends_on:
      - ipfs-cluster-0

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
    networks:
      - cluster-net

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - ./monitoring/grafana/dashboards:/var/lib/grafana/dashboards
      - ./monitoring/grafana/provisioning:/etc/grafana/provisioning
    networks:
      - cluster-net

networks:
  cluster-net:
    driver: bridge
```

### Production-Ready Cluster with Load Balancer

```yaml
version: '3.8'

services:
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf
      - ./nginx/ssl:/etc/nginx/ssl
    networks:
      - cluster-net
    depends_on:
      - ipfs-cluster-0
      - ipfs-cluster-1
      - ipfs-cluster-2

  ipfs-cluster-0:
    image: ipfs/ipfs-cluster:latest
    environment:
      - CLUSTER_PEERNAME=cluster-0
      - CLUSTER_SECRET=${CLUSTER_SECRET}
      - BOXO_CONFIG_TYPE=large
      - BOXO_METRICS_ENABLED=true
    volumes:
      - cluster-0-data:/data/ipfs-cluster
      - ./configs/large-cluster.json:/data/ipfs-cluster/service.json
    networks:
      - cluster-net
    deploy:
      resources:
        limits:
          memory: 8G
          cpus: '4'
        reservations:
          memory: 4G
          cpus: '2'

  ipfs-cluster-1:
    image: ipfs/ipfs-cluster:latest
    environment:
      - CLUSTER_PEERNAME=cluster-1
      - CLUSTER_SECRET=${CLUSTER_SECRET}
      - BOXO_CONFIG_TYPE=large
      - BOXO_METRICS_ENABLED=true
    volumes:
      - cluster-1-data:/data/ipfs-cluster
      - ./configs/large-cluster.json:/data/ipfs-cluster/service.json
    networks:
      - cluster-net
    deploy:
      resources:
        limits:
          memory: 8G
          cpus: '4'
        reservations:
          memory: 4G
          cpus: '2'

  ipfs-cluster-2:
    image: ipfs/ipfs-cluster:latest
    environment:
      - CLUSTER_PEERNAME=cluster-2
      - CLUSTER_SECRET=${CLUSTER_SECRET}
      - BOXO_CONFIG_TYPE=large
      - BOXO_METRICS_ENABLED=true
    volumes:
      - cluster-2-data:/data/ipfs-cluster
      - ./configs/large-cluster.json:/data/ipfs-cluster/service.json
    networks:
      - cluster-net
    deploy:
      resources:
        limits:
          memory: 8G
          cpus: '4'
        reservations:
          memory: 4G
          cpus: '2'

volumes:
  cluster-0-data:
  cluster-1-data:
  cluster-2-data:

networks:
  cluster-net:
    driver: overlay
    attachable: true
```

## Configuration Files

### Medium Cluster Service Configuration

```json
{
  "cluster": {
    "peername": "cluster-node",
    "secret": "${CLUSTER_SECRET}",
    "leave_on_shutdown": false,
    "listen_multiaddress": "/ip4/0.0.0.0/tcp/9096",
    "connection_manager": {
      "high_water": 2000,
      "low_water": 1000,
      "grace_period": "30s"
    }
  },
  "api": {
    "ipfsproxy": {
      "listen_multiaddress": "/ip4/0.0.0.0/tcp/9095"
    },
    "restapi": {
      "http_listen_multiaddress": "/ip4/0.0.0.0/tcp/9094"
    }
  },
  "ipfs_connector": {
    "ipfshttp": {
      "node_multiaddress": "/ip4/127.0.0.1/tcp/5001",
      "connect_swarms_delay": "30s",
      "pin_timeout": "2m",
      "unpin_timeout": "3h"
    }
  },
  "pin_tracker": {
    "stateless": {
      "concurrent_pins": 10,
      "priority_pin_max_age": "24h",
      "priority_pin_max_retries": 5
    }
  },
  "monitor": {
    "pubsubmon": {
      "check_interval": "15s"
    }
  },
  "allocator": {
    "balanced": {
      "allocate_by": ["tag:group", "freespace"]
    }
  },
  "informer": {
    "disk": {
      "metric_ttl": "30s",
      "metric_type": "freespace"
    },
    "numpin": {
      "metric_ttl": "30s"
    }
  }
}
```

## Kubernetes Deployment

### Cluster StatefulSet

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ipfs-cluster
  namespace: ipfs
spec:
  serviceName: ipfs-cluster
  replicas: 3
  selector:
    matchLabels:
      app: ipfs-cluster
  template:
    metadata:
      labels:
        app: ipfs-cluster
    spec:
      containers:
      - name: ipfs-cluster
        image: ipfs/ipfs-cluster:latest
        env:
        - name: CLUSTER_PEERNAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: CLUSTER_SECRET
          valueFrom:
            secretKeyRef:
              name: cluster-secret
              key: secret
        - name: BOXO_CONFIG_TYPE
          value: "large"
        ports:
        - containerPort: 9094
          name: api
        - containerPort: 9095
          name: proxy
        - containerPort: 9096
          name: cluster
        volumeMounts:
        - name: cluster-data
          mountPath: /data/ipfs-cluster
        - name: config
          mountPath: /data/ipfs-cluster/service.json
          subPath: service.json
        resources:
          requests:
            memory: "4Gi"
            cpu: "2"
          limits:
            memory: "8Gi"
            cpu: "4"
      volumes:
      - name: config
        configMap:
          name: cluster-config
  volumeClaimTemplates:
  - metadata:
      name: cluster-data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 100Gi
      storageClassName: fast-ssd
```

## Next Steps

- Review [Troubleshooting Guide](troubleshooting-guide.md) for common deployment issues
- Check [Migration Guide](migration-guide.md) for upgrading existing clusters
- See [Integration Examples](integration-examples/) for application-specific configurations