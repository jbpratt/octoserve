# Enhancement 001: Basic OCI Registry Implementation

## Enhancement Information

- **Enhancement Number**: 001
- **Title**: Basic OCI Registry Implementation
- **Status**: Approved
- **Authors**: Development Team
- **Creation Date**: 2025-09-15
- **Last Updated**: 2025-09-15

## Summary

Implement a minimal but compliant OCI (Open Container Initiative) registry from scratch in Go using primarily the standard library. This registry will support the core functionality required to push and pull container images according to the OCI Distribution Specification v1.1.

## Problem Statement

Currently, there is no lightweight, dependency-minimal OCI registry implementation that can serve as:
- A learning tool for understanding OCI Distribution Specification
- A foundation for building custom registry solutions
- A reference implementation showcasing Go standard library capabilities
- A simple registry for development and testing environments

Existing solutions either have heavy dependency requirements or lack the simplicity needed for educational and lightweight deployment scenarios.

## Goals

- Implement core OCI Distribution Specification v1.1 compliance
- Support basic push/pull workflows for container images
- Minimize external dependencies (use Go standard library wherever possible)
- Provide clean, extensible architecture for future enhancements
- Ensure proper error handling according to OCI specification
- Support filesystem-based storage backend
- Include comprehensive test coverage

## Non-Goals

- Advanced authentication/authorization mechanisms (basic auth only)
- Multi-backend storage support (filesystem only initially)
- High-availability clustering
- Advanced monitoring and metrics
- Garbage collection and cleanup (manual for now)
- Content trust and signing
- Performance optimization for large-scale deployments

## Proposal

### Overview

Build a complete OCI registry server that implements the required endpoints from the OCI Distribution Specification using Go's standard library as much as possible. The registry will store blobs and manifests on the local filesystem and provide HTTP APIs for container image operations.

### Design Details

#### Architecture

```
┌─────────────────────────────────────────┐
│               HTTP Layer                │
│  (net/http with custom routing)         │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────┴───────────────────────┐
│            API Handlers                 │
│  - Registry Check                       │
│  - Manifest Operations                  │
│  - Blob Operations                      │
│  - Upload Management                    │
│  - Tag Listing                          │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────┴───────────────────────┐
│           Storage Interface             │
│  - BlobStore                            │
│  - ManifestStore                        │
│  - UploadManager                        │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────┴───────────────────────┐
│         Filesystem Backend              │
│  - File-based blob storage              │
│  - Directory-based organization         │
│  - Atomic operations                    │
└─────────────────────────────────────────┘
```

#### Core Interfaces

```go
type Store interface {
    BlobStore
    ManifestStore
    UploadManager
}

type BlobStore interface {
    GetBlob(ctx context.Context, digest string) (io.ReadCloser, error)
    PutBlob(ctx context.Context, digest string, content io.Reader) error
    BlobExists(ctx context.Context, digest string) (bool, error)
    DeleteBlob(ctx context.Context, digest string) error
}

type ManifestStore interface {
    GetManifest(ctx context.Context, repo, reference string) (*Manifest, error)
    PutManifest(ctx context.Context, repo, reference string, manifest *Manifest) error
    ListTags(ctx context.Context, repo string) ([]string, error)
}

type UploadManager interface {
    CreateUpload(ctx context.Context, repo string) (*Upload, error)
    WriteUploadChunk(ctx context.Context, uploadID string, offset int64, chunk io.Reader) error
    CompleteUpload(ctx context.Context, uploadID string, digest string) error
}
```

#### OCI Endpoints Implementation

| Endpoint | Method | Purpose | Implementation Status |
|----------|--------|---------|----------------------|
| `/v2/` | GET | Registry check | Phase 1 |
| `/v2/<name>/manifests/<reference>` | GET/HEAD | Pull manifest | Phase 1 |
| `/v2/<name>/manifests/<reference>` | PUT | Push manifest | Phase 1 |
| `/v2/<name>/blobs/<digest>` | GET/HEAD | Pull blob | Phase 1 |
| `/v2/<name>/blobs/uploads/` | POST | Start upload | Phase 1 |
| `/v2/<name>/blobs/uploads/<reference>` | PUT | Complete upload | Phase 1 |
| `/v2/<name>/tags/list` | GET | List tags | Phase 1 |

