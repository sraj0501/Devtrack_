#!/usr/bin/env python3
"""
ticket_picker.py — Interactive TUI for linking a commit to a PM ticket.

Called as a subprocess by devtrack-git-wrapper.sh after the user accepts an
AI-enhanced commit message.  All interactive output uses curses on the terminal;
the selected ticket ID (or empty string) is written to the file given by --output.

Controls:
  ↑ / ↓  (or k / j)   navigate ticket list
  Enter                select highlighted ticket
  /                    start filtering (real-time, client-side)
  Esc (in filter)      clear filter and return to full list
  n                    create a new issue
  Esc / q              skip (no ticket linked)

Usage:
    python backend/ticket_picker.py \
        --repo-path /path/to/repo \
        --workspaces-file /path/to/workspaces.yaml \
        --commit-message "the current commit message" \
        --output /tmp/ticket_id.txt
"""

from __future__ import annotations

import argparse
import asyncio
import curses
import os
import sys
import textwrap
from pathlib import Path
from typing import Any, List, NamedTuple, Optional

# ---------------------------------------------------------------------------
# Bootstrap
# ---------------------------------------------------------------------------
PROJECT_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(PROJECT_ROOT))

from backend.config import _load_env  # noqa: E402
_load_env()

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

LIST_W_MIN   = 30      # minimum left-pane width
LIST_W_MAX   = 44      # maximum left-pane width
LIST_W_RATIO = 0.38    # fraction of terminal width used for list pane
MAX_FETCH    = 30      # max tickets to fetch per request
TITLE_TRUNC  = 36      # max title chars in list pane (truncated with …)


def _trunc(text: str, max_len: int = TITLE_TRUNC) -> str:
    return text if len(text) <= max_len else text[: max_len - 1] + "\u2026"


# ---------------------------------------------------------------------------
# Workspace lookup
# ---------------------------------------------------------------------------

class WorkspaceConfig(NamedTuple):
    pm_platform: str
    pm_project:  str


def _find_workspace(repo_path: str, workspaces_file: str) -> Optional[WorkspaceConfig]:
    ws_path = Path(workspaces_file)
    if not ws_path.exists():
        return None
    try:
        import yaml
    except ImportError:
        return None

    with ws_path.open() as fh:
        data = yaml.safe_load(fh) or {}

    canonical_repo = os.path.realpath(repo_path)
    for ws in data.get("workspaces", []):
        if os.path.realpath(ws.get("path", "")) == canonical_repo:
            return WorkspaceConfig(
                pm_platform=(ws.get("pm_platform") or "").lower(),
                pm_project=ws.get("pm_project") or "",
            )
    return None


# ---------------------------------------------------------------------------
# Ticket model
# ---------------------------------------------------------------------------

class Ticket(NamedTuple):
    display_id: str   # "#123"  written to output file
    title:      str
    status:     str
    body:       str   # full description / body text for the detail pane


# ---------------------------------------------------------------------------
# Platform adapters — fetch
# ---------------------------------------------------------------------------

async def _fetch_azure(search: Optional[str] = None) -> List[Ticket]:
    from backend.azure.client import AzureDevOpsClient
    client = AzureDevOpsClient()
    if not client.is_configured():
        return []
    try:
        if search:
            items = await client.search_work_items(search, max_results=MAX_FETCH)
        else:
            items = await client.get_my_work_items(
                states=["New", "Active", "In Progress", "Committed", "Doing"],
                max_results=MAX_FETCH,
            )
        return [Ticket(f"#{i.id}", i.title, i.state, i.description or "") for i in items]
    finally:
        await client.close()


async def _fetch_github(search: Optional[str] = None) -> List[Ticket]:
    from backend.github.client import GitHubClient
    client = GitHubClient()
    if not client.is_configured():
        return []
    try:
        if search:
            items = await client.search_issues(search, max_results=MAX_FETCH)
        else:
            items = await client.get_my_issues(state="open")
        return [Ticket(f"#{i.number}", i.title, i.state, i.body or "") for i in items]
    finally:
        await client.close()


