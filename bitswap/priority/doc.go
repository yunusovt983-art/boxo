// Package priority provides request prioritization and resource management
// for Bitswap to handle high-load scenarios efficiently.
//
// This package implements a comprehensive priority request management system
// that classifies incoming requests based on urgency, deadline constraints,
// retry counts, and user-specified priorities. It manages multiple priority
// queues and dynamically allocates resources to ensure critical requests
// are processed first while maintaining overall system throughput.
//
// Key Features:
//
// - Multi-level priority classification (Critical, High, Normal, Low)
// - Dynamic resource allocation with bandwidth and concurrency limits
// - Integration with adaptive configuration for automatic tuning
// - Comprehensive metrics collection and monitoring
// - Circuit breaker patterns for overload protection
//
// Usage:
//
//	// Create a priority request manager
//	prm := priority.NewPriorityRequestManager(
//		100,                    // max concurrent requests
//		10*1024*1024,          // max bandwidth per second (10MB)
//		50*time.Millisecond,   // high priority threshold
//		10*time.Millisecond,   // critical priority threshold
//	)
//
//	// Start processing
//	ctx := context.Background()
//	prm.Start(ctx)
//	defer prm.Stop()
//
//	// Enqueue a request
//	reqCtx := priority.RequestContext{
//		CID:         someCID,
//		PeerID:      somePeerID,
//		RequestTime: time.Now(),
//		Deadline:    time.Now().Add(100 * time.Millisecond),
//		BlockSize:   1024,
//	}
//	err := prm.EnqueueRequest(reqCtx)
//
// For adaptive behavior, use the AdaptivePriorityManager:
//
//	adaptiveConfig := adaptive.NewAdaptiveBitswapConfig()
//	apm := priority.NewAdaptivePriorityManager(adaptiveConfig, 30*time.Second)
//	apm.Start()
//	defer apm.Stop()
//
// Priority Classification:
//
// Requests are automatically classified based on several factors:
//
// - Deadline urgency: Requests with tight deadlines get higher priority
// - Retry count: Requests that have been retried multiple times get elevated priority
// - Session context: Session-based requests get moderate priority boost
// - User specification: Explicit priority can override automatic classification
//
// Resource Management:
//
// The system manages two types of resources:
//
// - Concurrent request slots: Limits the number of simultaneous requests
// - Bandwidth allocation: Limits the total data transfer rate per second
//
// Both limits are enforced to prevent system overload while ensuring
// fair resource distribution across priority levels.
//
// Metrics and Monitoring:
//
// The package provides comprehensive metrics including:
//
// - Request processing statistics (throughput, latency, success rates)
// - Priority distribution and wait times
// - Resource utilization and queue depths
// - Adaptive configuration changes over time
//
// Integration with Adaptive Configuration:
//
// When used with the adaptive package, the priority manager automatically
// adjusts its configuration based on system load:
//
// - Scales worker pools up/down based on demand
// - Adjusts priority thresholds based on response times
// - Modifies resource limits based on system capacity
//
// This ensures optimal performance across varying load conditions while
// maintaining system stability and responsiveness.
package priority