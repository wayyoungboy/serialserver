package driver

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PortPair represents a virtual port pair created by com0com
type PortPair struct {
	HiddenPort  string // e.g., CNCA0, CNCB0 (internal port name)
	VisiblePort string // e.g., COM5 (visible COM port)
}

// Com0ComManager manages com0com virtual serial port pairs.
// Note: Running setupc.exe requires administrator privileges on Windows.
// The application must be run as administrator or the user must approve UAC prompts.
type Com0ComManager struct {
	setupcPath  string
	com0comPath string
	createdMu   sync.Mutex
	createdPorts []PortPair
}

// NewCom0ComManager creates a new Com0ComManager instance.
// It searches for setupc.exe in multiple possible locations:
// 1. Application directory/com0com
// 2. Program Files (x86)/com0com
// 3. Program Files/com0com
func NewCom0ComManager() *Com0ComManager {
	m := &Com0ComManager{}

	// Try multiple possible paths for com0com installation
	possiblePaths := []string{
		// Application directory
		filepath.Join(filepath.Dir(os.Args[0]), "com0com"),
		// Program Files (x86)
		os.Getenv("ProgramFiles(x86)") + "\\com0com",
		// Program Files
		os.Getenv("ProgramFiles") + "\\com0com",
	}

	for _, path := range possiblePaths {
		setupcPath := filepath.Join(path, "setupc.exe")
		if _, err := os.Stat(setupcPath); err == nil {
			m.com0comPath = path
			m.setupcPath = setupcPath
			return m
		}
	}

	// Default to application directory
	m.com0comPath = possiblePaths[0]
	m.setupcPath = filepath.Join(m.com0comPath, "setupc.exe")

	return m
}

// IsInstalled checks if com0com is installed
func (m *Com0ComManager) IsInstalled() bool {
	_, err := os.Stat(m.setupcPath)
	return err == nil
}

// RunSetupcCommand executes setupc.exe with the given arguments.
// Note: This requires administrator privileges.
// On Windows, this will trigger a UAC prompt if not already elevated.
func (m *Com0ComManager) RunSetupcCommand(args ...string) (string, error) {
	if !m.IsInstalled() {
		return "", fmt.Errorf("com0com not installed at %s", m.setupcPath)
	}

	cmd := exec.Command(m.setupcPath, args...)
	cmd.Dir = m.com0comPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()
	output := stdout.String()
	if output == "" {
		output = stderr.String()
	}

	if err != nil {
		return output, fmt.Errorf("setupc.exe failed: %w, output: %s", err, output)
	}

	return output, nil
}

// RunSetupcCommandAsAdmin executes setupc.exe with administrator privileges.
// This uses PowerShell to run the command with elevation.
func (m *Com0ComManager) RunSetupcCommandAsAdmin(args ...string) error {
	if !m.IsInstalled() {
		return fmt.Errorf("com0com not installed at %s", m.setupcPath)
	}

	// Build the command arguments
	cmdStr := fmt.Sprintf("& '%s' %s", m.setupcPath, strings.Join(args, " "))

	// Use PowerShell to run with elevation
	psCmd := fmt.Sprintf("Start-Process -FilePath '%s' -ArgumentList '%s' -Verb RunAs -Wait",
		m.setupcPath, strings.Join(args, " "))

	cmd := exec.Command("powershell", "-Command", psCmd)
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run setupc.exe as admin: %w", err)
	}

	// Log for debugging
	_ = cmdStr // suppress unused variable warning
	return nil
}

// parsePortList parses the output of "setupc.exe list" command
// Output format: "       CNCA0 PortName=COM#,RealPortName=COM3"
func parsePortList(output string) map[string]map[string]string {
	ports := make(map[string]map[string]string)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: CNCA0 PortName=xxx,RealPortName=yyy,...
		portMatch := regexp.MustCompile(`^(CNCA\d+|CNCB\d+)\s+(.+)$`)
		if matches := portMatch.FindStringSubmatch(line); matches != nil {
			portName := matches[1]
			paramsStr := matches[2]

			if ports[portName] == nil {
				ports[portName] = make(map[string]string)
			}

			// Parse parameters (key=value, comma separated)
			params := strings.Split(paramsStr, ",")
			for _, param := range params {
				param = strings.TrimSpace(param)
				if strings.Contains(param, "=") {
					parts := strings.SplitN(param, "=", 2)
					if len(parts) == 2 {
						ports[portName][strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
					}
				}
			}
		}
	}

	return ports
}

