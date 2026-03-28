package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ServerHost != "localhost" {
		t.Errorf("Expected ServerHost 'localhost', got '%s'", cfg.ServerHost)
	}

	if cfg.ServerPort != 9000 {
		t.Errorf("Expected ServerPort 9000, got %d", cfg.ServerPort)
	}

	if cfg.AutoStart != false {
		t.Errorf("Expected AutoStart false, got %v", cfg.AutoStart)
	}

	if cfg.AutoConnect != false {
		t.Errorf("Expected AutoConnect false, got %v", cfg.AutoConnect)
	}
}

func TestNewManager(t *testing.T) {
	path := "/tmp/test-config.json"
	m := NewManager(path)

	if m.GetConfigPath() != path {
		t.Errorf("Expected configPath '%s', got '%s'", path, m.GetConfigPath())
	}

	if m.Get() == nil {
		t.Error("Expected config to be initialized")
	}
}

func TestNewManagerWithDefaultPath(t *testing.T) {
	m := NewManagerWithDefaultPath()

	// Config path should be absolute
	configPath := m.GetConfigPath()
	if !filepath.IsAbs(configPath) {
		t.Errorf("Expected absolute path, got '%s'", configPath)
	}

	// Path should contain "vsp-manager"
	if !contains(configPath, "vsp-manager") {
		t.Errorf("Expected path to contain 'vsp-manager', got '%s'", configPath)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		len(s) > len(substr) && containsHelper(s, substr)
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestConfigLoadAndSave(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	m := NewManager(configPath)

	// Test loading non-existent config (should create default)
	cfg, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.ServerHost != "localhost" {
		t.Errorf("Expected default ServerHost, got '%s'", cfg.ServerHost)
	}

	// Modify and save
	cfg.ServerHost = "192.168.1.100"
	cfg.ServerPort = 8080
	cfg.Username = "testuser"
	cfg.AutoConnect = true

	err = m.Save(cfg)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load again and verify
	cfg2, err := m.Load()
	if err != nil {
		t.Fatalf("Second load failed: %v", err)
	}

	if cfg2.ServerHost != "192.168.1.100" {
		t.Errorf("Expected ServerHost '192.168.1.100', got '%s'", cfg2.ServerHost)
	}

	if cfg2.ServerPort != 8080 {
		t.Errorf("Expected ServerPort 8080, got %d", cfg2.ServerPort)
	}

	if cfg2.Username != "testuser" {
		t.Errorf("Expected Username 'testuser', got '%s'", cfg2.Username)
	}

	if cfg2.AutoConnect != true {
		t.Errorf("Expected AutoConnect true, got %v", cfg2.AutoConnect)
	}
}

func TestConfigDirectoryCreation(t *testing.T) {
	// Create temp directory with nested path
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "nested", "deep", "config.json")

	m := NewManager(configPath)

	// Load should create nested directories
	_, err := m.Load()
	if err != nil {
		t.Fatalf("Load with nested path failed: %v", err)
	}

	// Verify directory was created
	dir := filepath.Dir(configPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("Directory '%s' was not created", dir)
	}
}

func TestGetAndSet(t *testing.T) {
	m := NewManagerWithDefaultPath()

	// Set should update config
	newCfg := &Config{
		ServerHost: "test.example.com",
		ServerPort: 5000,
	}
	m.Set(newCfg)

	cfg2 := m.Get()
	if cfg2.ServerHost != "test.example.com" {
		t.Errorf("Expected ServerHost 'test.example.com', got '%s'", cfg2.ServerHost)
	}
}

func TestGetConfigPath(t *testing.T) {
	path := "/custom/path/config.json"
	m := NewManager(path)

	if m.GetConfigPath() != path {
		t.Errorf("Expected path '%s', got '%s'", path, m.GetConfigPath())
	}
}