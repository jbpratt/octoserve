package filesystem

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jbpratt/octoserve/pkg/types"
)

func TestStore(t *testing.T) {
	// Create temporary directory for tests
	tempDir, err := os.MkdirTemp("", "octoserve-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create store
	store, err := New(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	ctx := context.Background()

	t.Run("blob operations", func(t *testing.T) {
		testBlobOperations(t, store, ctx)
	})

	t.Run("manifest operations", func(t *testing.T) {
		testManifestOperations(t, store, ctx)
	})

	t.Run("upload operations", func(t *testing.T) {
		testUploadOperations(t, store, ctx)
	})
}

func testBlobOperations(t *testing.T, store *Store, ctx context.Context) {
	digest := "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	content := []byte("")

	// Test blob doesn't exist initially
	exists, err := store.BlobExists(ctx, digest)
	if err != nil {
		t.Fatalf("BlobExists failed: %v", err)
	}
	if exists {
		t.Error("Blob should not exist initially")
	}

	// Test PutBlob
	err = store.PutBlob(ctx, digest, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("PutBlob failed: %v", err)
	}

	// Test blob exists now
	exists, err = store.BlobExists(ctx, digest)
	if err != nil {
		t.Fatalf("BlobExists failed: %v", err)
	}
	if !exists {
		t.Error("Blob should exist after PutBlob")
	}

	// Test GetBlobSize
	size, err := store.GetBlobSize(ctx, digest)
	if err != nil {
		t.Fatalf("GetBlobSize failed: %v", err)
	}
	if size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), size)
	}

	// Test GetBlob
	reader, err := store.GetBlob(ctx, digest)
	if err != nil {
		t.Fatalf("GetBlob failed: %v", err)
	}
	defer reader.Close()

	readContent := make([]byte, len(content))
	n, err := reader.Read(readContent)
	if err != nil && err.Error() != "EOF" {
		t.Fatalf("Reading blob failed: %v", err)
	}
	if n != len(content) {
		t.Errorf("Expected to read %d bytes, got %d", len(content), n)
	}

	// Test DeleteBlob
	err = store.DeleteBlob(ctx, digest)
	if err != nil {
		t.Fatalf("DeleteBlob failed: %v", err)
	}

	// Test blob doesn't exist after deletion
	exists, err = store.BlobExists(ctx, digest)
	if err != nil {
		t.Fatalf("BlobExists failed: %v", err)
	}
	if exists {
		t.Error("Blob should not exist after deletion")
	}
}

