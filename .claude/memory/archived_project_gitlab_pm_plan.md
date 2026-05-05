---
name: GitLab + PM Agent implementation plan
description: Detailed plan for completing GitLab integration and building the Project Manager Agent — saved for next session
type: project
---

Full plan saved at: `docs/PLAN_GITLAB_PM_AGENT.md` (1625 lines)

**Why:** User asked to plan two parallel features before ending session, to pick up next time.

**Two features planned:**

1. **GitLab Integration** — wire existing `backend/gitlab/client.py` into full stack (mirrors Azure pattern)
   - New files: `backend/gitlab/sync.py`, `assignment_poller.py`, `check.py`, `list_items.py`, `view_item.py`, `run_sync.py`
   - Telegram: `/gitlab`, `/gitlabissue <iid>`, `/gitlabcreate <title>`
   - Cache: `Data/gitlab/sync_state.json`

2. **PM Agent** — LLM decomposes a problem statement into Epic → Story/Feature/Bug → Task hierarchy, creates items in Azure/GitLab/GitHub
   - New files: `backend/pm_agent.py`, `backend/tests/test_pm_agent.py`
   - Telegram: `/plan <problem statement>` with platform picker → preview → confirm → create
   - Builds on existing `backend/ai/create_tasks.py` (TaskGenerator) and `backend/llm/` provider factory

**How to apply:** At the start of the next session on this topic, read `docs/PLAN_GITLAB_PM_AGENT.md` for exact file specs, method signatures, env vars, handler registration snippets, and implementation order. Both features can be built in parallel using isolated worktrees.
