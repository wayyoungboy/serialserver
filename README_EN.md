# VSP - Virtual Serial Port Cloud Platform

**English** | **[中文](README.md)**

[![Build](https://github.com/wayyoungboy/serialserver/actions/workflows/release.yml/badge.svg)](https://github.com/wayyoungboy/serialserver/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)

VSP (Virtual Serial Port) is a remote serial gateway for PLC debugging, IoT device management, and industrial automation. The current architecture keeps serial settings on the field device, while the cloud server authenticates users and devices, pairs relay sessions, forwards binary frames, and records audit logs.

> **First-time clone:** this repo is multiple Go modules. There is **no root `go.mod`**. Run the server with cwd `vsp-server/` so it can load `configs/config.yaml` and `web/dist`. Without a PLC or USB serial adapter, use the no-hardware smoke below.

## Windows GUI Preview

![VSPManager GUI](docs/images/vspmanager-gui.png)

## Documentation

| Document | Audience | Contents |
|----------|----------|----------|
| [User Manual](docs/user-manual-en.md) | Administrators, field operators, desktop users | Device creation, field agent setup, Windows GUI, CLI gateway, and troubleshooting |
| [Relay Protocol](docs/relay-protocol.md) | Developers and integrators | WebSocket hello messages, mapping state, and binary forwarding rules |
| [Windows Release Checklist](docs/windows-release-checklist.md) | Release owners and testers | Icons, Wails build, NSIS installer, install/start/uninstall validation |
| [Test Notes](tests/README.md) | Developers and CI maintainers | Unit tests and Linux pseudo-terminal serial relay E2E |

## Architecture

```
[Serial device] <-> [device-agent] <-> [vsp-server relay] <-> [VSPManager / desktop-gateway] <-> [127.0.0.1:PORT] <-> [debug tool]
```

The first release exposes a local TCP endpoint such as `127.0.0.1:7000`. Virtual COM support is reserved for a later gateway adapter.

## Components

| Component | Language | Location | Purpose |
|-----------|----------|----------|---------|
| **vsp-server** | Go | `vsp-server/` | control plane, device identity, user auth, relay pairing, binary forwarding, and audit logs |
| **device-agent** | Go | `vsp-client/cmd/device-agent/` | Runs at the field site, opens the physical serial port, and connects to `/api/relay/device` with DeviceKey |
| **desktop-gateway** | Go | `vsp-client/cmd/desktop-gateway/` | Creates a local TCP endpoint and connects to `/api/relay/gateway` with a user JWT |
| **vsp-windows** | Go + Wails | `vsp-windows/` | Windows GUI for login, device mapping selection, local TCP gateway control, and bilingual UI |

## Requirements

- **Go 1.25+** (`go 1.25.0` in `vsp-server`, `vsp-client`, and `vsp-windows`)
- Separate modules: do not run `go mod download` / `go build` at the repo root
- Optional: `jq` for the curl examples (or copy `data.token`, `data.id`, `data.device_key` from the JSON)
- Windows GUI also needs Node.js 20+, Wails CLI, and NSIS for the installer

## Quick Start

See the [User Manual](docs/user-manual-en.md) for the full walkthrough. This is the path a stranger can actually complete.

### 0. No-hardware smoke (do this first)

No PLC, USB-serial, or Windows required. On **Linux** this compiles and starts a real `vsp-server`, `device-agent`, and `desktop-gateway`, uses a pseudo-terminal as the serial port, and checks TCP to serial and back:

```bash
cd tests/e2e
go test -v ./...
```

Compile local binaries and print CLI flags that match the source:

```bash
make smoke
```

The server has no `-h`; it listens as soon as it starts. Run it from `vsp-server/`.

### 1. Start the Server

cwd **must** be `vsp-server/` or the process will not find `configs/config.yaml` and `web/dist`:

```bash
cd vsp-server
go build -o vsp-server ./cmd
./vsp-server
```

Equivalent: `make dev-server` (it already `cd`s into `vsp-server`).

After startup:

- Web console: `http://localhost:9000`
- REST API: `http://localhost:9000/api`
- Default admin: `admin` / `admin123` (local dev only; change password and JWT secret before exposing the host)

Defaults come from `vsp-server/configs/config.yaml` (`0.0.0.0:9000`, SQLite `./data/vsp.db`). Override with `VSP_SERVER_PORT`, `VSP_JWT_SECRET`, `VSP_DB_PATH`.

### 2. Log in and create a device (DeviceKey)

The web console can create a device. CLI fields match `vsp-server/internal/api/handlers`:

```bash
TOKEN=$(curl -s http://localhost:9000/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')

curl -s http://localhost:9000/api/devices \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"factory-plc"}' | jq
```

From the JSON keep:

- `data.id` for desktop-gateway `-device-id`
- `data.device_key` for field `device-agent` `-key` only

DeviceKey is not a remote-access password. The desktop side must use a user JWT.

### 3. Start the field agent

Flags come from `vsp-client/cmd/device-agent/main.go`:

```bash
cd vsp-client
go build -o device-agent ./cmd/device-agent
./device-agent -h
```

Required: `-key`, `-port`. Defaults: `-server localhost:9000`, `-mapping default`, `-baud 115200`.

Linux serial example:

```bash
./device-agent \
  -server localhost:9000 \
  -key <device_key> \
  -mapping plc \
  -port /dev/ttyUSB0 \
  -baud 9600
```

On Windows use `COM3` (or the real COM name). Linux users often need membership in the `dialout` group. Without a physical port, skip this step and use `tests/e2e` instead.

### 4. Start the desktop gateway

Use `VSPManager.exe`, or the CLI after login:

```bash
cd vsp-client
go build -o desktop-gateway ./cmd/desktop-gateway
./desktop-gateway -h
```

Required: `-token` (or `VSP_TOKEN`) and `-device-id`. Defaults: `-listen 127.0.0.1:7000`, `-mapping default`.

```bash
./desktop-gateway \
  -server localhost:9000 \
  -token "$TOKEN" \
  -device-id 1 \
  -mapping plc \
  -listen 127.0.0.1:7000
```

Point the debug tool at `127.0.0.1:7000`. The field agent `-mapping` must match.

## API

JSON success bodies are `{"data": ...}`; errors are `{"error": "..."}`. The login token is `data.token`. Authenticated routes need `Authorization: Bearer <jwt>`.

### Authentication

```text
POST /api/auth/register
POST /api/auth/login
```

### Devices

```text
GET    /api/devices
POST   /api/devices
GET    /api/devices/:id
PUT    /api/devices/:id
DELETE /api/devices/:id
POST   /api/devices/:id/regenerate-key
GET    /api/devices/:id/mappings
```

Device REST APIs manage cloud-side identity and metadata only. Serial parameters are not stored as server-controlled device configuration.

### Relay

```text
WS /api/relay/device
WS /api/relay/gateway
```

See [docs/relay-protocol.md](docs/relay-protocol.md) for the hello messages, mapping metadata, and binary frame behavior.

## Build

```bash
make all              # server + CLI, no Wails
make smoke            # current-platform binaries + CLI help
make build-server
make build-client     # cross-compiled linux/windows CLI
make build-windows    # needs Wails, typically on Windows
```

`make all` does not build the Windows GUI.

Generate the Windows app icon assets and build a standard per-user NSIS installer:

```powershell
cd vsp-windows
go run tools/gen_windows_assets.go
wails build -clean
makensis /DAPP_VERSION=0.0.3 packaging/windows/VSPManager.nsi
```

The installer is written to `vsp-windows/build/installer/` and creates Start Menu, desktop shortcut, and uninstall entries.

Use [docs/windows-release-checklist.md](docs/windows-release-checklist.md) before release to verify icons, Wails build output, the NSIS installer, install/start behavior, and uninstall cleanup.

Validation commands:

```bash
cd vsp-server && go test ./...
cd vsp-client && go test ./...
cd vsp-windows && go test ./...
cd vsp-windows/frontend && npm ci && npm run build
cd tests/e2e && go test ./...
```

`tests/e2e` runs on Linux and uses a pseudo-terminal to simulate a serial device. It starts a real `vsp-server`, `device-agent`, and `desktop-gateway`, then verifies TCP-to-serial and serial-to-TCP byte forwarding.

## Project Structure

```text
serialserver/
├── vsp-server/             # Cloud server and relay
├── vsp-client/             # device-agent and desktop-gateway
├── vsp-windows/            # Windows GUI gateway
├── docs/user-manual.md     # Chinese user manual
├── docs/user-manual-en.md  # English user manual
├── docs/relay-protocol.md        # relay protocol
├── docs/windows-release-checklist.md
├── tests/                  # Test helpers and Linux PTY E2E
├── Makefile
└── README.md
```

## Requirements

Covered at the top of Quick Start: Go 1.25+, per-module `go.mod` files, optional `jq`. Node.js 20+, Wails, and NSIS are only for the Windows GUI / installer.

## License

[MIT License](LICENSE)
