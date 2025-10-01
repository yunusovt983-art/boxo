# Task 8.2 Deployment Diagram - Detailed Explanation

## Overview
The `task8.2-deployment.puml` diagram shows the physical deployment architecture of the Task 8.2 Integration Examples, illustrating how the various components are deployed across Docker containers and networks.

## Architectural Purpose
This diagram serves as the **deployment blueprint**, showing how logical components are mapped to physical infrastructure, network topology, and runtime configuration.

## Deployment Architecture Analysis

### Docker Host Infrastructure

#### Physical Host Configuration
- **Platform**: Linux Server with Docker Engine
- **Purpose**: Single-host deployment for development and testing
- **Scalability**: Can be extended to multi-host Docker Swarm or Kubernetes
- **Resource Requirements**: 
  - CPU: 8+ cores recommended
  - Memory: 16GB+ recommended
  - Storage: 100GB+ for data persistence
  - Network: Gigabit connectivity

### Network Architecture

#### IPFS Network (ipfs-cluster-net)
- **Type**: Docker Bridge Network
- **Subnet**: 172.20.0.0/16
- **Purpose**: Isolated network for IPFS and cluster communication
- **Services**: IPFS nodes, IPFS-cluster nodes, CDN service
- **Configuration**:
  ```yaml
  networks:
    ipfs-cluster-net:
      driver: bridge
      ipam:
        config:
          - subnet: 172.20.0.0/16
      driver_opts:
        com.docker.network.bridge.name: ipfs-cluster-br
        com.docker.network.bridge.enable_icc: "true"
        com.docker.network.bridge.enable_ip_masquerade: "true"
  ```

#### Monitoring Network (monitoring-net)
- **Type**: Docker Bridge Network
- **Subnet**: 172.21.0.0/16
- **Purpose**: Isolated network for monitoring infrastructure
- **Services**: Prometheus, Grafana, CDN service (dual-homed)
- **Configuration**:
  ```yaml
  networks:
    monitoring-net:
      driver: bridge
      ipam:
        config:
          - subnet: 172.21.0.0/16
  ```

### Container Deployment Details

#### IPFS Node Containers (ipfs-0, ipfs-1, ipfs-2)
- **Image**: ipfs/go-ipfs:latest
- **Purpose**: Distributed content storage nodes
- **Configuration**:
  ```yaml
  ipfs-0:
    image: ipfs/go-ipfs:latest
    environment:
      - IPFS_PROFILE=server
      - IPFS_SWARM_KEY_FILE=/data/ipfs/swarm.key
    volumes:
      - ipfs0_data:/data/ipfs
      - ./configs/swarm.key:/data/ipfs/swarm.key:ro
    ports:
      - "4001:4001"  # Swarm port
      - "5001:5001"  # API port
    networks:
      - ipfs-cluster-net
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: '2'
        reservations:
          memory: 1G
          cpus: '1'
  ```
- **Real Implementation**: Direct mapping to docker-compose.yml
- **Architecture Bridge**: Implements distributed storage from Task 8.1 deployment examples

#### IPFS-Cluster Containers (cluster-0, cluster-1, cluster-2)
- **Image**: ipfs/ipfs-cluster:latest
- **Purpose**: Cluster coordination and consensus
- **Configuration**:
  ```yaml
  ipfs-cluster-0:
    image: ipfs/ipfs-cluster:latest
    environment:
      - CLUSTER_PEERNAME=cluster0
      - CLUSTER_SECRET=${CLUSTER_SECRET}
      - CLUSTER_IPFSHTTP_NODEMULTIADDRESS=/dns4/ipfs-0/tcp/5001
      - CLUSTER_CRDT_TRUSTEDPEERS=*
      - CLUSTER_RESTAPI_HTTPLISTENMULTIADDRESS=/ip4/0.0.0.0/tcp/9094
      - CLUSTER_CONSENSUS=crdt
      - CLUSTER_MONITORPINGINTERVAL=2s
    volumes:
      - cluster0_data:/data/ipfs-cluster
      - ./configs/cluster-config.json:/data/ipfs-cluster/service.json:ro
    ports:
      - "9094:9094"  # REST API
      - "9095:9095"  # Cluster API
      - "9096:9096"  # IPFS Proxy
    networks:
      - ipfs-cluster-net
    depends_on:
      - ipfs-0
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "ipfs-cluster-ctl", "peers", "ls"]
      interval: 30s
      timeout: 10s
      retries: 3
  ```
