package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"vsp-client/internal/config"
	"vsp-client/internal/serial"
	"vsp-client/internal/tcp"
)

type Server struct {
	http.Server
	configMgr  *config.Manager
	serialMgr  *serial.PortManager
	tunnelMgr  *tcp.Manager
	logger     *Logger
	mu         sync.RWMutex
}

type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewServer(port int, cfg *config.Manager, sm *serial.PortManager, tm *tcp.Manager, logger *Logger) *Server {
	mux := http.NewServeMux()
	s := &Server{
		configMgr: cfg,
		serialMgr: sm,
		tunnelMgr: tm,
		logger:    logger,
	}

	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/ports", s.handlePorts)
	mux.HandleFunc("/api/tunnels", s.handleTunnels)
	mux.HandleFunc("/api/tunnel/create", s.handleCreateTunnel)
	mux.HandleFunc("/api/tunnel/delete", s.handleDeleteTunnel)
	mux.HandleFunc("/api/tunnel/start", s.handleStartTunnel)
	mux.HandleFunc("/api/tunnel/stop", s.handleStopTunnel)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/stats", s.handleStats)

	s.Addr = fmt.Sprintf(":%d", port)
	s.Handler = mux

	return s
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, APIResponse{Code: 0, Message: "ok", Data: map[string]interface{}{
		"version": "1.0.0",
		"uptime":  time.Now().Unix(),
	}})
}

func (s *Server) handlePorts(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		ports, err := serial.ListAvailable()
		if err != nil {
			s.writeJSON(w, APIResponse{Code: 1, Message: err.Error()})
			return
		}
		s.writeJSON(w, APIResponse{Code: 0, Message: "ok", Data: ports})
		return
	}
	s.writeJSON(w, APIResponse{Code: 1, Message: "method not allowed"})
}

func (s *Server) handleTunnels(w http.ResponseWriter, r *http.Request) {
	tunnels := s.configMgr.Get().Tunnels
	tunnelNames := s.tunnelMgr.ListTunnels()

	type TunnelStatus struct {
		Name     string               `json:"name"`
		Mode     string               `json:"mode"`
		Running  bool                 `json:"running"`
		Serial   config.SerialConfig  `json:"serial"`
		TCP      config.TCPConfig     `json:"tcp"`
		Enabled  bool                 `json:"enabled"`
		TCPAddr  string               `json:"tcp_addr,omitempty"`
	}

	var result []TunnelStatus
	for _, t := range tunnels {
		running := false
		var tcpAddr string
		for _, name := range tunnelNames {
			if name == t.Name {
				running = true
				if tnl, ok := s.tunnelMgr.GetTunnel(t.Name); ok {
					tcpAddr = tnl.GetTCPAddress()
				}
				break
			}
		}
		result = append(result, TunnelStatus{
			Name:    t.Name,
			Mode:    t.Mode,
			Running: running,
			Serial:  t.Serial,
			TCP:     t.TCP,
			Enabled: t.Enabled,
			TCPAddr: tcpAddr,
		})
	}

	s.writeJSON(w, APIResponse{Code: 0, Message: "ok", Data: result})
}

func (s *Server) handleCreateTunnel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, APIResponse{Code: 1, Message: "method not allowed"})
		return
	}

	var t config.TunnelConfig
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		s.writeJSON(w, APIResponse{Code: 1, Message: err.Error()})
		return
	}

	if err := s.configMgr.AddTunnel(t); err != nil {
		s.writeJSON(w, APIResponse{Code: 1, Message: err.Error()})
		return
	}

	s.logger.Log("tunnel", fmt.Sprintf("Created tunnel: %s", t.Name))
	s.writeJSON(w, APIResponse{Code: 0, Message: "ok"})
}

func (s *Server) handleDeleteTunnel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, APIResponse{Code: 1, Message: "method not allowed"})
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		s.writeJSON(w, APIResponse{Code: 1, Message: "name is required"})
		return
	}

	s.tunnelMgr.Stop(name)
	if err := s.configMgr.RemoveTunnel(name); err != nil {
		s.writeJSON(w, APIResponse{Code: 1, Message: err.Error()})
		return
	}

	s.logger.Log("tunnel", fmt.Sprintf("Deleted tunnel: %s", name))
	s.writeJSON(w, APIResponse{Code: 0, Message: "ok"})
}

