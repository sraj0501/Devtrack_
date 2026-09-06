[CmdletBinding()]
param(
    [ValidateSet('Check', 'Record')]
    [string]$Mode = 'Check',

    [ValidateRange(1, 3600)]
    [int]$StageTimeoutSeconds = 120,

    [switch]$Automated
)

$ErrorActionPreference = 'Stop'

function Assert-Command {
    param([Parameter(Mandatory)][string]$Name)

    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Missing required command: $Name"
    }
}

function Invoke-Checked {
    param(
        [Parameter(Mandatory, Position = 0)][string]$Executable,
        [Parameter(Position = 1, ValueFromRemainingArguments)][string[]]$Arguments
    )

    & $Executable @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Executable exited with code $LASTEXITCODE"
    }
}

function Invoke-Captured {
    param(
        [Parameter(Mandatory, Position = 0)][string]$Executable,
        [Parameter(Position = 1)][string[]]$Arguments = @()
    )

    # Windows PowerShell 5 wraps redirected native stderr as non-terminating
    # NativeCommandError records. DevTrack intentionally logs diagnostics there,
    # so capture them without allowing ErrorActionPreference=Stop to abort.
    $previousPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $output = @(& $Executable @Arguments 2>&1)
        $exitCode = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $previousPreference
    }
    if ($exitCode -ne 0) {
        throw "$Executable exited with code $exitCode`n$($output -join [Environment]::NewLine)"
    }
    return @($output | ForEach-Object { $_.ToString() })
}

function Wait-ForScene {
    param([Parameter(Mandatory)][string]$Title)

    Write-Host "`n$Title"
    if (-not $Automated) {
        [void](Read-Host 'Press Enter to continue')
    }
}

foreach ($commandName in @('devtrack', 'git')) {
    Assert-Command $commandName
}

Write-Host 'Checking DevTrack CLI surfaces...'
$mcpStatus = (Invoke-Captured devtrack @('mcp', 'status') | Out-String)
if ($mcpStatus -notmatch [regex]::Escape('get_active_context') -or
    $mcpStatus -notmatch [regex]::Escape('6 registered')) {
    throw 'This binary does not expose the six-tool MCP surface required by the demo.'
}
Write-Host $mcpStatus.TrimEnd()
Invoke-Checked devtrack status

if ($Mode -eq 'Check') {
    Write-Host 'Preflight complete. Run with -Mode Record after devtrack doctor reports the AI server ready.'
    exit 0
}

$tempBase = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$demoRoot = Join-Path $tempBase ("devtrack-demo-{0}" -f [guid]::NewGuid().ToString('N'))
$demoName = "devtrack-demo-$PID"
$workspaceAdded = $false
$previousBypass = $env:GIT_NO_DEVTRACK

