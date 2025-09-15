package metrics

import (
	"testing"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("NewRegistry() returned nil")
	}

	if registry.GetRegistry() == nil {
		t.Fatal("Registry.GetRegistry() returned nil")
	}

	// Test that metrics are properly initialized
	if registry.httpRequestsTotal == nil {
		t.Error("httpRequestsTotal metric not initialized")
	}
	if registry.httpRequestDuration == nil {
		t.Error("httpRequestDuration metric not initialized")
	}
	if registry.blobOperationsTotal == nil {
		t.Error("blobOperationsTotal metric not initialized")
	}
}

func TestRecordHTTPRequest(t *testing.T) {
	registry := NewRegistry()

	// Record a test HTTP request
	registry.RecordHTTPRequest("GET", "/v2/", "200", 0.5, 1024, 2048)

	// We can't easily test the actual metric values without accessing Prometheus internals,
	// but we can at least verify the method doesn't panic
}

func TestNormalizeEndpoint(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/v2/", "/v2/"},
		{"/v2/myrepo/blobs/sha256:abc123", "/v2/{name}/blobs/{digest}"},
		{"/v2/myrepo/manifests/latest", "/v2/{name}/manifests/{reference}"},
		{"/v2/myrepo/tags/list", "/v2/{name}/tags/list"},
		{"/v2/myrepo/blobs/uploads/uuid-123", "/v2/{name}/blobs/uploads/{uuid}"},
		{"/unknown/path", "/unknown/path"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeEndpoint(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeEndpoint(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"myrepo", []string{"myrepo"}},
		{"myrepo/blobs", []string{"myrepo", "blobs"}},
		{"myrepo/blobs/sha256:abc", []string{"myrepo", "blobs", "sha256:abc"}},
		{"namespace/myrepo/manifests/latest", []string{"namespace", "myrepo", "manifests", "latest"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitPath(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("splitPath(%q) returned %d segments, want %d", tt.input, len(result), len(tt.expected))
				return
			}
			for i, segment := range result {
				if segment != tt.expected[i] {
					t.Errorf("splitPath(%q)[%d] = %q, want %q", tt.input, i, segment, tt.expected[i])
				}
			}
		})
	}
}

func TestMetricsOperations(t *testing.T) {
	registry := NewRegistry()

	// Test various metric operations
	registry.RecordBlobOperation("upload", "myrepo", "success")
	registry.RecordManifestOperation("put", "myrepo", "success")
	registry.IncUploadSessionsActive()
	registry.DecUploadSessionsActive()
	registry.RecordStorageOperation("get_blob", "success", 0.1)
	registry.RecordStorageError("put_blob", "digest_mismatch")
	registry.SetStorageDiskUsage(1024*1024*1024) // 1GB
	registry.SetRepositoriesTotal(10)
	registry.SetTagsTotal(25)

	// If we get here without panicking, the basic operations work
}