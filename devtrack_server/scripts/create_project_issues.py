#!/usr/bin/env python3
"""
DevTrack Project Issue Creator
Reads GITHUB_TOKEN, GITHUB_OWNER, GITHUB_REPO from .env and creates the
remaining project work items on GitHub, then adds them to the project board.

Usage:
    uv run python scripts/create_project_issues.py [--project-id 1] [--dry-run]
"""

import asyncio
import argparse
import sys
import os

sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))

from backend.config import _load_env
_load_env()

from backend.github.client import GitHubClient

# ──────────────────────────────────────────────
# Project plan: every issue to create
# ──────────────────────────────────────────────
# Format: {title, body, labels, milestone_title}
# Labels that must exist: bug, enhancement, test, documentation
# ──────────────────────────────────────────────

MILESTONES = [
    {"title": "Multi-Repo Phase 2",     "description": "CLI workspace commands and hot-reload"},
    {"title": "Testing & Quality",      "description": "Test pass for all new integrations"},
    {"title": "Platform Stability",     "description": "launchd, commit/push, uninstall improvements"},
    {"title": "Ticket Alerter",         "description": "Background polling and OS notifications"},
    {"title": "Managed Cloud Mode",     "description": "Cloud API + TUI Dashboard"},
    {"title": "Slack Bot",              "description": "Remote control via Slack"},
]

