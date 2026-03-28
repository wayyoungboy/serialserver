package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config represents application configuration
type Config struct {
	ServerURL  string `json:"server_url"`   // e.g., "http://192.168.1.100:9000"
	ServerHost string `json:"server_host"`  // parsed host
	ServerPort int    `json:"server_port"`  // parsed port
	UseHTTPS   bool   `json:"use_https"`    // whether to use https
	Username   string `json:"username"`
	Password   string `json:"password"` // Stored for convenience, user should re-enter if security is important
	DeviceKey  string `json:"device_key"`
	PortName   string `json:"port_name"`
	AutoStart  bool   `json:"auto_start"`
	AutoConnect bool  `json:"auto_connect"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		ServerURL:  "http://localhost:9000",
		ServerHost: "localhost",
		ServerPort: 9000,
		UseHTTPS:   false,
		AutoStart:  false,
		AutoConnect: false,
	}
}

// ParseServerURL parses server URL and updates host/port/https fields
func (c *Config) ParseServerURL() error {
	if c.ServerURL == "" {
		c.ServerURL = "http://localhost:9000"
	}

	// Add scheme if missing
	serverURL := c.ServerURL
	if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
		serverURL = "http://" + serverURL
	}

	u, err := url.Parse(serverURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	c.ServerHost = u.Hostname()
	c.UseHTTPS = u.Scheme == "https"

	// Parse port
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil {
			return fmt.Errorf("invalid port: %w", err)
		}
		c.ServerPort = port
	} else {
		// Default ports
		if c.UseHTTPS {
			c.ServerPort = 443
		} else {
			c.ServerPort = 80
		}
	}

	return nil
}

// Manager handles configuration persistence
type Manager struct {
	configPath string
	config     *Config
}

// NewManager creates a new config manager
func NewManager(configPath string) *Manager {
	return &Manager{
		configPath: configPath,
		config:     DefaultConfig(),
	}
}

// NewManagerWithDefaultPath creates a config manager with default path
func NewManagerWithDefaultPath() *Manager {
	// Get user's config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}

	appConfigDir := filepath.Join(configDir, "vsp-manager")
	configPath := filepath.Join(appConfigDir, "config.json")

	return NewManager(configPath)
}

// Load loads configuration from file
func (m *Manager) Load() (*Config, error) {
	// Ensure directory exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Check if config file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// Create default config file
		m.config = DefaultConfig()
		if err := m.Save(m.config); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return m.config, nil
	}

	// Read config file
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	m.config = &config
	return m.config, nil
}

// Save saves configuration to file
func (m *Manager) Save(config *Config) error {
	// Ensure directory exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	m.config = config
	return nil
}

// Get returns current configuration
func (m *Manager) Get() *Config {
	return m.config
}

// Set updates the configuration
func (m *Manager) Set(config *Config) {
	m.config = config
}

// GetConfigPath returns the config file path
func (m *Manager) GetConfigPath() string {
	return m.configPath
}