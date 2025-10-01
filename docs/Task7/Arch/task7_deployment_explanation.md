# Task 7 Deployment Diagram - Подробное объяснение

## Обзор диаграммы

**Файл**: `task7_deployment_diagram.puml`  
**Тип**: Deployment Diagram  
**Назначение**: Показывает физическое развертывание системы тестирования производительности

## Архитектурный дизайн → Реализация инфраструктуры

### 💻 Developer Machine - Локальная разработка

#### Local Environment (Docker Desktop)
**Архитектурная роль**: Локальная среда разработки и тестирования

**Реализация инфраструктуры**:
```yaml
# docker-compose.local.yml
version: '3.8'
services:
  local-load-tests:
    build:
      context: .
      dockerfile: Dockerfile.loadtest
    environment:
      - TEST_MODE=local
      - LOG_LEVEL=debug
    volumes:
      - ./bitswap/loadtest:/app/loadtest
      - ./test_results:/app/results
    command: ["go", "test", "-v", "./loadtest/..."]
    
  local-integration-tests:
    build:
      context: .
      dockerfile: Dockerfile.integration
    environment:
      - CLUSTER_SIZE=3
      - TEST_DURATION=5m
    volumes:
      - ./integration:/app/integration
      - ./test_results:/app/results
    depends_on:
      - ipfs-node-1
      - ipfs-node-2
      - ipfs-node-3
      
  ipfs-node-1:
    image: ipfs/go-ipfs:latest
    ports:
      - "4001:4001"
      - "5001:5001"
    volumes:
      - ipfs1_data:/data/ipfs
      
  ipfs-node-2:
    image: ipfs/go-ipfs:latest
    ports:
      - "4002:4001"
      - "5002:5001"
    volumes:
      - ipfs2_data:/data/ipfs
      
  ipfs-node-3:
    image: ipfs/go-ipfs:latest
    ports:
      - "4003:4001"
      - "5003:5001"
    volumes:
      - ipfs3_data:/data/ipfs

volumes:
  ipfs1_data:
  ipfs2_data:
  ipfs3_data:
```

**Makefile для локальной разработки**:
```makefile
# Makefile.local
.PHONY: local-setup local-test local-cleanup

local-setup:
	@echo "Setting up local development environment..."
	docker-compose -f docker-compose.local.yml up -d
	@echo "Waiting for services to be ready..."
	sleep 30
	
local-test-load:
	@echo "Running local load tests..."
	docker-compose -f docker-compose.local.yml exec local-load-tests \
		go test -v -run TestBitswapComprehensiveLoadSuite/Test_1K_ConcurrentConnections
		
local-test-integration:
	@echo "Running local integration tests..."
	docker-compose -f docker-compose.local.yml exec local-integration-tests \
		go test -v ./integration/...
		
local-cleanup:
	@echo "Cleaning up local environment..."
	docker-compose -f docker-compose.local.yml down -v
```

### 🔄 CI/CD Infrastructure - GitHub Actions

#### GitHub Runners (Ubuntu 22.04)
**Архитектурная роль**: Автоматизированное выполнение тестов в CI/CD

**Реализация GitHub Actions**:
```yaml
# .github/workflows/task7-testing.yml
name: Task 7 - Performance Testing Suite

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM

jobs:
  load-tests:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        test-type: [concurrent, throughput, stability]
        scale: [small, medium]
    
    steps:
      - name: Checkout
        uses: actions/checkout@v3
        
      - name: Setup Go
        uses: actions/setup-go@v3
        with:
          go-version: '1.21'
          
      - name: Setup Test Environment
        run: |
          sudo sysctl -w net.core.somaxconn=65535
          sudo sysctl -w fs.file-max=2097152
          ulimit -n 65536
          
      - name: Run Load Tests
        env:
          TEST_TYPE: ${{ matrix.test-type }}
          TEST_SCALE: ${{ matrix.scale }}
        run: |
          make test-${{ matrix.test-type }} SCALE=${{ matrix.scale }}
          
      - name: Upload Test Results
        uses: actions/upload-artifact@v3
        with:
          name: load-test-results-${{ matrix.test-type }}-${{ matrix.scale }}
          path: test_results/
          
  integration-tests:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        cluster-size: [3, 5, 7]
        
    steps:
      - name: Checkout
        uses: actions/checkout@v3
        
      - name: Setup Kubernetes
        uses: helm/kind-action@v1.4.0
        with:
          cluster_name: test-cluster
          
      - name: Deploy Test Cluster
        run: |
          kubectl apply -f k8s/test-cluster.yaml
          kubectl wait --for=condition=ready pod -l app=ipfs-cluster --timeout=300s
          
      - name: Run Integration Tests
        env:
          CLUSTER_SIZE: ${{ matrix.cluster-size }}
        run: |
          make test-integration NODES=${{ matrix.cluster-size }}
          
  fault-tolerance-tests:
    runs-on: ubuntu-latest
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    
    steps:
      - name: Checkout
        uses: actions/checkout@v3
        
      - name: Setup Chaos Environment
        run: |
          # Install chaos engineering tools
          curl -sSL https://github.com/chaos-mesh/chaos-mesh/releases/download/v2.5.0/install.sh | bash
          
      - name: Run Fault Tolerance Tests
        run: |
          make test-chaos-engineering
          make test-circuit-breaker
          make test-recovery-scenarios
```

