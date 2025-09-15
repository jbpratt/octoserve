package storage

import (
	"context"
	"io"

	"github.com/jbpratt/octoserve/pkg/types"
)

// Store combines all storage interfaces
type Store interface {
	BlobStore
	ManifestStore
	UploadManager
}

// BlobStore handles blob storage operations
type BlobStore interface {
	// GetBlob returns a reader for the blob content
	GetBlob(ctx context.Context, digest string) (io.ReadCloser, error)

	// PutBlob stores blob content
	PutBlob(ctx context.Context, digest string, content io.Reader) error

	// BlobExists checks if a blob exists
	BlobExists(ctx context.Context, digest string) (bool, error)

	// DeleteBlob removes a blob
	DeleteBlob(ctx context.Context, digest string) error

	// GetBlobSize returns the size of a blob
	GetBlobSize(ctx context.Context, digest string) (int64, error)
}

// ManifestStore handles manifest storage operations
type ManifestStore interface {
	// GetManifest returns a manifest by repository and reference
	GetManifest(ctx context.Context, repo, reference string) (*types.Manifest, error)

	// PutManifest stores a manifest
	PutManifest(ctx context.Context, repo, reference string, manifest *types.Manifest) error

	// DeleteManifest removes a manifest
	DeleteManifest(ctx context.Context, repo, reference string) error

	// ListTags returns all tags for a repository
	ListTags(ctx context.Context, repo string) ([]string, error)

	// ManifestExists checks if a manifest exists
	ManifestExists(ctx context.Context, repo, reference string) (bool, error)
}

// UploadManager handles blob upload sessions
type UploadManager interface {
	// CreateUpload starts a new upload session
	CreateUpload(ctx context.Context, repo string) (*types.Upload, error)

	// GetUpload retrieves an upload session
	GetUpload(ctx context.Context, uploadID string) (*types.Upload, error)

	// WriteUploadChunk writes a chunk to an upload session
	WriteUploadChunk(ctx context.Context, uploadID string, offset int64, chunk io.Reader) error

	// CompleteUpload finalizes an upload with the given digest
	CompleteUpload(ctx context.Context, uploadID string, digest string) error

	// CancelUpload cancels an upload session
	CancelUpload(ctx context.Context, uploadID string) error

	// GetUploadStatus returns the current upload status
	GetUploadStatus(ctx context.Context, uploadID string) (offset int64, err error)
}
