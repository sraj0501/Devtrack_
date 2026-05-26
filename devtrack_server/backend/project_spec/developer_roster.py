"""
Fetch team members from the configured PM platform.

Returns a normalised list of Developer objects regardless of the backend
(Azure DevOps, GitHub, GitLab). All methods are async.

Configuration (via .env):
    AZURE_ORGANIZATION, AZURE_PROJECT, AZURE_DEVOPS_PAT  — Azure
    GITHUB_TOKEN, GITHUB_OWNER, GITHUB_REPO              — GitHub
    GITLAB_URL, GITLAB_PAT, GITLAB_PROJECT_ID            — GitLab
"""

import logging
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

logger = logging.getLogger(__name__)


@dataclass
class Developer:
    """Normalised developer record, platform-agnostic."""

    name: str
    email: str                          # unique identifier / login
    platform_user_id: str              # native platform ID (UPN / login / username)
    skills: Dict[str, List[str]] = field(default_factory=lambda: {"primary": [], "secondary": []})
    unavailable: List[Dict[str, str]] = field(default_factory=list)
    capacity_override: Optional[Dict[str, Any]] = None  # {"available_days": N, "reason": "..."}

    def to_dict(self) -> Dict[str, Any]:
        return {
            "name": self.name,
            "email": self.email,
            "platform_user_id": self.platform_user_id,
            "skills": self.skills,
            "unavailable": self.unavailable,
            "capacity_override": self.capacity_override,
        }

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "Developer":
        return cls(
            name=d.get("name", ""),
            email=d.get("email", ""),
            platform_user_id=d.get("platform_user_id", ""),
            skills=d.get("skills", {"primary": [], "secondary": []}),
            unavailable=d.get("unavailable", []),
            capacity_override=d.get("capacity_override"),
        )


class DeveloperRoster:
    """Fetch available team members from the configured PM platform."""

    async def list_team_members(self, platform: str) -> List[Developer]:
        """Return team members for the given platform.

        Args:
            platform: "azure" | "github" | "gitlab"

        Returns list of Developer objects with name + platform_user_id set.
        Skills are empty — the PM fills them in during the /newproject flow.
        """
        platform = (platform or "").lower().strip()
        try:
            if platform == "azure":
                return await self._from_azure()
            elif platform == "github":
                return await self._from_github()
            elif platform == "gitlab":
                return await self._from_gitlab()
            else:
                logger.warning(f"list_team_members: unknown platform '{platform}'")
                return []
        except Exception as e:
            logger.warning(f"list_team_members ({platform}) failed: {e}")
            return []

    # -- platform implementations -------------------------------------------

    async def _from_azure(self) -> List[Developer]:
        from backend.azure.client import AzureDevOpsClient
        client = AzureDevOpsClient()
        if not client.is_configured():
            logger.warning("Azure not configured — cannot list team members")
            return []
        try:
            members = await client.list_team_members()
            return [
                Developer(
                    name=m.get("displayName", m.get("uniqueName", "")),
                    email=m.get("uniqueName", ""),
                    platform_user_id=m.get("uniqueName", ""),
                )
                for m in members
                if m.get("uniqueName") or m.get("displayName")
            ]
        finally:
            await client.close()

    async def _from_github(self) -> List[Developer]:
        from backend.github.client import GitHubClient
        client = GitHubClient()
        if not client.is_configured():
            logger.warning("GitHub not configured — cannot list team members")
            return []
        try:
            # Try org members first; fall back to repo collaborators
            members = await client.list_org_members()
            if not members:
                members = await client.list_repo_collaborators()
            return [
                Developer(
                    name=m.get("login", ""),
                    email=m.get("login", ""),   # GitHub doesn't expose email in org member list
                    platform_user_id=m.get("login", ""),
                )
                for m in members
                if m.get("login")
            ]
        finally:
            await client.close()

    async def _from_gitlab(self) -> List[Developer]:
        from backend.gitlab.client import GitLabClient
        client = GitLabClient()
        if not client.is_configured():
            logger.warning("GitLab not configured — cannot list team members")
            return []
        try:
            members = await client.list_project_members()
            return [
                Developer(
                    name=m.get("name", m.get("username", "")),
                    email=m.get("username", ""),  # GitLab API requires username for assignments
                    platform_user_id=m.get("username", ""),
                )
                for m in members
                if m.get("username")
            ]
        finally:
            await client.close()
