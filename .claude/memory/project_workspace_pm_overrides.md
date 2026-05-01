---
name: workspace_pm_overrides
description: Per-workspace PM field overrides (assignee, iteration path, area path, milestone) now threaded from workspaces.yaml through WorkspaceRouter to platform API calls
type: project
---

# Per-Workspace PM Overrides

Four new per-workspace override fields are now fully wired end-to-end from `workspaces.yaml` → Go trigger → Python `WorkspaceRouter` → platform API.

**Why:** Multi-repo setups need each repo to route work items to the correct team/sprint/assignee. Previously all platform API calls used global defaults even when workspace-specific overrides were configured.

**How to apply:** When working on workspace routing or PM sync, these fields are now available throughout the stack. WorkspaceRouter.route() accepts them; _async_sync() applies them per-platform.

## Field Mapping

| workspaces.yaml field | Azure | GitHub | GitLab |
|---|---|---|---|
| `pm_assignee` | `assigned_to` (email/name) | `assignees=[login]` | `assignee_ids=[int user_id]` |
| `pm_iteration_path` | `iteration_path` | — | — |
| `pm_area_path` | `area_path` | — | — |
| `pm_milestone` | — | `milestone=int` | `milestone_id=int` |

## Key Files

- `backend/workspace_router.py` — `route()` params + `_async_sync()` platform branches
- `backend/webhook_server.py` — extracts `pm_assignee`, `pm_iteration_path`, `pm_area_path`, `pm_milestone` from POST data
- `backend/tests/test_workspace_router.py` — 4 new override tests (Azure/GitHub/GitLab/none)

## Notes

- GitLab `pm_assignee` must be an integer user ID string (not a username)
- GitHub `pm_milestone` must be an integer milestone number string
- Overrides are applied only on **create** (no-match path); matched items are commented/transitioned without re-assigning
- `pm_platform=none` still short-circuits before any override logic
