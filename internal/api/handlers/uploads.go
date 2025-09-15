package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jbpratt/octoserve/internal/api"
	"github.com/jbpratt/octoserve/internal/errors"
	"github.com/jbpratt/octoserve/internal/storage"
)

// UploadHandler handles blob upload operations
type UploadHandler struct {
	store storage.Store
}

// NewUploadHandler creates a new upload handler
func NewUploadHandler(store storage.Store) *UploadHandler {
	return &UploadHandler{
		store: store,
	}
}

// StartUpload handles POST /v2/{name}/blobs/uploads/
func (h *UploadHandler) StartUpload(w http.ResponseWriter, r *http.Request) {
	repo := api.GetParam(r, "name")

	// Check for monolithic upload (digest in query params)
	if digest := r.URL.Query().Get("digest"); digest != "" {
		h.handleMonolithicUpload(w, r, repo, digest)
		return
	}

	// Check for cross-repository blob mount
	if mountDigest := r.URL.Query().Get("mount"); mountDigest != "" {
		fromRepo := r.URL.Query().Get("from")
		if fromRepo != "" {
			h.handleBlobMount(w, r, repo, mountDigest, fromRepo)
			return
		}
	}

	// Start chunked upload
	upload, err := h.store.CreateUpload(r.Context(), repo)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	// Return upload location
	location := fmt.Sprintf("/v2/%s/blobs/uploads/%s", repo, upload.ID)

	w.Header().Set("Location", location)
	w.Header().Set("Range", "0-0")
	w.WriteHeader(http.StatusAccepted)
}

