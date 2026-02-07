# backend/app/api/routes/profile.py

from __future__ import annotations

from typing import Any, Dict

from fastapi import APIRouter, Depends
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.services.profile_service import get_profile, update_profile

router = APIRouter(prefix="/profile", tags=["profile"])


class ProfilePatch(BaseModel):
    user_id: str
    data: Dict[str, Any]


@router.get("")
def read_profile(user_id: str, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    return {"ok": True, "profile": get_profile(db, user_id)}


@router.patch("")
def patch_profile(payload: ProfilePatch, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    profile = update_profile(db, payload.user_id, payload.data)
    return {"ok": True, "profile": profile}
