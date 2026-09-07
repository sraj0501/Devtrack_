# Managed end-to-end automation plan

## Purpose

DevTrack needs a repeatable product-level test that exercises the complete Managed-mode user
journey, not only the Go/SQLite/MCP boundary. The existing `scripts/e2e.ps1` and `scripts/e2e.sh`
remain the fast, credential-free core lane. The existing `scripts/demo.ps1` and `scripts/demo.sh`
remain useful for an installation that is already configured. This plan adds the missing clean,
isolated Managed-mode lane around those tests.

Implementation starts on native Windows 11, then reuses the same contract through WSL for Linux.
Native Linux and macOS runs will follow on dedicated machines after the Windows workflow is stable.
Feature development remains paused while this validation work is active.

## Canonical product journey

Every platform implementation must validate the same sequence:

1. Start from an isolated, empty DevTrack state.
2. Build or install the DevTrack client under test.
3. Configure Managed mode without interactive input.
4. Provision an isolated PostgreSQL database.
5. Start the Go daemon and Managed Python server.
6. Verify migrations, HTTP health, admin UI, and configured LLM readiness.
7. Register a disposable Git workspace with outbound PM delivery disabled.
8. Create a real ticket-linked commit.
9. Verify commit detection, ticket extraction, persistence, and action staging.
10. Review the staged action through the admin UI.
11. Generate a real voice profile and EOD report.
12. Verify the resulting state through the MCP tools.
13. Stop all processes and remove only resources created by the test.

The test must use real DevTrack output. It must not insert canned queue rows or rewrite IDs,
counts, timestamps, generated text, or runtime evidence.

## Safety profiles

### Safe

This is the default local and CI profile. It uses an isolated PostgreSQL database and the configured
local Ollama service, but no real PM, email, Teams, Slack, Telegram, or telemetry destination. It
checks that an action can be reviewed and rejected, proving the review path without delivering it.

### LocalIntegration

This profile adds local mock PM and notification receivers. It approves a staged action and verifies
that the mock receiver gets the expected sanitized request exactly once. It exercises dispatch,
retry, and idempotency without contacting an external account.

### LiveSandbox

This future opt-in profile targets credentials and projects supplied explicitly for a disposable
external sandbox. It must never be selected by default, must not reuse production destinations, and
is outside the first implementation milestone.

## Phase 1 — Define the executable contract

- Turn the canonical journey above into stable assertions and timeouts.
- Define which state belongs in Go SQLite and which belongs in server PostgreSQL.
- Treat PostgreSQL-backed admin actions separately from the local CLI queue when continuous client
  event synchronization is disabled.
- Define a unique run ID used in temporary directories, database names, container names, workspace
  names, logs, and browser artifacts.
- Preserve diagnostics on failure and clean successful runs by default.

## Phase 2 — Add non-interactive setup

Extend `devtrack setup` with an automation-safe interface instead of piping answers into the current
wizard. The interface must support:

- Managed or External mode selection.
- Workspace path and PM type.
- PostgreSQL configuration supplied through the environment or a protected input file, never
  printed or placed directly in command-line arguments.
- LLM provider, host, and model selection.
- A custom isolated data/config root.
- Explicitly disabling autostart for test runs.
- Stable exit codes and optional machine-readable status.
- The same validation and generated configuration used by the interactive wizard.

Illustrative Windows invocation:

```powershell
$env:DEVTRACK_POSTGRES_URL = '<private test URL>'
devtrack setup --non-interactive `
  --mode managed `
  --workspace $testWorkspace `
  --pm none `
  --llm-provider ollama `
  --no-autostart
