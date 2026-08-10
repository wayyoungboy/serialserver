package service

import (
	"testing"
	"time"
)

func TestTunnelStatusStruct(t *testing.T) {
	now := time.Now()
	status := TunnelStatus{
		Connected:      true,
		LocalListening: true,
		RelayConnected: true,
		ListenAddress:  "127.0.0.1:7000",
		DeviceID:       12,
		MappingID:      "plc",
		MappingName:    "PLC",
		RemotePort:     "COM3",
		SessionID:      "session-1",
		BytesSent:      1024,
		BytesReceived:  512,
		ConnectedSince: now,
	}

	if !status.Connected {
		t.Error("Expected Connected to be true")
	}
	if !status.LocalListening {
		t.Error("Expected LocalListening to be true")
	}
	if status.ListenAddress != "127.0.0.1:7000" {
		t.Errorf("Expected ListenAddress '127.0.0.1:7000', got '%s'", status.ListenAddress)
	}
	if status.MappingID != "plc" {
		t.Errorf("Expected MappingID 'plc', got '%s'", status.MappingID)
	}
	if status.BytesSent != 1024 {
		t.Errorf("Expected BytesSent 1024, got %d", status.BytesSent)
	}
	if status.BytesReceived != 512 {
		t.Errorf("Expected BytesReceived 512, got %d", status.BytesReceived)
	}
}

func TestTunnelStatusWithError(t *testing.T) {
	status := TunnelStatus{
		Connected: false,
		Error:     "mapping offline",
	}

	if status.Connected {
		t.Error("Expected Connected to be false")
	}
	if status.Error != "mapping offline" {
		t.Errorf("Expected Error 'mapping offline', got '%s'", status.Error)
	}
}

func TestNewTunnelService(t *testing.T) {
	service := NewTunnelService()
	if service == nil {
		t.Fatal("NewTunnelService returned nil")
	}
	if service.running {
		t.Error("Expected running to be false initially")
	}
	if service.IsConnected() {
		t.Error("Expected IsConnected to be false initially")
	}
	if service.IsListening() {
		t.Error("Expected IsListening to be false initially")
	}
}

func TestTunnelServiceRequiresUserAuth(t *testing.T) {
	service := NewTunnelService()
	err := service.Connect(TunnelConfig{
		ServerURL:     "http://localhost:9000",
		DeviceID:      1,
		MappingID:     "plc",
		ListenAddress: "127.0.0.1:0",
	})
	if err == nil {
		t.Fatal("Expected missing user token to fail")
	}
}

func TestTunnelServiceListenAndDisconnect(t *testing.T) {
	service := NewTunnelService()
	err := service.Connect(TunnelConfig{
		ServerURL:     "http://localhost:9000",
		UserToken:     "token",
		DeviceID:      1,
		MappingID:     "plc",
		ListenAddress: "127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	status := service.GetStatus()
	if !status.LocalListening {
		t.Error("Expected LocalListening to be true")
	}
	if status.MappingID != "plc" {
		t.Errorf("Expected MappingID 'plc', got '%s'", status.MappingID)
	}

	if err := service.Disconnect(); err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}
	if service.IsListening() {
		t.Error("Expected IsListening to be false after disconnect")
	}
}

func TestCallbackRegistration(t *testing.T) {
	service := NewTunnelService()
	service.OnStatusChange(func(status TunnelStatus) {})
	if service.onStatusChange == nil {
		t.Error("OnStatusChange callback not registered")
	}

	service.OnDataTransfer(func(direction string, bytes int) {})
	if service.onDataTransfer == nil {
		t.Error("OnDataTransfer callback not registered")
	}
}

func TestBuildRelayURL(t *testing.T) {
	tests := []struct {
		name   string
		server string
		want   string
	}{
		{name: "http", server: "http://localhost:9000", want: "ws://localhost:9000/api/relay/gateway"},
		{name: "https", server: "https://relay.example.com", want: "wss://relay.example.com/api/relay/gateway"},
		{name: "host only", server: "localhost:9000", want: "ws://localhost:9000/api/relay/gateway"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildRelayURL(tt.server, "/api/relay/gateway")
			if err != nil {
				t.Fatalf("buildRelayURL failed: %v", err)
			}
			if got != tt.want {
				t.Errorf("Expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestTunnelStatusTimeFormat(t *testing.T) {
	now := time.Now()
	status := TunnelStatus{ConnectedSince: now}
	if status.ConnectedSince.Format("2006-01-02 15:04:05") == "" {
		t.Error("Expected non-empty formatted time")
	}
}

func TestTunnelStatusZeroTime(t *testing.T) {
	status := TunnelStatus{ConnectedSince: time.Time{}}
	if !status.ConnectedSince.IsZero() {
		t.Error("Expected ConnectedSince to be zero")
	}
}
