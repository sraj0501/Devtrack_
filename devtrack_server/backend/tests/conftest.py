"""
Pytest configuration and fixtures for DevTrack backend tests.
"""
import os
import sys
from pathlib import Path
from unittest.mock import patch

import pytest

# Add project root to path so we can import backend modules
project_root = Path(__file__).resolve().parent.parent.parent
if str(project_root) not in sys.path:
    sys.path.insert(0, str(project_root))

# Set required scrypt env vars before any auth module is imported.
# These match the production defaults and must be present at import time
# because backend/admin/auth.py reads them into module-level constants.
os.environ.setdefault("SCRYPT_N", "16384")
os.environ.setdefault("SCRYPT_R", "8")
os.environ.setdefault("SCRYPT_P", "1")
os.environ.setdefault("SCRYPT_DKLEN", "32")
os.environ.setdefault("ADMIN_SESSION_HOURS", "8")
os.environ.setdefault("STATS_REFRESH_INTERVAL_SECONDS", "30")
os.environ.setdefault("PROCESS_REFRESH_INTERVAL_SECONDS", "15")
os.environ.setdefault("SHUTDOWN_GRACE_PERIOD_SECONDS", "0.5")
os.environ.setdefault("AUDIT_LOG_LIMIT", "200")
os.environ.setdefault("LICENSE_CONTACT_EMAIL", "license@devtrack.dev")


def pytest_configure(config):
    """Ensure config is loaded before tests that need it."""
    pass


