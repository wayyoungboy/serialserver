# VSP 用户使用说明书

本文面向 VSP 的日常使用者和现场部署人员。当前版本采用“现场串口 Agent + 云端 Relay + 远程本地 TCP 网关”的模式，远程桌面端会创建一个 `127.0.0.1:PORT` TCP 入口，调试软件通过这个入口访问现场串口设备。

## 1. 角色和凭证

| 角色 | 使用组件 | 需要的凭证 | 用途 |
|------|----------|------------|------|
| 管理员 | `vsp-server` Web/API | 用户名和密码 | 创建用户、创建设备、查看状态 |
| 现场人员 | `device-agent` | DeviceKey | 在设备现场连接物理串口并上线映射 |
| 远程使用者 | VSPManager 或 `desktop-gateway` | 用户名和密码，或登录后 JWT | 选择在线设备映射并启动本地 TCP 网关 |

重要边界:

- DeviceKey 只给现场 `device-agent` 使用。
- 桌面端不接受 DeviceKey，远程访问必须使用用户登录身份。
- 串口参数由现场 Agent 配置，云端不再保存或下发串口名、波特率、校验位等参数。

## 2. 使用前准备

确认以下信息已经准备好:

- 云服务地址，例如 `https://vsp.example.com` 或 `http://localhost:9000`。
- 一个可登录的用户账号。
- 已创建好的设备，或管理员权限用于创建设备。
- 现场串口参数：端口名、波特率、数据位、停止位、校验位、流控。
- 远程桌面端可用的本地 TCP 端口，例如 `127.0.0.1:7000`。

Windows 用户建议使用标准安装包安装 VSPManager。Linux、macOS 或自动化场景可以直接使用 CLI。

## 3. 启动云服务端

开发或单机测试:

```bash
cd vsp-server
go build -o vsp-server ./cmd
./vsp-server
```

启动后默认地址:

- Web 管理后台: `http://localhost:9000`
- REST API: `http://localhost:9000/api`
- 默认管理员: `admin` / `admin123`

生产环境应修改默认管理员密码、JWT secret、数据库路径和反向代理 TLS 配置。不要把测试默认密码暴露到公网。

## 4. 创建设备

管理员可以通过 Web 管理后台或 API 创建设备。CLI 示例:

```bash
TOKEN=$(curl -s http://localhost:9000/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')

curl -s http://localhost:9000/api/devices \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"factory-plc"}' | jq
```

返回结果中的 `device_key` 是现场 Agent 的凭证，请只交给负责连接物理串口的现场人员。

## 5. 现场设备端上线

在连接物理串口的机器上运行 `device-agent`:

```bash
cd vsp-client
go build -o device-agent ./cmd/device-agent

./device-agent \
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

Linux 串口示例:

```bash
./device-agent \
  -server vsp.example.com \
  -key <device_key> \
  -mapping plc-main \
  -port /dev/ttyUSB0 \
  -baud 115200
```

常用参数:

| 参数 | 说明 |
|------|------|
| `-server` | 云服务地址。没有协议时默认使用 WebSocket；HTTPS 地址会自动转成 WSS |
| `-key` | 设备 DeviceKey |
| `-mapping` | 映射 ID，远程端选择同一个映射 ID 连接 |
| `-name` | 映射显示名 |
| `-port` | 本地物理串口名，例如 `COM3`、`/dev/ttyUSB0` |
| `-baud` | 波特率 |
| `-data-bits` | 数据位，通常为 `8` |
| `-stop-bits` | 停止位，支持 `1`、`1.5`、`2` |
| `-parity` | 校验位，支持 `N`、`O`、`E`、`M`、`S` |
| `-reconnect` | 是否断线自动重连，默认开启 |

上线成功后，远程用户可以在设备映射列表中看到该映射处于在线状态。

## 6. Windows 桌面端使用

1. 安装 VSPManager。
2. 启动 VSPManager。
3. 在语言切换中选择中文或 English。
4. 填写服务端地址，例如 `https://vsp.example.com`。
5. 使用用户账号登录。
6. 选择目标设备。
7. 刷新并选择在线映射。
8. 设置本地监听地址，例如 `127.0.0.1:7000`。
9. 点击启动网关。
10. 在调试软件中连接 `127.0.0.1:7000`。

