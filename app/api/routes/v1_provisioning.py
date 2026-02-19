from __future__ import annotations

from pydantic import BaseModel
from fastapi import APIRouter, Depends, Query
from fastapi.responses import HTMLResponse, RedirectResponse
from sqlalchemy.orm import Session

from app.api.deps import get_db
from app.blueprint.contracts import AuthType, ProvisionTrigger, ProvisioningState, ServerProvisionedEvent
from app.core.config import settings
from app.services.provisioning_activator import activate_server
from app.services.analytics import emit_event_async
from app.services.billing_middleware import get_billing_subscription
from app.services.provisioning_pipeline import ProvisioningPipeline, get_request, validate_catalog_security_for_server
from app.services.provisioning_retry import retry_original_task_after_provisioning
from app.services.provisioning_sessions import delete_provisioning_session, get_provisioning_session
from app.services.url_shortener import resolve_short_url
from app.services.provisioning_catalog import available_servers_for_user
from app.services.provisioning_handlers import ProvisionAuthContext, get_auth_handler

router = APIRouter(prefix="/api/v1/provision", tags=["provisioning-v1"])

_PLAN_RANK = {
    "free": 0,
    "trial": 0,
    "free_trial": 0,
    "starter": 1,
    "personal": 1,
    "plus": 2,
    "professional": 3,
    "pro": 3,
    "enterprise": 4,
}


class ProvisionStartRequest(BaseModel):
    user_id: str
    server_id: str
    reason: str = "Server connection requested"
    trigger: ProvisionTrigger = ProvisionTrigger.USER_INITIATED
    original_task_id: str | None = None


def _plan_rank(value: str | None) -> int:
    return _PLAN_RANK.get(str(value or "free").strip().lower(), 0)


def _wave56_server_ids() -> set[str]:
    raw = str(settings.WAVE56_PLAN_GATED_SERVER_IDS or "")
    return {
        item.strip().lower()
        for item in raw.replace("|", ",").split(",")
        if item.strip()
    }


def _enforce_wave56_plan_gate(db: Session, *, user_id: str, server_id: str) -> tuple[bool, str | None]:
    target = str(server_id or "").strip().lower()
    if not target or target not in _wave56_server_ids():
        return True, None
    try:
        sub = get_billing_subscription(db, user_id)
        status = str(getattr(sub, "status", "") or "").strip().lower()
        plan = str(getattr(sub, "plan", "") or "").strip().lower()
        if status == "trialing" or plan in {"trial", "free_trial", "trialing", ""}:
            plan = "free"
    except Exception:
        plan = "free"
    min_plan = str(settings.WAVE56_MIN_PLAN or "professional").strip().lower()
    if _plan_rank(plan) >= _plan_rank(min_plan):
        return True, None
    upgrade_url = (
        f"{settings.APP_BASE_URL.rstrip('/')}/api/v1/billing/checkout?user_id={user_id}&plan={min_plan}"
        if settings.APP_BASE_URL
        else ""
    )
    message = (
        f"{target} requires the {min_plan} plan. "
        f"{('Upgrade path: ' + upgrade_url) if upgrade_url else ''}"
    ).strip()
    return False, message


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


@router.get("/short/{token}")
def redirect_short_url(token: str):
    target = resolve_short_url(token)
    if not target:
        return RedirectResponse(url="/api/v1/provision/expired", status_code=302)
    return RedirectResponse(url=target, status_code=302)