func testManifestOperations(t *testing.T, store *Store, ctx context.Context) {
	repo := "test/repo"
	tag := "latest"

	manifest := &types.Manifest{
		MediaType:     types.MediaTypes.ImageManifest,
		SchemaVersion: 2,
		Config: types.Descriptor{
			MediaType: types.MediaTypes.ImageConfig,
			Digest:    "sha256:config",
			Size:      1000,
		},
		Layers: []types.Descriptor{
			{
				MediaType: types.MediaTypes.ImageLayerGzip,
				Digest:    "sha256:layer1",
				Size:      2000,
			},
		},
	}

	// Test manifest doesn't exist initially
	exists, err := store.ManifestExists(ctx, repo, tag)
	if err != nil {
		t.Fatalf("ManifestExists failed: %v", err)
	}
	if exists {
		t.Error("Manifest should not exist initially")
	}

	// Test PutManifest
	err = store.PutManifest(ctx, repo, tag, manifest)
	if err != nil {
		t.Fatalf("PutManifest failed: %v", err)
	}

	// Test manifest exists now
	exists, err = store.ManifestExists(ctx, repo, tag)
	if err != nil {
		t.Fatalf("ManifestExists failed: %v", err)
	}
	if !exists {
		t.Error("Manifest should exist after PutManifest")
	}

	// Test GetManifest
	retrievedManifest, err := store.GetManifest(ctx, repo, tag)
	if err != nil {
		t.Fatalf("GetManifest failed: %v", err)
	}

	if retrievedManifest.MediaType != manifest.MediaType {
		t.Errorf("Expected MediaType %s, got %s", manifest.MediaType, retrievedManifest.MediaType)
	}

	if retrievedManifest.Repository != repo {
		t.Errorf("Expected Repository %s, got %s", repo, retrievedManifest.Repository)
	}

	if retrievedManifest.Tag != tag {
		t.Errorf("Expected Tag %s, got %s", tag, retrievedManifest.Tag)
	}

	// Test ListTags
	tags, err := store.ListTags(ctx, repo)
	if err != nil {
		t.Fatalf("ListTags failed: %v", err)
	}

	found := false
	for _, t := range tags {
		if t == tag {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Tag %s not found in tags list: %v", tag, tags)
	}

	// Test DeleteManifest
	err = store.DeleteManifest(ctx, repo, tag)
	if err != nil {
		t.Fatalf("DeleteManifest failed: %v", err)
	}

	// Test manifest doesn't exist after deletion
	exists, err = store.ManifestExists(ctx, repo, tag)
	if err != nil {
		t.Fatalf("ManifestExists failed: %v", err)
	}
	if exists {
		t.Error("Manifest should not exist after deletion")
	}
}

func testUploadOperations(t *testing.T, store *Store, ctx context.Context) {
	repo := "test/repo"

	// Test CreateUpload
	upload, err := store.CreateUpload(ctx, repo)
	if err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	if upload.Repository != repo {
		t.Errorf("Expected repository %s, got %s", repo, upload.Repository)
	}

	// Test GetUpload
	retrievedUpload, err := store.GetUpload(ctx, upload.ID)
	if err != nil {
		t.Fatalf("GetUpload failed: %v", err)
	}

	if retrievedUpload.ID != upload.ID {
		t.Errorf("Expected upload ID %s, got %s", upload.ID, retrievedUpload.ID)
	}

	// Test WriteUploadChunk
	content := []byte("test content")
	err = store.WriteUploadChunk(ctx, upload.ID, 0, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("WriteUploadChunk failed: %v", err)
	}

	// Test GetUploadStatus
	offset, err := store.GetUploadStatus(ctx, upload.ID)
	if err != nil {
		t.Fatalf("GetUploadStatus failed: %v", err)
	}

	expectedOffset := int64(len(content))
	if offset != expectedOffset {
		t.Errorf("Expected offset %d, got %d", expectedOffset, offset)
	}

	// Test CancelUpload
	err = store.CancelUpload(ctx, upload.ID)
	if err != nil {
		t.Fatalf("CancelUpload failed: %v", err)
	}

	// Test upload doesn't exist after cancellation
	_, err = store.GetUpload(ctx, upload.ID)
	if err == nil {
		t.Error("Upload should not exist after cancellation")
	}
}

func TestStorePaths(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "octoserve-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := New(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	tests := []struct {
		name     string
		digest   string
		expected string
	}{
		{
			name:     "sha256 digest",
			digest:   "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expected: filepath.Join(tempDir, "blobs", "sha256", "ab", "cd", "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
		},
		{
			name:     "sha512 digest",
			digest:   "sha512:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected: filepath.Join(tempDir, "blobs", "sha512", "12", "34", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := store.blobPath(tt.digest)
			if path != tt.expected {
				t.Errorf("Expected path %s, got %s", tt.expected, path)
			}
		})
	}
}

func TestManifestPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "octoserve-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := New(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	tests := []struct {
		name      string
		repo      string
		reference string
		expected  string
	}{
		{
			name:      "simple repo and tag",
			repo:      "myrepo",
			reference: "latest",
			expected:  filepath.Join(tempDir, "manifests", "myrepo", "latest"),
		},
		{
			name:      "namespaced repo",
			repo:      "namespace/myrepo",
			reference: "v1.0.0",
			expected:  filepath.Join(tempDir, "manifests", "namespace_myrepo", "v1.0.0"),
		},
		{
			name:      "digest reference",
			repo:      "myrepo",
			reference: "sha256:abcdef",
			expected:  filepath.Join(tempDir, "manifests", "myrepo", "sha256:abcdef"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := store.manifestPath(tt.repo, tt.reference)
			if path != tt.expected {
				t.Errorf("Expected path %s, got %s", tt.expected, path)
			}
		})
	}
}
