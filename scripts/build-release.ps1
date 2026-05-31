# MioNeko Release Build Script (Windows)
param(
    [string]$Version = "v0.1.1-beta"
)

$ErrorActionPreference = "Stop"
$OutputDir = "dist"

Write-Host "Building MioNeko $Version..." -ForegroundColor Cyan

# Create output directory
if (Test-Path $OutputDir) {
    Remove-Item -Recurse -Force $OutputDir
}
New-Item -ItemType Directory -Path $OutputDir | Out-Null

# Build matrix
$platforms = @(
    @{OS="windows"; Arch="amd64"},
    @{OS="windows"; Arch="arm64"},
    @{OS="linux"; Arch="amd64"},
    @{OS="linux"; Arch="arm64"},
    @{OS="darwin"; Arch="arm64"}
)

foreach ($platform in $platforms) {
    $os = $platform.OS
    $arch = $platform.Arch

    $packageName = "mimoneko-${os}-${arch}"
    $packageDir = Join-Path $OutputDir "pkg-${os}-${arch}"
    New-Item -ItemType Directory -Path $packageDir -Force | Out-Null

    $binaryName = "mimoneko"
    if ($os -eq "windows") {
        $binaryName = "mimoneko.exe"
    }

    Write-Host "Building ${packageName}..." -ForegroundColor Yellow
    
    $env:GOOS = $os
    $env:GOARCH = $arch
    
    go build -o (Join-Path $packageDir $binaryName) .\cmd\mimoneko
    
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Failed to build ${packageName}"
        exit 1
    }
    
    if ($os -eq "windows") {
        Copy-Item ".\install.ps1" (Join-Path $packageDir "install.ps1") -Force
        Copy-Item ".\start-mimoneko.bat" (Join-Path $packageDir "start-mimoneko.bat") -Force
        Compress-Archive -Path (Join-Path $packageDir "*") -DestinationPath "${OutputDir}\${packageName}.zip" -Force
    } else {
        Copy-Item ".\install.sh" (Join-Path $packageDir "install.sh") -Force
        tar -czf "${OutputDir}\${packageName}.tar.gz" -C $packageDir .
    }

    Remove-Item -Recurse -Force $packageDir
}

# Reset environment
Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue

# Generate SHA256
Write-Host "Generating SHA256 checksums..." -ForegroundColor Yellow

$sha256File = "${OutputDir}\SHA256SUMS"
$files = Get-ChildItem -Path $OutputDir -File | Where-Object { $_.Name -ne "SHA256SUMS" }

foreach ($file in $files) {
    $hash = Get-FileHash -Path $file.FullName -Algorithm SHA256
    "$($hash.Hash.ToLower())  $($file.Name)" | Out-File -FilePath $sha256File -Append -Encoding UTF8
}

Write-Host ""
Write-Host "Build complete! Files in ${OutputDir}\" -ForegroundColor Green
Write-Host ""
Get-ChildItem -Path $OutputDir | Format-Table Name, Length -AutoSize
Write-Host ""
Write-Host "SHA256SUMS:" -ForegroundColor Cyan
Get-Content $sha256File