ISSUES = [

    # ── Multi-Repo Phase 2 ──────────────────────────────────────────────

    {
        "title": "[Feature] devtrack workspace list — show all monitored repos and their status",
        "body": """## Summary
Add `devtrack workspace list` CLI command that reads `workspaces.yaml` and prints a table showing each workspace name, path, pm_platform, and whether the daemon is currently monitoring it.

## Acceptance Criteria
- [ ] Works when `workspaces.yaml` is absent (shows single-repo mode notice)
- [ ] Works when `workspaces.yaml` is present (shows all entries with enabled/disabled state)
- [ ] Shows PM platform for each workspace
- [ ] Output is readable in a terminal (table or structured list)

## Related Files
- `devtrack-bin/cli.go` — add `workspace` subcommand
- `devtrack-bin/config.go` — `LoadWorkspacesConfig()`
""",
        "labels": ["enhancement"],
        "milestone": "Multi-Repo Phase 2",
    },
    {
        "title": "[Feature] devtrack workspace add — add a repo to workspaces.yaml from CLI",
        "body": """## Summary
Add `devtrack workspace add <path> --name <name> --platform <platform>` CLI command that appends a new workspace entry to `workspaces.yaml`, creating the file if it does not exist.

## Acceptance Criteria
- [ ] Validates that `<path>` is a valid Git repository
- [ ] Validates `--platform` is one of: azure, gitlab, github, jira, none
- [ ] Creates `workspaces.yaml` at `$PROJECT_ROOT/workspaces.yaml` if absent
- [ ] Appends to existing file without corrupting other entries
- [ ] Outputs confirmation with the entry added
- [ ] Does NOT restart the daemon automatically (user must restart)

## CLI Example
```bash
devtrack workspace add ~/work/api --name work-api --platform azure
devtrack workspace add ~/oss/lib  --name oss-lib  --platform github --project ""
```

## Related Files
- `devtrack-bin/cli.go` — add `workspace add` subcommand
- `devtrack-bin/config.go` — `WorkspacesConfig.Save()`
""",
        "labels": ["enhancement"],
        "milestone": "Multi-Repo Phase 2",
    },
    {
        "title": "[Feature] devtrack workspace remove — remove a repo from workspaces.yaml",
        "body": """## Summary
Add `devtrack workspace remove <name|path>` to delete a workspace entry by name or path.

## Acceptance Criteria
- [ ] Accepts workspace name or repo path as identifier
- [ ] Removes entry and saves the updated YAML
- [ ] Warns if no entry matches
- [ ] Does NOT restart daemon automatically

## Related Files
- `devtrack-bin/cli.go`
- `devtrack-bin/config.go`
""",
        "labels": ["enhancement"],
        "milestone": "Multi-Repo Phase 2",
    },
    {
        "title": "[Feature] devtrack workspace enable/disable — toggle monitoring without removing",
        "body": """## Summary
Add `devtrack workspace enable <name>` and `devtrack workspace disable <name>` to toggle the `enabled` field on a workspace entry.

## Acceptance Criteria
- [ ] Updates `enabled: true/false` in the YAML file
- [ ] Outputs current state after change
- [ ] Hot-reload: if daemon is running, sends `workspace_reload` IPC message

## Related Files
- `devtrack-bin/cli.go`
- `devtrack-bin/ipc.go` — `MsgTypeWorkspaceReload` already defined
- `devtrack-bin/integrated.go` — handler for reload message
""",
        "labels": ["enhancement"],
        "milestone": "Multi-Repo Phase 2",
    },
    {
        "title": "[Feature] Hot-reload workspaces.yaml via MsgTypeWorkspaceReload IPC",
        "body": """## Summary
When `devtrack workspace enable/disable/add/remove` modifies `workspaces.yaml`, the daemon should reload it without a full restart: stop old monitors, re-read YAML, start new monitors.

## Design
`IntegratedMonitor` registers a handler for `MsgTypeWorkspaceReload`:
1. Stop all current `WorkspaceMonitor` instances
2. Call `LoadWorkspacesConfig()` to re-read `workspaces.yaml`
3. Start new `WorkspaceMonitor` instances for enabled workspaces
4. Send ACK back to CLI

## Acceptance Criteria
- [ ] Reload completes without dropping queued IPC messages
- [ ] New workspace starts monitoring immediately
- [ ] Disabled workspace stops receiving events
- [ ] Scheduler continues uninterrupted during reload

## Related Files
- `devtrack-bin/integrated.go` — `registerIPCHandlers()`
- `devtrack-bin/ipc.go` — `MsgTypeWorkspaceReload` (constant already added)
""",
        "labels": ["enhancement"],
        "milestone": "Multi-Repo Phase 2",
    },

    # ── Testing & Quality ───────────────────────────────────────────────

    {
        "title": "[Test] Integration tests for GitLab bidirectional sync",
        "body": """## Summary
Write pytest tests for `backend/gitlab/client.py` and the GitLab sync path in `python_bridge.py`.

## Test Cases
- [ ] `GitLabClient.is_configured()` returns False when env vars missing
- [ ] `get_my_issues()` returns typed `GitLabIssue` objects
- [ ] `add_comment()` posts to correct project+iid
- [ ] `close_issue()` transitions state
- [ ] `create_issue()` returns issue with iid
- [ ] Sync path: match found → comment added
- [ ] Sync path: no match + create_on_no_match → issue created
- [ ] Sync path: no match + create disabled → None returned

## Notes
- Use aioresponses or httpretty to mock the GitLab REST API
- Call `reset_provider_cache()` if LLM_PROVIDER changes in any test
""",
        "labels": ["test"],
        "milestone": "Testing & Quality",
    },
    {
        "title": "[Test] Integration tests for GitHub bidirectional sync",
        "body": """## Summary
Write pytest tests for `backend/github/client.py` and the GitHub sync path in `python_bridge.py`.

## Test Cases
- [ ] `GitHubClient.is_configured()` returns False when env vars missing
- [ ] `get_my_issues()` paginates via Link header and filters out PRs
- [ ] `get_issue()` returns typed `GitHubIssue`
- [ ] `add_comment()`, `close_issue()`, `create_issue()` call correct endpoints
- [ ] `GITHUB_API_URL` override is respected (GHE support)
- [ ] `GITHUB_API_VERSION` header is set on every request
- [ ] Sync path: match found → comment posted
- [ ] Sync path: no match + create → issue created with label
""",
        "labels": ["test"],
        "milestone": "Testing & Quality",
    },
    {
        "title": "[Test] Unit tests for WorkspaceRouter",
        "body": """## Summary
Write unit tests for `backend/workspace_router.py`.

## Test Cases
- [ ] `route('none', ...)` → `(None, None)` immediately
- [ ] `route('azure', ...)` with no client → `(None, None)`
- [ ] `route('gitlab', ...)` calls only gitlab client
- [ ] `route('github', ...)` calls only github client
- [ ] `route('', ...)` runs priority chain (Azure → GitLab → GitHub)
- [ ] Unknown pm_platform falls back to priority chain with warning log
- [ ] `pm_project` as numeric string is parsed correctly for GitLab
- [ ] `pm_project` as non-numeric string is ignored with warning

## Notes
- Mock all three clients
- Test `_route_priority_chain` stops at first match
""",
        "labels": ["test"],
        "milestone": "Testing & Quality",
    },
    {
        "title": "[Test] Unit tests for LoadWorkspacesConfig and WorkspaceConfig",
        "body": """## Summary
Write tests for the Go `LoadWorkspacesConfig()` function and `workspaces.yaml` parsing.

## Test Cases
- [ ] Returns `(nil, nil)` when file is absent (backward compat)
- [ ] Parses all fields correctly from a sample YAML
- [ ] `GetEnabledWorkspaces()` filters disabled entries
- [ ] `~` in paths is expanded to home directory
- [ ] Invalid YAML returns an error (not a panic)
- [ ] Empty `workspaces` list is handled gracefully

## Files
- `devtrack-bin/config_test.go` (new)
""",
        "labels": ["test"],
        "milestone": "Testing & Quality",
    },
    {
        "title": "[Test] Smoke tests for Azure DevOps client",
        "body": """## Summary
Write pytest tests for `backend/azure/client.py` covering the async client methods.

## Test Cases
- [ ] `is_configured()` returns False when required vars absent
- [ ] `get_my_work_items()` returns typed objects
- [ ] `add_comment()`, `update_work_item_state()`, `create_work_item()` call correct endpoints
- [ ] Session is properly closed on `close()`

## Notes
- Mock aiohttp session using aioresponses
""",
        "labels": ["test"],
        "milestone": "Testing & Quality",
    },
    {
        "title": "[Bug] GitLab /gitlab Telegram command requires prior sync run — improve UX",
        "body": """## Problem
The `/gitlab` Telegram command reads from the local cache file `Data/gitlab/sync_state.json`. If the user hasn't run `devtrack gitlab-sync` first, the command silently returns no results or errors without a helpful message.

## Expected Behavior
If the cache file is missing or empty, the bot should:
1. Reply with a clear message: "No cached issues found. Run `devtrack gitlab-sync` or wait for the next scheduled sync."
2. Optionally offer a button or command to trigger a live fetch

## Files
- `backend/telegram/handlers.py` — `_cmd_gitlab()`
""",
        "labels": ["bug"],
        "milestone": "Testing & Quality",
    },
    {
        "title": "[Bug] Timer trigger carries empty workspace context in multi-repo mode",
        "body": """## Problem
In multi-repo mode, timer triggers fire from the scheduler (not from a specific workspace), so they carry empty `workspace_name`, `pm_platform`, and `pm_project` fields. This means timer-triggered work updates always fall back to the priority chain, ignoring per-workspace routing.

## Expected Behavior (Phase 3 — deferred)
Timer trigger should carry the most-recently-active workspace context so PM sync routes correctly.

## Current Workaround
Single-repo users are unaffected. Multi-repo users with timer triggers see priority chain behavior (Azure → GitLab → GitHub) regardless of per-workspace config.

## Design
`IntegratedMonitor` tracks a `lastActiveWorkspace *WorkspaceMonitor` field, updated every time `handleCommitForWorkspace` fires. Scheduler passes this into `TimerTriggerData`.

## Labels
Priority: low (deferred to Phase 3)
""",
        "labels": ["bug"],
        "milestone": "Testing & Quality",
    },

    # ── Platform Stability ──────────────────────────────────────────────

    {
        "title": "[Feature] launchd plist for macOS auto-start on login",
        "body": """## Summary
Create a launchd plist that starts the DevTrack daemon automatically when the user logs in to macOS, replacing the manual `devtrack start` step.

## Implementation
1. Generate plist at `~/Library/LaunchAgents/com.devtrack.daemon.plist`
2. Add `devtrack install-launchd` / `devtrack uninstall-launchd` CLI commands
3. Use `PROJECT_ROOT` and `DEVTRACK_ENV_FILE` from environment

## Plist Requirements
- `RunAtLoad: true`
- `KeepAlive: false` (daemon manages its own restarts via health monitor)
- `StandardOutPath` and `StandardErrorPath` pointing to `LOG_DIR`
- `EnvironmentVariables` with `DEVTRACK_ENV_FILE`

## Acceptance Criteria
- [ ] `devtrack install-launchd` writes plist and runs `launchctl load`
- [ ] `devtrack uninstall-launchd` runs `launchctl unload` and removes plist
- [ ] `devtrack launchd-status` shows whether the agent is loaded and running
- [ ] Daemon starts automatically on next login without manual intervention
""",
        "labels": ["enhancement"],
        "milestone": "Platform Stability",
    },

    # ── Ticket Alerter ──────────────────────────────────────────────────

    {
        "title": "[Feature] Ticket Alerter — poll Jira/Azure/GitHub for new assignments",
        "body": """## Summary
Background polling service that watches Azure DevOps, GitLab, GitHub (and optionally Jira) for ticket events relevant to the developer and delivers OS/terminal notifications.

## Events to Watch
| Source | Events |
|--------|--------|
| Azure DevOps | Work item assigned, comment added, state changed |
| GitLab | Issue assigned, comment added, state changed |
| GitHub | Issue/PR assigned, review requested, comment on my issue |
| Jira | Assigned to me, comment added, status changed, priority changed |

## Architecture
```
backend/alert_poller.py     — async coordinator
backend/alerters/
  azure_alerter.py          — Azure REST API polling
  gitlab_alerter.py         — GitLab REST API polling
  github_alerter.py         — GitHub REST API polling
backend/alert_notifier.py   — OS + terminal notification delivery
```

## Config Vars to Add
```env
ALERT_ENABLED=true
ALERT_POLL_INTERVAL_SECS=300
ALERT_AZURE_ENABLED=true
ALERT_GITLAB_ENABLED=true
ALERT_GITHUB_ENABLED=true
ALERT_NOTIFY_ASSIGNED=true
ALERT_NOTIFY_COMMENTS=true
ALERT_NOTIFY_STATUS_CHANGES=true
```

## Acceptance Criteria
- [ ] Poller runs as a managed child process (same pattern as Telegram bot)
- [ ] OS notifications via `osascript` on macOS
- [ ] Terminal bell + formatted output when devtrack is in foreground
- [ ] State tracked in MongoDB `alert_state` collection (last_checked per source)
- [ ] `devtrack alerts` CLI shows unread notifications

## CLI Commands
```bash
devtrack alerts              # show unread (last 24h)
devtrack alerts --all        # show all
devtrack alerts --clear      # mark all read
devtrack alerts --pause      # pause polling
devtrack alerts --resume     # resume polling
```
""",
        "labels": ["enhancement"],
        "milestone": "Ticket Alerter",
    },
    {
        "title": "[Feature] OS notification delivery for Ticket Alerter (macOS + terminal)",
        "body": """## Summary
`backend/alert_notifier.py` — delivers alerts via macOS OS notifications and terminal output.

## Delivery Methods
1. **macOS**: `osascript -e 'display notification ...'` or `terminal-notifier` (richer, with click actions)
2. **Terminal**: Print formatted alert to stdout when a TTY is attached (bell + colored output)

## Acceptance Criteria
- [ ] macOS notification shows title (ticket ID), subtitle (event type), body (summary)
- [ ] Clicking the notification opens the ticket URL in the browser
- [ ] Graceful fallback to terminal-only when not on macOS or osascript unavailable
- [ ] Rate limiting: max 3 notifications per minute per source
""",
        "labels": ["enhancement"],
        "milestone": "Ticket Alerter",
    },

    # ── Managed Cloud Mode ──────────────────────────────────────────────

    {
        "title": "[Feature] Managed Mode Phase 1 — FastAPI cloud API",
        "body": """## Summary
Cloud-hosted API that replaces the local Python bridge for users who prefer not to run a local daemon. The local Go daemon stays unchanged; only the Python bridge moves to the cloud.

## Architecture
```
Local Go Daemon → WebSocket → Cloud FastAPI → LLM + PM integrations
```

## Phase 1 Scope
- [ ] FastAPI app with WebSocket endpoint replacing TCP IPC
- [ ] API key authentication (issued per user, stored in cloud DB)
- [ ] `/trigger/commit` and `/trigger/timer` endpoints
- [ ] Stateless — no local storage required on cloud side
- [ ] Deployable to Railway / Render / Fly.io

## Config Vars to Add
```env
MANAGED_MODE=false              # enable cloud mode
MANAGED_API_URL=                # cloud API base URL
MANAGED_API_KEY=                # user API key
```

## Files to Create
```
backend/managed/
  api.py           — FastAPI app
  auth.py          — API key validation
  ws_bridge.py     — WebSocket → IPC adapter
  cloud_config.py  — cloud-side config
devtrack-bin/
  managed_client.go — WebSocket client replacing TCP IPC client
```
""",
        "labels": ["enhancement"],
        "milestone": "Managed Cloud Mode",
    },
    {
        "title": "[Feature] Bubble Tea TUI Dashboard for Managed Mode",
        "body": """## Summary
A Bubble Tea (Go TUI) dashboard for managed mode users: login, credential vault, team management, and activity stream — all in the terminal, same binary as the CLI.

## Views
1. **Login** — API key entry with masked input, saved to `~/.devtrack/auth.json`
2. **Dashboard** — Activity stream: recent commits, triggers, PM syncs
3. **Credentials** — View/edit connected platforms (Azure, GitLab, GitHub)
4. **Team** — (future) shared workspace management

## Acceptance Criteria
- [ ] `devtrack dashboard` launches the TUI
- [ ] Works without Managed Mode active (shows local status instead)
- [ ] Keyboard navigation: tab between sections, q/Ctrl+C to quit
- [ ] Connects to local IPC or managed API transparently

## Dependencies
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/lipgloss`
""",
        "labels": ["enhancement"],
        "milestone": "Managed Cloud Mode",
    },

    # ── Slack Bot ───────────────────────────────────────────────────────

    {
        "title": "[Feature] Slack bot — remote control and mobile access",
        "body": """## Summary
A Slack bot providing the same remote-control capabilities as the Telegram bot: live notifications for commits/triggers, and slash commands for PM platform management.

## Commands (matching Telegram parity)
| Command | Description |
|---------|-------------|
| `/devtrack status` | Daemon status and service health |
| `/devtrack trigger` | Force an immediate work update |
| `/devtrack azure` | List Azure work items |
| `/devtrack gitlab` | List GitLab issues |
| `/devtrack github` | List GitHub issues |
| `/devtrack plan <problem>` | Run PM Agent decomposition |

## Architecture
- Slack app via Bolt for Python (`slack-bolt`)
- Runs as a managed child process (same as Telegram bot)
- Connects to Go IPC server to receive live events
- Events (commits, triggers, webhooks) pushed to configured Slack channel

## Config Vars to Add
```env
SLACK_ENABLED=false
SLACK_BOT_TOKEN=                 # xoxb-... token
SLACK_APP_TOKEN=                 # xapp-... for Socket Mode
SLACK_CHANNEL_ID=                # channel to post notifications
SLACK_ALLOWED_USER_IDS=          # comma-separated, leave empty for all workspace members
```

## Acceptance Criteria
- [ ] `devtrack slack-status` shows bot process state
- [ ] Commands work from both DMs and channel mentions
- [ ] Live notifications push to `SLACK_CHANNEL_ID`
- [ ] Health monitor auto-restarts bot on crash
""",
        "labels": ["enhancement"],
        "milestone": "Slack Bot",
    },
]


