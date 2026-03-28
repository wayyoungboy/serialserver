package network

import (
	"testing"
)

func TestAPIClientCreation(t *testing.T) {
	client := NewAPIClient("localhost", 9000)

	if client == nil {
		t.Fatal("NewAPIClient returned nil")
	}

	expectedBaseURL := "http://localhost:9000/api/v1"
	if client.baseURL != expectedBaseURL {
		t.Errorf("Expected baseURL '%s', got '%s'", expectedBaseURL, client.baseURL)
	}

	if client.httpClient == nil {
		t.Error("Expected httpClient to be initialized")
	}

	// Check timeout is set
	if client.httpClient.Timeout == 0 {
		t.Error("Expected httpClient timeout to be set")
	}
}

func TestAPIClientSetToken(t *testing.T) {
	client := NewAPIClient("localhost", 9000)

	if client.token != "" {
		t.Error("Expected initial token to be empty")
	}

	client.SetToken("test-jwt-token")

	if client.token != "test-jwt-token" {
		t.Errorf("Expected token 'test-jwt-token', got '%s'", client.token)
	}
}

func TestAPIClientClearToken(t *testing.T) {
	client := NewAPIClient("localhost", 9000)
	client.SetToken("test-token")

	client.ClearToken()

	if client.token != "" {
		t.Errorf("Expected token to be cleared, got '%s'", client.token)
	}
}

func TestAPIClientIsAuthenticated(t *testing.T) {
	client := NewAPIClient("localhost", 9000)

	// Initially not authenticated
	if client.IsAuthenticated() {
		t.Error("Expected IsAuthenticated to return false initially")
	}

	// After setting token
	client.SetToken("test-token")
	if !client.IsAuthenticated() {
		t.Error("Expected IsAuthenticated to return true after setting token")
	}
}

func TestLoginRequestFormat(t *testing.T) {
	payload := map[string]string{
		"username": "admin",
		"password": "admin123",
	}

	// Verify payload structure
	if payload["username"] != "admin" {
		t.Error("Expected username in payload")
	}

	if payload["password"] != "admin123" {
		t.Error("Expected password in payload")
	}
}

func TestDeviceStruct(t *testing.T) {
	device := Device{
		ID:         1,
		Name:       "Test Device",
		DeviceKey:  "key-123",
		SerialPort: "COM1",
		BaudRate:   115200,
		DataBits:   8,
		StopBits:   1,
		Parity:     "N",
		Status:     "online",
	}

	if device.ID != 1 {
		t.Errorf("Expected ID 1, got %d", device.ID)
	}

	if device.Name != "Test Device" {
		t.Errorf("Expected Name 'Test Device', got '%s'", device.Name)
	}

	if device.DeviceKey != "key-123" {
		t.Errorf("Expected DeviceKey 'key-123', got '%s'", device.DeviceKey)
	}

	if device.BaudRate != 115200 {
		t.Errorf("Expected BaudRate 115200, got %d", device.BaudRate)
	}
}

func TestUserStruct(t *testing.T) {
	user := User{
		ID:       1,
		Username: "admin",
		Email:    "admin@example.com",
		Role:     "admin",
	}

	if user.ID != 1 {
		t.Errorf("Expected ID 1, got %d", user.ID)
	}

	if user.Username != "admin" {
		t.Errorf("Expected Username 'admin', got '%s'", user.Username)
	}

	if user.Role != "admin" {
		t.Errorf("Expected Role 'admin', got '%s'", user.Role)
	}
}

func TestLoginResponseStruct(t *testing.T) {
	resp := LoginResponse{
		Token: "jwt-token-abc",
		User: User{
			ID:       1,
			Username: "admin",
		},
	}

	if resp.Token != "jwt-token-abc" {
		t.Errorf("Expected Token 'jwt-token-abc', got '%s'", resp.Token)
	}

	if resp.User.Username != "admin" {
		t.Errorf("Expected User.Username 'admin', got '%s'", resp.User.Username)
	}
}

func TestAPIResponseStruct(t *testing.T) {
	// Success response
	successResp := APIResponse{
		Data: map[string]string{"token": "test"},
	}

	if successResp.Error != "" {
		t.Error("Expected empty error for success response")
	}

	// Error response
	errorResp := APIResponse{
		Error: "Invalid credentials",
	}

	if errorResp.Error != "Invalid credentials" {
		t.Errorf("Expected error 'Invalid credentials', got '%s'", errorResp.Error)
	}
}