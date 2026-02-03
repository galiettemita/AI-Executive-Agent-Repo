# backend/app/services/usage.py

from __future__ import annotations

from datetime import datetime

from sqlalchemy.orm import Session

from app.db.models import Usage


def _period_key(dt: datetime | None = None) -> str:
    d = dt or datetime.utcnow()
    return f"{d.year:04d}-{d.month:02d}"


def _get_or_create_usage(db: Session, user_id: str, period: str) -> Usage:
    row = (
        db.query(Usage)
        .filter(Usage.user_id == user_id, Usage.period == period)
        .first()
    )
    if row:
        return row
    row = Usage(user_id=user_id, period=period)
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def record_message(db: Session, user_id: str, *, count: int = 1) -> None:
    period = _period_key()
    row = _get_or_create_usage(db, user_id, period)
    row.messages_count += count
    db.commit()


def record_tokens(db: Session, user_id: str, *, count: int) -> None:
    period = _period_key()
    row = _get_or_create_usage(db, user_id, period)
    row.tokens_count += int(count)
    db.commit()


def record_proposal(db: Session, user_id: str, *, count: int = 1) -> None:
    period = _period_key()
    row = _get_or_create_usage(db, user_id, period)
    row.proposals_count += count
    db.commit()


def get_usage(db: Session, user_id: str, period: str | None = None) -> Usage:
    period = period or _period_key()
    return _get_or_create_usage(db, user_id, period)
