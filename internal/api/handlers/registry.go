package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jbpratt/octoserve/internal/storage"
)

// RegistryHandler handles registry-level operations
type RegistryHandler struct {
	store storage.Store
}

// NewRegistryHandler creates a new registry handler
func NewRegistryHandler(store storage.Store) *RegistryHandler {
	return &RegistryHandler{
		store: store,
	}
}

// CheckAPI handles GET /v2/ - API version check
func (h *RegistryHandler) CheckAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	// Simple response indicating API v2 support
	response := map[string]string{
		"message": "OCI Registry API v2",
	}
	
	json.NewEncoder(w).Encode(response)
}