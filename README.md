# VSP - 虚拟串口云平台

**[English](README_EN.md)** | **中文**

[![Build](https://github.com/wayyoungboy/serialserver/actions/workflows/release.yml/badge.svg)](https://github.com/wayyoungboy/serialserver/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)

VSP (Virtual Serial Port) 是一个商业化虚拟串口云平台，支持通过网络远程访问串口设备。适用于 PLC 远程调试、IoT 设备管理、工业自动化等场景。

当前主线采用远程串口网关架构：现场设备端自行配置物理串口并主动连接云端，远程桌面端创建本地 TCP 入口，云端只负责鉴权、配对、二进制转发和审计。

> **给第一次打开仓库的人：** 这是一个多 module 的 Go 仓库，**根目录没有 `go.mod`**。服务端必须在 `vsp-server/` 目录下启动（读取 `configs/config.yaml` 和 `web/dist`）。没有 PLC / 物理串口时，用下面的「无硬件冒烟」即可验证主路径。

## Windows GUI 预览

![VSPManager GUI](docs/images/vspmanager-gui.png)

## 文档导航

| 文档 | 适合读者 | 内容 |
|------|----------|------|
| [用户使用说明书](docs/user-manual.md) | 管理员、现场人员、远程使用者 | 从创建设备到现场上线、Windows GUI、CLI 网关和排错 |
| [Relay 协议说明](docs/relay-protocol.md) | 开发者、集成方 | WebSocket hello、映射状态、二进制帧转发规则 |
| [Windows 发版检查清单](docs/windows-release-checklist.md) | 发版负责人、测试人员 | 图标、Wails 构建、NSIS 安装包、安装和卸载验收 |
| [测试说明](tests/README.md) | 开发者、CI 维护者 | 单元测试和 Linux pseudo-terminal 串口模拟 E2E |

## 系统架构

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           VSP Cloud Platform                                 │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────┐                    ┌─────────────────────────────────────┐
│    设备端        │                    │           云服务端                   │
│  (工厂现场)      │                    │         (vsp-server)                │
├─────────────────┤                    ├─────────────────────────────────────┤
│  [串口设备]      │                    │  ┌─────────────────────────────┐    │
│  PLC/传感器     │                    │  │   Web 管理后台              │    │
│       │         │                    │  │   设备管理 / 用户管理        │    │
│       ▼         │                    │  └─────────────────────────────┘    │
│ device-agent     │◄──── WebSocket ─────│                                 │
│  (Go 客户端)    │     端口 9000       │  ┌─────────────────────────────┐    │
│ 本地串口配置    │                    │  │  REST API / Relay            │    │
│                 │                    │  └─────────────────────────────┘    │
└─────────────────┘                    └──────────────────┬──────────────────┘
                                                          │
                                                          │ WebSocket
                                                          │
                                       ┌──────────────────▼──────────────────┐
                                       │           Windows 端                 │
                                       │         (vsp-windows)               │
                                       ├─────────────────────────────────────┤
                                       │  [调试软件]                          │
                                       │  串口工具 / SCADA / PLC编程软件      │
                                       │       │                             │
                                       │       ▼                             │
                                       │  VSPManager (Go+Wails)              │
                                       │  本地 TCP 网关                       │
                                       │  127.0.0.1:PORT                     │
                                       └─────────────────────────────────────┘
```

## 组件说明

| 组件 | 语言 | 位置 | 用途 |
|------|------|------|------|
| **vsp-server** | Go | `vsp-server/` | 云服务端，REST API，WebSocket 中继，多租户管理 |
| **vsp-client** | Go | `vsp-client/` | 设备 Agent，现场配置串口并主动连接云端 |
| **vsp-windows** | Go + Wails | `vsp-windows/` | 桌面网关 GUI，创建本地 TCP 出口 |
| **com0com** | C++ | `com0com/` | 后续虚拟 COM 适配器实现材料 |

### 远程串口网关

远程串口网关面向“类似远程桌面软件”的连接体验：现场设备端自行配置串口参数并主动连接云端，云端只做鉴权、配对和二进制中继；远程端先提供本地 TCP 出口，第三方工具可连接 `127.0.0.1:PORT` 使用。详见 [docs/relay-protocol.md](docs/relay-protocol.md)。

## 核心功能

### vsp-server (云服务端)

- **用户管理**: 注册、登录、JWT 认证
- **设备管理**: 添加设备、生成现场设备凭证、设备状态监控
- **多租户**: 租户隔离、配额管理
- **Relay**: WebSocket 二进制帧实时双向转发
- **REST API**: 完整的 API 接口
- **Web 管理后台**: 仪表盘、设备管理

### vsp-client (设备端)

- 物理串口读取
- 主动连接云服务器
- 现场本地配置串口参数
- 上报串口映射，云端只做中继
- 断线重连

### vsp-windows (Windows GUI)

- Wails + Vue.js 构建的现代化 GUI
- 登录后选择设备和在线串口映射
- 创建本地 TCP 出口
- Relay 数据双向转发
- 连接状态监控
- 支持 HTTP/HTTPS

## 要求

- **Go 1.25+**（`vsp-server`、`vsp-client`、`vsp-windows` 的 `go.mod` 都写的是 `go 1.25.0`）
- 本仓库是独立 module，不要在仓库根目录执行 `go mod download` / `go build`
- 可选：`jq`（下面 curl 示例用来抽 token；没有 jq 就从 JSON 的 `data.token` / `data.id` / `data.device_key` 手抄）
- Windows GUI 额外需要：Node.js 20+、[Wails CLI](https://wails.io/)、NSIS（打安装包时）

## 快速开始

详细操作请看 [用户使用说明书](docs/user-manual.md)。下面是一个陌生人按文档能跑通的最小路径。

### 0. 无硬件冒烟（推荐先做）

不需要 PLC、USB 转串口或 Windows。在 **Linux** 上会用 pseudo-terminal 模拟串口，真实编译并拉起 `vsp-server`、`device-agent`、`desktop-gateway`，再验证 TCP 与串口双向字节：

```bash
cd tests/e2e
go test -v ./...
```

只想确认能编译、CLI 参数和代码一致：

```bash
make smoke
```

`make smoke` 会在当前系统编译服务端和两个 CLI，并打印 `device-agent -h` / `desktop-gateway -h`。服务端没有 `-h`，启动即监听。

### 1. 启动云服务端

必须在 `vsp-server/` 目录运行，否则找不到 `configs/config.yaml` 和 `web/dist` 管理后台：

```bash
cd vsp-server
go build -o vsp-server ./cmd
./vsp-server
```

等价：`make dev-server`（内部是 `cd vsp-server && go run ./cmd`）。

服务启动后:

- Web 管理后台: `http://localhost:9000`
- REST API: `http://localhost:9000/api`
- 默认管理员: `admin` / `admin123`（仅本地开发；公网必须改密码和 JWT secret）

配置默认值来自 `vsp-server/configs/config.yaml`：监听 `0.0.0.0:9000`，SQLite 在 `./data/vsp.db`。也可用环境变量覆盖：`VSP_SERVER_PORT`、`VSP_JWT_SECRET`、`VSP_DB_PATH`。

### 2. 登录并创建设备（拿到 DeviceKey）

Web 后台可以点选创建。CLI / 脚本路径（字段与 `vsp-server/internal/api/handlers` 一致）：

```bash
TOKEN=$(curl -s http://localhost:9000/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')

curl -s http://localhost:9000/api/devices \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"factory-plc"}' | jq
```

从返回 JSON 记下：

- `data.id` — 桌面网关的 `-device-id`
- `data.device_key` — 只给现场 `device-agent` 的 `-key`

DeviceKey 不是远程登录密码；桌面端必须用用户 JWT。

### 3. 启动设备端

先看真实 flag（与 `vsp-client/cmd/device-agent/main.go` 一致）：

```bash
cd vsp-client
go build -o device-agent ./cmd/device-agent
./device-agent -h
```

必填：`-key`、`-port`。默认 `-server localhost:9000`、`-mapping default`、`-baud 115200`。

Linux 现场串口示例：

```bash
./device-agent \
  -server localhost:9000 \
  -key <device_key> \
  -mapping plc \
  -port /dev/ttyUSB0 \
  -baud 9600
```

Windows 把 `-port` 换成 `COM3`（或设备管理器里的实际 COM 号）。Linux 权限不够时，把用户加入 `dialout` 组后重新登录。没有物理串口时不要硬跑这一步，用第 0 步的 `tests/e2e`。

### 4. 启动桌面端

双击 `VSPManager.exe`，登录后选择设备映射并启动本地 TCP 网关。也可以使用 CLI：

```bash
cd vsp-client
go build -o desktop-gateway ./cmd/desktop-gateway
./desktop-gateway -h
```

必填：`-token`（或环境变量 `VSP_TOKEN`）、`-device-id`。默认 `-listen 127.0.0.1:7000`、`-mapping default`。

```bash
./desktop-gateway \
  -server localhost:9000 \
  -token "$TOKEN" \
  -device-id 1 \
  -mapping plc \
  -listen 127.0.0.1:7000
```

随后让调试软件连接 `127.0.0.1:7000`。现场 Agent 的 `-mapping` 必须和这里一致。

## 构建

### 本地构建

```bash
# 服务端 + 多平台 CLI（不需要 Wails）
make all

# 当前机器可执行文件，并打印 CLI 帮助
make smoke

# 单独构建
make build-server     # 服务端 -> build/release/vsp-server（运行时仍要把 cwd 设到 vsp-server/）
make build-client     # CLI: 交叉编译 linux/windows 的 device-agent / desktop-gateway
make build-windows    # Windows GUI，需要 Wails，通常在 Windows 上跑

# 打包发布（含 Windows 安装包，需要 NSIS/Wails）
make package
```

`make all` **不会**构建 Windows GUI。GUI 请用 `make build-windows` 或下面的 Wails 命令。

### Windows GUI 构建 (需要 Wails)

```powershell
cd vsp-windows
go run tools/gen_windows_assets.go
wails build -clean
```

### Windows 标准安装包 (需要 NSIS)

```powershell
cd vsp-windows
go run tools/gen_windows_assets.go
wails build -clean
makensis /DAPP_VERSION=0.0.3 packaging/windows/VSPManager.nsi
```

安装器输出到 `vsp-windows/build/installer/`，会写入当前用户的开始菜单、桌面快捷方式和卸载项。

发版前请按 [docs/windows-release-checklist.md](docs/windows-release-checklist.md) 验收图标、Wails 构建、NSIS 安装包、安装启动和卸载流程。

## 测试

```bash
cd vsp-server && go test ./...
cd vsp-client && go test ./...
cd vsp-windows && go test ./...
cd vsp-windows/frontend && npm ci && npm run build
cd tests/e2e && go test ./...
```

或：`make test`（前端会先安装依赖再构建）。只要 Linux 串口模拟 E2E：`make test-e2e`。

`tests/e2e` 带 Linux build tag，会在 Linux 上使用 pseudo-terminal 模拟串口，启动真实的 `vsp-server`、`device-agent` 和 `desktop-gateway`，验证 TCP 到串口再回到 TCP 的双向数据链路。非 Linux 上该包没有测试文件。

## API 文档

响应一律包在 `{"data": ...}` 里；错误是 `{"error": "..."}`。登录 token 在 `data.token`。需要认证的接口使用 `Authorization: Bearer <jwt>`。

### 认证

```
POST /api/auth/register   # {"username","email","password"}
POST /api/auth/login      # {"username","password"} -> data.token
```

### 设备

```
GET    /api/devices               # 设备列表
POST   /api/devices               # {"name":"..."} -> data.id, data.device_key
DELETE /api/devices/:id           # 删除设备
GET    /api/devices/:id/mappings  # 在线串口映射
POST   /api/devices/:id/regenerate-key
```

### WebSocket

```
WS /api/relay/device   # 设备端 relay 连接
WS /api/relay/gateway  # 桌面网关 relay 连接
```

## 项目结构

```
serialserver/
├── .github/workflows/      # GitHub Actions CI/CD
├── vsp-server/             # 云服务端（独立 go.mod）
│   ├── cmd/main.go
│   ├── internal/
│   ├── configs/config.yaml
│   └── web/dist/           # 管理后台静态文件，启动时 cwd 需在 vsp-server/
├── vsp-client/             # 设备端客户端
│   ├── cmd/device-agent/
│   └── cmd/desktop-gateway/
├── vsp-windows/            # Windows GUI 客户端
│   ├── main.go
│   ├── app.go
│   ├── frontend/           # Vue.js 前端
│   ├── internal/
│   └── wails.json
├── com0com/                # 后续虚拟 COM 适配器材料
├── docs/                   # 协议和发版文档
│   ├── user-manual.md
│   ├── relay-protocol.md
│   └── windows-release-checklist.md
├── tests/                  # 测试辅助程序和 Linux PTY E2E
├── Makefile
└── README.md
```

## 数据流

```
[物理设备] ←→ [device-agent] ←→ [Relay] ←→ [vsp-server] ←→ [Relay] ←→ [VSPManager] ←→ [本地 TCP] ←→ [调试软件]
```

## 依赖

- **Go 1.25+**
- **Node.js 20+** (构建 Windows GUI)
- **Wails CLI** (构建 Windows GUI)
- **NSIS** (构建 Windows 标准安装包)
- **com0com** 仅作为后续虚拟 COM 出口的研究材料，当前主线不依赖

## 🤝 参与贡献

欢迎参与 VSP 项目共建！无论是提交 Bug、建议新功能、改进文档还是提交代码，我们都非常欢迎。

### 贡献方式

- **报告问题**: [提交 Issue](https://github.com/wayyoungboy/serialserver/issues) 描述 Bug 或功能建议
- **提交代码**: Fork → 修改 → 提交 Pull Request
- **完善文档**: 帮助改进 README、API 文档或添加使用示例
- **分享经验**: 在社区分享你的使用场景和最佳实践

### 开发指南

```bash
git clone https://github.com/wayyoungboy/serialserver.git
cd serialserver

# 每个组件是独立 module，在对应目录拉依赖
(cd vsp-server && go mod download)
(cd vsp-client && go mod download)
(cd vsp-windows && go mod download)
(cd tests/e2e && go mod download)

# 运行测试（Linux 上包含 PTY E2E）
make test

# 启动开发服务（cwd 已切到 vsp-server）
make dev-server
```

### 代码规范

- 遵循 Go 官方代码规范
- 提交前运行 `go fmt` 格式化代码
- 为新功能添加测试用例
- 提交信息使用约定式提交格式 (Conventional Commits)

## 📞 联系方式

- **Issues**: [GitHub Issues](https://github.com/wayyoungboy/serialserver/issues)
- **Discussions**: [GitHub Discussions](https://github.com/wayyoungboy/serialserver/discussions)

## ⭐ Star History

如果这个项目对你有帮助，欢迎给一个 Star ⭐ 支持一下！

[![Star History Chart](https://api.star-history.com/svg?repos=wayyoungboy/serialserver&type=Date)](https://star-history.com/#wayyoungboy/serialserver&Date)

## 许可证

[MIT License](LICENSE)
