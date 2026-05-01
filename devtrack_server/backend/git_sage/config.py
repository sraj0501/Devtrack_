"""Configuration management for git-sage."""
import json
import os
from pathlib import Path
from typing import Optional


CONFIG_DIR = Path.home() / ".config" / "git-sage"
CONFIG_FILE = CONFIG_DIR / "config.json"

def _get_default_model() -> str:
    """Get default model from env var GIT_SAGE_DEFAULT_MODEL."""
    try:
        from backend.config import git_sage_default_model
        return git_sage_default_model()
    except (ImportError, ValueError):
        # Fallback if config not available (required var missing)
        return "llama3"


DEFAULT_CONFIG = {
    "provider": "ollama",
    "model": _get_default_model(),
    "base_url": None,
    "api_key": None,
    "auto_confirm": False,
    "verbose": False,
}


def load_config() -> dict:
    """Load config from ~/.config/git-sage/config.json or return defaults."""
    if CONFIG_FILE.exists():
        try:
            with open(CONFIG_FILE, "r") as f:
                cfg = json.load(f)
            # Merge with defaults to ensure all keys present
            return {**DEFAULT_CONFIG, **cfg}
        except (json.JSONDecodeError, IOError):
            return DEFAULT_CONFIG.copy()
    return DEFAULT_CONFIG.copy()


def save_config(cfg: dict) -> None:
    """Save config to ~/.config/git-sage/config.json."""
    CONFIG_DIR.mkdir(parents=True, exist_ok=True)
    with open(CONFIG_FILE, "w") as f:
        json.dump(cfg, f, indent=2)


def show_config(cfg: dict) -> None:
    """Display current configuration."""
    print(f"\n  Provider:    {cfg.get('provider', 'ollama')}")
    print(f"  Model:       {cfg.get('model', 'llama3')}")
    if cfg.get("base_url"):
        print(f"  Base URL:    {cfg['base_url']}")
    print(f"  Auto-confirm: {cfg.get('auto_confirm', False)}")
    print(f"  Verbose:     {cfg.get('verbose', False)}")
    print(f"  Config file: {CONFIG_FILE}\n")
