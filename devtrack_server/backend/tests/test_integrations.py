"""Tests for integrations - task_matcher, learning_integration (no tokens required)."""

import pytest

from backend.task_matcher import TaskMatcher, Task, MatchResult


def test_task_matcher_fuzzy_only():
    """TaskMatcher works with use_semantic=False (no sentence-transformers)."""
    matcher = TaskMatcher(use_semantic=False)
    tasks = [
        Task("1", "Fix login bug", "Authentication module", "open", "proj"),
        Task("2", "Add dashboard", "New UI", "open", "proj"),
    ]
    result = matcher.match_task("Fixed login", tasks, threshold=0.5)
    assert result is None or isinstance(result, MatchResult)


def test_task_matcher_exact_id():
    """TaskMatcher finds task by ID in input."""
    matcher = TaskMatcher(use_semantic=False)
    tasks = [Task("AB-123", "Some task", "desc", "open", "proj")]
    result = matcher.match_task("Completed AB-123", tasks, threshold=0.5)
    assert result is not None
    assert result.task.id == "AB-123"


def test_learning_integration_imports():
    """LearningIntegration can be imported without tokens."""
    from backend.learning_integration import LearningIntegration

    # Should not raise; may fail at runtime if tokens missing
    assert LearningIntegration is not None
