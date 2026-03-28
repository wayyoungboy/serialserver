package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tarm/serial"
)

var (
	serverAddr = flag.String("server", "localhost:9000", "服务器地址")
	deviceKey  = flag.String("key", "", "设备Key (必需)")
	port       = flag.String("port", "", "串口名称 (可选，从服务器获取)")
	baud       = flag.Int("baud", 0, "波特率 (可选，从服务器获取)")
	reconnect  = flag.Bool("reconnect", true, "断线重连")
)

type DeviceConfig struct {
	SerialPort string `json:"serial_port"`
	BaudRate   int    `json:"baud_rate"`
	DataBits   int    `json:"data_bits"`
	StopBits   int    `json:"stop_bits"`
	Parity     string `json:"parity"`
}

type DeviceClient struct {
	serverAddr string
	deviceKey  string
	config     *DeviceConfig
	serialPort *serial.Port
	wsConn     *websocket.Conn
	mu         sync.Mutex
	stopChan   chan struct{}
}

func NewDeviceClient(serverAddr, deviceKey string) *DeviceClient {
	return &DeviceClient{
		serverAddr: serverAddr,
		deviceKey:  deviceKey,
		stopChan:   make(chan struct{}),
	}
}

func (c *DeviceClient) FetchConfig() error {
	url := fmt.Sprintf("http://%s/api/v1/devices/config?device_key=%s", c.serverAddr, c.deviceKey)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("获取配置失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("服务器返回错误: %d", resp.StatusCode)
	}

	var result struct {
		Config DeviceConfig `json:"config"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	c.config = &result.Config
	log.Printf("从服务器获取配置: %s @ %d baud", c.config.SerialPort, c.config.BaudRate)
	return nil
}

func (c *DeviceClient) OpenSerialPort(portOverride string, baudOverride int) error {
	if c.config == nil {
		return fmt.Errorf("未加载配置")
	}

	// 使用命令行参数覆盖服务器配置
	portName := c.config.SerialPort
	baudRate := c.config.BaudRate

	if portOverride != "" {
		portName = portOverride
	}
	if baudOverride > 0 {
		baudRate = baudOverride
	}

	if portName == "" {
		return fmt.Errorf("未指定串口")
	}

	parity := serial.ParityNone
	switch strings.ToUpper(c.config.Parity) {
	case "O":
		parity = serial.ParityOdd
	case "E":
		parity = serial.ParityEven
	}

	stopBits := serial.Stop1
	switch c.config.StopBits {
	case 15, 2:
		stopBits = serial.Stop2
	}

	cfg := &serial.Config{
		Name:     portName,
		Baud:     baudRate,
		Parity:   parity,
		StopBits: stopBits,
		Size:     byte(c.config.DataBits),
	}

	sp, err := serial.OpenPort(cfg)
	if err != nil {
		return fmt.Errorf("打开串口失败: %w", err)
	}

	c.serialPort = sp
	log.Printf("串口已打开: %s @ %d", portName, baudRate)
	return nil
}

func (c *DeviceClient) ConnectWebSocket() error {
	u := url.URL{
		Scheme:   "ws",
		Host:     c.serverAddr,
		Path:     "/api/v1/ws/device",
	}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("WebSocket连接失败: %w", err)
	}

	// 发送认证消息
	authMsg := map[string]interface{}{
		"type": "auth",
		"payload": map[string]string{
			"device_key": c.deviceKey,
		},
	}
	if err := conn.WriteJSON(authMsg); err != nil {
		conn.Close()
		return fmt.Errorf("发送认证消息失败: %w", err)
	}

	// 读取认证响应
	var resp struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := conn.ReadJSON(&resp); err != nil {
		conn.Close()
		return fmt.Errorf("读取认证响应失败: %w", err)
	}

	if resp.Type == "error" {
		conn.Close()
		return fmt.Errorf("认证失败: %s", string(resp.Payload))
	}

	c.wsConn = conn
	log.Printf("已连接到服务器: %s", u.String())
	return nil
}

func (c *DeviceClient) Start() error {
	// 串口 -> WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			select {
			case <-c.stopChan:
				return
			default:
				if c.serialPort == nil {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				n, err := c.serialPort.Read(buf)
				if err != nil {
					log.Printf("串口读取错误: %v", err)
					continue
				}
				if n > 0 && c.wsConn != nil {
					// 发送 JSON 格式的数据消息
					msg := map[string]interface{}{
						"type": "data",
						"payload": map[string]interface{}{
							"data": buf[:n],
						},
					}
					c.mu.Lock()
					c.wsConn.WriteJSON(msg)
					c.mu.Unlock()
					log.Printf("[串口→服务器] %d 字节", n)
				}
			}
		}
	}()

	// WebSocket -> 串口 (含心跳处理)
	go func() {
		for {
			select {
			case <-c.stopChan:
				return
			default:
				if c.wsConn == nil {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				var msg struct {
					Type    string          `json:"type"`
					Payload json.RawMessage `json:"payload"`
				}
				if err := c.wsConn.ReadJSON(&msg); err != nil {
					log.Printf("WebSocket读取错误: %v", err)
					if *reconnect {
						c.reconnect()
					}
					continue
				}

				// 处理心跳
				if msg.Type == "ping" {
					c.mu.Lock()
					c.wsConn.WriteJSON(map[string]string{"type": "pong"})
					c.mu.Unlock()
					continue
				}

				if msg.Type == "data" {
					var dataMsg struct {
						Data []byte `json:"data"`
					}
					if err := json.Unmarshal(msg.Payload, &dataMsg); err == nil && len(dataMsg.Data) > 0 && c.serialPort != nil {
						c.serialPort.Write(dataMsg.Data)
						log.Printf("[服务器→串口] %d 字节", len(dataMsg.Data))
					}
				}
			}
		}
	}()

	return nil
}

func (c *DeviceClient) reconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	log.Println("正在重连...")

	// 关闭旧连接
	if c.wsConn != nil {
		c.wsConn.Close()
		c.wsConn = nil
	}

	// 重试连接
	for i := 1; i <= 10; i++ {
		time.Sleep(time.Duration(i) * time.Second)
		if err := c.ConnectWebSocket(); err == nil {
			log.Println("重连成功")
			return
		}
		log.Printf("重连失败，第 %d 次", i)
	}

	log.Println("重连失败，停止尝试")
}

func (c *DeviceClient) Stop() {
	close(c.stopChan)
	if c.serialPort != nil {
		c.serialPort.Close()
	}
	if c.wsConn != nil {
		c.wsConn.Close()
	}
}

func main() {
	flag.Parse()

	if *deviceKey == "" {
		fmt.Println("错误: 必须指定设备Key (-key)")
		flag.Usage()
		os.Exit(1)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("=== VSP Device Client ===")
	log.Printf("服务器: %s", *serverAddr)
	log.Printf("设备Key: %s...", (*deviceKey)[:8])

	client := NewDeviceClient(*serverAddr, *deviceKey)

	// 1. 从服务器获取配置
	if err := client.FetchConfig(); err != nil {
		log.Fatalf("获取配置失败: %v", err)
	}

	// 2. 打开串口
	if err := client.OpenSerialPort(*port, *baud); err != nil {
		log.Fatalf("打开串口失败: %v", err)
	}

	// 3. 连接WebSocket
	if err := client.ConnectWebSocket(); err != nil {
		log.Fatalf("连接服务器失败: %v", err)
	}

	// 4. 启动数据转发
	if err := client.Start(); err != nil {
		log.Fatalf("启动失败: %v", err)
	}

	log.Println("设备客户端运行中，按 Ctrl+C 退出")

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("正在关闭...")
	client.Stop()
	log.Println("已退出")
}