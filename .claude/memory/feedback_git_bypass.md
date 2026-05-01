---
name: Always bypass devtrack for git commands
description: Use GIT_NO_DEVTRACK=1 prefix on all git commands to avoid devtrack wrapper interception
type: feedback
---

Always prefix git commands with `GIT_NO_DEVTRACK=1` when running git add, commit, push, tag, etc.

**Why:** The devtrack git wrapper intercepts git commands and can block or error when Claude runs them (e.g. "Use 'devtrack git add' to stage changes first"). This causes commits to silently fail or hang.

**How to apply:** Every git command in Bash tool calls must use:
```bash
GIT_NO_DEVTRACK=1 git add ...
GIT_NO_DEVTRACK=1 git commit ...
GIT_NO_DEVTRACK=1 git push ...
GIT_NO_DEVTRACK=1 git tag ...
```
No exceptions — always bypass even when the daemon is not running, to be safe.
