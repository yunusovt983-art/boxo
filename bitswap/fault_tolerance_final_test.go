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

// TestFaultToleranceFinal implements comprehensive fault tolerance tests
func TestFaultToleranceFinal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Run("NetworkFailureSimulation", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(20 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(6)
		testBlocks := createFinalTestBlocks(t, 15)

		// Distribute blocks across first 3 nodes
		for i, block := range testBlocks {
			nodeIdx := i % 3
			err := instances[nodeIdx].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests int64

		// Request load from last 3 nodes
		requestCtx, requestCancel := context.WithTimeout(ctx, 60*time.Second)
		defer requestCancel()

		for i := 0; i < 60; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

				atomic.AddInt64(&totalRequests, 1)
				requesterIdx := 3 + (requestID % 3)
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(requestCtx, 3*time.Second)
				defer fetchCancel()

				_, err := instances[requesterIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				}
			}(i)
		}

		// Simulate node failure and recovery
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Wait then fail a node
			time.Sleep(10 * time.Second)
			failedNodeIdx := 1
			instances[failedNodeIdx].Exchange.Close()
			t.Logf("Failed node %d", failedNodeIdx)

			// Wait then recover
			time.Sleep(15 * time.Second)
			newInstance := ig.Next()
			instances[failedNodeIdx] = newInstance

			// Restore blocks
			for i, block := range testBlocks {
				if i%3 == failedNodeIdx {
					instances[failedNodeIdx].Blockstore.Put(ctx, block)
				}
			}
			t.Logf("Recovered node %d", failedNodeIdx)
		}()

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		successRate := float64(successful) / float64(total)

		t.Logf("Network failure simulation: %d/%d successful (%.2f%%)", successful, total, successRate*100)
		assert.Greater(t, successRate, 0.4, "System should maintain >40% success rate during failures")
	})

	t.Run("NetworkPartitionRecovery", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(15 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(8)
		testBlocks := createFinalTestBlocks(t, 12)

		// Distribute blocks in first half
		for i, block := range testBlocks {
			nodeIdx := i % 4
			err := instances[nodeIdx].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests int64

		// Requests from second half
		requestCtx, requestCancel := context.WithTimeout(ctx, 45*time.Second)
		defer requestCancel()

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1500)) * time.Millisecond)

				atomic.AddInt64(&totalRequests, 1)
				requesterIdx := 4 + (requestID % 4)
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(requestCtx, 4*time.Second)
				defer fetchCancel()

				_, err := instances[requesterIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				}
			}(i)
		}

		// Partition and heal
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Create partition after 8 seconds
			time.Sleep(8 * time.Second)
			for i := 0; i < 4; i++ {
				for j := 4; j < 8; j++ {
					instances[i].Adapter.DisconnectFrom(ctx, instances[j].Identity.ID())
				}
			}
			t.Log("Created network partition")

			// Heal partition after 15 seconds
			time.Sleep(15 * time.Second)
			for i := 0; i < 4; i++ {
				for j := 4; j < 8; j++ {
					instances[i].Adapter.Connect(ctx, peer.AddrInfo{ID: instances[j].Identity.ID()})
				}
			}
			t.Log("Healed network partition")
		}()

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		successRate := float64(successful) / float64(total)

		t.Logf("Network partition recovery: %d/%d successful (%.2f%%)", successful, total, successRate*100)
		assert.Greater(t, successRate, 0.25, "System should maintain >25% success rate during partitions")
	})
}

