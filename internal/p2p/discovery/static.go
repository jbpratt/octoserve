package discovery

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/jbpratt/octoserve/internal/storage"
)

// StaticDiscovery implements static peer discovery using a predefined list of peers
type StaticDiscovery struct {
	peers   []storage.PeerInfo
	nodeID  string
	watchCh chan []storage.PeerInfo
	stopCh  chan struct{}
	closed  bool
}

// NewStaticDiscovery creates a new static discovery instance
func NewStaticDiscovery(bootstrapPeers []string, nodeID string) *StaticDiscovery {
	peers := make([]storage.PeerInfo, 0, len(bootstrapPeers))

	for _, peerAddr := range bootstrapPeers {
		if peerAddr == "" {
			continue
		}

		peerInfo, err := parsePeerAddress(peerAddr)
		if err != nil {
			// Log error but continue with other peers
			continue
		}

		peers = append(peers, peerInfo)
	}

	return &StaticDiscovery{
		peers:   peers,
		nodeID:  nodeID,
		watchCh: make(chan []storage.PeerInfo, 1),
		stopCh:  make(chan struct{}),
	}
}

// Discover returns the list of configured peers
func (s *StaticDiscovery) Discover(ctx context.Context) ([]storage.PeerInfo, error) {
	if s.closed {
		return nil, fmt.Errorf("discovery is closed")
	}

	// Update last seen timestamp for all peers
	peers := make([]storage.PeerInfo, len(s.peers))
	for i, peer := range s.peers {
		peer.LastSeen = time.Now()
		peers[i] = peer
	}

	return peers, nil
}

// Announce is a no-op for static discovery since peers are predefined
func (s *StaticDiscovery) Announce(ctx context.Context, info storage.PeerInfo) error {
	// Static discovery doesn't support dynamic announcement
	return nil
}

// Watch returns a channel that receives peer updates
func (s *StaticDiscovery) Watch(ctx context.Context) <-chan []storage.PeerInfo {
	// For static discovery, we only send the initial list
	go func() {
		defer close(s.watchCh)

		peers, err := s.Discover(ctx)
		if err != nil {
			return
		}

		select {
		case s.watchCh <- peers:
		case <-ctx.Done():
		case <-s.stopCh:
		}
	}()

	return s.watchCh
}

// Close closes the discovery instance
func (s *StaticDiscovery) Close() error {
	if s.closed {
		return nil
	}

	s.closed = true
	close(s.stopCh)
	return nil
}

// parsePeerAddress parses a peer address string into PeerInfo
// Supports formats:
// - "hostname:port"
// - "ip:port"
// - "node-id@hostname:port"
func parsePeerAddress(addr string) (storage.PeerInfo, error) {
	var peerID string
	var hostPort string

	// Check if address contains node ID
	if strings.Contains(addr, "@") {
		parts := strings.SplitN(addr, "@", 2)
		if len(parts) != 2 {
			return storage.PeerInfo{}, fmt.Errorf("invalid peer address format: %s", addr)
		}
		peerID = parts[0]
		hostPort = parts[1]
	} else {
		hostPort = addr
		// Generate ID from address if not provided
		peerID = generatePeerIDFromAddress(hostPort)
	}

	// Parse host and port
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		return storage.PeerInfo{}, fmt.Errorf("invalid host:port format: %s", hostPort)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return storage.PeerInfo{}, fmt.Errorf("invalid port: %s", portStr)
	}

	if port < 1 || port > 65535 {
		return storage.PeerInfo{}, fmt.Errorf("port out of range: %d", port)
	}

	return storage.PeerInfo{
		ID:       peerID,
		Address:  host,
		Port:     port,
		Metadata: make(map[string]string),
		LastSeen: time.Now(),
	}, nil
}

// generatePeerIDFromAddress generates a peer ID from an address
func generatePeerIDFromAddress(addr string) string {
	// Simple approach: use the address itself as ID
	// In production, you might want to use a hash or other unique identifier
	return strings.ReplaceAll(addr, ":", "_")
}
