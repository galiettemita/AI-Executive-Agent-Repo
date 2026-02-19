from __future__ import annotations

import json
import logging
import time
import uuid
from typing import Any, Literal

import httpx
from fastapi import APIRouter, BackgroundTasks, HTTPException, Request
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field
from sqlalchemy import text

from app.blueprint.ace import build_dual_memory_signals, classify_action
from app.blueprint.capability_tokens import issue_capability_token
from app.blueprint.brain.responder import generate_reply
from app.blueprint.brain.tier_router import route_tier
from app.blueprint.db import is_uuid, insert_message, record_side_effect
from app.blueprint.contracts import Channel, ContentProvenance, InboundMessage, InputModality, MessageDirection, OutboundMessage
from app.blueprint.knowledge_files import get_latest_knowledge_file
from app.blueprint.memory_engine import record_memory_dual_path
from app.blueprint.multimodal import preprocess_inbound_message, transcribe_audio_url
from app.blueprint.preferences_learning import record_feedback_signal
from app.blueprint.progress import append_progress, get_progress_events, set_run_result, get_run_result
from app.channels.imessage import apply_imessage_constraints
from app.channels.slack import apply_slack_constraints
from app.channels.whatsapp import mark_whatsapp_read
from app.channels.whatsapp_adapter import send_whatsapp_adapted
from app.channels.whatsapp_templates import WHATSAPP_TEMPLATE_REGISTRY
from app.core.config import settings
from app.core.redis import get_redis
from app.db.database import SessionLocal
from app.middleware.rate_limiter import rate_limit_user
from app.services.billing_middleware import enforce_billing_for_inbound_message
from app.services.clerk_auth import verify_clerk_user_id
from app.services.content_safety import (
    classify_input_async,
    classify_output_sync,
    enforce_gateway_burst_limit,
    enforce_safety_circuit_rate_limit,
)
from app.services.analytics import emit_event_async
from app.services.quality_eval import enqueue_live_quality_eval
from app.services.slack_connector import slack_send_message

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/v1", tags=["gateway-v1"])


class MessageRequest(BaseModel):
    channel: Channel = Channel.WEB
    channel_identifier: str = ""
    user_id: str | None = None
    content: str = ""
    modality: InputModality = InputModality.TEXT
    media_url: str | None = None
    media_type: str | None = None
    wa_message_id: str | None = None
    reply_to_id: str | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)


class MessageAccepted(BaseModel):
    run_id: str
    status: Literal["queued", "processing", "completed", "failed"]


def _resolve_user_id(payload: MessageRequest, request: Request) -> str | None:
    if payload.user_id:
        return payload.user_id

    clerk_user = (request.headers.get("x-clerk-user-id") or "").strip()
    if clerk_user and settings.CLERK_SECRET_KEY:
        if verify_clerk_user_id(clerk_user):
            return clerk_user
        raise HTTPException(status_code=401, detail="Invalid Clerk user")
    return None


def _default_brain_base_url() -> str | None:
    if settings.BRAIN_INTERNAL_BASE_URL:
        return settings.BRAIN_INTERNAL_BASE_URL.rstrip("/")
    if settings.ENV in ("staging", "production"):
        return "http://brain.executive-os.local:8000"
    return None


def _session_key(channel: Channel, channel_identifier: str) -> str:
    return f"bp:v1:session:{channel.value}:{channel_identifier}"


def _participant_key(*, user_id: str | None, channel: Channel, channel_identifier: str | None) -> str:
    if user_id:
        return user_id
    return f"{channel.value}:{channel_identifier or 'anonymous'}"


def _active_channel_key(participant_key: str) -> str:
    return f"bp:v1:active-channel:{participant_key}"


def _processing_lock_key(participant_key: str) -> str:
    return f"bp:v1:processing-lock:{participant_key}"


def _set_active_channel(*, participant_key: str, channel: Channel) -> None:
    client = get_redis()
    if client is None:
        return
    try:
        client.set(
            _active_channel_key(participant_key),
            channel.value,
            ex=max(60, settings.REDIS_SESSION_TTL_SECONDS),
        )
    except Exception:
        pass


