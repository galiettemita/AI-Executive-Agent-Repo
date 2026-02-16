from __future__ import annotations

import logging
from typing import Any

from googleapiclient.discovery import build
from sqlalchemy import text

from app.core.celery_app import celery_app
from app.db.database import SessionLocal
from app.services.google_oauth import get_valid_google_credentials


logger = logging.getLogger(__name__)


def _get_cursor(db, *, user_id: str) -> str | None:
    row = db.execute(
        text(
            """
            select cursor_value
            from sync_cursors
            where user_id = :user_id and provider = 'google' and resource_type = 'gmail'
            limit 1
            """
        ),
        {"user_id": user_id},
    ).mappings().first()
    if not row:
        return None
    value = row.get("cursor_value")
    return str(value) if value else None


def _upsert_cursor(db, *, user_id: str, cursor_value: str) -> None:
    db.execute(
        text(
            """
            insert into sync_cursors (user_id, provider, resource_type, cursor_value, last_sync_at)
            values (:user_id, 'google', 'gmail', :cursor_value, now())
            on conflict (user_id, provider, resource_type)
            do update set cursor_value = excluded.cursor_value, last_sync_at = excluded.last_sync_at
            """
        ),
        {"user_id": user_id, "cursor_value": cursor_value},
    )
    db.commit()


def _full_sync(service, *, max_results: int) -> tuple[list[str], str | None]:
    listed = service.users().messages().list(userId="me", maxResults=max_results, q="in:inbox").execute()
    message_ids = [str((item or {}).get("id")) for item in (listed.get("messages") or []) if (item or {}).get("id")]
    profile = service.users().getProfile(userId="me").execute()
    cursor = profile.get("historyId")
    return message_ids, str(cursor) if cursor else None


@celery_app.task(name="sync.google_gmail_delta")
def sync_google_gmail_delta(user_id: str, max_results: int = 100) -> dict[str, Any]:
    """
    Gmail delta sync baseline for Phase 1.
    - Reads Gmail history from the last sync cursor when available.
    - Falls back to inbox bootstrap on first sync or stale/invalid history IDs.
    - Persists the latest Gmail history ID in sync_cursors.
    """
    db = SessionLocal()
    try:
        creds = get_valid_google_credentials(db=db, user_id=user_id)
        if creds is None:
            return {"ok": False, "error": "google_not_connected"}

        service = build("gmail", "v1", credentials=creds, cache_discovery=False)
        existing_cursor = _get_cursor(db, user_id=user_id)
        changed_message_ids: list[str] = []
        new_cursor: str | None = existing_cursor

        if existing_cursor:
            try:
                history = (
                    service.users()
                    .history()
                    .list(
                        userId="me",
                        startHistoryId=existing_cursor,
                        historyTypes=["messageAdded", "labelAdded", "labelRemoved"],
                        maxResults=max_results,
                    )
                    .execute()
                )
                for event in history.get("history") or []:
                    for msg in event.get("messages") or []:
                        message_id = (msg or {}).get("id")
                        if message_id:
                            changed_message_ids.append(str(message_id))
                cursor = history.get("historyId")
                if cursor:
                    new_cursor = str(cursor)
            except Exception as exc:
                logger.warning(
                    "gmail delta cursor invalid or stale user_id=%s cursor=%s err=%s; bootstrapping full sync",
                    user_id,
                    existing_cursor,
                    exc,
                )
                changed_message_ids, new_cursor = _full_sync(service, max_results=max_results)
        else:
            changed_message_ids, new_cursor = _full_sync(service, max_results=max_results)

        deduped_ids = list(dict.fromkeys(changed_message_ids))
        if new_cursor:
            _upsert_cursor(db, user_id=user_id, cursor_value=new_cursor)

        return {
            "ok": True,
            "user_id": user_id,
            "provider": "google",
            "resource_type": "gmail",
            "cursor_before": existing_cursor,
            "cursor_after": new_cursor,
            "changed_message_ids": deduped_ids,
            "changed_count": len(deduped_ids),
        }
    except Exception as exc:
        logger.exception("sync_google_gmail_delta failed user_id=%s", user_id)
        return {"ok": False, "error": str(exc)}
    finally:
        try:
            db.close()
        except Exception:
            pass
