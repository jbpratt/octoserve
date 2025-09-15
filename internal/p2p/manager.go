package p2p

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jbpratt/octoserve/internal/config"
	"github.com/jbpratt/octoserve/internal/p2p/discovery"
	"github.com/jbpratt/octoserve/internal/p2p/transport"
	"github.com/jbpratt/octoserve/internal/storage"
)

// Manager coordinates P2P operations including discovery, health checking, and replication
type Manager struct {
	nodeID    string
	config    config.P2PConfig
	store     storage.Store
	transport storage.Transport
	discovery storage.NodeDiscovery
	logger    *slog.Logger

	// Peer management
	peers     map[string]storage.PeerInfo
	peerMutex sync.RWMutex

	// Health tracking
	healthStatus map[string]storage.PeerHealth
	healthMutex  sync.RWMutex

	// Lifecycle management
	started    bool
	stopCh     chan struct{}
	workerWg   sync.WaitGroup
	startMutex sync.Mutex
}

// NewManager creates a new P2P manager
func NewManager(cfg config.P2PConfig, store storage.Store, logger *slog.Logger) (*Manager, error) {
	// Generate node ID if not provided
	nodeID := cfg.NodeID
	if nodeID == "" {
		nodeID = uuid.New().String()
	}

	// Create transport
	grpcTransport := transport.NewGRPCTransport(nodeID, cfg.Port, store)

	// Create discovery
	var nodeDiscovery storage.NodeDiscovery
	switch cfg.Discovery.Method {
	case "static":
		nodeDiscovery = discovery.NewStaticDiscovery(cfg.BootstrapPeers, nodeID)
	default:
		return nil, fmt.Errorf("unsupported discovery method: %s", cfg.Discovery.Method)
	}

	return &Manager{
		nodeID:       nodeID,
		config:       cfg,
		store:        store,
		transport:    grpcTransport,
		discovery:    nodeDiscovery,
		logger:       logger,
		peers:        make(map[string]storage.PeerInfo),
		healthStatus: make(map[string]storage.PeerHealth),
		stopCh:       make(chan struct{}),
	}, nil
}

// Start starts the P2P manager and all its background processes
func (m *Manager) Start(ctx context.Context) error {
	m.startMutex.Lock()
	defer m.startMutex.Unlock()

	if m.started {
		return fmt.Errorf("P2P manager already started")
	}

	m.logger.Info("Starting P2P manager", "node_id", m.nodeID)

	// Start transport
	if err := m.transport.Start(ctx); err != nil {
		return fmt.Errorf("failed to start transport: %w", err)
	}

	// Start background workers
	m.workerWg.Add(3)
	go m.discoveryLoop(ctx)
	go m.healthCheckLoop(ctx)
	go m.maintenanceLoop(ctx)

	m.started = true
	m.logger.Info("P2P manager started successfully")

	return nil
}

// Stop stops the P2P manager and all background processes
func (m *Manager) Stop() error {
	m.startMutex.Lock()
	defer m.startMutex.Unlock()

	if !m.started {
		return nil
	}

	m.logger.Info("Stopping P2P manager")

	// Signal all workers to stop
	close(m.stopCh)

	// Wait for workers to finish
	m.workerWg.Wait()

	// Stop transport
	if err := m.transport.Close(); err != nil {
		m.logger.Error("Error stopping transport", "error", err)
	}

	// Stop discovery
	if err := m.discovery.Close(); err != nil {
		m.logger.Error("Error stopping discovery", "error", err)
	}

	m.started = false
	m.logger.Info("P2P manager stopped")

	return nil
}

// GetNodeID returns the current node's ID
func (m *Manager) GetNodeID() string {
	return m.nodeID
}

// IsHealthy returns whether the P2P manager is healthy
func (m *Manager) IsHealthy() bool {
	return m.started
}

// GetPeers returns a list of known peers
func (m *Manager) GetPeers(ctx context.Context) ([]storage.PeerInfo, error) {
	m.peerMutex.RLock()
	defer m.peerMutex.RUnlock()

	peers := make([]storage.PeerInfo, 0, len(m.peers))
	for _, peer := range m.peers {
		peers = append(peers, peer)
	}

	return peers, nil
}

// GetPeerHealth returns the health status of all peers
func (m *Manager) GetPeerHealth(ctx context.Context) map[string]storage.PeerHealth {
	m.healthMutex.RLock()
	defer m.healthMutex.RUnlock()

	// Return a copy of the health status map
	result := make(map[string]storage.PeerHealth)
	for id, health := range m.healthStatus {
		result[id] = health
	}

	return result
}

