package bitswap_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bitswap "github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/boxo/bitswap/circuitbreaker"
	testinstance "github.com/ipfs/boxo/bitswap/testinstance"
	tn "github.com/ipfs/boxo/bitswap/testnet"
	mockrouting "github.com/ipfs/boxo/routing/mock"
	blocks "github.com/ipfs/go-block-format"
	delay "github.com/ipfs/go-ipfs-delay"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ChaosMonkey represents a chaos engineering controller
type ChaosMonkey struct {
	instances   []testinstance.Instance
	testBlocks  []blocks.Block
	ctx         context.Context
	net         tn.Network
	ig          *testinstance.InstanceGenerator
	mu          sync.RWMutex
	killedNodes map[int]bool
	partitions  map[string]bool
	isActive    bool
	events      []ChaosEvent
}

// ChaosEvent represents a chaos engineering event
type ChaosEvent struct {
	Type      string
	Timestamp time.Time
	NodeID    int
	Details   string
}

// NewChaosMonkey creates a new chaos engineering controller
func NewChaosMonkey(ctx context.Context, instances []testinstance.Instance, testBlocks []blocks.Block, net tn.Network, ig *testinstance.InstanceGenerator) *ChaosMonkey {
	return &ChaosMonkey{
		instances:   instances,
		testBlocks:  testBlocks,
		ctx:         ctx,
		net:         net,
		ig:          ig,
		killedNodes: make(map[int]bool),
		partitions:  make(map[string]bool),
		events:      make([]ChaosEvent, 0),
	}
}

// Start begins chaos engineering activities
func (cm *ChaosMonkey) Start(duration time.Duration) {
	cm.mu.Lock()
	cm.isActive = true
	cm.mu.Unlock()

	chaosCtx, cancel := context.WithTimeout(cm.ctx, duration)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-chaosCtx.Done():
			cm.mu.Lock()
			cm.isActive = false
			cm.mu.Unlock()
			return
		case <-ticker.C:
			cm.executeRandomChaos()
		}
	}
}

// executeRandomChaos executes a random chaos event
func (cm *ChaosMonkey) executeRandomChaos() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.instances) == 0 {
		return
	}

	chaosType := rand.Intn(6)
	switch chaosType {
	case 0:
		cm.killRandomNode()
	case 1:
		cm.reviveRandomNode()
	case 2:
		cm.createNetworkPartition()
	case 3:
		cm.healNetworkPartition()
	case 4:
		cm.injectHighLatency()
	case 5:
		cm.simulateResourceExhaustion()
	}
}

// killRandomNode kills a random active node
func (cm *ChaosMonkey) killRandomNode() {
	aliveNodes := make([]int, 0)
	for i := 0; i < len(cm.instances); i++ {
		if !cm.killedNodes[i] {
			aliveNodes = append(aliveNodes, i)
		}
	}

	if len(aliveNodes) <= 2 { // Keep at least 2 nodes alive
		return
	}

	nodeIdx := aliveNodes[rand.Intn(len(aliveNodes))]
	cm.instances[nodeIdx].Exchange.Close()
	cm.killedNodes[nodeIdx] = true

	event := ChaosEvent{
		Type:      "NODE_KILL",
		Timestamp: time.Now(),
		NodeID:    nodeIdx,
		Details:   fmt.Sprintf("Killed node %d", nodeIdx),
	}
	cm.events = append(cm.events, event)
}

// reviveRandomNode revives a random killed node
func (cm *ChaosMonkey) reviveRandomNode() {
	killedNodesList := make([]int, 0)
	for nodeIdx, killed := range cm.killedNodes {
		if killed {
			killedNodesList = append(killedNodesList, nodeIdx)
		}
	}

	if len(killedNodesList) == 0 {
		return
	}

	nodeIdx := killedNodesList[rand.Intn(len(killedNodesList))]
	cm.instances[nodeIdx] = cm.ig.Next()
	delete(cm.killedNodes, nodeIdx)

	// Reconnect to some random nodes
	for i := 0; i < 3 && i < len(cm.instances); i++ {
		targetIdx := rand.Intn(len(cm.instances))
		if targetIdx != nodeIdx && !cm.killedNodes[targetIdx] {
			cm.instances[nodeIdx].Exchange.(*bitswap.Bitswap).Network().Connect(
				cm.ctx, peer.AddrInfo{ID: cm.instances[targetIdx].Identity.ID()})
		}
	}

	event := ChaosEvent{
		Type:      "NODE_REVIVE",
		Timestamp: time.Now(),
		NodeID:    nodeIdx,
		Details:   fmt.Sprintf("Revived node %d", nodeIdx),
	}
	cm.events = append(cm.events, event)
}

