# backend/app/api/routes/gifts.py

from __future__ import annotations

from typing import Optional
from datetime import datetime

from fastapi import APIRouter, Depends, HTTPException, Request
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.middleware.rate_limiter import rate_limit_user
from app.schemas.gifts import (
    GiftOccasionCreate,
    GiftOccasionUpdate,
    GiftIdeaCreate,
    GiftIdeaUpdate,
    GiftRecommendationRequest,
    GiftThankYouRequest,
    GiftPurchaseProposalRequest,
    GiftRetailerCreate,
    GiftRetailerUpdate,
    GiftOrderCreate,
    GiftOrderUpdate,
    GiftOrderAuthorize,
    GiftOrderEventCreate,
    GiftOrderRefundRequest,
)
from app.services.gift_service import (
    create_occasion,
    update_occasion,
    delete_occasion,
    list_occasions,
    serialize_occasion,
    create_idea,
    update_idea,
    delete_idea,
    list_ideas,
    serialize_idea,
)
from app.services.gift_recommendations import get_gift_recommendations, DiscoverNotConfiguredError
from app.services.gift_thankyou import generate_thank_you_note
from app.services.gift_reminders import enqueue_gift_reminders_for_user
from app.services.proposals import create_proposal_with_link
from app.db.models import GiftIdea, GiftOccasion
from app.core.config import settings
from app.services.gift_orders import (
    create_retailer,
    list_retailers,
    update_retailer,
    delete_retailer,
    serialize_retailer,
    is_retailer_allowed,
    create_order,
    list_orders,
    get_order,
    update_order,
    authorize_order,
    record_order_event,
    list_order_events,
    refund_order,
    serialize_order,
    serialize_order_event,
)


router = APIRouter(prefix="/gifts", tags=["gifts"])


