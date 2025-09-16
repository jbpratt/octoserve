package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/jbpratt/octoserve/pkg/types"
)

// PeerStore extends the base Store interface with distributed operations
type PeerStore interface {
	Store
	DistributedOperations
}

// DistributedOperations defines peer-to-peer operations for distributed storage
type DistributedOperations interface {
	// Peer management
	JoinCluster(ctx context.Context, peers []string) error
	LeaveCluster(ctx context.Context) error
	GetPeers(ctx context.Context) ([]PeerInfo, error)

	// Distributed blob operations
	GetBlobFromPeer(ctx context.Context, peerID, digest string) (io.ReadCloser, error)
	FindPeersWithBlob(ctx context.Context, digest string) ([]PeerInfo, error)
	ReplicateBlob(ctx context.Context, digest string, targetPeers []string) error

	// Health checking
	PingPeer(ctx context.Context, peerID string) error
	GetPeerHealth(ctx context.Context) map[string]PeerHealth

	// Node information
	GetNodeID() string
	IsHealthy() bool
}

// NodeDiscovery handles peer discovery mechanisms
type NodeDiscovery interface {
	Discover(ctx context.Context) ([]PeerInfo, error)
	Announce(ctx context.Context, info PeerInfo) error
	Watch(ctx context.Context) <-chan []PeerInfo
	Close() error
}

// Transport handles communication between peers
type Transport interface {
	Start(ctx context.Context) error
	SendBlob(ctx context.Context, peer PeerInfo, digest string, data io.Reader) error
	RequestBlob(ctx context.Context, peer PeerInfo, digest string) (io.ReadCloser, error)
	HasBlob(ctx context.Context, peer PeerInfo, digest string) (bool, int64, error)
	SendManifest(ctx context.Context, peer PeerInfo, repo, ref string, manifest *types.Manifest) error
	RequestManifest(ctx context.Context, peer PeerInfo, repo, ref string) (*types.Manifest, error)
	HasManifest(ctx context.Context, peer PeerInfo, repo, ref string) (bool, string, int64, error)
	Ping(ctx context.Context, peer PeerInfo) error
	Close() error
}

// PeerInfo contains information about a peer node
type PeerInfo struct {
	ID       string            `json:"id"`
	Address  string            `json:"address"`
	Port     int               `json:"port"`
	Metadata map[string]string `json:"metadata"`
	LastSeen time.Time         `json:"last_seen"`
}

// String returns a string representation of the peer
func (p PeerInfo) String() string {
	return p.ID + "@" + p.Address + ":" + fmt.Sprintf("%d", p.Port)
}

// Addr returns the network address of the peer
func (p PeerInfo) Addr() string {
	return p.Address + ":" + fmt.Sprintf("%d", p.Port)
}

// PeerHealth represents the health status of a peer
type PeerHealth struct {
	Status    HealthStatus  `json:"status"`
	Latency   time.Duration `json:"latency"`
	LastCheck time.Time     `json:"last_check"`
	Error     string        `json:"error,omitempty"`
}

// HealthStatus represents the health state of a peer
type HealthStatus int

const (
	HealthUnknown HealthStatus = iota
	HealthHealthy
	HealthDegraded
	HealthUnhealthy
)

// String returns a string representation of the health status
func (h HealthStatus) String() string {
	switch h {
	case HealthHealthy:
		return "healthy"
	case HealthDegraded:
		return "degraded"
	case HealthUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// ReplicationStrategy defines how content is replicated across peers
type ReplicationStrategy int

const (
	ReplicationEager  ReplicationStrategy = iota // Replicate immediately
	ReplicationLazy                              // Replicate on demand
	ReplicationHybrid                            // Smart replication based on content
)

// String returns a string representation of the replication strategy
func (r ReplicationStrategy) String() string {
	switch r {
	case ReplicationEager:
		return "eager"
	case ReplicationLazy:
		return "lazy"
	case ReplicationHybrid:
		return "hybrid"
	default:
		return "unknown"
	}
}

// DistributedError represents an error in distributed operations
type DistributedError struct {
	Op        string
	PeerID    string
	NodeID    string
	Err       error
	Retryable bool
}

func (e *DistributedError) Error() string {
	if e.PeerID != "" {
		return "p2p " + e.Op + " failed on peer " + e.PeerID + ": " + e.Err.Error()
	}
	return "p2p " + e.Op + " failed: " + e.Err.Error()
}

func (e *DistributedError) Unwrap() error {
	return e.Err
}

func (e *DistributedError) IsRetryable() bool {
	return e.Retryable
}

// Common distributed errors
var (
	ErrPeerUnreachable   = &DistributedError{Op: "peer_connect", Retryable: true}
	ErrBlobNotReplicated = &DistributedError{Op: "blob_replicate", Retryable: true}
	ErrConsensusFailure  = &DistributedError{Op: "consensus", Retryable: false}
	ErrNetworkPartition  = &DistributedError{Op: "network", Retryable: true}
	ErrNodeNotFound      = &DistributedError{Op: "node_lookup", Retryable: false}
	ErrInvalidPeer       = &DistributedError{Op: "peer_validation", Retryable: false}
)