async def get_or_create_milestone(client, title, description=""):
    """Get existing milestone by title or create it."""
    url = client._api(f"/repos/{client._owner}/{client._repo}/milestones")
    milestones = await client._get(url, params={"state": "all", "per_page": 100})
    if milestones:
        for m in milestones:
            if m["title"] == title:
                return m["number"]

    result = await client._post(url, json_body={
        "title": title,
        "description": description,
        "state": "open",
    })
    if result:
        print(f"  ✓ Created milestone: {title}")
        return result["number"]
    return None


async def ensure_labels(client, needed_labels):
    """Create labels that don't exist yet."""
    label_colors = {
        "enhancement": "a2eeef",
        "bug":         "d73a4a",
        "test":        "0e8a16",
        "documentation": "0075ca",
    }
    url = client._api(f"/repos/{client._owner}/{client._repo}/labels")
    existing = await client._get(url, params={"per_page": 100})
    existing_names = {l["name"] for l in (existing or [])}

    for label in needed_labels:
        if label not in existing_names:
            await client._post(url, json_body={
                "name": label,
                "color": label_colors.get(label, "ededed"),
            })
            print(f"  ✓ Created label: {label}")


async def add_to_project(client, issue_node_id, project_id, owner):
    """Add an issue to a GitHub Project (v2) using GraphQL."""
    graphql_url = "https://api.github.com/graphql"

    # First get the project node ID
    query = """
    query($login: String!, $number: Int!) {
      user(login: $login) {
        projectV2(number: $number) { id }
      }
    }
    """
    resp = await client._post(graphql_url, json_body={
        "query": query,
        "variables": {"login": owner, "number": project_id}
    })
    if not resp:
        return False

    project_node_id = None
    try:
        project_node_id = resp["data"]["user"]["projectV2"]["id"]
    except (KeyError, TypeError):
        return False

    mutation = """
    mutation($projectId: ID!, $contentId: ID!) {
      addProjectV2ItemById(input: {projectId: $projectId, contentId: $contentId}) {
        item { id }
      }
    }
    """
    result = await client._post(graphql_url, json_body={
        "query": mutation,
        "variables": {"projectId": project_node_id, "contentId": issue_node_id}
    })
    return result is not None


