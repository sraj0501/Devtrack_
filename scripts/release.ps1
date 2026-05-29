# DevTrack local release script
# Usage: .\scripts\release.ps1 [-Bump patch|minor|major]
#
# Requires: go, gh (GitHub CLI), git, tar (Windows 10+)
# Run from the repo root.

param(
    [ValidateSet("patch","minor","major")]
    [string]$Bump = "patch"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
function Step($msg) { Write-Host "`n==> $msg" -ForegroundColor Cyan }
function Die($msg)  { Write-Host "ERROR: $msg" -ForegroundColor Red; exit 1 }

# ---------------------------------------------------------------------------
# Preflight
# ---------------------------------------------------------------------------
Step "Preflight checks"

Push-Location $PSScriptRoot\..   # repo root

if (git status --porcelain) {
    # Non-empty output = dirty. On a clean tree `git status --porcelain` yields
    # no output, which PowerShell captures as $null; `if ($null)` is correctly
    # false. (The old `-ne ""` check mis-fired because `$null -ne ""` is $true.)
    Die "Working tree is dirty. Commit or stash changes first."
}
if ((git rev-parse --abbrev-ref HEAD) -ne "main") {
    Die "Must be on main branch."
}
git pull origin main --quiet

foreach ($cmd in @("go","gh","tar")) {
    if (-not (Get-Command $cmd -ErrorAction SilentlyContinue)) {
        Die "'$cmd' not found in PATH."
    }
}

# ---------------------------------------------------------------------------
# Compute next version
# ---------------------------------------------------------------------------
Step "Computing next version (bump: $Bump)"

$latest = git tag --sort=-version:refname | Where-Object { $_ -match '^v\d' } | Select-Object -First 1
if (-not $latest) { $latest = "v0.0.0" }

$ver = $latest.TrimStart("v")
$parts = $ver.Split(".")
$major = [int]$parts[0]; $minor = [int]$parts[1]; $patch = [int]$parts[2]

switch ($Bump) {
    "major" { $major++; $minor = 0; $patch = 0 }
    "minor" { $minor++;             $patch = 0 }
    "patch" { $patch++ }
}

$tag = "v$major.$minor.$patch"
$ver = "$major.$minor.$patch"
Write-Host "  $latest  ->  $tag"

# ---------------------------------------------------------------------------
# Tag
# ---------------------------------------------------------------------------
Step "Creating and pushing tag $tag"

git tag -a $tag -m "Release $tag"
git push origin $tag

# ---------------------------------------------------------------------------
# Build client binaries
# ---------------------------------------------------------------------------
Step "Building client binaries"

$dist = "dist"
Remove-Item -Recurse -Force $dist -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $dist | Out-Null

Push-Location devtrack_client

$commit    = git rev-parse --short HEAD
$buildtime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags   = "-s -w -X main.Version=$tag -X main.GitCommit=$commit -X main.BuildTime=$buildtime"

$targets = @(
    @{ OS="linux";   Arch="amd64" },
    @{ OS="linux";   Arch="arm64" },
    @{ OS="darwin";  Arch="amd64" },
    @{ OS="darwin";  Arch="arm64" }
)

foreach ($t in $targets) {
    $name = "devtrack_$($t.OS)_$($t.Arch)"
    Write-Host "  Building $name ..."
    $env:CGO_ENABLED = "0"; $env:GOOS = $t.OS; $env:GOARCH = $t.Arch
    go build -ldflags $ldflags -o "..\$dist\$name" .
    Push-Location ..\$dist
    Copy-Item $name devtrack
    tar -czf "$name.tar.gz" devtrack
    Remove-Item devtrack, $name
    Pop-Location
}

# Windows
Write-Host "  Building devtrack_windows_amd64.exe ..."
$env:CGO_ENABLED = "0"; $env:GOOS = "windows"; $env:GOARCH = "amd64"
go build -ldflags $ldflags -o "..\$dist\devtrack_windows_amd64.exe" .
$env:GOOS = ""; $env:GOARCH = ""; $env:CGO_ENABLED = ""

Push-Location ..\$dist
Compress-Archive -Path devtrack_windows_amd64.exe -DestinationPath devtrack_windows_amd64.zip
Remove-Item devtrack_windows_amd64.exe
Pop-Location

Pop-Location  # back to repo root

# ---------------------------------------------------------------------------
# Build server archive
# ---------------------------------------------------------------------------
Step "Building server archive"

$serverDir = "$dist\devtrack-server-$ver"
New-Item -ItemType Directory -Path $serverDir | Out-Null

Copy-Item -Recurse devtrack_server\backend "$serverDir\backend"
@("devtrack_server\Dockerfile","devtrack_server\docker-compose.yml",
  "devtrack_server\pyproject.toml","README.md","TERMS.md") | ForEach-Object {
    if (Test-Path $_) { Copy-Item $_ $serverDir }
}
if (Test-Path "devtrack_server\uv.lock")  { Copy-Item devtrack_server\uv.lock  $serverDir }
if (Test-Path ".env_sample")              { Copy-Item .env_sample              $serverDir }
@("devtrack_client\workspaces.yaml.example","devtrack_client\workspaces.yaml.sample",
  "devtrack_client\devtrack-git-wrapper.sh") | ForEach-Object {
    if (Test-Path $_) { Copy-Item $_ $serverDir }
}
Set-Content "$serverDir\VERSION" $ver

# Strip Python artifacts
Get-ChildItem $serverDir -Recurse -Include "__pycache__","*.pyc","*.pyo" | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue

Push-Location $dist
tar -czf "devtrack-server-$ver.tar.gz" "devtrack-server-$ver"
Remove-Item -Recurse "devtrack-server-$ver"
Pop-Location

# ---------------------------------------------------------------------------
# Checksums
# ---------------------------------------------------------------------------
Step "Computing checksums"

Push-Location $dist
$files = Get-ChildItem -File -Path "devtrack_*.tar.gz","devtrack_*.zip","devtrack-server-*.tar.gz"
$checksums = $files | ForEach-Object {
    $hash = (Get-FileHash $_.Name -Algorithm SHA256).Hash.ToLower()
    "$hash  $($_.Name)"
}
$checksums | Set-Content checksums.txt
Write-Host ($checksums -join "`n")
Pop-Location

# ---------------------------------------------------------------------------
# Publish GitHub release
# ---------------------------------------------------------------------------
Step "Publishing GitHub release $tag"

$prev = git tag --sort=-version:refname | Where-Object { $_ -match '^v\d' } | Where-Object { $_ -ne $tag } | Select-Object -First 1

$notes = @"
## DevTrack $tag

Pre-built binaries for macOS (Apple Silicon + Intel), Linux (amd64 + arm64), and Windows (amd64).

> **Two deployment modes** — choose during ``devtrack setup``:
> - **Standalone**: git tracking, CLI tools, and AI commit messages — no Python backend required.
> - **Full**: all of the above plus AI work updates, project management sync, and reporting (requires ``devtrack-server``).

### Quick install

``````bash
OS=`$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=`$(uname -m); [ "`$ARCH" = "x86_64" ] && ARCH="amd64"; [ "`$ARCH" = "aarch64" ] && ARCH="arm64"
curl -fsSL "https://github.com/sraj0501/Devtrack_/releases/latest/download/devtrack_`${OS}_`${ARCH}.tar.gz" | tar xz
sudo mv devtrack /usr/local/bin/
devtrack setup
``````

**Full Changelog**: https://github.com/sraj0501/Devtrack_/compare/$prev...$tag
"@

$assets = @(
    "dist\devtrack_linux_amd64.tar.gz",
    "dist\devtrack_linux_arm64.tar.gz",
    "dist\devtrack_darwin_amd64.tar.gz",
    "dist\devtrack_darwin_arm64.tar.gz",
    "dist\devtrack_windows_amd64.zip",
    "dist\devtrack-server-$ver.tar.gz",
    "dist\checksums.txt"
)

gh release create $tag `
    --title "DevTrack $tag" `
    --notes $notes `
    @assets

# ---------------------------------------------------------------------------
# Update wiki version + push (triggers Netlify deploy)
# ---------------------------------------------------------------------------
Step "Updating wiki version to $tag"

$htmlPath = "devtrack_wiki\wiki\download.html"
(Get-Content $htmlPath) -replace 'Latest: v[\d.]+', "Latest: $tag" | Set-Content $htmlPath

git add $htmlPath
git commit -m "chore: bump wiki version to $tag"
git push origin main

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
Step "Release $tag complete"
Write-Host "  GitHub release : https://github.com/sraj0501/Devtrack_/releases/tag/$tag"
Write-Host "  Wiki deploys   : Netlify will pick up the push to main automatically"

Pop-Location  # back to original dir
