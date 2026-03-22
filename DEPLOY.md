# VSP 虚拟串口系统 - 部署与使用指南

## 系统架构

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              VSP Serial Server System                        │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────┐         TCP网络          ┌─────────────┐         TCP网络          ┌─────────────┐
│   设备端     │ ◄──────────────────────► │   服务器端   │ ◄──────────────────────► │  Windows端  │
│ (VSP Manager)│                          │ (VSP Server)│                          │ (VSP Client)│
│  读取物理串口 │                          │   数据转发   │                          │  虚拟串口   │
└─────────────┘                          └─────────────┘                          └─────────────┘
```

## 两种虚拟串口方案对比

| 方案 | Windows端组件 | 优点 | 缺点 |
|------|--------------|------|------|
| **方案A: 自定义驱动** | VSPDriver.sys + VSPManager(WPF) | 真正的虚拟串口，无需第三方软件 | 需要签名驱动，部署稍复杂 |
| **方案B: com0com** | com0com + vsp-client(Go) | 部署简单，无需签名 | 依赖第三方驱动 |

---

## 方案A: 自定义驱动 (推荐)

### 系统架构

```
[设备端]                           [远程端]
┌──────────┐    TCP/IP         ┌──────────────┐
│  PLC设备  │                   │  调试软件     │
│  传感器   │                   │  (串口工具)   │
└────┬─────┘                   └──────┬───────┘
     │                                │
     ▼                                ▼
┌──────────┐    TCP/IP         ┌──────────────┐
│vsp-manager│◄────────────────► │ VSPManager   │
│  (Go)    │    vsp-server     │  (WPF GUI)   │
└────┬─────┘                   └──────┬───────┘
     │                                │
     ▼                                ▼
 物理串口 COM3                    VSPDriver.sys
                                 (虚拟串口 VSP1)
```

### 部署步骤

#### 1. 服务器端部署 (vsp-server)

**环境要求**: Linux/Windows, Go 1.21+

```bash
# 编译
cd vsp-server
go build -o vsp-server ./cmd

# 运行 (默认端口9000)
./vsp-server -port 9000

# Linux后台运行
nohup ./vsp-server -port 9000 > server.log 2>&1 &
```

**防火墙**: 开放9000端口

---

#### 2. 设备端部署 (vsp-manager Go版)

**环境要求**: Windows/Linux, Go 1.21+

```bash
# 编译
cd vsp-manager
go build -o vsp-manager ./cmd

# 创建配置文件 config.json
# 运行
./vsp-manager -config config.json

# 访问 Web UI: http://localhost:8080
```

**配置文件** (`config.json`):
```json
{
  "tunnels": [{
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
      "host": "your-server.com",
      "port": 9000
    },
    "enabled": true
  }],
  "ui": { "port": 8080 },
  "api": { "port": 8081 }
}
```

---

#### 3. 远程端部署 (VSPDriver + VSPManager WPF)

**环境要求**: Windows 10/11 x64

**Step 1: 启用测试签名模式** (管理员PowerShell)

```powershell
bcdedit /set testsigning on
# 重启电脑
```

**Step 2: 创建测试证书并签名驱动**

```powershell
cd E:\code\serialserver\vsp-driver\src\driver\bin\Release

# 创建证书
makecert -r -pe -ss PrivateCertStore -n "CN=VSPTestCert" VSPTest.cer

# 签名驱动
signtool sign /s PrivateCertStore /n VSPTestCert /fd SHA256 VSPDriver.sys
```

**Step 3: 安装驱动**

```powershell
# 安装驱动
pnputil /add-driver VSPDriver.inf /install

# 验证: 打开设备管理器 → 端口(COM和LPT) → 应看到 "VSP Virtual Serial Port"
```

**Step 4: 编译并运行VSPManager**

```powershell
cd E:\code\serialserver\vsp-wpf
dotnet build -c Release

# 运行
.\VSPManager\bin\Release\net8.0-windows\VSPManager.exe
```

**Step 5: 连接服务器**

1. 打开VSPManager
2. 设置服务器地址: `your-server.com:9000`
3. 点击"连接"
4. 虚拟串口 `VSP1` 即可使用

---

## 方案B: com0com 方案

### 架构

```
[设备端]                    [远程端]
vsp-manager/vsp-client ──── vsp-client + com0com
    │                            │
    ▼                            ▼
 物理串口                     虚拟串口对 (COM5↔COM6)
