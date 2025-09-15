package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPMetrics(t *testing.T) {
	registry := NewRegistry()

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	})

	// Wrap with metrics middleware
	wrappedHandler := HTTPMetrics(registry)(testHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "/v2/", nil)
	w := httptest.NewRecorder()

	// Execute the request
	wrappedHandler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if body := w.Body.String(); body != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got %q", body)
	}
}

func TestHTTPMetricsWithNilRegistry(t *testing.T) {
	// Test with nil registry (metrics disabled)
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with metrics middleware (nil registry)
	wrappedHandler := HTTPMetrics(nil)(testHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "/v2/", nil)
	w := httptest.NewRecorder()

	// Execute the request
	wrappedHandler.ServeHTTP(w, req)

	// Should work normally even with nil registry
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSkipMetricsEndpoint(t *testing.T) {
	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with skip middleware
	wrappedHandler := SkipMetricsEndpoint("/metrics")(testHandler)

	// Test non-metrics endpoint
	req := httptest.NewRequest("GET", "/v2/", nil)
	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	if !called {
		t.Error("Handler should have been called for non-metrics endpoint")
	}

	// Reset
	called = false

	// Test metrics endpoint
	req = httptest.NewRequest("GET", "/metrics", nil)
	w = httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	if !called {
		t.Error("Handler should have been called even for metrics endpoint")
	}
}
