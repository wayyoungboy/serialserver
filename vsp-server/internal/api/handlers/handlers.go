package handlers

import (
	"net/http"
	"strconv"

	"vsp-server/internal/database"
	"vsp-server/internal/models"
	"vsp-server/internal/services"

	"github.com/gin-gonic/gin"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authService *services.AuthService
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

// Register 用户注册
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取默认租户
	var tenant models.Tenant
	if err := database.DB.First(&tenant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "租户不存在"})
		return
	}

	user, err := h.authService.Register(req.Username, req.Email, req.Password, tenant.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": user})
}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, token, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": LoginResponse{
		Token: token,
		User:  *user,
	}})
}

// GetProfile 获取当前用户信息
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID := c.GetUint("user_id")
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": user})
}

// DeviceHandler 设备处理器
type DeviceHandler struct {
	deviceService *services.DeviceService
}

// NewDeviceHandler 创建设备处理器
func NewDeviceHandler(deviceService *services.DeviceService) *DeviceHandler {
	return &DeviceHandler{deviceService: deviceService}
}

// CreateDeviceRequest 创建设备请求
type CreateDeviceRequest struct {
	Name       string `json:"name" binding:"required"`
	SerialPort string `json:"serial_port"`
	BaudRate   int    `json:"baud_rate"`
}

// ListDevices 列出设备
func (h *DeviceHandler) ListDevices(c *gin.Context) {
	userID := c.GetUint("user_id")
	tenantID := c.GetUint("tenant_id")
	role := c.GetString("role")

	devices, err := h.deviceService.ListDevices(tenantID, userID, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": devices})
}

// CreateDevice 创建设备
func (h *DeviceHandler) CreateDevice(c *gin.Context) {
	userID := c.GetUint("user_id")
	tenantID := c.GetUint("tenant_id")

	var req CreateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	device, err := h.deviceService.CreateDevice(userID, tenantID, req.Name, req.SerialPort, req.BaudRate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": device})
}

// GetDevice 获取设备详情
func (h *DeviceHandler) GetDevice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	device, err := h.deviceService.GetDevice(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": device})
}

// UpdateDevice 更新设备
func (h *DeviceHandler) UpdateDevice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.deviceService.UpdateDevice(uint(id), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteDevice 删除设备
func (h *DeviceHandler) DeleteDevice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	if err := h.deviceService.DeleteDevice(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetDeviceConfig 获取设备配置 (设备端通过 DeviceKey 获取)
func (h *DeviceHandler) GetDeviceConfig(c *gin.Context) {
	deviceKey := c.Query("device_key")
	if deviceKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 device_key"})
		return
	}

	device, err := h.deviceService.GetDeviceByKey(deviceKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config": gin.H{
			"serial_port": device.SerialPort,
			"baud_rate":   device.BaudRate,
			"data_bits":   device.DataBits,
			"stop_bits":   device.StopBits,
			"parity":      device.Parity,
		},
	})
}

// UpdateDeviceConfigByKey 通过 DeviceKey 更新设备配置
func (h *DeviceHandler) UpdateDeviceConfigByKey(c *gin.Context) {
	deviceKey := c.Param("key")
	if deviceKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 device_key"})
		return
	}

	device, err := h.deviceService.GetDeviceByKey(deviceKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}

	var req struct {
		SerialPort string  `json:"serial_port"`
		BaudRate   int     `json:"baud_rate"`
		DataBits   int     `json:"data_bits"`
		StopBits   float64 `json:"stop_bits"`
		Parity     string  `json:"parity"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stopBits := int(req.StopBits)
	if req.StopBits == 1.5 {
		stopBits = 15 // 特殊值表示 1.5
	}

	updates := map[string]interface{}{
		"serial_port": req.SerialPort,
		"baud_rate":   req.BaudRate,
		"data_bits":   req.DataBits,
		"stop_bits":   stopBits,
		"parity":      req.Parity,
	}

	if err := h.deviceService.UpdateDevice(device.ID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置更新成功"})
}

// UpdateDeviceConfig 更新设备配置
func (h *DeviceHandler) UpdateDeviceConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var req struct {
		SerialPort string `json:"serial_port"`
		BaudRate   int    `json:"baud_rate"`
		DataBits   int    `json:"data_bits"`
		StopBits   int    `json:"stop_bits"`
		Parity     string `json:"parity"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{
		"serial_port": req.SerialPort,
		"baud_rate":   req.BaudRate,
		"data_bits":   req.DataBits,
		"stop_bits":   req.StopBits,
		"parity":      req.Parity,
	}

	if err := h.deviceService.UpdateDevice(uint(id), updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置更新成功"})
}

// RegenerateKey 重新生成设备Key
func (h *DeviceHandler) RegenerateKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	newKey, err := h.deviceService.GenerateDeviceKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.deviceService.UpdateDevice(uint(id), map[string]interface{}{"device_key": newKey}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"device_key": newKey}})
}

// StatsHandler 统计处理器
type StatsHandler struct {
	statsService *services.StatsService
}

// NewStatsHandler 创建统计处理器
func NewStatsHandler(statsService *services.StatsService) *StatsHandler {
	return &StatsHandler{statsService: statsService}
}

// GetStats 获取统计信息
func (h *StatsHandler) GetStats(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")
	stats := h.statsService.GetStats(tenantID)
	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// LogHandler 日志处理器
type LogHandler struct {
	logService *services.LogService
}

// NewLogHandler 创建日志处理器
func NewLogHandler(logService *services.LogService) *LogHandler {
	return &LogHandler{logService: logService}
}

// GetLogs 获取日志
func (h *LogHandler) GetLogs(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")
	limit := 50
	if l := c.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil {
			limit = val
		}
	}

	logs, err := h.logService.GetLogs(tenantID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": logs})
}