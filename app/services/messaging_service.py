from __future__ import annotations

import json
import logging
from datetime import datetime
from typing import Any, Dict, Optional

from sqlalchemy.orm import Session
from twilio.rest import Client

from app.core.config import settings
from app.db.models import OutboundMessage, Contact
from app.channels.whatsapp import send_whatsapp_text
from app.services.contacts_service import normalize_phone, normalize_email

logger = logging.getLogger(__name__)


def _to_json(value: Any) -> str:
    return json.dumps(value or {}, ensure_ascii=False)


def queue_outbound_message(
    db: Session,
    user_id: str,
    channel: str,
    to_address: Optional[str],
    body: str,
    contact_id: Optional[int] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> OutboundMessage:
    channel = (channel or "").lower()
    if channel not in {"whatsapp", "sms", "email"}:
        raise ValueError("Unsupported channel")

    resolved_to = to_address
    if contact_id:
        contact = db.query(Contact).filter(Contact.id == contact_id, Contact.user_id == user_id).first()
        if not contact:
            raise ValueError("Contact not found")
        if channel in {"whatsapp", "sms"}:
            resolved_to = contact.phone
        elif channel == "email":
            resolved_to = contact.email

    if not resolved_to:
        raise ValueError("Destination address is required")

    message = OutboundMessage(
        user_id=user_id,
        contact_id=contact_id,
        channel=channel,
        to_address=resolved_to,
        body=body,
        status="queued",
        metadata_json=_to_json(metadata) if metadata else None,
    )
    db.add(message)
    db.commit()
    db.refresh(message)
    return message


def _twilio_client() -> Client:
    account_sid = settings.TWILIO_ACCOUNT_SID or ""
    auth_token = settings.TWILIO_AUTH_TOKEN or ""
    if not account_sid or not auth_token:
        raise RuntimeError("Twilio credentials not configured")
    return Client(account_sid, auth_token)


def _send_sms(to_number: str, body: str) -> str:
    from_number = settings.TWILIO_PHONE_NUMBER or ""
    if not from_number:
        raise RuntimeError("TWILIO_PHONE_NUMBER not configured")
    client = _twilio_client()
    msg = client.messages.create(to=to_number, from_=from_number, body=body)
    return msg.sid


def _send_email(to_email: str, body: str) -> str:
    # Email sending is handled elsewhere; this is a placeholder
    raise RuntimeError("Email outbound not wired yet")


def deliver_pending_messages(db: Session, limit: int = 50) -> Dict[str, int]:
    if settings.ENABLE_MESSAGING != "1":
        return {"sent": 0, "failed": 0, "skipped": 0}

    pending = (
        db.query(OutboundMessage)
        .filter(OutboundMessage.status == "queued")
        .order_by(OutboundMessage.created_at.asc())
        .limit(limit)
        .all()
    )

    if not pending:
        return {"sent": 0, "failed": 0, "skipped": 0}

    sent = 0
    failed = 0
    skipped = 0

    for msg in pending:
        try:
            msg.status = "sending"
            db.commit()

            if msg.channel == "whatsapp":
                if not settings.WHATSAPP_TOKEN or not settings.WHATSAPP_PHONE_NUMBER_ID:
                    raise RuntimeError("WhatsApp credentials not configured")
                to_phone = normalize_phone(msg.to_address) or msg.to_address
                send_whatsapp_text(to_phone_e164=to_phone, text=msg.body)
                msg.provider_message_id = None
            elif msg.channel == "sms":
                to_phone = normalize_phone(msg.to_address) or msg.to_address
                msg.provider_message_id = _send_sms(to_phone, msg.body)
            elif msg.channel == "email":
                to_email = normalize_email(msg.to_address) or msg.to_address
                msg.provider_message_id = _send_email(to_email, msg.body)
            else:
                raise RuntimeError("Unsupported channel")

            msg.status = "sent"
            msg.sent_at = datetime.utcnow()
            sent += 1
            db.commit()
        except Exception as exc:
            msg.status = "failed"
            msg.error_message = str(exc)
            db.commit()
            failed += 1
            logger.warning("Outbound message %s failed: %s", msg.id, exc)

    return {"sent": sent, "failed": failed, "skipped": skipped}


def list_messages(db: Session, user_id: str, limit: int = 50) -> list[OutboundMessage]:
    return (
        db.query(OutboundMessage)
        .filter(OutboundMessage.user_id == user_id)
        .order_by(OutboundMessage.created_at.desc())
        .limit(limit)
        .all()
    )


def get_message(db: Session, message_id: int, user_id: Optional[str] = None) -> Optional[OutboundMessage]:
    q = db.query(OutboundMessage).filter(OutboundMessage.id == message_id)
    if user_id:
        q = q.filter(OutboundMessage.user_id == user_id)
    return q.one_or_none()
