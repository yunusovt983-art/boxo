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

// TestFaultToleranceComprehensive tests comprehensive fault tolerance scenarios
func TestFaultToleranceComprehensive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Run("NodeFailureRecoveryPattern", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(20 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(6)
		testBlocks := createFaultTestBlocks(t, 15)

		// Distribute blocks across first 3 nodes
		for i, block := range testBlocks {
			nodeIdx := i % 3
			err := instances[nodeIdx].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests int64

		// Continuous request load
		requestCtx, requestCancel := context.WithTimeout(ctx, 60*time.Second)
		defer requestCancel()

		for i := 0; i < 80; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

				atomic.AddInt64(&totalRequests, 1)
				requesterIdx := 3 + (requestID % 3) // Use last 3 nodes as requesters
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(requestCtx, 3*time.Second)
				defer fetchCancel()

				_, err := instances[requesterIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				}
			}(i)
		}

		// Simulate node failures and recoveries
		wg.Add(1)
		go func() {
			defer wg.Done()
			failureCtx, failureCancel := context.WithTimeout(ctx, 50*time.Second)
			defer failureCancel()

			ticker := time.NewTicker(8 * time.Second)
			defer ticker.Stop()

			failedNode := -1

			for {
				select {
				case <-failureCtx.Done():
					return
				case <-ticker.C:
					if failedNode == -1 {
						// Fail a node
						failedNode = rand.Intn(3) // Fail one of the block-holding nodes
						instances[failedNode].Exchange.Close()
						t.Logf("Failed node %d", failedNode)
					} else {
						// Recover the node
						newInstance := ig.Next()
						instances[failedNode] = newInstance

						// Restore blocks to recovered node
						for i, block := range testBlocks {
							if i%3 == failedNode {
								instances[failedNode].Blockstore.Put(ctx, block)
							}
						}

						t.Logf("Recovered node %d", failedNode)
						failedNode = -1
					}
				}
			}
		}()

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		successRate := float64(successful) / float64(total)

		t.Logf("Node failure/recovery results: %d/%d successful (%.2f%%)", successful, total, successRate*100)
		assert.Greater(t, successRate, 0.4, "System should maintain >40% success rate during node failures")
	})

	t.Run("NetworkPartitionTolerance", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(15 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(8)
		testBlocks := createFaultTestBlocks(t, 12)

		// Distribute blocks across first half
		for i, block := range testBlocks {
			nodeIdx := i % 4
			err := instances[nodeIdx].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests int64

		// Request load from second half
		requestCtx, requestCancel := context.WithTimeout(ctx, 45*time.Second)
		defer requestCancel()

		for i := 0; i < 60; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1500)) * time.Millisecond)

				atomic.AddInt64(&totalRequests, 1)
				requesterIdx := 4 + (requestID % 4) // Use second half as requesters
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(requestCtx, 4*time.Second)
				defer fetchCancel()

				_, err := instances[requesterIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				}
			}(i)
		}

		// Partition controller
		wg.Add(1)
		go func() {
			defer wg.Done()
			partitionCtx, partitionCancel := context.WithTimeout(ctx, 40*time.Second)
			defer partitionCancel()

			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()

			partitioned := false

			for {
				select {
				case <-partitionCtx.Done():
					return
				case <-ticker.C:
					if !partitioned {
						// Create partition
						for i := 0; i < 4; i++ {
							for j := 4; j < 8; j++ {
								instances[i].Adapter.DisconnectFrom(ctx, instances[j].Identity.ID())
							}
						}
						partitioned = true
						t.Log("Created network partition")
					} else {
						// Heal partition
						for i := 0; i < 4; i++ {
							for j := 4; j < 8; j++ {
								instances[i].Adapter.Connect(ctx, peer.AddrInfo{ID: instances[j].Identity.ID()})
							}
						}
						partitioned = false
						t.Log("Healed network partition")
					}
				}
			}
		}()

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		successRate := float64(successful) / float64(total)

		t.Logf("Network partition results: %d/%d successful (%.2f%%)", successful, total, successRate*100)
		assert.Greater(t, successRate, 0.25, "System should maintain >25% success rate during partitions")
	})
}

