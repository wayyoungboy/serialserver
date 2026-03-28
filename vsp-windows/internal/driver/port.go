package driver

import (
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/tarm/serial"
)

// PortClient represents a serial port client
type PortClient struct {
	portName  string
	baudRate  int
	handle    syscall.Handle // Windows HANDLE for CreateFile fallback
	serialPort *serial.Port  // go-serial port object
	mu        sync.RWMutex
	closed    bool
}

// NewPortClient creates a new PortClient instance
func NewPortClient() *PortClient {
	return &PortClient{}
}

// Open opens a serial port with the specified name and baud rate
// First tries go-serial, then falls back to Windows CreateFile API for special ports
func (p *PortClient) Open(portName string, baudRate int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.closed && (p.serialPort != nil || p.handle != syscall.InvalidHandle) {
		return nil // Already open
	}

	p.portName = portName
	p.baudRate = baudRate
	p.closed = false

	// First, try go-serial (tarm/serial)
	err := p.openWithGoSerial()
	if err == nil {
		return nil
	}

	// If go-serial fails (e.g., for CNCA/CNCB special ports), try CreateFile
	err = p.openWithCreateFile()
	if err != nil {
		return fmt.Errorf("failed to open port %s: both go-serial and CreateFile failed", portName)
	}

	return nil
}

// openWithGoSerial attempts to open the port using go-serial library
func (p *PortClient) openWithGoSerial() error {
	config := &serial.Config{
		Name:        p.portName,
		Baud:        p.baudRate,
		ReadTimeout: time.Second * 5,
		Size:        8,          // Data bits
		Parity:      serial.ParityNone,
		StopBits:    serial.Stop1,
	}

	port, err := serial.OpenPort(config)
	if err != nil {
		return fmt.Errorf("go-serial open failed: %w", err)
	}

	p.serialPort = port
	p.handle = syscall.InvalidHandle
	return nil
}

// openWithCreateFile opens the port using Windows CreateFile API
// This is used for special ports like CNCA0, CNCB0 that go-serial cannot handle
func (p *PortClient) openWithCreateFile() error {
	// Prepare device name (e.g., COM1 -> \\.\COM1, CNCA0 -> \\.\CNCA0)
	deviceName := p.portName
	if len(p.portName) > 0 && p.portName[0] != '\\' {
		deviceName = "\\\\.\\" + p.portName
	}

	// Convert to UTF16 pointer
	deviceNamePtr, err := syscall.UTF16PtrFromString(deviceName)
	if err != nil {
		return fmt.Errorf("failed to convert device name: %w", err)
	}

	// CreateFile parameters
	const (
		GENERIC_READ  = 0x80000000
		GENERIC_WRITE = 0x40000000
		FILE_SHARE_READ  = 0x00000001
		FILE_SHARE_WRITE = 0x00000002
		OPEN_EXISTING = 3
		FILE_ATTRIBUTE_NORMAL = 0x80
	)

	handle, err := syscall.CreateFile(
		deviceNamePtr,
		GENERIC_READ|GENERIC_WRITE,
		FILE_SHARE_READ|FILE_SHARE_WRITE,
		nil,
		OPEN_EXISTING,
		FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return fmt.Errorf("CreateFile failed: %w", err)
	}

	// Configure serial port parameters (baud rate, etc.)
	if err := p.configureSerialPort(handle); err != nil {
		syscall.CloseHandle(handle)
		return fmt.Errorf("failed to configure serial port: %w", err)
	}

	p.handle = handle
	p.serialPort = nil
	return nil
}

