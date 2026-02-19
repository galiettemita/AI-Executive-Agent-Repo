# backend/app/api/routes/webhooks_sms.py

from __future__ import annotations

import logging

from fastapi import APIRouter, Depends, HTTPException, Request
from fastapi.responses import Response
from sqlalchemy.orm import Session
from twilio.request_validator import RequestValidator

from app.api.deps import get_db, get_or_create_user
from app.core.config import settings
from app.middleware.rate_limiter import rate_limit_webhook
from app.services.contacts_service import upsert_contact
from app.services.history import (
    get_or_create_conversation,
    get_recent_history,
    store_message,
    trim_history,
)
from app.services.inbound_events import already_processed, record_inbound
from app.services.memory import update_memory_from_turn
from app.services.messaging_service import (
    deliver_pending_messages,
    queue_outbound_message,
    record_delivery_status,
)
from app.services.orchestrator import run_orchestrator
from app.services.usage import record_message

router = APIRouter(prefix="/webhooks/sms", tags=["webhooks"])
logger = logging.getLogger(__name__)

_EMPTY_TWIML = '<?xml version="1.0" encoding="UTF-8"?><Response></Response>'


def _validate_twilio(request: Request, params: dict) -> bool:
    if settings.ENFORCE_WEBHOOK_SIGNATURES != "1":
        return True
    if not settings.TWILIO_AUTH_TOKEN:
        return False
    signature = request.headers.get("X-Twilio-Signature", "")
    validator = RequestValidator(settings.TWILIO_AUTH_TOKEN)
    return validator.validate(str(request.url), params, signature)


def _twiml_ack() -> Response:
    return Response(content=_EMPTY_TWIML, media_type="application/xml")


@rate_limit_webhook()
@router.post("")
async def sms_inbound_webhook(request: Request, db: Session = Depends(get_db)):
    """
    Twilio inbound SMS/MMS webhook.
    Accepts inbound messages, runs the assistant, and sends an outbound SMS reply.
    """
    form = await request.form()
    params = dict(form)
    if not _validate_twilio(request, params):
        raise HTTPException(status_code=403, detail="Invalid Twilio signature")

    message_sid = (params.get("MessageSid") or params.get("SmsSid") or "").strip()
    from_phone = (params.get("From") or "").strip()
    body = (params.get("Body") or "").strip()
    status = (params.get("MessageStatus") or params.get("SmsStatus") or "").strip().lower()
    try:
        num_media = int(params.get("NumMedia") or "0")
    except (TypeError, ValueError):
        num_media = 0

    # Defensive fallback: if status callbacks are accidentally pointed here, record and exit.
    if status and status not in {"received", "inbound"} and not body and num_media == 0:
        if message_sid:
            record_delivery_status(
                db,
                provider="twilio",
                provider_message_id=message_sid,
                provider_status=status,
                payload=params,
            )
        return _twiml_ack()

    if not message_sid or not from_phone:
        logger.warning("SMS inbound ignored: missing MessageSid or From")
        return _twiml_ack()

    if already_processed(db, message_sid):
        return _twiml_ack()

    user_id = from_phone
    get_or_create_user(db, user_id)

    try:
        upsert_contact(db, user_id=user_id, name=None, phone=from_phone, tags=["sms"])
    except Exception:
        logger.exception("Failed to upsert SMS contact for %s", user_id)

    user_text = body if body else ("[Inbound MMS attachment]" if num_media > 0 else "")

    # Ignore empty inbound payloads, but still record idempotency token.
    if not user_text:
        record_inbound(db, channel="sms", external_id=message_sid, user_id=user_id)
        return _twiml_ack()

    try:
        if num_media > 0 and not body:
            assistant_reply = (
                "I received your attachment. MMS understanding is not enabled yet, "
                "but text messages are fully supported."
            )
        else:
            history = get_recent_history(db, user_id)
            assistant_reply = str(
                run_orchestrator(db=db, user_id=user_id, history=history, user_message=user_text) or ""
            ).strip()
            if not assistant_reply:
                assistant_reply = "I received your message and I am ready to help."

        convo = get_or_create_conversation(db, user_id)
        store_message(db, user_id, convo.id, "user", user_text)
        store_message(db, user_id, convo.id, "assistant", assistant_reply)
        trim_history(db, user_id)
        record_message(db, user_id, count=1)

        try:
            update_memory_from_turn(
                db=db,
                user_id=user_id,
                user_message=user_text,
                assistant_message=assistant_reply,
            )
        except Exception:
            logger.exception("Failed to update memory from inbound SMS turn")

        # Queue and attempt immediate send (no-op when ENABLE_MESSAGING != 1).
        queue_outbound_message(
            db=db,
            user_id=user_id,
            channel="sms",
            to_address=from_phone,
            body=assistant_reply,
            metadata={
                "source": "sms_inbound_webhook",
                "in_reply_to": message_sid,
            },
        )
        deliver_pending_messages(db, limit=1)

    except Exception:
        # Return 200/TwiML so Twilio does not retry aggressively.
        logger.exception("SMS inbound processing failed for sid=%s", message_sid)
    finally:
        try:
            if not already_processed(db, message_sid):
                record_inbound(db, channel="sms", external_id=message_sid, user_id=user_id)
        except Exception:
            logger.exception("Failed to record inbound SMS idempotency token sid=%s", message_sid)

    return _twiml_ack()


@rate_limit_webhook()
@router.post("/status")
async def sms_status_webhook(request: Request, db: Session = Depends(get_db)):
    form = await request.form()
    params = dict(form)
    if not _validate_twilio(request, params):
        raise HTTPException(status_code=403, detail="Invalid Twilio signature")

    message_sid = params.get("MessageSid") or params.get("SmsSid")
    status = params.get("MessageStatus") or params.get("SmsStatus") or ""

    if message_sid:
        record_delivery_status(
            db,
            provider="twilio",
            provider_message_id=message_sid,
            provider_status=status,
            payload=params,
        )

    return {"ok": True}
