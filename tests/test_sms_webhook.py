import uuid

from fastapi.testclient import TestClient

from app.core import config as app_config
from app.db.database import SessionLocal
from app.db.models import ChatMessage, InboundEvent, OutboundMessage, User
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


def test_sms_inbound_webhook_creates_conversation_and_outbound_reply(monkeypatch):
    app_config.settings.ENFORCE_WEBHOOK_SIGNATURES = "0"

    monkeypatch.setattr(
        "app.api.routes.webhooks_sms.run_orchestrator",
        lambda db, user_id, history, user_message: "SMS reply: received.",
    )
    monkeypatch.setattr(
        "app.api.routes.webhooks_sms.deliver_pending_messages",
        lambda db, limit=1: {"sent": 0, "failed": 0, "skipped": 1},
    )

    from_phone = f"+1555{uuid.uuid4().int % 10_000_000:07d}"
    message_sid = f"SMIN{uuid.uuid4().hex[:12]}"

    client = TestClient(app)
    resp = client.post(
        "/webhooks/sms",
        data={
            "MessageSid": message_sid,
            "From": from_phone,
            "To": "+15551230000",
            "Body": "hello from sms",
            "NumMedia": "0",
            "SmsStatus": "received",
        },
    )
    assert resp.status_code == 200
    assert "<Response>" in resp.text

    db = SessionLocal()
    assert db.get(User, from_phone) is not None

    ev = db.query(InboundEvent).filter(InboundEvent.external_id == message_sid).first()
    assert ev is not None
    assert ev.channel == "sms"
    assert ev.user_id == from_phone

    msgs = (
        db.query(ChatMessage)
        .filter(ChatMessage.user_id == from_phone)
        .order_by(ChatMessage.id.asc())
        .all()
    )
    assert len(msgs) >= 2
    assert msgs[-2].role == "user"
    assert msgs[-2].content == "hello from sms"
    assert msgs[-1].role == "assistant"
    assert msgs[-1].content == "SMS reply: received."

    outbound = (
        db.query(OutboundMessage)
        .filter(OutboundMessage.user_id == from_phone, OutboundMessage.channel == "sms")
        .order_by(OutboundMessage.id.desc())
        .first()
    )
    assert outbound is not None
    assert outbound.to_address == from_phone
    assert outbound.body == "SMS reply: received."
    db.close()


def test_sms_inbound_webhook_dedupes_by_message_sid(monkeypatch):
    app_config.settings.ENFORCE_WEBHOOK_SIGNATURES = "0"
    call_count = {"n": 0}

    def _fake_orchestrator(db, user_id, history, user_message):
        call_count["n"] += 1
        return "once only"

    monkeypatch.setattr("app.api.routes.webhooks_sms.run_orchestrator", _fake_orchestrator)
    monkeypatch.setattr(
        "app.api.routes.webhooks_sms.deliver_pending_messages",
        lambda db, limit=1: {"sent": 0, "failed": 0, "skipped": 1},
    )

    from_phone = f"+1555{uuid.uuid4().int % 10_000_000:07d}"
    message_sid = f"SMDUPE{uuid.uuid4().hex[:10]}"

    payload = {
        "MessageSid": message_sid,
        "From": from_phone,
        "To": "+15551230000",
        "Body": "dedupe me",
        "NumMedia": "0",
        "SmsStatus": "received",
    }

    client = TestClient(app)
    first = client.post("/webhooks/sms", data=payload)
    second = client.post("/webhooks/sms", data=payload)

    assert first.status_code == 200
    assert second.status_code == 200
    assert call_count["n"] == 1

    db = SessionLocal()
    inbound_count = (
        db.query(InboundEvent)
        .filter(InboundEvent.external_id == message_sid, InboundEvent.channel == "sms")
        .count()
    )
    outbound_count = (
        db.query(OutboundMessage)
        .filter(OutboundMessage.user_id == from_phone, OutboundMessage.channel == "sms")
        .count()
    )
    assert inbound_count == 1
    assert outbound_count == 1
    db.close()
