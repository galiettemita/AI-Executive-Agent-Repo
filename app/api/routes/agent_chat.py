import logging
import time
from typing import Any

from fastapi import APIRouter, Depends, HTTPException, Request
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.api.deps import get_or_create_user
from app.schemas.chat import ChatRequest, ChatResponse
from app.services.history import get_recent_history, get_or_create_conversation, store_message, trim_history
from app.services.orchestrator import infer_tier_for_message, run_orchestrator
from app.services.memory import update_memory_from_turn
from app.services.usage import record_message
from app.middleware.rate_limiter import rate_limit_user
from app.core.config import settings

router = APIRouter(prefix="/agent", tags=["agent"])
logger = logging.getLogger(__name__)


def _current_span() -> Any:
    try:
        from opentelemetry import trace

        return trace.get_current_span()
    except Exception:
        return None


def _set_span_attr(span: Any, key: str, value: Any) -> None:
    if span is None or value is None:
        return
    try:
        span.set_attribute(key, value)
    except Exception:
        return


@rate_limit_user()
@router.post("/chat", response_model=ChatResponse)
def agent_chat(request: Request, req: ChatRequest, db: Session = Depends(get_db)):
    started_at = time.perf_counter()
    span = _current_span()
    intent, tier = infer_tier_for_message(req.message)
    _set_span_attr(span, "exec.intent", intent.value)
    _set_span_attr(span, "exec.tier", tier)
    _set_span_attr(span, "exec.entrypoint", "agent_chat")

    # Ensure tier attributes always land in Axiom even if OTEL context doesn't
    # propagate into sync endpoints (threadpool). This creates an explicit span
    # we can query for tier mix dashboards.
    tier_span = None
    if settings.OTEL_ENABLED:
        try:
            from opentelemetry import trace

            tier_span = trace.get_tracer(__name__).start_span("exec.tier")
            tier_span.set_attribute("exec.intent", intent.value)
            tier_span.set_attribute("exec.tier", tier)
            tier_span.set_attribute("exec.user_id", req.user_id)
            tier_span.set_attribute("exec.entrypoint", "agent_chat")
        except Exception:
            tier_span = None

    outcome = "success"
    try:
        get_or_create_user(db, req.user_id)
        convo = get_or_create_conversation(db, req.user_id)
        conversation_id = req.conversation_id or convo.id

        # Load recent history (minimal)
        history = get_recent_history(db, req.user_id)

        # Save user message
        store_message(db, req.user_id, conversation_id, "user", req.message)

        # Run agent
        reply = run_orchestrator(db=db, user_id=req.user_id, history=history, user_message=req.message)

        # Save assistant message
        store_message(db, req.user_id, conversation_id, "assistant", reply)
        trim_history(db, req.user_id)
        record_message(db, req.user_id, count=1)

        # Update memory (non-blocking)
        try:
            update_memory_from_turn(
                db=db,
                user_id=req.user_id,
                user_message=req.message,
                assistant_message=reply,
            )
        except Exception:
            logger.exception("memory update failed")

        return ChatResponse(conversation_id=conversation_id, reply=reply)
    except HTTPException:
        outcome = "http_exception"
        raise
    except Exception as exc:
        outcome = "error"
        _set_span_attr(span, "exec.error_type", exc.__class__.__name__)
        if tier_span is not None:
            try:
                tier_span.record_exception(exc)
            except Exception:
                pass
        raise HTTPException(status_code=500, detail=str(exc))
    finally:
        latency_ms = (time.perf_counter() - started_at) * 1000
        _set_span_attr(span, "exec.outcome", outcome)
        _set_span_attr(span, "exec.latency_ms", round(latency_ms, 2))
        if tier_span is not None:
            try:
                tier_span.set_attribute("exec.outcome", outcome)
                tier_span.set_attribute("exec.latency_ms", round(latency_ms, 2))
                tier_span.end()
            except Exception:
                pass
        logger.info(
            "agent_chat_request intent=%s tier=%s outcome=%s latency_ms=%.2f",
            intent.value,
            tier,
            outcome,
            latency_ms,
        )
