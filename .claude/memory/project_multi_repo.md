---
name: Multi-Repo Monitoring Design
description: Architecture and implementation plan for monitoring multiple git repos, each routing to its own PM platform via workspaces.yaml
type: project
---

## Problem

Current single-repo design uses a priority chain (Azure → GitLab → GitHub fallback) regardless of which repo triggered the event. Wrong platform gets hit for repos that belong to a different PM.

## Solution: workspaces.yaml

File at `$PROJECT_ROOT/workspaces.yaml`. Absence = backward compat (single-repo, DEVTRACK_WORKSPACE, priority chain unchanged).

```yaml
version: "1"
workspaces:
  - name: "work-api"
    path: "/Users/sraj/git_apps/work/api"
    pm_platform: "azure"        # azure | gitlab | github | jira | none
    pm_project: ""              # optional override (e.g. GITLAB_PROJECT_ID for this repo)
    enabled: true
    ignore_branches: []
    tags: ["work"]
```

**Why:** One truth for all monitored repos. Each repo has exactly one PM platform. No guessing.
**How to apply:** When starting multi-repo work, create this file. Never put PM credentials here — only routing config.

## Key Design Decisions

- `DEVTRACK_WORKSPACE` stays required when no `workspaces.yaml`. Silently ignored when file exists.
- `Config.Repositories` in `config.yaml` was always empty — retire it, `workspaces.yaml` is the one truth.
- Timer prompts: merged mode (one prompt, most-recently-active workspace context). `sequential` mode is future.
- Priority chain kept for backward compat when `pm_platform` is empty in IPC message.

## IPC Protocol Changes

`CommitTriggerData` and `TimerTriggerData` get new `omitempty` fields:
- `workspace_name` string
- `pm_platform` string
- `pm_project` string

Python reads these from `msg.data` dict — zero value = fall back to priority chain.

## Python Routing

New module: `backend/workspace_router.py`
- `WorkspaceRouter(azure_client, gitlab_client, github_client)`
- `route(pm_platform, description, ticket_id, status, pm_project, commit_info)` → `(work_item_id, platform)`
- If `pm_platform` empty → falls back to priority chain (backward compat)
- Replaces the three-block if/elif chain in `python_bridge.py`

## Build Phases

**Phase 1 (next)** — Core wiring:
- `config.go`: extend `RepositoryConfig` (add `PMPlatform`, `Tags`), add `WorkspacesConfig` struct + `LoadWorkspacesConfig()`
- `config_env.go`: add `GetWorkspacesFilePath()`
- `ipc.go`: add new fields to trigger structs, add `MsgTypeWorkspaceReload`
- `integrated.go`: replace single `GitMonitor` with `[]*WorkspaceMonitor`, add `handleCommitForWorkspace`
- `backend/workspace_router.py`: new file
- `python_bridge.py`: instantiate router, replace PM chain with `router.route()`
- `backend/ipc_client.py`: add `WORKSPACE_RELOAD` message type

**Phase 2** — CLI commands:
- `devtrack workspace list/add/remove/enable/disable`
- Hot-reload via `MsgTypeWorkspaceReload` IPC when YAML changes
- `integrated.go`: handle reload — stop old monitors, re-read YAML, start new ones

**Phase 3** — Timer context:
- Scheduler tracks last-commit-time per workspace (in-memory map)
- Timer trigger carries most-recently-active workspace fields
- TUI prompt shows workspace context

## Skeleton Already Exists

`devtrack-bin/config.go` already has `RepositoryConfig` struct and `Config.Repositories []RepositoryConfig`. `Data/configs/config.yaml` has `repositories: []`. We are wiring up, not starting from scratch.

## Critical Files for Phase 1

- `devtrack-bin/integrated.go` — replace single GitMonitor, add WorkspaceMonitor slice
- `devtrack-bin/config.go` — extend RepositoryConfig, add WorkspacesConfig + loader
- `devtrack-bin/ipc.go` — add workspace fields to trigger structs
- `python_bridge.py` — replace PM chain with router.route()
- `backend/workspace_router.py` — new file (core of Python-side change)
