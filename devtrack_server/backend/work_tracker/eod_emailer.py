"""
Send an EOD report via MS Graph email.

Configuration (via .env):
    AZURE_CLIENT_ID, AZURE_TENANT_ID, AZURE_CLIENT_SECRET  — MS Graph auth
    EOD_REPORT_EMAIL  — default recipient when none provided explicitly
"""

import logging
from typing import TYPE_CHECKING, Optional

from backend.config import (
    get_eod_report_email as _eod_email,
    get_azure_client_id as _az_client_id,
    get_azure_tenant_id as _az_tenant_id,
    get_azure_client_secret as _az_client_secret,
)

if TYPE_CHECKING:
    from backend.work_tracker.eod_report_generator import EODReport

logger = logging.getLogger(__name__)

def _build_html(report: "EODReport") -> str:
    from html import escape
    return "<html><body><pre>" + escape(report.as_text()) + "</pre></body></html>"


class EODEmailer:
    """Send EOD reports via MS Graph email."""

    async def send(self, report: "EODReport", recipient: Optional[str] = None) -> bool:
        """Email the EOD report.

        Args:
            report:    Populated EODReport.
            recipient: Override recipient; falls back to EOD_REPORT_EMAIL env var.

        Returns True on success, False on failure (non-fatal).
        """
        to = recipient or _eod_email()
        if not to:
            logger.warning("EODEmailer: no recipient — set EOD_REPORT_EMAIL or pass --email")
            return False

        subject = f"[DevTrack] EOD Report — {report.date}"
        html_body = _build_html(report)
        return await self._send_via_graph(subject, html_body, to)

    async def _send_via_graph(self, subject: str, html_body: str, recipient: str) -> bool:
        try:
            import configparser as cp
            parser = cp.ConfigParser()
            parser["graph"] = {
                "clientId": _az_client_id(),
                "tenantId": _az_tenant_id(),
                "clientSecret": _az_client_secret(),
                "graphUserScopes": "Mail.Send",
            }
            section = parser["graph"]

            from backend.msgraph_python.graph import Graph
            graph = Graph(section)
            await graph.send_mail(subject=subject, body=html_body, recipient=recipient)
            logger.info(f"EOD report emailed to {recipient}")
            return True
        except Exception as e:
            logger.warning(f"EODEmailer: graph send failed: {e}")
            return False
