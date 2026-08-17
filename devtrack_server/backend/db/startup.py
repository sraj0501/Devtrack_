"""Fail-fast PostgreSQL initialization for every server entry point."""

from sqlalchemy import text


def initialize_server_database() -> None:
    """Validate connectivity and migrate the mandatory server database."""
    from backend.config import require_postgres_url
    from backend.db.engine import get_engine, init_all_tables

    require_postgres_url()
    engine = get_engine()
    if engine.dialect.name != "postgresql":
        raise RuntimeError("devtrack_server requires PostgreSQL persistence")
    try:
        with engine.connect() as connection:
            connection.execute(text("SELECT 1"))
    except Exception as exc:
        raise RuntimeError(
            "Unable to connect to PostgreSQL using POSTGRES_URL. "
            "Check the host, credentials, database, and network access."
        ) from exc
    init_all_tables()
