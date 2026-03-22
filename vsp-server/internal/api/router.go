package api

import (
	"vsp-server/internal/api/handlers"
	"vsp-server/internal/api/middleware"
	"vsp-server/internal/services"
	"vsp-server/internal/websocket"

	"github.com/gin-gonic/gin"
)

// Router API路由
type Router struct {
	engine        *gin.Engine
	authHandler   *handlers.AuthHandler
	deviceHandler *handlers.DeviceHandler
	statsHandler  *handlers.StatsHandler
	logHandler    *handlers.LogHandler
	authService   *services.AuthService
	wsHub         *websocket.Hub
}

// NewRouter 创建路由
func NewRouter(engine *gin.Engine, authService *services.AuthService, deviceService *services.DeviceService, statsService *services.StatsService, logService *services.LogService) *Router {
	return &Router{
		engine:        engine,
		authHandler:   handlers.NewAuthHandler(authService),
		deviceHandler: handlers.NewDeviceHandler(deviceService),
		statsHandler:  handlers.NewStatsHandler(statsService),
		logHandler:    handlers.NewLogHandler(logService),
		authService:   authService,
		wsHub:         websocket.NewHub(logService),
	}
}

// Setup 设置路由
func (r *Router) Setup() {
	// 中间件
	r.engine.Use(middleware.CORS())
	r.engine.Use(gin.Recovery())

	// 静态文件
	r.engine.Static("/static", "./web/dist/static")
	r.engine.StaticFile("/", "./web/dist/index.html")

	// API路由组
	api := r.engine.Group("/api/v1")
	{
		// 认证路由（无需认证）
		auth := api.Group("/auth")
		{
			auth.POST("/register", r.authHandler.Register)
			auth.POST("/login", r.authHandler.Login)
		}

		// 需要认证的路由
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(r.authService))
		{
			// 用户信息
			protected.GET("/profile", r.authHandler.GetProfile)

			// 设备管理
			devices := protected.Group("/devices")
			{
				devices.GET("", r.deviceHandler.ListDevices)
				devices.POST("", r.deviceHandler.CreateDevice)
				devices.GET("/:id", r.deviceHandler.GetDevice)
				devices.PUT("/:id", r.deviceHandler.UpdateDevice)
				devices.DELETE("/:id", r.deviceHandler.DeleteDevice)
				devices.PUT("/:id/config", r.deviceHandler.UpdateDeviceConfig)
				devices.POST("/:id/regenerate-key", r.deviceHandler.RegenerateKey)
			}

			// 设备配置 (设备端通过 DeviceKey 获取，无需登录)
			api.GET("/devices/config", r.deviceHandler.GetDeviceConfig)
			api.PUT("/devices/by-key/:key/config", r.deviceHandler.UpdateDeviceConfigByKey)

			// 统计信息
			protected.GET("/stats", r.statsHandler.GetStats)

			// 日志
			protected.GET("/logs", r.logHandler.GetLogs)
		}

		// WebSocket路由
		api.GET("/ws/device", r.wsHub.HandleDevice)
		api.GET("/ws/client", r.wsHub.HandleClient)
	}
}