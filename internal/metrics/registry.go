package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Registry holds all Prometheus metrics for the octoserve registry
type Registry struct {
	// HTTP metrics
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	httpRequestSize     *prometheus.HistogramVec
	httpResponseSize    *prometheus.HistogramVec

	// OCI operation metrics
	blobOperationsTotal     *prometheus.CounterVec
	manifestOperationsTotal *prometheus.CounterVec
	uploadSessionsActive    prometheus.Gauge

	// Storage metrics
	storageOperationDuration *prometheus.HistogramVec
	storageErrorsTotal       *prometheus.CounterVec
	storageDiskUsage         prometheus.Gauge

	// Registry statistics
	repositoriesTotal prometheus.Gauge
	tagsTotal         prometheus.Gauge

	// Prometheus registry
	registry *prometheus.Registry
	once     sync.Once
}

// NewRegistry creates a new metrics registry with all metrics defined
func NewRegistry() *Registry {
	reg := prometheus.NewRegistry()

	m := &Registry{
		registry: reg,
	}

	m.initializeMetrics()
	return m
}

// initializeMetrics creates and registers all Prometheus metrics
func (m *Registry) initializeMetrics() {
	// HTTP metrics
	m.httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	m.httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	m.httpRequestSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "Size of HTTP request bodies in bytes",
			Buckets: prometheus.ExponentialBuckets(1024, 2, 10), // 1KB to 512MB
		},
		[]string{"method", "endpoint"},
	)

	m.httpResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "Size of HTTP response bodies in bytes",
			Buckets: prometheus.ExponentialBuckets(1024, 2, 10), // 1KB to 512MB
		},
		[]string{"method", "endpoint"},
	)

	// OCI operation metrics
	m.blobOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "registry_blob_operations_total",
			Help: "Total number of blob operations",
		},
		[]string{"operation", "repository", "status"},
	)

	m.manifestOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "registry_manifest_operations_total",
			Help: "Total number of manifest operations",
		},
		[]string{"operation", "repository", "status"},
	)

	m.uploadSessionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "registry_upload_sessions_active",
			Help: "Number of active upload sessions",
		},
	)

	// Storage metrics
	m.storageOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "storage_operation_duration_seconds",
			Help:    "Duration of storage operations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "status"},
	)

	m.storageErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "storage_errors_total",
			Help: "Total number of storage operation errors",
		},
		[]string{"operation", "error_type"},
	)

	m.storageDiskUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "storage_disk_usage_bytes",
			Help: "Total disk space used by the registry in bytes",
		},
	)

	// Registry statistics
	m.repositoriesTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "registry_repositories_total",
			Help: "Total number of repositories in the registry",
		},
	)

	m.tagsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "registry_tags_total",
			Help: "Total number of tags across all repositories",
		},
	)

	// Register all metrics
	m.registry.MustRegister(
		m.httpRequestsTotal,
		m.httpRequestDuration,
		m.httpRequestSize,
		m.httpResponseSize,
		m.blobOperationsTotal,
		m.manifestOperationsTotal,
		m.uploadSessionsActive,
		m.storageOperationDuration,
		m.storageErrorsTotal,
		m.storageDiskUsage,
		m.repositoriesTotal,
		m.tagsTotal,
	)
}

// GetRegistry returns the underlying Prometheus registry
func (m *Registry) GetRegistry() *prometheus.Registry {
	return m.registry
}

// RecordHTTPRequest records an HTTP request metric
func (m *Registry) RecordHTTPRequest(method, endpoint, status string, duration float64, requestSize, responseSize int64) {
	m.httpRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
	m.httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration)
	if requestSize > 0 {
		m.httpRequestSize.WithLabelValues(method, endpoint).Observe(float64(requestSize))
	}
	if responseSize > 0 {
		m.httpResponseSize.WithLabelValues(method, endpoint).Observe(float64(responseSize))
	}
}

// RecordBlobOperation records a blob operation metric
func (m *Registry) RecordBlobOperation(operation, repository, status string) {
	m.blobOperationsTotal.WithLabelValues(operation, repository, status).Inc()
}

// RecordManifestOperation records a manifest operation metric
func (m *Registry) RecordManifestOperation(operation, repository, status string) {
	m.manifestOperationsTotal.WithLabelValues(operation, repository, status).Inc()
}

// SetUploadSessionsActive sets the number of active upload sessions
func (m *Registry) SetUploadSessionsActive(count float64) {
	m.uploadSessionsActive.Set(count)
}

// IncUploadSessionsActive increments the active upload sessions counter
func (m *Registry) IncUploadSessionsActive() {
	m.uploadSessionsActive.Inc()
}

// DecUploadSessionsActive decrements the active upload sessions counter
func (m *Registry) DecUploadSessionsActive() {
	m.uploadSessionsActive.Dec()
}

// RecordStorageOperation records a storage operation metric
func (m *Registry) RecordStorageOperation(operation, status string, duration float64) {
	m.storageOperationDuration.WithLabelValues(operation, status).Observe(duration)
}

// RecordStorageError records a storage error metric
func (m *Registry) RecordStorageError(operation, errorType string) {
	m.storageErrorsTotal.WithLabelValues(operation, errorType).Inc()
}

// SetStorageDiskUsage sets the current disk usage in bytes
func (m *Registry) SetStorageDiskUsage(bytes float64) {
	m.storageDiskUsage.Set(bytes)
}

// SetRepositoriesTotal sets the total number of repositories
func (m *Registry) SetRepositoriesTotal(count float64) {
	m.repositoriesTotal.Set(count)
}

// SetTagsTotal sets the total number of tags
func (m *Registry) SetTagsTotal(count float64) {
	m.tagsTotal.Set(count)
}

// NormalizeEndpoint normalizes HTTP endpoints for consistent labeling
func NormalizeEndpoint(path string) string {
	// Normalize common OCI registry endpoints to avoid high cardinality
	if path == "/v2/" {
		return "/v2/"
	}
	
	// Extract endpoint patterns
	if len(path) >= 4 && path[:4] == "/v2/" {
		segments := splitPath(path[4:]) // Remove "/v2/" prefix
		
		switch len(segments) {
		case 0:
			return "/v2/"
		case 2:
			if segments[1] == "tags" {
				return "/v2/{name}/tags"
			}
		case 3:
			switch segments[1] {
			case "blobs":
				return "/v2/{name}/blobs/{digest}"
			case "manifests":
				return "/v2/{name}/manifests/{reference}"
			case "tags":
				if segments[2] == "list" {
					return "/v2/{name}/tags/list"
				}
			}
		case 4:
			if segments[1] == "blobs" && segments[2] == "uploads" {
				return "/v2/{name}/blobs/uploads/{uuid}"
			}
		}
	}
	
	// Fallback to the original path for unknown patterns
	return path
}

// splitPath splits a URL path into segments
func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}
	
	segments := []string{}
	start := 0
	
	for i, c := range path {
		if c == '/' {
			if i > start {
				segments = append(segments, path[start:i])
			}
			start = i + 1
		}
	}
	
	// Add the last segment
	if start < len(path) {
		segments = append(segments, path[start:])
	}
	
	return segments
}