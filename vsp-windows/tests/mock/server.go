package mock

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// AuthPayload represents auth message payload
type AuthPayload struct {
	DeviceKey string `json:"device_key"`
}

// DataPayload represents data message payload
type DataPayload struct {
	Data string `json:"data"`
}

// StatusPayload represents status message payload
type StatusPayload struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// MockVSPServer is a mock VSP server for testing
type MockVSPServer struct {
	server     *http.Server
	wsUpgrader websocket.Upgrader

	mu sync.RWMutex

	// Connections
	deviceConn *websocket.Conn
	clientConn *websocket.Conn

	// Test controls
	shouldAuthSuccess bool
	deviceOnline       bool
	receivedData      [][]byte
	deviceKey         string

	// Port
	port int
}

// NewMockServer creates a new mock server
func NewMockServer() *MockVSPServer {
	return &MockVSPServer{
		wsUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		shouldAuthSuccess: true,
		deviceOnline:      false,
		deviceKey:         "test-device-key",
	}
}

// SetDeviceKey sets the expected device key
func (s *MockVSPServer) SetDeviceKey(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deviceKey = key
}

// SetAuthSuccess sets whether auth should succeed
func (s *MockVSPServer) SetAuthSuccess(success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shouldAuthSuccess = success
}

// SetDeviceOnline sets the device online status
func (s *MockVSPServer) SetDeviceOnline(online bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deviceOnline = online
}

// GetReceivedData returns received data
func (s *MockVSPServer) GetReceivedData() [][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([][]byte, len(s.receivedData))
	copy(result, s.receivedData)
	return result
}

// ClearReceivedData clears received data
func (s *MockVSPServer) ClearReceivedData() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.receivedData = nil
}

// Start starts the mock server
func (s *MockVSPServer) Start(port int) error {
	s.mu.Lock()
	s.port = port
	s.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ws/device", s.handleDeviceWS)
	mux.HandleFunc("/api/v1/ws/client", s.handleClientWS)
	mux.HandleFunc("/api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("/api/v1/devices", s.handleDevices)
	mux.HandleFunc("/api/v1/devices/config", s.handleDeviceConfig)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		s.server.ListenAndServe()
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	return nil
}

// Stop stops the mock server
func (s *MockVSPServer) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

func (s *MockVSPServer) handleDeviceWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.mu.Lock()
	s.deviceConn = conn
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.deviceConn = nil
		s.mu.Unlock()
		conn.Close()
	}()

	// Handle device messages
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			continue
		}

		switch wsMsg.Type {
		case "auth":
			s.mu.RLock()
			shouldSuccess := s.shouldAuthSuccess
			s.mu.RUnlock()

			if shouldSuccess {
				s.sendAuthResponse(conn, "auth_success", "connected")
				// Notify client that device is online
				go s.notifyDeviceOnline()
			} else {
				s.sendAuthResponse(conn, "error", "invalid_key")
			}
		case "data":
			// Forward to client if connected
			s.mu.RLock()
			clientConn := s.clientConn
			s.mu.RUnlock()

			if clientConn != nil {
				clientConn.WriteMessage(websocket.TextMessage, msg)
			}
		}
	}
}

func (s *MockVSPServer) handleClientWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.mu.Lock()
	s.clientConn = conn
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.clientConn = nil
		s.mu.Unlock()
		conn.Close()
	}()

	// Handle client messages
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			continue
		}

		switch wsMsg.Type {
		case "auth":
			s.mu.RLock()
			shouldSuccess := s.shouldAuthSuccess
			deviceOnline := s.deviceOnline
			s.mu.RUnlock()

			if shouldSuccess {
				status := "connected"
				if deviceOnline {
					status = "device_online"
				} else {
					status = "device_offline"
				}
				s.sendAuthResponse(conn, "auth_success", status)
			} else {
				s.sendAuthResponse(conn, "error", "invalid_key")
			}
		case "data":
			// Store received data
			s.mu.Lock()
			s.receivedData = append(s.receivedData, msg)
			s.mu.Unlock()

			// Forward to device if connected
			s.mu.RLock()
			deviceConn := s.deviceConn
			s.mu.RUnlock()

			if deviceConn != nil {
				deviceConn.WriteMessage(websocket.TextMessage, msg)
			}
		}
	}
}

