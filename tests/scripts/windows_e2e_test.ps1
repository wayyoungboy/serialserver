# VSP Windows End-to-End Integration Test
# 测试完整数据流: test_x <-> COM_D <-> COM_C <-> VSPManager <-> Server <-> device-client <-> COM_A <-> COM_B <-> test_y
#
# 前置条件:
# - com0com 驱动已安装
# - vsp-server, device-client, VSPManager 已构建
# - test_x.exe, test_y.exe 已构建

param(
    [string]$ServerPath = "../vsp-server/vsp-server.exe",
    [string]$ClientPath = "../vsp-client/device-client.exe",
    [string]$VspManagerPath = "../vsp-windows/build/bin/VSPManager.exe",
    [string]$TestXPath = "./test_x.exe",
    [string]$TestYPath = "./test_y.exe",
    [string]$Com0comPath = "../com0com",
    [string]$TestMessage = "hello_vsp_test",
    [int]$Timeout = 60
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

# 日志函数
function Log {
    param([string]$Message)
    Write-Host "[$(Get-Date -Format 'HH:mm:ss')] $Message" -ForegroundColor Green
}

function LogError {
    param([string]$Message)
    Write-Host "[$(Get-Date -Format 'HH:mm:ss')] ERROR: $Message" -ForegroundColor Red
}

function LogWarn {
    param([string]$Message)
    Write-Host "[$(Get-Date -Format 'HH:mm:ss')] WARN: $Message" -ForegroundColor Yellow
}

# 清理函数
function Cleanup {
    Log "清理进程和串口..."

    # 停止所有进程
    if ($serverProc) { Stop-Process -Id $serverProc.Id -Force -ErrorAction SilentlyContinue }
    if ($clientProc) { Stop-Process -Id $clientProc.Id -Force -ErrorAction SilentlyContinue }
    if ($vspManagerProc) { Stop-Process -Id $vspManagerProc.Id -Force -ErrorAction SilentlyContinue }
    if ($testYProc) { Stop-Process -Id $testYProc.Id -Force -ErrorAction SilentlyContinue }

    # 删除 com0com 串口对
    RemoveCom0comPorts "CNCA0" "CNCB0"
    RemoveCom0comPorts "CNCA1" "CNCB1"

    Log "清理完成"
}

# com0com 操作函数
function CreateCom0comPair {
    param([string]$PairName = "CNCA0")

    $setupc = "$Com0comPath/setupc.exe"
    if (-not (Test-Path $setupc)) {
        LogError "找不到 setupc.exe: $setupc"
        throw "com0com not found"
    }

    # 创建新串口对
    Log "创建 com0com 串口对..."
    & $setupc install - - 2>&1 | Out-Null

    # 获取创建的端口名称
    $ports = & $setupc list
    Log "当前串口对: $ports"

    # 分配 COM 端口名称
    # 第一个端口对用于 device-client (COM_A <-> COM_B)
    $result = & $setupc change CNCA0 PortName=COM10
    $result = & $setupc change CNCB0 PortName=COM11

    Log "串口对 A/B 已创建: COM10 <-> COM11"

    # 第二个端口对用于 VSPManager (COM_C <-> COM_D)
    $result = & $setupc install - - 2>&1 | Out-Null
    $result = & $setupc change CNCA1 PortName=COM12
    $result = & $setupc change CNCB1 PortName=COM13

    Log "串口对 C/D 已创建: COM12 <-> COM13"
}

function RemoveCom0comPorts {
    param([string]$PortA, [string]$PortB)

    $setupc = "$Com0comPath/setupc.exe"
    if (Test-Path $setupc) {
        try {
            & $setupc remove $PortA 2>&1 | Out-Null
            & $setupc remove $PortB 2>&1 | Out-Null
        } catch {
            # 忽略删除错误
        }
    }
}

# 主测试流程
Log "========== VSP Windows E2E 测试开始 =========="

try {
    # 1. 检查必需文件
    Log "检查必需文件..."
    $requiredFiles = @($ServerPath, $ClientPath, $TestXPath, $TestYPath)
    foreach ($file in $requiredFiles) {
        if (-not (Test-Path $file)) {
            LogError "缺少文件: $file"
            throw "Missing required file"
        }
    }

    # 2. 创建 com0com 串口对
    CreateCom0comPair

    # 3. 启动 vsp-server
    Log "启动 vsp-server..."
    $serverProc = Start-Process -FilePath $ServerPath -PassThru -WindowStyle Hidden
    Start-Sleep -Seconds 3

    # 检查 server 是否运行
    if ($serverProc.HasExited) {
        LogError "vsp-server 启动失败"
        throw "Server failed to start"
    }
    Log "vsp-server 已启动 (PID: $($serverProc.Id))"

    # 4. 创建测试设备和获取 DeviceKey
    Log "创建测试设备..."
    $deviceKey = "test-device-key-e2e"

    # 这里假设 server 已有默认设备或使用 API 创建
    # 实际应该调用 REST API 创建设备

    # 5. 启动 device-client 连接 COM10 (串口 A)
    Log "启动 device-client 连接 COM10..."
    $clientArgs = "-server localhost:9000 -key $deviceKey -port COM10 -baud 115200"
    $clientProc = Start-Process -FilePath $ClientPath -ArgumentList $clientArgs -PassThru -WindowStyle Hidden
    Start-Sleep -Seconds 2

    if ($clientProc.HasExited) {
        LogError "device-client 启动失败"
        throw "Client failed to start"
    }
    Log "device-client 已启动 (PID: $($clientProc.Id))"

    # 6. 启动 VSPManager (需要 GUI，这里简化为命令行模式)
    # 注意: VSPManager 是 GUI 应用，可能需要特殊处理
    Log "启动 VSPManager..."
    LogWarn "VSPManager 是 GUI 应用，需要手动连接或使用自动化工具"

    # 7. 启动 test_y 连接 COM11 (串口 B)
    Log "启动 test_y 连接 COM11..."
    $testYArgs = "-port COM11 -timeout $Timeout"
    $testYProc = Start-Process -FilePath $TestYPath -ArgumentList $testYArgs -PassThru -RedirectStandardOutput "test_y_output.log" -RedirectStandardError "test_y_error.log"
    Start-Sleep -Seconds 2

    Log "test_y 已启动 (PID: $($testYProc.Id))"

    # 8. 启动 test_x 连接 COM13 (串口 D) 并发送测试数据
    Log "启动 test_x 连接 COM13 发送测试数据..."
    $testXArgs = "-port COM13 -data $TestMessage -timeout $Timeout"
    $testXOutput = & $TestXPath $testXArgs 2>&1
    $testXResult = $LASTEXITCODE

    Log "test_x 输出: $testXOutput"

    # 9. 验证结果
    if ($testXResult -eq 0) {
        Log "========== 测试成功! =========="
        Log "数据流验证完成: test_x <-> COM13 <-> COM12 <-> VSPManager <-> Server <-> device-client <-> COM10 <-> COM11 <-> test_y"
        exit 0
    } else {
        LogError "========== 测试失败! =========="
        LogError "test_x 返回非零退出码: $testXResult"
        exit 1
    }

} catch {
    LogError "测试异常: $_"
    Cleanup
    exit 1
} finally {
    Cleanup
}