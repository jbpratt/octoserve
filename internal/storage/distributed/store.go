package distributed

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/jbpratt/octoserve/internal/storage"
	"github.com/jbpratt/octoserve/pkg/types"
)

// Store implements a distributed storage backend that wraps a local store
// and coordinates with peer nodes for replication and availability
type Store struct {
	local     storage.Store    // Local storage backend (e.g., filesystem)
	transport storage.Transport // Transport layer for peer communication
	p2pMgr    P2PManager       // P2P manager interface for peer coordination
	logger    *slog.Logger
	nodeID    string
	config    Config
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
func New(local storage.Store, transport storage.Transport, p2pMgr P2PManager, logger *slog.Logger, config Config) *Store {
	return &Store{
		local:     local,
		transport: transport,
		p2pMgr:    p2pMgr,
		logger:    logger,
		nodeID:    p2pMgr.GetNodeID(),
		config:    config,
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

	// If not found locally, try to fetch from peers
	if err == storage.ErrBlobNotFound {
		s.logger.Debug("Blob not found locally, trying peers", "digest", digest[:12])
		return s.getBlobFromPeers(ctx, digest)
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
	switch s.config.Strategy {
	case "eager":
		// Replicate immediately in background
		go s.replicateBlobToPeers(context.Background(), digest)
	case "lazy":
		// Lazy replication will happen when blob is requested by other nodes
		s.logger.Debug("Lazy replication configured, will replicate on demand", "digest", digest[:12])
	case "hybrid":
		// Make decision based on blob characteristics
		// For now, treat small blobs as eager, large as lazy
		size, err := s.local.GetBlobSize(ctx, digest)
		if err == nil && size < 1024*1024 { // 1MB threshold
			go s.replicateBlobToPeers(context.Background(), digest)
		}
	default:
		s.logger.Warn("Unknown replication strategy, defaulting to eager", "strategy", s.config.Strategy)
		go s.replicateBlobToPeers(context.Background(), digest)
	}

	return nil
}

// BlobExists checks if a blob exists locally or on peers
func (s *Store) BlobExists(ctx context.Context, digest string) (bool, error) {
	// Check local storage first
	exists, err := s.local.BlobExists(ctx, digest)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}

	// If not found locally, check peers
	return s.checkBlobExistsOnPeers(ctx, digest)
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
	// Try local storage first
	manifest, err := s.local.GetManifest(ctx, repo, reference)
	if err == nil {
		s.logger.Debug("Manifest found locally", "repo", repo, "ref", reference)
		return manifest, nil
	}

	// If not found locally, try to fetch from peers
	if err == storage.ErrManifestNotFound {
		s.logger.Debug("Manifest not found locally, trying peers", "repo", repo, "ref", reference)
		return s.getManifestFromPeers(ctx, repo, reference)
	}

	return nil, err
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
	// Check local storage first
	exists, err := s.local.ManifestExists(ctx, repo, reference)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}

	// If not found locally, check peers
	return s.checkManifestExistsOnPeers(ctx, repo, reference)
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
	// Find the peer by ID
	peers, err := s.p2pMgr.GetPeers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get peers: %w", err)
	}

	var targetPeer storage.PeerInfo
	found := false
	for _, peer := range peers {
		if peer.ID == peerID {
			targetPeer = peer
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("peer %s not found", peerID)
	}

	// Request blob from the specific peer
	return s.transport.RequestBlob(ctx, targetPeer, digest)
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
	// First find peers that have the blob
	peers, err := s.findPeersWithBlobDirect(ctx, digest)
	if err != nil {
		s.logger.Error("Failed to find peers with blob", "digest", digest[:12], "error", err)
		return nil, storage.ErrBlobNotFound
	}

	if len(peers) == 0 {
		s.logger.Debug("No peers found with blob", "digest", digest[:12])
		return nil, storage.ErrBlobNotFound
	}

	// Try to fetch from each peer until one succeeds
	for _, peer := range peers {
		s.logger.Debug("Attempting to fetch blob from peer", "digest", digest[:12], "peer", peer.ID)

		reader, err := s.transport.RequestBlob(ctx, peer, digest)
		if err != nil {
			s.logger.Warn("Failed to fetch blob from peer", "digest", digest[:12], "peer", peer.ID, "error", err)
			continue
		}

		s.logger.Info("Successfully fetched blob from peer", "digest", digest[:12], "peer", peer.ID)
		
		// Store the blob locally for future requests (cache the fetched blob)
		if err := s.local.PutBlob(ctx, digest, reader); err != nil {
			s.logger.Warn("Failed to cache fetched blob locally", "digest", digest[:12], "error", err)
			// Even if caching fails, try to continue by returning a fresh reader
			reader.Close()
			continue
		}

		// Return a fresh reader from local storage
		return s.local.GetBlob(ctx, digest)
	}

	return nil, storage.ErrBlobNotFound
}

// checkBlobExistsOnPeers checks if any peer has the specified blob
func (s *Store) checkBlobExistsOnPeers(ctx context.Context, digest string) (bool, error) {
	peers, err := s.findPeersWithBlobDirect(ctx, digest)
	if err != nil {
		s.logger.Error("Failed to check blob existence on peers", "digest", digest[:12], "error", err)
		return false, nil // Return false instead of error for existence checks
	}
	return len(peers) > 0, nil
}

// findPeersWithBlobDirect directly queries peers to find who has a blob
func (s *Store) findPeersWithBlobDirect(ctx context.Context, digest string) ([]storage.PeerInfo, error) {
	// Get all known peers
	allPeers, err := s.p2pMgr.GetPeers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get peers: %w", err)
	}

	if len(allPeers) == 0 {
		return nil, nil
	}

	// Create a channel to collect results
	type peerResult struct {
		peer storage.PeerInfo
		has  bool
		err  error
	}

	results := make(chan peerResult, len(allPeers))
	ctx, cancel := context.WithTimeout(ctx, s.config.RequestTimeout)
	defer cancel()

	// Query each peer concurrently using HasBlob gRPC call
	for _, peer := range allPeers {
		go func(p storage.PeerInfo) {
			// Use actual HasBlob gRPC call through transport layer
			has, _, err := s.transport.HasBlob(ctx, p, digest)
			results <- peerResult{peer: p, has: has, err: err}
		}(peer)
	}

	// Collect results
	var peersWithBlob []storage.PeerInfo
	for i := 0; i < len(allPeers); i++ {
		select {
		case result := <-results:
			if result.has {
				peersWithBlob = append(peersWithBlob, result.peer)
			}
		case <-ctx.Done():
			return peersWithBlob, ctx.Err()
		}
	}

	return peersWithBlob, nil
}

