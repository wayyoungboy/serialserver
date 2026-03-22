package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"vsp-server/internal/api"
	"vsp-server/internal/config"
	"vsp-server/internal/database"
	"vsp-server/internal/services"

	"github.com/gin-gonic/gin"
)

var (
	Version = "2.0.0"
)

func main() {
	// 加载配置
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Printf("加载配置失败，使用默认配置: %v", err)
	}

	// 初始化数据库
	if err := database.Init(cfg.Database.Path); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer database.Close()

	// 创建默认数据
	if err := database.CreateDefaultData(); err != nil {
		log.Printf("创建默认数据失败: %v", err)
	}

	// 设置运行模式
	gin.SetMode(cfg.Server.Mode)

	// 创建Gin引擎
	engine := gin.New()
	engine.Use(gin.Logger())

	// 创建服务
	authService := services.NewAuthService(cfg.JWT.Secret, cfg.JWT.ExpireTime)
	deviceService := services.NewDeviceService()
	statsService := services.NewStatsService()
	logService := services.NewLogService()

	// 设置路由
	router := api.NewRouter(engine, authService, deviceService, statsService, logService)
	router.Setup()

	// 启动服务器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("=== VSP Server v%s ===", Version)
	log.Printf("服务启动: http://%s", addr)
	log.Printf("API文档: http://%s/api/v1", addr)
	log.Printf("默认管理员: admin / admin123")

	// 优雅关闭
	go func() {
		if err := engine.Run(addr); err != nil {
			log.Fatalf("启动服务器失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")
}