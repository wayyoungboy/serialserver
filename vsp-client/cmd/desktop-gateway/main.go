package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/gorilla/websocket"
)

const protocolVersion = "vsp.relay"

var (
	serverAddr = flag.String("server", "localhost:9000", "VSP server host or URL")
	userToken  = flag.String("token", "", "user JWT token; can also be set with VSP_TOKEN")
	deviceID   = flag.Uint("device-id", 0, "device id")
	mappingID  = flag.String("mapping", "default", "remote serial mapping id")
	listenAddr = flag.String("listen", "127.0.0.1:7000", "local TCP endpoint for third-party tools")
	secure     = flag.Bool("secure", false, "use wss when server has no scheme")
)

type ControlMessage struct {
	Type      string   `json:"type"`
	Protocol  string   `json:"protocol,omitempty"`
	Role      string   `json:"role,omitempty"`
	DeviceID  uint     `json:"device_id,omitempty"`
	UserToken string   `json:"user_token,omitempty"`
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

type Gateway struct {
	serverURL string
	userToken string
	deviceID  uint
	mappingID string
}

type lockedWS struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func main() {
	flag.Parse()

	token := *userToken
	if token == "" {
		token = os.Getenv("VSP_TOKEN")
	}
	if token == "" {
		log.Fatal("-token or VSP_TOKEN is required")
	}
	if *deviceID == 0 {
		log.Fatal("-device-id is required")
	}

	wsURL, err := buildWebSocketURL(*serverAddr, *secure, "/api/relay/gateway")
	if err != nil {
		log.Fatalf("invalid server: %v", err)
	}

	gateway := Gateway{
		serverURL: wsURL,
		userToken: token,
		deviceID:  uint(*deviceID),
		mappingID: *mappingID,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := gateway.Listen(ctx, *listenAddr); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}

func (g Gateway) Listen(ctx context.Context, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	defer listener.Close()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	log.Printf("local TCP endpoint ready: %s mapping=%s", addr, g.mappingID)
	for {
		localConn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return ctx.Err()
			}
			return fmt.Errorf("accept local connection: %w", err)
		}

		remoteAddr := localConn.RemoteAddr().String()
		log.Printf("local client connected: %s", remoteAddr)
		if err := g.HandleSession(ctx, localConn); err != nil {
			log.Printf("session ended for %s: %v", remoteAddr, err)
		} else {
			log.Printf("session ended for %s", remoteAddr)
		}
	}
}

func (g Gateway) HandleSession(ctx context.Context, localConn net.Conn) error {
	defer localConn.Close()

	conn, _, err := websocket.DefaultDialer.Dial(g.serverURL, nil)
	if err != nil {
		return fmt.Errorf("connect relay: %w", err)
	}
	defer conn.Close()

	ws := &lockedWS{conn: conn}
	if err := ws.writeJSON(ControlMessage{
		Type:      "hello",
		Protocol:  protocolVersion,
		Role:      "gateway",
		DeviceID:  g.deviceID,
		UserToken: g.userToken,
		MappingID: g.mappingID,
	}); err != nil {
		return fmt.Errorf("send hello: %w", err)
	}

	var ready ControlMessage
	if err := conn.ReadJSON(&ready); err != nil {
		return fmt.Errorf("read session response: %w", err)
	}
	if ready.Type == "error" {
		return fmt.Errorf("relay rejected gateway: %s", ready.Message)
	}
	if ready.Type != "session_ready" {
		return fmt.Errorf("unexpected relay response: %+v", ready)
	}
	if ready.Mapping != nil {
		log.Printf("relay session ready: %s serial=%s %d,%s,%d,%d", ready.SessionID, ready.Mapping.Serial.Port, ready.Mapping.Serial.BaudRate, ready.Mapping.Serial.Parity, ready.Mapping.Serial.DataBits, ready.Mapping.Serial.StopBits)
	} else {
		log.Printf("relay session ready: %s", ready.SessionID)
	}

	errc := make(chan error, 2)
	done := make(chan struct{})
	go localToRelay(ctx, localConn, ws, done, errc)
	go relayToLocal(ctx, localConn, ws, done, errc)

	select {
	case err := <-errc:
		close(done)
		return err
	case <-ctx.Done():
		close(done)
		return ctx.Err()
	}
}

func localToRelay(ctx context.Context, localConn net.Conn, ws *lockedWS, done <-chan struct{}, errc chan<- error) {
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

		n, err := localConn.Read(buf)
		if n > 0 {
			payload := append([]byte(nil), buf[:n]...)
			if writeErr := ws.writeBinary(payload); writeErr != nil {
				errc <- fmt.Errorf("write relay binary: %w", writeErr)
				return
			}
			log.Printf("[local->relay] %d bytes", n)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				errc <- fmt.Errorf("local client closed")
				return
			}
			errc <- fmt.Errorf("read local client: %w", err)
			return
		}
	}
}

func relayToLocal(ctx context.Context, localConn net.Conn, ws *lockedWS, done <-chan struct{}, errc chan<- error) {
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
			if _, err := localConn.Write(data); err != nil {
				errc <- fmt.Errorf("write local client: %w", err)
				return
			}
			log.Printf("[relay->local] %d bytes", len(data))
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
