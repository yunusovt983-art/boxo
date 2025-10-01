package bitswap_test

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	testinstance "github.com/ipfs/boxo/bitswap/testinstance"
	tn "github.com/ipfs/boxo/bitswap/testnet"
	mockrouting "github.com/ipfs/boxo/routing/mock"
	blocks "github.com/ipfs/go-block-format"
	delay "github.com/ipfs/go-ipfs-delay"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSimpleFaultTolerance implements a simple fault tolerance test to verify task completion
func TestSimpleFaultTolerance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Run("BasicNodeFailureRecovery", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(20 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(6)
		testBlocks := createSimpleTestBlocks(t, 10)

		// Distribute blocks across first 3 nodes
		for i, block := range testBlocks {
			nodeIdx := i % 3
			err := instances[nodeIdx].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests int64

		// Request load from last 3 nodes
		requestCtx, requestCancel := context.WithTimeout(ctx, 30*time.Second)
		defer requestCancel()

		for i := 0; i < 30; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

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
			time.Sleep(5 * time.Second)
			failedNodeIdx := 1
			instances[failedNodeIdx].Exchange.Close()
			t.Logf("Failed node %d", failedNodeIdx)

			// Wait then recover
			time.Sleep(8 * time.Second)
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

		t.Logf("Simple fault tolerance test: %d/%d successful (%.2f%%)", successful, total, successRate*100)
		assert.Greater(t, successRate, 0.3, "System should maintain >30% success rate during node failure")
		assert.Greater(t, total, int64(25), "Should have processed substantial requests")
	})

	t.Run("NetworkPartitionTest", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(15 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(8)
		testBlocks := createSimpleTestBlocks(t, 12)

		// Distribute blocks in first half
		for i, block := range testBlocks {
			nodeIdx := i % 4
			err := instances[nodeIdx].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		var totalRequests, successfulRequests int64

		// Requests from second half
		requestCtx, requestCancel := context.WithTimeout(ctx, 25*time.Second)
		defer requestCancel()

		for i := 0; i < 40; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(800)) * time.Millisecond)

				atomic.AddInt64(&totalRequests, 1)
				requesterIdx := 4 + (requestID % 4)
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(requestCtx, 3*time.Second)
				defer fetchCancel()

				_, err := instances[requesterIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&successfulRequests, 1)
				}
			}(i)
		}

		// Create and heal partition
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Create partition after 5 seconds
			time.Sleep(5 * time.Second)
			for i := 0; i < 4; i++ {
				for j := 4; j < 8; j++ {
					instances[i].Adapter.DisconnectFrom(ctx, instances[j].Identity.ID())
				}
			}
			t.Log("Created network partition")

			// Heal partition after 10 seconds
			time.Sleep(10 * time.Second)
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

		t.Logf("Network partition test: %d/%d successful (%.2f%%)", successful, total, successRate*100)
		assert.Greater(t, successRate, 0.2, "System should maintain >20% success rate during partition")
		assert.Greater(t, total, int64(30), "Should have processed substantial requests")
	})
}

// TestChaosEngineeringBasic implements basic chaos engineering tests
func TestChaosEngineeringBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t.Run("RandomNodeFailures", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(25 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(10)
		testBlocks := createSimpleTestBlocks(t, 20)

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
		requestCtx, requestCancel := context.WithTimeout(ctx, 50*time.Second)
		defer requestCancel()

		for i := 0; i < 80; i++ {
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

		// Chaos controller
		wg.Add(1)
		go func() {
			defer wg.Done()
			chaosCtx, chaosCancel := context.WithTimeout(ctx, 45*time.Second)
			defer chaosCancel()

			ticker := time.NewTicker(4 * time.Second)
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

		assert.Greater(t, successRate, 0.25, "System should maintain >25% success rate under chaos")
		assert.Greater(t, events, int64(3), "Should have generated chaos events")
		assert.Greater(t, total, int64(60), "Should have processed substantial requests")
	})
}

// Helper function to create test blocks for simple tests
func createSimpleTestBlocks(t *testing.T, count int) []blocks.Block {
	testBlocks := make([]blocks.Block, count)
	for i := 0; i < count; i++ {
		data := make([]byte, 256+rand.Intn(256))
		rand.Read(data)
		block := blocks.NewBlock(data)
		testBlocks[i] = block
	}
	return testBlocks
}
