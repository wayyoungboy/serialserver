# VSP - 虚拟串口云平台

**[English](README_EN.md)** | **中文**

[![Build](https://github.com/wayyoungboy/serialserver/actions/workflows/release.yml/badge.svg)](https://github.com/wayyoungboy/serialserver/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)

VSP (Virtual Serial Port) 是一个商业化虚拟串口云平台，支持通过网络远程访问串口设备。适用于 PLC 远程调试、IoT 设备管理、工业自动化等场景。

当前主线采用远程串口网关架构：现场设备端自行配置物理串口并主动连接云端，远程桌面端创建本地 TCP 入口，云端只负责鉴权、配对、二进制转发和审计。

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

## 快速开始

详细操作请看 [用户使用说明书](docs/user-manual.md)。最小链路如下。

### 1. 启动云服务端

```bash
cd vsp-server
go build -o vsp-server ./cmd
./vsp-server
```

服务启动后:
- Web 管理后台: `http://localhost:9000`
- REST API: `http://localhost:9000/api`
- 默认管理员: `admin` / `admin123`

### 2. 创建设备

通过 Web 管理后台或 API 创建设备，获取现场设备 Agent 使用的设备凭证。

### 3. 启动设备端

```bash
cd vsp-client
go build -o device-agent ./cmd/device-agent
./device-agent -server your-server:9000 -key <device_key> -mapping plc -port COM3 -baud 9600
```

### 4. 启动桌面端

双击 `VSPManager.exe`，登录后选择设备映射并启动本地 TCP 网关。也可以使用 CLI:

```bash
cd vsp-client
go build -o desktop-gateway ./cmd/desktop-gateway
./desktop-gateway -server your-server:9000 -token <user_jwt> -device-id 1 -mapping plc -listen 127.0.0.1:7000
```

随后让调试软件连接 `127.0.0.1:7000`。

## 构建

### 本地构建

```bash
# 构建所有组件
make all

# 单独构建
make build-server     # 服务端
make build-client     # CLI: device-agent / desktop-gateway
make build-windows    # Windows GUI 客户端

# 打包发布
make package
```

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
cd vsp-windows/frontend && npm run build
cd tests/e2e && go test ./...
```

`tests/e2e` 会在 Linux 上使用 pseudo-terminal 模拟串口，启动真实的 `vsp-server`、`device-agent` 和 `desktop-gateway`，验证 TCP 到串口再回到 TCP 的双向数据链路。

## API 文档

### 认证

```
POST /api/auth/register   # 用户注册
POST /api/auth/login      # 用户登录 (返回 JWT Token)
```

### 设备

```
GET    /api/devices               # 设备列表
POST   /api/devices               # 创建设备
DELETE /api/devices/:id           # 删除设备
GET    /api/devices/:id/mappings  # 在线串口映射
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
├── vsp-server/             # 云服务端
│   ├── cmd/main.go
│   ├── internal/
│   └── configs/
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
# 克隆项目
git clone https://github.com/wayyoungboy/serialserver.git
cd serialserver

# 安装依赖
go mod download

# 运行测试
make test

# 启动开发服务
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
