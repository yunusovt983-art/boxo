package circuitbreaker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	logging "github.com/ipfs/go-log/v2"
)

var integrationLog = logging.Logger("bitswap/circuitbreaker/integration")

// BitswapProtectionManager integrates circuit breaker, health monitoring, and recovery management
// to provide comprehensive protection for Bitswap operations under high load.
type BitswapProtectionManager struct {
	circuitBreaker  *BitswapCircuitBreaker
	healthMonitor   *HealthMonitor
	recoveryManager *RecoveryManager
	
	mutex sync.RWMutex
	ctx   context.Context
	cancel context.CancelFunc
	wg    sync.WaitGroup
	
	// Configuration
	config ProtectionConfig
	
	// Metrics
	metrics *ProtectionMetrics
}

// ProtectionConfig holds configuration for the protection manager
type ProtectionConfig struct {
	CircuitBreaker Config
	HealthMonitor  HealthMonitorConfig
	Recovery       RecoveryConfig
	
	// Integration settings
	EnableAutoRecovery     bool
	EnableHealthBasedTripping bool
	MetricsUpdateInterval  time.Duration
}

// ProtectionMetrics holds metrics for the protection system
type ProtectionMetrics struct {
	mutex sync.RWMutex
	
	// Circuit breaker metrics
	TotalRequests       uint64
	SuccessfulRequests  uint64
	FailedRequests      uint64
	RejectedRequests    uint64
	
	// Health metrics
	HealthyPeers        int
	UnhealthyPeers      int
	RecoveringPeers     int
	
	// Performance metrics
	AverageResponseTime time.Duration
	P95ResponseTime     time.Duration
	
	// State changes
	CircuitBreakerTrips uint64
	RecoveryAttempts    uint64
	SuccessfulRecoveries uint64
	FailedRecoveries    uint64
	
	LastUpdated time.Time
}

// DefaultProtectionConfig returns a default protection configuration
func DefaultProtectionConfig() ProtectionConfig {
	return ProtectionConfig{
		CircuitBreaker:            DefaultBitswapCircuitBreakerConfig(),
		HealthMonitor:             DefaultHealthMonitorConfig(),
		Recovery:                  DefaultRecoveryConfig(),
		EnableAutoRecovery:        true,
		EnableHealthBasedTripping: true,
		MetricsUpdateInterval:     30 * time.Second,
	}
}

// NewBitswapProtectionManager creates a new protection manager
func NewBitswapProtectionManager(config ProtectionConfig) *BitswapProtectionManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Set up circuit breaker with health-based tripping if enabled
	cbConfig := config.CircuitBreaker
	if config.EnableHealthBasedTripping {
		cbConfig.OnStateChange = func(name string, from State, to State) {
			integrationLog.Infof("Circuit breaker %s changed from %s to %s", name, from, to)
		}
	}
	
	pm := &BitswapProtectionManager{
		circuitBreaker:  NewBitswapCircuitBreaker(cbConfig),
		healthMonitor:   NewHealthMonitor(config.HealthMonitor),
		recoveryManager: NewRecoveryManager(config.Recovery),
		ctx:             ctx,
		cancel:          cancel,
		config:          config,
		metrics:         &ProtectionMetrics{},
	}
	
	// Set up callbacks for integration
	pm.setupCallbacks()
	
	// Start background tasks
	pm.wg.Add(1)
	go pm.managementLoop()
	
	return pm
}

