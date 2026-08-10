package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const protocolVersion = "vsp.relay"

// TunnelConfig configures a local TCP gateway session.
type TunnelConfig struct {
	ServerURL     string
	UserToken     string
	DeviceID      uint
	MappingID     string
	ListenAddress string
}

// TunnelStatus represents the current gateway status.
type TunnelStatus struct {
	Connected      bool      `json:"connected"`
	LocalListening bool      `json:"local_listening"`
	RelayConnected bool      `json:"relay_connected"`
	ListenAddress  string    `json:"listen_address,omitempty"`
	DeviceID       uint      `json:"device_id,omitempty"`
	MappingID      string    `json:"mapping_id,omitempty"`
	MappingName    string    `json:"mapping_name,omitempty"`
	RemotePort     string    `json:"remote_port,omitempty"`
	SessionID      string    `json:"session_id,omitempty"`
	BytesSent      int64     `json:"bytes_sent"`
	BytesReceived  int64     `json:"bytes_received"`
	ConnectedSince time.Time `json:"connected_since,omitempty"`
	Error          string    `json:"error,omitempty"`
	LastEvent      string    `json:"last_event,omitempty"`
}

type controlMessage struct {
	Type      string   `json:"type"`
	Protocol  string   `json:"protocol,omitempty"`
	Role      string   `json:"role,omitempty"`
	DeviceID  uint     `json:"device_id,omitempty"`
	UserToken string   `json:"user_token,omitempty"`
	MappingID string   `json:"mapping_id,omitempty"`
	Mapping   *Mapping `json:"mapping,omitempty"`
	Status    string   `json:"status,omitempty"`
	Message   string   `json:"message,omitempty"`
	SessionID string   `json:"session_id,omitempty"`
}

// Mapping describes a device-side serial mapping announced through relay.
type Mapping struct {
	ID     string         `json:"id"`
	Name   string         `json:"name,omitempty"`
	Serial SerialSettings `json:"serial"`
}

// SerialSettings are metadata from the device; the desktop gateway does not apply them.
type SerialSettings struct {
	Port        string `json:"port"`
	BaudRate    int    `json:"baud_rate"`
	DataBits    int    `json:"data_bits"`
	StopBits    int    `json:"stop_bits"`
	Parity      string `json:"parity"`
	FlowControl string `json:"flow_control,omitempty"`
}

type lockedWS struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// TunnelService manages a local TCP endpoint bridged to the relay.
type TunnelService struct {
	mu        sync.RWMutex
	cfg       TunnelConfig
	wsURL     string
	running   bool
	listener  net.Listener
	wsConn    *websocket.Conn
	localConn net.Conn
	ctx       context.Context
	cancel    context.CancelFunc

	relayConnected bool
	sessionID      string
	mappingName    string
	remotePort     string
	errorMessage   string
	lastEvent      string
	bytesSent      int64
	bytesReceived  int64
	connectedTime  time.Time

	onStatusChange func(TunnelStatus)
	onDataTransfer func(direction string, bytes int)
}

// NewTunnelService creates a TCP gateway service.
func NewTunnelService() *TunnelService {
	return &TunnelService{}
}

// Connect starts a local TCP listener and bridges each accepted connection to the relay.
func (s *TunnelService) Connect(cfg TunnelConfig) error {
	if cfg.UserToken == "" {
		return fmt.Errorf("user token is required")
	}
	if cfg.DeviceID == 0 {
		return fmt.Errorf("device id is required")
	}
	if cfg.MappingID == "" {
		cfg.MappingID = "default"
	}
	if cfg.ListenAddress == "" {
		cfg.ListenAddress = "127.0.0.1:7000"
	}

	wsURL, err := buildRelayURL(cfg.ServerURL, "/api/relay/gateway")
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.ListenAddress, err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		_ = listener.Close()
		cancel()
		return fmt.Errorf("gateway already running")
	}
	s.cfg = cfg
	s.wsURL = wsURL
	s.listener = listener
	s.ctx = ctx
	s.cancel = cancel
	s.running = true
	s.relayConnected = false
	s.sessionID = ""
	s.mappingName = ""
	s.remotePort = ""
	s.errorMessage = ""
	s.lastEvent = fmt.Sprintf("Listening on %s", cfg.ListenAddress)
	s.bytesSent = 0
	s.bytesReceived = 0
	s.connectedTime = time.Time{}
	s.mu.Unlock()

	log.Printf("[TunnelService] local TCP endpoint listening on %s", cfg.ListenAddress)
	s.notifyStatusChange()
	go s.acceptLoop(ctx, listener)
	return nil
}

