# DevTrack Git Features Guide

Complete guide to DevTrack's git-powered workflows: enhanced commits, conflict resolution, and work update parsing.

---

## Overview

DevTrack adds three powerful layers to your Git workflow:

1. **Enhanced Commits** - AI-powered commit messages with context awareness
2. **Conflict Resolution** - Automatic detection and smart resolution of merge conflicts
3. **Work Update Parsing** - Natural language work updates with PR/issue auto-detection

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

# Use devtrack git commit
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

```
You type natural language
"Working on PR #42 - fixing OAuth (2h)"
        │
        ▼
NLP Parsing (spaCy)
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
devtrack git commit -m "updated auth module"
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
# Check NLP model
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
- **Phase 3 Details**: See [Phase 3 Implementation](../PHASE_3_IMPLEMENTATION.md)
- **Troubleshooting**: See [Troubleshooting Guide](TROUBLESHOOTING.md)
