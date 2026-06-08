---
name: Project current state
description: v3.0.2 — motor optional, upgrade fixed; dev synced with main; PG-5 + Redis next
type: project
---

**Version:** v3.0.2. Branch: `dev` (synced with main 2026-06-08). GitHub (`sraj0501/Devtrack_`) sole source. Releases: `scripts/release.ps1 [-Bump patch|minor|major]`.

**Layout:** `devtrack_client/` (Go + gitsage), `devtrack_server/` (Python, port 8089), `devtrack_wiki/` (Netlify→devtrack.cloud). Legacy: `devtrack-bin/` + root `backend/` (TASK-048, no new code).

**Go internal packages** (`devtrack_client/internal/`): `config`, `db`, `health`, `learning`, `trigger`, `infra`, `daemon`, `tui`, `match`, `alerts`, `notify`. Layer: config/db/health/learning ← trigger ← infra ← daemon; trigger ← tui.

**Completed:** Phase 1+2 decoupling → ticket sync → runtime-narrative → boardroom → SQLAlchemy PG-1→PG-4.

**Recent fixes (2026-06-08):** v3.0.1 removed `motor` from mandatory deps (MongoDB optional: `uv sync --extra mongodb`). v3.0.2 switched `upgrade.go` from retired GitLab API to GitHub API + semver guard.

**Bootstrap problem:** Users on v2.2.19 cannot self-upgrade; must manually install v3.0.2 from GitHub releases.

**NEXT:** PG-5 (`stats_client.py` → `GET /internal/stats`, stop reading Go SQLite directly) → Redis R-1→R-6.

**Open decision:** gitsage AI commit enhancement is client-native (→Ollama), collides with "AI=server" rule — `docs/CAPABILITIES_OWNERSHIP.md`.

**Keep:** `migration` branch — do NOT delete.
