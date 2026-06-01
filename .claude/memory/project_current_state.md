---
name: Project current state
description: v3.0.0 — decoupling Phases 1+2 complete; PG-1→PG-4 done, PG-5 + Redis next
type: project
---

**Version:** v3.0.0 (2026-06-02). Active branch: `feat/client-server-decoupling`.

**Monorepo layout (all on GitHub `sraj0501/Devtrack_`):**
- `devtrack_client/` — Go binary + Go-native gitsage. Module: `github.com/sraj0501/Devtrack_/devtrack_client`
- `devtrack_server/` — Python AI pipeline, NLP, LLM, boardroom, admin UI. Port: **8089**
- `devtrack_wiki/` — docs site, Netlify auto-deploys on push to main (devtrack.cloud)
- `devtrack-bin/` and root `backend/` — LEGACY (TASK-048), no new code

**GitLab: fully retired.** GitHub is sole source of truth. No GitHub Actions — releases via `.\scripts\release.ps1 [-Bump patch|minor|major]`.

**Go internal packages** (`devtrack_client/internal/`): `config`, `db`, `health`, `learning`, `trigger`, `infra`, `daemon`, `tui`, `match`, `alerts`, `notify`. Layer order (acyclic): config/db/health/learning ← trigger ← infra ← daemon; trigger ← tui; package main on top.

**Client-server decoupling — Phases 1+2 COMPLETE** (`abad449`):
- Phase 1: `workspaces.yaml` sole non-secret connector config; reports/learning/auth/license → HTTP; `devtrack workspace add` offers git init; help/status rewritten.
- Phase 2: `internal/alerts/` + `internal/notify/` — native Go alerts (GitHub + Azure) and notifiers (Terminal, Telegram, Slack, OS). Daemon no longer spawns Python alert subprocesses.

**Ticket sync SHIPPED:** `ticket_sync.go` pushes PM tickets to `/trigger/ticket_sync`; Python upserts into `ticket_cache` via SQLAlchemy.

**PostgreSQL migration status:**
- PG-1 `db/engine.py` (SQLAlchemy factory, dialect-aware upsert): DONE
- PG-2 `project_store`, `ticket_db`, `platform_store` → SQLAlchemy Core: DONE
- PG-3 `learning_store` → SQLAlchemy Core: DONE
- PG-4 `admin/user_manager.py` → shared engine (PG) / separate admin.db (SQLite): DONE
- PG-5 boundary enforcement (`stats_client.py` calls `/internal/stats` in PG mode): IN PROGRESS
- Redis R-1→R-6: NOT STARTED

**Open decision:** git-sage AI enhancement is client-native (gitsage → Ollama directly), collides with "AI=server" rule — see `docs/CAPABILITIES_OWNERSHIP.md`.

**Keep:** `migration` branch — historical reference, do NOT delete.