// TestCircuitBreakerFaultToleranceComprehensive tests circuit breaker fault tolerance
func TestCircuitBreakerFaultToleranceComprehensive(t *testing.T) {
	t.Run("HighConcurrencyStressTest", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 8,
			Interval:    2 * time.Second,
			Timeout:     3 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 4
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("stress-test", config)

		var wg sync.WaitGroup
		var totalRequests, successfulRequests, rejectedRequests int64

		// High concurrency stress test
		numWorkers := 50
		requestsPerWorker := 15

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < requestsPerWorker; j++ {
					atomic.AddInt64(&totalRequests, 1)

					err := cb.Execute(func() error {
						// Variable failure rate based on worker ID
						failureRate := 0.2 + 0.5*float64(workerID%4)/4.0 // 20-70% failure rate
						if rand.Float64() < failureRate {
							return errors.New("stress test failure")
						}
						// Simulate processing time
						time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond)
						return nil
					})

					switch err {
					case nil:
						atomic.AddInt64(&successfulRequests, 1)
					case circuitbreaker.ErrOpenState, circuitbreaker.ErrTooManyRequests:
						atomic.AddInt64(&rejectedRequests, 1)
					}

					time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
				}
			}(i)
		}

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		rejected := atomic.LoadInt64(&rejectedRequests)

		t.Logf("High concurrency stress test results:")
		t.Logf("  Total requests: %d", total)
		t.Logf("  Successful: %d", successful)
		t.Logf("  Rejected: %d", rejected)

		expectedTotal := int64(numWorkers * requestsPerWorker)
		assert.Equal(t, expectedTotal, total, "All requests should be accounted for")
		assert.Greater(t, rejected, int64(50), "Circuit breaker should have rejected substantial requests")
		assert.Greater(t, successful, int64(100), "Should have substantial successful requests")
	})

	t.Run("PeerIsolationStressTest", func(t *testing.T) {
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
		numPeers := 8
		peers := make([]peer.ID, numPeers)
		for i := 0; i < numPeers; i++ {
			peerID, err := peer.Decode(fmt.Sprintf("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXt%02d", i))
			require.NoError(t, err)
			peers[i] = peerID
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests, rejectedRequests int64
		var peerStats [8]struct {
			requests  int64
			successes int64
		}

		// Generate load with different failure patterns per peer
		for i := 0; i < 160; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()

				peerIdx := requestID % numPeers
				peerID := peers[peerIdx]

				atomic.AddInt64(&totalRequests, 1)
				atomic.AddInt64(&peerStats[peerIdx].requests, 1)

				err := bcb.ExecuteWithPeer(peerID, func() error {
					// Different failure patterns for different peers
					var failureRate float64
					switch peerIdx % 4 {
					case 0: // Healthy peers
						failureRate = 0.15
					case 1: // Moderately unhealthy peers
						failureRate = 0.45
					case 2: // Very unhealthy peers
						failureRate = 0.75
					case 3: // Extremely unhealthy peers
						failureRate = 0.9
					}

					if rand.Float64() < failureRate {
						return errors.New("peer-specific failure")
					}

					time.Sleep(time.Duration(rand.Intn(30)) * time.Millisecond)
					return nil
				})

				switch err {
				case nil:
					atomic.AddInt64(&successfulRequests, 1)
					atomic.AddInt64(&peerStats[peerIdx].successes, 1)
				case circuitbreaker.ErrOpenState, circuitbreaker.ErrTooManyRequests:
					atomic.AddInt64(&rejectedRequests, 1)
				}

				time.Sleep(time.Duration(rand.Intn(150)) * time.Millisecond)
			}(i)
		}

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		rejected := atomic.LoadInt64(&rejectedRequests)

		t.Logf("Peer isolation stress test results:")
		t.Logf("  Total requests: %d", total)
		t.Logf("  Successful: %d", successful)
		t.Logf("  Rejected: %d", rejected)

		// Analyze per-peer results
		for i := 0; i < numPeers; i++ {
			requests := atomic.LoadInt64(&peerStats[i].requests)
			successes := atomic.LoadInt64(&peerStats[i].successes)
			state := bcb.GetPeerState(peers[i])
			successRate := float64(successes) / float64(requests)

			t.Logf("  Peer %d: %d requests, %d successes (%.2f%%), state: %v",
				i, requests, successes, successRate*100, state)
		}

		assert.Equal(t, int64(160), total, "All requests should be processed")
		assert.Greater(t, successful, int64(30), "Should have successful requests from healthy peers")
		assert.Greater(t, rejected, int64(40), "Should have rejected requests from unhealthy peers")
	})

	t.Run("RecoveryPatternValidation", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 3,
			Interval:    1500 * time.Millisecond,
			Timeout:     2500 * time.Millisecond,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("recovery-pattern-test", config)

		var wg sync.WaitGroup
		var phaseResults []struct {
			phase      string
			successful int64
			total      int64
		}

		// Phase 1: High failure rate to trip circuit breaker
		t.Log("Phase 1: Tripping circuit breaker")
		var phase1Successful, phase1Total int64

		for i := 0; i < 25; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				atomic.AddInt64(&phase1Total, 1)

				err := cb.Execute(func() error {
					return errors.New("phase 1 failure") // 100% failure rate
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

		// Wait for potential reset
		time.Sleep(3 * time.Second)

		// Phase 2: Gradual recovery
		t.Log("Phase 2: Gradual recovery")
		var phase2Successful, phase2Total int64

		for i := 0; i < 30; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)

				atomic.AddInt64(&phase2Total, 1)

				err := cb.Execute(func() error {
					// Decreasing failure rate
					failureRate := 0.6 - 0.4*float64(requestID)/30.0 // 60% -> 20% failure rate
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

		// Phase 3: Stable operation
		t.Log("Phase 3: Stable operation")
		var phase3Successful, phase3Total int64

		for i := 0; i < 35; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)

				atomic.AddInt64(&phase3Total, 1)

				err := cb.Execute(func() error {
					// Low stable failure rate
					if rand.Float64() < 0.1 { // 10% failure rate
						return errors.New("phase 3 failure")
					}
					return nil
				})

				if err == nil {
					atomic.AddInt64(&phase3Successful, 1)
				}
			}()
		}

		wg.Wait()

		p3Total := atomic.LoadInt64(&phase3Total) - p2Total - p1Total
		p3Successful := atomic.LoadInt64(&phase3Successful) - p2Successful - p1Successful
		phaseResults = append(phaseResults, struct {
			phase      string
			successful int64
			total      int64
		}{"Phase 3 (Stable)", p3Successful, p3Total})

		// Analyze recovery pattern
		t.Logf("Recovery pattern validation results:")
		for _, phase := range phaseResults {
			if phase.total > 0 {
				successRate := float64(phase.successful) / float64(phase.total)
				t.Logf("  %s: %d/%d successful (%.2f%%)", phase.phase, phase.successful, phase.total, successRate*100)
			}
		}

		// Verify recovery pattern
		if len(phaseResults) >= 3 {
			phase1Rate := float64(phaseResults[0].successful) / float64(phaseResults[0].total)
			phase2Rate := float64(phaseResults[1].successful) / float64(phaseResults[1].total)
			phase3Rate := float64(phaseResults[2].successful) / float64(phaseResults[2].total)

			assert.Greater(t, phase2Rate, phase1Rate, "Recovery phase should show improvement")
			assert.Greater(t, phase3Rate, phase2Rate, "Stable phase should show further improvement")
			assert.Greater(t, phase3Rate, 0.7, "Stable phase should have >70% success rate")
		}
	})
}