// Disconnect closes the local listener, active local client, and relay session.
func (s *TunnelService) Disconnect() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}

	cancel := s.cancel
	listener := s.listener
	wsConn := s.wsConn
	localConn := s.localConn

	s.running = false
	s.relayConnected = false
	s.sessionID = ""
	s.lastEvent = "Disconnected"
	s.listener = nil
	s.wsConn = nil
	s.localConn = nil
	s.cancel = nil
	s.ctx = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if listener != nil {
		_ = listener.Close()
	}
	if wsConn != nil {
		_ = wsConn.Close()
	}
	if localConn != nil {
		_ = localConn.Close()
	}

	log.Printf("[TunnelService] gateway disconnected")
	s.notifyStatusChange()
	return nil
}

func (s *TunnelService) acceptLoop(ctx context.Context, listener net.Listener) {
	for {
		localConn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			s.setError(fmt.Sprintf("accept local connection: %v", err))
			continue
		}

		s.setLocalConn(localConn)
		s.setLastEvent(fmt.Sprintf("Local client connected from %s", localConn.RemoteAddr()))
		if err := s.handleSession(ctx, localConn); err != nil && ctx.Err() == nil {
			s.setError(err.Error())
			log.Printf("[TunnelService] session ended: %v", err)
		}
		s.clearSession(localConn)
	}
}

func (s *TunnelService) handleSession(ctx context.Context, localConn net.Conn) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, s.wsURL, nil)
	if err != nil {
		return fmt.Errorf("connect relay: %w", err)
	}
	defer conn.Close()

	ws := &lockedWS{conn: conn}
	s.setWSConn(conn)

	if err := ws.writeJSON(controlMessage{
		Type:      "hello",
		Protocol:  protocolVersion,
		Role:      "gateway",
		DeviceID:  s.cfg.DeviceID,
		UserToken: s.cfg.UserToken,
		MappingID: s.cfg.MappingID,
	}); err != nil {
		return fmt.Errorf("send relay hello: %w", err)
	}

	var ready controlMessage
	if err := conn.ReadJSON(&ready); err != nil {
		return fmt.Errorf("read relay response: %w", err)
	}
	if ready.Type == "error" {
		return fmt.Errorf("relay rejected gateway: %s", ready.Message)
	}
	if ready.Type != "session_ready" {
		return fmt.Errorf("unexpected relay response: %+v", ready)
	}

	s.setRelayReady(ready)

	errc := make(chan error, 2)
	done := make(chan struct{})
	go s.localToRelay(ctx, localConn, ws, done, errc)
	go s.relayToLocal(ctx, localConn, ws, done, errc)

	select {
	case err := <-errc:
		close(done)
		return err
	case <-ctx.Done():
		close(done)
		return ctx.Err()
	}
}

func (s *TunnelService) localToRelay(ctx context.Context, localConn net.Conn, ws *lockedWS, done <-chan struct{}, errc chan<- error) {
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			errc <- ctx.Err()
			return
		case <-done:
			return
		default:
		}

		n, err := localConn.Read(buf)
		if n > 0 {
			payload := append([]byte(nil), buf[:n]...)
			if writeErr := ws.writeBinary(payload); writeErr != nil {
				errc <- fmt.Errorf("write relay binary: %w", writeErr)
				return
			}
			s.addBytes("send", n)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				errc <- fmt.Errorf("local client closed")
				return
			}
			errc <- fmt.Errorf("read local client: %w", err)
			return
		}
	}
}

func (s *TunnelService) relayToLocal(ctx context.Context, localConn net.Conn, ws *lockedWS, done <-chan struct{}, errc chan<- error) {
	for {
		select {
		case <-ctx.Done():
			errc <- ctx.Err()
			return
		case <-done:
			return
		default:
		}

		messageType, data, err := ws.conn.ReadMessage()
		if err != nil {
			errc <- fmt.Errorf("read relay: %w", err)
			return
		}

		switch messageType {
		case websocket.BinaryMessage:
			if len(data) == 0 {
				continue
			}
			if _, err := localConn.Write(data); err != nil {
				errc <- fmt.Errorf("write local client: %w", err)
				return
			}
			s.addBytes("receive", len(data))
		case websocket.TextMessage:
			if err := s.handleControl(ws, data); err != nil {
				errc <- err
				return
			}
		}
	}
}

func (s *TunnelService) handleControl(ws *lockedWS, data []byte) error {
	var msg controlMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil
	}

	switch msg.Type {
	case "ping":
		return ws.writeJSON(controlMessage{Type: "pong", Protocol: protocolVersion})
	case "session_ready":
		s.setRelayReady(msg)
	case "session_closed":
		return fmt.Errorf("session closed: %s", msg.Message)
	case "error":
		return fmt.Errorf("relay error: %s", msg.Message)
	}
	return nil
}

