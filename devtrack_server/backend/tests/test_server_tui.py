"""
Tests for backend.server_tui non-Textual helpers.

All tests are headless — no Textual app is started.  Tests are written
Linux-first: no macOS-specific paths, process names, signal names, or
service-management references anywhere in this file.

Modules under test:
  - backend.server_tui.process_monitor  (ProcessMonitor)
  - backend.server_tui.log_viewer       (tail, LogTailer)
  - backend.server_tui.stats_client     (get_trigger_stats, _query_stats)
  - backend.server_tui.health_client    (check_all)
"""
from __future__ import annotations

import sqlite3
from datetime import datetime, timedelta, timezone
from pathlib import Path
from unittest.mock import MagicMock, call, patch

import psutil
import pytest


# ---------------------------------------------------------------------------
# process_monitor
# ---------------------------------------------------------------------------

class TestProcessMonitorRefresh:
    """ProcessMonitor.refresh() — psutil scanning logic."""

    def _make_fake_proc(self, pid, cmdline, status="sleeping",
                        cpu_percent=0.5, mem_rss=1024 * 1024 * 10):
        """Return a MagicMock that looks like a psutil.Process with .info."""
        mem = MagicMock()
        mem.rss = mem_rss
        proc = MagicMock()
        proc.info = {
            "pid": pid,
            "name": "python3",
            "cmdline": cmdline,
            "status": status,
            "cpu_percent": cpu_percent,
            "memory_info": mem,
        }
        return proc

    def test_refresh_matches_webhook_server(self):
        """A process with 'webhook_server' in its cmdline should be detected."""
        from backend.server_tui.process_monitor import ProcessMonitor

        fake = self._make_fake_proc(
            pid=1234,
            cmdline=["python3", "-m", "uvicorn", "backend.webhook_server:app"],
            status="sleeping",
        )
        with patch("psutil.process_iter", return_value=[fake]):
            mon = ProcessMonitor()
            mon.refresh()

        info = mon.get("webhook_server")
        assert info is not None
        assert info.pid == 1234
        assert info.status == "sleeping"
        assert info.running is True
        assert info.mem_mb == pytest.approx(10.0, abs=0.1)

    def test_refresh_unmatched_process_leaves_stopped(self):
        """A process that matches no pattern must leave all entries as stopped."""
        from backend.server_tui.process_monitor import ProcessMonitor

        fake = self._make_fake_proc(
            pid=9999,
            cmdline=["bash", "/usr/local/bin/some-unrelated-script"],
        )
        with patch("psutil.process_iter", return_value=[fake]):
            mon = ProcessMonitor()
            mon.refresh()

        for proc_info in mon.processes:
            assert proc_info.pid is None
            assert proc_info.status == "stopped"
            assert proc_info.running is False

    def test_refresh_empty_cmdline_skipped(self):
        """Processes with empty cmdline must be skipped without error."""
        from backend.server_tui.process_monitor import ProcessMonitor

        fake = self._make_fake_proc(pid=111, cmdline=[])
        with patch("psutil.process_iter", return_value=[fake]):
            mon = ProcessMonitor()
            mon.refresh()  # should not raise

        for proc_info in mon.processes:
            assert proc_info.status == "stopped"

    def test_refresh_access_denied_does_not_raise(self):
        """AccessDenied raised while accessing proc.info must be swallowed.

        The production code catches (NoSuchProcess, AccessDenied) inside the
        loop body when accessing proc.info.  We simulate that by returning a
        proc whose .info property raises AccessDenied on access.
        """
        from backend.server_tui.process_monitor import ProcessMonitor

        bad_proc = MagicMock()
        # Make the 'info' attribute raise AccessDenied when read
        type(bad_proc).info = property(lambda self: (_ for _ in ()).throw(
            psutil.AccessDenied(pid=0)
        ))

        with patch("psutil.process_iter", return_value=[bad_proc]):
            mon = ProcessMonitor()
            mon.refresh()  # must not raise

        for proc_info in mon.processes:
            assert proc_info.status == "stopped"

    def test_refresh_no_such_process_skipped(self):
        """NoSuchProcess raised while accessing proc.info must be swallowed."""
        from backend.server_tui.process_monitor import ProcessMonitor

        good = self._make_fake_proc(
            pid=200, cmdline=["python3", "-m", "uvicorn", "backend.webhook_server:app"], status="running"
        )
        # bad_proc raises NoSuchProcess when .info is accessed
        bad_proc = MagicMock()
        type(bad_proc).info = property(lambda self: (_ for _ in ()).throw(
            psutil.NoSuchProcess(pid=300)
        ))

        with patch("psutil.process_iter", return_value=[good, bad_proc]):
            mon = ProcessMonitor()
            mon.refresh()

        # webhook_server was detected from the good process
        assert mon.get("webhook_server").pid == 200


