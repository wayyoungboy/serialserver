# VSP 完整集成测试脚本

param(
    [string]$ServerHost = "localhost",
    [int]$ServerPort = 9000,
    [string]$AdminUser = "admin",
    [string]$AdminPass = "admin123"
)

$ErrorActionPreference = "Continue"
$Results = @()

function Write-Step { param([string]$S, [string]$M) Write-Host "`n[$S] $M" -ForegroundColor Cyan }
function Write-Success { param([string]$M) Write-Host "  OK $M" -ForegroundColor Green }
function Write-Fail { param([string]$M) Write-Host "  FAIL $M" -ForegroundColor Red }
function Write-Info { param([string]$M) Write-Host "  -> $M" -ForegroundColor Gray }

function Add-Result {
    param([string]$Name, [bool]$Pass, [string]$Detail = "")
    $script:Results += [PSCustomObject]@{
        Test = $Name
        Result = if ($Pass) { "PASS" } else { "FAIL" }
        Detail = $Detail
    }
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

Write-Host "`n========== VSP Integration Test ==========`n" -ForegroundColor White

# Step 1: Server Connection
Write-Step "1" "Server Connection Test"
$r = Invoke-API -Endpoint "/devices"
if ($r.Success -or $r.Error -match "401") {
    Write-Success "Server API is available"
    Add-Result -Name "Server Connection" -Pass $true
} else {
    Write-Fail "Server not available: $($r.Error)"
    Add-Result -Name "Server Connection" -Pass $false -Detail $r.Error
    exit 1
}

# Step 2: User Authentication
Write-Step "2" "User Authentication Test"
$r = Invoke-API -Endpoint "/auth/login" -Method "POST" -Body @{
    username = $AdminUser
    password = $AdminPass
}

if ($r.Success -and $r.Data.data.token) {
    $Token = $r.Data.data.token
    Write-Success "Login successful"
    Write-Info "Token: $($Token.Substring(0,20))..."
    Add-Result -Name "User Login" -Pass $true
} else {
    Write-Fail "Login failed"
    Add-Result -Name "User Login" -Pass $false
    exit 1
}

# Step 3: Device Management
Write-Step "3" "Device Management Test"

# Get device list
$r = Invoke-API -Endpoint "/devices" -Token $Token
if ($r.Success) {
    $deviceCount = $r.Data.data.Count
    Write-Success "Got $deviceCount devices"
    Add-Result -Name "Device List" -Pass $true
}

# Create test device
$deviceName = "TestDevice_$(Get-Date -Format 'HHmmss')"
$r = Invoke-API -Endpoint "/devices" -Method "POST" -Body @{
    name = $deviceName
    serial_port = "VIRTUAL"
    baud_rate = 115200
} -Token $Token

if ($r.Success -and $r.Data.data.device_key) {
    $TestDevice = $r.Data.data
    $DeviceKey = $TestDevice.device_key
    $DeviceId = $TestDevice.id
    Write-Success "Device created"
    Write-Info "ID: $DeviceId, Key: $DeviceKey"
    Add-Result -Name "Create Device" -Pass $true
} else {
    Write-Fail "Device creation failed"
    Add-Result -Name "Create Device" -Pass $false
    $DeviceKey = ""
    $DeviceId = 0
}

# Step 4: WebSocket Test
Write-Step "4" "WebSocket Connection Test"

if ($DeviceKey) {
    $wsTestPath = Join-Path $PSScriptRoot "..\test_programs\test_x.exe"
    if (Test-Path $wsTestPath) {
        Write-Info "Using test_x.exe for WebSocket test..."
        # Skip for now - needs actual serial port
        Write-Info "WebSocket test requires serial port - skipping"
        Add-Result -Name "WebSocket" -Pass $true -Detail "Skipped (needs serial port)"
    } else {
        Write-Info "WebSocket test skipped (test_x.exe not found)"
        Add-Result -Name "WebSocket" -Pass $true -Detail "Skipped"
    }
} else {
    Write-Info "Skipping WebSocket test (no device key)"
    Add-Result -Name "WebSocket" -Pass $false -Detail "No device key"
}

# Step 5: Check Executables
Write-Step "5" "Component Files Check"

$deviceClientPath = Join-Path $PSScriptRoot "..\..\vsp-client\device-client.exe"
if (Test-Path $deviceClientPath) {
    Write-Success "device-client.exe exists"
    Add-Result -Name "device-client" -Pass $true
} else {
    Write-Fail "device-client.exe not found"
    Add-Result -Name "device-client" -Pass $false
}

$vspManagerPath = Join-Path $PSScriptRoot "..\..\vsp-windows-go\build\bin\VSPManager.exe"
if (Test-Path $vspManagerPath) {
    Write-Success "VSPManager.exe exists"
    Add-Result -Name "VSPManager" -Pass $true
} else {
    Write-Fail "VSPManager.exe not found"
    Add-Result -Name "VSPManager" -Pass $false
}

$testXPath = Join-Path $PSScriptRoot "..\test_programs\test_x.exe"
if (Test-Path $testXPath) {
    Write-Success "test_x.exe exists"
    Add-Result -Name "test_x" -Pass $true
} else {
    Write-Fail "test_x.exe not found"
    Add-Result -Name "test_x" -Pass $false
}

$testYPath = Join-Path $PSScriptRoot "..\test_programs\test_y.exe"
if (Test-Path $testYPath) {
    Write-Success "test_y.exe exists"
    Add-Result -Name "test_y" -Pass $true
} else {
    Write-Fail "test_y.exe not found"
    Add-Result -Name "test_y" -Pass $false
}

# Step 6: com0com Check
Write-Step "6" "com0com Driver Check"

# Check if com0com driver is installed in Windows
$com0comDevices = Get-CimInstance Win32_PnPEntity | Where-Object { $_.Name -match 'com0com' }
if ($com0comDevices.Count -gt 0) {
    $portCount = ($com0comDevices | Where-Object { $_.Name -match 'COM\d+' }).Count
    Write-Success "com0com driver installed ($portCount virtual ports)"
    Add-Result -Name "com0com" -Pass $true -Detail "$portCount ports available"
} else {
    # Check for setupc.exe as fallback
    $com0comPaths = @(
        "$PSScriptRoot\..\..\com0com\setupc.exe",
        "$PSScriptRoot\..\..\vsp-windows-go\build\bin\com0com\setupc.exe",
        "C:\Program Files (x86)\com0com\setupc.exe",
        "C:\Program Files\com0com\setupc.exe"
    )

    $com0comFound = $false
    foreach ($p in $com0comPaths) {
        if (Test-Path $p) {
            Write-Success "com0com setupc.exe found: $p"
            $com0comFound = $true
            break
        }
    }

    if (-not $com0comFound) {
        Write-Fail "com0com not installed"
        Write-Info "Download: https://sourceforge.net/projects/com0com/"
        Write-Info "After install: bcdedit /set testsigning on (admin)"
        Add-Result -Name "com0com" -Pass $false -Detail "Not installed"
    } else {
        Write-Info "Driver not loaded - run setupc.exe to install driver"
        Add-Result -Name "com0com" -Pass $false -Detail "setupc.exe found but driver not installed"
    }
}

# Step 7: Cleanup
Write-Step "7" "Cleanup Test Device"

if ($DeviceId -gt 0 -and $Token) {
    $r = Invoke-API -Endpoint "/devices/$DeviceId" -Method "DELETE" -Token $Token
    if ($r.Success) {
        Write-Success "Test device deleted"
        Add-Result -Name "Cleanup" -Pass $true
    } else {
        Write-Info "Delete result: $($r.Error)"
        Add-Result -Name "Cleanup" -Pass $false
    }
}

# Summary
Write-Host "`n========== Test Summary ==========`n" -ForegroundColor Cyan

$pass = ($Results | Where-Object { $_.Result -eq "PASS" }).Count
$fail = ($Results | Where-Object { $_.Result -eq "FAIL" }).Count

$Results | Format-Table -AutoSize

Write-Host "Total: PASS=$pass, FAIL=$fail`n" -ForegroundColor $(if ($fail -eq 0) { "Green" } else { "Yellow" })

if ($fail -gt 0) {
    Write-Host "Failed tests:" -ForegroundColor Yellow
    $Results | Where-Object { $_.Result -eq "FAIL" } | ForEach-Object {
        Write-Host "  - $($_.Test): $($_.Detail)" -ForegroundColor Red
    }
    Write-Host ""
    Write-Host "To complete end-to-end test, install com0com:" -ForegroundColor Yellow
    Write-Host "  1. Download from https://sourceforge.net/projects/com0com/" -ForegroundColor White
    Write-Host "  2. Run as admin: bcdedit /set testsigning on" -ForegroundColor White
    Write-Host "  3. Reboot and run test again" -ForegroundColor White
}

exit $(if ($fail -eq 0) { 0 } else { 1 })