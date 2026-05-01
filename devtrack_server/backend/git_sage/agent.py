"""
Agent executor — runs the agentic loop.

The LLM is given a set of "tools" it can call (git operations, file reads, etc.).
Each tool returns output back to the LLM, which decides the next action.
This continues until the LLM emits DONE or ABORT.
"""
import json
import os
import subprocess
from dataclasses import dataclass, field
from typing import Optional

from .llm import LLMBackend
from .context import get_repo_context, format_context, run_git
from .git_operations import GitOperations
from .conflict_resolver import ConflictResolver, ConflictAnalyzer
from .pr_finder import PRFinder

# ─── ANSI ────────────────────────────────────────────────────────────────────
RESET  = "\033[0m"
BOLD  = "\033[1m"
DIM   = "\033[2m"
GREEN  = "\033[92m"
YELLOW= "\033[93m"
CYAN  = "\033[96m"
RED    = "\033[91m"
MAGENTA="\033[95m"
BLUE  = "\033[94m"

# ─── SYSTEM PROMPT ───────────────────────────────────────────────────────────
AGENT_SYSTEM_PROMPT = """You are git-sage, an autonomous git agent running inside the user's terminal.

You have the ability to execute git commands, read files, and make decisions — exactly like a senior engineer would when handed a task.

## Your capabilities (tools you can call)

You respond with a JSON object choosing ONE action per turn:

### Run a git or shell command
```json
{"action": "run", "cmd": "git rebase -i HEAD~3", "reason": "Squash last 3 commits interactively"}
```

### Read a file (e.g. to inspect conflict markers)
```json
{"action": "read_file", "path": "src/main.py", "reason": "Check merge conflict in this file"}
```

### Edit a file (write the FULL new content)
```json
{"action": "write_file", "path": "src/main.py", "content": "...full file content...", "reason": "Resolve conflict by keeping both changes"}
```

### Create a checkpoint (saves current HEAD sha for possible rollback)
```json
{"action": "checkpoint", "label": "before-rebase", "reason": "Save state before risky operation"}
```

### Roll back to a checkpoint
```json
{"action": "rollback", "label": "before-rebase", "reason": "Rebase went wrong, restoring"}
```

### Ask the user a question (when you genuinely need input)
```json
{"action": "ask", "question": "Which branch should I merge into main?", "reason": "Ambiguous target"}
```

### Signal task complete
```json
{"action": "done", "summary": "Squashed 3 commits into one. New HEAD: abc1234"}
```

### Abort with explanation
```json
{"action": "abort", "reason": "Conflict in src/auth.py is ambiguous — both versions add different logic. Manual resolution required."}
```

## Conflict Resolution Guide

When resolving conflicts:
1. **Read the conflicted file** to understand both sides
2. **Analyze the conflict**: Is one side empty? Are changes adjacent? Do they overlap?
3. **Apply resolution strategy**:
   - If one side is empty: use the non-empty side
   - If changes are adjacent (no overlap): merge both
   - If changes overlap logically: ask the user OR use judgment
   - If identical: use either side
4. **Verify**: After resolving, check the file makes sense
5. **Format properly**: Ensure proper syntax, no leftover markers

## Behavioral rules

1. **Always checkpoint before destructive operations** (rebase, reset, force-push, merge with conflicts).
2. **Read before writing** — if you need to resolve a conflict, read the file first, then write the resolved version.
3. **Verify after each step** — after a git command, check its output. If it failed, diagnose and either retry or rollback.
4. **Prefer safe flags** — use `--no-ff` for merges, avoid `--force` unless necessary.
5. **Resolve conflicts intelligently** — read conflict markers, analyze both sides, merge when safe, ask user for ambiguous cases.
6. **One action per response** — output only the JSON object, nothing else. No markdown, no explanation outside the JSON.
7. **Use the `reason` field** to explain your thinking — it's shown to the user as a log.
8. **If something unexpected happens**, adapt. Don't blindly retry the same failing command.
9. **Smart conflict resolution**: Prefer keeping both changes unless contradictory. For feature branches: prefer incoming. For fixes: prefer HEAD.
10. **Never truncate file content in write_file** — write the complete file.

## Reading tool output

After each action you'll receive:
```
TOOL_RESULT: <stdout/stderr or file content or user answer>
EXIT_CODE: <0 = success, nonzero = failure>
```

Use this to decide your next action.
"""


@dataclass
class Checkpoint:
    label: str
    sha: str
    stash_sha: Optional[str] = None  # if there were dirty files


