from datetime import date, timedelta

from fastapi.testclient import TestClient

from app.main import app
from app.db.database import SessionLocal
from app.db.models import NotificationQueue


def test_gift_flow(monkeypatch):
    client = TestClient(app)
    user_id = "gift_user_1"

    occasion_payload = {
        "user_id": user_id,
        "recipient_name": "Alex",
        "relationship": "friend",
        "occasion_type": "birthday",
        "occasion_date": (date.today() + timedelta(days=7)).isoformat(),
    }
    resp = client.post("/gifts/occasions", json=occasion_payload)
    assert resp.status_code == 200
    occasion_id = resp.json()["occasion"]["id"]

    resp = client.get("/gifts/occasions", params={"user_id": user_id})
    assert resp.status_code == 200
    assert any(o["id"] == occasion_id for o in resp.json()["occasions"])

    resp = client.patch(
        f"/gifts/occasions/{occasion_id}",
        json={"user_id": user_id, "notes": "Loves books"},
    )
    assert resp.status_code == 200
    assert resp.json()["occasion"]["notes"] == "Loves books"

    idea_payload = {
        "user_id": user_id,
        "occasion_id": occasion_id,
        "title": "Hardcover novel",
        "price": 25.0,
        "currency": "USD",
    }
    resp = client.post("/gifts/ideas", json=idea_payload)
    assert resp.status_code == 200
    idea_id = resp.json()["idea"]["id"]

    resp = client.patch(
        f"/gifts/ideas/{idea_id}",
        json={"user_id": user_id, "status": "shortlisted"},
    )
    assert resp.status_code == 200
    assert resp.json()["idea"]["status"] == "shortlisted"

    resp = client.post(
        f"/gifts/ideas/{idea_id}/proposal",
        json={"user_id": user_id, "gift_idea_id": idea_id, "quantity": 1},
    )
    assert resp.status_code == 200
    assert resp.json()["proposal"]["approval_url"]

    class Stub:
        def model_dump(self):
            return {
                "title": "Test gift",
                "url": "https://example.com",
                "snippet": "Nice gift",
                "source": "stub",
                "retailer_domain": "example.com",
            }

    async def fake_discover_search(query: str, max_results: int = 6):
        return [Stub()]

    monkeypatch.setattr("app.services.gift_recommendations.discover_search", fake_discover_search)

    resp = client.post(
        "/gifts/recommendations",
        json={"user_id": user_id, "occasion_id": occasion_id},
    )
    assert resp.status_code == 200
    assert resp.json()["recommendations"]["results"]

    resp = client.post(
        "/gifts/thank-you",
        json={"user_id": user_id, "occasion_id": occasion_id, "gift_idea_id": idea_id},
    )
    assert resp.status_code == 200
    assert "Alex" in resp.json()["draft"]["message"]

    resp = client.post("/gifts/reminders/run", params={"user_id": user_id})
    assert resp.status_code == 200
    db = SessionLocal()
    try:
        queued = (
            db.query(NotificationQueue)
            .filter(NotificationQueue.user_id == user_id, NotificationQueue.event_type == "gift_reminder")
            .all()
        )
        assert queued
    finally:
        db.close()
