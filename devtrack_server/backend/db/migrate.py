"""Small, config-safe entry point for Alembic server migrations."""

from __future__ import annotations

import argparse
from pathlib import Path
from typing import Sequence

from alembic import command
from alembic.config import Config


SERVER_ROOT = Path(__file__).resolve().parents[2]


def alembic_config() -> Config:
    cfg = Config(str(SERVER_ROOT / "alembic.ini"))
    cfg.set_main_option("script_location", str(SERVER_ROOT / "migrations"))
    return cfg


def upgrade(revision: str = "head") -> None:
    command.upgrade(alembic_config(), revision)


def downgrade(revision: str) -> None:
    command.downgrade(alembic_config(), revision)


def current() -> None:
    command.current(alembic_config(), verbose=True)


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Manage the DevTrack PostgreSQL schema")
    subparsers = parser.add_subparsers(dest="command", required=True)
    upgrade_parser = subparsers.add_parser("upgrade", help="upgrade the schema")
    upgrade_parser.add_argument("revision", nargs="?", default="head")
    downgrade_parser = subparsers.add_parser("downgrade", help="downgrade the schema")
    downgrade_parser.add_argument("revision")
    subparsers.add_parser("current", help="show the current schema revision")
    args = parser.parse_args(argv)

    if args.command == "upgrade":
        upgrade(args.revision)
    elif args.command == "downgrade":
        downgrade(args.revision)
    else:
        current()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
