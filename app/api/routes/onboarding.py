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
from app.services.analytics import emit_event_async
from app.blueprint.contracts import AuthType, ProvisionTrigger, ProvisioningState
from app.services.provisioning_catalog import available_servers_for_user
from app.services.provisioning_handlers import ProvisionAuthContext, get_auth_handler
from app.services.provisioning_pipeline import ProvisioningPipeline
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


class OnboardingConnectRequest(BaseModel):
    user_id: str
    server_id: str
    reason: Optional[str] = None


def _map_auth_type(auth_value: str | None) -> AuthType:
    value = str(auth_value or "oauth2").strip().lower()
    if value in {"api_key", "apikey"}:
        return AuthType.API_KEY
    if value in {"pre_provisioned", "none", "internal"}:
        return AuthType.PRE_PROVISIONED
    if value in {"plaid_link"}:
        return AuthType.PLAID_LINK
    if value in {"tesla_sso"}:
        return AuthType.TESLA_SSO
    if value in {"oauth2_consolidated"}:
        return AuthType.OAUTH2_CONSOLIDATED
    return AuthType.OAUTH2


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
        emit_event_async(
            event_name="onboarding_step_completed",
            user_id=payload.user_id,
            source="onboarding_answer",
            payload={"step": "preferences"},
        )
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
    if result.get("ok") is True:
        emit_event_async(
            event_name="onboarding_step_completed",
            user_id=payload.user_id,
            source="onboarding_phone_verify",
            payload={"step": "phone_verification"},
        )
    return result


@rate_limit_user()
@router.post("/connect")
def onboarding_connect(request: Request, payload: OnboardingConnectRequest, db: Session = Depends(get_db)):
    server_id = str(payload.server_id or "").strip().lower()
    if not server_id:
        raise HTTPException(status_code=400, detail="server_id is required")

    pipeline = ProvisioningPipeline(db)
    _ = pipeline.expire_timeouts()

    available = available_servers_for_user(db, user_id=payload.user_id, connected_server_ids=set())
    matched = next((item for item in available if str(item.get("server_id") or "").strip().lower() == server_id), None)
    if not matched:
        raise HTTPException(status_code=403, detail=f"{server_id} is not available on this account or plan")

    reason = str(payload.reason or "").strip() or "Onboarding connection requested"
    auth_type = _map_auth_type(str(matched.get("auth_type") or "oauth2"))
    request_record = pipeline.begin(
        user_id=payload.user_id,
        server_id=server_id,
        reason=reason,
        trigger=ProvisionTrigger.ONBOARDING,
        auth_type=auth_type,
    )
    auth_handler = get_auth_handler(auth_type.value)
    auth_payload = auth_handler.begin(
        ProvisionAuthContext(
            request_id=request_record.id,
            user_id=payload.user_id,
            server_id=server_id,
            reason=reason,
            original_task_id=None,
        )
    )

    status_value = str(auth_payload.get("status") or "awaiting_auth").strip().lower()
    if status_value == "auth_received":
        request_record = pipeline.transition(
            request_id=request_record.id,
            new_state=ProvisioningState.AUTH_RECEIVED,
            note="onboarding_auth_not_required",
        )
    elif request_record.state != ProvisioningState.AWAITING_AUTH:
        request_record = pipeline.transition(
            request_id=request_record.id,
            new_state=ProvisioningState.AWAITING_AUTH,
            note="onboarding_auth_pending",
        )

    emit_event_async(
        event_name="provisioning_requested",
        user_id=payload.user_id,
        source="onboarding_connect",
        payload={
            "request_id": request_record.id,
            "server_id": server_id,
            "trigger": "onboarding",
            "state": request_record.state.value,
        },
    )
    if request_record.state == ProvisioningState.AWAITING_AUTH:
        emit_event_async(
            event_name="awaiting_auth",
            user_id=payload.user_id,
            source="onboarding_connect",
            payload={"request_id": request_record.id, "server_id": server_id, "trigger": "onboarding"},
        )

    return {
        "ok": True,
        "request_id": request_record.id,
        "server_id": server_id,
        "trigger": request_record.trigger.value,
        "state": request_record.state.value,
        "auth_type": auth_type.value,
        **auth_payload,
    }