async def _fetch_gitlab(search: Optional[str] = None) -> List[Ticket]:
    from backend.gitlab.client import GitLabClient
    client = GitLabClient()
    if not client.is_configured():
        return []
    try:
        if search:
            items = await client.search_issues(search, max_results=MAX_FETCH)
        else:
            items = await client.get_my_issues(state="opened")
        return [Ticket(f"#{i.iid}", i.title, i.state, i.description or "") for i in items]
    finally:
        await client.close()


async def _fetch(platform: str, search: Optional[str] = None) -> List[Ticket]:
    if platform == "azure":
        return await _fetch_azure(search)
    if platform == "github":
        return await _fetch_github(search)
    if platform == "gitlab":
        return await _fetch_gitlab(search)
    return []


# ---------------------------------------------------------------------------
# Platform adapters — create
# ---------------------------------------------------------------------------

async def _create_azure(title: str, commit_message: str = "") -> Optional[Ticket]:
    from backend.azure.client import AzureDevOpsClient
    client = AzureDevOpsClient()
    if not client.is_configured():
        return None
    try:
        item = await client.create_work_item(title=title)
        return Ticket(f"#{item.id}", item.title, item.state, item.description or "") if item else None
    finally:
        await client.close()


async def _create_github(title: str, commit_message: str = "") -> Optional[Ticket]:
    from backend.github.client import GitHubClient
    client = GitHubClient()
    if not client.is_configured():
        return None
    try:
        issue = await client.create_issue(title=title, body=commit_message)
        return Ticket(f"#{issue.number}", issue.title, issue.state, issue.body or "") if issue else None
    finally:
        await client.close()


async def _create_gitlab(title: str, commit_message: str = "") -> Optional[Ticket]:
    from backend.gitlab.client import GitLabClient
    client = GitLabClient()
    if not client.is_configured():
        return None
    try:
        issue = await client.create_issue(title=title)
        return Ticket(f"#{issue.iid}", issue.title, issue.state, issue.description or "") if issue else None
    finally:
        await client.close()


async def _create(platform: str, title: str, commit_message: str = "") -> Optional[Ticket]:
    if platform == "azure":
        return await _create_azure(title, commit_message)
    if platform == "github":
        return await _create_github(title, commit_message)
    if platform == "gitlab":
        return await _create_gitlab(title, commit_message)
    return None


# ---------------------------------------------------------------------------
# Curses color pairs (initialised inside curses.wrapper)
# ---------------------------------------------------------------------------

CP_SELECTED  = 1   # highlighted list row
CP_ACCENT    = 2   # headers, separators
CP_STATUS_OK = 3   # Open / Active labels
CP_STATUS_WA = 4   # Closed / Done labels
CP_SEARCH    = 5   # search / new-issue input bar
CP_DETAIL_H  = 6   # detail pane title


def _init_colors() -> None:
    curses.use_default_colors()
    curses.init_pair(CP_SELECTED,  curses.COLOR_BLACK,  curses.COLOR_CYAN)
    curses.init_pair(CP_ACCENT,    curses.COLOR_CYAN,   -1)
    curses.init_pair(CP_STATUS_OK, curses.COLOR_GREEN,  -1)
    curses.init_pair(CP_STATUS_WA, curses.COLOR_YELLOW, -1)
    curses.init_pair(CP_SEARCH,    curses.COLOR_WHITE,  curses.COLOR_BLUE)
    curses.init_pair(CP_DETAIL_H,  curses.COLOR_WHITE,  -1)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _wrap_body(body: str, width: int) -> List[str]:
    """Word-wrap body text into lines of *width* chars, preserving blank lines."""
    lines: List[str] = []
    for para in body.splitlines():
        stripped = para.strip()
        if not stripped:
            lines.append("")
        else:
            wrapped = textwrap.wrap(stripped, width=max(1, width))
            lines.extend(wrapped or [""])
    return lines


