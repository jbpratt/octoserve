package distributed

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jbpratt/octoserve/internal/storage"
	"github.com/jbpratt/octoserve/pkg/types"
)

// MockStore implements the storage.Store interface for testing
type MockStore struct {
	blobs     map[string][]byte
	manifests map[string]*types.Manifest
}

func NewMockStore() *MockStore {
	return &MockStore{
		blobs:     make(map[string][]byte),
		manifests: make(map[string]*types.Manifest),
	}
}

func (m *MockStore) GetBlob(ctx context.Context, digest string) (io.ReadCloser, error) {
	data, exists := m.blobs[digest]
	if !exists {
		return nil, storage.ErrBlobNotFound
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

func (m *MockStore) PutBlob(ctx context.Context, digest string, content io.Reader) error {
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}
	m.blobs[digest] = data
	return nil
}

func (m *MockStore) BlobExists(ctx context.Context, digest string) (bool, error) {
	_, exists := m.blobs[digest]
	return exists, nil
}

func (m *MockStore) DeleteBlob(ctx context.Context, digest string) error {
	delete(m.blobs, digest)
	return nil
}

func (m *MockStore) GetBlobSize(ctx context.Context, digest string) (int64, error) {
	data, exists := m.blobs[digest]
	if !exists {
		return 0, storage.ErrBlobNotFound
	}
	return int64(len(data)), nil
}

func (m *MockStore) GetManifest(ctx context.Context, repo, reference string) (*types.Manifest, error) {
	key := repo + "/" + reference
	manifest, exists := m.manifests[key]
	if !exists {
		return nil, storage.ErrManifestNotFound
	}
	return manifest, nil
}

func (m *MockStore) PutManifest(ctx context.Context, repo, reference string, manifest *types.Manifest) error {
	key := repo + "/" + reference
	m.manifests[key] = manifest
	return nil
}

func (m *MockStore) DeleteManifest(ctx context.Context, repo, reference string) error {
	key := repo + "/" + reference
	delete(m.manifests, key)
	return nil
}

func (m *MockStore) ListTags(ctx context.Context, repo string) ([]string, error) {
	var tags []string
	for key := range m.manifests {
		if strings.HasPrefix(key, repo+"/") {
			tag := strings.TrimPrefix(key, repo+"/")
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

func (m *MockStore) ManifestExists(ctx context.Context, repo, reference string) (bool, error) {
	key := repo + "/" + reference
	_, exists := m.manifests[key]
	return exists, nil
}

func (m *MockStore) GetReferrers(ctx context.Context, repo, digest string) ([]types.Descriptor, error) {
	return []types.Descriptor{}, nil
}

func (m *MockStore) CreateUpload(ctx context.Context, repo string) (*types.Upload, error) {
	return types.NewUpload("test-upload", repo), nil
}

func (m *MockStore) GetUpload(ctx context.Context, uploadID string) (*types.Upload, error) {
	return types.NewUpload(uploadID, "test-repo"), nil
}

func (m *MockStore) WriteUploadChunk(ctx context.Context, uploadID string, offset int64, chunk io.Reader) error {
	return nil
}

func (m *MockStore) CompleteUpload(ctx context.Context, uploadID string, digest string) error {
	return nil
}

func (m *MockStore) CancelUpload(ctx context.Context, uploadID string) error {
	return nil
}

func (m *MockStore) GetUploadStatus(ctx context.Context, uploadID string) (int64, error) {
	return 0, nil
}

// MockP2PManager implements the P2PManager interface for testing
type MockP2PManager struct {
	nodeID string
	peers  []storage.PeerInfo
}

func NewMockP2PManager(nodeID string) *MockP2PManager {
	return &MockP2PManager{
		nodeID: nodeID,
		peers:  []storage.PeerInfo{},
	}
}

func (m *MockP2PManager) GetNodeID() string {
	return m.nodeID
}

func (m *MockP2PManager) IsHealthy() bool {
	return true
}

func (m *MockP2PManager) GetPeers(ctx context.Context) ([]storage.PeerInfo, error) {
	return m.peers, nil
}

func (m *MockP2PManager) GetPeerHealth(ctx context.Context) map[string]storage.PeerHealth {
	health := make(map[string]storage.PeerHealth)
	for _, peer := range m.peers {
		health[peer.ID] = storage.PeerHealth{
			Status:    storage.HealthHealthy,
			LastCheck: time.Now(),
		}
	}
	return health
}

func (m *MockP2PManager) FindPeersWithBlob(ctx context.Context, digest string) ([]storage.PeerInfo, error) {
	// For testing, return empty list (no peers have the blob)
	return []storage.PeerInfo{}, nil
}

func TestDistributedStore(t *testing.T) {
	// Create test components
	mockStore := NewMockStore()
	mockP2P := NewMockP2PManager("test-node")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	config := Config{
		ReplicationFactor: 3,
		Strategy:          "eager",
		ConsistencyMode:   "eventual",
		RequestTimeout:    5 * time.Second,
	}

	// Create distributed store
	store := New(mockStore, mockP2P, logger, config)

	// Test blob operations
	t.Run("blob operations", func(t *testing.T) {
		digest := "sha256:abc123"
		content := "test blob content"

		// Test PutBlob
		err := store.PutBlob(context.Background(), digest, strings.NewReader(content))
		if err != nil {
			t.Fatalf("PutBlob failed: %v", err)
		}

		// Test BlobExists
		exists, err := store.BlobExists(context.Background(), digest)
		if err != nil {
			t.Fatalf("BlobExists failed: %v", err)
		}
		if !exists {
			t.Fatal("blob should exist after putting")
		}

		// Test GetBlob
		reader, err := store.GetBlob(context.Background(), digest)
		if err != nil {
			t.Fatalf("GetBlob failed: %v", err)
		}
		defer reader.Close()

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("reading blob failed: %v", err)
		}

		if string(data) != content {
			t.Fatalf("expected %s, got %s", content, string(data))
		}

		// Test GetBlobSize
		size, err := store.GetBlobSize(context.Background(), digest)
		if err != nil {
			t.Fatalf("GetBlobSize failed: %v", err)
		}
		if size != int64(len(content)) {
			t.Fatalf("expected size %d, got %d", len(content), size)
		}
	})

	// Test manifest operations
	t.Run("manifest operations", func(t *testing.T) {
		repo := "test/repo"
		reference := "latest"
		manifest := &types.Manifest{
			MediaType:     "application/vnd.oci.image.manifest.v1+json",
			SchemaVersion: 2,
			Repository:    repo,
			Tag:           reference,
		}

		// Test PutManifest
		err := store.PutManifest(context.Background(), repo, reference, manifest)
		if err != nil {
			t.Fatalf("PutManifest failed: %v", err)
		}

		// Test ManifestExists
		exists, err := store.ManifestExists(context.Background(), repo, reference)
		if err != nil {
			t.Fatalf("ManifestExists failed: %v", err)
		}
		if !exists {
			t.Fatal("manifest should exist after putting")
		}

		// Test GetManifest
		retrievedManifest, err := store.GetManifest(context.Background(), repo, reference)
		if err != nil {
			t.Fatalf("GetManifest failed: %v", err)
		}

		if retrievedManifest.MediaType != manifest.MediaType {
			t.Fatalf("expected media type %s, got %s", manifest.MediaType, retrievedManifest.MediaType)
		}
	})

	// Test P2P interface methods
	t.Run("p2p interface", func(t *testing.T) {
		// Test GetNodeID
		nodeID := store.GetNodeID()
		if nodeID != "test-node" {
			t.Fatalf("expected node ID 'test-node', got '%s'", nodeID)
		}

		// Test IsHealthy
		if !store.IsHealthy() {
			t.Fatal("store should be healthy")
		}

		// Test GetPeers
		peers, err := store.GetPeers(context.Background())
		if err != nil {
			t.Fatalf("GetPeers failed: %v", err)
		}
		if len(peers) != 0 {
			t.Fatalf("expected 0 peers, got %d", len(peers))
		}

		// Test GetPeerHealth
		health := store.GetPeerHealth(context.Background())
		if len(health) != 0 {
			t.Fatalf("expected 0 health entries, got %d", len(health))
		}
	})
}

func TestDistributedStoreBlobNotFound(t *testing.T) {
	mockStore := NewMockStore()
	mockP2P := NewMockP2PManager("test-node")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	config := Config{
		ReplicationFactor: 3,
		Strategy:          "eager",
		ConsistencyMode:   "eventual",
		RequestTimeout:    5 * time.Second,
	}

	store := New(mockStore, mockP2P, logger, config)

	// Test GetBlob for non-existent blob
	_, err := store.GetBlob(context.Background(), "sha256:nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent blob")
	}

	// Since we have no peers with the blob, it should try local first then fail
	if err != storage.ErrBlobNotFound {
		t.Fatalf("expected ErrBlobNotFound, got %v", err)
	}
}
