package bitswap_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bitswap "github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/boxo/bitswap/circuitbreaker"
	tn "github.com/ipfs/boxo/bitswap/testnet"
	blocks "github.com/ipfs/go-block-format"
	delay "github.com/ipfs/go-ipfs-delay"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNetworkFailureSimulation tests the system's behavior under various network failure scenarios
func TestNetworkFailureSimulation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a test network with controlled latency
	net := tn.VirtualNetwork(delay.Fixed(50 * time.Millisecond))

	// Create multiple test instances
	instances := make([]bitswap.Instance, 5)
	for i := 0; i < 5; i++ {
		instances[i] = bitswap.NewTestInstance(ctx, net, "fault-test")
	}

	// Connect all instances in a mesh
	connectInstances(t, instances)

	// Create test blocks
	testBlocks := generateTestBlocks(10)

	// Add blocks to first instance
	for _, block := range testBlocks {
		err := instances[0].Blockstore().Put(ctx, block)
		require.NoError(t, err)
	}

	t.Run("PartialNetworkPartition", func(t *testing.T) {
		// Simulate network partition by disconnecting some nodes
		err := instances[0].Exchange.(*bitswap.Bitswap).Network().DisconnectFrom(ctx, instances[2].Peer)
		require.NoError(t, err)
		err = instances[1].Exchange.(*bitswap.Bitswap).Network().DisconnectFrom(ctx, instances[3].Peer)
		require.NoError(t, err)

		// Try to fetch blocks from partitioned network
		fetchCtx, fetchCancel := context.WithTimeout(ctx, 5*time.Second)
		defer fetchCancel()

		var successCount int32
		var wg sync.WaitGroup

		for i := 1; i < 5; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				for _, block := range testBlocks[:3] {
					_, err := instances[idx].Exchange.GetBlock(fetchCtx, block.Cid())
					if err == nil {
						atomic.AddInt32(&successCount, 1)
					}
				}
			}(i)
		}

		wg.Wait()

		// Some requests should succeed despite partition
		assert.Greater(t, int(successCount), 0, "Some blocks should be retrievable despite network partition")

		// Reconnect nodes
		err = instances[0].Exchange.(*bitswap.Bitswap).Network().Connect(ctx, peer.AddrInfo{ID: instances[2].Peer})
		require.NoError(t, err)
		err = instances[1].Exchange.(*bitswap.Bitswap).Network().Connect(ctx, peer.AddrInfo{ID: instances[3].Peer})
		require.NoError(t, err)
	})

	t.Run("NodeFailureRecovery", func(t *testing.T) {
		// Simulate node failure by stopping an instance
		instances[2].Exchange.Close()

		// Wait for failure detection
		time.Sleep(1 * time.Second)

		// Other nodes should continue working
		fetchCtx, fetchCancel := context.WithTimeout(ctx, 3*time.Second)
		defer fetchCancel()

		_, err := instances[1].Exchange.GetBlock(fetchCtx, testBlocks[0].Cid())
		assert.NoError(t, err, "Other nodes should continue working after node failure")

		// Restart the failed node
		instances[2] = bitswap.NewTestInstance(ctx, net, "fault-test-recovery")
		err = instances[2].Exchange.(*bitswap.Bitswap).Network().Connect(ctx, peer.AddrInfo{ID: instances[0].Peer})
		require.NoError(t, err)
		err = instances[2].Exchange.(*bitswap.Bitswap).Network().Connect(ctx, peer.AddrInfo{ID: instances[1].Peer})
		require.NoError(t, err)

		// Recovered node should be able to fetch blocks
		_, err = instances[2].Exchange.GetBlock(ctx, testBlocks[0].Cid())
		assert.NoError(t, err, "Recovered node should be able to fetch blocks")
	})
}

