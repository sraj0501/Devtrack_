"""
Jira integration for DevTrack.

Usage:
    from backend.jira import JiraClient, JiraIssue
    client = JiraClient()
    if client.is_configured():
        issues = client.get_my_issues()
"""

from backend.jira.client import JiraClient, JiraIssue

__all__ = ["JiraClient", "JiraIssue"]
