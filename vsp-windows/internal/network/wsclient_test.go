package network

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestWSMessageMarshal(t *testing.T) {
	tests := []struct {
		name     string
		msg      WSMessage
		expected string
	}{
		{
			name: "auth message",
			msg: WSMessage{
				Type: "auth",
				Payload: mustMarshalTest(AuthPayload{DeviceKey: "test-key-123"}),
			},
			expected: `{"type":"auth","payload":{"device_key":"test-key-123"}}`,
		},
		{
			name: "data message",
			msg: WSMessage{
				Type: "data",
				Payload: mustMarshalTest(DataPayload{Data: "SGVsbG8="}), // "Hello" base64
			},
			expected: `{"type":"data","payload":{"data":"SGVsbG8="}}`,
		},
		{
			name: "status message",
			msg: WSMessage{
				Type: "status",
				Payload: mustMarshalTest(StatusPayload{Status: "device_online"}),
			},
			expected: `{"type":"status","payload":{"status":"device_online"}}`,
		},
		{
			name: "pong message",
			msg: WSMessage{
				Type: "pong",
			},
			expected: `{"type":"pong"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.msg)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			// Verify type field
			var parsed WSMessage
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if parsed.Type != tt.msg.Type {
				t.Errorf("Expected type '%s', got '%s'", tt.msg.Type, parsed.Type)
			}
		})
	}
}

func mustMarshalTest(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func TestAuthPayload(t *testing.T) {
	payload := AuthPayload{DeviceKey: "abc123xyz"}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed AuthPayload
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if parsed.DeviceKey != "abc123xyz" {
		t.Errorf("Expected DeviceKey 'abc123xyz', got '%s'", parsed.DeviceKey)
	}
}

func TestDataPayload(t *testing.T) {
	// Test encoding
	originalData := []byte("Hello, World!")
	encoded := base64.StdEncoding.EncodeToString(originalData)

	payload := DataPayload{Data: encoded}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed DataPayload
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Test decoding
	decoded, err := base64.StdEncoding.DecodeString(parsed.Data)
	if err != nil {
		t.Fatalf("Base64 decode failed: %v", err)
	}

	if string(decoded) != "Hello, World!" {
		t.Errorf("Expected decoded 'Hello, World!', got '%s'", string(decoded))
	}
}

func TestStatusPayload(t *testing.T) {
	tests := []struct {
		status  string
		message string
	}{
		{"device_online", ""},
		{"device_offline", ""},
		{"connected", "Device connected successfully"},
		{"error", "Connection timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			payload := StatusPayload{
				Status:  tt.status,
				Message: tt.message,
			}

			data, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var parsed StatusPayload
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if parsed.Status != tt.status {
				t.Errorf("Expected status '%s', got '%s'", tt.status, parsed.Status)
			}

			if parsed.Message != tt.message {
				t.Errorf("Expected message '%s', got '%s'", tt.message, parsed.Message)
			}
		})
	}
}

func TestWSClientCreation(t *testing.T) {
	client := NewWSClient("localhost", 9000)

	if client == nil {
		t.Fatal("NewWSClient returned nil")
	}

	expectedURL := "ws://localhost:9000/api/v1/ws/client"
	if client.serverURL != expectedURL {
		t.Errorf("Expected serverURL '%s', got '%s'", expectedURL, client.serverURL)
	}

	if client.autoReconnect != true {
		t.Error("Expected autoReconnect to be true by default")
	}

	if client.DeviceOnlineStatus != "unknown" {
		t.Errorf("Expected initial status 'unknown', got '%s'", client.DeviceOnlineStatus)
	}
}

func TestWSClientSetDeviceKey(t *testing.T) {
	client := NewWSClient("localhost", 9000)
	client.SetDeviceKey("test-device-key")

	if client.deviceKey != "test-device-key" {
		t.Errorf("Expected deviceKey 'test-device-key', got '%s'", client.deviceKey)
	}
}

func TestWSClientSetAutoReconnect(t *testing.T) {
	client := NewWSClient("localhost", 9000)

	// Disable auto reconnect
	client.SetAutoReconnect(false)

	if client.autoReconnect != false {
		t.Error("Expected autoReconnect to be false")
	}

	// Enable again
	client.SetAutoReconnect(true)

	if client.autoReconnect != true {
		t.Error("Expected autoReconnect to be true")
	}
}

func TestWSClientIsConnected(t *testing.T) {
	client := NewWSClient("localhost", 9000)

	// Initially not connected
	if client.IsConnected() {
		t.Error("Expected IsConnected to return false initially")
	}
}

func TestWSClientCallbacks(t *testing.T) {
	client := NewWSClient("localhost", 9000)

	// Test OnData callback
	client.OnData(func(data []byte) {})
	if client.onData == nil {
		t.Error("OnData callback not set")
	}

	// Test OnConnected callback
	client.OnConnected(func() {})
	if client.onConnected == nil {
		t.Error("OnConnected callback not set")
	}

	// Test OnDisconnected callback
	client.OnDisconnected(func() {})
	if client.onDisconnected == nil {
		t.Error("OnDisconnected callback not set")
	}

	// Test OnError callback
	client.OnError(func(err error) {})
	if client.onError == nil {
		t.Error("OnError callback not set")
	}

	// Test OnStatus callback
	client.OnStatus(func(status string) {})
	if client.onStatus == nil {
		t.Error("OnStatus callback not set")
	}
}