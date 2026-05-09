# DevTrack Git Features Guide

Complete guide to DevTrack's git-powered workflows: enhanced commits, conflict resolution, and work update parsing.

---

## Overview

DevTrack adds five powerful layers to your Git workflow:

0. **Shell Integration** - Type `git commit` as normal; DevTrack intercepts it transparently
1. **Enhanced Commits** - AI-powered commit messages with context awareness
2. **Conflict Resolution** - Automatic detection and smart resolution of merge conflicts
3. **Work Update Parsing** - Natural language work updates with PR/issue auto-detection
4. **Interactive PM Sync** - After every commit, an interactive ticket picker lets you link the commit to the right issue and log time spent — no context-switching to a browser

---

## Shell Integration — Type `git commit` Natively

The biggest friction with DevTrack's git features is remembering to type `devtrack git commit` instead of `git commit`. Shell integration eliminates that entirely.

### One-Time Setup

Add the `devtrack` binary to your `PATH` and then add one line to `~/.zshrc` (or `~/.bashrc`):

```bash
# 1. Add the binary to PATH (if not already on it)
export PATH="$HOME/.local/bin:$PATH"

# 2. Install the git() interceptor
eval "$(devtrack shell-init)"
```

Reload your shell:

```bash
source ~/.zshrc    # or ~/.bashrc
```

That `eval` installs a **`git()`** shell function that intercepts `git commit`, `git history`, and `git messages` for DevTrack workspaces. Everything else (`git push`, `git pull`, `git status`, …) is passed straight through to the real git binary — completely unaffected.

> **Note on the devtrack() wrapper:** Earlier versions of `shell-init` also installed a `devtrack()` wrapper function. This wrapper has been removed to avoid a zsh alias conflict. The binary is now called directly by name; make sure it is on your `PATH` as shown above.

### Opt Repos In

**Option A — Per-repo git config (fastest)**

```bash
cd /path/to/your/repo
devtrack enable-git
```

This sets `devtrack.enabled = true` in the repo's `.git/config`. From now on `git commit` in that repo goes through DevTrack.

To undo: `devtrack disable-git`

> **Auto enable-git on start:** When you run `devtrack start`, DevTrack automatically sets `devtrack.enabled = true` in the git config of all workspaces that are enabled in `workspaces.yaml`. You do not need to run `devtrack enable-git` manually for repos that are already listed there.

**Option B — workspaces.yaml (automatic)**

Any repo already listed in `workspaces.yaml` is intercepted automatically — no `devtrack enable-git` needed.

### How Detection Works

The shell `git()` function runs two checks before deciding to intercept:

1. **Fast path** — reads `.git/config` locally (no subprocess): was `devtrack enable-git` run here?
2. **Slow path** — runs `devtrack is-workspace`, which checks `workspaces.yaml`

If either check passes and the command is `commit`, `history`, or `messages`, the call routes to `devtrack git`. Everything else (`git push`, `git pull`, `git status`, `git log`, …) goes straight to the real git binary — completely unaffected.

### Bypass DevTrack for One Command

Set `GIT_NO_DEVTRACK=1` to skip the interceptor for a single command:

```bash
GIT_NO_DEVTRACK=1 git commit -m "skip devtrack this time"
```

Or call the real git binary directly:

```bash
command git commit -m "message"
```

To disable interception for an entire repo permanently:

```bash
git config devtrack.enabled false
# or
devtrack disable-git
```

### After Setup

```bash
# These all work identically now:
git commit -m "fix auth redirect"       # intercepted → AI enhanced
git push                                 # not intercepted → real git
git status                               # not intercepted → real git

# Same result, always works without shell setup:
devtrack git commit -m "fix auth redirect"
```

### `git add` with No Arguments

When shell integration is active, running `git add` with no path arguments automatically stages all changes (`git add .`):

```bash
git add
# No path specified — staging all changes (git add .)
```

Paths work normally when provided:

```bash
git add src/auth.js       # stages only that file, as usual
git add src/ tests/       # multiple paths, unchanged behaviour
```

---

## Feature 1: Enhanced Commit Messages

Write quick commit messages - DevTrack enhances them with AI.

### How It Works

```
Your commit message + git context
        │
        ▼
    AI Enhancement
    ├─ Adds branch information
    ├─ Includes PR/issue links
    ├─ Improves clarity
    └─ Adds technical context
        │
        ▼
    Enhanced message
    (you review and accept)
```

### Usage

#### Interactive Mode (Default)

```bash
# Stage your changes
git add .

# With shell integration active (recommended):
git commit -m "fixed auth bug"

# Or always works without shell setup:
devtrack git commit -m "fixed auth bug"
```