// TestCircuitBreakerFaultTolerance tests circuit breaker behavior under various failure conditions
func TestCircuitBreakerFaultTolerance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	config := circuitbreaker.Config{
		MaxRequests: 3,
		Interval:    1 * time.Second,
		Timeout:     2 * time.Second,
		ReadyToTrip: func(counts circuitbreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
		OnStateChange: func(name string, from circuitbreaker.State, to circuitbreaker.State) {
			t.Logf("Circuit breaker %s: %s -> %s", name, from, to)
		},
	}

	bcb := circuitbreaker.NewBitswapCircuitBreaker(config)
	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	t.Run("CircuitBreakerTripsOnConsecutiveFailures", func(t *testing.T) {
		testErr := errors.New("simulated network error")

		// Generate consecutive failures
		for i := 0; i < 3; i++ {
			err := bcb.ExecuteWithPeer(peerID, func() error {
				return testErr
			})
			assert.Equal(t, testErr, err)
		}

		// Circuit breaker should be open
		assert.Equal(t, circuitbreaker.StateOpen, bcb.GetPeerState(peerID))

		// Subsequent requests should be rejected
		err = bcb.ExecuteWithPeer(peerID, func() error {
			return nil
		})
		assert.Equal(t, circuitbreaker.ErrOpenState, err)
	})

	t.Run("CircuitBreakerRecoveryAfterTimeout", func(t *testing.T) {
		// Wait for circuit breaker timeout
		time.Sleep(3 * time.Second)

		// Should transition to half-open
		assert.Equal(t, circuitbreaker.StateHalfOpen, bcb.GetPeerState(peerID))

		// Successful requests should close the circuit
		for i := 0; i < 3; i++ {
			err := bcb.ExecuteWithPeer(peerID, func() error {
				return nil
			})
			assert.NoError(t, err)
		}

		// Circuit should be closed now
		assert.Equal(t, circuitbreaker.StateClosed, bcb.GetPeerState(peerID))
	})
}

// TestCascadingFailureProtection tests protection against cascading failures
func TestCascadingFailureProtection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a larger network to test cascading failures
	net := tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond))
	instances := make([]bitswap.Instance, 10)
	for i := 0; i < 10; i++ {
		instances[i] = bitswap.NewTestInstance(ctx, net, "cascade-test")
	}

	// Connect instances in a chain topology to simulate cascading failures
	for i := 0; i < 9; i++ {
		err := instances[i].Exchange.(*bitswap.Bitswap).Network().Connect(ctx, peer.AddrInfo{ID: instances[i+1].Peer})
		require.NoError(t, err)
	}

	// Create test blocks
	testBlocks := generateTestBlocks(20)

	// Distribute blocks across first few instances
	for i, block := range testBlocks {
		err := instances[i%3].Blockstore().Put(ctx, block)
		require.NoError(t, err)
	}

	t.Run("IsolateFailingNodes", func(t *testing.T) {
		// Simulate multiple node failures
		failedNodes := []int{1, 3, 5}
		for _, nodeIdx := range failedNodes {
			instances[nodeIdx].Exchange.Close()
		}

		// Wait for failure detection
		time.Sleep(2 * time.Second)

		// Remaining nodes should still be able to fetch blocks
		var successCount int32
		var wg sync.WaitGroup

		for i := 0; i < 10; i++ {
			if contains(failedNodes, i) {
				continue // Skip failed nodes
			}

			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				fetchCtx, fetchCancel := context.WithTimeout(ctx, 5*time.Second)
				defer fetchCancel()

				for _, block := range testBlocks[:5] {
					_, err := instances[idx].Exchange.GetBlock(fetchCtx, block.Cid())
					if err == nil {
						atomic.AddInt32(&successCount, 1)
					}
				}
			}(i)
		}

		wg.Wait()

		// Some blocks should still be retrievable
		assert.Greater(t, int(successCount), 0, "Some blocks should be retrievable despite cascading failures")
	})
}

