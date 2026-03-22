package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"vsp-server/internal/database"
	"vsp-server/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound      = errors.New("用户不存在")
	ErrInvalidPassword   = errors.New("密码错误")
	ErrUserAlreadyExists = errors.New("用户已存在")
	ErrInvalidToken      = errors.New("无效的Token")
)

// Claims JWT Claims
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	TenantID uint   `json:"tenant_id"`
	jwt.RegisteredClaims
}

// AuthService 认证服务
type AuthService struct {
	jwtSecret []byte
	expireHours int
}

// NewAuthService 创建认证服务
func NewAuthService(jwtSecret string, expireHours int) *AuthService {
	return &AuthService{
		jwtSecret:   []byte(jwtSecret),
		expireHours: expireHours,
	}
}

// Register 用户注册
func (s *AuthService) Register(username, email, password string, tenantID uint) (*models.User, error) {
	// 检查用户是否存在
	var count int64
	database.DB.Model(&models.User{}).Where("username = ? OR email = ?", username, email).Count(&count)
	if count > 0 {
		return nil, ErrUserAlreadyExists
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hashedPassword),
		Role:         "user",
		TenantID:     tenantID,
	}

	if err := database.DB.Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

// Login 用户登录
func (s *AuthService) Login(username, password string) (*models.User, string, error) {
	var user models.User
	if err := database.DB.Where("username = ? OR email = ?", username, username).First(&user).Error; err != nil {
		return nil, "", ErrUserNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidPassword
	}

	// 更新最后登录时间
	now := time.Now()
	database.DB.Model(&user).Update("last_login", now)

	// 生成Token
	token, err := s.GenerateToken(&user)
	if err != nil {
		return nil, "", err
	}

	return &user, token, nil
}

// GenerateToken 生成JWT Token
func (s *AuthService) GenerateToken(user *models.User) (string, error) {
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		TenantID: user.TenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(s.expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "vsp-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ValidateToken 验证Token
func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// DeviceService 设备服务
type DeviceService struct{}

// NewDeviceService 创建设备服务
func NewDeviceService() *DeviceService {
	return &DeviceService{}
}

// GenerateDeviceKey 生成设备Key
func (s *DeviceService) GenerateDeviceKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateDevice 创建设备
func (s *DeviceService) CreateDevice(userID, tenantID uint, name, serialPort string, baudRate int) (*models.Device, error) {
	deviceKey, err := s.GenerateDeviceKey()
	if err != nil {
		return nil, err
	}

	device := &models.Device{
		TenantID:   tenantID,
		UserID:     userID,
		Name:       name,
		DeviceKey:  deviceKey,
		SerialPort: serialPort,
		BaudRate:   baudRate,
		Status:     "offline",
	}

	if err := database.DB.Create(device).Error; err != nil {
		return nil, err
	}

	return device, nil
}

// GetDevice 获取设备
func (s *DeviceService) GetDevice(id uint) (*models.Device, error) {
	var device models.Device
	if err := database.DB.First(&device, id).Error; err != nil {
		return nil, err
	}
	return &device, nil
}

// GetDeviceByKey 通过Key获取设备
func (s *DeviceService) GetDeviceByKey(deviceKey string) (*models.Device, error) {
	var device models.Device
	if err := database.DB.Where("device_key = ?", deviceKey).First(&device).Error; err != nil {
		return nil, err
	}
	return &device, nil
}

// ListDevices 列出设备
func (s *DeviceService) ListDevices(tenantID uint, userID uint, role string) ([]models.Device, error) {
	var devices []models.Device
	query := database.DB.Model(&models.Device{})

	if role != "admin" {
		query = query.Where("user_id = ?", userID)
	} else {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Find(&devices).Error; err != nil {
		return nil, err
	}
	return devices, nil
}

// UpdateDevice 更新设备
func (s *DeviceService) UpdateDevice(id uint, updates map[string]interface{}) error {
	return database.DB.Model(&models.Device{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteDevice 删除设备
func (s *DeviceService) DeleteDevice(id uint) error {
	return database.DB.Delete(&models.Device{}, id).Error
}

// UpdateDeviceStatus 更新设备状态
func (s *DeviceService) UpdateDeviceStatus(deviceKey string, status string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if status == "online" {
		now := time.Now()
		updates["last_online"] = &now
	}
	return database.DB.Model(&models.Device{}).Where("device_key = ?", deviceKey).Updates(updates).Error
}

// LogService 日志服务
type LogService struct{}

// NewLogService 创建日志服务
func NewLogService() *LogService {
	return &LogService{}
}

// Log 记录日志
func (s *LogService) Log(tenantID, deviceID, userID uint, action, details string) error {
	log := &models.ConnectionLog{
		TenantID: tenantID,
		DeviceID: deviceID,
		UserID:   userID,
		Action:   action,
		Details:  details,
	}
	return database.DB.Create(log).Error
}

// GetLogs 获取日志
func (s *LogService) GetLogs(tenantID uint, limit int) ([]models.ConnectionLog, error) {
	var logs []models.ConnectionLog
	if err := database.DB.Where("tenant_id = ?", tenantID).Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// StatsService 统计服务
type StatsService struct{}

// NewStatsService 创建统计服务
func NewStatsService() *StatsService {
	return &StatsService{}
}

// GetStats 获取统计信息
func (s *StatsService) GetStats(tenantID uint) map[string]interface{} {
	var deviceCount, userCount, sessionCount int64
	database.DB.Model(&models.Device{}).Where("tenant_id = ?", tenantID).Count(&deviceCount)
	database.DB.Model(&models.User{}).Where("tenant_id = ?", tenantID).Count(&userCount)
	database.DB.Model(&models.Session{}).Where("device_id IN (SELECT id FROM devices WHERE tenant_id = ?)", tenantID).Count(&sessionCount)

	return map[string]interface{}{
		"devices":  deviceCount,
		"users":    userCount,
		"sessions": sessionCount,
	}
}

// FormatBytes 格式化字节数
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}