// getManifestFromPeers attempts to retrieve a manifest from peer nodes
func (s *Store) getManifestFromPeers(ctx context.Context, repo, reference string) (*types.Manifest, error) {
	// First find peers that have the manifest
	peers, err := s.findPeersWithManifest(ctx, repo, reference)
	if err != nil {
		s.logger.Error("Failed to find peers with manifest", "repo", repo, "ref", reference, "error", err)
		return nil, storage.ErrManifestNotFound
	}

	if len(peers) == 0 {
		s.logger.Debug("No peers found with manifest", "repo", repo, "ref", reference)
		return nil, storage.ErrManifestNotFound
	}

	// Try to fetch from each peer until one succeeds
	for _, peer := range peers {
		s.logger.Debug("Attempting to fetch manifest from peer", "repo", repo, "ref", reference, "peer", peer.ID)

		manifest, err := s.transport.RequestManifest(ctx, peer, repo, reference)
		if err != nil {
			s.logger.Warn("Failed to fetch manifest from peer", "repo", repo, "ref", reference, "peer", peer.ID, "error", err)
			continue
		}

		s.logger.Info("Successfully fetched manifest from peer", "repo", repo, "ref", reference, "peer", peer.ID)
		
		// Store the manifest locally for future requests
		if err := s.local.PutManifest(ctx, repo, reference, manifest); err != nil {
			s.logger.Warn("Failed to cache fetched manifest locally", "repo", repo, "ref", reference, "error", err)
		}

		return manifest, nil
	}

	return nil, storage.ErrManifestNotFound
}

