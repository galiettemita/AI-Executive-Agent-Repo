# backend/app/api/routes/webhooks_whatsapp.py

import logging
from fastapi import APIRouter, Depends, HTTPException, Request, Query
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.channels.whatsapp import normalize_whatsapp_webhook, send_whatsapp_text
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
from app.core.config import settings

# Set up logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

router = APIRouter(prefix="/webhooks/whatsapp", tags=["webhooks"])

VERIFY_TOKEN = settings.WHATSAPP_VERIFY_TOKEN


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


@router.post("")
async def receive(request: Request, db: Session = Depends(get_db)):
    """
    Receives incoming WhatsApp messages from Facebook webhook.
    """
    try:
        payload = await request.json()
        
        logger.info("=" * 80)
        logger.info("📨 INCOMING WHATSAPP WEBHOOK")
        logger.info(f"Raw payload: {payload}")
        logger.info("=" * 80)
        
        # Normalize the webhook payload
        event = normalize_whatsapp_webhook(payload)
        
        if not event:
            logger.warning("⚠️  Event normalized to None (likely a status update or non-text message)")
            logger.warning(f"Payload was: {payload}")
            return {"ok": True, "reason": "ignored_non_text_or_status"}
        
        logger.info(f"✅ Normalized event: {event}")
        
        external_id = event["external_id"]
        from_phone = event["from"]
        text = event["text"]
        
        logger.info(f"📱 Message from: {from_phone}")
        logger.info(f"💬 Message text: {text}")
        logger.info(f"🆔 External ID: {external_id}")
        
        # Use phone as user_id for now (simple MVP). Later map to a real users table row.
        user_id = from_phone
        get_or_create_user(db, user_id)
        
        # Idempotency: avoid processing the same message twice
        if already_processed(db, external_id):
            logger.info(f"⏭️  Message {external_id} already processed (deduped)")
            return {"ok": True, "deduped": True}
        
        # Load last messages for this user (minimal history)
        history = get_recent_history(db, user_id)
        
        logger.info("🤖 Running orchestrator to generate reply...")
        reply = run_orchestrator(db=db, user_id=user_id, history=history, user_message=text)
        logger.info(f"💡 Generated reply: {reply}")
        
        # Send reply back to WhatsApp (no-op if not configured)
        logger.info(f"📤 Sending reply to {from_phone}...")
        send_whatsapp_text(to_phone_e164=from_phone, text=reply)
        logger.info("✅ Reply sent successfully")

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
        logger.info(f"💾 Recorded inbound message {external_id}")
        logger.info("=" * 80)
        
        return {"ok": True}
    
    except Exception as e:
        logger.error("=" * 80)
        logger.error(f"❌ ERROR processing webhook: {str(e)}")
        logger.error(f"Exception type: {type(e).__name__}")
        logger.exception("Full traceback:")
        logger.error("=" * 80)
        # Return 200 to prevent Facebook from retrying
        return {"ok": False, "error": str(e)}