try {
    New-Item -ItemType Directory -Path $demoRoot | Out-Null
    $env:GIT_NO_DEVTRACK = '1'

    Wait-ForScene 'Scene 1/5 - Go-native MCP status and self-test'
    Invoke-Checked devtrack mcp status
    Invoke-Checked devtrack mcp test

    Invoke-Checked git -C $demoRoot init -q
    Invoke-Checked git -C $demoRoot config user.name 'DevTrack Demo'
    Invoke-Checked git -C $demoRoot config user.email 'demo@localhost'
    Set-Content -LiteralPath (Join-Path $demoRoot 'README.md') -Value 'credential-free demo' -Encoding utf8
    Invoke-Checked git -C $demoRoot add README.md
    Invoke-Checked git -C $demoRoot commit -q -m 'initial demo repository'

    Invoke-Checked devtrack workspace add $demoName $demoRoot --pm none
    $workspaceAdded = $true

    # Windows does not support the workspace-reload signal used on Unix. Restart
    # before committing so the daemon is definitely watching the disposable repo.
    Invoke-Checked devtrack restart
    $serverReady = $false
    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        $statusOutput = (Invoke-Captured devtrack @('status') | Out-String)
        if ($statusOutput -match 'AI server:\s+connected') {
            $serverReady = $true
            break
        }
        Start-Sleep -Seconds 1
    }
    if (-not $serverReady) {
        throw 'DevTrack did not become fully ready within 60 seconds after the workspace restart.'
    }

    Wait-ForScene 'Scene 2/5 - a normal commit on feature/DEMO-101-standup'
    Invoke-Checked git -C $demoRoot switch -q -c feature/DEMO-101-standup
    Add-Content -LiteralPath (Join-Path $demoRoot 'README.md') -Value 'The standup follows the commit.' -Encoding utf8
    Invoke-Checked git -C $demoRoot add README.md

    $stagingPattern = 'staged .*confidence=|PM sync staged .*confidence='
    $baseline = @(Invoke-Captured devtrack @('logs') | Where-Object { $_ -match $stagingPattern } | Sort-Object -Unique)
    Invoke-Checked git -C $demoRoot commit -m 'DEMO-101: document standup outcome'
    $demoHash = (& git -C $demoRoot rev-parse --short=12 HEAD).Trim()
    if ($LASTEXITCODE -ne 0) {
        throw 'Could not resolve the demo commit hash.'
    }

    Wait-ForScene 'Scene 3/5 - real detection and queue-staging evidence from daemon logs'
    $matched = $false
    for ($attempt = 0; $attempt -lt $StageTimeoutSeconds; $attempt++) {
        $logOutput = @(Invoke-Captured devtrack @('logs'))
        $current = @($logOutput | Where-Object { $_ -match $stagingPattern } | Sort-Object -Unique)
        $newStaging = @($current | Where-Object { $_ -notin $baseline })
        if (($logOutput -match [regex]::Escape($demoHash)) -and $newStaging.Count -gt 0) {
            $logOutput | Where-Object { $_ -match [regex]::Escape($demoHash) } | Select-Object -Last 6
            $newStaging | Select-Object -Last 6
            $matched = $true
            break
        }
        Start-Sleep -Seconds 1
    }
    if (-not $matched) {
        throw "Could not verify commit detection plus confidence-bearing staging within $StageTimeoutSeconds seconds. Run devtrack doctor and retry."
    }
    Invoke-Checked devtrack queue list

    Wait-ForScene 'Scene 4/5 - on-demand EOD narrative; no email and no approval'
    $eodOutput = (Invoke-Captured devtrack @('eod') | Out-String)
    Write-Host $eodOutput.TrimEnd()
    if ($eodOutput -notmatch 'Queued as action [0-9]+') {
        throw 'EOD generation returned without proof that the report was staged.'
    }

    Wait-ForScene "Scene 5/5 - MCP context after today's real commit"
    Invoke-Checked devtrack mcp test
    Write-Host "`nDemo complete. The disposable PM-none workspace will now be removed."
}
finally {
    $env:GIT_NO_DEVTRACK = $previousBypass
    if ($workspaceAdded) {
        try {
            Invoke-Captured devtrack @('workspace', 'remove', $demoName) | Out-Null
            Invoke-Captured devtrack @('restart') | Out-Null
        }
        catch {
            Write-Warning "Demo workspace cleanup needs manual review: $_"
        }
    }

    $resolvedDemoRoot = [IO.Path]::GetFullPath($demoRoot)
    $validName = [IO.Path]::GetFileName($resolvedDemoRoot).StartsWith('devtrack-demo-', [StringComparison]::Ordinal)
    $insideTemp = $resolvedDemoRoot.StartsWith($tempBase, [StringComparison]::OrdinalIgnoreCase)
    if ((Test-Path -LiteralPath $resolvedDemoRoot) -and $validName -and $insideTemp) {
        Remove-Item -LiteralPath $resolvedDemoRoot -Recurse -Force
    }
    elseif (Test-Path -LiteralPath $resolvedDemoRoot) {
        Write-Warning "Refusing to remove unexpected demo path: $resolvedDemoRoot"
    }
}
