package neko

import (
	"fmt"
	"strings"
)

func localAutoSaveResponse(message string) (string, bool) {
	if !hasAutoSaveIntent(message) {
		return "", false
	}
	if drive, ok := desktopMigrationDrive(message); ok {
		return fencedCode("bat", desktopMigrationBatchScript(drive)), true
	}
	return "", false
}

func desktopMigrationDrive(message string) (string, bool) {
	compact := strings.ToLower(strings.Join(strings.Fields(message), ""))
	if !strings.Contains(compact, "桌面") {
		return "", false
	}
	if !strings.Contains(compact, "迁移") && !strings.Contains(compact, "移动") &&
		!strings.Contains(compact, "move") && !strings.Contains(compact, "migrate") {
		return "", false
	}
	if match := driveWordPattern.FindStringSubmatch(message); len(match) == 2 {
		return strings.ToUpper(match[1]), true
	}
	return "", false
}

func desktopMigrationBatchScript(drive string) string {
	target := strings.ToUpper(strings.TrimSpace(drive)) + `:\Desktop`
	return fmt.Sprintf(`@echo off
setlocal EnableExtensions

set "SOURCE=%%USERPROFILE%%\Desktop"
set "TARGET=%s"

echo Migrating Desktop from "%%SOURCE%%" to "%%TARGET%%"
if not exist "%%TARGET%%" mkdir "%%TARGET%%"

robocopy "%%SOURCE%%" "%%TARGET%%" /E /COPY:DAT /DCOPY:DAT /R:2 /W:2 /XJ
if errorlevel 8 (
    echo Robocopy failed with exit code %%ERRORLEVEL%%.
    pause
    exit /b %%ERRORLEVEL%%
)

reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders" /v Desktop /t REG_EXPAND_SZ /d "%%TARGET%%" /f
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\Shell Folders" /v Desktop /t REG_SZ /d "%%TARGET%%" /f

taskkill /f /im explorer.exe >nul 2>nul
start explorer.exe

echo Done. Desktop path is now "%%TARGET%%".
pause
`, target)
}

func fencedCode(language, content string) string {
	return "```" + language + "\n" + strings.TrimSpace(content) + "\n```"
}
