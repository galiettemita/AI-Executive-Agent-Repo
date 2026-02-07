from __future__ import annotations

from typing import Any, Dict, List, Optional

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.services.contacts_service import (
    upsert_contact,
    update_contact,
    get_contact,
    list_contacts,
    search_contacts,
)
from app.middleware.rate_limiter import rate_limit_user

router = APIRouter(prefix="/contacts", tags=["contacts"])


class ContactUpsertRequest(BaseModel):
    user_id: str
    name: Optional[str] = None
    phone: Optional[str] = None
    email: Optional[str] = None
    tags: Optional[List[str]] = None
    metadata: Optional[Dict[str, Any]] = None


class ContactUpdateRequest(BaseModel):
    name: Optional[str] = None
    phone: Optional[str] = None
    email: Optional[str] = None
    tags: Optional[List[str]] = None
    metadata: Optional[Dict[str, Any]] = None


def _serialize(contact) -> dict:
    def _parse(text: str | None):
        if not text:
            return None
        try:
            import json
            return json.loads(text)
        except Exception:
            return None
    return {
        "id": contact.id,
        "user_id": contact.user_id,
        "name": contact.name,
        "phone": contact.phone,
        "email": contact.email,
        "tags": _parse(contact.tags_json) or [],
        "metadata": _parse(contact.metadata_json) or {},
        "created_at": contact.created_at.isoformat() if contact.created_at else None,
        "updated_at": contact.updated_at.isoformat() if contact.updated_at else None,
    }


@rate_limit_user()
@router.post("")
def create_contact(request: Request, payload: ContactUpsertRequest, db: Session = Depends(get_db)):
    contact = upsert_contact(
        db,
        user_id=payload.user_id,
        name=payload.name,
        phone=payload.phone,
        email=payload.email,
        tags=payload.tags,
        metadata=payload.metadata,
    )
    return {"ok": True, "contact": _serialize(contact)}


@rate_limit_user()
@router.get("")
def list_contacts_endpoint(request: Request, user_id: str, limit: int = 100, db: Session = Depends(get_db)):
    contacts = list_contacts(db, user_id, limit=limit)
    return {"items": [_serialize(c) for c in contacts]}


@rate_limit_user()
@router.get("/search")
def search_contacts_endpoint(request: Request, user_id: str, q: str, limit: int = 50, db: Session = Depends(get_db)):
    contacts = search_contacts(db, user_id, q, limit=limit)
    return {"items": [_serialize(c) for c in contacts]}


@rate_limit_user()
@router.get("/{contact_id}")
def get_contact_endpoint(request: Request, contact_id: int, user_id: str, db: Session = Depends(get_db)):
    contact = get_contact(db, contact_id, user_id=user_id)
    if not contact:
        raise HTTPException(status_code=404, detail="Contact not found")
    return {"contact": _serialize(contact)}


@rate_limit_user()
@router.patch("/{contact_id}")
def update_contact_endpoint(request: Request, contact_id: int, payload: ContactUpdateRequest, user_id: str, db: Session = Depends(get_db)):
    contact = get_contact(db, contact_id, user_id=user_id)
    if not contact:
        raise HTTPException(status_code=404, detail="Contact not found")
    contact = update_contact(db, contact, payload.model_dump(exclude_unset=True))
    return {"ok": True, "contact": _serialize(contact)}


@rate_limit_user()
@router.delete("/{contact_id}")
def delete_contact_endpoint(request: Request, contact_id: int, user_id: str, db: Session = Depends(get_db)):
    contact = get_contact(db, contact_id, user_id=user_id)
    if not contact:
        raise HTTPException(status_code=404, detail="Contact not found")
    db.delete(contact)
    db.commit()
    return {"ok": True}
