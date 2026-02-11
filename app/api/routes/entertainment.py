from __future__ import annotations

from typing import Optional
from datetime import datetime

from fastapi import APIRouter, Depends, HTTPException, Request
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.middleware.rate_limiter import rate_limit_user
from app.schemas.entertainment import (
    EntertainmentItemCreate,
    EntertainmentItemUpdate,
    EntertainmentConsumptionCreate,
    EntertainmentRecommendationRequest,
)
from app.schemas.events import (
    EventDiscoverRequest,
    EventCreate,
    EventUpdate,
    EventBookingCreate,
    EventBookingUpdate,
)
from app.services.entertainment_service import (
    create_item,
    update_item,
    list_items,
    delete_item,
    log_consumption,
    list_consumption,
    serialize_item,
    serialize_consumption,
    get_recommendations,
)
from app.services.entertainment_events_service import (
    create_event,
    update_event,
    list_events,
    delete_event,
    create_booking,
    update_booking,
    list_bookings,
    serialize_event,
    serialize_booking,
    discover_events,
)
from app.services.discover_provider import DiscoverNotConfiguredError
from app.services.proposals import create_proposal_with_link
from app.db.models import EntertainmentEvent


router = APIRouter(prefix="/entertainment", tags=["entertainment"])


@rate_limit_user()
@router.post("/items")
def add_item(request: Request, payload: EntertainmentItemCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    row = create_item(db, **payload.model_dump())
    return {"ok": True, "item": serialize_item(row)}


@rate_limit_user()
@router.get("/items")
def list_items_endpoint(
    request: Request,
    user_id: str,
    limit: int = 50,
    status: Optional[str] = None,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    rows = list_items(db, user_id, limit=limit, status=status)
    return {"ok": True, "items": [serialize_item(r) for r in rows]}


@rate_limit_user()
@router.patch("/items/{item_id}")
def update_item_endpoint(
    request: Request,
    item_id: int,
    payload: EntertainmentItemUpdate,
    db: Session = Depends(get_db),
):
    row = update_item(db, payload.user_id, item_id, **payload.model_dump(exclude={"user_id"}))
    if not row:
        raise HTTPException(status_code=404, detail="Item not found")
    return {"ok": True, "item": serialize_item(row)}


@rate_limit_user()
@router.delete("/items/{item_id}")
def delete_item_endpoint(request: Request, item_id: int, user_id: str, db: Session = Depends(get_db)):
    ok = delete_item(db, user_id, item_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Item not found")
    return {"ok": True}


@rate_limit_user()
@router.post("/consumption")
def log_consumption_endpoint(
    request: Request,
    payload: EntertainmentConsumptionCreate,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, payload.user_id)
    row = log_consumption(db, **payload.model_dump())
    if not row:
        raise HTTPException(status_code=404, detail="Item not found")
    return {"ok": True, "consumption": serialize_consumption(row)}


@rate_limit_user()
@router.get("/consumption")
def list_consumption_endpoint(
    request: Request,
    user_id: str,
    item_id: Optional[int] = None,
    limit: int = 100,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    rows = list_consumption(db, user_id, item_id=item_id, limit=limit)
    return {"ok": True, "consumption": [serialize_consumption(r) for r in rows]}


@rate_limit_user()
@router.post("/recommendations")
async def recommend_content(
    request: Request,
    payload: EntertainmentRecommendationRequest,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, payload.user_id)
    try:
        results = await get_recommendations(
            db,
            user_id=payload.user_id,
            query=payload.query,
            content_type=payload.content_type,
            max_results=payload.max_results or 6,
            save=bool(payload.save),
        )
    except DiscoverNotConfiguredError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    return {"ok": True, "recommendations": results}


@rate_limit_user()
@router.post("/events")
def add_event(request: Request, payload: EventCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    row = create_event(db, **payload.model_dump())
    return {"ok": True, "event": serialize_event(row)}


@rate_limit_user()
@router.get("/events")
def list_events_endpoint(
    request: Request,
    user_id: str,
    status: Optional[str] = None,
    limit: int = 50,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    rows = list_events(db, user_id, status=status, limit=limit)
    return {"ok": True, "events": [serialize_event(r) for r in rows]}


@rate_limit_user()
@router.patch("/events/{event_id}")
def update_event_endpoint(
    request: Request,
    event_id: int,
    payload: EventUpdate,
    db: Session = Depends(get_db),
):
    row = update_event(db, payload.user_id, event_id, **payload.model_dump(exclude={"user_id"}))
    if not row:
        raise HTTPException(status_code=404, detail="Event not found")
    return {"ok": True, "event": serialize_event(row)}


@rate_limit_user()
@router.delete("/events/{event_id}")
def delete_event_endpoint(request: Request, event_id: int, user_id: str, db: Session = Depends(get_db)):
    ok = delete_event(db, user_id, event_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Event not found")
    return {"ok": True}


@rate_limit_user()
@router.post("/events/discover")
async def discover_events_endpoint(
    request: Request,
    payload: EventDiscoverRequest,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, payload.user_id)
    try:
        results = await discover_events(
            db,
            user_id=payload.user_id,
            query=payload.query,
            location=payload.location,
            start_date=payload.start_date,
            end_date=payload.end_date,
            max_results=payload.max_results or 6,
            save=bool(payload.save),
        )
    except DiscoverNotConfiguredError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    return {"ok": True, "events": results}


@rate_limit_user()
@router.post("/events/{event_id}/proposal")
def propose_event_booking(
    request: Request,
    event_id: int,
    payload: EventBookingCreate,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, payload.user_id)
    event = (
        db.query(EntertainmentEvent)
        .filter(EntertainmentEvent.user_id == payload.user_id, EntertainmentEvent.id == event_id)
        .one_or_none()
    )
    if not event:
        raise HTTPException(status_code=404, detail="Event not found")

    booking = create_booking(
        db,
        user_id=payload.user_id,
        event_id=event.id,
        quantity=payload.quantity or 1,
        total_price=payload.total_price,
        currency=payload.currency,
        ticket_delivery=payload.ticket_delivery,
        notes=payload.notes,
        status="pending_approval" if payload.require_approval else "approved",
    )

    proposal = None
    if payload.require_approval:
        proposal_payload = {
            "event_id": event.id,
            "title": event.title,
            "event_url": event.external_url,
            "quantity": booking.quantity,
            "total_price": booking.total_price,
            "currency": booking.currency,
        }
        proposal = create_proposal_with_link(
            db=db,
            user_id=payload.user_id,
            proposal_type="event_ticket",
            payload=proposal_payload,
            ttl_hours=24,
        )
        booking.proposal_id = int(proposal["proposal_id"])
        booking.status = "pending_approval"
        booking.updated_at = datetime.utcnow()
        db.commit()
        db.refresh(booking)

    return {"ok": True, "booking": serialize_booking(booking), "proposal": proposal}


@rate_limit_user()
@router.get("/events/bookings")
def list_event_bookings(
    request: Request,
    user_id: str,
    status: Optional[str] = None,
    limit: int = 50,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    rows = list_bookings(db, user_id, status=status, limit=limit)
    return {"ok": True, "bookings": [serialize_booking(r) for r in rows]}


@rate_limit_user()
@router.patch("/events/bookings/{booking_id}")
def update_event_booking(
    request: Request,
    booking_id: int,
    payload: EventBookingUpdate,
    db: Session = Depends(get_db),
):
    row = update_booking(
        db,
        payload.user_id,
        booking_id,
        status=payload.status,
        ticket_delivery=payload.ticket_delivery,
        notes=payload.notes,
        metadata=payload.metadata,
    )
    if not row:
        raise HTTPException(status_code=404, detail="Booking not found")
    return {"ok": True, "booking": serialize_booking(row)}
