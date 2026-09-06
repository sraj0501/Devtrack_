# End-to-end validation hold

DevTrack feature development is paused while the product is exercised as a real user would use it.
Only changes that fix a reproduced installation, runtime, safety, or demo blocker belong in this
period. Do not resume roadmap feature work until the validation exit criteria below are met.

## Rules

- Use real DevTrack output. Do not seed fake queue entries or rewrite IDs, counts, timestamps, or
  generated text for screenshots and recordings.
- Keep PM integrations and delivery disabled during the first pass. Never approve a queued action
  merely to make a demo look complete.
- Keep passwords, tokens, local `.env` files, login sessions, databases, and raw recordings with
  private paths out of Git.
- Record every failure before fixing it. A fix must include a focused regression check and the
  relevant end-to-end scene must be rerun.
- Crop private paths and usernames in published media; do not replace runtime values with fabricated
  values.

## Validation environment

- Windows PowerShell host
- Go and `uv` available
- PostgreSQL reachable locally on port `55432`
- Ollama reachable locally with a generation-capable model
- Managed Python server installed under the user's DevTrack data directory

Credentials and full connection URLs are intentionally not recorded here.

## Current results

| Check | Result | Evidence |
|---|---|---|
| Go test suite | Pass | `go test ./...` |
| Python test suite | Pass | 959 passed, 10 skipped; existing deprecation warnings only |
| Fresh-checkout MCP smoke test | Pass after reproduced blocker fix | Six tools listed; `get_active_context` called successfully against disposable SQLite |
| PostgreSQL connectivity and migrations | Pass | `initialize_server_database()` completed against the local PostgreSQL instance |
| Python service startup | Pass | Local server started on `127.0.0.1:8089` |
| HTTP health | Pass | `GET /health` returned `status: ok` |
| Embedded admin login | Pass with complete documented admin environment | `/admin/` redirected to `/admin/login`; login page returned HTTP 200 |
| Managed setup | Pass after reproduced bootstrap fixes | Sparse checkout, Python environment, generation model, embedding model, and first-run profile completed |
| Readiness | Pass | `devtrack doctor` reports the AI server connected and all local capability groups ready |
| First-run voice profile | Pass after batching and bootstrap fixes | 23 commits mined; a 286-word profile was generated |
| Commit detection and ticket mapping | Pass twice | Disposable commits were detected and mapped to `DEMO-101` without blocking Git |
| Confidence-bearing action staging | Pass twice | Server staged `post_comment` actions at confidence `0.95`; no PM destination was configured |
| EOD narrative | Pass twice after PostgreSQL/local-data boundary fix | Real local commits were grouped under `DEMO-101`; reports staged as actions 10 and 12 |
| MCP active context | Pass twice after timestamp fix | Six tools introspected; `today_commits` advanced from 4 to 5 on the second pass |
| Disposable workspace cleanup | Pass twice | Only the real `Devtrack_` workspace remains after each run |
| Isolated native Windows core lane | Pass locally | Real `DEMO-201` commit observed by the daemon and exposed through MCP; isolated state cleaned |
| Isolated Linux core lane | Pass locally in Docker | Same script passed in disposable `golang:1.24-bookworm`; the local WSL distribution did not require modification |
| Hosted Windows/Ubuntu core lane | Pass | GitHub Actions End-to-end run `34045590767` passed both OS jobs at `ed0f571`; CI `34045590760` and wiki CI `34045590730` also passed for the commit |

## Automated cross-platform lane

The repository includes an isolated, credential-free E2E test for the Go-native product boundary.
It builds the current client, creates temporary configuration and a disposable Git repository,
starts the daemon in lightweight mode, makes a real ticket-linked commit, waits for observation,
and verifies that MCP exposes the commit and a non-zero local-day count. PM delivery, telemetry,
email, continuous server-event sync, and automatic approvals remain disabled.

Run both local lanes from Windows 11 with WSL:

```powershell
.\scripts\e2e-local.ps1
```

