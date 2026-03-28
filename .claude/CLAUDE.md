# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

VSP (Virtual Serial Port) cloud platform for remote serial port access. Enables PLC remote debugging, IoT device management, and industrial automation through bidirectional serial-to-TCP tunneling.

## Components

| Component | Language | Directory | Purpose |
|-----------|----------|-----------|---------|
| **vsp-server** | Go | `vsp-server/` | Cloud server with REST API, WebSocket relay, multi-tenant management |
| **vsp-client** | Go | `vsp-client/` | Device-side client (reads physical serial port, uploads to server) |
| **vsp-windows** | C# (.NET 8) | `vsp-windows/` | Windows client with com0com virtual serial port support |

## Build Commands

```bash
# Go components
cd vsp-server && go build -o vsp-server ./cmd
cd vsp-client && go build -o vsp-client ./cmd
cd vsp-client && go build -o device-client ./cmd/device-client  # ARM/Linux device client

# Windows WPF application
cd vsp-windows && dotnet build VSPManager.sln -c Release

# Windows single-file publish
cd vsp-windows/VSPManager && dotnet publish -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true
```

## Run Commands

```bash
# Server (default: port 9000)
./vsp-server
# Access: Web UI at http://localhost:9000, API at http://localhost:9000/api/v1
# Default admin: admin / admin123

# Device client (connects physical serial port to server)
./device-client -server your-server:9000 -key <device_key>

# vsp-client with config file
./vsp-client -config config.json
```

## Architecture

### System Data Flow

```
[Serial Device] ↔ [device-client] ↔ [WebSocket] ↔ [vsp-server] ↔ [WebSocket] ↔ [vsp-windows/VSPManager] ↔ [Virtual COM Port]
```

### vsp-server Structure

- `cmd/main.go` - Entry point, initializes DB, services, routes
- `internal/api/handlers/` - REST API handlers (auth, devices, stats)
- `internal/websocket/` - WebSocket hub for device/client relay
- `internal/database/` - SQLite with GORM (pure Go, no CGO)
- `internal/services/` - Business logic (auth, device, stats, log)

Key dependencies: `gin-gonic/gin`, `gorilla/websocket`, `glebarez/sqlite`, `golang-jwt/jwt`

### vsp-client Structure

- `cmd/main.go` - Main application with config-driven tunnels
- `cmd/device-client/main.go` - Lightweight device client (fetches config from server)
- `internal/serial/` - Serial port management with `tarm/serial`
- `internal/tcp/` - Tunnel manager (client/server/tunnel modes)

### vsp-windows Structure

- `VSPManager/` - WPF GUI application
- `VSPManager.Core/Driver/Com0ComManager.cs` - Manages com0com virtual port pairs
- `VSPManager.Core/Network/VspTcpClient.cs` - TCP connection to vsp-server

**Virtual Port Creation Flow:**
1. `setupc.exe install - -` creates CNCA0/CNCB0 pair
2. `setupc.exe change CNCA0 PortName=COM#` assigns visible COM port (auto-numbered)
3. `setupc.exe change CNCB0 PortName=-` hides the paired port

## WebSocket Protocol

Both device and client authenticate first, then exchange data messages:

```json
// Authentication
{"type": "auth", "payload": {"device_key": "xxx"}}

// Data transfer
{"type": "data", "payload": {"data": [base64 encoded bytes]}}

// Status notification
{"type": "status", "payload": {"status": "device_connected"}}
```

Server (`websocket.go`) maintains a `Hub` that routes messages between `DeviceConn` and `ClientConn` per device.

## Configuration

**vsp-server** (`configs/config.yaml`):
```yaml
server: {host: "0.0.0.0", port: 9000, mode: "release"}
database: {path: "data/vsp.db"}
jwt: {secret: "your-secret", expire_time: 24}
```

**vsp-client** (`config.json`):
```json
{
  "tunnels": [{
    "name": "PLC-1",
    "mode": "tunnel",
    "serial": {"port": "COM3", "baud": 115200, "dataBits": 8, "stopBits": 1, "parity": "N"},
    "tcp": {"host": "192.168.1.100", "port": 9000},
    "enabled": true
  }],
  "ui": {"port": 8080}
}
```

## REST API Endpoints

```
POST /api/v1/auth/login          # Login (returns JWT)
POST /api/v1/auth/register       # Register user
GET  /api/v1/devices             # List devices
POST /api/v1/devices             # Create device (returns device_key)
GET  /api/v1/devices/config?device_key=xxx  # Get device config (device-client uses this)
WS   /api/v1/ws/device           # Device WebSocket
WS   /api/v1/ws/client           # Windows client WebSocket
```

## Platform Notes

- **Windows virtual ports**: Requires com0com driver installed at `Program Files\com0com\` or bundled with app
- **Linux serial access**: User must be in `dialout` group
- **Driver signing**: Windows kernel drivers require test signing mode (`bcdedit /set testsigning on`)