@dataclass
class AgentState:
    checkpoints: dict[str, Checkpoint] = field(default_factory=dict)
    history: list[dict] = field(default_factory=list)   # LLM conversation turns
    step_log: list[dict] = field(default_factory=list)  # for display
    cwd: str = field(default_factory=os.getcwd)


class GitAgent:
    def __init__(self, backend: LLMBackend, cwd: str = ".", verbose: bool = False, auto: bool = False):
        self.backend = backend
        self.cwd = os.path.abspath(cwd)
        self.verbose = verbose
        self.auto = auto
        self.state = AgentState(cwd=self.cwd)
        self.git_ops = GitOperations(cwd=self.cwd)
        self.pr_finder = PRFinder(cwd=self.cwd)
        self.conflict_resolver = ConflictResolver(strategy="smart")

    # ── public entry point ──────────────────────────────────────────────────

    def run(self, task: str, max_steps: int = 30) -> bool:
        """Run the agent on a task. Returns True on success."""
        ctx = get_repo_context(self.cwd)
        context_str = format_context(ctx)

        # Seed the conversation
        initial_message = f"{context_str}\n\nTask: {task}"
        self.state.history = [{"role": "user", "content": initial_message}]

        self._print_header(task)

        for step in range(max_steps):
            # Ask LLM for next action
            raw = self.backend.raw_chat(self.state.history, AGENT_SYSTEM_PROMPT)
            action = self._parse_action(raw)

            if action is None:
                self._log_error("LLM returned unparseable response", raw)
                # Feed back the parse error
                self.state.history.append({"role": "assistant", "content": raw})
                self.state.history.append({
                    "role": "user",
                    "content": "TOOL_RESULT: Invalid JSON response. Respond with ONLY a JSON action object.\nEXIT_CODE: 1"
                })
                continue

            self.state.history.append({"role": "assistant", "content": raw})

            result, exit_code = self._dispatch(action)

            # Terminal actions
            if action["action"] == "done":
                self._print_done(action.get("summary", "Task complete."))
                return True
            if action["action"] == "abort":
                self._print_abort(action.get("reason", "Aborted."))
                return False

            # Feed result back
            feedback = f"TOOL_RESULT: {result}\nEXIT_CODE: {exit_code}"
            self.state.history.append({"role": "user", "content": feedback})

        self._print_error(f"Reached maximum steps ({max_steps}). Task incomplete.")
        return False

    # ── action dispatcher ───────────────────────────────────────────────────

    def _dispatch(self, action: dict) -> tuple[str, int]:
        kind = action.get("action")
        reason = action.get("reason", "")
        self._print_step(kind, action, reason)

        if kind == "run":
            return self._do_run(action["cmd"])
        elif kind == "read_file":
            return self._do_read_file(action["path"])
        elif kind == "write_file":
            return self._do_write_file(action["path"], action["content"])
        elif kind == "checkpoint":
            return self._do_checkpoint(action["label"])
        elif kind == "rollback":
            return self._do_rollback(action["label"])
        elif kind == "ask":
            return self._do_ask(action["question"])
        elif kind in ("done", "abort"):
            return "", 0
        else:
            return f"Unknown action: {kind}", 1

    # ── tool implementations ─────────────────────────────────────────────────

    def _do_run(self, cmd: str) -> tuple[str, int]:
        """Execute a shell command."""
        if not self.auto:
            if not self._confirm_run(cmd):
                return "User skipped this command.", 0

        try:
            result = subprocess.run(
                cmd, shell=True, capture_output=True, text=True, cwd=self.cwd
            )
            output = (result.stdout + result.stderr).strip()
            if not output:
                output = "(no output)"

            # Handle interactive git commands that need stdin (rebase -i, etc.)
            # We auto-set GIT_SEQUENCE_EDITOR for interactive rebases
            return output, result.returncode
        except Exception as e:
            return str(e), 1

    def _do_read_file(self, path: str) -> tuple[str, int]:
        full = os.path.join(self.cwd, path)
        try:
            with open(full, "r", encoding="utf-8", errors="replace") as f:
                content = f.read()
            if len(content) > 8000:
                content = content[:8000] + "\n... (truncated)"
            return content, 0
        except FileNotFoundError:
            return f"File not found: {path}", 1
        except Exception as e:
            return str(e), 1

    def _do_write_file(self, path: str, content: str) -> tuple[str, int]:
        full = os.path.join(self.cwd, path)
        if not self.auto:
            print(f"\n  {YELLOW}Write file:{RESET} {path}")
            preview = content[:400] + ("..." if len(content) > 400 else "")
            print(f"  {DIM}{preview}{RESET}")
            if not self._confirm("Apply this file write?"):
                return "User declined file write.", 0
        try:
            os.makedirs(os.path.dirname(full), exist_ok=True) if os.path.dirname(full) else None
            with open(full, "w", encoding="utf-8") as f:
                f.write(content)
            return f"Written {len(content)} bytes to {path}", 0
        except Exception as e:
            return str(e), 1

    def _do_checkpoint(self, label: str) -> tuple[str, int]:
        sha, ok = run_git(["rev-parse", "HEAD"], self.cwd)
        if not ok:
            return "Not in a git repo or no commits yet.", 1

        # Stash if dirty
        status, _ = run_git(["status", "--porcelain"], self.cwd)
        stash_sha = None
        if status.strip():
            stash_out, stash_ok = run_git(
                ["stash", "push", "-m", f"git-sage-checkpoint-{label}"], self.cwd
            )
            if stash_ok:
                stash_sha_out, _ = run_git(["stash", "show", "--format=%H", "-s"], self.cwd)
                stash_sha = stash_sha_out.strip()

        self.state.checkpoints[label] = Checkpoint(label=label, sha=sha.strip(), stash_sha=stash_sha)
        return f"Checkpoint '{label}' saved at {sha[:8]}" + (f" (stash: {stash_sha[:8]})" if stash_sha else ""), 0

    def _do_rollback(self, label: str) -> tuple[str, int]:
        cp = self.state.checkpoints.get(label)
        if not cp:
            return f"No checkpoint named '{label}'. Available: {list(self.state.checkpoints.keys())}", 1

        print(f"\n  {RED}⚠ Rolling back to checkpoint '{label}' ({cp.sha[:8]}){RESET}")
        if not self.auto and not self._confirm("Confirm rollback?"):
            return "Rollback cancelled by user.", 0

        # Abort any ongoing operation
        for abort_cmd in ["git rebase --abort", "git merge --abort", "git cherry-pick --abort"]:
            subprocess.run(abort_cmd, shell=True, capture_output=True, cwd=self.cwd)

        # Reset to checkpoint sha
        out, code = run_git(["reset", "--hard", cp.sha], self.cwd)
        if code != 0:
            return f"Reset failed: {out}", 1

        # Restore stash if there was one
        if cp.stash_sha:
            run_git(["stash", "pop"], self.cwd)

        return f"Rolled back to '{label}' ({cp.sha[:8]})", 0

    def _do_ask(self, question: str) -> tuple[str, int]:
        print(f"\n  {CYAN}{BOLD}❓ {question}{RESET}")
        answer = input(f"  {BOLD}your answer>{RESET} ").strip()
        return answer or "(no answer)", 0

    # ── git operation helpers ────────────────────────────────────────────────

    def detect_conflicts_in_repo(self) -> list[str]:
        """Detect conflicted files using GitOperations."""
        return self.git_ops.detect_conflicts()

    def resolve_conflict_in_file(self, path: str, strategy: str = "smart") -> tuple[str, bool]:
        """Read, analyze, and resolve conflicts in a file."""
        try:
            content = self.git_ops.read_conflict_file(path)
            if "<<<<<<< " not in content:
                return f"No conflicts found in {path}", 1

            resolver = ConflictResolver(strategy=strategy)
            resolved, has_unresolvable = resolver.resolve_file(content)

            if has_unresolvable:
                # Show what couldn't be resolved
                unresolvable = resolver.extract_unresolvable_conflicts(content)
                msg = f"Resolved some conflicts in {path}, but {len(unresolvable)} remain unresolvable"
                return msg, 0  # Return 0 because we made progress

            return f"Resolved all conflicts in {path}", 0
        except Exception as e:
            return f"Error resolving conflicts: {e}", 1

    def analyze_conflict_file(self, path: str) -> str:
        """Analyze conflicts in a file and provide summary."""
        try:
            content = self.git_ops.read_conflict_file(path)
            analyzer = ConflictAnalyzer()
            summary = analyzer.conflict_summary(content)
            sections = analyzer.get_conflicted_sections(content)

            msg = f"{summary}\n"
            for sec in sections[:3]:  # Show first 3 conflicts
                msg += f"\nConflict #{sec['number']}: {sec['current_lines']} vs {sec['incoming_lines']} lines\n"
                msg += f"  From: {sec['branch_from']} → To: {sec['branch_to']}"

            return msg
        except Exception as e:
            return f"Error analyzing conflicts: {e}"

    def get_branch_info(self) -> str:
        """Get detailed branch information."""
        try:
            current = self.git_ops.get_current_branch()
            tracking = self.git_ops.check_tracking_branch()
            ahead, behind = self.git_ops.get_ahead_behind()

            msg = f"Current branch: {current}\n"
            if tracking:
                msg += f"Tracking: {tracking}\n"
                msg += f"Ahead: {ahead}, Behind: {behind}"
            return msg
        except Exception as e:
            return f"Error getting branch info: {e}"

    def list_changes_for_pr(self) -> str:
        """Get PR metadata and diff statistics."""
        try:
            metadata = self.pr_finder.suggest_pr_metadata()
            stats = self.pr_finder.get_diff_stats()

            msg = "PR Metadata:\n"
            for key, val in metadata.items():
                msg += f"  {key}: {val}\n"

            msg += f"\nDiff Stats:\n"
            msg += f"  Files: {stats['files']}\n"
            msg += f"  Additions: {stats['additions']}\n"
            msg += f"  Deletions: {stats['deletions']}"

            return msg
        except Exception as e:
            return f"Error getting PR info: {e}"

    # ── display helpers ──────────────────────────────────────────────────────

    def _print_header(self, task: str):
        print(f"\n{CYAN}{BOLD}🌿 git-sage agent{RESET}")
        print(f"  {DIM}Task:{RESET} {task}")
        print(f"  {DIM}{'─'*50}{RESET}\n")

    def _print_step(self, kind: str, action: dict, reason: str):
        icons = {
            "run": "⚡", "read_file": "📄", "write_file": "✏️ ",
            "checkpoint": "📌", "rollback": "⏪", "ask": "❓",
            "done": "✅", "abort": "🛑",
        }
        colors = {
            "run": GREEN, "read_file": BLUE, "write_file": YELLOW,
            "checkpoint": MAGENTA, "rollback": RED, "ask": CYAN,
            "done": GREEN, "abort": RED,
        }
        icon = icons.get(kind, "•")
        color = colors.get(kind, RESET)

        detail = ""
        if kind == "run":
            detail = f" {BOLD}{action.get('cmd', '')}{RESET}"
        elif kind in ("read_file", "write_file"):
            detail = f" {action.get('path', '')}"
        elif kind in ("checkpoint", "rollback"):
            detail = f" [{action.get('label', '')}]"

        print(f"  {color}{icon} {kind.upper()}{RESET}{detail}")
        if reason:
            print(f"     {DIM}{reason}{RESET}")

    def _print_done(self, summary: str):
        print(f"\n  {GREEN}{BOLD}✅ Done:{RESET} {summary}\n")

    def _print_abort(self, reason: str):
        print(f"\n  {RED}{BOLD}🛑 Aborted:{RESET} {reason}\n")

    def _print_error(self, msg: str):
        print(f"\n  {RED}{BOLD}❌ {msg}{RESET}\n")

    def _log_error(self, msg: str, detail: str = ""):
        print(f"  {RED}[parse error]{RESET} {msg}")
        if self.verbose and detail:
            print(f"  {DIM}{detail[:300]}{RESET}")

    def _confirm_run(self, cmd: str) -> bool:
        print(f"  {DIM}  $ {cmd}{RESET}")
        return self._confirm("Run?")

    def _confirm(self, prompt: str) -> bool:
        while True:
            ans = input(f"  {YELLOW}{prompt}{RESET} [y/n/abort]: ").strip().lower()
            if ans in ("y", "yes"):
                return True
            if ans in ("n", "no", "skip"):
                return False
            if ans == "abort":
                raise KeyboardInterrupt("User aborted agent.")

    def _parse_action(self, raw: str) -> Optional[dict]:
        """Parse JSON action from LLM output, tolerating markdown fences."""
        raw = raw.strip()
        # Strip markdown code fences if present
        if raw.startswith("```"):
            lines = raw.split("\n")
            raw = "\n".join(lines[1:-1] if lines[-1].strip() == "```" else lines[1:])
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            # Try to extract JSON object from response
            import re
            match = re.search(r'\{.*\}', raw, re.DOTALL)
            if match:
                try:
                    return json.loads(match.group())
                except json.JSONDecodeError:
                    pass
        return None
