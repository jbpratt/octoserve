# Octoserve - Minimal OCI Registry

A minimal but compliant OCI (Open Container Initiative) registry implementation built from scratch in Go using primarily the standard library.

## Features

- **OCI Distribution Specification v1.1 compliant**
- **Peer-to-peer clustering** - distributed registry with automatic content replication
- **Minimal dependencies** - uses Go standard library wherever possible
- **Clean architecture** - extensible design with clear separation of concerns
- **Filesystem storage** - simple file-based blob and manifest storage
- **HTTP/2 support** - built on Go's excellent net/http package
- **Graceful shutdown** - proper signal handling and cleanup
- **Structured logging** - JSON and text format support
- **Configuration** - JSON-based config with human-readable duration support

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
    "idle_timeout": "2m"
  },
  "storage": {
    "type": "filesystem",
    "path": "./data"
  },
  "logging": {
    "level": "info",
    "format": "json"
  },
  "p2p": {
    "enabled": false,
    "node_id": "registry-node-1",
    "port": 6000,
    "bootstrap_peers": ["localhost:6001"],
    "discovery": {
      "method": "static",
      "interval": "30s",
      "timeout": "10s"
    },
    "replication": {
      "factor": 2,
      "strategy": "eager",
      "sync_interval": "1m",
      "consistency_mode": "eventual"
    },
    "health_check": {
      "interval": "15s",
      "timeout": "5s",
      "failure_threshold": 3,
      "workers": 5
    },
    "transport": {
      "protocol": "grpc",
      "max_connections": 50,
      "idle_timeout": "5m",
      "max_message_size": 33554432
    }
  }
}
```

### Environment Variables

You can override configuration with environment variables:

```bash
# Server configuration
export OCTOSERVE_SERVER_ADDRESS=0.0.0.0
export OCTOSERVE_SERVER_PORT=8080
export OCTOSERVE_STORAGE_PATH=/var/lib/octoserve
export OCTOSERVE_LOG_LEVEL=debug
export OCTOSERVE_LOG_FORMAT=text

# P2P configuration
export OCTOSERVE_P2P_ENABLED=true
export OCTOSERVE_P2P_NODE_ID=registry-node-1
export OCTOSERVE_P2P_PORT=6000
export OCTOSERVE_P2P_BOOTSTRAP_PEERS=peer1:6000,peer2:6000
export OCTOSERVE_P2P_DISCOVERY_METHOD=static
export OCTOSERVE_P2P_REPLICATION_STRATEGY=eager
export OCTOSERVE_P2P_REPLICATION_FACTOR=2
export OCTOSERVE_P2P_TRANSPORT_PROTOCOL=grpc
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

## Peer-to-Peer Clustering

Octoserve supports distributed operation through peer-to-peer clustering, enabling high availability and automatic content replication across multiple registry nodes.

### Key Features

- **Automatic replication** - blobs and manifests are automatically distributed across cluster nodes
- **Cross-node discovery** - images pushed to one node become available on all healthy nodes
- **Health monitoring** - continuous peer health checking with automatic failure detection
- **Multiple replication strategies** - eager, lazy, and hybrid replication modes
- **gRPC transport** - efficient binary protocol for inter-node communication
- **Static discovery** - simple peer discovery using bootstrap peer configuration

### Quick P2P Setup

1. **Create cluster configuration files:**

```bash
# Node 1 configuration (config-node1.json)
{
  "server": {"address": "0.0.0.0", "port": 5001},
  "storage": {"type": "filesystem", "path": "./data-node1"},
  "p2p": {
    "enabled": true,
    "node_id": "registry-node-1",
    "port": 6001,
    "bootstrap_peers": ["localhost:6002"],
    "replication": {"factor": 2, "strategy": "eager"}
  }
}

# Node 2 configuration (config-node2.json)
{
  "server": {"address": "0.0.0.0", "port": 5002},
  "storage": {"type": "filesystem", "path": "./data-node2"},
  "p2p": {
    "enabled": true,
    "node_id": "registry-node-2", 
    "port": 6002,
    "bootstrap_peers": ["localhost:6001"],
    "replication": {"factor": 2, "strategy": "eager"}
  }
}
```

2. **Start the cluster nodes:**

