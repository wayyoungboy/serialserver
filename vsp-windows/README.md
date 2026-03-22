# VSP Windows 端

Windows 客户端组件，包含内核驱动和 WPF GUI 管理器。

## 组件

| 组件 | 目录 | 说明 |
|------|------|------|
| **VSPDriver** | `VSPDriver/` | Windows 内核驱动，创建虚拟串口 |
| **VSPManager** | `VSPManager/` | WPF GUI 应用，连接服务器管理虚拟串口 |
| **VSPManager.Core** | `VSPManager.Core/` | 核心类库，驱动通信和网络连接 |

## 目录结构

```
vsp-windows/
├── VSPDriver/                    # 内核驱动
│   ├── src/
│   │   ├── driver/              # 驱动源码
│   │   │   ├── driver.cpp       # 主驱动实现
│   │   │   ├── ringbuffer.cpp   # 环形缓冲区
│   │   │   └── bin/Release/
│   │   │       └── VSPDriver.sys
│   │   └── inf/
│   │       └── VSPDriver.inf
│   └── README.md
│
├── VSPManager/                   # WPF GUI
│   ├── MainWindow.xaml          # 主窗口
│   ├── ViewModels/              # MVVM 视图模型
│   ├── Services/                # 服务层
│   └── bin/Release/
│       └── VSPManager.exe
│
├── VSPManager.Core/              # 核心类库
│   ├── Driver/                  # 驱动客户端
│   ├── Network/                 # TCP 客户端
│   └── Configuration/           # 配置管理
│
└── VSPManager.sln               # Visual Studio 解决方案
```

## 构建依赖

- Visual Studio 2022
- Windows Driver Kit (WDK)
- .NET 8 SDK
- Windows SDK 10.0.22621.0

## 构建

### 内核驱动

```powershell
cd VSPDriver/src
& "D:\Microsoft Visual Studio\2022\Community\MSBuild\Current\Bin\MSBuild.exe" VSPDriver.sln -p:Configuration=Release -p:Platform=x64
```

### WPF GUI

```powershell
dotnet build VSPManager.sln -c Release
```

## 安装驱动 (首次使用)

**管理员 PowerShell:**

```powershell
# 启用测试签名模式
bcdedit /set testsigning on
# 重启电脑

# 复制驱动
copy "VSPDriver\src\driver\bin\Release\VSPDriver.sys" "C:\Windows\System32\drivers\"

# 创建测试证书
$cert = New-SelfSignedCertificate -Type CodeSigningCert -Subject "CN=VSPTestCert" -CertStoreLocation "Cert:\CurrentUser\My"

# 签名驱动
Set-AuthenticodeSignature -FilePath "C:\Windows\System32\drivers\VSPDriver.sys" -Certificate $cert

# 创建并启动服务
sc.exe create VSPDriver type= kernel start= demand binPath= "C:\Windows\System32\drivers\VSPDriver.sys"
sc.exe start VSPDriver
```

## 使用

1. 启动驱动: `sc.exe start VSPDriver`
2. 运行 VSPManager: `.\VSPManager.exe`
3. 配置服务器地址和 DeviceKey
4. 点击"连接"
5. 用串口工具连接 `\\.\VSP1`

## 驱动管理命令

```powershell
sc.exe start VSPDriver     # 启动驱动
sc.exe stop VSPDriver      # 停止驱动
sc.exe query VSPDriver     # 查看状态
sc.exe delete VSPDriver    # 删除服务
```