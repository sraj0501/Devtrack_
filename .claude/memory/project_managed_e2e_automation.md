---
name: Managed end-to-end automation
description: Approved plan for full product-level Windows, WSL/Linux, and macOS validation
type: project
---

`docs/MANAGED_E2E_AUTOMATION_PLAN.md` is the plan authority. It supplements rather than replaces the
fast `scripts/e2e.ps1`/`scripts/e2e.sh` client lane and the already-configured
`scripts/demo.ps1`/`scripts/demo.sh` acceptance flow.

The new canonical test must isolate all configuration and data, provision PostgreSQL, run Managed
Go/Python startup and migrations, use a real disposable ticket-linked commit, verify server-backed
staging, exercise admin review in a real browser, generate voice and EOD output, confirm the same
state through all six MCP tools, retain sanitized failure evidence, and clean only resources bearing
the generated run ID.

Safety profiles are part of the contract: **Safe** is the default and rejects staged actions without
external delivery; **LocalIntegration** may approve only to disposable local mock receivers and must
prove exactly-once dispatch; **LiveSandbox** is a future explicit opt-in and is not part of the first
milestone. Secrets belong in process environment or protected input files, never command arguments,
logs, or Git.

Implementation order: non-interactive `devtrack setup` → native Windows Safe runner → Playwright
admin coverage → local mock delivery/idempotency → WSL launcher plus POSIX runner → dedicated Linux
machine → dedicated macOS machine → hosted and packaged-release lanes. WSL is valid Linux product
evidence but not proof of standalone systemd/desktop behavior; it cannot provide macOS evidence.

No implementation existed when this plan was recorded on 2026-09-07. The development hold and the
existing validation rules in `docs/END_TO_END_VALIDATION.md` remain active.
