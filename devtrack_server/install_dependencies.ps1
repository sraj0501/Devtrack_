# DevTrack Docker Installer for Windows
# Builds Docker images and deploys DevTrack as a containerized application

param(
    [string]$WorkspacePath = "$env:USERPROFILE\workspace"
)

$ErrorActionPreference = "Stop"

# Configuration
$INSTALL_DIR = "$env:USERPROFILE\.local\bin"

Write-Host ""
Write-Host "╔══════════════════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║        DevTrack - Docker Installer (Windows)             ║" -ForegroundColor Cyan
Write-Host "╚══════════════════════════════════════════════════════════╝" -ForegroundColor Cyan
Write-Host ""

function Write-Success {
    param([string]$Message)
    Write-Host "   ✓ $Message" -ForegroundColor Green
}

function Write-Error {
    param([string]$Message)
    Write-Host "   ✗ $Message" -ForegroundColor Red
}

function Write-Warning {
    param([string]$Message)
    Write-Host "   ⚠️  $Message" -ForegroundColor Yellow
}

# Check Docker
Write-Host "🔍 Checking Docker installation..." -ForegroundColor Blue
try {
    docker info | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Docker not running" }
    Write-Success "Docker is installed and running"
} catch {
    Write-Error "Docker is not running."
    Write-Host ""
    Write-Host "   Please install and start Docker Desktop from:"
    Write-Host "   https://www.docker.com/products/docker-desktop" -ForegroundColor Cyan
    exit 1
}

# Check Docker Compose
Write-Host ""
Write-Host "🔍 Checking Docker Compose..." -ForegroundColor Blue
try {
    docker compose version | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Docker Compose not found" }
    Write-Success "Docker Compose is available"
} catch {
    Write-Error "Docker Compose is not installed."
    Write-Host ""
    Write-Host "   Docker Compose is usually included with Docker Desktop."
    exit 1
}

# Check/create installation directory
Write-Host ""
Write-Host "🔍 Checking installation directory..." -ForegroundColor Blue
if (-not (Test-Path $INSTALL_DIR)) {
    Write-Host "   Creating $INSTALL_DIR" -ForegroundColor Yellow
    New-Item -ItemType Directory -Path $INSTALL_DIR -Force | Out-Null
}
Write-Success "Installation directory ready: $INSTALL_DIR"

# Check PATH
if ($env:PATH -notlike "*$INSTALL_DIR*") {
    Write-Warning "$INSTALL_DIR is not in your PATH"
    Write-Host ""
    Write-Host "   Add it to your PATH with:"
    Write-Host "   [System.Environment]::SetEnvironmentVariable('PATH', `$env:PATH + ';$INSTALL_DIR', 'User')" -ForegroundColor Yellow
    Write-Host ""
}

# Check/create workspace
Write-Host ""
Write-Host "🔍 Checking workspace directory..." -ForegroundColor Blue
if (-not (Test-Path $WorkspacePath)) {
    Write-Host "   Creating workspace at $WorkspacePath" -ForegroundColor Yellow
    New-Item -ItemType Directory -Path $WorkspacePath -Force | Out-Null
}
Write-Success "Workspace ready: $WorkspacePath"

# Check host Ollama availability
Write-Host ""
Write-Host "🔍 Checking host Ollama service..." -ForegroundColor Blue
try {
    Get-Command ollama -ErrorAction Stop | Out-Null
    Write-Success "Ollama CLI detected"
} catch {
    Write-Warning "Ollama CLI not found on host. Download it from https://ollama.com/download"
}

try {
    Add-Type -AssemblyName System.Net.Http -ErrorAction SilentlyContinue
    $client = [System.Net.Http.HttpClient]::new()
    $client.Timeout = [TimeSpan]::FromSeconds(3)
    $response = $client.GetAsync('http://127.0.0.1:11434/api/version').GetAwaiter().GetResult()
    if ($response.IsSuccessStatusCode) {
        Write-Success "Ollama service responding on localhost:11434"
    } else {
        Write-Warning "Ollama service returned status $($response.StatusCode). Ensure it's running with 'ollama serve' or update OLLAMA_HOST in .env"
    }
} catch {
    Write-Warning "Unable to reach Ollama on localhost:11434. Start it with 'ollama serve' or set OLLAMA_HOST in .env"
} finally {
    if ($client) { $client.Dispose() }
}

