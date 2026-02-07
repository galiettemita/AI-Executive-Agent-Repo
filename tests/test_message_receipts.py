import uuid

from app.db.database import SessionLocal
from app.db.models import User, OutboundMessage, OutboundMessageEvent
from app.services.messaging_service import record_delivery_status


def test_record_delivery_status_updates_message():
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"

    db.add(User(id=user_id))
    db.commit()

    msg = OutboundMessage(
        user_id=user_id,
        channel="sms",
        to_address="+15551234567",
        body="Hello",
        status="sent",
        provider="twilio",
        provider_message_id="SM123",
    )
    db.add(msg)
    db.commit()
    db.refresh(msg)

    record_delivery_status(
        db,
        provider="twilio",
        provider_message_id="SM123",
        provider_status="delivered",
        payload={"MessageStatus": "delivered"},
    )

    refreshed = db.query(OutboundMessage).filter(OutboundMessage.id == msg.id).first()
    assert refreshed.status == "delivered"
    assert refreshed.delivered_at is not None

    events = db.query(OutboundMessageEvent).filter(OutboundMessageEvent.message_id == msg.id).all()
    assert events

    db.close()