def _get_active_channel(*, participant_key: str, fallback: Channel) -> Channel:
    client = get_redis()
    if client is None:
        return fallback
    try:
        raw = client.get(_active_channel_key(participant_key))
    except Exception:
        return fallback
    if not raw:
        return fallback
    try:
        return Channel(str(raw))
    except Exception:
        return fallback


def _acquire_processing_lock(*, participant_key: str) -> tuple[bool, str | None]:
    client = get_redis()
    if client is None:
        return True, None
    token = str(uuid.uuid4())
    try:
        ok = client.set(_processing_lock_key(participant_key), token, nx=True, ex=30)
        return bool(ok), token if ok else None
    except Exception:
        # Prefer availability over strict lock enforcement if Redis is unavailable.
        return True, None


def _release_processing_lock(*, participant_key: str, token: str | None) -> None:
    if not token:
        return
    client = get_redis()
    if client is None:
        return
    key = _processing_lock_key(participant_key)
    try:
        current = client.get(key)
        if current == token:
            client.delete(key)
    except Exception:
        pass


def _resolve_channel_identifier_for_user(*, user_id: str | None, channel: Channel, fallback: str) -> str:
    if not user_id:
        return fallback
    db = SessionLocal()
    try:
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            row = db.execute(
                text(
                    """
                    select channel_identifier
                    from channel_connections
                    where user_id = :user_id and channel = :channel
                    order by is_primary desc
                    limit 1
                    """
                ),
                {"user_id": user_id, "channel": channel.value},
            ).mappings().first()
        else:
            row = db.execute(
                text(
                    """
                    select channel_identifier
                    from channel_connections
                    where user_id::text = :user_id and channel = (:channel)::channel_type
                    order by is_primary desc
                    limit 1
                    """
                ),
                {"user_id": user_id, "channel": channel.value},
            ).mappings().first()
        ident = str((row or {}).get("channel_identifier") or "").strip()
        return ident or fallback
    except Exception:
        return fallback
    finally:
        try:
            db.close()
        except Exception:
            pass


def _format_response_for_channel(*, channel: Channel, text_value: str) -> str:
    if channel == Channel.IMESSAGE:
        return apply_imessage_constraints(text_value)
    if channel == Channel.SLACK:
        return apply_slack_constraints(text_value)
    # WhatsApp formatting/splitting is handled by adapter at send time.
    return text_value


def _update_conversation_channel_state(*, conversation_id: str, channel: Channel) -> None:
    db = SessionLocal()
    try:
        dialect = db.bind.dialect.name if db.bind is not None else ""
        has_table = False
        try:
            if dialect == "sqlite":
                has_table = bool(
                    db.execute(
                        text("select name from sqlite_master where type='table' and name='conversations'")
                    ).first()
                )
            else:
                has_table = bool(
                    db.execute(
                        text(
                            "select 1 from information_schema.tables "
                            "where table_schema = current_schema() and table_name = 'conversations'"
                        )
                    ).first()
                )
        except Exception:
            has_table = False
        if not has_table:
            return

        if dialect == "sqlite":
            row = db.execute(
                text("select channels_used from conversations where id = :id"),
                {"id": conversation_id},
            ).mappings().first()
            used = []
            if row and row.get("channels_used"):
                try:
                    used = json.loads(str(row.get("channels_used") or "[]"))
                except Exception:
                    used = []
            if channel.value not in used:
                used.append(channel.value)
            db.execute(
                text(
                    """
                    update conversations
                    set active_channel = :active_channel,
                        channels_used = :channels_used
                    where id = :id
                    """
                ),
                {
                    "active_channel": channel.value,
                    "channels_used": json.dumps(used, ensure_ascii=False),
                    "id": conversation_id,
                },
            )
        else:
            # v5 uses UUID IDs and JSONB channels_used.
            db.execute(
                text(
                    """
                    update conversations
                    set active_channel = :active_channel,
                        channels_used = (
                          case
                            when channels_used is null then to_jsonb(array[:active_channel]::text[])
                            when jsonb_typeof(channels_used) = 'array' and not (channels_used ? :active_channel)
                              then channels_used || to_jsonb(array[:active_channel]::text[])
                            else channels_used
                          end
                        )
                    where id::text = :id
                    """
                ),
                {"active_channel": channel.value, "id": conversation_id},
            )
        db.commit()
    except Exception:
        try:
            db.rollback()
        except Exception:
            pass
    finally:
        try:
            db.close()
        except Exception:
            pass


