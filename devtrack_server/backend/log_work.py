"""
log_work.py — immediately log a commit as a work entry to the configured PM platform.

Called by devtrack-git-wrapper.sh after a successful AI-enhanced commit when the user
presses 'y' to log their work. Provides real PM sync with immediate terminal feedback,
unlike the daemon's background fsnotify-triggered flow.

Usage:
    python backend/log_work.py \
        --commit <hash> \
        --message "commit message" \
        --branch <branch> \
        --repo /path/to/repo \
        [--time "2h"]
"""

import argparse
import asyncio
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))


# ── Env loading ────────────────────────────────────────────────────────────────

def _load_env() -> None:
    here = Path(__file__).resolve()
    for parent in [here, *here.parents]:
        candidate = parent / ".env"
        if candidate.exists():
            try:
                from dotenv import load_dotenv
                load_dotenv(candidate, override=True)
            except ImportError:
                pass
            return


_load_env()


# ── Workspace detection ────────────────────────────────────────────────────────

def _find_workspace(repo_path: str):
    """Return (pm_platform, pm_project, workspace_settings) for the given repo.

    Checks WORKSPACES_FILE env var first, then walks up from repo_path looking
    for workspaces.yaml.  Returns (None, None, {}) when no match is found.
    """
    try:
        import yaml
    except ImportError:
        return None, None, {}

    repo_abs = str(Path(repo_path).resolve())
    home = str(Path.home())

    from backend.config import get_workspaces_file
    ws_file_env = get_workspaces_file()
    candidates = []
    if ws_file_env:
        candidates.append(Path(ws_file_env))
    # Check the repo dir itself first, then walk up
    repo_path_obj = Path(repo_abs)
    for p in [repo_path_obj, *repo_path_obj.parents]:
        candidates.append(p / "workspaces.yaml")

    workspace_file = next((p for p in candidates if p.exists()), None)
    if not workspace_file:
        return None, None, {}

    try:
        with open(workspace_file) as f:
            cfg = yaml.safe_load(f)
    except Exception:
        return None, None, {}

    if not cfg or "workspaces" not in cfg:
        return None, None, {}

    for ws in cfg["workspaces"]:
        if not ws.get("enabled", True):
            continue
        ws_path = ws.get("path", "")
        if ws_path.startswith("~/"):
            ws_path = home + ws_path[1:]
        ws_abs = str(Path(ws_path).resolve())
        if ws_abs == repo_abs or repo_abs.startswith(ws_abs + "/"):
            return (
                ws.get("pm_platform", ""),
                ws.get("pm_project", ""),
                {
                    "assignee":       ws.get("pm_assignee", ""),
                    "iteration_path": ws.get("pm_iteration_path", ""),
                    "area_path":      ws.get("pm_area_path", ""),
                    "milestone":      ws.get("pm_milestone", 0),
                },
            )

    return None, None, {}


# ── NLP parsing ────────────────────────────────────────────────────────────────

def _parse_message(message: str, repo_path: str = "."):
    """Return (ticket_id, status, description) from the commit message."""
    try:
        from backend.nlp_parser import NLPParser
        parsed = NLPParser().parse(message, repo_path=repo_path)
        return (
            parsed.ticket_id or "",
            parsed.status or "in_progress",
            parsed.description or message,
        )
    except Exception:
        return "", "in_progress", message


# ── Comment builder ────────────────────────────────────────────────────────────

def _build_comment(commit_info: dict, description: str, status: str, time_spent: str) -> str:
    short_hash = commit_info.get("commit_hash", "")[:12]
    author     = commit_info.get("author", "")
    orig_msg   = commit_info.get("commit_message", "")

    lines = [
        f"**Commit**: `{short_hash}`",
        f"**Author**: {author}",
        f"**Message**: {orig_msg}",
    ]
    if time_spent:
        lines.append(f"**Time spent**: {time_spent}")
    if description and description != orig_msg:
        lines.append(f"**Update**: {description}")
    if status and status != "in_progress":
        lines.append(f"**Status**: {status}")

    return "\n\n".join(lines)


