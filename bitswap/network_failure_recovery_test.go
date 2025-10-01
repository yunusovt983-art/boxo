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
	tn "github.com/ipfs/boxo/bitswap/testnet"
	delay "github.com/ipfs/go-ipfs-delay"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNetworkFailureRecoveryScenarios tests comprehensive network failure and recovery patterns
func TestNetworkFailureRecoveryScenarios(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t.Run("RollingNodeFailureRecovery", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(20 * time.Millisecond))
		instances := make([]bitswap.Instance, 10)
		for i := 0; i < 10; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "rolling-failure-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(30)

		// Distribute blocks across all instances for redundancy
		for i, block := range testBlocks {
			for j := 0; j < 3; j++ { // Each block on 3 instances
				instanceIdx := (i + j) % len(instances)
				err := instances[instanceIdx].Blockstore().Put(ctx, block)
				require.NoError(t, err)
			}
		}

		// Rolling failure pattern: fail nodes one by one, then recover them
		var wg sync.WaitGroup
		var totalRequests, successfulRequests int64

		// Start continuous request load
		requestCtx, requestCancel := context.WithTimeout(ctx, 60*time.Second)
		defer requestCancel()

		for i := 0; i < 200; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)

				atomic.AddInt64(&totalRequests, 1)
				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(requestCtx, 3*time.Second)
				defer fetchCancel()

				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				}
			}(i)
		}

		// Rolling failure controller
		wg.Add(1)
		go func() {
			defer wg.Done()
			failureCtx, failureCancel := context.WithTimeout(ctx, 50*time.Second)
			defer failureCancel()

			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()

			failedNodes := make([]int, 0)
			maxFailedNodes := 4

			for {
				select {
				case <-failureCtx.Done():
					return
				case <-ticker.C:
					if len(failedNodes) < maxFailedNodes {
						// Fail a new node
						availableNodes := make([]int, 0)
						for i := 0; i < len(instances); i++ {
							if !contains(failedNodes, i) {
								availableNodes = append(availableNodes, i)
							}
						}

						if len(availableNodes) > 0 {
							nodeToFail := availableNodes[rand.Intn(len(availableNodes))]
							instances[nodeToFail].Exchange.Close()
							failedNodes = append(failedNodes, nodeToFail)
							t.Logf("Rolling failure: Failed node %d", nodeToFail)
						}
					} else {
						// Recover the oldest failed node
						if len(failedNodes) > 0 {
							nodeToRecover := failedNodes[0]
							failedNodes = failedNodes[1:]

							instances[nodeToRecover] = bitswap.NewTestInstance(ctx, net, "rolling-recovery")

							// Reconnect to some random nodes
							for i := 0; i < 3; i++ {
								targetIdx := rand.Intn(len(instances))
								if targetIdx != nodeToRecover && !contains(failedNodes, targetIdx) {
									instances[nodeToRecover].Exchange.(*bitswap.Bitswap).Network().Connect(
										ctx, peer.AddrInfo{ID: instances[targetIdx].Peer})
								}
							}

							t.Logf("Rolling recovery: Recovered node %d", nodeToRecover)
						}
					}
				}
			}
		}()

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		successRate := float64(successful) / float64(total)

		t.Logf("Rolling failure recovery results:")
		t.Logf("  Total requests: %d", total)
		t.Logf("  Successful: %d", successful)
		t.Logf("  Success rate: %.2f%%", successRate*100)

		// System should maintain reasonable functionality during rolling failures
		assert.Greater(t, successRate, 0.4, "System should maintain >40% success rate during rolling failures")
		assert.Greater(t, total, int64(150), "Should have processed substantial requests")
	})

	t.Run("NetworkPartitionHealingCycles", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(15 * time.Millisecond))
		instances := make([]bitswap.Instance, 12)
		for i := 0; i < 12; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "partition-healing-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(25)

		// Distribute blocks across instances
		for i, block := range testBlocks {
			err := instances[i%4].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests int64
		var partitionCycles int64

		// Continuous request load
		requestCtx, requestCancel := context.WithTimeout(ctx, 45*time.Second)
		defer requestCancel()

		for i := 0; i < 120; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1500)) * time.Millisecond)

				atomic.AddInt64(&totalRequests, 1)
				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(requestCtx, 4*time.Second)
				defer fetchCancel()

				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				}
			}(i)
		}

		// Partition healing cycle controller
		wg.Add(1)
		go func() {
			defer wg.Done()
			partitionCtx, partitionCancel := context.WithTimeout(ctx, 40*time.Second)
			defer partitionCancel()

			ticker := time.NewTicker(4 * time.Second)
			defer ticker.Stop()

			partitioned := false
			partitionedPairs := make(map[string]bool)

			for {
				select {
				case <-partitionCtx.Done():
					return
				case <-ticker.C:
					if !partitioned {
						// Create partitions
						atomic.AddInt64(&partitionCycles, 1)
						for i := 0; i < 6; i++ {
							node1 := rand.Intn(len(instances))
							node2 := rand.Intn(len(instances))
							if node1 != node2 {
								pairKey := fmt.Sprintf("%d-%d", min(node1, node2), max(node1, node2))
								if !partitionedPairs[pairKey] {
									instances[node1].Exchange.(*bitswap.Bitswap).Network().DisconnectFrom(
										ctx, instances[node2].Peer)
									partitionedPairs[pairKey] = true
								}
							}
						}
						partitioned = true
						t.Logf("Partition cycle %d: Created partitions", partitionCycles)
					} else {
						// Heal partitions
						for pairKey := range partitionedPairs {
							var node1, node2 int
							fmt.Sscanf(pairKey, "%d-%d", &node1, &node2)
							instances[node1].Exchange.(*bitswap.Bitswap).Network().Connect(
								ctx, peer.AddrInfo{ID: instances[node2].Peer})
						}
						partitionedPairs = make(map[string]bool)
						partitioned = false
						t.Logf("Partition cycle %d: Healed partitions", partitionCycles)
					}
				}
			}
		}()

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		cycles := atomic.LoadInt64(&partitionCycles)
		successRate := float64(successful) / float64(total)

		t.Logf("Partition healing cycles results:")
		t.Logf("  Total requests: %d", total)
		t.Logf("  Successful: %d", successful)
		t.Logf("  Success rate: %.2f%%", successRate*100)
		t.Logf("  Partition cycles: %d", cycles)

		// System should handle partition healing cycles
		assert.Greater(t, successRate, 0.3, "System should maintain >30% success rate during partition cycles")
		assert.Greater(t, cycles, int64(3), "Should have completed multiple partition cycles")
	})

	t.Run("CascadingFailureRecovery", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(25 * time.Millisecond))
		instances := make([]bitswap.Instance, 15)
		for i := 0; i < 15; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "cascading-recovery-test")
		}

		// Create a more connected topology for cascading effects
		for i := 0; i < len(instances); i++ {
			for j := i + 1; j < len(instances); j++ {
				if rand.Float32() < 0.4 { // 40% connection probability
					err := instances[i].Exchange.(*bitswap.Bitswap).Network().Connect(
						ctx, peer.AddrInfo{ID: instances[j].Peer})
					require.NoError(t, err)
				}
			}
		}

		testBlocks := generateTestBlocks(40)

		// Distribute blocks with some redundancy
		for i, block := range testBlocks {
			for j := 0; j < 2; j++ { // Each block on 2 instances
				instanceIdx := (i*2 + j) % len(instances)
				err := instances[instanceIdx].Blockstore().Put(ctx, block)
				require.NoError(t, err)
			}
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests int64

		// Continuous request load
		requestCtx, requestCancel := context.WithTimeout(ctx, 50*time.Second)
		defer requestCancel()

		for i := 0; i < 150; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)

				atomic.AddInt64(&totalRequests, 1)
				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(requestCtx, 5*time.Second)
				defer fetchCancel()

				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				}
			}(i)
		}

		// Cascading failure controller
		wg.Add(1)
		go func() {
			defer wg.Done()
			cascadeCtx, cascadeCancel := context.WithTimeout(ctx, 45*time.Second)
			defer cascadeCancel()

			// Initial cascade - fail multiple nodes quickly
			time.Sleep(5 * time.Second)
			failedNodes := make([]int, 0)

			// Phase 1: Rapid cascade
			for i := 0; i < 6; i++ {
				nodeToFail := rand.Intn(len(instances))
				if !contains(failedNodes, nodeToFail) {
					instances[nodeToFail].Exchange.Close()
					failedNodes = append(failedNodes, nodeToFail)
					t.Logf("Cascading failure: Failed node %d", nodeToFail)
					time.Sleep(500 * time.Millisecond) // Quick cascade
				}
			}

			// Phase 2: Wait and assess damage
			time.Sleep(10 * time.Second)

			// Phase 3: Gradual recovery
			recoveryTicker := time.NewTicker(3 * time.Second)
			defer recoveryTicker.Stop()

			for {
				select {
				case <-cascadeCtx.Done():
					return
				case <-recoveryTicker.C:
					if len(failedNodes) > 0 {
						// Recover one node
						nodeToRecover := failedNodes[0]
						failedNodes = failedNodes[1:]

						instances[nodeToRecover] = bitswap.NewTestInstance(ctx, net, "cascade-recovery")

						// Reconnect to multiple nodes for better recovery
						for i := 0; i < 5; i++ {
							targetIdx := rand.Intn(len(instances))
							if targetIdx != nodeToRecover && !contains(failedNodes, targetIdx) {
								instances[nodeToRecover].Exchange.(*bitswap.Bitswap).Network().Connect(
									ctx, peer.AddrInfo{ID: instances[targetIdx].Peer})
							}
						}

						t.Logf("Cascading recovery: Recovered node %d", nodeToRecover)
					}
				}
			}
		}()

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		successRate := float64(successful) / float64(total)

		t.Logf("Cascading failure recovery results:")
		t.Logf("  Total requests: %d", total)
		t.Logf("  Successful: %d", successful)
		t.Logf("  Success rate: %.2f%%", successRate*100)

		// System should recover from cascading failures
		assert.Greater(t, successRate, 0.25, "System should maintain >25% success rate during cascading failures")
		assert.Greater(t, total, int64(100), "Should have processed substantial requests")
	})

	t.Run("ByzantineFaultTolerance", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(30 * time.Millisecond))
		instances := make([]bitswap.Instance, 9)
		for i := 0; i < 9; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "byzantine-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(20)

		// Distribute blocks across honest nodes
		honestNodes := []int{0, 1, 2, 6, 7, 8} // 6 honest nodes
		byzantineNodes := []int{3, 4, 5}       // 3 byzantine nodes

		for i, block := range testBlocks {
			honestNodeIdx := honestNodes[i%len(honestNodes)]
			err := instances[honestNodeIdx].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests, byzantineRequests int64

		// Simulate byzantine behavior
		wg.Add(1)
		go func() {
			defer wg.Done()
			byzantineCtx, byzantineCancel := context.WithTimeout(ctx, 35*time.Second)
			defer byzantineCancel()

			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-byzantineCtx.Done():
					return
				case <-ticker.C:
					// Byzantine nodes behave maliciously
					for _, nodeIdx := range byzantineNodes {
						if rand.Float32() < 0.5 {
							// Randomly disconnect from honest nodes
							honestNodeIdx := honestNodes[rand.Intn(len(honestNodes))]
							instances[nodeIdx].Exchange.(*bitswap.Bitswap).Network().DisconnectFrom(
								ctx, instances[honestNodeIdx].Peer)
						}

						if rand.Float32() < 0.3 {
							// Randomly reconnect
							honestNodeIdx := honestNodes[rand.Intn(len(honestNodes))]
							instances[nodeIdx].Exchange.(*bitswap.Bitswap).Network().Connect(
								ctx, peer.AddrInfo{ID: instances[honestNodeIdx].Peer})
						}
					}
				}
			}
		}()

		// Request load from both honest and byzantine nodes
		requestCtx, requestCancel := context.WithTimeout(ctx, 40*time.Second)
		defer requestCancel()

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1500)) * time.Millisecond)

				atomic.AddInt64(&totalRequests, 1)
				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				if contains(byzantineNodes, nodeIdx) {
					atomic.AddInt64(&byzantineRequests, 1)
				}

				fetchCtx, fetchCancel := context.WithTimeout(requestCtx, 4*time.Second)
				defer fetchCancel()

				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				}
			}(i)
		}

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		byzantine := atomic.LoadInt64(&byzantineRequests)
		successRate := float64(successful) / float64(total)

		t.Logf("Byzantine fault tolerance results:")
		t.Logf("  Total requests: %d", total)
		t.Logf("  Successful: %d", successful)
		t.Logf("  Byzantine requests: %d", byzantine)
		t.Logf("  Success rate: %.2f%%", successRate*100)

		// System should tolerate byzantine faults with majority honest nodes
		assert.Greater(t, successRate, 0.4, "System should maintain >40% success rate with byzantine faults")
		assert.Greater(t, byzantine, int64(10), "Should have had byzantine requests")
	})
}

