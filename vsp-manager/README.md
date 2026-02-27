# VSP Manager - 虚拟串口管理器

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Platform](https://img.shields.io/badge/Platform-Windows/Linux/macOS-blue.svg)](https://github.com)

VSP Manager (Virtual Serial Port Manager) 是一款商业级串口管理工具，支持串口与TCP网络之间的双向数据透传。

## 功能特性

- **串口转TCP** - 将本地串口数据通过TCP发送
- **TCP转串口** - 将TCP接收的数据转发到串口
- **双向隧道模式** - 串口与TCP双向透传
- **串口共享服务器** - 通过TCP远程访问本地串口
- **配置文件管理** - 支持JSON/YAML配置
- **跨平台支持** - Windows/Linux/macOS

## 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                        VSP Manager                           │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────┐    │
│  │                    GUI 层 (Fyne)                    │    │
│  │  • 端口配置   • 流量监控   • 日志查看   • 设置     │    │
│  └──────────────────────┬──────────────────────────────┘    │
│                         │                                    │
│  ┌──────────────────────▼──────────────────────────────┐    │
│  │                   业务逻辑层                         │    │
│  │  • PortManager    • TunnelManager    • ConfigMgr  │    │
│  └──────────────────────┬──────────────────────────────┘    │
│                         │                                    │
│  ┌──────────────────────▼──────────────────────────────┐    │
│  │                   核心引擎层                         │    │
│  │  • Serial Driver  • TCP Client/Server  • Protocol  │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

## 快速开始

### 下载预构建版本

从 [Releases](https://github.com/your-repo/vsp-manager/releases) 下载对应平台的二进制文件。

### 构建

```bash
# 克隆项目
git clone https://github.com/your-repo/vsp-manager.git
cd vsp-manager

# 构建当前平台
make build

# 构建所有平台
make build-all

# 或使用Go命令
go build -o vsp-manager ./cmd
```

### 使用

```bash
# 查看版本
./vsp-manager -version

# 查看帮助
./vsp-manager -h

# 指定配置文件运行
./vsp-manager -config config.json

# 编辑配置文件启用隧道
# 设置 config.json 中 tunnels[].enabled = true
```

## 配置说明

### 配置文件 (config.json)

```json
{
  "version": "1.0.0",
  "tunnels": [
    {
      "name": "PLC-连接1",
      "mode": "tunnel",
      "serial": {
        "port": "COM3",
        "baud": 115200,
        "dataBits": 8,
        "stopBits": 1,
        "parity": "N"
      },
      "tcp": {
        "host": "192.168.1.100",
        "port": 9000
      },
      "enabled": true
    }
  ],
  "ui": {
    "port": 8080,
    "theme": "dark",
    "startMinimized": false,
    "minimizeToTray": true
  }
}
```

### 模式说明

| 模式 | 说明 | 典型用途 |
|------|------|----------|
| `client` | 作为TCP客户端连接远程服务器 | 串口数据上传到服务器 |
| `server` | 作为TCP服务器接受连接 | 远程应用访问本地串口 |
| `tunnel` | 双向透传 | PLC远程调试、串口透传 |

### 串口参数

| 参数 | 说明 | 常用值 |
|------|------|--------|
| port | 串口名称 | COM1-COM10, /dev/ttyUSB0 |
| baud | 波特率 | 9600, 19200, 38400, 57600, 115200 |
| dataBits | 数据位 | 5, 6, 7, 8 |
| stopBits | 停止位 | 1, 2 |
| parity | 校验位 | N(无), O(奇), E(偶) |

## Makefile 使用

```bash
make help          # 显示帮助
make build         # 构建当前平台
make build-windows # 构建Windows版本
make build-linux   # 构建Linux版本
make build-darwin  # 构建macOS版本
make build-all     # 构建所有平台
make clean         # 清理构建产物
make test          # 运行测试
make deps          # 下载依赖
```

## 目录结构

```
vsp-manager/
├── cmd/
│   └── main.go              # 入口文件
├── internal/
│   ├── config/
│   │   └── config.go        # 配置管理
│   ├── serial/
│   │   └── serial.go        # 串口操作
│   ├── tcp/
│   │   └── tunnel.go        # TCP透传引擎
│   └── ui/
│       └── main.go          # GUI界面
├── assets/
│   └── icon.png             # 应用图标
├── config.json              # 配置文件示例
├── Makefile                 # 构建脚本
├── LICENSE                  # MIT许可证
└── README.md                # 本文档
```

## 典型应用场景

### 1. PLC远程调试

```
[三菱PLC FX3U] ←→ [串口] ←→ [VSP Manager] ←→ [TCP网络] ←→ [远程PC调试软件]
```

### 2. 串口设备物联网接入

```
[串口传感器] ←→ [VSP Manager] ←→ [TCP服务器] ←→ [云平台]
```

### 3. 串口设备远程共享

```
[串口设备] ←→ [VSP Manager(服务端)] ←→ [TCP] ←→ [VSP Manager(客户端)] ←→ [远程应用]
```

## 技术栈

- **Go 1.21+** - 开发语言
- **Fyne** - 跨平台GUI框架
- **tarm/serial** - 串口通信库

## 注意事项

1. **Windows虚拟串口**: 建议使用 com0com 创建虚拟串口对进行测试
2. **Linux串口权限**: 可能需要将用户加入 dialout 用户组
3. **GUI版本**: 需要Windows + Visual Studio环境编译

## 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件

## 作者

韦永博

---

如有问题，请提交 Issue 或 Pull Request。
