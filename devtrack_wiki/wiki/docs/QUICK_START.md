# Quick start: one repository, one commit

Follow this after [Installation](INSTALLATION.md). These commands use your normal shell on Linux
or macOS and PowerShell on Windows. Use `devtrack.exe` in PowerShell if that is the filename on
your `PATH`.

## 1. Configure the repository

Open a terminal **inside the Git repository** you want DevTrack to watch:

```sh
devtrack version
devtrack setup
devtrack doctor
```

Setup asks for a deployment mode, repository path, identity, and PM integration. For a first
local evaluation, choose **External** and **None** as the PM platform, and leave the server URL and
API key unset. This exercises Git capture and ticket mapping without starting a Python server,
PostgreSQL, an AI model, or a PM destination. The external server will show as unavailable until
you configure one. Choose
**Managed** if you also want generated reports: it requires a reachable PostgreSQL database and
the Python server may continue installing after setup returns. `devtrack doctor` reports progress.
The wizard registers its environment file; you do not need to source it in every shell.

## 2. Start observation

```sh
devtrack ticket convention
devtrack start
devtrack status
```

`ticket convention` shows the branch format and workspace key. Keep the daemon running while you
work. In the v3.3.0 development line, a commit made while it is stopped is not guaranteed to be
captured after restart.

## 3. Make a normal commit

Use a **real work item** and your workspace's key. For a practice run, use a disposable repository
and `pm_platform: none`. For a workspace with key `PROJ`, the branch below maps to `PROJ-123`:

```sh
git switch -c feature/PROJ-123-first-change
devtrack ticket check feature/PROJ-123-first-change
# Edit a file as you normally would, then:
git add path/to/changed-file
git commit -m "Describe the change"
```

The branch is the strongest mapping signal. A Jira incident, Azure story, GitHub bug, or GitLab
task all use the same mechanism; the connected PM system determines the item's type. See
[Ticket Mapping](../../../docs/TICKET_MAPPING.md) for provider examples and conflicts.

## 4. Inspect the result

```sh
devtrack status
devtrack logs
devtrack queue list
```

Check that the daemon is running and that the commit and its ticket appear in local activity.
The queue can be empty in a local-only setup. If an action exists, review its target, text, and
confidence before using `devtrack queue approve <id>`; approving can contact the configured
destination. Keep any `server:` prefix shown on server-owned action IDs.

If no commit appears, confirm the repository is enabled, the daemon was running when you committed,
and the branch is not in `ignore_branches`. Run `devtrack doctor` and consult
[Troubleshooting](TROUBLESHOOTING.md). To correct an observed commit's mapping, use
`devtrack ticket link <commit-hash> PROJ-123`.

## 5. Add optional features

| Feature | Command or guide | Requirement |
|---|---|---|
| Local assistant context | `devtrack mcp setup`, then `devtrack mcp test` | Go client and local SQLite. Reload your MCP client after setup. |
| Existing EOD report | `devtrack eod show` | An earlier report. This only reads it. |
| New EOD report | `devtrack eod generate`, then `devtrack queue list` | Ready Python service; generation may stage a new action. |
| PM delivery | [Usage Guide](USAGE_GUIDE.md) | Configured connector and credentials; review the pending action first. |
| More repositories | [Configuration](CONFIGURATION.md) | A workspace entry for each repository. |

The [Verification Guide](VERIFICATION.md) covers qualification of an installed build before
unattended use.