class TestProcessMonitorRestart:
    """ProcessMonitor.restart() — kill-then-spawn logic."""

    def test_restart_happy_path_with_existing_pid(self):
        """With a live pid, terminate is called then Popen spawns the restart cmd."""
        from backend.server_tui.process_monitor import ProcessMonitor

        mon = ProcessMonitor()
        info = mon.get("webhook_server")
        info.pid = 5678

        mock_proc = MagicMock()
        mock_proc.wait.return_value = None

        with patch("psutil.Process", return_value=mock_proc) as mock_psutil_proc, \
             patch("subprocess.Popen") as mock_popen:
            result = mon.restart("webhook_server")

        assert result is True
        mock_psutil_proc.assert_called_once_with(5678)
        mock_proc.terminate.assert_called_once()
        mock_popen.assert_called_once()
        popen_args = mock_popen.call_args[0][0]
        assert any("webhook_server" in arg for arg in popen_args)

    def test_restart_no_pid_skips_kill_still_spawns(self):
        """When pid is None the kill step is skipped but Popen is still called."""
        from backend.server_tui.process_monitor import ProcessMonitor

        mon = ProcessMonitor()
        # pid is None by default

        with patch("psutil.Process") as mock_psutil_proc, \
             patch("subprocess.Popen") as mock_popen:
            result = mon.restart("webhook_server")

        assert result is True
        mock_psutil_proc.assert_not_called()
        mock_popen.assert_called_once()

    def test_restart_unknown_name_returns_false(self):
        """Restarting an unregistered name returns False and never calls Popen."""
        from backend.server_tui.process_monitor import ProcessMonitor

        mon = ProcessMonitor()
        with patch("subprocess.Popen") as mock_popen:
            result = mon.restart("nonexistent_service")

        assert result is False
        mock_popen.assert_not_called()

    def test_restart_popen_exception_returns_false(self):
        """If Popen raises, restart() returns False gracefully."""
        from backend.server_tui.process_monitor import ProcessMonitor

        mon = ProcessMonitor()
        with patch("subprocess.Popen", side_effect=OSError("spawn failed")):
            result = mon.restart("webhook_server")

        assert result is False


class TestProcessMonitorStop:
    """ProcessMonitor.stop() — terminate-by-name logic."""

    def test_stop_happy_path(self):
        """stop() terminates the process and returns True."""
        from backend.server_tui.process_monitor import ProcessMonitor

        mon = ProcessMonitor()
        mon.get("webhook_server").pid = 4321

        mock_proc = MagicMock()
        mock_proc.wait.return_value = None

        with patch("psutil.Process", return_value=mock_proc) as mock_psutil_proc:
            result = mon.stop("webhook_server")

        assert result is True
        mock_psutil_proc.assert_called_once_with(4321)
        mock_proc.terminate.assert_called_once()

    def test_stop_no_pid_returns_false(self):
        """stop() on a process with no pid returns False without calling psutil."""
        from backend.server_tui.process_monitor import ProcessMonitor

        mon = ProcessMonitor()
        # pid is None by default

        with patch("psutil.Process") as mock_psutil_proc:
            result = mon.stop("webhook_server")

        assert result is False
        mock_psutil_proc.assert_not_called()

    def test_stop_unknown_name_returns_false(self):
        """stop() with an unregistered name returns False."""
        from backend.server_tui.process_monitor import ProcessMonitor

        mon = ProcessMonitor()
        result = mon.stop("does_not_exist")
        assert result is False

    def test_stop_no_such_process_returns_false(self):
        """If psutil raises NoSuchProcess, stop() returns False without raising."""
        from backend.server_tui.process_monitor import ProcessMonitor

        mon = ProcessMonitor()
        mon.get("webhook_server").pid = 9001

        with patch("psutil.Process", side_effect=psutil.NoSuchProcess(pid=9001)):
            result = mon.stop("webhook_server")

        assert result is False


