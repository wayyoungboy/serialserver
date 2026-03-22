package models

import (
	"time"
)

// User 用户模型
type User struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Username     string    `json:"username" gorm:"uniqueIndex;size:50;not null"`
	Email        string    `json:"email" gorm:"uniqueIndex;size:100;not null"`
	PasswordHash string    `json:"-" gorm:"size:255;not null"`
	Role         string    `json:"role" gorm:"size:20;default:user"` // admin, user
	TenantID     uint      `json:"tenant_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	LastLogin    *time.Time `json:"last_login"`
	Status       string    `json:"status" gorm:"size:20;default:active"` // active, disabled, deleted
}

// Tenant 租户模型
type Tenant struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	Name          string    `json:"name" gorm:"size:100;not null"`
	Slug          string    `json:"slug" gorm:"uniqueIndex;size:50;not null"`
	Plan          string    `json:"plan" gorm:"size:20;default:free"` // free, pro, enterprise
	MaxDevices    int       `json:"max_devices" gorm:"default:5"`
	MaxConnections int      `json:"max_connections" gorm:"default:10"`
	CreatedAt     time.Time `json:"created_at"`
	Status        string    `json:"status" gorm:"size:20;default:active"`
}

// Device 设备模型
type Device struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	TenantID    uint      `json:"tenant_id" gorm:"index;not null"`
	UserID      uint      `json:"user_id" gorm:"index;not null"`
	Name        string    `json:"name" gorm:"size:100;not null"`
	DeviceKey   string    `json:"device_key" gorm:"uniqueIndex;size:64;not null"`
	SerialPort  string    `json:"serial_port" gorm:"size:20"`
	BaudRate    int       `json:"baud_rate" gorm:"default:115200"`
	DataBits    int       `json:"data_bits" gorm:"default:8"`
	StopBits    int       `json:"stop_bits" gorm:"default:1"`
	Parity      string    `json:"parity" gorm:"size:1;default:N"`
	Description string    `json:"description"`
	Location    string    `json:"location" gorm:"size:200"`
	CreatedAt   time.Time `json:"created_at"`
	LastOnline  *time.Time `json:"last_online"`
	Status      string    `json:"status" gorm:"size:20;default:offline"` // online, offline, disabled
}

// Session 连接会话模型
type Session struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	DeviceID      uint      `json:"device_id" gorm:"index;not null"`
	UserID        uint      `json:"user_id" gorm:"index"`
	ClientType    string    `json:"client_type" gorm:"size:20"` // device, windows
	ClientAddr    string    `json:"client_addr" gorm:"size:50"`
	StartedAt     time.Time `json:"started_at"`
	EndedAt       *time.Time `json:"ended_at"`
	BytesSent     int64     `json:"bytes_sent" gorm:"default:0"`
	BytesReceived int64     `json:"bytes_received" gorm:"default:0"`
	Status        string    `json:"status" gorm:"size:20;default:active"` // active, closed, error
}

// ConnectionLog 连接日志模型
type ConnectionLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	TenantID  uint      `json:"tenant_id" gorm:"index;not null"`
	DeviceID  uint      `json:"device_id" gorm:"index"`
	UserID    uint      `json:"user_id" gorm:"index"`
	Action    string    `json:"action" gorm:"size:50"` // connect, disconnect, data_transfer, error
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}

// APIKey API密钥模型
type APIKey struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	TenantID    uint      `json:"tenant_id" gorm:"index;not null"`
	UserID      uint      `json:"user_id" gorm:"index;not null"`
	Name        string    `json:"name" gorm:"size:100"`
	KeyHash     string    `json:"-" gorm:"uniqueIndex;size:64;not null"`
	Permissions string    `json:"permissions"` // JSON: ["read", "write", "admin"]
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsed    *time.Time `json:"last_used"`
	Status      string    `json:"status" gorm:"size:20;default:active"`
}

// ActiveConnection 活跃连接（内存中的状态）
type ActiveConnection struct {
	DeviceConn  *ClientConn
	WindowsConn *ClientConn
}

// ClientConn 客户端连接
type ClientConn struct {
	ID         string
	DeviceID   uint
	UserID     uint
	ClientType string // device, windows
	RemoteAddr string
	ConnectedAt time.Time
	BytesSent  int64
	BytesRecv  int64
}