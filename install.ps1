# MioNeko user-scope installer for Windows.
# Run from the extracted release folder. Administrator permission is not required.

$ErrorActionPreference = "Stop"

$InstallDir = Join-Path $env:LOCALAPPDATA "MioNeko\bin"
$TargetExe = Join-Path $InstallDir "mimoneko.exe"

function Normalize-PathForCompare {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return ""
    }

    try {
        return [System.IO.Path]::GetFullPath($Value).TrimEnd("\")
    } catch {
        return $Value.Trim().TrimEnd("\")
    }
}

Write-Host "MioNeko Windows installer" -ForegroundColor Cyan
Write-Host ""

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$sourceCandidates = @(
    (Join-Path $scriptDir "mimoneko.exe"),
    (Join-Path $scriptDir "MioNeko.exe"),
    (Join-Path $scriptDir "MimoNeko.exe")
)

$sourceExe = $null
foreach ($candidate in $sourceCandidates) {
    if (Test-Path $candidate) {
        $sourceExe = $candidate
        break
    }
}

if (-not $sourceExe) {
    $platformExe = Get-ChildItem -Path $scriptDir -Filter "mimoneko-*.exe" -File -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($platformExe) {
        $sourceExe = $platformExe.FullName
    }
}

if (-not $sourceExe) {
    Write-Host "Could not find mimoneko.exe in $scriptDir" -ForegroundColor Red
    exit 1
}

New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Copy-Item -LiteralPath $sourceExe -Destination $TargetExe -Force
$TargetExe = (Resolve-Path -LiteralPath $TargetExe).Path

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($null -eq $userPath) {
    $userPath = ""
}

$pathParts = $userPath -split ";" | Where-Object { $_ -ne "" }
$alreadyInPath = $false
foreach ($part in $pathParts) {
    if ($part.TrimEnd("\") -ieq $InstallDir.TrimEnd("\")) {
        $alreadyInPath = $true
        break
    }
}

$pathUpdated = $false
if (-not $alreadyInPath) {
    if ($userPath.Trim() -eq "") {
        $newPath = $InstallDir
    } else {
        $newPath = "$userPath;$InstallDir"
    }
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    $pathUpdated = $true
} else {
    $newPath = $userPath
}

$machinePath = [Environment]::GetEnvironmentVariable("Path", "Machine")
$updatedUserPath = [Environment]::GetEnvironmentVariable("Path", "User")
$diagnosticPathParts = @()
if (-not [string]::IsNullOrWhiteSpace($machinePath)) {
    $diagnosticPathParts += $machinePath
}
if (-not [string]::IsNullOrWhiteSpace($updatedUserPath)) {
    $diagnosticPathParts += $updatedUserPath
}

$whereMatches = @()
$oldProcessPath = $env:Path
$whereWorkingDir = $env:USERPROFILE
if ([string]::IsNullOrWhiteSpace($whereWorkingDir) -or -not (Test-Path -LiteralPath $whereWorkingDir)) {
    $whereWorkingDir = $env:TEMP
}

try {
    $env:Path = ($diagnosticPathParts -join ";")
    Push-Location -LiteralPath $whereWorkingDir
    try {
        $whereMatches = @(where.exe mimoneko 2>$null)
        if ($LASTEXITCODE -ne 0) {
            $whereMatches = @()
        }
    } finally {
        Pop-Location
    }
} finally {
    $env:Path = $oldProcessPath
}

Write-Host ""
Write-Host "Installation complete." -ForegroundColor Green
Write-Host "当前安装路径：$TargetExe" -ForegroundColor White
if ($pathUpdated) {
    Write-Host "PATH 是否已更新：是，已加入用户 PATH" -ForegroundColor Green
} else {
    Write-Host "PATH 是否已更新：否，用户 PATH 已包含安装目录" -ForegroundColor Green
}

Write-Host "where.exe mimoneko 结果（按重新打开 PowerShell 后的 PATH 检测）："
if ($whereMatches.Count -eq 0) {
    Write-Host "  (未找到)" -ForegroundColor Yellow
} else {
    foreach ($match in $whereMatches) {
        Write-Host "  $match" -ForegroundColor White
    }
}

$firstMatch = $whereMatches | Select-Object -First 1
if ($firstMatch) {
    $normalizedFirstMatch = Normalize-PathForCompare $firstMatch
    $normalizedTargetExe = Normalize-PathForCompare $TargetExe
    if ($normalizedFirstMatch -ine $normalizedTargetExe) {
        Write-Host ""
        Write-Host "检测到旧版本 mimoneko 优先于当前安装版本" -ForegroundColor Yellow
        Write-Host "请删除或调整该路径" -ForegroundColor Yellow
        Write-Host "当前优先命中路径：$firstMatch" -ForegroundColor Yellow
        Write-Host "新版本安装路径：$TargetExe" -ForegroundColor Yellow
    } else {
        Write-Host "PATH 优先级：新版本优先" -ForegroundColor Green
    }
} else {
    Write-Host ""
    Write-Host "未能通过 where.exe 找到 mimoneko，请重新打开 PowerShell 后再试。" -ForegroundColor Yellow
}

Write-Host "是否需要重新打开 PowerShell：是，安装后请重新打开 PowerShell。" -ForegroundColor Yellow
Write-Host "重新打开后运行：" -ForegroundColor Yellow
Write-Host "  mimoneko" -ForegroundColor White
