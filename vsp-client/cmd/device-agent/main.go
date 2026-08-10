package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	serialport "github.com/tarm/serial"
)

const protocolVersion = "vsp.relay"

var (
	serverAddr  = flag.String("server", "localhost:9000", "VSP server host or URL")
	deviceKey   = flag.String("key", "", "DeviceKey for this device")
	mappingID   = flag.String("mapping", "default", "local serial mapping id")
	mappingName = flag.String("name", "", "display name for this mapping")
	serialName  = flag.String("port", "", "local serial port, for example COM3 or /dev/ttyUSB0")
	baudRate    = flag.Int("baud", 115200, "serial baud rate")
	dataBits    = flag.Int("data-bits", 8, "serial data bits")
	stopBits    = flag.String("stop-bits", "1", "serial stop bits: 1, 1.5, or 2")
	parity      = flag.String("parity", "N", "serial parity: N, O, E, M, or S")
	flowControl = flag.String("flow-control", "none", "serial flow control metadata")
	secure      = flag.Bool("secure", false, "use wss when server has no scheme")
	reconnect   = flag.Bool("reconnect", true, "reconnect after disconnect")
)

type ControlMessage struct {
	Type      string   `json:"type"`
	Protocol  string   `json:"protocol,omitempty"`
	Role      string   `json:"role,omitempty"`
	DeviceKey string   `json:"device_key,omitempty"`
	MappingID string   `json:"mapping_id,omitempty"`
	Mapping   *Mapping `json:"mapping,omitempty"`
	Status    string   `json:"status,omitempty"`
	Message   string   `json:"message,omitempty"`
	SessionID string   `json:"session_id,omitempty"`
}

type Mapping struct {
	ID     string         `json:"id"`
	Name   string         `json:"name,omitempty"`
	Serial SerialSettings `json:"serial"`
}

type SerialSettings struct {
	Port        string `json:"port"`
	BaudRate    int    `json:"baud_rate"`
	DataBits    int    `json:"data_bits"`
	StopBits    int    `json:"stop_bits"`
	Parity      string `json:"parity"`
	FlowControl string `json:"flow_control,omitempty"`
}

type Agent struct {
	serverURL string
	deviceKey string
	mapping   Mapping
	stopBits  serialport.StopBits
	parity    serialport.Parity
}