// PatchUpload handles PATCH /v2/{name}/blobs/uploads/{uuid}
func (h *UploadHandler) PatchUpload(w http.ResponseWriter, r *http.Request) {
	repo := api.GetParam(r, "name")
	uploadID := api.GetParam(r, "uuid")

	// Get current upload status to validate offset
	currentOffset, err := h.store.GetUploadStatus(r.Context(), uploadID)
	if err != nil {
		if err == storage.ErrUploadNotFound {
			errors.WriteErrorResponse(w, http.StatusNotFound,
				errors.BlobUploadUnknown(uploadID))
			return
		}
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	// Parse Content-Range header
	contentRange := r.Header.Get("Content-Range")
	requestedOffset, err := parseContentRange(contentRange)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusBadRequest,
			errors.BlobUploadInvalid("invalid content range"))
		return
	}

	// Validate that the requested offset matches the expected offset
	if requestedOffset != currentOffset {
		// Return 416 Range Not Satisfiable for out-of-order or duplicate chunks
		w.Header().Set("Location", fmt.Sprintf("/v2/%s/blobs/uploads/%s", repo, uploadID))
		w.Header().Set("Range", fmt.Sprintf("0-%d", currentOffset-1))
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	// Write chunk
	err = h.store.WriteUploadChunk(r.Context(), uploadID, requestedOffset, r.Body)
	if err != nil {
		if err == storage.ErrUploadNotFound {
			errors.WriteErrorResponse(w, http.StatusNotFound,
				errors.BlobUploadUnknown(uploadID))
			return
		}
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	// Get updated status after write
	newOffset, err := h.store.GetUploadStatus(r.Context(), uploadID)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	// Return updated location and range
	location := fmt.Sprintf("/v2/%s/blobs/uploads/%s", repo, uploadID)

	w.Header().Set("Location", location)
	w.Header().Set("Range", fmt.Sprintf("0-%d", newOffset-1))
	w.WriteHeader(http.StatusAccepted)
}

// PutUpload handles PUT /v2/{name}/blobs/uploads/{uuid}?digest={digest}
func (h *UploadHandler) PutUpload(w http.ResponseWriter, r *http.Request) {
	repo := api.GetParam(r, "name")
	uploadID := api.GetParam(r, "uuid")
	digest := r.URL.Query().Get("digest")

	if digest == "" {
		errors.WriteErrorResponse(w, http.StatusBadRequest,
			errors.DigestInvalid("digest parameter required"))
		return
	}

	// If there's a body, write it as the final chunk
	if r.ContentLength > 0 {
		contentRange := r.Header.Get("Content-Range")
		offset, err := parseContentRange(contentRange)
		if err != nil {
			// If no range specified, assume continuing from current offset
			currentOffset, err := h.store.GetUploadStatus(r.Context(), uploadID)
			if err != nil {
				errors.WriteErrorResponse(w, http.StatusInternalServerError,
					errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
				return
			}
			offset = currentOffset
		}

		err = h.store.WriteUploadChunk(r.Context(), uploadID, offset, r.Body)
		if err != nil {
			if err == storage.ErrUploadNotFound {
				errors.WriteErrorResponse(w, http.StatusNotFound,
					errors.BlobUploadUnknown(uploadID))
				return
			}
			errors.WriteErrorResponse(w, http.StatusInternalServerError,
				errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
			return
		}
	}

	// Complete upload
	err := h.store.CompleteUpload(r.Context(), uploadID, digest)
	if err != nil {
		if err == storage.ErrUploadNotFound {
			errors.WriteErrorResponse(w, http.StatusNotFound,
				errors.BlobUploadUnknown(uploadID))
			return
		}
		errors.WriteErrorResponse(w, http.StatusBadRequest,
			errors.DigestInvalid(digest))
		return
	}

	// Return blob location
	location := fmt.Sprintf("/v2/%s/blobs/%s", repo, digest)

	w.Header().Set("Location", location)
	w.Header().Set("Docker-Content-Digest", digest)
	w.WriteHeader(http.StatusCreated)
}

// GetUpload handles GET /v2/{name}/blobs/uploads/{uuid}
func (h *UploadHandler) GetUpload(w http.ResponseWriter, r *http.Request) {
	repo := api.GetParam(r, "name")
	uploadID := api.GetParam(r, "uuid")

	// Get upload status
	offset, err := h.store.GetUploadStatus(r.Context(), uploadID)
	if err != nil {
		if err == storage.ErrUploadNotFound {
			errors.WriteErrorResponse(w, http.StatusNotFound,
				errors.BlobUploadUnknown(uploadID))
			return
		}
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	// Return current status
	location := fmt.Sprintf("/v2/%s/blobs/uploads/%s", repo, uploadID)

	w.Header().Set("Location", location)
	w.Header().Set("Range", fmt.Sprintf("0-%d", offset-1))
	w.WriteHeader(http.StatusNoContent)
}

// DeleteUpload handles DELETE /v2/{name}/blobs/uploads/{uuid}
func (h *UploadHandler) DeleteUpload(w http.ResponseWriter, r *http.Request) {
	uploadID := api.GetParam(r, "uuid")

	err := h.store.CancelUpload(r.Context(), uploadID)
	if err != nil {
		if err == storage.ErrUploadNotFound {
			errors.WriteErrorResponse(w, http.StatusNotFound,
				errors.BlobUploadUnknown(uploadID))
			return
		}
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleMonolithicUpload handles single POST upload with digest
func (h *UploadHandler) handleMonolithicUpload(w http.ResponseWriter, r *http.Request, repo, digest string) {
	// Store blob directly
	err := h.store.PutBlob(r.Context(), digest, r.Body)
	if err != nil {
		if err == storage.ErrDigestMismatch {
			errors.WriteErrorResponse(w, http.StatusBadRequest,
				errors.DigestInvalid(digest))
			return
		}
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	// Return blob location
	location := fmt.Sprintf("/v2/%s/blobs/%s", repo, digest)

	w.Header().Set("Location", location)
	w.Header().Set("Docker-Content-Digest", digest)
	w.WriteHeader(http.StatusCreated)
}

// handleBlobMount handles cross-repository blob mounting
func (h *UploadHandler) handleBlobMount(w http.ResponseWriter, r *http.Request, repo, digest, fromRepo string) {
	// Check if blob exists in source repository
	exists, err := h.store.BlobExists(r.Context(), digest)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	if !exists {
		// Fall back to regular upload
		upload, err := h.store.CreateUpload(r.Context(), repo)
		if err != nil {
			errors.WriteErrorResponse(w, http.StatusInternalServerError,
				errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
			return
		}

		location := fmt.Sprintf("/v2/%s/blobs/uploads/%s", repo, upload.ID)
		w.Header().Set("Location", location)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Blob exists, return mounted location
	location := fmt.Sprintf("/v2/%s/blobs/%s", repo, digest)

	w.Header().Set("Location", location)
	w.Header().Set("Docker-Content-Digest", digest)
	w.WriteHeader(http.StatusCreated)
}

// parseContentRange parses Content-Range header to extract offset
func parseContentRange(contentRange string) (int64, error) {
	if contentRange == "" {
		return 0, nil
	}

	// Parse "bytes=start-end" or "start-end" format
	rangeStr := strings.TrimPrefix(contentRange, "bytes=")
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid content range format")
	}

	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid start offset: %w", err)
	}

	return start, nil
}
