# Basic High-Load Cluster

This Docker Compose setup provides a complete 3-node IPFS cluster with Boxo high-load optimizations, monitoring, and load balancing.

## Quick Start

1. **Copy environment file:**
```bash
cp .env.example .env
# Edit .env file with your configuration
```

2. **Generate cluster secret:**
```bash
export CLUSTER_SECRET=$(openssl rand -hex 32)
echo "CLUSTER_SECRET=$CLUSTER_SECRET" >> .env
```

3. **Start the cluster:**
```bash
docker-compose up -d
```

4. **Verify cluster health:**
```bash
# Check cluster status
curl http://localhost:9094/api/v0/peers

# Check IPFS nodes
curl http://localhost:5001/api/v0/id
```

5. **Access monitoring:**
- Grafana: http://localhost:3000 (admin/admin)
- Prometheus: http://localhost:9090

## Services

### IPFS Cluster Nodes
- **cluster-0**: Bootstrap node (ports 9094-9096)
- **cluster-1**: Follower node (ports 9097-9099)
- **cluster-2**: Follower node (ports 9100-9102)

### IPFS Nodes
- **ipfs-0**: Primary IPFS node (ports 4001, 5001, 8080)
- **ipfs-1**: Secondary IPFS node (ports 4002, 5002, 8081)
- **ipfs-2**: Tertiary IPFS node (ports 4003, 5003, 8082)

### Monitoring
- **Prometheus**: Metrics collection (port 9090)
- **Grafana**: Visualization dashboard (port 3000)

### Load Balancer
- **Nginx**: Load balancer for cluster API (ports 80, 443)

## Configuration

### High-Load Optimizations

The cluster is configured with the following optimizations:

```json
{
  "connection_manager": {
    "high_water": 2000,
    "low_water": 1000,
    "grace_period": "30s"
  },
  "batching": {
    "max_batch_size": 1000,
    "max_batch_age": "20ms"
  }
}
```

### Resource Limits

Each service has resource limits configured:
- **Cluster nodes**: 2GB RAM, 1 CPU
- **IPFS nodes**: 4GB RAM, 2 CPU

## Testing

### Basic Functionality Test
```bash
# Add a file to the cluster
echo "Hello, Boxo!" | curl -X POST -F "file=@-" http://localhost:9094/api/v0/add

# Pin the file
curl -X POST http://localhost:9094/api/v0/pins/QmHash

# Retrieve the file
curl http://localhost:9094/api/v0/cat?arg=QmHash
```

### Performance Benchmark
```bash
# Run the benchmark suite
cd ../../benchmarking/load-tests
./run-basic-benchmark.sh
```

## Monitoring

### Key Metrics

The setup monitors the following metrics:
- API request rate and response times
- Memory and CPU usage
- Network connections and peer count
- Bitswap block transfer rates
- Cache hit rates
- Error rates

### Alerts

Prometheus alerts are configured for:
- High response times (> 100ms P95)
- High error rates (> 5%)
- Memory usage (> 85%)
- Node failures

### Grafana Dashboards

Pre-configured dashboards include:
- **Boxo Performance**: Overall cluster performance
- **System Resources**: CPU, memory, disk usage
- **Network Metrics**: Connection counts, bandwidth
- **Error Analysis**: Error rates and types

## Troubleshooting

### Common Issues

1. **Cluster nodes not connecting:**
```bash
# Check cluster secret consistency
docker-compose logs cluster-0 | grep secret

# Verify network connectivity
docker-compose exec cluster-0 ipfs-cluster-ctl peers ls
```

2. **High memory usage:**
```bash
# Check memory metrics
curl http://localhost:9090/api/v1/query?query=process_resident_memory_bytes

# Restart services if needed
docker-compose restart cluster-0
```

3. **Performance issues:**
```bash
# Check system resources
docker stats

# Review configuration
docker-compose exec cluster-0 cat /data/ipfs-cluster/service.json
```

### Log Analysis

```bash
# View cluster logs
docker-compose logs -f cluster-0

# View IPFS logs
docker-compose logs -f ipfs-0

# View all logs
docker-compose logs -f
```

## Scaling

### Adding More Nodes

To add additional cluster nodes:

1. **Add new service to docker-compose.yml:**
```yaml
ipfs-cluster-3:
  image: ipfs/ipfs-cluster:latest
  environment:
    - CLUSTER_PEERNAME=cluster-3
    - CLUSTER_SECRET=${CLUSTER_SECRET}
    - IPFS_API=/dns4/ipfs-3/tcp/5001
  # ... rest of configuration
```

2. **Add corresponding IPFS node:**
```yaml
ipfs-3:
  image: ipfs/kubo:latest
  # ... configuration
```

3. **Update monitoring configuration:**
```yaml
# Add to prometheus.yml
- targets:
  - 'cluster-3:8888'
  - 'ipfs-3:5001'
```

### Vertical Scaling

To increase resources for existing nodes:

```yaml
deploy:
  resources:
    limits:
      memory: 8G      # Increased from 2G
      cpus: '4'       # Increased from 1
    reservations:
      memory: 4G      # Increased from 1G
      cpus: '2'       # Increased from 0.5
```

## Production Considerations

### Security

1. **Change default passwords:**
```bash
# Update Grafana password
export GRAFANA_PASSWORD=$(openssl rand -base64 32)
```

2. **Use proper secrets management:**
```bash
# Use Docker secrets instead of environment variables
docker secret create cluster_secret cluster_secret.txt
```

3. **Enable TLS:**
```bash
# Configure TLS certificates in nginx
# Update cluster configuration for HTTPS
```

### Backup

```bash
# Backup cluster data
docker-compose exec cluster-0 tar -czf /backup/cluster-data.tar.gz /data/ipfs-cluster

# Backup IPFS data
docker-compose exec ipfs-0 tar -czf /backup/ipfs-data.tar.gz /data/ipfs
```

### Monitoring

```bash
# Set up external monitoring
# Configure alertmanager for notifications
# Set up log aggregation (ELK stack)
```

## Next Steps

- Review [Configuration Guide](../../../configuration-guide.md) for advanced tuning
- Check [Troubleshooting Guide](../../../troubleshooting-guide.md) for common issues
- See [Production Setup](../production/) for enterprise deployment