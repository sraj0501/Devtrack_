---
name: project_local_agents
description: Local Claude Code agents for DevTrack PM workflow — project-vision (PM), devtrack-engineer, post-generator
type: project
---

## Overview

Three local agents in `.claude/agents/` define an automated PM↔engineer workflow. The developer hands a plan to the PM agent; the PM breaks it into tasks, dispatches the engineer, and tracks everything via shared files.

**Why:** Automates the project management overhead of feature work — task breakdown, branch enforcement, commit logging, and post-task documentation — using the same devtrack CLI the tool is designed to demonstrate.

**How to apply:** Always invoke `project-vision` first when starting new work. It creates the task on the board and dispatches `devtrack-engineer`. Never invoke `devtrack-engineer` directly without a board task.

---

## Agent Files

| Agent | File | Role |
|---|---|---|
| `project-vision` | `.claude/agents/project-vision.md` | PM: plan intake, task breakdown, engineer dispatch, board tracking, vision enforcement |
| `devtrack-engineer` | `.claude/agents/devtrack-engineer.md` | Engineer: code work, commits via devtrack CLI, board updates, engineer log |
| `post-generator` | `.claude/agents/post-generator.md` | Weekly post generation from engineer logs (dev.to, HN Show HN, LinkedIn) |

---

## Runtime Files (gitignored, created at runtime)

| File | Purpose |
|---|---|
| `Data/agent_logs/project_board.md` | Shared PM↔engineer task board; single source of truth for task state |
| `Data/agent_logs/engineer_log.md` | Per-commit and per-task log: AI enhancement quality, ticket linking, time saved |

---

## Project Board Format

States: `IN PROGRESS`, `PLANNED`, `DONE`, `BLOCKED`

Each task has:
- Task ID: `TASK-NNN`
- Branch name (mandatory; engineer checks out before first commit)
- Spec + acceptance criteria (written by PM)
- Engineer status field (updated by engineer after every commit)
- PR URL (added by engineer when task completes)

---

## Core Rules (enforced by both agents)

1. **No direct push to `main`** — every task lives on a `fix/TASK-NNN-*`, `features/TASK-NNN-*`, or `docs/TASK-NNN-*` branch
2. **All commits via `devtrack git commit`** — never raw `git commit` (raw git allowed only for checkout/branch/merge/status/diff/log/push)
3. **Board updated after every commit** — engineer status field + acceptance criteria checkboxes
4. **Engineer log updated after every commit** — records original vs. enhanced message, ticket auto-link result, time, friction level
5. **PR on task completion** — `gh pr create`; do NOT merge without PM/developer approval

---

## Engineer Log Entry Format

```markdown
### [YYYY-MM-DD HH:MM] TASK-NNN — <what was committed>

**Original message**: "..."
**DevTrack enhanced it to**: "..."
**Ticket auto-linked**: YES/NO — <ID if yes>
**PM system updated**: YES/NO — <what was posted>
**Time**: ~X seconds
**Friction**: LOW / MEDIUM / HIGH
**Notes**: <anything interesting, surprising, or broken>
```

---

## Post-Generator Usage

Invoke weekly to turn engineer log data into content:
- **dev.to**: long-form technical article with code snippets
- **HN Show HN**: concise project pitch
- **LinkedIn**: personal professional update

Source: `Data/agent_logs/engineer_log.md` — reads task summaries, commit stats, time-saved metrics.

---

## Shipped

- April 5, 2026: `devtrack-engineer.md` + `post-generator.md` added (commit `0cd0fad`)
- April 5, 2026: `project-vision.md` added with full vision-enforcement responsibilities (commit `8431dc3`)
- April 5, 2026: PM↔engineer coordination protocol wired in — board format, dispatch flow, status reporting (commit `3c4a037`)
- April 5, 2026: No-direct-push-to-main rule enforced in both agents (commit `fe56f2c`)
