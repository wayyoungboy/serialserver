# VSP - 虚拟串口云平台

[![Build](https://github.com/wayyoungboy/serialserver/actions/workflows/release.yml/badge.svg)](https://github.com/wayyoungboy/serialserver/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)

VSP (Virtual Serial Port) 是一个商业化虚拟串口云平台，支持通过网络远程访问串口设备。适用于 PLC 远程调试、IoT 设备管理、工业自动化等场景。

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
│  device-client │◄──── WebSocket ─────│                                      │
│  (Go 客户端)    │     端口 9000       │  ┌─────────────────────────────┐    │
│  DeviceKey认证  │                    │  │   REST API / WebSocket      │    │
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
                                       │  + com0com 虚拟串口驱动              │
                                       │  虚拟 COM 端口                       │
                                       └─────────────────────────────────────┘
```

## 组件说明

| 组件 | 语言 | 位置 | 用途 |
|------|------|------|------|
| **vsp-server** | Go | `vsp-server/` | 云服务端，REST API，WebSocket 中继，多租户管理 |
| **vsp-client** | Go | `vsp-client/` | 设备端客户端，读取物理串口，上传到服务器 |
| **vsp-windows** | Go + Wails | `vsp-windows/` | Windows GUI 客户端，创建虚拟串口，连接服务器 |
| **com0com** | C++ | `com0com/` | 开源虚拟串口驱动 |

## 核心功能

### vsp-server (云服务端)

- **用户管理**: 注册、登录、JWT 认证
- **设备管理**: 添加设备、生成 DeviceKey、设备状态监控
- **多租户**: 租户隔离、配额管理
- **WebSocket**: 实时双向数据转发
- **REST API**: 完整的 API 接口
- **Web 管理后台**: 仪表盘、设备管理

### vsp-client (设备端)

- 物理串口读取
- 主动连接云服务器
- DeviceKey 认证
- 从服务器获取串口配置
- 断线重连

### vsp-windows (Windows GUI)

- Wails + Vue.js 构建的现代化 GUI
- 自动创建虚拟 COM 端口
- 数据双向转发
- 连接状态监控
- 支持 HTTP/HTTPS

## 快速开始

### 1. 启动云服务端

```bash
cd vsp-server
go build -o vsp-server ./cmd
./vsp-server
```

服务启动后:
- Web 管理后台: `http://localhost:9000`
- REST API: `http://localhost:9000/api/v1`
- 默认管理员: `admin` / `admin123`

### 2. 创建设备

通过 Web 管理后台或 API 创建设备，获取 DeviceKey。

### 3. 启动设备端

```bash
cd vsp-client
go build -o device-client ./cmd/device-client
./device-client -server your-server:9000 -key <device_key>
```

### 4. 启动 Windows 客户端

双击 `VSPManager.exe`，输入服务器地址，登录后选择设备连接。

## 构建

### 本地构建

```bash
# 构建所有组件
make all

# 单独构建
make build-server     # 服务端
make build-client     # 设备客户端 (跨平台)
make build-windows    # Windows GUI 客户端

# 打包发布
make package
```

### Windows GUI 构建 (需要 Wails)

```powershell
cd vsp-windows
wails build -clean
```

## API 文档

### 认证

```
POST /api/v1/auth/register   # 用户注册
POST /api/v1/auth/login      # 用户登录 (返回 JWT Token)
```

### 设备

```
GET    /api/v1/devices           # 设备列表
POST   /api/v1/devices           # 创建设备
DELETE /api/v1/devices/:id       # 删除设备
GET    /api/v1/devices/config?device_key=xxx  # 获取配置 (设备端调用)
```

### WebSocket

```
WS /api/v1/ws/device   # 设备端连接
WS /api/v1/ws/client   # Windows 客户端连接
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
│   ├── cmd/device-client/
│   └── internal/
├── vsp-windows/            # Windows GUI 客户端
│   ├── main.go
│   ├── app.go
│   ├── frontend/           # Vue.js 前端
│   ├── internal/
│   └── wails.json
├── com0com/                # 虚拟串口驱动
├── tests/                  # 测试脚本
├── Makefile
└── README.md
```

## 数据流

```
[物理设备] ←→ [device-client] ←→ [WebSocket] ←→ [vsp-server] ←→ [WebSocket] ←→ [VSPManager] ←→ [虚拟COM口] ←→ [调试软件]
```

## 依赖

- **Go 1.24+**
- **Node.js 20+** (构建 Windows GUI)
- **Wails CLI** (构建 Windows GUI)
- **com0com** (Windows 虚拟串口驱动)

## 许可证

MIT License