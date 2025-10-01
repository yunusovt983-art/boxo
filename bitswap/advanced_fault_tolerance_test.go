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

// TestAdvancedNetworkFailures implements advanced network failure simulation tests
func TestAdvancedNetworkFailures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Run("CascadingNodeFailures", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(20 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(10)
		testBlocks := createAdvancedTestBlocks(t, 25)

		// Distribute blocks across first 5 nodes with redundancy
		for i, block := range testBlocks {
			for j := 0; j < 2; j++ { // Each block on 2 nodes
				nodeIdx := (i*2 + j) % 5
				err := instances[nodeIdx].Blockstore.Put(ctx, block)
				require.NoError(t, err)
			}
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests int64

		// Request load from last 5 nodes
		requestCtx, requestCancel := context.WithTimeout(ctx, 60*time.Second)
		defer requestCancel()

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

				atomic.AddInt64(&totalRequests, 1)
				requesterIdx := 5 + (requestID % 5)
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(requestCtx, 4*time.Second)
				defer fetchCancel()

				_, err := instances[requesterIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				}
			}(i)
		}

		// Simulate cascading failures
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Fail nodes in cascade
			failureNodes := []int{1, 2, 3}
			for i, nodeIdx := range failureNodes {
				time.Sleep(time.Duration(8+i*5) * time.Second)
				instances[nodeIdx].Exchange.Close()
				t.Logf("Cascading failure: killed node %d", nodeIdx)
			}

			// Wait and then recover some nodes
			time.Sleep(15 * time.Second)
			for i, nodeIdx := range failureNodes[:2] { // Recover first 2 nodes
				newInstance := ig.Next()
				instances[nodeIdx] = newInstance

				// Restore blocks
				for j, block := range testBlocks {
					if (j*2+i)%5 == nodeIdx {
						instances[nodeIdx].Blockstore.Put(ctx, block)
					}
				}
				t.Logf("Cascading recovery: recovered node %d", nodeIdx)
			}
		}()

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		successRate := float64(successful) / float64(total)

		t.Logf("Cascading node failures: %d/%d successful (%.2f%%)", successful, total, successRate*100)
		assert.Greater(t, successRate, 0.3, "System should maintain >30% success rate during cascading failures")
	})

	t.Run("SplitBrainScenario", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(15 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(12)
		testBlocks := createAdvancedTestBlocks(t, 20)

		// Distribute blocks across both halves for redundancy
		for i, block := range testBlocks {
			// Put each block in both halves
			err := instances[i%6].Blockstore.Put(ctx, block)
			require.NoError(t, err)
			err = instances[6+(i%6)].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests int64

		// Request load from all nodes
		requestCtx, requestCancel := context.WithTimeout(ctx, 45*time.Second)
		defer requestCancel()

		for i := 0; i < 80; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1500)) * time.Millisecond)

				atomic.AddInt64(&totalRequests, 1)
				requesterIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(requestCtx, 4*time.Second)
				defer fetchCancel()

				_, err := instances[requesterIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				}
			}(i)
		}

		// Create split-brain partition
		wg.Add(1)
		go func() {
			defer wg.Done()

			time.Sleep(10 * time.Second)
			// Partition the network in half
			for i := 0; i < 6; i++ {
				for j := 6; j < 12; j++ {
					instances[i].Adapter.DisconnectFrom(ctx, instances[j].Identity.ID())
				}
			}
			t.Log("Created split-brain partition")

			// Wait and then heal
			time.Sleep(20 * time.Second)
			for i := 0; i < 6; i++ {
				for j := 6; j < 12; j++ {
					instances[i].Adapter.Connect(ctx, peer.AddrInfo{ID: instances[j].Identity.ID()})
				}
			}
			t.Log("Healed split-brain partition")
		}()

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		successRate := float64(successful) / float64(total)

		t.Logf("Split-brain scenario: %d/%d successful (%.2f%%)", successful, total, successRate*100)
		assert.Greater(t, successRate, 0.35, "System should maintain >35% success rate during split-brain")
	})

	t.Run("ByzantineFaultTolerance", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(25 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(15) // 15 nodes, up to 5 can be byzantine
		testBlocks := createAdvancedTestBlocks(t, 30)

		// Distribute blocks across honest nodes (first 10 nodes)
		honestNodes := []int{0, 1, 2, 3, 4, 10, 11, 12, 13, 14}
		for i, block := range testBlocks {
			honestNodeIdx := honestNodes[i%len(honestNodes)]
			err := instances[honestNodeIdx].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests int64

		// Request load from all nodes
		requestCtx, requestCancel := context.WithTimeout(ctx, 40*time.Second)
		defer requestCancel()

		for i := 0; i < 120; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1200)) * time.Millisecond)

				atomic.AddInt64(&totalRequests, 1)
				requesterIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(requestCtx, 4*time.Second)
				defer fetchCancel()

				_, err := instances[requesterIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				}
			}(i)
		}

		// Simulate byzantine behavior
		wg.Add(1)
		go func() {
			defer wg.Done()
			byzantineCtx, byzantineCancel := context.WithTimeout(ctx, 35*time.Second)
			defer byzantineCancel()

			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()

			byzantineNodes := []int{5, 6, 7, 8, 9} // 5 byzantine nodes

			for {
				select {
				case <-byzantineCtx.Done():
					return
				case <-ticker.C:
					// Byzantine nodes behave maliciously
					for _, nodeIdx := range byzantineNodes {
						if rand.Float32() < 0.6 {
							// Randomly disconnect from honest nodes
							honestNodeIdx := honestNodes[rand.Intn(len(honestNodes))]
							instances[nodeIdx].Adapter.DisconnectFrom(ctx, instances[honestNodeIdx].Identity.ID())
						}

						if rand.Float32() < 0.3 {
							// Randomly reconnect
							honestNodeIdx := honestNodes[rand.Intn(len(honestNodes))]
							instances[nodeIdx].Adapter.Connect(ctx, peer.AddrInfo{ID: instances[honestNodeIdx].Identity.ID()})
						}
					}
				}
			}
		}()

		wg.Wait()

		total := atomic.LoadInt64(&totalRequests)
		successful := atomic.LoadInt64(&successfulRequests)
		successRate := float64(successful) / float64(total)

		t.Logf("Byzantine fault tolerance: %d/%d successful (%.2f%%)", successful, total, successRate*100)
		assert.Greater(t, successRate, 0.4, "System should maintain >40% success rate with byzantine faults")
	})
}