### ☸️ Test Cluster - Kubernetes

#### Control Plane (K8s Master)
**Архитектурная роль**: Управление кластерными тестами

**Реализация Kubernetes манифестов**:
```yaml
# k8s/test-orchestrator.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-orchestrator
  namespace: task7-testing
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-orchestrator
  template:
    metadata:
      labels:
        app: test-orchestrator
    spec:
      containers:
      - name: orchestrator
        image: task7/test-orchestrator:latest
        env:
        - name: CLUSTER_SIZE
          value: "5"
        - name: TEST_DURATION
          value: "30m"
        resources:
          requests:
            cpu: 500m
            memory: 1Gi
          limits:
            cpu: 2
            memory: 4Gi
        ports:
        - containerPort: 8080
          name: api
        - containerPort: 9090
          name: metrics
          
---
apiVersion: v1
kind: Service
metadata:
  name: test-orchestrator-service
  namespace: task7-testing
spec:
  selector:
    app: test-orchestrator
  ports:
  - name: api
    port: 8080
    targetPort: 8080
  - name: metrics
    port: 9090
    targetPort: 9090
```

#### Worker Nodes
**Архитектурная роль**: Выполнение тестовых нагрузок

**Реализация StatefulSet для IPFS узлов**:
```yaml
# k8s/ipfs-cluster.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ipfs-cluster
  namespace: task7-testing
spec:
  serviceName: ipfs-cluster
  replicas: 5
  selector:
    matchLabels:
      app: ipfs-cluster
  template:
    metadata:
      labels:
        app: ipfs-cluster
    spec:
      containers:
      - name: ipfs
        image: ipfs/go-ipfs:latest
        env:
        - name: IPFS_PROFILE
          value: "server"
        ports:
        - containerPort: 4001
          name: swarm
        - containerPort: 5001
          name: api
        - containerPort: 8080
          name: gateway
        volumeMounts:
        - name: ipfs-data
          mountPath: /data/ipfs
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 1
            memory: 2Gi
            
      - name: bitswap-test
        image: task7/bitswap-test:latest
        env:
        - name: IPFS_API_URL
          value: "http://localhost:5001"
        - name: TEST_MODE
          value: "cluster"
        resources:
          requests:
            cpu: 200m
            memory: 512Mi
          limits:
            cpu: 2
            memory: 4Gi
            
      - name: load-generator
        image: task7/load-generator:latest
        env:
        - name: TARGET_RPS
          value: "1000"
        - name: CONCURRENT_CONNECTIONS
          value: "100"
        resources:
          requests:
            cpu: 500m
            memory: 1Gi
          limits:
            cpu: 2
            memory: 4Gi
            
  volumeClaimTemplates:
  - metadata:
      name: ipfs-data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
```

### 📊 Monitoring Infrastructure

#### Prometheus & Grafana
**Архитектурная роль**: Сбор метрик и визуализация