**Output**:
```
Analyzing staged changes (5 files changed)...

Original message:
  fixed auth bug

Enhanced message (Attempt 1/5):
  Fixed OAuth authentication bug in login flow
  - Resolved JWT token expiration issue
  - Updated refresh token logic
  - Added error handling for edge cases

Choose an action:
  1) Accept     - Use enhanced message
  2) Enhance    - Ask AI to improve it more
  3) Regenerate - Start over with AI
  4) Cancel     - Don't commit
  5) Original   - Use original message

Your choice [1-5]: 1

Committed with enhanced message.
Log this work? (y/n): y
```

> **Tip — pressing `2` (Enhance):** When you choose Enhance to ask the AI to improve the message further, the model is given double the normal token budget (`COMMIT_LLM_MAX_TOKENS × 2`) for that single call, allowing a richer and more detailed result. The budget resets to the normal limit for any subsequent iteration unless you press Enhance again.

### Log This Work — Interactive PM Sync

When you answer `y` to "Log this work?", DevTrack first asks how long the work took, then opens a full-screen split-pane ticket picker so you can link the commit to the right issue without leaving the terminal.

#### Time Prompt

```
How long did this take? (e.g. 2h, 30m) [Enter to skip]: 1h
→ Syncing to project management...
```

#### Interactive Ticket Picker

The picker fetches your open issues once and displays them in a split-pane curses UI — no network round-trips while you browse:

```
╔══════════════════════════════════════════════════════════════════════════════════╗
║  Select a ticket to link  (filter: /)   select: Enter   new: n   skip: Esc/q   ║
╠════════════════════════════╦═══════════════════════════════════════════════════ ╣
║  #12  Refactor DB layer    ║  #42  Fix OAuth authentication bug                 ║
║  #35  Add rate limiting    ║  ─────────────────────────────────────────────     ║
║  #38  Update CI pipeline   ║  Reported by: alice · opened 3 days ago            ║
║  #41  Improve error msgs   ║                                                     ║
║ ▶ #42  Fix OAuth auth bug  ║  Users are occasionally seeing a 401 on the         ║
║  #47  Docs: API reference  ║  /refresh endpoint after a token rotation.          ║
║  #53  Bump dependencies    ║  Root cause appears to be a race condition in        ║
║                            ║  the JWT validation middleware when two requests     ║
║                            ║  arrive within the same millisecond window.         ║
║                            ║                                                     ║
║                            ║  Steps to reproduce:                                ║
║                            ║    1. Issue two simultaneous refresh calls          ║
║                            ║    2. Observe 401 on the second response            ║
║                            ║                                                     ║
║                            ║  Labels: bug · auth · priority:high                 ║
╠════════════════════════════╩════════════════════════════════════════════════════╣
║  ↑/↓ or j/k  navigate    PgUp/PgDn  page    Home/End  jump    /  filter        ║
╚══════════════════════════════════════════════════════════════════════════════════╝
```

**Controls**:

| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Move highlight up / down |
| `PgUp` / `PgDn` | Scroll one page |
| `Home` / `End` | Jump to first / last ticket |
| `/` | Enter filter mode — type to narrow the list in real time (client-side, no network) |
| `Enter` | Select highlighted ticket and link the commit |
| `n` | Create a new issue from the commit message |
| `Esc` or `q` | Skip without linking |

The right pane updates instantly as you move the highlight, showing the full issue body with automatic word-wrap. If the terminal is not a TTY (e.g. piped output or a dumb terminal), the picker falls back to a simple numbered list.

#### Result

```
✓ Commented on GitHub issue #42: Fix OAuth authentication bug
  → 1h logged · commit a3f9c12 linked
```

#### Auto-Push Prompt

After the log-work flow completes, DevTrack asks whether to push the commit immediately:

```
🚀 Push to origin/bot_automation? (y/n)
y
→ Pushing...
✓ Pushed to origin/bot_automation
```

Pressing `n` prints a reminder instead of pushing:

```
Skipped. Run 'git push origin bot_automation' when ready.
```

#### Dry-Run Mode (Preview Only)

```bash
# See enhancement without committing
devtrack git commit -m "fixed auth bug" --dry-run
```

**Output**:
```
Analyzing staged changes...

Original: fixed auth bug

Enhanced:
  Fixed OAuth authentication bug in login flow
  - Resolved JWT token expiration issue
  - Updated refresh token logic
  - Added error handling for edge cases

(Preview only - no commit created)
```

### Git Context Included

The AI enhancement has access to:

