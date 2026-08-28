# Quick Start Guide

Reach DevTrack's first local value in about two minutes, then let the optional AI service finish
preparing in the background.

> **Release note:** the latest public release is v3.0.10. It predates MCP and the Phase 9 onboarding
> flow below. Until a newer release ships, build the client from upstream `dev` as shown here.

---

## 1. Build the current client

You need Go and Git. Python, PostgreSQL, and Ollama are not required for the first MCP-backed local
context check.

```bash
git clone --branch dev --single-branch https://github.com/sraj0501/Devtrack_.git
cd Devtrack_/devtrack_client
go build -o devtrack .
sudo install -m 0755 devtrack /usr/local/bin/devtrack

devtrack mcp status   # should report six registered tools
```

On Windows PowerShell, build `devtrack.exe` and place it in a directory on `PATH`:

```powershell
git clone --branch dev --single-branch https://github.com/sraj0501/Devtrack_.git
Set-Location Devtrack_/devtrack_client
go build -o devtrack.exe .
devtrack.exe mcp status
```

See the [Installation Guide](INSTALLATION.md) for release binaries, external-server mode, Docker,
and platform-specific installation details.

---

## 2. Run setup from the repository you want to watch

```bash
cd /path/to/your/project
devtrack setup
```

The wizard writes and registers a complete environment file; future commands load it automatically.
You do not need to copy or source `.env` manually.

- Choose **Managed** for the default local AI service.
- Choose **None** for the PM integration if you want a local-only evaluation with no external target.
- Managed mode requires a PostgreSQL URL for the Python service.
- Ollama is the default LLM. If a usable local generation model is installed, setup reuses it.
- Checkout, `uv sync`, and any required model pull continue in a detached worker after setup returns.

No daemon, PM post, email, or Git push is started by the wizard.

---

## 3. Minute two: give Claude Code local context

The MCP server is Go-native, SQLite-backed, and started on demand over stdio. It does not wait for
Python, PostgreSQL, or an LLM model.

```bash
devtrack mcp setup
devtrack mcp test
```

Reload Claude Code after `mcp setup`, then ask:

> What am I working on?

The six local tools expose the active context, today's commits, pending actions, voice profile,
ticket context, and an EOD summary. An empty response on a brand-new install is expected; the value
is that the connection already works and begins filling as you commit.

---

## 4. Start the silent worker

```bash
devtrack start
devtrack status
devtrack doctor
```

`status` and `doctor` distinguish what works now from what is still preparing. Git monitoring,
ticket extraction, scheduling, SQLite, and MCP remain available if the optional AI service is still
installing or unavailable. Use `devtrack doctor --repair` after a failed or interrupted bootstrap.

The daemon is silent during normal work. It observes commits and stages actions; it does not prompt
or block your Git workflow.

---

## 5. Try the real workflow

When `devtrack doctor` reports the AI service ready, use a ticket-named branch and commit normally:

```bash
git switch -c feature/AUTH-42-refresh-token
# edit files as usual
git add path/to/changed-file
git commit -m "fix auth redirect"

devtrack logs
devtrack queue
devtrack eod
```

The expected flow is:

1. DevTrack detects the commit without interrupting it.
2. The branch name resolves ticket `AUTH-42`.
3. A ticket comment is staged with explicit confidence before any external action.
4. `devtrack eod` generates the current narrative and prints `Queued as action <id>` when server
   staging succeeds.

Do not run `devtrack queue approve` during a no-send evaluation. With `pm_platform: none` and no
`--email` argument, there is no PM or email destination.

Server-generated actions are stored in PostgreSQL. Depending on whether opt-in client-event sync is
enabled, `devtrack queue` may not mirror that server row in local SQLite immediately. The daemon log
and the `Queued as action <id>` response are the authoritative staging evidence; an empty local queue
does not mean the server bypassed staging.

For a disposable, recorder-friendly run that verifies live output and never manufactures queue
rows, use the repository's [credential-free demo storyboard](../../../docs/DEMO_STORYBOARD.md).

---

## Readiness map

| Capability | Before AI readiness | Requires the Python service |
|---|---:|---:|
| Git monitoring and ticket extraction | Yes | No |
| Local SQLite history and pending queue | Yes | No |
| MCP setup, self-test, and context tools | Yes | No |
| Voice-aware ticket-comment generation | No | Yes |
| Generated EOD narrative and server staging | No | Yes |

If model preparation takes longer than ten minutes, keep coding. DevTrack records the local signal
and never blocks the commit; `devtrack doctor` shows the recovery path.

---

## Everyday commands

```bash
devtrack status                 # daemon and capability summary
devtrack doctor                 # bootstrap readiness and recovery
devtrack logs -f                # follow daemon activity
devtrack queue                  # inspect local pending actions
devtrack queue approve <id>     # explicitly dispatch a reviewed local action
devtrack queue reject <id>      # discard a local pending action
devtrack eod                    # generate and stage today's narrative
devtrack work report            # immediate work-session report
devtrack mcp test               # verify local MCP context
devtrack stop                   # stop the daemon
```

Next: [Setup Wizard](../wiki.html#SETUP_WIZARD) · [Configuration](CONFIGURATION.md) ·
[Troubleshooting](TROUBLESHOOTING.md)
