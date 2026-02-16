from __future__ import annotations

import logging
from typing import Any

from app.channels.whatsapp import send_whatsapp_template, send_whatsapp_text
from app.channels.whatsapp_templates import get_template, render_template_components

logger = logging.getLogger(__name__)

_WHATSAPP_TEXT_HARD_LIMIT = 4096
_SAFE_CHUNK_SIZE = 3500


def split_whatsapp_text(text: str, max_chars: int = _SAFE_CHUNK_SIZE) -> list[str]:
    body = (text or "").strip()
    if not body:
        return []
    if len(body) <= max_chars:
        return [body]

    chunks: list[str] = []
    current = ""
    for line in body.splitlines(keepends=True):
        if len(current) + len(line) <= max_chars:
            current += line
            continue
        if current:
            chunks.append(current.strip())
            current = ""
        if len(line) <= max_chars:
            current = line
            continue
        # line itself is too large
        start = 0
        while start < len(line):
            part = line[start : start + max_chars]
            chunks.append(part.strip())
            start += max_chars
    if current.strip():
        chunks.append(current.strip())
    return chunks


def _append_buttons_as_text(text: str, buttons: list[dict[str, Any]]) -> str:
    if not buttons:
        return text
    rendered = []
    for idx, button in enumerate(buttons, start=1):
        label = str(button.get("title") or button.get("id") or f"Option {idx}")
        rendered.append(f"{idx}. {label}")
    return f"{text}\n\nReply with:\n" + "\n".join(rendered)


def send_whatsapp_adapted(
    *,
    to_phone_e164: str,
    text: str,
    buttons: list[dict[str, Any]] | None = None,
    metadata: dict[str, Any] | None = None,
) -> list[str]:
    """
    Channel adapter behavior for WhatsApp:
    - template dispatch when `metadata.template_name` is provided
    - button fallback rendering for non-template sends
    - automatic text splitting to respect message size limits
    """
    metadata = metadata or {}
    message_ids: list[str] = []

    template_name = str(metadata.get("template_name") or "").strip()
    template_params = metadata.get("template_params")

    if template_name:
        template = get_template(template_name)
        if not template:
            logger.warning("Unknown WhatsApp template '%s'; falling back to text", template_name)
        else:
            components = render_template_components(template_params if isinstance(template_params, list) else None)
            message_id = send_whatsapp_template(
                to_phone_e164=to_phone_e164,
                template_name=template.name,
                language_code=template.language_code,
                components=components or None,
            )
            if message_id:
                message_ids.append(message_id)
                return message_ids

    outgoing = _append_buttons_as_text(text, buttons or [])
    chunks = split_whatsapp_text(outgoing, max_chars=min(_SAFE_CHUNK_SIZE, _WHATSAPP_TEXT_HARD_LIMIT))

    for chunk in chunks:
        msg_id = send_whatsapp_text(to_phone_e164=to_phone_e164, text=chunk)
        if msg_id:
            message_ids.append(msg_id)

    return message_ids
