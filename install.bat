@echo off
echo =====================================
echo   MimoNeko 安装程序
echo =====================================
echo.
echo 此脚本将:
echo   1. 复制 MimoNeko.exe 到 C:\Tools\MimoNeko
echo   2. 添加到系统 PATH 环境变量
echo.
echo 需要管理员权限！
echo.
pause

REM 以管理员身份运行 PowerShell 脚本
powershell -Command "Start-Process PowerShell -ArgumentList '-ExecutionPolicy Bypass -File \"%~dp0install.ps1\"' -Verb RunAs"

echo.
echo 安装程序已启动，请在弹出的窗口中完成安装。
pause
