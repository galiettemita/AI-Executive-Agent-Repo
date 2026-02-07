# backend/app/api/routes/consent.py

from __future__ import annotations

from typing import Any, Dict, Optional

from fastapi import APIRouter, Depends
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.services.consent_service import grant_consent, revoke_consent, has_consent

router = APIRouter(prefix="/consent", tags=["consent"])


class ConsentRequest(BaseModel):
    user_id: str
    integration: str
    metadata: Optional[Dict[str, Any]] = None


@router.get("")
def check_consent(user_id: str, integration: str, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    return {"ok": True, "granted": has_consent(db, user_id, integration)}


@router.post("/grant")
def grant(payload: ConsentRequest, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    row = grant_consent(db, payload.user_id, payload.integration, payload.metadata)
    return {
        "ok": True,
        "integration": row.integration,
        "granted_at": row.granted_at.isoformat() if row.granted_at else None,
    }


@router.post("/revoke")
def revoke(payload: ConsentRequest, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    row = revoke_consent(db, payload.user_id, payload.integration)
    return {
        "ok": True,
        "integration": row.integration,
        "revoked_at": row.revoked_at.isoformat() if row.revoked_at else None,
    }
