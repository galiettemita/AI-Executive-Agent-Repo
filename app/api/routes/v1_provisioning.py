from __future__ import annotations

from fastapi import APIRouter, Depends, Query
from fastapi.responses import HTMLResponse, RedirectResponse
from sqlalchemy.orm import Session

from app.api.deps import get_db
from app.blueprint.contracts import ProvisioningState
from app.services.provisioning_activator import activate_server
from app.services.provisioning_pipeline import ProvisioningPipeline, get_request
from app.services.provisioning_sessions import delete_provisioning_session, get_provisioning_session
from app.services.url_shortener import resolve_short_url

router = APIRouter(prefix="/api/v1/provision", tags=["provisioning-v1"])


@router.get("/short/{token}")
def redirect_short_url(token: str):
    target = resolve_short_url(token)
    if not target:
        return RedirectResponse(url="/api/v1/provision/expired", status_code=302)
    return RedirectResponse(url=target, status_code=302)


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
    request_row = get_request(db, request_id=request_id)
    if not request_row:
        delete_provisioning_session(state)
        return RedirectResponse(url="/api/v1/provision/expired?reason=request_missing", status_code=302)

    if request_row.state in {ProvisioningState.EXPIRED, ProvisioningState.CANCELED}:
        delete_provisioning_session(state)
        return RedirectResponse(url="/api/v1/provision/expired?reason=request_closed", status_code=302)

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
        delete_provisioning_session(state)
        return RedirectResponse(
            url=f"/api/v1/provision/success?server_id={server_id}&request_id={request_id}",
            status_code=302,
        )

    pipeline.transition(
        request_id=request_row.id,
        new_state=ProvisioningState.FAILED,
        note="activation_failed",
        error_message=str(activation.get("error") or "activation_failed"),
    )
    delete_provisioning_session(state)
    return RedirectResponse(url="/api/v1/provision/expired?reason=activation_failed", status_code=302)


@router.get("/success", response_class=HTMLResponse)
def provision_success(
    server_id: str = Query(default=""),
    request_id: str = Query(default=""),
):
    safe_server = (server_id or "").strip() or "server"
    safe_request = (request_id or "").strip()
    return HTMLResponse(
        content=(
            "<html><body style='font-family: sans-serif; padding: 24px;'>"
            "<h2>Connected! Return to chat.</h2>"
            f"<p>{safe_server} is now connected.</p>"
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
