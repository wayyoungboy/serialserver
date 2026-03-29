//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"vsp-manager/internal/network"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultServerHost = "localhost"
	defaultServerPort = 9000
	defaultUsername   = "admin"
	defaultPassword   = "admin123"
)

// checkServerRunning checks if vsp-server is accessible
func checkServerRunning(t *testing.T) bool {
	url := fmt.Sprintf("http://%s:%d/api/v1/devices", defaultServerHost, defaultServerPort)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer test")

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return true
}

// E2ETestConfig holds test configuration
type E2ETestConfig struct {
	ServerHost string
	ServerPort int
	Username   string
	Password   string
	DeviceKey  string
}

func getTestConfig() E2ETestConfig {
	return E2ETestConfig{
		ServerHost: defaultServerHost,
		ServerPort: defaultServerPort,
		Username:   defaultUsername,
		Password:   defaultPassword,
	}
}

func TestE2ELoginFlow(t *testing.T) {
	if !checkServerRunning(t) {
		t.Skip("vsp-server not running on port 9000")
	}

	config := getTestConfig()
	client := network.NewAPIClient(config.ServerHost, config.ServerPort)

	// Test login
	t.Run("login_success", func(t *testing.T) {
		resp, err := client.Login(config.Username, config.Password)

		require.NoError(t, err)
		assert.NotEmpty(t, resp.Token)
		assert.Equal(t, config.Username, resp.User.Username)
	})

	t.Run("login_failure", func(t *testing.T) {
		badClient := network.NewAPIClient(config.ServerHost, config.ServerPort)

		_, err := badClient.Login("wronguser", "wrongpass")

		assert.Error(t, err)
	})
}

func TestE2EDeviceList(t *testing.T) {
	if !checkServerRunning(t) {
		t.Skip("vsp-server not running on port 9000")
	}

	config := getTestConfig()
	client := network.NewAPIClient(config.ServerHost, config.ServerPort)

	// Login first
	_, err := client.Login(config.Username, config.Password)
	require.NoError(t, err)

	// Get device list
	devices, err := client.GetDeviceList()

	require.NoError(t, err)
	t.Logf("Found %d devices", len(devices))

	for _, d := range devices {
		t.Logf("Device: %s (key: %s, status: %s)", d.Name, d.DeviceKey, d.Status)
	}
}

func TestE2ECreateDevice(t *testing.T) {
	if !checkServerRunning(t) {
		t.Skip("vsp-server not running on port 9000")
	}

	config := getTestConfig()

	// Login first
	client := network.NewAPIClient(config.ServerHost, config.ServerPort)
	_, err := client.Login(config.Username, config.Password)
	require.NoError(t, err)

	// Create device via direct HTTP call
	createURL := fmt.Sprintf("http://%s:%d/api/v1/devices", config.ServerHost, config.ServerPort)

	deviceData := map[string]interface{}{
		"name":        fmt.Sprintf("E2E Test Device %d", time.Now().Unix()),
		"serial_port": "COM1",
		"baud_rate":   115200,
		"data_bits":   8,
		"stop_bits":   1,
		"parity":      "N",
	}

	jsonData, _ := json.Marshal(deviceData)

	req, _ := http.NewRequest("POST", createURL, bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.GetToken()))

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("Create device response: %s", string(body))

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		t.Log("Device created successfully")
	}
}

func TestE2EWebSocketConnection(t *testing.T) {
	if !checkServerRunning(t) {
		t.Skip("vsp-server not running on port 9000")
	}

	config := getTestConfig()

	// First get a device key
	client := network.NewAPIClient(config.ServerHost, config.ServerPort)
	_, err := client.Login(config.Username, config.Password)
	require.NoError(t, err)

	devices, err := client.GetDeviceList()
	require.NoError(t, err)

	if len(devices) == 0 {
		t.Skip("No devices available for testing")
	}

	deviceKey := devices[0].DeviceKey

	// Connect WebSocket
	wsClient := network.NewWSClient(config.ServerHost, config.ServerPort)
	wsClient.SetDeviceKey(deviceKey)

	// Track connection events
	connected := false
	wsClient.OnConnected(func() {
		connected = true
		t.Log("WebSocket connected")
	})

	wsClient.OnDisconnected(func() {
		connected = false
		t.Log("WebSocket disconnected")
	})

	wsClient.OnError(func(err error) {
		t.Logf("WebSocket error: %v", err)
	})

	err = wsClient.Connect(nil)
	require.NoError(t, err)

	// Wait a bit
	time.Sleep(2 * time.Second)

	assert.True(t, wsClient.IsConnected())
	t.Logf("Device online status: %s", wsClient.DeviceOnlineStatus)

	// Cleanup
	wsClient.Close()
}

func TestE2EFullFlow(t *testing.T) {
	// This test requires:
	// 1. vsp-server running
	// 2. A device configured
	// 3. device-client connected (optional for full data flow)

	if !checkServerRunning(t) {
		t.Skip("vsp-server not running on port 9000")
	}

	config := getTestConfig()

	t.Log("=== Starting E2E Full Flow Test ===")

	// Step 1: Login
	t.Log("Step 1: Logging in...")
	apiClient := network.NewAPIClient(config.ServerHost, config.ServerPort)
	_, err := apiClient.Login(config.Username, config.Password)
	require.NoError(t, err)
	t.Log("Login successful")

	// Step 2: Get devices
	t.Log("Step 2: Getting device list...")
	devices, err := apiClient.GetDeviceList()
	require.NoError(t, err)
	t.Logf("Found %d devices", len(devices))

	if len(devices) == 0 {
		t.Log("No devices found - skipping remaining steps")
		return
	}

	deviceKey := devices[0].DeviceKey
	t.Logf("Using device: %s (key: %s)", devices[0].Name, deviceKey)

	// Step 3: Connect WebSocket
	t.Log("Step 3: Connecting WebSocket...")
	wsClient := network.NewWSClient(config.ServerHost, config.ServerPort)
	wsClient.SetDeviceKey(deviceKey)

	dataReceived := make([][]byte, 0)
	wsClient.OnData(func(data []byte) {
		dataReceived = append(dataReceived, data)
		t.Logf("Received %d bytes", len(data))
	})

	err = wsClient.Connect(nil)
	require.NoError(t, err)
	t.Log("WebSocket connected")

	// Step 4: Send test data
	t.Log("Step 4: Sending test data...")
	testData := []byte("E2E Test Message")
	err = wsClient.Send(testData)
	if err != nil {
		t.Logf("Send error (device may not be online): %v", err)
	} else {
		t.Log("Test data sent successfully")
	}

	// Step 5: Wait and check status
	time.Sleep(2 * time.Second)
	t.Logf("Device status: %s", wsClient.DeviceOnlineStatus)

	// Step 6: Disconnect
	t.Log("Step 6: Disconnecting...")
	wsClient.Close()
	t.Log("Disconnected")

	t.Log("=== E2E Full Flow Test Complete ===")
}