```

The exact flag names should be finalized with focused CLI tests before the Managed runner depends
on them.

## Phase 3 — Native Windows Managed runner

Add `scripts/e2e-managed.ps1`. It will:

- Build the current Windows binary into a uniquely named temporary directory.
- Set isolated DevTrack home, environment, config, SQLite, logs, learning, and PID paths.
- Start a dedicated PostgreSQL Docker container or create a uniquely prefixed database in an
  explicitly selected test instance.
- Run non-interactive Managed setup.
- Wait for background Python installation, dependency synchronization, migrations, model
  preparation, server startup, and daemon readiness.
- Require `devtrack doctor` to report the capabilities needed by the selected profile.
- Create a disposable repository and make a real `DEMO-301` commit.
- Wait for detection and verify the commit hash, `DEMO-301` mapping, confidence-bearing staging,
  voice generation, and EOD staging.
- Exercise all six MCP tools against the same run state.
- Collect diagnostics before cleanup when an assertion fails.

Target commands:

```powershell
.\scripts\e2e-managed.ps1 -Profile Safe
.\scripts\e2e-managed.ps1 -Profile LocalIntegration
.\scripts\e2e-managed.ps1 -Profile Safe -KeepArtifacts
```

## Phase 4 — Admin-console browser automation

Add a small Playwright suite that validates the UI as a user would:

- Load the login page and authenticate with per-run test credentials.
- Confirm the dashboard and service-readiness state.
- Find the action staged for the disposable commit.
- Inspect its type, ticket, confidence, and payload summary.
- Reject it in the Safe profile and verify the audit result.
- Approve it only in the LocalIntegration profile and verify mock delivery.
- Confirm that the generated EOD action is visible.

On failure, preserve a screenshot, Playwright trace, relevant HTML snapshot, Go log, Python log,
`devtrack doctor` output, and PostgreSQL/container logs. Artifacts must not contain credentials or
unredacted private machine paths when published.

## Phase 5 — Local integration receivers

Provide disposable local receivers for PM and notification traffic. The LocalIntegration profile
must verify:

1. DevTrack stages rather than immediately dispatches an action.
2. Admin approval causes one outbound request to the configured local receiver.
3. The method, route, destination, and sanitized payload are correct.
4. The action transitions to the posted state.
5. A replay or retry does not deliver the action twice.
6. Rejection produces no outbound request.

These receivers are test fixtures, not new production integrations.

## Phase 6 — WSL/Linux reuse

Add `scripts/e2e-managed.sh` for POSIX hosts and `scripts/e2e-managed-local.ps1` as the Windows
launcher. From Windows, the intended command is:

```powershell
.\scripts\e2e-managed-local.ps1 -Platform Linux -Profile Safe
```

The launcher should translate the repository path and run the same contract inside WSL. Prefer an
isolated PostgreSQL container reachable from WSL rather than the contributor's normal database.
WSL validates Linux binaries, paths, permissions, Managed Python startup, database behavior, Git
observation, MCP, and the admin UI. It is not sufficient evidence for a clean standalone Linux
installation, systemd autostart, or Linux desktop notifications.

Equivalent direct POSIX commands:

```bash
./scripts/e2e-managed.sh --profile safe
./scripts/e2e-managed.sh --profile local-integration
```

## Phase 7 — Dedicated Linux and macOS machines

After Windows and WSL are stable, run the same POSIX contract on the dedicated machines. Add
platform-specific acceptance checks separately:

- Linux: supported clean install, systemd user service, filesystem permissions, and native desktop
  behavior where applicable.
- macOS: supported clean install, Darwin binary, launchd, application data paths, executable
  permissions, and Gatekeeper/quarantine behavior.

WSL and Linux containers must never be presented as native macOS evidence. Autostart tests remain a
separate lane because they modify user-level machine configuration.

## Phase 8 — CI and packaged-install acceptance

- Keep the current lightweight Windows/Ubuntu E2E as the fast pull-request lane.
- Add a deterministic mock-LLM Managed lane where practical.
- Run the full Ollama-backed Managed lane periodically or on demand because model setup is slower.
- Run Safe on hosted Windows and Ubuntu, then add macOS after its PostgreSQL service setup is stable.
- Test both current source and packaged release artifacts; source success does not prove installation
  and packaging correctness.
- Upload diagnostic artifacts on failure and apply retention limits.

## Completion criteria

The Managed automation milestone is complete when:

- Native Windows passes twice from isolated clean state.
- WSL passes the same Managed product journey.
- Browser automation proves admin login, inspection, rejection, and audit behavior.
- LocalIntegration proves approval and exactly-once delivery to a local receiver.
- Voice, EOD, and all six MCP tools reflect the real disposable commit.
- No default-profile run contacts a real outbound destination.
- Failed runs retain useful, sanitized evidence.
- Cleanup leaves the user's real DevTrack installation, databases, containers, and configuration
  untouched.
- The same contract later passes on the dedicated Linux and macOS machines.

## Implementation order

1. Non-interactive setup contract and focused tests.
2. Native Windows Safe runner.
3. Admin Playwright coverage.
4. LocalIntegration mock delivery and idempotency.
5. Windows-to-WSL launcher and POSIX runner.
6. Dedicated Linux validation.
7. Dedicated macOS validation.
8. Hosted CI and packaged-release lanes.

