# VSP - Virtual Serial Port Cloud Platform

[![Build](https://github.com/wayyoungboy/serialserver/actions/workflows/release.yml/badge.svg)](https://github.com/wayyoungboy/serialserver/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)

VSP (Virtual Serial Port) is a commercial cloud platform for remote serial port access. It enables PLC remote debugging, IoT device management, and industrial automation through bidirectional serial-to-TCP tunneling.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           VSP Cloud Platform                                 │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────┐                    ┌─────────────────────────────────────┐
│    Device Side   │                    │           Cloud Server              │
│  (Factory Site)  │                    │         (vsp-server)                │
├─────────────────┤                    ├─────────────────────────────────────┤
│  [Serial Device] │                    │  ┌─────────────────────────────┐    │
│  PLC/Sensors    │                    │  │   Web Management Console    │    │
│       │         │                    │  │   Device/User Management    │    │
│       ▼         │                    │  └─────────────────────────────┘    │
│  device-client │◄──── WebSocket ─────│                                      │
│  (Go Client)    │     Port 9000       │  ┌─────────────────────────────┐    │
│  DeviceKey Auth │                    │  │   REST API / WebSocket      │    │
│                 │                    │  └─────────────────────────────┘    │
└─────────────────┘                    └──────────────────┬──────────────────┘
                                                          │
                                                          │ WebSocket
                                                          │
                                       ┌──────────────────▼──────────────────┐
                                       │           Windows Client             │
                                       │         (vsp-windows)               │
                                       ├─────────────────────────────────────┤
                                       │  [Debug Software]                   │
                                       │  Serial Tools / SCADA / PLC IDE     │
                                       │       │                             │
                                       │       ▼                             │
                                       │  VSPManager (Go+Wails)              │
                                       │  + com0com Virtual Serial Driver    │
                                       │  Virtual COM Port                   │
                                       └─────────────────────────────────────┘
```

## Components

| Component | Language | Location | Purpose |
|-----------|----------|----------|---------|
| **vsp-server** | Go | `vsp-server/` | Cloud server with REST API, WebSocket relay, multi-tenant management |
| **vsp-client** | Go | `vsp-client/` | Device-side client (reads physical serial port, uploads to server) |
| **vsp-windows** | Go + Wails | `vsp-windows/` | Windows GUI client with com0com virtual serial port support |
| **com0com** | C++ | `com0com/` | Open-source virtual serial port driver |

## Features

### vsp-server (Cloud Server)

- **User Management**: Registration, login, JWT authentication
- **Device Management**: Add devices, generate DeviceKey, device status monitoring
- **Multi-tenancy**: Tenant isolation, quota management
- **WebSocket**: Real-time bidirectional data relay
- **REST API**: Complete API interface
- **Web Console**: Dashboard, device management

### vsp-client (Device Side)

- Physical serial port reading
- Active connection to cloud server
- DeviceKey authentication
- Fetch serial port configuration from server
- Auto-reconnect on disconnect

### vsp-windows (Windows GUI)

- Modern GUI built with Wails + Vue.js
- Automatic virtual COM port creation
- Bidirectional data forwarding
- Connection status monitoring
- HTTP/HTTPS support

## Quick Start

### 1. Start Cloud Server

```bash
cd vsp-server
go build -o vsp-server ./cmd
./vsp-server
```

After server starts:
- Web Console: `http://localhost:9000`
- REST API: `http://localhost:9000/api/v1`
- Default Admin: `admin` / `admin123`

### 2. Create Device

Create a device through Web Console or API to get DeviceKey.

### 3. Start Device Client

```bash
cd vsp-client
go build -o device-client ./cmd/device-client
./device-client -server your-server:9000 -key <device_key>
```

### 4. Start Windows Client

Double-click `VSPManager.exe`, enter server address, login and select device to connect.

## Build

### Local Build

```bash
# Build all components
make all

# Build individually
make build-server     # Server
make build-client     # Device client (cross-platform)
make build-windows    # Windows GUI client

# Package for release
make package
```

### Windows GUI Build (Requires Wails)

```powershell
cd vsp-windows
wails build -clean
```

## API Documentation

### Authentication

```
POST /api/v1/auth/register   # User registration
POST /api/v1/auth/login      # User login (returns JWT Token)
```

### Devices

```
GET    /api/v1/devices           # Device list
POST   /api/v1/devices           # Create device
DELETE /api/v1/devices/:id       # Delete device
GET    /api/v1/devices/config?device_key=xxx  # Get config (device client uses)
```

### WebSocket

```
WS /api/v1/ws/device   # Device side connection
WS /api/v1/ws/client   # Windows client connection
```

## Project Structure

```
serialserver/
├── .github/workflows/      # GitHub Actions CI/CD
├── vsp-server/             # Cloud server
│   ├── cmd/main.go
│   ├── internal/
│   └── configs/
├── vsp-client/             # Device client
│   ├── cmd/device-client/
│   └── internal/
├── vsp-windows/            # Windows GUI client
│   ├── main.go
│   ├── app.go
│   ├── frontend/           # Vue.js frontend
│   ├── internal/
│   └── wails.json
├── com0com/                # Virtual serial port driver
├── tests/                  # Test scripts
├── Makefile
└── README.md
```

## Data Flow

```
[Physical Device] ↔ [device-client] ↔ [WebSocket] ↔ [vsp-server] ↔ [WebSocket] ↔ [VSPManager] ↔ [Virtual COM] ↔ [Debug Software]
```

## Requirements

- **Go 1.25+**
- **Node.js 20+** (for Windows GUI build)
- **Wails CLI** (for Windows GUI build)
- **com0com** (Windows virtual serial port driver)

## License

MIT License