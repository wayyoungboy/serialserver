package serial

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/tarm/serial"
	"vsp-client/internal/config"
)

// Port 串口封装
type Port struct {
	port    *serial.Port
	config  *serial.Config
	name    string
	mu      sync.RWMutex
	closed  bool
	onData  func([]byte)
	onError func(error)
}

// Stats 统计信息
type Stats struct {
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
	LastRx    time.Time
	LastTx    time.Time
}

// PortManager 串口管理器
type PortManager struct {
	ports map[string]*Port
	mu    sync.Mutex
	stats map[string]*Stats
}

// NewPortManager 创建串口管理器
func NewPortManager() *PortManager {
	return &PortManager{
		ports: make(map[string]*Port),
		stats: make(map[string]*Stats),
	}
}

// Open 打开串口
func (pm *PortManager) Open(cfg config.SerialConfig) (*Port, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	name := cfg.Port
	if p, ok := pm.ports[name]; ok && p != nil {
		return p, nil
	}

	serialCfg := &serial.Config{
		Name:        cfg.Port,
		Baud:        cfg.Baud,
		ReadTimeout: time.Second * 5,
	}

	port, err := serial.OpenPort(serialCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open serial port %s: %w", cfg.Port, err)
	}

	p := &Port{
		port:   port,
		config: serialCfg,
		name:   name,
	}

	pm.ports[name] = p
	pm.stats[name] = &Stats{}

	log.Printf("Serial port %s opened at %d baud", name, cfg.Baud)
	return p, nil
}

// Close 关闭串口
func (pm *PortManager) Close(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	p, ok := pm.ports[name]
	if !ok || p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	err := p.port.Close()
	if err != nil {
		return fmt.Errorf("failed to close serial port %s: %w", name, err)
	}

	delete(pm.ports, name)
	log.Printf("Serial port %s closed", name)
	return nil
}

// Read 读取数据
func (p *Port) Read(buf []byte) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return 0, fmt.Errorf("port closed")
	}

	return p.port.Read(buf)
}

// Write 写入数据
func (p *Port) Write(data []byte) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return 0, fmt.Errorf("port closed")
	}

	return p.port.Write(data)
}

// SetReadCallback 设置读取回调
func (p *Port) SetReadCallback(onData func([]byte), onError func(error)) {
	p.onData = onData
	p.onError = onError
}

// GetStats 获取统计信息
func (pm *PortManager) GetStats(name string) (*Stats, bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	stats, ok := pm.stats[name]
	return stats, ok
}

// UpdateStats 更新统计
func (pm *PortManager) UpdateStats(name string, rx, tx int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if stats, ok := pm.stats[name]; ok {
		if rx > 0 {
			stats.RxBytes += uint64(rx)
			stats.RxPackets++
			stats.LastRx = time.Now()
		}
		if tx > 0 {
			stats.TxBytes += uint64(tx)
			stats.TxPackets++
			stats.LastTx = time.Now()
		}
	}
}

// ListAvailable 列出可用串口
func ListAvailable() ([]string, error) {
	var ports []string

	switch runtime.GOOS {
	case "windows":
		ports = []string{}
		for i := 1; i <= 256; i++ {
			ports = append(ports, fmt.Sprintf("COM%d", i))
		}
	case "darwin":
		ports = []string{}
		for i := 0; i <= 256; i++ {
			ports = append(ports, fmt.Sprintf("/dev/tty.usbserial-%d", i))
			ports = append(ports, fmt.Sprintf("/dev/cu.usbserial-%d", i))
			ports = append(ports, fmt.Sprintf("/dev/ttyUSB%d", i))
			ports = append(ports, fmt.Sprintf("/dev/cuUSB%d", i))
		}
	default:
		ports = []string{}
		for i := 0; i <= 256; i++ {
			ports = append(ports, fmt.Sprintf("/dev/ttyUSB%d", i))
			ports = append(ports, fmt.Sprintf("/dev/ttyACM%d", i))
			ports = append(ports, fmt.Sprintf("/dev/ttyS%d", i))
		}
	}

	var available []string
	for _, port := range ports {
		cfg := &serial.Config{Name: port, Baud: 9600, ReadTimeout: 100 * time.Millisecond}
		if p, err := serial.OpenPort(cfg); err == nil {
			p.Close()
			available = append(available, port)
		}
	}

	log.Printf("Found %d available serial ports", len(available))
	return available, nil
}

// GetPortName 获取端口名称
func (p *Port) GetPortName() string {
	return p.name
}