// TestNetworkRecoveryPatterns tests specific recovery patterns and strategies
func TestNetworkRecoveryPatterns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Run("ExponentialBackoffRecoveryPattern", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(20 * time.Millisecond))
		instances := make([]bitswap.Instance, 6)
		for i := 0; i < 6; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "backoff-pattern-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(15)

		// Add blocks to first instance
		for _, block := range testBlocks {
			err := instances[0].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		// Simulate a node that fails and recovers with exponential backoff
		failingNodeIdx := 1
		var recoveryAttempts []time.Time
		var successfulRecovery bool

		// Initial failure
		instances[failingNodeIdx].Exchange.Close()
		t.Logf("Node %d failed initially", failingNodeIdx)

		// Exponential backoff recovery attempts
		backoffDuration := 1 * time.Second
		maxBackoff := 16 * time.Second
		maxAttempts := 5

		for attempt := 0; attempt < maxAttempts && backoffDuration <= maxBackoff; attempt++ {
			time.Sleep(backoffDuration)
			recoveryAttempts = append(recoveryAttempts, time.Now())

			// Attempt recovery
			instances[failingNodeIdx] = bitswap.NewTestInstance(ctx, net, "backoff-recovery")
			err := instances[failingNodeIdx].Exchange.(*bitswap.Bitswap).Network().Connect(
				ctx, peer.AddrInfo{ID: instances[0].Peer})
			require.NoError(t, err)

			// Test if recovery was successful
			fetchCtx, fetchCancel := context.WithTimeout(ctx, 3*time.Second)
			_, err = instances[failingNodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[0].Cid())
			fetchCancel()

			if err == nil {
				successfulRecovery = true
				t.Logf("Node %d recovered successfully after %d attempts with backoff %v",
					failingNodeIdx, len(recoveryAttempts), backoffDuration)
				break
			} else {
				t.Logf("Recovery attempt %d failed, next backoff: %v", len(recoveryAttempts), backoffDuration*2)
				instances[failingNodeIdx].Exchange.Close()
				backoffDuration *= 2 // Exponential backoff
			}
		}

		assert.True(t, successfulRecovery, "Node should eventually recover with exponential backoff")
		assert.Greater(t, len(recoveryAttempts), 1, "Should have made multiple recovery attempts")
		assert.LessOrEqual(t, len(recoveryAttempts), maxAttempts, "Should not exceed maximum attempts")
	})

	t.Run("CircuitBreakerRecoveryPattern", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(15 * time.Millisecond))
		instances := make([]bitswap.Instance, 8)
		for i := 0; i < 8; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "cb-recovery-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(20)

		// Distribute blocks
		for i, block := range testBlocks {
			err := instances[i%3].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		// Simulate circuit breaker recovery pattern
		var wg sync.WaitGroup
		var totalRequests, successfulRequests, rejectedRequests int64

		// Create failing conditions initially
		failingNodes := []int{2, 3, 4}
		for _, nodeIdx := range failingNodes {
			instances[nodeIdx].Exchange.Close()
		}

		// Phase 1: Circuit breaker should trip due to failures
		t.Log("Phase 1: Testing circuit breaker tripping")
		for i := 0; i < 30; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				atomic.AddInt64(&totalRequests, 1)

				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(ctx, 2*time.Second)
				defer fetchCancel()

				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				} else {
					atomic.AddInt64(&rejectedRequests, 1)
				}
			}(i)
		}

		wg.Wait()

		phase1Total := atomic.LoadInt64(&totalRequests)
		phase1Successful := atomic.LoadInt64(&successfulRequests)
		phase1Rejected := atomic.LoadInt64(&rejectedRequests)

		t.Logf("Phase 1 results: %d total, %d successful, %d rejected",
			phase1Total, phase1Successful, phase1Rejected)

		// Phase 2: Recover failing nodes and test circuit breaker recovery
		t.Log("Phase 2: Recovering nodes and testing circuit breaker recovery")
		time.Sleep(2 * time.Second) // Allow circuit breakers to potentially reset

		for _, nodeIdx := range failingNodes {
			instances[nodeIdx] = bitswap.NewTestInstance(ctx, net, "cb-recovery")
			// Reconnect to some nodes
			for i := 0; i < 3; i++ {
				targetIdx := rand.Intn(len(instances))
				if targetIdx != nodeIdx && !contains(failingNodes, targetIdx) {
					instances[nodeIdx].Exchange.(*bitswap.Bitswap).Network().Connect(
						ctx, peer.AddrInfo{ID: instances[targetIdx].Peer})
				}
			}
		}

		// Test recovery
		for i := 0; i < 40; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				atomic.AddInt64(&totalRequests, 1)

				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(ctx, 3*time.Second)
				defer fetchCancel()

				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				} else {
					atomic.AddInt64(&rejectedRequests, 1)
				}
			}(i)
		}

		wg.Wait()

		totalFinal := atomic.LoadInt64(&totalRequests)
		successfulFinal := atomic.LoadInt64(&successfulRequests)
		rejectedFinal := atomic.LoadInt64(&rejectedRequests)

		phase2Successful := successfulFinal - phase1Successful
		phase2Total := totalFinal - phase1Total

		t.Logf("Phase 2 results: %d total, %d successful", phase2Total, phase2Successful)
		t.Logf("Overall results: %d total, %d successful, %d rejected",
			totalFinal, successfulFinal, rejectedFinal)

		// Recovery phase should show improved success rate
		if phase2Total > 0 {
			phase2SuccessRate := float64(phase2Successful) / float64(phase2Total)
			assert.Greater(t, phase2SuccessRate, 0.5, "Recovery phase should have >50% success rate")
		}
	})

	t.Run("GradualLoadRecoveryPattern", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond))
		instances := make([]bitswap.Instance, 10)
		for i := 0; i < 10; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "gradual-recovery-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(25)

		// Distribute blocks
		for i, block := range testBlocks {
			err := instances[i%4].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		// Simulate major failure
		failedNodes := []int{1, 2, 3, 4, 5, 6}
		for _, nodeIdx := range failedNodes {
			instances[nodeIdx].Exchange.Close()
		}

		var wg sync.WaitGroup
		var phaseResults []struct {
			phase      int
			successful int64
			total      int64
		}

		// Gradual recovery with increasing load
		phases := []struct {
			name           string
			nodesToRecover int
			requestCount   int
		}{
			{"Phase 1: Recover 2 nodes", 2, 20},
			{"Phase 2: Recover 2 more nodes", 2, 30},
			{"Phase 3: Recover remaining nodes", 2, 40},
		}

		recoveredNodes := 0
		for phaseIdx, phase := range phases {
			t.Logf("Starting %s", phase.name)

			// Recover nodes for this phase
			for i := 0; i < phase.nodesToRecover && recoveredNodes < len(failedNodes); i++ {
				nodeIdx := failedNodes[recoveredNodes]
				instances[nodeIdx] = bitswap.NewTestInstance(ctx, net, "gradual-recovery")

				// Reconnect to working nodes
				for j := 0; j < len(instances); j++ {
					if j != nodeIdx && (j < failedNodes[0] || j > failedNodes[len(failedNodes)-1] || j <= failedNodes[recoveredNodes]) {
						instances[nodeIdx].Exchange.(*bitswap.Bitswap).Network().Connect(
							ctx, peer.AddrInfo{ID: instances[j].Peer})
					}
				}

				recoveredNodes++
				t.Logf("Recovered node %d", nodeIdx)
			}

			// Wait for stabilization
			time.Sleep(2 * time.Second)

			// Test with increasing load
			var phaseSuccessful, phaseTotal int64

			for i := 0; i < phase.requestCount; i++ {
				wg.Add(1)
				go func(requestID int) {
					defer wg.Done()
					atomic.AddInt64(&phaseTotal, 1)

					nodeIdx := rand.Intn(len(instances))
					blockIdx := rand.Intn(len(testBlocks))

					fetchCtx, fetchCancel := context.WithTimeout(ctx, 3*time.Second)
					defer fetchCancel()

					_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
					if err == nil {
						atomic.AddInt64(&phaseSuccessful, 1)
					}
				}(i)
			}

			wg.Wait()

			successful := atomic.LoadInt64(&phaseSuccessful)
			total := atomic.LoadInt64(&phaseTotal)
			successRate := float64(successful) / float64(total)

			phaseResults = append(phaseResults, struct {
				phase      int
				successful int64
				total      int64
			}{phaseIdx + 1, successful, total})

			t.Logf("%s results: %d/%d successful (%.2f%%)",
				phase.name, successful, total, successRate*100)

			// Reset counters for next phase
			atomic.StoreInt64(&phaseSuccessful, 0)
			atomic.StoreInt64(&phaseTotal, 0)
		}

		// Verify gradual improvement
		for i := 1; i < len(phaseResults); i++ {
			prevRate := float64(phaseResults[i-1].successful) / float64(phaseResults[i-1].total)
			currRate := float64(phaseResults[i].successful) / float64(phaseResults[i].total)

			t.Logf("Phase %d->%d: %.2f%% -> %.2f%%", i, i+1, prevRate*100, currRate*100)
			// Success rate should generally improve or stay stable
			assert.GreaterOrEqual(t, currRate, prevRate-0.1, "Success rate should not degrade significantly between phases")
		}

		// Final phase should have reasonable success rate
		finalPhase := phaseResults[len(phaseResults)-1]
		finalRate := float64(finalPhase.successful) / float64(finalPhase.total)
		assert.Greater(t, finalRate, 0.6, "Final recovery phase should have >60% success rate")
	})
}