// TestRecoveryAfterFailures tests various recovery scenarios
func TestRecoveryAfterFailures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	net := tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond))
	instances := make([]bitswap.Instance, 6)
	for i := 0; i < 6; i++ {
		instances[i] = bitswap.NewTestInstance(ctx, net, "recovery-test")
	}

	connectInstances(t, instances)
	testBlocks := generateTestBlocks(15)

	// Distribute blocks
	for i, block := range testBlocks {
		err := instances[i%3].Blockstore().Put(ctx, block)
		require.NoError(t, err)
	}

	t.Run("GradualRecoveryAfterMassFailure", func(t *testing.T) {
		// Simulate mass failure
		failedInstances := []int{1, 2, 3, 4}
		for _, idx := range failedInstances {
			instances[idx].Exchange.Close()
		}

		// Only instance 0 and 5 should be working
		time.Sleep(1 * time.Second)

		// Gradually recover instances
		for i, idx := range failedInstances {
			// Wait between recoveries to simulate gradual recovery
			time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)

			// Recover instance
			instances[idx] = bitswap.NewTestInstance(ctx, net, "recovery-test")

			// Reconnect to working instances
			for _, workingIdx := range []int{0, 5} {
				err := instances[idx].Exchange.(*bitswap.Bitswap).Network().Connect(ctx, peer.AddrInfo{ID: instances[workingIdx].Peer})
				require.NoError(t, err)
			}

			// Test that recovered instance can fetch blocks
			fetchCtx, fetchCancel := context.WithTimeout(ctx, 3*time.Second)
			_, err := instances[idx].Exchange.GetBlock(fetchCtx, testBlocks[0].Cid())
			fetchCancel()
			assert.NoError(t, err, "Recovered instance %d should be able to fetch blocks", idx)
		}

		// All instances should be working now
		var wg sync.WaitGroup
		var totalSuccess int32

		for i := 0; i < len(instances); i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				fetchCtx, fetchCancel := context.WithTimeout(ctx, 3*time.Second)
				defer fetchCancel()
				_, err := instances[idx].Exchange.GetBlock(fetchCtx, testBlocks[1].Cid())
				if err == nil {
					atomic.AddInt32(&totalSuccess, 1)
				}
			}(i)
		}

		wg.Wait()
		assert.Equal(t, int32(len(instances)), totalSuccess, "All instances should be working after recovery")
	})
}

// Helper functions

func generateTestBlocks(count int) []blocks.Block {
	testBlocks := make([]blocks.Block, count)
	for i := 0; i < count; i++ {
		data := make([]byte, 256)
		rand.Read(data)
		testBlocks[i] = blocks.NewBlock(data)
	}
	return testBlocks
}

func connectInstances(t *testing.T, instances []bitswap.Instance) {
	for i := 0; i < len(instances); i++ {
		for j := i + 1; j < len(instances); j++ {
			err := instances[i].Exchange.(*bitswap.Bitswap).Network().Connect(context.Background(), peer.AddrInfo{ID: instances[j].Peer})
			require.NoError(t, err)
		}
	}
}