- **Branch name**: Suggests related issues/PRs
- **Recent commits**: Understands ongoing work
- **PR information**: Links to open pull requests
- **Diff statistics**: Shows scale of changes
- **File names**: Understands which components changed

### Configuration

In `.env`:

```bash
# AI provider for commit enhancement
LLM_PROVIDER=ollama              # or openai, anthropic
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=mistral

# Commit enhancement settings
COMMIT_MAX_ATTEMPTS=5            # Max refinement attempts
COMMIT_CONTEXT_ENABLED=true      # Include git context
COMMIT_LLM_MAX_TOKENS=512        # Normal token budget per LLM call
                                 # Pressing E (Enhance) doubles this to 1024
                                 # for that call only; resets to normal afterwards
```

### Examples

#### Example 1: Bug Fix

```bash
$ devtrack git commit -m "fixed login"

Original: fixed login
Enhanced: Fixed login page form validation
          - Corrected email regex pattern
          - Added password strength validation
          - Improved error messages for invalid input
```

#### Example 2: Feature Addition

```bash
$ devtrack git commit -m "added dark mode"

Original: added dark mode
Enhanced: Add dark mode support with system preference detection
          - Created theme toggle component
          - Added CSS variables for theming
          - Integrated with localStorage for persistence
          - Respects system dark mode setting (prefers-color-scheme)
```

#### Example 3: Refactoring

```bash
$ devtrack git commit -m "refactor payment logic"

Original: refactor payment logic
Enhanced: Refactor payment processing module for testability
          - Extracted PaymentProcessor class
          - Separated validation from transaction logic
          - Added comprehensive unit tests
          - Improved error handling and logging
```

---

## Feature 2: Conflict Resolution

Automatically detect and resolve merge conflicts without manual intervention.

### How It Works

When you run a merge or rebase that has conflicts, DevTrack can:

1. **Detect** conflicts automatically
2. **Analyze** the conflict structure
3. **Suggest** resolution strategies
4. **Resolve** automatically (when safe)
5. **Report** resolution status

### Usage

#### Automatic Detection

DevTrack monitors for conflicts and alerts you:

```bash
# Attempt a merge with conflicts
git merge feature/other-branch

# Conflicts detected!
# DevTrack automatically analyzes them

# Check status
git status
# Shows conflict files

# DevTrack attempts resolution
devtrack resolve-conflicts --auto

# Options:
#   --auto    : Resolve safely (skip ambiguous)
#   --smart   : Use AI-guided resolution
#   --manual  : Show options and ask user
```

#### Manual Resolution Workflow

```bash
# See what conflicts exist
devtrack conflicts list

# Output:
# File: src/auth.js (2 conflicts)
# File: src/config.json (1 conflict)
# Total: 3 conflicts

# Get detailed analysis
devtrack conflicts analyze src/auth.js

# Output:
# Conflict 1 (lines 45-67):
#   Our version: Uses JWT tokens
#   Their version: Uses session cookies
#   Status: Needs manual decision (incompatible approaches)
#
# Conflict 2 (lines 120-145):
#   Our version: New error handling
#   Their version: Old error handling
#   Status: Auto-resolvable (can take both)

# Auto-resolve what's safe
devtrack conflicts resolve --auto

# Manually resolve the rest
nano src/auth.js

# After manual resolution
git add src/auth.js
git merge --continue
```

### Resolution Strategies

DevTrack uses multiple strategies:

| Strategy | When Used | Success Rate |
|----------|-----------|---|
| **Both** | No logic conflict | ~90% |
| **Ours** | We're more recent | ~70% |
| **Theirs** | They're more recent | ~70% |
| **Diff3** | Complex (3-way diff) | ~60% |
| **Smart** | AI-guided (requires Ollama) | ~85% |

### Configuration

In `.env`:

```bash
# Conflict resolution
CONFLICT_RESOLUTION_ENABLED=true
CONFLICT_RESOLUTION_AUTO=true    # Resolve without asking
CONFLICT_RESOLUTION_STRATEGY=smart  # or: both, ours, theirs
CONFLICT_RESOLUTION_AI_ENABLED=true  # Use AI for complex cases
```

### Examples

#### Example 1: Merge with Simple Conflict

