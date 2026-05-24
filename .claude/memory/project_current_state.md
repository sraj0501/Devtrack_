---
name: Project current state
description: v2.2.14 — completed work and immediate next tasks
type: project
---

**Version:** v2.2.14. Active branch: `features/SPLIT-001-monorepo-restructure`.

**Done:**
- EPIC-SPLIT (TASK-041–TASK-048): `devtrack_client/`, `devtrack_server/`, `devtrack_wiki/` canonical; `devtrack-bin/` and root `backend/` deleted.
- TASK-049 (wiki rewrite): COMPLETE 2026-05-24. Wiki is how-to only — no ADRs, no phase docs.
- Boardroom (`devtrack boardroom` / `devtrack plan`, 7 AI personas), upgrade via GitLab Releases, uninstall, health, Windows single-instance lock.
- Server port: **8089**.

**What's next (in order):**
1. TASK-050: GitLab cut-over — subtree-push after full e2e testing.
2. Delete `devtrack_contract` GitLab repo (DEAD).
3. Build-runner: create `devtrack3_cloud/build-runner` repo, then run engineer against `docs/build-runner-plan.md` (BR-001–BR-009 on board).

**Keep:** `migration` branch — historical reference, do NOT delete.