// configureSerialPort configures the serial port parameters using Windows DCB
func (p *PortClient) configureSerialPort(handle syscall.Handle) error {
	// DCB structure for serial port configuration
	type DCB struct {
		DCBlength           uint32
		BaudRate            uint32
		fBinary             uint32
		fParity             uint32
		fOutxCtsFlow        uint32
		fOutxDsrFlow        uint32
		fDtrControl         uint32
		fDsrSensitivity     uint32
		fTXContinueOnXoff   uint32
		fOutX               uint32
		fInX                uint32
		fErrorChar          uint32
		fNull               uint32
		fRtsControl         uint32
		fAbortOnError       uint32
		fDummy2             uint32
		wReserved           uint16
		XonLim               uint16
		XoffLim             uint16
		ByteSize            byte
		Parity              byte
		StopBits            byte
		XonChar             byte
		XoffChar            byte
		ErrorChar           byte
		EofChar             byte
		EvtChar             byte
		wReserved1          uint16
	}

	// GetCommState
	var dcb DCB
	dcb.DCBlength = uint32(unsafe.Sizeof(dcb))

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getCommState := kernel32.NewProc("GetCommState")
	setCommState := kernel32.NewProc("SetCommState")

	ret, _, err := getCommState.Call(uintptr(handle), uintptr(unsafe.Pointer(&dcb)))
	if ret == 0 {
		return fmt.Errorf("GetCommState failed: %v", err)
	}

	// Set baud rate and other parameters
	dcb.BaudRate = uint32(p.baudRate)
	dcb.ByteSize = 8
	dcb.Parity = 0 // NOPARITY
	dcb.StopBits = 0 // ONESTOPBIT
	dcb.fBinary = 1

	ret, _, err = setCommState.Call(uintptr(handle), uintptr(unsafe.Pointer(&dcb)))
	if ret == 0 {
		return fmt.Errorf("SetCommState failed: %v", err)
	}

	// Set timeouts
	type COMMTIMEOUTS struct {
		ReadIntervalTimeout         uint32
		ReadTotalTimeoutMultiplier  uint32
		ReadTotalTimeoutConstant    uint32
		WriteTotalTimeoutMultiplier uint32
		WriteTotalTimeoutConstant   uint32
	}

	var timeouts COMMTIMEOUTS
	timeouts.ReadIntervalTimeout = 50
	timeouts.ReadTotalTimeoutConstant = 5000
	timeouts.ReadTotalTimeoutMultiplier = 10
	timeouts.WriteTotalTimeoutConstant = 5000
	timeouts.WriteTotalTimeoutMultiplier = 10

	setCommTimeouts := kernel32.NewProc("SetCommTimeouts")
	ret, _, err = setCommTimeouts.Call(uintptr(handle), uintptr(unsafe.Pointer(&timeouts)))
	if ret == 0 {
		return fmt.Errorf("SetCommTimeouts failed: %v", err)
	}

	return nil
}

// Read reads data from the serial port (blocking)
func (p *PortClient) Read(buf []byte) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return 0, fmt.Errorf("port closed")
	}

	// Use go-serial if available
	if p.serialPort != nil {
		return p.serialPort.Read(buf)
	}

	// Use Windows ReadFile if using CreateFile handle
	if p.handle != syscall.InvalidHandle && p.handle != 0 {
		var bytesRead uint32
		err := syscall.ReadFile(p.handle, buf, &bytesRead, nil)
		if err != nil {
			return 0, fmt.Errorf("ReadFile failed: %w", err)
		}
		return int(bytesRead), nil
	}

	return 0, fmt.Errorf("port not opened")
}

// Write writes data to the serial port
func (p *PortClient) Write(data []byte) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return 0, fmt.Errorf("port closed")
	}

	// Use go-serial if available
	if p.serialPort != nil {
		return p.serialPort.Write(data)
	}

	// Use Windows WriteFile if using CreateFile handle
	if p.handle != syscall.InvalidHandle && p.handle != 0 {
		var bytesWritten uint32
		err := syscall.WriteFile(p.handle, data, &bytesWritten, nil)
		if err != nil {
			return 0, fmt.Errorf("WriteFile failed: %w", err)
		}
		return int(bytesWritten), nil
	}

	return 0, fmt.Errorf("port not opened")
}

// Close closes the serial port
func (p *PortClient) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	var err error

	// Close go-serial port
	if p.serialPort != nil {
		err = p.serialPort.Close()
		p.serialPort = nil
	}

	// Close Windows handle
	if p.handle != syscall.InvalidHandle && p.handle != 0 {
		if closeErr := syscall.CloseHandle(p.handle); closeErr != nil {
			if err == nil {
				err = closeErr
			}
		}
		p.handle = syscall.InvalidHandle
	}

	return err
}

// IsOpen returns whether the port is currently open
func (p *PortClient) IsOpen() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return false
	}

	return p.serialPort != nil || (p.handle != syscall.InvalidHandle && p.handle != 0)
}

// GetPortName returns the port name
func (p *PortClient) GetPortName() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.portName
}

// GetBaudRate returns the baud rate
func (p *PortClient) GetBaudRate() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.baudRate
}