package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jbpratt/octoserve/internal/storage/filesystem"
)

func TestRegistryHandler_CheckAPI(t *testing.T) {
	// Create temporary directory for test storage
	tempDir, err := os.MkdirTemp("", "octoserve-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create store
	store, err := filesystem.New(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create handler
	handler := NewRegistryHandler(store)

	// Create test request
	req := httptest.NewRequest("GET", "/v2/", nil)
	w := httptest.NewRecorder()

	// Call handler
	handler.CheckAPI(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Check response body
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	expectedMessage := "OCI Registry API v2"
	if response["message"] != expectedMessage {
		t.Errorf("Expected message %s, got %s", expectedMessage, response["message"])
	}
}

func TestRegistryHandler_Methods(t *testing.T) {
	// Create temporary directory for test storage
	tempDir, err := os.MkdirTemp("", "octoserve-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create store
	store, err := filesystem.New(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create handler
	handler := NewRegistryHandler(store)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "GET request",
			method:         "GET",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "HEAD request", 
			method:         "HEAD",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/v2/", nil)
			w := httptest.NewRecorder()

			handler.CheckAPI(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}