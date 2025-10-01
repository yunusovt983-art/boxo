# Migration Guide

This guide provides step-by-step instructions for upgrading existing Boxo installations to use high-load optimizations.

## Pre-Migration Checklist

### System Requirements Assessment

```bash
#!/bin/bash
# pre-migration-check.sh

echo "=== Pre-Migration System Assessment ==="

# Check current system resources
echo "Current Memory:"
free -h

echo "Current Disk Space:"
df -h

echo "Current CPU:"
nproc

echo "Current Network Connections:"
netstat -an | grep :9096 | wc -l

# Check current Boxo version
echo "Current Boxo Version:"
ipfs-cluster-service --version

# Check current configuration
echo "Current Configuration Size:"
wc -l /data/ipfs-cluster/service.json

# Backup current state
echo "Creating backup..."
tar -czf "pre-migration-backup-$(date +%Y%m%d).tar.gz" /data/ipfs-cluster/

echo "=== Assessment Complete ==="
```

### Compatibility Matrix

| Current Version | Target Version | Migration Path | Downtime Required |
|----------------|----------------|----------------|-------------------|
| v1.0.x | v1.1.x | Direct | < 5 minutes |
| v0.14.x | v1.1.x | Staged | 15-30 minutes |
| v0.13.x | v1.1.x | Full Migration | 1-2 hours |
| < v0.13.x | v1.1.x | Complete Rebuild | 4+ hours |

## Migration Scenarios

### Scenario 1: Direct Upgrade (v1.0.x → v1.1.x)

**Characteristics:**
- Minimal configuration changes
- Backward compatible
- Rolling upgrade possible

**Steps:**

1. **Prepare Migration Environment:**
```bash
# Create migration workspace
mkdir -p /tmp/boxo-migration
cd /tmp/boxo-migration

# Download new configuration templates
curl -O https://raw.githubusercontent.com/ipfs/boxo/main/config/templates/high-load.json
```

2. **Update Configuration:**
```go
// migration-config.go
package main

import (
    "encoding/json"
    "io/ioutil"
    "github.com/ipfs/boxo/config/adaptive"
)

func migrateConfig() error {
    // Read existing configuration
    oldConfig, err := ioutil.ReadFile("/data/ipfs-cluster/service.json")
    if err != nil {
        return err
    }
    
    // Parse and enhance with high-load optimizations
    var config map[string]interface{}
    json.Unmarshal(oldConfig, &config)
    
    // Add high-load optimizations
    config["bitswap"] = map[string]interface{}{
        "max_outstanding_bytes_per_peer": 20 << 20, // 20MB
        "worker_count": 512,
        "batch_size": 500,
    }
    
    config["blockstore"] = map[string]interface{}{
        "memory_cache_size": 2 << 30, // 2GB
        "batch_enabled": true,
    }
    
    // Write enhanced configuration
    newConfig, _ := json.MarshalIndent(config, "", "  ")
    return ioutil.WriteFile("/data/ipfs-cluster/service.json.new", newConfig, 0644)
}
```

3. **Rolling Upgrade Process:**
```bash
#!/bin/bash
# rolling-upgrade.sh

NODES=("node1" "node2" "node3")

for node in "${NODES[@]}"; do
    echo "Upgrading $node..."
    
    # Stop node gracefully
    ssh $node "systemctl stop ipfs-cluster"
    
    # Update binary
    ssh $node "wget -O /usr/local/bin/ipfs-cluster-service https://dist.ipfs.io/ipfs-cluster-service/latest/ipfs-cluster-service"
    ssh $node "chmod +x /usr/local/bin/ipfs-cluster-service"
    
    # Update configuration
    scp service.json.new $node:/data/ipfs-cluster/service.json
    
    # Start node
    ssh $node "systemctl start ipfs-cluster"
    
    # Wait for node to rejoin cluster
    sleep 30
    
    # Verify node health
    ssh $node "ipfs-cluster-ctl peers ls | grep $node"
    
    echo "$node upgrade complete"
done
```

### Scenario 2: Staged Migration (v0.14.x → v1.1.x)

**Characteristics:**
- Significant configuration changes
- Requires careful planning
- Some downtime required

**Steps:**

