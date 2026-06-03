---
name: Project current state
description: v3.0.0 — decoupling Phases 1+2 + runtime-narrative DONE; PG-5 + Redis next
type: project
---

**Version:** v3.0.0 (2026-06-04). Active branch: `feat/client-server-decoupling`.

**Monorepo layout (GitHub `sraj0501/Devtrack_`):**
- `devtrack_client/` — Go binary + Go-native `gitsage/` (Python git_sage removed). Module: `github.com/sraj0501/Devtrack_/devtrack_client`
- `devtrack_server/` — Python AI pipeline, NLP, LLM, boardroom, admin UI. Port: **8089**
- `devtrack_wiki/` — docs site, Netlify → devtrack.cloud
- `devtrack-bin/` and root `backend/` — LEGACY (TASK-048), no new code

**GitLab fully retired.** GitHub sole source of truth. Releases via `.\scripts\release.ps1 [-Bump patch|minor|major]`.

**Go internal packages** (`devtrack_client/internal/`): `config`, `db`, `health`, `learning`, `trigger`, `infra`, `daemon`, `tui`, `match`, `alerts`, `notify`. Layer order (acyclic): config/db/health/learning <- trigger <- infra <- daemon; trigger <- tui; package main on top.

**Completed work (in order):**
- Decoupling Phase 1: `workspaces.yaml` sole connector config; reports/learning/auth/license -> HTTP; help/status rewritten.
- Decoupling Phase 2 (`abad449`): native Go alerts (GitHub + Azure) + notifiers (Terminal, Telegram, Slack, OS). Daemon no longer spawns Python alert subprocesses.
- Ticket sync: `ticket_sync.go` -> `POST /trigger/ticket_sync`; Python upserts `ticket_cache` via SQLAlchemy.
- Runtime-narrative (`cac67a4`): all 4 phases shipped — OllamaFailureAnalyzer wired, stage coverage (LLM/PM/boardroom/report), `narrative_reader.py`, admin panel, Go `devtrack logs --narrative` + status last-failure, docs at `devtrack_server/docs/RUNTIME_NARRATIVE.md`.
- PG-1->PG-4: `engine.py`, `project_store`, `ticket_db`, `platform_store`, `learning_store`, `admin/user_manager` all on SQLAlchemy.

**NEXT (ordered):** PG-5 boundary enforcement (`stats_client.py` -> `/internal/stats`) -> Redis R-1->R-6.

**Open decision:** gitsage AI commit enhancement is client-native (gitsage -> Ollama), collides with "AI=server" rule — `docs/CAPABILITIES_OWNERSHIP.md`.

**Keep:** `migration` branch — historical reference, do NOT delete.
