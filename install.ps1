# MimoNeko 安装脚本
# 以管理员身份运行 PowerShell 执行此脚本

param(
    [string]$InstallPath = "C:\Tools\MimoNeko",
    [string]$ApiKey = ""
)

Write-Host "=====================================" -ForegroundColor Cyan
Write-Host "  MimoNeko 安装程序" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host ""

# 检查管理员权限
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[错误] 请以管理员身份运行此脚本！" -ForegroundColor Red
    Write-Host "右键点击 PowerShell，选择'以管理员身份运行'" -ForegroundColor Yellow
    pause
    exit 1
}

# 创建安装目录
Write-Host "[1/4] 创建安装目录: $InstallPath" -ForegroundColor Yellow
if (-not (Test-Path $InstallPath)) {
    New-Item -ItemType Directory -Path $InstallPath -Force | Out-Null
    Write-Host "  目录已创建" -ForegroundColor Green
} else {
    Write-Host "  目录已存在" -ForegroundColor Green
}

# 复制 exe 文件
Write-Host "[2/4] 复制 MimoNeko.exe" -ForegroundColor Yellow
$sourcePath = Join-Path $PSScriptRoot "MimoNeko.exe"
$destPath = Join-Path $InstallPath "MimoNeko.exe"

if (Test-Path $sourcePath) {
    Copy-Item -Path $sourcePath -Destination $destPath -Force
    Write-Host "  已复制到: $destPath" -ForegroundColor Green
} else {
    Write-Host "  [错误] 未找到 MimoNeko.exe，请确保脚本在同一目录" -ForegroundColor Red
    pause
    exit 1
}

# 添加到 PATH
Write-Host "[3/4] 添加到 PATH 环境变量" -ForegroundColor Yellow
$currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")

if ($currentPath -notlike "*$InstallPath*") {
    $newPath = "$currentPath;$InstallPath"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")
    Write-Host "  已添加到系统 PATH" -ForegroundColor Green
} else {
    Write-Host "  PATH 中已存在" -ForegroundColor Green
}

# 设置 API Key
Write-Host "[4/4] 配置环境变量" -ForegroundColor Yellow
if ($ApiKey -ne "") {
    [Environment]::SetEnvironmentVariable("MIMO_API_KEY", $ApiKey, "Machine")
    Write-Host "  MIMO_API_KEY 已设置" -ForegroundColor Green
} else {
    Write-Host "  跳过 API Key 设置（未提供）" -ForegroundColor Yellow
    Write-Host "  稍后可手动设置: setx /M MIMO_API_KEY `"your-key`"" -ForegroundColor Gray
}

Write-Host ""
Write-Host "=====================================" -ForegroundColor Green
Write-Host "  安装完成！" -ForegroundColor Green
Write-Host "=====================================" -ForegroundColor Green
Write-Host ""
Write-Host "请重新打开命令行窗口，然后运行:" -ForegroundColor Cyan
Write-Host "  MimoNeko.exe --help" -ForegroundColor White
Write-Host ""
pause
