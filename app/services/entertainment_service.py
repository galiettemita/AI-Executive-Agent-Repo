from __future__ import annotations

import json
from datetime import datetime
from typing import Any, Dict, List, Optional

from sqlalchemy.orm import Session

from app.db.models import EntertainmentItem, EntertainmentConsumption
from app.services.discover_provider import discover_search, DiscoverNotConfiguredError


DEFAULT_STATUS = "planned"


def _dump_json(value: Optional[Any]) -> str:
    if value is None:
        return "{}"
    return json.dumps(value, ensure_ascii=False)


def _load_json(value: Optional[str], default: Any) -> Any:
    if not value:
        return default
    try:
        return json.loads(value)
    except Exception:
        return default


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


def serialize_item(row: EntertainmentItem) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "title": row.title,
        "content_type": row.content_type,
        "status": row.status,
        "rating": row.rating,
        "external_url": row.external_url,
        "source": row.source,
        "tags": _load_tags(row.tags_json),
        "metadata": _load_json(row.metadata_json, {}),
        "notes": row.notes,
        "last_consumed_at": row.last_consumed_at.isoformat() if row.last_consumed_at else None,
        "created_at": row.created_at.isoformat() if row.created_at else None,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }


def serialize_consumption(row: EntertainmentConsumption) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "item_id": row.item_id,
        "event_type": row.event_type,
        "duration_minutes": row.duration_minutes,
        "notes": row.notes,
        "metadata": _load_json(row.metadata_json, {}),
        "occurred_at": row.occurred_at.isoformat() if row.occurred_at else None,
        "created_at": row.created_at.isoformat() if row.created_at else None,
    }


def create_item(
    db: Session,
    user_id: str,
    title: str,
    content_type: str,
    status: Optional[str] = None,
    rating: Optional[float] = None,
    external_url: Optional[str] = None,
    source: Optional[str] = None,
    tags: Optional[List[str]] = None,
    metadata: Optional[Dict[str, Any]] = None,
    notes: Optional[str] = None,
) -> EntertainmentItem:
    row = EntertainmentItem(
        user_id=user_id,
        title=title,
        content_type=content_type,
        status=status or DEFAULT_STATUS,
        rating=rating,
        external_url=external_url,
        source=source,
        tags_json=_dump_tags(tags),
        metadata_json=_dump_json(metadata),
        notes=notes,
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def update_item(db: Session, user_id: str, item_id: int, **fields: Any) -> Optional[EntertainmentItem]:
    row = (
        db.query(EntertainmentItem)
        .filter(EntertainmentItem.user_id == user_id, EntertainmentItem.id == item_id)
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
        if key == "metadata":
            existing = _load_json(row.metadata_json, {})
            existing.update(value)
            row.metadata_json = _dump_json(existing)
            continue
        if hasattr(row, key):
            setattr(row, key, value)

    row.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(row)
    return row


def list_items(db: Session, user_id: str, limit: int = 50, status: Optional[str] = None) -> List[EntertainmentItem]:
    q = db.query(EntertainmentItem).filter(EntertainmentItem.user_id == user_id)
    if status:
        q = q.filter(EntertainmentItem.status == status)
    return q.order_by(EntertainmentItem.updated_at.desc()).limit(limit).all()


def delete_item(db: Session, user_id: str, item_id: int) -> bool:
    row = (
        db.query(EntertainmentItem)
        .filter(EntertainmentItem.user_id == user_id, EntertainmentItem.id == item_id)
        .one_or_none()
    )
    if not row:
        return False
    db.delete(row)
    db.commit()
    return True


def log_consumption(
    db: Session,
    user_id: str,
    item_id: int,
    event_type: Optional[str] = None,
    duration_minutes: Optional[int] = None,
    notes: Optional[str] = None,
    occurred_at: Optional[datetime] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> Optional[EntertainmentConsumption]:
    item = (
        db.query(EntertainmentItem)
        .filter(EntertainmentItem.user_id == user_id, EntertainmentItem.id == item_id)
        .one_or_none()
    )
    if not item:
        return None

    event = EntertainmentConsumption(
        user_id=user_id,
        item_id=item_id,
        event_type=(event_type or "watched"),
        duration_minutes=duration_minutes,
        notes=notes,
        metadata_json=_dump_json(metadata),
        occurred_at=occurred_at or datetime.utcnow(),
        created_at=datetime.utcnow(),
    )
    db.add(event)

    item.last_consumed_at = event.occurred_at
    if item.status == "planned":
        item.status = "in_progress"
    item.updated_at = datetime.utcnow()

    db.commit()
    db.refresh(event)
    return event


def list_consumption(
    db: Session,
    user_id: str,
    item_id: Optional[int] = None,
    limit: int = 100,
) -> List[EntertainmentConsumption]:
    q = db.query(EntertainmentConsumption).filter(EntertainmentConsumption.user_id == user_id)
    if item_id:
        q = q.filter(EntertainmentConsumption.item_id == item_id)
    return q.order_by(EntertainmentConsumption.occurred_at.desc()).limit(limit).all()


async def get_recommendations(
    db: Session,
    user_id: str,
    query: str,
    content_type: Optional[str] = None,
    max_results: int = 6,
    save: bool = False,
) -> Dict[str, Any]:
    search_query = query.strip()
    if content_type:
        search_query = f"{content_type} {search_query}".strip()

    results = await discover_search(search_query, max_results=max_results)
    payload = {
        "query": search_query,
        "results": [r.model_dump() for r in results],
    }

    created_ids: List[int] = []
    if save and results:
        for r in results:
            # Support both DiscoverResult and test stubs
            data = r.model_dump() if hasattr(r, "model_dump") else {}
            url = getattr(r, "url", None) or data.get("url")
            title = getattr(r, "title", None) or data.get("title") or "Untitled"
            snippet = getattr(r, "snippet", None) or data.get("snippet")
            source = getattr(r, "source", None) or data.get("source")
            domain = getattr(r, "retailer_domain", None) or data.get("retailer_domain")

            existing = None
            if url:
                existing = (
                    db.query(EntertainmentItem)
                    .filter(
                        EntertainmentItem.user_id == user_id,
                        EntertainmentItem.external_url == url,
                    )
                    .one_or_none()
                )
            if existing:
                continue
            row = create_item(
                db,
                user_id=user_id,
                title=title,
                content_type=content_type or "recommendation",
                status="planned",
                external_url=url,
                source=source,
                metadata={"snippet": snippet, "domain": domain},
                tags=[content_type] if content_type else None,
            )
            created_ids.append(row.id)

    payload["saved_ids"] = created_ids
    return payload