// replicateBlobToPeers replicates a blob to peer nodes
func (s *Store) replicateBlobToPeers(ctx context.Context, digest string) {
	s.logger.Debug("Starting blob replication", "digest", digest[:12])

	// Get the blob from local storage
	reader, err := s.local.GetBlob(ctx, digest)
	if err != nil {
		s.logger.Error("Failed to get blob for replication", "digest", digest[:12], "error", err)
		return
	}
	defer reader.Close()

	// Read blob data into memory for replication
	// TODO: For large blobs, implement streaming replication
	data, err := io.ReadAll(reader)
	if err != nil {
		s.logger.Error("Failed to read blob for replication", "digest", digest[:12], "error", err)
		return
	}

	// Get list of healthy peers
	peers, err := s.p2pMgr.GetPeers(ctx)
	if err != nil {
		s.logger.Error("Failed to get peers for replication", "digest", digest[:12], "error", err)
		return
	}

	// Filter to healthy peers
	healthyPeers := s.filterHealthyPeers(ctx, peers)
	if len(healthyPeers) == 0 {
		s.logger.Debug("No healthy peers available for replication", "digest", digest[:12])
		return
	}

	// Select target peers based on replication factor
	targetPeers := s.selectReplicationTargets(healthyPeers)
	if len(targetPeers) == 0 {
		s.logger.Debug("No target peers selected for replication", "digest", digest[:12])
		return
	}

	// Replicate to selected peers concurrently
	var wg sync.WaitGroup
	successCount := 0
	var successMutex sync.Mutex

	for _, peer := range targetPeers {
		wg.Add(1)
		go func(p storage.PeerInfo) {
			defer wg.Done()

			// Create a new reader for each goroutine
			blobReader := io.NopCloser(bytes.NewReader(data))
			err := s.transport.SendBlob(ctx, p, digest, blobReader)
			if err != nil {
				s.logger.Warn("Failed to replicate blob to peer", 
					"digest", digest[:12], "peer", p.ID, "error", err)
			} else {
				s.logger.Debug("Successfully replicated blob to peer", 
					"digest", digest[:12], "peer", p.ID)
				successMutex.Lock()
				successCount++
				successMutex.Unlock()
			}
		}(peer)
	}

	wg.Wait()

	s.logger.Info("Blob replication completed", 
		"digest", digest[:12], 
		"success_count", successCount, 
		"target_count", len(targetPeers))
}

// replicateManifestToPeers replicates a manifest to peer nodes
func (s *Store) replicateManifestToPeers(ctx context.Context, repo, reference string, manifest *types.Manifest) {
	s.logger.Debug("Starting manifest replication", "repo", repo, "ref", reference)

	// Get list of healthy peers
	peers, err := s.p2pMgr.GetPeers(ctx)
	if err != nil {
		s.logger.Error("Failed to get peers for manifest replication", "repo", repo, "ref", reference, "error", err)
		return
	}

	// Filter to healthy peers
	healthyPeers := s.filterHealthyPeers(ctx, peers)
	if len(healthyPeers) == 0 {
		s.logger.Debug("No healthy peers available for manifest replication", "repo", repo, "ref", reference)
		return
	}

	// Select target peers (for manifests, always replicate to all healthy peers for consistency)
	targetPeers := healthyPeers
	if len(targetPeers) == 0 {
		s.logger.Debug("No target peers selected for manifest replication", "repo", repo, "ref", reference)
		return
	}

	// Replicate to selected peers concurrently
	var wg sync.WaitGroup
	successCount := 0
	var successMutex sync.Mutex

	for _, peer := range targetPeers {
		wg.Add(1)
		go func(p storage.PeerInfo) {
			defer wg.Done()

			err := s.transport.SendManifest(ctx, p, repo, reference, manifest)
			if err != nil {
				s.logger.Warn("Failed to replicate manifest to peer", 
					"repo", repo, "ref", reference, "peer", p.ID, "error", err)
			} else {
				s.logger.Debug("Successfully replicated manifest to peer", 
					"repo", repo, "ref", reference, "peer", p.ID)
				successMutex.Lock()
				successCount++
				successMutex.Unlock()
			}
		}(peer)
	}

	wg.Wait()

	s.logger.Info("Manifest replication completed", 
		"repo", repo, "ref", reference,
		"success_count", successCount, 
		"target_count", len(targetPeers))
}

