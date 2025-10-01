package circuitbreaker_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/boxo/bitswap/circuitbreaker"
	"github.com/libp2p/go-libp2p/core/peer"
)

// ExampleBitswapProtectionManager demonstrates how to use the circuit breaker
// for protecting Bitswap operations under high load.
func ExampleBitswapProtectionManager() {
	// Create a protection manager with default configuration
	config := circuitbreaker.DefaultProtectionConfig()
	
	// Customize configuration for your use case
	config.CircuitBreaker.ReadyToTrip = func(counts circuitbreaker.Counts) bool {
		// Trip circuit breaker after 5 consecutive failures
		return counts.ConsecutiveFailures >= 5
	}
	config.CircuitBreaker.Timeout = 30 * time.Second // Wait 30s before trying half-open
	config.EnableAutoRecovery = true
	
	pm := circuitbreaker.NewBitswapProtectionManager(config)
	defer pm.Close()
	
	// Example peer ID
	peerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	
	ctx := context.Background()
	
	// Simulate successful Bitswap operations
	for i := 0; i < 5; i++ {
		err := pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
			// Your Bitswap operation here
			fmt.Printf("Successful request %d\n", i+1)
			return nil
		})
		if err != nil {
			fmt.Printf("Request failed: %v\n", err)
		}
	}
	
	// Simulate some failures
	for i := 0; i < 3; i++ {
		err := pm.ExecuteRequest(ctx, peerID, func(ctx context.Context) error {
			// Simulate a failing operation
			return errors.New("network timeout")
		})
		if err != nil {
			fmt.Printf("Request failed: %v\n", err)
		}
	}
	
	// Output:
	// Successful request 1
	// Successful request 2
	// Successful request 3
	// Successful request 4
	// Successful request 5
	// Request failed: network timeout
	// Request failed: network timeout
	// Request failed: network timeout
}

// ExampleCircuitBreaker demonstrates basic circuit breaker usage
func ExampleCircuitBreaker() {
	config := circuitbreaker.Config{
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts circuitbreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	}
	
	cb := circuitbreaker.NewCircuitBreaker("example", config)
	
	// Successful operation
	err := cb.Execute(func() error {
		fmt.Println("Operation succeeded")
		return nil
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	
	// Failing operations to trip the circuit breaker
	for i := 0; i < 3; i++ {
		err := cb.Execute(func() error {
			return errors.New("operation failed")
		})
		fmt.Printf("Attempt %d: %v\n", i+1, err)
	}
	
	// Circuit breaker should now be open
	fmt.Printf("Circuit breaker state: %s\n", cb.State())
	
	// This request will be rejected immediately
	err = cb.Execute(func() error {
		fmt.Println("This won't be executed")
		return nil
	})
	fmt.Printf("Rejected request: %v\n", err)
	
	// Output:
	// Operation succeeded
	// Attempt 1: operation failed
	// Attempt 2: operation failed
	// Attempt 3: operation failed
	// Circuit breaker state: OPEN
	// Rejected request: circuit breaker is open
}

// ExampleHealthMonitor demonstrates health monitoring functionality
func ExampleHealthMonitor() {
	config := circuitbreaker.DefaultHealthMonitorConfig()
	hm := circuitbreaker.NewHealthMonitor(config)
	defer hm.Close()
	
	peerID, _ := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	
	// Set up health change callback
	hm.SetHealthChangeCallback(func(p peer.ID, from, to circuitbreaker.HealthStatus) {
		fmt.Printf("Peer %s health changed: %s -> %s\n", 
			p.String()[:8], from, to)
	})
	
	// Record successful requests
	for i := 0; i < 10; i++ {
		hm.RecordRequest(peerID, 50*time.Millisecond, nil)
	}
	
	fmt.Printf("Health status: %s\n", hm.GetHealthStatus(peerID))
	
	// Record some failures
	for i := 0; i < 5; i++ {
		hm.RecordRequest(peerID, 200*time.Millisecond, errors.New("timeout"))
	}
	
	fmt.Printf("Health status after failures: %s\n", hm.GetHealthStatus(peerID))
	
	// Get health statistics
	stats := hm.GetHealthStats()
	fmt.Printf("Health stats: %d total peers, %d healthy, %d degraded, %d unhealthy\n",
		stats.TotalPeers, stats.HealthyPeers, stats.DegradedPeers, stats.UnhealthyPeers)
}