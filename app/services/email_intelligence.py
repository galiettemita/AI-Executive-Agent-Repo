from __future__ import annotations

import json
import logging
from typing import Any, Dict, List, Optional

from openai import OpenAI
from sqlalchemy.orm import Session

from app.core.config import settings
from app.services.email_router import list_recent_emails, search_emails, get_email_by_id

logger = logging.getLogger(__name__)

client = OpenAI(api_key=settings.OPENAI_API_KEY)


def _truncate(text: str | None, limit: int = 1200) -> str:
    if not text:
        return ""
    t = text.strip()
    if len(t) <= limit:
        return t
    return t[:limit] + "…"


def summarize_inbox(
    db: Session,
    user_id: str,
    max_results: int = 10,
    hours_back: int = 24,
    provider: Optional[str] = None,
) -> Dict[str, Any]:
    emails = list_recent_emails(
        db=db,
        user_id=user_id,
        max_results=max_results,
        hours_back=hours_back,
        unread_only=True,
        provider=provider,
        include_body=True,
    )

    if not emails:
        return {
            "summary": "No unread emails in the selected window.",
            "priorities": [],
        }

    payload = []
    for msg in emails:
        payload.append(
            {
                "id": msg.get("id"),
                "from": msg.get("from"),
                "subject": msg.get("subject"),
                "date": msg.get("date"),
                "snippet": _truncate(msg.get("snippet"), 400),
                "body": _truncate(msg.get("body"), 1200),
            }
        )

    system = (
        "You are an executive inbox analyst. "
        "Return ONLY valid JSON with keys: summary (string) and priorities (list). "
        "Each priority item: {id, from, subject, priority (1-5), reason}. "
        "Priority 5 = urgent/action required, 1 = low."
    )

    try:
        resp = client.chat.completions.create(
            model=settings.OPENAI_MODEL,
            messages=[
                {"role": "system", "content": system},
                {"role": "user", "content": json.dumps({"emails": payload})},
            ],
            temperature=0.2,
        )
        raw = resp.choices[0].message.content or "{}"
        data = json.loads(raw)
        summary = data.get("summary") or "Inbox summary generated."
        priorities = data.get("priorities") or []
        return {"summary": summary, "priorities": priorities}
    except Exception as exc:
        logger.warning("Email intelligence summary failed: %s", exc)
        return {
            "summary": "Inbox summary failed; please try again.",
            "priorities": [],
        }


def draft_reply(
    db: Session,
    user_id: str,
    *,
    message_id: Optional[str] = None,
    query: Optional[str] = None,
    tone: Optional[str] = None,
    instruction: Optional[str] = None,
    provider: Optional[str] = None,
) -> Dict[str, Any]:
    if not message_id and not query:
        raise ValueError("message_id or query is required")

    target: Optional[Dict[str, Any]] = None
    if message_id:
        target = get_email_by_id(
            db=db,
            user_id=user_id,
            message_id=message_id,
            provider=provider,
            include_body=True,
        )
    if not target:
        emails = search_emails(
            db=db,
            user_id=user_id,
            query=query or message_id or "",
            max_results=3,
            provider=provider,
            include_body=True,
        )
        if emails:
            target = emails[0]

    if not target:
        raise RuntimeError("No matching email found to reply to.")
    from_addr = target.get("from") or ""
    subject = target.get("subject") or ""
    body = target.get("body") or target.get("snippet") or ""

    prompt = {
        "from": from_addr,
        "subject": subject,
        "body": _truncate(body, 2000),
        "tone": tone or "professional and concise",
        "instruction": instruction or "",
    }

    system = (
        "You are an executive assistant drafting an email reply. "
        "Return ONLY valid JSON with keys: subject, body. "
        "Keep it concise, polite, and action-oriented."
    )

    resp = client.chat.completions.create(
        model=settings.OPENAI_MODEL,
        messages=[
            {"role": "system", "content": system},
            {"role": "user", "content": json.dumps(prompt)},
        ],
        temperature=0.3,
    )
    raw = resp.choices[0].message.content or "{}"
    data = json.loads(raw)

    return {
        "to_email": from_addr,
        "subject": data.get("subject") or f"Re: {subject}",
        "body": data.get("body") or "",
        "source_message_id": target.get("id"),
        "provider": target.get("provider"),
    }
