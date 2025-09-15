package filesystem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"

	"github.com/jbpratt/octoserve/internal/storage"
	"github.com/jbpratt/octoserve/pkg/types"
)

// Store implements storage.Store using filesystem backend
type Store struct {
	basePath string
	uploads  sync.Map // uploadID -> *types.Upload
}

// New creates a new filesystem store
func New(basePath string) (*Store, error) {
	// Create base directories
	dirs := []string{
		filepath.Join(basePath, "blobs"),
		filepath.Join(basePath, "manifests"),
		filepath.Join(basePath, "uploads"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return &Store{
		basePath: basePath,
	}, nil
}

// blobPath returns the path for storing a blob
func (s *Store) blobPath(digestStr string) string {
	// Parse digest to get algorithm and hex
	d, err := digest.Parse(digestStr)
	if err != nil {
		// Fallback: manually parse algorithm:hex format
		parts := strings.SplitN(digestStr, ":", 2)
		if len(parts) != 2 {
			return filepath.Join(s.basePath, "blobs", digestStr)
		}
		algorithm, hex := parts[0], parts[1]
		if len(hex) >= 4 {
			return filepath.Join(s.basePath, "blobs", algorithm, hex[:2], hex[2:4], hex)
		}
		return filepath.Join(s.basePath, "blobs", algorithm, hex)
	}

	algorithm := d.Algorithm().String()
	hex := d.Hex()

	// Store as blobs/sha256/ab/cd/abcd...
	if len(hex) >= 4 {
		return filepath.Join(s.basePath, "blobs", algorithm, hex[:2], hex[2:4], hex)
	}
	return filepath.Join(s.basePath, "blobs", algorithm, hex)
}

// manifestPath returns the path for storing a manifest
func (s *Store) manifestPath(repo, reference string) string {
	// Clean repository name for filesystem
	cleanRepo := strings.ReplaceAll(repo, "/", "_")
	return filepath.Join(s.basePath, "manifests", cleanRepo, reference)
}

// uploadPath returns the path for storing upload data
func (s *Store) uploadPath(uploadID string) string {
	return filepath.Join(s.basePath, "uploads", uploadID)
}

// GetBlob implements BlobStore
func (s *Store) GetBlob(ctx context.Context, digest string) (io.ReadCloser, error) {
	path := s.blobPath(digest)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, storage.ErrBlobNotFound
		}
		return nil, fmt.Errorf("failed to open blob %s: %w", digest, err)
	}
	return file, nil
}

// PutBlob implements BlobStore
func (s *Store) PutBlob(ctx context.Context, digest string, content io.Reader) error {
	path := s.blobPath(digest)

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create blob directory: %w", err)
	}

	// Create temporary file first
	tempPath := path + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempPath) // Clean up on error
	}()

	// Copy content while calculating digest
	hasher := sha256.New()
	writer := io.MultiWriter(tempFile, hasher)

	if _, err := io.Copy(writer, content); err != nil {
		return fmt.Errorf("failed to write blob content: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Verify digest
	calculated := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	if calculated != digest {
		return fmt.Errorf("digest mismatch: expected %s, got %s", digest, calculated)
	}

	// Atomic move
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("failed to move blob to final location: %w", err)
	}

	return nil
}

// BlobExists implements BlobStore
func (s *Store) BlobExists(ctx context.Context, digest string) (bool, error) {
	path := s.blobPath(digest)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DeleteBlob implements BlobStore
func (s *Store) DeleteBlob(ctx context.Context, digest string) error {
	path := s.blobPath(digest)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete blob %s: %w", digest, err)
	}
	return nil
}

// GetBlobSize implements BlobStore
func (s *Store) GetBlobSize(ctx context.Context, digest string) (int64, error) {
	path := s.blobPath(digest)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, storage.ErrBlobNotFound
		}
		return 0, fmt.Errorf("failed to stat blob %s: %w", digest, err)
	}
	return info.Size(), nil
}

// GetManifest implements ManifestStore
func (s *Store) GetManifest(ctx context.Context, repo, reference string) (*types.Manifest, error) {
	path := s.manifestPath(repo, reference)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, storage.ErrManifestNotFound
		}
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	manifest, err := types.ParseManifest(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	manifest.Repository = repo
	if !strings.HasPrefix(reference, "sha256:") {
		manifest.Tag = reference
	}

	return manifest, nil
}

// PutManifest implements ManifestStore
func (s *Store) PutManifest(ctx context.Context, repo, reference string, manifest *types.Manifest) error {
	// Marshal manifest once
	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Store by the given reference (tag or digest)
	path := s.manifestPath(repo, reference)
	if err := s.writeManifestFile(path, data); err != nil {
		return err
	}

	// If storing by tag, also store by digest for retrieval
	if !strings.HasPrefix(reference, "sha256:") && manifest.Digest != "" {
		digestPath := s.manifestPath(repo, manifest.Digest)
		if err := s.writeManifestFile(digestPath, data); err != nil {
			return err
		}
	}

	return nil
}

// writeManifestFile writes manifest data to a file atomically
func (s *Store) writeManifestFile(path string, data []byte) error {
	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create manifest directory: %w", err)
	}

	// Write to temporary file first
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary manifest: %w", err)
	}

	// Atomic move
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("failed to move manifest to final location: %w", err)
	}

	return nil
}

// DeleteManifest implements ManifestStore
func (s *Store) DeleteManifest(ctx context.Context, repo, reference string) error {
	path := s.manifestPath(repo, reference)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete manifest: %w", err)
	}
	return nil
}

