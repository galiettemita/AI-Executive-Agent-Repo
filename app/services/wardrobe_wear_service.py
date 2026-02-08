# app/services/wardrobe_wear_service.py

from __future__ import annotations

from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional

from sqlalchemy import func
from sqlalchemy.orm import Session

from app.db.models import WardrobeItem, WardrobeWearEvent
from app.services.wardrobe_service import serialize_item


def log_wear_event(
    db: Session,
    user_id: str,
    item_id: int,
    worn_at: Optional[datetime] = None,
    source: str = "manual",
    notes: Optional[str] = None,
) -> Dict[str, Any]:
    item = (
        db.query(WardrobeItem)
        .filter(WardrobeItem.user_id == user_id, WardrobeItem.id == item_id)
        .one_or_none()
    )
    if not item:
        raise ValueError("Wardrobe item not found")

    ts = worn_at or datetime.utcnow()
    event = WardrobeWearEvent(
        user_id=user_id,
        wardrobe_item_id=item_id,
        worn_at=ts,
        source=source or "manual",
        notes=notes,
        created_at=datetime.utcnow(),
    )
    db.add(event)

    item.last_worn_at = ts
    item.wear_count = (item.wear_count or 0) + 1
    item.updated_at = datetime.utcnow()

    db.commit()
    db.refresh(item)
    db.refresh(event)
    return {"event_id": event.id, "item": serialize_item(item)}


def list_wear_events(
    db: Session,
    user_id: str,
    item_id: int,
    limit: int = 50,
) -> List[Dict[str, Any]]:
    rows = (
        db.query(WardrobeWearEvent)
        .filter(
            WardrobeWearEvent.user_id == user_id,
            WardrobeWearEvent.wardrobe_item_id == item_id,
        )
        .order_by(WardrobeWearEvent.worn_at.desc())
        .limit(limit)
        .all()
    )
    return [
        {
            "id": r.id,
            "wardrobe_item_id": r.wardrobe_item_id,
            "worn_at": r.worn_at.isoformat() if r.worn_at else None,
            "source": r.source,
            "notes": r.notes,
            "created_at": r.created_at.isoformat() if r.created_at else None,
        }
        for r in rows
    ]


def get_wear_stats(
    db: Session,
    user_id: str,
    lookback_days: int = 90,
    limit: int = 10,
) -> Dict[str, Any]:
    since = datetime.utcnow() - timedelta(days=lookback_days)
    rows = (
        db.query(
            WardrobeWearEvent.wardrobe_item_id,
            func.count(WardrobeWearEvent.id).label("wear_count"),
            func.max(WardrobeWearEvent.worn_at).label("last_worn_at"),
        )
        .filter(WardrobeWearEvent.user_id == user_id, WardrobeWearEvent.worn_at >= since)
        .group_by(WardrobeWearEvent.wardrobe_item_id)
        .order_by(func.count(WardrobeWearEvent.id).desc())
        .limit(limit)
        .all()
    )
    stats = []
    for item_id, wear_count, last_worn_at in rows:
        stats.append(
            {
                "wardrobe_item_id": item_id,
                "wear_count": int(wear_count or 0),
                "last_worn_at": last_worn_at.isoformat() if last_worn_at else None,
            }
        )
    return {"lookback_days": lookback_days, "items": stats}


def find_rotation_candidates(
    db: Session,
    user_id: str,
    min_days_since_worn: int,
    limit: int = 5,
    cooldown_days: int = 7,
) -> List[WardrobeItem]:
    now = datetime.utcnow()
    cutoff = now - timedelta(days=min_days_since_worn)
    cooldown_cutoff = now - timedelta(days=cooldown_days)

    query = (
        db.query(WardrobeItem)
        .filter(WardrobeItem.user_id == user_id)
        .filter(
            (WardrobeItem.last_worn_at.is_(None)) | (WardrobeItem.last_worn_at <= cutoff)
        )
        .filter(
            (WardrobeItem.last_rotation_notified_at.is_(None))
            | (WardrobeItem.last_rotation_notified_at <= cooldown_cutoff)
        )
        .order_by(
            WardrobeItem.last_worn_at.is_(None).desc(),
            WardrobeItem.last_worn_at.asc(),
            WardrobeItem.created_at.asc(),
        )
        .limit(limit)
    )
    return query.all()
