package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/jbpratt/octoserve/internal/p2p/proto"
	"github.com/jbpratt/octoserve/internal/storage"
	"github.com/jbpratt/octoserve/pkg/types"
)

// GRPCTransport implements the Transport interface using gRPC
type GRPCTransport struct {
	port        int
	server      *grpc.Server
	connections sync.Map // map[string]*grpc.ClientConn
	listener    net.Listener
	store       storage.Store
	nodeID      string

	// Connection pool settings
	maxConnections int
	idleTimeout    time.Duration
	maxMessageSize int

	// Lifecycle
	started bool
	mu      sync.RWMutex
}

// NewGRPCTransport creates a new gRPC transport
func NewGRPCTransport(nodeID string, port int, store storage.Store) *GRPCTransport {
	return &GRPCTransport{
		port:           port,
		store:          store,
		nodeID:         nodeID,
		maxConnections: 100,
		idleTimeout:    5 * time.Minute,
		maxMessageSize: 32 * 1024 * 1024, // 32MB
	}
}

// Start starts the gRPC server
func (g *GRPCTransport) Start(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.started {
		return fmt.Errorf("transport already started")
	}

	// Create listener
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", g.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", g.port, err)
	}
	g.listener = lis

	// Create gRPC server with options
	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    60 * time.Second,
			Timeout: 5 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             30 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.MaxRecvMsgSize(g.maxMessageSize),
		grpc.MaxSendMsgSize(g.maxMessageSize),
	}

	g.server = grpc.NewServer(opts...)

	// Register service (would register the generated P2P service here)
	// For now, we'll implement a simple service manually
	g.registerServices()

	// Start server in goroutine
	go func() {
		if err := g.server.Serve(lis); err != nil {
			// Log error but don't panic
			fmt.Printf("gRPC server error: %v\n", err)
		}
	}()

	g.started = true
	return nil
}

// Stop stops the gRPC server and closes all connections
func (g *GRPCTransport) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.started {
		return nil
	}

	// Close all client connections
	g.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*grpc.ClientConn); ok {
			conn.Close()
		}
		g.connections.Delete(key)
		return true
	})

	// Stop server
	if g.server != nil {
		g.server.GracefulStop()
	}

	if g.listener != nil {
		g.listener.Close()
	}

	g.started = false
	return nil
}

// SendBlob sends a blob to a peer
func (g *GRPCTransport) SendBlob(ctx context.Context, peer storage.PeerInfo, digest string, data io.Reader) error {
	conn, err := g.getConnection(ctx, peer)
	if err != nil {
		return fmt.Errorf("failed to get connection to peer %s: %w", peer.ID, err)
	}

	client := proto.NewP2PServiceClient(conn)
	stream, err := client.PutBlob(ctx)
	if err != nil {
		return fmt.Errorf("failed to create put blob stream: %w", err)
	}

	const chunkSize = 64 * 1024 // 64KB chunks
	buffer := make([]byte, chunkSize)
	offset := int64(0)

	for {
		n, err := data.Read(buffer)
		if n > 0 {
			chunk := &proto.BlobChunk{
				Digest:  digest,
				Offset:  offset,
				Data:    buffer[:n],
				IsFinal: false,
			}

			if err := stream.Send(chunk); err != nil {
				return fmt.Errorf("failed to send blob chunk: %w", err)
			}

			offset += int64(n)
		}

		if err == io.EOF {
			// Send final chunk
			finalChunk := &proto.BlobChunk{
				Digest:  digest,
				Offset:  offset,
				Data:    nil,
				IsFinal: true,
			}
			if err := stream.Send(finalChunk); err != nil {
				return fmt.Errorf("failed to send final blob chunk: %w", err)
			}
			break
		}

		if err != nil {
			return fmt.Errorf("failed to read blob data: %w", err)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("failed to close blob stream: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("blob upload failed: %s", resp.Error)
	}

	return nil
}

// RequestBlob requests a blob from a peer
func (g *GRPCTransport) RequestBlob(ctx context.Context, peer storage.PeerInfo, digest string) (io.ReadCloser, error) {
	conn, err := g.getConnection(ctx, peer)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection to peer %s: %w", peer.ID, err)
	}

	client := proto.NewP2PServiceClient(conn)
	stream, err := client.GetBlob(ctx, &proto.GetBlobRequest{
		Digest: digest,
		Offset: 0,
		Limit:  0, // 0 means no limit
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create get blob stream: %w", err)
	}

	// Create a pipe to stream the data back
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				pw.CloseWithError(fmt.Errorf("failed to receive blob chunk: %w", err))
				return
			}

			if len(chunk.Data) > 0 {
				if _, err := pw.Write(chunk.Data); err != nil {
					pw.CloseWithError(fmt.Errorf("failed to write blob chunk: %w", err))
					return
				}
			}

			if chunk.IsFinal {
				return
			}
		}
	}()

	return pr, nil
}

