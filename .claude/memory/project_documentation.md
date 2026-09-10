---
name: Documentation surfaces
description: Canonical documentation sources and stale-claim rules
type: project
---

**Sources:** `PRODUCT_BIBLE.md` owns product direction; `docs/ARCHITECTURE.md` owns runtime boundaries; `CLAUDE.md` owns build guidance; `Data/agent_logs/project_board.md` owns task status and IDs; `devtrack_wiki/wiki/` is the public site.

**Runtime claims:** the client uses SQLite and the server uses PostgreSQL. Use `OLLAMA_HOST` and `LMSTUDIO_HOST`; `health`, `server-tui`, and `admin-start` are not Go commands.

**Contracts:** `docs/HTTP_API.md` records the Go/Python HTTP boundary. MCP uses local stdio, not the Python HTTP API. `docs/REGISTRY_SUBMISSION_PACKAGE.md` owns listing copy and submission gates.

**Active documentation rule:** closed validation steps, task histories, dated test runs, and superseded plans must not be surfaced as current work. Current next-step documents should show only packaged-build acceptance, privacy-reviewed media, the Glama path/badge follow-up, current UI WIP, and DevTrack Sage planning where applicable.

**Channels:** Telegram controls daemon/queue and supports corrections, not PM browsing/planning. Go Slack is outbound webhook delivery; optional legacy Python Socket Mode is not the client path.

**History boundary:** `docs/split-manifest.md` and `docs/CLIENT_SERVER_DECOUPLING_PLAN.md` are historical records, not installation guides.
