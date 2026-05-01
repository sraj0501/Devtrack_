---
name: No auto git commits
description: Do not make git commits autonomously — user controls all commits and pushes
type: feedback
---

Do not run `git add`, `git commit`, or `git push` unless the user explicitly says "do it" or "commit this" in that specific message.

**Why:** User was interrupted mid-session when Claude made commits unprompted. Commits affect shared history and the user wants full control over what goes into the repo and when.

**How to apply:** Make file edits freely, but stop before staging/committing. Present the commands for the user to run themselves, or wait for explicit "yes do it" / "commit" instruction scoped to that moment. When bypassing the devtrack git wrapper is needed, use `GIT_NO_DEVTRACK=1 git ...`.
