from __future__ import annotations

import json
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional, Tuple

from sqlalchemy.orm import Session

from app.core.config import settings
from app.db.models import (
    RelationshipProfile,
    RelationshipInteraction,
    Contact,
    NotificationQueue,
)


DEDUP_WINDOW_HOURS = 24


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


def _load_tags(value: Optional[str]) -> List[str]:
    if not value:
        return []
    try:
        data = json.loads(value)
        if isinstance(data, list):
            return [str(item) for item in data]
    except Exception:
        return []
    return []


def _compute_next_checkin(base_time: datetime, cadence_days: int) -> datetime:
    return base_time + timedelta(days=cadence_days)


def _get_cadence_days(profile: RelationshipProfile) -> int:
    return profile.cadence_days or settings.RELATIONSHIP_DEFAULT_CADENCE_DAYS


def serialize_profile(
    row: RelationshipProfile,
    contact: Optional[Contact] = None,
    stats: Optional[Dict[str, Any]] = None,
) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "contact_id": row.contact_id,
        "relationship": row.relationship,
        "priority": row.priority,
        "cadence_days": row.cadence_days,
        "preferred_channel": row.preferred_channel,
        "tags": _load_tags(row.tags_json),
        "notes": row.notes,
        "metadata": _load_json(row.metadata_json),
        "last_interaction_at": row.last_interaction_at.isoformat() if row.last_interaction_at else None,
        "last_inbound_at": row.last_inbound_at.isoformat() if row.last_inbound_at else None,
        "last_outbound_at": row.last_outbound_at.isoformat() if row.last_outbound_at else None,
        "next_checkin_at": row.next_checkin_at.isoformat() if row.next_checkin_at else None,
        "created_at": row.created_at.isoformat() if row.created_at else None,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
        "contact": {
            "id": contact.id,
            "name": contact.name,
            "phone": contact.phone,
            "email": contact.email,
            "tags": _load_tags(contact.tags_json),
        }
        if contact
        else None,
        "stats": stats or {},
    }


def _interaction_stats(db: Session, user_id: str, contact_id: int, window_days: int = 30) -> Dict[str, Any]:
    since = datetime.utcnow() - timedelta(days=window_days)
    base = (
        db.query(RelationshipInteraction)
        .filter(
            RelationshipInteraction.user_id == user_id,
            RelationshipInteraction.contact_id == contact_id,
            RelationshipInteraction.occurred_at >= since,
        )
    )
    total = base.count()
    inbound = base.filter(RelationshipInteraction.direction == "inbound").count()
    outbound = base.filter(RelationshipInteraction.direction == "outbound").count()
    return {
        "window_days": window_days,
        "total": total,
        "inbound": inbound,
        "outbound": outbound,
    }


