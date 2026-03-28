package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/tarm/serial"
)

// 测试程序X - 串口读写测试
// 功能: 向串口写入数据，读取响应，验证是否为"get <发送的数据>"

func main() {
	portName := flag.String("port", "COM1", "串口名称")
	baudRate := flag.Int("baud", 115200, "波特率")
	testData := flag.String("data", "Hello VSP", "测试数据")
	interval := flag.Int("interval", 1000, "发送间隔(毫秒)")
	count := flag.Int("count", 5, "发送次数")
	flag.Parse()

	log.Printf("=== 测试程序X - 串口读写测试 ===")
	log.Printf("串口: %s, 波特率: %d", *portName, *baudRate)
	log.Printf("测试数据: %s", *testData)
	log.Printf("发送次数: %d, 间隔: %dms", *count, *interval)

	// 打开串口
	config := &serial.Config{
		Name:        *portName,
		Baud:        *baudRate,
		ReadTimeout: time.Second * 5,
		Size:        8,
		Parity:      serial.ParityNone,
		StopBits:    serial.Stop1,
	}

	port, err := serial.OpenPort(config)
	if err != nil {
		log.Fatalf("无法打开串口 %s: %v", *portName, err)
	}
	defer port.Close()

	log.Printf("串口 %s 已打开", *portName)

	// 统计结果
	successCount := 0
	failCount := 0
	totalLatency := time.Duration(0)

	// 启动读取goroutine
	readChan := make(chan string, 10)
	errChan := make(chan error, 1)

	go func() {
		reader := bufio.NewReader(port)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				errChan <- err
				return
			}
			select {
			case readChan <- strings.TrimSpace(line):
			default:
				// 缓冲区满，丢弃
			}
		}
	}()

	// 发送并接收测试
	for i := 1; i <= *count; i++ {
		log.Printf("\n--- 第 %d/%d 次测试 ---", i, *count)

		// 发送数据
		sendData := fmt.Sprintf("%s_%d", *testData, i)
		sendBytes := []byte(sendData + "\n")

		startTime := time.Now()
		n, err := port.Write(sendBytes)
		if err != nil {
			log.Printf("发送失败: %v", err)
			failCount++
			continue
		}
		log.Printf("发送: %s (%d bytes)", sendData, n)

		// 等待响应
		select {
		case response := <-readChan:
			latency := time.Since(startTime)
			totalLatency += latency

			log.Printf("接收: %s (延迟: %v)", response, latency)

			// 验证响应
			expected := "get " + sendData
			if response == expected {
				log.Printf("✓ 测试通过: 响应正确", )
				successCount++
			} else {
				log.Printf("✗ 测试失败: 期望 '%s', 实际 '%s'", expected, response)
				failCount++
			}

		case err := <-errChan:
			log.Printf("读取错误: %v", err)
			failCount++

		case <-time.After(5 * time.Second):
			log.Printf("✗ 超时: 5秒内未收到响应")
			failCount++
		}

		// 等待间隔
		if i < *count {
			time.Sleep(time.Duration(*interval) * time.Millisecond)
		}
	}

	// 统计结果
	log.Printf("\n========== 测试结果 ==========")
	log.Printf("总测试次数: %d", *count)
	log.Printf("成功: %d, 失败: %d", successCount, failCount)
	if successCount > 0 {
		avgLatency := totalLatency / time.Duration(successCount)
		log.Printf("平均延迟: %v", avgLatency)
	}
	log.Printf("==============================")

	if failCount > 0 {
		os.Exit(1)
	}
}