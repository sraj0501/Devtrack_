[CmdletBinding()]
param(
    [ValidateRange(10, 300)]
    [int]$TimeoutSeconds = 60,

    [switch]$SkipWindows,
    [switch]$SkipLinux
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot

if (-not $SkipWindows) {
    Write-Host "`n=== Native Windows E2E ==="
    & (Join-Path $PSScriptRoot 'e2e.ps1') -TimeoutSeconds $TimeoutSeconds
    if (-not $?) { throw 'Native Windows E2E failed.' }
}

if (-not $SkipLinux) {
    if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
        throw 'WSL is not available. Use -SkipLinux to run only the Windows lane.'
    }

    Write-Host "`n=== WSL Linux E2E ==="
    $wslInputPath = $repoRoot.Replace('\', '/')
    $linuxRepo = ((& wsl.exe wslpath -a $wslInputPath) -join '').Trim()
    if ($LASTEXITCODE -ne 0 -or -not $linuxRepo) {
        throw 'Could not translate the repository path for WSL.'
    }
    & wsl.exe sh -lc 'command -v go >/dev/null 2>&1'
    if ($LASTEXITCODE -eq 0) {
        & wsl.exe env "DEVTRACK_E2E_TIMEOUT_SECS=$TimeoutSeconds" sh "$linuxRepo/scripts/e2e.sh"
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    else {
        if (-not (Get-Command docker.exe -ErrorAction SilentlyContinue)) {
            throw 'The WSL distribution has no Go toolchain and Docker is unavailable.'
        }
        Write-Host 'WSL has no Go toolchain; running the same Linux lane in a disposable Docker container.'
        & docker.exe run --rm `
            --env "DEVTRACK_E2E_TIMEOUT_SECS=$TimeoutSeconds" `
            --volume "${repoRoot}:/workspace" `
            --workdir /workspace `
            golang:1.24-bookworm sh ./scripts/e2e.sh
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
}

Write-Host "`nPASS: requested local E2E lanes completed."