# ---------------------------------------------------------------------------
# log_viewer
# ---------------------------------------------------------------------------

class TestTail:
    """log_viewer.tail() — efficient last-N-lines from file."""

    def test_tail_last_n_lines(self, tmp_path):
        """tail() returns the last N lines from a file."""
        from backend.server_tui.log_viewer import tail

        log = tmp_path / "test.log"
        lines = [f"line {i}\n" for i in range(10)]
        log.write_text("".join(lines), encoding="utf-8")

        result = tail(log, lines=3)
        assert result == ["line 7", "line 8", "line 9"]

    def test_tail_large_file_correct_count(self, tmp_path):
        """tail() on a 300-line file returns exactly 10 lines when asked."""
        from backend.server_tui.log_viewer import tail

        log = tmp_path / "big.log"
        content = "".join(f"entry {i}\n" for i in range(300))
        log.write_text(content, encoding="utf-8")

        result = tail(log, lines=10)
        assert len(result) == 10
        assert result[-1] == "entry 299"

    def test_tail_nonexistent_returns_empty(self, tmp_path):
        """tail() on a missing file returns [] without raising."""
        from backend.server_tui.log_viewer import tail

        missing = tmp_path / "no_such.log"
        result = tail(missing, lines=5)
        assert result == []

    def test_tail_entire_file_when_fewer_lines_than_requested(self, tmp_path):
        """tail() returns all lines when file has fewer lines than requested."""
        from backend.server_tui.log_viewer import tail

        log = tmp_path / "short.log"
        log.write_text("alpha\nbeta\n", encoding="utf-8")

        result = tail(log, lines=50)
        assert result == ["alpha", "beta"]


class TestLogTailer:
    """LogTailer.read_new() — incremental log reading."""

    def test_read_new_returns_appended_lines(self, tmp_path):
        """read_new() returns lines appended after the tailer was created."""
        from backend.server_tui.log_viewer import LogTailer

        log = tmp_path / "service.log"
        log.write_text("existing line\n", encoding="utf-8")

        tailer = LogTailer(log)  # offset set to end of "existing line\n"

        log.open("a", encoding="utf-8").write("new line 1\nnew line 2\n")

        result = tailer.read_new()
        assert result == ["new line 1", "new line 2"]

    def test_read_new_no_change_returns_empty(self, tmp_path):
        """Calling read_new() twice with no new data returns [] on second call."""
        from backend.server_tui.log_viewer import LogTailer

        log = tmp_path / "service.log"
        log.write_text("static content\n", encoding="utf-8")

        tailer = LogTailer(log)
        # First call after creation: file has not grown past offset (offset=EOF)
        assert tailer.read_new() == []
        # Second call: still no change
        assert tailer.read_new() == []

    def test_read_new_truncation_resets_offset(self, tmp_path):
        """When file shrinks (rotation), offset resets and new content is returned."""
        from backend.server_tui.log_viewer import LogTailer

        log = tmp_path / "rotating.log"
        # Write initial content so offset is past 0
        log.write_text("old content line 1\nold content line 2\n", encoding="utf-8")

        tailer = LogTailer(log)
        # Simulate log rotation: truncate and write shorter content
        log.write_text("after rotation\n", encoding="utf-8")

        result = tailer.read_new()
        assert "after rotation" in result

    def test_read_new_empty_file_at_start(self, tmp_path):
        """LogTailer on an empty file starts at offset 0 and reads new appends."""
        from backend.server_tui.log_viewer import LogTailer

        log = tmp_path / "fresh.log"
        log.write_text("", encoding="utf-8")

        tailer = LogTailer(log)
        log.open("a", encoding="utf-8").write("first entry\n")

        result = tailer.read_new()
        assert result == ["first entry"]


# ---------------------------------------------------------------------------
# stats_client
# ---------------------------------------------------------------------------

def _create_triggers_table(conn: sqlite3.Connection) -> None:
    conn.execute(
        """CREATE TABLE triggers (
            id           INTEGER PRIMARY KEY,
            trigger_type TEXT,
            timestamp    TEXT,
            processed    INTEGER DEFAULT 0
        )"""
    )
    conn.commit()


