package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// SerialConfig 串口配置
type SerialConfig struct {
	Port     string `json:"port" yaml:"port"`         // 串口名称
	Baud     int    `json:"baud" yaml:"baud"`         // 波特率
	DataBits int    `json:"dataBits" yaml:"dataBits"` // 数据位
	StopBits int    `json:"stopBits" yaml:"stopBits"` // 停止位
	Parity   string `json:"parity" yaml:"parity"`     // 校验位 N/O/E/M/S
}

// TCPConfig TCP配置
type TCPConfig struct {
	Host string `json:"host" yaml:"host"` // 监听地址/连接地址
	Port int    `json:"port" yaml:"port"` // 端口
}

// TunnelConfig 隧道配置
type TunnelConfig struct {
	Name    string       `json:"name" yaml:"name"`       // 隧道名称
	Mode    string       `json:"mode" yaml:"mode"`       // 模式: client/server/tunnel
	Serial  SerialConfig `json:"serial" yaml:"serial"`   // 串口配置
	TCP     TCPConfig    `json:"tcp" yaml:"tcp"`         // TCP配置
	Enabled bool         `json:"enabled" yaml:"enabled"` // 是否启用
}

// UIConfig UI配置
type UIConfig struct {
	Port           int    `json:"port" yaml:"port"`                     // Web端口
	Theme          string `json:"theme" yaml:"theme"`                   // 主题
	StartMinimized bool   `json:"startMinimized" yaml:"startMinimized"` // 启动时最小化
	MinimizeToTray bool   `json:"minimizeToTray" yaml:"minimizeToTray"` // 最小化到托盘
}

// AppConfig 应用配置
type AppConfig struct {
	Version string         `json:"version" yaml:"version"`
	Tunnels []TunnelConfig `json:"tunnels" yaml:"tunnels"`
	UI      UIConfig       `json:"ui" yaml:"ui"`
}

// Manager 配置管理器
type Manager struct {
	config *AppConfig
	mu     sync.RWMutex
	path   string
}

// New 创建配置管理器
func New(path string) *Manager {
	return &Manager{
		config: &AppConfig{
			Version: "1.0.0",
			Tunnels: []TunnelConfig{},
			UI: UIConfig{
				Port:           8080,
				Theme:          "dark",
				StartMinimized: false,
				MinimizeToTray: true,
			},
		},
		path: path,
	}
}

// Load 加载配置
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return m.save()
		}
		return err
	}

	// 尝试JSON格式
	if err := json.Unmarshal(data, m.config); err == nil {
		return nil
	}

	// 尝试YAML格式
	return yaml.Unmarshal(data, m.config)
}

// save 保存配置
func (m *Manager) save() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0644)
}

// Save 保存配置到文件
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.save()
}

// Get 获取配置
func (m *Manager) Get() *AppConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// Set 设置配置
func (m *Manager) Set(config *AppConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// AddTunnel 添加隧道
func (m *Manager) AddTunnel(tunnel TunnelConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Tunnels = append(m.config.Tunnels, tunnel)
	return m.save()
}

// RemoveTunnel 删除隧道
func (m *Manager) RemoveTunnel(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnels := m.config.Tunnels
	for i, t := range tunnels {
		if t.Name == name {
			m.config.Tunnels = append(tunnels[:i], tunnels[i+1:]...)
			return m.save()
		}
	}
	return fmt.Errorf("tunnel not found: %s", name)
}

// UpdateTunnel 更新隧道
func (m *Manager) UpdateTunnel(tunnel TunnelConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, t := range m.config.Tunnels {
		if t.Name == tunnel.Name {
			m.config.Tunnels[i] = tunnel
			return m.save()
		}
	}
	return fmt.Errorf("tunnel not found: %s", tunnel.Name)
}

// GetEnabledTunnels 获取启用的隧道
func (m *Manager) GetEnabledTunnels() []TunnelConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []TunnelConfig
	for _, t := range m.config.Tunnels {
		if t.Enabled {
			result = append(result, t)
		}
	}
	return result
}