// TestCircuitBreakerFaultToleranceFinal tests circuit breaker fault tolerance
func TestCircuitBreakerFaultToleranceFinal(t *testing.T) {
	t.Run("HighConcurrencyCircuitBreaker", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 6,
			Interval:    2 * time.Second,
			Timeout:     3 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("concurrency-test", config)

		var wg sync.WaitGroup
		var totalRequests, successfulRequests, rejectedRequests int64

		// High concurrency test
		numWorkers := 40
		requestsPerWorker := 10

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < requestsPerWorker; j++ {
					atomic.AddInt64(&totalRequests, 1)

					err := cb.Execute(func() error {
						// Variable failure rate
						failureRate := 0.3 + 0.4*float64(workerID%3)/3.0
						if rand.Float64() < failureRate {
							return errors.New("worker failure")
						}
						time.Sleep(time.Duration(rand.Intn(15)) * time.Millisecond)
						return nil
					})

					switch err {
					case nil:
						atomic.AddInt64(&successfulRequests, 1)
					case circuitbreaker.ErrOpenState, circuitbreaker.ErrTooManyRequests:
						atomic.AddInt64(&rejectedRequests, 1)
					}

					time.Sleep(time.Duration(rand.Intn(80)) * time.Millisecond)
				}
			}(i)
		}

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		rejected := atomic.LoadInt64(&rejectedRequests)

		t.Logf("High concurrency circuit breaker:")
		t.Logf("  Total: %d, Successful: %d, Rejected: %d", total, successful, rejected)

		expectedTotal := int64(numWorkers * requestsPerWorker)
		assert.Equal(t, expectedTotal, total, "All requests should be accounted for")
		assert.Greater(t, rejected, int64(30), "Circuit breaker should reject requests")
		assert.Greater(t, successful, int64(80), "Should have successful requests")
	})

	t.Run("PeerIsolationCircuitBreaker", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 4,
			Interval:    1 * time.Second,
			Timeout:     2 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
		}

		bcb := circuitbreaker.NewBitswapCircuitBreaker(config)

		// Create test peers
		numPeers := 6
		peers := make([]peer.ID, numPeers)
		for i := 0; i < numPeers; i++ {
			peerID, err := peer.Decode(fmt.Sprintf("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXt%02d", i))
			require.NoError(t, err)
			peers[i] = peerID
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests, rejectedRequests int64

		// Generate load with different failure rates per peer
		for i := 0; i < 120; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()

				peerIdx := requestID % numPeers
				peerID := peers[peerIdx]

				atomic.AddInt64(&totalRequests, 1)

				err := bcb.ExecuteWithPeer(peerID, func() error {
					// Different failure rates for different peers
					var failureRate float64
					switch peerIdx % 3 {
					case 0: // Healthy peers
						failureRate = 0.2
					case 1: // Moderately unhealthy peers
						failureRate = 0.5
					case 2: // Very unhealthy peers
						failureRate = 0.8
					}

					if rand.Float64() < failureRate {
						return errors.New("peer failure")
					}

					time.Sleep(time.Duration(rand.Intn(25)) * time.Millisecond)
					return nil
				})

				switch err {
				case nil:
					atomic.AddInt64(&successfulRequests, 1)
				case circuitbreaker.ErrOpenState, circuitbreaker.ErrTooManyRequests:
					atomic.AddInt64(&rejectedRequests, 1)
				}

				time.Sleep(time.Duration(rand.Intn(120)) * time.Millisecond)
			}(i)
		}

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		rejected := atomic.LoadInt64(&rejectedRequests)

		t.Logf("Peer isolation circuit breaker:")
		t.Logf("  Total: %d, Successful: %d, Rejected: %d", total, successful, rejected)

		// Check peer states
		healthyPeers := 0
		for i, peerID := range peers {
			state := bcb.GetPeerState(peerID)
			if state == circuitbreaker.StateClosed {
				healthyPeers++
			}
			t.Logf("  Peer %d state: %v", i, state)
		}

		assert.Equal(t, int64(120), total, "All requests should be processed")
		assert.Greater(t, successful, int64(20), "Should have successful requests from healthy peers")
		assert.Greater(t, rejected, int64(25), "Should have rejected requests from unhealthy peers")
	})

	t.Run("CircuitBreakerRecoveryPattern", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 3,
			Interval:    1500 * time.Millisecond,
			Timeout:     2000 * time.Millisecond,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("recovery-test", config)

		var wg sync.WaitGroup
		var phaseResults []struct {
			phase      string
			successful int64
			total      int64
		}

		// Phase 1: Trip circuit breaker
		t.Log("Phase 1: Tripping circuit breaker")
		var phase1Successful, phase1Total int64

		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				atomic.AddInt64(&phase1Total, 1)

				err := cb.Execute(func() error {
					return errors.New("phase 1 failure")
				})

				if err == nil {
					atomic.AddInt64(&phase1Successful, 1)
				}
			}()
		}

		wg.Wait()

		p1Total := atomic.LoadInt64(&phase1Total)
		p1Successful := atomic.LoadInt64(&phase1Successful)
		phaseResults = append(phaseResults, struct {
			phase      string
			successful int64
			total      int64
		}{"Phase 1 (Trip)", p1Successful, p1Total})

		// Wait for reset
		time.Sleep(2500 * time.Millisecond)

		// Phase 2: Recovery
		t.Log("Phase 2: Recovery")
		var phase2Successful, phase2Total int64

		for i := 0; i < 25; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(250)) * time.Millisecond)

				atomic.AddInt64(&phase2Total, 1)

				err := cb.Execute(func() error {
					// Decreasing failure rate
					failureRate := 0.5 - 0.3*float64(requestID)/25.0
					if rand.Float64() < failureRate {
						return errors.New("phase 2 failure")
					}
					return nil
				})

				if err == nil {
					atomic.AddInt64(&phase2Successful, 1)
				}
			}(i)
		}

		wg.Wait()

		p2Total := atomic.LoadInt64(&phase2Total) - p1Total
		p2Successful := atomic.LoadInt64(&phase2Successful) - p1Successful
		phaseResults = append(phaseResults, struct {
			phase      string
			successful int64
			total      int64
		}{"Phase 2 (Recovery)", p2Successful, p2Total})

		// Analyze recovery pattern
		t.Logf("Circuit breaker recovery pattern:")
		for _, phase := range phaseResults {
			if phase.total > 0 {
				successRate := float64(phase.successful) / float64(phase.total)
				t.Logf("  %s: %d/%d successful (%.2f%%)", phase.phase, phase.successful, phase.total, successRate*100)
			}
		}

		// Verify recovery
		if len(phaseResults) >= 2 {
			phase1Rate := float64(phaseResults[0].successful) / float64(phaseResults[0].total)
			phase2Rate := float64(phaseResults[1].successful) / float64(phaseResults[1].total)

			assert.Greater(t, phase2Rate, phase1Rate, "Recovery phase should show improvement")
			assert.Greater(t, phase2Rate, 0.4, "Recovery phase should have >40% success rate")
		}
	})
}

