"""
DevTrack Webhook Event Handlers

Processes webhook payloads from Azure DevOps, GitHub, and Jira.
Separated from HTTP routing for testability.
"""

import logging
from typing import Any, Dict, Optional

from backend.webhook_notifier import WebhookNotifier

logger = logging.getLogger(__name__)


class WebhookEventHandler:
    """Routes and processes incoming webhook events."""

    def __init__(self, ipc_client=None, notifier: Optional[WebhookNotifier] = None, project_sync=None):
        self.ipc_client = ipc_client
        self.notifier = notifier or WebhookNotifier()
        self._project_sync = project_sync  # Optional AzureProjectSync instance

    # ------------------------------------------------------------------
    # Azure DevOps
    # ------------------------------------------------------------------

    async def handle_azure_event(self, event_type: str, resource: Dict[str, Any], raw_payload: Dict[str, Any]) -> Dict[str, Any]:
        """Route Azure DevOps service hook events to specific handlers."""
        handler_map = {
            "workitem.updated": self._handle_azure_work_item_updated,
            "workitem.commented": self._handle_azure_work_item_commented,
            "workitem.created": self._handle_azure_work_item_created,
            "workitem.deleted": self._handle_azure_work_item_deleted,
        }

        handler = handler_map.get(event_type)
        if not handler:
            logger.info(f"Unhandled Azure DevOps event type: {event_type}")
            return {"status": "ignored", "reason": f"unhandled event type: {event_type}"}

        return await handler(resource, raw_payload)

    async def _handle_azure_work_item_updated(self, resource: Dict[str, Any], raw_payload: Dict[str, Any]) -> Dict[str, Any]:
        """Work item fields changed (state, assignment, etc.)."""
        work_item_id = resource.get("workItemId") or resource.get("id")
        revised_by = resource.get("revisedBy", {})
        changed_by_name = revised_by.get("displayName", "Unknown")
        fields = resource.get("fields", {})

        # Build a human-readable summary of changes
        changes = []
        for field_path, change in fields.items():
            field_name = field_path.split(".")[-1]
            old_val = change.get("oldValue", "")
            new_val = change.get("newValue", "")
            changes.append(f"{field_name}: {old_val} → {new_val}")

        summary = "; ".join(changes) if changes else "fields updated"
        title = f"Work Item #{work_item_id} updated by {changed_by_name}"

        await self.notifier.notify(title, summary, source="azure")
        await self._send_ipc_event("workitem.updated", {
            "work_item_id": work_item_id,
            "changed_by": changed_by_name,
            "changes": fields,
        })

        logger.info(f"Azure: {title} — {summary}")

        # Sync to local project if a mapping exists
        await self._sync_work_item_to_local({
            "work_item_id": work_item_id,
            "changed_by": changed_by_name,
            "changes": fields,
        })

        return {"status": "processed", "work_item_id": work_item_id, "changes": len(fields)}

    async def _handle_azure_work_item_commented(self, resource: Dict[str, Any], raw_payload: Dict[str, Any]) -> Dict[str, Any]:
        """Comment added to a work item."""
        work_item_id = resource.get("workItemId") or resource.get("id")
        comment_text = resource.get("comment", "")
        revised_by = resource.get("revisedBy", {})
        commenter = revised_by.get("displayName", "Unknown")

        # Truncate long comments for notification
        display_comment = comment_text[:120] + "…" if len(comment_text) > 120 else comment_text
        title = f"Comment on #{work_item_id} by {commenter}"

        await self.notifier.notify(title, display_comment, source="azure")
        await self._send_ipc_event("workitem.commented", {
            "work_item_id": work_item_id,
            "commenter": commenter,
            "comment": comment_text,
        })

        logger.info(f"Azure: {title}")
        return {"status": "processed", "work_item_id": work_item_id}

    async def _handle_azure_work_item_created(self, resource: Dict[str, Any], raw_payload: Dict[str, Any]) -> Dict[str, Any]:
        """New work item created."""
        work_item_id = resource.get("id")
        fields = resource.get("fields", {})
        title_field = fields.get("System.Title", "Untitled")
        assigned_to = fields.get("System.AssignedTo", {})
        assignee_name = assigned_to.get("displayName", "") if isinstance(assigned_to, dict) else str(assigned_to)

        title = f"New work item #{work_item_id}: {title_field}"
        body = f"Assigned to: {assignee_name}" if assignee_name else "Unassigned"

        await self.notifier.notify(title, body, source="azure")
        await self._send_ipc_event("workitem.created", {
            "work_item_id": work_item_id,
            "title": title_field,
            "assigned_to": assignee_name,
        })

        logger.info(f"Azure: {title}")
        return {"status": "processed", "work_item_id": work_item_id}

    async def _handle_azure_work_item_deleted(self, resource: Dict[str, Any], raw_payload: Dict[str, Any]) -> Dict[str, Any]:
        """Work item deleted."""
        work_item_id = resource.get("id")
        title = f"Work item #{work_item_id} deleted"

        await self.notifier.notify(title, "", source="azure")
        await self._send_ipc_event("workitem.deleted", {"work_item_id": work_item_id})

        logger.info(f"Azure: {title}")
        return {"status": "processed", "work_item_id": work_item_id}

    # ------------------------------------------------------------------
    # GitHub (placeholder)
    # ------------------------------------------------------------------

    async def handle_github_event(self, event_type: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        """Placeholder for GitHub webhook handling."""
        logger.info(f"GitHub event received: {event_type} (not yet implemented)")
        return {"status": "ignored", "reason": "github handler not implemented"}

    # ------------------------------------------------------------------
    # GitLab
    # ------------------------------------------------------------------

    async def handle_gitlab_event(self, event_type: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        """Route GitLab webhook events to specific handlers."""
        handler_map = {
            "Issue Hook": self._handle_gitlab_issue,
            "Merge Request Hook": self._handle_gitlab_merge_request,
            "Note Hook": self._handle_gitlab_note,
        }
        handler = handler_map.get(event_type)
        if not handler:
            logger.info(f"Unhandled GitLab event type: {event_type}")
            return {"handled": False, "reason": f"unknown event type: {event_type}"}
        return await handler(payload)

    async def _handle_gitlab_issue(self, payload: Dict[str, Any]) -> Dict[str, Any]:
        """GitLab Issue Hook — issue opened/closed/updated."""
        attrs = payload.get("object_attributes", {})
        project = payload.get("project", {})
        assignees = payload.get("assignees", [])

        issue_iid = attrs.get("iid")
        title = attrs.get("title", "Untitled")
        state = attrs.get("state", "")
        action = attrs.get("action", "updated")
        url = attrs.get("url", "")
        project_name = project.get("name", "Unknown project")
        assignee_name = assignees[0].get("name", "") if assignees else ""

        notification_title = f"GitLab issue #{issue_iid} {action} in {project_name}"
        message_parts = [title]
        if assignee_name:
            message_parts.append(f"Assigned to: {assignee_name}")
        message = " — ".join(message_parts)

        await self.notifier.notify(notification_title, message, source="gitlab")
        await self._send_ipc_event("gitlab.issue", {
            "issue_iid": issue_iid, "title": title, "state": state,
            "action": action, "project": project_name, "assignee": assignee_name,
        })
        logger.info(f"GitLab: {notification_title}")
        return {"handled": True, "event": "Issue Hook", "action": action}

    async def _handle_gitlab_merge_request(self, payload: Dict[str, Any]) -> Dict[str, Any]:
        """GitLab Merge Request Hook — MR opened/merged/closed."""
        attrs = payload.get("object_attributes", {})
        project = payload.get("project", {})
        assignees = payload.get("assignees", [])

        mr_iid = attrs.get("iid")
        title = attrs.get("title", "Untitled")
        state = attrs.get("state", "")
        action = attrs.get("action", "updated")
        target_branch = attrs.get("target_branch", "")
        project_name = project.get("name", "Unknown project")
        assignee_name = assignees[0].get("name", "") if assignees else ""

        notification_title = f"GitLab MR !{mr_iid} {action} in {project_name}"
        message_parts = [title]
        if target_branch:
            message_parts.append(f"→ {target_branch}")
        if assignee_name:
            message_parts.append(f"Assigned to: {assignee_name}")
        message = " — ".join(message_parts)

        await self.notifier.notify(notification_title, message, source="gitlab")
        await self._send_ipc_event("gitlab.merge_request", {
            "mr_iid": mr_iid, "title": title, "state": state,
            "action": action, "target_branch": target_branch,
            "project": project_name, "assignee": assignee_name,
        })
        logger.info(f"GitLab: {notification_title}")
        return {"handled": True, "event": "Merge Request Hook", "action": action}

    async def _handle_gitlab_note(self, payload: Dict[str, Any]) -> Dict[str, Any]:
        """GitLab Note Hook — comment on issue or MR."""
        attrs = payload.get("object_attributes", {})
        user = payload.get("user", {})

        noteable_type = payload.get("noteable_type", "")
        note_body = attrs.get("note", "")
        commenter = user.get("name", "Unknown")

        display_note = note_body[:120] + "…" if len(note_body) > 120 else note_body
        notification_title = f"GitLab comment by {commenter} on {noteable_type}"

        await self.notifier.notify(notification_title, display_note, source="gitlab")
        await self._send_ipc_event("gitlab.note", {
            "noteable_type": noteable_type, "commenter": commenter, "note": note_body,
        })
        logger.info(f"GitLab: {notification_title}")
        return {"handled": True, "event": "Note Hook", "action": "commented"}

    # ------------------------------------------------------------------
    # Jira (placeholder)
    # ------------------------------------------------------------------

    async def handle_jira_event(self, event_type: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        """Placeholder for Jira webhook handling."""
        logger.info(f"Jira event received: {event_type} (not yet implemented)")
        return {"status": "ignored", "reason": "jira handler not implemented"}

    # ------------------------------------------------------------------
    # Project sync helpers
    # ------------------------------------------------------------------

    async def _sync_work_item_to_local(self, event_data: Dict[str, Any]) -> None:
        """Delegate a work-item change to the project sync layer if configured."""
        if not self._project_sync:
            return
        try:
            updated = await self._project_sync.handle_webhook_update(event_data)
            if updated:
                logger.info(
                    "Local project updated from webhook for WI #%s",
                    event_data.get("work_item_id"),
                )
        except Exception as e:
            logger.warning("Project sync from webhook failed: %s", e)

    # ------------------------------------------------------------------
    # IPC helpers
    # ------------------------------------------------------------------

    async def _send_ipc_event(self, event_type: str, data: Dict[str, Any]) -> None:
        """Forward event to Go daemon via IPC."""
        if not self.ipc_client:
            return
        try:
            from backend.ipc_client import MessageType
            msg_data = {"event_type": event_type, **data}
            self.ipc_client.send_message(MessageType.WEBHOOK_EVENT, msg_data)
        except Exception as e:
            logger.warning(f"Failed to send IPC event: {e}")
