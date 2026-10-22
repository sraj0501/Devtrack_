[CmdletBinding()]
param(
    [ValidateRange(10, 300)]
    [int]$TimeoutSeconds = 60
)

$ErrorActionPreference = 'Stop'

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

    $previousPreference = $ErrorActionPreference
    $stderrFile = [IO.Path]::GetTempFileName()
    try {
        $ErrorActionPreference = 'Continue'
        # Only stdout is command data. Timestamped startup logs on stderr
        # must not participate in status comparisons across daemon restarts.
        $output = @(& $Executable @Arguments 2> $stderrFile)
        $exitCode = $LASTEXITCODE
        $diagnostics = Get-Content -Raw -LiteralPath $stderrFile
    }
    finally {
        $ErrorActionPreference = $previousPreference
        Remove-Item -LiteralPath $stderrFile -Force
    }
    if ($exitCode -ne 0) {
        throw "$Executable exited with code $exitCode`n$($output -join [Environment]::NewLine)`n$diagnostics"
    }
    if ($diagnostics) {
        Write-Host $diagnostics.TrimEnd()
    }
    return @($output | ForEach-Object { $_.ToString() })
}

foreach ($name in @('go', 'git')) {
    if (-not (Get-Command $name -ErrorAction SilentlyContinue)) {
        throw "Missing required command: $name"
    }
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$tempBase = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$testRoot = Join-Path $tempBase ("devtrack-e2e-{0}" -f [guid]::NewGuid().ToString('N'))
$stateRoot = Join-Path $testRoot 'state'
$workspace = Join-Path $testRoot 'workspace'
$binary = Join-Path $testRoot 'bin/devtrack.exe'
$envFile = Join-Path $testRoot 'devtrack.env'
$daemonStarted = $false
$previousEnvFile = $env:DEVTRACK_ENV_FILE
$previousGitBypass = $env:GIT_NO_DEVTRACK
$previousGoCache = $env:GOCACHE
$previousXdgDataHome = $env:XDG_DATA_HOME

try {
    foreach ($path in @(
        (Split-Path -Parent $binary),
        $workspace,
        (Join-Path $stateRoot 'db'),
        (Join-Path $stateRoot 'logs'),
        (Join-Path $stateRoot 'pids'),
        (Join-Path $stateRoot 'configs'),
        (Join-Path $stateRoot 'learning')
    )) {
        New-Item -ItemType Directory -Path $path -Force | Out-Null
    }

    Write-Host 'Building the Windows DevTrack binary...'
    $env:GOCACHE = Join-Path $testRoot 'go-cache'
    Push-Location (Join-Path $repoRoot 'devtrack_client')
    try {
        Invoke-Checked -Executable go -Arguments @('build', '-o', $binary, '.')
    }
    finally {
        Pop-Location
    }

    Invoke-Checked git -C $workspace init -q
    Invoke-Checked git -C $workspace config user.name 'DevTrack E2E'
    Invoke-Checked git -C $workspace config user.email 'e2e@localhost'
    Set-Content -LiteralPath (Join-Path $workspace 'README.md') -Value 'isolated e2e workspace' -Encoding utf8
    Invoke-Checked git -C $workspace add README.md
    Invoke-Checked git -C $workspace commit -q -m 'initial e2e repository'

    $ipcPort = Get-Random -Minimum 39000 -Maximum 44000
    $httpPort = $ipcPort + 1
    $unreachableServerPort = $ipcPort + 2
    $lines = @(
        "PROJECT_ROOT=$workspace"
        "DEVTRACK_HOME=$stateRoot"
        "DEVTRACK_WORKSPACE=$workspace"
        "WORKSPACES_FILE=$(Join-Path $stateRoot 'workspaces.yaml')"
        "DATABASE_DIR=$(Join-Path $stateRoot 'db')"
        "LOG_DIR=$(Join-Path $stateRoot 'logs')"
        "PID_DIR=$(Join-Path $stateRoot 'pids')"
        "CONFIG_DIR_PATH=$(Join-Path $stateRoot 'configs')"
        "LEARNING_DIR_PATH=$(Join-Path $stateRoot 'learning')"
        'CLI_BINARY_NAME=devtrack.exe'
        'CONFIG_FILE_NAME=config.yaml'
        'DATABASE_FILE_NAME=devtrack.db'
        'PID_FILE_NAME=daemon.pid'
        'LOG_FILE_NAME=daemon.log'
        'LEARNING_DIR_NAME=learning'
        'CONFIG_DIR_NAME=.devtrack'
        'CLI_APP_NAME=DevTrack-E2E'
        'CLI_DAEMON_NAME=devtrack-e2e'
        'DEVTRACK_SERVER_MODE=lightweight'
        "DEVTRACK_SERVER_URL=http://127.0.0.1:$unreachableServerPort"
        'DEVTRACK_TLS=false'
        'DEVTRACK_API_KEY='
        'IPC_HOST=127.0.0.1'
        "IPC_PORT=$ipcPort"
        'IPC_CONNECT_TIMEOUT_SECS=2'
        "DEVTRACK_SERVER_HTTP_PORT=$httpPort"
        'PROMPT_INTERVAL=30'
        'WORK_HOURS_ONLY=false'
        'WORK_START_HOUR=0'
        'WORK_END_HOUR=23'
        'TIMEZONE=UTC'
        'LOG_LEVEL=info'
        'AUTO_SYNC=false'
        'OUTPUT_TYPE=console'
        'DAILY_REPORT_TIME=18:00'
        'WEEKLY_REPORT_DAY=Friday'
        'SEND_ON_TRIGGER=false'
        'SEND_DAILY_SUMMARY=false'
        'TEAMS_MENTION_USER=false'
        'LEARNING_DEFAULT_DAYS=30'
        'SERVER_EVENT_SYNC_ENABLED=false'
        'TICKET_SYNC_ON_START=false'
        'QUEUE_POLL_INTERVAL_SECS=60'
        'HEALTH_CHECK_INTERVAL_SECS=60'
        'VOICE_SYNC_INTERVAL_HOURS=24'
        'DEVTRACK_AUTO_ACCEPT_TERMS=1'
        'PYTHONIOENCODING=utf-8'
    )
    Set-Content -LiteralPath $envFile -Value $lines -Encoding ascii
    $env:DEVTRACK_ENV_FILE = $envFile
    $env:GIT_NO_DEVTRACK = '1'
    $env:XDG_DATA_HOME = Join-Path $stateRoot 'xdg'

    Write-Host 'Starting an isolated no-send daemon...'
    Invoke-Checked $binary workspace add e2e $workspace --pm none
    Invoke-Checked $binary start
    $daemonStarted = $true

    $pidFile = Join-Path $stateRoot 'pids/daemon.pid'
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    while (-not (Test-Path -LiteralPath $pidFile)) {
        if ([DateTime]::UtcNow -ge $deadline) {
            throw 'Daemon PID file was not created before the timeout.'
        }
        Start-Sleep -Milliseconds 250
    }

    Invoke-Checked git -C $workspace switch -q -c feature/E2E-201-automated-e2e
    Add-Content -LiteralPath (Join-Path $workspace 'README.md') -Value 'observed by the real daemon' -Encoding utf8
    Invoke-Checked git -C $workspace add README.md
    Invoke-Checked git -C $workspace commit -q -m 'E2E-201: verify automated end-to-end flow'
    $commitHash = (& git -C $workspace rev-parse --short=12 HEAD).Trim()
    if ($LASTEXITCODE -ne 0) {
        throw 'Could not resolve the E2E commit hash.'
    }

    $logFile = Join-Path $stateRoot 'logs/daemon.log'
    $observed = $false
    while ([DateTime]::UtcNow -lt $deadline) {
        if ((Test-Path -LiteralPath $logFile) -and
            ((Get-Content -Raw -LiteralPath $logFile) -match [regex]::Escape($commitHash))) {
            $observed = $true
            break
        }
        Start-Sleep -Milliseconds 500
    }
    if (-not $observed) {
        throw "Daemon did not observe commit $commitHash before the timeout."
    }

    $mcpOutput = (Invoke-Captured $binary @('mcp', 'test') | Out-String)
    if ($mcpOutput -notmatch '=== PASS ===' -or
        $mcpOutput -notmatch 'E2E-201' -or
        $mcpOutput -notmatch 'today_commits[^0-9]*1([^0-9]|$)') {
        throw "MCP context did not contain the observed commit and local-day count:`n$mcpOutput"
    }

    # Ticket-context commands are repository-scoped and must run in the test workspace.
    Push-Location $workspace
    try {
        $initialStatus = (Invoke-Captured $binary @('work', 'status') | Out-String)
        if ($initialStatus -notmatch 'No active ticket selected\.') {
            throw "Unexpected initial ticket context:`n$initialStatus"
        }
        Invoke-Checked $binary work start E2E-202
        $selectedStatus = (Invoke-Captured $binary @('work', 'status') | Out-String)
        if ($selectedStatus -notmatch 'Active ticket: E2E-202') {
            throw "Ticket was not selected:`n$selectedStatus"
        }
        Invoke-Checked $binary stop
        # Cross a timestamp boundary; startup logs must not enter status comparisons.
        Start-Sleep -Milliseconds 1100
        Invoke-Checked $binary start
        $restartedStatus = (Invoke-Captured $binary @('work', 'status') | Out-String)
        if ($restartedStatus -ne $selectedStatus) {
            throw "Ticket context changed across daemon restart:`n$restartedStatus"
        }
        Invoke-Checked $binary work stop
        $clearedStatus = (Invoke-Captured $binary @('work', 'status') | Out-String)
        if ($clearedStatus -notmatch 'No active ticket selected\.') {
            throw "Ticket selection was not cleared:`n$clearedStatus"
        }
    }
    finally { Pop-Location }

    # Persisted commits must remain visible after restarting the daemon.
    $restartedMcp = (Invoke-Captured $binary @('mcp', 'test') | Out-String)
    if ($restartedMcp -notmatch '=== PASS ===' -or
        $restartedMcp -notmatch 'E2E-201' -or
        $restartedMcp -notmatch 'today_commits[^0-9]*1([^0-9]|$)') {
        throw "Persisted commit context changed across daemon restart:`n$restartedMcp"
    }

    # This lane deliberately has no server and no staged local actions.
    # Unavailable combined history must not appear as a successful empty queue.
    $queueErrorFile = Join-Path $testRoot 'queue-error.txt'
    $queuePreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $queueOutput = @(& $binary queue list 2> $queueErrorFile)
        $queueExitCode = $LASTEXITCODE
        $queueDiagnostics = Get-Content -Raw -LiteralPath $queueErrorFile
    }
    finally { $ErrorActionPreference = $queuePreference }
    if ($queueExitCode -ne 1 -or
        $queueDiagnostics -notmatch 'warning: server queue unavailable; showing local queue only:' -or
        $queueDiagnostics -notmatch 'Error: server queue history unavailable:') {
        throw "Unexpected offline queue result (exit $queueExitCode):`n$queueOutput`n$queueDiagnostics"
    }
    Write-Host 'PASS: ticket selection, restart persistence, clearing, and persisted commit context.'
    Write-Host "PASS: Windows no-send E2E observed $commitHash and exposed it through MCP."
}
finally {
    if ($daemonStarted -and (Test-Path -LiteralPath $binary)) {
        $pidFile = Join-Path $stateRoot 'pids/daemon.pid'
        $daemonPid = $null
        if (Test-Path -LiteralPath $pidFile) {
            $daemonPid = [int](Get-Content -Raw -LiteralPath $pidFile).Trim()
        }
        $cleanupPreference = $ErrorActionPreference
        try {
            $ErrorActionPreference = 'Continue'
            & $binary stop 2>&1 | Out-Null
        }
        finally {
            $ErrorActionPreference = $cleanupPreference
        }
        if ($daemonPid -and (Get-Process -Id $daemonPid -ErrorAction SilentlyContinue)) {
            Stop-Process -Id $daemonPid -Force -ErrorAction SilentlyContinue
        }
        $exitDeadline = [DateTime]::UtcNow.AddSeconds(5)
        while ($daemonPid -and (Get-Process -Id $daemonPid -ErrorAction SilentlyContinue) -and
            [DateTime]::UtcNow -lt $exitDeadline) {
            Start-Sleep -Milliseconds 100
        }
    }
    $env:DEVTRACK_ENV_FILE = $previousEnvFile
    $env:GIT_NO_DEVTRACK = $previousGitBypass
    $env:GOCACHE = $previousGoCache
    $env:XDG_DATA_HOME = $previousXdgDataHome

    $resolved = [IO.Path]::GetFullPath($testRoot)
    $insideTemp = $resolved.StartsWith($tempBase, [StringComparison]::OrdinalIgnoreCase)
    $validName = [IO.Path]::GetFileName($resolved).StartsWith('devtrack-e2e-', [StringComparison]::Ordinal)
    if ((Test-Path -LiteralPath $resolved) -and $insideTemp -and $validName) {
        try { Remove-Item -LiteralPath $resolved -Recurse -Force }
        catch { Write-Warning "Could not remove temporary E2E directory ${resolved}: $_" }
    }
}