def _ts(dt: datetime) -> str:
    """Format a datetime as a space-separated UTC timestamp.

    The stats_client error and 24-hour cutoffs are formatted as
    "%Y-%m-%d %H:%M:%S" for SQLite string comparison.  Using the same format
    for inserted rows ensures the inequality checks work correctly.
    Separately, the timestamp-parsing tests exercise the ISO-Z and ISO formats
    by inserting literal strings.
    """
    return dt.strftime("%Y-%m-%d %H:%M:%S")


@pytest.fixture()
def isolated_stats_engine(tmp_path, monkeypatch):
    """Point DATABASE_DIR at a fresh temp directory and reset the shared
    SQLAlchemy engine singleton so stats_client's engine-based reads hit an
    isolated SQLite file instead of whatever engine a prior test built (same
    fixture shape as test_skill_detector.py's isolated_engine and
    test_voice_add_status.py's isolated_dialectic_engine).

    Yields the temp directory; the DB file itself is `devtrack.db` inside it
    (backend.config.database_path()'s default filename) — _query_stats() no
    longer takes a path argument, it resolves its own path via
    backend.db.engine.get_engine() same as production code.
    """
    from backend.db.engine import reset_engine

    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    reset_engine()
    yield tmp_path
    reset_engine()


class TestGetTriggerStats:
    """stats_client.get_trigger_stats() / _query_stats() — SQLite queries."""

    def test_happy_path_counts_today(self, isolated_stats_engine):
        """triggers_today and commits_today reflect only today's rows."""
        from backend.server_tui.stats_client import _query_stats

        db = isolated_stats_engine / "devtrack.db"
        now = datetime.now(timezone.utc)
        yesterday = now - timedelta(days=1)

        with sqlite3.connect(str(db)) as conn:
            _create_triggers_table(conn)
            # Today: 2 generic triggers, 1 commit trigger
            conn.execute("INSERT INTO triggers (trigger_type, timestamp, processed) VALUES (?, ?, 1)",
                         ("timer", _ts(now)))
            conn.execute("INSERT INTO triggers (trigger_type, timestamp, processed) VALUES (?, ?, 1)",
                         ("commit", _ts(now)))
            conn.execute("INSERT INTO triggers (trigger_type, timestamp, processed) VALUES (?, ?, 1)",
                         ("timer", _ts(now)))
            # Yesterday: should not count for today
            conn.execute("INSERT INTO triggers (trigger_type, timestamp, processed) VALUES (?, ?, 1)",
                         ("commit", _ts(yesterday)))
            conn.commit()

        stats = _query_stats()

        assert stats.triggers_today == 3
        assert stats.commits_today == 1

    def test_last_trigger_formatted_as_hhmm(self, isolated_stats_engine):
        """last_trigger is returned as HH:MM."""
        from backend.server_tui.stats_client import _query_stats

        db = isolated_stats_engine / "devtrack.db"
        now = datetime.now(timezone.utc)

        with sqlite3.connect(str(db)) as conn:
            _create_triggers_table(conn)
            conn.execute("INSERT INTO triggers (trigger_type, timestamp, processed) VALUES (?, ?, 1)",
                         ("commit", _ts(now)))
            conn.commit()

        stats = _query_stats()
        # Must be HH:MM format — 5 characters, digit:digit
        assert len(stats.last_trigger) == 5
        assert stats.last_trigger[2] == ":"

    def test_graceful_zero_when_db_absent(self, isolated_stats_engine):
        """get_trigger_stats() returns zeros when the DB file does not exist.

        isolated_stats_engine points DATABASE_DIR at a fresh tmp_path with no
        devtrack.db file in it yet — database_path() resolves to a path that
        does not exist, so _stats_from_sqlite()'s own existence pre-check
        short-circuits before the engine is ever touched. No patch of
        _db_path needed (mirrors test_voice_add_status.py's
        test_get_inference_summary_nonexistent_db_returns_safe_default).
        """
        from backend.server_tui.stats_client import get_trigger_stats

        stats = get_trigger_stats()

        assert stats.triggers_today == 0
        assert stats.commits_today == 0
        assert stats.last_trigger == "—"
        assert stats.errors_24h == 0

    def test_graceful_zero_when_table_missing(self, isolated_stats_engine):
        """get_trigger_stats() returns zeros when 'triggers' table does not exist.

        The DB file is created at the isolated_stats_engine's own
        database_path() (devtrack.db under the fixture's tmp_path) so the
        existence pre-check passes and the engine reads the same file — no
        need to patch _db_path directly since DATABASE_DIR + reset_engine()
        already make get_engine() and _db_path() agree on the same file.
        """
        from backend.server_tui.stats_client import get_trigger_stats

        db = isolated_stats_engine / "devtrack.db"
        with sqlite3.connect(str(db)) as conn:
            conn.execute("CREATE TABLE unrelated (id INTEGER PRIMARY KEY)")
            conn.commit()

        stats = get_trigger_stats()

        assert stats.triggers_today == 0
        assert stats.commits_today == 0
        assert stats.errors_24h == 0

    def test_error_counting_unprocessed_old_row(self, isolated_stats_engine):
        """Unprocessed rows older than 5 min but within 24 h count as errors."""
        from backend.server_tui.stats_client import _query_stats

        db = isolated_stats_engine / "devtrack.db"
        now = datetime.now(timezone.utc)
        # 10 minutes ago — old enough to be an error, within 24 h window
        error_ts = now - timedelta(minutes=10)

        with sqlite3.connect(str(db)) as conn:
            _create_triggers_table(conn)
            conn.execute(
                "INSERT INTO triggers (trigger_type, timestamp, processed) VALUES (?, ?, 0)",
                ("commit", _ts(error_ts)),
            )
            conn.commit()

        stats = _query_stats()
        assert stats.errors_24h == 1

    def test_recent_unprocessed_row_not_counted_as_error(self, isolated_stats_engine):
        """Unprocessed rows less than 5 min old are NOT errors yet."""
        from backend.server_tui.stats_client import _query_stats

        db = isolated_stats_engine / "devtrack.db"
        now = datetime.now(timezone.utc)
        # 2 minutes ago — too recent to be an error
        recent_ts = now - timedelta(minutes=2)

        with sqlite3.connect(str(db)) as conn:
            _create_triggers_table(conn)
            conn.execute(
                "INSERT INTO triggers (trigger_type, timestamp, processed) VALUES (?, ?, 0)",
                ("commit", _ts(recent_ts)),
            )
            conn.commit()

        stats = _query_stats()
        assert stats.errors_24h == 0

    def test_timestamp_format_iso_z(self, isolated_stats_engine):
        """Timestamp format '2006-01-02T15:04:05Z' is parsed to HH:MM."""
        from backend.server_tui.stats_client import _query_stats

        db = isolated_stats_engine / "devtrack.db"
        with sqlite3.connect(str(db)) as conn:
            _create_triggers_table(conn)
            conn.execute(
                "INSERT INTO triggers (trigger_type, timestamp, processed) VALUES (?, ?, 1)",
                ("timer", "2026-04-05T14:30:00Z"),
            )
            conn.commit()

        stats = _query_stats()
        assert stats.last_trigger == "14:30"

    def test_timestamp_format_iso_no_z(self, isolated_stats_engine):
        """Timestamp format '2006-01-02T15:04:05' (no Z) is parsed to HH:MM."""
        from backend.server_tui.stats_client import _query_stats

        db = isolated_stats_engine / "devtrack.db"
        with sqlite3.connect(str(db)) as conn:
            _create_triggers_table(conn)
            conn.execute(
                "INSERT INTO triggers (trigger_type, timestamp, processed) VALUES (?, ?, 1)",
                ("timer", "2026-04-05T09:15:00"),
            )
            conn.commit()

        stats = _query_stats()
        assert stats.last_trigger == "09:15"

    def test_timestamp_format_space_separated(self, isolated_stats_engine):
        """Timestamp format '2006-01-02 15:04:05' (space separator) is parsed to HH:MM."""
        from backend.server_tui.stats_client import _query_stats

        db = isolated_stats_engine / "devtrack.db"
        with sqlite3.connect(str(db)) as conn:
            _create_triggers_table(conn)
            conn.execute(
                "INSERT INTO triggers (trigger_type, timestamp, processed) VALUES (?, ?, 1)",
                ("commit", "2026-04-05 22:45:00"),
            )
            conn.commit()

        stats = _query_stats()
        assert stats.last_trigger == "22:45"


