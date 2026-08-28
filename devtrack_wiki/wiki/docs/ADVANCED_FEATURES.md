# Advanced features

DevTrack's advanced capabilities are daemon or server capabilities exposed through the Go CLI.
They are not separate interactive product flows.

## Reports and voice

- `devtrack eod generate` stages an end-of-day report with explicit confidence.
- `devtrack work report` previews the current work session on demand.
- `devtrack voice seed|profile|add|sync|status` manages local voice evidence.
- `devtrack learning-status`, `learning-sync`, and `learning-reset` manage personalization.

The Python server generates reports and profiles and persists server-owned data in PostgreSQL. The
Go client retains local activity and the pending queue in SQLite.

## Agent context

`devtrack mcp setup` configures the JSON-RPC 2.0 stdio server. Its six read-only tools expose active
work, recent activity, pending actions, ticket context, work history, and the EOD roll-up from local
SQLite without requiring Python or a network request.

## Review automation

`devtrack review` and `devtrack review status` expose the PR review loop. Fixes, commits, pushes, and
external updates still pass through the pending-actions trust boundary and escalate when human
judgment is required.

## Offline behavior

Git observation, local SQLite, scheduling, the queue, MCP, and backlog replay remain usable when the
Python server or LLM is unavailable. `devtrack status` and `devtrack doctor` report the resulting
capability degradation.
