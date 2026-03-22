package tcp

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"vsp-client/internal/serial"
)

// Tunnel 隧道
type Tunnel struct {
	Name       string
	Mode       string // "client", "server", "tunnel"
	SerialPort *serial.Port
	TCPConn    net.Conn

	mu           sync.RWMutex
	closed       atomic.Bool
	onConnect    func(net.Conn)
	onDisconnect func()
	onData       func([]byte) // 回调用于UI显示
}

// Manager 隧道管理器
type Manager struct {
	tunnels map[string]*Tunnel
	mu      sync.Mutex
}

// NewManager 创建隧道管理器
func NewManager() *Manager {
	return &Manager{
		tunnels: make(map[string]*Tunnel),
	}
}

// StartClient 启动客户端模式 (Serial -> TCP)
func (m *Manager) StartClient(name string, serialPort *serial.Port, addr string) (*Tunnel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tunnels[name]; ok {
		return nil, fmt.Errorf("tunnel %s already exists", name)
	}

	tunnel := &Tunnel{
		Name:       name,
		Mode:       "client",
		SerialPort: serialPort,
	}

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	conn.Write([]byte("DEVICE\n"))
	
	buf := make([]byte, 64)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("等待服务器确认失败: %w", err)
	}
	response := string(buf[:n])
	if response[:2] != "OK" {
		conn.Close()
		return nil, fmt.Errorf("服务器拒绝连接: %s", response)
	}

	tunnel.TCPConn = conn
	m.tunnels[name] = tunnel

	go tunnel.run()

	log.Printf("Client tunnel %s started: serial -> tcp://%s", name, addr)
	return tunnel, nil
}

// StartServer 启动服务器模式 (Serial <- TCP)
func (m *Manager) StartServer(name string, serialPort *serial.Port, listenAddr string) (*Tunnel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tunnels[name]; ok {
		return nil, fmt.Errorf("tunnel %s already exists", name)
	}

	tunnel := &Tunnel{
		Name:       name,
		Mode:       "server",
		SerialPort: serialPort,
	}

	// 启动TCP监听
	go func() {
		ln, err := net.Listen("tcp", listenAddr)
		if err != nil {
			log.Printf("Failed to listen on %s: %v", listenAddr, err)
			return
		}
		defer ln.Close()

		log.Printf("Server tunnel %s listening on %s", name, listenAddr)

		for {
			conn, err := ln.Accept()
			if err != nil {
				if tunnel.closed.Load() {
					break
				}
				log.Printf("Accept error: %v", err)
				continue
			}

			tunnel.mu.Lock()
			if tunnel.TCPConn != nil {
				tunnel.TCPConn.Close()
			}
			tunnel.TCPConn = conn
			tunnel.mu.Unlock()

			if tunnel.onConnect != nil {
				tunnel.onConnect(conn)
			}

			log.Printf("Client connected to server tunnel %s: %s", name, conn.RemoteAddr())

			// 处理连接
			go tunnel.handleConnection()
		}
	}()

	m.tunnels[name] = tunnel
	return tunnel, nil
}

// StartTunnel 启动隧道模式 (Serial <-> TCP)
func (m *Manager) StartTunnel(name string, serialPort *serial.Port, serverAddr string) (*Tunnel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tunnels[name]; ok {
		return nil, fmt.Errorf("tunnel %s already exists", name)
	}

	tunnel := &Tunnel{
		Name:       name,
		Mode:       "tunnel",
		SerialPort: serialPort,
	}

	// 连接TCP服务器
	conn, err := net.DialTimeout("tcp", serverAddr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", serverAddr, err)
	}
	tunnel.TCPConn = conn

	m.tunnels[name] = tunnel

	// 启动数据转发
	go tunnel.run()

	log.Printf("Tunnel %s started: serial <-> tcp://%s", name, serverAddr)
	return tunnel, nil
}

