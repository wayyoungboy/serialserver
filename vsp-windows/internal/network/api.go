package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// APIClient represents a REST API client for VSP server
type APIClient struct {
	baseURL    string
	originURL  string
	token      string
	httpClient *http.Client
}

// Device represents a device from the API
type Device struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Status      string `json:"status"`
	LastOnline  string `json:"last_online"`
	CreatedAt   string `json:"created_at"`
}

// MappingState represents a serial mapping currently announced by a device.
type MappingState struct {
	Mapping Mapping `json:"mapping"`
	Online  bool    `json:"online"`
	Busy    bool    `json:"busy"`
}

// Mapping describes a device-side serial mapping.
type Mapping struct {
	ID     string         `json:"id"`
	Name   string         `json:"name,omitempty"`
	Serial SerialSettings `json:"serial"`
}

// SerialSettings are announced by the device agent; the server only relays them.
type SerialSettings struct {
	Port        string `json:"port"`
	BaudRate    int    `json:"baud_rate"`
	DataBits    int    `json:"data_bits"`
	StopBits    int    `json:"stop_bits"`
	Parity      string `json:"parity"`
	FlowControl string `json:"flow_control,omitempty"`
}

// User represents a user from the API
type User struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// LoginResponse represents the login API response
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// APIResponse represents a generic API response wrapper
type APIResponse struct {
	Data  interface{} `json:"data"`
	Error string      `json:"error,omitempty"`
}

// NewAPIClient creates a new API client
func NewAPIClient(host string, port int) *APIClient {
	return NewAPIClientFromURL(fmt.Sprintf("http://%s:%d", host, port))
}

// NewAPIClientFromURL creates a client from an http(s) server URL.
func NewAPIClientFromURL(serverURL string) *APIClient {
	origin := normalizeOrigin(serverURL)
	return &APIClient{
		baseURL:   fmt.Sprintf("%s/api", origin),
		originURL: origin,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RelayWebSocketURL builds a relay WebSocket URL.
func (c *APIClient) RelayWebSocketURL(path string) string {
	u, err := url.Parse(c.originURL)
	if err != nil {
		return ""
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// SetToken sets the authentication token
func (c *APIClient) SetToken(token string) {
	c.token = token
}

// Login authenticates with the server and returns a JWT token
func (c *APIClient) Login(username, password string) (*LoginResponse, error) {
	url := fmt.Sprintf("%s/auth/login", c.baseURL)

	payload := map[string]string{
		"username": username,
		"password": password,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiResp APIResponse
		if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.Error != "" {
			return nil, fmt.Errorf("login failed: %s", apiResp.Error)
		}
		return nil, fmt.Errorf("login failed: status %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Parse the data field as LoginResponse
	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal data error: %w", err)
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(dataBytes, &loginResp); err != nil {
		return nil, fmt.Errorf("parse login response error: %w", err)
	}

	// Store the token
	c.token = loginResp.Token

	return &loginResp, nil
}

// GetDeviceList retrieves the list of devices from the server
func (c *APIClient) GetDeviceList() ([]Device, error) {
	if c.token == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	url := fmt.Sprintf("%s/devices", c.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request error: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiResp APIResponse
		if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.Error != "" {
			return nil, fmt.Errorf("get devices failed: %s", apiResp.Error)
		}
		return nil, fmt.Errorf("get devices failed: status %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Parse the data field as Device array
	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal data error: %w", err)
	}

	var devices []Device
	if err := json.Unmarshal(dataBytes, &devices); err != nil {
		return nil, fmt.Errorf("parse devices error: %w", err)
	}

	return devices, nil
}

// GetDeviceMappings retrieves online mappings announced by a device agent.
func (c *APIClient) GetDeviceMappings(deviceID uint) ([]MappingState, error) {
	if c.token == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	reqURL := fmt.Sprintf("%s/devices/%d/mappings", c.baseURL, deviceID)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request error: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiResp APIResponse
		if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.Error != "" {
			return nil, fmt.Errorf("get mappings failed: %s", apiResp.Error)
		}
		return nil, fmt.Errorf("get mappings failed: status %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal data error: %w", err)
	}

	var mappings []MappingState
	if err := json.Unmarshal(dataBytes, &mappings); err != nil {
		return nil, fmt.Errorf("parse mappings error: %w", err)
	}
	return mappings, nil
}

// IsAuthenticated returns whether the client has a token
func (c *APIClient) IsAuthenticated() bool {
	return c.token != ""
}

// ClearToken clears the authentication token
func (c *APIClient) ClearToken() {
	c.token = ""
}

// GetToken returns the current authentication token
func (c *APIClient) GetToken() string {
	return c.token
}

func normalizeOrigin(serverURL string) string {
	serverURL = strings.TrimSpace(serverURL)
	if serverURL == "" {
		serverURL = "http://localhost:9000"
	}
	if !strings.Contains(serverURL, "://") {
		serverURL = "http://" + serverURL
	}
	u, err := url.Parse(serverURL)
	if err != nil || u.Host == "" {
		return "http://localhost:9000"
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/")
}