// createNetworkPartition creates a random network partition
func (cm *ChaosMonkey) createNetworkPartition() {
	if len(cm.instances) < 4 {
		return
	}

	node1 := rand.Intn(len(cm.instances))
	node2 := rand.Intn(len(cm.instances))

	if node1 == node2 || cm.killedNodes[node1] || cm.killedNodes[node2] {
		return
	}

	partitionKey := fmt.Sprintf("%d-%d", min(node1, node2), max(node1, node2))
	if cm.partitions[partitionKey] {
		return // Already partitioned
	}

	cm.instances[node1].Exchange.(*bitswap.Bitswap).Network().DisconnectFrom(cm.ctx, cm.instances[node2].Identity.ID())
	cm.partitions[partitionKey] = true

	event := ChaosEvent{
		Type:      "NETWORK_PARTITION",
		Timestamp: time.Now(),
		NodeID:    node1,
		Details:   fmt.Sprintf("Partitioned nodes %d and %d", node1, node2),
	}
	cm.events = append(cm.events, event)
}

// healNetworkPartition heals a random network partition
func (cm *ChaosMonkey) healNetworkPartition() {
	if len(cm.partitions) == 0 {
		return
	}

	// Get random partition to heal
	partitionKeys := make([]string, 0, len(cm.partitions))
	for key := range cm.partitions {
		partitionKeys = append(partitionKeys, key)
	}

	partitionKey := partitionKeys[rand.Intn(len(partitionKeys))]
	var node1, node2 int
	fmt.Sscanf(partitionKey, "%d-%d", &node1, &node2)

	if !cm.killedNodes[node1] && !cm.killedNodes[node2] {
		cm.instances[node1].Exchange.(*bitswap.Bitswap).Network().Connect(
			cm.ctx, peer.AddrInfo{ID: cm.instances[node2].Identity.ID()})
	}

	delete(cm.partitions, partitionKey)

	event := ChaosEvent{
		Type:      "NETWORK_HEAL",
		Timestamp: time.Now(),
		NodeID:    node1,
		Details:   fmt.Sprintf("Healed partition between nodes %d and %d", node1, node2),
	}
	cm.events = append(cm.events, event)
}

// injectHighLatency simulates high latency conditions
func (cm *ChaosMonkey) injectHighLatency() {
	// This is a placeholder for latency injection
	// In a real implementation, this would modify network delays
	event := ChaosEvent{
		Type:      "HIGH_LATENCY",
		Timestamp: time.Now(),
		NodeID:    -1,
		Details:   "Injected high latency conditions",
	}
	cm.events = append(cm.events, event)
}

// simulateResourceExhaustion simulates resource exhaustion
func (cm *ChaosMonkey) simulateResourceExhaustion() {
	// This is a placeholder for resource exhaustion simulation
	// In a real implementation, this would create memory/CPU pressure
	event := ChaosEvent{
		Type:      "RESOURCE_EXHAUSTION",
		Timestamp: time.Now(),
		NodeID:    -1,
		Details:   "Simulated resource exhaustion",
	}
	cm.events = append(cm.events, event)
}

// GetEvents returns all chaos events
func (cm *ChaosMonkey) GetEvents() []ChaosEvent {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return append([]ChaosEvent(nil), cm.events...)
}

// IsActive returns whether chaos monkey is currently active
func (cm *ChaosMonkey) IsActive() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.isActive
}

// TestChaosEngineeringWithMonkey tests system behavior under controlled chaos
func TestChaosEngineeringWithMonkey(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Run("ChaosMonkeyStabilityTest", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(25 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(12)
		testBlocks := generateTestBlocks(t, 40)

		// Distribute blocks across instances
		for i, block := range testBlocks {
			err := instances[i%4].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		// Create chaos monkey
		chaosMonkey := NewChaosMonkey(ctx, instances, testBlocks, net, &ig)

		// Start chaos monkey
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			chaosMonkey.Start(30 * time.Second)
		}()

		// Run continuous load while chaos is happening
		var totalRequests, successfulRequests int64
		loadCtx, loadCancel := context.WithTimeout(ctx, 35*time.Second)
		defer loadCancel()

		for i := 0; i < 150; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

				atomic.AddInt64(&totalRequests, 1)

				// Try to use a random alive node
				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(loadCtx, 3*time.Second)
				defer fetchCancel()

				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				}
			}(i)
		}

		wg.Wait()

		// Analyze results
		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		successRate := float64(successful) / float64(total)

		events := chaosMonkey.GetEvents()
		t.Logf("Chaos Monkey Test Results:")
		t.Logf("  Total requests: %d", total)
		t.Logf("  Successful requests: %d", successful)
		t.Logf("  Success rate: %.2f%%", successRate*100)
		t.Logf("  Chaos events: %d", len(events))

		// Log chaos events
		for _, event := range events {
			t.Logf("  Event: %s at %v - %s", event.Type, event.Timestamp.Format("15:04:05"), event.Details)
		}

		// System should maintain reasonable functionality despite chaos
		assert.Greater(t, successRate, 0.25, "System should maintain >25% success rate under chaos")
		assert.Greater(t, len(events), 5, "Chaos monkey should have generated multiple events")
		assert.Greater(t, total, int64(100), "Should have processed substantial requests")
	})

	t.Run("ChaosWithCircuitBreaker", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(15 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(8)
		testBlocks := generateTestBlocks(t, 20)

		// Add blocks to instances
		for i, block := range testBlocks {
			err := instances[i%3].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		// Create circuit breaker
		config := circuitbreaker.Config{
			MaxRequests: 5,
			Interval:    1 * time.Second,
			Timeout:     2 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
		}
		bcb := circuitbreaker.NewBitswapCircuitBreaker(config)

		// Create chaos monkey
		chaosMonkey := NewChaosMonkey(ctx, instances, testBlocks, net, &ig)

		// Start chaos and circuit breaker test
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			chaosMonkey.Start(20 * time.Second)
		}()

		// Test circuit breaker behavior under chaos
		var cbSuccessful, cbRejected, cbFailed int64

		for i := 0; i < 80; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

				nodeIdx := rand.Intn(len(instances))
				peerID := instances[nodeIdx].Peer

				err := bcb.ExecuteWithPeer(peerID, func() error {
					blockIdx := rand.Intn(len(testBlocks))
					fetchCtx, fetchCancel := context.WithTimeout(ctx, 2*time.Second)
					defer fetchCancel()

					_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
					return err
				})

				if err == nil {
					atomic.AddInt64(&cbSuccessful, 1)
				} else if err == circuitbreaker.ErrOpenState || err == circuitbreaker.ErrTooManyRequests {
					atomic.AddInt64(&cbRejected, 1)
				} else {
					atomic.AddInt64(&cbFailed, 1)
				}
			}(i)
		}

		wg.Wait()

		events := chaosMonkey.GetEvents()
		t.Logf("Chaos + Circuit Breaker Results:")
		t.Logf("  Successful: %d", cbSuccessful)
		t.Logf("  Rejected by CB: %d", cbRejected)
		t.Logf("  Failed: %d", cbFailed)
		t.Logf("  Chaos events: %d", len(events))

		// Circuit breaker should protect the system
		assert.Greater(t, cbSuccessful, int64(10), "Should have successful requests")
		assert.Greater(t, cbRejected, int64(5), "Circuit breaker should reject some requests")
	})
}