func contains(slice []int, item int) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// TestChaosEngineeringScenarios implements chaos engineering tests for system stability
func TestChaosEngineeringScenarios(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Run("RandomNodeKilling", func(t *testing.T) {
		// Create a larger network for chaos testing
		net := tn.VirtualNetwork(delay.Fixed(20 * time.Millisecond))
		instances := make([]bitswap.Instance, 15)
		for i := 0; i < 15; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "chaos-kill-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(50)

		// Distribute blocks across multiple nodes
		for i, block := range testBlocks {
			err := instances[i%5].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		// Start chaos monkey - randomly kill nodes
		var killedNodes []int
		var mu sync.Mutex
		chaosCtx, chaosCancel := context.WithTimeout(ctx, 30*time.Second)
		defer chaosCancel()

		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-chaosCtx.Done():
					return
				case <-ticker.C:
					mu.Lock()
					// Kill a random node that's not already killed
					availableNodes := make([]int, 0)
					for i := 0; i < len(instances); i++ {
						if !contains(killedNodes, i) {
							availableNodes = append(availableNodes, i)
						}
					}

					if len(availableNodes) > 5 { // Keep at least 5 nodes alive
						nodeToKill := availableNodes[rand.Intn(len(availableNodes))]
						instances[nodeToKill].Exchange.Close()
						killedNodes = append(killedNodes, nodeToKill)
						t.Logf("Chaos: Killed node %d", nodeToKill)
					}
					mu.Unlock()
				}
			}
		}()

		// Continuously try to fetch blocks while chaos is happening
		var successCount, failureCount int64
		var wg sync.WaitGroup

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond) // Random start time

				mu.Lock()
				aliveNodes := make([]int, 0)
				for j := 0; j < len(instances); j++ {
					if !contains(killedNodes, j) {
						aliveNodes = append(aliveNodes, j)
					}
				}
				mu.Unlock()

				if len(aliveNodes) == 0 {
					return
				}

				nodeIdx := aliveNodes[rand.Intn(len(aliveNodes))]
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(ctx, 3*time.Second)
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

		t.Logf("Chaos test results: %d successes, %d failures, %d nodes killed",
			successCount, failureCount, len(killedNodes))

		// System should maintain some level of functionality despite chaos
		assert.Greater(t, successCount, int64(10), "System should maintain some functionality during chaos")
		assert.Greater(t, len(killedNodes), 2, "Chaos should have killed some nodes")
	})

	t.Run("NetworkPartitionChaos", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond))
		instances := make([]bitswap.Instance, 12)
		for i := 0; i < 12; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "chaos-partition-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(30)

		// Distribute blocks
		for i, block := range testBlocks {
			err := instances[i%4].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		// Create random network partitions
		chaosCtx, chaosCancel := context.WithTimeout(ctx, 25*time.Second)
		defer chaosCancel()

		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-chaosCtx.Done():
					return
				case <-ticker.C:
					// Create random partitions
					partitionSize := 2 + rand.Intn(3) // 2-4 nodes per partition
					for i := 0; i < partitionSize; i++ {
						node1 := rand.Intn(len(instances))
						node2 := rand.Intn(len(instances))
						if node1 != node2 {
							instances[node1].Exchange.(*bitswap.Bitswap).Network().DisconnectFrom(ctx, instances[node2].Peer)
							t.Logf("Chaos: Partitioned nodes %d and %d", node1, node2)
						}
					}

					// Randomly reconnect some nodes
					if rand.Float32() < 0.3 {
						node1 := rand.Intn(len(instances))
						node2 := rand.Intn(len(instances))
						if node1 != node2 {
							instances[node1].Exchange.(*bitswap.Bitswap).Network().Connect(ctx, peer.AddrInfo{ID: instances[node2].Peer})
							t.Logf("Chaos: Reconnected nodes %d and %d", node1, node2)
						}
					}
				}
			}
		}()

		// Test system resilience during network chaos
		var successCount, failureCount int64
		var wg sync.WaitGroup

		for i := 0; i < 80; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)

				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(ctx, 4*time.Second)
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

		t.Logf("Network partition chaos results: %d successes, %d failures", successCount, failureCount)
		assert.Greater(t, successCount, int64(15), "System should handle network partitions gracefully")
	})

	t.Run("ResourceExhaustionChaos", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(5 * time.Millisecond))
		instances := make([]bitswap.Instance, 8)
		for i := 0; i < 8; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "chaos-resource-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(100)

		// Add blocks to first few instances
		for i, block := range testBlocks {
			err := instances[i%3].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		// Simulate resource exhaustion by overwhelming the system
		var wg sync.WaitGroup
		var successCount, failureCount int64

		// Create high load to exhaust resources
		for i := 0; i < 200; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()

				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				// Use very short timeout to simulate resource pressure
				fetchCtx, fetchCancel := context.WithTimeout(ctx, 500*time.Millisecond)
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

		t.Logf("Resource exhaustion chaos results: %d successes, %d failures", successCount, failureCount)
		// System should handle resource pressure without complete failure
		assert.Greater(t, successCount, int64(20), "System should handle some requests under resource pressure")
	})
}

