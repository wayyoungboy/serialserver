# 远程串口网关

远程串口网关将串口参数保留在设备现场配置。云端只负责设备鉴权、用户授权、会话配对、二进制转发和审计，不再作为串口名、波特率、校验位等参数的权威来源。

## 最小范围

- 单设备、单串口映射。
- 同一映射同时只允许一个可写会话。
- 设备端主动连接云端，无需现场公网 IP 或端口映射。
- 远程端先提供本地 TCP 出口，第三方工具连接 `127.0.0.1:PORT`。

虚拟 COM 出口仍是后续能力；当前主线先提供本地 TCP 出口，跑通协议和会话状态机。

## 组件

| 组件 | 位置 | 职责 |
|------|------|------|
| `vsp-server` | `vsp-server/` | relay、鉴权、配对、转发、映射状态查询 |
| `device-agent` | `vsp-client/cmd/device-agent/` | 现场打开物理串口并主动连接 relay |
| `desktop-gateway` | `vsp-client/cmd/desktop-gateway/` | 创建本地 TCP 端点，连接 relay 并转发数据 |

## 服务端接口

```text
WS  /api/relay/device
WS  /api/relay/gateway
GET /api/devices/:id/mappings
```

`/api/devices/:id/mappings` 需要用户 JWT，用于查看设备当前在线映射。

## WebSocket 协议

连接建立后，第一帧必须是文本 JSON `hello`。认证和配对完成后，串口数据全部使用 WebSocket binary frame。

设备端 hello:

```json
{
  "type": "hello",
  "protocol": "vsp.relay",
  "role": "device",
  "device_key": "DEVICE_KEY",
  "mapping_id": "plc-fx3u",
  "mapping": {
    "id": "plc-fx3u",
    "name": "PLC FX3U",
    "serial": {
      "port": "COM3",
      "baud_rate": 9600,
      "data_bits": 7,
      "stop_bits": 1,
      "parity": "E",
      "flow_control": "none"
    }
  }
}
```

网关 hello:

```json
{
  "type": "hello",
  "protocol": "vsp.relay",
  "role": "gateway",
  "device_id": 1,
  "user_token": "USER_JWT",
  "mapping_id": "plc-fx3u"
}
```

桌面网关只支持 `user_token + device_id`。`device_key` 仅用于现场设备 Agent 连接云端。

## 运行示例

启动服务端:

```bash
cd vsp-server
go run ./cmd
```

现场设备端:

```bash
cd vsp-client
go run ./cmd/device-agent \
  -server localhost:9000 \
  -key DEVICE_KEY \
  -mapping plc-fx3u \
  -name "PLC FX3U" \
  -port COM3 \
  -baud 9600 \
  -data-bits 7 \
  -parity E \
  -stop-bits 1
```

远程桌面网关:

```bash
cd vsp-client
go run ./cmd/desktop-gateway \
  -server localhost:9000 \
  -token USER_JWT \
  -device-id 1 \
  -mapping plc-fx3u \
  -listen 127.0.0.1:7000
```

然后让 SecureCRT、Xshell、MobaXterm、WindTerm、PuTTY 或 Termius 连接 `127.0.0.1:7000`。它们看到的是一个普通 TCP 端点，数据会被 relay 转发到现场串口。

## 后续计划

- 增加短期一次性会话票据，替代直接携带用户 JWT 建立 relay。
- 增加 Windows 虚拟 COM 适配器，作为 Gateway 的另一种本地出口。
- 支持一台设备上报多个串口映射。
- 增加断线恢复、会话超时、只读观察和更完整审计。