class TestGetTriggerStatsPostgresDispatch:
    """get_trigger_stats() dispatch in PostgreSQL mode.

    Not the fail-closed pattern used by skill_detector.py/dialectic_status.py
    tests — the ``triggers`` table is Go-owned with no Postgres equivalent,
    but stats_client already has a working Go-HTTP branch for it
    (_stats_from_go_http()), so this asserts get_trigger_stats() dispatches
    there — and never touches the SQLite engine — when POSTGRES_URL is set.
    """

    def test_dispatches_to_go_http_and_skips_sqlite_engine(self, monkeypatch):
        from backend.server_tui import stats_client
        from backend.server_tui.stats_client import TriggerStats, get_trigger_stats

        monkeypatch.setattr(
            "backend.config.postgres_url",
            lambda: "postgresql://user:pass@localhost/devtrack",
        )
        sentinel = TriggerStats(triggers_today=7, commits_today=2,
                                 last_trigger="10:15", errors_24h=1)
        with patch.object(stats_client, "_stats_from_go_http", return_value=sentinel) as mock_http, \
             patch.object(stats_client, "_stats_from_sqlite") as mock_sqlite:
            stats = get_trigger_stats()

        mock_http.assert_called_once()
        mock_sqlite.assert_not_called()
        assert stats == sentinel