// TestChaosEngineeringFinal implements chaos engineering tests
func TestChaosEngineeringFinal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Run("RandomNodeChaos", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(25 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(8)
		testBlocks := createFinalTestBlocks(t, 16)

		// Distribute blocks with redundancy
		for i, block := range testBlocks {
			for j := 0; j < 2; j++ { // Each block on 2 nodes
				nodeIdx := (i*2 + j) % len(instances)
				err := instances[nodeIdx].Blockstore.Put(ctx, block)
				require.NoError(t, err)
			}
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests int64
		var chaosEvents int64

		// Request load
		requestCtx, requestCancel := context.WithTimeout(ctx, 70*time.Second)
		defer requestCancel()

		for i := 0; i < 80; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)

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

		// Chaos controller
		wg.Add(1)
		go func() {
			defer wg.Done()
			chaosCtx, chaosCancel := context.WithTimeout(ctx, 65*time.Second)
			defer chaosCancel()

			ticker := time.NewTicker(4 * time.Second)
			defer ticker.Stop()

			killedNodes := make(map[int]bool)

			for {
				select {
				case <-chaosCtx.Done():
					return
				case <-ticker.C:
					if rand.Float32() < 0.7 { // 70% chance of chaos event
						atomic.AddInt64(&chaosEvents, 1)

						if len(killedNodes) < 2 && rand.Float32() < 0.6 {
							// Kill a node
							availableNodes := make([]int, 0)
							for i := 0; i < len(instances); i++ {
								if !killedNodes[i] {
									availableNodes = append(availableNodes, i)
								}
							}

							if len(availableNodes) > 0 {
								nodeIdx := availableNodes[rand.Intn(len(availableNodes))]
								instances[nodeIdx].Exchange.Close()
								killedNodes[nodeIdx] = true
								t.Logf("Chaos: Killed node %d", nodeIdx)
							}
						} else if len(killedNodes) > 0 {
							// Restart a node
							for nodeIdx := range killedNodes {
								newInstance := ig.Next()
								instances[nodeIdx] = newInstance

								// Restore some blocks
								for i, block := range testBlocks {
									for j := 0; j < 2; j++ {
										if (i*2+j)%len(instances) == nodeIdx && rand.Float32() < 0.7 {
											instances[nodeIdx].Blockstore.Put(ctx, block)
										}
									}
								}

								delete(killedNodes, nodeIdx)
								t.Logf("Chaos: Restarted node %d", nodeIdx)
								break
							}
						}
					}
				}
			}
		}()

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		events := atomic.LoadInt64(&chaosEvents)
		successRate := float64(successful) / float64(total)

		t.Logf("Random node chaos: %d/%d successful (%.2f%%), %d chaos events",
			successful, total, successRate*100, events)

		assert.Greater(t, successRate, 0.3, "System should maintain >30% success rate under chaos")
		assert.Greater(t, events, int64(5), "Should have generated chaos events")
	})

	t.Run("HighLatencyWithFailures", func(t *testing.T) {
		// High latency network
		highLatencyNet := tn.VirtualNetwork(delay.Fixed(150 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(highLatencyNet, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(6)
		testBlocks := createFinalTestBlocks(t, 12)

		// Distribute blocks
		for i, block := range testBlocks {
			nodeIdx := i % 3
			err := instances[nodeIdx].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests, timeoutRequests int64

		// Request load with longer timeouts
		requestCtx, requestCancel := context.WithTimeout(ctx, 50*time.Second)
		defer requestCancel()

		for i := 0; i < 40; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1500)) * time.Millisecond)

				atomic.AddInt64(&totalRequests, 1)
				requesterIdx := 3 + (requestID % 3)
				blockIdx := rand.Intn(len(testBlocks))

				// Longer timeout for high latency
				timeout := time.Duration(800+rand.Intn(1200)) * time.Millisecond
				fetchCtx, fetchCancel := context.WithTimeout(requestCtx, timeout)
				defer fetchCancel()

				_, err := instances[requesterIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				} else if err == context.DeadlineExceeded {
					atomic.AddInt64(&timeoutRequests, 1)
				}
			}(i)
		}

		// Occasional failures
		wg.Add(1)
		go func() {
			defer wg.Done()

			time.Sleep(8 * time.Second)
			// Kill and restart a node
			nodeIdx := 1
			instances[nodeIdx].Exchange.Close()
			t.Logf("High latency chaos: Killed node %d", nodeIdx)

			time.Sleep(10 * time.Second)
			newInstance := ig.Next()
			instances[nodeIdx] = newInstance

			// Restore blocks
			for i, block := range testBlocks {
				if i%3 == nodeIdx {
					instances[nodeIdx].Blockstore.Put(ctx, block)
				}
			}
			t.Logf("High latency chaos: Restarted node %d", nodeIdx)
		}()

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		timeouts := atomic.LoadInt64(&timeoutRequests)
		successRate := float64(successful) / float64(total)

		t.Logf("High latency with failures: %d/%d successful (%.2f%%), %d timeouts",
			successful, total, successRate*100, timeouts)

		assert.Greater(t, successRate, 0.2, "System should handle high latency + failures with >20% success rate")
		assert.Greater(t, timeouts, int64(3), "Should have timeouts due to high latency")
	})
}

// Helper function to create test blocks for final tests
func createFinalTestBlocks(t *testing.T, count int) []blocks.Block {
	testBlocks := make([]blocks.Block, count)
	for i := 0; i < count; i++ {
		data := make([]byte, 256+rand.Intn(512))
		rand.Read(data)
		block := blocks.NewBlock(data)
		testBlocks[i] = block
	}
	return testBlocks
}
