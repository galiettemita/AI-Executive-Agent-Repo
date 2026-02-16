from __future__ import annotations

import logging
from typing import Any

import httpx
from sqlalchemy import text

from app.core.celery_app import celery_app
from app.db.database import SessionLocal
from app.services.microsoft_oauth import get_valid_microsoft_access_token


logger = logging.getLogger(__name__)


def _get_cursor(db, *, user_id: str, resource_type: str) -> str | None:
    row = db.execute(
        text(
            """
            select cursor_value
            from sync_cursors
            where user_id = :user_id and provider = 'microsoft' and resource_type = :resource_type
            limit 1
            """
        ),
        {"user_id": user_id, "resource_type": resource_type},
    ).mappings().first()
    if not row:
        return None
    value = row.get("cursor_value")
    return str(value) if value else None


def _upsert_cursor(db, *, user_id: str, resource_type: str, cursor_value: str) -> None:
    db.execute(
        text(
            """
            insert into sync_cursors (user_id, provider, resource_type, cursor_value, last_sync_at)
            values (:user_id, 'microsoft', :resource_type, :cursor_value, now())
            on conflict (user_id, provider, resource_type)
            do update set cursor_value = excluded.cursor_value, last_sync_at = excluded.last_sync_at
            """
        ),
        {"user_id": user_id, "resource_type": resource_type, "cursor_value": cursor_value},
    )
    db.commit()


def _sync_graph_delta(*, token: str, delta_url: str, timeout: float = 15.0) -> tuple[list[dict[str, Any]], str | None]:
    url = delta_url
    items: list[dict[str, Any]] = []
    next_delta_link: str | None = None
    with httpx.Client(timeout=timeout) as client:
        while url:
            resp = client.get(url, headers={"Authorization": f"Bearer {token}"})
            resp.raise_for_status()
            payload = resp.json() or {}
            batch = payload.get("value") or []
            if isinstance(batch, list):
                items.extend([x for x in batch if isinstance(x, dict)])
            next_url = payload.get("@odata.nextLink")
            next_delta_link = payload.get("@odata.deltaLink") or next_delta_link
            url = str(next_url) if next_url else None
    return items, next_delta_link


@celery_app.task(name="sync.microsoft_mail_delta")
def sync_microsoft_mail_delta(user_id: str, max_results: int = 100) -> dict[str, Any]:
    db = SessionLocal()
    try:
        token = get_valid_microsoft_access_token(db=db, user_id=user_id)
        if not token:
            return {"ok": False, "error": "microsoft_not_connected"}

        cursor = _get_cursor(db, user_id=user_id, resource_type="mail")
        if cursor:
            delta_url = cursor
        else:
            delta_url = (
                "https://graph.microsoft.com/v1.0/me/mailFolders/inbox/messages/delta"
                f"?$top={max(1, min(100, int(max_results)))}"
            )
        items, next_cursor = _sync_graph_delta(token=token, delta_url=delta_url)
        if next_cursor:
            _upsert_cursor(db, user_id=user_id, resource_type="mail", cursor_value=next_cursor)
        return {
            "ok": True,
            "provider": "microsoft",
            "resource_type": "mail",
            "cursor_before": cursor,
            "cursor_after": next_cursor or cursor,
            "changed_count": len(items),
            "changed_message_ids": [str(x.get("id")) for x in items if x.get("id")],
        }
    except Exception as exc:
        logger.exception("sync_microsoft_mail_delta failed user_id=%s", user_id)
        return {"ok": False, "error": str(exc)}
    finally:
        try:
            db.close()
        except Exception:
            pass


@celery_app.task(name="sync.microsoft_calendar_delta")
def sync_microsoft_calendar_delta(user_id: str, max_results: int = 100) -> dict[str, Any]:
    db = SessionLocal()
    try:
        token = get_valid_microsoft_access_token(db=db, user_id=user_id)
        if not token:
            return {"ok": False, "error": "microsoft_not_connected"}

        cursor = _get_cursor(db, user_id=user_id, resource_type="calendar")
        if cursor:
            delta_url = cursor
        else:
            delta_url = (
                "https://graph.microsoft.com/v1.0/me/events/delta"
                f"?$top={max(1, min(100, int(max_results)))}"
            )
        items, next_cursor = _sync_graph_delta(token=token, delta_url=delta_url)
        if next_cursor:
            _upsert_cursor(db, user_id=user_id, resource_type="calendar", cursor_value=next_cursor)
        return {
            "ok": True,
            "provider": "microsoft",
            "resource_type": "calendar",
            "cursor_before": cursor,
            "cursor_after": next_cursor or cursor,
            "changed_count": len(items),
            "changed_event_ids": [str(x.get("id")) for x in items if x.get("id")],
        }
    except Exception as exc:
        logger.exception("sync_microsoft_calendar_delta failed user_id=%s", user_id)
        return {"ok": False, "error": str(exc)}
    finally:
        try:
            db.close()
        except Exception:
            pass
