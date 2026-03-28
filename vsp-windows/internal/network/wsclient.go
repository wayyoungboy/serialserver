package network

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Message types for WebSocket protocol
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type AuthPayload struct {
	DeviceKey string `json:"device_key"`
}

type DataPayload struct {
	Data string `json:"data"` // Base64 encoded
}

type StatusPayload struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// WSClient represents a WebSocket client for VSP server connection
type WSClient struct {
	serverURL      string
	deviceKey      string
	conn           *websocket.Conn
	mu             sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
	disconnecting  bool
	autoReconnect  bool
	reconnectDelay time.Duration

	// Event callbacks
	onData       func([]byte)
	onConnected  func()
	onDisconnected func()
	onError      func(error)
	onStatus     func(string)

	// State
	DeviceOnlineStatus string
}

const (
	pingInterval = 30 * time.Second
)

// NewWSClient creates a new WebSocket client
func NewWSClient(host string, port int) *WSClient {
	return &WSClient{
		serverURL:      fmt.Sprintf("ws://%s:%d/api/v1/ws/client", host, port),
		autoReconnect:  true,
		reconnectDelay: 1 * time.Second,
		DeviceOnlineStatus: "unknown",
	}
}

// SetDeviceKey sets the device key for authentication
func (c *WSClient) SetDeviceKey(key string) {
	c.deviceKey = key
}

// SetAutoReconnect enables or disables auto reconnect
func (c *WSClient) SetAutoReconnect(enable bool) {
	c.mu.Lock()
	c.autoReconnect = enable
	c.mu.Unlock()
}

// IsConnected returns whether the client is connected
func (c *WSClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

// Connect establishes WebSocket connection and authenticates
func (c *WSClient) Connect(ctx context.Context) error {
	return c.connectInternal(ctx)
}

func (c *WSClient) connectInternal(ctx context.Context) error {
	c.mu.Lock()
	c.disconnecting = false
	c.mu.Unlock()

	// Parse URL
	u, err := url.Parse(c.serverURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	log.Printf("[WSClient] Connecting to: %s", u.String())

	// Establish WebSocket connection
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.mu.Unlock()

	log.Printf("[WSClient] WebSocket connected")

	// Send authentication message
	authMsg := WSMessage{
		Type: "auth",
		Payload: mustMarshal(AuthPayload{DeviceKey: c.deviceKey}),
	}
	if err := c.sendJSON(authMsg); err != nil {
		c.closeConn()
		return fmt.Errorf("auth send failed: %w", err)
	}
	log.Printf("[WSClient] Sent auth message")

	// Wait for auth response
	_, message, err := conn.ReadMessage()
	if err != nil {
		c.closeConn()
		return fmt.Errorf("auth response read failed: %w", err)
	}

	var response WSMessage
	if err := json.Unmarshal(message, &response); err != nil {
		c.closeConn()
		return fmt.Errorf("auth response parse failed: %w", err)
	}

	log.Printf("[WSClient] Auth response: %s", string(message))

	if response.Type == "error" {
		c.closeConn()
		return fmt.Errorf("authentication failed: %s", string(response.Payload))
	}

	// Parse device status from auth response
	if response.Type == "auth" {
		var status StatusPayload
		if err := json.Unmarshal(response.Payload, &status); err == nil {
			if status.Message != "" {
				c.DeviceOnlineStatus = status.Message
				log.Printf("[WSClient] Device status: %s", c.DeviceOnlineStatus)
				if c.onStatus != nil {
					c.onStatus(c.DeviceOnlineStatus)
				}
			}
		}
	}

	log.Printf("[WSClient] Auth successful")

	// Fire connected event
	if c.onConnected != nil {
		c.onConnected()
	}

	// Start receive loop and heartbeat
	go c.receiveLoop()
	go c.heartbeatLoop()

	return nil
}

// Close closes the WebSocket connection
func (c *WSClient) Close() error {
	c.mu.Lock()
	if c.disconnecting {
		c.mu.Unlock()
		return nil
	}
	c.disconnecting = true
	c.autoReconnect = false
	c.mu.Unlock()

	log.Printf("[WSClient] Disconnecting...")

	c.closeConn()

	log.Printf("[WSClient] Disconnect complete")

	if c.onDisconnected != nil {
		c.onDisconnected()
	}

	return nil
}

func (c *WSClient) closeConn() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	if c.conn != nil {
		c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Closing"))
		c.conn.Close()
		c.conn = nil
	}
}

// Send sends data through WebSocket (base64 encoded in JSON)
func (c *WSClient) Send(data []byte) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	msg := WSMessage{
		Type: "data",
		Payload: mustMarshal(DataPayload{
			Data: base64.StdEncoding.EncodeToString(data),
		}),
	}

	return c.sendJSON(msg)
}