```

### 部署步骤

#### 1. 安装 com0com

1. 下载: https://sourceforge.net/projects/com0com/
2. 安装后创建虚拟串口对: COM5 ↔ COM6

```powershell
# 使用 setupc 创建
setupc install PortName=COM5 PortName=COM6
```

#### 2. 运行 vsp-client

```bash
# 编译
cd vsp-client
go build -o vsp-client ./cmd

# 运行
./vsp-client -server your-server.com:9000 -port COM5 -baud 115200
```

#### 3. 使用虚拟串口

- 串口软件连接 `COM6`
- 数据流: COM6 ↔ COM5 ↔ vsp-client ↔ 服务器 ↔ 设备端

---

## 完整使用流程

### 场景: 远程调试工厂PLC

```
时间线:
─────────────────────────────────────────────────────────────►

[部署阶段]                          [使用阶段]
├─ 服务器: 启动vsp-server            ├─ 远程端: 启动VSPManager
├─ 设备端: 启动vsp-manager           ├─ 连接服务器
└─ 远程端: 安装驱动+VSPManager       └─ 打开串口工具连接VSP1调试
```

**详细步骤**:

1. **服务器端**
   ```bash
   ./vsp-server -port 9000
   ```

2. **设备端 (工厂现场)**
   ```bash
   ./vsp-manager -config config.json
   # 配置串口COM3连接PLC
   ```

3. **远程端 (办公室)**
   ```powershell
   # 启动VSPManager
   VSPManager.exe

   # GUI操作:
   # - 服务器: your-server.com:9000
   # - 点击"连接"

   # 打开串口调试软件
   # - 端口: VSP1
   # - 开始调试PLC
   ```

---

## 数据流详解

```
[PLC设备]
    │
    ▼ (串口数据: RS232/485)
[设备端 vsp-manager]
    │
    ▼ (TCP连接, 握手: "WINDOWS" → "OK")
[vsp-server 中继]
    │
    ▼ (TCP连接, 握手: "WINDOWS" → "OK")
[远程端 VSPManager (WPF)]
    │
    ├── DeviceIoControl(IOCTL_VSP_REGISTER_NET_CLIENT)
    │
    ▼ (读写驱动缓冲区)
[VSPDriver.sys]
    │
    ├── RxBuffer (64KB) ← 网络数据写入，应用程序读取
    └── TxBuffer (64KB) ← 应用程序写入，网络读取
    │
    ▼ (虚拟串口 \\.\VSP1)
[调试软件]
```

---

## 端口说明

| 端口 | 组件 | 用途 |
|------|------|------|
| 9000 | vsp-server | 数据透传主端口 |
| 8080 | vsp-manager | Web UI界面 |
| 8081 | vsp-manager | REST API接口 |

---

## 常见问题

### Q1: 驱动安装失败 - 签名验证错误

```
解决:
1. 确认已启用测试签名模式: bcdedit /set testsigning on
2. 重启电脑
3. 使用测试证书签名驱动
```

### Q2: 设备管理器看不到虚拟串口

```
检查:
1. 驱动安装状态: pnputil /enum-drivers | findstr VSP
2. 设备管理器 → 查看 → 显示隐藏的设备
3. 重新安装驱动
```

### Q3: 连接服务器超时

```
检查:
1. 服务器vsp-server是否运行
2. 防火墙是否开放端口
3. 网络连通性: ping your-server.com
```

### Q4: 数据传输中断

```
检查:
1. 网络稳定性
2. 服务器日志
3. VSPManager连接状态
```

---

## 文件清单

```
serialserver/
├── vsp-driver/                    # 自定义内核驱动
│   └── src/driver/bin/Release/
│       ├── VSPDriver.sys          # 驱动文件
│       └── VSPDriver.inf          # 安装文件
│
├── vsp-wpf/                       # WPF GUI (方案A)
│   └── VSPManager/bin/Release/
│       └── VSPManager.exe
│
├── vsp-manager/                   # Go管理器 (设备端)
│   └── vsp-manager.exe
│
├── vsp-server/                    # TCP中继服务器
│   └── vsp-server
│
├── vsp-client/                    # Go客户端 (方案B)
│   └── vsp-client.exe
│
└── DEPLOY.md                      # 本文档
```

---

## 生产环境建议

### 安全
- 启用TLS加密
- 添加设备认证机制
- 限制IP访问

### 高可用
- 多vsp-server实例 + 负载均衡
- 自动重连机制
- 心跳检测

### 监控
- 集成日志系统
- 连接状态监控
- 性能指标采集