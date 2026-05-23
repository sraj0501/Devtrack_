"""git-sage: Local LLM-powered git agent."""

__version__ = "0.1.0"
__author__ = "git-sage contributors"

from .cli import main
from .agent import GitAgent
from .llm import LLMBackend
from .context import get_repo_context, format_context, run_git
from .config import load_config, save_config, show_config
from .git_operations import GitOperations
from .conflict_resolver import ConflictResolver, ConflictAnalyzer, ConflictMarker
from .pr_finder import PRFinder

__all__ = [
    "main",
    "GitAgent",
    "LLMBackend",
    "get_repo_context",
    "format_context",
    "run_git",
    "load_config",
    "save_config",
    "show_config",
    "GitOperations",
    "ConflictResolver",
    "ConflictAnalyzer",
    "ConflictMarker",
    "PRFinder",
]
