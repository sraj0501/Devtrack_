"""Tests for backend.ai.create_tasks - TaskGenerator CSV import."""

import tempfile
import csv

import pytest

from backend.ai.create_tasks import TaskGenerator


def test_import_from_csv():
    """Import tasks from CSV with Title, Description, State columns."""
    with tempfile.NamedTemporaryFile(mode="w", suffix=".csv", delete=False) as f:
        w = csv.writer(f)
        w.writerow(["Title", "Description", "State"])
        w.writerow(["Fix login bug", "Auth module", "New"])
        w.writerow(["Add feature", "New feature", "In Progress"])
        path = f.name

    gen = TaskGenerator()
    tasks = gen.import_from_csv(path)
    assert len(tasks) == 2
    assert tasks[0]["Title"] == "Fix login bug"
    assert tasks[0]["State"] == "New"
    assert tasks[1]["Title"] == "Add feature"
    assert tasks[1]["State"] == "In Progress"


def test_import_from_csv_minimal():
    """Import tasks from CSV with only Title column."""
    with tempfile.NamedTemporaryFile(mode="w", suffix=".csv", delete=False) as f:
        w = csv.writer(f)
        w.writerow(["Title"])
        w.writerow(["Task A"])
        path = f.name

    gen = TaskGenerator()
    tasks = gen.import_from_csv(path, description_col=None, state_col=None, assignee_col=None)
    assert len(tasks) == 1
    assert tasks[0]["Title"] == "Task A"
    assert tasks[0]["State"] == "New"