type lockedWS struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func main() {
	flag.Parse()

	if *deviceKey == "" {
		log.Fatal("-key is required")
	}
	if *serialName == "" {
		log.Fatal("-port is required")
	}
	if *dataBits < 5 || *dataBits > 8 {
		log.Fatal("-data-bits must be between 5 and 8")
	}

	wsURL, err := buildWebSocketURL(*serverAddr, *secure, "/api/relay/device")
	if err != nil {
		log.Fatalf("invalid server: %v", err)
	}
	serialStopBits, stopBitsValue, err := parseStopBits(*stopBits)
	if err != nil {
		log.Fatal(err)
	}
	serialParity, parityValue, err := parseParity(*parity)
	if err != nil {
		log.Fatal(err)
	}

	name := *mappingName
	if name == "" {
		name = *mappingID
	}
	if strings.ToLower(*flowControl) != "none" {
		log.Printf("flow-control=%s will be announced to the server; the current serial library does not apply it locally", *flowControl)
	}

	agent := Agent{
		serverURL: wsURL,
		deviceKey: *deviceKey,
		mapping: Mapping{
			ID:   *mappingID,
			Name: name,
			Serial: SerialSettings{
				Port:        *serialName,
				BaudRate:    *baudRate,
				DataBits:    *dataBits,
				StopBits:    stopBitsValue,
				Parity:      parityValue,
				FlowControl: *flowControl,
			},
		},
		stopBits: serialStopBits,
		parity:   serialParity,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	for {
		err := agent.Run(ctx)
		if ctx.Err() != nil {
			return
		}
		if !*reconnect {
			if err != nil {
				log.Fatal(err)
			}
			return
		}
		log.Printf("connection ended: %v; reconnecting in 3s", err)
		select {
		case <-time.After(3 * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

func (a Agent) Run(ctx context.Context) error {
	cfg := &serialport.Config{
		Name:        a.mapping.Serial.Port,
		Baud:        a.mapping.Serial.BaudRate,
		ReadTimeout: time.Second,
		Size:        byte(a.mapping.Serial.DataBits),
		Parity:      a.parity,
		StopBits:    a.stopBits,
	}

	sp, err := serialport.OpenPort(cfg)
	if err != nil {
		return fmt.Errorf("open serial %s: %w", a.mapping.Serial.Port, err)
	}
	defer sp.Close()
	log.Printf("serial opened: %s %d,%s,%d,%s", a.mapping.Serial.Port, a.mapping.Serial.BaudRate, a.mapping.Serial.Parity, a.mapping.Serial.DataBits, *stopBits)

	conn, _, err := websocket.DefaultDialer.Dial(a.serverURL, nil)
	if err != nil {
		return fmt.Errorf("connect relay: %w", err)
	}
	defer conn.Close()

	ws := &lockedWS{conn: conn}
	if err := ws.writeJSON(ControlMessage{
		Type:      "hello",
		Protocol:  protocolVersion,
		Role:      "device",
		DeviceKey: a.deviceKey,
		MappingID: a.mapping.ID,
		Mapping:   &a.mapping,
	}); err != nil {
		return fmt.Errorf("send hello: %w", err)
	}

	var auth ControlMessage
	if err := conn.ReadJSON(&auth); err != nil {
		return fmt.Errorf("read auth: %w", err)
	}
	if auth.Type == "error" {
		return fmt.Errorf("relay rejected device: %s", auth.Message)
	}
	if auth.Type != "auth" || auth.Status != "ok" {
		return fmt.Errorf("unexpected relay response: %+v", auth)
	}
	log.Printf("device registered: mapping=%s server=%s", a.mapping.ID, a.serverURL)

	errc := make(chan error, 2)
	done := make(chan struct{})
	go serialToRelay(ctx, sp, ws, done, errc)
	go relayToSerial(ctx, sp, ws, done, errc)

	select {
	case err := <-errc:
		close(done)
		return err
	case <-ctx.Done():
		close(done)
		return ctx.Err()
	}
}

func serialToRelay(ctx context.Context, sp *serialport.Port, ws *lockedWS, done <-chan struct{}, errc chan<- error) {
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			errc <- ctx.Err()
			return
		case <-done:
			return
		default:
		}

		n, err := sp.Read(buf)
		if n > 0 {
			payload := append([]byte(nil), buf[:n]...)
			if writeErr := ws.writeBinary(payload); writeErr != nil {
				errc <- fmt.Errorf("write relay binary: %w", writeErr)
				return
			}
			log.Printf("[serial->relay] %d bytes", n)
		}
		if err != nil && !errors.Is(err, io.EOF) {
			errc <- fmt.Errorf("read serial: %w", err)
			return
		}
	}
}

func relayToSerial(ctx context.Context, sp *serialport.Port, ws *lockedWS, done <-chan struct{}, errc chan<- error) {
	for {
		select {
		case <-ctx.Done():
			errc <- ctx.Err()
			return
		case <-done:
			return
		default:
		}

		messageType, data, err := ws.conn.ReadMessage()
		if err != nil {
			errc <- fmt.Errorf("read relay: %w", err)
			return
		}

		switch messageType {
		case websocket.BinaryMessage:
			if len(data) == 0 {
				continue
			}
			if _, err := sp.Write(data); err != nil {
				errc <- fmt.Errorf("write serial: %w", err)
				return
			}
			log.Printf("[relay->serial] %d bytes", len(data))
		case websocket.TextMessage:
			if err := handleControl(ws, data); err != nil {
				errc <- err
				return
			}
		}
	}
}

func handleControl(ws *lockedWS, data []byte) error {
	var msg ControlMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil
	}
	switch msg.Type {
	case "ping":
		return ws.writeJSON(ControlMessage{Type: "pong", Protocol: protocolVersion})
	case "session_ready":
		log.Printf("session ready: %s", msg.SessionID)
	case "session_closed":
		return fmt.Errorf("session closed: %s", msg.Message)
	case "error":
		return fmt.Errorf("relay error: %s", msg.Message)
	}
	return nil
}

func (ws *lockedWS) writeJSON(value any) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.conn.WriteJSON(value)
}

func (ws *lockedWS) writeBinary(data []byte) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.conn.WriteMessage(websocket.BinaryMessage, data)
}

func buildWebSocketURL(server string, secure bool, path string) (string, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return "", fmt.Errorf("server is empty")
	}

	if !strings.Contains(server, "://") {
		scheme := "ws"
		if secure {
			scheme = "wss"
		}
		return (&url.URL{Scheme: scheme, Host: server, Path: path}).String(), nil
	}

	u, err := url.Parse(server)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	basePath := strings.TrimRight(u.Path, "/")
	u.Path = basePath + path
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func parseParity(value string) (serialport.Parity, string, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "", "N", "NONE":
		return serialport.ParityNone, "N", nil
	case "O", "ODD":
		return serialport.ParityOdd, "O", nil
	case "E", "EVEN":
		return serialport.ParityEven, "E", nil
	case "M", "MARK":
		return serialport.ParityMark, "M", nil
	case "S", "SPACE":
		return serialport.ParitySpace, "S", nil
	default:
		return serialport.ParityNone, "", fmt.Errorf("unsupported parity %q", value)
	}
}

func parseStopBits(value string) (serialport.StopBits, int, error) {
	switch strings.TrimSpace(value) {
	case "", "1":
		return serialport.Stop1, 1, nil
	case "1.5", "15":
		return serialport.Stop1Half, 15, nil
	case "2":
		return serialport.Stop2, 2, nil
	default:
		return serialport.Stop1, 0, fmt.Errorf("unsupported stop bits %q", value)
	}
}
