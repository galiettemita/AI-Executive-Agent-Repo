from __future__ import annotations

import json
import re
import threading
import uuid
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Any
from zoneinfo import ZoneInfo

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.core.redis import get_redis

_TABLE_LOCK = threading.Lock()
_TABLE_READY = False


@dataclass
class NotificationRunResult:
    sent: int = 0
    deferred_quiet_hours: int = 0
    deferred_rate_limit: int = 0
    failed: int = 0


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


def ensure_scheduled_notifications_table(db: Session) -> None:
    global _TABLE_READY
    if _TABLE_READY and _table_exists(db, "scheduled_notifications"):
        return
    with _TABLE_LOCK:
        if _TABLE_READY and _table_exists(db, "scheduled_notifications"):
            return
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            db.execute(
                text(
                    """
                    create table if not exists scheduled_notifications (
                      id text primary key,
                      user_id text not null,
                      notification_type text not null,
                      channel text,
                      timezone text,
                      payload_json text,
                      scheduled_for datetime not null,
                      status text not null default 'scheduled',
                      sent_at datetime,
                      created_at datetime default current_timestamp,
                      updated_at datetime default current_timestamp
                    )
                    """
                )
            )
        else:
            db.execute(
                text(
                    """
                    create table if not exists scheduled_notifications (
                      id text primary key,
                      user_id text not null,
                      notification_type text not null,
                      channel text,
                      timezone text,
                      payload_json jsonb,
                      scheduled_for timestamptz not null,
                      status text not null default 'scheduled',
                      sent_at timestamptz,
                      created_at timestamptz default now(),
                      updated_at timestamptz default now()
                    )
                    """
                )
            )
        db.execute(text("create index if not exists idx_scheduled_notifications_due on scheduled_notifications(status, scheduled_for)"))
        db.execute(text("create index if not exists idx_scheduled_notifications_user on scheduled_notifications(user_id, status)"))
        db.commit()
        _TABLE_READY = True


def schedule_notification(
    db: Session,
    *,
    user_id: str,
    notification_type: str,
    payload: dict[str, Any],
    scheduled_for: datetime,
    channel: str | None = None,
    timezone_name: str | None = None,
) -> str:
    ensure_scheduled_notifications_table(db)
    notification_id = str(uuid.uuid4())
    payload_json = json.dumps(payload or {}, ensure_ascii=False)
    dialect = db.bind.dialect.name if db.bind is not None else ""
    params = {
        "id": notification_id,
        "user_id": user_id,
        "notification_type": notification_type,
        "channel": channel,
        "timezone": timezone_name,
        "payload_json": payload_json,
        "scheduled_for": scheduled_for,
    }
    if dialect == "sqlite":
        db.execute(
            text(
                """
                insert into scheduled_notifications (
                  id, user_id, notification_type, channel, timezone, payload_json, scheduled_for, status
                ) values (
                  :id, :user_id, :notification_type, :channel, :timezone, :payload_json, :scheduled_for, 'scheduled'
                )
                """
            ),
            params,
        )
    else:
        db.execute(
            text(
                """
                insert into scheduled_notifications (
                  id, user_id, notification_type, channel, timezone, payload_json, scheduled_for, status
                ) values (
                  :id, :user_id, :notification_type, :channel, :timezone, cast(:payload_json as jsonb), :scheduled_for, 'scheduled'
                )
                """
            ),
            params,
        )
    db.commit()
    return notification_id


def _resolve_timezone(db: Session, *, user_id: str, fallback: str | None) -> str:
    if fallback:
        return fallback
    for table in ("users", "accounts"):
        if not _table_exists(db, table):
            continue
        try:
            row = db.execute(
                text(f"select timezone from {table} where id = :id limit 1"),
                {"id": user_id},
            ).first()
            if row and row[0]:
                return str(row[0])
        except Exception:
            continue
    return "UTC"


def _active_channel(user_id: str, fallback: str | None) -> str:
    client = get_redis()
    if client is not None:
        try:
            raw = client.get(f"bp:v1:active-channel:{user_id}")
            if raw:
                return str(raw)
        except Exception:
            pass
    return (fallback or "web").strip().lower() or "web"


def _strip_markdown(text_value: str) -> str:
    text_value = re.sub(r"[*_`#>\[\]\(\)]", "", str(text_value or ""))
    return re.sub(r"\s{2,}", " ", text_value).strip()


def _format_for_channel(channel: str, text_value: str) -> str:
    channel = (channel or "web").strip().lower()
    msg = str(text_value or "").strip()
    if channel == "imessage":
        return _strip_markdown(msg)
    if channel == "whatsapp":
        return msg[:4096]
    return msg


def _quiet_hours_active(now_local: datetime, *, quiet_start_hour: int = 22, quiet_end_hour: int = 7) -> bool:
    hour = now_local.hour
    if quiet_start_hour > quiet_end_hour:
        return hour >= quiet_start_hour or hour < quiet_end_hour
    return quiet_start_hour <= hour < quiet_end_hour