**Реализация мониторинга**:
```yaml
# k8s/monitoring.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus
  namespace: monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app: prometheus
  template:
    metadata:
      labels:
        app: prometheus
    spec:
      containers:
      - name: prometheus
        image: prom/prometheus:latest
        args:
          - '--config.file=/etc/prometheus/prometheus.yml'
          - '--storage.tsdb.path=/prometheus/'
          - '--web.console.libraries=/etc/prometheus/console_libraries'
          - '--web.console.templates=/etc/prometheus/consoles'
          - '--storage.tsdb.retention.time=30d'
          - '--web.enable-lifecycle'
        ports:
        - containerPort: 9090
        volumeMounts:
        - name: prometheus-config
          mountPath: /etc/prometheus/
        - name: prometheus-storage
          mountPath: /prometheus/
      volumes:
      - name: prometheus-config
        configMap:
          name: prometheus-config
      - name: prometheus-storage
        persistentVolumeClaim:
          claimName: prometheus-storage

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
  namespace: monitoring
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
      evaluation_interval: 15s
    
    scrape_configs:
    - job_name: 'task7-load-tests'
      static_configs:
      - targets: ['test-orchestrator-service.task7-testing:9090']
      metrics_path: /metrics
      scrape_interval: 5s
      
    - job_name: 'ipfs-cluster'
      kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
          - task7-testing
      relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: ipfs-cluster
```

### 🚀 Load Testing Infrastructure

#### High-Performance Cluster (Bare Metal)
**Архитектурная роль**: Генерация экстремальной нагрузки

**Реализация Terraform для bare metal**:
```hcl
# terraform/load-testing-cluster.tf
resource "aws_instance" "load_generator" {
  count         = 5
  ami           = "ami-0c02fb55956c7d316" # Ubuntu 22.04
  instance_type = "c5n.4xlarge" # High network performance
  
  vpc_security_group_ids = [aws_security_group.load_testing.id]
  subnet_id              = aws_subnet.load_testing.id
  
  user_data = <<-EOF
    #!/bin/bash
    apt-get update
    apt-get install -y docker.io docker-compose
    
    # Optimize for high load testing
    echo 'net.core.somaxconn = 65535' >> /etc/sysctl.conf
    echo 'net.core.netdev_max_backlog = 5000' >> /etc/sysctl.conf
    echo 'fs.file-max = 2097152' >> /etc/sysctl.conf
    sysctl -p
    
    # Set ulimits
    echo '* soft nofile 1048576' >> /etc/security/limits.conf
    echo '* hard nofile 1048576' >> /etc/security/limits.conf
    
    # Install load testing tools
    docker pull task7/extreme-load-generator:latest
    docker pull task7/concurrent-connection-tester:latest
    docker pull task7/throughput-tester:latest
  EOF
  
  tags = {
    Name = "load-generator-${count.index + 1}"
    Type = "LoadTesting"
  }
}

resource "aws_security_group" "load_testing" {
  name_description = "Security group for load testing cluster"
  
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  
  ingress {
    from_port   = 4001
    to_port     = 4001
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/8"]
  }
  
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}
```

**Ansible playbook для настройки**:
```yaml
# ansible/load-testing-setup.yml
---
- hosts: load_generators
  become: yes
  tasks:
    - name: Install performance monitoring tools
      apt:
        name:
          - htop
          - iotop
          - nethogs
          - tcpdump
          - wireshark-common
        state: present
        
    - name: Configure kernel parameters for high load
      sysctl:
        name: "{{ item.name }}"
        value: "{{ item.value }}"
        state: present
        reload: yes
      loop:
        - { name: 'net.core.somaxconn', value: '65535' }
        - { name: 'net.core.netdev_max_backlog', value: '5000' }
        - { name: 'net.ipv4.tcp_max_syn_backlog', value: '8192' }
        - { name: 'fs.file-max', value: '2097152' }
        
    - name: Deploy load testing containers
      docker_container:
        name: "{{ item.name }}"
        image: "{{ item.image }}"
        state: started
        restart_policy: always
        env: "{{ item.env }}"
        ports: "{{ item.ports | default([]) }}"
      loop:
        - name: extreme-load-generator
          image: task7/extreme-load-generator:latest
          env:
            TARGET_CLUSTER: "{{ target_cluster_ip }}"
            MAX_CONNECTIONS: "50000"
            MAX_RPS: "200000"
        - name: concurrent-tester
          image: task7/concurrent-connection-tester:latest
          env:
            TARGET_CLUSTER: "{{ target_cluster_ip }}"
            CONNECTION_COUNT: "10000"
          ports:
            - "8080:8080"
```

## Сетевые взаимодействия

### Load Testing → Test Cluster
**Архитектурная роль**: Генерация нагрузки на тестируемую систему