// TestAdvancedNetworkFailureScenarios tests complex network failure patterns
func TestAdvancedNetworkFailureScenarios(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	t.Run("SplitBrainScenario", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(30 * time.Millisecond))
		instances := make([]bitswap.Instance, 10)
		for i := 0; i < 10; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "split-brain-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(20)

		// Distribute blocks across all instances
		for i, block := range testBlocks {
			err := instances[i%len(instances)].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		// Create split-brain scenario: divide network into two partitions
		partition1 := instances[:5]
		partition2 := instances[5:]

		// Disconnect partitions from each other
		for _, inst1 := range partition1 {
			for _, inst2 := range partition2 {
				err := inst1.Exchange.(*bitswap.Bitswap).Network().DisconnectFrom(ctx, inst2.Peer)
				require.NoError(t, err)
			}
		}

		// Each partition should still function internally
		var wg sync.WaitGroup
		var partition1Success, partition2Success int64

		// Test partition 1
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				nodeIdx := rand.Intn(len(partition1))
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(ctx, 3*time.Second)
				defer fetchCancel()

				_, err := partition1[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&partition1Success, 1)
				}
			}(i)
		}

		// Test partition 2
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				nodeIdx := rand.Intn(len(partition2))
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(ctx, 3*time.Second)
				defer fetchCancel()

				_, err := partition2[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&partition2Success, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Split-brain results: Partition1=%d successes, Partition2=%d successes",
			partition1Success, partition2Success)

		// Both partitions should have some successful requests
		assert.Greater(t, partition1Success, int64(2), "Partition 1 should function independently")
		assert.Greater(t, partition2Success, int64(2), "Partition 2 should function independently")

		// Heal the split-brain by reconnecting partitions
		for _, inst1 := range partition1 {
			for _, inst2 := range partition2 {
				err := inst1.Exchange.(*bitswap.Bitswap).Network().Connect(ctx, peer.AddrInfo{ID: inst2.Peer})
				require.NoError(t, err)
			}
		}

		// Wait for network to stabilize
		time.Sleep(2 * time.Second)

		// Test that the healed network functions properly
		var healedSuccess int64
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(ctx, 3*time.Second)
				defer fetchCancel()

				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&healedSuccess, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Healed network results: %d successes", healedSuccess)
		assert.Greater(t, healedSuccess, int64(15), "Healed network should function well")
	})

	t.Run("FlappingNetworkConnections", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(20 * time.Millisecond))
		instances := make([]bitswap.Instance, 6)
		for i := 0; i < 6; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "flapping-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(15)

		// Add blocks to first instance
		for _, block := range testBlocks {
			err := instances[0].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		// Simulate flapping connections
		flappingCtx, flappingCancel := context.WithTimeout(ctx, 20*time.Second)
		defer flappingCancel()

		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			connected := true
			for {
				select {
				case <-flappingCtx.Done():
					return
				case <-ticker.C:
					if connected {
						// Disconnect some nodes
						for i := 1; i < 4; i++ {
							instances[0].Exchange.(*bitswap.Bitswap).Network().DisconnectFrom(ctx, instances[i].Peer)
						}
						connected = false
						t.Log("Flapping: Disconnected nodes")
					} else {
						// Reconnect nodes
						for i := 1; i < 4; i++ {
							instances[0].Exchange.(*bitswap.Bitswap).Network().Connect(ctx, peer.AddrInfo{ID: instances[i].Peer})
						}
						connected = true
						t.Log("Flapping: Reconnected nodes")
					}
				}
			}
		}()

		// Test system behavior during flapping connections
		var successCount, failureCount int64
		var wg sync.WaitGroup

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

				nodeIdx := 1 + rand.Intn(4) // Use nodes 1-4 (flapping nodes)
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(ctx, 2*time.Second)
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

		t.Logf("Flapping connections results: %d successes, %d failures", successCount, failureCount)
		assert.Greater(t, successCount, int64(10), "System should handle flapping connections")
	})

	t.Run("HighLatencyNetworkConditions", func(t *testing.T) {
		// Create network with high and variable latency
		net := tn.VirtualNetwork(delay.Fixed(500 * time.Millisecond)) // High base latency
		instances := make([]bitswap.Instance, 8)
		for i := 0; i < 8; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "high-latency-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(20)

		// Distribute blocks
		for i, block := range testBlocks {
			err := instances[i%3].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		// Test system behavior under high latency
		var successCount, timeoutCount int64
		var wg sync.WaitGroup

		for i := 0; i < 30; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()

				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				// Use longer timeout to account for high latency
				fetchCtx, fetchCancel := context.WithTimeout(ctx, 2*time.Second)
				defer fetchCancel()

				start := time.Now()
				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				duration := time.Since(start)

				if err == nil {
					atomic.AddInt64(&successCount, 1)
					t.Logf("Request %d completed in %v", requestID, duration)
				} else if errors.Is(err, context.DeadlineExceeded) {
					atomic.AddInt64(&timeoutCount, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("High latency results: %d successes, %d timeouts", successCount, timeoutCount)
		// System should handle high latency conditions
		assert.Greater(t, successCount, int64(5), "System should handle high latency")
	})
}

// TestCircuitBreakerAdvancedScenarios tests advanced circuit breaker scenarios
func TestCircuitBreakerAdvancedScenarios(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("CircuitBreakerUnderChaos", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 5,
			Interval:    1 * time.Second,
			Timeout:     2 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
		}

		bcb := circuitbreaker.NewBitswapCircuitBreaker(config)

		// Create test peers
		peers := make([]peer.ID, 10)
		for i := 0; i < 10; i++ {
			peerID, err := peer.Decode(fmt.Sprintf("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXt%02d", i))
			require.NoError(t, err)
			peers[i] = peerID
		}

		// Chaos scenario: randomly fail peers and recover them
		chaosCtx, chaosCancel := context.WithTimeout(ctx, 15*time.Second)
		defer chaosCancel()

		var failingPeers sync.Map
		var wg sync.WaitGroup

		// Chaos goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-chaosCtx.Done():
					return
				case <-ticker.C:
					// Randomly mark peers as failing or recovering
					for _, peerID := range peers {
						if rand.Float32() < 0.3 { // 30% chance to change state
							if _, failing := failingPeers.Load(peerID); failing {
								failingPeers.Delete(peerID)
								t.Logf("Chaos: Peer %s recovered", peerID.String()[:8])
							} else {
								failingPeers.Store(peerID, true)
								t.Logf("Chaos: Peer %s started failing", peerID.String()[:8])
							}
						}
					}
				}
			}
		}()

		// Request goroutines
		var successCount, failureCount, rejectedCount int64

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

				peerID := peers[rand.Intn(len(peers))]
				_, failing := failingPeers.Load(peerID)

				err := bcb.ExecuteWithPeer(peerID, func() error {
					if failing {
						return errors.New("peer is failing")
					}
					return nil
				})

				if err == nil {
					atomic.AddInt64(&successCount, 1)
				} else if err == circuitbreaker.ErrOpenState || err == circuitbreaker.ErrTooManyRequests {
					atomic.AddInt64(&rejectedCount, 1)
				} else {
					atomic.AddInt64(&failureCount, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Circuit breaker chaos results: %d successes, %d failures, %d rejected",
			successCount, failureCount, rejectedCount)

		assert.Greater(t, successCount, int64(20), "Should have successful requests")
		assert.Greater(t, rejectedCount, int64(5), "Circuit breaker should reject some requests")
	})

	t.Run("CircuitBreakerRecoveryUnderLoad", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 3,
			Interval:    500 * time.Millisecond,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
		}

		bcb := circuitbreaker.NewBitswapCircuitBreaker(config)
		peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
		require.NoError(t, err)

		// Trip the circuit breaker
		testError := errors.New("initial failure")
		for i := 0; i < 2; i++ {
			bcb.ExecuteWithPeer(peerID, func() error { return testError })
		}
		assert.Equal(t, circuitbreaker.StateOpen, bcb.GetPeerState(peerID))

		// Wait for half-open
		time.Sleep(1200 * time.Millisecond)
		assert.Equal(t, circuitbreaker.StateHalfOpen, bcb.GetPeerState(peerID))

		// Test recovery under concurrent load
		var wg sync.WaitGroup
		var successCount, rejectedCount int64

		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()

				err := bcb.ExecuteWithPeer(peerID, func() error {
					// Simulate gradual recovery - first few requests succeed
					if requestID < 5 {
						return nil
					}
					// Later requests have some failures
					if rand.Float32() < 0.2 {
						return errors.New("occasional failure")
					}
					return nil
				})

				if err == nil {
					atomic.AddInt64(&successCount, 1)
				} else if err == circuitbreaker.ErrOpenState || err == circuitbreaker.ErrTooManyRequests {
					atomic.AddInt64(&rejectedCount, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Recovery under load results: %d successes, %d rejected", successCount, rejectedCount)
		assert.Greater(t, successCount, int64(5), "Should have successful requests during recovery")
	})
}

