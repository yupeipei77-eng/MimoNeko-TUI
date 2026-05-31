# 修复 PATH 并安装 MimoNeko
# 以管理员身份运行此脚本

Write-Host "====================================" -ForegroundColor Cyan
Write-Host "  修复 PATH 并安装 MimoNeko" -ForegroundColor Cyan
Write-Host "====================================" -ForegroundColor Cyan
Write-Host ""

# 检查管理员权限
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[错误] 请以管理员身份运行此脚本！" -ForegroundColor Red
    Write-Host "右键点击 PowerShell，选择'以管理员身份运行'" -ForegroundColor Yellow
    pause
    exit 1
}

# 复制到 System32
$sourcePath = "$env:USERPROFILE\.mimoneko\bin\*.exe"
$destPath = "C:\Windows\System32\"

Write-Host "[1/2] 复制文件到 $destPath" -ForegroundColor Yellow
Copy-Item -Path $sourcePath -Destination $destPath -Force
Write-Host "  已复制 mimoneko.exe 和 neko.exe" -ForegroundColor Green

# 修复用户 PATH（去除重复和截断）
Write-Host "[2/2] 修复用户 PATH" -ForegroundColor Yellow
$currentPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$mimonekoPath = "$env:USERPROFILE\.mimoneko\bin"

# 清理 PATH（去除重复项）
$pathItems = $currentPath -split ';' | Where-Object { $_ -ne '' } | Select-Object -Unique
if ($pathItems -notcontains $mimonekoPath) {
    $pathItems += $mimonekoPath
}
$newPath = $pathItems -join ';'
[Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
Write-Host "  PATH 已修复" -ForegroundColor Green

Write-Host ""
Write-Host "====================================" -ForegroundColor Green
Write-Host "  安装完成！" -ForegroundColor Green
Write-Host "====================================" -ForegroundColor Green
Write-Host ""
Write-Host "请重新打开命令行窗口，然后运行:" -ForegroundColor Cyan
Write-Host "  mimoneko --help" -ForegroundColor White
Write-Host ""
pause
