# git-sage Integration Guide

This document explains how git-sage integrates with DevTrack and other tools.

## Overview

git-sage is a local LLM-powered git agent with the following capabilities:

- **Autonomous git operations**: merge, rebase, cherry-pick, squash, etc.
- **Intelligent conflict resolution**: analyzes and resolves conflicts automatically
- **PR/MR utilities**: finds, analyzes, and manages pull requests
- **Interactive ask mode**: explains git operations and suggests commands
- **Agentic do mode**: plans, executes, and recovers from failures

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      CLI Interface                          │
│  ask "how do I..."  |  do "task"  |  interactive mode      │
└────────────────────┬────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────┐
│                  GitAgent (Agentic Loop)                    │
│  - Interprets task from user/LLM                           │
│  - Calls git operations via tools                          │
│  - Handles failures and rollback                           │
└────────────────┬──────────────────┬──────────────────────────┘
                 │                  │
     ┌───────────▼────────┐  ┌──────▼──────────────┐
     │  Git Operations    │  │  Conflict Resolver  │
     │  - branch ops      │  │  - analyze markers  │
     │  - commit ops      │  │  - resolve safely   │
     │  - merge/rebase    │  │  - suggest strategy │
     │  - status          │  └─────────────────────┘
     └────────────────────┘
                 │
     ┌───────────▼────────┐
     │   git command      │
     │   execution        │
     └────────────────────┘
```

## Integration with DevTrack

### Use Case 1: Enhanced Git Commits

When devtrack triggers `devtrack git commit`, it can now use git-sage for:

```python
from backend.git_sage import GitAgent, LLMBackend

backend = LLMBackend(provider="ollama", model="llama3")
agent = GitAgent(backend, cwd="/path/to/repo", auto=True)

# Task: improve the commit message using git context
success = agent.run("Improve the commit message based on staged changes, suggesting a well-structured message")
```

### Use Case 2: Automatic Conflict Resolution

After a failed merge/rebase, devtrack can invoke git-sage:

```python
from backend.git_sage import ConflictResolver, GitOperations

git_ops = GitOperations(cwd="/path/to/repo")
conflicts = git_ops.detect_conflicts()

for conflict_file in conflicts:
    content = git_ops.read_conflict_file(conflict_file)
    resolver = ConflictResolver(strategy="smart")
    resolved, has_unresolvable = resolver.resolve_file(content)

    if not has_unresolvable:
        # Write resolved content
        with open(f"{cwd}/{conflict_file}", "w") as f:
            f.write(resolved)
        # Stage the fix
        subprocess.run(["git", "add", conflict_file])
```

### Use Case 3: PR Intelligence

Enhance work updates with PR metadata:

```python
from backend.git_sage import PRFinder

pr_finder = PRFinder(cwd="/path/to/repo")
metadata = pr_finder.suggest_pr_metadata()
stats = pr_finder.get_diff_stats()

# Use in work update prompt
work_update = f"""
Branch: {metadata['branch']}
Changes: {stats['additions']} additions, {stats['deletions']} deletions
Issue: {metadata.get('issue_number', 'N/A')}
"""
```

## Python API Reference

### GitAgent

```python
from backend.git_sage import GitAgent, LLMBackend

agent = GitAgent(
    backend=LLMBackend(provider="ollama", model="llama3"),
    cwd="/path/to/repo",
    verbose=False,
    auto=False
)

# Run an agentic task
success = agent.run(
    task="merge feature-x into main and resolve any conflicts",
    max_steps=30
)
```

#### Methods

- `run(task: str, max_steps: int = 30) -> bool`: Execute a task, return success status
- `detect_conflicts_in_repo() -> List[str]`: Find conflicted files
- `resolve_conflict_in_file(path: str, strategy: str = "smart") -> Tuple[str, bool]`: Resolve conflicts
- `analyze_conflict_file(path: str) -> str`: Get conflict summary
- `get_branch_info() -> str`: Get current branch details
- `list_changes_for_pr() -> str`: Get PR-ready metadata

### GitOperations

```python
from backend.git_sage import GitOperations

ops = GitOperations(cwd="/path/to/repo")

# Branch operations
ops.get_current_branch()
ops.list_branches()
ops.branch_exists("feature-x")
ops.delete_merged_branches("main")

# Commit operations
ops.get_commit_log(n=10)
ops.get_commit_diff("abc123")
ops.find_commit_by_message("fixes #123")

# Status
ops.is_clean()
ops.get_status_files()

# Merge intelligence
complexity = ops.estimate_merge_complexity("main", "feature-x")
```

### ConflictResolver

```python
from backend.git_sage import ConflictResolver, ConflictAnalyzer

