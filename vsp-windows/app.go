package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"vsp-manager/internal/config"
	"vsp-manager/internal/network"
	"vsp-manager/internal/service"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx          context.Context
	configManager *config.Manager
	apiClient    *network.APIClient
	tunnelService *service.TunnelService

	// State
	mu           sync.RWMutex
	loggedIn     bool
	currentUser  *network.User
	currentToken string

	// Event handling
	statusUpdateChan chan service.TunnelStatus
}

// AppConfig for frontend initialization
type AppConfig struct {
	ServerURL   string `json:"server_url"`
	Username    string `json:"username"`
	AutoConnect bool   `json:"auto_connect"`
}

// ConnectionStatus for frontend display
type ConnectionStatus struct {
	Connected      bool   `json:"connected"`
	VisiblePort    string `json:"visible_port,omitempty"`
	HiddenPort     string `json:"hidden_port,omitempty"`
	DeviceOnline   bool   `json:"device_online"`
	DeviceStatus   string `json:"device_status,omitempty"`
	BytesSent      int64  `json:"bytes_sent"`
	BytesReceived  int64  `json:"bytes_received"`
	ConnectedSince string `json:"connected_since,omitempty"`
	Error          string `json:"error,omitempty"`
	LoggedIn       bool   `json:"logged_in"`
	Username       string `json:"username,omitempty"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		configManager: config.NewManagerWithDefaultPath(),
		statusUpdateChan: make(chan service.TunnelStatus, 10),
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	log.Println("VSP Manager starting...")

	// Load configuration
	cfg, err := a.configManager.Load()
	if err != nil {
		log.Printf("Failed to load config: %v", err)
	} else {
		// Parse ServerURL if needed
		if cfg.ServerURL != "" {
			cfg.ParseServerURL()
		}
		log.Printf("Config loaded: server=%s", cfg.ServerURL)
	}

	// Initialize API client
	a.apiClient = network.NewAPIClient(cfg.ServerHost, cfg.ServerPort)

	// Initialize tunnel service
	a.tunnelService = service.NewTunnelService()

	// Set up status change callback
	a.tunnelService.OnStatusChange(func(status service.TunnelStatus) {
		a.statusUpdateChan <- status
	})

	// Start event emitter goroutine
	go a.emitStatusEvents()

	// Check if com0com is installed
	if !a.tunnelService.CheckCom0ComInstalled() {
		log.Println("WARNING: com0com driver not installed!")
	}

	// Auto-login if credentials saved
	if cfg.Username != "" && cfg.Password != "" {
		log.Printf("Attempting auto-login with saved credentials...")
		_, err := a.Login(cfg.Username, cfg.Password)
		if err != nil {
			log.Printf("Auto-login failed: %v", err)
		} else {
			log.Printf("Auto-login successful")
			// Auto-connect if enabled and device key saved
			if cfg.AutoConnect && cfg.DeviceKey != "" {
				log.Printf("Attempting auto-connect...")
				if err := a.Connect(cfg.DeviceKey); err != nil {
					log.Printf("Auto-connect failed: %v", err)
				}
			}
		}
	}
}

// shutdown is called when the app is shutting down
func (a *App) shutdown(ctx context.Context) {
	log.Println("VSP Manager shutting down...")

	// Disconnect tunnel
	if a.tunnelService != nil {
		if err := a.tunnelService.Cleanup(); err != nil {
			log.Printf("Cleanup error: %v", err)
		}
	}

	// Save current configuration
	cfg := a.configManager.Get()
	if err := a.configManager.Save(cfg); err != nil {
		log.Printf("Failed to save config: %v", err)
	}
}

// emitStatusEvents emits status updates to frontend via Wails events
func (a *App) emitStatusEvents() {
	for {
		select {
		case status := <-a.statusUpdateChan:
			// Emit event to frontend
			statusJSON, err := json.Marshal(a.convertStatus(status))
			if err == nil {
				wailsRuntime.EventsEmit(a.ctx, "statusUpdate", string(statusJSON))
			}
		case <-a.ctx.Done():
			return
		}
	}
}

// convertStatus converts TunnelStatus to ConnectionStatus
func (a *App) convertStatus(ts service.TunnelStatus) ConnectionStatus {
	cs := ConnectionStatus{
		Connected:      ts.Connected,
		VisiblePort:    ts.VisiblePort,
		HiddenPort:     ts.HiddenPort,
		DeviceOnline:   ts.DeviceOnline,
		DeviceStatus:   ts.DeviceStatus,
		BytesSent:      ts.BytesSent,
		BytesReceived:  ts.BytesReceived,
		Error:          ts.Error,
		LoggedIn:       a.loggedIn,
	}

	if !ts.ConnectedSince.IsZero() {
		cs.ConnectedSince = ts.ConnectedSince.Format("2006-01-02 15:04:05")
	}

	a.mu.RLock()
	if a.currentUser != nil {
		cs.Username = a.currentUser.Username
	}
	a.mu.RUnlock()

	return cs
}

// GetVersion returns the application version
func (a *App) GetVersion() string {
	return "0.1.0"
}

// GetAppConfig returns the current application configuration
func (a *App) GetAppConfig() AppConfig {
	cfg := a.configManager.Get()
	return AppConfig{
		ServerURL:   cfg.ServerURL,
		Username:    cfg.Username,
		AutoConnect: cfg.AutoConnect,
	}
}

// Login authenticates with the server
func (a *App) Login(username, password string) (*network.User, error) {
	resp, err := a.apiClient.Login(username, password)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	a.mu.Lock()
	a.loggedIn = true
	a.currentUser = &resp.User
	a.currentToken = resp.Token
	a.mu.Unlock()

	// Save credentials to config
	cfg := a.configManager.Get()
	cfg.Username = username
	cfg.Password = password // Note: storing password for convenience
	a.configManager.Set(cfg)

	log.Printf("Login successful: user=%s", resp.User.Username)
	return &resp.User, nil
}

// Logout clears the authentication state
func (a *App) Logout() error {
	a.mu.Lock()
	a.loggedIn = false
	a.currentUser = nil
	a.currentToken = ""
	a.mu.Unlock()

	a.apiClient.ClearToken()

	// Disconnect tunnel if connected
	if a.tunnelService.IsConnected() {
		if err := a.tunnelService.Disconnect(); err != nil {
			log.Printf("Disconnect during logout: %v", err)
		}
	}

	// Clear saved password
	cfg := a.configManager.Get()
	cfg.Password = ""
	cfg.AutoConnect = false
	a.configManager.Set(cfg)

	log.Println("Logout successful")
	return nil
}

// GetDevices retrieves the list of devices from the server
func (a *App) GetDevices() ([]network.Device, error) {
	a.mu.RLock()
	loggedIn := a.loggedIn
	a.mu.RUnlock()

	if !loggedIn {
		return nil, fmt.Errorf("not logged in")
	}

	devices, err := a.apiClient.GetDeviceList()
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	log.Printf("Retrieved %d devices", len(devices))
	return devices, nil
}

// Connect establishes the tunnel connection with the specified device
func (a *App) Connect(deviceKey string) error {
	cfg := a.configManager.Get()

	// Update API client if server changed
	a.apiClient = network.NewAPIClient(cfg.ServerHost, cfg.ServerPort)

	// Restore token if we have one
	if a.currentToken != "" {
		a.apiClient.SetToken(a.currentToken)
	}

	err := a.tunnelService.Connect(cfg.ServerHost, cfg.ServerPort, deviceKey)
	if err != nil {
		log.Printf("Connect failed: %v", err)
		return fmt.Errorf("connect failed: %w", err)
	}

	// Save device key to config
	cfg.DeviceKey = deviceKey
	a.configManager.Set(cfg)

	log.Printf("Connected to device: %s", deviceKey)
	return nil
}

// Disconnect closes the tunnel connection
func (a *App) Disconnect() error {
	err := a.tunnelService.Disconnect()
	if err != nil {
		log.Printf("Disconnect failed: %v", err)
		return fmt.Errorf("disconnect failed: %w", err)
	}

	log.Println("Disconnected successfully")
	return nil
}

// GetStatus returns the current connection status
func (a *App) GetStatus() ConnectionStatus {
	ts := a.tunnelService.GetStatus()
	return a.convertStatus(ts)
}

// SaveConfig saves the application configuration
func (a *App) SaveConfig(serverURL string, autoConnect bool) error {
	cfg := a.configManager.Get()
	cfg.ServerURL = serverURL
	cfg.AutoConnect = autoConnect

	// Parse URL to get host and port
	if err := cfg.ParseServerURL(); err != nil {
		return err
	}

	if err := a.configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Update API client
	a.apiClient = network.NewAPIClient(cfg.ServerHost, cfg.ServerPort)

	log.Printf("Config saved: server=%s", serverURL)
	return nil
}

// LoadConfig loads the application configuration
func (a *App) LoadConfig() *AppConfig {
	cfg, err := a.configManager.Load()
	if err != nil {
		log.Printf("Failed to load config: %v", err)
		return &AppConfig{ServerURL: "http://localhost:9000"}
	}

	// Parse URL if needed
	if cfg.ServerURL == "" && cfg.ServerHost != "" {
		scheme := "http"
		if cfg.UseHTTPS {
			scheme = "https"
		}
		cfg.ServerURL = fmt.Sprintf("%s://%s:%d", scheme, cfg.ServerHost, cfg.ServerPort)
	}

	return &AppConfig{
		ServerURL:   cfg.ServerURL,
		Username:    cfg.Username,
		AutoConnect: cfg.AutoConnect,
	}
}

// CheckCom0ComInstalled checks if com0com driver is installed
func (a *App) CheckCom0ComInstalled() bool {
	return a.tunnelService.CheckCom0ComInstalled()
}

// GetLogPath returns the path where logs are stored
func (a *App) GetLogPath() string {
	logDir, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	return filepath.Join(logDir, "vsp-manager", "logs")
}

// IsLoggedIn returns the login state
func (a *App) IsLoggedIn() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.loggedIn
}

// GetCurrentUsername returns the current logged-in username
func (a *App) GetCurrentUsername() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.currentUser != nil {
		return a.currentUser.Username
	}
	return ""
}