// handleConnection 处理服务器模式的连接
func (t *Tunnel) handleConnection() {
	var wg sync.WaitGroup
	wg.Add(2)

	// TCP -> Serial
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			t.mu.RLock()
			conn := t.TCPConn
			t.mu.RUnlock()

			if conn == nil || t.closed.Load() {
				break
			}

			n, err := conn.Read(buf)
			if err != nil {
				break
			}

			if n > 0 {
				t.SerialPort.Write(buf[:n])
				if t.onData != nil {
					t.onData(buf[:n])
				}
			}
		}
	}()

	// Serial -> TCP
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			if t.closed.Load() {
				break
			}

			n, err := t.SerialPort.Read(buf)
			if err != nil {
				break
			}

			if n > 0 {
				t.mu.RLock()
				conn := t.TCPConn
				t.mu.RUnlock()

				if conn != nil {
					conn.Write(buf[:n])
					if t.onData != nil {
						t.onData(buf[:n])
					}
				}
			}
		}
	}()

	wg.Wait()

	if t.onDisconnect != nil {
		t.onDisconnect()
	}
}

// run 运行隧道
func (t *Tunnel) run() {
	var wg sync.WaitGroup
	wg.Add(2)

	// TCP -> Serial
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			if t.closed.Load() {
				break
			}

			t.mu.RLock()
			conn := t.TCPConn
			t.mu.RUnlock()

			if conn == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			n, err := conn.Read(buf)
			if err != nil {
				if !t.closed.Load() {
					log.Printf("Tunnel %s TCP read error: %v", t.Name, err)
				}
				break
			}

			if n > 0 {
				t.SerialPort.Write(buf[:n])
				if t.onData != nil {
					t.onData(buf[:n])
				}
			}
		}
	}()

	// Serial -> TCP
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			if t.closed.Load() {
				break
			}

			n, err := t.SerialPort.Read(buf)
			if err != nil {
				if !t.closed.Load() {
					log.Printf("Tunnel %s Serial read error: %v", t.Name, err)
				}
				break
			}

			if n > 0 {
				t.mu.RLock()
				conn := t.TCPConn
				t.mu.RUnlock()

				if conn != nil {
					conn.Write(buf[:n])
					if t.onData != nil {
						t.onData(buf[:n])
					}
				}
			}
		}
	}()

	wg.Wait()

	if t.onDisconnect != nil {
		t.onDisconnect()
	}
}

// Stop 停止隧道
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tunnels[name]
	if !ok {
		return nil
	}

	t.closed.Store(true)

	if t.TCPConn != nil {
		t.TCPConn.Close()
	}

	delete(m.tunnels, name)
	log.Printf("Tunnel %s stopped", name)
	return nil
}

// GetTunnel 获取隧道
func (m *Manager) GetTunnel(name string) (*Tunnel, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tunnels[name]
	return t, ok
}

// ListTunnels 列出所有隧道
func (m *Manager) ListTunnels() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	names := make([]string, 0, len(m.tunnels))
	for name := range m.tunnels {
		names = append(names, name)
	}
	return names
}

// SetConnectCallback 设置连接回调
func (t *Tunnel) SetConnectCallback(cb func(net.Conn)) {
	t.onConnect = cb
}

// SetDisconnectCallback 设置断开回调
func (t *Tunnel) SetDisconnectCallback(cb func()) {
	t.onDisconnect = cb
}

// SetDataCallback 设置数据回调
func (t *Tunnel) SetDataCallback(cb func([]byte)) {
	t.onData = cb
}

// CopyData 直接复制数据 (用于测试)
func CopyData(dst io.Writer, src io.Reader, onData func([]byte)) error {
	buf := make([]byte, 4096)
	for {
		n, err := src.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if n > 0 {
			dst.Write(buf[:n])
			if onData != nil {
				onData(buf[:n])
			}
		}
	}
}

// GetTCPAddress 获取TCP地址
func (t *Tunnel) GetTCPAddress() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.TCPConn != nil {
		return t.TCPConn.RemoteAddr().String()
	}
	return ""
}

// IsConnected 检查连接状态
func (t *Tunnel) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.TCPConn != nil && !t.closed.Load()
}
