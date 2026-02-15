# backend/app/api/routes/webhooks_whatsapp.py

import hashlib
import hmac
import json
import logging
from fastapi import APIRouter, Depends, HTTPException, Request, Query
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.channels.whatsapp import (
    normalize_whatsapp_webhook,
    send_whatsapp_text,
    extract_whatsapp_statuses,
    mark_whatsapp_read,
)
from app.services.inbound_events import already_processed, record_inbound
from app.services.history import (
    get_or_create_conversation,
    get_recent_history,
    store_message,
    trim_history,
)
from app.services.memory import update_memory_from_turn
from app.services.usage import record_message
from app.services.orchestrator import run_orchestrator
from app.services.contacts_service import upsert_contact
from app.services.messaging_service import record_delivery_status
from app.services.profile_service import update_profile
from app.services.location_service import build_location_patch
from app.core.config import settings
from app.middleware.rate_limiter import rate_limit_webhook

# Blueprint gateway async processing (staging/prod only)
from app.core.redis import get_redis
from app.blueprint.contracts import Channel
from app.blueprint.session import dedup_inbound

# Set up logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

router = APIRouter(prefix="/webhooks/whatsapp", tags=["webhooks"])

VERIFY_TOKEN = settings.WHATSAPP_VERIFY_TOKEN
APP_SECRET = settings.WHATSAPP_APP_SECRET


def _verify_signature(raw_body: bytes, signature_header: str) -> bool:
    if not APP_SECRET:
        # Enforce in staging/production when configured.
        if settings.ENV in ("production", "staging") and settings.ENFORCE_WEBHOOK_SIGNATURES == "1":
            logger.error("WHATSAPP_APP_SECRET not set; signature verification required")
            return False
        logger.warning("WHATSAPP_APP_SECRET not set; skipping signature verification")
        return True
    if not signature_header:
        return False
    try:
        algo, sig = signature_header.split("=", 1)
    except ValueError:
        return False
    if algo != "sha256":
        return False
    expected = hmac.new(APP_SECRET.encode("utf-8"), raw_body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, sig)


@router.get("")
def verify_webhook(
    hub_mode: str = Query("", alias="hub.mode"),
    hub_challenge: str = Query("", alias="hub.challenge"),
    hub_verify_token: str = Query("", alias="hub.verify_token"),
):
    """
    Facebook/WhatsApp webhook verification endpoint.
    Called when you first set up the webhook in Meta dashboard.
    """
    logger.info("=" * 80)
    logger.info("WEBHOOK VERIFICATION REQUEST RECEIVED")
    logger.info(f"Mode: {hub_mode}")
    logger.info(f"Challenge: {hub_challenge}")
    logger.info(f"Verify Token Received: {hub_verify_token}")
    logger.info(f"Expected Verify Token: {VERIFY_TOKEN}")
    logger.info("=" * 80)
    
    if not VERIFY_TOKEN:
        logger.error("⚠️  WHATSAPP_VERIFY_TOKEN not set in environment!")
        raise HTTPException(status_code=500, detail="Verify token not configured")
    
    if hub_verify_token != VERIFY_TOKEN:
        logger.error(f"❌ Token mismatch! Received: {hub_verify_token}, Expected: {VERIFY_TOKEN}")
        raise HTTPException(status_code=403, detail="Invalid verify token")
    
    logger.info(f"✅ Verification successful! Returning challenge: {hub_challenge}")
    # MUST return the challenge as plain text / int
    return int(hub_challenge)


