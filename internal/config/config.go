package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Config represents the application configuration
type Config struct {
	Server  ServerConfig  `json:"server"`
	Storage StorageConfig `json:"storage"`
	Logging LoggingConfig `json:"logging"`
	Metrics MetricsConfig `json:"metrics"`
	P2P     P2PConfig     `json:"p2p"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Address      string        `json:"address"`
	Port         int           `json:"port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout"`
}

// StorageConfig contains storage backend configuration
type StorageConfig struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

// MetricsConfig contains metrics configuration
type MetricsConfig struct {
	Enabled   bool             `json:"enabled"`
	Endpoint  string           `json:"endpoint"`
	BasicAuth *BasicAuthConfig `json:"basic_auth,omitempty"`
}

// BasicAuthConfig contains basic authentication configuration
type BasicAuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// P2PConfig contains peer-to-peer configuration
type P2PConfig struct {
	Enabled        bool              `json:"enabled"`
	NodeID         string            `json:"node_id"`
	Port           int               `json:"port"`
	BootstrapPeers []string          `json:"bootstrap_peers"`
	Discovery      DiscoveryConfig   `json:"discovery"`
	Replication    ReplicationConfig `json:"replication"`
	HealthCheck    HealthCheckConfig `json:"health_check"`
	Transport      TransportConfig   `json:"transport"`
}

// DiscoveryConfig contains peer discovery configuration
type DiscoveryConfig struct {
	Method        string        `json:"method"` // "static", "dns", "consul", "mdns"
	Interval      time.Duration `json:"interval"`
	Timeout       time.Duration `json:"timeout"`
	ConsulAddress string        `json:"consul_address,omitempty"`
	DNSName       string        `json:"dns_name,omitempty"`
}

// ReplicationConfig contains replication strategy configuration
type ReplicationConfig struct {
	Factor          int           `json:"factor"`   // Number of replicas
	Strategy        string        `json:"strategy"` // "eager", "lazy", "hybrid"
	SyncInterval    time.Duration `json:"sync_interval"`
	ConsistencyMode string        `json:"consistency_mode"` // "eventual", "strong"
}

// HealthCheckConfig contains health checking configuration
type HealthCheckConfig struct {
	Interval         time.Duration `json:"interval"`
	Timeout          time.Duration `json:"timeout"`
	FailureThreshold int           `json:"failure_threshold"`
	Workers          int           `json:"workers"`
}

// TransportConfig contains transport layer configuration
type TransportConfig struct {
	Protocol       string        `json:"protocol"` // "grpc", "http"
	MaxConnections int           `json:"max_connections"`
	IdleTimeout    time.Duration `json:"idle_timeout"`
	MaxMessageSize int           `json:"max_message_size"`
}

// Default returns a configuration with sensible defaults
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Address:      "localhost",
			Port:         5000,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		Storage: StorageConfig{
			Type: "filesystem",
			Path: "./data",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Metrics: MetricsConfig{
			Enabled:  true,
			Endpoint: "/metrics",
		},
		P2P: P2PConfig{
			Enabled: false,
			Port:    6000,
			Discovery: DiscoveryConfig{
				Method:   "static",
				Interval: 30 * time.Second,
				Timeout:  10 * time.Second,
			},
			Replication: ReplicationConfig{
				Factor:          3,
				Strategy:        "eager",
				SyncInterval:    60 * time.Second,
				ConsistencyMode: "eventual",
			},
			HealthCheck: HealthCheckConfig{
				Interval:         15 * time.Second,
				Timeout:          5 * time.Second,
				FailureThreshold: 3,
				Workers:          10,
			},
			Transport: TransportConfig{
				Protocol:       "grpc",
				MaxConnections: 100,
				IdleTimeout:    5 * time.Minute,
				MaxMessageSize: 32 * 1024 * 1024, // 32MB
			},
		},
	}
}

// Load loads configuration from a JSON file
func Load(filename string) (*Config, error) {
	// Start with defaults
	config := Default()

	// If file doesn't exist, return defaults
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return config, nil
	}

	// Read configuration file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filename, err)
	}

	// Parse JSON
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", filename, err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// Save saves configuration to a JSON file
func (c *Config) Save(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", filename, err)
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Server.ReadTimeout < 0 {
		return fmt.Errorf("invalid read timeout: %v", c.Server.ReadTimeout)
	}

	if c.Server.WriteTimeout < 0 {
		return fmt.Errorf("invalid write timeout: %v", c.Server.WriteTimeout)
	}

	// Validate storage config
	if c.Storage.Type == "" {
		return fmt.Errorf("storage type cannot be empty")
	}

	if c.Storage.Type == "filesystem" && c.Storage.Path == "" {
		return fmt.Errorf("filesystem storage requires a path")
	}

	// Validate logging config
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	validFormats := map[string]bool{
		"json": true,
		"text": true,
	}

	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("invalid log format: %s", c.Logging.Format)
	}

	// Validate P2P config
	if err := c.validateP2P(); err != nil {
		return fmt.Errorf("invalid P2P configuration: %w", err)
	}

	return nil
}

// validateP2P validates P2P configuration
func (c *Config) validateP2P() error {
	if !c.P2P.Enabled {
		return nil // Skip validation if P2P is disabled
	}

	// Validate port
	if c.P2P.Port < 1 || c.P2P.Port > 65535 {
		return fmt.Errorf("invalid P2P port: %d", c.P2P.Port)
	}

	// Validate node ID if provided
	if c.P2P.NodeID == "" {
		// Generate a default node ID if not provided
		hostname, _ := os.Hostname()
		if hostname != "" {
			c.P2P.NodeID = hostname
		}
	}

	// Validate discovery method
	validDiscoveryMethods := map[string]bool{
		"static": true,
		"dns":    true,
		"consul": true,
		"mdns":   true,
	}
	if !validDiscoveryMethods[c.P2P.Discovery.Method] {
		return fmt.Errorf("invalid discovery method: %s", c.P2P.Discovery.Method)
	}

	// Validate replication factor
	if c.P2P.Replication.Factor < 1 {
		return fmt.Errorf("replication factor must be at least 1: %d", c.P2P.Replication.Factor)
	}

	// Validate replication strategy
	validStrategies := map[string]bool{
		"eager":  true,
		"lazy":   true,
		"hybrid": true,
	}
	if !validStrategies[c.P2P.Replication.Strategy] {
		return fmt.Errorf("invalid replication strategy: %s", c.P2P.Replication.Strategy)
	}

	// Validate consistency mode
	validConsistency := map[string]bool{
		"eventual": true,
		"strong":   true,
	}
	if !validConsistency[c.P2P.Replication.ConsistencyMode] {
		return fmt.Errorf("invalid consistency mode: %s", c.P2P.Replication.ConsistencyMode)
	}

	// Validate transport protocol
	validProtocols := map[string]bool{
		"grpc": true,
		"http": true,
	}
	if !validProtocols[c.P2P.Transport.Protocol] {
		return fmt.Errorf("invalid transport protocol: %s", c.P2P.Transport.Protocol)
	}

	// Validate timeouts
	if c.P2P.Discovery.Interval <= 0 {
		return fmt.Errorf("discovery interval must be positive")
	}
	if c.P2P.Discovery.Timeout <= 0 {
		return fmt.Errorf("discovery timeout must be positive")
	}
	if c.P2P.HealthCheck.Interval <= 0 {
		return fmt.Errorf("health check interval must be positive")
	}
	if c.P2P.HealthCheck.Timeout <= 0 {
		return fmt.Errorf("health check timeout must be positive")
	}

	// Validate worker count
	if c.P2P.HealthCheck.Workers < 1 {
		return fmt.Errorf("health check workers must be at least 1")
	}

	return nil
}

// GetServerAddress returns the complete server address
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Address, c.Server.Port)
}

// LoadFromEnv loads configuration values from environment variables
func (c *Config) LoadFromEnv() {
	if addr := os.Getenv("OCTOSERVE_SERVER_ADDRESS"); addr != "" {
		c.Server.Address = addr
	}

	if port := os.Getenv("OCTOSERVE_SERVER_PORT"); port != "" {
		if p, err := parsePort(port); err == nil {
			c.Server.Port = p
		}
	}

	if path := os.Getenv("OCTOSERVE_STORAGE_PATH"); path != "" {
		c.Storage.Path = path
	}

	if level := os.Getenv("OCTOSERVE_LOG_LEVEL"); level != "" {
		c.Logging.Level = level
	}

	if format := os.Getenv("OCTOSERVE_LOG_FORMAT"); format != "" {
		c.Logging.Format = format
	}

	if enabled := os.Getenv("OCTOSERVE_METRICS_ENABLED"); enabled != "" {
		c.Metrics.Enabled = enabled == "true" || enabled == "1"
	}

	if endpoint := os.Getenv("OCTOSERVE_METRICS_ENDPOINT"); endpoint != "" {
		c.Metrics.Endpoint = endpoint
	}

	if username := os.Getenv("OCTOSERVE_METRICS_USERNAME"); username != "" {
		if c.Metrics.BasicAuth == nil {
			c.Metrics.BasicAuth = &BasicAuthConfig{}
		}
		c.Metrics.BasicAuth.Username = username
	}

	if password := os.Getenv("OCTOSERVE_METRICS_PASSWORD"); password != "" {
		if c.Metrics.BasicAuth == nil {
			c.Metrics.BasicAuth = &BasicAuthConfig{}
		}
		c.Metrics.BasicAuth.Password = password
	}

	// P2P configuration
	if enabled := os.Getenv("OCTOSERVE_P2P_ENABLED"); enabled != "" {
		c.P2P.Enabled = enabled == "true" || enabled == "1"
	}

	if nodeID := os.Getenv("OCTOSERVE_P2P_NODE_ID"); nodeID != "" {
		c.P2P.NodeID = nodeID
	}

	if port := os.Getenv("OCTOSERVE_P2P_PORT"); port != "" {
		if p, err := parsePort(port); err == nil {
			c.P2P.Port = p
		}
	}

	if peers := os.Getenv("OCTOSERVE_P2P_BOOTSTRAP_PEERS"); peers != "" {
		// Split comma-separated peers
		c.P2P.BootstrapPeers = strings.Split(peers, ",")
		for i, peer := range c.P2P.BootstrapPeers {
			c.P2P.BootstrapPeers[i] = strings.TrimSpace(peer)
		}
	}

	if method := os.Getenv("OCTOSERVE_P2P_DISCOVERY_METHOD"); method != "" {
		c.P2P.Discovery.Method = method
	}

	if strategy := os.Getenv("OCTOSERVE_P2P_REPLICATION_STRATEGY"); strategy != "" {
		c.P2P.Replication.Strategy = strategy
	}

	if factor := os.Getenv("OCTOSERVE_P2P_REPLICATION_FACTOR"); factor != "" {
		if f, err := parsePositiveInt(factor); err == nil {
			c.P2P.Replication.Factor = f
		}
	}

	if protocol := os.Getenv("OCTOSERVE_P2P_TRANSPORT_PROTOCOL"); protocol != "" {
		c.P2P.Transport.Protocol = protocol
	}
}

// parsePort parses a port string to int
func parsePort(portStr string) (int, error) {
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		return 0, err
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port out of range: %d", port)
	}
	return port, nil
}

// parsePositiveInt parses a positive integer string
func parsePositiveInt(intStr string) (int, error) {
	var value int
	if _, err := fmt.Sscanf(intStr, "%d", &value); err != nil {
		return 0, err
	}
	if value < 1 {
		return 0, fmt.Errorf("value must be positive: %d", value)
	}
	return value, nil
}
