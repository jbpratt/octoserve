package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/jbpratt/octoserve/internal/storage"
)

func TestStaticDiscovery(t *testing.T) {
	tests := []struct {
		name           string
		bootstrapPeers []string
		nodeID         string
		expectedPeers  int
		expectError    bool
	}{
		{
			name:           "valid peers",
			bootstrapPeers: []string{"peer1:6000", "peer2:6000", "peer3@peer3.example.com:6000"},
			nodeID:         "test-node",
			expectedPeers:  3,
			expectError:    false,
		},
		{
			name:           "empty peers",
			bootstrapPeers: []string{},
			nodeID:         "test-node",
			expectedPeers:  0,
			expectError:    false,
		},
		{
			name:           "invalid peer format",
			bootstrapPeers: []string{"peer1:6000", "invalid-peer", "peer3:6000"},
			nodeID:         "test-node",
			expectedPeers:  2, // Should skip invalid peer
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discovery := NewStaticDiscovery(tt.bootstrapPeers, tt.nodeID)
			defer discovery.Close()

			peers, err := discovery.Discover(context.Background())

			if tt.expectError && err == nil {
				t.Fatalf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(peers) != tt.expectedPeers {
				t.Fatalf("expected %d peers, got %d", tt.expectedPeers, len(peers))
			}

			// Verify peer info
			for _, peer := range peers {
				if peer.ID == "" {
					t.Error("peer ID should not be empty")
				}
				if peer.Address == "" {
					t.Error("peer address should not be empty")
				}
				if peer.Port <= 0 || peer.Port > 65535 {
					t.Errorf("invalid peer port: %d", peer.Port)
				}
				if peer.LastSeen.IsZero() {
					t.Error("peer LastSeen should be set")
				}
			}
		})
	}
}

func TestParsePeerAddress(t *testing.T) {
	tests := []struct {
		name         string
		addr         string
		expectedID   string
		expectedAddr string
		expectedPort int
		expectError  bool
	}{
		{
			name:         "hostname with port",
			addr:         "example.com:6000",
			expectedID:   "example.com_6000",
			expectedAddr: "example.com",
			expectedPort: 6000,
			expectError:  false,
		},
		{
			name:         "IP with port",
			addr:         "192.168.1.100:6000",
			expectedID:   "192.168.1.100_6000",
			expectedAddr: "192.168.1.100",
			expectedPort: 6000,
			expectError:  false,
		},
		{
			name:         "with node ID",
			addr:         "node-123@example.com:6000",
			expectedID:   "node-123",
			expectedAddr: "example.com",
			expectedPort: 6000,
			expectError:  false,
		},
		{
			name:         "invalid format - no port",
			addr:         "example.com",
			expectedID:   "",
			expectedAddr: "",
			expectedPort: 0,
			expectError:  true,
		},
		{
			name:         "invalid port",
			addr:         "example.com:invalid",
			expectedID:   "",
			expectedAddr: "",
			expectedPort: 0,
			expectError:  true,
		},
		{
			name:         "port out of range",
			addr:         "example.com:70000",
			expectedID:   "",
			expectedAddr: "",
			expectedPort: 0,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peer, err := parsePeerAddress(tt.addr)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if peer.ID != tt.expectedID {
				t.Errorf("expected ID %s, got %s", tt.expectedID, peer.ID)
			}
			if peer.Address != tt.expectedAddr {
				t.Errorf("expected address %s, got %s", tt.expectedAddr, peer.Address)
			}
			if peer.Port != tt.expectedPort {
				t.Errorf("expected port %d, got %d", tt.expectedPort, peer.Port)
			}
		})
	}
}

func TestStaticDiscoveryWatch(t *testing.T) {
	discovery := NewStaticDiscovery([]string{"peer1:6000", "peer2:6000"}, "test-node")
	defer discovery.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	watchCh := discovery.Watch(ctx)

	select {
	case peers := <-watchCh:
		if len(peers) != 2 {
			t.Fatalf("expected 2 peers, got %d", len(peers))
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for peers")
	}
}

func TestStaticDiscoveryAnnounce(t *testing.T) {
	discovery := NewStaticDiscovery([]string{}, "test-node")
	defer discovery.Close()

	// Create a test peer info
	testPeer := storage.PeerInfo{
		ID:      "new-peer",
		Address: "new-peer",
		Port:    6000,
	}

	// Announce should be a no-op for static discovery
	err := discovery.Announce(context.Background(), testPeer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