// TestSystemStabilityUnderContinuousFailures tests long-term system stability
func TestSystemStabilityUnderContinuousFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running stability test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Run("LongTermStabilityTest", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(10 * time.Millisecond))
		instances := make([]bitswap.Instance, 12)
		for i := 0; i < 12; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "stability-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(100)

		// Distribute blocks
		for i, block := range testBlocks {
			err := instances[i%4].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		// Run continuous chaos for extended period
		var totalRequests, successfulRequests, failedRequests int64
		var wg sync.WaitGroup

		// Chaos controller
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// Random chaos events
					chaosType := rand.Intn(4)
					switch chaosType {
					case 0: // Kill a node
						nodeIdx := rand.Intn(len(instances))
						instances[nodeIdx].Exchange.Close()
						instances[nodeIdx] = bitswap.NewTestInstance(ctx, net, "stability-test")
						t.Logf("Stability chaos: Restarted node %d", nodeIdx)

					case 1: // Network partition
						node1, node2 := rand.Intn(len(instances)), rand.Intn(len(instances))
						if node1 != node2 {
							instances[node1].Exchange.(*bitswap.Bitswap).Network().DisconnectFrom(ctx, instances[node2].Peer)
						}

					case 2: // Network healing
						node1, node2 := rand.Intn(len(instances)), rand.Intn(len(instances))
						if node1 != node2 {
							instances[node1].Exchange.(*bitswap.Bitswap).Network().Connect(ctx, peer.AddrInfo{ID: instances[node2].Peer})
						}

					case 3: // High load burst
						for i := 0; i < 10; i++ {
							wg.Add(1)
							go func() {
								defer wg.Done()
								nodeIdx := rand.Intn(len(instances))
								blockIdx := rand.Intn(len(testBlocks))
								fetchCtx, fetchCancel := context.WithTimeout(ctx, 1*time.Second)
								defer fetchCancel()
								instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
							}()
						}
					}
				}
			}
		}()

		// Continuous request load
		requestTicker := time.NewTicker(100 * time.Millisecond)
		defer requestTicker.Stop()

		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-requestTicker.C:
					wg.Add(1)
					go func() {
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
							atomic.AddInt64(&failedRequests, 1)
						}
					}()
				}
			}
		}()

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		failed := atomic.LoadInt64(&failedRequests)

		t.Logf("Long-term stability results:")
		t.Logf("  Total requests: %d", total)
		t.Logf("  Successful: %d (%.2f%%)", successful, float64(successful)/float64(total)*100)
		t.Logf("  Failed: %d (%.2f%%)", failed, float64(failed)/float64(total)*100)

		// System should maintain reasonable success rate despite continuous chaos
		successRate := float64(successful) / float64(total)
		assert.Greater(t, successRate, 0.3, "System should maintain >30% success rate under continuous chaos")
		assert.Greater(t, total, int64(100), "Should have processed substantial number of requests")
	})
}

