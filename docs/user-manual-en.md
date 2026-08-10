# VSP V2 User Manual

This manual is for VSP V2 users and field operators. V2 uses a field-side serial agent, a cloud relay, and a desktop-side local TCP gateway. Desktop tools connect to a local endpoint such as `127.0.0.1:7000`, and VSP forwards bytes to the field serial device.

## 1. Roles and Credentials

| Role | Component | Credential | Purpose |
|------|-----------|------------|---------|
| Administrator | `vsp-server` Web/API | Username and password | Create users, create devices, inspect status |
| Field operator | `device-agent-v2` | DeviceKey | Open the physical serial port and publish a mapping |
| Desktop user | VSPManager or `desktop-gateway-v2` | Username and password, or JWT after login | Select an online mapping and start a local TCP gateway |

Important boundaries:

- DeviceKey is only for the field-side `device-agent-v2`.
- Desktop access does not accept DeviceKey. It requires a logged-in user identity.
- Serial settings are configured on the field agent. The cloud server does not store or push serial port parameters.

## 2. Before You Start

Prepare:

- Server URL, for example `https://vsp.example.com` or `http://localhost:9000`.
- A user account that can log in.
- A device created in the server, or admin permission to create one.
- Field serial settings: port, baud rate, data bits, stop bits, parity, and flow control.
- A local desktop TCP address, for example `127.0.0.1:7000`.

Windows users should install VSPManager from the standard installer. Linux, macOS, and automation users can use the CLI tools directly.

## 3. Start the Server

For development or local testing:

```bash
cd vsp-server
go build -o vsp-server ./cmd
./vsp-server
```

Defaults:

- Web console: `http://localhost:9000`
- REST API: `http://localhost:9000/api/v2`
- Default admin: `admin` / `admin123`

For production, change the default admin password, JWT secret, database path, and reverse proxy TLS settings.

## 4. Create a Device

Create a device from the web console or API. CLI example:

```bash
TOKEN=$(curl -s http://localhost:9000/api/v2/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')

curl -s http://localhost:9000/api/v2/devices \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"factory-plc"}' | jq
```

The returned `device_key` is the field agent credential. Share it only with the operator responsible for the physical serial connection.

## 5. Bring the Field Device Online

Run `device-agent-v2` on the machine connected to the physical serial port:

```bash
cd vsp-client
go build -o device-agent-v2 ./cmd/device-agent-v2

./device-agent-v2 \
  -server vsp.example.com \
  -key <device_key> \
  -mapping plc-main \
  -name "PLC Main" \
  -port COM3 \
  -baud 9600 \
  -data-bits 8 \
  -stop-bits 1 \
  -parity N \
  -flow-control none
```

Linux serial example:

```bash
./device-agent-v2 \
  -server vsp.example.com \
  -key <device_key> \
  -mapping plc-main \
  -port /dev/ttyUSB0 \
  -baud 115200
```

Common flags:

| Flag | Description |
|------|-------------|
| `-server` | Server host or URL. HTTPS URLs become WSS for relay connections |
| `-key` | DeviceKey for the device |
| `-mapping` | Mapping ID selected by desktop users |
| `-name` | Mapping display name |
| `-port` | Local physical serial port, such as `COM3` or `/dev/ttyUSB0` |
| `-baud` | Baud rate |
| `-data-bits` | Data bits, usually `8` |
| `-stop-bits` | Stop bits: `1`, `1.5`, or `2` |
| `-parity` | Parity: `N`, `O`, `E`, `M`, or `S` |
| `-reconnect` | Reconnect automatically after disconnect, enabled by default |

After the agent registers, the mapping appears online for desktop users.

## 6. Use the Windows Desktop App

1. Install VSPManager.
2. Start VSPManager.
3. Choose Chinese or English.
4. Enter the server URL, for example `https://vsp.example.com`.
5. Log in with a user account.
6. Select a device.
7. Refresh and select an online mapping.
8. Set the local listen address, for example `127.0.0.1:7000`.
9. Start the gateway.
10. Point your desktop tool at `127.0.0.1:7000`.

Your desktop tool sees a normal TCP endpoint. VSPManager forwards TCP bytes through the relay to the field agent, and the field agent writes them to the physical serial port. Serial replies travel back to the local TCP connection.

When finished, disconnect the desktop tool first, then stop the gateway in VSPManager.

## 7. CLI Desktop Gateway

If you do not use the GUI, log in and get a JWT:

```bash
TOKEN=$(curl -s http://localhost:9000/api/v2/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')
```

Start the local TCP gateway:

```bash
cd vsp-client
go build -o desktop-gateway-v2 ./cmd/desktop-gateway-v2

./desktop-gateway-v2 \
  -server localhost:9000 \
  -token "$TOKEN" \
  -device-id 1 \
  -mapping plc-main \
  -listen 127.0.0.1:7000
```

You can also pass the token with an environment variable:

```bash
VSP_TOKEN="$TOKEN" ./desktop-gateway-v2 \
  -server localhost:9000 \
  -device-id 1 \
  -mapping plc-main \
  -listen 127.0.0.1:7000
```

## 8. Verify the Link

Minimal checklist:

- `device-agent-v2` logs `device registered`.
- The target device mapping is online in the web console or API.
- The desktop gateway starts and accepts a local TCP connection.
- Data sent from the desktop tool reaches the field serial device.
- Replies from the field serial device return to the desktop tool.

Developers can run the automated serial simulation E2E:

```bash
cd tests/e2e
go test ./...
```

The test uses a Linux pseudo-terminal, starts the real server and both V2 clients, and verifies TCP-to-serial and serial-to-TCP echo.

## 9. Troubleshooting

### Mapping List Is Empty

- Check that `device-agent-v2` is running.
- Check that the DeviceKey belongs to the selected device.
- Check that the agent `-mapping` value matches the desktop selection.
- Look for `invalid device_key` or `mapping.serial.port required` in server logs.

### Gateway Says mapping offline

- The field agent is not online or has disconnected.
- The desktop side selected the wrong device or mapping ID.
- The agent and desktop gateway are connected to different server URLs.

### Local TCP Port Cannot Start

- The port may already be used by another process.
- Try another port, for example `127.0.0.1:7001`.
- Windows firewall usually does not affect loopback, but security software can still block a listener.

### Serial Port Cannot Open

- Check the serial port name.
- On Windows, make sure no other program is using the COM port.
- On Linux, make sure the current user has serial permissions, commonly by joining the `dialout` group and logging in again.
- Check baud rate, data bits, stop bits, and parity.

### Connected but No Data

- Make sure the desktop tool connects to the local TCP address shown by VSPManager or the CLI gateway.
- Make sure the field serial device actually replies.
- Check that serial settings match the device.
- Inspect byte counters and logs from the agent, gateway, and server.

## 10. Security Notes

- Change the default admin password before production use.
- Do not publish DeviceKey values in repos, screenshots, or chat logs.
- Do not store JWTs in long-lived config files. Prefer environment variables for temporary CLI testing.
- Use HTTPS/WSS for public server deployments.
- Regenerate DeviceKey immediately if a device is lost or a credential may be leaked.

## 11. Related Documents

- [V2 Relay Protocol](v2-relay.md)
- [Windows Release Checklist](windows-release-checklist.md)
- [Test Notes](../tests/README.md)
