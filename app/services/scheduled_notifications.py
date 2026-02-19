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

from app.blueprint.knowledge_files import get_latest_knowledge_file
from app.core.redis import get_redis
from app.services.preferences import get_preferences

_TABLE_LOCK = threading.Lock()
_TABLE_READY = False


@dataclass
class NotificationRunResult:
    sent: int = 0
    deferred_quiet_hours: int = 0
    deferred_rate_limit: int = 0
    deferred_inactive: int = 0
    paused_inactive: int = 0
    skipped_preferences: int = 0
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


def _parse_payload(payload_raw: Any) -> dict[str, Any]:
    if isinstance(payload_raw, dict):
        return payload_raw
    if isinstance(payload_raw, str) and payload_raw.strip():
        try:
            parsed = json.loads(payload_raw)
            return parsed if isinstance(parsed, dict) else {}
        except Exception:
            return {}
    return {}


def _is_proactive_notification(notification_type: str) -> bool:
    value = (notification_type or "").strip().lower()
    return value in {"morning_briefing", "deadline_alert", "task_reminder", "weekly_summary", "proactive"}


def _is_critical_notification(notification_type: str, payload: dict[str, Any]) -> bool:
    if bool(payload.get("critical")):
        return True
    value = (notification_type or "").strip().lower()
    return value in {"deadline_alert", "critical_alert"}


def _notification_allowed_by_preferences(db: Session, *, user_id: str, notification_type: str) -> bool:
    prefs = get_preferences(db, user_id)
    if not isinstance(prefs, dict) or not prefs:
        return True
    if bool(prefs.get("notifications_disabled")):
        return False
    if bool(prefs.get("proactive_disabled")) and _is_proactive_notification(notification_type):
        return False
    proactive_enabled = prefs.get("proactive_notifications_enabled")
    if proactive_enabled is False and _is_proactive_notification(notification_type):
        return False
    if (notification_type or "").strip().lower() == "morning_briefing" and prefs.get("morning_briefings_enabled") is False:
        return False
    if (notification_type or "").strip().lower() == "weekly_summary" and prefs.get("weekly_summaries_enabled") is False:
        return False
    return True


def _latest_inbound_message_at(db: Session, *, user_id: str) -> datetime | None:
    if not _table_exists(db, "messages"):
        return None
    dialect = db.bind.dialect.name if db.bind is not None else ""
    queries: list[tuple[str, dict[str, Any]]] = []
    if dialect == "sqlite":
        queries.append(
            (
                "select max(created_at) as ts from messages where user_id = :user_id and lower(direction) = 'inbound'",
                {"user_id": user_id},
            )
        )
    else:
        queries.append(
            (
                "select max(created_at) as ts from messages where user_id::text = :user_id and direction::text = 'inbound'",
                {"user_id": user_id},
            )
        )
        queries.append(
            (
                "select max(created_at) as ts from messages where user_id::text = :user_id and lower(direction::text) = 'inbound'",
                {"user_id": user_id},
            )
        )
    for sql, params in queries:
        try:
            row = db.execute(text(sql), params).mappings().first()
        except Exception:
            continue
        value = (row or {}).get("ts")
        if isinstance(value, datetime):
            if value.tzinfo is None:
                return value.replace(tzinfo=timezone.utc)
            return value.astimezone(timezone.utc)
        if isinstance(value, str) and value.strip():
            try:
                parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
                if parsed.tzinfo is None:
                    return parsed.replace(tzinfo=timezone.utc)
                return parsed.astimezone(timezone.utc)
            except Exception:
                continue
    return None


def _inactivity_days(db: Session, *, user_id: str, now_utc: datetime) -> float:
    latest = _latest_inbound_message_at(db, user_id=user_id)
    if latest is None:
        return 999.0
    delta = now_utc - latest
    return max(0.0, float(delta.total_seconds()) / 86400.0)


def _detect_travel_timezone(payload: dict[str, Any]) -> str | None:
    events = payload.get("calendar_events")
    if not isinstance(events, list):
        return None
    for event in events:
        if not isinstance(event, dict):
            continue
        tz_value = str(event.get("timezone") or event.get("tz") or "").strip()
        if tz_value:
            return tz_value
    return None