// ListExistingPorts queries and returns all existing com0com port pairs.
// Returns a list of PortPair structures.
func (m *Com0ComManager) ListExistingPorts() ([]PortPair, error) {
	if !m.IsInstalled() {
		return nil, fmt.Errorf("com0com not installed")
	}

	output, err := m.RunSetupcCommand("list")
	if err != nil {
		return nil, fmt.Errorf("failed to list ports: %w", err)
	}

	ports := parsePortList(output)
	var pairs []PortPair

	// Group ports by their index number (CNCA0 pairs with CNCB0)
	seenPairs := make(map[int]bool)

	for portName, props := range ports {
		// Extract port number
		numStr := regexp.MustCompile(`\d+`).FindString(portName)
		if numStr == "" {
			continue
		}
		num, _ := strconv.Atoi(numStr)

		// Skip if we've already processed this pair
		if seenPairs[num] {
			continue
		}

		// Find the paired port
		var portA, portB string
		var visiblePort string

		if strings.HasPrefix(portName, "CNCA") {
			portA = portName
			portB = fmt.Sprintf("CNCB%d", num)
		} else {
			portA = fmt.Sprintf("CNCA%d", num)
			portB = portName
		}

		// Check which port is visible (has a COM port assigned)
		if props["RealPortName"] != "" && props["RealPortName"] != "-" {
			visiblePort = props["RealPortName"]
		} else if ports[portB] != nil && ports[portB]["RealPortName"] != "" && ports[portB]["RealPortName"] != "-" {
			visiblePort = ports[portB]["RealPortName"]
		}

		// Determine which port is hidden
		var hiddenPort string
		if ports[portA] != nil && ports[portA]["RealPortName"] == "-" {
			hiddenPort = portA
		} else if ports[portB] != nil && ports[portB]["RealPortName"] == "-" {
			hiddenPort = portB
		} else {
			hiddenPort = portA // default
		}

		pairs = append(pairs, PortPair{
			HiddenPort:  hiddenPort,
			VisiblePort: visiblePort,
		})
		seenPairs[num] = true
	}

	return pairs, nil
}

// CreatePortPair creates a new virtual port pair.
// Returns the PortPair with hiddenPort and visiblePort on success.
//
// Note: This requires administrator privileges.
// The visiblePort COM number is automatically assigned by Windows.
func (m *Com0ComManager) CreatePortPair() (*PortPair, error) {
	if !m.IsInstalled() {
		return nil, fmt.Errorf("com0com not installed")
	}

	// Get existing ports before creation
	existingOutput, err := m.RunSetupcCommand("list")
	if err != nil {
		return nil, fmt.Errorf("failed to list existing ports: %w", err)
	}
	existingPorts := parsePortList(existingOutput)
	existingSet := make(map[string]bool)
	for portName := range existingPorts {
		existingSet[portName] = true
	}

	// Step 1: Create basic port pair
	_, err = m.RunSetupcCommand("install", "-", "-")
	if err != nil {
		return nil, fmt.Errorf("failed to install port pair: %w", err)
	}

	// Wait for ports to be created
	time.Sleep(1 * time.Second)

	// Step 2: Find newly created port pair
	listOutput, err := m.RunSetupcCommand("list")
	if err != nil {
		return nil, fmt.Errorf("failed to list ports after creation: %w", err)
	}

	newPorts := parsePortList(listOutput)
	var newPortA, newPortB string

	for portName := range newPorts {
		if !existingSet[portName] {
			if strings.HasPrefix(portName, "CNCA") {
				newPortA = portName
			} else if strings.HasPrefix(portName, "CNCB") {
				newPortB = portName
			}
		}
	}

	if newPortA == "" || newPortB == "" {
		return nil, fmt.Errorf("failed to find new port pair (A=%s, B=%s)", newPortA, newPortB)
	}

	// Step 3: Set visible port (COM# auto-assigned by Windows)
	_, err = m.RunSetupcCommand("change", newPortA, "PortName=COM#")
	if err != nil {
		return nil, fmt.Errorf("failed to set visible port: %w", err)
	}

	// Step 4: Set hidden port
	_, err = m.RunSetupcCommand("change", newPortB, "PortName=-")
	if err != nil {
		return nil, fmt.Errorf("failed to set hidden port: %w", err)
	}

	// Wait for configuration to take effect
	time.Sleep(500 * time.Millisecond)

	// Step 5: Get the actual assigned COM port number
	listOutput, err = m.RunSetupcCommand("list")
	if err != nil {
		return nil, fmt.Errorf("failed to list ports after configuration: %w", err)
	}

	newPorts = parsePortList(listOutput)
	var visiblePort string
	if props, ok := newPorts[newPortA]; ok {
		visiblePort = props["RealPortName"]
	}

	if visiblePort == "" || visiblePort == "-" {
		return nil, fmt.Errorf("failed to get RealPortName for %s", newPortA)
	}

	pair := &PortPair{
		HiddenPort:  newPortB,
		VisiblePort: visiblePort,
	}

	// Record the created port pair
	m.createdMu.Lock()
	m.createdPorts = append(m.createdPorts, *pair)
	m.createdMu.Unlock()

	return pair, nil
}