// TestNetworkFailureInjection tests specific network failure injection scenarios
func TestNetworkFailureInjection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	t.Run("MessageDropInjection", func(t *testing.T) {
		// Create a custom network that drops messages randomly
		net := &MessageDroppingNetwork{
			VirtualNetwork: tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond)),
			dropRate:       0.2, // 20% message drop rate
		}

		instances := make([]bitswap.Instance, 6)
		for i := 0; i < 6; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "drop-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(t, 15)

		// Add blocks to first instance
		for _, block := range testBlocks {
			err := instances[0].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		// Test system behavior with message drops
		var successCount, failureCount int64
		var wg sync.WaitGroup

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()

				nodeIdx := 1 + rand.Intn(len(instances)-1) // Don't use node 0 (has all blocks)
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(ctx, 4*time.Second) // Longer timeout for drops
				defer fetchCancel()

				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failureCount, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Message drop test results: %d successes, %d failures", successCount, failureCount)
		assert.Greater(t, successCount, int64(15), "System should handle message drops")
	})

	t.Run("DuplicateMessageInjection", func(t *testing.T) {
		// Create a custom network that duplicates messages
		net := &MessageDuplicatingNetwork{
			VirtualNetwork: tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond)),
			duplicateRate:  0.3, // 30% message duplication rate
		}

		instances := make([]bitswap.Instance, 5)
		for i := 0; i < 5; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "duplicate-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(t, 10)

		// Add blocks to first instance
		for _, block := range testBlocks {
			err := instances[0].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		// Test system behavior with duplicate messages
		var successCount int64
		var wg sync.WaitGroup

		for i := 0; i < 30; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()

				nodeIdx := 1 + rand.Intn(len(instances)-1)
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(ctx, 3*time.Second)
				defer fetchCancel()

				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successCount, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Message duplication test results: %d successes", successCount)
		assert.Greater(t, successCount, int64(20), "System should handle duplicate messages")
	})
}

// MessageDroppingNetwork wraps a virtual network and randomly drops messages
type MessageDroppingNetwork struct {
	tn.Network
	dropRate float64
}

func (n *MessageDroppingNetwork) SendMessage(ctx context.Context, from peer.ID, to peer.ID, msg interface{}) error {
	if rand.Float64() < n.dropRate {
		// Drop the message
		return nil
	}
	return n.Network.SendMessage(ctx, from, to, msg)
}

// MessageDuplicatingNetwork wraps a virtual network and randomly duplicates messages
type MessageDuplicatingNetwork struct {
	tn.Network
	duplicateRate float64
}

func (n *MessageDuplicatingNetwork) SendMessage(ctx context.Context, from peer.ID, to peer.ID, msg interface{}) error {
	// Send original message
	err := n.Network.SendMessage(ctx, from, to, msg)
	if err != nil {
		return err
	}

	// Randomly send duplicate
	if rand.Float64() < n.duplicateRate {
		// Send duplicate with slight delay
		go func() {
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
			n.Network.SendMessage(ctx, from, to, msg)
		}()
	}

	return nil
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
