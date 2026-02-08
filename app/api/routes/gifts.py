# backend/app/api/routes/gifts.py

from __future__ import annotations

from typing import Optional

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
