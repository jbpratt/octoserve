package types

import (
	"sync"
	"time"
)

// Upload represents an active blob upload session
type Upload struct {
	ID         string
	Repository string
	StartedAt  time.Time
	Size       int64
	mu         sync.RWMutex
	offset     int64
	tempFile   string
}

// NewUpload creates a new upload session
func NewUpload(id, repository string) *Upload {
	return &Upload{
		ID:         id,
		Repository: repository,
		StartedAt:  time.Now(),
		Size:       0,
		offset:     0,
	}
}

// GetOffset returns the current upload offset
func (u *Upload) GetOffset() int64 {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.offset
}

// SetOffset sets the current upload offset
func (u *Upload) SetOffset(offset int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.offset = offset
}

// AddSize adds to the total uploaded size
func (u *Upload) AddSize(size int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Size += size
}

// GetSize returns the total uploaded size
func (u *Upload) GetSize() int64 {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.Size
}

// SetTempFile sets the temporary file path for this upload
func (u *Upload) SetTempFile(path string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.tempFile = path
}

// GetTempFile returns the temporary file path
func (u *Upload) GetTempFile() string {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.tempFile
}