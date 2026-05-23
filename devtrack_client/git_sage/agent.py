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

try:
    from backend.personalization import get_style_instruction as _get_style_instruction
except ImportError:
    def _get_style_instruction(context_type: str = "general") -> str:
        return ""

try:
    from backend import config as _backend_config
except ImportError:
    _backend_config = None  # type: ignore[assignment]

# ─── devtrack git integration ────────────────────────────────────────────────

def _devtrack_git_cmd(cmd: str) -> str:
    """Replace 'git commit' with 'devtrack git commit' to use AI-enhanced commits."""
    import re
    project_root = _backend_config.get_project_root() if _backend_config else ""
    if not project_root:
        return cmd
    devtrack_bin = os.path.join(project_root, "devtrack")
    return re.sub(r'(?<!\w)git(\s+commit\b)', rf'{devtrack_bin} git\1', cmd)


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
_AGENT_SYSTEM_PROMPT_BASE = """You are git-sage, an autonomous git agent running inside the user's terminal.

CRITICAL: You must respond with a single raw JSON object and NOTHING else — no markdown, no prose, no code fences, no explanation. Every single response must be valid JSON starting with `{` and ending with `}`.

You have the ability to execute git commands, read files, and make decisions — exactly like a senior engineer would when handed a task.

## Your capabilities (tools you can call)

You respond with a JSON object choosing ONE action per turn:

### Run a git or shell command
```json
{"action": "run", "cmd": "git --no-pager log -5 --oneline", "reason": "See which commits will be squashed"}
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
{"action": "checkpoint", "label": "before-squash", "reason": "Save state before squash reset"}
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

For **informational tasks** (check, inspect, show, analyze, list, explain), the `summary` MUST contain the full answer — not "analysis completed" or "task done". Include the actual commit details, diff, file contents, or whatever the user asked for. The user can only see what you put in `summary`; they cannot see your internal tool results.


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
11. **Never use `git rebase -i` for squashing** — it opens an interactive editor and blocks. Instead follow this exact squash workflow:
    - Step 1: `git --no-pager log -N --oneline` → read the commit messages you will squash
    - Step 2: `git reset --soft HEAD~N` → move HEAD back N commits, keep all changes staged
    - Step 3: `git commit -m "<synthesized summary of the N commits>"` → create the squashed commit
    Use `git --no-pager log` and `git --no-pager diff` instead of plain `git log`/`git diff` to avoid pager blocking.

## Reading tool output

After each action you'll receive:
```
TOOL_RESULT: <stdout/stderr or file content or user answer>
EXIT_CODE: <0 = success, nonzero = failure>
```

Use this to decide your next action.
"""


def _get_agent_system_prompt() -> str:
    """Build the system prompt, appending the user's personal style if available."""
    style = _get_style_instruction("commit")
    if not style:
        return _AGENT_SYSTEM_PROMPT_BASE
    return (
        _AGENT_SYSTEM_PROMPT_BASE
        + "\n\n## Output Style\n"
        + style
        + "\nApply this style when writing `summary`, `reason`, and `question` fields."
    )


@dataclass
class Checkpoint:
    label: str
    sha: str
    stash_sha: Optional[str] = None  # if there were dirty files


@dataclass
class AgentState:
    checkpoints: dict[str, Checkpoint] = field(default_factory=dict)
    history: list[dict] = field(default_factory=list)   # LLM conversation turns
    step_log: list[dict] = field(default_factory=list)  # executed run actions
    cwd: str = field(default_factory=os.getcwd)


