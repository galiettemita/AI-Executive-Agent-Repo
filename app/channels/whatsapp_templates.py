from __future__ import annotations

from dataclasses import dataclass
from typing import Any


@dataclass(frozen=True)
class WhatsAppTemplate:
    name: str
    language_code: str = "en_US"
    description: str = ""


# Appendix C registry (phase-1 minimal set; expand as additional templates are approved in Meta).
WHATSAPP_TEMPLATE_REGISTRY: dict[str, WhatsAppTemplate] = {
    "hello_world": WhatsAppTemplate(
        name="hello_world",
        language_code="en_US",
        description="Meta starter verification template",
    ),
    "approval_request_v1": WhatsAppTemplate(
        name="approval_request_v1",
        language_code="en_US",
        description="Request user approval with quick reply buttons",
    ),
    "delegation_reminder_v1": WhatsAppTemplate(
        name="delegation_reminder_v1",
        language_code="en_US",
        description="Reminder template for delegated tasks",
    ),
    "proactive_briefing_v1": WhatsAppTemplate(
        name="proactive_briefing_v1",
        language_code="en_US",
        description="Scheduled proactive summary",
    ),
}


def get_template(template_name: str) -> WhatsAppTemplate | None:
    return WHATSAPP_TEMPLATE_REGISTRY.get((template_name or "").strip())


def render_template_components(params: list[str] | None = None) -> list[dict[str, Any]]:
    values = params or []
    if not values:
        return []
    return [
        {
            "type": "body",
            "parameters": [{"type": "text", "text": value} for value in values],
        }
    ]