// SendManifest sends a manifest to a peer
func (g *GRPCTransport) SendManifest(ctx context.Context, peer storage.PeerInfo, repo, ref string, manifest *types.Manifest) error {
	conn, err := g.getConnection(ctx, peer)
	if err != nil {
		return fmt.Errorf("failed to get connection to peer %s: %w", peer.ID, err)
	}

	client := proto.NewP2PServiceClient(conn)

	// Serialize manifest data
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Add timeout for manifest send
	manifestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.PutManifest(manifestCtx, &proto.PutManifestRequest{
		Repository:   repo,
		Reference:    ref,
		ManifestData: manifestData,
		MediaType:    manifest.MediaType,
	})
	if err != nil {
		return fmt.Errorf("PutManifest request failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("manifest upload failed: %s", resp.Error)
	}

	return nil
}

// RequestManifest requests a manifest from a peer
func (g *GRPCTransport) RequestManifest(ctx context.Context, peer storage.PeerInfo, repo, ref string) (*types.Manifest, error) {
	conn, err := g.getConnection(ctx, peer)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection to peer %s: %w", peer.ID, err)
	}

	client := proto.NewP2PServiceClient(conn)

	// Add timeout for manifest request
	manifestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.GetManifest(manifestCtx, &proto.GetManifestRequest{
		Repository: repo,
		Reference:  ref,
	})
	if err != nil {
		return nil, fmt.Errorf("GetManifest request failed: %w", err)
	}

	// Unmarshal manifest data
	manifest := &types.Manifest{}
	err = json.Unmarshal(resp.ManifestData, manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	// Set additional fields from response
	manifest.MediaType = resp.MediaType
	manifest.Digest = resp.Digest
	manifest.Size = resp.Size

	return manifest, nil
}

// HasManifest checks if a peer has a specific manifest
func (g *GRPCTransport) HasManifest(ctx context.Context, peer storage.PeerInfo, repo, ref string) (bool, string, int64, error) {
	conn, err := g.getConnection(ctx, peer)
	if err != nil {
		return false, "", 0, fmt.Errorf("failed to get connection to peer %s: %w", peer.ID, err)
	}

	client := proto.NewP2PServiceClient(conn)

	// Add timeout for HasManifest request
	manifestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := client.HasManifest(manifestCtx, &proto.HasManifestRequest{
		Repository: repo,
		Reference:  ref,
	})
	if err != nil {
		return false, "", 0, fmt.Errorf("HasManifest request failed: %w", err)
	}

	return resp.Exists, resp.Digest, resp.Size, nil
}

// HasBlob checks if a peer has a specific blob
func (g *GRPCTransport) HasBlob(ctx context.Context, peer storage.PeerInfo, digest string) (bool, int64, error) {
	conn, err := g.getConnection(ctx, peer)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get connection to peer %s: %w", peer.ID, err)
	}

	client := proto.NewP2PServiceClient(conn)

	// Add timeout for HasBlob request
	blobCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := client.HasBlob(blobCtx, &proto.HasBlobRequest{
		Digest: digest,
	})
	if err != nil {
		return false, 0, fmt.Errorf("HasBlob request failed: %w", err)
	}

	return resp.Exists, resp.Size, nil
}

// Ping sends a ping to a peer
func (g *GRPCTransport) Ping(ctx context.Context, peer storage.PeerInfo) error {
	conn, err := g.getConnection(ctx, peer)
	if err != nil {
		return fmt.Errorf("failed to get connection to peer %s: %w", peer.ID, err)
	}

	client := proto.NewP2PServiceClient(conn)

	// Add timeout for ping
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := client.Ping(pingCtx, &proto.PingRequest{
		NodeId:    g.nodeID,
		Timestamp: time.Now().Unix(),
	})
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	// Validate response
	if resp.NodeId == "" {
		return fmt.Errorf("invalid ping response: empty node ID")
	}

	return nil
}