def _get_account_emotion_sensitivity(user_id: str | None) -> float:
    if not user_id:
        return 0.5
    db = SessionLocal()
    try:
        row = db.execute(
            text("select emotion_sensitivity from accounts where id = :user_id"),
            {"user_id": user_id},
        ).mappings().first()
        value = (row or {}).get("emotion_sensitivity")
        return float(value) if value is not None else 0.5
    except Exception:
        return 0.5
    finally:
        try:
            db.close()
        except Exception:
            pass


def _get_agents_overrides(user_id: str | None) -> str:
    if not user_id:
        return ""
    db = SessionLocal()
    try:
        item = get_latest_knowledge_file(db, user_id=user_id, file_path="AGENTS.md")
        return str((item or {}).get("content") or "")
    except Exception:
        return ""
    finally:
        try:
            db.close()
        except Exception:
            pass


def _get_ace_dual_memory_signals(user_id: str | None) -> dict[str, Any]:
    if not user_id or not is_uuid(user_id):
        return {}
    db = SessionLocal()
    try:
        return build_dual_memory_signals(db, user_id=user_id)
    except Exception:
        return {}
    finally:
        try:
            db.close()
        except Exception:
            pass


def _persist_v1_message(
    *,
    user_id: str | None,
    conversation_id: str | None,
    run_id: str,
    direction: MessageDirection,
    channel: Channel,
    text: str,
    input_modality: InputModality = InputModality.TEXT,
    media_url: str | None = None,
    transcription_confidence: float | None = None,
    extracted_entities: dict[str, Any] | None = None,
    content_provenance: ContentProvenance = ContentProvenance.USER_DIRECT,
    emotion_detected: str = "neutral",
    channel_msg_id: str | None = None,
    intent: str | None = None,
    tier: int | None = None,
    latency_ms: int | None = None,
) -> None:
    if not user_id or not is_uuid(user_id):
        return
    db = SessionLocal()
    try:
        insert_message(
            db,
            conversation_id=conversation_id or "",
            user_id=user_id,
            direction=direction,
            content={
                "text": text,
                "channel": channel.value,
                "input_modality": input_modality.value,
                "media_url": media_url,
                "transcription_confidence": transcription_confidence,
                "extracted_entities": extracted_entities or {},
                "content_provenance": content_provenance.value,
                "emotion_detected": emotion_detected,
            },
            channel_msg_id=channel_msg_id,
            intent=intent,
            tier=tier,
            run_id=run_id,
            latency_ms=latency_ms,
        )
    except Exception:
        logger.warning("v1 message persistence failed run_id=%s", run_id, exc_info=True)
    finally:
        try:
            db.close()
        except Exception:
            pass


def _resolve_session_id(channel: Channel, channel_identifier: str) -> str:
    key = _session_key(channel, channel_identifier)
    client = get_redis()
    if client is not None:
        existing = client.get(key)
        if existing:
            return existing
        session_id = str(uuid.uuid4())
        client.set(key, session_id, ex=settings.REDIS_SESSION_TTL_SECONDS)
        return session_id

    # Fallback if Redis is unavailable.
    return str(uuid.uuid4())


def _sse(data: dict[str, Any]) -> str:
    return f"data: {json.dumps(data, ensure_ascii=False)}\n\n"


def _brain_respond(msg: InboundMessage) -> OutboundMessage:
    base = _default_brain_base_url()
    if not base:
        tier = route_tier(msg.text)
        raw_meta = msg.raw or {}
        reply, meta = generate_reply(
            user_text=msg.text,
            tier=tier,
            user_id=msg.user_id,
            conversation_id=msg.conversation_id,
            run_id=msg.run_id,
            input_provenance=str(raw_meta.get("content_provenance") or "user_direct"),
            capability_token=(str(raw_meta.get("capability_token") or "").strip() or None),
            emotion_detected=(str(raw_meta.get("emotion_detected") or "").strip() or None),
            emotion_sensitivity=float(raw_meta.get("emotion_sensitivity") or 0.5),
        )
        metadata = dict(meta or {})
        metadata.setdefault("tier", tier)
        metadata.setdefault("channel_msg_id", msg.channel_msg_id)
        return OutboundMessage(channel=msg.channel, recipient_id=msg.channel_identifier, content=reply, metadata=metadata)

    with httpx.Client(timeout=45.0) as client:
        resp = client.post(f"{base}/internal/brain/respond", json=msg.model_dump(mode="json"))
        resp.raise_for_status()
        return OutboundMessage.model_validate(resp.json())


