---
name: Project current state
description: v2.2.22 — GitLab retired, GitHub is sole source of truth, devtrack.cloud live
type: project
---

**Version:** v2.2.22 (2026-05-27). Branch: `main`.

**Monorepo layout (all on GitHub `sraj0501/Devtrack_`):**
- `devtrack_client/` — Go binary + bundled git-sage
- `devtrack_server/` — Python AI pipeline, NLP, LLM, boardroom, admin UI. Port: **8089**
- `devtrack_wiki/` — docs site, Netlify auto-deploys from `devtrack_wiki/wiki/` on push to main
- `devtrack-bin/` and root `backend/` — LEGACY, being retired in TASK-048, no new code

**GitLab: fully retired.** All repos deleted/archived as of 2026-05-27.

**Releases:** Local only — `.\scripts\release.ps1 [-Bump patch|minor|major]`. Builds 5 targets, creates GitHub release via `gh`, updates wiki version badge, pushes to main. No GitHub Actions workflows remain.

**Website:** devtrack.cloud on Netlify (paid). `install.sh` / `install.ps1` at devtrack.cloud/install.* fetch from GitHub releases.

**Keep:** `migration` branch — historical reference, do NOT delete.
