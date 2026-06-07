---
name: Project current state
description: v3.0.0 — decoupling Phases 1+2 + runtime-narrative DONE; SQLAlchemy PG-1→PG-4 done; PG-5 + Redis next
type: project
---

**Version:** v3.0.0. Current branch: `main`. GitHub (`sraj0501/Devtrack_`) sole source. Releases: `scripts/release.ps1 [-Bump patch|minor|major]`.

**Layout:** `devtrack_client/` (Go + gitsage), `devtrack_server/` (Python, port 8089), `devtrack_wiki/` (Netlify→devtrack.cloud). Legacy: `devtrack-bin/` + root `backend/` (TASK-048, no new code).

**Go internal packages** (`devtrack_client/internal/`): `config`, `db`, `health`, `learning`, `trigger`, `infra`, `daemon`, `tui`, `match`, `alerts`, `notify`. Layer: config/db/health/learning ← trigger ← infra ← daemon; trigger ← tui.

**Completed:** client-server decoupling Phases 1+2 → ticket sync → runtime-narrative → SQLAlchemy PG-1→PG-4.

**NEXT:** PG-5 (`stats_client.py` → `GET /internal/stats`, stop reading Go SQLite directly) → Redis R-1→R-6.

**Open decision:** gitsage AI commit enhancement is client-native (→Ollama), collides with "AI=server" rule — `docs/CAPABILITIES_OWNERSHIP.md`.

**Keep:** `migration` branch — do NOT delete.
