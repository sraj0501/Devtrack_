---
name: DevTrack workflow rules
description: All non-negotiable rules for git, commits, architecture, and code style — merged from individual feedback files
type: feedback
---

## Git & PR Rules

- **PRs always target `dev`, never `main`.** Use `--base dev` on every `gh pr create`. `dev→main` is an explicit developer action only. (Reinforced after PM agent opened PR #79 directly to `main` on 2026-04-23.)
- **Never commit without explicit user instruction.** Make edits freely; stop before staging. Present commands for the user to run, or wait for "commit it" in that message.
- **Always work on a feature branch.** Never edit files while on `main` or `dev`. Naming: `features/TASK-NNN-*`, `fix/TASK-NNN-*`, `docs/TASK-NNN-*`.
- **Use `GIT_NO_DEVTRACK=1` prefix on all git commands** (add, commit, push, tag). The devtrack wrapper intercepts git and can block Claude's commands. No exceptions even when daemon is not running.

## Architecture Rules

- **Offline-first is Rule 0 — unbreakable.** Everything must work on a laptop with Ollama + SQLite, no internet. `DEVTRACK_SERVER_MODE=managed` is the primary mode, not legacy. Ollama = first-class LLM; SQLite = primary DB. Degrade gracefully: cloud LLM → Ollama → raw text.
- **CLI stays CLI/TUI.** The Go binary must never launch a browser, serve HTML, or become a GUI. Bubble Tea TUI is fine. Admin GUI lives on the Python server side only.
- **No hardcoded values.** All config via env vars. Go: add to `config_env.go`. Python: add to `backend/config.py`. `os.getenv` is banned outside `backend/config.py`.
- **No API keys or credentials in committed files.** Use `<placeholder>` in `.env_sample` and docs. Real or fabricated tokens must never appear. (Two tokens were committed to markdown and had to be scrubbed from history.)

## Content Rules

- **Wiki GIFs only when the feature actually works end-to-end.** VHS `.tape` files live in `wiki/tapes/` until the workflow is functional. Never add a GIF to the hero or sales pages for an unverified feature.
