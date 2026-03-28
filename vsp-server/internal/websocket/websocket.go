package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"vsp-server/internal/database"
	"vsp-server/internal/models"
	"vsp-server/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，生产环境应限制
	},
}

// 心跳配置
const (
	PingInterval = 30 * time.Second // 发送 Ping 间隔
	PongTimeout  = 10 * time.Second // 等待 Pong 超时
	WriteWait    = 10 * time.Second // 写操作超时
)

// Hub 连接管理中心
type Hub struct {
	connections map[uint]*DeviceConnection // deviceID -> connection
	mu          sync.RWMutex
	logService  *services.LogService
}

// DeviceConnection 设备连接
type DeviceConnection struct {
	DeviceID    uint
	DeviceKey   string
	DeviceConn  *websocket.Conn // 设备端连接
	ClientConn  *websocket.Conn // Windows 客户端连接
	TenantID    uint
	UserID      uint
	ConnectedAt time.Time
	BytesSent   int64
	BytesRecv   int64
	mu          sync.Mutex // 保护连接的并发访问
}

// Message WebSocket 消息
type Message struct {
	Type    string          `json:"type"`    // data, status, error, auth
	Payload json.RawMessage `json:"payload"` // 消息内容
}

// AuthMessage 认证消息
type AuthMessage struct {
	DeviceKey string `json:"device_key"`
	UserToken string `json:"user_token"`
}

// DataMessage 数据消息
type DataMessage struct {
	Data []byte `json:"data"`
}

// StatusMessage 状态消息
type StatusMessage struct {
	Status  string `json:"status"`  // connected, disconnected
	Message string `json:"message"` // 描述信息
}

// NewHub 创建连接中心
func NewHub(logService *services.LogService) *Hub {
	return &Hub{
		connections: make(map[uint]*DeviceConnection),
		logService:  logService,
	}
}

// HandleDevice 设备端 WebSocket 处理
func (h *Hub) HandleDevice(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v", err)
		return
	}
	defer conn.Close()

	// 设置读取超时和 Pong 处理
	conn.SetReadDeadline(time.Now().Add(PingInterval + PongTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(PingInterval + PongTimeout))
		return nil
	})

	// 等待认证消息
	var msg Message
	if err := conn.ReadJSON(&msg); err != nil {
		conn.WriteJSON(Message{Type: "error", Payload: json.RawMessage(`"认证失败"`)})
		return
	}

	if msg.Type != "auth" {
		conn.WriteJSON(Message{Type: "error", Payload: json.RawMessage(`"需要先认证"`)})
		return
	}

	var auth AuthMessage
	if err := json.Unmarshal(msg.Payload, &auth); err != nil {
		conn.WriteJSON(Message{Type: "error", Payload: json.RawMessage(`"认证消息格式错误"`)})
		return
	}

	// 验证 DeviceKey
	deviceService := services.NewDeviceService()
	device, err := deviceService.GetDeviceByKey(auth.DeviceKey)
	if err != nil {
		conn.WriteJSON(Message{Type: "error", Payload: json.RawMessage(`"设备Key无效"`)})
		return
	}

	if device.Status == "disabled" {
		conn.WriteJSON(Message{Type: "error", Payload: json.RawMessage(`"设备已禁用"`)})
		return
	}

	// 更新设备状态为在线
	deviceService.UpdateDeviceStatus(auth.DeviceKey, "online")

	// 创建或更新连接
	h.mu.Lock()
	dc, exists := h.connections[device.ID]
	if !exists {
		dc = &DeviceConnection{
			DeviceID:    device.ID,
			DeviceKey:   auth.DeviceKey,
			TenantID:    device.TenantID,
			UserID:      device.UserID,
			ConnectedAt: time.Now(),
		}
		h.connections[device.ID] = dc
	}
	dc.mu.Lock()
	dc.DeviceConn = conn
	dc.mu.Unlock()
	h.mu.Unlock()

	// 发送认证成功
	conn.WriteJSON(Message{Type: "auth", Payload: json.RawMessage(`"认证成功"`)})

	// 记录日志
	h.logService.Log(device.TenantID, device.ID, 0, "device_connect", "设备端连接")

	log.Printf("设备端连接成功: DeviceID=%d, Key=%s", device.ID, auth.DeviceKey[:8]+"...")

	// 通知客户端设备上线
	h.notifyClient(device.ID, "device_online")

	// 启动心跳协程
	stopPing := make(chan struct{})
	go func() {
		ticker := time.NewTicker(PingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// 向设备端发送心跳
				conn.SetWriteDeadline(time.Now().Add(WriteWait))
				if err := conn.WriteJSON(Message{Type: "ping"}); err != nil {
					log.Printf("发送心跳到设备端失败: %v", err)
					return
				}
			case <-stopPing:
				return
			}
		}
	}()

	// 读取数据循环
	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		// 处理心跳响应
		if msg.Type == "pong" {
			conn.SetReadDeadline(time.Now().Add(PingInterval + PongTimeout))
			continue
		}

		if msg.Type == "data" {
			var data DataMessage
			if err := json.Unmarshal(msg.Payload, &data); err != nil {
				continue
			}

			dc.BytesRecv += int64(len(data.Data))

			// 转发给客户端
			h.mu.RLock()
			dc.mu.Lock()
			if dc.ClientConn != nil {
				dc.ClientConn.WriteJSON(msg)
				dc.BytesSent += int64(len(data.Data))
			}
			dc.mu.Unlock()
			h.mu.RUnlock()
		}
	}

	// 停止心跳
	close(stopPing)

	// 连接断开
	h.mu.Lock()
	dc.mu.Lock()
	if dc != nil {
		dc.DeviceConn = nil
		if dc.ClientConn == nil {
			delete(h.connections, device.ID)
		}
	}
	dc.mu.Unlock()
	h.mu.Unlock()

	deviceService.UpdateDeviceStatus(auth.DeviceKey, "offline")
	h.logService.Log(device.TenantID, device.ID, 0, "device_disconnect", "设备端断开")
	h.notifyClient(device.ID, "device_offline")
	log.Printf("设备端断开: DeviceID=%d", device.ID)
}

