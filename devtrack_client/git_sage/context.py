"""Gather git repo context to send to the LLM."""
import subprocess
import os
from typing import Optional


def run_git(args: list[str], cwd: str = ".") -> tuple[str, bool]:
    """Run a git command and return (output, success)."""
    try:
        result = subprocess.run(
            ["git"] + args,
            capture_output=True,
            text=True,
            cwd=cwd,
        )
        out = result.stdout.strip()
        err = result.stderr.strip()
        combined = out if out else err
        return combined, result.returncode == 0
    except FileNotFoundError:
        return "git not found", False


def get_repo_context(cwd: str = ".") -> dict:
    """Collect full repo context: branch, status, log, remotes, stash."""
    ctx = {}

    ctx["branch"], _ = run_git(["branch", "--show-current"], cwd)
    ctx["status"], _ = run_git(["status", "--short"], cwd)
    ctx["log"], _ = run_git(["log", "--oneline", "-10"], cwd)
    ctx["remotes"], _ = run_git(["remote", "-v"], cwd)
    ctx["stash"], _ = run_git(["stash", "list"], cwd)
    ctx["diff_staged"], _ = run_git(["diff", "--cached", "--stat"], cwd)
    ctx["diff_unstaged"], _ = run_git(["diff", "--stat"], cwd)
    ctx["branches"], _ = run_git(["branch", "-a"], cwd)

    # Detect if we're mid-merge/rebase
    git_dir_out, _ = run_git(["rev-parse", "--git-dir"], cwd)
    git_dir = git_dir_out.strip()

    ctx["mid_merge"] = os.path.exists(os.path.join(cwd, git_dir, "MERGE_HEAD"))
    ctx["mid_rebase"] = os.path.exists(os.path.join(cwd, git_dir, "rebase-merge")) or \
                        os.path.exists(os.path.join(cwd, git_dir, "rebase-apply"))
    ctx["mid_cherry_pick"] = os.path.exists(os.path.join(cwd, git_dir, "CHERRY_PICK_HEAD"))

    return ctx


def format_context(ctx: dict) -> str:
    """Format context dict into a readable block for the LLM prompt."""
    lines = [
        "=== CURRENT GIT REPO STATE ===",
        f"Branch: {ctx.get('branch', 'unknown')}",
    ]

    if ctx.get("mid_merge"):
        lines.append("⚠️  Currently in the middle of a MERGE")
    if ctx.get("mid_rebase"):
        lines.append("⚠️  Currently in the middle of a REBASE")
    if ctx.get("mid_cherry_pick"):
        lines.append("⚠️  Currently in the middle of a CHERRY-PICK")

    if ctx.get("status"):
        lines += ["", "Working tree status:", ctx["status"]]
    if ctx.get("diff_staged"):
        lines += ["", "Staged changes:", ctx["diff_staged"]]
    if ctx.get("diff_unstaged"):
        lines += ["", "Unstaged changes:", ctx["diff_unstaged"]]
    if ctx.get("stash"):
        lines += ["", "Stash:", ctx["stash"]]
    if ctx.get("log"):
        lines += ["", "Recent commits:", ctx["log"]]
    if ctx.get("branches"):
        lines += ["", "Branches:", ctx["branches"]]
    if ctx.get("remotes"):
        lines += ["", "Remotes:", ctx["remotes"]]

    lines.append("=== END STATE ===")
    return "\n".join(lines)
