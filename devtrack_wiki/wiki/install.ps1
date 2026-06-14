# install.ps1 — DevTrack installer for Windows
# Usage: irm https://devtrack.cloud/install.ps1 | iex
$ErrorActionPreference = 'Stop'

$REPO        = "sraj0501/Devtrack_"
$INSTALL_DIR = if ($env:DEVTRACK_INSTALL_DIR) { $env:DEVTRACK_INSTALL_DIR }
               else { "$env:LOCALAPPDATA\DevTrack" }

# Fetch latest version tag
try {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$REPO/releases/latest" -ErrorAction Stop
    $VERSION = $release.tag_name
} catch {
    Write-Error "Could not fetch latest version. Check your network or visit https://devtrack.cloud/download"
    exit 1
}

$EXE = "devtrack_windows_amd64.exe"
$URL = "https://github.com/$REPO/releases/download/$VERSION/$EXE"

Write-Host "Detected: windows/amd64"
Write-Host "Downloading DevTrack $VERSION..."

New-Item -ItemType Directory -Force -Path $INSTALL_DIR | Out-Null

$destPath = Join-Path $INSTALL_DIR "devtrack.exe"

try {
    Invoke-WebRequest -Uri $URL -OutFile $destPath -UseBasicParsing
} catch {
    Write-Error "Download failed: $_"
    exit 1
}

Write-Host "Installed to $destPath"

# Add install dir to user PATH if not already present
$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($userPath -notlike "*$INSTALL_DIR*") {
    [Environment]::SetEnvironmentVariable("PATH", "$userPath;$INSTALL_DIR", "User")
    Write-Host ""
    Write-Host "Added $INSTALL_DIR to your PATH."
    Write-Host "Restart your terminal, then run: devtrack setup"
} else {
    Write-Host ""
    Write-Host "Run: devtrack setup"
}
