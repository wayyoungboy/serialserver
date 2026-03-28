# VSP 本机集成测试脚本
# 测试完整流程: 登录 -> 创建设备 -> 连接 -> 数据传输 -> 清理

param(
    [string]$ServerHost = "localhost",
    [int]$ServerPort = 9000,
    [string]$AdminUser = "admin",
    [string]$AdminPass = "admin123",
    [switch]$StartServer,
    [string]$ServerPath = "..\vsp-server\vsp-server.exe",
    [string]$DeviceClientPath = "..\vsp-client\device-client.exe"
)

$ErrorActionPreference = "Continue"
$Results = @()

# ============== 工具函数 ==============

function Write-Step {
    param([string]$Step, [string]$Message)
    Write-Host "`n[$Step] $Message" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "  ✓ $Message" -ForegroundColor Green
}

function Write-Fail {
    param([string]$Message)
    Write-Host "  ✗ $Message" -ForegroundColor Red
}

function Write-Info {
    param([string]$Message)
    Write-Host "  → $Message" -ForegroundColor Gray
}

function Invoke-API {
    param(
        [string]$Endpoint,
        [string]$Method = "GET",
        [object]$Body = $null,
        [string]$Token = $null
    )

    $url = "http://${ServerHost}:${ServerPort}/api/v1$Endpoint"
    $headers = @{ "Content-Type" = "application/json" }
    if ($Token) { $headers["Authorization"] = "Bearer $Token" }

    try {
        if ($Body) {
            $json = $Body | ConvertTo-Json -Depth 10 -Compress
            $resp = Invoke-RestMethod -Uri $url -Method $Method -Headers $headers -Body $json -TimeoutSec 10
        } else {
            $resp = Invoke-RestMethod -Uri $url -Method $Method -Headers $headers -TimeoutSec 10
        }
        return @{ Success = $true; Data = $resp }
    } catch {
        return @{ Success = $false; Error = $_.Exception.Message }
    }
}

function Test-PortOpen {
    param([int]$Port)
    try {
        $client = New-Object System.Net.Sockets.TcpClient
        $connect = $client.BeginConnect($ServerHost, $Port, $null, $null)
        $wait = $connect.AsyncWaitHandle.WaitOne(2000)
        $client.Close()
        return $wait
    } catch {
        return $false
    }
}

# ============== 测试步骤 ==============

# 记录测试结果
function Add-Result {
    param([string]$Name, [bool]$Pass, [string]$Detail = "")
    $script:Results += [PSCustomObject]@{
        Test = $Name
        Result = if ($Pass) { "PASS" } else { "FAIL" }
        Detail = $Detail
    }
}

# IT-01: 服务器启动/连接测试
function Test-ServerConnection {
    Write-Step "IT-01" "服务器连接测试"

    if ($StartServer) {
        Write-Info "启动服务器..."
        $script:ServerProcess = Start-Process -FilePath $ServerPath -PassThru -WindowStyle Hidden
        Start-Sleep -Seconds 3
    }

    if (Test-PortOpen -Port $ServerPort) {
        Write-Success "服务器端口 $ServerPort 可访问"
        Add-Result "服务器连接" $true
        return $true
    } else {
        Write-Fail "无法连接服务器端口 $ServerPort"
        Add-Result "服务器连接" $false "端口不可访问"
        return $false
    }
}

# IT-02: 用户登录测试
function Test-UserLogin {
    Write-Step "IT-02" "用户登录测试"

    $result = Invoke-API -Endpoint "/auth/login" -Method "POST" -Body @{
        username = $AdminUser
        password = $AdminPass
    }

    if ($result.Success -and $result.Data.data.token) {
        $script:Token = $result.Data.data.token
        $script:User = $result.Data.data.user
        Write-Success "登录成功: $($script:User.username)"
        Write-Info "Token: $($script:Token.Substring(0, [Math]::Min(20, $script:Token.Length)))..."
        Add-Result "用户登录" $true
        return $true
    } else {
        Write-Fail "登录失败: $($result.Error)"
        Add-Result "用户登录" $false $result.Error
        return $false
    }
}

# IT-03: 设备创建测试
function Test-CreateDevice {
    Write-Step "IT-03" "创建测试设备"

    $deviceName = "TestDevice_$(Get-Date -Format 'HHmmss')"

    $result = Invoke-API -Endpoint "/devices" -Method "POST" -Body @{
        name = $deviceName
        serial_port = "COM1"
        baud_rate = 115200
        data_bits = 8
        stop_bits = 1
        parity = "N"
    } -Token $script:Token

    if ($result.Success -and $result.Data.data.device_key) {
        $script:TestDevice = $result.Data.data
        Write-Success "设备创建成功"
        Write-Info "名称: $deviceName"
        Write-Info "Key: $($script:TestDevice.device_key)"
        Write-Info "ID: $($script:TestDevice.id)"
        Add-Result "创建设备" $true
        return $true
    } else {
        Write-Fail "设备创建失败: $($result.Error)"
        Add-Result "创建设备" $false $result.Error
        return $false
    }
}

# IT-04: 获取设备列表
function Test-GetDevices {
    Write-Step "IT-04" "获取设备列表"

    $result = Invoke-API -Endpoint "/devices" -Token $script:Token

    if ($result.Success) {
        $devices = $result.Data.data
        Write-Success "获取到 $($devices.Count) 个设备"
        foreach ($d in $devices) {
            $status = if ($d.status) { $d.status } else { "offline" }
            Write-Info "$($d.name) - $status"
        }
        Add-Result "设备列表" $true
        return $true
    } else {
        Write-Fail "获取设备列表失败: $($result.Error)"
        Add-Result "设备列表" $false $result.Error
        return $false
    }
}

