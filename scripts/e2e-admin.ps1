[CmdletBinding()]
param(
    [string]$EnvFile = (Join-Path $HOME '.local/share/devtrack/.env'),
    [ValidateRange(1, 3600)][int]$StageTimeoutSeconds = 300,
    [switch]$Showcase,
    [string]$InspectResult
)
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path $PSScriptRoot -Parent
$showcaseArgs = @()
if ($Showcase) { $showcaseArgs += '--showcase' }
if ($InspectResult) { $showcaseArgs += @('--inspect-result', $InspectResult) }
& uv run --project (Join-Path $repoRoot 'devtrack_server') --group e2e python `
    (Join-Path $PSScriptRoot 'e2e_admin.py') --env-file $EnvFile --stage-timeout $StageTimeoutSeconds @showcaseArgs
if ($LASTEXITCODE -ne 0) { throw 'Admin browser acceptance failed; see the private artifact directory above.' }
