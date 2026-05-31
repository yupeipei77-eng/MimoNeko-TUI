# MioNeko Release Build Script (Windows)
param(
    [string]$Version = "v0.1.1"
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
    
    $outputName = "mimoneko-${os}-${arch}"
    if ($os -eq "windows") {
        $outputName = "${outputName}.exe"
    }
    
    Write-Host "Building ${outputName}..." -ForegroundColor Yellow
    
    $env:GOOS = $os
    $env:GOARCH = $arch
    
    go build -o "${OutputDir}\${outputName}" .\cmd\mimoneko
    
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Failed to build ${outputName}"
        exit 1
    }
    
    # Package
    if ($os -eq "windows") {
        Compress-Archive -Path "${OutputDir}\${outputName}" -DestinationPath "${OutputDir}\mimoneko-${os}-${arch}.zip" -Force
        Remove-Item "${OutputDir}\${outputName}"
    } else {
        # For non-Windows, use tar (available in Windows 10+)
        tar -czf "${OutputDir}\mimoneko-${os}-${arch}.tar.gz" -C $OutputDir $outputName
        Remove-Item "${OutputDir}\${outputName}"
    }
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