// ListTags implements ManifestStore
func (s *Store) ListTags(ctx context.Context, repo string) ([]string, error) {
	cleanRepo := strings.ReplaceAll(repo, "/", "_")
	repoPath := filepath.Join(s.basePath, "manifests", cleanRepo)

	entries, err := os.ReadDir(repoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read repository directory: %w", err)
	}

	var tags []string
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasPrefix(entry.Name(), "sha256:") {
			tags = append(tags, entry.Name())
		}
	}

	return tags, nil
}

// ManifestExists implements ManifestStore
func (s *Store) ManifestExists(ctx context.Context, repo, reference string) (bool, error) {
	path := s.manifestPath(repo, reference)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetReferrers implements ManifestStore
func (s *Store) GetReferrers(ctx context.Context, repo, digest string) ([]types.Descriptor, error) {
	var referrers []types.Descriptor

	// Get the repository path
	cleanRepo := strings.ReplaceAll(repo, "/", "_")
	repoPath := filepath.Join(s.basePath, "manifests", cleanRepo)

	// Check if repository exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		// Return empty list for non-existent repositories
		return referrers, nil
	}

	// Read all manifest files in the repository
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read repository directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Read manifest file
		manifestPath := filepath.Join(repoPath, entry.Name())
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			// Skip files that can't be read
			continue
		}

		// Parse manifest to check subject field
		manifest, err := types.ParseManifest(data)
		if err != nil {
			// Skip invalid manifests
			continue
		}

		// Check if this manifest references the target digest as subject
		if manifest.Subject != nil && manifest.Subject.Digest == digest {
			// Create descriptor for this referring manifest
			descriptor := types.Descriptor{
				MediaType: manifest.MediaType,
				Digest:    manifest.Digest,
				Size:      manifest.Size,
			}

			// Set artifactType for filtering with priority: explicit ArtifactType > config.MediaType
			if manifest.ArtifactType != "" {
				descriptor.ArtifactType = manifest.ArtifactType
			} else if manifest.Config.MediaType != "" {
				descriptor.ArtifactType = manifest.Config.MediaType
			}

			// Copy annotations if present
			if manifest.Annotations != nil {
				descriptor.Annotations = make(map[string]string)
				for k, v := range manifest.Annotations {
					descriptor.Annotations[k] = v
				}
			}

			referrers = append(referrers, descriptor)
		}
	}

	return referrers, nil
}

// CreateUpload implements UploadManager
func (s *Store) CreateUpload(ctx context.Context, repo string) (*types.Upload, error) {
	uploadID := uuid.New().String()
	upload := types.NewUpload(uploadID, repo)

	// Create upload directory and temp file
	uploadDir := filepath.Join(s.basePath, "uploads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	tempFile := s.uploadPath(uploadID)
	upload.SetTempFile(tempFile)

	// Create empty temp file
	file, err := os.Create(tempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create upload file: %w", err)
	}
	file.Close()

	s.uploads.Store(uploadID, upload)
	return upload, nil
}

// GetUpload implements UploadManager
func (s *Store) GetUpload(ctx context.Context, uploadID string) (*types.Upload, error) {
	value, ok := s.uploads.Load(uploadID)
	if !ok {
		return nil, storage.ErrUploadNotFound
	}
	return value.(*types.Upload), nil
}

// WriteUploadChunk implements UploadManager
func (s *Store) WriteUploadChunk(ctx context.Context, uploadID string, offset int64, chunk io.Reader) error {
	upload, err := s.GetUpload(ctx, uploadID)
	if err != nil {
		return err
	}

	tempFile := upload.GetTempFile()
	file, err := os.OpenFile(tempFile, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open upload file: %w", err)
	}
	defer file.Close()

	// Seek to offset
	if _, err := file.Seek(offset, 0); err != nil {
		return fmt.Errorf("failed to seek to offset: %w", err)
	}

	// Write chunk
	written, err := io.Copy(file, chunk)
	if err != nil {
		return fmt.Errorf("failed to write chunk: %w", err)
	}

	// Update upload info
	upload.SetOffset(offset + written)
	upload.AddSize(written)

	return nil
}

// CompleteUpload implements UploadManager
func (s *Store) CompleteUpload(ctx context.Context, uploadID string, digest string) error {
	upload, err := s.GetUpload(ctx, uploadID)
	if err != nil {
		return err
	}

	tempFile := upload.GetTempFile()

	// Open temp file for reading
	file, err := os.Open(tempFile)
	if err != nil {
		return fmt.Errorf("failed to open upload file: %w", err)
	}
	defer file.Close()

	// Store as blob
	if err := s.PutBlob(ctx, digest, file); err != nil {
		return fmt.Errorf("failed to store blob: %w", err)
	}

	// Clean up
	s.uploads.Delete(uploadID)
	os.Remove(tempFile)

	return nil
}

// CancelUpload implements UploadManager
func (s *Store) CancelUpload(ctx context.Context, uploadID string) error {
	upload, err := s.GetUpload(ctx, uploadID)
	if err != nil {
		return err
	}

	tempFile := upload.GetTempFile()
	s.uploads.Delete(uploadID)
	os.Remove(tempFile)

	return nil
}

// GetUploadStatus implements UploadManager
func (s *Store) GetUploadStatus(ctx context.Context, uploadID string) (int64, error) {
	upload, err := s.GetUpload(ctx, uploadID)
	if err != nil {
		return 0, err
	}
	return upload.GetOffset(), nil
}

// Verify Store implements storage.Store interface
var _ storage.Store = (*Store)(nil)
