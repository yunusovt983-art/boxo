package priority_test

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/boxo/bitswap/adaptive"
	"github.com/ipfs/boxo/bitswap/priority"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
)

func ExamplePriorityRequestManager() {
	// Create a priority request manager
	prm := priority.NewPriorityRequestManager(
		100,                    // max concurrent requests
		10*1024*1024,          // max bandwidth per second (10MB)
		50*time.Millisecond,   // high priority threshold
		10*time.Millisecond,   // critical priority threshold
	)

	// Start processing
	ctx := context.Background()
	prm.Start(ctx)
	defer prm.Stop()

	// Create a sample CID and peer ID
	testCID, _ := cid.Parse("QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG")
	testPeerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

	// Enqueue a high-priority request
	reqCtx := priority.RequestContext{
		CID:         testCID,
		PeerID:      testPeerID,
		RequestTime: time.Now(),
		Deadline:    time.Now().Add(20 * time.Millisecond), // Urgent deadline
		BlockSize:   1024,
		SessionID:   "session-123",
		IsSession:   true,
	}

	err := prm.EnqueueRequest(reqCtx)
	if err != nil {
		fmt.Printf("Failed to enqueue request: %v\n", err)
		return
	}

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)

	// Get statistics
	stats := prm.GetStats()
	fmt.Printf("Total requests: %d\n", stats.TotalRequests)
	fmt.Printf("Manager started successfully\n")

	// Output:
	// Total requests: 1
	// Manager started successfully
}

func ExampleAdaptivePriorityManager() {
	// Create adaptive configuration
	adaptiveConfig := adaptive.NewAdaptiveBitswapConfig()

	// Create adaptive priority manager
	apm := priority.NewAdaptivePriorityManager(adaptiveConfig, 30*time.Second)

	// Start the adaptive manager
	apm.Start()
	defer apm.Stop()

	// Create sample request
	testCID, _ := cid.Parse("QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG")
	testPeerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

	// Enqueue requests with different priorities
	requests := []priority.RequestContext{
		{
			CID:          testCID,
			PeerID:       testPeerID,
			RequestTime:  time.Now(),
			Deadline:     time.Now().Add(5 * time.Millisecond), // Critical
			BlockSize:    512,
			UserPriority: priority.CriticalPriority,
		},
		{
			CID:         testCID,
			PeerID:      testPeerID,
			RequestTime: time.Now(),
			Deadline:    time.Now().Add(100 * time.Millisecond), // Normal
			BlockSize:   1024,
			IsSession:   true,
		},
	}

	for _, req := range requests {
		err := apm.EnqueueRequest(req)
		if err != nil {
			fmt.Printf("Failed to enqueue request: %v\n", err)
			continue
		}
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Get adaptive metrics
	metrics := apm.GetAdaptiveMetrics()
	fmt.Printf("Total requests enqueued: %d\n", len(requests))
	fmt.Printf("Current worker count: %d\n", metrics.ConfigSnapshot.CurrentWorkerCount)

	// Output:
	// Total requests enqueued: 2
	// Current worker count: 128
}

func ExamplePriorityRequestManager_ClassifyRequest() {
	prm := priority.NewPriorityRequestManager(10, 1000, 50*time.Millisecond, 10*time.Millisecond)

	testCID, _ := cid.Parse("QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG")
	testPeerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

	// Different request scenarios
	scenarios := []struct {
		name    string
		context priority.RequestContext
	}{
		{
			name: "Critical - Very urgent deadline",
			context: priority.RequestContext{
				CID:         testCID,
				PeerID:      testPeerID,
				RequestTime: time.Now(),
				Deadline:    time.Now().Add(5 * time.Millisecond),
				BlockSize:   100,
			},
		},
		{
			name: "High - Multiple retries",
			context: priority.RequestContext{
				CID:         testCID,
				PeerID:      testPeerID,
				RequestTime: time.Now(),
				Deadline:    time.Now().Add(200 * time.Millisecond),
				BlockSize:   100,
				RetryCount:  3,
			},
		},
		{
			name: "Normal - Session request",
			context: priority.RequestContext{
				CID:         testCID,
				PeerID:      testPeerID,
				RequestTime: time.Now(),
				Deadline:    time.Now().Add(200 * time.Millisecond),
				BlockSize:   100,
				IsSession:   true,
			},
		},
		{
			name: "Low - Default",
			context: priority.RequestContext{
				CID:         testCID,
				PeerID:      testPeerID,
				RequestTime: time.Now(),
				Deadline:    time.Now().Add(200 * time.Millisecond),
				BlockSize:   100,
			},
		},
	}

	for _, scenario := range scenarios {
		priority := prm.ClassifyRequest(scenario.context)
		fmt.Printf("%s: %s\n", scenario.name, priority)
	}

	// Output:
	// Critical - Very urgent deadline: critical
	// High - Multiple retries: high
	// Normal - Session request: normal
	// Low - Default: low
}

func ExampleNewResourcePool() {
	// Create a resource pool with limits
	rp := priority.NewResourcePool(
		5,    // max 5 concurrent requests
		1000, // max 1000 bytes per second
	)

	// Check if we can allocate resources
	canAllocate := rp.CanAllocate(200) // 200 bytes
	fmt.Printf("Can allocate 200 bytes: %t\n", canAllocate)

	// Allocate resources
	allocated := rp.Allocate(200)
	fmt.Printf("Successfully allocated: %t\n", allocated)

	// Check current usage
	current, max, bandwidth, maxBandwidth := rp.GetStats()
	fmt.Printf("Current requests: %d/%d\n", current, max)
	fmt.Printf("Current bandwidth: %d/%d bytes/sec\n", bandwidth, maxBandwidth)

	// Release resources
	rp.Release(200)

	// Check usage after release
	current, _, _, _ = rp.GetStats()
	fmt.Printf("Requests after release: %d\n", current)

	// Output:
	// Can allocate 200 bytes: true
	// Successfully allocated: true
	// Current requests: 1/5
	// Current bandwidth: 200/1000 bytes/sec
	// Requests after release: 0
}