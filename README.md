# VSP 虚拟串口云平台

商业化虚拟串口系统，支持多租户 SaaS 模式，用于通过网络远程访问串口设备。支持 PLC 远程调试、IoT 设备管理、工业自动化等场景。

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
│  vsp-client    │◄──── TCP/WebSocket ─│                                      │
│  (Go 客户端)    │     端口 9000       │  ┌─────────────────────────────┐    │
│  DeviceKey认证  │                    │  │   REST API / WebSocket      │    │
│                 │                    │  └─────────────────────────────┘    │
└─────────────────┘                    │                                      │
                                       │  ┌─────────────────────────────┐    │
                                       │  │   SQLite 数据库             │    │
                                       │  │   用户 / 设备 / 会话        │    │
                                       │  └─────────────────────────────┘    │
                                       └──────────────────┬──────────────────┘
                                                          │
                                                          │ TCP/WebSocket
                                                          │
                                       ┌──────────────────▼──────────────────┐
                                       │           Windows 端                 │
                                       │         (vsp-windows)               │
                                       ├─────────────────────────────────────┤
                                       │  [调试软件]                          │
                                       │  串口工具 / SCADA                    │
                                       │       │                             │
                                       │       ▼                             │
                                       │  VSPDriver.sys + VSPManager (WPF)   │
                                       │  虚拟串口 VSP1                       │
                                       └─────────────────────────────────────┘
```

## 组件说明

| 组件 | 语言 | 位置 | 用途 |
|------|------|------|------|
| **vsp-server** | Go | `vsp-server/` | 云服务端，多租户管理，REST API |
| **vsp-client** | Go | `vsp-client/` | 设备端客户端，连接服务器上传串口数据 |
| **vsp-windows** | C# + C++ | `vsp-windows/` | Windows 端 (VSPManager WPF + VSPDriver 内核驱动) |

## 核心功能

### vsp-server (云服务端)

- **用户管理**: 注册、登录、JWT 认证、角色权限
- **设备管理**: 添加设备、生成 DeviceKey、设备状态监控
- **多租户**: 租户隔离、配额管理
- **连接管理**: 设备/客户端连接、会话管理
- **WebSocket**: 实时数据转发
- **REST API**: 完整的 API 接口
- **Web 管理后台**: 仪表盘、设备管理、日志查询
- **数据存储**: SQLite 数据库 (纯 Go 实现，无需 CGO)

### vsp-client (设备端)

- 物理串口读取
- 主动连接云服务器
- DeviceKey 认证
- **从服务器获取串口配置** (波特率、数据位等)
- 心跳保活、断线重连

#### 设备端使用

```bash
# 简易设备客户端 (ARM-Linux 推荐)
cd vsp-client
go build -o device-client ./cmd/device-client

# 运行 - 配置从服务器获取
./device-client -server your-server:9000 -key <device_key>

# 或覆盖配置
./device-client -server your-server:9000 -key <device_key> -port COM3 -baud 115200
```

### vsp-windows (Windows 端)

- **VSPManager (WPF)**: GUI 管理界面
- **VSPDriver.sys**: 内核驱动，创建虚拟串口
- **远程配置设备串口参数**: 波特率、数据位、停止位、校验
- 连接云服务器
- 数据双向转发

#### Windows 端打包

```powershell
cd vsp-windows/VSPManager
dotnet publish -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true
# 输出: bin/Release/net8.0-windows/win-x64/publish/VSPManager.exe
```

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

```bash
# 登录获取 Token
curl -X POST http://localhost:9000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 创建设备
curl -X POST http://localhost:9000/api/v1/devices \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"PLC-1","serial_port":"COM3","baud_rate":115200}'
```

### 3. 启动设备端

```bash
cd vsp-client
go build -o vsp-client ./cmd
./vsp-client -server localhost:9000 -device-key <device_key> -port COM3
```

### 4. 启动 Windows 端

```powershell
# 安装驱动 (首次)
sc.exe start VSPDriver