func (s *Server) handleStartTunnel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, APIResponse{Code: 1, Message: "method not allowed"})
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		s.writeJSON(w, APIResponse{Code: 1, Message: "name is required"})
		return
	}

	cfg := s.configMgr.Get()
	var tunnelCfg config.TunnelConfig
	for _, t := range cfg.Tunnels {
		if t.Name == name {
			tunnelCfg = t
			break
		}
	}

	if tunnelCfg.Name == "" {
		s.writeJSON(w, APIResponse{Code: 1, Message: "tunnel not found"})
		return
	}

	sp, err := s.serialMgr.Open(tunnelCfg.Serial)
	if err != nil {
		s.writeJSON(w, APIResponse{Code: 1, Message: err.Error()})
		return
	}

	addr := fmt.Sprintf("%s:%d", tunnelCfg.TCP.Host, tunnelCfg.TCP.Port)
	var tnl *tcp.Tunnel
	switch tunnelCfg.Mode {
	case "client":
		tnl, err = s.tunnelMgr.StartClient(tunnelCfg.Name, sp, addr)
	case "server":
		tnl, err = s.tunnelMgr.StartServer(tunnelCfg.Name, sp, addr)
	case "tunnel":
		tnl, err = s.tunnelMgr.StartTunnel(tunnelCfg.Name, sp, addr)
	default:
		s.writeJSON(w, APIResponse{Code: 1, Message: "unknown mode"})
		return
	}

	if err != nil {
		s.writeJSON(w, APIResponse{Code: 1, Message: err.Error()})
		return
	}

	tnl.SetDataCallback(func(data []byte) {
		s.logger.Log("data", fmt.Sprintf("[%s] %s", name, string(data)))
	})

	s.logger.Log("tunnel", fmt.Sprintf("Started tunnel: %s", name))
	s.writeJSON(w, APIResponse{Code: 0, Message: "ok"})
}

func (s *Server) handleStopTunnel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, APIResponse{Code: 1, Message: "method not allowed"})
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		s.writeJSON(w, APIResponse{Code: 1, Message: "name is required"})
		return
	}

	if err := s.tunnelMgr.Stop(name); err != nil {
		s.writeJSON(w, APIResponse{Code: 1, Message: err.Error()})
		return
	}

	s.logger.Log("tunnel", fmt.Sprintf("Stopped tunnel: %s", name))
	s.writeJSON(w, APIResponse{Code: 0, Message: "ok"})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.writeJSON(w, APIResponse{Code: 0, Message: "ok", Data: s.configMgr.Get()})
		return
	}

	if r.Method == http.MethodPost {
		var cfg config.AppConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			s.writeJSON(w, APIResponse{Code: 1, Message: err.Error()})
			return
		}
		s.configMgr.Set(&cfg)
		s.configMgr.Save()
		s.writeJSON(w, APIResponse{Code: 0, Message: "ok"})
		return
	}

	s.writeJSON(w, APIResponse{Code: 1, Message: "method not allowed"})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	s.writeJSON(w, APIResponse{Code: 0, Message: "ok", Data: s.logger.GetLogs(limit)})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	tunnelNames := s.tunnelMgr.ListTunnels()
	type TunnelStats struct {
		Name   string            `json:"name"`
		Stats  map[string]uint64 `json:"stats"`
	}

	var result []TunnelStats
	for _, name := range tunnelNames {
		if stats, ok := s.serialMgr.GetStats(name); ok {
			result = append(result, TunnelStats{
				Name:   name,
				Stats:  map[string]uint64{"txBytes": stats.TxBytes, "rxBytes": stats.RxBytes},
			})
		}
	}

	s.writeJSON(w, APIResponse{Code: 0, Message: "ok", Data: result})
}

func (s *Server) writeJSON(w http.ResponseWriter, resp APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) Start() error {
	log.Printf("API server starting on %s", s.Addr)
	return s.ListenAndServe()
}