def _status_color(status: str) -> int:
    s = status.lower()
    if any(x in s for x in ("open", "active", "new", "doing", "in progress")):
        return curses.color_pair(CP_STATUS_OK)
    return curses.color_pair(CP_STATUS_WA)


# ---------------------------------------------------------------------------
# Main curses loop
# ---------------------------------------------------------------------------

def _curses_picker(
    stdscr,
    all_tickets: List[Ticket],
    platform: str,
    pm_project: str,
    commit_message: str,
) -> str:
    """Full split-pane ticket picker.  Returns selected display_id or ''."""

    curses.curs_set(0)
    stdscr.keypad(True)
    stdscr.timeout(50)   # non-blocking getch so we can redraw on resize

    if curses.has_colors():
        _init_colors()

    # Aliases
    SEL      = curses.color_pair(CP_SELECTED) | curses.A_BOLD
    ACCENT   = curses.color_pair(CP_ACCENT)
    SRCH     = curses.color_pair(CP_SEARCH)
    BOLD     = curses.A_BOLD
    DIM      = curses.A_DIM

    # Mutable state
    filtered:    List[Ticket] = list(all_tickets)
    cursor:      int          = 0
    scroll_top:  int          = 0
    filter_mode: bool         = False
    filter_buf:  str          = ""
    new_mode:    bool         = False
    new_buf:     str          = ""
    status_msg:  str          = ""

    def _apply_filter(buf: str) -> None:
        nonlocal filtered, cursor, scroll_top
        if buf:
            q = buf.lower()
            filtered = [t for t in all_tickets if q in t.title.lower() or q in t.display_id.lower()]
        else:
            filtered = list(all_tickets)
        cursor     = 0
        scroll_top = 0

    def _clamp() -> None:
        nonlocal cursor, scroll_top
        if not filtered:
            cursor = 0
            scroll_top = 0
            return
        cursor = max(0, min(cursor, len(filtered) - 1))
        if cursor < scroll_top:
            scroll_top = cursor
        if cursor >= scroll_top + list_rows:
            scroll_top = cursor - list_rows + 1

    while True:
        rows, cols = stdscr.getmaxyx()

        # ── Layout ────────────────────────────────────────────────────────
        list_w   = max(LIST_W_MIN, min(LIST_W_MAX, int(cols * LIST_W_RATIO)))
        div_x    = list_w          # column of │ divider
        det_x    = list_w + 1     # detail pane start
        det_w    = max(1, cols - det_x - 1)

        HDR_ROWS = 3               # header lines (sep + title + sep)
        FTR_ROWS = 2               # footer lines (input bar + hint)
        list_rows = max(1, rows - HDR_ROWS - FTR_ROWS)
        det_rows  = list_rows

        stdscr.erase()

        # ── Header ────────────────────────────────────────────────────────
        sep = "─" * (cols - 1)
        try:
            stdscr.addstr(0, 0, sep, ACCENT)
            title_str = f"  \U0001f3ab  Link commit to a ticket   {platform}"
            if pm_project:
                title_str += f" · {pm_project}"
            stdscr.addstr(1, 0, title_str[:cols - 1])
            stdscr.addstr(2, 0, sep, ACCENT)
        except curses.error:
            pass

        # ── Vertical divider ──────────────────────────────────────────────
        for r in range(HDR_ROWS, rows - FTR_ROWS):
            try:
                stdscr.addch(r, div_x, "│", ACCENT)
            except curses.error:
                pass

        # ── List column header ────────────────────────────────────────────
        col_hdr = f"  {'ID':<8} {'Title'}"
        try:
            stdscr.addstr(HDR_ROWS, 0, col_hdr[:list_w], ACCENT | BOLD)
        except curses.error:
            pass

        # ── Ticket list ───────────────────────────────────────────────────
        visible = filtered[scroll_top : scroll_top + list_rows - 1]
        for i, t in enumerate(visible):
            r      = HDR_ROWS + 1 + i
            abs_i  = scroll_top + i
            is_sel = abs_i == cursor

            marker = "▶ " if is_sel else "  "
            id_str = t.display_id[:7]
            title_str = _trunc(t.title, list_w - 12)
            line = f"{marker}{id_str:<7} {title_str}"

            try:
                if is_sel:
                    stdscr.addstr(r, 0, line[:list_w], SEL)
                else:
                    stdscr.addstr(r, 0, line[:list_w])
            except curses.error:
                pass

        # Empty-list notice
        if not filtered:
            try:
                msg = "  No tickets" + (" matched" if filter_buf else " found")
                stdscr.addstr(HDR_ROWS + 2, 0, msg[:list_w], DIM)
            except curses.error:
                pass

        # Scroll indicator (bottom-left corner of list)
        if len(filtered) > list_rows - 1:
            scroll_info = f" {cursor + 1}/{len(filtered)}"
            try:
                stdscr.addstr(rows - FTR_ROWS - 1, 0, scroll_info[:list_w], ACCENT)
            except curses.error:
                pass

        # ── Detail pane ───────────────────────────────────────────────────
        if filtered and 0 <= cursor < len(filtered):
            t = filtered[cursor]

            # Title
            det_title = f" {t.display_id}: {t.title}"
            try:
                stdscr.addstr(HDR_ROWS, det_x, det_title[:det_w], BOLD)
            except curses.error:
                pass

            # Status
            try:
                stdscr.addstr(HDR_ROWS + 1, det_x,
                               f" {t.status}"[:det_w],
                               _status_color(t.status))
            except curses.error:
                pass

            # Separator
            try:
                stdscr.addstr(HDR_ROWS + 2, det_x, " " + "─" * (det_w - 2), ACCENT)
            except curses.error:
                pass

            # Body text
            body_text = t.body.strip() or "(no description)"
            body_lines = _wrap_body(body_text, det_w - 2)
            avail = det_rows - 3   # rows below title/status/sep

            for i, ln in enumerate(body_lines[:avail]):
                try:
                    stdscr.addstr(HDR_ROWS + 3 + i, det_x, f" {ln}"[:det_w])
                except curses.error:
                    pass

            # Overflow notice
            if len(body_lines) > avail:
                try:
                    stdscr.addstr(rows - FTR_ROWS - 1, det_x,
                                   f" … {len(body_lines) - avail} more lines"[:det_w],
                                   DIM)
                except curses.error:
                    pass

        # ── Footer ────────────────────────────────────────────────────────
        ftr_r = rows - FTR_ROWS
        if filter_mode:
            bar = f" / {filter_buf}_"
            try:
                stdscr.addstr(ftr_r, 0, bar[:cols - 1], SRCH)
                stdscr.addstr(ftr_r, len(bar), " " * (cols - len(bar) - 1), SRCH)
            except curses.error:
                pass
            hint = "  Type to filter   Esc clear   Enter done"
        elif new_mode:
            bar = f" + {new_buf}_"
            try:
                stdscr.addstr(ftr_r, 0, bar[:cols - 1], SRCH)
                stdscr.addstr(ftr_r, len(bar), " " * (cols - len(bar) - 1), SRCH)
            except curses.error:
                pass
            hint = "  Enter: create issue   Esc: cancel"
        else:
            hint = "  \u2191\u2193 navigate   Enter select   / filter   n new   Esc skip"

        if status_msg:
            try:
                stdscr.addstr(ftr_r + 1, 0, f"  {status_msg}"[:cols - 1],
                               curses.color_pair(CP_STATUS_WA))
            except curses.error:
                pass
        else:
            try:
                stdscr.addstr(ftr_r + 1, 0, hint[:cols - 1], ACCENT)
            except curses.error:
                pass

        stdscr.refresh()
        status_msg = ""

        # ── Input ─────────────────────────────────────────────────────────
        key = stdscr.getch()
        if key == curses.ERR:
            continue

        # ── Filter mode ───────────────────────────────────────────────────
        if filter_mode:
            if key in (27,):                          # Esc → clear filter
                filter_mode = False
                filter_buf  = ""
                _apply_filter("")
            elif key in (curses.KEY_ENTER, 10, 13):  # Enter → done
                filter_mode = False
            elif key in (curses.KEY_BACKSPACE, 127, 8):
                filter_buf = filter_buf[:-1]
                _apply_filter(filter_buf)
            elif 32 <= key <= 126:
                filter_buf += chr(key)
                _apply_filter(filter_buf)
            _clamp()
            continue

        # ── New-issue mode ────────────────────────────────────────────────
        if new_mode:
            if key in (27,):                          # Esc → cancel
                new_mode = False
                new_buf  = ""
            elif key in (curses.KEY_ENTER, 10, 13):
                new_mode = False
                title    = new_buf.strip() or (commit_message.splitlines()[0] if commit_message else "Untitled")
                new_buf  = ""
                status_msg = "Creating issue…"
                stdscr.addstr(ftr_r + 1, 0, f"  {status_msg}"[:cols - 1])
                stdscr.refresh()
                try:
                    created = asyncio.run(_create(platform, title, commit_message))
                    if created:
                        return created.display_id
                    status_msg = "Issue creation failed — skipping."
                except Exception as exc:
                    status_msg = f"Error: {exc}"
            elif key in (curses.KEY_BACKSPACE, 127, 8):
                new_buf = new_buf[:-1]
            elif 32 <= key <= 126:
                new_buf += chr(key)
            continue

        # ── Normal navigation ─────────────────────────────────────────────
        if key in (curses.KEY_UP, ord("k")):
            cursor -= 1
        elif key in (curses.KEY_DOWN, ord("j")):
            cursor += 1
        elif key == curses.KEY_HOME:
            cursor = 0
        elif key == curses.KEY_END:
            cursor = max(0, len(filtered) - 1)
        elif key == curses.KEY_PPAGE:
            cursor    = max(0, cursor - (list_rows - 2))
            scroll_top = max(0, scroll_top - (list_rows - 2))
        elif key == curses.KEY_NPAGE:
            cursor    = min(len(filtered) - 1, cursor + (list_rows - 2))
        elif key in (curses.KEY_ENTER, 10, 13):
            if filtered and 0 <= cursor < len(filtered):
                return filtered[cursor].display_id
        elif key == ord("/"):
            filter_mode = True
            filter_buf  = ""
        elif key == ord("n"):
            new_mode = True
            new_buf  = ""
        elif key in (27, ord("q")):   # Esc / q → skip
            return ""

        _clamp()

    return ""


