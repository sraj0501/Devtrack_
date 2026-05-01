"""
Jira Cloud REST API v3 client.

Uses Basic auth with email + API token (no third-party jira library needed).
The 'requests' package is already a project dependency.

Configuration (via .env):
    JIRA_URL        = https://yourorg.atlassian.net
    JIRA_EMAIL      = your_email@example.com
    JIRA_API_TOKEN  = your_jira_api_token
    JIRA_PROJECT_KEY = PROJ  (optional default project)
"""

import logging
from base64 import b64encode
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

logger = logging.getLogger(__name__)


@dataclass
class JiraIssue:
    """Represents a Jira issue/ticket."""
    id: str
    key: str                  # e.g. PROJ-123
    summary: str
    description: str
    status: str
    assignee: Optional[str]
    issue_type: str
    priority: str
    labels: List[str] = field(default_factory=list)
    url: str = ""


class JiraClient:
    """Jira Cloud REST API v3 client using Basic auth (email + API token)."""

    def __init__(
        self,
        base_url: Optional[str] = None,
        email: Optional[str] = None,
        api_token: Optional[str] = None,
        project_key: Optional[str] = None,
    ):
        _url = base_url
        _email = email
        _token = api_token
        _project = project_key
        if any(v is None for v in [_url, _email, _token, _project]):
            try:
                from backend.config import jira_url, jira_email, jira_api_token, jira_project_key
                _url = _url if _url is not None else jira_url()
                _email = _email if _email is not None else jira_email()
                _token = _token if _token is not None else jira_api_token()
                _project = _project if _project is not None else jira_project_key()
            except Exception:
                pass

        self.base_url = (_url or "").rstrip("/")
        self._email = _email or ""
        self._token = _token or ""
        self.project_key = _project or ""

        credentials = b64encode(f"{self._email}:{self._token}".encode()).decode()
        self._headers = {
            "Authorization": f"Basic {credentials}",
            "Content-Type": "application/json",
            "Accept": "application/json",
        }

    def is_configured(self) -> bool:
        """Return True if all required credentials are present."""
        return bool(self.base_url and self._email and self._token)

    def _get(self, path: str, params: Optional[Dict] = None, timeout: int = 15) -> Optional[Dict]:
        """Make a GET request to the Jira API."""
        if not self.is_configured():
            return None
        try:
            import requests
            url = f"{self.base_url}/rest/api/3{path}"
            resp = requests.get(url, headers=self._headers, params=params, timeout=timeout)
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            logger.warning(f"Jira GET {path} failed: {e}")
            return None

    def get_my_issues(
        self,
        assignee_email: Optional[str] = None,
        status_filter: Optional[List[str]] = None,
        max_results: int = 50,
    ) -> List[JiraIssue]:
        """Fetch issues assigned to the given user (defaults to configured email).

        Args:
            assignee_email: Filter by this assignee. Uses JIRA_EMAIL from config if None.
            status_filter: Optional list of status names to filter on (e.g. ['In Progress']).
            max_results: Maximum number of issues to return.
        """
        assignee = assignee_email or self._email
        if not assignee:
            logger.warning("No assignee email configured for Jira query")
            return []

        status_clause = ""
        if status_filter:
            statuses = ", ".join(f'"{s}"' for s in status_filter)
            status_clause = f" AND status in ({statuses})"

        jql = f'assignee = "{assignee}"{status_clause} ORDER BY updated DESC'
        params = {
            "jql": jql,
            "maxResults": max_results,
            "fields": "summary,description,status,assignee,issuetype,priority,labels",
        }
        data = self._get("/search", params=params)
        if not data:
            return []
        return [self._parse_issue(issue) for issue in data.get("issues", [])]

    def get_issue(self, issue_key: str) -> Optional[JiraIssue]:
        """Fetch a single issue by key (e.g. PROJ-123)."""
        data = self._get(f"/issue/{issue_key}")
        if not data:
            return None
        return self._parse_issue(data)

    def _parse_issue(self, raw: Dict) -> JiraIssue:
        """Parse a raw Jira issue JSON response into a JiraIssue."""
        fields = raw.get("fields", {})
        assignee_field = fields.get("assignee") or {}
        return JiraIssue(
            id=raw.get("id", ""),
            key=raw.get("key", ""),
            summary=fields.get("summary", ""),
            description=self._extract_description(fields.get("description")),
            status=(fields.get("status") or {}).get("name", "Unknown"),
            assignee=(
                assignee_field.get("displayName")
                or assignee_field.get("emailAddress", "")
            ),
            issue_type=(fields.get("issuetype") or {}).get("name", ""),
            priority=(fields.get("priority") or {}).get("name", ""),
            labels=fields.get("labels", []),
            url=f"{self.base_url}/browse/{raw.get('key', '')}",
        )

    def _extract_description(self, desc: Any) -> str:
        """Convert a Jira description field (ADF or plain text) to a string."""
        if desc is None:
            return ""
        if isinstance(desc, str):
            return desc
        if isinstance(desc, dict):
            return self._adf_to_text(desc)
        return ""

    def _adf_to_text(self, node: Dict) -> str:
        """Recursively extract plain text from an Atlassian Document Format (ADF) node."""
        if node.get("type") == "text":
            return node.get("text", "")
        parts = [self._adf_to_text(child) for child in node.get("content", [])]
        return " ".join(p for p in parts if p)