@rate_limit_user()
@router.post("/occasions")
def add_occasion(request: Request, payload: GiftOccasionCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    row = create_occasion(db, **payload.model_dump())
    return {"ok": True, "occasion": serialize_occasion(row)}


@rate_limit_user()
@router.get("/occasions")
def list_occasions_endpoint(
    request: Request,
    user_id: str,
    limit: int = 50,
    upcoming_only: bool = False,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    rows = list_occasions(db, user_id, limit=limit, upcoming_only=upcoming_only)
    return {"ok": True, "occasions": [serialize_occasion(r) for r in rows]}


@rate_limit_user()
@router.patch("/occasions/{occasion_id}")
def update_occasion_endpoint(
    request: Request,
    occasion_id: int,
    payload: GiftOccasionUpdate,
    db: Session = Depends(get_db),
):
    row = update_occasion(db, payload.user_id, occasion_id, **payload.model_dump(exclude={"user_id"}))
    if not row:
        raise HTTPException(status_code=404, detail="Occasion not found")
    return {"ok": True, "occasion": serialize_occasion(row)}


@rate_limit_user()
@router.delete("/occasions/{occasion_id}")
def delete_occasion_endpoint(request: Request, occasion_id: int, user_id: str, db: Session = Depends(get_db)):
    ok = delete_occasion(db, user_id, occasion_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Occasion not found")
    return {"ok": True}


@rate_limit_user()
@router.post("/ideas")
def add_idea(request: Request, payload: GiftIdeaCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    row = create_idea(db, **payload.model_dump())
    return {"ok": True, "idea": serialize_idea(row)}


@rate_limit_user()
@router.get("/ideas")
def list_ideas_endpoint(
    request: Request,
    user_id: str,
    occasion_id: Optional[int] = None,
    status: Optional[str] = None,
    limit: int = 50,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    rows = list_ideas(db, user_id, occasion_id=occasion_id, status=status, limit=limit)
    return {"ok": True, "ideas": [serialize_idea(r) for r in rows]}


@rate_limit_user()
@router.patch("/ideas/{idea_id}")
def update_idea_endpoint(
    request: Request,
    idea_id: int,
    payload: GiftIdeaUpdate,
    db: Session = Depends(get_db),
):
    row = update_idea(db, payload.user_id, idea_id, **payload.model_dump(exclude={"user_id"}))
    if not row:
        raise HTTPException(status_code=404, detail="Idea not found")
    return {"ok": True, "idea": serialize_idea(row)}


@rate_limit_user()
@router.delete("/ideas/{idea_id}")
def delete_idea_endpoint(request: Request, idea_id: int, user_id: str, db: Session = Depends(get_db)):
    ok = delete_idea(db, user_id, idea_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Idea not found")
    return {"ok": True}


@rate_limit_user()
@router.post("/recommendations")
async def gift_recommendations(request: Request, payload: GiftRecommendationRequest, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    occasion = None
    if payload.occasion_id:
        occasion = (
            db.query(GiftOccasion)
            .filter(GiftOccasion.user_id == payload.user_id, GiftOccasion.id == payload.occasion_id)
            .one_or_none()
        )
    try:
        max_results = payload.max_results or settings.GIFT_SHOPPING_MAX_RESULTS
        data = await get_gift_recommendations(occasion, payload.query, max_results=max_results)
        return {"ok": True, "recommendations": data}
    except DiscoverNotConfiguredError as exc:
        raise HTTPException(status_code=501, detail=str(exc))
    except Exception as exc:
        raise HTTPException(status_code=502, detail=str(exc))


@rate_limit_user()
@router.post("/thank-you")
def gift_thank_you(request: Request, payload: GiftThankYouRequest, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    draft = generate_thank_you_note(
        db=db,
        user_id=payload.user_id,
        occasion_id=payload.occasion_id,
        gift_idea_id=payload.gift_idea_id,
        tone=payload.tone or "grateful",
        length=payload.length or "short",
        extra_notes=payload.extra_notes,
    )
    return {
        "ok": True,
        "draft": {
            "id": draft.id,
            "message": draft.message,
            "status": draft.status,
        },
    }


@rate_limit_user()
@router.post("/ideas/{idea_id}/proposal")
def create_purchase_proposal(
    request: Request,
    idea_id: int,
    payload: GiftPurchaseProposalRequest,
    db: Session = Depends(get_db),
):
    row = (
        db.query(GiftIdea)
        .filter(GiftIdea.user_id == payload.user_id, GiftIdea.id == idea_id)
        .one_or_none()
    )
    if not row:
        raise HTTPException(status_code=404, detail="Idea not found")
    payload_data = {
        "gift_idea_id": row.id,
        "title": row.title,
        "description": row.description,
        "link_url": row.link_url,
        "price": row.price,
        "currency": row.currency,
        "quantity": payload.quantity or 1,
        "notes": payload.notes,
    }
    proposal = create_proposal_with_link(
        db=db,
        user_id=payload.user_id,
        proposal_type="gift_purchase",
        payload=payload_data,
        ttl_hours=24,
    )
    return {"ok": True, "proposal": proposal}


@rate_limit_user()
@router.post("/reminders/run")
def run_gift_reminders(request: Request, user_id: str, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    result = enqueue_gift_reminders_for_user(db, user_id)
    return {"ok": True, "result": result}


@rate_limit_user()
@router.post("/retailers")
def add_retailer(request: Request, payload: GiftRetailerCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    row = create_retailer(db, payload.user_id, payload.domain, payload.status or "allowed", payload.notes)
    return {"ok": True, "retailer": serialize_retailer(row)}


@rate_limit_user()
@router.get("/retailers")
def list_retailers_endpoint(
    request: Request,
    user_id: str,
    status: Optional[str] = None,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    rows = list_retailers(db, user_id, status=status)
    return {"ok": True, "retailers": [serialize_retailer(r) for r in rows]}


@rate_limit_user()
@router.patch("/retailers/{retailer_id}")
def update_retailer_endpoint(
    request: Request,
    retailer_id: int,
    payload: GiftRetailerUpdate,
    db: Session = Depends(get_db),
):
    row = update_retailer(db, payload.user_id, retailer_id, status=payload.status, notes=payload.notes)
    if not row:
        raise HTTPException(status_code=404, detail="Retailer not found")
    return {"ok": True, "retailer": serialize_retailer(row)}


@rate_limit_user()
@router.delete("/retailers/{retailer_id}")
def delete_retailer_endpoint(request: Request, retailer_id: int, user_id: str, db: Session = Depends(get_db)):
    ok = delete_retailer(db, user_id, retailer_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Retailer not found")
    return {"ok": True}


@rate_limit_user()
@router.post("/orders")
def create_gift_order(request: Request, payload: GiftOrderCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    domain = payload.retailer_domain
    if not domain and payload.product_url:
        domain = payload.product_url

    if payload.enforce_allowlist:
        if not domain or not is_retailer_allowed(db, payload.user_id, domain):
            raise HTTPException(status_code=400, detail="Retailer not in allowlist")

    status = "pending_approval" if payload.require_approval else "authorized"
    row = create_order(
        db,
        user_id=payload.user_id,
        gift_idea_id=payload.gift_idea_id,
        occasion_id=payload.occasion_id,
        title=payload.title,
        product_url=payload.product_url,
        retailer_domain=payload.retailer_domain or domain,
        quantity=payload.quantity or 1,
        unit_price=payload.unit_price,
        total_price=payload.total_price,
        currency=payload.currency,
        shipping_address=payload.shipping_address,
        notes=payload.notes,
        status=status,
        payment_method_id=payload.payment_method_id,
        metadata={"approval_required": bool(payload.require_approval)},
    )

    proposal = None
    if payload.require_approval:
        proposal_payload = {
            "gift_order_id": row.id,
            "title": row.product_title,
            "product_url": row.product_url,
            "quantity": row.quantity,
            "total_price": row.total_price,
            "currency": row.currency,
        }
        proposal = create_proposal_with_link(
            db=db,
            user_id=payload.user_id,
            proposal_type="gift_purchase",
            payload=proposal_payload,
            ttl_hours=24,
        )
        row.proposal_id = int(proposal["proposal_id"])
        row.status = "pending_approval"
        row.updated_at = datetime.utcnow()
        db.commit()
        db.refresh(row)

    return {"ok": True, "order": serialize_order(row), "proposal": proposal}


@rate_limit_user()
@router.get("/orders")
def list_gift_orders(
    request: Request,
    user_id: str,
    status: Optional[str] = None,
    limit: int = 50,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    rows = list_orders(db, user_id, status=status, limit=limit)
    return {"ok": True, "orders": [serialize_order(r) for r in rows]}


@rate_limit_user()
@router.get("/orders/{order_id}")
def get_gift_order(request: Request, order_id: int, user_id: str, db: Session = Depends(get_db)):
    row = get_order(db, user_id, order_id)
    if not row:
        raise HTTPException(status_code=404, detail="Order not found")
    return {"ok": True, "order": serialize_order(row)}


@rate_limit_user()
@router.patch("/orders/{order_id}")
def update_gift_order(
    request: Request,
    order_id: int,
    payload: GiftOrderUpdate,
    db: Session = Depends(get_db),
):
    row = update_order(
        db,
        payload.user_id,
        order_id,
        status=payload.status,
        tracking_number=payload.tracking_number,
        tracking_url=payload.tracking_url,
        notes=payload.notes,
        metadata=payload.metadata,
    )
    if not row:
        raise HTTPException(status_code=404, detail="Order not found")
    return {"ok": True, "order": serialize_order(row)}


@rate_limit_user()
@router.post("/orders/{order_id}/authorize")
def authorize_gift_order(
    request: Request,
    order_id: int,
    payload: GiftOrderAuthorize,
    db: Session = Depends(get_db),
):
    row = authorize_order(db, payload.user_id, order_id, payment_method_id=payload.payment_method_id)
    if not row:
        raise HTTPException(status_code=404, detail="Order or payment method not found")
    return {"ok": True, "order": serialize_order(row)}


@rate_limit_user()
@router.post("/orders/{order_id}/events")
def add_gift_order_event(
    request: Request,
    order_id: int,
    payload: GiftOrderEventCreate,
    db: Session = Depends(get_db),
):
    row = get_order(db, payload.user_id, order_id)
    if not row:
        raise HTTPException(status_code=404, detail="Order not found")
    event = record_order_event(
        db,
        order_id,
        payload.status,
        message=payload.message,
        metadata=payload.metadata,
        occurred_at=payload.occurred_at,
    )
    return {"ok": True, "event": serialize_order_event(event)}


@rate_limit_user()
@router.get("/orders/{order_id}/events")
def list_gift_order_events(
    request: Request,
    order_id: int,
    user_id: str,
    limit: int = 50,
    db: Session = Depends(get_db),
):
    row = get_order(db, user_id, order_id)
    if not row:
        raise HTTPException(status_code=404, detail="Order not found")
    events = list_order_events(db, order_id, limit=limit)
    return {"ok": True, "events": [serialize_order_event(e) for e in events]}


@rate_limit_user()
@router.post("/orders/{order_id}/refund")
def refund_gift_order(
    request: Request,
    order_id: int,
    payload: GiftOrderRefundRequest,
    db: Session = Depends(get_db),
):
    row = refund_order(db, payload.user_id, order_id, reason=payload.reason, amount=payload.amount)
    if not row:
        raise HTTPException(status_code=404, detail="Order not found")
    return {"ok": True, "order": serialize_order(row)}
