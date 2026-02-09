from __future__ import annotations

import json
import logging
from datetime import datetime
from typing import Any, Dict, Optional

from sqlalchemy.orm import Session
from twilio.rest import Client

from app.core.config import settings
from app.db.models import OutboundMessage, OutboundMessageEvent, Contact
from app.channels.whatsapp import send_whatsapp_text
from app.services.contacts_service import normalize_phone, normalize_email
from app.services.analytics_service import record_usage_event

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
        provider_status="queued",
        metadata_json=_to_json(metadata) if metadata else None,
    )
    db.add(message)
    db.commit()
    db.refresh(message)
    if contact_id:
        try:
            from app.services.relationship_service import log_interaction
            log_interaction(
                db,
                user_id=user_id,
                contact_id=contact_id,
                direction="outbound",
                channel=channel,
                summary=None,
                occurred_at=message.created_at or datetime.utcnow(),
                metadata={"source": "outbound_message", "message_id": message.id},
            )
        except Exception:
            pass
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


def _send_whatsapp(to_number: str, body: str) -> Optional[str]:
    if not settings.WHATSAPP_TOKEN or not settings.WHATSAPP_PHONE_NUMBER_ID:
        raise RuntimeError("WhatsApp credentials not configured")
    return send_whatsapp_text(to_phone_e164=to_number, text=body)


CHANNEL_PROVIDERS = {
    "whatsapp": ("whatsapp", _send_whatsapp),
    "sms": ("twilio", _send_sms),
    "email": ("email", _send_email),
}


def _record_event(
    db: Session,
    *,
    message_id: Optional[int],
    provider: Optional[str],
    event_type: str,
    payload: Optional[Dict[str, Any]] = None,
) -> None:
    event = OutboundMessageEvent(
        message_id=message_id,
        provider=provider,
        event_type=event_type,
        payload_json=_to_json(payload) if payload else None,
    )
    db.add(event)
    db.commit()


def _apply_provider_status(msg: OutboundMessage, provider_status: str) -> None:
    status = (provider_status or "").lower()
    if status in {"delivered", "read"}:
        msg.status = "delivered"
        if not msg.delivered_at:
            msg.delivered_at = datetime.utcnow()
    elif status in {"failed", "undelivered"}:
        msg.status = "failed"
        if not msg.failed_at:
            msg.failed_at = datetime.utcnow()
    elif status in {"sent", "queued"}:
        if msg.status not in {"delivered", "failed"}:
            msg.status = "sent"


def record_delivery_status(
    db: Session,
    *,
    provider: str,
    provider_message_id: str,
    provider_status: str,
    payload: Optional[Dict[str, Any]] = None,
) -> Optional[OutboundMessage]:
    if not provider_message_id:
        return None

    msg = (
        db.query(OutboundMessage)
        .filter(OutboundMessage.provider_message_id == provider_message_id)
        .first()
    )

    if not msg:
        _record_event(
            db,
            message_id=None,
            provider=provider,
            event_type=provider_status,
            payload=payload,
        )
        return None

    msg.provider = provider
    msg.provider_status = provider_status
    msg.last_status_at = datetime.utcnow()
    if provider_status.lower() in {"failed", "undelivered"} and payload:
        error = payload.get("ErrorMessage") or payload.get("error_message") or payload.get("error")
        if error:
            msg.error_message = str(error)
    _apply_provider_status(msg, provider_status)
    db.commit()

    _record_event(
        db,
        message_id=msg.id,
        provider=provider,
        event_type=provider_status,
        payload=payload,
    )
    return msg


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

            provider_name, sender = CHANNEL_PROVIDERS.get(msg.channel, (None, None))
            if not provider_name or not sender:
                raise RuntimeError("Unsupported channel")

            if msg.channel in {"whatsapp", "sms"}:
                to_address = normalize_phone(msg.to_address) or msg.to_address
            else:
                to_address = normalize_email(msg.to_address) or msg.to_address

            msg.provider = provider_name
            msg.provider_message_id = sender(to_address, msg.body)

            msg.status = "sent"
            msg.provider_status = "sent"
            msg.sent_at = datetime.utcnow()
            sent += 1
            db.commit()
            _record_event(
                db,
                message_id=msg.id,
                provider=msg.provider,
                event_type="sent",
                payload={"channel": msg.channel},
            )
            try:
                record_usage_event(
                    db,
                    user_id=msg.user_id,
                    event_type="outbound_message_sent",
                    source="messaging",
                    channel=msg.channel,
                    provider=msg.provider,
                    metadata={"message_id": msg.id, "to": msg.to_address},
                )
            except Exception:
                pass
        except Exception as exc:
            msg.status = "failed"
            msg.provider_status = "failed"
            msg.error_message = str(exc)
            msg.failed_at = datetime.utcnow()
            db.commit()
            _record_event(
                db,
                message_id=msg.id,
                provider=msg.provider or msg.channel,
                event_type="failed",
                payload={"error": str(exc)},
            )
            try:
                record_usage_event(
                    db,
                    user_id=msg.user_id,
                    event_type="outbound_message_failed",
                    source="messaging",
                    channel=msg.channel,
                    provider=msg.provider,
                    metadata={"message_id": msg.id, "error": str(exc)},
                )
            except Exception:
                pass
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