@router.post("/start")
def provision_start(payload: ProvisionStartRequest, db: Session = Depends(get_db)):
    server_id = str(payload.server_id or "").strip().lower()
    if not server_id:
        return {"ok": False, "error": "server_id is required"}

    pipeline = ProvisioningPipeline(db)
    _ = pipeline.expire_timeouts()

    available = available_servers_for_user(db, user_id=payload.user_id, connected_server_ids=set())
    matched = next((item for item in available if str(item.get("server_id") or "").strip().lower() == server_id), None)
    if not matched:
        return {"ok": False, "error": f"{server_id} is not available on this account or plan"}
    plan_allowed, plan_error = _enforce_wave56_plan_gate(db, user_id=payload.user_id, server_id=server_id)
    if not plan_allowed:
        return {"ok": False, "error": plan_error or f"{server_id} is not available on this account or plan"}

    reason = str(payload.reason or "").strip() or "Server connection requested"
    auth_type = _map_auth_type(str(matched.get("auth_type") or "oauth2"))
    request_row = pipeline.begin(
        user_id=payload.user_id,
        server_id=server_id,
        reason=reason,
        trigger=payload.trigger,
        auth_type=auth_type,
        original_task_id=payload.original_task_id,
    )
    auth_handler = get_auth_handler(auth_type.value)
    auth_payload = auth_handler.begin(
        ProvisionAuthContext(
            request_id=request_row.id,
            user_id=payload.user_id,
            server_id=server_id,
            reason=reason,
            original_task_id=payload.original_task_id,
        )
    )
    status_value = str(auth_payload.get("status") or "awaiting_auth").strip().lower()
    if status_value == "auth_received":
        request_row = pipeline.transition(
            request_id=request_row.id,
            new_state=ProvisioningState.AUTH_RECEIVED,
            note="auth_not_required",
        )
    elif request_row.state != ProvisioningState.AWAITING_AUTH:
        request_row = pipeline.transition(
            request_id=request_row.id,
            new_state=ProvisioningState.AWAITING_AUTH,
            note="auth_pending",
        )

    emit_event_async(
        event_name="provisioning_requested",
        user_id=payload.user_id,
        source="provision_start",
        payload={
            "request_id": request_row.id,
            "server_id": server_id,
            "trigger": request_row.trigger.value,
            "state": request_row.state.value,
        },
    )
    if request_row.state == ProvisioningState.AWAITING_AUTH:
        emit_event_async(
            event_name="awaiting_auth",
            user_id=payload.user_id,
            source="provision_start",
            payload={"request_id": request_row.id, "server_id": server_id, "trigger": request_row.trigger.value},
        )

    return {
        "ok": True,
        "request_id": request_row.id,
        "server_id": server_id,
        "trigger": request_row.trigger.value,
        "state": request_row.state.value,
        "auth_type": auth_type.value,
        **auth_payload,
    }


