//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"vsp-manager/internal/network"
	"vsp-manager/tests/mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWSAuthentication(t *testing.T) {
	mockServer := mock.NewMockServer()
	mockServer.SetDeviceKey("test-key-123")
	mockServer.SetAuthSuccess(true)

	err := mockServer.Start(19000)
	require.NoError(t, err)
	defer mockServer.Stop()

	time.Sleep(100 * time.Millisecond)

	t.Run("success", func(t *testing.T) {
		client := network.NewWSClient("localhost", 19000)
		client.SetDeviceKey("test-key-123")

		ctx := context.Background()
		err := client.Connect(ctx)

		assert.NoError(t, err)
		assert.True(t, client.IsConnected())

		client.Close()
	})

	t.Run("failure", func(t *testing.T) {
		mockServer.SetAuthSuccess(false)

		client := network.NewWSClient("localhost", 19000)
		client.SetDeviceKey("invalid-key")

		ctx := context.Background()
		err := client.Connect(ctx)

		assert.Error(t, err)
	})
}

func TestWSDataSending(t *testing.T) {
	mockServer := mock.NewMockServer()
	mockServer.SetDeviceKey("test-key")
	mockServer.SetAuthSuccess(true)
	mockServer.SetDeviceOnline(true)

	err := mockServer.Start(19001)
	require.NoError(t, err)
	defer mockServer.Stop()

	time.Sleep(100 * time.Millisecond)

	client := network.NewWSClient("localhost", 19001)
	client.SetDeviceKey("test-key")

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	defer client.Close()

	// Test sending data
	testData := []byte("Hello Device")
	err = client.Send(testData)
	assert.NoError(t, err)

	// Wait for server to receive
	time.Sleep(200 * time.Millisecond)

	// Verify server received data
	received := mockServer.GetReceivedData()
	assert.Greater(t, len(received), 0, "Server should have received data")

	// Parse received message
	var msg mock.WSMessage
	err = json.Unmarshal(received[0], &msg)
	require.NoError(t, err)
	assert.Equal(t, "data", msg.Type)

	// Decode data payload
	var payload mock.DataPayload
	err = json.Unmarshal(msg.Payload, &payload)
	require.NoError(t, err)

	decoded, err := base64.StdEncoding.DecodeString(payload.Data)
	require.NoError(t, err)
	assert.Equal(t, testData, decoded)
}

func TestWSDataReceiving(t *testing.T) {
	mockServer := mock.NewMockServer()
	mockServer.SetDeviceKey("test-key")
	mockServer.SetAuthSuccess(true)

	err := mockServer.Start(19002)
	require.NoError(t, err)
	defer mockServer.Stop()

	time.Sleep(100 * time.Millisecond)

	client := network.NewWSClient("localhost", 19002)
	client.SetDeviceKey("test-key")

	// Set up data callback
	receivedData := make([]byte, 0)
	client.OnData(func(data []byte) {
		receivedData = append(receivedData, data...)
	})

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	defer client.Close()

	// Server sends data to client
	testData := []byte("Response from server")
	mockServer.SendDataToClient(testData)

	// Wait for client to receive
	time.Sleep(200 * time.Millisecond)

	assert.Greater(t, len(receivedData), 0, "Client should have received data")
	assert.Equal(t, testData, receivedData)
}

func TestDeviceStatusChanges(t *testing.T) {
	mockServer := mock.NewMockServer()
	mockServer.SetDeviceKey("test-key")
	mockServer.SetAuthSuccess(true)
	mockServer.SetDeviceOnline(false)

	err := mockServer.Start(19003)
	require.NoError(t, err)
	defer mockServer.Stop()

	time.Sleep(100 * time.Millisecond)

	client := network.NewWSClient("localhost", 19003)
	client.SetDeviceKey("test-key")

	// Track status changes
	statusChanges := make([]string, 0)
	client.OnStatus(func(status string) {
		statusChanges = append(statusChanges, status)
	})

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	defer client.Close()

	// Initial status should be device_offline or connected
	time.Sleep(100 * time.Millisecond)

	// Simulate device online
	mockServer.SendStatusToClient("device_online")
	time.Sleep(200 * time.Millisecond)

	assert.Contains(t, statusChanges, "device_online")
	assert.Equal(t, "device_online", client.DeviceOnlineStatus)

	// Simulate device offline
	mockServer.SendStatusToClient("device_offline")
	time.Sleep(200 * time.Millisecond)

	assert.Contains(t, statusChanges, "device_offline")
}

func TestAPILogin(t *testing.T) {
	mockServer := mock.NewMockServer()
	mockServer.SetDeviceKey("test-key")

	err := mockServer.Start(19004)
	require.NoError(t, err)
	defer mockServer.Stop()

	time.Sleep(100 * time.Millisecond)

	client := network.NewAPIClient("localhost", 19004)

	t.Run("success", func(t *testing.T) {
		resp, err := client.Login("admin", "admin123")

		require.NoError(t, err)
		assert.NotEmpty(t, resp.Token)
		assert.Equal(t, "admin", resp.User.Username)
		assert.True(t, client.IsAuthenticated())
	})

	t.Run("failure", func(t *testing.T) {
		client := network.NewAPIClient("localhost", 19004)

		_, err := client.Login("invalid", "invalid")

		assert.Error(t, err)
		assert.False(t, client.IsAuthenticated())
	})
}

func TestAPIGetDevices(t *testing.T) {
	mockServer := mock.NewMockServer()
	mockServer.SetDeviceKey("device-abc123")

	err := mockServer.Start(19005)
	require.NoError(t, err)
	defer mockServer.Stop()

	time.Sleep(100 * time.Millisecond)

	client := network.NewAPIClient("localhost", 19005)

	// Login first
	_, err = client.Login("admin", "admin123")
	require.NoError(t, err)

	// Get devices
	devices, err := client.GetDeviceList()

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(devices), 1)
	assert.Equal(t, "device-abc123", devices[0].DeviceKey)
	assert.Equal(t, "Test Device", devices[0].Name)
}

func TestConnectionMultipleClients(t *testing.T) {
	mockServer := mock.NewMockServer()
	mockServer.SetDeviceKey("test-key")

	err := mockServer.Start(19006)
	require.NoError(t, err)
	defer mockServer.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create multiple clients
	clients := make([]*network.WSClient, 3)
	for i := 0; i < 3; i++ {
		clients[i] = network.NewWSClient("localhost", 19006)
		clients[i].SetDeviceKey("test-key")

		err := clients[i].Connect(context.Background())
		assert.NoError(t, err, "Client %d should connect", i)
	}

	// Cleanup
	for _, c := range clients {
		c.Close()
	}
}

func TestConnectionTimeout(t *testing.T) {
	// Test connecting to non-existent server
	client := network.NewWSClient("localhost", 19999) // Non-existent port
	client.SetDeviceKey("test-key")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	assert.Error(t, err, "Should fail to connect to non-existent server")
}