**Реализация сетевого взаимодействия**:
```go
// loadtest/network_client.go
type NetworkClient struct {
    targetCluster    string
    connectionPool   *ConnectionPool
    loadBalancer     *LoadBalancer
    metricsCollector *NetworkMetricsCollector
}

func (nc *NetworkClient) EstablishConnections(count int) error {
    // Создание пула соединений к кластеру
    pool, err := nc.connectionPool.CreatePool(&PoolConfig{
        Size:        count,
        Target:      nc.targetCluster,
        Protocol:    "tcp",
        KeepAlive:   true,
        Timeout:     30 * time.Second,
    })
    if err != nil {
        return fmt.Errorf("failed to create connection pool: %w", err)
    }
    
    // Распределение соединений по узлам кластера
    clusterNodes, err := nc.discoverClusterNodes()
    if err != nil {
        return fmt.Errorf("failed to discover cluster nodes: %w", err)
    }
    
    connectionsPerNode := count / len(clusterNodes)
    
    var wg sync.WaitGroup
    for i, node := range clusterNodes {
        wg.Add(1)
        go func(nodeAddr string, connCount int) {
            defer wg.Done()
            
            for j := 0; j < connCount; j++ {
                conn, err := net.DialTimeout("tcp", nodeAddr, 10*time.Second)
                if err != nil {
                    nc.metricsCollector.RecordConnectionError(nodeAddr, err)
                    continue
                }
                
                pool.AddConnection(conn)
                nc.metricsCollector.RecordSuccessfulConnection(nodeAddr)
            }
        }(node, connectionsPerNode)
    }
    
    wg.Wait()
    return nil
}
```

### Monitoring → All Systems
**Архитектурная роль**: Сбор метрик со всех компонентов

**Реализация сбора метрик**:
```go
// monitoring/metrics_collector.go
type DistributedMetricsCollector struct {
    prometheusClient *prometheus.Client
    targets          map[string]*MetricsTarget
    scrapeInterval   time.Duration
}

func (dmc *DistributedMetricsCollector) StartCollection() error {
    ticker := time.NewTicker(dmc.scrapeInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            dmc.collectFromAllTargets()
        case <-dmc.ctx.Done():
            return nil
        }
    }
}

func (dmc *DistributedMetricsCollector) collectFromAllTargets() {
    var wg sync.WaitGroup
    
    for name, target := range dmc.targets {
        wg.Add(1)
        go func(targetName string, t *MetricsTarget) {
            defer wg.Done()
            
            metrics, err := dmc.scrapeTarget(t)
            if err != nil {
                log.Printf("Failed to scrape %s: %v", targetName, err)
                return
            }
            
            // Отправка метрик в Prometheus
            dmc.prometheusClient.Push(targetName, metrics)
        }(name, target)
    }
    
    wg.Wait()
}
```

## Автоматизация развертывания

### Infrastructure as Code
```bash
#!/bin/bash
# deploy-task7-infrastructure.sh

set -e

echo "Deploying Task 7 Testing Infrastructure..."

# 1. Deploy Kubernetes cluster
echo "Setting up Kubernetes cluster..."
terraform -chdir=terraform/k8s apply -auto-approve

# 2. Deploy monitoring infrastructure
echo "Setting up monitoring..."
kubectl apply -f k8s/monitoring/

# 3. Deploy test cluster
echo "Setting up test cluster..."
kubectl apply -f k8s/test-cluster/

# 4. Deploy load testing infrastructure
echo "Setting up load testing cluster..."
terraform -chdir=terraform/load-testing apply -auto-approve

# 5. Configure networking
echo "Configuring network policies..."
kubectl apply -f k8s/network-policies/

# 6. Wait for all services to be ready
echo "Waiting for services to be ready..."
kubectl wait --for=condition=ready pod -l app=ipfs-cluster --timeout=300s
kubectl wait --for=condition=ready pod -l app=prometheus --timeout=300s

echo "Infrastructure deployment completed!"
echo "Access points:"
echo "  - Grafana: http://$(kubectl get svc grafana -o jsonpath='{.status.loadBalancer.ingress[0].ip}'):3000"
echo "  - Prometheus: http://$(kubectl get svc prometheus -o jsonpath='{.status.loadBalancer.ingress[0].ip}'):9090"
echo "  - Test Orchestrator: http://$(kubectl get svc test-orchestrator-service -o jsonpath='{.status.loadBalancer.ingress[0].ip}'):8080"
```

Эта диаграмма служит основой для понимания физического развертывания и настройки инфраструктуры для выполнения всех тестов Task 7.