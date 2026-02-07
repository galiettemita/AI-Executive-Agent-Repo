import uuid

from app.db.database import SessionLocal
from app.db.models import User, OutboundMessage
from app.services.messaging_service import queue_outbound_message, deliver_pending_messages
from app.core import config as app_config


def test_outbound_messages_queue_without_send():
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"

    db.add(User(id=user_id))
    db.commit()

    msg = queue_outbound_message(
        db,
        user_id=user_id,
        channel="whatsapp",
        to_address="+15551234567",
        body="Hello",
    )

    app_config.settings.ENABLE_MESSAGING = "0"
    result = deliver_pending_messages(db)
    assert result.get("sent") == 0

    refreshed = db.query(OutboundMessage).filter(OutboundMessage.id == msg.id).first()
    assert refreshed.status == "queued"

    db.close()