// filterHealthyPeers filters peers to only include healthy ones
func (s *Store) filterHealthyPeers(ctx context.Context, peers []storage.PeerInfo) []storage.PeerInfo {
	healthStatus := s.p2pMgr.GetPeerHealth(ctx)
	var healthyPeers []storage.PeerInfo

	for _, peer := range peers {
		if health, exists := healthStatus[peer.ID]; exists && health.Status == storage.HealthHealthy {
			healthyPeers = append(healthyPeers, peer)
		}
	}

	return healthyPeers
}

// selectReplicationTargets selects target peers for replication based on configured strategy
func (s *Store) selectReplicationTargets(peers []storage.PeerInfo) []storage.PeerInfo {
	targetCount := s.config.ReplicationFactor
	if targetCount <= 0 {
		targetCount = 1 // Default to at least 1 replica
	}

	// Don't replicate to more peers than available
	if targetCount > len(peers) {
		targetCount = len(peers)
	}

	// For now, use simple round-robin selection
	// In a production system, you might consider factors like:
	// - Network latency
	// - Storage capacity
	// - Geographic distribution
	// - Load balancing

	selected := make([]storage.PeerInfo, targetCount)
	for i := 0; i < targetCount; i++ {
		selected[i] = peers[i]
	}

	return selected
}

// TriggerLazyReplication triggers replication for a blob when it's accessed
func (s *Store) TriggerLazyReplication(ctx context.Context, digest string) {
	if s.config.Strategy != "lazy" && s.config.Strategy != "hybrid" {
		return // Only trigger for lazy/hybrid strategies
	}

	s.logger.Debug("Triggering lazy replication for accessed blob", "digest", digest[:12])

	// Check if we should replicate based on access patterns
	// For now, always replicate when accessed from another peer
	go s.replicateBlobToPeers(ctx, digest)
}

// checkManifestExistsOnPeers checks if any peer has the specified manifest
func (s *Store) checkManifestExistsOnPeers(ctx context.Context, repo, reference string) (bool, error) {
	peers, err := s.findPeersWithManifest(ctx, repo, reference)
	if err != nil {
		s.logger.Error("Failed to check manifest existence on peers", "repo", repo, "ref", reference, "error", err)
		return false, nil // Return false instead of error for existence checks
	}
	return len(peers) > 0, nil
}

// findPeersWithManifest directly queries peers to find who has a manifest
func (s *Store) findPeersWithManifest(ctx context.Context, repo, reference string) ([]storage.PeerInfo, error) {
	// Get all known peers
	allPeers, err := s.p2pMgr.GetPeers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get peers: %w", err)
	}

	if len(allPeers) == 0 {
		return nil, nil
	}

	// Create a channel to collect results
	type peerResult struct {
		peer storage.PeerInfo
		has  bool
		err  error
	}

	results := make(chan peerResult, len(allPeers))
	ctx, cancel := context.WithTimeout(ctx, s.config.RequestTimeout)
	defer cancel()

	// Query each peer concurrently using HasManifest gRPC call
	for _, peer := range allPeers {
		go func(p storage.PeerInfo) {
			// Use actual HasManifest gRPC call through transport layer
			has, _, _, err := s.transport.HasManifest(ctx, p, repo, reference)
			results <- peerResult{peer: p, has: has, err: err}
		}(peer)
	}

	// Collect results
	var peersWithManifest []storage.PeerInfo
	for i := 0; i < len(allPeers); i++ {
		select {
		case result := <-results:
			if result.has {
				peersWithManifest = append(peersWithManifest, result.peer)
			}
		case <-ctx.Done():
			return peersWithManifest, ctx.Err()
		}
	}

	return peersWithManifest, nil
}

// Verify that Store implements the required interfaces
var _ storage.Store = (*Store)(nil)
var _ storage.PeerStore = (*Store)(nil)
