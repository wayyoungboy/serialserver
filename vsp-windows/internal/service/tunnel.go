package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"vsp-manager/internal/driver"
	"vsp-manager/internal/network"
)

// TunnelStatus represents the current status of the tunnel
type TunnelStatus struct {
	Connected      bool      `json:"connected"`
	VisiblePort    string    `json:"visible_port,omitempty"`
	HiddenPort     string    `json:"hidden_port,omitempty"`
	DeviceOnline   bool      `json:"device_online"`
	DeviceStatus   string    `json:"device_status,omitempty"`
	BytesSent      int64     `json:"bytes_sent"`
	BytesReceived  int64     `json:"bytes_received"`
	ConnectedSince time.Time `json:"connected_since,omitempty"`
	Error          string    `json:"error,omitempty"`
}

// TunnelService manages the serial-to-WebSocket tunnel
type TunnelService struct {
	wsClient    *network.WSClient
	portClient  *driver.PortClient
	com0com     *driver.Com0ComManager

	mu          sync.RWMutex
	running     bool
	currentPair *driver.PortPair
	stopChan    chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc

	// Statistics
	bytesSent      int64
	bytesReceived  int64
	connectedTime  time.Time

	// Event callbacks
	onStatusChange func(TunnelStatus)
	onDataTransfer func(direction string, bytes int)
}

// NewTunnelService creates a new tunnel service
func NewTunnelService() *TunnelService {
	return &TunnelService{
		com0com:     driver.NewCom0ComManager(),
		portClient:  driver.NewPortClient(),
		stopChan:    make(chan struct{}),
	}
}

// Connect establishes the complete tunnel connection
// Steps:
// 1. Create virtual port pair with com0com
// 2. Open hidden port (CNCA/CNCB) with PortClient
// 3. Connect WebSocket to server with device key
// 4. Set up data callback from WebSocket -> PortClient
// 5. Start data forwarding goroutine
func (s *TunnelService) Connect(host string, port int, deviceKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("tunnel already running")
	}

	// Check if com0com is installed
	if !s.com0com.IsInstalled() {
		return fmt.Errorf("com0com driver not installed")
	}

	log.Printf("[TunnelService] Starting connection...")

	// Step 1: Create virtual port pair
	pair, err := s.com0com.CreatePortPair()
	if err != nil {
		return fmt.Errorf("failed to create port pair: %w", err)
	}
	s.currentPair = pair
	log.Printf("[TunnelService] Created port pair: visible=%s, hidden=%s", pair.VisiblePort, pair.HiddenPort)

	// Step 2: Open hidden port
	err = s.portClient.Open(pair.HiddenPort, 115200)
	if err != nil {
		// Clean up port pair on failure
		_ = s.com0com.RemovePortPair(pair.HiddenPort)
		s.currentPair = nil
		return fmt.Errorf("failed to open hidden port %s: %w", pair.HiddenPort, err)
	}
	log.Printf("[TunnelService] Opened hidden port: %s", pair.HiddenPort)

	// Step 3: Create WebSocket client and connect
	s.wsClient = network.NewWSClient(host, port)
	s.wsClient.SetDeviceKey(deviceKey)

	// Create context for this connection
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Step 4: Set up WebSocket data callback -> write to serial port
	s.wsClient.OnData(func(data []byte) {
		n, err := s.portClient.Write(data)
		if err != nil {
			log.Printf("[TunnelService] Write to serial failed: %v", err)
			return
		}
		s.mu.Lock()
		s.bytesReceived += int64(n)
		s.mu.Unlock()
		log.Printf("[TunnelService] WS->Serial: %d bytes", n)

		if s.onDataTransfer != nil {
			s.onDataTransfer("receive", n)
		}
	})

	// Set up status callback
	s.wsClient.OnStatus(func(status string) {
		s.notifyStatusChange()
	})

	// Set up error callback
	s.wsClient.OnError(func(err error) {
		log.Printf("[TunnelService] WebSocket error: %v", err)
		s.notifyStatusChange()
	})

	// Set up disconnected callback
	s.wsClient.OnDisconnected(func() {
		log.Printf("[TunnelService] WebSocket disconnected")
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		s.notifyStatusChange()
	})

	// Connect to server
	err = s.wsClient.Connect(s.ctx)
	if err != nil {
		// Clean up on failure
		_ = s.portClient.Close()
		_ = s.com0com.RemovePortPair(pair.HiddenPort)
		s.currentPair = nil
		return fmt.Errorf("WebSocket connection failed: %w", err)
	}
	log.Printf("[TunnelService] WebSocket connected")

	// Step 5: Start data forwarding (Serial -> WebSocket)
	s.running = true
	s.connectedTime = time.Now()
	s.stopChan = make(chan struct{})
	go s.forwardingLoop()

	log.Printf("[TunnelService] Tunnel established successfully")
	s.notifyStatusChange()

	return nil
}

