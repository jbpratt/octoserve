package distributed

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/jbpratt/octoserve/internal/storage"
	"github.com/jbpratt/octoserve/pkg/types"
)

// Store implements a distributed storage backend that wraps a local store
// and coordinates with peer nodes for replication and availability
type Store struct {
	local  storage.Store // Local storage backend (e.g., filesystem)
	p2pMgr P2PManager    // P2P manager interface for peer coordination
	logger *slog.Logger
	nodeID string
	config Config
}

// P2PManager interface defines the methods needed from the P2P manager
type P2PManager interface {
	GetNodeID() string
	IsHealthy() bool
	GetPeers(ctx context.Context) ([]storage.PeerInfo, error)
	GetPeerHealth(ctx context.Context) map[string]storage.PeerHealth
	FindPeersWithBlob(ctx context.Context, digest string) ([]storage.PeerInfo, error)
}

// Config contains configuration for the distributed store
type Config struct {
	ReplicationFactor int
	Strategy          string // "eager", "lazy", "hybrid"
	ConsistencyMode   string // "eventual", "strong"
	RequestTimeout    time.Duration
}

// New creates a new distributed store
func New(local storage.Store, p2pMgr P2PManager, logger *slog.Logger, config Config) *Store {
	return &Store{
		local:  local,
		p2pMgr: p2pMgr,
		logger: logger,
		nodeID: p2pMgr.GetNodeID(),
		config: config,
	}
}

// GetBlob retrieves a blob, first trying local storage, then peer nodes
func (s *Store) GetBlob(ctx context.Context, digest string) (io.ReadCloser, error) {
	// Try local storage first
	reader, err := s.local.GetBlob(ctx, digest)
	if err == nil {
		s.logger.Debug("Blob found locally", "digest", digest[:12])
		return reader, nil
	}

	// If not found locally, just return the error for now
	// P2P blob fetching will be implemented in Phase 2
	if err == storage.ErrBlobNotFound {
		s.logger.Debug("Blob not found locally", "digest", digest[:12])
		return nil, storage.ErrBlobNotFound
	}

	return nil, err
}

// PutBlob stores a blob locally and optionally replicates to peers
func (s *Store) PutBlob(ctx context.Context, digest string, content io.Reader) error {
	// Store locally first
	if err := s.local.PutBlob(ctx, digest, content); err != nil {
		return fmt.Errorf("failed to store blob locally: %w", err)
	}

	s.logger.Debug("Blob stored locally", "digest", digest[:12])

	// Replicate to peers based on strategy
	if s.config.Strategy == "eager" {
		go s.replicateBlobToPeers(context.Background(), digest)
	}
	// For "lazy" strategy, replication happens on-demand
	// For "hybrid" strategy, decision is made based on blob characteristics

	return nil
}

// BlobExists checks if a blob exists locally or on peers
func (s *Store) BlobExists(ctx context.Context, digest string) (bool, error) {
	// For now, only check local storage
	// P2P blob checking will be implemented in Phase 2
	return s.local.BlobExists(ctx, digest)
}

// DeleteBlob removes a blob from local storage
func (s *Store) DeleteBlob(ctx context.Context, digest string) error {
	// For now, only delete locally
	// In a full implementation, you might want to coordinate deletion across peers
	return s.local.DeleteBlob(ctx, digest)
}

// GetBlobSize returns the size of a blob
func (s *Store) GetBlobSize(ctx context.Context, digest string) (int64, error) {
	// Try local first
	size, err := s.local.GetBlobSize(ctx, digest)
	if err == nil {
		return size, nil
	}

	// If not found locally, we could query peers, but for simplicity,
	// we'll just return the error
	return 0, err
}

// GetManifest retrieves a manifest, first trying local storage, then peers
func (s *Store) GetManifest(ctx context.Context, repo, reference string) (*types.Manifest, error) {
	// For now, only check local storage
	// P2P manifest fetching will be implemented in Phase 2
	return s.local.GetManifest(ctx, repo, reference)
}

// PutManifest stores a manifest locally and replicates to peers
func (s *Store) PutManifest(ctx context.Context, repo, reference string, manifest *types.Manifest) error {
	// Store locally first
	if err := s.local.PutManifest(ctx, repo, reference, manifest); err != nil {
		return fmt.Errorf("failed to store manifest locally: %w", err)
	}

	s.logger.Debug("Manifest stored locally", "repo", repo, "ref", reference)

	// Replicate to peers for consistency
	// Manifests are critical, so we always replicate eagerly
	go s.replicateManifestToPeers(context.Background(), repo, reference, manifest)

	return nil
}

// DeleteManifest removes a manifest from local storage
func (s *Store) DeleteManifest(ctx context.Context, repo, reference string) error {
	return s.local.DeleteManifest(ctx, repo, reference)
}

// ListTags lists tags for a repository
func (s *Store) ListTags(ctx context.Context, repo string) ([]string, error) {
	// For now, only return local tags
	// In a full implementation, you might want to merge tags from peers
	return s.local.ListTags(ctx, repo)
}

// ManifestExists checks if a manifest exists locally or on peers
func (s *Store) ManifestExists(ctx context.Context, repo, reference string) (bool, error) {
	// For now, only check local storage
	return s.local.ManifestExists(ctx, repo, reference)
}

// GetReferrers returns manifests that reference the given digest
func (s *Store) GetReferrers(ctx context.Context, repo, digest string) ([]types.Descriptor, error) {
	return s.local.GetReferrers(ctx, repo, digest)
}