1. **Configuration Migration Script:**
```go
// staged-migration.go
package main

import (
    "fmt"
    "github.com/ipfs/boxo/config/migration"
)

func performStagedMigration() error {
    migrator := migration.NewMigrator()
    
    // Stage 1: Update core configuration
    stage1 := &migration.Stage{
        Name: "Core Configuration Update",
        Actions: []migration.Action{
            &migration.UpdateBitswapConfig{
                MaxOutstandingBytesPerPeer: 10 << 20,
                WorkerCount: 256,
            },
            &migration.UpdateBlockstoreConfig{
                MemoryCacheSize: 1 << 30,
                BatchEnabled: true,
            },
        },
    }
    
    // Stage 2: Network optimizations
    stage2 := &migration.Stage{
        Name: "Network Optimizations",
        Actions: []migration.Action{
            &migration.UpdateNetworkConfig{
                HighWater: 1000,
                LowWater: 500,
                GracePeriod: "30s",
            },
        },
    }
    
    // Stage 3: Monitoring setup
    stage3 := &migration.Stage{
        Name: "Monitoring Setup",
        Actions: []migration.Action{
            &migration.EnableMetrics{
                PrometheusPort: 9090,
                Interval: "15s",
            },
        },
    }
    
    // Execute migration stages
    for _, stage := range []*migration.Stage{stage1, stage2, stage3} {
        fmt.Printf("Executing stage: %s\n", stage.Name)
        if err := migrator.ExecuteStage(stage); err != nil {
            return fmt.Errorf("stage %s failed: %v", stage.Name, err)
        }
        fmt.Printf("Stage %s completed successfully\n", stage.Name)
    }
    
    return nil
}
```

2. **Data Migration:**
```bash
#!/bin/bash
# data-migration.sh

echo "Starting data migration..."

# Create new data structure
mkdir -p /data/ipfs-cluster-new/{blocks,cache,config}

# Migrate block data
echo "Migrating block data..."
rsync -av --progress /data/ipfs-cluster/blocks/ /data/ipfs-cluster-new/blocks/

# Migrate configuration
echo "Migrating configuration..."
./staged-migration

# Migrate peer information
echo "Migrating peer information..."
cp /data/ipfs-cluster/peerstore /data/ipfs-cluster-new/

# Verify migration
echo "Verifying migration..."
if [ -d "/data/ipfs-cluster-new/blocks" ] && [ -f "/data/ipfs-cluster-new/service.json" ]; then
    echo "Migration verification successful"
    
    # Backup old data
    mv /data/ipfs-cluster /data/ipfs-cluster-backup-$(date +%Y%m%d)
    mv /data/ipfs-cluster-new /data/ipfs-cluster
    
    echo "Data migration complete"
else
    echo "Migration verification failed"
    exit 1
fi
```

### Scenario 3: Full Migration (v0.13.x → v1.1.x)

**Characteristics:**
- Complete configuration overhaul
- Extended downtime required
- Data format changes

**Steps:**

1. **Complete Backup:**
```bash
#!/bin/bash
# full-backup.sh

BACKUP_DIR="/backup/ipfs-cluster-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"

# Backup all data
tar -czf "$BACKUP_DIR/cluster-data.tar.gz" /data/ipfs-cluster/

# Backup system configuration
cp /etc/systemd/system/ipfs-cluster.service "$BACKUP_DIR/"
cp /etc/security/limits.conf "$BACKUP_DIR/"

# Export current pin list
ipfs-cluster-ctl pin ls > "$BACKUP_DIR/pin-list.txt"

# Export peer list
ipfs-cluster-ctl peers ls > "$BACKUP_DIR/peer-list.txt"

echo "Full backup completed in: $BACKUP_DIR"
```

2. **Clean Installation:**
```bash
#!/bin/bash
# clean-install.sh

# Stop all services
systemctl stop ipfs-cluster
systemctl stop ipfs

# Remove old installation
rm -rf /data/ipfs-cluster/*

# Install new version
wget -O /usr/local/bin/ipfs-cluster-service https://dist.ipfs.io/ipfs-cluster-service/latest/ipfs-cluster-service
chmod +x /usr/local/bin/ipfs-cluster-service

# Initialize with high-load configuration
ipfs-cluster-service init --config-template high-load

# Restore peer identity if needed
if [ -f "/backup/identity.json" ]; then
    cp /backup/identity.json /data/ipfs-cluster/
fi
```