# ── Platform sync functions ────────────────────────────────────────────────────

async def _sync_github(
    description: str,
    ticket_id: str,
    status: str,
    commit_info: dict,
    workspace_settings: dict,
    time_spent: str,
) -> bool:
    try:
        from backend.github.client import GitHubClient
    except ImportError as e:
        print(f"  ERROR: could not import GitHub client: {e}")
        return False

    client = GitHubClient()
    if not client.is_configured():
        print("  GitHub not configured — skipping.")
        return False

    comment = _build_comment(commit_info, description, status, time_spent)
    query   = description or commit_info.get("commit_message", "")

    try:
        # 1. Explicit ticket selected by user (picker or #ref) — use directly
        if ticket_id and ticket_id.startswith("#"):
            try:
                num = int(ticket_id[1:])
                ok  = await client.add_comment(num, comment)
                if ok:
                    print(f"  ✓ Commented on GitHub issue #{num}")
                    return True
            except Exception:
                pass

        # 2. Try matching an open issue via TaskMatcher
        try:
            from backend.task_matcher import TaskMatcher
            issues = await client.get_my_issues(state="open")
            if issues:
                match = TaskMatcher().match_task(query, issues)
                if match and match.confidence >= 0.4:
                    ok = await client.add_comment(match.task.number, comment)
                    if ok:
                        print(f"  ✓ Commented on GitHub issue #{match.task.number}: {match.task.title[:55]}")
                        return True
        except Exception:
            pass

        # 3. create_on_no_match
        from backend.config import get_github_create_on_no_match
        create = get_github_create_on_no_match()
        if create:
            title = (description or commit_info.get("commit_message", ""))[:80]
            body  = (
                f"Auto-created from commit `{commit_info.get('commit_hash','')[:12]}`."
                f"\n\n{comment}"
            )
            milestone = workspace_settings.get("milestone") or None
            issue = await client.create_issue(
                title=title,
                body=body,
                milestone=milestone if isinstance(milestone, int) and milestone > 0 else None,
            )
            if issue:
                print(f"  ✓ Created GitHub issue #{issue.number}: {title}")
                return True

        print("  ℹ GitHub: no matching issue found (set GITHUB_CREATE_ON_NO_MATCH=true to auto-create)")
        return False

    except Exception as e:
        print(f"  ERROR syncing to GitHub: {e}")
        return False
    finally:
        await client.close()


async def _sync_gitlab(
    description: str,
    ticket_id: str,
    status: str,
    commit_info: dict,
    workspace_settings: dict,
    time_spent: str,
) -> bool:
    try:
        from backend.gitlab.client import GitLabClient
    except ImportError as e:
        print(f"  ERROR: could not import GitLab client: {e}")
        return False

    client = GitLabClient()
    if not client.is_configured():
        print("  GitLab not configured — skipping.")
        return False

    comment  = _build_comment(commit_info, description, status, time_spent)
    query    = description or commit_info.get("commit_message", "")
    from backend.config import get_gitlab_project_id_str
    proj_env = get_gitlab_project_id_str()

    try:
        # 1. Explicit ticket selected by user (picker) — numeric IID like #42
        if ticket_id and ticket_id.startswith("#"):
            try:
                iid = int(ticket_id[1:])
                ok  = await client.add_comment(int(proj_env), iid, comment)
                if ok:
                    print(f"  ✓ Commented on GitLab issue #{iid}")
                    return True
            except Exception:
                pass

        # 2. TaskMatcher fallback
        try:
            from backend.task_matcher import TaskMatcher
            issues = await client.get_my_issues(state="opened")
            if issues:
                match = TaskMatcher().match_task(query, issues)
                if match and match.confidence >= 0.4:
                    pid = match.task.project_id or proj_env
                    ok  = await client.add_comment(int(pid), match.task.iid, comment)
                    if ok:
                        print(f"  ✓ Commented on GitLab issue #{match.task.iid}: {match.task.title[:55]}")
                        return True
        except Exception:
            pass

        print("  ℹ GitLab: no matching issue found")
        return False

    except Exception as e:
        print(f"  ERROR syncing to GitLab: {e}")
        return False


