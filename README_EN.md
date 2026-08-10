# VSP - Virtual Serial Port Cloud Platform

**English** | **[中文](README.md)**

[![Build](https://github.com/wayyoungboy/serialserver/actions/workflows/release.yml/badge.svg)](https://github.com/wayyoungboy/serialserver/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)

VSP (Virtual Serial Port) is a remote serial gateway for PLC debugging, IoT device management, and industrial automation. The main branch is now V2-only: field devices configure their own serial settings, while the cloud server only authenticates, authorizes, pairs relay sessions, forwards binary frames, and records audit logs.

## Documentation

| Document | Audience | Contents |
|----------|----------|----------|
| [User Manual](docs/user-manual-en.md) | Administrators, field operators, desktop users | Device creation, field agent setup, Windows GUI, CLI gateway, and troubleshooting |
| [V2 Relay Protocol](docs/v2-relay.md) | Developers and integrators | WebSocket hello messages, mapping state, and binary forwarding rules |
| [Windows Release Checklist](docs/windows-release-checklist.md) | Release owners and testers | Icons, Wails build, NSIS installer, install/start/uninstall validation |
| [Test Notes](tests/README.md) | Developers and CI maintainers | Unit tests and Linux pseudo-terminal serial relay E2E |

## Architecture

```
[Serial device] <-> [device-agent-v2] <-> [vsp-server V2 relay] <-> [VSPManager / desktop-gateway-v2] <-> [127.0.0.1:PORT] <-> [debug tool]
```

The first V2 release exposes a local TCP endpoint such as `127.0.0.1:7000`. Virtual COM support is reserved for a later gateway adapter.

## Components

| Component | Language | Location | Purpose |
|-----------|----------|----------|---------|
| **vsp-server** | Go | `vsp-server/` | V2 control plane, device identity, user auth, relay pairing, binary forwarding, and audit logs |
| **device-agent-v2** | Go | `vsp-client/cmd/device-agent-v2/` | Runs at the field site, opens the physical serial port, and connects to `/api/v2/relay/device` with DeviceKey |
| **desktop-gateway-v2** | Go | `vsp-client/cmd/desktop-gateway-v2/` | Creates a local TCP endpoint and connects to `/api/v2/relay/gateway` with a user JWT |
| **vsp-windows** | Go + Wails | `vsp-windows/` | Windows GUI for login, device mapping selection, local TCP gateway control, and bilingual UI |

## Quick Start

See the [User Manual](docs/user-manual-en.md) for full step-by-step usage. The minimum flow is:

### 1. Start the Server

```bash
cd vsp-server
go build -o vsp-server ./cmd
./vsp-server
```

After startup:

- Web console: `http://localhost:9000`
- REST API: `http://localhost:9000/api/v2`
- Default admin: `admin` / `admin123`

### 2. Create a Device

Create a device from the web console or API. The generated DeviceKey is only for the field-side `device-agent-v2`.

### 3. Start the Field Agent

```bash
cd vsp-client
go build -o device-agent-v2 ./cmd/device-agent-v2
./device-agent-v2 \
  -server localhost:9000 \
  -key <device_key> \
  -mapping plc \
  -name "PLC" \
  -port COM3 \
  -baud 9600
```

Serial settings are local to the field agent and are announced in the V2 hello message.

### 4. Start the Desktop Gateway

Use VSPManager, or run the CLI gateway after logging in and obtaining a user JWT:

```bash
cd vsp-client
go build -o desktop-gateway-v2 ./cmd/desktop-gateway-v2
./desktop-gateway-v2 \
  -server localhost:9000 \
  -token <user_jwt> \
  -device-id 1 \
  -mapping plc \
  -listen 127.0.0.1:7000
```

Desktop access requires a user login token and a device ID. DeviceKey is not accepted as a remote-access password.

Then point your desktop tool at `127.0.0.1:7000`.

## API

### Authentication

```text
POST /api/v2/auth/register
POST /api/v2/auth/login
```

### Devices

```text
GET    /api/v2/devices
POST   /api/v2/devices
GET    /api/v2/devices/:id
PUT    /api/v2/devices/:id
DELETE /api/v2/devices/:id
POST   /api/v2/devices/:id/regenerate-key
GET    /api/v2/devices/:id/mappings
```

Device REST APIs manage cloud-side identity and metadata only. Serial parameters are not stored as server-controlled device configuration.

### Relay

```text
WS /api/v2/relay/device
WS /api/v2/relay/gateway
```

See [docs/v2-relay.md](docs/v2-relay.md) for the V2 hello messages, mapping metadata, and binary frame behavior.

## Build

```bash
make build-server
make build-client
make build-windows
```

Generate the Windows app icon assets and build a standard per-user NSIS installer:

```powershell
cd vsp-windows
go run tools/gen_windows_assets.go
wails build -clean
makensis /DAPP_VERSION=0.2.0 packaging/windows/VSPManager.nsi
```

The installer is written to `vsp-windows/build/installer/` and creates Start Menu, desktop shortcut, and uninstall entries.

Use [docs/windows-release-checklist.md](docs/windows-release-checklist.md) before release to verify icons, Wails build output, the NSIS installer, install/start behavior, and uninstall cleanup.

Validation commands:

```bash
cd vsp-server && go test ./...
cd vsp-client && go test ./...
cd vsp-windows && go test ./...
cd vsp-windows/frontend && npm run build
cd tests/e2e && go test ./...
```

`tests/e2e` runs on Linux and uses a pseudo-terminal to simulate a serial device. It starts a real `vsp-server`, `device-agent-v2`, and `desktop-gateway-v2`, then verifies TCP-to-serial and serial-to-TCP byte forwarding.

## Project Structure

```text
serialserver/
├── vsp-server/             # Cloud server and V2 relay
├── vsp-client/             # device-agent-v2 and desktop-gateway-v2
├── vsp-windows/            # Windows GUI gateway
├── docs/user-manual.md     # Chinese user manual
├── docs/user-manual-en.md  # English user manual
├── docs/v2-relay.md        # V2 relay protocol
├── docs/windows-release-checklist.md
├── tests/                  # Test helpers and Linux PTY E2E
├── Makefile
└── README.md
```

## Requirements

- Go 1.25+
- Node.js 20+ for the Windows GUI frontend
- Wails CLI for building the Windows GUI
- NSIS for building the Windows installer

## License

[MIT License](LICENSE)