调试软件看到的是普通 TCP 入口。VSPManager 会把 TCP 数据通过云端 Relay 转发到现场 Agent，再写入现场物理串口；现场串口回复的数据会沿相反方向返回到本地 TCP 连接。

停止使用时，先断开调试软件，再在 VSPManager 中停止网关。

## 7. CLI 桌面网关

不使用 GUI 时，可以直接运行 `desktop-gateway`。先登录获取 JWT:

```bash
TOKEN=$(curl -s http://localhost:9000/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')
```

启动本地 TCP 网关:

```bash
cd vsp-client
go build -o desktop-gateway ./cmd/desktop-gateway

./desktop-gateway \
  -server localhost:9000 \
  -token "$TOKEN" \
  -device-id 1 \
  -mapping plc-main \
  -listen 127.0.0.1:7000
```

也可以用环境变量传递 token:

```bash
VSP_TOKEN="$TOKEN" ./desktop-gateway \
  -server localhost:9000 \
  -device-id 1 \
  -mapping plc-main \
  -listen 127.0.0.1:7000
```

## 8. 验证链路

最小验证步骤:

- 设备端 `device-agent` 日志出现 `device registered`。
- 管理后台或 API 中目标设备映射显示在线。
- 桌面端启动网关后，本地端口可以连接。
- 从调试软件发送数据，现场串口设备能收到。
- 现场串口设备回复后，调试软件能收到回复。

开发者可以运行自动化串口模拟 E2E:

```bash
cd tests/e2e
go test ./...
```

该测试会使用 Linux pseudo-terminal 模拟串口，自动启动服务端、现场 Agent 和桌面网关，并验证 TCP 与串口之间的双向回显。

## 9. 常见问题

### 映射列表为空

- 确认 `device-agent` 正在运行。
- 确认 Agent 使用的 DeviceKey 属于当前设备。
- 确认 Agent 的 `-mapping` 与桌面端选择的映射一致。
- 查看服务端日志中是否有 `invalid device_key` 或 `mapping.serial.port required`。

### 桌面网关提示 mapping offline

- 现场 Agent 未上线，或现场 Agent 已断线。
- 桌面端选择了错误的设备或映射 ID。
- 服务端地址不一致，Agent 和桌面端连接到了不同服务端。

### 本地 TCP 端口无法启动

- 端口可能已被其他程序占用。
- 换一个端口，例如 `127.0.0.1:7001`。
- Windows 防火墙一般不影响 `127.0.0.1` 本地回环，但安全软件可能拦截进程监听。

### 串口打不开

- 检查串口名是否正确。
- Windows 上确认没有其他程序占用该 COM 口。
- Linux 上确认当前用户有串口权限，常见做法是加入 `dialout` 组后重新登录。
- 检查波特率、数据位、停止位和校验位是否与现场设备一致。

### 能连接但没有数据

- 确认调试软件连接的是 VSPManager 或 CLI 网关显示的本地 TCP 地址。
- 确认现场串口设备实际会回复。
- 检查串口参数是否匹配。
- 查看 Agent、Gateway 和 Server 日志中的字节计数。

## 10. 安全建议

- 生产环境必须修改默认管理员密码。
- 不要把 DeviceKey 写入公开仓库、截图或聊天记录。
- 不要把 JWT 放进长期配置文件；CLI 临时调试时可以用环境变量传递。
- 服务端公网部署应启用 HTTPS/WSS。
- 离职、设备遗失或凭证疑似泄露时，立即重新生成 DeviceKey。

## 11. 相关文档

- [Relay 协议说明](relay-protocol.md)
- [Windows 发版检查清单](windows-release-checklist.md)
- [测试说明](../tests/README.md)
