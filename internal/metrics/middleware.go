package metrics

import (
	"net/http"
	"strconv"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture metrics
type responseWriter struct {
	http.ResponseWriter
	status       int
	size         int64
	wroteHeader  bool
}

func (rw *responseWriter) WriteHeader(status int) {
	if !rw.wroteHeader {
		rw.status = status
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(status)
	}
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	size, err := rw.ResponseWriter.Write(data)
	rw.size += int64(size)
	return size, err
}

// HTTPMetrics returns a middleware that records HTTP request metrics
func HTTPMetrics(registry *Registry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if registry == nil {
				// If metrics are disabled, pass through without recording
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			
			// Wrap the response writer to capture metrics
			wrapped := &responseWriter{
				ResponseWriter: w,
				status:         http.StatusOK,
			}

			// Get request size
			requestSize := r.ContentLength
			if requestSize < 0 {
				requestSize = 0
			}

			// Process the request
			next.ServeHTTP(wrapped, r)

			// Calculate metrics
			duration := time.Since(start).Seconds()
			endpoint := NormalizeEndpoint(r.URL.Path)
			status := strconv.Itoa(wrapped.status)

			// Record the metrics
			registry.RecordHTTPRequest(
				r.Method,
				endpoint,
				status,
				duration,
				requestSize,
				wrapped.size,
			)
		})
	}
}

// SkipMetricsEndpoint returns a middleware that skips metrics collection for the metrics endpoint itself
func SkipMetricsEndpoint(metricsPath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == metricsPath {
				// Skip metrics collection for the metrics endpoint to avoid recursion
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}