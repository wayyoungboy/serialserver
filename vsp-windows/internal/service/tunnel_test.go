package service

import (
	"testing"
	"time"
)

func TestTunnelStatusStruct(t *testing.T) {
	now := time.Now()
	status := TunnelStatus{
		Connected:      true,
		VisiblePort:    "COM5",
		HiddenPort:     "CNCB0",
		DeviceOnline:   true,
		DeviceStatus:   "device_online",
		BytesSent:      1024,
		BytesReceived:  512,
		ConnectedSince: now,
		Error:          "",
	}

	if !status.Connected {
		t.Error("Expected Connected to be true")
	}

	if status.VisiblePort != "COM5" {
		t.Errorf("Expected VisiblePort 'COM5', got '%s'", status.VisiblePort)
	}

	if status.BytesSent != 1024 {
		t.Errorf("Expected BytesSent 1024, got %d", status.BytesSent)
	}

	if status.BytesReceived != 512 {
		t.Errorf("Expected BytesReceived 512, got %d", status.BytesReceived)
	}

	if status.DeviceStatus != "device_online" {
		t.Errorf("Expected DeviceStatus 'device_online', got '%s'", status.DeviceStatus)
	}
}

func TestTunnelStatusWithError(t *testing.T) {
	status := TunnelStatus{
		Connected: false,
		Error:     "Connection timeout",
	}

	if status.Connected {
		t.Error("Expected Connected to be false")
	}

	if status.Error != "Connection timeout" {
		t.Errorf("Expected Error 'Connection timeout', got '%s'", status.Error)
	}
}

func TestNewTunnelService(t *testing.T) {
	service := NewTunnelService()

	if service == nil {
		t.Fatal("NewTunnelService returned nil")
	}

	// Check initial state
	if service.running {
		t.Error("Expected running to be false initially")
	}

	if service.wsClient != nil {
		t.Error("Expected wsClient to be nil initially")
	}

	if service.portClient == nil {
		t.Error("Expected portClient to be initialized")
	}

	if service.com0com == nil {
		t.Error("Expected com0com to be initialized")
	}
}

func TestTunnelServiceIsConnected(t *testing.T) {
	service := NewTunnelService()

	// Initially not connected
	if service.IsConnected() {
		t.Error("Expected IsConnected to return false initially")
	}
}

func TestTunnelServiceGetStatus(t *testing.T) {
	service := NewTunnelService()

	status := service.GetStatus()

	// Should return status with initial values
	if status.Connected {
		t.Error("Expected status.Connected to be false")
	}

	if status.VisiblePort != "" {
		t.Errorf("Expected empty VisiblePort, got '%s'", status.VisiblePort)
	}

	if status.BytesSent != 0 {
		t.Errorf("Expected BytesSent 0, got %d", status.BytesSent)
	}
}

func TestTunnelServiceGetVisiblePort(t *testing.T) {
	service := NewTunnelService()

	// Initially no port
	port := service.GetVisiblePort()
	if port != "" {
		t.Errorf("Expected empty visible port initially, got '%s'", port)
	}
}

func TestCallbackRegistration(t *testing.T) {
	service := NewTunnelService()

	// Test OnStatusChange callback
	service.OnStatusChange(func(status TunnelStatus) {})
	if service.onStatusChange == nil {
		t.Error("OnStatusChange callback not registered")
	}

	// Test OnDataTransfer callback
	service.OnDataTransfer(func(direction string, bytes int) {})
	if service.onDataTransfer == nil {
		t.Error("OnDataTransfer callback not registered")
	}
}

func TestNotifyStatusChange(t *testing.T) {
	service := NewTunnelService()

	// Register callback
	service.OnStatusChange(func(status TunnelStatus) {
		// Callback received
	})

	// Call notify - verifies mechanism exists
	service.notifyStatusChange()
}

func TestCheckCom0ComInstalled(t *testing.T) {
	service := NewTunnelService()

	// This will return false if com0com not installed
	// We just verify the function doesn't panic
	installed := service.CheckCom0ComInstalled()
	// No assertion on result since it depends on system state
	t.Logf("com0com installed: %v", installed)
}

func TestTunnelServiceStopForwarding(t *testing.T) {
	service := NewTunnelService()

	// Stop when not running should not error
	service.StopForwarding()

	if service.running {
		t.Error("Expected running to be false after StopForwarding")
	}
}

func TestTunnelStatusTimeFormat(t *testing.T) {
	now := time.Now()
	status := TunnelStatus{
		ConnectedSince: now,
	}

	// Verify time can be formatted
	formatted := status.ConnectedSince.Format("2006-01-02 15:04:05")
	if formatted == "" {
		t.Error("Expected non-empty formatted time")
	}

	t.Logf("Formatted time: %s", formatted)
}

func TestTunnelStatusZeroTime(t *testing.T) {
	status := TunnelStatus{
		ConnectedSince: time.Time{}, // Zero time
	}

	// Check if zero time
	if !status.ConnectedSince.IsZero() {
		t.Error("Expected ConnectedSince to be zero")
	}
}