3. **Data Recovery:**
```go
// data-recovery.go
package main

import (
    "bufio"
    "context"
    "os"
    "strings"
    "github.com/ipfs/boxo/client"
)

func recoverPins() error {
    // Connect to cluster
    client, err := client.NewDefaultClient()
    if err != nil {
        return err
    }
    
    // Read backup pin list
    file, err := os.Open("/backup/pin-list.txt")
    if err != nil {
        return err
    }
    defer file.Close()
    
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := scanner.Text()
        if strings.Contains(line, "Qm") {
            // Extract CID and re-pin
            parts := strings.Fields(line)
            if len(parts) > 0 {
                cid := parts[0]
                _, err := client.Pin(context.Background(), cid, client.PinOptions{})
                if err != nil {
                    log.Printf("Failed to re-pin %s: %v", cid, err)
                } else {
                    log.Printf("Successfully re-pinned %s", cid)
                }
            }
        }
    }
    
    return scanner.Err()
}
```

## Configuration Migration Templates

### Basic to High-Load Migration

```json
{
  "before": {
    "cluster": {
      "connection_manager": {
        "high_water": 400,
        "low_water": 100,
        "grace_period": "2m"
      }
    }
  },
  "after": {
    "cluster": {
      "connection_manager": {
        "high_water": 2000,
        "low_water": 1000,
        "grace_period": "30s"
      }
    },
    "bitswap": {
      "max_outstanding_bytes_per_peer": 20971520,
      "worker_count": 512,
      "batch_size": 500,
      "batch_timeout": "20ms"
    },
    "blockstore": {
      "memory_cache_size": 2147483648,
      "batch_enabled": true,
      "compression_enabled": true
    },
    "monitoring": {
      "metrics_enabled": true,
      "prometheus_port": 9090,
      "alert_thresholds": {
        "response_time_p95": "100ms",
        "error_rate": 0.05,
        "memory_usage": 0.85
      }
    }
  }
}
```

### Environment-Specific Migrations

#### Development Environment
```bash
# dev-migration.sh
export BOXO_CONFIG_PROFILE="development"
export BOXO_LOG_LEVEL="debug"
export BOXO_METRICS_INTERVAL="60s"

# Use smaller resource limits for development
sed -i 's/"memory_cache_size": 2147483648/"memory_cache_size": 536870912/' /data/ipfs-cluster/service.json
sed -i 's/"high_water": 2000/"high_water": 500/' /data/ipfs-cluster/service.json
```

#### Production Environment
```bash
# prod-migration.sh
export BOXO_CONFIG_PROFILE="production"
export BOXO_LOG_LEVEL="info"
export BOXO_METRICS_INTERVAL="15s"

# Apply production optimizations
sed -i 's/"memory_cache_size": 536870912/"memory_cache_size": 8589934592/' /data/ipfs-cluster/service.json
sed -i 's/"high_water": 500/"high_water": 5000/' /data/ipfs-cluster/service.json

# Enable auto-tuning
echo '{"auto_tuning": {"enabled": true, "interval": "5m"}}' | jq -s '.[0] * .[1]' /data/ipfs-cluster/service.json - > /tmp/service.json
mv /tmp/service.json /data/ipfs-cluster/service.json
```

## Post-Migration Validation

### Performance Validation Script

```bash
#!/bin/bash
# post-migration-validation.sh

echo "=== Post-Migration Validation ==="

# Test basic functionality
echo "Testing basic cluster functionality..."
if ipfs-cluster-ctl peers ls > /dev/null 2>&1; then
    echo "✓ Cluster communication working"
else
    echo "✗ Cluster communication failed"
    exit 1
fi

# Test pin operations
echo "Testing pin operations..."
TEST_CID="QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG"
if ipfs-cluster-ctl pin add $TEST_CID > /dev/null 2>&1; then
    echo "✓ Pin operations working"
    ipfs-cluster-ctl pin rm $TEST_CID > /dev/null 2>&1
else
    echo "✗ Pin operations failed"
fi

# Test metrics endpoint
echo "Testing metrics endpoint..."
if curl -s http://localhost:9090/metrics | grep -q "boxo_"; then
    echo "✓ Metrics endpoint working"
else
    echo "✗ Metrics endpoint not responding"
fi

# Performance baseline test
echo "Running performance baseline test..."
START_TIME=$(date +%s)
for i in {1..100}; do
    ipfs-cluster-ctl peers ls > /dev/null 2>&1
done
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

if [ $DURATION -lt 10 ]; then
    echo "✓ Performance baseline acceptable ($DURATION seconds for 100 operations)"
else
    echo "⚠ Performance baseline concerning ($DURATION seconds for 100 operations)"
fi

echo "=== Validation Complete ==="
```

