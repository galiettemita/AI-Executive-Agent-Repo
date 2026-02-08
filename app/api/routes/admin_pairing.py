from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.services.pairing_service import generate_pairing_code

router = APIRouter(prefix="/admin/pairing", tags=["admin"])


class PairingCodeRequest(BaseModel):
    user_id: str
    ttl_minutes: int | None = None


@router.post("/code")
def create_pairing_code(payload: PairingCodeRequest, db: Session = Depends(get_db)):
    if not payload.user_id:
        raise HTTPException(status_code=400, detail="user_id is required")
    get_or_create_user(db, payload.user_id)
    record = generate_pairing_code(db, payload.user_id, ttl_minutes=payload.ttl_minutes)
    return {
        "ok": True,
        "code": record.code,
        "user_id": record.user_id,
        "expires_at": record.expires_at.isoformat() if record.expires_at else None,
    }