// Cleanup stops the gateway.
func (s *TunnelService) Cleanup() error {
	return s.Disconnect()
}

// GetStatus returns the current tunnel status.
func (s *TunnelService) GetStatus() TunnelStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return TunnelStatus{
		Connected:      s.relayConnected,
		LocalListening: s.running,
		RelayConnected: s.relayConnected,
		ListenAddress:  s.cfg.ListenAddress,
		DeviceID:       s.cfg.DeviceID,
		MappingID:      s.cfg.MappingID,
		MappingName:    s.mappingName,
		RemotePort:     s.remotePort,
		SessionID:      s.sessionID,
		BytesSent:      s.bytesSent,
		BytesReceived:  s.bytesReceived,
		ConnectedSince: s.connectedTime,
		Error:          s.errorMessage,
		LastEvent:      s.lastEvent,
	}
}

// IsConnected returns whether a relay session is active.
func (s *TunnelService) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.relayConnected
}

// IsListening returns whether the local TCP endpoint is open.
func (s *TunnelService) IsListening() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// OnStatusChange registers a callback for status changes.
func (s *TunnelService) OnStatusChange(callback func(TunnelStatus)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onStatusChange = callback
}

// OnDataTransfer registers a callback for data transfer events.
func (s *TunnelService) OnDataTransfer(callback func(direction string, bytes int)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onDataTransfer = callback
}

func (s *TunnelService) setLocalConn(conn net.Conn) {
	s.mu.Lock()
	s.localConn = conn
	s.errorMessage = ""
	s.mu.Unlock()
	s.notifyStatusChange()
}

func (s *TunnelService) setWSConn(conn *websocket.Conn) {
	s.mu.Lock()
	s.wsConn = conn
	s.mu.Unlock()
}

func (s *TunnelService) setRelayReady(msg controlMessage) {
	s.mu.Lock()
	s.relayConnected = true
	s.sessionID = msg.SessionID
	s.connectedTime = time.Now()
	s.errorMessage = ""
	s.lastEvent = fmt.Sprintf("Relay session ready: %s", msg.SessionID)
	if msg.Mapping != nil {
		s.mappingName = msg.Mapping.Name
		s.remotePort = msg.Mapping.Serial.Port
		if msg.Mapping.ID != "" {
			s.cfg.MappingID = msg.Mapping.ID
		}
	}
	s.mu.Unlock()
	s.notifyStatusChange()
}

func (s *TunnelService) clearSession(localConn net.Conn) {
	s.mu.Lock()
	if s.localConn == localConn {
		s.localConn = nil
	}
	if s.wsConn != nil {
		_ = s.wsConn.Close()
		s.wsConn = nil
	}
	s.relayConnected = false
	s.sessionID = ""
	if s.running {
		s.lastEvent = "Waiting for local TCP client"
	}
	s.mu.Unlock()

	_ = localConn.Close()
	s.notifyStatusChange()
}

func (s *TunnelService) setError(message string) {
	s.mu.Lock()
	s.errorMessage = message
	s.lastEvent = message
	s.relayConnected = false
	s.mu.Unlock()
	s.notifyStatusChange()
}

func (s *TunnelService) setLastEvent(message string) {
	s.mu.Lock()
	s.lastEvent = message
	s.mu.Unlock()
	s.notifyStatusChange()
}

func (s *TunnelService) addBytes(direction string, n int) {
	s.mu.Lock()
	switch direction {
	case "send":
		s.bytesSent += int64(n)
		s.lastEvent = fmt.Sprintf("TX %d bytes", n)
	case "receive":
		s.bytesReceived += int64(n)
		s.lastEvent = fmt.Sprintf("RX %d bytes", n)
	}
	callback := s.onDataTransfer
	s.mu.Unlock()

	if callback != nil {
		callback(direction, n)
	}
	s.notifyStatusChange()
}

func (s *TunnelService) notifyStatusChange() {
	s.mu.RLock()
	callback := s.onStatusChange
	s.mu.RUnlock()
	if callback != nil {
		callback(s.GetStatus())
	}
}

func (ws *lockedWS) writeJSON(value any) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.conn.WriteJSON(value)
}

func (ws *lockedWS) writeBinary(data []byte) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.conn.WriteMessage(websocket.BinaryMessage, data)
}

func buildRelayURL(serverURL, path string) (string, error) {
	serverURL = strings.TrimSpace(serverURL)
	if serverURL == "" {
		serverURL = "http://localhost:9000"
	}
	if !strings.Contains(serverURL, "://") {
		serverURL = "http://" + serverURL
	}

	u, err := url.Parse(serverURL)
	if err != nil {
		return "", fmt.Errorf("invalid server URL: %w", err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid server URL: missing host")
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported server URL scheme %q", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
