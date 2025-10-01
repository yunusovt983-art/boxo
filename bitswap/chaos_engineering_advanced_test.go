package bitswap_test

import (
	"context"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AdvancedChaosScenario represents a complex chaos engineering scenario
type AdvancedChaosScenario struct {
	Name       string
	Duration   time.Duration
	Events     []ChaosEventConfig
	Validators []ValidationFunc
	Metrics    *ChaosMetrics
}

// ChaosEventConfig defines a chaos event configuration
type ChaosEventConfig struct {
	Type        string
	Probability float64
	Delay       time.Duration
	Parameters  map[string]interface{}
}

// ValidationFunc validates system behavior during chaos
type ValidationFunc func(metrics *ChaosMetrics) bool

// ChaosMetrics tracks chaos engineering metrics
type ChaosMetrics struct {
	TotalRequests       int64
	SuccessfulRequests  int64
	FailedRequests      int64
	TimeoutRequests     int64
	CircuitBreakerTrips int64
	NodeFailures        int64
	NetworkPartitions   int64
	RecoveryEvents      int64
	AverageLatency      time.Duration
	MaxLatency          time.Duration
	StartTime           time.Time
	EndTime             time.Time
}

// AdvancedChaosController manages complex chaos scenarios
type AdvancedChaosController struct {
	instances   []testinstance.Instance
	testBlocks  []blocks.Block
	ctx         context.Context
	net         tn.Network
	ig          *testinstance.InstanceGenerator
	mu          sync.RWMutex
	killedNodes map[int]bool
	partitions  map[string]bool
	metrics     *ChaosMetrics
	isActive    bool
	events      []ChaosEvent
}

// NewAdvancedChaosController creates a new advanced chaos controller
func NewAdvancedChaosController(ctx context.Context, instances []testinstance.Instance, testBlocks []blocks.Block, net tn.Network, ig *testinstance.InstanceGenerator) *AdvancedChaosController {
	return &AdvancedChaosController{
		instances:   instances,
		testBlocks:  testBlocks,
		ctx:         ctx,
		net:         net,
		ig:          ig,
		killedNodes: make(map[int]bool),
		partitions:  make(map[string]bool),
		metrics:     &ChaosMetrics{StartTime: time.Now()},
		events:      make([]ChaosEvent, 0),
	}
}

// ExecuteScenario executes a complex chaos scenario
func (acc *AdvancedChaosController) ExecuteScenario(scenario AdvancedChaosScenario) error {
	acc.mu.Lock()
	acc.isActive = true
	acc.metrics.StartTime = time.Now()
	acc.mu.Unlock()

	scenarioCtx, cancel := context.WithTimeout(acc.ctx, scenario.Duration)
	defer cancel()

	// Start chaos events
	var wg sync.WaitGroup
	for _, eventConfig := range scenario.Events {
		wg.Add(1)
		go func(config ChaosEventConfig) {
			defer wg.Done()
			acc.executeChaosEvent(scenarioCtx, config)
		}(eventConfig)
	}

	// Wait for scenario completion
	<-scenarioCtx.Done()

	acc.mu.Lock()
	acc.isActive = false
	acc.metrics.EndTime = time.Now()
	acc.mu.Unlock()

	wg.Wait()

	// Run validators
	for _, validator := range scenario.Validators {
		if !validator(acc.metrics) {
			return fmt.Errorf("scenario validation failed for %s", scenario.Name)
		}
	}

	return nil
}

// executeChaosEvent executes a specific chaos event
func (acc *AdvancedChaosController) executeChaosEvent(ctx context.Context, config ChaosEventConfig) {
	ticker := time.NewTicker(config.Delay)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if rand.Float64() < config.Probability {
				acc.executeSpecificChaos(config)
			}
		}
	}
}

// executeSpecificChaos executes a specific type of chaos
func (acc *AdvancedChaosController) executeSpecificChaos(config ChaosEventConfig) {
	acc.mu.Lock()
	defer acc.mu.Unlock()

	switch config.Type {
	case "cascade_failure":
		acc.executeCascadeFailure(config.Parameters)
	case "byzantine_behavior":
		acc.executeByzantineBehavior(config.Parameters)
	case "resource_exhaustion":
		acc.executeResourceExhaustion(config.Parameters)
	case "network_congestion":
		acc.executeNetworkCongestion(config.Parameters)
	case "split_brain":
		acc.executeSplitBrain(config.Parameters)
	}
}

