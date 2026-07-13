---
name: DevTrack workflow rules
description: Non-negotiable rules for git, commits, consent, architecture, and code style
type: feedback
---

## Git & PR Rules

- **PRs always target `dev`, never `main`.** Use `--base dev`. `dev→main` is an explicit developer action only. (Agent opened PR #79 to `main` 2026-04-23 — lesson learned.)
- **Never commit without explicit user instruction.** Stop before staging; wait for "commit it."
- **Always work on a feature branch.** Never edit on `main` or `dev`. Naming: `features/TASK-NNN-*`, `fix/TASK-NNN-*`, `docs/TASK-NNN-*`.
- **Use `GIT_NO_DEVTRACK=1` on all git commands** (add, commit, push, tag). The devtrack wrapper intercepts git. No exceptions even when the daemon is stopped.
- **Use `devtrack git commit`, not raw `git commit`.** Raw git OK for checkout, branch, merge, status, diff, log, push.

## Consent & Data Rules

- **Nothing leaves the machine without consent.** Telemetry is **opt-in** (TASK-109; the previous opt-out was silently broken — `devtrack telemetry off` called the server and never wrote the local marker `ping.go` checks). Every outbound PM action stages in the pending-actions queue first.
- **No API keys/credentials in committed files.** `<placeholder>` in `.env_sample`. (Tokens were once committed to markdown — scrubbed from history.)

## Architecture Rules

- **Offline-first is Rule 0.** Ollama + SQLite, no internet required. `DEVTRACK_SERVER_MODE=managed` is primary. Degrade: cloud LLM → Ollama → raw text.
- **No prompts in the main daemon flow — ever.** TUI is visibility only: zero feature difference vs headless.
- **CLI stays CLI/TUI.** The Go binary never launches a browser or serves HTML. Admin GUI is Python-side only.
- **No hardcoded values.** Go: `config_env.go`. Python: `backend/config.py`. `os.getenv` banned outside `config.py`.
- **`docs/ARCHITECTURE.md` is the client↔server boundary doc.** New trigger types go through HTTP, not the legacy TCP IPC.
- **Python deps use `uv` throughout — never `pip`.**