On a machine that blocks local scripts by policy, run the same test with a process-only bypass:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\e2e-local.ps1
```

The launcher uses the WSL distribution when it has Go installed. If WSL has no Go toolchain, it
runs the identical Linux script in the disposable `golang:1.24-bookworm` Docker container instead.
It does not install packages or modify the WSL distribution.

Or run one platform directly:

```powershell
.\scripts\e2e.ps1
```

```bash
sh ./scripts/e2e.sh
```

The included GitHub Actions workflow runs the same scripts on native `windows-latest` and
`ubuntu-latest` runners. Both jobs passed in End-to-end run `34045590767` at `ed0f571`. This
lane covers the cross-platform client/daemon/SQLite/MCP path. It does not replace the Managed-mode
acceptance gate below: PostgreSQL migrations, the real Python server, LLM generation, embedded admin
review, and media capture still require the full no-send demo and their existing focused tests.

Once a platform has a configured Managed environment, its full no-send acceptance flow can also run
without scene prompts:

```powershell
.\scripts\demo.ps1 -Mode Record -Automated
```

```bash
./scripts/demo.sh --record --automated
```

That acceptance lane intentionally remains environment-dependent: it uses the real PostgreSQL and
Python services and the configured local LLM, but still uses a disposable PM-`none` workspace and
never approves or delivers the staged actions.

### Reproduced blockers

`devtrack mcp test` panicked on a fresh checkout because opening the default database read mandatory
daemon configuration before its advertised fallback could run. The validation fix makes the command
use an explicitly selected database, the configured database, or a disposable SQLite database when
setup has not run.

Go trigger timestamps were stored in a driver-specific form that SQLite's `date()` could not parse,
so MCP showed the latest commit but reported zero commits today. New trigger timestamps are stored as
RFC 3339 and date queries remain compatible with existing rows.

The PostgreSQL-backed report generator could not see the Go-owned SQLite commits while continuous
client-event synchronization was correctly disabled by default. Explicit manual and scheduled EOD
requests now send only that day's minimal commit summaries; the privacy default remains unchanged.

First-run voice generation required the optional AI dependency group and an embedding model that
Managed setup did not install. Managed bootstrap now uses `uv sync --extra ai`, prepares
`nomic-embed-text`, batches embedding requests, and limits initial mining to 25 recent commits so the
first profile completes within the client timeout.

After a disposable demo workspace was removed, MCP could still present its newest historical commit
as active. Active-context selection now requires an existing, enabled configured workspace while
preserving those commits in the daily history.

## Remaining end-to-end gate

The Windows local-user gate passed twice on 2026-09-04. Remaining release-level checks are:

1. Repeat the supported installation path on a clean Windows account or machine without manual
   dependency preparation.
2. Repeat the full Managed `scripts/demo.sh --check` and `scripts/demo.sh --record` flow on Linux or
   CI. The isolated Linux core lane has passed, but it intentionally excludes PostgreSQL, Python,
   LLM generation, and server-backed staging.
3. Review server-backed queue actions in the admin UI; the local `devtrack queue list` intentionally
   does not mirror PostgreSQL actions when continuous event sync is disabled.
4. Capture the approved screenshot/video set from a privacy-reviewed terminal and browser session.

## Capture list

Capture screenshots only after the corresponding scene passes twice:

- `devtrack doctor` readiness map
- MCP tool list and active context containing real observed work
- a normal commit completing without interruption
- daemon evidence of ticket extraction and staged action confidence
- pending queue review
- generated EOD preview and its staging confirmation
- admin dashboard with secrets, usernames, machine paths, and private work data excluded or cropped

The launch video should follow `docs/DEMO_STORYBOARD.md`: immediate local memory, silent commit
detection, reviewable staging, EOD preview, then MCP reading the resulting context.

## Exit criteria

The hold ends only when a clean-machine user can complete the documented quickstart without help,
the full no-send demo passes twice from a clean local state, captured media matches actual behavior,
and all reproduced blockers have regression coverage. At that point, update this document with the
date and evidence before resuming feature development.
