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

Use a disposable repository and no PM or email destination. On Windows PowerShell:

```powershell
.\scripts\demo.ps1 -Mode Check
.\scripts\demo.ps1 -Mode Record
```

On Linux or macOS:

```bash
./scripts/demo.sh --check
./scripts/demo.sh --record
```

A passing run must use real output and show all six MCP tools, a completed commit, ticket extraction,
confidence-bearing action staging, an EOD narrative containing the observed commit, an incremented
local-day commit count, and cleanup of the disposable workspace. Do not approve any action merely to
complete the demonstration.

The Windows local-user gate passed twice on 2026-09-04. The remaining release gates are a clean
Windows account or machine, the POSIX demo, PostgreSQL-backed action review in the admin UI, and
privacy-reviewed screenshots/video. Roadmap feature development remains on hold until those gates
pass.

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
