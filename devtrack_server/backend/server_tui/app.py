"""
DevTrack Server TUI — Textual-based process monitor.

Usage:
    python -m backend.server_tui
    python -m backend.server_tui --help
"""
from __future__ import annotations

import asyncio
from pathlib import Path
from typing import Optional

from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.reactive import reactive
from textual.widgets import (
    DataTable,
    Footer,
    Header,
    Label,
    RichLog,
    Static,
)

from backend.server_tui.health_client import ServiceHealth, check_all
from backend.server_tui.log_viewer import LogTailer, available_logs, tail
from backend.server_tui.process_monitor import ProcessInfo, ProcessMonitor
from backend.server_tui.stats_client import TriggerStats, get_trigger_stats

# Refresh intervals (seconds)
PROCESS_REFRESH = 3.0
HEALTH_REFRESH = 10.0
LOG_REFRESH = 1.5
STATS_REFRESH = 15.0


def _status_cell(info: ProcessInfo) -> str:
    if info.running:
        return f"[green]● running[/green]"
    return "[red]○ stopped[/red]"


def _health_line(results: list[ServiceHealth]) -> str:
    parts = []
    for r in results:
        icon = "[green]✓[/green]" if r.ok else "[red]✗[/red]"
        parts.append(f"{icon} {r.name}")
    return "  ".join(parts)


class StatsBar(Static):
    """One-line summary of service health shown above the process table."""

    health: reactive[list[ServiceHealth]] = reactive([], recompose=False)

    def render(self) -> str:
        if not self.health:
            return " Checking services…"
        return " " + _health_line(self.health)


class StatsRow(Static):
    """One-line trigger throughput stats shown between the health bar and process table."""

    stats: reactive[Optional[TriggerStats]] = reactive(None, recompose=False)

    def render(self) -> str:
        s = self.stats
        if s is None:
            return " Loading stats…"
        errors_color = "red" if s.errors_24h > 0 else "green"
        errors_part = (
            f"[{errors_color}]Errors (24h): {s.errors_24h}[/{errors_color}]"
        )
        return (
            f" Triggers today: {s.triggers_today}"
            f"  |  Commits: {s.commits_today}"
            f"  |  Last: {s.last_trigger}"
            f"  |  {errors_part}"
        )


