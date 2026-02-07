import uuid

from app.db.database import SessionLocal
from app.db.models import User, NotificationQueue
from app.services.proactive_rules import create_rule, run_rule


def test_proactive_notify_rule_runs():
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"

    db.add(User(id=user_id))
    db.commit()

    rule = create_rule(
        db,
        user_id=user_id,
        name="Morning nudge",
        trigger_type="interval",
        trigger_config={"interval_minutes": 1},
        action_type="notify",
        action_payload={"title": "Check in", "message": "Time for your daily check-in"},
        conditions={},
        is_active=True,
    )

    result = run_rule(db, rule, force=True)
    assert result.get("status") == "ok"
    assert result.get("action_status") == "notified"

    notif = (
        db.query(NotificationQueue)
        .filter(NotificationQueue.user_id == user_id)
        .order_by(NotificationQueue.created_at.desc())
        .first()
    )
    assert notif is not None
    assert notif.title == "Check in"

    db.close()
