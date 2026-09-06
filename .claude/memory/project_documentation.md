---
name: Documentation surfaces
description: Canonical docs/site surfaces, release boundary, and stale-claim rules
type: project
---

**Why:** Public prose had mixed removed commands, release artifacts, and pre-split architecture with current behavior; verify claims against executable routes and workflows.
**Sources:** `PRODUCT_BIBLE.md` → product; `docs/ARCHITECTURE.md` → boundary; `CLAUDE.md` → build; board → tasks; `devtrack_wiki/wiki/` → public site.
**Release:** v3.1.1 is latest. Public assets include Linux/macOS `.tar.gz`, a Windows `.exe`, five platform/architecture `.mcpb` bundles, `checksums.txt`, and `server.json`. Keep historical v3.0.10/v3.1.0 statements only inside dated records; current guides must describe v3.1.1.
**Runtime:** client SQLite; server PostgreSQL required. Use `OLLAMA_HOST`/`LMSTUDIO_HOST`; `health`, `server-tui`, and `admin-start` are not Go commands.
**Contracts:** `docs/HTTP_API.md` records the implemented Go/Python HTTP boundary. MCP uses local stdio, not the Python HTTP API; `docs/REGISTRY_SUBMISSION_PACKAGE.md` owns held listing copy and release/submission gates.
**Validation:** `docs/END_TO_END_VALIDATION.md` owns the temporary development hold, current test evidence, remaining clean-Windows/full-Managed-Linux/admin/media gates, capture privacy rules, and exit criteria. The isolated core lane uses `scripts/e2e.ps1` on Windows and `scripts/e2e.sh` on Linux; `scripts/e2e-local.ps1` orchestrates Windows plus WSL or a Docker fallback. It passed locally and in hosted Windows/Ubuntu run `34045590767` at `ed0f571`, but validates client/daemon/SQLite/MCP only and must not be presented as PostgreSQL/Python/LLM acceptance. `docs/DEMO_STORYBOARD.md` owns the real-output Managed demo/recording sequence; Windows uses `scripts/demo.ps1`, while POSIX hosts use `scripts/demo.sh`, and both have noninteractive automation options. Do not resume roadmap documentation or present screenshots as approved until the full exit criteria pass.
**Channels:** Telegram controls daemon/queue and supports corrections, not PM browsing/planning. Go Slack is outbound webhook delivery; optional legacy Python Socket Mode is not the client path.
**History:** `docs/split-manifest.md` and `docs/CLIENT_SERVER_DECOUPLING_PLAN.md` are historical records, not installation guides.