// executeCascadeFailure simulates cascading failures
func (acc *AdvancedChaosController) executeCascadeFailure(params map[string]interface{}) {
	failureCount := 3
	if count, ok := params["failure_count"].(int); ok {
		failureCount = count
	}

	aliveNodes := make([]int, 0)
	for i := 0; i < len(acc.instances); i++ {
		if !acc.killedNodes[i] {
			aliveNodes = append(aliveNodes, i)
		}
	}

	if len(aliveNodes) <= failureCount {
		return
	}

	// Fail nodes in cascade
	for i := 0; i < failureCount && i < len(aliveNodes); i++ {
		nodeIdx := aliveNodes[rand.Intn(len(aliveNodes))]
		if !acc.killedNodes[nodeIdx] {
			acc.instances[nodeIdx].Exchange.Close()
			acc.killedNodes[nodeIdx] = true
			atomic.AddInt64(&acc.metrics.NodeFailures, 1)

			event := ChaosEvent{
				Type:      "CASCADE_FAILURE",
				Timestamp: time.Now(),
				NodeID:    nodeIdx,
				Details:   fmt.Sprintf("Cascade failure killed node %d", nodeIdx),
			}
			acc.events = append(acc.events, event)

			// Remove from alive nodes
			for j, n := range aliveNodes {
				if n == nodeIdx {
					aliveNodes = append(aliveNodes[:j], aliveNodes[j+1:]...)
					break
				}
			}

			time.Sleep(500 * time.Millisecond) // Cascade delay
		}
	}
}

// executeByzantineBehavior simulates byzantine node behavior
func (acc *AdvancedChaosController) executeByzantineBehavior(params map[string]interface{}) {
	byzantineCount := 2
	if count, ok := params["byzantine_count"].(int); ok {
		byzantineCount = count
	}

	aliveNodes := make([]int, 0)
	for i := 0; i < len(acc.instances); i++ {
		if !acc.killedNodes[i] {
			aliveNodes = append(aliveNodes, i)
		}
	}

	if len(aliveNodes) < byzantineCount {
		return
	}

	// Make nodes behave byzantinely
	for i := 0; i < byzantineCount; i++ {
		nodeIdx := aliveNodes[rand.Intn(len(aliveNodes))]

		// Disconnect from random peers
		for j := 0; j < 3 && j < len(acc.instances); j++ {
			targetIdx := rand.Intn(len(acc.instances))
			if targetIdx != nodeIdx && !acc.killedNodes[targetIdx] {
				acc.instances[nodeIdx].Adapter.DisconnectFrom(acc.ctx, acc.instances[targetIdx].Identity.ID())
			}
		}

		event := ChaosEvent{
			Type:      "BYZANTINE_BEHAVIOR",
			Timestamp: time.Now(),
			NodeID:    nodeIdx,
			Details:   fmt.Sprintf("Node %d exhibiting byzantine behavior", nodeIdx),
		}
		acc.events = append(acc.events, event)
	}
}

// executeResourceExhaustion simulates resource exhaustion
func (acc *AdvancedChaosController) executeResourceExhaustion(params map[string]interface{}) {
	// This is a placeholder for resource exhaustion simulation
	// In a real implementation, this would create memory/CPU pressure
	event := ChaosEvent{
		Type:      "RESOURCE_EXHAUSTION",
		Timestamp: time.Now(),
		NodeID:    -1,
		Details:   "Simulated resource exhaustion across cluster",
	}
	acc.events = append(acc.events, event)
}

// executeNetworkCongestion simulates network congestion
func (acc *AdvancedChaosController) executeNetworkCongestion(params map[string]interface{}) {
	// This is a placeholder for network congestion simulation
	// In a real implementation, this would increase network delays
	event := ChaosEvent{
		Type:      "NETWORK_CONGESTION",
		Timestamp: time.Now(),
		NodeID:    -1,
		Details:   "Simulated network congestion",
	}
	acc.events = append(acc.events, event)
}

