from __future__ import annotations

import json
from datetime import date, datetime
from typing import Any, Dict, List, Optional

from sqlalchemy.orm import Session

from app.db.models import EntertainmentEvent, EntertainmentEventBooking
from app.services.discover_provider import discover_search, DiscoverNotConfiguredError


DEFAULT_STATUS = "interested"


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


def serialize_event(row: EntertainmentEvent) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "title": row.title,
        "event_type": row.event_type,
        "venue": row.venue,
        "location": row.location,
        "starts_at": row.starts_at.isoformat() if row.starts_at else None,
        "ends_at": row.ends_at.isoformat() if row.ends_at else None,
        "external_url": row.external_url,
        "provider": row.provider,
        "provider_event_id": row.provider_event_id,
        "price_min": row.price_min,
        "price_max": row.price_max,
        "currency": row.currency,
        "status": row.status,
        "metadata": _load_json(row.metadata_json, {}),
        "created_at": row.created_at.isoformat() if row.created_at else None,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }


def serialize_booking(row: EntertainmentEventBooking) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "event_id": row.event_id,
        "proposal_id": row.proposal_id,
        "transaction_id": row.transaction_id,
        "quantity": row.quantity,
        "total_price": row.total_price,
        "currency": row.currency,
        "status": row.status,
        "ticket_delivery": row.ticket_delivery,
        "notes": row.notes,
        "metadata": _load_json(row.metadata_json, {}),
        "created_at": row.created_at.isoformat() if row.created_at else None,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }


def create_event(
    db: Session,
    user_id: str,
    title: str,
    event_type: Optional[str] = None,
    venue: Optional[str] = None,
    location: Optional[str] = None,
    starts_at: Optional[datetime] = None,
    ends_at: Optional[datetime] = None,
    external_url: Optional[str] = None,
    provider: Optional[str] = None,
    provider_event_id: Optional[str] = None,
    price_min: Optional[float] = None,
    price_max: Optional[float] = None,
    currency: Optional[str] = None,
    status: Optional[str] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> EntertainmentEvent:
    row = EntertainmentEvent(
        user_id=user_id,
        title=title,
        event_type=event_type,
        venue=venue,
        location=location,
        starts_at=starts_at,
        ends_at=ends_at,
        external_url=external_url,
        provider=provider,
        provider_event_id=provider_event_id,
        price_min=price_min,
        price_max=price_max,
        currency=currency,
        status=status or DEFAULT_STATUS,
        metadata_json=_dump_json(metadata),
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def update_event(db: Session, user_id: str, event_id: int, **fields: Any) -> Optional[EntertainmentEvent]:
    row = (
        db.query(EntertainmentEvent)
        .filter(EntertainmentEvent.user_id == user_id, EntertainmentEvent.id == event_id)
        .one_or_none()
    )
    if not row:
        return None

    for key, value in fields.items():
        if value is None:
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


def list_events(db: Session, user_id: str, status: Optional[str] = None, limit: int = 50) -> List[EntertainmentEvent]:
    q = db.query(EntertainmentEvent).filter(EntertainmentEvent.user_id == user_id)
    if status:
        q = q.filter(EntertainmentEvent.status == status)
    return q.order_by(EntertainmentEvent.updated_at.desc()).limit(limit).all()


def delete_event(db: Session, user_id: str, event_id: int) -> bool:
    row = (
        db.query(EntertainmentEvent)
        .filter(EntertainmentEvent.user_id == user_id, EntertainmentEvent.id == event_id)
        .one_or_none()
    )
    if not row:
        return False
    db.delete(row)
    db.commit()
    return True


def create_booking(
    db: Session,
    user_id: str,
    event_id: Optional[int],
    quantity: int = 1,
    total_price: Optional[float] = None,
    currency: Optional[str] = None,
    ticket_delivery: Optional[str] = None,
    notes: Optional[str] = None,
    status: Optional[str] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> EntertainmentEventBooking:
    row = EntertainmentEventBooking(
        user_id=user_id,
        event_id=event_id,
        quantity=quantity or 1,
        total_price=total_price,
        currency=currency,
        status=status or "pending_approval",
        ticket_delivery=ticket_delivery,
        notes=notes,
        metadata_json=_dump_json(metadata),
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def update_booking(
    db: Session,
    user_id: str,
    booking_id: int,
    status: Optional[str] = None,
    ticket_delivery: Optional[str] = None,
    notes: Optional[str] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> Optional[EntertainmentEventBooking]:
    row = (
        db.query(EntertainmentEventBooking)
        .filter(EntertainmentEventBooking.user_id == user_id, EntertainmentEventBooking.id == booking_id)
        .one_or_none()
    )
    if not row:
        return None

    if status is not None:
        row.status = status
    if ticket_delivery is not None:
        row.ticket_delivery = ticket_delivery
    if notes is not None:
        row.notes = notes
    if metadata is not None:
        existing = _load_json(row.metadata_json, {})
        existing.update(metadata)
        row.metadata_json = _dump_json(existing)

    row.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(row)
    return row


def list_bookings(db: Session, user_id: str, status: Optional[str] = None, limit: int = 50) -> List[EntertainmentEventBooking]:
    q = db.query(EntertainmentEventBooking).filter(EntertainmentEventBooking.user_id == user_id)
    if status:
        q = q.filter(EntertainmentEventBooking.status == status)
    return q.order_by(EntertainmentEventBooking.updated_at.desc()).limit(limit).all()


def _build_event_search_query(query: Optional[str], location: Optional[str], start_date: Optional[date], end_date: Optional[date]) -> str:
    base = "events"
    if query:
        base = f"{query} {base}".strip()
    if location:
        base = f"{base} in {location}".strip()
    if start_date and end_date:
        base = f"{base} {start_date.isoformat()} to {end_date.isoformat()}"
    elif start_date:
        base = f"{base} {start_date.isoformat()}"
    return base


async def discover_events(
    db: Session,
    *,
    user_id: str,
    query: Optional[str] = None,
    location: Optional[str] = None,
    start_date: Optional[date] = None,
    end_date: Optional[date] = None,
    max_results: int = 6,
    save: bool = False,
) -> Dict[str, Any]:
    search_query = _build_event_search_query(query, location, start_date, end_date)

    results = await discover_search(search_query, max_results=max_results)
    payload = {
        "query": search_query,
        "results": [r.model_dump() for r in results],
    }

    created_ids: List[int] = []
    if save and results:
        for r in results:
            data = r.model_dump() if hasattr(r, "model_dump") else {}
            url = getattr(r, "url", None) or data.get("url")
            title = getattr(r, "title", None) or data.get("title") or "Untitled event"
            snippet = getattr(r, "snippet", None) or data.get("snippet")
            source = getattr(r, "source", None) or data.get("source")
            domain = getattr(r, "retailer_domain", None) or data.get("retailer_domain")

            existing = None
            if url:
                existing = (
                    db.query(EntertainmentEvent)
                    .filter(EntertainmentEvent.user_id == user_id, EntertainmentEvent.external_url == url)
                    .one_or_none()
                )
            if existing:
                continue

            row = create_event(
                db,
                user_id=user_id,
                title=title,
                event_type=query,
                location=location,
                external_url=url,
                provider=source,
                metadata={"snippet": snippet, "domain": domain},
            )
            created_ids.append(row.id)

    payload["saved_ids"] = created_ids
    return payload
