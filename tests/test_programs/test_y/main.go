package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/tarm/serial"
)

// 测试程序Y - 串口回显服务
// 功能: 接收串口数据，返回 "get <接收到的数据>"

func main() {
	portName := flag.String("port", "COM1", "串口名称")
	baudRate := flag.Int("baud", 115200, "波特率")
	verbose := flag.Bool("v", false, "详细输出")
	flag.Parse()

	log.Printf("=== 测试程序Y - 串口回显服务 ===")
	log.Printf("串口: %s, 波特率: %d", *portName, *baudRate)
	log.Printf("功能: 接收数据后返回 'get <数据>'")
	log.Printf("按 Ctrl+C 退出")

	// 打开串口
	config := &serial.Config{
		Name:        *portName,
		Baud:        *baudRate,
		ReadTimeout: time.Second * 10,
		Size:        8,
		Parity:      serial.ParityNone,
		StopBits:    serial.Stop1,
	}

	port, err := serial.OpenPort(config)
	if err != nil {
		log.Fatalf("无法打开串口 %s: %v", *portName, err)
	}
	defer port.Close()

	log.Printf("串口 %s 已打开，等待数据...", *portName)

	// 统计
	receiveCount := 0
	sendCount := 0

	// 读取并回显
	reader := bufio.NewReader(port)

	for {
		// 读取一行数据
		line, err := reader.ReadString('\n')
		if err != nil {
			if strings.Contains(err.Error(), "timeout") {
				continue // 读超时，继续等待
			}
			log.Printf("读取错误: %v", err)
			continue
		}

		// 处理数据
		data := strings.TrimSpace(line)
		if data == "" {
			continue
		}

		receiveCount++
		if *verbose {
			log.Printf("收到 [%d]: %s", receiveCount, data)
		} else {
			fmt.Printf("收到: %s\n", data)
		}

		// 构造响应
		response := fmt.Sprintf("get %s\n", data)

		// 发送响应
		n, err := port.Write([]byte(response))
		if err != nil {
			log.Printf("发送失败: %v", err)
			continue
		}
		sendCount++

		if *verbose {
			log.Printf("发送 [%d]: %s (%d bytes)", sendCount, strings.TrimSpace(response), n)
		} else {
			fmt.Printf("发送: %s", response)
		}
	}
}