// TestFailureRecoveryPatterns tests various failure recovery patterns
func TestFailureRecoveryPatterns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	t.Run("ExponentialBackoffRecovery", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(20 * time.Millisecond))
		instances := make([]bitswap.Instance, 6)
		for i := 0; i < 6; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "backoff-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(10)

		// Add blocks to first instance
		for _, block := range testBlocks {
			err := instances[0].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		// Simulate a failing node that recovers with exponential backoff
		failingNode := 1
		var failureCount int64
		var recoveryAttempts []time.Time

		// Kill the node initially
		instances[failingNode].Exchange.Close()

		// Attempt recovery with exponential backoff
		backoffDuration := 1 * time.Second
		maxBackoff := 16 * time.Second

		for backoffDuration <= maxBackoff {
			time.Sleep(backoffDuration)
			recoveryAttempts = append(recoveryAttempts, time.Now())

			// Attempt to restart the node
			instances[failingNode] = bitswap.NewTestInstance(ctx, net, "backoff-test")
			err := instances[failingNode].Exchange.(*bitswap.Bitswap).Network().Connect(ctx, peer.AddrInfo{ID: instances[0].Peer})
			require.NoError(t, err)

			// Test if recovery was successful
			fetchCtx, fetchCancel := context.WithTimeout(ctx, 2*time.Second)
			_, err = instances[failingNode].Exchange.GetBlock(fetchCtx, testBlocks[0].Cid())
			fetchCancel()

			if err == nil {
				t.Logf("Node recovered after %d attempts with backoff %v", len(recoveryAttempts), backoffDuration)
				break
			} else {
				atomic.AddInt64(&failureCount, 1)
				instances[failingNode].Exchange.Close()
				backoffDuration *= 2 // Exponential backoff
			}
		}

		assert.Greater(t, len(recoveryAttempts), 1, "Should have made multiple recovery attempts")
		assert.Less(t, len(recoveryAttempts), 6, "Should not require too many attempts")
	})

	t.Run("GracefulDegradationRecovery", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(15 * time.Millisecond))
		instances := make([]bitswap.Instance, 9)
		for i := 0; i < 9; i++ {
			instances[i] = bitswap.NewTestInstance(ctx, net, "degradation-test")
		}

		connectInstances(t, instances)
		testBlocks := generateTestBlocks(30)

		// Distribute blocks across multiple instances
		for i, block := range testBlocks {
			err := instances[i%3].Blockstore().Put(ctx, block)
			require.NoError(t, err)
		}

		// Gradually fail nodes and measure system degradation
		var baselineSuccess int64

		// Measure baseline performance
		for i := 0; i < 20; i++ {
			nodeIdx := rand.Intn(len(instances))
			blockIdx := rand.Intn(len(testBlocks))

			fetchCtx, fetchCancel := context.WithTimeout(ctx, 1*time.Second)
			_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
			fetchCancel()

			if err == nil {
				atomic.AddInt64(&baselineSuccess, 1)
			}
		}

		t.Logf("Baseline success rate: %d/20", baselineSuccess)

		// Gradually fail nodes and test degradation
		failedNodes := 0
		for failedNodes < 6 {
			// Fail another node
			instances[failedNodes].Exchange.Close()
			failedNodes++

			// Test current performance
			var currentSuccess int64
			for i := 0; i < 20; i++ {
				// Only use non-failed nodes
				nodeIdx := failedNodes + rand.Intn(len(instances)-failedNodes)
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(ctx, 2*time.Second)
				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				fetchCancel()

				if err == nil {
					atomic.AddInt64(&currentSuccess, 1)
				}
			}

			degradationRatio := float64(currentSuccess) / float64(baselineSuccess)
			t.Logf("With %d failed nodes: %d/20 success (%.2f%% of baseline)",
				failedNodes, currentSuccess, degradationRatio*100)

			// System should degrade gracefully, not fail completely
			assert.Greater(t, currentSuccess, int64(2), "System should maintain some functionality")

			time.Sleep(1 * time.Second) // Allow system to adapt
		}
	})
}
