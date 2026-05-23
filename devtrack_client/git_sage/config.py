"""Configuration management for git-sage.

Priority (highest → lowest):
  1. CLI flags (--provider, --model, --yes)
  2. .env file (via backend.config)
  3. ~/.config/git-sage/config.json  (manual overrides / legacy)
  4. Hard defaults
"""
import json
import os
from pathlib import Path
from typing import Optional


CONFIG_DIR = Path.home() / ".config" / "git-sage"
CONFIG_FILE = CONFIG_DIR / "config.json"

_HARD_DEFAULTS = {
    "provider": "ollama",
    "model":    "llama3",
    "base_url": None,
    "api_key":  None,
    "auto_confirm": False,
    "verbose":  False,
}


def _config_from_env() -> dict:
    """Build config dict from .env via backend.config. Returns only keys that are set."""
    try:
        from backend.config import (
            git_sage_provider, git_sage_model,
            git_sage_api_key, git_sage_base_url,
        )
        env_cfg: dict = {}
        provider = git_sage_provider()
        if provider:
            env_cfg["provider"] = provider
        model = git_sage_model()
        if model:
            env_cfg["model"] = model
        api_key = git_sage_api_key()
        if api_key:
            env_cfg["api_key"] = api_key
        base_url = git_sage_base_url()
        if base_url:
            env_cfg["base_url"] = base_url
        return env_cfg
    except (ImportError, Exception):
        return {}


def _config_from_file() -> dict:
    """Load ~/.config/git-sage/config.json. Returns {} if missing or invalid."""
    if CONFIG_FILE.exists():
        try:
            with open(CONFIG_FILE, "r") as f:
                return json.load(f)
        except (json.JSONDecodeError, IOError):
            pass
    return {}


def load_config() -> dict:
    """Build config: hard defaults <- file <- .env (env wins over file)."""
    cfg = {**_HARD_DEFAULTS}
    cfg.update(_config_from_file())   # file can override hard defaults
    cfg.update(_config_from_env())    # .env overrides everything
    return cfg


def save_config(cfg: dict) -> None:
    """Save config to ~/.config/git-sage/config.json (for --config wizard)."""
    CONFIG_DIR.mkdir(parents=True, exist_ok=True)
    with open(CONFIG_FILE, "w") as f:
        json.dump(cfg, f, indent=2)


def show_config(cfg: dict) -> None:
    """Display current configuration."""
    print(f"\n  Provider:     {cfg.get('provider', 'ollama')}")
    print(f"  Model:        {cfg.get('model', 'llama3')}")
    if cfg.get("base_url"):
        print(f"  Base URL:     {cfg['base_url']}")
    key = cfg.get("api_key") or ""
    if key:
        print(f"  API key:      {key[:8]}...")
    print(f"  Auto-confirm: {cfg.get('auto_confirm', False)}")
    print(f"  Verbose:      {cfg.get('verbose', False)}")
    print(f"  Config file:  {CONFIG_FILE}  (env overrides this file)\n")