#### Dependencies

**Required External Dependencies:**
- `github.com/opencontainers/go-digest` - Standard OCI digest implementation

**Optional Dependencies:**
- `gopkg.in/yaml.v3` - Configuration (can use JSON if preferred)

**Standard Library Usage:**
- `net/http` - HTTP server and routing
- `encoding/json` - JSON marshaling for API responses
- `crypto/sha256` - Digest calculation and verification
- `io`, `bufio` - Streaming operations
- `os`, `path/filepath` - Filesystem operations
- `sync` - Concurrency primitives
- `context` - Request context and cancellation
- `log/slog` - Structured logging

### Implementation Phases

1. **Phase 1: Core Foundation**
   - Project structure and Go module setup
   - Storage interfaces and filesystem backend
   - Basic HTTP routing with net/http
   - OCI error handling system

2. **Phase 2: API Implementation**
   - Registry check endpoint
   - Blob operations (GET, HEAD, upload)
   - Manifest operations (GET, HEAD, PUT)
   - Tag listing

3. **Phase 3: Testing and Polish**
   - Comprehensive unit tests
   - Integration tests
   - Configuration management
   - Documentation

## Alternative Approaches

### Using External HTTP Frameworks

**Considered:** Using frameworks like Chi, Gorilla Mux, or Gin
**Rejected:** To minimize dependencies and showcase standard library capabilities

### Database Backend

**Considered:** Using SQL database for metadata storage
**Rejected:** Filesystem-based approach is simpler and sufficient for initial implementation

### Content-Addressable Storage Libraries

**Considered:** Using existing CAS libraries
**Rejected:** Custom implementation provides better learning value and control

## Testing Strategy

- **Unit Tests:** All storage interfaces and business logic
- **Integration Tests:** End-to-end HTTP API testing using httptest
- **Conformance Tests:** Validate against OCI Distribution Specification requirements
- **Performance Tests:** Basic load testing for blob upload/download
- **Error Handling Tests:** Comprehensive error scenario coverage

## Security Considerations

- Input validation for repository names and digests
- Path traversal prevention in filesystem operations
- Proper HTTP header handling
- Content-length validation
- Digest verification for all blob operations

## Performance Impact

- **Memory Usage:** Minimal - streaming operations avoid loading large blobs into memory
- **CPU Usage:** Low - primarily I/O bound operations
- **Network Impact:** Efficient streaming for large blob transfers
- **Storage Impact:** Direct filesystem storage with content-addressable organization

## Backward Compatibility

This is a new implementation, so no backward compatibility concerns exist.

## Migration Path

Not applicable for initial implementation.

## Documentation Requirements

- **User Documentation:** Setup and usage guide
- **API Documentation:** Complete OCI endpoint documentation
- **Developer Documentation:** Architecture and extension guide
- **Configuration Documentation:** Configuration options and examples

## Future Considerations

- Plugin architecture for storage backends
- Advanced authentication mechanisms
- Metrics and monitoring integration
- Garbage collection and cleanup
- Content trust and signing support

## Implementation Timeline

- **Week 1:** Project foundation and storage layer
- **Week 2:** HTTP API implementation
- **Week 3:** Testing and documentation
- **Week 4:** Polish and optimization

## References

- [OCI Distribution Specification](https://github.com/opencontainers/distribution-spec/blob/main/spec.md)
- [OCI Image Format Specification](https://github.com/opencontainers/image-spec)
- [Go HTTP Package Documentation](https://pkg.go.dev/net/http)
- [Docker Registry HTTP API V2](https://docs.docker.com/registry/spec/api/)