// executeSplitBrain simulates split-brain scenarios
func (acc *AdvancedChaosController) executeSplitBrain(params map[string]interface{}) {
	if len(acc.instances) < 4 {
		return
	}

	mid := len(acc.instances) / 2

	// Create split-brain partition
	for i := 0; i < mid; i++ {
		for j := mid; j < len(acc.instances); j++ {
			if !acc.killedNodes[i] && !acc.killedNodes[j] {
				acc.instances[i].Adapter.DisconnectFrom(acc.ctx, acc.instances[j].Identity.ID())
				partitionKey := fmt.Sprintf("split-%d-%d", i, j)
				acc.partitions[partitionKey] = true
			}
		}
	}

	atomic.AddInt64(&acc.metrics.NetworkPartitions, 1)

	event := ChaosEvent{
		Type:      "SPLIT_BRAIN",
		Timestamp: time.Now(),
		NodeID:    -1,
		Details:   fmt.Sprintf("Created split-brain partition at mid-point %d", mid),
	}
	acc.events = append(acc.events, event)
}

// GetMetrics returns current chaos metrics
func (acc *AdvancedChaosController) GetMetrics() *ChaosMetrics {
	acc.mu.RLock()
	defer acc.mu.RUnlock()
	return acc.metrics
}

// TestAdvancedChaosEngineering implements advanced chaos engineering tests
func TestAdvancedChaosEngineering(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	t.Run("CascadingFailureScenario", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(20 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(15)
		testBlocks := generateTestBlocks(t, 40)

		// Distribute blocks with high redundancy
		for i, block := range testBlocks {
			for j := 0; j < 4; j++ { // Each block on 4 nodes
				nodeIdx := (i*4 + j) % len(instances)
				err := instances[nodeIdx].Blockstore.Put(ctx, block)
				require.NoError(t, err)
			}
		}

		controller := NewAdvancedChaosController(ctx, instances, testBlocks, net, &ig)

		// Define cascading failure scenario
		scenario := AdvancedChaosScenario{
			Name:     "CascadingFailure",
			Duration: 60 * time.Second,
			Events: []ChaosEventConfig{
				{
					Type:        "cascade_failure",
					Probability: 0.3,
					Delay:       8 * time.Second,
					Parameters:  map[string]interface{}{"failure_count": 3},
				},
			},
			Validators: []ValidationFunc{
				func(metrics *ChaosMetrics) bool {
					return metrics.NodeFailures >= 3
				},
			},
		}

		// Start load testing
		var wg sync.WaitGroup
		loadCtx, loadCancel := context.WithTimeout(ctx, 70*time.Second)
		defer loadCancel()

		for i := 0; i < 200; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)

				atomic.AddInt64(&controller.metrics.TotalRequests, 1)
				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				start := time.Now()
				fetchCtx, fetchCancel := context.WithTimeout(loadCtx, 5*time.Second)
				defer fetchCancel()

				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				latency := time.Since(start)

				if err == nil {
					atomic.AddInt64(&controller.metrics.SuccessfulRequests, 1)
				} else if err == context.DeadlineExceeded {
					atomic.AddInt64(&controller.metrics.TimeoutRequests, 1)
				} else {
					atomic.AddInt64(&controller.metrics.FailedRequests, 1)
				}

				// Update latency metrics (simplified)
				if latency > controller.metrics.MaxLatency {
					controller.metrics.MaxLatency = latency
				}
			}(i)
		}

		// Execute chaos scenario
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := controller.ExecuteScenario(scenario)
			require.NoError(t, err)
		}()

		wg.Wait()

		metrics := controller.GetMetrics()
		successRate := float64(metrics.SuccessfulRequests) / float64(metrics.TotalRequests)

		t.Logf("Cascading Failure Scenario Results:")
		t.Logf("  Total requests: %d", metrics.TotalRequests)
		t.Logf("  Successful: %d", metrics.SuccessfulRequests)
		t.Logf("  Failed: %d", metrics.FailedRequests)
		t.Logf("  Timeouts: %d", metrics.TimeoutRequests)
		t.Logf("  Success rate: %.2f%%", successRate*100)
		t.Logf("  Node failures: %d", metrics.NodeFailures)
		t.Logf("  Max latency: %v", metrics.MaxLatency)

		assert.Greater(t, successRate, 0.3, "System should maintain >30% success rate during cascading failures")
		assert.Greater(t, metrics.NodeFailures, int64(2), "Should have experienced node failures")
		assert.Greater(t, metrics.TotalRequests, int64(150), "Should have processed substantial requests")
	})

	t.Run("ByzantineFaultToleranceScenario", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(25 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(12) // 12 nodes, up to 4 can be byzantine
		testBlocks := generateTestBlocks(t, 30)

		// Distribute blocks across honest nodes
		honestNodes := []int{0, 1, 2, 3, 8, 9, 10, 11} // 8 honest nodes
		for i, block := range testBlocks {
			honestNodeIdx := honestNodes[i%len(honestNodes)]
			err := instances[honestNodeIdx].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		controller := NewAdvancedChaosController(ctx, instances, testBlocks, net, &ig)

		// Define byzantine fault scenario
		scenario := AdvancedChaosScenario{
			Name:     "ByzantineFault",
			Duration: 45 * time.Second,
			Events: []ChaosEventConfig{
				{
					Type:        "byzantine_behavior",
					Probability: 0.4,
					Delay:       5 * time.Second,
					Parameters:  map[string]interface{}{"byzantine_count": 3},
				},
			},
			Validators: []ValidationFunc{
				func(metrics *ChaosMetrics) bool {
					return metrics.TotalRequests > 50
				},
			},
		}

		// Start load testing
		var wg sync.WaitGroup
		loadCtx, loadCancel := context.WithTimeout(ctx, 55*time.Second)
		defer loadCancel()

		for i := 0; i < 120; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1500)) * time.Millisecond)

				atomic.AddInt64(&controller.metrics.TotalRequests, 1)
				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(loadCtx, 4*time.Second)
				defer fetchCancel()

				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&controller.metrics.SuccessfulRequests, 1)
				} else if err == context.DeadlineExceeded {
					atomic.AddInt64(&controller.metrics.TimeoutRequests, 1)
				} else {
					atomic.AddInt64(&controller.metrics.FailedRequests, 1)
				}
			}(i)
		}

		// Execute chaos scenario
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := controller.ExecuteScenario(scenario)
			require.NoError(t, err)
		}()

		wg.Wait()

		metrics := controller.GetMetrics()
		successRate := float64(metrics.SuccessfulRequests) / float64(metrics.TotalRequests)

		t.Logf("Byzantine Fault Tolerance Scenario Results:")
		t.Logf("  Total requests: %d", metrics.TotalRequests)
		t.Logf("  Successful: %d", metrics.SuccessfulRequests)
		t.Logf("  Failed: %d", metrics.FailedRequests)
		t.Logf("  Timeouts: %d", metrics.TimeoutRequests)
		t.Logf("  Success rate: %.2f%%", successRate*100)

		assert.Greater(t, successRate, 0.4, "System should maintain >40% success rate with byzantine faults")
		assert.Greater(t, metrics.TotalRequests, int64(80), "Should have processed substantial requests")
	})

	t.Run("SplitBrainScenario", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(15 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(10)
		testBlocks := generateTestBlocks(t, 25)

		// Distribute blocks across both halves
		for i, block := range testBlocks {
			// Put each block in both halves for redundancy
			err := instances[i%5].Blockstore.Put(ctx, block)
			require.NoError(t, err)
			err = instances[5+(i%5)].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		controller := NewAdvancedChaosController(ctx, instances, testBlocks, net, &ig)

		// Define split-brain scenario
		scenario := AdvancedChaosScenario{
			Name:     "SplitBrain",
			Duration: 40 * time.Second,
			Events: []ChaosEventConfig{
				{
					Type:        "split_brain",
					Probability: 0.5,
					Delay:       10 * time.Second,
					Parameters:  map[string]interface{}{},
				},
			},
			Validators: []ValidationFunc{
				func(metrics *ChaosMetrics) bool {
					return metrics.NetworkPartitions >= 1
				},
			},
		}

		// Start load testing
		var wg sync.WaitGroup
		loadCtx, loadCancel := context.WithTimeout(ctx, 50*time.Second)
		defer loadCancel()

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1200)) * time.Millisecond)

				atomic.AddInt64(&controller.metrics.TotalRequests, 1)
				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(loadCtx, 4*time.Second)
				defer fetchCancel()

				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&controller.metrics.SuccessfulRequests, 1)
				} else if err == context.DeadlineExceeded {
					atomic.AddInt64(&controller.metrics.TimeoutRequests, 1)
				} else {
					atomic.AddInt64(&controller.metrics.FailedRequests, 1)
				}
			}(i)
		}

		// Execute chaos scenario
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := controller.ExecuteScenario(scenario)
			require.NoError(t, err)
		}()

		wg.Wait()

		metrics := controller.GetMetrics()
		successRate := float64(metrics.SuccessfulRequests) / float64(metrics.TotalRequests)

		t.Logf("Split-Brain Scenario Results:")
		t.Logf("  Total requests: %d", metrics.TotalRequests)
		t.Logf("  Successful: %d", metrics.SuccessfulRequests)
		t.Logf("  Failed: %d", metrics.FailedRequests)
		t.Logf("  Timeouts: %d", metrics.TimeoutRequests)
		t.Logf("  Success rate: %.2f%%", successRate*100)
		t.Logf("  Network partitions: %d", metrics.NetworkPartitions)

		assert.Greater(t, successRate, 0.35, "System should maintain >35% success rate during split-brain")
		assert.Greater(t, metrics.NetworkPartitions, int64(0), "Should have experienced network partitions")
		assert.Greater(t, metrics.TotalRequests, int64(70), "Should have processed substantial requests")
	})
}

