import uuid

from app.db.database import SessionLocal
from app.db.models import User
from app.services.email_draft_service import create_email_draft, get_latest_pending_draft, cancel_pending_draft


def test_email_draft_lifecycle():
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"
    db.add(User(id=user_id))
    db.commit()

    draft = create_email_draft(
        db,
        user_id=user_id,
        to_email="test@example.com",
        subject="Hello",
        body_text="Draft body",
    )
    assert draft.status == "pending"

    latest = get_latest_pending_draft(db, user_id)
    assert latest is not None
    assert latest.id == draft.id

    canceled = cancel_pending_draft(db, latest)
    assert canceled.status == "canceled"

    db.close()