# ---------------------------------------------------------------------------
# health_client
# ---------------------------------------------------------------------------

def _mock_urlopen_ok(status=200):
    """Return a context-manager mock that simulates a 200 response."""
    resp = MagicMock()
    resp.status = status
    resp.__enter__ = lambda s: resp
    resp.__exit__ = MagicMock(return_value=False)
    return resp


class TestCheckAll:
    """health_client.check_all() — HTTP health polling."""

    def test_both_services_healthy(self):
        """When both urlopen calls succeed with 200, both ServiceHealth.ok are True."""
        from backend.server_tui.health_client import check_all

        with patch("backend.server_tui.health_client._check",
                   return_value=(True, "HTTP 200")) as mock_check:
            results = check_all()

        assert len(results) == 2
        assert results[0].ok is True
        assert results[1].ok is True

    def test_webhook_down_ollama_up(self):
        """When webhook fails and Ollama succeeds, statuses reflect that."""
        from backend.server_tui.health_client import check_all

        side_effects = [(False, "Connection refused"), (True, "HTTP 200")]

        with patch("backend.server_tui.health_client._check",
                   side_effect=side_effects):
            results = check_all()

        assert results[0].name == "webhook_server"
        assert results[0].ok is False
        assert results[1].name == "ollama"
        assert results[1].ok is True

    def test_both_services_down(self):
        """When both services are unreachable, both ok are False."""
        from backend.server_tui.health_client import check_all

        with patch("backend.server_tui.health_client._check",
                   return_value=(False, "Connection refused")):
            results = check_all()

        assert all(r.ok is False for r in results)

    def test_url_normalises_0000_to_localhost(self):
        """_webhook_url() normalises 0.0.0.0 → localhost in the outbound URL.

        health_client.py imports get_webhook_port / get_webhook_host inside the
        function body (lazy import from backend.config), so we patch them at
        their source in backend.config rather than as module-level attributes.
        """
        from backend.server_tui.health_client import _webhook_url

        with patch("backend.config.get_webhook_port", return_value=8089), \
             patch("backend.config.get_webhook_host", return_value="0.0.0.0"):
            url = _webhook_url()

        assert "localhost" in url
        assert "0.0.0.0" not in url
        assert url == "http://localhost:8089/health"

    def test_check_helper_returns_true_on_200(self):
        """_check() returns (True, 'HTTP 200') for a 200 response."""
        from backend.server_tui.health_client import _check

        mock_resp = _mock_urlopen_ok(200)
        with patch("urllib.request.urlopen", return_value=mock_resp):
            ok, detail = _check("http://localhost:8089/health", timeout=1)

        assert ok is True
        assert "200" in detail

    def test_check_helper_returns_false_on_url_error(self):
        """_check() returns (False, reason_str) when URLError is raised."""
        from backend.server_tui.health_client import _check
        from urllib.error import URLError

        with patch("urllib.request.urlopen", side_effect=URLError("Connection refused")):
            ok, detail = _check("http://localhost:8089/health", timeout=1)

        assert ok is False
        assert "refused" in detail.lower() or detail != ""

    def test_check_all_names_are_set(self):
        """check_all() populates the name field on each ServiceHealth entry."""
        from backend.server_tui.health_client import check_all

        with patch("backend.server_tui.health_client._check",
                   return_value=(True, "HTTP 200")):
            results = check_all()

        names = {r.name for r in results}
        assert "webhook_server" in names
        assert "ollama" in names
