package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler(t *testing.T) {
	registry := NewRegistry()
	
	// Test handler without authentication
	handler := Handler(registry, "", "")
	
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// Should contain Prometheus metrics format
	body := w.Body.String()
	if body == "" {
		t.Error("Expected non-empty response body")
	}
}

func TestHandlerWithAuth(t *testing.T) {
	registry := NewRegistry()
	
	// Test handler with authentication
	handler := Handler(registry, "admin", "secret")
	
	// Test without credentials
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
	
	// Test with correct credentials
	req = httptest.NewRequest("GET", "/metrics", nil)
	req.SetBasicAuth("admin", "secret")
	w = httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// Test with wrong credentials
	req = httptest.NewRequest("GET", "/metrics", nil)
	req.SetBasicAuth("admin", "wrong")
	w = httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestHandlerWithNilRegistry(t *testing.T) {
	// Test handler with nil registry (metrics disabled)
	handler := Handler(nil, "", "")
	
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}