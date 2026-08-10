package relayv2

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"vsp-server/internal/models"
	"vsp-server/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	ProtocolVersion = "vsp.relay.v2"

	roleDevice  = "device"
	roleGateway = "gateway"

	defaultMappingID = "default"
	writeWait        = 10 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Hub struct {
	mu            sync.RWMutex
	devices       map[endpointKey]*devicePeer
	sessions      map[endpointKey]*session
	deviceService *services.DeviceService
	authService   *services.AuthService
	logService    *services.LogService
}

type endpointKey struct {
	deviceID  uint
	mappingID string
}

type peer struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (p *peer) writeJSON(v any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return p.conn.WriteJSON(v)
}

func (p *peer) writeBinary(data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return p.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (p *peer) close() {
	_ = p.conn.Close()
}

type devicePeer struct {
	peer
	record  *models.Device
	mapping Mapping
	joined  time.Time
}

type gatewayPeer struct {
	peer
	userID uint
	joined time.Time
}

type session struct {
	mu           sync.Mutex
	key          endpointKey
	device       *devicePeer
	gateway      *gatewayPeer
	started      time.Time
	bytesToDev   int64
	bytesToGate  int64
	sessionLabel string
}

func (s *session) addBytesToDevice(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bytesToDev += int64(n)
}

func (s *session) addBytesToGateway(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bytesToGate += int64(n)
}

func (s *session) byteTotals() (int64, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bytesToDev, s.bytesToGate
}

type ControlMessage struct {
	Type      string         `json:"type"`
	Protocol  string         `json:"protocol,omitempty"`
	Role      string         `json:"role,omitempty"`
	DeviceKey string         `json:"device_key,omitempty"`
	DeviceID  uint           `json:"device_id,omitempty"`
	UserToken string         `json:"user_token,omitempty"`
	MappingID string         `json:"mapping_id,omitempty"`
	Mapping   *Mapping       `json:"mapping,omitempty"`
	Status    string         `json:"status,omitempty"`
	Message   string         `json:"message,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Mappings  []MappingState `json:"mappings,omitempty"`
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

type MappingState struct {
	Mapping Mapping `json:"mapping"`
	Online  bool    `json:"online"`
	Busy    bool    `json:"busy"`
}

func NewHub(deviceService *services.DeviceService, authService *services.AuthService, logService *services.LogService) *Hub {
	return &Hub{
		devices:       make(map[endpointKey]*devicePeer),
		sessions:      make(map[endpointKey]*session),
		deviceService: deviceService,
		authService:   authService,
		logService:    logService,
	}
}

func (h *Hub) HandleDevice(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[relay-v2] device upgrade failed: %v", err)
		return
	}

	first, err := readControl(conn)
	if err != nil {
		writeControlAndClose(conn, "error", "invalid hello")
		return
	}
	if first.Type != "hello" || first.Protocol != ProtocolVersion || first.Role != roleDevice {
		writeControlAndClose(conn, "error", "device hello required")
		return
	}
	if first.DeviceKey == "" {
		writeControlAndClose(conn, "error", "device_key required")
		return
	}

	device, err := h.deviceService.GetDeviceByKey(first.DeviceKey)
	if err != nil {
		writeControlAndClose(conn, "error", "invalid device_key")
		return
	}
	if device.Status == "disabled" {
		writeControlAndClose(conn, "error", "device disabled")
		return
	}

	mapping := normalizeMapping(first.Mapping, first.MappingID)
	if mapping.Serial.Port == "" {
		writeControlAndClose(conn, "error", "mapping.serial.port required")
		return
	}

	key := endpointKey{deviceID: device.ID, mappingID: mapping.ID}
	dp := &devicePeer{
		peer:    peer{conn: conn},
		record:  device,
		mapping: mapping,
		joined:  time.Now(),
	}

	var replaced *devicePeer
	h.mu.Lock()
	if existing := h.devices[key]; existing != nil {
		replaced = existing
	}
	h.devices[key] = dp
	h.mu.Unlock()
	if replaced != nil {
		_ = replaced.writeJSON(ControlMessage{Type: "error", Message: "device connection replaced"})
		replaced.close()
	}

	_ = h.deviceService.UpdateDeviceStatus(first.DeviceKey, "online")
	_ = h.logService.Log(device.TenantID, device.ID, 0, "v2_device_connect", fmt.Sprintf("mapping=%s serial=%s", mapping.ID, mapping.Serial.Port))
	_ = dp.writeJSON(ControlMessage{Type: "auth", Protocol: ProtocolVersion, Status: "ok", Message: "device registered"})

	log.Printf("[relay-v2] device online device=%d mapping=%s serial=%s", device.ID, mapping.ID, mapping.Serial.Port)
	h.readLoop(key, roleDevice, &dp.peer)
	h.deviceDisconnected(key, dp)
}

func (h *Hub) HandleGateway(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[relay-v2] gateway upgrade failed: %v", err)
		return
	}

	first, err := readControl(conn)
	if err != nil {
		writeControlAndClose(conn, "error", "invalid hello")
		return
	}
	if first.Type != "hello" || first.Protocol != ProtocolVersion || first.Role != roleGateway {
		writeControlAndClose(conn, "error", "gateway hello required")
		return
	}

	device, userID, err := h.authorizeGateway(first)
	if err != nil {
		writeControlAndClose(conn, "error", err.Error())
		return
	}

	mappingID := first.MappingID
	if mappingID == "" {
		mappingID = defaultMappingID
	}
	key := endpointKey{deviceID: device.ID, mappingID: mappingID}
	gp := &gatewayPeer{peer: peer{conn: conn}, userID: userID, joined: time.Now()}

	h.mu.Lock()
	dp := h.devices[key]
	if dp == nil {
		h.mu.Unlock()
		writeControlAndClose(conn, "error", "mapping offline")
		return
	}
	if h.sessions[key] != nil {
		h.mu.Unlock()
		writeControlAndClose(conn, "error", "mapping busy")
		return
	}
	sess := &session{
		key:          key,
		device:       dp,
		gateway:      gp,
		started:      time.Now(),
		sessionLabel: fmt.Sprintf("%d/%s/%d", device.ID, mappingID, time.Now().UnixNano()),
	}
	h.sessions[key] = sess
	h.mu.Unlock()

	_ = h.logService.Log(device.TenantID, device.ID, userID, "v2_gateway_connect", fmt.Sprintf("mapping=%s", mappingID))
	ready := ControlMessage{
		Type:      "session_ready",
		Protocol:  ProtocolVersion,
		Status:    "ok",
		SessionID: sess.sessionLabel,
		Mapping:   &dp.mapping,
	}
	_ = gp.writeJSON(ready)
	_ = dp.writeJSON(ready)

	log.Printf("[relay-v2] session ready device=%d mapping=%s user=%d", device.ID, mappingID, userID)
	h.readLoop(key, roleGateway, &gp.peer)
	h.gatewayDisconnected(key, gp)
}

func (h *Hub) ListMappings(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device id"})
		return
	}

	userID := c.GetUint("user_id")
	tenantID := c.GetUint("tenant_id")
	role := c.GetString("role")

	device, err := h.deviceService.GetDevice(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	if device.TenantID != tenantID || (role != "admin" && device.UserID != userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	var mappings []MappingState
	for key, dp := range h.devices {
		if key.deviceID != device.ID {
			continue
		}
		mappings = append(mappings, MappingState{
			Mapping: dp.mapping,
			Online:  true,
			Busy:    h.sessions[key] != nil,
		})
	}
	sort.Slice(mappings, func(i, j int) bool {
		return mappings[i].Mapping.ID < mappings[j].Mapping.ID
	})

	c.JSON(http.StatusOK, gin.H{"data": mappings})
}

func (h *Hub) readLoop(key endpointKey, role string, source *peer) {
	for {
		messageType, data, err := source.conn.ReadMessage()
		if err != nil {
			return
		}

		switch messageType {
		case websocket.BinaryMessage:
			h.forwardBinary(key, role, data)
		case websocket.TextMessage:
			h.handleControlFrame(source, data)
		}
	}
}

func (h *Hub) forwardBinary(key endpointKey, fromRole string, data []byte) {
	h.mu.RLock()
	sess := h.sessions[key]
	h.mu.RUnlock()
	if sess == nil {
		return
	}

	if fromRole == roleDevice {
		if err := sess.gateway.writeBinary(data); err == nil {
			sess.addBytesToGateway(len(data))
		}
		return
	}

	if err := sess.device.writeBinary(data); err == nil {
		sess.addBytesToDevice(len(data))
	}
}

func (h *Hub) handleControlFrame(source *peer, data []byte) {
	var msg ControlMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}
	switch msg.Type {
	case "ping":
		_ = source.writeJSON(ControlMessage{Type: "pong", Protocol: ProtocolVersion})
	case "pong":
		return
	}
}

func (h *Hub) deviceDisconnected(key endpointKey, dp *devicePeer) {
	var sess *session
	wasCurrent := false
	h.mu.Lock()
	if h.devices[key] == dp {
		delete(h.devices, key)
		wasCurrent = true
	}
	if h.sessions[key] != nil && h.sessions[key].device == dp {
		sess = h.sessions[key]
		delete(h.sessions, key)
	}
	h.mu.Unlock()

	if wasCurrent {
		_ = h.deviceService.UpdateDeviceStatus(dp.record.DeviceKey, "offline")
		_ = h.logService.Log(dp.record.TenantID, dp.record.ID, 0, "v2_device_disconnect", fmt.Sprintf("mapping=%s", key.mappingID))
	}
	if sess != nil {
		_ = sess.gateway.writeJSON(ControlMessage{Type: "session_closed", Protocol: ProtocolVersion, Message: "device disconnected"})
		sess.gateway.close()
	}
	log.Printf("[relay-v2] device offline device=%d mapping=%s", dp.record.ID, key.mappingID)
}

func (h *Hub) gatewayDisconnected(key endpointKey, gp *gatewayPeer) {
	var sess *session
	h.mu.Lock()
	if h.sessions[key] != nil && h.sessions[key].gateway == gp {
		sess = h.sessions[key]
		delete(h.sessions, key)
	}
	h.mu.Unlock()

	if sess != nil {
		bytesToDev, bytesToGate := sess.byteTotals()
		_ = h.logService.Log(sess.device.record.TenantID, sess.device.record.ID, gp.userID, "v2_gateway_disconnect", fmt.Sprintf("mapping=%s to_device=%d to_gateway=%d", key.mappingID, bytesToDev, bytesToGate))
		_ = sess.device.writeJSON(ControlMessage{Type: "session_closed", Protocol: ProtocolVersion, Message: "gateway disconnected"})
	}
	log.Printf("[relay-v2] gateway offline device=%d mapping=%s", key.deviceID, key.mappingID)
}

func (h *Hub) authorizeGateway(msg ControlMessage) (*models.Device, uint, error) {
	if msg.DeviceID == 0 {
		return nil, 0, fmt.Errorf("device_id required")
	}
	if msg.UserToken == "" {
		return nil, 0, fmt.Errorf("user_token required")
	}

	device, err := h.deviceService.GetDevice(msg.DeviceID)
	if err != nil {
		return nil, 0, fmt.Errorf("device not found")
	}
	if device.Status == "disabled" {
		return nil, 0, fmt.Errorf("device disabled")
	}

	claims, err := h.authService.ValidateToken(msg.UserToken)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid user_token")
	}
	if claims.TenantID != device.TenantID || (claims.Role != "admin" && claims.UserID != device.UserID) {
		return nil, 0, fmt.Errorf("forbidden")
	}
	return device, claims.UserID, nil
}

func normalizeMapping(mapping *Mapping, fallbackID string) Mapping {
	if mapping == nil {
		mapping = &Mapping{}
	}
	if mapping.ID == "" {
		mapping.ID = fallbackID
	}
	if mapping.ID == "" {
		mapping.ID = defaultMappingID
	}
	if mapping.Name == "" {
		mapping.Name = mapping.ID
	}
	if mapping.Serial.BaudRate == 0 {
		mapping.Serial.BaudRate = 115200
	}
	if mapping.Serial.DataBits == 0 {
		mapping.Serial.DataBits = 8
	}
	if mapping.Serial.StopBits == 0 {
		mapping.Serial.StopBits = 1
	}
	if mapping.Serial.Parity == "" {
		mapping.Serial.Parity = "N"
	}
	return *mapping
}

func readControl(conn *websocket.Conn) (ControlMessage, error) {
	messageType, data, err := conn.ReadMessage()
	if err != nil {
		return ControlMessage{}, err
	}
	if messageType != websocket.TextMessage {
		return ControlMessage{}, fmt.Errorf("expected text control frame")
	}
	var msg ControlMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return ControlMessage{}, err
	}
	return msg, nil
}

func writeControlAndClose(conn *websocket.Conn, typ, message string) {
	_ = conn.WriteJSON(ControlMessage{Type: typ, Protocol: ProtocolVersion, Message: message})
	_ = conn.Close()
}
