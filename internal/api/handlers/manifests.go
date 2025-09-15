package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jbpratt/octoserve/internal/api"
	"github.com/jbpratt/octoserve/internal/errors"
	"github.com/jbpratt/octoserve/internal/storage"
	"github.com/jbpratt/octoserve/pkg/types"
)

// ManifestHandler handles manifest operations
type ManifestHandler struct {
	store storage.Store
}

// NewManifestHandler creates a new manifest handler
func NewManifestHandler(store storage.Store) *ManifestHandler {
	return &ManifestHandler{
		store: store,
	}
}

// GetManifest handles GET /v2/{name}/manifests/{reference}
func (h *ManifestHandler) GetManifest(w http.ResponseWriter, r *http.Request) {
	repo := api.GetParam(r, "name")
	reference := api.GetParam(r, "reference")

	// Get manifest
	manifest, err := h.store.GetManifest(r.Context(), repo, reference)
	if err != nil {
		if err == storage.ErrManifestNotFound {
			errors.WriteErrorResponse(w, http.StatusNotFound,
				errors.ManifestUnknown(reference))
			return
		}
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	// Set content type based on manifest media type
	contentType := manifest.MediaType
	if contentType == "" {
		contentType = types.MediaTypes.ImageManifest
	}

	// Handle content negotiation
	if accept := r.Header.Get("Accept"); accept != "" {
		if !isAcceptableMediaType(accept, contentType) {
			errors.WriteErrorResponse(w, http.StatusNotAcceptable,
				errors.ManifestInvalid("unsupported media type"))
			return
		}
	}

	// Marshal manifest
	data, err := json.Marshal(manifest)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	// Set headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Docker-Content-Digest", manifest.Digest)

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// HeadManifest handles HEAD /v2/{name}/manifests/{reference}
func (h *ManifestHandler) HeadManifest(w http.ResponseWriter, r *http.Request) {
	repo := api.GetParam(r, "name")
	reference := api.GetParam(r, "reference")

	// Check if manifest exists
	exists, err := h.store.ManifestExists(r.Context(), repo, reference)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	if !exists {
		errors.WriteErrorResponse(w, http.StatusNotFound,
			errors.ManifestUnknown(reference))
		return
	}

	// Get manifest for headers
	manifest, err := h.store.GetManifest(r.Context(), repo, reference)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	// Set content type
	contentType := manifest.MediaType
	if contentType == "" {
		contentType = types.MediaTypes.ImageManifest
	}

	// Set headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(manifest.Size, 10))
	w.Header().Set("Docker-Content-Digest", manifest.Digest)

	w.WriteHeader(http.StatusOK)
}

// PutManifest handles PUT /v2/{name}/manifests/{reference}
func (h *ManifestHandler) PutManifest(w http.ResponseWriter, r *http.Request) {
	repo := api.GetParam(r, "name")
	reference := api.GetParam(r, "reference")

	// Read manifest data
	data, err := io.ReadAll(r.Body)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusBadRequest,
			errors.ManifestInvalid("failed to read manifest"))
		return
	}

	// Parse manifest
	manifest, err := types.ParseManifest(data)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusBadRequest,
			errors.ManifestInvalid("invalid manifest format"))
		return
	}

	// Validate content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && manifest.MediaType != "" {
		if contentType != manifest.MediaType {
			errors.WriteErrorResponse(w, http.StatusBadRequest,
				errors.ManifestInvalid("content type mismatch"))
			return
		}
	}

	// Set media type if not present
	if manifest.MediaType == "" {
		if contentType != "" {
			manifest.MediaType = contentType
		} else {
			manifest.MediaType = types.MediaTypes.ImageManifest
		}
	}

	// Calculate digest
	manifest.Digest = calculateDigest(data)

	// Verify referenced blobs exist
	if err := h.verifyManifestBlobs(r, manifest); err != nil {
		errors.WriteErrorResponse(w, http.StatusBadRequest, err)
		return
	}

	// Store manifest
	err = h.store.PutManifest(r.Context(), repo, reference, manifest)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	// Return location
	location := r.URL.Path

	w.Header().Set("Location", location)
	w.Header().Set("Docker-Content-Digest", manifest.Digest)

	// Set OCI-Subject header if manifest has a subject field
	if manifest.Subject != nil && manifest.Subject.Digest != "" {
		w.Header().Set("OCI-Subject", manifest.Subject.Digest)
	}

	w.WriteHeader(http.StatusCreated)
}