class GitAgent:
    def __init__(self, backend: LLMBackend, cwd: str = ".", verbose: bool = False,
                 auto: bool = False, suggest_only: bool = False):
        self.backend = backend
        self.cwd = os.path.abspath(cwd)
        self.verbose = verbose
        self.auto = auto
        self.suggest_only = suggest_only
        self.state = AgentState(cwd=self.cwd)
        self.git_ops = GitOperations(cwd=self.cwd)
        self.pr_finder = PRFinder(cwd=self.cwd)
        self.conflict_resolver = ConflictResolver(strategy="smart")

    # ── public entry points ──────────────────────────────────────────────────

    def run(self, task: str, max_steps: int = 30) -> bool:
        """Start a new agent session for a task. Resets conversation history."""
        ctx = get_repo_context(self.cwd)
        context_str = format_context(ctx)

        initial_message = (
            f"{context_str}\n\nTask: {task}\n\n"
            "Respond with a JSON action object only. No prose, no markdown."
        )
        self.state.history = [{"role": "user", "content": initial_message}]
        self._print_header(task)
        return self._run_loop(max_steps)

    def followup(self, question: str, max_steps: int = 20) -> bool:
        """Continue the current session with a follow-up question.

        Appends to existing conversation history so the agent has full context
        of everything it already ran and saw.
        """
        self.state.history.append({
            "role": "user",
            "content": (
                f"Follow-up question: {question}\n\n"
                "Respond with a JSON action object only. No prose, no markdown."
            ),
        })
        self._print_header(f"Follow-up: {question}")
        return self._run_loop(max_steps)

    # ── shared agent loop ────────────────────────────────────────────────────

    def _run_loop(self, max_steps: int) -> bool:
        """Core LLM → dispatch → feedback loop. Uses self.state.history as context."""
        for _ in range(max_steps):
            raw = self.backend.raw_chat(self.state.history, _get_agent_system_prompt(),
                                        json_mode=True)
            action = self._parse_action(raw)

            if action is None:
                snippet = raw.strip()[:400].replace("\n", " ")
                self._log_error("LLM returned unparseable response", raw)
                print(f"  {DIM}Raw: {snippet}{RESET}")
                self.state.history.append({"role": "assistant", "content": raw})
                self.state.history.append({
                    "role": "user",
                    "content": "TOOL_RESULT: Invalid JSON. You MUST respond with ONLY a JSON object (no markdown, no prose). Choose your next action.\nEXIT_CODE: 1"
                })
                continue

            self.state.history.append({"role": "assistant", "content": raw})

            result, exit_code = self._dispatch(action)

            if action["action"] == "done":
                self._print_done(action.get("summary", "Task complete."))
                return True
            if action["action"] == "abort":
                self._print_abort(action.get("reason", "Aborted."))
                return False

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

    # Commands that open a pager or editor — must run with stdin/stdout attached
    _INTERACTIVE_PATTERNS = [
        r'\bgit\s+rebase\s+(-i|--interactive)\b',
        r'\bgit\s+add\s+(-p|--patch|-i|--interactive)\b',
        r'\bgit\s+commit\b(?!.*\s+-m\b)',   # commit without -m flag
        r'\bgit\s+log\b(?!.*--no-pager)',
        r'\bgit\s+diff\b(?!.*--no-pager)',
    ]

    def _is_interactive(self, cmd: str) -> bool:
        import re
        return any(re.search(p, cmd) for p in self._INTERACTIVE_PATTERNS)

    def _do_run(self, cmd: str) -> tuple[str, int]:
        """Execute a shell command, capturing git HEAD before for undo support."""
        cmd = _devtrack_git_cmd(cmd)

        # Snapshot HEAD before execution so user can undo this step
        head_before, _ = run_git(["rev-parse", "HEAD"], self.cwd)
        head_before = head_before.strip() if head_before else None

        if self.suggest_only:
            print(f"  {DIM}  [suggest] $ {cmd}{RESET}")
            self.state.step_log.append({
                "cmd": cmd, "git_head_before": head_before,
                "exit_code": 0, "skipped": True,
            })
            return "Command suggested (not executed).", 0

        if not self.auto:
            if not self._confirm_run(cmd):
                self.state.step_log.append({
                    "cmd": cmd, "git_head_before": head_before,
                    "exit_code": 0, "skipped": True,
                })
                return "User skipped this command.", 0

        try:
            if self._is_interactive(cmd):
                # Let editor/pager render in the user's terminal
                result = subprocess.run(cmd, shell=True, text=True, cwd=self.cwd)
                output = "(interactive command completed)" if result.returncode == 0 \
                    else f"(exited with code {result.returncode})"
            else:
                result = subprocess.run(
                    cmd, shell=True, capture_output=True, text=True, cwd=self.cwd
                )
                output = (result.stdout + result.stderr).strip() or "(no output)"

            self.state.step_log.append({
                "cmd": cmd, "git_head_before": head_before,
                "exit_code": result.returncode, "skipped": False,
            })
            return output, result.returncode
        except Exception as e:
            self.state.step_log.append({
                "cmd": cmd, "git_head_before": head_before,
                "exit_code": 1, "skipped": False,
            })
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

        for abort_cmd in ["git rebase --abort", "git merge --abort", "git cherry-pick --abort"]:
            subprocess.run(abort_cmd, shell=True, capture_output=True, cwd=self.cwd)

        out, code = run_git(["reset", "--hard", cp.sha], self.cwd)
        if code != 0:
            return f"Reset failed: {out}", 1

        if cp.stash_sha:
            run_git(["stash", "pop"], self.cwd)

        return f"Rolled back to '{label}' ({cp.sha[:8]})", 0

    def _do_ask(self, question: str) -> tuple[str, int]:
        print(f"\n  {CYAN}{BOLD}❓ {question}{RESET}")
        answer = input(f"  {BOLD}your answer>{RESET} ").strip()
        return answer or "(no answer)", 0

    # ── session history & undo ───────────────────────────────────────────────

    def get_step_history(self) -> list[dict]:
        """Return executed (non-skipped) run-action steps."""
        return [s for s in self.state.step_log if not s.get("skipped")]

    def undo_step(self, step_idx: int) -> bool:
        """Reset git to state before step_idx (0-indexed). Removes that step and all after."""
        steps = self.get_step_history()
        if step_idx < 0 or step_idx >= len(steps):
            print(f"{RED}Invalid step number.{RESET}")
            return False

        entry = steps[step_idx]
        head_before = entry.get("git_head_before")
        if not head_before:
            print(f"{YELLOW}No git snapshot for step {step_idx + 1} — cannot undo automatically.{RESET}")
            print(f"  {DIM}Command was: {entry['cmd']}{RESET}")
            return False

        print(f"{YELLOW}Undoing step {step_idx + 1}:{RESET} {entry['cmd']}")
        print(f"  {DIM}Resetting to {head_before[:8]}...{RESET}")

        for abort_cmd in ["git rebase --abort", "git merge --abort", "git cherry-pick --abort"]:
            subprocess.run(abort_cmd, shell=True, capture_output=True, cwd=self.cwd)

        out, code = run_git(["reset", "--hard", head_before], self.cwd)
        if code == 0:
            original_idx = self.state.step_log.index(entry)
            self.state.step_log = self.state.step_log[:original_idx]
            print(f"{GREEN}Undone. Restored to {head_before[:8]}.{RESET}")
            return True

        print(f"{RED}Undo failed: {out}{RESET}")
        print(f"  {DIM}Manual: git reset --hard {head_before[:8]}{RESET}")
        return False

    # ── git operation helpers ────────────────────────────────────────────────

    def detect_conflicts_in_repo(self) -> list[str]:
        return self.git_ops.detect_conflicts()

    def resolve_conflict_in_file(self, path: str, strategy: str = "smart") -> tuple[str, bool]:
        try:
            content = self.git_ops.read_conflict_file(path)
            if "<<<<<<< " not in content:
                return f"No conflicts found in {path}", 1
            resolver = ConflictResolver(strategy=strategy)
            resolved, has_unresolvable = resolver.resolve_file(content)
            if has_unresolvable:
                unresolvable = resolver.extract_unresolvable_conflicts(content)
                return f"Resolved some conflicts in {path}, but {len(unresolvable)} remain unresolvable", 0
            return f"Resolved all conflicts in {path}", 0
        except Exception as e:
            return f"Error resolving conflicts: {e}", 1

    def analyze_conflict_file(self, path: str) -> str:
        try:
            content = self.git_ops.read_conflict_file(path)
            analyzer = ConflictAnalyzer()
            summary = analyzer.conflict_summary(content)
            sections = analyzer.get_conflicted_sections(content)
            msg = f"{summary}\n"
            for sec in sections[:3]:
                msg += f"\nConflict #{sec['number']}: {sec['current_lines']} vs {sec['incoming_lines']} lines\n"
                msg += f"  From: {sec['branch_from']} → To: {sec['branch_to']}"
            return msg
        except Exception as e:
            return f"Error analyzing conflicts: {e}"

    def get_branch_info(self) -> str:
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
        lines = summary.strip().splitlines()
        if len(lines) <= 1:
            print(f"\n  {GREEN}{BOLD}✅ Done:{RESET} {summary}\n")
        else:
            print(f"\n  {GREEN}{BOLD}✅ Done:{RESET}")
            for line in lines:
                print(f"  {line}")
            print()

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
            if ans in ("y", "yes"):   return True
            if ans in ("n", "no", "skip"): return False
            if ans == "abort":        raise KeyboardInterrupt("User aborted agent.")

    def _parse_action(self, raw: str) -> Optional[dict]:
        raw = raw.strip()
        if raw.startswith("```"):
            lines = raw.split("\n")
            raw = "\n".join(lines[1:-1] if lines[-1].strip() == "```" else lines[1:])
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            import re
            match = re.search(r'\{.*\}', raw, re.DOTALL)
            if match:
                try:
                    return json.loads(match.group())
                except json.JSONDecodeError:
                    pass
        return None
