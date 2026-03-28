# VSP 本机集成测试

## 快速测试

### 方式1: 一键启动测试 (推荐)

```powershell
cd tests/scripts
.\start_and_test.ps1
```

这会自动启动 vsp-server 并运行所有测试。

### 方式2: 手动启动服务器后测试

```powershell
# 终端1: 启动服务器
cd vsp-server
.\vsp-server.exe

# 终端2: 运行测试
cd tests/scripts
.\integration_test.ps1
```

### 方式3: Python 测试

```powershell
# 确保服务器运行后
cd tests/scripts
pip install websocket-client requests
python simple_test.py
```

## 测试内容

| 测试项 | 描述 |
|--------|------|
| 服务器连接 | 检查端口 9000 是否可访问 |
| 用户登录 | 使用 admin/admin123 登录 |
| 设备列表 | 获取已创建的设备 |
| 创建设备 | 创建临时测试设备 |
| 设备连接 | 启动 device-client 连接 |
| WebSocket | 测试客户端 WebSocket 认证 |
| 数据传输 | 手动验证数据收发 |
| 清理环境 | 删除测试设备 |

## 目录结构

```
tests/
├── INTEGRATION_TEST.md      # 测试说明
├── scripts/
│   ├── start_and_test.ps1   # 一键启动测试
│   ├── integration_test.ps1 # PowerShell 测试脚本
│   └── simple_test.py       # Python 测试脚本
└── mock/                    # Mock 服务器 (可选)
```

## 预期输出

```
╔══════════════════════════════════════════════════╗
║       VSP 系统集成测试                            ║
║       Server: localhost:9000
╚══════════════════════════════════════════════════╝

[IT-01] 服务器连接测试
  ✓ 服务器端口 9000 可访问

[IT-02] 用户登录测试
  ✓ 登录成功: admin

[IT-03] 创建测试设备
  ✓ 设备创建成功

...

==================================================
测试结果汇总
==================================================
Test        Result Detail
----        ------ ------
服务器连接   PASS
用户登录     PASS
创建设备     PASS
...
==================================================
```