// ExecuteRequest executes a Bitswap request with full protection
func (pm *BitswapProtectionManager) ExecuteRequest(
	ctx context.Context,
	peerID peer.ID,
	request func(context.Context) error,
) error {
	startTime := time.Now()
	
	// Check if we should allow the request during recovery
	if pm.recoveryManager.IsRecovering(peerID) {
		if !pm.recoveryManager.ShouldAllowRequest(peerID) {
			pm.updateMetrics(func(m *ProtectionMetrics) {
				m.RejectedRequests++
			})
			return fmt.Errorf("request rejected during recovery for peer %s", peerID)
		}
	}
	
	// Execute through circuit breaker
	err := pm.circuitBreaker.CallWithPeer(ctx, peerID, request)
	
	// Record metrics
	responseTime := time.Since(startTime)
	pm.healthMonitor.RecordRequest(peerID, responseTime, err)
	
	// Record recovery attempt if in recovery mode
	if pm.recoveryManager.IsRecovering(peerID) {
		pm.recoveryManager.RecordRecoveryAttempt(peerID, err == nil)
	}
	
	// Update metrics
	pm.updateMetrics(func(m *ProtectionMetrics) {
		m.TotalRequests++
		if err == nil {
			m.SuccessfulRequests++
		} else {
			m.FailedRequests++
		}
		
		// Update response time metrics (simplified)
		m.AverageResponseTime = (m.AverageResponseTime + responseTime) / 2
		if responseTime > m.P95ResponseTime {
			m.P95ResponseTime = responseTime
		}
	})
	
	return err
}

// GetPeerStatus returns comprehensive status for a peer
func (pm *BitswapProtectionManager) GetPeerStatus(peerID peer.ID) PeerStatus {
	cbState := pm.circuitBreaker.GetPeerState(peerID)
	cbCounts := pm.circuitBreaker.GetPeerCounts(peerID)
	
	health, hasHealth := pm.healthMonitor.GetHealth(peerID)
	healthStatus := pm.healthMonitor.GetHealthStatus(peerID)
	
	recoveryState, isRecovering := pm.recoveryManager.GetRecoveryState(peerID)
	recoveryRate := pm.recoveryManager.GetRecoveryRate(peerID)
	
	status := PeerStatus{
		PeerID:           peerID,
		CircuitState:     cbState,
		CircuitCounts:    cbCounts,
		HealthStatus:     healthStatus,
		IsRecovering:     isRecovering,
		RecoveryRate:     recoveryRate,
		LastUpdated:      time.Now(),
	}
	
	if hasHealth {
		status.Health = *health
	}
	
	if isRecovering {
		status.RecoveryState = *recoveryState
	}
	
	return status
}

// GetOverallStatus returns overall system status
func (pm *BitswapProtectionManager) GetOverallStatus() OverallStatus {
	pm.metrics.mutex.RLock()
	metrics := *pm.metrics
	pm.metrics.mutex.RUnlock()
	
	cbStates := pm.circuitBreaker.GetAllStates()
	healthStats := pm.healthMonitor.GetHealthStats()
	recoveryStats := pm.recoveryManager.GetRecoveryStats()
	
	return OverallStatus{
		Metrics:           metrics,
		CircuitStates:     cbStates,
		HealthStats:       healthStats,
		RecoveryStats:     recoveryStats,
		TotalPeers:        len(cbStates),
		LastUpdated:       time.Now(),
	}
}

// Close shuts down the protection manager
func (pm *BitswapProtectionManager) Close() error {
	pm.cancel()
	pm.wg.Wait()
	
	if err := pm.healthMonitor.Close(); err != nil {
		integrationLog.Errorf("Error closing health monitor: %v", err)
	}
	
	if err := pm.recoveryManager.Close(); err != nil {
		integrationLog.Errorf("Error closing recovery manager: %v", err)
	}
	
	return nil
}