// TestChaosEngineeringComprehensive implements comprehensive chaos engineering tests
func TestChaosEngineeringComprehensive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	t.Run("RandomNodeChaos", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(25 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(10)
		testBlocks := createFaultTestBlocks(t, 20)

		// Distribute blocks with redundancy
		for i, block := range testBlocks {
			for j := 0; j < 3; j++ { // Each block on 3 nodes
				nodeIdx := (i*3 + j) % len(instances)
				err := instances[nodeIdx].Blockstore.Put(ctx, block)
				require.NoError(t, err)
			}
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests int64
		var chaosEvents int64

		// Request load
		requestCtx, requestCancel := context.WithTimeout(ctx, 80*time.Second)
		defer requestCancel()

		for i := 0; i < 120; i++ {
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
			chaosCtx, chaosCancel := context.WithTimeout(ctx, 75*time.Second)
			defer chaosCancel()

			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()

			killedNodes := make(map[int]bool)

			for {
				select {
				case <-chaosCtx.Done():
					return
				case <-ticker.C:
					if rand.Float32() < 0.6 { // 60% chance of chaos event
						atomic.AddInt64(&chaosEvents, 1)

						if len(killedNodes) < 3 && rand.Float32() < 0.7 {
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
									for j := 0; j < 3; j++ {
										if (i*3+j)%len(instances) == nodeIdx && rand.Float32() < 0.7 {
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

		t.Logf("Random node chaos results: %d/%d successful (%.2f%%), %d chaos events",
			successful, total, successRate*100, events)

		assert.Greater(t, successRate, 0.3, "System should maintain >30% success rate under chaos")
		assert.Greater(t, events, int64(8), "Should have generated substantial chaos events")
	})

	t.Run("HighLatencyWithFailures", func(t *testing.T) {
		// Create network with high latency
		highLatencyNet := tn.VirtualNetwork(delay.Fixed(200 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(highLatencyNet, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(8)
		testBlocks := createFaultTestBlocks(t, 15)

		// Distribute blocks with redundancy
		for i, block := range testBlocks {
			for j := 0; j < 2; j++ { // Each block on 2 nodes
				nodeIdx := (i*2 + j) % len(instances)
				err := instances[nodeIdx].Blockstore.Put(ctx, block)
				require.NoError(t, err)
			}
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests, timeoutRequests int64

		// Request load with longer timeouts for high latency
		requestCtx, requestCancel := context.WithTimeout(ctx, 60*time.Second)
		defer requestCancel()

		for i := 0; i < 60; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)

				atomic.AddInt64(&totalRequests, 1)
				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				// Longer timeout to handle high latency
				timeout := time.Duration(1000+rand.Intn(2000)) * time.Millisecond
				fetchCtx, fetchCancel := context.WithTimeout(requestCtx, timeout)
				defer fetchCancel()

				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				} else if err == context.DeadlineExceeded {
					atomic.AddInt64(&timeoutRequests, 1)
				}
			}(i)
		}

		// Occasional node failures in high latency environment
		wg.Add(1)
		go func() {
			defer wg.Done()
			failureCtx, failureCancel := context.WithTimeout(ctx, 55*time.Second)
			defer failureCancel()

			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-failureCtx.Done():
					return
				case <-ticker.C:
					if rand.Float32() < 0.5 {
						// Randomly kill and restart a node
						nodeIdx := rand.Intn(len(instances))
						instances[nodeIdx].Exchange.Close()
						t.Logf("High latency chaos: Killed node %d", nodeIdx)

						time.Sleep(2 * time.Second)

						newInstance := ig.Next()
						instances[nodeIdx] = newInstance

						// Restore blocks
						for i, block := range testBlocks {
							for j := 0; j < 2; j++ {
								if (i*2+j)%len(instances) == nodeIdx {
									instances[nodeIdx].Blockstore.Put(ctx, block)
								}
							}
						}

						t.Logf("High latency chaos: Restarted node %d", nodeIdx)
					}
				}
			}
		}()

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		timeouts := atomic.LoadInt64(&timeoutRequests)
		successRate := float64(successful) / float64(total)

		t.Logf("High latency with failures results: %d/%d successful (%.2f%%), %d timeouts",
			successful, total, successRate*100, timeouts)

		assert.Greater(t, successRate, 0.2, "System should handle high latency + failures with >20% success rate")
		assert.Greater(t, timeouts, int64(5), "Should have experienced timeouts due to high latency")
	})
}

// Helper function to create test blocks for fault tolerance tests
func createFaultTestBlocks(t *testing.T, count int) []blocks.Block {
	testBlocks := make([]blocks.Block, count)
	for i := 0; i < count; i++ {
		data := make([]byte, 256+rand.Intn(512)) // Variable size blocks
		rand.Read(data)
		block := blocks.NewBlock(data)
		testBlocks[i] = block
	}
	return testBlocks
}