// Disconnect closes all connections and removes the virtual port pair
func (s *TunnelService) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil // Already disconnected
	}

	log.Printf("[TunnelService] Disconnecting...")

	// Stop forwarding
	s.running = false
	if s.cancel != nil {
		s.cancel()
	}
	close(s.stopChan)

	// Close WebSocket
	if s.wsClient != nil {
		_ = s.wsClient.Close()
		s.wsClient = nil
	}

	// Close serial port
	if s.portClient != nil {
		_ = s.portClient.Close()
	}

	// Remove port pair
	if s.currentPair != nil {
		err := s.com0com.RemovePortPair(s.currentPair.HiddenPort)
		if err != nil {
			log.Printf("[TunnelService] Failed to remove port pair: %v", err)
		}
		s.currentPair = nil
	}

	// Reset statistics
	s.bytesSent = 0
	s.bytesReceived = 0

	log.Printf("[TunnelService] Disconnected successfully")
	s.notifyStatusChange()

	return nil
}

// forwardingLoop reads from serial port and sends to WebSocket
func (s *TunnelService) forwardingLoop() {
	log.Printf("[TunnelService] Forwarding loop started")

	buf := make([]byte, 4096) // Buffer for serial reads

	for {
		select {
		case <-s.stopChan:
			log.Printf("[TunnelService] Forwarding loop stopped by stopChan")
			return
		case <-s.ctx.Done():
			log.Printf("[TunnelService] Forwarding loop stopped by context")
			return
		default:
		}

		// Read from serial port
		n, err := s.portClient.Read(buf)
		if err != nil {
			s.mu.RLock()
			running := s.running
			s.mu.RUnlock()

			if !running {
				log.Printf("[TunnelService] Read error during shutdown: %v", err)
				return
			}

			log.Printf("[TunnelService] Serial read error: %v", err)
			// Brief pause before retrying
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if n == 0 {
			continue // No data
		}

		// Send to WebSocket
		data := buf[:n]
		err = s.wsClient.Send(data)
		if err != nil {
			log.Printf("[TunnelService] WebSocket send error: %v", err)
			continue
		}

		s.mu.Lock()
		s.bytesSent += int64(n)
		s.mu.Unlock()

		log.Printf("[TunnelService] Serial->WS: %d bytes", n)

		if s.onDataTransfer != nil {
			s.onDataTransfer("send", n)
		}
	}
}

// StartForwarding starts the data forwarding loop (called internally by Connect)
func (s *TunnelService) StartForwarding() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopChan = make(chan struct{})
	s.mu.Unlock()

	go s.forwardingLoop()
}

// StopForwarding stops the data forwarding loop
func (s *TunnelService) StopForwarding() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.stopChan)
}

// GetStatus returns the current tunnel status
func (s *TunnelService) GetStatus() TunnelStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := TunnelStatus{
		Connected:      s.running,
		BytesSent:      s.bytesSent,
		BytesReceived:  s.bytesReceived,
		ConnectedSince: s.connectedTime,
	}

	if s.currentPair != nil {
		status.VisiblePort = s.currentPair.VisiblePort
		status.HiddenPort = s.currentPair.HiddenPort
	}

	if s.wsClient != nil {
		status.DeviceStatus = s.wsClient.DeviceOnlineStatus
		status.DeviceOnline = s.wsClient.DeviceOnlineStatus == "device_online"
	}

	return status
}

// IsConnected returns whether the tunnel is active
func (s *TunnelService) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetVisiblePort returns the visible COM port name (e.g., COM5)
func (s *TunnelService) GetVisiblePort() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.currentPair == nil {
		return ""
	}
	return s.currentPair.VisiblePort
}

// notifyStatusChange fires the status change callback
func (s *TunnelService) notifyStatusChange() {
	if s.onStatusChange != nil {
		status := s.GetStatus()
		s.onStatusChange(status)
	}
}

// OnStatusChange registers a callback for status changes
func (s *TunnelService) OnStatusChange(callback func(TunnelStatus)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onStatusChange = callback
}

// OnDataTransfer registers a callback for data transfer events
func (s *TunnelService) OnDataTransfer(callback func(direction string, bytes int)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onDataTransfer = callback
}

// Cleanup removes all created port pairs (for application exit)
// This should be called when the application is shutting down
func (s *TunnelService) Cleanup() error {
	// First disconnect if running
	s.Disconnect()

	// Remove all created ports by this session
	return s.com0com.RemoveAllCreatedPorts()
}

// CheckCom0ComInstalled checks if com0com driver is installed
func (s *TunnelService) CheckCom0ComInstalled() bool {
	return s.com0com.IsInstalled()
}