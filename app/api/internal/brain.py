from __future__ import annotations

import json

from fastapi import APIRouter
from fastapi.responses import StreamingResponse
from openai import OpenAI

from app.blueprint.db import list_conversation_messages
from app.db.database import SessionLocal

from app.blueprint.brain.responder import generate_reply
from app.blueprint.brain.responder import SYSTEM_PROMPT
from app.blueprint.brain.tier_router import route_tier
from app.blueprint.contracts import InboundMessage, OutboundMessage
from app.core.config import settings
from app.services.semantic_cache import get_cached_response


router = APIRouter(prefix="/internal/brain", tags=["internal-brain"])


@router.post("/respond", response_model=OutboundMessage)
def respond(msg: InboundMessage) -> OutboundMessage:
    """
    Brain Plane: minimal internal API (Phase 1).

    Accepts an inbound message and returns an outbound message.
    """
    tier = route_tier(msg.text)

    history: list[dict[str, str]] = []
    if msg.user_id and msg.conversation_id:
        db = SessionLocal()
        try:
            history = list_conversation_messages(
                db,
                user_id=msg.user_id,
                conversation_id=msg.conversation_id,
                limit=12,
            )
        except Exception:
            history = []
        finally:
            try:
                db.close()
            except Exception:
                pass

        # Avoid duplicating the current inbound message in the prompt context.
        if history and (msg.text or "").strip() and history[-1].get("role") == "user":
            if (history[-1].get("content") or "").strip() == (msg.text or "").strip():
                history = history[:-1]

    reply, meta = generate_reply(
        user_text=msg.text,
        tier=tier,
        user_id=msg.user_id,
        conversation_id=msg.conversation_id,
        run_id=msg.run_id,
        history_messages=history,
    )

    metadata = dict(meta or {})
    metadata.setdefault("tier", tier)
    metadata.setdefault("channel_msg_id", msg.channel_msg_id)
    if msg.run_id:
        metadata.setdefault("run_id", msg.run_id)

    return OutboundMessage(
        channel=msg.channel,
        to_phone=msg.from_phone,
        text=reply,
        metadata=metadata,
    )


def _sse(data: dict) -> str:
    return f"data: {json.dumps(data, ensure_ascii=False)}\n\n"


@router.post("/respond_stream")
def respond_stream(msg: InboundMessage):
    """
    Brain Plane streaming endpoint (SSE).

    Used for web/UIs where streaming is beneficial. WhatsApp does not consume SSE.
    """
    tier = route_tier(msg.text)
    model = settings.OPENAI_MODEL or "gpt-4o-mini"

    history: list[dict[str, str]] = []
    if msg.user_id and msg.conversation_id:
        db = SessionLocal()
        try:
            history = list_conversation_messages(
                db,
                user_id=msg.user_id,
                conversation_id=msg.conversation_id,
                limit=12,
            )
        except Exception:
            history = []
        finally:
            try:
                db.close()
            except Exception:
                pass

        if history and (msg.text or "").strip() and history[-1].get("role") == "user":
            if (history[-1].get("content") or "").strip() == (msg.text or "").strip():
                history = history[:-1]

    # Cache hit: stream the full cached message in a single chunk.
    if msg.user_id:
        cached = get_cached_response(
            user_id=msg.user_id,
            query_text=msg.text,
            model=model,
            tier=tier,
            context={"conversation_id": msg.conversation_id} if msg.conversation_id else None,
        )
        if cached:
            def cached_gen():
                yield _sse({"type": "delta", "text": cached})
                yield _sse({"type": "done", "tier": tier, "model": model, "cache_hit": True})
            return StreamingResponse(cached_gen(), media_type="text/event-stream")

    def gen():
        if not settings.OPENAI_API_KEY:
            yield _sse({"type": "error", "error": "OPENAI_API_KEY not configured"})
            yield _sse({"type": "done", "tier": tier, "model": model})
            return

        client = OpenAI(api_key=settings.OPENAI_API_KEY, timeout=60, max_retries=1)
        messages = [{"role": "system", "content": SYSTEM_PROMPT}]
        if history:
            messages.extend(history)
        messages.append({"role": "user", "content": msg.text})

        stream = client.chat.completions.create(
            model=model,
            messages=messages,
            temperature=0.4,
            stream=True,
        )
        for chunk in stream:
            try:
                delta = chunk.choices[0].delta
                text = getattr(delta, "content", None)
            except Exception:
                text = None
            if text:
                yield _sse({"type": "delta", "text": text})
        yield _sse({"type": "done", "tier": tier, "model": model})

    return StreamingResponse(gen(), media_type="text/event-stream")