// Close implements the Transport interface
func (g *GRPCTransport) Close() error {
	return g.Stop()
}

// getConnection gets or creates a connection to a peer
func (g *GRPCTransport) getConnection(ctx context.Context, peer storage.PeerInfo) (*grpc.ClientConn, error) {
	addr := peer.Addr()

	// Check if we already have a connection
	if value, ok := g.connections.Load(addr); ok {
		if conn, ok := value.(*grpc.ClientConn); ok {
			state := conn.GetState()
			if state == connectivity.Ready || state == connectivity.Idle {
				return conn, nil
			}
			// Connection is bad, remove it
			g.connections.Delete(addr)
			conn.Close()
		}
	}

	// Create new connection
	conn, err := g.createConnection(ctx, addr)
	if err != nil {
		return nil, err
	}

	g.connections.Store(addr, conn)
	return conn, nil
}

// createConnection creates a new gRPC connection
func (g *GRPCTransport) createConnection(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(g.maxMessageSize),
			grpc.MaxCallSendMsgSize(g.maxMessageSize),
		),
	}

	conn, err := grpc.DialContext(ctx, addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", addr, err)
	}

	return conn, nil
}

// registerServices registers the gRPC services
func (g *GRPCTransport) registerServices() {
	p2pServer := &P2PServer{
		store:  g.store,
		nodeID: g.nodeID,
	}
	proto.RegisterP2PServiceServer(g.server, p2pServer)
}

// P2PServer implements the generated P2P service interface
type P2PServer struct {
	proto.UnimplementedP2PServiceServer
	store  storage.Store
	nodeID string
}

// Ping implements the Ping RPC method
func (s *P2PServer) Ping(ctx context.Context, req *proto.PingRequest) (*proto.PingResponse, error) {
	return &proto.PingResponse{
		NodeId:    s.nodeID,
		Timestamp: time.Now().Unix(),
		Status: &proto.NodeStatus{
			Health:        proto.HealthStatus_HEALTH_HEALTHY,
			UptimeSeconds: int64(time.Since(time.Now()).Seconds()), // Placeholder
			Version:       "0.1.0",
		},
	}, nil
}

// HasBlob implements the HasBlob RPC method
func (s *P2PServer) HasBlob(ctx context.Context, req *proto.HasBlobRequest) (*proto.HasBlobResponse, error) {
	exists, err := s.store.BlobExists(ctx, req.Digest)
	if err != nil {
		return &proto.HasBlobResponse{Exists: false}, err
	}

	var size int64
	if exists {
		size, err = s.store.GetBlobSize(ctx, req.Digest)
		if err != nil {
			return &proto.HasBlobResponse{Exists: true, Size: 0}, nil
		}
	}

	return &proto.HasBlobResponse{
		Exists: exists,
		Size:   size,
	}, nil
}

// GetBlob implements the GetBlob RPC method (server streaming)
func (s *P2PServer) GetBlob(req *proto.GetBlobRequest, stream proto.P2PService_GetBlobServer) error {
	reader, err := s.store.GetBlob(stream.Context(), req.Digest)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Trigger lazy replication if this blob is being requested
	// This helps ensure popular blobs get replicated automatically
	if distributedStore, ok := s.store.(interface {
		TriggerLazyReplication(ctx context.Context, digest string)
	}); ok {
		go distributedStore.TriggerLazyReplication(stream.Context(), req.Digest)
	}

	const chunkSize = 64 * 1024 // 64KB chunks
	buffer := make([]byte, chunkSize)
	offset := req.Offset

	// Seek to offset if provided
	if offset > 0 {
		// For simplicity, we're not implementing seeking in this basic version
		// In a production implementation, you'd need to support range requests
	}

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			chunk := &proto.BlobChunk{
				Digest:  req.Digest,
				Offset:  offset,
				Data:    buffer[:n],
				IsFinal: false,
			}

			if err := stream.Send(chunk); err != nil {
				return err
			}

			offset += int64(n)
		}

		if err == io.EOF {
			// Send final chunk
			finalChunk := &proto.BlobChunk{
				Digest:  req.Digest,
				Offset:  offset,
				Data:    nil,
				IsFinal: true,
			}
			return stream.Send(finalChunk)
		}

		if err != nil {
			return err
		}
	}
}