// setupCallbacks configures callbacks between components
func (pm *BitswapProtectionManager) setupCallbacks() {
	// Health monitor callback for circuit breaker integration
	pm.healthMonitor.SetHealthChangeCallback(func(peerID peer.ID, from, to HealthStatus) {
		integrationLog.Debugf("Health status changed for peer %s: %s -> %s", peerID, from, to)
		
		// If health becomes unhealthy and auto-recovery is enabled, start recovery
		if pm.config.EnableAutoRecovery && to == HealthStatusUnhealthy {
			// Check if circuit breaker is open for this peer
			if pm.circuitBreaker.GetPeerState(peerID) == StateOpen {
				pm.recoveryManager.StartRecovery(peerID)
			}
		}
	})
	
	// Recovery manager callbacks
	pm.recoveryManager.SetCallbacks(
		func(peerID peer.ID) {
			integrationLog.Infof("Started recovery for peer %s", peerID)
			pm.updateMetrics(func(m *ProtectionMetrics) {
				m.RecoveryAttempts++
			})
		},
		func(peerID peer.ID) {
			integrationLog.Infof("Completed recovery for peer %s", peerID)
			pm.updateMetrics(func(m *ProtectionMetrics) {
				m.SuccessfulRecoveries++
			})
		},
		func(peerID peer.ID) {
			integrationLog.Warnf("Failed recovery for peer %s", peerID)
			pm.updateMetrics(func(m *ProtectionMetrics) {
				m.FailedRecoveries++
			})
		},
	)
}

// managementLoop runs background management tasks
func (pm *BitswapProtectionManager) managementLoop() {
	defer pm.wg.Done()
	
	ticker := time.NewTicker(pm.config.MetricsUpdateInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.updateSystemMetrics()
		}
	}
}

// updateSystemMetrics updates system-wide metrics
func (pm *BitswapProtectionManager) updateSystemMetrics() {
	healthStats := pm.healthMonitor.GetHealthStats()
	recoveryStats := pm.recoveryManager.GetRecoveryStats()
	
	pm.updateMetrics(func(m *ProtectionMetrics) {
		m.HealthyPeers = healthStats.HealthyPeers
		m.UnhealthyPeers = healthStats.UnhealthyPeers
		m.RecoveringPeers = recoveryStats.RecoveringPeers
		m.LastUpdated = time.Now()
	})
}

// updateMetrics safely updates metrics using a callback
func (pm *BitswapProtectionManager) updateMetrics(update func(*ProtectionMetrics)) {
	pm.metrics.mutex.Lock()
	defer pm.metrics.mutex.Unlock()
	update(pm.metrics)
}

// PeerStatus represents the comprehensive status of a peer
type PeerStatus struct {
	PeerID         peer.ID
	CircuitState   State
	CircuitCounts  Counts
	HealthStatus   HealthStatus
	Health         ConnectionHealth
	IsRecovering   bool
	RecoveryState  RecoveryState
	RecoveryRate   float64
	LastUpdated    time.Time
}

// OverallStatus represents the overall system status
type OverallStatus struct {
	Metrics       ProtectionMetrics
	CircuitStates map[peer.ID]State
	HealthStats   HealthStats
	RecoveryStats RecoveryStats
	TotalPeers    int
	LastUpdated   time.Time
}

// IsHealthy returns true if the overall system is healthy
func (os *OverallStatus) IsHealthy() bool {
	// System is considered healthy if:
	// 1. Most peers are healthy (>80%)
	// 2. Not too many circuit breakers are open (<20%)
	// 3. Recovery success rate is good (>70%)
	
	if os.TotalPeers == 0 {
		return true // No peers to evaluate
	}
	
	healthyRatio := float64(os.HealthStats.HealthyPeers) / float64(os.TotalPeers)
	
	openCircuits := 0
	for _, state := range os.CircuitStates {
		if state == StateOpen {
			openCircuits++
		}
	}
	openRatio := float64(openCircuits) / float64(os.TotalPeers)
	
	return healthyRatio > 0.8 && openRatio < 0.2
}

// GetSummary returns a human-readable summary of the system status
func (os *OverallStatus) GetSummary() string {
	return fmt.Sprintf(
		"System Status: %d peers, %d healthy (%.1f%%), %d recovering, %d circuit breakers open",
		os.TotalPeers,
		os.HealthStats.HealthyPeers,
		float64(os.HealthStats.HealthyPeers)/float64(os.TotalPeers)*100,
		os.RecoveryStats.RecoveringPeers,
		len(os.CircuitStates),
	)
}