from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Any

import httpx
from sqlalchemy import text
from sqlalchemy.orm import Session

from app.blueprint.db import _column_exists, _table_exists
from app.core.config import settings
from app.services.token_crypto import decrypt_token

logger = logging.getLogger(__name__)

_SLACK_API_BASE = "https://slack.com/api"


class SlackNotConfiguredError(RuntimeError):
    pass


def _extract_access_token(row: dict[str, Any]) -> str:
    plain = str(row.get("access_token") or "").strip()
    if plain:
        return plain
    encrypted = str(row.get("encrypted_access_token") or "").strip()
    if not encrypted:
        return ""
    try:
        return decrypt_token(encrypted)
    except Exception:
        logger.warning("Failed to decrypt Slack token from oauth_tokens")
        return ""


def _get_slack_access_token(db: Session, *, user_id: str) -> str:
    env_token = str(settings.SLACK_BOT_TOKEN or "").strip()
    if env_token:
        return env_token

    if not _table_exists(db, "oauth_tokens"):
        raise SlackNotConfiguredError("oauth_tokens table not found")

    select_fields = ["provider"]
    if _column_exists(db, "oauth_tokens", "access_token"):
        select_fields.append("access_token")
    if _column_exists(db, "oauth_tokens", "encrypted_access_token"):
        select_fields.append("encrypted_access_token")
    if not {"access_token", "encrypted_access_token"} & set(select_fields):
        raise SlackNotConfiguredError("oauth_tokens is missing token columns")

    row = db.execute(
        text(
            f"""
            select {", ".join(select_fields)}
            from oauth_tokens
            where user_id = :user_id and provider in ('slack', 'slack-mcp')
            order by updated_at desc
            limit 1
            """
        ),
        {"user_id": user_id},
    ).mappings().first()
    if not row:
        raise SlackNotConfiguredError("Slack OAuth token not connected")

    token = _extract_access_token(dict(row))
    if not token:
        raise SlackNotConfiguredError("Slack OAuth token is empty")
    return token


def _call_slack(
    *,
    token: str,
    method: str,
    payload: dict[str, Any],
    timeout_s: float = 20.0,
) -> dict[str, Any]:
    url = f"{_SLACK_API_BASE}/{method}"
    with httpx.Client(timeout=timeout_s) as client:
        resp = client.post(url, headers={"Authorization": f"Bearer {token}"}, json=payload)
        resp.raise_for_status()
        body = resp.json()
    if not bool(body.get("ok")):
        raise RuntimeError(f"Slack API error ({method}): {body.get('error') or 'unknown_error'}")
    return body


def slack_list_messages(
    db: Session,
    *,
    user_id: str,
    channel_id: str,
    limit: int = 20,
) -> list[dict[str, Any]]:
    token = _get_slack_access_token(db, user_id=user_id)
    payload = {"channel": channel_id, "limit": max(1, min(int(limit), 200))}
    body = _call_slack(token=token, method="conversations.history", payload=payload)
    items: list[dict[str, Any]] = []
    for msg in body.get("messages") or []:
        if not isinstance(msg, dict):
            continue
        items.append(
            {
                "ts": str(msg.get("ts") or ""),
                "user": str(msg.get("user") or msg.get("bot_id") or ""),
                "text": str(msg.get("text") or ""),
                "thread_ts": str(msg.get("thread_ts") or ""),
            }
        )
    return items


def slack_send_message(
    db: Session,
    *,
    user_id: str,
    channel_id: str,
    text_body: str,
    thread_ts: str | None = None,
) -> dict[str, Any]:
    token = _get_slack_access_token(db, user_id=user_id)
    payload: dict[str, Any] = {
        "channel": channel_id,
        "text": str(text_body or "").strip(),
        "unfurl_links": False,
        "unfurl_media": False,
    }
    if thread_ts:
        payload["thread_ts"] = thread_ts
    body = _call_slack(token=token, method="chat.postMessage", payload=payload)
    return {
        "channel": str(body.get("channel") or channel_id),
        "ts": str((body.get("message") or {}).get("ts") or body.get("ts") or ""),
        "text": str((body.get("message") or {}).get("text") or payload["text"]),
    }


def slack_channel_summary(
    db: Session,
    *,
    user_id: str,
    channel_id: str,
    limit: int = 50,
) -> dict[str, Any]:
    items = slack_list_messages(db, user_id=user_id, channel_id=channel_id, limit=limit)
    if not items:
        return {
            "channel_id": channel_id,
            "message_count": 0,
            "summary": "No recent messages.",
            "highlights": [],
            "as_of_utc": datetime.now(timezone.utc).isoformat(),
        }

    latest = items[: max(1, min(int(limit), 50))]
    highlights: list[str] = []
    for item in latest[:5]:
        text_value = str(item.get("text") or "").strip()
        if text_value:
            highlights.append(text_value[:180])

    summary = " | ".join(highlights) if highlights else "Recent Slack activity captured."
    return {
        "channel_id": channel_id,
        "message_count": len(latest),
        "summary": summary[:700],
        "highlights": highlights,
        "as_of_utc": datetime.now(timezone.utc).isoformat(),
    }
