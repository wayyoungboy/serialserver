# VSP 端到端测试指南

## 数据流图

```
测试程序X <---> 串口D <---> 串口C <---> VSPManager <---> VSP-Server <---> device-client <---> 串口A <---> 串口B <---> 测试程序Y
    │                                                                                                    │
    └── 发送数据 "Hello" ──────────────────────────────────────────────────────────────────────────────────┘
                                                                                                    └── 返回 "get Hello"
```

## 测试程序说明

### 测试程序X (test_x.exe)
- 功能: 向串口写入数据，读取并验证响应
- 用法: `test_x.exe -port COM10 -baud 115200 -data "Hello" -count 5`
- 预期: 收到 "get <发送的数据>" 的响应

### 测试程序Y (test_y.exe)
- 功能: 接收串口数据，返回 "get <接收到的数据>"
- 用法: `test_y.exe -port COM5 -baud 115200 -v`

## 手动测试步骤

### 1. 启动服务器
```powershell
cd vsp-server
.\vsp-server.exe
```

### 2. 创建测试设备
打开浏览器访问 http://localhost:9000 或使用 API:
```powershell
# 登录获取token
$token = (Invoke-RestMethod -Uri "http://localhost:9000/api/v1/auth/login" `
    -Method POST -Body '{"username":"admin","password":"admin123"}' -ContentType "application/json").data.token

# 创建设备
$device = Invoke-RestMethod -Uri "http://localhost:9000/api/v1/devices" `
    -Method POST -Body '{"name":"E2ETest","serial_port":"VIRTUAL","baud_rate":115200}' `
    -Headers @{Authorization="Bearer $token"} -ContentType "application/json"

$device.data.device_key
```

### 3. 创建串口对 A-B (使用 com0com)
```powershell
# 如果安装了com0com
cd "C:\Program Files (x86)\com0com"
.\setupc.exe install - -

# 设置端口名
.\setupc.exe change CNCA0 PortName=COM#
.\setupc.exe change CNCB0 PortName=-
```
假设得到: A=COM5 (可见), B=CNCA0 (隐藏，供device-client内部使用)

### 4. 启动 device-client
```powershell
cd vsp-client
.\device-client.exe -server localhost:9000 -key <device_key>
```

### 5. 启动 VSPManager
```powershell
cd vsp-windows-go\build\bin
.\VSPManager.exe
```
在 VSPManager 中:
1. 登录 (admin/admin123)
2. 选择设备并连接
3. 记录创建的虚拟COM口 (假设为 COM10)

### 6. 启动测试程序Y (连接串口B)
```powershell
cd tests\test_programs
.\test_y.exe -port COM5 -baud 115200 -v
```

### 7. 运行测试程序X (连接串口D)
```powershell
cd tests\test_programs
.\test_x.exe -port COM10 -baud 115200 -data "HelloVSP" -count 5
```

### 8. 预期结果
```
--- 第 1/5 次测试 ---
发送: HelloVSP_1 (12 bytes)
接收: get HelloVSP_1 (延迟: 45.123ms)
✓ 测试通过: 响应正确

--- 第 2/5 次测试 ---
...
```

## 一键测试脚本

```powershell
cd tests\scripts
.\e2e_test.ps1
```

该脚本会自动执行完整测试流程。

## 常见问题

### Q: test_y 收不到数据
检查:
1. 串口名是否正确
2. 波特率是否一致
3. device-client 是否连接成功

### Q: test_x 超时无响应
检查:
1. VSPManager 是否已连接设备
2. 虚拟COM口是否创建成功
3. 整个数据链路是否连通

### Q: 串口被占用
```powershell
# 查看串口占用
mode
# 或使用设备管理器
```