// HasManifest implements the HasManifest RPC method
func (s *P2PServer) HasManifest(ctx context.Context, req *proto.HasManifestRequest) (*proto.HasManifestResponse, error) {
	exists, err := s.store.ManifestExists(ctx, req.Repository, req.Reference)
	if err != nil {
		return &proto.HasManifestResponse{Exists: false}, err
	}

	var digest string
	var size int64
	if exists {
		manifest, err := s.store.GetManifest(ctx, req.Repository, req.Reference)
		if err == nil {
			digest = manifest.Digest
			size = manifest.Size
		}
	}

	return &proto.HasManifestResponse{
		Exists: exists,
		Digest: digest,
		Size:   size,
	}, nil
}

// PutBlob implements the PutBlob RPC method (client streaming)
func (s *P2PServer) PutBlob(stream proto.P2PService_PutBlobServer) error {
	var digest string
	var buffer bytes.Buffer
	var totalSize int64

	// Receive chunks from the stream
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to receive blob chunk: %w", err)
		}

		// Set digest from first chunk
		if digest == "" {
			digest = chunk.Digest
		}

		// Validate digest consistency
		if chunk.Digest != digest {
			return fmt.Errorf("digest mismatch in chunk: expected %s, got %s", digest, chunk.Digest)
		}

		// Write chunk data to buffer
		if len(chunk.Data) > 0 {
			n, err := buffer.Write(chunk.Data)
			if err != nil {
				return fmt.Errorf("failed to write chunk data: %w", err)
			}
			totalSize += int64(n)
		}

		// Check if this is the final chunk
		if chunk.IsFinal {
			break
		}
	}

	// Store the blob in local storage
	reader := bytes.NewReader(buffer.Bytes())
	err := s.store.PutBlob(stream.Context(), digest, reader)
	if err != nil {
		return stream.SendAndClose(&proto.PutBlobResponse{
			Digest:  digest,
			Size:    totalSize,
			Success: false,
			Error:   err.Error(),
		})
	}

	// Send success response
	return stream.SendAndClose(&proto.PutBlobResponse{
		Digest:  digest,
		Size:    totalSize,
		Success: true,
	})
}

func (s *P2PServer) GetManifest(ctx context.Context, req *proto.GetManifestRequest) (*proto.GetManifestResponse, error) {
	manifest, err := s.store.GetManifest(ctx, req.Repository, req.Reference)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	// Marshal manifest data
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	return &proto.GetManifestResponse{
		ManifestData: manifestData,
		MediaType:    manifest.MediaType,
		Digest:       manifest.Digest,
		Size:         manifest.Size,
	}, nil
}

func (s *P2PServer) PutManifest(ctx context.Context, req *proto.PutManifestRequest) (*proto.PutManifestResponse, error) {
	// Create manifest object from request
	manifest := &types.Manifest{
		MediaType: req.MediaType,
	}

	// Unmarshal manifest data
	err := json.Unmarshal(req.ManifestData, manifest)
	if err != nil {
		return &proto.PutManifestResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to unmarshal manifest: %v", err),
		}, nil
	}

	// Store the manifest
	err = s.store.PutManifest(ctx, req.Repository, req.Reference, manifest)
	if err != nil {
		return &proto.PutManifestResponse{
			Digest:  manifest.Digest,
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &proto.PutManifestResponse{
		Digest:  manifest.Digest,
		Success: true,
	}, nil
}

func (s *P2PServer) GetPeers(ctx context.Context, req *proto.GetPeersRequest) (*proto.GetPeersResponse, error) {
	return nil, fmt.Errorf("GetPeers not implemented yet")
}

func (s *P2PServer) AnnounceNode(ctx context.Context, req *proto.AnnounceNodeRequest) (*proto.AnnounceNodeResponse, error) {
	return nil, fmt.Errorf("AnnounceNode not implemented yet")
}

func (s *P2PServer) ReplicateBlob(ctx context.Context, req *proto.ReplicateBlobRequest) (*proto.ReplicateBlobResponse, error) {
	return nil, fmt.Errorf("ReplicateBlob not implemented yet")
}

func (s *P2PServer) SyncManifest(ctx context.Context, req *proto.SyncManifestRequest) (*proto.SyncManifestResponse, error) {
	return nil, fmt.Errorf("SyncManifest not implemented yet")
}