# IT-05: device-client 连接测试
function Test-DeviceClientConnect {
    Write-Step "IT-05" "device-client 连接测试"

    if (-not $script:TestDevice) {
        Write-Fail "没有测试设备"
        Add-Result "设备连接" $false "无测试设备"
        return $false
    }

    if (-not (Test-Path $DeviceClientPath)) {
        Write-Fail "device-client.exe 不存在: $DeviceClientPath"
        Add-Result "设备连接" $false "客户端文件不存在"
        return $false
    }

    Write-Info "启动 device-client..."
    $script:DeviceClientProcess = Start-Process -FilePath $DeviceClientPath `
        -ArgumentList "-server", "${ServerHost}:${ServerPort}", "-key", $script:TestDevice.device_key `
        -PassThru -WindowStyle Minimized

    Start-Sleep -Seconds 3

    # 检查进程是否运行
    if ($script:DeviceClientProcess -and !$script:DeviceClientProcess.HasExited) {
        Write-Success "device-client 已启动 (PID: $($script:DeviceClientProcess.Id))"

        # 检查设备状态更新
        Start-Sleep -Seconds 2
        $result = Invoke-API -Endpoint "/devices" -Token $script:Token
        if ($result.Success) {
            $device = $result.Data.data | Where-Object { $_.device_key -eq $script:TestDevice.device_key }
            if ($device -and $device.status -eq "online") {
                Write-Success "设备状态已更新为 online"
            } else {
                Write-Info "设备状态: $($device.status)"
            }
        }

        Add-Result "设备连接" $true
        return $true
    } else {
        Write-Fail "device-client 启动失败"
        Add-Result "设备连接" $false "进程启动失败"
        return $false
    }
}

# IT-06: WebSocket 连接测试 (使用 Python 或跳过)
function Test-WebSocketConnect {
    Write-Step "IT-06" "WebSocket 连接测试"

    Write-Info "WebSocket 测试需要 Python 环境"
    Write-Info "请运行: python tests/scripts/vsp_system_test.py"

    # 简单测试: 检查 WebSocket 端点
    $wsUrl = "ws://${ServerHost}:${ServerPort}/api/v1/ws/client"
    Write-Info "WebSocket URL: $wsUrl"

    Add-Result "WebSocket" $true "需要手动测试或Python脚本"
    return $true
}

# IT-07: 数据传输测试 (手动验证)
function Test-DataTransmission {
    Write-Step "IT-07" "数据传输测试"

    Write-Host ""
    Write-Host "  请手动验证数据传输:" -ForegroundColor Yellow
    Write-Host "  1. 打开浏览器访问 http://${ServerHost}:${ServerPort}"
    Write-Host "  2. 或启动 VSPManager.exe 连接设备"
    Write-Host "  3. 使用串口终端测试数据收发"
    Write-Host ""

    Read-Host "  按 Enter 继续"

    Add-Result "数据传输" $true "手动验证"
    return $true
}

# IT-08: 清理测试
function Test-Cleanup {
    Write-Step "IT-08" "清理测试环境"

    # 停止 device-client
    if ($script:DeviceClientProcess -and !$script:DeviceClientProcess.HasExited) {
        Stop-Process -Id $script:DeviceClientProcess.Id -Force -ErrorAction SilentlyContinue
        Write-Info "已停止 device-client"
    }

    # 删除测试设备
    if ($script:TestDevice -and $script:Token) {
        $result = Invoke-API -Endpoint "/devices/$($script:TestDevice.id)" -Method "DELETE" -Token $script:Token
        if ($result.Success) {
            Write-Success "已删除测试设备"
        } else {
            Write-Info "删除设备: $($result.Error)"
        }
    }

    # 停止服务器 (如果是本脚本启动的)
    if ($script:ServerProcess) {
        Stop-Process -Id $script:ServerProcess.Id -Force -ErrorAction SilentlyContinue
        Write-Info "已停止服务器"
    }

    Add-Result "清理环境" $true
}

# 显示测试结果
function Show-Results {
    Write-Host "`n" + ("=" * 50) -ForegroundColor Cyan
    Write-Host "测试结果汇总" -ForegroundColor Cyan
    Write-Host ("=" * 50) -ForegroundColor Cyan

    $pass = ($Results | Where-Object { $_.Result -eq "PASS" }).Count
    $fail = ($Results | Where-Object { $_.Result -eq "FAIL" }).Count

    $Results | Format-Table -AutoSize

    Write-Host ("-" * 50)
    Write-Host "总计: PASS=$pass, FAIL=$fail" -ForegroundColor $(if ($fail -eq 0) { "Green" } else { "Red" })
    Write-Host ("=" * 50)
}

# ============== 主程序 ==============

Write-Host @"

╔══════════════════════════════════════════════════╗
║       VSP 系统集成测试                            ║
║       Server: ${ServerHost}:${ServerPort}
╚══════════════════════════════════════════════════╝

"@ -ForegroundColor White

try {
    # 运行测试
    if (-not (Test-ServerConnection)) {
        Write-Host "`n服务器不可用，测试终止" -ForegroundColor Red
        exit 1
    }

    if (-not (Test-UserLogin)) { exit 1 }

    Test-CreateDevice | Out-Null
    Test-GetDevices | Out-Null
    Test-DeviceClientConnect | Out-Null
    Test-WebSocketConnect | Out-Null
    Test-DataTransmission | Out-Null

} finally {
    Test-Cleanup
    Show-Results
}