@router.get("/callback")
async def provision_callback(
    state: str = Query(...),
    code: str | None = Query(default=None),
    db: Session = Depends(get_db),
):
    _ = code  # callback code is accepted and logged upstream; this stub stores flow state.
    session = get_provisioning_session(state)
    if not session:
        return RedirectResponse(url="/api/v1/provision/expired?reason=session_expired", status_code=302)

    request_id = str(session.get("request_id") or "").strip()
    user_id = str(session.get("user_id") or "").strip()
    server_id = str(session.get("server_id") or "").strip()
    if not request_id or not user_id or not server_id:
        delete_provisioning_session(state)
        return RedirectResponse(url="/api/v1/provision/expired?reason=invalid_session", status_code=302)

    pipeline = ProvisioningPipeline(db)
    _ = pipeline.expire_timeouts()
    request_row = get_request(db, request_id=request_id)
    if not request_row:
        delete_provisioning_session(state)
        return RedirectResponse(url="/api/v1/provision/expired?reason=request_missing", status_code=302)

    if request_row.state in {ProvisioningState.EXPIRED, ProvisioningState.CANCELED}:
        emit_event_async(
            event_name="provisioning_expired",
            user_id=user_id,
            source="provision_callback",
            payload={"request_id": request_id, "server_id": server_id, "state": request_row.state.value},
        )
        delete_provisioning_session(state)
        return RedirectResponse(url="/api/v1/provision/expired?reason=request_closed", status_code=302)

    try:
        _ = validate_catalog_security_for_server(db, server_id=server_id)
    except Exception as exc:
        pipeline.transition(
            request_id=request_row.id,
            new_state=ProvisioningState.FAILED,
            note="security_validation_failed",
            error_message=str(exc),
        )
        emit_event_async(
            event_name="provisioning_failed",
            user_id=user_id,
            source="provision_callback",
            payload={"request_id": request_id, "server_id": server_id, "error": str(exc)},
        )
        delete_provisioning_session(state)
        return RedirectResponse(url="/api/v1/provision/expired?reason=security_validation_failed", status_code=302)

    if request_row.state != ProvisioningState.AUTH_RECEIVED:
        request_row = pipeline.transition(
            request_id=request_row.id,
            new_state=ProvisioningState.AUTH_RECEIVED,
            note="oauth_callback_received",
        )
    if request_row.state != ProvisioningState.PROVISIONING:
        request_row = pipeline.transition(
            request_id=request_row.id,
            new_state=ProvisioningState.PROVISIONING,
            note="activation_started",
        )

    activation = await activate_server(db, user_id=user_id, server_id=server_id)
    if activation.get("ok"):
        pipeline.transition(
            request_id=request_row.id,
            new_state=ProvisioningState.ACTIVE,
            note="activation_complete",
        )
        emit_event_async(
            event_name="server_provisioned",
            user_id=user_id,
            source="provision_callback",
            payload={"request_id": request_id, "server_id": server_id},
        )
        delete_provisioning_session(state)
        original_task_id = str(session.get("original_task_id") or "").strip()
        retried = "0"
        if original_task_id:
            retry_result = retry_original_task_after_provisioning(
                ServerProvisionedEvent(
                    request_id=request_id,
                    user_id=user_id,
                    server_id=server_id,
                    original_task_id=original_task_id,
                    connected_tools=[],
                )
            )
            retried = "1" if bool(retry_result.get("ok")) else "0"
        missing_task = "1" if not original_task_id else "0"
        return RedirectResponse(
            url=(
                f"/api/v1/provision/success?server_id={server_id}&request_id={request_id}"
                f"&missing_task={missing_task}&retried={retried}"
            ),
            status_code=302,
        )

    pipeline.transition(
        request_id=request_row.id,
        new_state=ProvisioningState.FAILED,
        note="activation_failed",
        error_message=str(activation.get("error") or "activation_failed"),
    )
    emit_event_async(
        event_name="provisioning_failed",
        user_id=user_id,
        source="provision_callback",
        payload={
            "request_id": request_id,
            "server_id": server_id,
            "error": str(activation.get("error") or "activation_failed"),
        },
    )
    delete_provisioning_session(state)
    return RedirectResponse(url="/api/v1/provision/expired?reason=activation_failed", status_code=302)


@router.get("/success", response_class=HTMLResponse)
def provision_success(
    server_id: str = Query(default=""),
    request_id: str = Query(default=""),
    missing_task: str = Query(default="0"),
    retried: str = Query(default="0"),
):
    safe_server = (server_id or "").strip() or "server"
    safe_request = (request_id or "").strip()
    next_step = (
        "<p>Connection complete. Your prior task context expired, so tell me what you want to do next.</p>"
        if str(missing_task or "0") == "1"
        else (
            "<p>Connection complete. Your original request has been retried in chat.</p>"
            if str(retried or "0") == "1"
            else "<p>Connection complete. Returning to chat will resume your original request.</p>"
        )
    )
    return HTMLResponse(
        content=(
            "<html><body style='font-family: sans-serif; padding: 24px;'>"
            "<h2>Connected! Return to chat.</h2>"
            f"<p>{safe_server} is now connected.</p>"
            f"{next_step}"
            f"<p style='color:#666;'>request_id={safe_request}</p>"
            "</body></html>"
        )
    )


@router.get("/expired", response_class=HTMLResponse)
def provision_expired(reason: str = Query(default="expired")):
    safe_reason = (reason or "expired").strip()
    return HTMLResponse(
        content=(
            "<html><body style='font-family: sans-serif; padding: 24px;'>"
            "<h2>Link expired. Ask your assistant to try again.</h2>"
            f"<p style='color:#666;'>reason={safe_reason}</p>"
            "</body></html>"
        )
    )
