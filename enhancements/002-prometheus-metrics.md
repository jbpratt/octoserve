# Enhancement 002: Prometheus Metrics Instrumentation

## Enhancement Information

- **Enhancement Number**: 002
- **Title**: Prometheus Metrics Instrumentation
- **Status**: Draft
- **Authors**: Development Team
- **Creation Date**: 2025-09-15
- **Last Updated**: 2025-09-15

## Summary

Add comprehensive Prometheus metrics instrumentation to octoserve to provide observability into registry operations, performance characteristics, and resource utilization. This enhancement will expose HTTP request metrics, OCI-specific operation metrics, storage backend performance metrics, and registry statistics through a standard `/metrics` endpoint that can be scraped by Prometheus.

The implementation will follow octoserve's minimal dependency philosophy while providing production-ready observability capabilities.

## Problem Statement

Currently, octoserve lacks observability metrics, making it difficult to:

- **Monitor Performance**: No visibility into request latencies, throughput, or error rates
- **Track Usage Patterns**: Unable to understand which repositories, tags, or operations are most common
- **Detect Issues**: No alerting capabilities for high error rates, slow operations, or resource constraints  
- **Capacity Planning**: No data on storage usage growth, upload patterns, or resource consumption
- **Operational Visibility**: Limited insight into active upload sessions, concurrent operations, or system health

This impacts production deployments where monitoring and alerting are essential for reliable service operation.

## Goals

- **Comprehensive HTTP Metrics**: Track all HTTP requests with duration, status codes, and endpoint patterns
- **OCI Operation Metrics**: Monitor blob uploads/downloads, manifest operations, and tag management
- **Storage Performance**: Measure storage operation latencies and error rates
- **Registry Statistics**: Expose repository counts, tag counts, active uploads, and disk usage
- **Standard Integration**: Provide Prometheus-compatible `/metrics` endpoint
- **Minimal Overhead**: Keep performance impact under 5% for typical workloads
- **Optional Configuration**: Allow metrics to be disabled or configured per environment
- **Clean Architecture**: Maintain existing interfaces and add metrics as a composable layer

## Non-Goals

- **Custom Dashboards**: Grafana dashboards will be provided as examples, not built-in
- **Alerting Rules**: Alert definitions are deployment-specific and not included
- **Alternative Formats**: Only Prometheus format supported (no StatsD, InfluxDB, etc.)
- **Historical Data**: Metrics collection only, no built-in time-series storage
- **Authentication Beyond Basic Auth**: Advanced auth mechanisms not included in initial version
- **Custom Metric APIs**: No user-defined metrics or runtime metric creation

## Proposal

### Overview

Implement Prometheus metrics using the official `prometheus/client_golang` library with a decorator pattern that wraps existing components without breaking current interfaces. Metrics will be collected through middleware for HTTP operations and storage wrappers for backend operations.

### Design Details

#### Architecture Integration

```
┌─────────────────────────────────────────┐
│               HTTP Layer                │
│  (Metrics Middleware + Existing Chain)  │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────┴───────────────────────┐
│            API Handlers                 │
│  (Unchanged - metrics via middleware)   │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────┴───────────────────────┐
│           Storage Interface             │
│  (Metrics Wrapper around existing impl) │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────┴───────────────────────┐
│         Filesystem Backend              │
│  (Unchanged - metrics via wrapper)      │
└─────────────────────────────────────────┘
```

#### Metrics Package Structure

```go
// internal/metrics/registry.go
type Registry struct {
    // HTTP metrics
    httpRequestsTotal    *prometheus.CounterVec
    httpRequestDuration  *prometheus.HistogramVec
    httpRequestSize      *prometheus.HistogramVec
    httpResponseSize     *prometheus.HistogramVec
    
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
}
```

#### HTTP Metrics Middleware

```go
// internal/metrics/middleware.go
func HTTPMetrics(registry *Registry) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            wrapped := &responseWriter{ResponseWriter: w}
            
            next.ServeHTTP(wrapped, r)
            
            // Record metrics
            duration := time.Since(start)
            registry.RecordHTTPRequest(r.Method, r.URL.Path, wrapped.status, duration)
        })
    }
}
```

#### Storage Metrics Wrapper

```go
// internal/metrics/storage.go
type StorageMetrics struct {
    storage.Store
    registry *Registry
}

func (s *StorageMetrics) GetBlob(ctx context.Context, digest string) (io.ReadCloser, error) {
    start := time.Now()
    reader, err := s.Store.GetBlob(ctx, digest)
    s.registry.RecordStorageOperation("get_blob", time.Since(start), err)
    return reader, err
}
```

#### Configuration Extension

```go
// internal/config/config.go
type MetricsConfig struct {
    Enabled   bool              `json:"enabled"`
    Endpoint  string            `json:"endpoint"`
    BasicAuth *BasicAuthConfig  `json:"basic_auth,omitempty"`
}

type Config struct {
    Server  ServerConfig  `json:"server"`
    Storage StorageConfig `json:"storage"`
    Logging LoggingConfig `json:"logging"`
    Metrics MetricsConfig `json:"metrics"`  // New field
}
```

