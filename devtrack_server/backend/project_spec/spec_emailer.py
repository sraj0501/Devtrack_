"""
Send project spec drafts to the PM via email.

Uses the MS Graph API (backend/msgraph_python/graph.py) when configured.
Falls back to logging a warning if Graph is not available.

Configuration (via .env):
    AZURE_CLIENT_ID, AZURE_TENANT_ID, AZURE_CLIENT_SECRET  — for Graph auth
    or the existing Graph settings used elsewhere in DevTrack.
"""

import logging
from typing import Any, Optional

from backend.config import get_azure_client_id as _az_client_id, get_azure_tenant_id as _az_tenant_id, get_azure_client_secret as _az_client_secret

logger = logging.getLogger(__name__)

_HTML_TEMPLATE = """\
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <style>
    body {{ font-family: sans-serif; max-width: 800px; margin: 2em auto; color: #222; }}
    h1 {{ color: #0066cc; }}
    h2 {{ color: #444; border-bottom: 1px solid #ccc; padding-bottom: 4px; }}
    pre {{ background: #f6f8fa; padding: 1em; border-radius: 4px; overflow: auto; font-size: 0.85em; }}
    .risk-high {{ color: #c0392b; font-weight: bold; }}
    .risk-medium {{ color: #e67e22; }}
    .risk-low {{ color: #27ae60; }}
    .btn {{ display: inline-block; padding: 10px 20px; margin: 6px; border-radius: 4px;
            text-decoration: none; font-weight: bold; }}
    .btn-approve {{ background: #27ae60; color: white; }}
    .btn-revise {{ background: #2980b9; color: white; }}
    .meta {{ color: #666; font-size: 0.9em; }}
  </style>
</head>
<body>
  <h1>DevTrack Project Spec — {project_name}</h1>
  <p class="meta">
    Spec ID: <code>{spec_id}</code> &nbsp;|&nbsp;
    Platform: <strong>{platform}</strong> &nbsp;|&nbsp;
    Status: <strong>{status}</strong><br>
    Deadline: <strong>{deadline}</strong> &nbsp;|&nbsp;
    Generated: {created_at}
  </p>

  <h2>Project Goals</h2>
  <ul>
    {goals_html}
  </ul>

  <h2>Team ({num_developers} developers)</h2>
  <ul>
    {team_html}
  </ul>

  <h2>Capacity Analysis</h2>
  <p>
    <strong>On track:</strong> {on_track} &nbsp;|&nbsp;
    <strong>Buffer:</strong> {buffer_days} days &nbsp;|&nbsp;
    <strong>Estimated effort:</strong> {effort_days} days
  </p>
  {risks_html}

  <h2>Sprints ({num_sprints})</h2>
  {sprints_html}

  <h2>Full Spec (YAML)</h2>
  <pre>{spec_yaml}</pre>

  <hr>
  {review_buttons}
</body>
</html>
"""


def _build_html(spec: Any) -> str:
    """Render a ProjectSpec as an HTML email body."""
    project = spec.project or {}
    goals = project.get("goals", [])
    goals_html = "".join(f"<li>{g}</li>" for g in goals) or "<li>(none)</li>"

    team = spec.team or {}
    devs = team.get("developers", [])
    team_html = "".join(
        f"<li><strong>{d.get('name', '?')}</strong> — "
        f"{', '.join(d.get('skills', {}).get('primary', []) or ['?'])}</li>"
        for d in devs
    ) or "<li>(none)</li>"

    ca = spec.capacity_analysis or {}
    on_track = "✅ Yes" if ca.get("on_track") else "⚠️ No"
    buffer_days = ca.get("buffer_days", "?")
    effort_days = ca.get("estimated_effort_days", "?")

    risks = ca.get("risks", [])
    if risks:
        risk_items = []
        for r in risks:
            sev = r.get("severity", "low")
            css = f"risk-{sev}"
            risk_items.append(
                f'<li class="{css}"><strong>[{sev.upper()}]</strong> {r.get("message", "")} '
                f'— <em>{r.get("recommendation", "")}</em></li>'
            )
        risks_html = f"<ul>{''.join(risk_items)}</ul>"
    else:
        risks_html = "<p><em>No risks flagged.</em></p>"

    sprints = spec.sprints or []
    sprint_rows = "".join(
        f"<li><strong>{s.get('name', 'Sprint')}</strong> "
        f"({s.get('start', '?')} → {s.get('end', '?')}): {s.get('goal', '')}</li>"
        for s in sprints
    ) or "<li>(none)</li>"
    sprints_html = f"<ul>{sprint_rows}</ul>"

    import html as html_mod
    spec_yaml = html_mod.escape(spec.to_yaml())

    if spec.review_url:
        review_buttons = (
            f'<p>'
            f'<a class="btn btn-approve" href="{spec.review_url}?action=approve">✅ Approve</a>'
            f'<a class="btn btn-revise" href="{spec.review_url}">📝 Review &amp; Edit</a>'
            f'</p>'
        )
    else:
        review_buttons = "<p><em>Reply to this email with your feedback or approval.</em></p>"

    return _HTML_TEMPLATE.format(
        project_name=project.get("name", "Untitled"),
        spec_id=spec.spec_id,
        platform=spec.pm_platform,
        status=spec.status,
        deadline=project.get("deadline", "?"),
        created_at=spec.created_at,
        goals_html=goals_html,
        num_developers=len(devs),
        team_html=team_html,
        on_track=on_track,
        buffer_days=buffer_days,
        effort_days=effort_days,
        risks_html=risks_html,
        num_sprints=len(sprints),
        sprints_html=sprints_html,
        spec_yaml=spec_yaml,
        review_buttons=review_buttons,
    )


class SpecEmailer:
    """Send spec drafts to the PM via MS Graph email."""

    async def send_draft(self, spec: Any, recipient_email: Optional[str] = None) -> bool:
        """Email the spec to the PM for review.

        Args:
            spec:             ProjectSpec instance.
            recipient_email:  Override recipient; defaults to spec.pm_email.

        Returns True on success, False on failure (non-fatal).
        """
        to = recipient_email or spec.pm_email
        if not to:
            logger.warning("SpecEmailer: no recipient email — skipping email send")
            return False

        project_name = (spec.project or {}).get("name", "Untitled")
        subject = f"[DevTrack] Project Spec Ready for Review: {project_name}"
        html_body = _build_html(spec)

        sent = await self._send_via_graph(subject, html_body, to)
        if sent:
            logger.info(f"Spec email sent to {to} (spec_id={spec.spec_id})")
        else:
            logger.warning(f"SpecEmailer: could not send email to {to}")
        return sent

    async def _send_via_graph(self, subject: str, html_body: str, recipient: str) -> bool:
        """Attempt to send via MS Graph. Returns False gracefully on any error."""
        import asyncio
        try:
            import configparser
            config = configparser.SectionProxy(
                configparser.ConfigParser(), "graph"
            )
            # Build a minimal config object from env vars
            props = {
                "clientId": _az_client_id(),
                "tenantId": _az_tenant_id(),
                "clientSecret": _az_client_secret(),
                "graphUserScopes": "Mail.Send",
            }
            # Re-use the existing Graph class
            import configparser as cp
            parser = cp.ConfigParser()
            parser["graph"] = props
            section = parser["graph"]

            from backend.msgraph_python.graph import Graph
            graph = Graph(section)

            # send_mail is async
            await graph.send_mail(subject=subject, body=html_body, recipient=recipient)
            return True
        except Exception as e:
            logger.warning(f"SpecEmailer._send_via_graph failed: {e}")
            return False