// TestCircuitBreakerChaosIntegration tests circuit breaker behavior under chaos
func TestCircuitBreakerChaosIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t.Run("CircuitBreakerWithChaos", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(20 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(8)
		testBlocks := generateTestBlocks(t, 20)

		// Add blocks to instances
		for i, block := range testBlocks {
			err := instances[i%4].Blockstore.Put(ctx, block)
			require.NoError(t, err)
		}

		// Create circuit breaker
		config := circuitbreaker.Config{
			MaxRequests: 5,
			Interval:    2 * time.Second,
			Timeout:     3 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
		}
		bcb := circuitbreaker.NewBitswapCircuitBreaker(config)

		controller := NewAdvancedChaosController(ctx, instances, testBlocks, net, &ig)

		// Define chaos scenario
		scenario := AdvancedChaosScenario{
			Name:     "CircuitBreakerChaos",
			Duration: 30 * time.Second,
			Events: []ChaosEventConfig{
				{
					Type:        "cascade_failure",
					Probability: 0.4,
					Delay:       6 * time.Second,
					Parameters:  map[string]interface{}{"failure_count": 2},
				},
			},
		}

		// Start chaos scenario
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			controller.ExecuteScenario(scenario)
		}()

		// Test circuit breaker behavior under chaos
		var cbSuccessful, cbRejected, cbFailed int64

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(800)) * time.Millisecond)

				nodeIdx := rand.Intn(len(instances))
				peerID := instances[nodeIdx].Identity.ID()

				err := bcb.ExecuteWithPeer(peerID, func() error {
					blockIdx := rand.Intn(len(testBlocks))
					fetchCtx, fetchCancel := context.WithTimeout(ctx, 3*time.Second)
					defer fetchCancel()

					_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
					return err
				})

				if err == nil {
					atomic.AddInt64(&cbSuccessful, 1)
				} else if err == circuitbreaker.ErrOpenState || err == circuitbreaker.ErrTooManyRequests {
					atomic.AddInt64(&cbRejected, 1)
					atomic.AddInt64(&controller.metrics.CircuitBreakerTrips, 1)
				} else {
					atomic.AddInt64(&cbFailed, 1)
				}
			}(i)
		}

		wg.Wait()

		metrics := controller.GetMetrics()
		t.Logf("Circuit Breaker + Chaos Results:")
		t.Logf("  Successful: %d", cbSuccessful)
		t.Logf("  Rejected by CB: %d", cbRejected)
		t.Logf("  Failed: %d", cbFailed)
		t.Logf("  CB trips: %d", metrics.CircuitBreakerTrips)
		t.Logf("  Node failures: %d", metrics.NodeFailures)

		// Circuit breaker should protect the system during chaos
		assert.Greater(t, cbSuccessful, int64(15), "Should have successful requests")
		assert.Greater(t, cbRejected, int64(10), "Circuit breaker should reject some requests")
		assert.Greater(t, metrics.NodeFailures, int64(1), "Should have experienced node failures")
	})
}

