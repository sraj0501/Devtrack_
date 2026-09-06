# Verification

## Installed application

```bash
devtrack version
devtrack doctor
devtrack start
devtrack status
devtrack mcp test
devtrack queue
devtrack work report
```

In managed mode, `doctor` must distinguish immediately available Go capabilities from Python,
PostgreSQL, and LLM readiness. A degraded optional capability must not be reported as a broken Git
workflow.

`devtrack mcp test` must list and call all six read-only tools. It uses the configured SQLite
database when available and a disposable one when setup has not run, so the protocol smoke test is
valid from a fresh checkout. Once the daemon has observed work, confirm MCP's `today_commits` count
uses the user's local calendar date.

## No-send end-to-end workflow

The source checkout now includes a deterministic, credential-free test of the Go-native product
boundary. On Windows 11, run native Windows and the Linux lane together:

```powershell
.\scripts\e2e-local.ps1
```

If local PowerShell policy blocks repository scripts, use a process-scoped bypass that does not
change machine policy:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\e2e-local.ps1
```

The launcher uses WSL when Go is installed in that distribution. If WSL has no Go toolchain, it
runs the same Linux test in a disposable `golang:1.24-bookworm` container through Docker Desktop;
it does not install packages or alter WSL. Run one platform directly with:

```powershell
.\scripts\e2e.ps1
```

```bash
sh ./scripts/e2e.sh
```

The test builds the current client into isolated temporary paths, starts the lightweight daemon,
creates a real `DEMO-201` commit, waits for local SQLite observation, and verifies that MCP exposes
the ticket and a non-zero local-day commit count. PM delivery, telemetry, email, server-event sync,
and automatic approvals stay disabled. GitHub Actions is configured to run the same scripts on
native Windows and Ubuntu. Local native-Windows and Linux-container runs passed on 2026-09-06; that
evidence does not imply the uncommitted workflow has already passed on GitHub-hosted runners.

## Full Managed acceptance workflow

Use a disposable repository and no PM or email destination. On Windows PowerShell:

```powershell
.\scripts\demo.ps1 -Mode Check
.\scripts\demo.ps1 -Mode Record
.\scripts\demo.ps1 -Mode Record -Automated
```

On Linux or macOS:

```bash
./scripts/demo.sh --check
./scripts/demo.sh --record
./scripts/demo.sh --record --automated
```

A passing run must use real output and show all six MCP tools, a completed commit, ticket extraction,
confidence-bearing action staging, an EOD narrative containing the observed commit, an incremented
local-day commit count, and cleanup of the disposable workspace. Do not approve any action merely to
complete the demonstration.

The Windows local-user Managed gate passed twice on 2026-09-04. The remaining release gates are a
clean Windows account or machine, the full POSIX Managed demo, PostgreSQL-backed action review in
the admin UI, and privacy-reviewed screenshots/video. The automated client lane supplements rather
than replaces those gates. Roadmap feature development remains on hold until the full gates pass.

## Source checkout

```bash
cd devtrack_client
GOCACHE=/tmp/devtrack-go-cache GOMODCACHE=/tmp/devtrack-go-mod go test ./...
GOCACHE=/tmp/devtrack-go-cache GOMODCACHE=/tmp/devtrack-go-mod go vet ./...

cd ../devtrack_server
UV_CACHE_DIR=/tmp/devtrack-uv-cache uv sync --extra ai
UV_CACHE_DIR=/tmp/devtrack-uv-cache uv run pytest backend/tests/ -q
```

The Python server test environment needs a valid PostgreSQL lane where integration behavior is under
test. Tests that mutate `DATABASE_DIR` or `LLM_PROVIDER` must isolate and reset their state.

## Documentation and website

```bash
python3 devtrack_wiki/check_inline_js.py
sh -n devtrack_wiki/wiki/install.sh
git diff --check
```

Also validate internal wiki page IDs and links before publishing.

The 2026-09-04 validation snapshot was all Go packages passing and Python at 959 passed, 10 skipped.
Treat those numbers as recorded evidence, not a permanent expected count.