def _process_message_run(run_id: str, inbound: InboundMessage) -> None:
    started = time.perf_counter()
    participant_key = _participant_key(
        user_id=inbound.user_id,
        channel=inbound.channel,
        channel_identifier=inbound.channel_identifier,
    )
    lock_ok, lock_token = _acquire_processing_lock(participant_key=participant_key)
    if not lock_ok:
        msg = "I’m still finishing your previous message. Please try again in a few seconds."
        payload = {
            "ok": False,
            "blocked": True,
            "run_id": run_id,
            "reason": "concurrent_message_in_progress",
            "reply": msg,
        }
        set_run_result(run_id, payload)
        append_progress(run_id, step="done", status="failed", partial_result=payload)
        return

    _set_active_channel(participant_key=participant_key, channel=inbound.channel)
    if inbound.conversation_id:
        _update_conversation_channel_state(conversation_id=inbound.conversation_id, channel=inbound.channel)

    try:
        # Billing is the first gate: if blocked, do not run any LLM work.
        append_progress(run_id, step="billing", status="running")
        if inbound.user_id:
            db = SessionLocal()
            try:
                decision = enforce_billing_for_inbound_message(db, inbound.user_id)
            finally:
                try:
                    db.close()
                except Exception:
                    pass
            if not decision.allowed:
                block = decision.block
                msg = (block.message if block else "Billing restricted. Please try again later.").strip()
                _persist_v1_message(
                    user_id=inbound.user_id,
                    conversation_id=inbound.conversation_id,
                    run_id=run_id,
                    direction=MessageDirection.INBOUND,
                    channel=inbound.channel,
                    text=inbound.content or "",
                    input_modality=inbound.input_modality,
                    media_url=inbound.media_url,
                    content_provenance=ContentProvenance.USER_DIRECT,
                    emotion_detected="neutral",
                    channel_msg_id=inbound.channel_msg_id,
                    intent="billing_block",
                    tier=0,
                )
                if inbound.channel == Channel.WHATSAPP and inbound.channel_identifier:
                    send_whatsapp_adapted(to_phone_e164=inbound.channel_identifier, text=msg)
                if inbound.channel == Channel.SLACK and inbound.channel_identifier and inbound.user_id:
                    try:
                        db2 = SessionLocal()
                        try:
                            slack_send_message(
                                db=db2,
                                user_id=inbound.user_id,
                                channel_id=inbound.channel_identifier,
                                text_body=msg,
                            )
                        finally:
                            db2.close()
                    except Exception:
                        logger.warning("Slack billing block send failed run_id=%s", run_id, exc_info=True)

                _persist_v1_message(
                    user_id=inbound.user_id,
                    conversation_id=inbound.conversation_id,
                    run_id=run_id,
                    direction=MessageDirection.OUTBOUND,
                    channel=inbound.channel,
                    text=msg,
                    input_modality=InputModality.TEXT,
                    content_provenance=ContentProvenance.USER_DIRECT,
                    emotion_detected="neutral",
                    channel_msg_id=inbound.channel_msg_id,
                    intent="billing_block",
                    tier=0,
                    latency_ms=int((time.perf_counter() - started) * 1000),
                )

                payload = {
                    "ok": False,
                    "blocked": True,
                    "run_id": run_id,
                    "reason": (block.reason if block else "billing_blocked"),
                    "plan": decision.plan,
                    "status": decision.status,
                    "reply": msg,
                    "retry_after_seconds": (block.retry_after_seconds if block else None),
                }
                set_run_result(run_id, payload)
                append_progress(
                    run_id,
                    step="done",
                    status="completed",
                    partial_result=payload,
                )
                return

        if inbound.user_id:
            burst = enforce_gateway_burst_limit(inbound.user_id, limit_per_minute=10)
            if not burst.allowed:
                msg = "You’re sending messages too quickly. Please wait a moment and try again."
                payload = {
                    "ok": False,
                    "blocked": True,
                    "run_id": run_id,
                    "reason": burst.reason,
                    "reply": msg,
                    "retry_after_seconds": burst.retry_after_seconds,
                }
                set_run_result(run_id, payload)
                append_progress(run_id, step="done", status="completed", partial_result=payload)
                return

            safety_limit = enforce_safety_circuit_rate_limit(inbound.user_id)
            if not safety_limit.allowed:
                msg = "I’m temporarily limiting message volume while recent safety alerts are reviewed."
                payload = {
                    "ok": False,
                    "blocked": True,
                    "run_id": run_id,
                    "reason": safety_limit.reason,
                    "reply": msg,
                    "retry_after_seconds": safety_limit.retry_after_seconds,
                }
                set_run_result(run_id, payload)
                append_progress(run_id, step="done", status="completed", partial_result=payload)
                return

        append_progress(run_id, step="preprocess", status="running")
        processed = preprocess_inbound_message(inbound)

        append_progress(
            run_id,
            step="route",
            status="running",
            partial_result={"tier_hint": route_tier(processed.normalized_text)},
        )
        ace_decision = classify_action(
            processed.normalized_text,
            agents_content=_get_agents_overrides(inbound.user_id),
            dual_memory_signals=_get_ace_dual_memory_signals(inbound.user_id),
        )
        append_progress(run_id, step="ace", status="running", partial_result=ace_decision)
        emotion_sensitivity = _get_account_emotion_sensitivity(inbound.user_id)
        capability_token: str | None = None
        if inbound.user_id:
            try:
                capability_token = issue_capability_token(
                    run_id=run_id,
                    user_id=inbound.user_id,
                    provenance=processed.content_provenance,
                    capabilities=[
                        "web:search",
                        "calendar:read",
                        "calendar:write",
                        "email:read",
                        "email:send",
                        "contacts:read",
                    ],
                )
            except Exception:
                capability_token = None

        normalized_inbound = InboundMessage(
            channel=inbound.channel,
            channel_identifier=inbound.channel_identifier,
            content=processed.normalized_text,
            input_modality=inbound.input_modality,
            media_url=inbound.media_url,
            media_type=inbound.media_type,
            wa_message_id=inbound.wa_message_id,
            reply_to_id=inbound.reply_to_id,
            channel_msg_id=inbound.channel_msg_id,
            user_id=inbound.user_id,
            conversation_id=inbound.conversation_id,
            run_id=run_id,
            from_phone=inbound.from_phone,
            raw={
                **(inbound.raw or {}),
                "extracted_entities": processed.extracted_entities,
                "transcription_confidence": processed.transcription_confidence,
                "emotion_detected": processed.emotion_detected.value,
                "emotion_sensitivity": emotion_sensitivity,
                "content_provenance": (processed.content_provenance if isinstance(processed.content_provenance, ContentProvenance) else ContentProvenance.USER_DIRECT).value,
                "capability_token": capability_token,
            },
        )

        _persist_v1_message(
            user_id=inbound.user_id,
            conversation_id=inbound.conversation_id,
            run_id=run_id,
            direction=MessageDirection.INBOUND,
            channel=inbound.channel,
            text=processed.normalized_text,
            input_modality=inbound.input_modality,
            media_url=inbound.media_url,
            transcription_confidence=processed.transcription_confidence,
            extracted_entities=processed.extracted_entities,
            content_provenance=processed.content_provenance if isinstance(processed.content_provenance, ContentProvenance) else ContentProvenance.USER_DIRECT,
            emotion_detected=processed.emotion_detected.value,
            channel_msg_id=inbound.channel_msg_id,
            intent=ace_decision.get("action_type"),
            tier=route_tier(processed.normalized_text),
        )

        emit_event_async(
            event_name="message_received",
            user_id=inbound.user_id,
            source="gateway_v1",
            payload={
                "run_id": run_id,
                "channel": inbound.channel.value,
                "conversation_id": inbound.conversation_id,
            },
        )

        # Input safety classification runs asynchronously so it does not add user-visible latency.
        classify_input_async(
            user_id=inbound.user_id,
            run_id=run_id,
            channel=inbound.channel.value,
            text_value=processed.normalized_text,
            metadata={
                "conversation_id": inbound.conversation_id,
                "source": "gateway_inbound",
            },
        )

        append_progress(run_id, step="brain", status="running")

        outbound = _brain_respond(normalized_inbound)

        target_channel = _get_active_channel(participant_key=participant_key, fallback=inbound.channel)
        target_identifier = _resolve_channel_identifier_for_user(
            user_id=inbound.user_id,
            channel=target_channel,
            fallback=inbound.channel_identifier,
        )
        formatted_text = _format_response_for_channel(channel=target_channel, text_value=outbound.content)
        outbound = outbound.model_copy(
            update={
                "channel": target_channel,
                "recipient_id": target_identifier,
                "content": formatted_text,
                "text": formatted_text,
            }
        )

        try:
            output_safety = classify_output_sync(
                user_id=inbound.user_id,
                run_id=run_id,
                channel=target_channel.value,
                text_value=outbound.content,
                metadata={
                    "conversation_id": inbound.conversation_id,
                    "source": "gateway_outbound",
                },
            )
            if output_safety.flagged:
                meta = dict(outbound.metadata or {})
                meta["output_safety"] = {
                    "flagged": True,
                    "risk_score": output_safety.risk_score,
                    "categories": output_safety.categories,
                    "classifier": output_safety.classifier,
                }
                if output_safety.risk_score >= 0.55:
                    safe_text = _format_response_for_channel(
                        channel=target_channel,
                        text_value="I can’t help with that request, but I can help with a safer alternative.",
                    )
                    outbound = outbound.model_copy(update={"content": safe_text, "text": safe_text, "metadata": meta})
                else:
                    outbound = outbound.model_copy(update={"metadata": meta})
        except Exception:
            logger.warning("output_safety_classifier_failed run_id=%s", run_id, exc_info=True)

        if inbound.conversation_id:
            _update_conversation_channel_state(conversation_id=inbound.conversation_id, channel=target_channel)

        elapsed_ms = int((time.perf_counter() - started) * 1000)

        if outbound.channel == Channel.WHATSAPP and outbound.recipient_id:
            # WhatsApp has no direct typing indicator API; marking as read gives immediate UX feedback.
            if inbound.wa_message_id:
                try:
                    mark_whatsapp_read(inbound.wa_message_id)
                except Exception:
                    pass
            send_whatsapp_adapted(
                to_phone_e164=outbound.recipient_id,
                text=outbound.content,
                buttons=outbound.buttons,
                metadata=outbound.metadata,
            )
            if inbound.user_id:
                db = SessionLocal()
                try:
                    record_side_effect(
                        db,
                        run_id=run_id,
                        user_id=inbound.user_id,
                        effect_type="whatsapp_send",
                        description="Sent WhatsApp outbound message",
                        metadata={
                            "to": outbound.recipient_id,
                            "reply_preview": (outbound.content or "")[:280],
                        },
                        reversible=False,
                    )
                finally:
                    try:
                        db.close()
                    except Exception:
                        pass
        elif outbound.channel == Channel.SLACK and outbound.recipient_id and inbound.user_id:
            try:
                db = SessionLocal()
                try:
                    sent = slack_send_message(
                        db=db,
                        user_id=inbound.user_id,
                        channel_id=outbound.recipient_id,
                        text_body=outbound.content,
                    )
                    if inbound.user_id:
                        record_side_effect(
                            db,
                            run_id=run_id,
                            user_id=inbound.user_id,
                            effect_type="slack_send",
                            description="Sent Slack outbound message",
                            metadata={"channel": outbound.recipient_id, "ts": sent.get("ts")},
                            reversible=False,
                        )
                finally:
                    try:
                        db.close()
                    except Exception:
                        pass
            except Exception:
                logger.warning("Slack send failed run_id=%s", run_id, exc_info=True)
        elif outbound.channel == Channel.IMESSAGE and outbound.recipient_id:
            # iMessage delivery is currently handled by upstream MBfB transport.
            logger.info("iMessage outbound prepared run_id=%s recipient=%s", run_id, outbound.recipient_id)

        if inbound.user_id:
            db = SessionLocal()
            try:
                record_memory_dual_path(
                    db,
                    user_id=inbound.user_id,
                    run_id=run_id,
                    user_text=processed.normalized_text,
                    assistant_text=outbound.content,
                )
            except Exception:
                logger.warning("memory dual-path update failed run_id=%s", run_id, exc_info=True)
            finally:
                try:
                    db.close()
                except Exception:
                    pass

        _persist_v1_message(
            user_id=inbound.user_id,
            conversation_id=inbound.conversation_id,
            run_id=run_id,
            direction=MessageDirection.OUTBOUND,
            channel=outbound.channel,
            text=outbound.content,
            input_modality=InputModality.TEXT,
            content_provenance=ContentProvenance.USER_DIRECT,
            emotion_detected=str(processed.emotion_detected.value or "neutral"),
            channel_msg_id=inbound.channel_msg_id,
            intent=ace_decision.get("action_type"),
            tier=int((outbound.metadata or {}).get("tier") or route_tier(processed.normalized_text)),
            latency_ms=elapsed_ms,
        )

        try:
            context_chunks = list((outbound.metadata or {}).get("context_chunks") or [])
            used_tools = any(str((chunk or {}).get("source") or "").startswith("tool:") for chunk in context_chunks)
            enqueue_live_quality_eval(
                user_id=inbound.user_id,
                conversation_id=inbound.conversation_id,
                run_id=run_id,
                message_id=inbound.channel_msg_id,
                user_text=processed.normalized_text,
                assistant_text=outbound.content,
                used_tools=used_tools,
                prompt_version_id=(str((outbound.metadata or {}).get("prompt_version_id") or "").strip() or None),
                metadata={"channel": outbound.channel.value, "tier": int((outbound.metadata or {}).get("tier") or 1)},
            )
            emit_event_async(
                event_name="message_sent",
                user_id=inbound.user_id,
                source="gateway_v1",
                payload={
                    "run_id": run_id,
                    "channel": outbound.channel.value,
                    "conversation_id": inbound.conversation_id,
                },
            )
            if used_tools:
                emit_event_async(
                    event_name="tool_invoked",
                    user_id=inbound.user_id,
                    source="gateway_v1",
                    payload={"run_id": run_id, "conversation_id": inbound.conversation_id},
                )
        except Exception:
            logger.warning("live_quality_eval_enqueue_failed run_id=%s", run_id, exc_info=True)

        payload = {
            "ok": True,
            "run_id": run_id,
            "inbound": normalized_inbound.model_dump(mode="json"),
            "outbound": outbound.model_dump(mode="json"),
            "reply": outbound.content,
            "metadata": outbound.metadata,
            "latency_ms": elapsed_ms,
            "entities": processed.extracted_entities,
            "modality": processed.modality.value,
            "ace": ace_decision,
        }
        set_run_result(run_id, payload)
        if inbound.user_id and is_uuid(inbound.user_id):
            db = SessionLocal()
            try:
                record_feedback_signal(
                    db,
                    user_id=inbound.user_id,
                    signal_type="outcome_success",
                    original_output=outbound.content[:400],
                    corrected_output=None,
                    context={
                        "run_id": run_id,
                        "source": "gateway_run_complete",
                        "action_type": ace_decision.get("action_type"),
                    },
                )
            except Exception:
                db.rollback()
            finally:
                try:
                    db.close()
                except Exception:
                    pass
        append_progress(
            run_id,
            step="done",
            status="completed",
            partial_result={
                "reply": outbound.content,
                "latency_ms": elapsed_ms,
            },
            metadata=outbound.metadata,
        )
    except Exception as exc:
        logger.exception("/api/v1/message run failed")
        payload = {
            "ok": False,
            "run_id": run_id,
            "inbound": normalized_inbound.model_dump(mode="json") if "normalized_inbound" in locals() else None,
            "error": str(exc),
        }
        set_run_result(run_id, payload)
        if inbound.user_id and is_uuid(inbound.user_id):
            db = SessionLocal()
            try:
                record_feedback_signal(
                    db,
                    user_id=inbound.user_id,
                    signal_type="outcome_failed",
                    original_output=str(exc)[:400],
                    corrected_output=None,
                    context={
                        "run_id": run_id,
                        "source": "gateway_run_failed",
                        "action_type": "failure",
                    },
                )
            except Exception:
                db.rollback()
            finally:
                try:
                    db.close()
                except Exception:
                    pass
        append_progress(run_id, step="done", status="failed", partial_result={"error": str(exc)})
    finally:
        _release_processing_lock(participant_key=participant_key, token=lock_token)


