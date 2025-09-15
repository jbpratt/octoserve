package metrics

import (
	"context"
	"io"
	"time"

	"github.com/jbpratt/octoserve/internal/storage"
	"github.com/jbpratt/octoserve/pkg/types"
)

// StorageMetrics wraps a storage.Store with Prometheus metrics
type StorageMetrics struct {
	store    storage.Store
	registry *Registry
}

// NewStorageMetrics creates a new storage wrapper with metrics
func NewStorageMetrics(store storage.Store, registry *Registry) *StorageMetrics {
	return &StorageMetrics{
		store:    store,
		registry: registry,
	}
}

// recordOperation records a storage operation with timing and error handling
func (s *StorageMetrics) recordOperation(operation string, start time.Time, err error) {
	duration := time.Since(start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
		s.registry.RecordStorageError(operation, classifyError(err))
	}
	s.registry.RecordStorageOperation(operation, status, duration)
}

// classifyError categorizes errors for metrics
func classifyError(err error) string {
	switch err {
	case storage.ErrBlobNotFound:
		return "blob_not_found"
	case storage.ErrManifestNotFound:
		return "manifest_not_found"
	case storage.ErrUploadNotFound:
		return "upload_not_found"
	case storage.ErrInvalidDigest:
		return "invalid_digest"
	case storage.ErrDigestMismatch:
		return "digest_mismatch"
	case storage.ErrInvalidReference:
		return "invalid_reference"
	default:
		return "unknown"
	}
}

// GetBlob implements storage.BlobStore
func (s *StorageMetrics) GetBlob(ctx context.Context, digest string) (io.ReadCloser, error) {
	start := time.Now()
	reader, err := s.store.GetBlob(ctx, digest)
	s.recordOperation("get_blob", start, err)
	return reader, err
}

// PutBlob implements storage.BlobStore
func (s *StorageMetrics) PutBlob(ctx context.Context, digest string, content io.Reader) error {
	start := time.Now()
	err := s.store.PutBlob(ctx, digest, content)
	s.recordOperation("put_blob", start, err)
	return err
}

// BlobExists implements storage.BlobStore
func (s *StorageMetrics) BlobExists(ctx context.Context, digest string) (bool, error) {
	start := time.Now()
	exists, err := s.store.BlobExists(ctx, digest)
	s.recordOperation("blob_exists", start, err)
	return exists, err
}

// DeleteBlob implements storage.BlobStore
func (s *StorageMetrics) DeleteBlob(ctx context.Context, digest string) error {
	start := time.Now()
	err := s.store.DeleteBlob(ctx, digest)
	s.recordOperation("delete_blob", start, err)
	return err
}

// GetBlobSize implements storage.BlobStore
func (s *StorageMetrics) GetBlobSize(ctx context.Context, digest string) (int64, error) {
	start := time.Now()
	size, err := s.store.GetBlobSize(ctx, digest)
	s.recordOperation("get_blob_size", start, err)
	return size, err
}

// GetManifest implements storage.ManifestStore
func (s *StorageMetrics) GetManifest(ctx context.Context, repo, reference string) (*types.Manifest, error) {
	start := time.Now()
	manifest, err := s.store.GetManifest(ctx, repo, reference)
	s.recordOperation("get_manifest", start, err)
	if s.registry != nil && err == nil {
		s.registry.RecordManifestOperation("get", repo, "success")
	} else if s.registry != nil {
		s.registry.RecordManifestOperation("get", repo, "error")
	}
	return manifest, err
}

// PutManifest implements storage.ManifestStore
func (s *StorageMetrics) PutManifest(ctx context.Context, repo, reference string, manifest *types.Manifest) error {
	start := time.Now()
	err := s.store.PutManifest(ctx, repo, reference, manifest)
	s.recordOperation("put_manifest", start, err)
	if s.registry != nil && err == nil {
		s.registry.RecordManifestOperation("put", repo, "success")
	} else if s.registry != nil {
		s.registry.RecordManifestOperation("put", repo, "error")
	}
	return err
}

