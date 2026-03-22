package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"vsp-client/internal/api"
	"vsp-client/internal/config"
	"vsp-client/internal/serial"
	"vsp-client/internal/tcp"
	"vsp-client/internal/web"
)

//go:embed static/*
var staticAssets embed.FS

var (
	version    = flag.Bool("version", false, "显示版本信息")
	configPath = flag.String("config", "config.json", "配置文件路径")
	nogui      = flag.Bool("nogui", false, "无GUI模式")
)

func main() {
	flag.Parse()

	if *version {
		fmt.Println("VSP Manager v1.0.0")
		fmt.Println("虚拟串口管理器 - 支持串口转TCP/TCP转串口")
		fmt.Println("")
		fmt.Println("用法:")
		fmt.Println("  vsp-client -config config.json")
		fmt.Println("  vsp-client -nogui  # 命令行模式")
		os.Exit(0)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stdout)

	log.Println("=== VSP Manager v1.0.0 ===")

	configMgr := config.New(*configPath)
	if err := configMgr.Load(); err != nil {
		log.Printf("加载配置失败: %v，使用默认配置", err)
	}

	serialMgr := serial.NewPortManager()
	tunnelMgr := tcp.NewManager()
	logger := api.NewLogger()

	cfg := configMgr.Get()
	apiPort := cfg.UI.Port + 1
	if apiPort == cfg.UI.Port {
		apiPort = 8081
	}

	apiServer := api.NewServer(apiPort, configMgr, serialMgr, tunnelMgr, logger)
	go apiServer.Start()

	webServer := web.NewServer(cfg.UI.Port, apiPort, staticAssets)
	go webServer.Start()

	log.Printf("Web UI: http://localhost:%d", cfg.UI.Port)
	log.Printf("API:   http://localhost:%d", apiPort)

	runConfiguredTunnels(configMgr, serialMgr, tunnelMgr, logger)

	runCLI(configMgr, serialMgr, tunnelMgr, logger)
}

// runConfiguredTunnels 启动配置中的隧道
func runConfiguredTunnels(cfg *config.Manager, serialMgr *serial.PortManager, tunnelMgr *tcp.Manager, logger *api.Logger) {
	tunnels := cfg.GetEnabledTunnels()
	log.Printf("配置了 %d 个隧道", len(tunnels))

	for _, t := range tunnels {
		go startTunnel(t, serialMgr, tunnelMgr, logger)
	}
}

// startTunnel 启动单个隧道
func startTunnel(t config.TunnelConfig, serialMgr *serial.PortManager, tunnelMgr *tcp.Manager, logger *api.Logger) error {
	sp, err := serialMgr.Open(t.Serial)
	if err != nil {
		log.Printf("打开串口失败 %s: %v", t.Serial.Port, err)
		logger.Log("error", fmt.Sprintf("打开串口失败 %s: %v", t.Serial.Port, err))
		return err
	}

	addr := fmt.Sprintf("%s:%d", t.TCP.Host, t.TCP.Port)

	var tnl *tcp.Tunnel
	switch t.Mode {
	case "client":
		tnl, err = tunnelMgr.StartClient(t.Name, sp, addr)
	case "server":
		tnl, err = tunnelMgr.StartServer(t.Name, sp, addr)
	case "tunnel":
		tnl, err = tunnelMgr.StartTunnel(t.Name, sp, addr)
	default:
		err = fmt.Errorf("未知模式: %s", t.Mode)
	}

	if err != nil {
		log.Printf("启动隧道失败 %s: %v", t.Name, err)
		logger.Log("error", fmt.Sprintf("启动隧道失败 %s: %v", t.Name, err))
		return err
	}

	tnl.SetDataCallback(func(data []byte) {
		logger.Log("data", fmt.Sprintf("[%s] %s", t.Name, string(data)))
	})

	logger.Log("tunnel", fmt.Sprintf("隧道 [%s] 已启动: %s <-> tcp://%s", t.Name, t.Serial.Port, addr))
	log.Printf("隧道 [%s] 已启动: %s <-> tcp://%s", t.Name, t.Serial.Port, addr)
	return nil
}

// runCLI 运行命令行
func runCLI(cfg *config.Manager, serialMgr *serial.PortManager, tunnelMgr *tcp.Manager, logger *api.Logger) {
	log.Println("VSP Manager 运行中...")
	log.Println("按 Ctrl+C 退出")

	log.Printf("当前活动隧道: %d", len(tunnelMgr.ListTunnels()))
	log.Printf("Web UI: http://localhost:%d", cfg.Get().UI.Port)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("正在关闭...")
	for _, name := range tunnelMgr.ListTunnels() {
		tunnelMgr.Stop(name)
		logger.Log("tunnel", fmt.Sprintf("隧道 %s 已停止", name))
		log.Printf("隧道 %s 已停止", name)
	}
	logger.Close()
	log.Println("已退出")
}
