---
name: DevTrack workflow rules
description: Non-negotiable rules for git, commits, architecture, and code style
type: feedback
---

## Git & PR Rules

- **PRs always target `dev`, never `main`.** Use `--base dev`. `dev→main` is an explicit developer action only. (Agent opened PR #79 to `main` 2026-04-23 — lesson learned.)
- **Never commit without explicit user instruction.** Stop before staging; wait for "commit it."
- **Always work on a feature branch.** Never edit on `main` or `dev`. Naming: `features/TASK-NNN-*`, `fix/TASK-NNN-*`, `docs/TASK-NNN-*`.
- **Use `GIT_NO_DEVTRACK=1` on all git commands** (add, commit, push, tag). The devtrack wrapper intercepts git. No exceptions even when daemon is stopped.
- **Use `devtrack git commit`, not raw `git commit`.** Raw git OK for checkout, branch, merge, status, diff, log, push.

## Architecture Rules

- **Offline-first is Rule 0.** Ollama + SQLite, no internet required. `DEVTRACK_SERVER_MODE=managed` is primary. Degrade: cloud LLM → Ollama → raw text.
- **CLI stays CLI/TUI.** Go binary never launches browser or serves HTML. Admin GUI is Python-side only.
- **No hardcoded values.** Go: `config_env.go`. Python: `backend/config.py`. `os.getenv` banned outside `config.py`.
- **No API keys/credentials in committed files.** Use `<placeholder>` in `.env_sample`. (Tokens were once committed to markdown — scrubbed from history.)
- **Wiki GIFs only when feature works end-to-end.** VHS `.tape` files live in `wiki/tapes/` until verified.