// DeleteManifest implements storage.ManifestStore
func (s *StorageMetrics) DeleteManifest(ctx context.Context, repo, reference string) error {
	start := time.Now()
	err := s.store.DeleteManifest(ctx, repo, reference)
	s.recordOperation("delete_manifest", start, err)
	if s.registry != nil && err == nil {
		s.registry.RecordManifestOperation("delete", repo, "success")
	} else if s.registry != nil {
		s.registry.RecordManifestOperation("delete", repo, "error")
	}
	return err
}

// ListTags implements storage.ManifestStore
func (s *StorageMetrics) ListTags(ctx context.Context, repo string) ([]string, error) {
	start := time.Now()
	tags, err := s.store.ListTags(ctx, repo)
	s.recordOperation("list_tags", start, err)
	return tags, err
}

// ManifestExists implements storage.ManifestStore
func (s *StorageMetrics) ManifestExists(ctx context.Context, repo, reference string) (bool, error) {
	start := time.Now()
	exists, err := s.store.ManifestExists(ctx, repo, reference)
	s.recordOperation("manifest_exists", start, err)
	return exists, err
}

// GetReferrers implements storage.ManifestStore
func (s *StorageMetrics) GetReferrers(ctx context.Context, repo, digest string) ([]types.Descriptor, error) {
	start := time.Now()
	referrers, err := s.store.GetReferrers(ctx, repo, digest)
	s.recordOperation("get_referrers", start, err)
	return referrers, err
}

// CreateUpload implements storage.UploadManager
func (s *StorageMetrics) CreateUpload(ctx context.Context, repo string) (*types.Upload, error) {
	start := time.Now()
	upload, err := s.store.CreateUpload(ctx, repo)
	s.recordOperation("create_upload", start, err)
	if s.registry != nil && err == nil {
		s.registry.IncUploadSessionsActive()
	}
	return upload, err
}

// GetUpload implements storage.UploadManager
func (s *StorageMetrics) GetUpload(ctx context.Context, uploadID string) (*types.Upload, error) {
	start := time.Now()
	upload, err := s.store.GetUpload(ctx, uploadID)
	s.recordOperation("get_upload", start, err)
	return upload, err
}

// WriteUploadChunk implements storage.UploadManager
func (s *StorageMetrics) WriteUploadChunk(ctx context.Context, uploadID string, offset int64, chunk io.Reader) error {
	start := time.Now()
	err := s.store.WriteUploadChunk(ctx, uploadID, offset, chunk)
	s.recordOperation("write_upload_chunk", start, err)
	return err
}

// CompleteUpload implements storage.UploadManager
func (s *StorageMetrics) CompleteUpload(ctx context.Context, uploadID string, digest string) error {
	start := time.Now()
	err := s.store.CompleteUpload(ctx, uploadID, digest)
	s.recordOperation("complete_upload", start, err)
	if s.registry != nil {
		s.registry.DecUploadSessionsActive()
		if err == nil {
			// Extract repository from upload if possible
			if upload, getErr := s.store.GetUpload(ctx, uploadID); getErr == nil {
				s.registry.RecordBlobOperation("upload", upload.Repository, "success")
			}
		}
	}
	return err
}

// CancelUpload implements storage.UploadManager
func (s *StorageMetrics) CancelUpload(ctx context.Context, uploadID string) error {
	start := time.Now()
	err := s.store.CancelUpload(ctx, uploadID)
	s.recordOperation("cancel_upload", start, err)
	if s.registry != nil {
		s.registry.DecUploadSessionsActive()
	}
	return err
}

// GetUploadStatus implements storage.UploadManager
func (s *StorageMetrics) GetUploadStatus(ctx context.Context, uploadID string) (int64, error) {
	start := time.Now()
	offset, err := s.store.GetUploadStatus(ctx, uploadID)
	s.recordOperation("get_upload_status", start, err)
	return offset, err
}

// Verify StorageMetrics implements storage.Store interface
var _ storage.Store = (*StorageMetrics)(nil)