- **Real Implementation**: Production-ready cluster configuration
- **Architecture Bridge**: Implements cluster deployment patterns from Task 8.1

#### CDN Service Container
- **Build**: Custom image from application source
- **Purpose**: High-performance content delivery with Boxo optimizations
- **Configuration**:
  ```yaml
  cdn-service:
    build: 
      context: ../../applications/cdn-service/
      dockerfile: Dockerfile
    environment:
      - IPFS_CLUSTER_API=http://ipfs-cluster-0:9094
      - CACHE_SIZE=${CACHE_SIZE:-2147483648}  # 2GB default
      - WORKER_COUNT=${WORKER_COUNT:-1024}
      - MAX_OUTSTANDING_BYTES=${MAX_OUTSTANDING_BYTES:-20971520}  # 20MB
      - COMPRESSION_ENABLED=${COMPRESSION_ENABLED:-true}
      - PREFETCH_ENABLED=${PREFETCH_ENABLED:-true}
    ports:
      - "8080:8080"  # HTTP API
      - "9090:9090"  # Metrics
    networks:
      - ipfs-cluster-net
      - monitoring-net
    depends_on:
      - ipfs-cluster-0
      - ipfs-cluster-1
      - ipfs-cluster-2
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
    deploy:
      resources:
        limits:
          memory: 4G
          cpus: '4'
        reservations:
          memory: 2G
          cpus: '2'
  ```
- **Real Implementation**: CDN service from Task 8.2 components
- **Architecture Bridge**: Implements optimized configurations from Task 8.1

### Monitoring Infrastructure

#### Prometheus Container
- **Image**: prom/prometheus:latest
- **Purpose**: Metrics collection and storage
- **Configuration**:
  ```yaml
  prometheus:
    image: prom/prometheus:latest
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--storage.tsdb.retention.time=${PROMETHEUS_RETENTION:-30d}'
      - '--web.enable-lifecycle'
      - '--web.enable-admin-api'
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus_data:/prometheus
    ports:
      - "9091:9090"
    networks:
      - monitoring-net
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: '2'
  ```
- **Scrape Configuration**:
  ```yaml
  scrape_configs:
    - job_name: 'cdn-service'
      static_configs:
        - targets: ['cdn-service:9090']
      scrape_interval: 15s
      metrics_path: /metrics
      
    - job_name: 'ipfs-cluster'
      static_configs:
        - targets: ['ipfs-cluster-0:9094', 'ipfs-cluster-1:9095', 'ipfs-cluster-2:9096']
      scrape_interval: 30s
      metrics_path: /api/v0/monitor/metrics/prometheus
  ```

#### Grafana Container
- **Image**: grafana/grafana:latest
- **Purpose**: Metrics visualization and alerting
- **Configuration**:
  ```yaml
  grafana:
    image: grafana/grafana:latest
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_ADMIN_PASSWORD:-admin}
      - GF_USERS_ALLOW_SIGN_UP=false
      - GF_INSTALL_PLUGINS=grafana-piechart-panel,grafana-clock-panel
      - GF_RENDERING_SERVER_URL=http://renderer:8081/render
      - GF_RENDERING_CALLBACK_URL=http://grafana:3000/
    volumes:
      - ../../monitoring-dashboard/provisioning:/etc/grafana/provisioning:ro
      - ../../monitoring-dashboard/dashboards:/var/lib/grafana/dashboards:ro
      - grafana_data:/var/lib/grafana
    ports:
      - "3000:3000"
    networks:
      - monitoring-net
    depends_on:
      - prometheus
    restart: unless-stopped
  ```

### External Integration Points

#### Load Testing Infrastructure
- **Tool**: wrk + Lua scripts
- **Purpose**: Automated performance testing
- **Configuration**:
  ```bash
  # External load testing setup
  docker run --rm --network host \
    -v $(pwd)/benchmarking:/scripts \
    williamyeh/wrk \
    -t12 -c1000 -d300s -R5000 \
    --script=/scripts/cdn-benchmark.lua \
    --latency \
    http://localhost:8080
  ```
- **Integration**: Connects to CDN service via host network
- **Metrics Collection**: Queries Prometheus for performance data

#### External Client Access
- **Web Browsers**: HTTP clients accessing CDN content
- **API Clients**: Applications consuming CDN API
- **Access Pattern**:
  ```
  Internet → Docker Host:8080 → CDN Service Container → IPFS-Cluster → Content
  ```