async def main(dry_run: bool, project_id: int):
    client = GitHubClient()
    if not client.is_configured():
        print("ERROR: GitHub credentials not configured.")
        print("Set GITHUB_TOKEN, GITHUB_OWNER, GITHUB_REPO in .env")
        sys.exit(1)

    user = await client.get_current_user()
    print(f"Authenticated as: {user}")
    print(f"Target repo: {client._owner}/{client._repo}")
    print(f"Project board: #{project_id}")
    print(f"Dry run: {dry_run}")
    print()

    if dry_run:
        print(f"Would create {len(MILESTONES)} milestones and {len(ISSUES)} issues:")
        for issue in ISSUES:
            print(f"  [{', '.join(issue['labels'])}] {issue['title'][:80]}")
        await client.close()
        return

    # Ensure labels exist
    print("Ensuring labels...")
    await ensure_labels(client, ["enhancement", "bug", "test", "documentation"])

    # Create milestones
    print("\nCreating milestones...")
    milestone_map = {}
    for m in MILESTONES:
        number = await get_or_create_milestone(client, m["title"], m["description"])
        if number:
            milestone_map[m["title"]] = number

    # Create issues
    print(f"\nCreating {len(ISSUES)} issues...")
    created = 0
    for issue in ISSUES:
        milestone_number = milestone_map.get(issue.get("milestone"))
        payload = {
            "title": issue["title"],
            "body": issue["body"],
            "labels": issue["labels"],
        }
        if milestone_number:
            payload["milestone"] = milestone_number

        issues_url = client._api(f"/repos/{client._owner}/{client._repo}/issues")
        result = await client._post(issues_url, json_body=payload)
        if result:
            issue_number = result["number"]
            issue_node_id = result.get("node_id", "")
            print(f"  ✓ #{issue_number}: {issue['title'][:70]}")

            if project_id and issue_node_id:
                added = await add_to_project(client, issue_node_id, project_id, client._owner)
                if added:
                    print(f"       → Added to project #{project_id}")

            created += 1
        else:
            print(f"  ✗ Failed: {issue['title'][:70]}")

    await client.close()
    print(f"\nDone: {created}/{len(ISSUES)} issues created.")
    if project_id:
        print(f"View board: https://github.com/users/{client._owner}/projects/{project_id}/views/1")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Create DevTrack project issues on GitHub")
    parser.add_argument("--project-id", type=int, default=1, help="GitHub Projects v2 board number (default: 1)")
    parser.add_argument("--dry-run", action="store_true", help="Print what would be created without creating")
    args = parser.parse_args()

    asyncio.run(main(args.dry_run, args.project_id))