func (s *MockVSPServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request", 400)
		return
	}

	// Simple auth check
	if creds.Username == "admin" && creds.Password == "admin123" {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"token": "test-jwt-token-12345",
				"user": map[string]interface{}{
					"id":       1,
					"username": "admin",
					"email":    "admin@test.com",
					"role":     "admin",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	http.Error(w, `{"error": "Invalid credentials"}`, 401)
}

func (s *MockVSPServer) handleDevices(w http.ResponseWriter, r *http.Request) {
	// Check auth header
	auth := r.Header.Get("Authorization")
	if auth == "" || auth != "Bearer test-jwt-token-12345" {
		http.Error(w, `{"error": "Unauthorized"}`, 401)
		return
	}

	s.mu.RLock()
	deviceOnline := s.deviceOnline
	deviceKey := s.deviceKey
	s.mu.RUnlock()

	devices := []map[string]interface{}{
		{
			"id":           1,
			"name":         "Test Device",
			"device_key":   deviceKey,
			"serial_port":  "COM1",
			"baud_rate":    115200,
			"data_bits":    8,
			"stop_bits":    1,
			"parity":       "N",
			"status":       map[bool]string{true: "online", false: "offline"}[deviceOnline],
			"last_online":  time.Now().Format(time.RFC3339),
			"created_at":   time.Now().Format(time.RFC3339),
			"updated_at":   time.Now().Format(time.RFC3339),
		},
	}

	response := map[string]interface{}{
		"data": devices,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *MockVSPServer) handleDeviceConfig(w http.ResponseWriter, r *http.Request) {
	deviceKey := r.URL.Query().Get("device_key")

	s.mu.RLock()
	expectedKey := s.deviceKey
	s.mu.RUnlock()

	if deviceKey != expectedKey {
		http.Error(w, `{"error": "Device not found"}`, 404)
		return
	}

	config := map[string]interface{}{
		"device_key":  deviceKey,
		"serial_port": "COM1",
		"baud_rate":   115200,
		"data_bits":   8,
		"stop_bits":   1,
		"parity":      "N",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func (s *MockVSPServer) sendAuthResponse(conn *websocket.Conn, status, message string) {
	response := WSMessage{
		Type: status,
		Payload: mustMarshalJSON(StatusPayload{
			Status:  status,
			Message: message,
		}),
	}

	data, _ := json.Marshal(response)
	conn.WriteMessage(websocket.TextMessage, data)
}

// SendStatusToClient sends a status message to the connected client
func (s *MockVSPServer) SendStatusToClient(status string) {
	s.mu.RLock()
	conn := s.clientConn
	s.mu.RUnlock()

	if conn == nil {
		return
	}

	msg := WSMessage{
		Type: "status",
		Payload: mustMarshalJSON(StatusPayload{
			Status: status,
		}),
	}

	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)
}

// SendDataToClient sends data to the connected client
func (s *MockVSPServer) SendDataToClient(data []byte) {
	s.mu.RLock()
	conn := s.clientConn
	s.mu.RUnlock()

	if conn == nil {
		return
	}

	msg := WSMessage{
		Type: "data",
		Payload: mustMarshalJSON(DataPayload{
			Data: base64.StdEncoding.EncodeToString(data),
		}),
	}

	msgData, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, msgData)
}

func (s *MockVSPServer) notifyDeviceOnline() {
	s.mu.RLock()
	conn := s.clientConn
	s.mu.RUnlock()

	if conn == nil {
		return
	}

	msg := WSMessage{
		Type: "status",
		Payload: mustMarshalJSON(StatusPayload{
			Status: "device_online",
		}),
	}

	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)
}

// GetClientCount returns number of connected clients
func (s *MockVSPServer) GetClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	if s.clientConn != nil {
		count++
	}
	if s.deviceConn != nil {
		count++
	}
	return count
}

func mustMarshalJSON(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}