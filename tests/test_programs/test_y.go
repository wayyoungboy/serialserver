package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/tarm/serial"
)

// TestY - 串口响应测试程序
// 监听指定串口，接收数据后返回 "get xxxx"
// 用法: test_y -port COM# [-timeout seconds]

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: test_y -port COM# [-timeout seconds]")
		fmt.Println("Example: test_y -port COM6 -timeout 30")
		os.Exit(1)
	}

	portName := ""
	timeoutSec := 30

	// 解析参数
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "-port" && i+1 < len(os.Args) {
			portName = os.Args[i+1]
			i++
		} else if os.Args[i] == "-timeout" && i+1 < len(os.Args) {
			timeoutSec = parseInt(os.Args[i+1])
			i++
		}
	}

	if portName == "" {
		log.Fatal("必须指定 -port 参数")
	}

	// 打开串口
	cfg := &serial.Config{
		Name: portName,
		Baud: 115200,
		ReadTimeout: time.Duration(timeoutSec) * time.Second,
	}
	port, err := serial.OpenPort(cfg)
	if err != nil {
		log.Fatalf("打开串口 %s 失败: %v", portName, err)
	}
	defer port.Close()

	log.Printf("TestY 已连接串口 %s，等待接收数据...", portName)

	// 循环读取并响应
	scanner := bufio.NewScanner(port)
	timeout := time.Duration(timeoutSec) * time.Second
	start := time.Now()

	for time.Since(start) < timeout {
		if scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				log.Printf("收到数据: %s", line)

				// 返回 "get xxxx"
				response := fmt.Sprintf("get %s\n", line)
				n, err := port.Write([]byte(response))
				if err != nil {
					log.Printf("写入响应失败: %v", err)
				} else {
					log.Printf("发送响应: %s (%d bytes)", strings.TrimSpace(response), n)
				}
				start = time.Now() // 重置超时计时器
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("扫描错误: %v", err)
			break
		}
	}

	log.Printf("TestY 超时退出")
}

func parseInt(s string) int {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}