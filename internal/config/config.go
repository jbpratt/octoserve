package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config represents the application configuration
type Config struct {
	Server  ServerConfig  `json:"server"`
	Storage StorageConfig `json:"storage"`
	Logging LoggingConfig `json:"logging"`
	Metrics MetricsConfig `json:"metrics"`
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
