# 🌿 git-sage

A **local LLM-powered git agent** that lives in your terminal. It doesn't just suggest commands — it *plans*, *executes*, *verifies*, and **recovers from failure** autonomously.

---

## Two modes

| Mode | Command | What it does |
|------|---------|-------------|
| **ask** | `git-sage ask "..."` | Explains + suggests commands, you decide to run |
| **do** | `git-sage do "..."` | Agent plans, executes step-by-step, detects failures, resolves conflicts, rolls back if needed |

---

## Installation

**Requirements:** Python 3.10+ and a running local LLM.

```bash
git clone https://github.com/you/git-sage
cd git-sage
pip install -e .
```

### Recommended: Ollama

```bash
ollama pull llama3   # or mistral, codellama, deepseek-coder
ollama serve
git-sage --config
```

---

## Usage

### Agent mode — `do`

The LLM runs in an **agentic loop**: it decides actions, sees results, and adapts.

```bash
git-sage do "merge feature-auth into main and resolve any conflicts"
git-sage do "squash my last 5 commits into one with a clean message"
git-sage do "my last commit broke things, revert to the previous state"
git-sage do "rebase feature-x onto main, keep my changes if there are conflicts"
git-sage do "delete all local branches that have been merged into main"
```

**What the agent can do:**

- **Run** git/shell commands and read their output
- **Read files** to inspect conflict markers
- **Write files** to resolve conflicts intelligently
- **Checkpoint** before any destructive operation (saves HEAD + stashes dirty state)
- **Rollback** to a checkpoint if something goes wrong
- **Ask you** when intent is genuinely ambiguous
- **Abort cleanly** when a conflict needs human judgment

**Example session:**
```
🌿 git-sage agent
  Task: merge feature-auth into main and resolve any conflicts
  ─────────────────────────────────────────────────────

  📌 CHECKPOINT [before-merge]
     Save state before potentially conflicting merge

  ⚡ RUN  git checkout main
  ⚡ RUN  git merge feature-auth --no-ff
     Conflict in src/auth/middleware.py

  📄 READ_FILE  src/auth/middleware.py
     Reading conflict markers

  ✏️  WRITE_FILE  src/auth/middleware.py
     Resolved: kept incoming auth token validation + existing error handling

  ⚡ RUN  git add src/auth/middleware.py
  ⚡ RUN  git commit -m "Merge feature-auth: add token validation"

  ✅ Done: Merged feature-auth. Resolved 1 conflict by combining both sides.
```

### Ask mode — `ask`

```bash
git-sage ask "how do I undo my last commit but keep the changes"
git-sage ask "what's the difference between merge and rebase"
```

### Interactive shell

```
sage> do squash my last 3 commits
sage> ask how does cherry-pick work
sage> context
sage> quit
```

---

## How the agent recovers

Before every destructive op, the agent:
1. **Checkpoints** HEAD + stashes dirty state
2. Executes the operation, reads output
3. On failure: adapts (aborts mid-merge/rebase, retries differently)
4. If unrecoverable: **rolls back** to the checkpoint automatically

**Conflict resolution:** The agent reads the full file with markers, understands both sides, and picks the right resolution. It only asks you when the intent is genuinely unclear.

---

## Configuration

```bash
git-sage --config
```

Stored at `~/.config/git-sage/config.json`. Supports: `ollama`, `openai`, `lmstudio`, `custom`.

### Model recommendations

| Task | Minimum | Better |
|------|---------|--------|
| ask/suggest | 3B | 8B |
| Conflict resolution | 8B (mistral, llama3) | 13B+ |
| Complex rebases | 13B (codellama:13b) | 34B / GPT-4o |

---

## Optional: git alias

```ini
# ~/.gitconfig
[alias]
    sage = "!git-sage"

# git sage do "merge feature-x"
```

---

## Advanced Features (New!)

### ✨ Intelligent Conflict Resolution
- **Analyzes** conflict markers to understand both sides
- **Auto-resolves** when safe (addition-only, adjacent changes, identical)
- **Merges intelligently** for non-overlapping changes
- **Reports** unresolvable conflicts with context

### 📊 PR/MR Utilities
- Extract metadata from branch names
- Suggest PR titles and descriptions
- Analyze diff statistics (files, additions, deletions)
- Find related commits and issues

### 🔧 Advanced Git Operations
- **Branch**: list, delete, check tracking, estimate merge complexity
- **Commits**: log, blame, find by message pattern
- **Status**: clean check, file status by type
- **Merge**: estimate complexity, suggest merge base
- **Stash**: list and manage stashed changes

## Architecture

```
git_sage/
├── cli.py              # ask + do + interactive modes
├── agent.py            # agentic loop: plan → execute → verify → recover
├── llm.py              # Ollama + OpenAI-compatible backends
├── context.py          # repo state collector
├── config.py           # config wizard + persistence
├── git_operations.py   # advanced git operations (NEW!)
├── conflict_resolver.py # intelligent conflict resolution (NEW!)
└── pr_finder.py        # PR/MR utilities (NEW!)
```

Zero external dependencies. All context stays local unless you configure OpenAI.

## Documentation

- **[INTEGRATION.md](INTEGRATION.md)** - Detailed API reference and DevTrack integration examples
- [Configuration Guide](#configuration) - Setup and model recommendations
- [Examples](#usage) - Common use cases and workflows
