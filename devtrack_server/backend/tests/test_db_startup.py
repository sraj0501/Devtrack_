"""TASK-116 fail-fast server database startup tests."""

from unittest.mock import MagicMock, patch

import pytest

from backend.db.startup import initialize_server_database


def test_initialize_server_database_checks_connectivity_and_migrates():
    engine = MagicMock()
    engine.dialect.name = "postgresql"
    connection = engine.connect.return_value.__enter__.return_value
    with (
        patch("backend.config.require_postgres_url", return_value="postgresql:///devtrack"),
        patch("backend.db.engine.get_engine", return_value=engine),
        patch("backend.db.engine.init_all_tables") as migrate,
    ):
        initialize_server_database()

    connection.execute.assert_called_once()
    migrate.assert_called_once_with()


def test_initialize_server_database_rejects_non_postgres_engine():
    engine = MagicMock()
    engine.dialect.name = "sqlite"
    with (
        patch("backend.config.require_postgres_url", return_value="postgresql:///devtrack"),
        patch("backend.db.engine.get_engine", return_value=engine),
        pytest.raises(RuntimeError, match="requires PostgreSQL"),
    ):
        initialize_server_database()


def test_initialize_server_database_wraps_connection_error():
    engine = MagicMock()
    engine.dialect.name = "postgresql"
    engine.connect.side_effect = OSError("connection refused")
    with (
        patch("backend.config.require_postgres_url", return_value="postgresql:///devtrack"),
        patch("backend.db.engine.get_engine", return_value=engine),
        pytest.raises(RuntimeError, match="Unable to connect"),
    ):
        initialize_server_database()
