from __future__ import annotations

from datetime import datetime, timezone
from typing import Any

from googleapiclient.discovery import build
from sqlalchemy import text

from app.core.celery_app import celery_app
from app.db.database import SessionLocal
from app.services.google_oauth import get_valid_google_credentials


def _upsert_sync_cursor(db, *, user_id: str, provider: str, resource_type: str, cursor_value: str | None) -> None:
    if not cursor_value:
        return
    db.execute(
        text(
            """
            insert into sync_cursors (user_id, provider, resource_type, cursor_value, last_sync_at)
            values (:user_id, :provider, :resource_type, :cursor_value, now())
            on conflict (user_id, provider, resource_type)
            do update set cursor_value = excluded.cursor_value, last_sync_at = excluded.last_sync_at
            """
        ),
        {
            "user_id": user_id,
            "provider": provider,
            "resource_type": resource_type,
            "cursor_value": cursor_value,
        },
    )


def _get_sync_cursor(db, *, user_id: str, provider: str, resource_type: str) -> str | None:
    row = db.execute(
        text(
            """
            select cursor_value
            from sync_cursors
            where user_id = :user_id and provider = :provider and resource_type = :resource_type
            limit 1
            """
        ),
        {"user_id": user_id, "provider": provider, "resource_type": resource_type},
    ).mappings().first()
    return str((row or {}).get("cursor_value") or "").strip() or None


@celery_app.task(name="sync.google_calendar_delta")
def sync_google_calendar_delta(user_id: str) -> dict[str, Any]:
    """
    Phase-1 connector sync skeleton:
    - Uses Google incremental sync token when available
    - Persists cursor in sync_cursors
    """
    db = SessionLocal()
    try:
        creds = get_valid_google_credentials(db=db, user_id=user_id)
        if creds is None:
            return {"ok": False, "reason": "google_not_connected", "user_id": user_id}

        service = build("calendar", "v3", credentials=creds)
        sync_token = _get_sync_cursor(db, user_id=user_id, provider="google", resource_type="calendar")

        kwargs: dict[str, Any] = {
            "calendarId": "primary",
            "singleEvents": True,
            "maxResults": 250,
        }
        if sync_token:
            kwargs["syncToken"] = sync_token
        else:
            kwargs["timeMin"] = datetime.now(timezone.utc).isoformat()

        events_result = service.events().list(**kwargs).execute()
        items = events_result.get("items") or []
        next_token = events_result.get("nextSyncToken")
        if next_token:
            _upsert_sync_cursor(
                db,
                user_id=user_id,
                provider="google",
                resource_type="calendar",
                cursor_value=str(next_token),
            )
            db.commit()

        return {"ok": True, "user_id": user_id, "events_seen": len(items), "next_sync_token_set": bool(next_token)}
    except Exception as exc:
        db.rollback()
        return {"ok": False, "user_id": user_id, "error": str(exc)}
    finally:
        try:
            db.close()
        except Exception:
            pass
