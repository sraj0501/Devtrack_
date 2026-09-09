# Windows admin review acceptance

Validated on native Windows on 2026-09-09: real commit `5947fc5c7986` staged action 17
at confidence 0.95; Chromium rejected it and verified its reviewer, timestamp, and audit
event. EOD action 18 was generated and displayed with the demo ticket. MCP passed and
the disposable workspace was removed. Screenshots were visually inspected. The focused
admin, queue, HTTP-contract and EOD regression set passed **134 tests** (existing
deprecation warnings only). This is one complete pass against an existing installation.

The same runner supports a visible prototype and a headless acceptance run.

To see the working prototype using the existing installation, run:

```powershell
.\scripts\e2e-admin.ps1 -Showcase
```

This opens a visible browser, demonstrates the real commit/review/EOD flow, then leaves
authenticated dashboard, reviewed-action, and EOD tabs open for inspection. Close that
browser window to stop the temporary admin server. Automated trace capture stops before
the browser is handed to you. No installation or clean-machine check is performed.

To reopen a previous successful demo without creating another commit or report:

```powershell
.\scripts\e2e-admin.ps1 -Showcase -InspectResult .codex-cache/admin-e2e-<run-id>/result.json
```

This verifies the saved commit hash against the live action and opens the existing
PostgreSQL records. It requires the existing PostgreSQL service to be running. Closing
a tab or the browser after handoff is normal and does not invalidate the recorded demo.
Inspection starts only the admin UI; live generation also needs the Managed daemon
(`devtrack start`). A down webhook badge means its health endpoint is unreachable,
even when historical actions can still be viewed from PostgreSQL.

The visible browser uses the current window size with no fixed emulated viewport.
The admin layout fills the available width, shrinks wide content within cards, changes
dashboard columns for smaller windows, and moves navigation above the page on narrow
screens. Live dashboard, server, queue-list, and EOD-detail checks passed at widths
375, 768, 1024, 1440, and 1920 pixels without document-level horizontal overflow.

For the headless acceptance run:

```powershell
uv sync --project devtrack_server --group e2e
uv run --project devtrack_server --group e2e python -m playwright install chromium
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/e2e-admin.ps1
```

Use `-EnvFile C:\path\to\private.env` for a different existing Managed installation.
The default is `$HOME/.local/share/devtrack/.env`. The file must contain the working
PostgreSQL, admin, LLM, and client configuration. For a hashed `ADMIN_PASSWORD`, supply
the login password in the process environment as `DEVTRACK_E2E_ADMIN_PASSWORD`.
Never put passwords in command arguments or checked-in files.

The runner starts a temporary admin server from this checkout on a free loopback port,
sharing the configured PostgreSQL database. It uses the installed `devtrack` client and
Managed server for the real commit and EOD journey. It restarts the daemon to register
and later remove its uniquely named PM-`none` workspace, following `scripts/demo.ps1`.
Run when a brief daemon restart is acceptable. It requires an already working local
PostgreSQL and LLM environment; it is not clean-install or full isolated Managed coverage.

The browser logs in, checks the dashboard, finds the run's unique workspace/ticket action,
inspects its payload, commit hash and confidence, rejects that exact action as soon as it
appears (while the demo continues), checks the reviewer/time
and audit event, then opens the exact EOD action ID emitted by the CLI. No approval or
external delivery is performed. EOD generation reads today's actual tracked work, so its
contents may be private. The rejected action and pending EOD remain as real runtime evidence.
The run raises `HTTP_TIMEOUT_LONG` and `HTTP_TIMEOUT` to at least `-StageTimeoutSeconds` in its child environment
so a slow local model can finish staging. The private `.env` file is not changed.

Artifacts live in ignored `.codex-cache/admin-e2e-<run-id>/`: screenshots, a Playwright
trace, admin/demo logs, IDs, and a passing result JSON. Failures after login additionally
preserve HTML, exception details and `devtrack doctor` output. The trace starts after
login, but authenticated requests still contain session cookies; all artifacts are
private and must not be committed or published. Review and crop screenshots separately
before sharing. Browser network traffic is restricted to the temporary admin origin.

The temporary admin process and browser close on completion. Demo workspace cleanup uses
the existing demo's guarded temporary-directory removal. A failed cleanup is reported in
`demo.log`; inspect it before rerunning. No existing database or user configuration is deleted.

The UI blocker found during this work was a missing server-backed queue review page.
`/admin/queue` now lists pending actions, with individual detail/rejection pages. Rejection
and its audit record are transactional, and rejection competes atomically with execution
for a pending action. An execution already in progress cannot be rejected; the UI returns
409 on a stale review. A process crash after an execution claim leaves `executing` for
manual investigation rather than automatically retrying a potentially delivered action.

The first live attempt exposed two additional test assumptions: the existing 60-second
client request budget expired before real staging completed, and waiting for the whole
demo allowed the two-minute auto-approval window to expire before browser review. The
PM-`none` action was processed without PM delivery. The runner now reviews concurrently
with the demo, validates the exact commit hash, and budgets the request for local generation.
The next attempt successfully rejected and audited the real commit action, but EOD hit
the installed client's separate 30-second standard HTTP timeout. Both request budgets
are now set for the acceptance run.

Linux, mock-receiver approval/idempotency, clean installs, and launch media remain separate
work. Playwright trace behavior follows the [official tracing documentation](https://playwright.dev/python/docs/api/class-tracing).
