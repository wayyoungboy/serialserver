# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Serial port server system for bidirectional data tunneling between serial ports and TCP networks. Used for PLC remote debugging and IoT device connectivity.

## Components

| Component | Language | Description |
|-----------|----------|-------------|
| `vsp-manager/` | Go | Main application with GUI, API server, Web UI |
| `vsp-server/` | Go | TCP relay server (device ↔ Windows client) |
| `vsp-client/` | Go | Windows client with virtual serial port support |
| `SerialClient.go` | Go | Legacy simple serial-to-TCP client |

## Build Commands

```bash
# Build vsp-manager
cd vsp-manager && go build -o vsp-manager ./cmd

# Build vsp-server
cd vsp-server && go build -o vsp-server ./cmd

# Build vsp-client
cd vsp-client && go build -o vsp-client ./cmd
```

## Run Commands

```bash
# VSP Manager (with Web UI on :8080, API on :8081)
./vsp-manager -config config.json

# VSP Server (TCP relay on :9000)
./vsp-server -port 9000

# VSP Client (virtual serial port, requires com0com on Windows)
./vsp-client -server IP:9000 -port COM5 -baud 115200
```

## Architecture

**VSP Manager** (main component):
- `cmd/main.go` - Entry point, orchestrates all services
- `internal/config/` - Configuration management (JSON/YAML)
- `internal/serial/` - Serial port abstraction with `tarm/serial`
- `internal/tcp/` - Tunnel manager supporting 3 modes:
  - `client`: Serial → TCP (upload data to server)
  - `server`: TCP → Serial (remote access to local serial)
  - `tunnel`: Bidirectional Serial ↔ TCP
- `internal/api/` - HTTP API server
- `internal/web/` - Web UI server

**Data Flow**:
```
[Serial Device] ↔ [vsp-manager/vsp-client] ↔ [TCP Network] ↔ [vsp-server] ↔ [Remote Application]
```

## Configuration

Config file (`config.json`):
```json
{
  "tunnels": [{
    "name": "PLC-连接1",
    "mode": "tunnel",
    "serial": {"port": "COM3", "baud": 115200, "dataBits": 8, "stopBits": 1, "parity": "N"},
    "tcp": {"host": "192.168.1.100", "port": 9000},
    "enabled": true
  }],
  "ui": {"port": 8080}
}
```

## Key Dependencies

- `github.com/tarm/serial` - Serial port communication
- `fyne.io/fyne/v2` - Cross-platform GUI (optional)
- `gopkg.in/yaml.v3` - YAML config support

## Platform Notes

- Windows: Install com0com driver for virtual serial port pairs
- Linux: User needs `dialout` group for serial port access