// RemovePortPair removes a virtual port pair.
// The portName can be either:
//   - A CNC port name (e.g., "CNCA0", "CNCB0")
//   - A visible COM port name (e.g., "COM5")
//
// Note: This requires administrator privileges.
func (m *Com0ComManager) RemovePortPair(portName string) error {
	if !m.IsInstalled() {
		return fmt.Errorf("com0com not installed")
	}

	// Extract port number
	portNum := regexp.MustCompile(`\d+`).FindString(portName)
	if portNum == "" {
		return fmt.Errorf("invalid port name: %s", portName)
	}

	var portA, portB string

	// If the input is a COM port, find the corresponding CNC ports
	if strings.HasPrefix(portName, "COM") {
		output, err := m.RunSetupcCommand("list")
		if err != nil {
			return fmt.Errorf("failed to list ports: %w", err)
		}

		ports := parsePortList(output)
		for cncPort, props := range ports {
			if props["RealPortName"] == portName {
				num := regexp.MustCompile(`\d+`).FindString(cncPort)
				portA = "CNCA" + num
				portB = "CNCB" + num
				break
			}
		}

		if portA == "" {
			return fmt.Errorf("port %s not found", portName)
		}
	} else {
		// Use the port number directly
		portA = "CNCA" + portNum
		portB = "CNCB" + portNum
	}

	// Remove the port pair using just the number
	// According to setupc help: "remove <n> - remove a pair of linked ports with identifiers CNCA<n> and CNCB<n>"
	_, err := m.RunSetupcCommand("remove", portNum)
	if err != nil {
		return fmt.Errorf("failed to remove port pair %s/%s: %w", portA, portB, err)
	}

	// Remove from created ports list
	m.createdMu.Lock()
	for i, p := range m.createdPorts {
		if p.HiddenPort == portA || p.HiddenPort == portB || p.VisiblePort == portName {
			m.createdPorts = append(m.createdPorts[:i], m.createdPorts[i+1:]...)
			break
		}
	}
	m.createdMu.Unlock()

	return nil
}

// RemoveAllCreatedPorts removes all port pairs created during this session.
// This is typically called when the application exits.
func (m *Com0ComManager) RemoveAllCreatedPorts() error {
	m.createdMu.Lock()
	ports := make([]PortPair, len(m.createdPorts))
	copy(ports, m.createdPorts)
	m.createdMu.Unlock()

	var lastErr error
	for _, pair := range ports {
		if err := m.RemovePortPair(pair.HiddenPort); err != nil {
			lastErr = err
		}
	}

	m.createdMu.Lock()
	m.createdPorts = nil
	m.createdMu.Unlock()

	return lastErr
}

// GetCreatedPorts returns a copy of the port pairs created during this session
func (m *Com0ComManager) GetCreatedPorts() []PortPair {
	m.createdMu.Lock()
	defer m.createdMu.Unlock()

	result := make([]PortPair, len(m.createdPorts))
	copy(result, m.createdPorts)
	return result
}

// FindAvailableComPort finds an available COM port number starting from startFrom.
// Returns a port name like "COM5".
func (m *Com0ComManager) FindAvailableComPort(startFrom int) string {
	// Get existing COM ports from Windows
	// Note: This is a simplified implementation
	// For a complete implementation, you would need to use Windows API
	// or check the registry at HKLM\HARDWARE\DEVICEMAP\SERIALCOMM
	for i := startFrom; i < 100; i++ {
		portName := fmt.Sprintf("COM%d", i)
		// Check if port exists
		_, err := os.Stat("\\\\.\\" + portName)
		if err != nil {
			return portName
		}
	}
	return fmt.Sprintf("COM%d", startFrom)
}

// PortExists checks if a COM port already exists
func (m *Com0ComManager) PortExists(portName string) bool {
	_, err := os.Stat("\\\\.\\" + portName)
	return err == nil
}