```bash
$ git merge feature/new-ui

# CONFLICT (content): Merge conflict in src/styles.css
Auto-merging src/styles.css
CONFLICT (add/add): Merge conflict in src/package.json
Auto-merging src/package.json
CONFLICT (content): Merge conflict in src/index.js

# DevTrack detection
$ devtrack resolve-conflicts --auto

# Analysis:
# src/styles.css: Both added new styles (can merge)
# src/package.json: Both added dependencies (can merge, but check versions)
# src/index.js: Code changed incompatibly (manual intervention needed)

# Resolution:
# src/styles.css: RESOLVED
# src/package.json: RESOLVED (with warning about version conflicts)
# src/index.js: UNRESOLVABLE (needs you to decide)

# You manually fix the incompatible change
# Then:
$ git add src/index.js
$ git merge --continue
```

#### Example 2: Merge with AI-Guided Resolution

```bash
$ git rebase main

# CONFLICT (content): Merge conflict in src/auth.js

$ devtrack resolve-conflicts --smart

# Analysis from AI:
# "Conflict: Authentication strategy changed
#  - Our branch: Added OAuth2 support
#  - Main branch: Updated JWT token handling
#  - Resolution: Both changes are compatible; combine them
#  - Recommended merge: Keep both implementations"

# Resolution:
# src/auth.js: RESOLVED (with combined logic)

$ git rebase --continue
```

---

## Feature 3: Work Update Parsing

Natural language work updates that extract tasks, time, and status automatically.

### How It Works

> **Requires the `ai` tier.** spaCy NLP parsing is part of the optional `ai` dependency tier. If the tier is absent, the server falls back to basic string matching, which may miss task IDs and time durations. Install with `devtrack-server enable ai` then restart.

```
You type natural language
"Working on PR #42 - fixing OAuth (2h)"
        │
        ▼
NLP Parsing (spaCy) — requires ai tier
├─ Extract task: PR #42
├─ Extract action: working on
├─ Extract duration: 2 hours
└─ Extract description: fixing OAuth
        │
        ▼
Work Context Enhancement (Git)
├─ Current branch: feature/oauth-fix
├─ Related PR: Detects PR #42
├─ Recent commits: Shows context
└─ Related tasks: Finds linked issues
        │
        ▼
AI Enhancement
├─ Improve description clarity
├─ Categorize work type
└─ Verify task linkage
        │
        ▼
Create structured task update
{
  "task_id": "PR-42",
  "status": "in_progress",
  "description": "Fixed OAuth authentication flow",
  "time_spent": 2,
  "category": "feature"
}
        │
        ▼
Update project management system
(Azure DevOps, GitHub, Jira)
```

### Usage

#### Automatic Prompts

DevTrack prompts you at configured intervals (default: every 2 hours):

```bash
# DevTrack timer trigger shows:
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# What are you working on?
# (Type natural language, e.g., "Fixed login bug, took 1h")
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# You type:
# Fixed OAuth login flow in PR #42 (1h)

# DevTrack processes it:
# ✓ Task detected: PR #42
# ✓ Action: in progress → completed
# ✓ Time tracked: 1 hour
# ✓ Category: bug fix (OAuth authentication)
# ✓ Updated: Azure DevOps work item
# ✓ Posted: Teams notification

# Logged: "Fixed OAuth login flow in PR #42"
```

#### Manual Trigger

```bash
# Force a work update prompt now
devtrack force-trigger

# Or via Python directly
uv run python -c "from backend.user_prompt import show_work_update_prompt; show_work_update_prompt()"
```

### What It Extracts

DevTrack extracts:

| Item | How | Example |
|------|-----|---------|
| **Task ID** | Keywords: PR, JIRA, issue | "PR #42", "JIRA-123", "issue #99" |
| **Action** | Verbs: working, fixed, completed | "working on", "fixed", "completed" |
| **Duration** | Numbers + time unit | "2h", "1 hour", "30 minutes" |
| **Description** | Main text | "Fixed auth bug" |
| **Status** | Context from action | "in_progress", "done", "blocked" |
| **Category** | Type of work | "feature", "bug", "doc", "refactor" |

### NLP Examples

#### Example 1: Basic Update

```
Input: "Working on user dashboard (2h)"

Extracted:
  Task: None (use current branch)
  Status: in_progress
  Duration: 2 hours
  Description: user dashboard
  Category: feature
```

#### Example 2: Bug Fix with PR

```
Input: "Fixed authentication bug in PR #42, took 1.5 hours"

Extracted:
  Task: PR #42 (GitHub)
  Task: Could also be: GH-42, #42
  Status: completed (inferred from "fixed")
  Duration: 1.5 hours
  Description: Fixed authentication bug
  Category: bugfix
```

#### Example 3: Complex Update

```
Input: "Worked on PROJ-456 and related PROJ-789 - auth refactoring
        Also helped with design review for PROJ-111
        Total: 3h"

Extracted:
  Tasks: PROJ-456, PROJ-789, PROJ-111
  Status: in_progress
  Duration: 3 hours
  Description: Auth refactoring + design review
  Category: refactor
```

