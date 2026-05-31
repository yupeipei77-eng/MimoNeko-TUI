# MimoNeko Build & Deploy Script
# Usage: .\build.ps1

$ErrorActionPreference = "Stop"

$projectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$toolsPath = "D:\Tools\MimoNeko"
$appDataPath = "$env:LOCALAPPDATA\Programs\MimoNeko\bin"

Write-Host "Building MimoNeko..." -ForegroundColor Cyan

# Stop running neko process if exists
$nekoProcess = Get-Process -Name "neko" -ErrorAction SilentlyContinue
if ($nekoProcess) {
    Write-Host "Stopping running neko process..." -ForegroundColor Yellow
    Stop-Process -Name "neko" -Force
    Start-Sleep -Milliseconds 500
}

# Build
Set-Location $projectRoot
go build -o neko.exe ./cmd/neko
go build -o mimoneko.exe ./cmd/mimoneko

# Create target directories if not exist
if (-not (Test-Path $toolsPath)) {
    New-Item -ItemType Directory -Path $toolsPath -Force | Out-Null
}
if (-not (Test-Path $appDataPath)) {
    New-Item -ItemType Directory -Path $appDataPath -Force | Out-Null
}

# Copy to PATH locations
Copy-Item neko.exe "$toolsPath\neko.exe" -Force
Copy-Item mimoneko.exe "$toolsPath\mimoneko.exe" -Force
Copy-Item neko.exe "$appDataPath\neko.exe" -Force
Copy-Item mimoneko.exe "$appDataPath\mimoneko.exe" -Force

# Show result
$time = Get-Date -Format "HH:mm:ss"
Write-Host "`nDone! ($time)" -ForegroundColor Green
Write-Host "  -> $toolsPath"
Write-Host "  -> $appDataPath"
