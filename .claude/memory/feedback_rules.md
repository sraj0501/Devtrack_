---
name: DevTrack workflow rules
description: Non-negotiable Git, consent, architecture, and dependency rules
type: feedback
---

- PRs target `dev`; `dev` → `main` requires explicit authorization (PR #79 incident). Use only approved task-branch prefixes.
- Never mutate the board, commit, push, open/merge a PR, publish, or deploy without an explicit request. Prefix Git with `GIT_NO_DEVTRACK=1`; commit through `devtrack git commit`, with raw Git only as a reported fallback.
- Preserve unrelated work and never commit credentials (past Markdown token leak). Samples use placeholders.
- No telemetry leaves without opt-in (TASK-109 broken opt-out incident). Enabled integrations get required data; every PM/email/Git write uses `pending_actions`.
- Offline-first is Rule 0: Go + SQLite + Ollama default; optional cloud providers and Python services degrade without blocking Git.
- The daemon never prompts in the main flow. TUI/Telegram/notifications are visibility and correction channels, never capability gates.
- The Go client stays terminal-only. New client/server messages use HTTP/JSON; configuration is centralized in Go `internal/config/` and Python `backend/config.py`.
- Python uses `uv`, never `pip`. Isolate `DATABASE_DIR` tests and reset the LLM provider cache around tests that change `LLM_PROVIDER`.