# ---------------------------------------------------------------------------
# Fallback (non-curses) for environments without a real TTY
# ---------------------------------------------------------------------------

def _print(msg: str = "") -> None:
    print(msg, file=sys.stderr)

def _input(prompt: str) -> str:
    sys.stderr.write(prompt)
    sys.stderr.flush()
    return sys.stdin.readline().rstrip("\n")

def _run_tui_fallback(
    platform: str,
    pm_project: str,
    commit_message: str,
    tickets: List[Ticket],
) -> str:
    """Simple numbered-list fallback when curses is unavailable."""
    sep = "─" * 62
    _print(f"\n{sep}")
    _print(f"\U0001f3ab  Link commit to a ticket  ({platform} · {pm_project or 'default'})")
    _print(sep)

    if not tickets:
        _print("  No assigned tickets found.")
    else:
        _print(f"\n  {'#':<3}  {'ID':<8}  Title")
        _print(f"  {'─'*3}  {'─'*8}  {'─'*44}")
        for idx, t in enumerate(tickets, 1):
            _print(f"  {idx:<3}  {t.display_id:<8}  {_trunc(t.title, 44)}")
        _print()

    while True:
        raw = _input("[number] select  [n] new  Enter skip\n> ")
        if raw == "":
            return ""
        if raw.strip().lower() == "n":
            default = commit_message.splitlines()[0] if commit_message else ""
            title = _input(f"  New issue title [{default}]: ").strip() or default or "Untitled"
            _print("  Creating…")
            try:
                created = asyncio.run(_create(platform, title, commit_message))
                if created:
                    _print(f"  \u2713 Created {created.display_id}: {created.title}")
                    return created.display_id
                _print("  Failed — skipping ticket link.")
                return ""
            except Exception as exc:
                _print(f"  Error: {exc}")
                return ""
        try:
            idx = int(raw.strip())
            if 1 <= idx <= len(tickets):
                sel = tickets[idx - 1]
                _print(f"  \u2713 Selected {sel.display_id}: {_trunc(sel.title)}")
                return sel.display_id
            _print(f"  Out of range (1–{len(tickets)}).")
        except ValueError:
            _print("  Enter a number, 'n', or press Enter to skip.")


