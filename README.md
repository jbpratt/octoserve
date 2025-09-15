# Octoserve - Minimal OCI Registry

A minimal but compliant OCI (Open Container Initiative) registry implementation built from scratch in Go using primarily the standard library.

## Features

- **OCI Distribution Specification v1.1 compliant**
- **Minimal dependencies** - uses Go standard library wherever possible
- **Clean architecture** - extensible design with clear separation of concerns
- **Filesystem storage** - simple file-based blob and manifest storage
- **HTTP/2 support** - built on Go's excellent net/http package
- **Graceful shutdown** - proper signal handling and cleanup
- **Structured logging** - JSON and text format support
- **Configuration** - JSON-based config with environment variable overrides

## Quick Start

### Build and Run

```bash
# Clone the repository
git clone https://github.com/jbpratt/octoserve.git
cd octoserve

# Build the binary
go build -o bin/octoserve ./cmd/octoserve

# Run with default configuration
./bin/octoserve
```

The registry will start on `localhost:5000` by default.

### Configuration

Create a `config.json` file to customize settings:

```json
{
  "server": {
    "address": "localhost",
    "port": 5000,
    "read_timeout": "30s",
    "write_timeout": "30s",
    "idle_timeout": "120s"
  },
  "storage": {
    "type": "filesystem",
    "path": "./data"
  },
  "logging": {
    "level": "info",
    "format": "json"
  }
}
```

### Environment Variables

You can override configuration with environment variables:

```bash
export OCTOSERVE_SERVER_ADDRESS=0.0.0.0
export OCTOSERVE_SERVER_PORT=8080
export OCTOSERVE_STORAGE_PATH=/var/lib/octoserve
export OCTOSERVE_LOG_LEVEL=debug
export OCTOSERVE_LOG_FORMAT=text
```

## Usage

### Push an Image

```bash
# Tag an existing image
docker tag alpine:latest localhost:5000/my-alpine:latest

# Push to the registry
docker push localhost:5000/my-alpine:latest
```

### Pull an Image

```bash
# Pull from the registry
docker pull localhost:5000/my-alpine:latest
```

### List Tags

```bash
# List tags for a repository
curl http://localhost:5000/v2/my-alpine/tags/list
```

### Check Registry Status

```bash
# Verify registry is running
curl http://localhost:5000/v2/
```

## API Endpoints

Octoserve implements the OCI Distribution Specification endpoints:

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v2/` | Check API version |
| GET | `/v2/{name}/manifests/{reference}` | Get manifest |
| PUT | `/v2/{name}/manifests/{reference}` | Put manifest |
| HEAD | `/v2/{name}/manifests/{reference}` | Check manifest exists |
| DELETE | `/v2/{name}/manifests/{reference}` | Delete manifest |
| GET | `/v2/{name}/blobs/{digest}` | Get blob |
| HEAD | `/v2/{name}/blobs/{digest}` | Check blob exists |
| DELETE | `/v2/{name}/blobs/{digest}` | Delete blob |
| POST | `/v2/{name}/blobs/uploads/` | Start blob upload |
| PATCH | `/v2/{name}/blobs/uploads/{uuid}` | Upload blob chunk |
| PUT | `/v2/{name}/blobs/uploads/{uuid}` | Complete blob upload |
| GET | `/v2/{name}/blobs/uploads/{uuid}` | Get upload status |
| DELETE | `/v2/{name}/blobs/uploads/{uuid}` | Cancel upload |
| GET | `/v2/{name}/tags/list` | List repository tags |

## Architecture

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

## Storage Layout

The filesystem backend organizes data as follows:

```
data/
├── blobs/
│   └── sha256/
│       └── ab/
│           └── cd/
│               └── abcdef123...  # Blob content
├── manifests/
│   └── repository_name/
│       ├── latest              # Tag -> manifest
│       └── sha256:abc123...    # Digest -> manifest
└── uploads/
    └── uuid-1234...            # Temporary upload files
```

## Dependencies

Octoserve uses minimal external dependencies:

- `github.com/opencontainers/go-digest` - Standard OCI digest implementation
- `github.com/google/uuid` - UUID generation for upload sessions

Everything else uses the Go standard library:
- `net/http` - HTTP server and client
- `encoding/json` - JSON marshaling
- `crypto/sha256` - Digest calculation
- `log/slog` - Structured logging
- `context` - Request context and cancellation

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/storage/filesystem
```

### Building

```bash
# Build binary
go build -o bin/octoserve ./cmd/octoserve

# Build with optimizations
go build -ldflags="-s -w" -o bin/octoserve ./cmd/octoserve

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o bin/octoserve-linux ./cmd/octoserve
```

### Code Organization

- `cmd/octoserve/` - Main application entry point
- `internal/api/` - HTTP routing and handlers
- `internal/storage/` - Storage interface and implementations
- `internal/config/` - Configuration management
- `internal/errors/` - OCI-compliant error handling
- `pkg/types/` - Public types and data structures
- `enhancements/` - Design documents and enhancement proposals

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass
6. Submit a pull request

## Security

- Input validation for repository names and digests
- Path traversal prevention in filesystem operations
- Proper HTTP header handling
- Content-length validation
- Digest verification for all blob operations

## Performance

- Streaming I/O for large blob transfers
- Minimal memory usage through efficient buffering
- Concurrent upload handling with goroutines
- Content-addressable storage for deduplication

## Compatibility

Octoserve is compatible with:
- Docker clients
- Podman
- Containerd
- Any OCI-compliant container runtime

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.

## References

- [OCI Distribution Specification](https://github.com/opencontainers/distribution-spec)
- [OCI Image Format Specification](https://github.com/opencontainers/image-spec)
- [Docker Registry HTTP API V2](https://docs.docker.com/registry/spec/api/)