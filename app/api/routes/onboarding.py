from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.services.preferences import (
    get_preferences,
    handle_onboarding_step,
    update_preferences,
    is_onboarding_complete,
)
from app.services.profile_service import get_profile
from app.services.phone_verification import request_phone_verification, verify_phone_code
from app.middleware.rate_limiter import rate_limit_user

router = APIRouter(prefix="/onboarding", tags=["onboarding"])


class OnboardingAnswerRequest(BaseModel):
    user_id: str
    message: Optional[str] = ""


class PhoneStartRequest(BaseModel):
    user_id: str
    phone_number: str


class PhoneVerifyRequest(BaseModel):
    user_id: str
    phone_number: str
    code: str


def _phone_verified(prefs: dict, profile: dict) -> bool:
    if prefs.get("phone_verified") is True:
        return True
    return bool(profile.get("phone_verified_at"))


@rate_limit_user()
@router.get("/status")
def onboarding_status(request: Request, user_id: str, db: Session = Depends(get_db)):
    prefs = get_preferences(db, user_id)
    profile = get_profile(db, user_id)

    onboarding_complete = is_onboarding_complete(prefs)
    phone_verified = _phone_verified(prefs, profile)

    steps = [
        {
            "id": "phone_verification",
            "status": "complete" if phone_verified else "pending",
        },
        {
            "id": "preferences",
            "status": "complete" if onboarding_complete else "pending",
        },
    ]

    return {
        "user_id": user_id,
        "phone_verified": phone_verified,
        "onboarding_complete": onboarding_complete,
        "steps": steps,
    }


@rate_limit_user()
@router.post("/answer")
def onboarding_answer(request: Request, payload: OnboardingAnswerRequest, db: Session = Depends(get_db)):
    prefs = get_preferences(db, payload.user_id)
    reply, updated = handle_onboarding_step(payload.message or "", prefs)
    if reply:
        update_preferences(db, payload.user_id, updated)
    return {
        "ok": True,
        "reply": reply,
        "onboarding_complete": is_onboarding_complete(updated),
        "preferences": updated,
    }


@rate_limit_user()
@router.post("/phone/start")
def phone_start(request: Request, payload: PhoneStartRequest, db: Session = Depends(get_db)):
    try:
        result = request_phone_verification(db, payload.user_id, payload.phone_number)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    return result


@rate_limit_user()
@router.post("/phone/verify")
def phone_verify(request: Request, payload: PhoneVerifyRequest, db: Session = Depends(get_db)):
    try:
        result = verify_phone_code(db, payload.user_id, payload.phone_number, payload.code)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    return result
