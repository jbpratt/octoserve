package config

import (
	"os"
	"testing"
	"time"
)

func TestP2PConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      P2PConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid P2P config",
			config: P2PConfig{
				Enabled: true,
				NodeID:  "test-node",
				Port:    6000,
				Discovery: DiscoveryConfig{
					Method:   "static",
					Interval: Duration(30 * time.Second),
					Timeout:  Duration(10 * time.Second),
				},
				Replication: ReplicationConfig{
					Factor:          3,
					Strategy:        "eager",
					SyncInterval:    Duration(60 * time.Second),
					ConsistencyMode: "eventual",
				},
				HealthCheck: HealthCheckConfig{
					Interval:         Duration(15 * time.Second),
					Timeout:          Duration(5 * time.Second),
					FailureThreshold: 3,
					Workers:          10,
				},
				Transport: TransportConfig{
					Protocol:       "grpc",
					MaxConnections: 100,
					IdleTimeout:    Duration(5 * time.Minute),
					MaxMessageSize: 32 * 1024 * 1024,
				},
			},
			expectError: false,
		},
		{
			name: "invalid port",
			config: P2PConfig{
				Enabled: true,
				Port:    70000, // Invalid port
			},
			expectError: true,
			errorMsg:    "invalid P2P port",
		},
		{
			name: "invalid discovery method",
			config: P2PConfig{
				Enabled: true,
				Port:    6000,
				Discovery: DiscoveryConfig{
					Method: "unknown",
				},
			},
			expectError: true,
			errorMsg:    "invalid discovery method",
		},
		{
			name: "invalid replication factor",
			config: P2PConfig{
				Enabled: true,
				Port:    6000,
				Discovery: DiscoveryConfig{
					Method: "static",
				},
				Replication: ReplicationConfig{
					Factor: 0, // Invalid factor
				},
			},
			expectError: true,
			errorMsg:    "replication factor must be at least 1",
		},
		{
			name: "invalid replication strategy",
			config: P2PConfig{
				Enabled: true,
				Port:    6000,
				Discovery: DiscoveryConfig{
					Method: "static",
				},
				Replication: ReplicationConfig{
					Factor:   3,
					Strategy: "unknown",
				},
			},
			expectError: true,
			errorMsg:    "invalid replication strategy",
		},
		{
			name: "disabled P2P should pass validation",
			config: P2PConfig{
				Enabled: false,
				Port:    70000, // Invalid port, but should be ignored when disabled
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Server: ServerConfig{
					Address: "localhost",
					Port:    5000,
				},
				Storage: StorageConfig{
					Type: "filesystem",
					Path: "./data",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
				P2P: tt.config,
			}

			err := cfg.validateP2P()

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error containing '%s' but got none", tt.errorMsg)
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Fatalf("expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestP2PEnvironmentVariables(t *testing.T) {
	// Save original environment
	originalEnv := map[string]string{
		"OCTOSERVE_P2P_ENABLED":              os.Getenv("OCTOSERVE_P2P_ENABLED"),
		"OCTOSERVE_P2P_NODE_ID":              os.Getenv("OCTOSERVE_P2P_NODE_ID"),
		"OCTOSERVE_P2P_PORT":                 os.Getenv("OCTOSERVE_P2P_PORT"),
		"OCTOSERVE_P2P_BOOTSTRAP_PEERS":      os.Getenv("OCTOSERVE_P2P_BOOTSTRAP_PEERS"),
		"OCTOSERVE_P2P_DISCOVERY_METHOD":     os.Getenv("OCTOSERVE_P2P_DISCOVERY_METHOD"),
		"OCTOSERVE_P2P_REPLICATION_STRATEGY": os.Getenv("OCTOSERVE_P2P_REPLICATION_STRATEGY"),
		"OCTOSERVE_P2P_REPLICATION_FACTOR":   os.Getenv("OCTOSERVE_P2P_REPLICATION_FACTOR"),
		"OCTOSERVE_P2P_TRANSPORT_PROTOCOL":   os.Getenv("OCTOSERVE_P2P_TRANSPORT_PROTOCOL"),
	}

	// Cleanup function
	defer func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// Set test environment variables
	testEnv := map[string]string{
		"OCTOSERVE_P2P_ENABLED":              "true",
		"OCTOSERVE_P2P_NODE_ID":              "env-test-node",
		"OCTOSERVE_P2P_PORT":                 "7000",
		"OCTOSERVE_P2P_BOOTSTRAP_PEERS":      "peer1:6000,peer2:6000",
		"OCTOSERVE_P2P_DISCOVERY_METHOD":     "dns",
		"OCTOSERVE_P2P_REPLICATION_STRATEGY": "lazy",
		"OCTOSERVE_P2P_REPLICATION_FACTOR":   "5",
		"OCTOSERVE_P2P_TRANSPORT_PROTOCOL":   "http",
	}

	for key, value := range testEnv {
		os.Setenv(key, value)
	}

	// Create default config
	cfg := Default()

	// Load from environment
	cfg.LoadFromEnv()

	// Verify P2P configuration was loaded from environment
	if !cfg.P2P.Enabled {
		t.Error("P2P should be enabled from environment")
	}
	if cfg.P2P.NodeID != "env-test-node" {
		t.Errorf("expected node ID 'env-test-node', got '%s'", cfg.P2P.NodeID)
	}
	if cfg.P2P.Port != 7000 {
		t.Errorf("expected port 7000, got %d", cfg.P2P.Port)
	}
	if len(cfg.P2P.BootstrapPeers) != 2 {
		t.Errorf("expected 2 bootstrap peers, got %d", len(cfg.P2P.BootstrapPeers))
	}
	if cfg.P2P.BootstrapPeers[0] != "peer1:6000" {
		t.Errorf("expected first peer 'peer1:6000', got '%s'", cfg.P2P.BootstrapPeers[0])
	}
	if cfg.P2P.Discovery.Method != "dns" {
		t.Errorf("expected discovery method 'dns', got '%s'", cfg.P2P.Discovery.Method)
	}
	if cfg.P2P.Replication.Strategy != "lazy" {
		t.Errorf("expected replication strategy 'lazy', got '%s'", cfg.P2P.Replication.Strategy)
	}
	if cfg.P2P.Replication.Factor != 5 {
		t.Errorf("expected replication factor 5, got %d", cfg.P2P.Replication.Factor)
	}
	if cfg.P2P.Transport.Protocol != "http" {
		t.Errorf("expected transport protocol 'http', got '%s'", cfg.P2P.Transport.Protocol)
	}
}

func TestP2PDefaultNodeID(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Address: "localhost",
			Port:    5000,
		},
		Storage: StorageConfig{
			Type: "filesystem",
			Path: "./data",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		P2P: P2PConfig{
			Enabled: true,
			Port:    6000,
			Discovery: DiscoveryConfig{
				Method:   "static",
				Interval: Duration(30 * time.Second),
				Timeout:  Duration(10 * time.Second),
			},
			Replication: ReplicationConfig{
				Factor:          3,
				Strategy:        "eager",
				SyncInterval:    Duration(60 * time.Second),
				ConsistencyMode: "eventual",
			},
			HealthCheck: HealthCheckConfig{
				Interval:         Duration(15 * time.Second),
				Timeout:          Duration(5 * time.Second),
				FailureThreshold: 3,
				Workers:          10,
			},
			Transport: TransportConfig{
				Protocol:    "grpc",
				IdleTimeout: Duration(5 * time.Minute),
			},
		},
	}

	// NodeID should be empty initially
	if cfg.P2P.NodeID != "" {
		t.Fatalf("expected empty node ID, got '%s'", cfg.P2P.NodeID)
	}

	// Validation should set a default node ID
	err := cfg.validateP2P()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// NodeID should be set after validation
	if cfg.P2P.NodeID == "" {
		t.Fatal("node ID should be set after validation")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(substr == "" || findSubstring(s, substr) >= 0)
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
