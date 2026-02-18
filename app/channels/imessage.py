from __future__ import annotations

import re
from datetime import datetime
from typing import Any


_MAX_IMESSAGE_CHARS = 1800
_MARKDOWN_HEADING_RE = re.compile(r"^\s*#{1,6}\s*", re.MULTILINE)
_MARKDOWN_BOLD_RE = re.compile(r"\*\*(.*?)\*\*")
_MARKDOWN_ITALIC_RE = re.compile(r"\*(.*?)\*")


def normalize_imessage_webhook(payload: dict[str, Any]) -> dict[str, Any] | None:
    """
    Normalize iMessage/Messages for Business webhook payload into a channel-agnostic event.
    Supports several provider wrappers by checking common field names.
    """
    if not isinstance(payload, dict):
        return None

    message = payload.get("message") if isinstance(payload.get("message"), dict) else payload
    sender = (
        message.get("from")
        or message.get("sender")
        or (message.get("customer") or {}).get("id")
        or (payload.get("customer") or {}).get("id")
    )
    if not sender:
        return None

    text = message.get("text") or message.get("body") or ""
    media_url = message.get("media_url") or message.get("attachment_url")
    message_id = message.get("id") or message.get("message_id") or payload.get("id")

    event_type = "text"
    if media_url:
        event_type = "media"

    return {
        "external_id": str(message_id or f"imessage-{sender}-{int(datetime.utcnow().timestamp())}"),
        "from": str(sender),
        "text": str(text or "").strip(),
        "media_url": str(media_url) if media_url else None,
        "type": event_type,
        "raw": payload,
    }


def apply_imessage_constraints(text: str) -> str:
    """
    iMessage-friendly formatter:
    - remove markdown heading syntax
    - strip bold/italic markers
    - keep concise payload under channel soft limits
    """
    cleaned = str(text or "").strip()
    if not cleaned:
        return ""

    cleaned = _MARKDOWN_HEADING_RE.sub("", cleaned)
    cleaned = _MARKDOWN_BOLD_RE.sub(r"\1", cleaned)
    cleaned = _MARKDOWN_ITALIC_RE.sub(r"\1", cleaned)
    cleaned = cleaned.replace("```", "")

    if len(cleaned) > _MAX_IMESSAGE_CHARS:
        cleaned = cleaned[: _MAX_IMESSAGE_CHARS - 1].rstrip() + "…"
    return cleaned
