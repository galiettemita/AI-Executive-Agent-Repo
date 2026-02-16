from __future__ import annotations

import json
import uuid
from datetime import datetime, time, timedelta, timezone
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session


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
        row = db.execute(
            text("select name from sqlite_master where type='table' and name=:name"),
            {"name": table_name},
        ).first()
        return bool(row)
    except Exception:
        return False


def _parse_time(raw: Any) -> time | None:
    value = str(raw or "").strip()
    if not value:
        return None
    for fmt in ("%H:%M:%S", "%H:%M"):
        try:
            parsed = datetime.strptime(value, fmt)
            return parsed.time()
        except Exception:
            continue
    return None


def _within_quiet_hours(now_local: time, *, start: time | None, end: time | None) -> bool:
    if not start or not end:
        return False
    # Overnight windows are common (eg 22:00 - 07:00).
    if start <= end:
        return start <= now_local <= end
    return now_local >= start or now_local <= end


def _daily_fire_count(db: Session, *, user_id: str, now_utc: datetime) -> int:
    row = db.execute(
        text(
            """
            select count(*) as cnt
            from proactive_triggers
            where user_id = :user_id
              and fired_at >= :day_start
              and fired_at < :day_end
            """
        ),
        {
            "user_id": user_id,
            "day_start": now_utc.replace(hour=0, minute=0, second=0, microsecond=0),
            "day_end": now_utc.replace(hour=0, minute=0, second=0, microsecond=0) + timedelta(days=1),
        },
    ).mappings().first()
    return int((row or {}).get("cnt") or 0)


def create_proactive_trigger(
    db: Session,
    *,
    user_id: str,
    trigger_type: str,
    source: str,
    payload: dict[str, Any],
    fire_at: datetime | None = None,
    workflow_id: str | None = None,
    delegation_id: str | None = None,
) -> dict[str, Any]:
    if not _table_exists(db, "proactive_triggers"):
        raise RuntimeError("proactive_triggers table not found")
    trigger_id = str(uuid.uuid4())
    db.execute(
        text(
            """
            insert into proactive_triggers (
                id, user_id, trigger_type, source, payload,
                fire_at, workflow_id, delegation_id, created_at
            ) values (
                :id, :user_id, :trigger_type, :source, :payload,
                :fire_at, :workflow_id, :delegation_id, now()
            )
            """
        ),
        {
            "id": trigger_id,
            "user_id": user_id,
            "trigger_type": trigger_type,
            "source": source,
            "payload": json.dumps(payload or {}, ensure_ascii=False),
            "fire_at": fire_at or datetime.now(timezone.utc),
            "workflow_id": workflow_id,
            "delegation_id": delegation_id,
        },
    )
    db.commit()
    return {"id": trigger_id, "user_id": user_id, "trigger_type": trigger_type, "source": source}


def eligible_due_triggers(db: Session, *, user_id: str, now_utc: datetime | None = None) -> list[dict[str, Any]]:
    if not _table_exists(db, "proactive_triggers"):
        return []
    now = now_utc or datetime.now(timezone.utc)
    account = db.execute(
        text(
            """
            select timezone, quiet_hours_start, quiet_hours_end, max_daily_proactive
            from accounts where id = :user_id
            """
        ),
        {"user_id": user_id},
    ).mappings().first()
    if not account:
        return []

    cap = int(account.get("max_daily_proactive") or 5)
    if _daily_fire_count(db, user_id=user_id, now_utc=now) >= cap:
        return []

    quiet_start = _parse_time(account.get("quiet_hours_start"))
    quiet_end = _parse_time(account.get("quiet_hours_end"))
    if _within_quiet_hours(now.astimezone(timezone.utc).time(), start=quiet_start, end=quiet_end):
        return []

    rows = db.execute(
        text(
            """
            select id, trigger_type, source, payload, fire_at, workflow_id, delegation_id
            from proactive_triggers
            where user_id = :user_id
              and dismissed = false
              and fired_at is null
              and fire_at <= :now
            order by fire_at asc
            limit :limit
            """
        ),
        {"user_id": user_id, "now": now, "limit": max(1, cap)},
    ).mappings().all()
    out: list[dict[str, Any]] = []
    for row in rows:
        payload = row.get("payload")
        if isinstance(payload, str):
            try:
                payload = json.loads(payload)
            except Exception:
                payload = {"raw": payload}
        out.append(
            {
                "id": str(row.get("id")),
                "trigger_type": str(row.get("trigger_type")),
                "source": str(row.get("source") or ""),
                "payload": payload if isinstance(payload, dict) else {},
                "fire_at": row.get("fire_at"),
                "workflow_id": row.get("workflow_id"),
                "delegation_id": row.get("delegation_id"),
            }
        )
    return out


def mark_trigger_fired(db: Session, *, trigger_id: str) -> None:
    if not _table_exists(db, "proactive_triggers"):
        return
    db.execute(
        text(
            """
            update proactive_triggers
            set fired_at = now(), fire_count = coalesce(fire_count, 0) + 1
            where id = :trigger_id
            """
        ),
        {"trigger_id": trigger_id},
    )
    db.commit()


def render_proactive_message(*, trigger_source: str, payload: dict[str, Any]) -> str:
    source = (trigger_source or "").strip().lower()
    if source == "morning_briefing":
        top_priority = str((payload or {}).get("top_priority") or "your highest-priority item")
        return (
            f"Good morning. Here’s your brief: focus on {top_priority}. "
            "Reply PLAN and I’ll draft your day schedule."
        )
    if source == "pre_meeting_prep":
        meeting = str((payload or {}).get("meeting") or "your upcoming meeting")
        return (
            f"Prep reminder: {meeting} is coming up soon. "
            "Reply PREP and I’ll send talking points + attendee context."
        )
    if source == "followup_nudge":
        item = str((payload or {}).get("item") or "your pending follow-up")
        return (
            f"Follow-up nudge: {item}. "
            "Reply YES and I’ll draft/send the follow-up now."
        )
    if source == "delegation_reminder":
        who = str((payload or {}).get("delegate_name") or "delegate")
        task = str((payload or {}).get("task_description") or "assigned task")
        return f"Delegation check: {who} still has '{task}' pending. Reply REMIND to send a reminder."
    return "You have a proactive reminder ready."