@rate_limit_user()
@router.post("/message", response_model=MessageAccepted)
def post_message(request: Request, payload: MessageRequest, background_tasks: BackgroundTasks):
    if not payload.content and not payload.media_url:
        raise HTTPException(status_code=400, detail="Either content or media_url is required")

    run_id = str(uuid.uuid4())
    resolved_user_id = _resolve_user_id(payload, request)
    channel_identifier = payload.channel_identifier or resolved_user_id or "anonymous"
    conversation_id = _resolve_session_id(payload.channel, channel_identifier)

    inbound = InboundMessage(
        channel=payload.channel,
        channel_identifier=channel_identifier,
        content=payload.content,
        input_modality=payload.modality,
        media_url=payload.media_url,
        media_type=payload.media_type,
        wa_message_id=payload.wa_message_id,
        reply_to_id=payload.reply_to_id,
        channel_msg_id=payload.wa_message_id,
        user_id=resolved_user_id,
        conversation_id=conversation_id,
        run_id=run_id,
        from_phone=channel_identifier if payload.channel == Channel.WHATSAPP else None,
        raw=payload.metadata,
    )

    run_id = enqueue_inbound_message(background_tasks=background_tasks, inbound=inbound, run_id=run_id)
    return MessageAccepted(run_id=run_id, status="queued")


