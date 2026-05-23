"""
backend.project_spec — AI-powered project planning module.

Flow:
  DeveloperRoster.list_team_members()
    → WorkloadAnalyzer.analyze()
    → SpecGenerator.generate()
    → SpecStore.save()
    → SpecEmailer.send_draft()          (email to PM)
    → [PM reviews via web form or Telegram]
    → ProjectCreator.create()           (creates sprints/stories in PM tool)
"""

from .developer_roster import Developer, DeveloperRoster
from .workload_analyzer import WorkloadSnapshot, WorkloadAnalyzer
from .spec_generator import ProjectSpec, SpecGenerator
from .spec_store import SpecStore
from .project_creator import ProjectCreator
from .spec_emailer import SpecEmailer

__all__ = [
    "Developer",
    "DeveloperRoster",
    "WorkloadSnapshot",
    "WorkloadAnalyzer",
    "ProjectSpec",
    "SpecGenerator",
    "SpecStore",
    "ProjectCreator",
    "SpecEmailer",
]
