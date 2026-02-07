import uuid

from fastapi.testclient import TestClient

from app.core import config as app_config
from app.db.database import SessionLocal
from app.db.models import User, OutboundMessage
from app.main import app


def test_sms_status_webhook_updates_message():
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"
    db.add(User(id=user_id))
    db.commit()

    msg = OutboundMessage(
        user_id=user_id,
        channel="sms",
        to_address="+15551230000",
        body="Hello",
        status="sent",
        provider="twilio",
        provider_message_id="SM999",
    )
    db.add(msg)
    db.commit()
    db.close()

    app_config.settings.ENFORCE_WEBHOOK_SIGNATURES = "0"

    client = TestClient(app)
    resp = client.post(
        "/webhooks/sms/status",
        data={"MessageSid": "SM999", "MessageStatus": "delivered"},
    )
    assert resp.status_code == 200

    db = SessionLocal()
    refreshed = (
        db.query(OutboundMessage)
        .filter(OutboundMessage.provider_message_id == "SM999")
        .first()
    )
    assert refreshed.status == "delivered"
    db.close()