def _ensure_profile(db: Session, user_id: str, contact_id: int) -> RelationshipProfile:
    row = (
        db.query(RelationshipProfile)
        .filter(RelationshipProfile.user_id == user_id, RelationshipProfile.contact_id == contact_id)
        .one_or_none()
    )
    if row:
        return row
    row = RelationshipProfile(
        user_id=user_id,
        contact_id=contact_id,
        cadence_days=settings.RELATIONSHIP_DEFAULT_CADENCE_DAYS,
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    row.next_checkin_at = _compute_next_checkin(datetime.utcnow(), _get_cadence_days(row))
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def upsert_profile(
    db: Session,
    user_id: str,
    contact_id: int,
    relationship: Optional[str] = None,
    priority: Optional[int] = None,
    cadence_days: Optional[int] = None,
    preferred_channel: Optional[str] = None,
    tags: Optional[List[str]] = None,
    notes: Optional[str] = None,
    metadata: Optional[Dict[str, Any]] = None,
    next_checkin_at: Optional[datetime] = None,
) -> RelationshipProfile:
    row = (
        db.query(RelationshipProfile)
        .filter(RelationshipProfile.user_id == user_id, RelationshipProfile.contact_id == contact_id)
        .one_or_none()
    )
    is_new = False
    if not row:
        row = RelationshipProfile(user_id=user_id, contact_id=contact_id)
        is_new = True

    if relationship is not None:
        row.relationship = relationship
    if priority is not None:
        row.priority = priority
    if cadence_days is not None:
        row.cadence_days = cadence_days
    elif row.cadence_days is None:
        row.cadence_days = settings.RELATIONSHIP_DEFAULT_CADENCE_DAYS
    if preferred_channel is not None:
        row.preferred_channel = preferred_channel
    if notes is not None:
        row.notes = notes
    if tags is not None:
        row.tags_json = _dump_tags(tags)
    if metadata is not None:
        existing = _load_json(row.metadata_json)
        existing.update(metadata)
        row.metadata_json = _dump_json(existing)
    if next_checkin_at is not None:
        row.next_checkin_at = next_checkin_at

    if is_new:
        row.created_at = datetime.utcnow()
        db.add(row)

    if row.next_checkin_at is None:
        base_time = row.last_interaction_at or datetime.utcnow()
        row.next_checkin_at = _compute_next_checkin(base_time, _get_cadence_days(row))

    row.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(row)
    return row


def update_profile(
    db: Session,
    user_id: str,
    profile_id: int,
    **fields: Any,
) -> Optional[RelationshipProfile]:
    row = (
        db.query(RelationshipProfile)
        .filter(RelationshipProfile.user_id == user_id, RelationshipProfile.id == profile_id)
        .one_or_none()
    )
    if not row:
        return None

    cadence_changed = False

    for key, value in fields.items():
        if value is None:
            continue
        if key == "tags":
            row.tags_json = _dump_tags(value)
            continue
        if key == "metadata":
            existing = _load_json(row.metadata_json)
            existing.update(value)
            row.metadata_json = _dump_json(existing)
            continue
        if key == "cadence_days":
            cadence_changed = True
        if hasattr(row, key):
            setattr(row, key, value)

    if cadence_changed:
        base_time = row.last_interaction_at or datetime.utcnow()
        row.next_checkin_at = _compute_next_checkin(base_time, _get_cadence_days(row))

    row.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(row)
    return row


def list_profiles(
    db: Session,
    user_id: str,
    limit: int = 100,
) -> List[Tuple[RelationshipProfile, Optional[Contact]]]:
    return (
        db.query(RelationshipProfile, Contact)
        .join(Contact, Contact.id == RelationshipProfile.contact_id, isouter=True)
        .filter(RelationshipProfile.user_id == user_id)
        .order_by(RelationshipProfile.updated_at.desc())
        .limit(limit)
        .all()
    )


def log_interaction(
    db: Session,
    user_id: str,
    contact_id: int,
    direction: str,
    channel: Optional[str] = None,
    summary: Optional[str] = None,
    occurred_at: Optional[datetime] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> RelationshipInteraction:
    occurred_at = occurred_at or datetime.utcnow()
    direction = (direction or "outbound").lower()
    if direction not in {"inbound", "outbound"}:
        direction = "outbound"

    interaction = RelationshipInteraction(
        user_id=user_id,
        contact_id=contact_id,
        direction=direction,
        channel=channel,
        summary=summary,
        metadata_json=_dump_json(metadata),
        occurred_at=occurred_at,
        created_at=datetime.utcnow(),
    )
    db.add(interaction)

    profile = _ensure_profile(db, user_id, contact_id)

    if not profile.last_interaction_at or occurred_at >= profile.last_interaction_at:
        profile.last_interaction_at = occurred_at
    if direction == "inbound":
        if not profile.last_inbound_at or occurred_at >= profile.last_inbound_at:
            profile.last_inbound_at = occurred_at
    else:
        if not profile.last_outbound_at or occurred_at >= profile.last_outbound_at:
            profile.last_outbound_at = occurred_at

    cadence_days = _get_cadence_days(profile)
    profile.next_checkin_at = _compute_next_checkin(profile.last_interaction_at or occurred_at, cadence_days)
    profile.updated_at = datetime.utcnow()

    db.commit()
    db.refresh(interaction)
    return interaction


def _compute_due_at(profile: RelationshipProfile) -> datetime:
    cadence_days = _get_cadence_days(profile)
    base_time = profile.last_interaction_at or profile.created_at or datetime.utcnow()
    return profile.next_checkin_at or _compute_next_checkin(base_time, cadence_days)


def get_suggestions(
    db: Session,
    user_id: str,
    limit: int = 10,
    due_only: bool = True,
) -> List[Dict[str, Any]]:
    now = datetime.utcnow()
    suggestions: List[Dict[str, Any]] = []

    rows = list_profiles(db, user_id, limit=limit * 3)
    for profile, contact in rows:
        due_at = _compute_due_at(profile)
        if due_only and due_at > now:
            continue

        days_since = None
        if profile.last_interaction_at:
            days_since = (now - profile.last_interaction_at).days

        stats = _interaction_stats(db, user_id, profile.contact_id)
        suggestion = {
            "profile": serialize_profile(profile, contact, stats=stats),
            "due_at": due_at.isoformat(),
            "days_since_last": days_since,
            "recommended_channel": profile.preferred_channel or "whatsapp",
            "reason": "due_for_checkin" if profile.last_interaction_at else "no_recent_interaction",
        }
        suggestions.append(suggestion)
        if len(suggestions) >= limit:
            break

    return suggestions


def enqueue_relationship_reminders_for_user(db: Session, user_id: str, limit: int = 10) -> Dict[str, int]:
    suggestions = get_suggestions(db, user_id, limit=limit, due_only=True)
    now = datetime.utcnow()
    queued = 0
    skipped = 0

    for item in suggestions:
        profile = item["profile"]
        contact = profile.get("contact") or {}
        name = contact.get("name") or "contact"
        title = f"Reach out to {name}"

        dedup_since = now - timedelta(hours=DEDUP_WINDOW_HOURS)
        exists = (
            db.query(NotificationQueue)
            .filter(
                NotificationQueue.user_id == user_id,
                NotificationQueue.event_type == "relationship_checkin",
                NotificationQueue.title == title,
                NotificationQueue.created_at >= dedup_since,
                NotificationQueue.is_sent == False,
            )
            .first()
        )
        if exists:
            skipped += 1
            continue

        days_since = item.get("days_since_last")
        channel = item.get("recommended_channel")
        if days_since is None:
            message = f"No recent interaction logged. Suggested channel: {channel}."
        else:
            message = f"It has been {days_since} days since your last check-in. Suggested channel: {channel}."

        db.add(
            NotificationQueue(
                user_id=user_id,
                event_type="relationship_checkin",
                title=title,
                message=message,
                deep_link_url=None,
                is_sent=False,
            )
        )
        queued += 1

    if queued:
        db.commit()
    return {"queued": queued, "skipped": skipped, "suggestions": len(suggestions)}


def enqueue_relationship_reminders(db: Session, limit_per_user: int = 10) -> Dict[str, int]:
    results = {"queued": 0, "skipped": 0}
    user_ids = (
        db.query(RelationshipProfile.user_id)
        .distinct()
        .all()
    )
    for (user_id,) in user_ids:
        outcome = enqueue_relationship_reminders_for_user(db, user_id, limit=limit_per_user)
        results["queued"] += outcome.get("queued", 0)
        results["skipped"] += outcome.get("skipped", 0)
    return results
