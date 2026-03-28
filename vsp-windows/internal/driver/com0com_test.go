package driver

import (
	"testing"
)

func TestParsePortList(t *testing.T) {
	// Sample output from setupc.exe list
	sampleOutput := `
CNCA0
PortName=COM5
RealPortName=COM5
Enabled=1

CNCB0
PortName=-
RealPortName=-
Enabled=1

CNCA1
PortName=COM10
RealPortName=COM10
Enabled=1

CNCB1
PortName=-
RealPortName=-
Enabled=1
`

	ports := parsePortList(sampleOutput)

	// Should parse 4 ports
	if len(ports) != 4 {
		t.Errorf("Expected 4 ports, got %d", len(ports))
	}

	// Check CNCA0 properties
	if cncA0, ok := ports["CNCA0"]; ok {
		if cncA0["PortName"] != "COM5" {
			t.Errorf("Expected CNCA0 PortName 'COM5', got '%s'", cncA0["PortName"])
		}
		if cncA0["RealPortName"] != "COM5" {
			t.Errorf("Expected CNCA0 RealPortName 'COM5', got '%s'", cncA0["RealPortName"])
		}
	} else {
		t.Error("Expected CNCA0 to be parsed")
	}

	// Check CNCB0 properties (hidden port)
	if cncB0, ok := ports["CNCB0"]; ok {
		if cncB0["PortName"] != "-" {
			t.Errorf("Expected CNCB0 PortName '-', got '%s'", cncB0["PortName"])
		}
	} else {
		t.Error("Expected CNCB0 to be parsed")
	}
}

func TestParsePortListEmpty(t *testing.T) {
	ports := parsePortList("")
	if len(ports) != 0 {
		t.Errorf("Expected 0 ports from empty input, got %d", len(ports))
	}
}

func TestParsePortListMalformed(t *testing.T) {
	malformedOutput := `
Some random text
Not a port definition
Key without value
=
`

	_ = parsePortList(malformedOutput) // Should handle malformed input gracefully
}

func TestPortPairStruct(t *testing.T) {
	pair := PortPair{
		HiddenPort:  "CNCB0",
		VisiblePort: "COM5",
	}

	if pair.HiddenPort != "CNCB0" {
		t.Errorf("Expected HiddenPort 'CNCB0', got '%s'", pair.HiddenPort)
	}

	if pair.VisiblePort != "COM5" {
		t.Errorf("Expected VisiblePort 'COM5', got '%s'", pair.VisiblePort)
	}
}

func TestCom0ComManagerCreation(t *testing.T) {
	m := NewCom0ComManager()

	if m == nil {
		t.Fatal("NewCom0ComManager returned nil")
	}

	// createdPorts should be initialized (check via GetCreatedPorts)
	ports := m.GetCreatedPorts()
	if ports == nil {
		t.Error("Expected createdPorts to be initialized")
	}
}

func TestCom0ComManagerGetCreatedPorts(t *testing.T) {
	m := NewCom0ComManager()

	// Initially empty
	ports := m.GetCreatedPorts()
	if len(ports) != 0 {
		t.Errorf("Expected empty createdPorts initially, got %d", len(ports))
	}
}

func TestFindAvailableComPort(t *testing.T) {
	m := NewCom0ComManager()

	// Find available port starting from 5
	port := m.FindAvailableComPort(5)

	// Should return a COM port name
	if len(port) < 4 {
		t.Errorf("Expected COM port name, got '%s'", port)
	}

	// Should start with "COM"
	if port[:3] != "COM" {
		t.Errorf("Expected port to start with 'COM', got '%s'", port)
	}
}

func TestPortExists(t *testing.T) {
	m := NewCom0ComManager()

	// COM1 usually exists on Windows, but we can't guarantee
	// Just test the function doesn't panic
	_ = m.PortExists("COM1")
	_ = m.PortExists("COM999")
}

func TestListExistingPortsParsing(t *testing.T) {
	// This test verifies the parsing logic
	// Actual port listing requires com0com installed

	// Sample output with pair
	sampleOutput := `
CNCA5
PortName=COM20
RealPortName=COM20

CNCB5
PortName=-
RealPortName=-
`

	ports := parsePortList(sampleOutput)

	// Verify both ports parsed
	if len(ports) < 2 {
		t.Errorf("Expected at least 2 ports, got %d", len(ports))
	}

	// Find the visible port
	var visibleFound, hiddenFound bool
	for name, props := range ports {
		if name == "CNCA5" && props["RealPortName"] == "COM20" {
			visibleFound = true
		}
		if name == "CNCB5" && props["RealPortName"] == "-" {
			hiddenFound = true
		}
	}

	if !visibleFound {
		t.Error("Expected to find visible port CNCA5 with COM20")
	}
	if !hiddenFound {
		t.Error("Expected to find hidden port CNCB5")
	}
}