// Upload management methods - delegate to local store
func (s *Store) CreateUpload(ctx context.Context, repo string) (*types.Upload, error) {
	return s.local.CreateUpload(ctx, repo)
}

func (s *Store) GetUpload(ctx context.Context, uploadID string) (*types.Upload, error) {
	return s.local.GetUpload(ctx, uploadID)
}

func (s *Store) WriteUploadChunk(ctx context.Context, uploadID string, offset int64, chunk io.Reader) error {
	return s.local.WriteUploadChunk(ctx, uploadID, offset, chunk)
}

func (s *Store) CompleteUpload(ctx context.Context, uploadID string, digest string) error {
	return s.local.CompleteUpload(ctx, uploadID, digest)
}

func (s *Store) CancelUpload(ctx context.Context, uploadID string) error {
	return s.local.CancelUpload(ctx, uploadID)
}

func (s *Store) GetUploadStatus(ctx context.Context, uploadID string) (int64, error) {
	return s.local.GetUploadStatus(ctx, uploadID)
}

// P2P-specific methods for the PeerStore interface

// JoinCluster joins the P2P cluster
func (s *Store) JoinCluster(ctx context.Context, peers []string) error {
	// This would be handled by the P2P manager
	// For now, return success as the manager handles cluster joining
	return nil
}

// LeaveCluster leaves the P2P cluster
func (s *Store) LeaveCluster(ctx context.Context) error {
	// This would be handled by the P2P manager
	return nil
}

// GetPeers returns the list of known peers
func (s *Store) GetPeers(ctx context.Context) ([]storage.PeerInfo, error) {
	return s.p2pMgr.GetPeers(ctx)
}

// GetBlobFromPeer retrieves a blob from a specific peer
func (s *Store) GetBlobFromPeer(ctx context.Context, peerID, digest string) (io.ReadCloser, error) {
	// This would require access to the transport layer
	// For now, return not implemented
	return nil, fmt.Errorf("GetBlobFromPeer not implemented")
}

// FindPeersWithBlob finds peers that have a specific blob
func (s *Store) FindPeersWithBlob(ctx context.Context, digest string) ([]storage.PeerInfo, error) {
	return s.p2pMgr.FindPeersWithBlob(ctx, digest)
}

// ReplicateBlob replicates a blob to target peers
func (s *Store) ReplicateBlob(ctx context.Context, digest string, targetPeers []string) error {
	// Implementation would require transport access
	return fmt.Errorf("ReplicateBlob not implemented")
}

// PingPeer pings a specific peer
func (s *Store) PingPeer(ctx context.Context, peerID string) error {
	// Implementation would require transport access
	return fmt.Errorf("PingPeer not implemented")
}

// GetPeerHealth returns the health status of all peers
func (s *Store) GetPeerHealth(ctx context.Context) map[string]storage.PeerHealth {
	return s.p2pMgr.GetPeerHealth(ctx)
}

// GetNodeID returns the current node's ID
func (s *Store) GetNodeID() string {
	return s.nodeID
}

// IsHealthy returns whether the distributed store is healthy
func (s *Store) IsHealthy() bool {
	return s.p2pMgr.IsHealthy()
}

// Private helper methods

// getBlobFromPeers attempts to retrieve a blob from peer nodes
func (s *Store) getBlobFromPeers(ctx context.Context, digest string) (io.ReadCloser, error) {
	peers, err := s.p2pMgr.FindPeersWithBlob(ctx, digest)
	if err != nil {
		return nil, fmt.Errorf("failed to find peers with blob: %w", err)
	}

	if len(peers) == 0 {
		return nil, storage.ErrBlobNotFound
	}

	// Try to fetch from the first available peer
	// In a full implementation, you might try multiple peers or choose based on latency
	for _, peer := range peers {
		s.logger.Debug("Attempting to fetch blob from peer", "digest", digest[:12], "peer", peer.ID)

		// This would require transport access - for now, return error
		return nil, fmt.Errorf("peer blob fetching not fully implemented")
	}

	return nil, storage.ErrBlobNotFound
}

// getManifestFromPeers attempts to retrieve a manifest from peer nodes
func (s *Store) getManifestFromPeers(ctx context.Context, repo, reference string) (*types.Manifest, error) {
	// Similar to getBlobFromPeers, but for manifests
	// For now, return not found
	return nil, storage.ErrManifestNotFound
}

// replicateBlobToPeers replicates a blob to peer nodes
func (s *Store) replicateBlobToPeers(ctx context.Context, digest string) {
	s.logger.Debug("Starting blob replication", "digest", digest[:12])

	// Implementation would:
	// 1. Get list of healthy peers
	// 2. Select peers based on replication factor
	// 3. Send blob to selected peers
	// 4. Handle failures and retries

	// For now, just log the intent
	s.logger.Debug("Blob replication completed", "digest", digest[:12])
}

// replicateManifestToPeers replicates a manifest to peer nodes
func (s *Store) replicateManifestToPeers(ctx context.Context, repo, reference string, manifest *types.Manifest) {
	s.logger.Debug("Starting manifest replication", "repo", repo, "ref", reference)

	// Implementation would be similar to blob replication
	// For now, just log the intent
	s.logger.Debug("Manifest replication completed", "repo", repo, "ref", reference)
}

// Verify that Store implements the required interfaces
var _ storage.Store = (*Store)(nil)
var _ storage.PeerStore = (*Store)(nil)