// HandleClient Windows 客户端 WebSocket 处理
func (h *Hub) HandleClient(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v", err)
		return
	}
	defer conn.Close()

	// 设置读取超时和 Pong 处理
	conn.SetReadDeadline(time.Now().Add(PingInterval + PongTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(PingInterval + PongTimeout))
		return nil
	})

	// 等待认证消息
	var msg Message
	if err := conn.ReadJSON(&msg); err != nil {
		conn.WriteJSON(Message{Type: "error", Payload: json.RawMessage(`"认证失败"`)})
		return
	}

	if msg.Type != "auth" {
		conn.WriteJSON(Message{Type: "error", Payload: json.RawMessage(`"需要先认证"`)})
		return
	}

	var auth AuthMessage
	if err := json.Unmarshal(msg.Payload, &auth); err != nil {
		conn.WriteJSON(Message{Type: "error", Payload: json.RawMessage(`"认证消息格式错误"`)})
		return
	}

	// 验证 DeviceKey
	deviceService := services.NewDeviceService()
	device, err := deviceService.GetDeviceByKey(auth.DeviceKey)
	if err != nil {
		conn.WriteJSON(Message{Type: "error", Payload: json.RawMessage(`"设备Key无效"`)})
		return
	}

	// 获取连接
	h.mu.Lock()
	dc, exists := h.connections[device.ID]
	if !exists {
		dc = &DeviceConnection{
			DeviceID:    device.ID,
			DeviceKey:   auth.DeviceKey,
			TenantID:    device.TenantID,
			UserID:      device.UserID,
			ConnectedAt: time.Now(),
		}
		h.connections[device.ID] = dc
	}
	dc.mu.Lock()
	dc.ClientConn = conn
	dc.mu.Unlock()
	h.mu.Unlock()

	// 发送认证成功（包含设备在线状态）
	status := "device_offline"
	dc.mu.Lock()
	if dc.DeviceConn != nil {
		status = "device_online"
	}
	dc.mu.Unlock()

	conn.WriteJSON(Message{
		Type: "auth",
		Payload: mustMarshal(StatusMessage{
			Status:  "connected",
			Message: status,
		}),
	})

	// 记录日志
	h.logService.Log(device.TenantID, device.ID, device.UserID, "client_connect", "客户端连接")

	log.Printf("客户端连接成功: DeviceID=%d", device.ID)

	// 启动心跳协程
	stopPing := make(chan struct{})
	go func() {
		ticker := time.NewTicker(PingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(WriteWait))
				if err := conn.WriteJSON(Message{Type: "ping"}); err != nil {
					log.Printf("客户端心跳发送失败: %v", err)
					return
				}
			case <-stopPing:
				return
			}
		}
	}()

	// 读取数据循环
	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		// 处理心跳响应
		if msg.Type == "pong" {
			conn.SetReadDeadline(time.Now().Add(PingInterval + PongTimeout))
			continue
		}

		if msg.Type == "data" {
			var data DataMessage
			if err := json.Unmarshal(msg.Payload, &data); err != nil {
				continue
			}

			dc.BytesSent += int64(len(data.Data))

			// 转发给设备端
			h.mu.RLock()
			dc.mu.Lock()
			if dc.DeviceConn != nil {
				dc.DeviceConn.WriteJSON(msg)
				dc.BytesRecv += int64(len(data.Data))
			}
			dc.mu.Unlock()
			h.mu.RUnlock()
		}
	}

	// 停止心跳
	close(stopPing)

	// 连接断开
	h.mu.Lock()
	dc.mu.Lock()
	if dc != nil {
		dc.ClientConn = nil
		if dc.DeviceConn == nil {
			delete(h.connections, device.ID)
		}
	}
	dc.mu.Unlock()
	h.mu.Unlock()

	h.logService.Log(device.TenantID, device.ID, device.UserID, "client_disconnect", "客户端断开")
	log.Printf("客户端断开: DeviceID=%d", device.ID)
}

// notifyClient 通知客户端状态变化
func (h *Hub) notifyClient(deviceID uint, status string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	dc, exists := h.connections[deviceID]
	if !exists || dc.ClientConn == nil {
		return
	}

	dc.ClientConn.WriteJSON(Message{
		Type: "status",
		Payload: mustMarshal(StatusMessage{
			Status:  status,
			Message: status,
		}),
	})
}

// GetConnectionStatus 获取连接状态
func (h *Hub) GetConnectionStatus(deviceID uint) *DeviceConnection {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.connections[deviceID]
}

// ListOnlineDevices 列出在线设备
func (h *Hub) ListOnlineDevices() []uint {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var devices []uint
	for id, dc := range h.connections {
		if dc.DeviceConn != nil || dc.ClientConn != nil {
			devices = append(devices, id)
		}
	}
	return devices
}

// mustMarshal 必须成功的 JSON 序列化
func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

// SaveSession 保存会话到数据库
func (h *Hub) SaveSession(deviceID, userID uint, clientType string, bytesSent, bytesRecv int64) {
	session := &models.Session{
		DeviceID:      deviceID,
		UserID:        userID,
		ClientType:    clientType,
		EndedAt:       func() *time.Time { t := time.Now(); return &t }(),
		BytesSent:     bytesSent,
		BytesReceived: bytesRecv,
		Status:        "closed",
	}
	database.DB.Create(session)
}