async def _sync_azure(
    description: str,
    ticket_id: str,
    status: str,
    commit_info: dict,
    workspace_settings: dict,
    time_spent: str,
) -> bool:
    try:
        from backend.azure.client import AsyncAzureClient
    except ImportError as e:
        print(f"  ERROR: could not import Azure client: {e}")
        return False

    client = AsyncAzureClient()
    if not client.is_configured():
        print("  Azure DevOps not configured — skipping.")
        return False

    comment = _build_comment(commit_info, description, status, time_spent)
    query   = description or commit_info.get("commit_message", "")

    try:
        # 1. Explicit ticket selected by user (picker) — numeric ID like #42
        if ticket_id and ticket_id.startswith("#"):
            try:
                item_id = int(ticket_id[1:])
                ok = await client.add_comment(item_id, comment)
                if ok:
                    print(f"  ✓ Commented on Azure work item #{item_id}")
                    return True
            except Exception:
                pass

        # 2. TaskMatcher fallback
        try:
            from backend.task_matcher import TaskMatcher
            items = await client.get_my_work_items()
            if items:
                match = TaskMatcher().match_task(query, items)
                if match and match.confidence >= 0.4:
                    ok = await client.add_comment(match.task.id, comment)
                    if ok:
                        print(f"  ✓ Commented on Azure work item #{match.task.id}: {match.task.title[:55]}")
                        return True
        except Exception:
            pass

        print("  ℹ Azure DevOps: no matching work item found")
        return False

    except Exception as e:
        print(f"  ERROR syncing to Azure: {e}")
        return False


# ── Entry point ────────────────────────────────────────────────────────────────

async def run(commit: str, message: str, branch: str, repo: str, time_spent: str, ticket: str = "") -> int:
    pm_platform, pm_project, workspace_settings = _find_workspace(repo)

    if pm_platform is None:
        print("  ℹ No workspace configured for this repo — add it to workspaces.yaml.")
        return 0

    if pm_platform == "none":
        print("  ℹ PM sync disabled for this workspace (pm_platform: none).")
        return 0

    # If the user explicitly picked a ticket (from the interactive picker), use it directly
    if ticket:
        ticket_id, status, description = ticket, "in_progress", message
    else:
        ticket_id, status, description = _parse_message(message, repo_path=repo)

    commit_info = {
        "commit_hash":    commit,
        "commit_message": message,
        "author":         os.environ.get("GIT_AUTHOR_NAME", ""),
    }

    _DISPATCH = {
        "github": _sync_github,
        "gitlab": _sync_gitlab,
        "azure":  _sync_azure,
    }

    fn = _DISPATCH.get(pm_platform)
    if fn:
        await fn(description, ticket_id, status, commit_info, workspace_settings, time_spent)
    else:
        print(f"  ℹ PM platform '{pm_platform}' — no immediate sync implemented yet.")

    return 0


if __name__ == "__main__":
    parser = argparse.ArgumentParser(prog="log_work", add_help=False)
    parser.add_argument("--commit",  default="")
    parser.add_argument("--message", default="")
    parser.add_argument("--branch",  default="")
    parser.add_argument("--repo",    default=".")
    parser.add_argument("--time",    default="", dest="time_spent")
    parser.add_argument("--ticket",  default="")
    args, _ = parser.parse_known_args()

    sys.exit(asyncio.run(run(
        commit=args.commit,
        message=args.message,
        branch=args.branch,
        repo=args.repo,
        time_spent=args.time_spent,
        ticket=args.ticket,
    )))
