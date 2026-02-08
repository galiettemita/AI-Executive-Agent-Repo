# app/services/wardrobe_rotation.py

from __future__ import annotations

from datetime import datetime
from typing import Any, Dict, List

from sqlalchemy.orm import Session

from app.core.config import settings
from app.db.models import NotificationQueue, WardrobeItem
from app.services.wardrobe_wear_service import find_rotation_candidates


def build_rotation_message(items: List[WardrobeItem]) -> str:
    names = [item.name for item in items if item.name]
    if not names:
        return "Time to rotate a few pieces in your wardrobe."
    formatted = ", ".join(names[:5])
    if len(names) > 5:
        formatted += f", and {len(names) - 5} more"
    return f"You haven't worn these in a while: {formatted}."


def enqueue_rotation_notification(
    db: Session,
    user_id: str,
    items: List[WardrobeItem],
) -> Dict[str, Any]:
    if not items:
        return {"ok": True, "queued": False, "items": []}

    message = build_rotation_message(items)
    notification = NotificationQueue(
        user_id=user_id,
        watch_item_id=None,
        event_type="wardrobe_rotation",
        title="Wardrobe rotation",
        message=message,
        deep_link_url=None,
        is_sent=False,
    )
    db.add(notification)
    now = datetime.utcnow()
    for item in items:
        item.last_rotation_notified_at = now
    db.commit()
    return {"ok": True, "queued": True, "items": [item.id for item in items]}


def run_rotation_for_user(
    db: Session,
    user_id: str,
    min_days_since_worn: int | None = None,
    limit: int | None = None,
    cooldown_days: int | None = None,
    notify: bool = False,
) -> Dict[str, Any]:
    min_days = min_days_since_worn or settings.WARDROBE_ROTATION_DAYS
    max_items = limit or settings.WARDROBE_ROTATION_MAX_ITEMS
    cooldown = cooldown_days or settings.WARDROBE_ROTATION_COOLDOWN_DAYS

    candidates = find_rotation_candidates(
        db=db,
        user_id=user_id,
        min_days_since_worn=min_days,
        limit=max_items,
        cooldown_days=cooldown,
    )

    response = {
        "user_id": user_id,
        "min_days_since_worn": min_days,
        "items": [
            {
                "id": item.id,
                "name": item.name,
                "last_worn_at": item.last_worn_at.isoformat() if item.last_worn_at else None,
                "wear_count": item.wear_count,
            }
            for item in candidates
        ],
    }

    if notify:
        enqueue_rotation_notification(db, user_id, candidates)
        response["notification_queued"] = bool(candidates)

    return response


def run_rotation_for_all_users(db: Session) -> Dict[str, int]:
    user_ids = [row[0] for row in db.query(WardrobeItem.user_id).distinct().all()]
    queued = 0
    for user_id in user_ids:
        result = run_rotation_for_user(db, user_id, notify=True)
        if result.get("notification_queued"):
            queued += 1
    return {"users": len(user_ids), "notifications": queued}