### Git Context Enhancement

Work updates are enriched with git information:

```
You type: "Fixed auth bug (2h)"

Git Context Added:
├─ Current branch: feature/oauth-fix
├─ PR detected: PR #42 (via branch name or git history)
├─ Related issues: Linked to issue #123
├─ Recent commits: 3 commits on this branch
└─ Code changes: Auth.js modified (145 lines changed)

Enhanced task update:
  Task: PR #42
  Branch: feature/oauth-fix
  Description: Fixed authentication bug
           (changes in OAuth module, 145 lines)
  Related: Issue #123
```

### Configuration

In `.env`:

```bash
# Work update parsing
WORK_UPDATE_ENABLED=true
WORK_UPDATE_TIMER_INTERVAL=120        # Minutes between prompts
WORK_UPDATE_NLP_ENABLED=true          # Use spaCy parsing
WORK_UPDATE_CONTEXT_ENABLED=true      # Add git context
WORK_UPDATE_AI_ENHANCEMENT=true       # Improve descriptions

# Project integration
AZURE_DEVOPS_ENABLED=true             # Update Azure DevOps
GITHUB_ENABLED=true                   # Update GitHub
JIRA_ENABLED=true                     # Update Jira
```

---

## Combining All Features

The three features work together:

### Typical Workflow

```bash
# 1. Start work on a feature
git checkout -b feature/oauth-upgrade

# 2. Make changes and commit regularly
git add src/auth.js
git commit -m "updated auth module"    # intercepted if shell integration active
# → AI enhances with: "Updated OAuth authentication module with v2.0 support"
# → Extracts: Task = PR #42
# → Updates project management

# 3. Regular work updates
devtrack force-trigger
# → Prompt: "What are you working on?"
# → You: "Implementing OAuth v2.0 in PR #42 (3h)"
# → Extracted: Task PR-42, 3h work, category: feature
# → Updates project management

# 4. Merge with potential conflicts
git merge main
# → Conflicts detected
devtrack resolve-conflicts --smart
# → AI resolves: "Both branches have compatible changes; merging..."
# → Automatic or semi-automatic resolution

# 5. Push and see summary
git push origin feature/oauth-upgrade
# → DevTrack reports: "Completed PR #42: OAuth v2.0 upgrade (8h total)"
# → Daily report includes: Feature work, PR merged, time tracked
```

---

## Advanced: Custom Git Hooks

DevTrack can integrate with git hooks for even deeper integration:

```bash
# Edit .git/hooks/post-merge
#!/bin/bash
devtrack resolve-conflicts --auto

# Edit .git/hooks/post-commit
#!/bin/bash
devtrack analyze-commit HEAD
```

---

## Troubleshooting

### Commit Enhancement Not Working

```bash
# Check if Ollama is running
curl http://localhost:11434/api/tags

# Check logs
tail Data/logs/python_bridge.log | grep "enhancement"

# Verify LLM provider
grep LLM_PROVIDER .env

# Test manually
uv run python -c "from backend.commit_message_enhancer import enhance_message; print(enhance_message('test'))"
```

### Conflicts Not Resolving

```bash
# Check if conflict resolver is enabled
grep CONFLICT_RESOLUTION .env

# Test conflict detection
git status | grep "both added\|both modified"

# Try manual resolution mode
devtrack resolve-conflicts --manual
```

### Work Updates Not Parsing

```bash
# Check which server tiers are active
devtrack-server features
# If ai shows ✗, install it: devtrack-server enable ai && devtrack restart

# Check NLP model (only needed when ai tier is installed)
uv run python -c "import spacy; nlp = spacy.load('en_core_web_sm'); print('OK')"

# Test NLP parsing
uv run python -c "from backend.nlp_parser import parse_update; print(parse_update('Working on PR #42 (2h)'))"

# Check git context
uv run python -c "from backend.work_update_enhancer import enhance_update; print(enhance_update('Working on PR #42 (2h)', '.'))"
```

---

## Next Steps

- **Full Git Workflow**: See [Git Commit Workflow](../GIT_COMMIT_WORKFLOW.md)
- **Detailed Implementation**: See [Phase 1-2 Integration](../GIT_SAGE_INTEGRATION_PHASE_1_2.md)
- **Phase 3 Details**: See [Phase 3 Implementation](ARCHITECTURE.md)
- **Troubleshooting**: See [Troubleshooting Guide](TROUBLESHOOTING.md)
