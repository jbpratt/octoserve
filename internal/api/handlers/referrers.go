package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jbpratt/octoserve/internal/api"
	"github.com/jbpratt/octoserve/internal/errors"
	"github.com/jbpratt/octoserve/internal/storage"
	"github.com/jbpratt/octoserve/pkg/types"
)

// ReferrersHandler handles referrers API operations
type ReferrersHandler struct {
	store storage.Store
}

// NewReferrersHandler creates a new referrers handler
func NewReferrersHandler(store storage.Store) *ReferrersHandler {
	return &ReferrersHandler{
		store: store,
	}
}

// GetReferrers handles GET /v2/{name}/referrers/{digest}
func (h *ReferrersHandler) GetReferrers(w http.ResponseWriter, r *http.Request) {
	repo := api.GetParam(r, "name")
	digest := api.GetParam(r, "digest")

	// Validate digest format
	if !isValidDigest(digest) {
		errors.WriteErrorResponse(w, http.StatusBadRequest,
			errors.NewOCIError("DIGEST_INVALID", "invalid digest", digest))
		return
	}

	// Get artifactType filter if present
	artifactType := r.URL.Query().Get("artifactType")

	// Get referrers from storage
	referrers, err := h.store.GetReferrers(r.Context(), repo, digest)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	// Apply artifactType filter if requested
	filteredReferrers := referrers
	if artifactType != "" {
		filteredReferrers = filterByArtifactType(referrers, artifactType)
		// Set filter applied header
		w.Header().Set("OCI-Filters-Applied", "artifactType")
	}

	// Create OCI image index response
	response := &types.ImageIndex{
		SchemaVersion: 2,
		MediaType:     types.MediaTypes.ImageIndex,
		Manifests:     filteredReferrers,
	}

	// Marshal response
	data, err := json.Marshal(response)
	if err != nil {
		errors.WriteErrorResponse(w, http.StatusInternalServerError,
			errors.NewOCIError("UNKNOWN", "internal server error", err.Error()))
		return
	}

	// Set headers and return response
	w.Header().Set("Content-Type", types.MediaTypes.ImageIndex)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// isValidDigest checks if the digest has valid format
func isValidDigest(digest string) bool {
	// Basic validation for algorithm:hex format
	parts := strings.SplitN(digest, ":", 2)
	if len(parts) != 2 {
		return false
	}

	algorithm, encoded := parts[0], parts[1]

	// Check algorithm is not empty and encoded part exists
	if algorithm == "" || encoded == "" {
		return false
	}

	// Basic hex validation (sha256 should be 64 chars, but be flexible)
	if len(encoded) < 32 {
		return false
	}

	return true
}

// filterByArtifactType filters referrers by artifactType
func filterByArtifactType(referrers []types.Descriptor, artifactType string) []types.Descriptor {
	var filtered []types.Descriptor
	for _, ref := range referrers {
		if ref.ArtifactType == artifactType {
			filtered = append(filtered, ref)
		}
	}
	return filtered
}