```bash
# Terminal 1 - Start node 1
./bin/octoserve -config config-node1.json

# Terminal 2 - Start node 2  
./bin/octoserve -config config-node2.json
```

3. **Test cross-node replication:**

```bash
# Push to node 1
docker tag alpine:latest localhost:5001/my-alpine:latest
docker push localhost:5001/my-alpine:latest

# Pull from node 2 (automatically available!)
docker pull localhost:5002/my-alpine:latest
```

### P2P Configuration Reference

#### Discovery Settings
- `method`: Discovery mechanism ("static", "dns", "consul", "mdns")
- `interval`: How often to discover peers (e.g., "30s")
- `timeout`: Discovery operation timeout (e.g., "10s")

#### Replication Settings
- `factor`: Number of replicas to maintain across the cluster
- `strategy`: Replication timing ("eager", "lazy", "hybrid")
- `sync_interval`: Background synchronization interval
- `consistency_mode`: Consistency model ("eventual", "strong")

#### Health Monitoring
- `interval`: Peer health check frequency (e.g., "15s")
- `timeout`: Health check timeout (e.g., "5s")
- `failure_threshold`: Failed checks before marking peer unhealthy
- `workers`: Concurrent health check workers

#### Transport Settings
- `protocol`: Communication protocol ("grpc", "http")
- `max_connections`: Maximum connections per peer
- `idle_timeout`: Connection idle timeout
- `max_message_size`: Maximum message size in bytes

### Replication Strategies

**Eager Replication:**
- Content is replicated immediately upon write
- Provides fastest cross-node availability
- Higher network usage and write latency

**Lazy Replication:**
- Content is replicated only when accessed from another node
- Lower network usage
- Slight delay on first cross-node access

**Hybrid Replication:**
- Small content (< 1MB) uses eager replication
- Large content uses lazy replication
- Balances performance and network efficiency

### Monitoring P2P Health

Check cluster status via metrics endpoint:

```bash
# View P2P metrics for node health and replication stats
curl http://localhost:5001/metrics | grep p2p
```

### Troubleshooting P2P

**Common issues:**

1. **Peers not discovering each other:** Check bootstrap_peers configuration and network connectivity
2. **Replication not working:** Verify P2P ports are accessible and not blocked by firewalls
3. **High network usage:** Consider switching from "eager" to "lazy" or "hybrid" replication strategy
4. **Split-brain scenarios:** Ensure odd number of nodes or implement proper consensus mechanisms

**Debug logging:**

```bash
# Enable debug logging to see P2P operations
export OCTOSERVE_LOG_LEVEL=debug
./bin/octoserve -config config-node1.json
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
│  - PeerStore (P2P)                      │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────┴───────────────────────┐
│         Distributed Store               │
│  - Local storage wrapper               │
│  - Cross-node replication              │
│  - Peer coordination                   │
│  - Automatic failover                  │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────┴───────────────────────┐
│         Filesystem Backend              │
│  - File-based blob storage              │
│  - Directory-based organization         │
│  - Atomic operations                    │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│              P2P Layer                  │
│  ┌─────────────┬───────────────────────┐ │
│  │ P2P Manager │   gRPC Transport      │ │
│  │ - Discovery │   - Blob transfer     │ │
│  │ - Health    │   - Manifest sync     │ │
│  │ - Routing   │   - Peer communication│ │
│  └─────────────┴───────────────────────┘ │
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
- `google.golang.org/grpc` - gRPC framework for P2P communication
- `google.golang.org/protobuf` - Protocol Buffers for P2P messaging

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
  - `internal/storage/filesystem/` - Filesystem storage backend
  - `internal/storage/distributed/` - Distributed storage wrapper
- `internal/config/` - Configuration management
- `internal/errors/` - OCI-compliant error handling
- `internal/p2p/` - Peer-to-peer clustering components
  - `internal/p2p/discovery/` - Peer discovery mechanisms
  - `internal/p2p/transport/` - gRPC transport layer
- `internal/metrics/` - Prometheus metrics collection
- `pkg/types/` - Public types and data structures
- `proto/` - Protocol Buffer definitions
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
- Intelligent P2P replication with configurable strategies
- Efficient gRPC binary protocol for inter-node communication
- Automatic load distribution across cluster nodes

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