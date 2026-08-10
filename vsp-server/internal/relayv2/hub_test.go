package relayv2

import (
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"vsp-server/internal/database"
	"vsp-server/internal/models"
	"vsp-server/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestHubPairsDeviceAndGatewayWithBinaryFrames(t *testing.T) {
	server, device, userToken := newRelayTestServer(t)
	defer server.Close()

	deviceConn := dialRelay(t, server.URL, "/api/v2/relay/device")
	defer deviceConn.Close()

	err := deviceConn.WriteJSON(ControlMessage{
		Type:      "hello",
		Protocol:  ProtocolVersion,
		Role:      roleDevice,
		DeviceKey: device.DeviceKey,
		Mapping: &Mapping{
			ID:   "plc",
			Name: "PLC",
			Serial: SerialSettings{
				Port:     "loopback-test",
				BaudRate: 9600,
				DataBits: 7,
				StopBits: 1,
				Parity:   "E",
			},
		},
	})
	if err != nil {
		t.Fatalf("device hello: %v", err)
	}
	readControlMessage(t, deviceConn, "auth")

	gatewayConn := dialRelay(t, server.URL, "/api/v2/relay/gateway")
	defer gatewayConn.Close()

	err = gatewayConn.WriteJSON(ControlMessage{
		Type:      "hello",
		Protocol:  ProtocolVersion,
		Role:      roleGateway,
		DeviceID:  device.ID,
		UserToken: userToken,
		MappingID: "plc",
	})
	if err != nil {
		t.Fatalf("gateway hello: %v", err)
	}
	readControlMessage(t, gatewayConn, "session_ready")
	readControlMessage(t, deviceConn, "session_ready")

	fromGateway := []byte("hello-device")
	if err := gatewayConn.WriteMessage(websocket.BinaryMessage, fromGateway); err != nil {
		t.Fatalf("write gateway binary: %v", err)
	}
	assertBinaryMessage(t, deviceConn, fromGateway)

	fromDevice := []byte("hello-gateway")
	if err := deviceConn.WriteMessage(websocket.BinaryMessage, fromDevice); err != nil {
		t.Fatalf("write device binary: %v", err)
	}
	assertBinaryMessage(t, gatewayConn, fromDevice)
}

func newRelayTestServer(t *testing.T) (*httptest.Server, *models.Device, string) {
	t.Helper()

	if err := database.Init(filepath.Join(t.TempDir(), "relay-v2-test.db")); err != nil {
		t.Fatalf("init database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})
	if err := database.CreateDefaultData(); err != nil {
		t.Fatalf("create default data: %v", err)
	}

	var admin models.User
	if err := database.DB.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("load admin user: %v", err)
	}

	deviceService := services.NewDeviceService()
	authService := services.NewAuthService("relay-v2-test-secret", 24)
	logService := services.NewLogService()

	device, err := deviceService.CreateDevice(admin.ID, admin.TenantID, "test-device")
	if err != nil {
		t.Fatalf("create device: %v", err)
	}
	token, err := authService.GenerateToken(&admin)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	hub := NewHub(deviceService, authService, logService)
	engine.GET("/api/v2/relay/device", hub.HandleDevice)
	engine.GET("/api/v2/relay/gateway", hub.HandleGateway)

	return httptest.NewServer(engine), device, token
}

func dialRelay(t *testing.T, baseURL, path string) *websocket.Conn {
	t.Helper()

	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	u.Scheme = "ws"
	u.Path = path

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial %s: %v", path, err)
	}
	return conn
}

func readControlMessage(t *testing.T, conn *websocket.Conn, wantType string) ControlMessage {
	t.Helper()

	var msg ControlMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read control %s: %v", wantType, err)
	}
	if msg.Type != wantType {
		t.Fatalf("control type = %q, want %q: %+v", msg.Type, wantType, msg)
	}
	return msg
}

func assertBinaryMessage(t *testing.T, conn *websocket.Conn, want []byte) {
	t.Helper()

	messageType, got, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if messageType != websocket.BinaryMessage {
		t.Fatalf("message type = %d, want binary", messageType)
	}
	if string(got) != string(want) {
		t.Fatalf("binary payload = %q, want %q", string(got), string(want))
	}
}
