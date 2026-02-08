# app/services/gift_reminders.py

from __future__ import annotations

from datetime import date, datetime, timedelta
from typing import Any, Dict, List

from sqlalchemy.orm import Session

from app.db.models import GiftOccasion, NotificationQueue
from app.core.config import settings
from app.services.gift_service import serialize_occasion


def _next_occurrence(occasion_date: date | None, recurrence: str | None) -> date | None:
    if not occasion_date:
        return None
    if (recurrence or "annual") == "annual":
        today = date.today()
        target = date(today.year, occasion_date.month, occasion_date.day)
        if target < today:
            target = date(today.year + 1, occasion_date.month, occasion_date.day)
        return target
    return occasion_date


def find_due_occasions(db: Session) -> List[GiftOccasion]:
    rows = db.query(GiftOccasion).all()
    today = date.today()
    due = []
    for row in rows:
        next_date = _next_occurrence(row.occasion_date, row.recurrence)
        if not next_date:
            continue
        days_until = (next_date - today).days
        reminder_days = row.reminder_days_before or settings.GIFT_REMINDER_DEFAULT_DAYS
        if days_until < 0:
            continue
        if days_until <= reminder_days:
            if row.last_reminder_sent_at and row.last_reminder_sent_at.date() == today:
                continue
            due.append(row)
    return due


def enqueue_gift_reminders(db: Session) -> Dict[str, Any]:
    due = find_due_occasions(db)
    queued = 0
    now = datetime.utcnow()
    for row in due:
        next_date = _next_occurrence(row.occasion_date, row.recurrence)
        message = (
            f"Upcoming {row.occasion_type or 'occasion'} for {row.recipient_name} "
            f"on {next_date.isoformat() if next_date else 'unknown date'}."
        )
        db.add(
            NotificationQueue(
                user_id=row.user_id,
                event_type="gift_reminder",
                title="Gift reminder",
                message=message,
                deep_link_url=None,
                is_sent=False,
            )
        )
        row.last_reminder_sent_at = now
        queued += 1
    db.commit()
    return {"queued": queued, "due": [serialize_occasion(row) for row in due]}


def enqueue_gift_reminders_for_user(db: Session, user_id: str) -> Dict[str, Any]:
    due = [row for row in find_due_occasions(db) if row.user_id == user_id]
    now = datetime.utcnow()
    queued = 0
    for row in due:
        next_date = _next_occurrence(row.occasion_date, row.recurrence)
        message = (
            f"Upcoming {row.occasion_type or 'occasion'} for {row.recipient_name} "
            f"on {next_date.isoformat() if next_date else 'unknown date'}."
        )
        db.add(
            NotificationQueue(
                user_id=row.user_id,
                event_type="gift_reminder",
                title="Gift reminder",
                message=message,
                deep_link_url=None,
                is_sent=False,
            )
        )
        row.last_reminder_sent_at = now
        queued += 1
    db.commit()
    return {"queued": queued, "due": [serialize_occasion(row) for row in due]}
