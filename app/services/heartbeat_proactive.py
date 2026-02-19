from __future__ import annotations

from datetime import datetime, timedelta
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.blueprint.knowledge_files import get_latest_knowledge_file


def _table_exists(db: Session, table_name: str) -> bool:
    try:
        row = db.execute(
            text(
                "select 1 from information_schema.tables "
                "where table_schema = current_schema() and table_name = :name"
            ),
            {"name": table_name},
        ).first()
        if row:
            return True
    except Exception:
        pass
    try:
        row = db.execute(text("select name from sqlite_master where type='table' and name=:name"), {"name": table_name}).first()
        return bool(row)
    except Exception:
        return False


def _extract_focus(content: str, limit: int = 3) -> list[str]:
    lines: list[str] = []
    for raw in str(content or "").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("-"):
            lines.append(line[1:].strip())
        else:
            lines.append(line)
        if len(lines) >= limit:
            break
    return [line for line in lines if line]


def _recent_heartbeat_notification(db: Session, *, user_id: str, since: datetime) -> bool:
    if not _table_exists(db, "notification_queue"):
        return False
    row = db.execute(
        text(
            "select id from notification_queue "
            "where user_id = :user_id and event_type = 'heartbeat_checkin' and created_at >= :since "
            "limit 1"
        ),
        {"user_id": user_id, "since": since},
    ).first()
    return bool(row)


def enqueue_heartbeat_checkin(db: Session, *, user_id: str) -> dict[str, Any]:
    if not _table_exists(db, "notification_queue"):
        return {"ok": False, "reason": "notification_queue_missing"}
    item = get_latest_knowledge_file(db, user_id=user_id, file_path="HEARTBEAT.md")
    content = str((item or {}).get("content") or "").strip()
    if not content:
        return {"ok": False, "reason": "heartbeat_empty"}

    focus = _extract_focus(content)
    if not focus:
        return {"ok": False, "reason": "heartbeat_no_focus"}

    cutoff = datetime.utcnow() - timedelta(hours=24)
    if _recent_heartbeat_notification(db, user_id=user_id, since=cutoff):
        return {"ok": True, "queued": False, "reason": "recently_sent"}

    message = "Quick check-in: " + "; ".join(focus[:3])
    db.execute(
        text(
            "insert into notification_queue ("
            "user_id, watch_item_id, event_type, title, message, deep_link_url, "
            "prev_price, new_price, currency, is_sent, sent_at, created_at"
            ") values ("
            ":user_id, null, :event_type, :title, :message, null, "
            "null, null, null, false, null, :created_at"
            ")"
        ),
        {
            "user_id": user_id,
            "event_type": "heartbeat_checkin",
            "title": "Heartbeat check-in",
            "message": message[:3000],
            "created_at": datetime.utcnow(),
        },
    )
    db.commit()
    return {"ok": True, "queued": True, "message": message}
