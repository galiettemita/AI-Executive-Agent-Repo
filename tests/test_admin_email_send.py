import uuid

from app.db.database import SessionLocal
from app.db.models import User
from app.services.email_draft_service import create_email_draft
import app.services.admin_handler as admin_handler


def test_admin_send_confirmation_sends_pending(monkeypatch):
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"
    db.add(User(id=user_id))
    db.commit()

    draft = create_email_draft(
        db,
        user_id=user_id,
        to_email="recipient@example.com",
        subject="Test",
        body_text="Hello there",
    )

    def fake_send_email_draft(db_session, draft_row):
        draft_row.status = "sent"
        db_session.commit()
        return draft_row

    monkeypatch.setattr(admin_handler, "send_email_draft", fake_send_email_draft)

    reply = admin_handler.handle_admin(
        db=db,
        user_id=user_id,
        history=[],
        user_message="send",
    )

    assert "Email sent" in reply
    db.close()
