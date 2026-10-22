---
name: Documentation surfaces
description: Canonical documentation sources and stale-claim rules
type: project
---

**Sources:** `PRODUCT_BIBLE.md` owns product direction; `docs/ARCHITECTURE.md` owns runtime boundaries; `agent-memory/operations/build-guide.md` owns shared build guidance; `Data/agent_logs/project_board.md` owns task status and IDs; `devtrack_wiki/wiki/` is the public site.

**Runtime claims:** the client uses SQLite and the server uses PostgreSQL. Use `OLLAMA_HOST` and `LMSTUDIO_HOST`; `health`, `server-tui`, and `admin-start` are not Go commands.

**Contracts:** `docs/HTTP_API.md` records the Go/Python HTTP boundary. MCP uses local stdio, not the Python HTTP API. `docs/REGISTRY_SUBMISSION_PACKAGE.md` owns listing copy and submission gates.
`docs/TICKET_MAPPING.md` is the user-facing mapping guide for incidents, stories, bugs, and tasks;
`agent-memory/initiatives/ticket-mapping.md` preserves its maintenance contract.

**Active documentation rule:** closed validation steps, task histories, dated test runs, and superseded plans must not be surfaced as current work. The packaged/container checkpoint has passed; the immediate priority is native installed workflow acceptance, controlled delivery, and a representative workday. Sage qualification, learning repairs, and platform parity remain on hold. UI WIP, privacy-reviewed media, and the Glama path/badge are deferred follow-ups. Keep run evidence in docs/testing/ or release notes, not active memory.

**Version boundary:** v3.3.0 preparation is on `dev`; rolling development packages identify as `dev-<commit>`. The last recorded stable release is v3.2.0. A source push does not upgrade the installed host or establish packaged/native/delivery qualification. Historical release notes retain the behavior of their release, including retired time features.

**Channels:** Telegram controls daemon/queue and supports corrections, not PM browsing/planning. Go Slack is outbound webhook delivery; optional legacy Python Socket Mode is not the client path.

**History boundary:** `docs/split-manifest.md` and `docs/CLIENT_SERVER_DECOUPLING_PLAN.md` are historical records, not installation guides.
