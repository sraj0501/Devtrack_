param([switch]$KeepArtifacts)

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
$client = Join-Path $repo "devtrack_client"
$testRoot = Join-Path ([IO.Path]::GetTempPath()) ("devtrack-sage-acceptance-" + [guid]::NewGuid().ToString("N"))
$binary = Join-Path $testRoot "devtrack.exe"
$dataHome = Join-Path $testRoot "data-home"
$codexHome = Join-Path $testRoot "codex-home"

try {
    New-Item -ItemType Directory -Path $testRoot, $dataHome, $codexHome -Force | Out-Null
    Push-Location $client
    try { go build -o $binary . } finally { Pop-Location }

    $env:XDG_DATA_HOME = $dataHome
    $env:CODEX_HOME = $codexHome
    Set-Content -LiteralPath (Join-Path $codexHome "hooks.json") -Encoding utf8 -Value '{"custom":"keep","hooks":{"PostToolUse":[{"hooks":[{"type":"command","command":"warp-user-hook"}]}]}}'

    $available = (& $binary sage harness list | ConvertFrom-Json)
    if ($available.Count -ne 1 -or $available[0].id -ne "codex") { throw "unexpected harness registry" }
    $install = (& $binary sage harness install codex | ConvertFrom-Json)
    if (-not $install.installed -or $install.mode -ne "codex-history") { throw "unexpected install result" }
    $hookConfig = Get-Content -Raw -LiteralPath (Join-Path $codexHome "hooks.json")
    if ($hookConfig -notmatch "warp-user-hook") { throw "unrelated hook was not preserved" }

    $payload = '{"session_id":"acceptance-thread","cwd":"C:/private/project","hook_event_name":"PostToolUse","tool_name":"Bash","tool_use_id":"acceptance-call","tool_input":{"command":"git status --short"},"tool_response":"SAGE_PRIVATE_CANARY"}'
    $hookOutput = $payload | & $binary sage hook codex --devtrack-sage-hook
    if ($null -ne $hookOutput) { throw "capture hook wrote output" }

    $status = (& $binary sage status | ConvertFrom-Json)
    if ($status.capture -ne "spool" -or $status.backlog -ne 1) { throw "capture did not reach spool" }
    $eventPath = Get-ChildItem -LiteralPath (Join-Path $dataHome "devtrack/sage/spool/pending") -Filter "*.json" | Select-Object -First 1
    $event = Get-Content -Raw -LiteralPath $eventPath.FullName
    if ($event -match "SAGE_PRIVATE_CANARY" -or $event -match "C:/private") { throw "private hook data leaked" }

    $remove = (& $binary sage harness uninstall codex | ConvertFrom-Json)
    if ($remove.installed) { throw "uninstall did not disable capture" }
    Write-Host "SAGE packaged capture acceptance passed: install, silent capture, privacy, status, uninstall"
}
finally {
    Remove-Item Env:XDG_DATA_HOME -ErrorAction SilentlyContinue
    Remove-Item Env:CODEX_HOME -ErrorAction SilentlyContinue
    if (-not $KeepArtifacts -and (Test-Path -LiteralPath $testRoot)) {
        $resolved = (Resolve-Path -LiteralPath $testRoot).Path
        $tempBase = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
        if (-not $resolved.StartsWith($tempBase, [StringComparison]::OrdinalIgnoreCase)) { throw "refusing cleanup outside temp" }
        Remove-Item -LiteralPath $resolved -Recurse -Force
    } elseif ($KeepArtifacts) {
        Write-Host "Artifacts: $testRoot"
    }
}
