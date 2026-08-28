"""SQLAlchemy schema for PostgreSQL-backed administration data.

This module deliberately contains no authentication imports or configuration
lookups.  Alembic must be able to discover the database schema with only a
``POSTGRES_URL`` configured.
"""

from sqlalchemy import Column, ForeignKey, Integer, MetaData, Table, Text


admin_metadata = MetaData()

admin_users_table = Table(
    "admin_users",
    admin_metadata,
    Column("id", Integer, primary_key=True, autoincrement=True),
    Column("username", Text, nullable=False, unique=True),
    Column("password_hash", Text, nullable=False),
    Column("role", Text, nullable=False),
    Column("created_at", Text, nullable=False),
    Column("last_login", Text),
    Column("disabled", Integer, nullable=False),
)

admin_api_keys_table = Table(
    "admin_api_keys",
    admin_metadata,
    Column("id", Integer, primary_key=True, autoincrement=True),
    Column(
        "user_id",
        Integer,
        ForeignKey("admin_users.id", ondelete="CASCADE"),
        nullable=False,
    ),
    Column("key_prefix", Text, nullable=False),
    Column("key_hash", Text, nullable=False),
    Column("label", Text, nullable=False),
    Column("created_at", Text, nullable=False),
    Column("last_used", Text),
)

audit_log_table = Table(
    "audit_log",
    admin_metadata,
    Column("id", Integer, primary_key=True, autoincrement=True),
    Column("username", Text, nullable=False),
    Column("action", Text, nullable=False),
    Column("detail", Text, nullable=False),
    Column("ip", Text, nullable=False),
    Column("ts", Text, nullable=False),
)

connected_clients_table = Table(
    "connected_clients",
    admin_metadata,
    Column("id", Integer, primary_key=True, autoincrement=True),
    Column("client_id", Text, nullable=False, unique=True),
    Column("version", Text, nullable=False),
    Column("tls_enabled", Integer, nullable=False),
    Column("workspaces", Text, nullable=False),
    Column("last_seen", Text, nullable=False),
    Column("ip", Text, nullable=False),
)
