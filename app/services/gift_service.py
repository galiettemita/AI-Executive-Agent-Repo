# app/services/gift_service.py

from __future__ import annotations

import json
from datetime import date, datetime
from typing import Any, Dict, List, Optional

from sqlalchemy.orm import Session

from app.db.models import GiftOccasion, GiftIdea
from app.core.config import settings


def _dump_json(value: Optional[Dict[str, Any]]) -> str:
    if not value:
        return "{}"
    return json.dumps(value, ensure_ascii=False)


def _load_json(value: Optional[str]) -> Dict[str, Any]:
    if not value:
        return {}
    try:
        data = json.loads(value)
        return data if isinstance(data, dict) else {}
    except Exception:
        return {}


def _dump_tags(tags: Optional[List[str]]) -> str:
    if not tags:
        return "[]"
    cleaned = [t.strip() for t in tags if t and t.strip()]
    return json.dumps(cleaned, ensure_ascii=False)


def _load_tags(raw: Optional[str]) -> List[str]:
    if not raw:
        return []
    try:
        data = json.loads(raw)
        if isinstance(data, list):
            return [str(item) for item in data]
    except Exception:
        return []
    return []


def serialize_occasion(row: GiftOccasion) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "recipient_name": row.recipient_name,
        "relationship": row.relationship,
        "occasion_type": row.occasion_type,
        "occasion_date": row.occasion_date.isoformat() if row.occasion_date else None,
        "recurrence": row.recurrence,
        "reminder_days_before": row.reminder_days_before,
        "last_reminder_sent_at": row.last_reminder_sent_at.isoformat() if row.last_reminder_sent_at else None,
        "budget": row.budget,
        "currency": row.currency,
        "preferences": _load_json(row.preferences_json),
        "notes": row.notes,
        "created_at": row.created_at.isoformat() if row.created_at else None,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }


def serialize_idea(row: GiftIdea) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "occasion_id": row.occasion_id,
        "title": row.title,
        "description": row.description,
        "link_url": row.link_url,
        "price": row.price,
        "currency": row.currency,
        "status": row.status,
        "source": row.source,
        "tags": _load_tags(row.tags_json),
        "created_at": row.created_at.isoformat() if row.created_at else None,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }


def create_occasion(
    db: Session,
    user_id: str,
    recipient_name: str,
    relationship: Optional[str] = None,
    occasion_type: Optional[str] = None,
    occasion_date: Optional[date] = None,
    recurrence: Optional[str] = "annual",
    reminder_days_before: Optional[int] = None,
    budget: Optional[float] = None,
    currency: Optional[str] = None,
    preferences: Optional[Dict[str, Any]] = None,
    notes: Optional[str] = None,
) -> GiftOccasion:
    row = GiftOccasion(
        user_id=user_id,
        recipient_name=recipient_name,
        relationship=relationship,
        occasion_type=occasion_type,
        occasion_date=occasion_date,
        recurrence=recurrence,
        reminder_days_before=reminder_days_before or settings.GIFT_REMINDER_DEFAULT_DAYS,
        budget=budget,
        currency=currency,
        preferences_json=_dump_json(preferences),
        notes=notes,
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def update_occasion(
    db: Session,
    user_id: str,
    occasion_id: int,
    **fields: Any,
) -> Optional[GiftOccasion]:
    row = (
        db.query(GiftOccasion)
        .filter(GiftOccasion.user_id == user_id, GiftOccasion.id == occasion_id)
        .one_or_none()
    )
    if not row:
        return None

    for key, value in fields.items():
        if value is None:
            continue
        if key == "preferences":
            row.preferences_json = _dump_json(value)
            continue
        if hasattr(row, key):
            setattr(row, key, value)

    row.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(row)
    return row


def delete_occasion(db: Session, user_id: str, occasion_id: int) -> bool:
    row = (
        db.query(GiftOccasion)
        .filter(GiftOccasion.user_id == user_id, GiftOccasion.id == occasion_id)
        .one_or_none()
    )
    if not row:
        return False
    db.delete(row)
    db.commit()
    return True


def list_occasions(
    db: Session,
    user_id: str,
    limit: int = 50,
    upcoming_only: bool = False,
) -> List[GiftOccasion]:
    query = db.query(GiftOccasion).filter(GiftOccasion.user_id == user_id)
    if upcoming_only:
        query = query.order_by(GiftOccasion.occasion_date.asc())
    else:
        query = query.order_by(GiftOccasion.created_at.desc())
    return query.limit(limit).all()


def create_idea(
    db: Session,
    user_id: str,
    occasion_id: Optional[int],
    title: str,
    description: Optional[str] = None,
    link_url: Optional[str] = None,
    price: Optional[float] = None,
    currency: Optional[str] = None,
    status: Optional[str] = "idea",
    tags: Optional[List[str]] = None,
    source: Optional[str] = None,
) -> GiftIdea:
    row = GiftIdea(
        user_id=user_id,
        occasion_id=occasion_id,
        title=title,
        description=description,
        link_url=link_url,
        price=price,
        currency=currency,
        status=status or "idea",
        tags_json=_dump_tags(tags),
        source=source,
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def update_idea(
    db: Session,
    user_id: str,
    idea_id: int,
    **fields: Any,
) -> Optional[GiftIdea]:
    row = (
        db.query(GiftIdea)
        .filter(GiftIdea.user_id == user_id, GiftIdea.id == idea_id)
        .one_or_none()
    )
    if not row:
        return None

    for key, value in fields.items():
        if value is None:
            continue
        if key == "tags":
            row.tags_json = _dump_tags(value)
            continue
        if hasattr(row, key):
            setattr(row, key, value)

    row.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(row)
    return row


def delete_idea(db: Session, user_id: str, idea_id: int) -> bool:
    row = (
        db.query(GiftIdea)
        .filter(GiftIdea.user_id == user_id, GiftIdea.id == idea_id)
        .one_or_none()
    )
    if not row:
        return False
    db.delete(row)
    db.commit()
    return True


def list_ideas(
    db: Session,
    user_id: str,
    occasion_id: Optional[int] = None,
    status: Optional[str] = None,
    limit: int = 50,
) -> List[GiftIdea]:
    query = db.query(GiftIdea).filter(GiftIdea.user_id == user_id)
    if occasion_id:
        query = query.filter(GiftIdea.occasion_id == occasion_id)
    if status:
        query = query.filter(GiftIdea.status == status)
    return query.order_by(GiftIdea.created_at.desc()).limit(limit).all()
