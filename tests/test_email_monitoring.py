import uuid

from app.db.database import SessionLocal
from app.db.models import EmailAlert, NotificationQueue
from app.services.email_monitoring import upsert_email_monitor_config, run_email_monitoring, create_test_email_alert
import app.services.email_monitoring as email_monitoring


def test_email_monitoring_keyword_alert(monkeypatch):
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"

    upsert_email_monitor_config(
        db,
        user_id=user_id,
        keywords=["invoice"],
        enabled=True,
        alert_channel="whatsapp",
        window_minutes=60,
        max_results=10,
    )

    def fake_list_recent_emails(*args, **kwargs):
        return [
            {
                "id": "msg_1",
                "from": "billing@example.com",
                "subject": "Invoice due",
                "snippet": "Please pay by Friday",
                "body": "Invoice attached",
                "provider": "google",
            }
        ]

    monkeypatch.setattr(email_monitoring, "list_recent_emails", fake_list_recent_emails)

    result = run_email_monitoring(db, user_id=user_id)
    assert result["alerts"] == 1

    alert = db.query(EmailAlert).filter(EmailAlert.user_id == user_id).first()
    assert alert is not None

    queued = db.query(NotificationQueue).filter(NotificationQueue.user_id == user_id).first()
    assert queued is not None

    db.close()


def test_email_monitoring_test_alert():
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"

    alert = create_test_email_alert(
        db,
        user_id=user_id,
        subject="Test Subject",
        sender="tester@example.com",
        snippet="invoice attached",
        priority=5,
        alert_channel="whatsapp",
    )
    assert alert.id is not None

    queued = db.query(NotificationQueue).filter(NotificationQueue.user_id == user_id).first()
    assert queued is not None

    db.close()
