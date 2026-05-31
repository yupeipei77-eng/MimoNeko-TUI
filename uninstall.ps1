# MimoNeko 卸载脚本
# 以管理员身份运行 PowerShell 执行此脚本

param(
    [string]$InstallPath = "C:\Tools\MimoNeko"
)

Write-Host "=====================================" -ForegroundColor Cyan
Write-Host "  MimoNeko 卸载程序" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host ""

# 检查管理员权限
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[错误] 请以管理员身份运行此脚本！" -ForegroundColor Red
    pause
    exit 1
}

# 从 PATH 中移除
Write-Host "[1/3] 从 PATH 环境变量中移除" -ForegroundColor Yellow
$currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")

if ($currentPath -like "*$InstallPath*") {
    $newPath = ($currentPath -split ';' | Where-Object { $_ -ne $InstallPath }) -join ';'
    [Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")
    Write-Host "  已从 PATH 中移除" -ForegroundColor Green
} else {
    Write-Host "  PATH 中不存在该目录" -ForegroundColor Green
}

# 删除环境变量
Write-Host "[2/3] 删除环境变量" -ForegroundColor Yellow
[Environment]::SetEnvironmentVariable("MIMO_API_KEY", $null, "Machine")
Write-Host "  MIMO_API_KEY 已删除" -ForegroundColor Green

# 删除安装目录
Write-Host "[3/3] 删除安装目录" -ForegroundColor Yellow
if (Test-Path $InstallPath) {
    Remove-Item -Path $InstallPath -Recurse -Force
    Write-Host "  目录已删除: $InstallPath" -ForegroundColor Green
} else {
    Write-Host "  目录不存在" -ForegroundColor Green
}

Write-Host ""
Write-Host "=====================================" -ForegroundColor Green
Write-Host "  卸载完成！" -ForegroundColor Green
Write-Host "=====================================" -ForegroundColor Green
Write-Host ""
Write-Host "请重新打开命令行窗口使更改生效。" -ForegroundColor Cyan
Write-Host ""
pause
