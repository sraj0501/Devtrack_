---
name: Project current state
description: v3.0.5 — Telegram Go migration, upgrade fixed, wiki bumped; PG-5 + Redis next
type: project
---

**Version:** v3.0.5 (latest tag). Active branch: `dev`. GitHub (`sraj0501/Devtrack_`) sole source. Releases: `.\scripts\release.ps1 [-Bump patch|minor|major]`.

**Keep:** `migration` branch — do NOT delete.

**Layout:** `devtrack_client/` (Go + gitsage), `devtrack_server/` (Python, port 8089), `devtrack_wiki/` (Netlify → devtrack.cloud). Legacy: `devtrack-bin/` + root `backend/` (TASK-048, no new code).

**Go internal packages** (`devtrack_client/internal/`): `config`, `db`, `health`, `learning`, `trigger`, `infra`, `daemon`, `tui`, `match`, `alerts`, `notify`, `telegram`. Layer: config/db/health/learning ← trigger ← infra ← daemon; trigger ← tui.

**Recently shipped:**
- v3.0.1: `motor` moved from mandatory to optional dep (`uv sync --extra mongodb`)
- v3.0.2: `upgrade.go` switched from retired GitLab API → GitHub API; added semver guard
- v3.0.3–v3.0.5: Telegram bot migrated to Go (`devtrack_client/internal/telegram/`); Windows binary fix; wiki bumped
- Boardroom DONE: `devtrack_server/backend/boardroom/`; `devtrack boardroom` + `devtrack plan` live

**NEXT:** PG-5 (`stats_client.py` → `GET /internal/stats`, stop reading Go SQLite directly) → Redis R-1→R-6.

**Open decision:** gitsage AI commit enhancement is client-native (→Ollama), collides with "AI=server" rule — `docs/CAPABILITIES_OWNERSHIP.md`.