# Build Docker images
Write-Host ""
Write-Host "━━━ Building DevTrack Docker Images ━━━" -ForegroundColor Magenta
Write-Host ""

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $scriptDir

Write-Host "📦 Building DevTrack image..." -ForegroundColor Blue
Write-Host "   This may take several minutes on first run..." -ForegroundColor Yellow

docker build -t devtrack:latest .
if ($LASTEXITCODE -ne 0) {
    Write-Error "Failed to build DevTrack image"
    exit 1
}
Write-Success "DevTrack image built successfully"

# Deploy wrapper
Write-Host ""
Write-Host "━━━ Deploying DevTrack Wrapper ━━━" -ForegroundColor Magenta
Write-Host ""

$batSource = Join-Path $scriptDir "devtrack-wrapper.bat"
$ps1Source = Join-Path $scriptDir "devtrack-wrapper.ps1"
$batDest = Join-Path $INSTALL_DIR "devtrack.bat"
$ps1Dest = Join-Path $INSTALL_DIR "devtrack-wrapper.ps1"

if (Test-Path $batSource) {
    Copy-Item $batSource $batDest -Force
    Copy-Item $ps1Source $ps1Dest -Force
    Write-Success "devtrack.bat installed to $INSTALL_DIR"
} else {
    Write-Error "Wrapper scripts not found"
    exit 1
}

# Start services
Write-Host ""
Write-Host "━━━ Starting DevTrack Services ━━━" -ForegroundColor Magenta
Write-Host ""

$env:DEVTRACK_WORKSPACE = $WorkspacePath
docker compose up -d devtrack
if ($LASTEXITCODE -eq 0) {
    Write-Success "DevTrack services started"
} else {
    Write-Error "Failed to start services"
    exit 1
}

Start-Sleep -Seconds 3

# Check services
$containers = docker ps --format "{{.Names}}"
if ($containers -contains "devtrack-app") {
    Write-Success "DevTrack service is running"
}

Pop-Location

Write-Host ""
Write-Host "╔══════════════════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║              INSTALLATION COMPLETE!                      ║" -ForegroundColor Cyan
Write-Host "╚══════════════════════════════════════════════════════════╝" -ForegroundColor Cyan
Write-Host ""
Write-Host "🎉 DevTrack installation finished successfully!" -ForegroundColor Green
Write-Host ""

Write-Host "Installation Summary:" -ForegroundColor Magenta
Write-Host "  ✓ DevTrack CLI → $INSTALL_DIR\devtrack.bat" -ForegroundColor Green
Write-Host "  ✓ Workspace → $WorkspacePath" -ForegroundColor Green
Write-Host "  ✓ Running in Docker containers" -ForegroundColor Green
Write-Host ""

Write-Host "Quick Start:" -ForegroundColor Magenta
Write-Host "  devtrack                 - Launch interactive TUI" -ForegroundColor Yellow
Write-Host "  devtrack --help          - Show all commands" -ForegroundColor Yellow
Write-Host "  devtrack learning-status - Check AI learning status" -ForegroundColor Yellow
Write-Host ""

if ($env:PATH -notlike "*$INSTALL_DIR*") {
    Write-Host "⚠️  IMPORTANT: Add $INSTALL_DIR to your PATH" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "   Run in PowerShell (as Administrator):" -ForegroundColor Cyan
    Write-Host "   [System.Environment]::SetEnvironmentVariable('PATH', `$env:PATH + ';$INSTALL_DIR', 'User')" -ForegroundColor Green
    Write-Host ""
}

Write-Host "Ready to use DevTrack!" -ForegroundColor Green
Write-Host ""