### Implementation Phases

1. **Phase 1: Foundation (Week 1)**
   - Add Prometheus dependency
   - Create metrics package structure
   - Implement basic HTTP metrics middleware
   - Add metrics configuration

2. **Phase 2: Storage Instrumentation (Week 2)**
   - Implement storage metrics wrapper
   - Add operation duration and error tracking
   - Create custom collectors for registry stats

3. **Phase 3: Integration and Endpoints (Week 3)**
   - Integrate metrics into main application
   - Add `/metrics` endpoint with optional auth
   - Implement configuration-driven enabling/disabling

4. **Phase 4: Testing and Documentation (Week 4)**
   - Comprehensive unit and integration tests
   - Performance benchmarks
   - Update documentation and examples

## Alternative Approaches

### StatsD Integration
**Considered**: Using StatsD protocol for metrics
**Rejected**: Prometheus pull model is more common in container environments, and StatsD requires additional infrastructure

### Built-in Time Series Storage
**Considered**: Storing metrics internally with time-series database
**Rejected**: Increases complexity and dependencies; Prometheus ecosystem handles this better

### OpenTelemetry
**Considered**: Using OpenTelemetry for metrics
**Rejected**: More complex than needed for initial implementation; can be added later

### Custom Metrics Format
**Considered**: Creating custom metrics format
**Rejected**: Prometheus format is industry standard with excellent tooling support

## Testing Strategy

- **Unit Tests**: Mock Prometheus registries for isolated component testing
- **Integration Tests**: End-to-end metrics collection with real operations
- **Performance Tests**: Benchmark metrics overhead on request latency and throughput
- **Compatibility Tests**: Verify Prometheus scraping compatibility
- **Load Tests**: Validate metrics performance under high request volumes

## Security Considerations

- **Basic Authentication**: Optional username/password protection for `/metrics` endpoint
- **Endpoint Configuration**: Configurable metrics endpoint path to avoid conflicts
- **Label Sanitization**: Ensure no sensitive data leaks through metric labels
- **Resource Limits**: Prevent unbounded metric cardinality that could cause memory issues
- **Access Control**: Metrics endpoint should be restricted in production environments

## Performance Impact

- **Memory Usage**: Estimated 5-10MB additional memory for metrics storage
- **CPU Usage**: < 1% overhead for typical request volumes
- **Latency Impact**: < 1ms additional latency per request
- **Storage Impact**: No disk usage for metrics (in-memory only)
- **Network Impact**: Metrics endpoint adds ~50KB response size when scraped

Optimizations:
- Pre-allocated label values for hot paths
- Efficient histogram bucket selection
- Minimal allocations in request path
- Lazy initialization of metrics

## Backward Compatibility

- **Interface Preservation**: All existing storage and handler interfaces unchanged
- **Optional Feature**: Metrics can be completely disabled via configuration
- **Graceful Degradation**: Application functions normally if metrics fail
- **Configuration**: New metrics config section with sensible defaults
- **Zero Breaking Changes**: Existing deployments continue working without modification

## Migration Path

No migration required - this is a purely additive feature. Existing deployments:
1. Will continue working without any changes
2. Can opt-in to metrics by updating configuration
3. Can gradually enable metrics monitoring as needed

Configuration migration:
```json
// Before (still works)
{
  "server": {...},
  "storage": {...},
  "logging": {...}
}

// After (optional)
{
  "server": {...},
  "storage": {...}, 
  "logging": {...},
  "metrics": {
    "enabled": true,
    "endpoint": "/metrics"
  }
}
```

## Documentation Requirements

- **User Documentation**: How to enable and configure metrics
- **Operator Documentation**: Prometheus scraping configuration examples
- **API Documentation**: Complete metrics endpoint specification
- **Examples**: Sample Grafana dashboard and alert rules
- **Architecture Documentation**: Integration points and design decisions

## Future Considerations

- **Additional Storage Backends**: Metrics for S3, GCS, Azure storage when implemented
- **Advanced Authentication**: OAuth2, JWT tokens for metrics endpoint
- **Custom Labels**: User-configurable labels for multi-tenant deployments
- **Sampling**: Request sampling for high-volume deployments
- **OpenTelemetry**: Migration path to OpenTelemetry when ecosystem matures
- **Push Gateway**: Support for push-based metrics in restricted environments

## Implementation Timeline

- **Week 1**: Foundation and HTTP metrics
- **Week 2**: Storage metrics and custom collectors  
- **Week 3**: Integration and endpoint implementation
- **Week 4**: Testing, documentation, and polish

**Total Duration**: 4 weeks
**Milestone Deliverables**: Working `/metrics` endpoint with comprehensive registry observability

## References

- [Prometheus Go Client Library](https://github.com/prometheus/client_golang)
- [Prometheus Metrics Best Practices](https://prometheus.io/docs/practices/naming/)
- [HTTP Instrumentation Patterns](https://prometheus.io/docs/guides/go-application/)
- [Container Registry Metrics Examples](https://github.com/distribution/distribution/blob/main/docs/configuration.md#prometheus)
- [OCI Distribution Specification](https://github.com/opencontainers/distribution-spec)