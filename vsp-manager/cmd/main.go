package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"vsp-manager/internal/config"
	"vsp-manager/internal/serial"
	"vsp-manager/internal/tcp"
)

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
		fmt.Println("  vsp-manager -config config.json")
		fmt.Println("  vsp-manager -nogui  # 命令行模式")
		os.Exit(0)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stdout)

	log.Println("=== VSP Manager v1.0.0 ===")

	// 初始化组件
	configMgr := config.New(*configPath)
	if err := configMgr.Load(); err != nil {
		log.Printf("加载配置失败: %v，使用默认配置", err)
	}

	serialMgr := serial.NewPortManager()
	tunnelMgr := tcp.NewManager()

	// 启动配置的隧道
	runConfiguredTunnels(configMgr, serialMgr, tunnelMgr)

	// 命令行模式
	runCLI(configMgr, serialMgr, tunnelMgr)
}

// runConfiguredTunnels 启动配置中的隧道
func runConfiguredTunnels(cfg *config.Manager, serialMgr *serial.PortManager, tunnelMgr *tcp.Manager) {
	tunnels := cfg.GetEnabledTunnels()
	log.Printf("配置了 %d 个隧道", len(tunnels))

	for _, t := range tunnels {
		go startTunnel(t, serialMgr, tunnelMgr)
	}
}

// startTunnel 启动单个隧道
func startTunnel(t config.TunnelConfig, serialMgr *serial.PortManager, tunnelMgr *tcp.Manager) error {
	// 打开串口
	sp, err := serialMgr.Open(t.Serial)
	if err != nil {
		log.Printf("打开串口失败 %s: %v", t.Serial.Port, err)
		return err
	}

	// 根据模式启动
	addr := fmt.Sprintf("%s:%d", t.TCP.Host, t.TCP.Port)

	switch t.Mode {
	case "client":
		_, err = tunnelMgr.StartClient(t.Name, sp, addr)
	case "server":
		_, err = tunnelMgr.StartServer(t.Name, sp, addr)
	case "tunnel":
		_, err = tunnelMgr.StartTunnel(t.Name, sp, addr)
	default:
		err = fmt.Errorf("未知模式: %s", t.Mode)
	}

	if err != nil {
		log.Printf("启动隧道失败 %s: %v", t.Name, err)
		return err
	}

	log.Printf("隧道 [%s] 已启动: %s <-> tcp://%s", t.Name, t.Serial.Port, addr)
	return nil
}

// runCLI 运行命令行
func runCLI(cfg *config.Manager, serialMgr *serial.PortManager, tunnelMgr *tcp.Manager) {
	log.Println("VSP Manager 运行中...")
	log.Println("按 Ctrl+C 退出")

	// 显示状态
	log.Printf("当前活动隧道: %d", len(tunnelMgr.ListTunnels()))

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("正在关闭...")
	for _, name := range tunnelMgr.ListTunnels() {
		tunnelMgr.Stop(name)
		log.Printf("隧道 %s 已停止", name)
	}
	log.Println("已退出")
}
