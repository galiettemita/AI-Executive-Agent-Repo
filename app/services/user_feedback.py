from __future__ import annotations

import json
import threading
import uuid
from datetime import datetime, timedelta, timezone
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.core.alerting import send_alert
from app.services.analytics import emit_event_async
from app.services.content_safety import enqueue_moderation_item

_TABLE_LOCK = threading.Lock()
_TABLE_READY = False


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


def ensure_user_feedback_table(db: Session) -> None:
    global _TABLE_READY
    if _TABLE_READY and _table_exists(db, "user_feedback"):
        return
    with _TABLE_LOCK:
        if _TABLE_READY and _table_exists(db, "user_feedback"):
            return
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            db.execute(
                text(
                    """
                    create table if not exists user_feedback (
                      id text primary key,
                      user_id text,
                      message_id text,
                      run_id text,
                      feedback text not null,
                      comment text,
                      metadata_json text,
                      created_at datetime default current_timestamp
                    )
                    """
                )
            )
            db.execute(text("create index if not exists idx_user_feedback_created_at on user_feedback(created_at)"))
            db.execute(text("create index if not exists idx_user_feedback_user_id on user_feedback(user_id)"))
        else:
            db.execute(
                text(
                    """
                    create table if not exists user_feedback (
                      id text primary key,
                      user_id text,
                      message_id text,
                      run_id text,
                      feedback text not null,
                      comment text,
                      metadata_json jsonb,
                      created_at timestamptz default now()
                    )
                    """
                )
            )
            db.execute(text("create index if not exists idx_user_feedback_created_at on user_feedback(created_at)"))
            db.execute(text("create index if not exists idx_user_feedback_user_id on user_feedback(user_id)"))
        db.commit()
        _TABLE_READY = True


def _count_recent_negative_feedback(db: Session, *, user_id: str | None, window_minutes: int = 60) -> int:
    ensure_user_feedback_table(db)
    since = datetime.now(timezone.utc) - timedelta(minutes=window_minutes)
    if user_id:
        row = db.execute(
            text(
                "select count(*) as c from user_feedback "
                "where user_id = :user_id and feedback in ('down', 'negative') and created_at >= :since"
            ),
            {"user_id": user_id, "since": since.isoformat()},
        ).mappings().first()
    else:
        row = db.execute(
            text(
                "select count(*) as c from user_feedback "
                "where feedback in ('down', 'negative') and created_at >= :since"
            ),
            {"since": since.isoformat()},
        ).mappings().first()
    return int((row or {}).get("c") or 0)


def record_user_feedback(
    db: Session,
    *,
    user_id: str | None,
    message_id: str | None,
    run_id: str | None,
    feedback: str,
    comment: str | None = None,
    metadata: dict[str, Any] | None = None,
) -> dict[str, Any]:
    ensure_user_feedback_table(db)
    fb = str(feedback or "").strip().lower()
    if fb not in {"up", "down", "positive", "negative"}:
        raise ValueError("feedback must be up/down (or positive/negative)")
    payload = {
        "id": str(uuid.uuid4()),
        "user_id": user_id,
        "message_id": message_id,
        "run_id": run_id,
        "feedback": fb,
        "comment": str(comment or ""),
        "metadata_json": json.dumps(metadata or {}, ensure_ascii=False),
    }
    dialect = db.bind.dialect.name if db.bind is not None else ""
    if dialect == "sqlite":
        db.execute(
            text(
                """
                insert into user_feedback (
                  id, user_id, message_id, run_id, feedback, comment, metadata_json
                ) values (
                  :id, :user_id, :message_id, :run_id, :feedback, :comment, :metadata_json
                )
                """
            ),
            payload,
        )
    else:
        db.execute(
            text(
                """
                insert into user_feedback (
                  id, user_id, message_id, run_id, feedback, comment, metadata_json
                ) values (
                  :id, :user_id, :message_id, :run_id, :feedback, :comment, cast(:metadata_json as jsonb)
                )
                """
            ),
            payload,
        )
    db.commit()
    emit_event_async(
        event_name="feedback_given",
        user_id=user_id,
        source="feedback_api",
        payload={"run_id": run_id, "message_id": message_id, "feedback": fb},
    )

    moderation_item = None
    if fb in {"down", "negative"}:
        moderation_item = enqueue_moderation_item(
            user_id=user_id,
            run_id=run_id,
            direction="outbound",
            channel=None,
            text_value=(comment or "Negative user feedback"),
            categories=["negative_feedback"],
            risk_score=0.45,
            classifier="user_feedback",
            metadata={"message_id": message_id, "feedback": fb},
            increment_safety_flags=False,
        )
        negatives_last_hour = _count_recent_negative_feedback(db, user_id=user_id, window_minutes=60)
        if negatives_last_hour >= 3:
            send_alert(
                f"Negative feedback spike detected for user={user_id or 'unknown'} count={negatives_last_hour}/hour",
                provider="slack",
            )

    return {
        "ok": True,
        "id": payload["id"],
        "feedback": fb,
        "moderation_item_created": moderation_item is not None,
    }