class ServerTUI(App[None]):
    """DevTrack Server — Process Monitor."""

    TITLE = "DevTrack Server"
    SUB_TITLE = "Process Monitor"
    CSS = """
    Screen {
        layout: vertical;
    }
    #top-pane {
        height: 1fr;
        layout: vertical;
    }
    StatsBar {
        height: 1;
        background: $panel;
        color: $text;
        padding: 0 1;
    }
    StatsRow {
        height: 1;
        background: $panel-darken-1;
        color: $text-muted;
        padding: 0 1;
    }
    DataTable {
        height: 1fr;
        border: solid $panel-darken-2;
    }
    #log-pane {
        height: 12;
        border: solid $panel-darken-2;
    }
    #log-label {
        height: 1;
        background: $panel-darken-1;
        padding: 0 1;
        color: $text-muted;
    }
    RichLog {
        height: 1fr;
        scrollbar-gutter: stable;
    }
    """

    BINDINGS = [
        Binding("r", "restart_selected", "Restart"),
        Binding("s", "start_selected",   "Start"),
        Binding("x", "stop_selected",    "Stop"),
        Binding("l", "toggle_log",       "Logs"),
        Binding("q", "quit",             "Quit"),
        Binding("up,k",   "cursor_up",   "Up",   show=False),
        Binding("down,j", "cursor_down", "Down", show=False),
    ]

    def __init__(self) -> None:
        super().__init__()
        self._monitor = ProcessMonitor()
        self._health: list[ServiceHealth] = []
        self._log_tailer: Optional[LogTailer] = None
        self._log_name: str = ""
        self._log_visible: bool = True
        self._available_logs: dict[str, Path] = {}

    # ------------------------------------------------------------------
    # Layout
    # ------------------------------------------------------------------

    def compose(self) -> ComposeResult:
        yield Header()
        with Vertical(id="top-pane"):
            yield StatsBar(id="stats-bar")
            yield StatsRow(id="stats-row")
            yield DataTable(id="proc-table", cursor_type="row", show_cursor=True)
        with Vertical(id="log-pane"):
            yield Label("", id="log-label")
            yield RichLog(id="log-view", highlight=True, markup=True, wrap=True,
                          auto_scroll=True)
        yield Footer()

    def on_mount(self) -> None:
        table: DataTable = self.query_one("#proc-table", DataTable)
        table.add_columns("Process", "Status", "PID", "CPU%", "MEM (MB)")
        self._populate_table()
        self._available_logs = available_logs()
        # Timers
        self.set_interval(PROCESS_REFRESH, self._refresh_processes)
        self.set_interval(HEALTH_REFRESH,  self._refresh_health)
        self.set_interval(LOG_REFRESH,     self._refresh_log)
        self.set_interval(STATS_REFRESH,   self._refresh_stats)
        # Initial async checks
        asyncio.get_event_loop().call_soon(self._schedule_health)
        asyncio.get_event_loop().call_soon(self._refresh_stats)

    def _schedule_health(self) -> None:
        self.run_worker(self._async_health_check, exclusive=False)

    # ------------------------------------------------------------------
    # Data refresh
    # ------------------------------------------------------------------

    def _populate_table(self) -> None:
        """Rebuild table rows from current monitor state."""
        self._monitor.refresh()
        table: DataTable = self.query_one("#proc-table", DataTable)
        table.clear()
        for info in self._monitor.processes:
            pid_str = str(info.pid) if info.pid else "—"
            cpu_str = f"{info.cpu_percent:.1f}" if info.running else "—"
            mem_str = f"{info.mem_mb:.1f}" if info.running else "—"
            table.add_row(
                info.name,
                _status_cell(info),
                pid_str,
                cpu_str,
                mem_str,
                key=info.name,
            )

    def _refresh_processes(self) -> None:
        self._populate_table()

    async def _async_health_check(self) -> None:
        loop = asyncio.get_event_loop()
        results = await loop.run_in_executor(None, check_all)
        self._health = results
        bar: StatsBar = self.query_one("#stats-bar", StatsBar)
        bar.health = results
        bar.refresh()

    def _refresh_health(self) -> None:
        self.run_worker(self._async_health_check, exclusive=False)

    def _refresh_stats(self) -> None:
        """Update the StatsRow widget with the latest trigger throughput stats."""
        self.run_worker(self._async_stats_check, exclusive=False)

    async def _async_stats_check(self) -> None:
        loop = asyncio.get_event_loop()
        stats = await loop.run_in_executor(None, get_trigger_stats)
        row: StatsRow = self.query_one("#stats-row", StatsRow)
        row.stats = stats
        row.refresh()

    def _refresh_log(self) -> None:
        if self._log_tailer is None:
            return
        new_lines = self._log_tailer.read_new()
        if new_lines:
            log_view: RichLog = self.query_one("#log-view", RichLog)
            for line in new_lines:
                log_view.write(line)

    # ------------------------------------------------------------------
    # Log pane helpers
    # ------------------------------------------------------------------

    def _selected_process_name(self) -> Optional[str]:
        table: DataTable = self.query_one("#proc-table", DataTable)
        if table.cursor_row < 0:
            return None
        procs = self._monitor.processes
        if table.cursor_row >= len(procs):
            return None
        return procs[table.cursor_row].name

    def _open_log(self, name: str) -> None:
        self._available_logs = available_logs()
        path = self._available_logs.get(name)
        label: Label = self.query_one("#log-label", Label)
        log_view: RichLog = self.query_one("#log-view", RichLog)
        log_view.clear()
        if path is None:
            label.update(f" No log file found for {name}")
            self._log_tailer = None
            self._log_name = ""
            return
        label.update(f" {name} → {path}")
        self._log_name = name
        self._log_tailer = LogTailer(path)
        # Show last 100 lines on open
        for line in tail(path, lines=100):
            log_view.write(line)

    # ------------------------------------------------------------------
    # Actions
    # ------------------------------------------------------------------

    def action_cursor_up(self) -> None:
        self.query_one("#proc-table", DataTable).action_cursor_up()

    def action_cursor_down(self) -> None:
        self.query_one("#proc-table", DataTable).action_cursor_down()

    def action_toggle_log(self) -> None:
        name = self._selected_process_name()
        if name and name != self._log_name:
            self._open_log(name)
        log_pane = self.query_one("#log-pane")
        self._log_visible = not self._log_visible
        log_pane.display = self._log_visible

    def action_restart_selected(self) -> None:
        name = self._selected_process_name()
        if not name:
            return
        ok = self._monitor.restart(name)
        self._notify_action("Restart", name, ok)
        self._populate_table()

    def action_start_selected(self) -> None:
        """Start a stopped process (same as restart for a stopped one)."""
        name = self._selected_process_name()
        if not name:
            return
        info = self._monitor.get(name)
        if info and info.running:
            self.notify(f"{name} is already running", severity="warning")
            return
        ok = self._monitor.restart(name)
        self._notify_action("Start", name, ok)
        self._populate_table()

    def action_stop_selected(self) -> None:
        name = self._selected_process_name()
        if not name:
            return
        ok = self._monitor.stop(name)
        self._notify_action("Stop", name, ok)
        self._populate_table()

    def _notify_action(self, action: str, name: str, ok: bool) -> None:
        if ok:
            self.notify(f"{action} → {name}", severity="information")
        else:
            self.notify(f"{action} failed for {name}", severity="error")

    def action_quit(self) -> None:
        self.exit()


def run() -> None:
    app = ServerTUI()
    app.run()