### Data Persistence

#### Volume Configuration
```yaml
volumes:
  # IPFS data persistence
  ipfs0_data:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /data/ipfs/node0
      
  # Cluster data persistence
  cluster0_data:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /data/cluster/node0
      
  # Monitoring data persistence
  prometheus_data:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /data/prometheus
      
  grafana_data:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /data/grafana
```

### Network Communication Patterns

#### IPFS Network Communication
- **IPFS Nodes**: Bitswap protocol for block exchange
- **Cluster Nodes**: CRDT consensus for coordination
- **CDN Service**: HTTP API calls to cluster
- **Port Mapping**:
  ```
  4001: IPFS Swarm (libp2p)
  5001: IPFS HTTP API
  9094-9096: Cluster APIs
  8080: CDN HTTP API
  ```

#### Monitoring Network Communication
- **Prometheus**: Scrapes metrics from all services
- **Grafana**: Queries Prometheus for visualization
- **CDN Service**: Dual-homed for metrics export
- **Port Mapping**:
  ```
  9090: Metrics endpoints
  9091: Prometheus web UI
  3000: Grafana web UI
  ```

### Performance Characteristics

#### Network Optimization
- **High Water Mark**: 2000 connections per node
- **Low Water Mark**: 1000 connections per node
- **Grace Period**: 30 seconds for connection cleanup
- **Bitswap Workers**: 1024 concurrent workers
- **Max Outstanding**: 20MB per peer

#### Resource Allocation
- **CDN Service**: 4 CPU cores, 4GB RAM
- **IPFS Nodes**: 2 CPU cores, 2GB RAM each
- **Cluster Nodes**: 1 CPU core, 1GB RAM each
- **Monitoring**: 2 CPU cores, 2GB RAM total

### Deployment Procedures

#### Startup Sequence
1. **IPFS Nodes**: Initialize and start IPFS daemons
2. **Cluster Nodes**: Bootstrap cluster with consensus
3. **CDN Service**: Connect to cluster and start serving
4. **Monitoring**: Start Prometheus and Grafana
5. **Health Checks**: Validate all services are healthy

#### Health Monitoring
```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 60s
```

### Scaling Considerations

#### Horizontal Scaling
- **IPFS Nodes**: Add more nodes to increase storage capacity
- **Cluster Nodes**: Maintain odd number for consensus
- **CDN Services**: Load balance multiple instances
- **Monitoring**: Federated Prometheus for large deployments

#### Vertical Scaling
- **Resource Limits**: Configurable via environment variables
- **Cache Sizes**: Adjustable based on available memory
- **Worker Counts**: Scalable with CPU cores
- **Connection Limits**: Tunable for network capacity

## Architecture-to-Deployment Mapping

### 1. Task 8.1 Configurations → Container Environment Variables
- Documented parameters → Environment variables → Runtime configuration

### 2. Task 8.2 Components → Docker Containers
- Logical components → Physical containers → Running services

### 3. Network Design → Docker Networks
- Communication patterns → Network topology → Container connectivity

## Operational Procedures

### 1. Deployment
```bash
# Clone repository
git clone <repository>
cd docs/boxo-high-load-optimization/integration-examples/docker-compose/basic-cluster/

# Configure environment
cp .env.example .env
# Edit .env with appropriate values

# Deploy stack
docker-compose up -d

# Verify deployment
docker-compose ps
docker-compose logs -f
```

### 2. Monitoring
```bash
# Access Grafana
open http://localhost:3000
# Login: admin/admin

# Access Prometheus
open http://localhost:9091

# Check CDN health
curl http://localhost:8080/health
```

### 3. Testing
```bash
# Run benchmarks
cd ../../benchmarking/load-tests/
./run-basic-benchmark.sh

# Check results
ls -la results/
```

## Conclusion

The deployment diagram shows how **architectural concepts become running systems** through:

1. **Physical infrastructure** that supports the logical architecture
2. **Network topology** that enables secure and efficient communication
3. **Container orchestration** that manages service lifecycle
4. **Monitoring integration** that provides operational visibility
5. **Scaling patterns** that support growth and performance requirements

This deployment serves as a **production-ready template** that can be adapted for different environments while maintaining the architectural principles established in Task 8.1 and implemented in Task 8.2 components.