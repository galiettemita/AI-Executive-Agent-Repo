from __future__ import annotations

import logging
import time
from typing import Any

import httpx
from sqlalchemy.orm import Session

from app.blueprint.contracts import Channel, MessageDirection
from app.blueprint.contracts import InboundMessage, InputModality, OutboundMessage
from app.blueprint.brain.responder import generate_reply
from app.blueprint.brain.tier_router import route_tier
from app.blueprint.multimodal import preprocess_inbound_message
from app.blueprint.db import (
    attach_run_to_message,
    complete_run,
    create_run,
    get_or_create_user_by_phone,
    insert_message,
    record_side_effect,
)
from app.services.billing_middleware import enforce_billing_for_inbound_message
from app.blueprint.session import get_or_create_conversation_session
from app.channels.whatsapp_adapter import send_whatsapp_adapted
from app.core.celery_app import celery_app
from app.core.config import settings
from app.core.redis import get_redis
from app.db.database import SessionLocal

logger = logging.getLogger(__name__)


def _db() -> Session:
    return SessionLocal()

def _default_brain_base_url() -> str | None:
    if settings.BRAIN_INTERNAL_BASE_URL:
        return settings.BRAIN_INTERNAL_BASE_URL.rstrip("/")
    if settings.ENV in ("staging", "production"):
        # Requires ECS Cloud Map namespace `executive-os.local`.
        return "http://brain.executive-os.local:8000"
    return None


def _brain_respond(msg: InboundMessage) -> OutboundMessage:
    base = _default_brain_base_url()
    if not base:
        tier = route_tier(msg.text)
        reply, meta = generate_reply(user_text=msg.text, tier=tier)
        metadata = dict(meta or {})
        metadata.setdefault("tier", tier)
        metadata.setdefault("channel_msg_id", msg.channel_msg_id)
        return OutboundMessage(channel=msg.channel, to_phone=msg.from_phone, text=reply, metadata=metadata)

    with httpx.Client(timeout=30.0) as client:
        resp = client.post(f"{base}/internal/brain/respond", json=msg.model_dump())
        resp.raise_for_status()
        return OutboundMessage.model_validate(resp.json())


