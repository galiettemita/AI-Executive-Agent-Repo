from __future__ import annotations

import hashlib
import json
import re
import time
import uuid
from datetime import datetime, timedelta, timezone

from fastapi import APIRouter, HTTPException
from sqlalchemy import text

from app.blueprint.capability_tokens import enforce_capability_token
from app.blueprint.contracts import AuthType, ProvisionTrigger, ProvisioningState, ToolCall, ToolResult
from app.blueprint.mcp.hub import invoke_mcp_tool
from app.blueprint.security import CapabilityViolation, PrivilegeViolation, validate_tool_privilege
from app.blueprint.tools import get_tool_registry
from app.blueprint.db import get_tool_execution_by_idempotency, insert_tool_execution, record_side_effect
from app.db.database import SessionLocal
from app.services.calendar_router import (
    create_event as calendar_create_event,
    delete_event as calendar_delete_event,
    find_free_slots as calendar_find_free_slots,
    list_events_in_range as calendar_list_events,
    update_event as calendar_update_event,
)
from app.services.tavily_client import TavilyNotConfiguredError, tavily_search
from app.services.google_gmail import (
    create_draft as gmail_create_draft,
    get_gmail_message,
    list_recent_gmail_messages,
    search_gmail_messages,
    send_email as gmail_send_email,
)
from app.services.microsoft_contacts import search_microsoft_contacts
from app.services.outlook_mail import (
    create_outlook_draft,
    get_outlook_message,
    list_recent_outlook_messages,
    search_outlook_messages,
    send_outlook_email,
)
from app.services.email_service import EmailService
from app.core.config import settings
from app.core.redis import get_redis
from app.services.slack_connector import (
    SlackNotConfiguredError,
    slack_channel_summary,
    slack_list_messages,
    slack_send_message,
)
from app.services.plaid_connector import (
    PlaidNotConfiguredError,
    list_accounts as plaid_list_accounts,
    list_transactions as plaid_list_transactions,
)
from app.services.docs_search import search_connected_docs
from app.services.analytics import emit_event_async
from app.services.billing_middleware import enforce_billing_for_tool_call, get_billing_subscription
from app.services.content_safety import (
    detect_transaction_abuse,
    enforce_transaction_operation_rate_limit,
    enqueue_moderation_item,
)
from app.services.provisioning_catalog import available_servers_for_user
from app.services.provisioning_handlers import ProvisionAuthContext, get_auth_handler
from app.services.provisioning_pipeline import ProvisioningPipeline
from app.services.remote_catalog import search_remote_catalog as search_remote_catalog_entries


router = APIRouter(prefix="/internal/hands", tags=["internal-hands"])


def _default_idempotency_key(*, tool: str, args: dict) -> str:
    payload = json.dumps({"tool": tool, "args": args or {}}, sort_keys=True, ensure_ascii=False).encode("utf-8")
    return f"hands:{tool}:{hashlib.sha256(payload).hexdigest()}"


def _assert_native_tool(*, is_mcp: bool) -> None:
    if is_mcp and not settings.FEATURE_MCP_CLIENT:
        raise NotImplementedError("MCP client is disabled")


def _parse_dt(value: str | None) -> datetime | None:
    if not value:
        return None
    raw = value.strip()
    if not raw:
        return None
    if raw.endswith("Z"):
        raw = raw[:-1] + "+00:00"
    try:
        dt = datetime.fromisoformat(raw)
        if dt.tzinfo is None:
            return dt.replace(tzinfo=timezone.utc)
        return dt.astimezone(timezone.utc)
    except Exception:
        return None


_EMAIL_RE = re.compile(r"^[^@\s]+@[^@\s]+\.[^@\s]+$")


def _normalize_email(value: str | None) -> str:
    return str(value or "").strip().lower()


def _validate_email_payload(*, to_email: str, subject: str, body_text: str) -> tuple[bool, str | None]:
    if not _EMAIL_RE.match(_normalize_email(to_email)):
        return False, "Invalid recipient email address"
    if not (subject or "").strip():
        return False, "Email subject is required"
    if not (body_text or "").strip():
        return False, "Email body is required"
    if len(subject) > 240:
        return False, "Email subject is too long"
    if len(body_text) > 12000:
        return False, "Email body is too long"
    return True, None


def _build_email_review_token(*, run_id: str | None, to_email: str, subject: str, body_text: str) -> str:
    payload = json.dumps(
        {
            "run_id": run_id or "",
            "to_email": _normalize_email(to_email),
            "subject": (subject or "").strip(),
            "body_text": (body_text or "").strip(),
        },
        sort_keys=True,
        ensure_ascii=False,
    )
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()[:24]


def _validate_side_effect_output(
    *,
    tool: str,
    args: dict[str, object],
    output_payload: dict[str, object] | None,
) -> str | None:
    """
    Lightweight output validation pass for side-effecting actions.
    Returns an error string when validation fails.
    """
    payload = output_payload or {}
    action_tool = tool.replace("microsoft.", "", 1) if tool.startswith("microsoft.") else tool

    if action_tool == "calendar.create":
        event = payload.get("event")
        if not isinstance(event, dict) or not str(event.get("id") or "").strip():
            return "calendar.create missing event.id in output"
    elif action_tool == "calendar.update":
        event = payload.get("event")
        if not isinstance(event, dict) or not str(event.get("id") or "").strip():
            return "calendar.update missing event.id in output"
    elif action_tool == "calendar.delete":
        result = payload.get("result")
        if not isinstance(result, dict):
            return "calendar.delete missing result payload"
        if result.get("deleted") is not True:
            return "calendar.delete did not confirm deletion"
    elif action_tool == "email.send":
        mode = str(args.get("mode") or "review").strip().lower()
        recipient = _normalize_email(str(args.get("to_email") or ""))
        subject = str(args.get("subject") or "").strip()
        status_value = str(payload.get("status") or "").strip().lower()
        if mode in {"draft", "review"}:
            if status_value != "awaiting_approval":
                return "email.send review mode did not return awaiting_approval"
            if not str(payload.get("approval_token") or "").strip():
                return "email.send review mode missing approval_token"
        elif mode == "send":
            if status_value != "sent":
                return "email.send send mode did not return sent status"
        if recipient and _normalize_email(str(payload.get("recipient") or "")) != recipient:
            return "email.send recipient mismatch between input and output"
        if subject and str(payload.get("subject") or "").strip() != subject:
            return "email.send subject mismatch between input and output"
    return None


