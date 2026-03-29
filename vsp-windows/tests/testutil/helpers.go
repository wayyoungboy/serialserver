package testutil

import (
	"fmt"
	"os"
	"time"
)

// CheckCom0ComInstalled checks if com0com driver is installed
func CheckCom0ComInstalled() bool {
	possiblePaths := []string{
		"C:\\Program Files (x86)\\com0com\\setupc.exe",
		"C:\\Program Files\\com0com\\setupc.exe",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// CheckServerRunning checks if a server is running on the given port
func CheckServerRunning(port int) bool {
	// Try to connect
	conn, err := os.Open(fmt.Sprintf("\\\\.\\pipe\\vsp-test-%d", port))
	if err == nil {
		conn.Close()
		return true
	}
	return false
}

// WaitForCondition waits for a condition to be true with timeout
func WaitForCondition(condition func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// GenerateTestData generates test data of specified size
func GenerateTestData(size int) []byte {
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte('A' + (i % 26))
	}
	return data
}

// GenerateBinaryTestData generates binary test data
func GenerateBinaryTestData(size int) []byte {
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte(i % 256)
	}
	return data
}

// ModbusTestPatterns contains common Modbus test patterns
var ModbusTestPatterns = map[string][]byte{
	"read_holding_query": {0x01, 0x03, 0x00, 0x00, 0x00, 0x0A, 0xC5, 0xCD},
	"read_holding_response": {
		0x01, 0x03, 0x14,
		0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x04,
		0x00, 0x05, 0x00, 0x06, 0x00, 0x07, 0x00, 0x08,
		0x00, 0x09, 0x00, 0x0A,
		0xCE, 0x4E,
	},
	"write_single_query":  {0x01, 0x06, 0x00, 0x01, 0x00, 0x03, 0x98, 0x0B},
	"write_single_response": {0x01, 0x06, 0x00, 0x01, 0x00, 0x03, 0x98, 0x0B},
}