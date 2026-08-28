---
name: Documentation surfaces
description: Canonical docs/site surfaces, release boundary, and stale-claim rules
type: project
---

**Why:** Public prose had mixed removed commands, release artifacts, and pre-split architecture with current behavior; verify claims against executable routes and workflows.
**Sources:** `PRODUCT_BIBLE.md` → product; `docs/ARCHITECTURE.md` → boundary; `CLAUDE.md` → build; board → tasks; `devtrack_wiki/wiki/` → public site.
**Release:** v3.0.10 stays latest until a newer tag; label `dev` work unreleased. Linux/macOS assets are `.tar.gz`; Windows is direct `.exe`.
**Runtime:** client SQLite; server PostgreSQL required. Use `OLLAMA_HOST`/`LMSTUDIO_HOST`; `health`, `server-tui`, and `admin-start` are not Go commands.
**Channels:** Telegram controls daemon/queue and supports corrections, not PM browsing/planning. Go Slack is outbound webhook delivery; optional legacy Python Socket Mode is not the client path.
**History:** `docs/split-manifest.md` and `docs/CLIENT_SERVER_DECOUPLING_PLAN.md` are historical records, not installation guides.
