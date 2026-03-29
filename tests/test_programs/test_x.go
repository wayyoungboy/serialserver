package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/tarm/serial"
)

// TestX - 串口写入测试程序
// 对指定串口写入数据，并读取返回数据验证
// 用法: test_x -port COM# -data "test message" -timeout 10

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: test_x -port COM# -data \"message\" [-timeout seconds]")
		fmt.Println("Example: test_x -port COM5 -data \"hello world\" -timeout 5")
		os.Exit(1)
	}

	portName := ""
	dataToSend := ""
	timeoutSec := 10

	// 解析参数
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "-port" && i+1 < len(os.Args) {
			portName = os.Args[i+1]
			i++
		} else if os.Args[i] == "-data" && i+1 < len(os.Args) {
			dataToSend = os.Args[i+1]
			i++
		} else if os.Args[i] == "-timeout" && i+1 < len(os.Args) {
			timeoutSec = parseInt(os.Args[i+1])
			i++
		}
	}

	if portName == "" || dataToSend == "" {
		log.Fatal("必须指定 -port 和 -data 参数")
	}

	// 打开串口
	cfg := &serial.Config{
		Name: portName,
		Baud: 115200,
	}
	port, err := serial.OpenPort(cfg)
	if err != nil {
		log.Fatalf("打开串口 %s 失败: %v", portName, err)
	}
	defer port.Close()

	log.Printf("已连接串口 %s", portName)

	// 写入数据
	n, err := port.Write([]byte(dataToSend))
	if err != nil {
		log.Fatalf("写入数据失败: %v", err)
	}
	log.Printf("发送数据: %s (%d bytes)", dataToSend, n)

	// 等待并读取返回数据
	buf := make([]byte, 1024)
	timeout := time.Duration(timeoutSec) * time.Second
	start := time.Now()

	var received string
	for time.Since(start) < timeout {
		n, err = port.Read(buf)
		if n > 0 {
			received += string(buf[:n])
			log.Printf("接收数据: %s (%d bytes)", string(buf[:n]), n)
			// 检查是否收到完整响应 (以换行符结尾或包含预期格式)
			if isValidResponse(received, dataToSend) {
				break
			}
		}
		if err != nil {
			log.Printf("读取错误: %v", err)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 验证结果
	expectedResponse := fmt.Sprintf("get %s", dataToSend)
	if received == expectedResponse || received == expectedResponse+"\n" || received == expectedResponse+"\r\n" {
		log.Printf("测试成功! 期望: %s, 收到: %s", expectedResponse, received)
		fmt.Println("PASS")
		os.Exit(0)
	} else {
		log.Printf("测试失败! 期望: %s, 收到: %s", expectedResponse, received)
		fmt.Println("FAIL")
		os.Exit(1)
	}
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

func isValidResponse(received, sent string) bool {
	expected := fmt.Sprintf("get %s", sent)
	return received == expected || received == expected+"\n" || received == expected+"\r\n"
}