# ---------------------------------------------------------------------------
# Main TUI dispatcher
# ---------------------------------------------------------------------------

def _run_tui(platform: str, pm_project: str, commit_message: str) -> str:
    # Fetch tickets (with a brief loading indicator)
    sys.stderr.write("  Fetching tickets\u2026")
    sys.stderr.flush()
    try:
        tickets = asyncio.run(_fetch(platform, None))
    except Exception as exc:
        sys.stderr.write(f"\r  Error: {exc}\n")
        sys.stderr.flush()
        return ""
    sys.stderr.write("\r" + " " * 30 + "\r")
    sys.stderr.flush()

    if not sys.stdin.isatty() or not sys.stdout.isatty():
        return _run_tui_fallback(platform, pm_project, commit_message, tickets)

    result: List[str] = [""]

    def _curses_main(stdscr):
        result[0] = _curses_picker(stdscr, tickets, platform, pm_project, commit_message)

    try:
        curses.wrapper(_curses_main)
    except Exception as exc:
        # Curses can fail on some terminals — fall back gracefully
        sys.stderr.write(f"\r  (falling back: {exc})\n")
        sys.stderr.flush()
        return _run_tui_fallback(platform, pm_project, commit_message, tickets)

    return result[0]


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main() -> None:
    parser = argparse.ArgumentParser(description="Interactive ticket picker for devtrack git commit")
    parser.add_argument("--repo-path",        required=True)
    parser.add_argument("--workspaces-file",  required=True)
    parser.add_argument("--commit-message",   default="")
    parser.add_argument("--output",           required=True)
    args = parser.parse_args()

    output_path = Path(args.output)

    def _write(ticket_id: str) -> None:
        output_path.write_text(ticket_id)

    ws = _find_workspace(args.repo_path, args.workspaces_file)
    if ws is None:
        _print("\u2139\ufe0f  Repo not in workspaces.yaml — skipping ticket link")
        _write("")
        return

    platform = ws.pm_platform
    if not platform or platform in ("none", "jira"):
        _write("")
        return

    if platform not in ("azure", "github", "gitlab"):
        _print(f"\u2139\ufe0f  Unknown PM platform '{platform}' — skipping")
        _write("")
        return

    try:
        ticket_id = _run_tui(platform, ws.pm_project, args.commit_message)
        _write(ticket_id)
    except KeyboardInterrupt:
        _print("\n  Skipping ticket link.")
        _write("")
    except Exception as exc:
        _print(f"\n  [ticket_picker] Error: {exc} — skipping")
        _write("")


if __name__ == "__main__":
    main()