@rate_limit_webhook()
@router.post("")
async def receive(request: Request, db: Session = Depends(get_db)):
    """
    Receives incoming WhatsApp messages from Facebook webhook.
    """
    try:
        raw_body = await request.body()
        sig_header = request.headers.get("X-Hub-Signature-256", "")
        if not _verify_signature(raw_body, sig_header):
            logger.warning("WhatsApp webhook signature verification failed")
            raise HTTPException(status_code=403, detail="Invalid signature")

        payload = json.loads(raw_body.decode("utf-8")) if raw_body else {}

        logger.info("WhatsApp webhook received")
        
        # Handle delivery receipts
        statuses = extract_whatsapp_statuses(payload)
        if statuses:
            processed = 0
            for status in statuses:
                message_id = status.get("id") or ""
                provider_status = status.get("status") or ""
                record_delivery_status(
                    db,
                    provider="whatsapp",
                    provider_message_id=message_id,
                    provider_status=provider_status,
                    payload=status.get("raw") if isinstance(status.get("raw"), dict) else status,
                )
                processed += 1
            return {"ok": True, "status_events": processed}

        # Normalize the webhook payload
        event = normalize_whatsapp_webhook(payload)
        
        if not event:
            logger.warning("⚠️  Event normalized to None (likely a status update or non-text message)")
            logger.warning("WhatsApp webhook ignored (non-text/status)")
            return {"ok": True, "reason": "ignored_non_text_or_status"}
        
        external_id = event["external_id"]
        from_phone = event["from"]
        event_type = event.get("type", "text")

        masked_phone = f"{from_phone[:3]}***{from_phone[-2:]}" if from_phone else "unknown"
        logger.info("WhatsApp message received: id=%s from=%s type=%s", external_id, masked_phone, event_type)
        
        # Blueprint behavior: in staging/prod we ACK fast and process async.
        if settings.ENV in ("staging", "production") and not settings.DATABASE_URL.startswith("sqlite"):
            # Best-effort fast signal: mark the inbound message as read.
            try:
                mark_whatsapp_read(external_id)
            except Exception:
                pass

            r = get_redis()
            if r:
                is_new = dedup_inbound(r, channel=Channel.WHATSAPP, channel_msg_id=external_id)
                if not is_new:
                    return {"ok": True, "deduped": True}

            try:
                from app.tasks.inbound_whatsapp import process_whatsapp_inbound

                process_whatsapp_inbound.delay({**event, "raw": payload})
            except Exception as e:
                logger.exception("Failed to enqueue WhatsApp inbound event")
                return {"ok": False, "error": str(e)}

            return {"ok": True, "queued": True}

        # DEV/TEST behavior: keep synchronous processing for local + unit tests.
        # Use phone as user_id for now (simple MVP).
        user_id = from_phone
        get_or_create_user(db, user_id)

        try:
            upsert_contact(db, user_id=user_id, name=None, phone=from_phone, tags=["whatsapp"])
        except Exception:
            pass
        
        # Idempotency: avoid processing the same message twice
        if already_processed(db, external_id):
            logger.info(f"⏭️  Message {external_id} already processed (deduped)")
            return {"ok": True, "deduped": True}
        
        if event_type == "location":
            location = event.get("location") or {}
            label = location.get("name") or location.get("address")
            patch = build_location_patch(
                source="whatsapp",
                latitude=location.get("latitude"),
                longitude=location.get("longitude"),
                location_label=label,
            )
            profile = update_profile(db, user_id, patch)

            reply = "Location saved. Thanks! You can revoke anytime in your preferences."
            send_whatsapp_text(to_phone_e164=from_phone, text=reply)

            convo = get_or_create_conversation(db, user_id)
            store_message(db, user_id, convo.id, "user", "[Location shared]")
            store_message(db, user_id, convo.id, "assistant", reply)
            trim_history(db, user_id)
            record_message(db, user_id, count=1)

            record_inbound(db, channel="whatsapp", external_id=external_id, user_id=user_id)
            logger.info("Recorded inbound location %s", external_id)
            return {"ok": True, "location_saved": True, "profile": profile}

        text = event["text"]
        logger.info("WhatsApp text message length=%s", len(text))

        # Load last messages for this user (minimal history)
        history = get_recent_history(db, user_id)
        
        logger.info("Running orchestrator to generate reply...")
        reply = run_orchestrator(db=db, user_id=user_id, history=history, user_message=text)
        logger.info("Generated reply (len=%s)", len(reply or ""))
        
        # Send reply back to WhatsApp (no-op if not configured)
        logger.info("Sending reply to %s", masked_phone)
        send_whatsapp_text(to_phone_e164=from_phone, text=reply)
        logger.info("Reply sent")

        # Persist conversation + messages
        convo = get_or_create_conversation(db, user_id)
        store_message(db, user_id, convo.id, "user", text)
        store_message(db, user_id, convo.id, "assistant", reply)
        trim_history(db, user_id)
        record_message(db, user_id, count=1)

        # Update rolling memory (non-blocking)
        try:
            update_memory_from_turn(
                db=db,
                user_id=user_id,
                user_message=text,
                assistant_message=reply,
            )
        except Exception:
            pass

        # Record this message as processed
        record_inbound(db, channel="whatsapp", external_id=external_id, user_id=user_id)
        logger.info("Recorded inbound message %s", external_id)
        
        return {"ok": True}
    
    except Exception as e:
        logger.error("=" * 80)
        logger.error(f"❌ ERROR processing webhook: {str(e)}")
        logger.error(f"Exception type: {type(e).__name__}")
        logger.exception("Full traceback:")
        logger.error("=" * 80)
        # Return 200 to prevent Facebook from retrying
        return {"ok": False, "error": str(e)}
