# VSP 端到端测试脚本
# 测试完整数据链路

param(
    [string]$ServerPath = "..\vsp-server\vsp-server.exe",
    [string]$ClientPath = "..\vsp-client\device-client.exe",
    [string]$VSPManagerPath = "..\vsp-windows-go\build\bin\VSPManager.exe",
    [string]$Com0ComPath = "C:\Program Files (x86)\com0com\setupc.exe",
    [int]$ServerPort = 9000,
    [string]$AdminUser = "admin",
    [string]$AdminPass = "admin123"
)

$ErrorActionPreference = "Continue"

# ============== 工具函数 ==============

function Write-Header {
    param([string]$Text)
    Write-Host "`n========== $Text ==========" -ForegroundColor Cyan
}

function Write-Step {
    param([string]$Step, [string]$Text)
    Write-Host "[$Step] $Text" -ForegroundColor Yellow
}

function Write-Success {
    param([string]$Text)
    Write-Host "  ✓ $Text" -ForegroundColor Green
}

function Write-Fail {
    param([string]$Text)
    Write-Host "  ✗ $Text" -ForegroundColor Red
}

function Write-Info {
    param([string]$Text)
    Write-Host "  → $Text" -ForegroundColor Gray
}

function Invoke-API {
    param(
        [string]$Endpoint,
        [string]$Method = "GET",
        [object]$Body = $null,
        [string]$Token = $null
    )

    $url = "http://localhost:${ServerPort}/api/v1$Endpoint"
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

function Test-Com0Com {
    if (-not (Test-Path $Com0ComPath)) {
        $Com0ComPath = "C:\Program Files\com0com\setupc.exe"
    }
    if (-not (Test-Path $Com0ComPath)) {
        Write-Fail "com0com 未安装"
        Write-Info "请从 https://sourceforge.net/projects/com0com/ 下载安装"
        return $null
    }
    return $Com0ComPath
}

function New-ComPortPair {
    param([string]$VisiblePrefix, [string]$HiddenPrefix)

    $setupc = Test-Com0Com
    if (-not $setupc) { return $null }

    # 创建端口对
    $output = & $setupc "install" "-" "-" 2>&1
    Start-Sleep -Milliseconds 500

    # 获取新创建的端口
    $listOutput = & $setupc "list" 2>&1
    $lines = $listOutput -split "`n"

    $lastCNCA = ""
    $lastCNCB = ""
    foreach ($line in $lines) {
        if ($line -match "CNCA(\d+)") {
            $lastCNCA = "CNCA" + $matches[1]
        }
        if ($line -match "CNCB(\d+)") {
            $lastCNCB = "CNCB" + $matches[1]
        }
    }

    if (-not $lastCNCA -or -not $lastCNCB) {
        return $null
    }

    # 设置可见端口名 (自动分配COM号)
    $output = & $setupc "change" $lastCNCA "PortName=COM#" 2>&1
    Start-Sleep -Milliseconds 300

    # 设置隐藏端口
    $output = & $setupc "change" $lastCNCB "PortName=-" 2>&1
    Start-Sleep -Milliseconds 300

    # 获取实际分配的COM号
    $listOutput = & $setupc "list" 2>&1
    $visiblePort = ""
    foreach ($line in $listOutput -split "`n") {
        if ($line -match "RealPortName=(COM\d+)") {
            $visiblePort = $matches[1]
        }
    }

    return @{
        Visible = $visiblePort
        HiddenA = $lastCNCA
        HiddenB = $lastCNCB
    }
}

function Remove-ComPortPair {
    param([string]$PortName)

    $setupc = Test-Com0Com
    if (-not $setupc) { return }

    & $setupc "uninstall" $PortName 2>&1 | Out-Null
}

# ============== 主测试流程 ==============

Write-Host @"

╔══════════════════════════════════════════════════════════════╗
║              VSP 端到端测试                                   ║
║                                                              ║
║  数据流:                                                     ║
║  测试X <-> 串口D <-> 串口C <-> VSPManager <-> Server          ║
║          <-> device-client <-> 串口A <-> 串口B <-> 测试Y      ║
╚══════════════════════════════════════════════════════════════╝

"@ -ForegroundColor White

# 记录进程和资源
$processes = @()
$portPairs = @()

try {
    # ============ 步骤1: 启动服务器 ============
    Write-Header "步骤1: 启动 VSP Server"

    Write-Step "1.1" "启动服务器进程"
    $serverProcess = Start-Process -FilePath $ServerPath -PassThru -WindowStyle Normal
    $processes += $serverProcess
    Write-Success "服务器已启动 (PID: $($serverProcess.Id))"
    Start-Sleep -Seconds 3

    Write-Step "1.2" "验证服务器连接"
    $result = Invoke-API -Endpoint "/devices"
    if ($result.Success) {
        Write-Success "服务器API可用"
    } else {
        Write-Fail "服务器API不可用: $($result.Error)"
        throw "服务器启动失败"
    }

    # ============ 步骤2: 登录并创建设备 ============
    Write-Header "步骤2: 创建测试设备"

    Write-Step "2.1" "用户登录"
    $result = Invoke-API -Endpoint "/auth/login" -Method "POST" -Body @{
        username = $AdminUser
        password = $AdminPass
    }
    if (-not $result.Success) {
        throw "登录失败: $($result.Error)"
    }
    $token = $result.Data.data.token
    Write-Success "登录成功"

    Write-Step "2.2" "创建设备"
    $deviceName = "E2ETest_$(Get-Date -Format 'HHmmss')"
    $result = Invoke-API -Endpoint "/devices" -Method "POST" -Body @{
        name = $deviceName
        serial_port = "VIRTUAL"
        baud_rate = 115200
    } -Token $token

    if (-not $result.Success) {
        throw "创建设备失败: $($result.Error)"
    }
    $deviceKey = $result.Data.data.device_key
    $deviceId = $result.Data.data.id
    Write-Success "设备已创建: $deviceName"
    Write-Info "Device Key: $deviceKey"

    # ============ 步骤3: 创建串口对A-B (device-client用) ============
    Write-Header "步骤3: 创建串口对 A-B"

    Write-Step "3.1" "创建端口对"
    $pairAB = New-ComPortPair -VisiblePrefix "AB"
    if (-not $pairAB) {
        throw "创建串口对A-B失败"
    }
    $portPairs += $pairAB
    Write-Success "端口对A-B已创建"
    Write-Info "可见端口: $($pairAB.Visible) (测试Y连接)"
    Write-Info "隐藏端口: $($pairAB.HiddenB) (device-client连接)"

    # ============ 步骤4: 启动 device-client ============
    Write-Header "步骤4: 启动 device-client"

    Write-Step "4.1" "启动设备客户端"
    $clientArgs = @(
        "-server", "localhost:${ServerPort}",
        "-key", $deviceKey
    )

    # 注意: device-client 需要连接到隐藏端口
    # 这里我们使用一个模拟的方式 - 实际使用时需要修改device-client支持指定端口
    $deviceClientProcess = Start-Process -FilePath $ClientPath `
        -ArgumentList $clientArgs `
        -PassThru -WindowStyle Minimized
    $processes += $deviceClientProcess
    Write-Success "device-client 已启动 (PID: $($deviceClientProcess.Id))"
    Start-Sleep -Seconds 2

    # ============ 步骤5: 启动 VSPManager ============
    Write-Header "步骤5: 启动 VSPManager"

    Write-Step "5.1" "启动 VSPManager"
    if (Test-Path $VSPManagerPath) {
        $vspManagerProcess = Start-Process -FilePath $VSPManagerPath -PassThru
        $processes += $vspManagerProcess
        Write-Success "VSPManager 已启动 (PID: $($vspManagerProcess.Id))"
        Write-Info "请在 VSPManager 中:"
        Write-Info "  1. 登录 (admin/admin123)"
        Write-Info "  2. 选择设备 '$deviceName' 并连接"
        Write-Info "  3. 记录创建的虚拟COM口号"
    } else {
        Write-Fail "VSPManager.exe 不存在"
        Write-Info "请手动编译 vsp-windows-go"
    }

    # ============ 步骤6: 等待用户操作 ============
    Write-Header "步骤6: 手动配置"

    Write-Host "`n请完成以下操作后继续:`n" -ForegroundColor Yellow
    Write-Host "  1. 在 VSPManager 中连接设备"
    Write-Host "  2. 记录虚拟COM口号 (串口C/D)"
    Write-Host ""

    $portD = Read-Host "请输入 VSPManager 创建的可见COM口号 (如 COM10)"
    $portB = $pairAB.Visible

    # ============ 步骤7: 启动测试程序Y ============
    Write-Header "步骤7: 启动测试程序Y"

    Write-Step "7.1" "编译测试程序"
    $testYPath = ".\test_programs\test_y\test_y.exe"
    if (-not (Test-Path $testYPath)) {
        Push-Location ".\test_programs"
        go build -o test_y.exe .\test_y\
        Pop-Location
    }

    Write-Step "7.2" "启动测试程序Y (连接 $portB)"
    $testYProcess = Start-Process -FilePath $testYPath `
        -ArgumentList "-port", $portB, "-baud", "115200", "-v" `
        -PassThru -WindowStyle Normal
    $processes += $testYProcess
    Write-Success "测试程序Y 已启动 (连接 $portB)"
    Start-Sleep -Seconds 1

    # ============ 步骤8: 运行测试程序X ============
    Write-Header "步骤8: 运行测试程序X"

    Write-Step "8.1" "编译测试程序"
    $testXPath = ".\test_programs\test_x\test_x.exe"
    if (-not (Test-Path $testXPath)) {
        Push-Location ".\test_programs"
        go build -o test_x.exe .\test_x\
        Pop-Location
    }

    Write-Step "8.2" "运行测试程序X (连接 $portD)"
    Write-Host "`n开始数据传输测试...`n" -ForegroundColor Yellow

    $testOutput = & $testXPath "-port" $portD "-baud" "115200" "-data" "VSP_TEST" "-count" "5" 2>&1
    Write-Host $testOutput

    # ============ 结果汇总 ============
    Write-Header "测试结果"

    if ($LASTEXITCODE -eq 0) {
        Write-Success "端到端测试通过!"
    } else {
        Write-Fail "端到端测试失败"
    }

} catch {
    Write-Fail "测试出错: $_"
} finally {
    # ============ 清理 ============
    Write-Header "清理环境"

    Write-Info "停止测试程序..."
    foreach ($p in $processes) {
        if ($p -and !$p.HasExited) {
            Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
        }
    }

    Write-Info "删除串口对..."
    foreach ($pair in $portPairs) {
        Remove-ComPortPair -PortName $pair.HiddenB
    }

    Write-Info "删除测试设备..."
    if ($token -and $deviceId) {
        Invoke-API -Endpoint "/devices/$deviceId" -Method "DELETE" -Token $token | Out-Null
    }

    Write-Success "清理完成"
}

Write-Host "`n测试结束`n" -ForegroundColor Green