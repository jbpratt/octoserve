package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/jbpratt/octoserve/internal/api"
	"github.com/jbpratt/octoserve/internal/errors"
	"github.com/jbpratt/octoserve/internal/storage"
)

// BlobHandler handles blob operations
type BlobHandler struct {
	store storage.Store
}

// NewBlobHandler creates a new blob handler
func NewBlobHandler(store storage.Store) *BlobHandler {
	return &BlobHandler{
		store: store,
	}
}

// GetBlob handles GET /v2/{name}/blobs/{digest}
func (h *BlobHandler) GetBlob(w http.ResponseWriter, r *http.Request) {
	digest := api.GetParam(r, "digest")

	// Check if blob exists
	exists, err := h.store.BlobExists(r.Context(), digest)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	if !exists {
		errors.WriteErrorResponse(w, http.StatusNotFound,
			errors.BlobUnknown(digest))
		return
	}

	// Get blob content
	reader, err := h.store.GetBlob(r.Context(), digest)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}
	defer reader.Close()

	// Get blob size for Content-Length header
	size, err := h.store.GetBlobSize(r.Context(), digest)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	// Set headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Docker-Content-Digest", digest)

	// Handle range requests for partial content
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		h.handleRangeRequest(w, r, reader, size, rangeHeader)
		return
	}

	// Stream blob content
	w.WriteHeader(http.StatusOK)
	_, err = io.Copy(w, reader)
	if err != nil {
		// Log error, but can't change response at this point
		return
	}
}

// HeadBlob handles HEAD /v2/{name}/blobs/{digest}
func (h *BlobHandler) HeadBlob(w http.ResponseWriter, r *http.Request) {
	digest := api.GetParam(r, "digest")

	// Check if blob exists
	exists, err := h.store.BlobExists(r.Context(), digest)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	if !exists {
		errors.WriteErrorResponse(w, http.StatusNotFound,
			errors.BlobUnknown(digest))
		return
	}

	// Get blob size
	size, err := h.store.GetBlobSize(r.Context(), digest)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	// Set headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Docker-Content-Digest", digest)

	w.WriteHeader(http.StatusOK)
}

// DeleteBlob handles DELETE /v2/{name}/blobs/{digest}
func (h *BlobHandler) DeleteBlob(w http.ResponseWriter, r *http.Request) {
	digest := api.GetParam(r, "digest")

	// Check if blob exists
	exists, err := h.store.BlobExists(r.Context(), digest)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	if !exists {
		errors.WriteErrorResponse(w, http.StatusNotFound,
			errors.BlobUnknown(digest))
		return
	}

	// Delete blob
	if err := h.store.DeleteBlob(r.Context(), digest); err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// handleRangeRequest handles HTTP range requests for partial content
func (h *BlobHandler) handleRangeRequest(w http.ResponseWriter, r *http.Request, reader io.ReadCloser, size int64, rangeHeader string) {
	// Simple range parsing for "bytes=start-end" format
	// This is a basic implementation - full HTTP range support would be more complex

	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", size-1, size))
	w.WriteHeader(http.StatusPartialContent)

	// For now, just return the full content
	// A full implementation would parse the range and seek appropriately
	io.Copy(w, reader)
}