// TestStabilityUnderContinuousChaos tests system stability under continuous chaos
func TestStabilityUnderContinuousChaos(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Run("ContinuousChaosStability", func(t *testing.T) {
		net := tn.VirtualNetwork(delay.Fixed(30 * time.Millisecond))
		router := mockrouting.NewServer()
		ig := testinstance.NewTestInstanceGenerator(net, router, nil, nil)
		defer ig.Close()

		instances := ig.Instances(20) // Larger cluster for stability testing
		testBlocks := generateTestBlocks(t, 50)

		// Distribute blocks with high redundancy
		for i, block := range testBlocks {
			for j := 0; j < 5; j++ { // Each block on 5 nodes
				nodeIdx := (i*5 + j) % len(instances)
				err := instances[nodeIdx].Blockstore.Put(ctx, block)
				require.NoError(t, err)
			}
		}

		controller := NewAdvancedChaosController(ctx, instances, testBlocks, net, &ig)

		// Define continuous chaos scenario
		scenario := AdvancedChaosScenario{
			Name:     "ContinuousChaos",
			Duration: 80 * time.Second,
			Events: []ChaosEventConfig{
				{
					Type:        "cascade_failure",
					Probability: 0.2,
					Delay:       10 * time.Second,
					Parameters:  map[string]interface{}{"failure_count": 2},
				},
				{
					Type:        "byzantine_behavior",
					Probability: 0.15,
					Delay:       8 * time.Second,
					Parameters:  map[string]interface{}{"byzantine_count": 2},
				},
				{
					Type:        "split_brain",
					Probability: 0.1,
					Delay:       15 * time.Second,
					Parameters:  map[string]interface{}{},
				},
			},
			Validators: []ValidationFunc{
				func(metrics *ChaosMetrics) bool {
					successRate := float64(metrics.SuccessfulRequests) / float64(metrics.TotalRequests)
					return successRate > 0.2 // At least 20% success rate
				},
			},
		}

		// Start continuous load testing
		var wg sync.WaitGroup
		loadCtx, loadCancel := context.WithTimeout(ctx, 90*time.Second)
		defer loadCancel()

		// High load with continuous requests
		for i := 0; i < 400; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(3000)) * time.Millisecond)

				atomic.AddInt64(&controller.metrics.TotalRequests, 1)
				nodeIdx := rand.Intn(len(instances))
				blockIdx := rand.Intn(len(testBlocks))

				fetchCtx, fetchCancel := context.WithTimeout(loadCtx, 6*time.Second)
				defer fetchCancel()

				_, err := instances[nodeIdx].Exchange.GetBlock(fetchCtx, testBlocks[blockIdx].Cid())
				if err == nil {
					atomic.AddInt64(&controller.metrics.SuccessfulRequests, 1)
				} else if err == context.DeadlineExceeded {
					atomic.AddInt64(&controller.metrics.TimeoutRequests, 1)
				} else {
					atomic.AddInt64(&controller.metrics.FailedRequests, 1)
				}
			}(i)
		}

		// Execute continuous chaos scenario
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := controller.ExecuteScenario(scenario)
			require.NoError(t, err)
		}()

		wg.Wait()

		metrics := controller.GetMetrics()
		successRate := float64(metrics.SuccessfulRequests) / float64(metrics.TotalRequests)
		duration := metrics.EndTime.Sub(metrics.StartTime)

		t.Logf("Continuous Chaos Stability Results:")
		t.Logf("  Duration: %v", duration)
		t.Logf("  Total requests: %d", metrics.TotalRequests)
		t.Logf("  Successful: %d", metrics.SuccessfulRequests)
		t.Logf("  Failed: %d", metrics.FailedRequests)
		t.Logf("  Timeouts: %d", metrics.TimeoutRequests)
		t.Logf("  Success rate: %.2f%%", successRate*100)
		t.Logf("  Node failures: %d", metrics.NodeFailures)
		t.Logf("  Network partitions: %d", metrics.NetworkPartitions)

		// System should maintain stability under continuous chaos
		assert.Greater(t, successRate, 0.2, "System should maintain >20% success rate under continuous chaos")
		assert.Greater(t, metrics.TotalRequests, int64(300), "Should have processed substantial requests")
		assert.Greater(t, metrics.NodeFailures, int64(3), "Should have experienced multiple node failures")
		assert.Greater(t, duration, 70*time.Second, "Should have run for expected duration")
	})
}