def _send_email_by_provider(
    *,
    provider: str | None,
    db,
    user_id: str,
    to_email: str,
    subject: str,
    body_text: str,
    cc: str | None = None,
    bcc: str | None = None,
) -> dict[str, str]:
    selected = (provider or "").strip().lower()
    if selected == "google":
        result = gmail_send_email(
            db=db,
            user_id=user_id,
            to_email=to_email,
            subject=subject,
            body_text=body_text,
            cc=cc,
            bcc=bcc,
        )
        return {"provider": "google", "message_id": str(result.get("id") or "")}
    if selected == "microsoft":
        send_outlook_email(
            db=db,
            user_id=user_id,
            to_email=to_email,
            subject=subject,
            body_text=body_text,
            cc=cc,
            bcc=bcc,
        )
        return {"provider": "microsoft", "message_id": ""}

    sent = EmailService().send_email(
        to_email=to_email,
        subject=subject,
        html_body=f"<p>{body_text}</p>",
        text_body=body_text,
    )
    if not sent:
        raise RuntimeError("Email provider failed to send")
    return {"provider": selected or settings.EMAIL_PROVIDER or "ses", "message_id": ""}


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


def _allow_provisioning_hourly_rate(user_id: str) -> bool:
    return _allow_hourly_rate(
        user_id=user_id,
        key_prefix="bp:provision:hour",
        limit=max(1, int(settings.PROVISIONING_RATE_LIMIT_PER_HOUR or 5)),
        window_seconds=3600,
    )


def _allow_remote_catalog_hourly_rate(user_id: str) -> bool:
    return _allow_hourly_rate(
        user_id=user_id,
        key_prefix="bp:provision:remote-search:hour",
        limit=max(1, int(settings.PROVISIONING_REMOTE_SEARCH_RATE_LIMIT_PER_HOUR or 20)),
        window_seconds=3600,
    )


def _allow_hourly_rate(*, user_id: str, key_prefix: str, limit: int, window_seconds: int) -> bool:
    client = get_redis()
    if client is None:
        return True
    now = datetime.now(timezone.utc).timestamp()
    key = f"{key_prefix}:{user_id}"
    client.zremrangebyscore(key, 0, now - window_seconds)
    if int(client.zcard(key)) >= max(1, int(limit)):
        return False
    member = f"{now:.6f}:{uuid.uuid4().hex[:8]}"
    client.zadd(key, {member: now})
    client.expire(key, max(window_seconds + 100, 3600))
    return True


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


def _plan_rank(plan: str | None) -> int:
    return _PLAN_RANK.get(str(plan or "free").strip().lower(), 0)


def _plan_meets_min(plan: str | None, min_plan: str | None) -> bool:
    return _plan_rank(plan) >= _plan_rank(min_plan)


def _current_user_plan(db, *, user_id: str | None) -> str:
    if not user_id:
        return "free"
    try:
        sub = get_billing_subscription(db, user_id)
    except Exception:
        return "free"
    status = str(getattr(sub, "status", "") or "").strip().lower()
    plan = str(getattr(sub, "plan", "") or "").strip().lower()
    if status == "trialing":
        return "free"
    if plan in {"trial", "free_trial", "trialing", ""}:
        return "free"
    return plan


def _wave56_server_ids() -> set[str]:
    raw = str(settings.WAVE56_PLAN_GATED_SERVER_IDS or "")
    return {
        item.strip().lower()
        for item in raw.replace("|", ",").split(",")
        if item.strip()
    }


def _enforce_wave56_plan_gate(
    db,
    *,
    user_id: str | None,
    mcp_server_id: str | None,
) -> tuple[bool, str | None]:
    server_id = str(mcp_server_id or "").strip().lower()
    if not user_id or not server_id:
        return True, None
    if server_id not in _wave56_server_ids():
        return True, None
    plan = _current_user_plan(db, user_id=user_id)
    min_plan = str(settings.WAVE56_MIN_PLAN or "professional").strip().lower()
    if _plan_meets_min(plan, min_plan):
        return True, None
    upgrade_url = (
        f"{settings.APP_BASE_URL.rstrip('/')}/api/v1/billing/checkout?user_id={user_id}&plan={min_plan}"
        if settings.APP_BASE_URL
        else ""
    )
    message = (
        f"{server_id} requires the {min_plan} plan. "
        f"{('Upgrade path: ' + upgrade_url) if upgrade_url else ''}"
    ).strip()
    return False, message


def _transaction_operation_for_call(*, tool_name: str, args: dict, mcp_server_id: str | None) -> str | None:
    server_id = str(mcp_server_id or "").strip().lower()
    lowered_tool = str(tool_name or "").strip().lower()
    try:
        args_blob = json.dumps(args or {}, ensure_ascii=False).lower()
    except Exception:
        args_blob = str(args or "").lower()
    signal_text = f"{lowered_tool} {args_blob} {server_id}"
    if server_id in {"stripe-mcp", "quickbooks-mcp"}:
        return "financial"
    if any(k in signal_text for k in ("invoice", "refund", "payout", "transfer", "bill", "expense", "ledger", "charge")):
        return "financial"
    if server_id in {"duffel-mcp", "booking-com-mcp"} or any(k in signal_text for k in ("book", "flight", "hotel", "reservation")):
        return "booking"
    if server_id in {"instacart-mcp"} or any(k in signal_text for k in ("checkout", "cart", "order", "purchase", "payment")):
        return "checkout"
    return None


def _requires_explicit_transaction_approval(*, tool_name: str, operation: str) -> bool:
    op = str(operation or "").strip().lower()
    if op not in {"booking", "checkout", "financial"}:
        return False
    lowered_tool = str(tool_name or "").strip().lower()
    write_markers = (
        "create",
        "book",
        "order",
        "checkout",
        "purchase",
        "charge",
        "pay",
        "confirm",
        "cancel",
        "update",
        "delete",
        "submit",
        "invoice",
        "refund",
        "payout",
        "transfer",
        "bill",
        "expense",
        "void",
    )
    return any(marker in lowered_tool for marker in write_markers)


def _has_explicit_transaction_approval(args: dict[str, object]) -> bool:
    approved = args.get("approval_confirmed")
    if isinstance(approved, bool) and approved:
        return True
    token = str(args.get("approval_token") or "").strip()
    return bool(token)