def _next_quiet_window_end(now_local: datetime, *, quiet_end_hour: int = 7) -> datetime:
    candidate = now_local.replace(hour=quiet_end_hour, minute=0, second=0, microsecond=0)
    if candidate <= now_local:
        candidate = candidate + timedelta(days=1)
    return candidate


def _allow_rate(user_id: str) -> bool:
    client = get_redis()
    if client is None:
        return True
    now = datetime.now(timezone.utc).timestamp()

    # Hour bucket: max 2
    hour_key = f"bp:notify:hour:{user_id}"
    client.zremrangebyscore(hour_key, 0, now - 3600)
    if int(client.zcard(hour_key)) >= 2:
        return False

    # Day bucket: max 5
    day_key = f"bp:notify:day:{user_id}"
    client.zremrangebyscore(day_key, 0, now - 86400)
    if int(client.zcard(day_key)) >= 5:
        return False

    member = f"{now:.6f}:{uuid.uuid4().hex[:8]}"
    client.zadd(hour_key, {member: now})
    client.zadd(day_key, {member: now})
    client.expire(hour_key, 3700)
    client.expire(day_key, 87000)
    return True


def _enqueue_outbound_message(
    db: Session,
    *,
    user_id: str,
    channel: str,
    body: str,
) -> None:
    if _table_exists(db, "outbound_messages"):
        db.execute(
            text(
                """
                insert into outbound_messages (
                  user_id, contact_id, channel, to_address, body, status, provider_status, created_at
                ) values (
                  :user_id, null, :channel, :to_address, :body, 'queued', 'queued', :created_at
                )
                """
            ),
            {
                "user_id": user_id,
                "channel": channel,
                "to_address": user_id,
                "body": body,
                "created_at": datetime.utcnow(),
            },
        )
        return
    if _table_exists(db, "notification_queue"):
        db.execute(
            text(
                """
                insert into notification_queue (
                  user_id, event_type, title, message, is_sent, created_at
                ) values (
                  :user_id, :event_type, :title, :message, 0, :created_at
                )
                """
            ),
            {
                "user_id": user_id,
                "event_type": "scheduled_notification",
                "title": "Scheduled notification",
                "message": body,
                "created_at": datetime.utcnow(),
            },
        )


def run_due_scheduled_notifications(db: Session) -> dict[str, int]:
    ensure_scheduled_notifications_table(db)
    now_utc = datetime.now(timezone.utc)
    rows = db.execute(
        text(
            "select * from scheduled_notifications "
            "where status = 'scheduled' and scheduled_for <= :now "
            "order by scheduled_for asc limit 200"
        ),
        {"now": now_utc},
    ).mappings().all()

    result = NotificationRunResult()
    for row in rows:
        user_id = str(row.get("user_id") or "")
        if not user_id:
            result.failed += 1
            continue
        tz_name = _resolve_timezone(db, user_id=user_id, fallback=str(row.get("timezone") or "") or None)
        try:
            tz = ZoneInfo(tz_name)
        except Exception:
            tz = ZoneInfo("UTC")
        now_local = now_utc.astimezone(tz)
        if _quiet_hours_active(now_local):
            deferred_to_local = _next_quiet_window_end(now_local)
            deferred_to_utc = deferred_to_local.astimezone(timezone.utc)
            db.execute(
                text("update scheduled_notifications set scheduled_for = :scheduled_for, updated_at = :updated_at where id = :id"),
                {"id": row["id"], "scheduled_for": deferred_to_utc, "updated_at": datetime.utcnow()},
            )
            result.deferred_quiet_hours += 1
            continue

        if not _allow_rate(user_id):
            db.execute(
                text(
                    "update scheduled_notifications set scheduled_for = :scheduled_for, updated_at = :updated_at where id = :id"
                ),
                {"id": row["id"], "scheduled_for": now_utc + timedelta(minutes=30), "updated_at": datetime.utcnow()},
            )
            result.deferred_rate_limit += 1
            continue

        payload_raw = row.get("payload_json")
        if isinstance(payload_raw, dict):
            payload = payload_raw
        elif isinstance(payload_raw, str) and payload_raw.strip():
            try:
                payload = json.loads(payload_raw)
            except Exception:
                payload = {}
        else:
            payload = {}

        message = str(payload.get("message") or payload.get("text") or "You have a scheduled update.")
        channel = _active_channel(user_id, fallback=str(row.get("channel") or "") or None)
        formatted = _format_for_channel(channel, message)
        _enqueue_outbound_message(db, user_id=user_id, channel=channel, body=formatted)
        db.execute(
            text(
                "update scheduled_notifications "
                "set status = 'sent', sent_at = :sent_at, updated_at = :updated_at, channel = :channel "
                "where id = :id"
            ),
            {
                "id": row["id"],
                "sent_at": datetime.utcnow(),
                "updated_at": datetime.utcnow(),
                "channel": channel,
            },
        )
        result.sent += 1

    db.commit()
    return {
        "sent": result.sent,
        "deferred_quiet_hours": result.deferred_quiet_hours,
        "deferred_rate_limit": result.deferred_rate_limit,
        "failed": result.failed,
    }
