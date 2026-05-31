@echo off
setlocal

set "MIMONEKO_EXE=mimoneko"
where mimoneko >nul 2>nul
if errorlevel 1 (
  if exist "%LOCALAPPDATA%\MioNeko\bin\mimoneko.exe" (
    set "MIMONEKO_EXE=%LOCALAPPDATA%\MioNeko\bin\mimoneko.exe"
  )
)

start "MioNeko" powershell -NoExit -Command "& '%MIMONEKO_EXE%'"