// TestAdvancedCircuitBreakerScenarios tests circuit breaker under various failure scenarios
func TestAdvancedCircuitBreakerScenarios(t *testing.T) {
	t.Run("CircuitBreakerUnderChaos", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 8,
			Interval:    2 * time.Second,
			Timeout:     3 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 4
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("chaos-test", config)

		var wg sync.WaitGroup
		var totalRequests, successfulRequests, rejectedRequests int64

		// Simulate chaos with high concurrency
		numWorkers := 60
		requestsPerWorker := 8

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < requestsPerWorker; j++ {
					atomic.AddInt64(&totalRequests, 1)

					err := cb.Execute(func() error {
						// Simulate chaos with varying failure rates
						var failureRate float64
						switch workerID % 4 {
						case 0: // Stable workers
							failureRate = 0.1
						case 1: // Moderately unstable
							failureRate = 0.4
						case 2: // Highly unstable
							failureRate = 0.7
						case 3: // Extremely unstable (chaos)
							failureRate = 0.9
						}

						if rand.Float64() < failureRate {
							return errors.New("chaos failure")
						}
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

		t.Logf("Circuit breaker under chaos:")
		t.Logf("  Total: %d, Successful: %d, Rejected: %d", total, successful, rejected)

		expectedTotal := int64(numWorkers * requestsPerWorker)
		assert.Equal(t, expectedTotal, total, "All requests should be accounted for")
		assert.Greater(t, rejected, int64(50), "Circuit breaker should reject substantial requests under chaos")
		assert.Greater(t, successful, int64(100), "Should have successful requests from stable workers")
	})

	t.Run("PeerCircuitBreakerWithFailures", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 5,
			Interval:    1500 * time.Millisecond,
			Timeout:     2500 * time.Millisecond,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
		}

		bcb := circuitbreaker.NewBitswapCircuitBreaker(config)

		// Create test peers with different reliability profiles
		numPeers := 8
		peers := make([]peer.ID, numPeers)
		for i := 0; i < numPeers; i++ {
			peerID, err := peer.Decode(fmt.Sprintf("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXt%02d", i))
			require.NoError(t, err)
			peers[i] = peerID
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests, rejectedRequests int64

		// Generate load with different failure patterns per peer
		for i := 0; i < 160; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()

				peerIdx := requestID % numPeers
				peerID := peers[peerIdx]

				atomic.AddInt64(&totalRequests, 1)

				err := bcb.ExecuteWithPeer(peerID, func() error {
					// Different reliability profiles
					var failureRate float64
					switch peerIdx % 4 {
					case 0: // Highly reliable peers
						failureRate = 0.05
					case 1: // Moderately reliable peers
						failureRate = 0.3
					case 2: // Unreliable peers
						failureRate = 0.6
					case 3: // Extremely unreliable peers
						failureRate = 0.85
					}

					if rand.Float64() < failureRate {
						return errors.New("peer failure")
					}

					time.Sleep(time.Duration(rand.Intn(30)) * time.Millisecond)
					return nil
				})

				switch err {
				case nil:
					atomic.AddInt64(&successfulRequests, 1)
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

		t.Logf("Peer circuit breaker with failures:")
		t.Logf("  Total: %d, Successful: %d, Rejected: %d", total, successful, rejected)

		// Analyze per-peer results
		reliablePeers := 0
		for i, peerID := range peers {
			state := bcb.GetPeerState(peerID)
			if state == circuitbreaker.StateClosed {
				reliablePeers++
			}
			t.Logf("  Peer %d (reliability profile %d) state: %v", i, i%4, state)
		}

		assert.Equal(t, int64(160), total, "All requests should be processed")
		assert.Greater(t, successful, int64(40), "Should have successful requests from reliable peers")
		assert.Greater(t, rejected, int64(30), "Should have rejected requests from unreliable peers")
		assert.Greater(t, reliablePeers, 1, "Should have some reliable peers remaining")
	})

	t.Run("CircuitBreakerRecoveryUnderLoad", func(t *testing.T) {
		config := circuitbreaker.Config{
			MaxRequests: 4,
			Interval:    2 * time.Second,
			Timeout:     3 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 2
			},
		}

		cb := circuitbreaker.NewCircuitBreaker("recovery-load-test", config)

		var wg sync.WaitGroup
		var phaseResults []struct {
			phase      string
			successful int64
			total      int64
		}

		// Phase 1: High failure rate to trip circuit breaker
		t.Log("Phase 1: Tripping circuit breaker with high failure rate")
		var phase1Successful, phase1Total int64

		for i := 0; i < 30; i++ {
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
		time.Sleep(4 * time.Second)

		// Phase 2: Gradual recovery under load
		t.Log("Phase 2: Gradual recovery under continuous load")
		var phase2Successful, phase2Total int64

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(400)) * time.Millisecond)

				atomic.AddInt64(&phase2Total, 1)

				err := cb.Execute(func() error {
					// Gradually decreasing failure rate
					failureRate := 0.7 - 0.5*float64(requestID)/50.0 // 70% -> 20% failure rate
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

		// Phase 3: Stable operation under load
		t.Log("Phase 3: Stable operation under high load")
		var phase3Successful, phase3Total int64

		for i := 0; i < 60; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)

				atomic.AddInt64(&phase3Total, 1)

				err := cb.Execute(func() error {
					// Low stable failure rate
					if rand.Float64() < 0.15 { // 15% failure rate
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
		t.Logf("Circuit breaker recovery under load results:")
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
			assert.Greater(t, phase3Rate, 0.75, "Stable phase should have >75% success rate")
		}
	})
}

// TestAdvancedRecoveryPatterns tests advanced recovery patterns and strategies
func TestAdvancedRecoveryPatterns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	t.Run("MultiPhaseRecoveryPattern", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(15 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(12)
		testBlocks := createAdvancedTestBlocks(t, 30)

		// Distribute blocks across first 6 nodes
		for i, block := range testBlocks {
			err := instances[i%6].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		// Simulate major failure - kill half the nodes
		failedNodes := []int{1, 2, 3, 4, 5, 6}
		for _, nodeIdx := range failedNodes {
			instances[nodeIdx].Exchange.Close()
		}

		var wg sync.WaitGroup
		var phaseResults []struct {
			phase      string
			successful int64
			total      int64
		}

		// Multi-phase recovery
		phases := []struct {
			name           string
			nodesToRecover int
			requestCount   int
			waitTime       time.Duration
		}{
			{"Phase 1: Emergency recovery (2 nodes)", 2, 20, 3 * time.Second},
			{"Phase 2: Partial recovery (2 more nodes)", 2, 30, 3 * time.Second},
			{"Phase 3: Full recovery (remaining nodes)", 2, 40, 3 * time.Second},
		}

		recoveredNodes := 0
		for _, phase := range phases {
			t.Logf("Starting %s", phase.name)

			// Recover nodes for this phase
			for i := 0; i < phase.nodesToRecover && recoveredNodes < len(failedNodes); i++ {
				nodeIdx := failedNodes[recoveredNodes]
				instances[nodeIdx] = ig.Next()

				// Reconnect to available nodes
				for j := 0; j < len(instances); j++ {
					if j != nodeIdx && (j < failedNodes[0] || j > failedNodes[len(failedNodes)-1] || j <= failedNodes[recoveredNodes]) {
						instances[nodeIdx].Adapter.Connect(ctx, peer.AddrInfo{ID: instances[j].Identity.ID()})
					}
				}

				// Restore some blocks
				for k, block := range testBlocks {
					if k%6 == nodeIdx {
						instances[nodeIdx].Blockstore.Put(ctx, block)
					}
				}

				recoveredNodes++
				t.Logf("Recovered node %d", nodeIdx)
			}

			// Wait for stabilization
			time.Sleep(phase.waitTime)

			// Test with load
			var phaseSuccessful, phaseTotal int64

			for i := 0; i < phase.requestCount; i++ {
				wg.Add(1)
				go func(requestID int) {
					defer wg.Done()
					atomic.AddInt64(&phaseTotal, 1)

					nodeIdx := rand.Intn(len(instances))
					blockIdx := rand.Intn(len(testBlocks))

					fetchCtx, fetchCancel := context.WithTimeout(ctx, 4*time.Second)
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
				phase      string
				successful int64
				total      int64
			}{phase.name, successful, total})

			t.Logf("%s results: %d/%d successful (%.2f%%)",
				phase.name, successful, total, successRate*100)

			// Reset counters for next phase
			atomic.StoreInt64(&phaseSuccessful, 0)
			atomic.StoreInt64(&phaseTotal, 0)
		}

		// Verify progressive improvement
		for i := 1; i < len(phaseResults); i++ {
			prevRate := float64(phaseResults[i-1].successful) / float64(phaseResults[i-1].total)
			currRate := float64(phaseResults[i].successful) / float64(phaseResults[i].total)

			t.Logf("Phase %d->%d improvement: %.2f%% -> %.2f%%", i, i+1, prevRate*100, currRate*100)
			assert.GreaterOrEqual(t, currRate, prevRate-0.1, "Success rate should generally improve between phases")
		}

		// Final phase should have good success rate
		finalPhase := phaseResults[len(phaseResults)-1]
		finalRate := float64(finalPhase.successful) / float64(finalPhase.total)
		assert.Greater(t, finalRate, 0.6, "Final recovery phase should have >60% success rate")
	})

	t.Run("AdaptiveRecoveryWithBackoff", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(25 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(8)
		testBlocks := createAdvancedTestBlocks(t, 15)

		// Add blocks to first instance
		for _, block := range testBlocks {
			err := instances[0].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		// Simulate adaptive recovery with backoff
		failingNodeIdx := 2
		var recoveryAttempts []struct {
			attempt   int
			timestamp time.Time
			backoff   time.Duration
			success   bool
		}

		// Initial failure
		instances[failingNodeIdx].Exchange.Close()
		t.Logf("Node %d failed initially", failingNodeIdx)

		// Adaptive recovery with exponential backoff and jitter
		baseBackoff := 500 * time.Millisecond
		maxBackoff := 10 * time.Second
		maxAttempts := 8
		successfulRecovery := false

		for attempt := 0; attempt < maxAttempts; attempt++ {
			// Calculate backoff with jitter
			backoffDuration := time.Duration(float64(baseBackoff) * float64(int(1)<<uint(attempt)))
			if backoffDuration > maxBackoff {
				backoffDuration = maxBackoff
			}
			// Add jitter (±25%)
			jitter := time.Duration(rand.Float64()*0.5-0.25) * backoffDuration
			backoffDuration += jitter

			time.Sleep(backoffDuration)

			// Attempt recovery
			instances[failingNodeIdx] = ig.Next()
			err := instances[failingNodeIdx].Adapter.Connect(
				ctx, peer.AddrInfo{ID: instances[0].Identity.ID()})
			require.NoError(t, err)

			// Test recovery success
			fetchCtx, fetchCancel := context.WithTimeout(ctx, 3*time.Second)
			_, err = instances[failingNodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[0].Cid())
			fetchCancel()

			success := err == nil
			recoveryAttempts = append(recoveryAttempts, struct {
				attempt   int
				timestamp time.Time
				backoff   time.Duration
				success   bool
			}{attempt + 1, time.Now(), backoffDuration, success})

			if success {
				successfulRecovery = true
				t.Logf("Node %d recovered successfully after %d attempts with adaptive backoff",
					failingNodeIdx, len(recoveryAttempts))
				break
			} else {
				t.Logf("Recovery attempt %d failed, backoff: %v", len(recoveryAttempts), backoffDuration)
				instances[failingNodeIdx].Exchange.Close()
			}
		}

		// Analyze recovery pattern
		t.Logf("Adaptive recovery attempts:")
		for _, attempt := range recoveryAttempts {
			t.Logf("  Attempt %d: backoff %v, success: %v", attempt.attempt, attempt.backoff, attempt.success)
		}

		assert.True(t, successfulRecovery, "Node should eventually recover with adaptive backoff")
		assert.Greater(t, len(recoveryAttempts), 2, "Should have made multiple recovery attempts")
		assert.LessOrEqual(t, len(recoveryAttempts), maxAttempts, "Should not exceed maximum attempts")

		// Verify backoff pattern
		for i := 1; i < len(recoveryAttempts)-1; i++ {
			// Backoff should generally increase (allowing for jitter)
			prevBackoff := recoveryAttempts[i-1].backoff
			currBackoff := recoveryAttempts[i].backoff
			assert.GreaterOrEqual(t, currBackoff, prevBackoff/2, "Backoff should not decrease too dramatically")
		}
	})
}

// Helper function to create test blocks for advanced tests
func createAdvancedTestBlocks(t *testing.T, count int) []blocks.Block {
	testBlocks := make([]blocks.Block, count)
	for i := 0; i < count; i++ {
		data := make([]byte, 512+rand.Intn(1024)) // Larger blocks for advanced tests
		rand.Read(data)
		block := blocks.NewBlock(data)
		testBlocks[i] = block
	}
	return testBlocks
}
