from datetime import datetime, timedelta

from fastapi.testclient import TestClient

from app.main import app
from app.db.database import SessionLocal
from app.db.models import NotificationQueue


def test_relationship_flow():
    client = TestClient(app)
    user_id = "rel_user_1"

    resp = client.post(
        "/contacts",
        json={"user_id": user_id, "name": "Sam", "phone": "+15551234567"},
    )
    assert resp.status_code == 200
    contact_id = resp.json()["contact"]["id"]

    resp = client.post(
        "/relationships/profiles",
        json={
            "user_id": user_id,
            "contact_id": contact_id,
            "relationship": "friend",
            "cadence_days": 7,
        },
    )
    assert resp.status_code == 200
    profile_id = resp.json()["profile"]["id"]
    assert profile_id

    past = (datetime.utcnow() - timedelta(days=10)).isoformat()
    resp = client.post(
        "/relationships/interactions",
        json={
            "user_id": user_id,
            "contact_id": contact_id,
            "direction": "outbound",
            "channel": "whatsapp",
            "occurred_at": past,
        },
    )
    assert resp.status_code == 200

    resp = client.get(
        "/relationships/suggestions",
        params={"user_id": user_id, "limit": 5, "due_only": True},
    )
    assert resp.status_code == 200
    suggestions = resp.json()["suggestions"]
    assert any(s["profile"]["contact_id"] == contact_id for s in suggestions)

    resp = client.post("/relationships/reminders/run", params={"user_id": user_id})
    assert resp.status_code == 200
    assert resp.json().get("queued", 0) >= 1

    db = SessionLocal()
    try:
        queued = (
            db.query(NotificationQueue)
            .filter(
                NotificationQueue.user_id == user_id,
                NotificationQueue.event_type == "relationship_checkin",
            )
            .all()
        )
        assert queued
    finally:
        db.close()