### Rollback Procedures

#### Quick Rollback (< 1 hour after migration)

```bash
#!/bin/bash
# quick-rollback.sh

echo "Initiating quick rollback..."

# Stop current service
systemctl stop ipfs-cluster

# Restore from backup
BACKUP_DIR=$(ls -t /backup/ | head -1)
tar -xzf "/backup/$BACKUP_DIR/cluster-data.tar.gz" -C /

# Restore old binary
cp "/backup/$BACKUP_DIR/ipfs-cluster-service" /usr/local/bin/

# Start service
systemctl start ipfs-cluster

# Verify rollback
if ipfs-cluster-ctl peers ls > /dev/null 2>&1; then
    echo "Rollback successful"
else
    echo "Rollback failed - manual intervention required"
    exit 1
fi
```

#### Full Rollback (> 1 hour after migration)

```bash
#!/bin/bash
# full-rollback.sh

echo "Initiating full rollback..."

# This requires more careful data reconciliation
# Stop all cluster nodes
for node in node1 node2 node3; do
    ssh $node "systemctl stop ipfs-cluster"
done

# Restore data on each node
for node in node1 node2 node3; do
    echo "Restoring $node..."
    ssh $node "rm -rf /data/ipfs-cluster/*"
    scp "/backup/cluster-data-$node.tar.gz" "$node:/tmp/"
    ssh $node "tar -xzf /tmp/cluster-data-$node.tar.gz -C /"
done

# Start nodes in sequence
for node in node1 node2 node3; do
    ssh $node "systemctl start ipfs-cluster"
    sleep 30  # Allow time for cluster formation
done

echo "Full rollback complete"
```

## Troubleshooting Migration Issues

### Common Migration Problems

1. **Configuration Validation Errors:**
```bash
# Validate configuration before applying
ipfs-cluster-service --config-path /data/ipfs-cluster/service.json.new --dry-run

# Fix common validation issues
jq '.cluster.connection_manager.grace_period = "30s"' service.json > service.json.tmp
mv service.json.tmp service.json
```

2. **Data Format Incompatibilities:**
```bash
# Convert old data format
ipfs-cluster-service migrate --from-version 0.13.0 --to-version 1.1.0 /data/ipfs-cluster/
```

3. **Network Connectivity Issues:**
```bash
# Reset network configuration
ipfs-cluster-service init --force-new-cluster
```

### Migration Monitoring

```bash
#!/bin/bash
# migration-monitor.sh

# Monitor migration progress
while true; do
    echo "$(date): Checking migration status..."
    
    # Check service status
    systemctl is-active ipfs-cluster
    
    # Check cluster health
    PEER_COUNT=$(ipfs-cluster-ctl peers ls 2>/dev/null | wc -l)
    echo "Active peers: $PEER_COUNT"
    
    # Check error rates
    ERROR_COUNT=$(journalctl -u ipfs-cluster --since "1 minute ago" | grep -c ERROR)
    echo "Recent errors: $ERROR_COUNT"
    
    sleep 60
done
```

## Best Practices

### Pre-Migration
1. **Always create complete backups**
2. **Test migration in staging environment first**
3. **Plan for extended maintenance windows**
4. **Notify users of potential service disruption**
5. **Prepare rollback procedures**

### During Migration
1. **Monitor system resources continuously**
2. **Validate each step before proceeding**
3. **Keep detailed logs of all actions**
4. **Have rollback plan ready**
5. **Communicate status to stakeholders**

### Post-Migration
1. **Run comprehensive validation tests**
2. **Monitor performance for 24-48 hours**
3. **Document any issues encountered**
4. **Update operational procedures**
5. **Plan for future migrations**

## Next Steps

After successful migration:
- Review [Configuration Guide](configuration-guide.md) for further optimizations
- Set up monitoring using [Integration Examples](integration-examples/)
- Refer to [Troubleshooting Guide](troubleshooting-guide.md) for ongoing maintenance