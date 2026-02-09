from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException, Request
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.middleware.rate_limiter import rate_limit_user
from app.schemas.relationships import (
    RelationshipProfileCreate,
    RelationshipProfileUpdate,
    RelationshipInteractionCreate,
)
from app.services.relationship_service import (
    upsert_profile,
    update_profile,
    list_profiles,
    serialize_profile,
    log_interaction,
    get_suggestions,
    enqueue_relationship_reminders_for_user,
)
from app.db.models import Contact


router = APIRouter(prefix="/relationships", tags=["relationships"])


def _get_contact(db: Session, user_id: str, contact_id: int) -> Contact:
    contact = (
        db.query(Contact)
        .filter(Contact.id == contact_id, Contact.user_id == user_id)
        .one_or_none()
    )
    if not contact:
        raise HTTPException(status_code=404, detail="Contact not found")
    return contact


@rate_limit_user()
@router.post("/profiles")
def create_profile(request: Request, payload: RelationshipProfileCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    contact = _get_contact(db, payload.user_id, payload.contact_id)
    row = upsert_profile(db, **payload.model_dump())
    return {"ok": True, "profile": serialize_profile(row, contact)}


@rate_limit_user()
@router.get("/profiles")
def list_profiles_endpoint(request: Request, user_id: str, limit: int = 100, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    rows = list_profiles(db, user_id, limit=limit)
    return {"ok": True, "profiles": [serialize_profile(profile, contact) for profile, contact in rows]}


@rate_limit_user()
@router.patch("/profiles/{profile_id}")
def update_profile_endpoint(
    request: Request,
    profile_id: int,
    payload: RelationshipProfileUpdate,
    db: Session = Depends(get_db),
):
    row = update_profile(db, payload.user_id, profile_id, **payload.model_dump(exclude={"user_id"}))
    if not row:
        raise HTTPException(status_code=404, detail="Relationship profile not found")
    contact = _get_contact(db, payload.user_id, row.contact_id)
    return {"ok": True, "profile": serialize_profile(row, contact)}


@rate_limit_user()
@router.post("/interactions")
def log_interaction_endpoint(
    request: Request,
    payload: RelationshipInteractionCreate,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, payload.user_id)
    contact = _get_contact(db, payload.user_id, payload.contact_id)
    interaction = log_interaction(
        db,
        user_id=payload.user_id,
        contact_id=payload.contact_id,
        direction=payload.direction,
        channel=payload.channel,
        summary=payload.summary,
        occurred_at=payload.occurred_at,
        metadata=payload.metadata,
    )
    return {
        "ok": True,
        "interaction_id": interaction.id,
        "occurred_at": interaction.occurred_at.isoformat() if interaction.occurred_at else None,
        "profile": serialize_profile(
            upsert_profile(db, user_id=payload.user_id, contact_id=payload.contact_id),
            contact,
        ),
    }


@rate_limit_user()
@router.get("/suggestions")
def relationship_suggestions(
    request: Request,
    user_id: str,
    limit: int = 10,
    due_only: bool = True,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    suggestions = get_suggestions(db, user_id, limit=limit, due_only=due_only)
    return {"ok": True, "suggestions": suggestions}


@rate_limit_user()
@router.post("/reminders/run")
def run_relationship_reminders(request: Request, user_id: str, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    result = enqueue_relationship_reminders_for_user(db, user_id)
    return {"ok": True, **result}