func (c *WSClient) sendJSON(msg WSMessage) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, data)
}

// OnData registers a callback for received data
func (c *WSClient) OnData(callback func([]byte)) {
	c.onData = callback
}

// OnConnected registers a callback for connection established
func (c *WSClient) OnConnected(callback func()) {
	c.onConnected = callback
}

// OnDisconnected registers a callback for disconnection
func (c *WSClient) OnDisconnected(callback func()) {
	c.onDisconnected = callback
}

// OnError registers a callback for errors
func (c *WSClient) OnError(callback func(error)) {
	c.onError = callback
}

// OnStatus registers a callback for status changes
func (c *WSClient) OnStatus(callback func(string)) {
	c.onStatus = callback
}

func (c *WSClient) receiveLoop() {
	log.Printf("[WSClient] ReceiveLoop started")

	for {
		c.mu.Lock()
		conn := c.conn
		ctx := c.ctx
		disconnecting := c.disconnecting
		c.mu.Unlock()

		if conn == nil || ctx == nil {
			break
		}

		select {
		case <-ctx.Done():
			log.Printf("[WSClient] ReceiveLoop cancelled")
			return
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[WSClient] Read error: %v", err)
			if c.onError != nil && !disconnecting {
				c.onError(err)
			}
			break
		}

		log.Printf("[WSClient] Received message: %s", string(message))

		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("[WSClient] JSON parse error: %v", err)
			continue
		}

		switch msg.Type {
		case "ping":
			// Reply with pong
			pong := WSMessage{Type: "pong"}
			c.sendJSON(pong)
			log.Printf("[WSClient] Replied pong to ping")

		case "status":
			var payload StatusPayload
			if err := json.Unmarshal(msg.Payload, &payload); err == nil {
				if payload.Status == "device_online" || payload.Status == "device_offline" {
					c.DeviceOnlineStatus = payload.Status
					log.Printf("[WSClient] Status update: %s", payload.Status)
					if c.onStatus != nil {
						c.onStatus(payload.Status)
					}
				}
			}

		case "data":
			var payload DataPayload
			if err := json.Unmarshal(msg.Payload, &payload); err == nil {
				data, err := base64.StdEncoding.DecodeString(payload.Data)
				if err != nil {
					log.Printf("[WSClient] Base64 decode error: %v", err)
					continue
				}
				log.Printf("[WSClient] Decoded data: %d bytes", len(data))
				if c.onData != nil {
					c.onData(data)
				}
			}
		}
	}

	log.Printf("[WSClient] ReceiveLoop ended")

	// Handle reconnect or disconnected event
	c.mu.Lock()
	disconnecting := c.disconnecting
	autoReconnect := c.autoReconnect
	c.mu.Unlock()

	if !disconnecting && autoReconnect {
		go c.reconnect()
	} else if !disconnecting {
		if c.onDisconnected != nil {
			c.onDisconnected()
		}
	}
}

func (c *WSClient) heartbeatLoop() {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		c.mu.Lock()
		ctx := c.ctx
		conn := c.conn
		c.mu.Unlock()

		if conn == nil || ctx == nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pong := WSMessage{Type: "pong"}
			if err := c.sendJSON(pong); err != nil {
				log.Printf("[WSClient] Heartbeat send error: %v", err)
				return
			}
			log.Printf("[WSClient] Sent pong heartbeat")
		}
	}
}

func (c *WSClient) reconnect() {
	for i := 1; i <= 5; i++ {
		time.Sleep(c.reconnectDelay * time.Duration(i))

		c.mu.Lock()
		autoReconnect := c.autoReconnect
		c.mu.Unlock()

		if !autoReconnect {
			return
		}

		log.Printf("[WSClient] Reconnect attempt %d", i)

		c.closeConn()

		if err := c.connectInternal(context.Background()); err != nil {
			log.Printf("[WSClient] Reconnect attempt %d failed: %v", i, err)
			continue
		}

		log.Printf("[WSClient] Reconnected successfully")
		return
	}

	log.Printf("[WSClient] Reconnect failed after 5 attempts")
	c.mu.Lock()
	c.disconnecting = true
	c.mu.Unlock()

	if c.onDisconnected != nil {
		c.onDisconnected()
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}