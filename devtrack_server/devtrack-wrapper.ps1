# DevTrack Docker Wrapper for Windows
# Makes DevTrack container callable like a native CLI application

# Configuration
$DOCKER_IMAGE = "devtrack:latest"
$CONTAINER_NAME = "devtrack-app"
$WORKSPACE_PATH = if ($env:WORKSPACE_PATH) { $env:WORKSPACE_PATH } else { "$env:USERPROFILE\workspace" }

# Check if Docker is running
try {
    docker info | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Docker not running"
    }
} catch {
    Write-Error "Error: Docker is not running. Please start Docker Desktop."
    exit 1
}

# Check if docker-compose services are running
$runningContainers = docker ps --format "{{.Names}}"
if ($runningContainers -notcontains $CONTAINER_NAME) {
    # Check if image exists
    try {
        docker image inspect $DOCKER_IMAGE | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "Image not found"
        }
    } catch {
        Write-Error "Error: DevTrack image not found. Please run the installer first."
        exit 1
    }
    
    # Start services if they're not running
    $scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
    $repoDir = Split-Path -Parent $scriptDir
    $composeFile = Join-Path $repoDir "docker-compose.yml"
    
    if (Test-Path $composeFile) {
        Push-Location $repoDir
        docker compose up -d | Out-Null
        Pop-Location
    } else {
        Write-Error "Error: docker-compose.yml not found. DevTrack may not be properly installed."
        exit 1
    }
}

# Get current directory and find git repository root
$currentDir = Get-Location | Select-Object -ExpandProperty Path
$gitRoot = $currentDir

# Find git repository root by traversing up
while ($gitRoot -ne "" -and $gitRoot -ne $null) {
    if (Test-Path (Join-Path $gitRoot ".git")) {
        break
    }
    $parent = Split-Path -Parent $gitRoot
    if ($parent -eq $gitRoot) {
        $gitRoot = $null
        break
    }
    $gitRoot = $parent
}

# If we found a git repo, calculate its path in the container (/home/host prefix)
if ($gitRoot -and (Test-Path (Join-Path $gitRoot ".git"))) {
    # Map Windows path to container path
    # Windows uses USERPROFILE as home directory
    $homeDir = if ($env:HOME) { $env:HOME } else { $env:USERPROFILE }
    
    # Replace home directory with /home/host prefix
    if ($gitRoot.StartsWith($homeDir)) {
        $relativePath = $gitRoot.Substring($homeDir.Length) -replace '\\', '/'
        $containerWorkDir = "/home/host$relativePath"
    } else {
        # If not under home, just map the drive
        $containerWorkDir = $gitRoot -replace '^[A-Za-z]:', '' -replace '\\', '/'
        $containerWorkDir = "/home/host$containerWorkDir"
    }
    
    # For 'start' command, run detached so daemon persists
    if ($args[0] -eq "start") {
        docker exec -d `
            -w $containerWorkDir `
            -e "TERM=$env:TERM" `
            $CONTAINER_NAME `
            devtrack $args
        
        # Give it a moment to start
        Start-Sleep -Seconds 1
        
        # Show status
        docker exec `
            -w $containerWorkDir `
            $CONTAINER_NAME `
            devtrack status
    } else {
        # Run interactively for other commands
        $env:TERM = if ($env:TERM) { $env:TERM } else { "xterm-256color" }
        docker exec -it `
            -w $containerWorkDir `
            -e "TERM=$env:TERM" `
            $CONTAINER_NAME `
            devtrack $args
    }
} else {
    # No git repo found, use workspace as fallback
    $workspacePathNormalized = $WORKSPACE_PATH.Replace('\', '/')
    
    if ($currentDir.StartsWith($WORKSPACE_PATH)) {
        $relPath = $currentDir.Substring($WORKSPACE_PATH.Length).Replace('\', '/')
        $workDir = "/workspace$relPath"
    } else {
        $workDir = "/workspace"
    }
    
    # For 'start' command, run detached
    if ($args[0] -eq "start") {
        docker exec -d `
            -w $workDir `
            -e "TERM=$env:TERM" `
            $CONTAINER_NAME `
            devtrack $args
        
        Start-Sleep -Seconds 1
        
        docker exec `
            -w $workDir `
            $CONTAINER_NAME `
            devtrack status
    } else {
        $env:TERM = if ($env:TERM) { $env:TERM } else { "xterm-256color" }
        docker exec -it `
            -w $workDir `
            -e "TERM=$env:TERM" `
            $CONTAINER_NAME `
            devtrack $args
    }
}

exit $LASTEXITCODE