@router.post("/execute", response_model=ToolResult)
async def execute(call: ToolCall) -> ToolResult:
    """
    Hands Plane: tool execution endpoint (Phase 1).

    Supported tools:
    - web.search (Tavily)
    """
    tool_registry = get_tool_registry()
    raw_tool = (call.tool or call.tool_name or "").strip()
    tool = tool_registry.resolve_tool_name(raw_tool)
    args = call.args or call.arguments or {}
    idempotency_key = (call.idempotency_key or "").strip() or _default_idempotency_key(tool=tool, args=args)

    try:
        spec = tool_registry.get(tool)
    except Exception:
        return ToolResult(tool_name=tool or raw_tool, tool=tool or raw_tool, ok=False, error=f"Unknown tool: {raw_tool}")

    try:
        granted_capabilities: list[str] = []
        if settings.FEATURE_PRIVILEGE_ISOLATION:
            granted_capabilities = enforce_capability_token(
                token=call.capability_token,
                run_id=call.run_id,
                user_id=call.user_id,
                tool_name=tool,
                required_capabilities=call.required_capabilities,
            )
        validate_tool_privilege(
            tool_name=tool,
            provenance=call.input_provenance,
            required_capabilities=call.required_capabilities if settings.FEATURE_PRIVILEGE_ISOLATION else [],
            granted_capabilities=granted_capabilities,
        )
    except (PrivilegeViolation, CapabilityViolation) as exc:
        return ToolResult(tool_name=tool, tool=tool, ok=False, error=str(exc))

    # Idempotency: if already executed, return the stored output.
    if call.user_id:
        db = SessionLocal()
        try:
            existing = get_tool_execution_by_idempotency(db, user_id=call.user_id, idempotency_key=idempotency_key)
        except Exception:
            existing = None
        finally:
            try:
                db.close()
            except Exception:
                pass
        if existing and existing.get("output") is not None:
            status = str(existing.get("status") or "")
            ok = status == "success"
            return ToolResult(
                tool_name=tool,
                tool=tool,
                ok=ok,
                result=existing.get("output") if ok else None,
                error=None if ok else json.dumps(existing.get("error") or {}, ensure_ascii=False),
            )

    if spec.is_mcp:
        if not settings.FEATURE_MCP_CLIENT:
            return ToolResult(tool_name=tool, tool=tool, ok=False, error="MCP client is disabled")
        if not spec.mcp_server_id:
            return ToolResult(tool_name=tool, tool=tool, ok=False, error="MCP tool missing server binding")

        started = time.perf_counter()
        output_payload: dict[str, object] | None = None
        error_payload: dict[str, str] | None = None
        status = "success"
        db = SessionLocal()
        try:
            if call.user_id:
                plan_allowed, plan_message = _enforce_wave56_plan_gate(
                    db,
                    user_id=call.user_id,
                    mcp_server_id=spec.mcp_server_id,
                )
                if not plan_allowed:
                    raise RuntimeError(plan_message or "This MCP server requires a higher plan.")

                operation = _transaction_operation_for_call(
                    tool_name=tool,
                    args=args if isinstance(args, dict) else {},
                    mcp_server_id=spec.mcp_server_id,
                )
                if operation:
                    if _requires_explicit_transaction_approval(tool_name=tool, operation=operation):
                        if not _has_explicit_transaction_approval(args if isinstance(args, dict) else {}):
                            raise RuntimeError(
                                "This financial/booking write action requires explicit approval. "
                                "Re-run with approval_confirmed=true after user confirmation."
                            )
                    decision = enforce_transaction_operation_rate_limit(call.user_id, operation=operation)
                    if not decision.allowed:
                        retry_after = (
                            f" Retry after {int(decision.retry_after_seconds)}s."
                            if decision.retry_after_seconds
                            else ""
                        )
                        raise RuntimeError(
                            f"Transaction rate limit reached for {operation} operations.{retry_after}".strip()
                        )
                    abuse = detect_transaction_abuse(
                        tool_name=tool,
                        args=args if isinstance(args, dict) else {},
                        mcp_server_id=spec.mcp_server_id,
                    )
                    if abuse.flagged:
                        enqueue_moderation_item(
                            user_id=call.user_id,
                            run_id=call.run_id,
                            direction="inbound",
                            channel="tool_execution",
                            text_value=f"Transaction operation blocked: {tool}",
                            categories=abuse.categories,
                            risk_score=max(0.5, abuse.risk_score),
                            classifier=abuse.classifier,
                            metadata={
                                "tool": tool,
                                "mcp_server_id": spec.mcp_server_id,
                                "reason": abuse.reason,
                                "operation": operation,
                            },
                            increment_safety_flags=True,
                        )
                        raise RuntimeError(
                            "Potentially unsafe transaction pattern detected. I blocked this operation for safety."
                        )

            bridge_call = call.model_copy(
                update={
                    "tool_name": tool,
                    "tool": tool,
                    "arguments": args,
                    "args": args,
                }
            )
            result = await invoke_mcp_tool(
                db,
                spec_server_id=spec.mcp_server_id,
                call=bridge_call,
            )
            output_payload = result.output if isinstance(result.output, dict) else result.result
            status = "success" if result.ok else "failed"
            if not result.ok:
                error_payload = {"type": "mcp_error", "message": str(result.error or "MCP execution failed")}
        except Exception as exc:
            status = "failed"
            result = ToolResult(tool_name=tool, tool=tool, ok=False, error=str(exc))
            output_payload = None
            error_payload = {"type": exc.__class__.__name__, "message": str(exc)}
        finally:
            try:
                db.close()
            except Exception:
                pass

        latency_ms = int((time.perf_counter() - started) * 1000)
        if call.user_id and call.run_id:
            db = SessionLocal()
            try:
                insert_tool_execution(
                    db,
                    run_id=call.run_id,
                    user_id=call.user_id,
                    tool_name=tool,
                    input_payload={
                        "args": args,
                        "is_mcp": True,
                        "mcp_server_id": spec.mcp_server_id,
                        "input_provenance": call.input_provenance.value,
                    },
                    output_payload=output_payload,
                    status=status,
                    error_payload=error_payload,
                    idempotency_key=idempotency_key,
                    risk_level=str(spec.risk_level.value),
                    cost_cents=int(result.cost_cents or 0),
                    latency_ms=latency_ms,
                )
            except Exception:
                pass
            finally:
                try:
                    db.close()
                except Exception:
                    pass
        return result

    if tool == "provision_server":
        server_id = str(args.get("server_id") or "").strip().lower()
        reason = str(args.get("reason") or "").strip()
        if not server_id:
            return ToolResult(tool_name=tool, tool=tool, ok=False, error="provision_server requires server_id")
        if not reason:
            reason = "Capability gap detected by planner"

        started = time.perf_counter()
        output_payload_provision: dict[str, object] | None = None
        error_payload_provision: dict[str, str] | None = None
        status_provision = "success"
        db = SessionLocal()
        try:
            if not call.user_id:
                raise RuntimeError("provision_server requires user_id")
            billing_decision = enforce_billing_for_tool_call(db, call.user_id, tool_name="provision_server")
            if not billing_decision.allowed:
                block = billing_decision.block
                msg = (block.message if block else "Billing restriction prevents provisioning right now.").strip()
                raise RuntimeError(msg)
            if not _allow_provisioning_hourly_rate(call.user_id):
                raise RuntimeError(
                    f"Provisioning rate limit reached (max {max(1, int(settings.PROVISIONING_RATE_LIMIT_PER_HOUR or 5))} requests/hour)"
                )

            plan_allowed, plan_message = _enforce_wave56_plan_gate(
                db,
                user_id=call.user_id,
                mcp_server_id=server_id,
            )
            if not plan_allowed:
                raise RuntimeError(plan_message or "This MCP server requires a higher plan.")

            pipeline = ProvisioningPipeline(db)
            _ = pipeline.expire_timeouts()
            max_concurrent = max(1, int(settings.PROVISIONING_MAX_CONCURRENT or 3))
            active_count = int(
                db.execute(
                    text(
                        """
                        select count(1) as cnt from provisioning_requests
                        where user_id = :user_id
                          and state in ('initiated', 'awaiting_auth', 'auth_received', 'provisioning')
                        """
                    ),
                    {"user_id": call.user_id},
                ).scalar()
                or 0
            )
            if active_count >= max_concurrent:
                raise RuntimeError(f"Too many concurrent provisioning sessions (max {max_concurrent})")

            available = available_servers_for_user(db, user_id=call.user_id, connected_server_ids=set())
            matched = next((item for item in available if str(item.get("server_id") or "").strip().lower() == server_id), None)
            if not matched:
                upgrade_url = (
                    f"{settings.APP_BASE_URL.rstrip('/')}/api/v1/billing/checkout?user_id={call.user_id}&plan=professional"
                    if settings.APP_BASE_URL
                    else ""
                )
                result = ToolResult(
                    tool_name=tool,
                    tool=tool,
                    ok=False,
                    error=(
                        f"{server_id} is not available on this account or plan. "
                        f"{('Upgrade path: ' + upgrade_url) if upgrade_url else ''}"
                    ).strip(),
                )
                status_provision = "failed"
                error_payload_provision = {"type": "catalog_not_available", "message": str(result.error or "")}
            else:
                auth_type = _map_auth_type(str(matched.get("auth_type") or "oauth2"))
                request_record = pipeline.begin(
                    user_id=call.user_id,
                    server_id=server_id,
                    reason=reason,
                    trigger=ProvisionTrigger.CAPABILITY_GAP,
                    auth_type=auth_type,
                    original_task_id=call.run_id,
                )
                auth_handler = get_auth_handler(auth_type.value)
                auth_payload = auth_handler.begin(
                    ProvisionAuthContext(
                        request_id=request_record.id,
                        user_id=call.user_id,
                        server_id=server_id,
                        reason=reason,
                        original_task_id=call.run_id,
                    )
                )

                status_value = str(auth_payload.get("status") or "awaiting_auth").strip().lower()
                if status_value == "auth_received":
                    request_record = pipeline.transition(
                        request_id=request_record.id,
                        new_state=ProvisioningState.AUTH_RECEIVED,
                        note="auth_not_required",
                    )
                else:
                    if request_record.state != ProvisioningState.AWAITING_AUTH:
                        request_record = pipeline.transition(
                            request_id=request_record.id,
                            new_state=ProvisioningState.AWAITING_AUTH,
                            note="auth_pending",
                        )

                output_payload_provision = {
                    "request_id": request_record.id,
                    "state": request_record.state.value,
                    "server_id": server_id,
                    "reason": reason,
                    "auth_type": auth_type.value,
                    "setup_seconds": int(matched.get("setup_seconds") or 30),
                    **auth_payload,
                }
                result = ToolResult(tool_name=tool, tool=tool, ok=True, result=output_payload_provision)
                emit_event_async(
                    event_name="provisioning_requested",
                    user_id=call.user_id,
                    source="hands_provision",
                    payload={
                        "request_id": request_record.id,
                        "server_id": server_id,
                        "state": request_record.state.value,
                        "auth_type": auth_type.value,
                    },
                )
                if request_record.state == ProvisioningState.AWAITING_AUTH:
                    emit_event_async(
                        event_name="awaiting_auth",
                        user_id=call.user_id,
                        source="hands_provision",
                        payload={
                            "request_id": request_record.id,
                            "server_id": server_id,
                        },
                    )
        except Exception as exc:
            result = ToolResult(tool_name=tool, tool=tool, ok=False, error=str(exc))
            status_provision = "failed"
            output_payload_provision = None
            error_payload_provision = {"type": exc.__class__.__name__, "message": str(exc)}
            if call.user_id:
                emit_event_async(
                    event_name="provisioning_failed",
                    user_id=call.user_id,
                    source="hands_provision",
                    payload={"server_id": server_id, "error": str(exc)},
                )
        finally:
            try:
                db.close()
            except Exception:
                pass

        latency_ms = int((time.perf_counter() - started) * 1000)
        if call.user_id and call.run_id:
            db = SessionLocal()
            try:
                insert_tool_execution(
                    db,
                    run_id=call.run_id,
                    user_id=call.user_id,
                    tool_name=tool,
                    input_payload={
                        "args": args,
                        "is_mcp": False,
                        "mcp_server_id": None,
                        "input_provenance": call.input_provenance.value,
                    },
                    output_payload=output_payload_provision,
                    status=status_provision,
                    error_payload=error_payload_provision,
                    idempotency_key=idempotency_key,
                    risk_level="low",
                    cost_cents=0,
                    latency_ms=latency_ms,
                )
            except Exception:
                pass
            finally:
                try:
                    db.close()
                except Exception:
                    pass

        return result

    if tool == "search_remote_catalog":
        capability = str(args.get("capability") or "").strip()
        if not capability:
            return ToolResult(tool_name=tool, tool=tool, ok=False, error="search_remote_catalog requires capability")
        limit = max(1, min(25, int(args.get("limit") or 10)))

        started = time.perf_counter()
        output_payload_search: dict[str, object] | None = None
        error_payload_search: dict[str, str] | None = None
        status_search = "success"
        db = SessionLocal()
        try:
            if call.user_id and not _allow_remote_catalog_hourly_rate(call.user_id):
                raise RuntimeError(
                    "Remote catalog search rate limit reached "
                    f"(max {max(1, int(settings.PROVISIONING_REMOTE_SEARCH_RATE_LIMIT_PER_HOUR or 20))} requests/hour)"
                )
            rows = search_remote_catalog_entries(capability=capability, limit=limit)
            plan = _current_user_plan(db, user_id=call.user_id) if call.user_id else "free"
            filtered: list[dict[str, object]] = []
            for row in rows:
                if not isinstance(row, dict):
                    continue
                server_id = str(row.get("server_id") or "").strip().lower()
                min_plan = str(row.get("min_plan") or "free").strip().lower()
                required_plan = min_plan
                if server_id and server_id in _wave56_server_ids():
                    required_plan = str(settings.WAVE56_MIN_PLAN or "professional").strip().lower()
                if not _plan_meets_min(plan, required_plan):
                    continue
                filtered.append(
                    {
                        "server_id": server_id,
                        "display_name": str(row.get("display_name") or row.get("server_id") or "").strip(),
                        "description": str(row.get("description") or "").strip(),
                        "auth_type": str(row.get("auth_type") or "oauth2").strip().lower(),
                        "min_plan": required_plan,
                        "setup_seconds": int(row.get("setup_seconds") or 30),
                        "capabilities": list(row.get("capabilities") or []),
                        "source": str(row.get("source") or "remote").strip().lower(),
                    }
                )
            output_payload_search = {
                "capability": capability,
                "matches": filtered,
                "count": len(filtered),
                "plan": plan,
            }
            result = ToolResult(tool_name=tool, tool=tool, ok=True, result=output_payload_search)
        except Exception as exc:
            result = ToolResult(tool_name=tool, tool=tool, ok=False, error=str(exc))
            status_search = "failed"
            output_payload_search = None
            error_payload_search = {"type": exc.__class__.__name__, "message": str(exc)}
        finally:
            try:
                db.close()
            except Exception:
                pass

        latency_ms = int((time.perf_counter() - started) * 1000)
        if call.user_id and call.run_id:
            db = SessionLocal()
            try:
                insert_tool_execution(
                    db,
                    run_id=call.run_id,
                    user_id=call.user_id,
                    tool_name=tool,
                    input_payload={
                        "args": args,
                        "is_mcp": False,
                        "mcp_server_id": None,
                        "input_provenance": call.input_provenance.value,
                    },
                    output_payload=output_payload_search,
                    status=status_search,
                    error_payload=error_payload_search,
                    idempotency_key=idempotency_key,
                    risk_level="none",
                    cost_cents=0,
                    latency_ms=latency_ms,
                )
            except Exception:
                pass
            finally:
                try:
                    db.close()
                except Exception:
                    pass
        return result

    if tool in ("web.search", "tavily.search"):
        query = str(args.get("query") or "").strip()
        if not query:
            raise HTTPException(status_code=400, detail="Missing args.query")

        started = time.perf_counter()
        output_payload: dict[str, object] | None = None
        error_payload: dict[str, str] | None = None
        status = "success"
        try:
            data = await tavily_search(query, max_results=5)
        except TavilyNotConfiguredError as exc:
            result = ToolResult(tool_name=tool, tool=tool, ok=False, error=str(exc))
            status = "failed"
            output_payload = None
            error_payload = {"type": "not_configured", "message": str(exc)}
        except Exception as exc:
            result = ToolResult(tool_name=tool, tool=tool, ok=False, error=str(exc))
            status = "failed"
            output_payload = None
            error_payload = {"type": exc.__class__.__name__, "message": str(exc)}
        else:
            output_payload = {"query": query, "data": data}
            result = ToolResult(tool_name=tool, tool=tool, ok=True, result=output_payload)
            status = "success"
            error_payload = None

        latency_ms = int((time.perf_counter() - started) * 1000)

        # Best-effort logging to blueprint table.
        if call.user_id and call.run_id:
            db = SessionLocal()
            try:
                insert_tool_execution(
                    db,
                    run_id=call.run_id,
                    user_id=call.user_id,
                    tool_name=tool,
                    input_payload={
                        "args": args,
                        "is_mcp": spec.is_mcp,
                        "mcp_server_id": spec.mcp_server_id,
                        "input_provenance": call.input_provenance.value,
                    },
                    output_payload=output_payload,
                    status=status,
                    error_payload=error_payload,
                    idempotency_key=idempotency_key,
                    risk_level="none",
                    cost_cents=0,
                    latency_ms=latency_ms,
                )
            except Exception:
                pass
            finally:
                try:
                    db.close()
                except Exception:
                    pass

        return result

    if tool == "docs.search":
        if not call.user_id:
            raise HTTPException(status_code=400, detail="docs.search requires user_id")
        query = str(args.get("query") or "").strip()
        if not query:
            raise HTTPException(status_code=400, detail="docs.search requires query")
        max_results = int(args.get("max_results") or 8)

        started = time.perf_counter()
        output_payload_docs: dict[str, object] | None = None
        error_payload_docs: dict[str, str] | None = None
        status_docs = "success"
        db = SessionLocal()
        try:
            output_payload_docs = search_connected_docs(
                db,
                user_id=call.user_id,
                query=query,
                max_results=max_results,
            )
            result = ToolResult(tool_name=tool, tool=tool, ok=True, result=output_payload_docs)
        except Exception as exc:
            result = ToolResult(tool_name=tool, tool=tool, ok=False, error=str(exc))
            status_docs = "failed"
            output_payload_docs = None
            error_payload_docs = {"type": exc.__class__.__name__, "message": str(exc)}
        finally:
            try:
                db.close()
            except Exception:
                pass

        latency_ms = int((time.perf_counter() - started) * 1000)
        if call.user_id and call.run_id:
            db = SessionLocal()
            try:
                insert_tool_execution(
                    db,
                    run_id=call.run_id,
                    user_id=call.user_id,
                    tool_name=tool,
                    input_payload={
                        "args": args,
                        "is_mcp": spec.is_mcp,
                        "mcp_server_id": spec.mcp_server_id,
                        "input_provenance": call.input_provenance.value,
                    },
                    output_payload=output_payload_docs,
                    status=status_docs,
                    error_payload=error_payload_docs,
                    idempotency_key=idempotency_key,
                    risk_level="none",
                    cost_cents=0,
                    latency_ms=latency_ms,
                )
            except Exception:
                pass
            finally:
                try:
                    db.close()
                except Exception:
                    pass
        return result

    if tool in (
        "calendar.list",
        "calendar.create",
        "calendar.update",
        "calendar.delete",
        "calendar.find_free_slots",
        "microsoft.calendar.list",
        "microsoft.calendar.create",
        "microsoft.calendar.update",
        "microsoft.calendar.delete",
    ):
        if not call.user_id:
            raise HTTPException(status_code=400, detail="calendar tools require user_id")

        started = time.perf_counter()
        db = SessionLocal()
        output_payload_cal: dict[str, object] | None = None
        error_payload_cal: dict[str, str] | None = None
        status_cal = "success"
        try:
            provider = (args.get("provider") if isinstance(args, dict) else None) or None
            if tool.startswith("microsoft.calendar."):
                provider = "microsoft"
            provider = str(provider) if provider else None

            action_tool = tool.replace("microsoft.", "", 1) if tool.startswith("microsoft.") else tool

            if action_tool == "calendar.list":
                start_utc = _parse_dt(str(args.get("start_utc") or "")) or datetime.now(timezone.utc)
                end_utc = _parse_dt(str(args.get("end_utc") or "")) or (start_utc + timedelta(days=7))
                max_results = int(args.get("max_results") or 20)
                output_payload_cal = {
                    "events": calendar_list_events(
                        db=db,
                        user_id=call.user_id,
                        start_utc=start_utc,
                        end_utc=end_utc,
                        provider=provider,
                        max_results=max_results,
                    )
                }
            elif action_tool == "calendar.create":
                start_raw = _parse_dt(str(args.get("start_utc") or ""))
                end_raw = _parse_dt(str(args.get("end_utc") or ""))
                if not start_raw or not end_raw:
                    raise HTTPException(status_code=400, detail="calendar.create requires start_utc and end_utc")
                start_utc = start_raw
                end_utc = end_raw
                output_payload_cal = {
                    "event": calendar_create_event(
                        db=db,
                        user_id=call.user_id,
                        title=str(args.get("title") or "Untitled"),
                        start_utc=start_utc,
                        end_utc=end_utc,
                        description=(str(args.get("description")) if args.get("description") is not None else None),
                        location=(str(args.get("location")) if args.get("location") is not None else None),
                        provider=provider,
                    )
                }
            elif action_tool == "calendar.update":
                event_id = str(args.get("event_id") or "").strip()
                if not event_id:
                    raise HTTPException(status_code=400, detail="calendar.update requires event_id")
                output_payload_cal = {
                    "event": calendar_update_event(
                        db=db,
                        user_id=call.user_id,
                        event_id=event_id,
                        title=(str(args.get("title")) if args.get("title") is not None else None),
                        start_utc=_parse_dt(str(args.get("start_utc") or "")),
                        end_utc=_parse_dt(str(args.get("end_utc") or "")),
                        description=(str(args.get("description")) if args.get("description") is not None else None),
                        location=(str(args.get("location")) if args.get("location") is not None else None),
                        provider=provider,
                    )
                }
            elif action_tool == "calendar.delete":
                event_id = str(args.get("event_id") or "").strip()
                if not event_id:
                    raise HTTPException(status_code=400, detail="calendar.delete requires event_id")
                output_payload_cal = {
                    "result": calendar_delete_event(
                        db=db,
                        user_id=call.user_id,
                        event_id=event_id,
                        provider=provider,
                    )
                }
            else:
                start_utc = _parse_dt(str(args.get("start_utc") or "")) or datetime.now(timezone.utc)
                end_utc = _parse_dt(str(args.get("end_utc") or "")) or (start_utc + timedelta(days=7))
                duration_minutes = int(args.get("duration_minutes") or 30)
                output_payload_cal = {
                    "slots": calendar_find_free_slots(
                        db=db,
                        user_id=call.user_id,
                        start_utc=start_utc,
                        end_utc=end_utc,
                        duration_minutes=duration_minutes,
                        provider=provider,
                    )
                }

            output_validation_error = _validate_side_effect_output(
                tool=tool,
                args=args,
                output_payload=output_payload_cal,
            )
            if output_validation_error:
                result = ToolResult(tool_name=tool, tool=tool, ok=False, error=output_validation_error)
                status_cal = "failed"
                error_payload_cal = {"type": "output_validation_error", "message": output_validation_error}
            else:
                result = ToolResult(tool_name=tool, tool=tool, ok=True, result=output_payload_cal)
                status_cal = "success"
                error_payload_cal = None
        except HTTPException:
            raise
        except Exception as exc:
            result = ToolResult(tool_name=tool, tool=tool, ok=False, error=str(exc))
            status_cal = "failed"
            output_payload_cal = None
            error_payload_cal = {"type": exc.__class__.__name__, "message": str(exc)}
        finally:
            try:
                db.close()
            except Exception:
                pass

        latency_ms = int((time.perf_counter() - started) * 1000)
        if call.user_id and call.run_id:
            db = SessionLocal()
            try:
                action_tool = tool.replace("microsoft.", "", 1) if tool.startswith("microsoft.") else tool
                risk_level = "low"
                compensating_action = None
                if action_tool in {"calendar.create", "calendar.update", "calendar.delete"}:
                    risk_level = "medium"
                if status_cal == "success" and action_tool == "calendar.create":
                    event_id = (((output_payload_cal or {}).get("event") or {}).get("id"))
                    if event_id:
                        compensating_action = {
                            "tool": "calendar.delete",
                            "arguments": {"event_id": event_id, "provider": provider},
                        }
                insert_tool_execution(
                    db,
                    run_id=call.run_id,
                    user_id=call.user_id,
                    tool_name=tool,
                    input_payload={
                        "args": args,
                        "is_mcp": spec.is_mcp,
                        "mcp_server_id": spec.mcp_server_id,
                        "input_provenance": call.input_provenance.value,
                    },
                    output_payload=output_payload_cal,
                    status=status_cal,
                    error_payload=error_payload_cal,
                    idempotency_key=idempotency_key,
                    risk_level=risk_level,
                    cost_cents=0,
                    latency_ms=latency_ms,
                    compensating_action=compensating_action,
                )
                if status_cal == "success" and action_tool in {"calendar.create", "calendar.update", "calendar.delete"}:
                    record_side_effect(
                        db,
                        run_id=call.run_id,
                        user_id=call.user_id,
                        effect_type=action_tool,
                        description=f"Calendar side effect executed via {tool}",
                        metadata={
                            "tool": tool,
                            "provider": provider,
                            "arguments": args,
                            "compensating_action": compensating_action,
                        },
                        reversible=bool(compensating_action),
                    )
            except Exception:
                pass
            finally:
                try:
                    db.close()
                except Exception:
                    pass
        return result

    if tool in ("gmail.list", "gmail.search", "gmail.get", "microsoft.mail.list", "microsoft.mail.search", "microsoft.mail.get", "microsoft.contacts.search"):
        if not call.user_id:
            raise HTTPException(status_code=400, detail=f"{tool} requires user_id")

        started = time.perf_counter()
        output_payload_data: dict[str, object] | None = None
        error_payload_data: dict[str, str] | None = None
        status_data = "success"
        db = SessionLocal()
        try:
            if tool == "gmail.list":
                output_payload_data = {
                    "messages": list_recent_gmail_messages(
                        db=db,
                        user_id=call.user_id,
                        max_results=int(args.get("max_results") or 10),
                        hours_back=int(args.get("hours_back") or 24),
                        unread_only=bool(args.get("unread_only", True)),
                    )
                }
            elif tool == "gmail.search":
                query = str(args.get("query") or "").strip()
                if not query:
                    raise HTTPException(status_code=400, detail="gmail.search requires query")
                output_payload_data = {
                    "messages": search_gmail_messages(
                        db=db,
                        user_id=call.user_id,
                        query=query,
                        max_results=int(args.get("max_results") or 10),
                        include_body=bool(args.get("include_body", False)),
                    )
                }
            elif tool == "gmail.get":
                message_id = str(args.get("message_id") or "").strip()
                if not message_id:
                    raise HTTPException(status_code=400, detail="gmail.get requires message_id")
                output_payload_data = {
                    "message": get_gmail_message(
                        db=db,
                        user_id=call.user_id,
                        message_id=message_id,
                        include_body=bool(args.get("include_body", True)),
                    )
                }
            elif tool == "microsoft.mail.list":
                output_payload_data = {
                    "messages": list_recent_outlook_messages(
                        db=db,
                        user_id=call.user_id,
                        max_results=int(args.get("max_results") or 10),
                        hours_back=int(args.get("hours_back") or 24),
                        unread_only=bool(args.get("unread_only", True)),
                        include_body=bool(args.get("include_body", False)),
                    )
                }
            elif tool == "microsoft.mail.search":
                query = str(args.get("query") or "").strip()
                if not query:
                    raise HTTPException(status_code=400, detail="microsoft.mail.search requires query")
                output_payload_data = {
                    "messages": search_outlook_messages(
                        db=db,
                        user_id=call.user_id,
                        query=query,
                        max_results=int(args.get("max_results") or 10),
                        include_body=bool(args.get("include_body", False)),
                    )
                }
            elif tool == "microsoft.mail.get":
                message_id = str(args.get("message_id") or "").strip()
                if not message_id:
                    raise HTTPException(status_code=400, detail="microsoft.mail.get requires message_id")
                output_payload_data = {
                    "message": get_outlook_message(
                        db=db,
                        user_id=call.user_id,
                        message_id=message_id,
                        include_body=bool(args.get("include_body", True)),
                    )
                }
            else:
                query = str(args.get("query") or "").strip()
                if not query:
                    raise HTTPException(status_code=400, detail="microsoft.contacts.search requires query")
                output_payload_data = {
                    "contacts": search_microsoft_contacts(
                        db=db,
                        user_id=call.user_id,
                        query=query,
                        max_results=int(args.get("max_results") or 10),
                    )
                }
            result = ToolResult(tool_name=tool, tool=tool, ok=True, result=output_payload_data)
        except HTTPException:
            raise
        except Exception as exc:
            result = ToolResult(tool_name=tool, tool=tool, ok=False, error=str(exc))
            status_data = "failed"
            output_payload_data = None
            error_payload_data = {"type": exc.__class__.__name__, "message": str(exc)}
        finally:
            try:
                db.close()
            except Exception:
                pass

        latency_ms = int((time.perf_counter() - started) * 1000)
        if call.user_id and call.run_id:
            db = SessionLocal()
            try:
                insert_tool_execution(
                    db,
                    run_id=call.run_id,
                    user_id=call.user_id,
                    tool_name=tool,
                    input_payload={
                        "args": args,
                        "is_mcp": spec.is_mcp,
                        "mcp_server_id": spec.mcp_server_id,
                        "input_provenance": call.input_provenance.value,
                    },
                    output_payload=output_payload_data,
                    status=status_data,
                    error_payload=error_payload_data,
                    idempotency_key=idempotency_key,
                    risk_level="low",
                    cost_cents=0,
                    latency_ms=latency_ms,
                )
            except Exception:
                pass
            finally:
                try:
                    db.close()
                except Exception:
                    pass
        return result

    if tool in ("slack.messages.list", "slack.messages.send", "slack.channel.summary"):
        if not call.user_id:
            raise HTTPException(status_code=400, detail=f"{tool} requires user_id")
        started = time.perf_counter()
        output_payload_slack: dict[str, object] | None = None
        error_payload_slack: dict[str, str] | None = None
        status_slack = "success"
        db = SessionLocal()
        try:
            channel_id = str(args.get("channel_id") or "").strip()
            if not channel_id:
                raise HTTPException(status_code=400, detail=f"{tool} requires channel_id")
            if tool == "slack.messages.list":
                output_payload_slack = {
                    "messages": slack_list_messages(
                        db=db,
                        user_id=call.user_id,
                        channel_id=channel_id,
                        limit=int(args.get("limit") or 20),
                    )
                }
            elif tool == "slack.messages.send":
                text_body = str(args.get("text") or "").strip()
                if not text_body:
                    raise HTTPException(status_code=400, detail="slack.messages.send requires text")
                output_payload_slack = {
                    "message": slack_send_message(
                        db=db,
                        user_id=call.user_id,
                        channel_id=channel_id,
                        text_body=text_body,
                        thread_ts=(str(args.get("thread_ts") or "").strip() or None),
                    )
                }
            else:
                output_payload_slack = {
                    "summary": slack_channel_summary(
                        db=db,
                        user_id=call.user_id,
                        channel_id=channel_id,
                        limit=int(args.get("limit") or 50),
                    )
                }
            result = ToolResult(tool_name=tool, tool=tool, ok=True, result=output_payload_slack)
        except SlackNotConfiguredError as exc:
            result = ToolResult(tool_name=tool, tool=tool, ok=False, error=str(exc))
            status_slack = "failed"
            output_payload_slack = None
            error_payload_slack = {"type": "slack_not_configured", "message": str(exc)}
        except HTTPException:
            raise
        except Exception as exc:
            result = ToolResult(tool_name=tool, tool=tool, ok=False, error=str(exc))
            status_slack = "failed"
            output_payload_slack = None
            error_payload_slack = {"type": exc.__class__.__name__, "message": str(exc)}
        finally:
            try:
                db.close()
            except Exception:
                pass

        latency_ms = int((time.perf_counter() - started) * 1000)
        if call.user_id and call.run_id:
            db = SessionLocal()
            try:
                risk = "low" if tool != "slack.messages.send" else "high"
                insert_tool_execution(
                    db,
                    run_id=call.run_id,
                    user_id=call.user_id,
                    tool_name=tool,
                    input_payload={
                        "args": args,
                        "is_mcp": spec.is_mcp,
                        "mcp_server_id": spec.mcp_server_id,
                        "input_provenance": call.input_provenance.value,
                    },
                    output_payload=output_payload_slack,
                    status=status_slack,
                    error_payload=error_payload_slack,
                    idempotency_key=idempotency_key,
                    risk_level=risk,
                    cost_cents=0,
                    latency_ms=latency_ms,
                )
                if status_slack == "success" and tool == "slack.messages.send":
                    record_side_effect(
                        db,
                        run_id=call.run_id,
                        user_id=call.user_id,
                        effect_type="slack_send",
                        description="Sent Slack message",
                        metadata={"channel_id": str(args.get("channel_id") or "")},
                        reversible=False,
                    )
            except Exception:
                pass
            finally:
                try:
                    db.close()
                except Exception:
                    pass
        return result

    if tool in ("plaid.accounts.list", "plaid.transactions.list"):
        if not call.user_id:
            raise HTTPException(status_code=400, detail=f"{tool} requires user_id")
        started = time.perf_counter()
        output_payload_plaid: dict[str, object] | None = None
        error_payload_plaid: dict[str, str] | None = None
        status_plaid = "success"
        db = SessionLocal()
        try:
            stage = str(args.get("stage") or "staging").strip().lower()
            if stage == "prod":
                # Phase 3 remains sandbox/staging-first.
                raise PlaidNotConfiguredError("Plaid production mode is disabled in Phase 3")

            if tool == "plaid.accounts.list":
                output_payload_plaid = {
                    "accounts": plaid_list_accounts(
                        db=db,
                        user_id=call.user_id,
                        stage="staging",
                    ).get("accounts", []),
                }
            else:
                start_raw = str(args.get("start_date") or "").strip()
                end_raw = str(args.get("end_date") or "").strip()
                if not start_raw or not end_raw:
                    raise HTTPException(status_code=400, detail="plaid.transactions.list requires start_date and end_date")
                start_date = datetime.fromisoformat(start_raw).date()
                end_date = datetime.fromisoformat(end_raw).date()
                output_payload_plaid = {
                    "transactions": plaid_list_transactions(
                        db=db,
                        user_id=call.user_id,
                        start_date=start_date,
                        end_date=end_date,
                        stage="staging",
                    ).get("transactions", []),
                }
            result = ToolResult(tool_name=tool, tool=tool, ok=True, result=output_payload_plaid)
        except PlaidNotConfiguredError as exc:
            result = ToolResult(tool_name=tool, tool=tool, ok=False, error=str(exc))
            status_plaid = "failed"
            output_payload_plaid = None
            error_payload_plaid = {"type": "plaid_not_configured", "message": str(exc)}
        except HTTPException:
            raise
        except Exception as exc:
            result = ToolResult(tool_name=tool, tool=tool, ok=False, error=str(exc))
            status_plaid = "failed"
            output_payload_plaid = None
            error_payload_plaid = {"type": exc.__class__.__name__, "message": str(exc)}
        finally:
            try:
                db.close()
            except Exception:
                pass

        latency_ms = int((time.perf_counter() - started) * 1000)
        if call.user_id and call.run_id:
            db = SessionLocal()
            try:
                insert_tool_execution(
                    db,
                    run_id=call.run_id,
                    user_id=call.user_id,
                    tool_name=tool,
                    input_payload={
                        "args": args,
                        "is_mcp": spec.is_mcp,
                        "mcp_server_id": spec.mcp_server_id,
                        "input_provenance": call.input_provenance.value,
                    },
                    output_payload=output_payload_plaid,
                    status=status_plaid,
                    error_payload=error_payload_plaid,
                    idempotency_key=idempotency_key,
                    risk_level="low",
                    cost_cents=0,
                    latency_ms=latency_ms,
                )
            except Exception:
                pass
            finally:
                try:
                    db.close()
                except Exception:
                    pass
        return result

    if tool == "email.send":
        if not call.user_id:
            raise HTTPException(status_code=400, detail="email.send requires user_id")

        to_email = str(args.get("to_email") or "").strip()
        subject = str(args.get("subject") or "").strip()
        body_text = str(args.get("body_text") or "").strip()
        mode = str(args.get("mode") or "review").strip().lower()
        provider = str(args.get("provider") or "").strip().lower() or None
        cc = str(args.get("cc") or "").strip() or None
        bcc = str(args.get("bcc") or "").strip() or None

        ok_payload, err_msg = _validate_email_payload(to_email=to_email, subject=subject, body_text=body_text)
        if not ok_payload:
            return ToolResult(tool_name=tool, tool=tool, ok=False, error=err_msg or "invalid email payload")

        started = time.perf_counter()
        db = SessionLocal()
        output_payload_email: dict[str, object] | None = None
        error_payload_email: dict[str, str] | None = None
        status_email = "success"
        compensating_action: dict[str, object] | None = None
        try:
            if mode in {"draft", "review"}:
                draft_provider = provider
                draft_id = ""
                if draft_provider == "google":
                    draft = gmail_create_draft(
                        db=db,
                        user_id=call.user_id,
                        to_email=to_email,
                        subject=subject,
                        body_text=body_text,
                        cc=cc,
                        bcc=bcc,
                    )
                    draft_id = str(draft.get("id") or "")
                elif draft_provider == "microsoft":
                    draft = create_outlook_draft(
                        db=db,
                        user_id=call.user_id,
                        to_email=to_email,
                        subject=subject,
                        body_text=body_text,
                        cc=cc,
                        bcc=bcc,
                    )
                    draft_id = str(draft.get("id") or "")

                approval_token = _build_email_review_token(
                    run_id=call.run_id,
                    to_email=to_email,
                    subject=subject,
                    body_text=body_text,
                )
                output_payload_email = {
                    "status": "awaiting_approval",
                    "provider": draft_provider or settings.EMAIL_PROVIDER or "ses",
                    "draft_id": draft_id,
                    "recipient": to_email,
                    "subject": subject,
                    "body_preview": body_text[:320],
                    "approval_token": approval_token,
                    "next_step": "Call email.send again with mode=send and the same approval_token.",
                }
                result = ToolResult(tool_name=tool, tool=tool, ok=True, result=output_payload_email)
            else:
                expected = _build_email_review_token(
                    run_id=call.run_id,
                    to_email=to_email,
                    subject=subject,
                    body_text=body_text,
                )
                supplied = str(args.get("approval_token") or "").strip()
                if not supplied or supplied != expected:
                    raise RuntimeError("email.send requires valid approval_token from draft/review step")
                send_result = _send_email_by_provider(
                    provider=provider,
                    db=db,
                    user_id=call.user_id,
                    to_email=to_email,
                    subject=subject,
                    body_text=body_text,
                    cc=cc,
                    bcc=bcc,
                )
                output_payload_email = {
                    "status": "sent",
                    "provider": send_result.get("provider"),
                    "message_id": send_result.get("message_id"),
                    "recipient": to_email,
                    "subject": subject,
                }
                compensating_action = {
                    "tool": "email.send",
                    "strategy": "send_followup_correction",
                    "note": "Email cannot be unsent; send correction message if needed.",
                }
                result = ToolResult(tool_name=tool, tool=tool, ok=True, result=output_payload_email)
            output_validation_error = _validate_side_effect_output(
                tool=tool,
                args=args,
                output_payload=output_payload_email,
            )
            if output_validation_error:
                result = ToolResult(tool_name=tool, tool=tool, ok=False, error=output_validation_error)
                status_email = "failed"
                output_payload_email = None
                error_payload_email = {"type": "output_validation_error", "message": output_validation_error}
        except Exception as exc:
            result = ToolResult(tool_name=tool, tool=tool, ok=False, error=str(exc))
            status_email = "failed"
            output_payload_email = None
            error_payload_email = {"type": exc.__class__.__name__, "message": str(exc)}
        finally:
            try:
                db.close()
            except Exception:
                pass

        latency_ms = int((time.perf_counter() - started) * 1000)
        if call.user_id and call.run_id:
            db = SessionLocal()
            try:
                insert_tool_execution(
                    db,
                    run_id=call.run_id,
                    user_id=call.user_id,
                    tool_name=tool,
                    input_payload={
                        "args": args,
                        "is_mcp": spec.is_mcp,
                        "mcp_server_id": spec.mcp_server_id,
                        "input_provenance": call.input_provenance.value,
                    },
                    output_payload=output_payload_email,
                    status=status_email,
                    error_payload=error_payload_email,
                    idempotency_key=idempotency_key,
                    risk_level="high",
                    cost_cents=0,
                    latency_ms=latency_ms,
                    compensating_action=compensating_action,
                )
                if status_email == "success" and mode == "send":
                    record_side_effect(
                        db,
                        run_id=call.run_id,
                        user_id=call.user_id,
                        effect_type="email.send",
                        description="Outbound email sent",
                        metadata={
                            "to_email": to_email,
                            "subject": subject,
                            "provider": provider or settings.EMAIL_PROVIDER or "ses",
                            "compensating_action": compensating_action,
                        },
                        reversible=bool(compensating_action),
                    )
            except Exception:
                pass
            finally:
                try:
                    db.close()
                except Exception:
                    pass
        return result

    return ToolResult(tool_name=tool, tool=tool, ok=False, error=f"Unknown tool: {tool}")
