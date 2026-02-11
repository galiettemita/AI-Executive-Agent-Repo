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

    # Retailer allowlist
    resp = client.post(
        "/gifts/retailers",
        json={"user_id": user_id, "domain": "example.com", "status": "allowed"},
    )
    assert resp.status_code == 200
    retailer_id = resp.json()["retailer"]["id"]

    resp = client.get("/gifts/retailers", params={"user_id": user_id})
    assert resp.status_code == 200
    assert any(r["id"] == retailer_id for r in resp.json()["retailers"])

    # Gift order flow
    resp = client.post(
        "/gifts/orders",
        json={
            "user_id": user_id,
            "gift_idea_id": idea_id,
            "product_url": "https://example.com/gift",
            "retailer_domain": "example.com",
            "quantity": 1,
            "unit_price": 25.0,
            "currency": "USD",
            "require_approval": True,
        },
    )
    assert resp.status_code == 200
    order_id = resp.json()["order"]["id"]
    assert resp.json()["proposal"]["approval_url"]

    resp = client.post(
        f"/gifts/orders/{order_id}/authorize",
        json={"user_id": user_id},
    )
    assert resp.status_code == 200

    resp = client.post(
        f"/gifts/orders/{order_id}/events",
        json={"user_id": user_id, "status": "shipped", "message": "Shipped"},
    )
    assert resp.status_code == 200

    resp = client.post(
        f"/gifts/orders/{order_id}/refund",
        json={"user_id": user_id, "reason": "Changed mind"},
    )
    assert resp.status_code == 200