@celery_app.task(name="gateway.process_whatsapp_inbound")
def process_whatsapp_inbound(event: dict[str, Any]) -> dict[str, Any]:
    """
    Gateway async handler (Phase 1).
    Stores inbound/outbound messages in blueprint tables and sends WhatsApp reply.

    Expected event:
      {
        "external_id": "...",
        "from": "+1555...",
        "type": "text"|"location"|...,
        "text": "...",
        "location": {...},
        "raw": {...}
      }
    """
    started = time.perf_counter()
    r = get_redis()
    if r is None:
        raise RuntimeError("REDIS_URL not configured; async processing requires Redis")

    external_id = str(event.get("external_id") or "").strip()
    from_phone = str(event.get("from") or "").strip()
    event_type = str(event.get("type") or "text").strip()
    text = str(event.get("text") or "").strip()
    media_url = str(event.get("media_url") or "").strip() or None

    db = _db()
    try:
        user = get_or_create_user_by_phone(db, from_phone)
        conversation_id = get_or_create_conversation_session(
            db=db,
            r=r,
            user_id=user.id,
            channel=Channel.WHATSAPP,
        )

        # Billing is the first gate: if blocked, do not run any LLM work.
        decision = enforce_billing_for_inbound_message(db, user.id)
        if not decision.allowed:
            block = decision.block
            msg = (block.message if block else "Billing restricted. Please try again later.").strip()
            send_whatsapp_adapted(to_phone_e164=from_phone, text=msg)
            return {
                "ok": False,
                "blocked": True,
                "user_id": user.id,
                "reason": (block.reason if block else "billing_blocked"),
                "plan": decision.plan,
                "status": decision.status,
            }

        inbound_content: dict[str, Any]
        if event_type == "location":
            inbound_content = {"location": event.get("location") or {}}
        else:
            inbound_content = {"text": text}
            if media_url:
                inbound_content["media_url"] = media_url
            if event_type in ("audio", "image", "document"):
                inbound_content["input_modality"] = event_type

        message_id = insert_message(
            db,
            conversation_id=conversation_id,
            user_id=user.id,
            direction=MessageDirection.INBOUND,
            content=inbound_content,
            channel_msg_id=external_id or None,
        )
        if message_id is None:
            return {"ok": True, "deduped": True}

        tier_guess = route_tier(text) if event_type != "location" else 0
        run_id = create_run(
            db,
            user_id=user.id,
            conversation_id=conversation_id,
            envelope={
                "channel": Channel.WHATSAPP.value,
                "channel_msg_id": external_id,
                "from_phone": from_phone,
                "event_type": event_type,
                "text": text,
            },
            intent="general",
            tier=tier_guess,
        )
        attach_run_to_message(db, user_id=user.id, message_id=message_id, run_id=run_id)

        if event_type == "location":
            reply = "Location saved. What do you want me to do next?"
            insert_message(
                db,
                conversation_id=conversation_id,
                user_id=user.id,
                direction=MessageDirection.OUTBOUND,
                content={"text": reply},
                intent="general",
                tier=0,
                run_id=run_id,
                latency_ms=int((time.perf_counter() - started) * 1000),
                cost_cents=0,
            )
            complete_run(
                db,
                run_id=run_id,
                user_id=user.id,
                state="completed",
                total_cost_cents=0,
                total_latency_ms=int((time.perf_counter() - started) * 1000),
                llm_provider="openai",
                knowledge_files_injected=[],
            )
            send_whatsapp_adapted(to_phone_e164=from_phone, text=reply)
            record_side_effect(
                db,
                run_id=run_id,
                user_id=user.id,
                effect_type="whatsapp_send",
                description="Sent WhatsApp outbound message",
                metadata={"to": from_phone, "reply_preview": reply[:280]},
                reversible=False,
            )
            return {"ok": True, "user_id": user.id, "conversation_id": conversation_id, "tier": 0, "meta": {}}

        modality = InputModality.TEXT
        if event_type == "audio":
            modality = InputModality.VOICE
        elif event_type == "image":
            modality = InputModality.IMAGE
        elif event_type == "document":
            modality = InputModality.DOCUMENT
        elif event_type == "location":
            modality = InputModality.LOCATION

        inbound_msg = InboundMessage(
            channel=Channel.WHATSAPP,
            channel_msg_id=external_id or "unknown",
            user_id=user.id,
            conversation_id=conversation_id,
            run_id=run_id,
            from_phone=from_phone,
            text=text,
            input_modality=modality,
            media_url=media_url,
            raw=event.get("raw") if isinstance(event.get("raw"), dict) else {},
        )

        processed = preprocess_inbound_message(inbound_msg)
        inbound_msg.content = processed.normalized_text
        inbound_msg.text = processed.normalized_text
        inbound_msg.raw = {
            **(inbound_msg.raw or {}),
            "extracted_entities": processed.extracted_entities,
            "transcription_confidence": processed.transcription_confidence,
            "emotion_detected": processed.emotion_detected.value,
            "content_provenance": processed.content_provenance.value,
        }

        try:
            outbound = _brain_respond(inbound_msg)
            reply = outbound.text
            tier = int((outbound.metadata or {}).get("tier") or tier_guess or 1)
            meta = dict(outbound.metadata or {})
            outcome_state = "completed"
            error_payload = None
        except Exception as exc:
            logger.exception("Brain call failed; returning fallback reply")
            reply = "I hit an internal error. Try again in a minute."
            tier = tier_guess or 1
            meta = {"error": True, "error_type": exc.__class__.__name__}
            outcome_state = "failed"
            error_payload = {"type": exc.__class__.__name__, "message": str(exc)}

        insert_message(
            db,
            conversation_id=conversation_id,
            user_id=user.id,
            direction=MessageDirection.OUTBOUND,
            content={"text": reply},
            intent="general",
            tier=tier,
            run_id=run_id,
            latency_ms=int((time.perf_counter() - started) * 1000),
            cost_cents=0,
        )

        complete_run(
            db,
            run_id=run_id,
            user_id=user.id,
            state=outcome_state,
            total_cost_cents=0,
            total_latency_ms=int((time.perf_counter() - started) * 1000),
            error=error_payload,
            llm_provider=str(meta.get("provider") or "openai") if isinstance(meta, dict) else "openai",
            knowledge_files_injected=list(meta.get("knowledge_files_injected") or []) if isinstance(meta, dict) else [],
        )

        # Send outbound message (no-op if WA creds not configured)
        send_whatsapp_adapted(
            to_phone_e164=from_phone,
            text=reply,
            metadata=meta if isinstance(meta, dict) else None,
        )
        record_side_effect(
            db,
            run_id=run_id,
            user_id=user.id,
            effect_type="whatsapp_send",
            description="Sent WhatsApp outbound message",
            metadata={"to": from_phone, "reply_preview": reply[:280]},
            reversible=False,
        )

        return {
            "ok": True,
            "user_id": user.id,
            "conversation_id": conversation_id,
            "tier": tier,
            "meta": meta,
        }
    except Exception:
        logger.exception("process_whatsapp_inbound failed")
        raise
    finally:
        try:
            db.close()
        except Exception:
            pass
