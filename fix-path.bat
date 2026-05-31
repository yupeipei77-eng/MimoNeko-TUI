@echo off
echo ====================================
echo   修复 PATH 并安装 MimoNeko
echo ====================================
echo.
echo 需要管理员权限！
echo.

REM 以管理员身份运行 PowerShell
powershell -Command "Start-Process PowerShell -ArgumentList '-ExecutionPolicy Bypass -Command \"& { Copy-Item -Path $env:USERPROFILE\.mimoneko\bin\*.exe -Destination C:\Windows\System32\ -Force; Write-Host Done; pause }\"' -Verb RunAs"

echo.
echo 如果弹出管理员窗口，请确认操作。
pause