def enqueue_inbound_message(*, background_tasks: BackgroundTasks, inbound: InboundMessage, run_id: str | None = None) -> str:
    resolved_run_id = (run_id or inbound.run_id or "").strip() or str(uuid.uuid4())
    normalized = inbound.model_copy(update={"run_id": resolved_run_id})
    append_progress(
        resolved_run_id,
        step="accepted",
        status="queued",
        partial_result={"channel": normalized.channel.value},
    )
    background_tasks.add_task(_process_message_run, resolved_run_id, normalized)
    return resolved_run_id


@router.get("/stream/{run_id}")
def stream_run(run_id: str):
    def _events():
        idx = 0
        started = time.time()
        timeout_s = 60 * 2

        while True:
            events, idx = get_progress_events(run_id, after_index=idx)
            for evt in events:
                yield _sse(evt)
                if evt.get("status") in {"completed", "failed", "cancelled"}:
                    return

            if get_run_result(run_id) is not None:
                result = get_run_result(run_id) or {}
                if not events:
                    terminal_status = "completed" if result.get("ok") else "failed"
                    yield _sse(
                        {
                            "ts": int(time.time() * 1000),
                            "step": "done",
                            "status": terminal_status,
                            "partial_result": result,
                        }
                    )
                return

            if (time.time() - started) > timeout_s:
                yield _sse(
                    {
                        "ts": int(time.time() * 1000),
                        "step": "timeout",
                        "status": "failed",
                        "partial_result": {"error": "Stream timeout"},
                    }
                )
                return

            time.sleep(0.35)

    return StreamingResponse(_events(), media_type="text/event-stream")


@router.post("/voice/transcribe")
def voice_transcribe(payload: dict[str, Any]):
    audio_url = str(payload.get("audio_url") or "").strip()
    if not audio_url:
        raise HTTPException(status_code=400, detail="audio_url is required")

    try:
        result = transcribe_audio_url(audio_url)
        return {
            "ok": True,
            "text": result.get("text") or "",
            "confidence": result.get("confidence"),
            "language": result.get("language"),
        }
    except Exception as exc:
        raise HTTPException(status_code=502, detail=f"Transcription failed: {exc}")


@router.get("/channels/whatsapp/templates")
def whatsapp_templates():
    return {
        "ok": True,
        "templates": [
            {
                "name": item.name,
                "language_code": item.language_code,
                "description": item.description,
            }
            for item in WHATSAPP_TEMPLATE_REGISTRY.values()
        ],
    }
