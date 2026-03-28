# VSP 一键启动并测试
# 自动启动服务器并运行集成测试

param(
    [string]$ServerPath = "..\vsp-server\vsp-server.exe",
    [int]$Port = 9000
)

Write-Host "========================================" -ForegroundColor Cyan
Write-Host " VSP 集成测试 - 一键启动" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# 检查服务器文件
if (-not (Test-Path $ServerPath)) {
    Write-Host "错误: 找不到服务器文件 $ServerPath" -ForegroundColor Red
    Write-Host "请确保 vsp-server 已编译" -ForegroundColor Yellow
    exit 1
}

# 启动服务器
Write-Host "`n[1/3] 启动 vsp-server..." -ForegroundColor Yellow
$serverProcess = Start-Process -FilePath $ServerPath -PassThru -WindowStyle Normal
Write-Host "服务器 PID: $($serverProcess.Id)" -ForegroundColor Gray

# 等待服务器启动
Write-Host "等待服务器启动..." -ForegroundColor Gray
Start-Sleep -Seconds 5

# 运行集成测试
Write-Host "`n[2/3] 运行集成测试..." -ForegroundColor Yellow
& "$PSScriptRoot\integration_test.ps1"

# 清理
Write-Host "`n[3/3] 清理..." -ForegroundColor Yellow
Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue
Write-Host "已停止服务器" -ForegroundColor Gray

Write-Host "`n========================================" -ForegroundColor Green
Write-Host " 测试完成" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green