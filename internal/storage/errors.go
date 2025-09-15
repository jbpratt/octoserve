package storage

import "errors"

// Common storage errors
var (
	ErrBlobNotFound     = errors.New("blob not found")
	ErrManifestNotFound = errors.New("manifest not found")
	ErrUploadNotFound   = errors.New("upload not found")
	ErrInvalidDigest    = errors.New("invalid digest")
	ErrDigestMismatch   = errors.New("digest mismatch")
	ErrInvalidReference = errors.New("invalid reference")
)