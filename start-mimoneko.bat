@echo off
setlocal

set "MIMONEKO_EXE=mimoneko"
where mimoneko >nul 2>nul
if errorlevel 1 (
  if exist "%LOCALAPPDATA%\MimoNeko\bin\mimoneko.exe" (
    set "MIMONEKO_EXE=%LOCALAPPDATA%\MimoNeko\bin\mimoneko.exe"
  ) else if exist "%LOCALAPPDATA%\MioNeko\bin\mimoneko.exe" (
    set "MIMONEKO_EXE=%LOCALAPPDATA%\MioNeko\bin\mimoneko.exe"
  )
)

start "MimoNeko" powershell -NoExit -Command "& '%MIMONEKO_EXE%'"
