from __future__ import annotations

from typing import Any, Optional

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel
from sqlalchemy.orm import Session
from sqlalchemy import text

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


def _table_exists(db: Session, table_name: str) -> bool:
    try:
        row = db.execute(
            text(
                "select 1 from information_schema.tables "
                "where table_schema = current_schema() and table_name = :name"
            ),
            {"name": table_name},
        ).first()
        if row:
            return True
    except Exception:
        pass
    try:
        row = db.execute(text("select name from sqlite_master where type='table' and name=:name"), {"name": table_name}).first()
        return bool(row)
    except Exception:
        return False


def _channel_hints(db: Session, *, user_id: str) -> set[str]:
    if not _table_exists(db, "channel_connections"):
        return set()
    rows = db.execute(
        text(
            "select channel_type, provider, external_user_id "
            "from channel_connections where user_id = :user_id"
        ),
        {"user_id": user_id},
    ).mappings().all()
    hints: set[str] = set()
    for row in rows:
        for key in ("channel_type", "provider", "external_user_id"):
            value = str(row.get(key) or "").strip().lower()
            if not value:
                continue
            hints.add(value)
            if "google" in value or "gmail" in value:
                hints.add("google")
            if "outlook" in value or "microsoft" in value or "teams" in value:
                hints.add("microsoft")
            if "slack" in value:
                hints.add("slack")
    return hints


_GOOGLE_SUITE = ("google-calendar-mcp", "google-drive-mcp", "gmail-mcp")
_MICROSOFT_SUITE = ("outlook-mcp", "teams-mcp")


def _build_connection_cards(
    *,
    available: list[dict[str, Any]],
    hints: set[str],
) -> tuple[list[dict[str, Any]], list[dict[str, Any]], list[str]]:
    available_map = {str(item.get("server_id") or "").strip().lower(): item for item in available}
    recommended: list[str] = []
    if "google" in hints or "gmail" in hints:
        recommended.extend([server_id for server_id in _GOOGLE_SUITE if server_id in available_map])
    if "microsoft" in hints:
        recommended.extend([server_id for server_id in _MICROSOFT_SUITE if server_id in available_map])
    if "slack" in hints and "slack-mcp" in available_map:
        recommended.append("slack-mcp")
    if "whatsapp" in hints and "whatsapp-business-mcp" in available_map:
        recommended.append("whatsapp-business-mcp")
    dedup_recommended = list(dict.fromkeys(recommended))

    def _card(item: dict[str, Any], *, recommended_card: bool) -> dict[str, Any]:
        setup_seconds = int(item.get("setup_seconds") or 0)
        return {
            "server_id": str(item.get("server_id") or "").strip().lower(),
            "label": str(item.get("display_name") or item.get("server_id") or "").strip(),
            "description": str(item.get("description") or "").strip(),
            "auth_type": str(item.get("auth_type") or "oauth2").strip().lower(),
            "estimated_setup_seconds": setup_seconds,
            "recommended": bool(recommended_card),
            "connect_endpoint": "/onboarding/connect",
            "confirmation_text": "After connect, the app returns a success page and confirms the server is active.",
        }

    cards: list[dict[str, Any]] = []
    for server_id in dedup_recommended:
        item = available_map.get(server_id)
        if item:
            cards.append(_card(item, recommended_card=True))

    for item in available:
        server_id = str(item.get("server_id") or "").strip().lower()
        if not server_id or server_id in dedup_recommended:
            continue
        cards.append(_card(item, recommended_card=False))

    oauth_groups = [
        {
            "group_id": "google_workspace",
            "label": "Google Workspace",
            "server_ids": [sid for sid in _GOOGLE_SUITE if sid in available_map],
            "consolidated_auth": True,
            "recommended": any(sid in dedup_recommended for sid in _GOOGLE_SUITE),
        },
        {
            "group_id": "microsoft_workspace",
            "label": "Microsoft Workspace",
            "server_ids": [sid for sid in _MICROSOFT_SUITE if sid in available_map],
            "consolidated_auth": True,
            "recommended": any(sid in dedup_recommended for sid in _MICROSOFT_SUITE),
        },
    ]
    oauth_groups = [group for group in oauth_groups if group.get("server_ids")]

    return cards, oauth_groups, dedup_recommended


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
@router.get("/ecosystem")
def onboarding_ecosystem(request: Request, user_id: str, db: Session = Depends(get_db)):
    available = available_servers_for_user(db, user_id=user_id, connected_server_ids=set())
    hints = _channel_hints(db, user_id=user_id)
    cards, oauth_groups, recommended = _build_connection_cards(available=available, hints=hints)
    return {
        "ok": True,
        "user_id": user_id,
        "recommended_servers": recommended,
        "channel_hints": sorted(hints),
        "connection_cards": cards,
        "oauth_consolidation_groups": oauth_groups,
        "confirmation_flow": [
            "Select a connection card.",
            "Complete OAuth/API-key setup in the provider flow.",
            "Return to the success page and verify the server state is active.",
        ],
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