def _heartbeat_focus(db: Session, *, user_id: str) -> list[str]:
    try:
        item = get_latest_knowledge_file(db, user_id=user_id, file_path="HEARTBEAT.md")
    except Exception:
        return []
    content = str((item or {}).get("content") or "")
    if not content.strip():
        return []
    lines: list[str] = []
    for raw in content.splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("-"):
            lines.append(line[1:].strip())
        elif line.lower().startswith("## "):
            continue
        else:
            lines.append(line)
        if len(lines) >= 4:
            break
    return [line for line in lines if line]


def _generate_notification_message(
    db: Session,
    *,
    user_id: str,
    notification_type: str,
    payload: dict[str, Any],
    now_local: datetime,
) -> str:
    explicit = str(payload.get("message") or payload.get("text") or "").strip()
    if explicit:
        return explicit

    ntype = (notification_type or "").strip().lower()
    if ntype == "morning_briefing":
        events = payload.get("calendar_events")
        tasks = payload.get("tasks")
        event_count = len(events) if isinstance(events, list) else 0
        task_count = len(tasks) if isinstance(tasks, list) else 0
        focus = _heartbeat_focus(db, user_id=user_id)
        focus_line = f" Focus: {focus[0]}." if focus else ""
        return (
            f"Good morning. You have {event_count} calendar item(s) and {task_count} task(s) today."
            f"{focus_line}"
        ).strip()

    if ntype == "deadline_alert":
        milestone = str(payload.get("milestone") or payload.get("title") or "a milestone").strip()
        due = str(payload.get("due_date") or payload.get("due_at") or "soon").strip()
        return f"Deadline alert: {milestone} is due {due}."

    if ntype == "task_reminder":
        task_name = str(payload.get("task") or payload.get("task_name") or "a task").strip()
        due = str(payload.get("due_date") or payload.get("due_at") or "soon").strip()
        return f"Reminder: {task_name} is due {due}."

    if ntype == "weekly_summary":
        completed = int(payload.get("completed_tasks") or 0)
        meetings = int(payload.get("meetings") or 0)
        return f"Weekly summary: {completed} task(s) completed, {meetings} meeting(s) attended."

    return f"You have a scheduled update at {now_local.strftime('%H:%M')}."


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

        notification_type = str(row.get("notification_type") or "custom")
        payload = _parse_payload(row.get("payload_json"))
        if not _notification_allowed_by_preferences(db, user_id=user_id, notification_type=notification_type):
            db.execute(
                text("update scheduled_notifications set status = 'canceled', updated_at = :updated_at where id = :id"),
                {"id": row["id"], "updated_at": datetime.utcnow()},
            )
            result.skipped_preferences += 1
            continue

        inactivity_days = _inactivity_days(db, user_id=user_id, now_utc=now_utc)
        if _is_proactive_notification(notification_type):
            if inactivity_days >= 7.0:
                db.execute(
                    text("update scheduled_notifications set status = 'paused', updated_at = :updated_at where id = :id"),
                    {"id": row["id"], "updated_at": datetime.utcnow()},
                )
                result.paused_inactive += 1
                continue
            if inactivity_days >= 3.0 and not _is_critical_notification(notification_type, payload):
                db.execute(
                    text(
                        "update scheduled_notifications set scheduled_for = :scheduled_for, updated_at = :updated_at where id = :id"
                    ),
                    {"id": row["id"], "scheduled_for": now_utc + timedelta(days=1), "updated_at": datetime.utcnow()},
                )
                result.deferred_inactive += 1
                continue

        tz_name = _resolve_timezone(db, user_id=user_id, fallback=str(row.get("timezone") or "") or None)
        if notification_type.strip().lower() == "morning_briefing":
            travel_tz = _detect_travel_timezone(payload)
            if travel_tz:
                tz_name = travel_tz
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

        message = _generate_notification_message(
            db,
            user_id=user_id,
            notification_type=notification_type,
            payload=payload,
            now_local=now_local,
        )
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
        "deferred_inactive": result.deferred_inactive,
        "paused_inactive": result.paused_inactive,
        "skipped_preferences": result.skipped_preferences,
        "failed": result.failed,
    }
