"""
Pytest configuration and fixtures for DevTrack backend tests.
"""
import sys
from pathlib import Path

# Add project root to path so we can import backend modules
project_root = Path(__file__).resolve().parent.parent.parent
if str(project_root) not in sys.path:
    sys.path.insert(0, str(project_root))


def pytest_configure(config):
    """Ensure config is loaded before tests that need it."""
    pass