// DeleteManifest handles DELETE /v2/{name}/manifests/{reference}
func (h *ManifestHandler) DeleteManifest(w http.ResponseWriter, r *http.Request) {
	repo := api.GetParam(r, "name")
	reference := api.GetParam(r, "reference")

	// Check if manifest exists
	exists, err := h.store.ManifestExists(r.Context(), repo, reference)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	if !exists {
		errors.WriteErrorResponse(w, http.StatusNotFound,
			errors.ManifestUnknown(reference))
		return
	}

	// Delete manifest
	err = h.store.DeleteManifest(r.Context(), repo, reference)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// ListTags handles GET /v2/{name}/tags/list
func (h *ManifestHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	repo := api.GetParam(r, "name")

	// Get query parameters for pagination
	nStr := r.URL.Query().Get("n")
	last := r.URL.Query().Get("last")

	// Get all tags
	allTags, err := h.store.ListTags(r.Context(), repo)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	// Apply pagination
	tags := allTags
	if last != "" {
		tags = filterTagsAfter(allTags, last)
	}

	if nStr != "" {
		if n, err := strconv.Atoi(nStr); err == nil && n > 0 {
			if len(tags) > n {
				tags = tags[:n]
				// Set Link header for pagination
				nextLast := tags[len(tags)-1]
				linkURL := r.URL
				q := linkURL.Query()
				q.Set("last", nextLast)
				linkURL.RawQuery = q.Encode()
				w.Header().Set("Link", `<`+linkURL.String()+`>; rel="next"`)
			}
		}
	}

	// Create response
	response := struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}{
		Name: repo,
		Tags: tags,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// verifyManifestBlobs checks that all referenced blobs exist
func (h *ManifestHandler) verifyManifestBlobs(r *http.Request, manifest *types.Manifest) *errors.OCIError {
	// Check config blob if present
	if manifest.Config.Digest != "" {
		exists, err := h.store.BlobExists(r.Context(), manifest.Config.Digest)
		if err != nil {
			return errors.NewOCIError("UNKNOWN", "internal server error", err.Error())
		}
		if !exists {
			return errors.ManifestBlobUnknown(manifest.Config.Digest)
		}
	}

	// Check layer blobs
	for _, layer := range manifest.Layers {
		if layer.Digest != "" {
			exists, err := h.store.BlobExists(r.Context(), layer.Digest)
			if err != nil {
				return errors.NewOCIError("UNKNOWN", "internal server error", err.Error())
			}
			if !exists {
				return errors.ManifestBlobUnknown(layer.Digest)
			}
		}
	}

	// For image index, referenced manifests might not exist yet - this is allowed by OCI spec
	// We'll skip validation of referenced manifests in image indexes

	return nil
}

// isAcceptableMediaType checks if the media type is acceptable
func isAcceptableMediaType(accept, mediaType string) bool {
	// Simple implementation - check if media type is in accept header
	return strings.Contains(accept, mediaType) || strings.Contains(accept, "*/*")
}

// filterTagsAfter returns tags that come after the given tag lexicographically
func filterTagsAfter(tags []string, after string) []string {
	var result []string
	for _, tag := range tags {
		if tag > after {
			result = append(result, tag)
		}
	}
	return result
}

// calculateDigest calculates the SHA256 digest of data
func calculateDigest(data []byte) string {
	hasher := sha256.New()
	hasher.Write(data)
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}
