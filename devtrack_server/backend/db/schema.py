"""Deterministic registry for every PostgreSQL server table.

Store modules historically registered their tables lazily when first imported.
Alembic and import tooling need the complete schema without depending on normal
request traffic, so this module imports each owner exactly once and exposes both
metadata collections used by the server.
"""

# Import for registration side effects. Keep this list aligned with the first
# Alembic revision and add future schema owners here before generating a revision.
from backend.db import (  # noqa: F401
    client_event_store,
    learning_store,
    platform_store,
    project_store,
    report_store,
    ticket_db,
    voice_seed_store,
    voice_sync_store,
)
import backend.queue_gateway  # noqa: F401

from backend.admin.schema import admin_metadata
from backend.db.engine import metadata


target_metadata = (metadata, admin_metadata)


def sorted_tables():
    """Return all managed tables in foreign-key-safe insertion order."""
    return [*metadata.sorted_tables, *admin_metadata.sorted_tables]