// FindPeersWithBlob finds peers that have a specific blob
func (m *Manager) FindPeersWithBlob(ctx context.Context, digest string) ([]storage.PeerInfo, error) {
	m.peerMutex.RLock()
	peers := make([]storage.PeerInfo, 0, len(m.peers))
	for _, peer := range m.peers {
		peers = append(peers, peer)
	}
	m.peerMutex.RUnlock()

	var peersWithBlob []storage.PeerInfo

	// Check each peer concurrently
	type result struct {
		peer storage.PeerInfo
		has  bool
	}

	results := make(chan result, len(peers))

	for _, peer := range peers {
		go func(p storage.PeerInfo) {
			defer func() {
				if r := recover(); r != nil {
					results <- result{peer: p, has: false}
				}
			}()

			// For now, we'll use a simple ping to check if peer is available
			// In a full implementation, we'd call HasBlob on the peer
			err := m.transport.Ping(ctx, p)
			results <- result{peer: p, has: err == nil}
		}(peer)
	}

	// Collect results
	for i := 0; i < len(peers); i++ {
		select {
		case res := <-results:
			if res.has {
				peersWithBlob = append(peersWithBlob, res.peer)
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return peersWithBlob, nil
}

// discoveryLoop handles peer discovery
func (m *Manager) discoveryLoop(ctx context.Context) {
	defer m.workerWg.Done()

	ticker := time.NewTicker(m.config.Discovery.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			if err := m.discoverPeers(ctx); err != nil {
				m.logger.Error("Peer discovery failed", "error", err)
			}
		}
	}
}

// healthCheckLoop handles periodic health checking of peers
func (m *Manager) healthCheckLoop(ctx context.Context) {
	defer m.workerWg.Done()

	ticker := time.NewTicker(m.config.HealthCheck.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkPeerHealth(ctx)
		}
	}
}

// maintenanceLoop handles periodic maintenance tasks
func (m *Manager) maintenanceLoop(ctx context.Context) {
	defer m.workerWg.Done()

	ticker := time.NewTicker(5 * time.Minute) // Run maintenance every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.performMaintenance(ctx)
		}
	}
}

// discoverPeers discovers new peers using the configured discovery method
func (m *Manager) discoverPeers(ctx context.Context) error {
	discoveredPeers, err := m.discovery.Discover(ctx)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	m.peerMutex.Lock()
	defer m.peerMutex.Unlock()

	for _, peer := range discoveredPeers {
		// Don't add ourselves
		if peer.ID == m.nodeID {
			continue
		}

		// Add or update peer
		peer.LastSeen = time.Now()
		m.peers[peer.ID] = peer

		m.logger.Debug("Discovered peer", "peer_id", peer.ID, "address", peer.Addr())
	}

	return nil
}

// checkPeerHealth checks the health of all known peers
func (m *Manager) checkPeerHealth(ctx context.Context) {
	m.peerMutex.RLock()
	peers := make([]storage.PeerInfo, 0, len(m.peers))
	for _, peer := range m.peers {
		peers = append(peers, peer)
	}
	m.peerMutex.RUnlock()

	// Use worker pool for health checks
	type healthResult struct {
		peerID string
		health storage.PeerHealth
	}

	results := make(chan healthResult, len(peers))

	// Limit concurrent health checks
	semaphore := make(chan struct{}, m.config.HealthCheck.Workers)

	for _, peer := range peers {
		go func(p storage.PeerInfo) {
			defer func() {
				if r := recover(); r != nil {
					results <- healthResult{
						peerID: p.ID,
						health: storage.PeerHealth{
							Status:    storage.HealthUnhealthy,
							LastCheck: time.Now(),
							Error:     fmt.Sprintf("panic: %v", r),
						},
					}
				}
			}()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Perform health check with timeout
			checkCtx, cancel := context.WithTimeout(ctx, m.config.HealthCheck.Timeout)
			defer cancel()

			start := time.Now()
			err := m.transport.Ping(checkCtx, p)
			latency := time.Since(start)

			health := storage.PeerHealth{
				LastCheck: time.Now(),
				Latency:   latency,
			}

			if err != nil {
				health.Status = storage.HealthUnhealthy
				health.Error = err.Error()
			} else {
				health.Status = storage.HealthHealthy
			}

			results <- healthResult{peerID: p.ID, health: health}
		}(peer)
	}

	// Collect results
	m.healthMutex.Lock()
	defer m.healthMutex.Unlock()

	for i := 0; i < len(peers); i++ {
		select {
		case res := <-results:
			m.healthStatus[res.peerID] = res.health
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		}
	}
}

// performMaintenance performs periodic maintenance tasks
func (m *Manager) performMaintenance(ctx context.Context) {
	// Remove stale peers
	m.removeStaleTimers()

	// Log cluster status
	m.logClusterStatus()
}

// removeStaleTimers removes peers that haven't been seen recently
func (m *Manager) removeStaleTimers() {
	staleThreshold := time.Now().Add(-10 * time.Minute) // 10 minutes

	m.peerMutex.Lock()
	defer m.peerMutex.Unlock()

	for id, peer := range m.peers {
		if peer.LastSeen.Before(staleThreshold) {
			delete(m.peers, id)
			m.logger.Debug("Removed stale peer", "peer_id", id)
		}
	}

	// Also clean up health status for removed peers
	m.healthMutex.Lock()
	defer m.healthMutex.Unlock()

	for id := range m.healthStatus {
		if _, exists := m.peers[id]; !exists {
			delete(m.healthStatus, id)
		}
	}
}

// logClusterStatus logs the current status of the P2P cluster
func (m *Manager) logClusterStatus() {
	m.peerMutex.RLock()
	peerCount := len(m.peers)
	m.peerMutex.RUnlock()

	m.healthMutex.RLock()
	healthyCount := 0
	for _, health := range m.healthStatus {
		if health.Status == storage.HealthHealthy {
			healthyCount++
		}
	}
	m.healthMutex.RUnlock()

	m.logger.Info("Cluster status",
		"total_peers", peerCount,
		"healthy_peers", healthyCount,
		"node_id", m.nodeID)
}