# 运行 VSPManager
cd vsp-windows/VSPManager
dotnet run
```

### 5. 连接并使用

1. 在 VSPManager 中输入服务器地址和 DeviceKey
2. 点击连接
3. 用串口工具连接 `\\.\VSP1`
4. 开始远程调试

## API 文档

### 认证

```
POST /api/v1/auth/register   # 用户注册
POST /api/v1/auth/login      # 用户登录 (返回 JWT Token)
GET  /api/v1/profile         # 获取当前用户信息
```

### 设备

```
GET    /api/v1/devices           # 设备列表
POST   /api/v1/devices           # 创建设备
GET    /api/v1/devices/:id       # 设备详情
PUT    /api/v1/devices/:id       # 更新设备
DELETE /api/v1/devices/:id       # 删除设备
PUT    /api/v1/devices/:id/config       # 更新设备串口配置
POST   /api/v1/devices/:id/regenerate-key  # 重新生成 DeviceKey
```

### 设备配置 (设备端使用)

```
GET /api/v1/devices/config?device_key=xxx  # 获取配置 (设备端调用)
PUT /api/v1/devices/by-key/:key/config     # 更新配置 (通过DeviceKey)
```

### 统计与日志

```
GET /api/v1/stats   # 系统统计 (设备数、用户数、会话数)
GET /api/v1/logs    # 连接日志
```

### WebSocket

```
WS /api/v1/ws/device   # 设备端连接 (携带 DeviceKey 认证)
WS /api/v1/ws/client   # Windows 客户端连接 (携带 JWT Token)
```

## 构建依赖

### vsp-server (Go)
- Go 1.21+
- 纯 Go SQLite 驱动 (无需 CGO/GCC)

```bash
cd vsp-server
go mod tidy
go build -o vsp-server ./cmd
```

### vsp-client (Go)
- Go 1.21+
- github.com/tarm/serial

```bash
cd vsp-client
go build -o vsp-client ./cmd
```

### vsp-windows (C# + C++)
- Visual Studio 2022
- Windows Driver Kit (WDK)
- .NET 8 SDK

```powershell
# 驱动
cd vsp-windows/VSPDriver
msbuild VSPDriver.sln -p:Configuration=Release -p:Platform=x64

# WPF GUI
cd vsp-windows/VSPManager
dotnet build -c Release
```

## 文件结构

```
serialserver/
├── vsp-server/              # 云服务端
│   ├── cmd/main.go
│   ├── internal/
│   │   ├── api/            # REST API
│   │   ├── models/         # 数据模型
│   │   ├── services/       # 业务服务
│   │   └── database/       # 数据库
│   ├── configs/
│   └── web/                # Web 前端
│
├── vsp-client/              # 设备端客户端
│   ├── cmd/main.go
│   ├── internal/
│   └── config.json
│
├── vsp-windows/             # Windows 端
│   ├── VSPDriver/          # 内核驱动
│   │   └── src/driver/bin/Release/
│   │       ├── VSPDriver.sys
│   │       └── VSPDriver.inf
│   └── VSPManager/         # WPF GUI
│       └── bin/Release/
│           └── VSPManager.exe
│
├── DEPLOY.md               # 详细部署文档
└── README.md
```

## 数据库模型

```sql
-- 用户表
users (id, username, email, password_hash, role, tenant_id, status)

-- 租户表
tenants (id, name, slug, plan, max_devices, max_connections)

-- 设备表
devices (id, tenant_id, user_id, name, device_key, serial_port, baud_rate, status)

-- 会话表
sessions (id, device_id, user_id, client_type, client_addr, bytes_sent, bytes_received)

-- 日志表
connection_logs (id, tenant_id, device_id, user_id, action, details)
```

## 常用命令

```powershell
# 驱动管理
sc.exe start VSPDriver     # 启动驱动
sc.exe stop VSPDriver      # 停止驱动
sc.exe query VSPDriver     # 查看状态

# 端口检查
netstat -an | findstr 9000
```

## 详细文档

参见 [DEPLOY.md](./DEPLOY.md) 获取完整部署指南。

## 许可证

MIT License