resolver = ConflictResolver(strategy="smart")
resolved, has_unresolvable = resolver.resolve_file(content)

analyzer = ConflictAnalyzer()
summary = analyzer.conflict_summary(content)
sections = analyzer.get_conflicted_sections(content)
```

Strategies:
- `smart`: Analyzes conflict markers and resolves when safe
- `ours`: Keeps our version
- `theirs`: Keeps their version
- `both`: Merges both versions

### PRFinder

```python
from backend.git_sage import PRFinder

finder = PRFinder(cwd="/path/to/repo")

# Get PR metadata
metadata = finder.suggest_pr_metadata()
# Returns: {branch, suggested_title, issue_number, commits_count}

# Get diff stats
stats = finder.get_diff_stats()
# Returns: {files, additions, deletions}

# List potential PRs
branches = finder.list_open_branches()
```

## CLI Usage

### Ask Mode

```bash
git-sage ask "how do I undo my last commit but keep changes"
git-sage ask "merge feature-x into main"
```

### Do Mode (Agentic)

```bash
git-sage do "squash my last 5 commits into one with clean message"
git-sage do "merge feature-auth into main and resolve any conflicts"
git-sage do "my last push broke prod, revert to the previous state"
```

### Interactive Shell

```bash
git-sage
sage> do merge feature-x into main
sage> ask what is cherry-pick
sage> context
sage> quit
```

### Configuration

```bash
git-sage --config
git-sage --show-config
```

## DevTrack Integration Workflow

### 1. Enhanced Git Commit Workflow

```bash
# User runs
devtrack git commit -m "initial message"

# DevTrack can now:
# 1. Use git-sage to analyze staged changes
# 2. Generate better commit message suggestions
# 3. Resolve any merge conflicts if rebase needed
# 4. Auto-format and validate
```

### 2. Automatic Conflict Resolution

```bash
# When a merge fails
git merge feature-x  # fails with conflicts

# DevTrack triggers:
agent = GitAgent(...)
success = agent.run("resolve the merge conflicts and complete the merge")

# Agent:
# 1. Detects conflicted files
# 2. Reads conflict markers
# 3. Resolves intelligently
# 4. Stages and commits
```

### 3. Work Update Intelligence

```bash
# In devtrack work update prompt
# Include git-sage PR metadata for better context

from backend.git_sage import PRFinder, GitOperations

pr = PRFinder()
git_ops = GitOperations()

context = {
    "branch": pr.get_current_branch(),
    "changes": git_ops.get_status_files(),
    "pr_metadata": pr.suggest_pr_metadata(),
    "diff_stats": pr.get_diff_stats()
}

# Use this in the NLP parser for better task extraction
```

## Model Recommendations

| Task | Model | Min Size |
|------|-------|----------|
| Ask/suggestions | Llama3, Mistral | 7B |
| Merge resolution | Llama3, CodeLlama | 13B |
| Complex rebases | CodeLlama, GPT-4o | 13B+ |
| Auto mode (no prompts) | Any | 7B+ |

## Error Handling

The agent handles:

1. **Merge conflicts** - Reads files, analyzes markers, resolves intelligently
2. **Rebase conflicts** - Aborts, suggests alternatives, rolls back if needed
3. **Connection errors** - Graceful failure with user-friendly message
4. **LLM parsing errors** - Re-requests with clearer instructions

## Testing

Run tests:
```bash
uv run pytest backend/tests/test_git_sage*.py
```

Test individual components:
```python
from backend.git_sage import ConflictAnalyzer, GitOperations

# Test conflict resolution
analyzer = ConflictAnalyzer()
summary = analyzer.conflict_summary(conflict_content)

# Test git operations
ops = GitOperations()
commits = ops.get_commit_log(n=5)
```

## Troubleshooting

**LLM connection failed**
- Ensure Ollama is running: `ollama serve`
- Check base URL in config: `git-sage --show-config`
- Verify port accessibility: `curl http://localhost:11434/api/tags`

**Agent stuck in loop**
- Press Ctrl+C to interrupt
- Check verbose logs: `git-sage do "task" --verbose`
- Manually resolve conflicts and `git add` them

**Conflicts not resolving**
- Strategy might be too conservative
- Try asking agent: `git-sage ask "help resolve conflict in src/main.py"`
- Manually resolve and use `git add`

## Contributing

To add new git operations:

1. Add method to `GitOperations` class
2. Add tests in `backend/tests/`
3. Document in this file
4. Update agent system prompt if LLM should know about it

## Future Enhancements

- [ ] GitHub/GitLab API integration for actual PR management
- [ ] Performance optimizations for large repos
- [ ] Custom resolution strategies via plugins
- [ ] Integration with devtrack's report generation
- [ ] Pre/post-commit hooks for auto-resolution
