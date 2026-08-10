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

// App is the Wails application bridge.
type App struct {
	ctx           context.Context
	configManager *config.Manager
	apiClient     *network.APIClient
	tunnelService *service.TunnelService

	mu           sync.RWMutex
	loggedIn     bool
	currentUser  *network.User
	currentToken string

	statusUpdateChan chan service.TunnelStatus
}

// AppConfig is sent to the frontend during startup.
type AppConfig struct {
	ServerURL     string `json:"server_url"`
	Username      string `json:"username"`
	AutoConnect   bool   `json:"auto_connect"`
	DeviceID      uint   `json:"device_id"`
	MappingID     string `json:"mapping_id"`
	ListenAddress string `json:"listen_address"`
}

// ConnectionStatus is formatted for frontend display.
type ConnectionStatus struct {
	Connected      bool   `json:"connected"`
	LocalListening bool   `json:"local_listening"`
	RelayConnected bool   `json:"relay_connected"`
	ListenAddress  string `json:"listen_address,omitempty"`
	DeviceID       uint   `json:"device_id,omitempty"`
	MappingID      string `json:"mapping_id,omitempty"`
	MappingName    string `json:"mapping_name,omitempty"`
	RemotePort     string `json:"remote_port,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	BytesSent      int64  `json:"bytes_sent"`
	BytesReceived  int64  `json:"bytes_received"`
	ConnectedSince string `json:"connected_since,omitempty"`
	Error          string `json:"error,omitempty"`
	LastEvent      string `json:"last_event,omitempty"`
	LoggedIn       bool   `json:"logged_in"`
	Username       string `json:"username,omitempty"`
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{
		configManager:    config.NewManagerWithDefaultPath(),
		tunnelService:    service.NewTunnelService(),
		statusUpdateChan: make(chan service.TunnelStatus, 10),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	log.Println("VSP Manager starting...")

	cfg, err := a.configManager.Load()
	if err != nil {
		log.Printf("Failed to load config: %v", err)
		cfg = config.DefaultConfig()
	}
	if cfg.ServerURL != "" {
		if err := cfg.ParseServerURL(); err != nil {
			log.Printf("Failed to parse server URL: %v", err)
		}
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "127.0.0.1:7000"
	}

	a.apiClient = network.NewAPIClientFromURL(cfg.ServerURL)
	a.tunnelService.OnStatusChange(func(status service.TunnelStatus) {
		select {
		case a.statusUpdateChan <- status:
		default:
			log.Printf("Dropped status update because channel is full")
		}
	})
	go a.emitStatusEvents()

	if cfg.Username != "" && cfg.Password != "" {
		log.Printf("Attempting auto-login with saved credentials...")
		if _, err := a.Login(cfg.Username, cfg.Password); err != nil {
			log.Printf("Auto-login failed: %v", err)
		} else if cfg.AutoConnect && cfg.DeviceID != 0 && cfg.MappingID != "" {
			log.Printf("Attempting auto-connect...")
			if err := a.Connect(cfg.DeviceID, cfg.MappingID, cfg.ListenAddr); err != nil {
				log.Printf("Auto-connect failed: %v", err)
			}
		}
	}
}

func (a *App) shutdown(ctx context.Context) {
	log.Println("VSP Manager shutting down...")
	if a.tunnelService != nil {
		if err := a.tunnelService.Cleanup(); err != nil {
			log.Printf("Cleanup error: %v", err)
		}
	}

	cfg := a.configManager.Get()
	if err := a.configManager.Save(cfg); err != nil {
		log.Printf("Failed to save config: %v", err)
	}
}

func (a *App) emitStatusEvents() {
	for {
		select {
		case status := <-a.statusUpdateChan:
			statusJSON, err := json.Marshal(a.convertStatus(status))
			if err == nil {
				wailsRuntime.EventsEmit(a.ctx, "statusUpdate", string(statusJSON))
			}
		case <-a.ctx.Done():
			return
		}
	}
}

func (a *App) convertStatus(ts service.TunnelStatus) ConnectionStatus {
	a.mu.RLock()
	username := ""
	if a.currentUser != nil {
		username = a.currentUser.Username
	}
	loggedIn := a.loggedIn
	a.mu.RUnlock()

	cs := ConnectionStatus{
		Connected:      ts.Connected,
		LocalListening: ts.LocalListening,
		RelayConnected: ts.RelayConnected,
		ListenAddress:  ts.ListenAddress,
		DeviceID:       ts.DeviceID,
		MappingID:      ts.MappingID,
		MappingName:    ts.MappingName,
		RemotePort:     ts.RemotePort,
		SessionID:      ts.SessionID,
		BytesSent:      ts.BytesSent,
		BytesReceived:  ts.BytesReceived,
		Error:          ts.Error,
		LastEvent:      ts.LastEvent,
		LoggedIn:       loggedIn,
		Username:       username,
	}
	if !ts.ConnectedSince.IsZero() {
		cs.ConnectedSince = ts.ConnectedSince.Format("2006-01-02 15:04:05")
	}
	return cs
}

// GetVersion returns the application version.
func (a *App) GetVersion() string {
	return "0.0.3"
}

// LoadConfig loads the saved configuration.
func (a *App) LoadConfig() *AppConfig {
	cfg, err := a.configManager.Load()
	if err != nil {
		log.Printf("Failed to load config: %v", err)
		cfg = config.DefaultConfig()
	}

	if cfg.ServerURL == "" && cfg.ServerHost != "" {
		scheme := "http"
		if cfg.UseHTTPS {
			scheme = "https"
		}
		cfg.ServerURL = fmt.Sprintf("%s://%s:%d", scheme, cfg.ServerHost, cfg.ServerPort)
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "127.0.0.1:7000"
	}

	return &AppConfig{
		ServerURL:     cfg.ServerURL,
		Username:      cfg.Username,
		AutoConnect:   cfg.AutoConnect,
		DeviceID:      cfg.DeviceID,
		MappingID:     cfg.MappingID,
		ListenAddress: cfg.ListenAddr,
	}
}

// SaveConfig saves the server URL and auto-connect preference.
func (a *App) SaveConfig(serverURL string, autoConnect bool) error {
	cfg := a.configManager.Get()
	cfg.ServerURL = serverURL
	cfg.AutoConnect = autoConnect
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "127.0.0.1:7000"
	}

	if err := cfg.ParseServerURL(); err != nil {
		return err
	}
	if err := a.configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	a.apiClient = network.NewAPIClientFromURL(cfg.ServerURL)
	log.Printf("Config saved: server=%s", serverURL)
	return nil
}

// Login authenticates with the control plane.
func (a *App) Login(username, password string) (*network.User, error) {
	if a.apiClient == nil {
		cfg := a.configManager.Get()
		a.apiClient = network.NewAPIClientFromURL(cfg.ServerURL)
	}

	resp, err := a.apiClient.Login(username, password)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	a.mu.Lock()
	a.loggedIn = true
	a.currentUser = &resp.User
	a.currentToken = resp.Token
	a.mu.Unlock()

	cfg := a.configManager.Get()
	cfg.Username = username
	cfg.Password = password
	a.configManager.Set(cfg)

	log.Printf("Login successful: user=%s", resp.User.Username)
	return &resp.User, nil
}

// Logout clears auth state and stops any active gateway.
func (a *App) Logout() error {
	a.mu.Lock()
	a.loggedIn = false
	a.currentUser = nil
	a.currentToken = ""
	a.mu.Unlock()

	if a.apiClient != nil {
		a.apiClient.ClearToken()
	}
	if a.tunnelService.IsListening() {
		if err := a.tunnelService.Disconnect(); err != nil {
			log.Printf("Disconnect during logout: %v", err)
		}
	}

	cfg := a.configManager.Get()
	cfg.Password = ""
	cfg.AutoConnect = false
	a.configManager.Set(cfg)
	log.Println("Logout successful")
	return nil
}

// GetDevices retrieves devices available to the current user.
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
	return devices, nil
}

// GetMappings returns mappings currently announced by a device agent.
func (a *App) GetMappings(deviceID uint) ([]network.MappingState, error) {
	a.mu.RLock()
	loggedIn := a.loggedIn
	a.mu.RUnlock()
	if !loggedIn {
		return nil, fmt.Errorf("not logged in")
	}
	if deviceID == 0 {
		return nil, fmt.Errorf("device id is required")
	}
	return a.apiClient.GetDeviceMappings(deviceID)
}

// Connect starts a local TCP gateway for the selected device mapping.
func (a *App) Connect(deviceID uint, mappingID string, listenAddress string) error {
	a.mu.RLock()
	token := a.currentToken
	a.mu.RUnlock()
	if token == "" {
		return fmt.Errorf("not logged in")
	}
	if mappingID == "" {
		return fmt.Errorf("mapping id is required")
	}
	if listenAddress == "" {
		listenAddress = "127.0.0.1:7000"
	}

	cfg := a.configManager.Get()
	if a.apiClient == nil {
		a.apiClient = network.NewAPIClientFromURL(cfg.ServerURL)
	}

	err := a.tunnelService.Connect(service.TunnelConfig{
		ServerURL:     cfg.ServerURL,
		UserToken:     token,
		DeviceID:      deviceID,
		MappingID:     mappingID,
		ListenAddress: listenAddress,
	})
	if err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	cfg.DeviceID = deviceID
	cfg.MappingID = mappingID
	cfg.ListenAddr = listenAddress
	a.configManager.Set(cfg)
	log.Printf("gateway listening: device=%d mapping=%s local=%s", deviceID, mappingID, listenAddress)
	return nil
}

// Disconnect closes the active local TCP gateway.
func (a *App) Disconnect() error {
	if err := a.tunnelService.Disconnect(); err != nil {
		return fmt.Errorf("disconnect failed: %w", err)
	}
	return nil
}

// GetStatus returns the current connection status.
func (a *App) GetStatus() ConnectionStatus {
	return a.convertStatus(a.tunnelService.GetStatus())
}

// IsLoggedIn returns the login state.
func (a *App) IsLoggedIn() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.loggedIn
}

// GetCurrentUsername returns the current logged-in username.
func (a *App) GetCurrentUsername() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.currentUser != nil {
		return a.currentUser.Username
	}
	return ""
}

// GetLogPath returns the path where logs are stored.
func (a *App) GetLogPath() string {
	logDir, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	return filepath.Join(logDir, "vsp-manager", "logs")
}
