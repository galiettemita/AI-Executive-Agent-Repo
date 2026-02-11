from fastapi.testclient import TestClient

from app.main import app


def test_entertainment_flow(monkeypatch):
    client = TestClient(app)
    user_id = "ent_user_1"

    resp = client.post(
        "/entertainment/items",
        json={
            "user_id": user_id,
            "title": "Dune",
            "content_type": "movie",
            "status": "planned",
        },
    )
    assert resp.status_code == 200
    item_id = resp.json()["item"]["id"]

    resp = client.get("/entertainment/items", params={"user_id": user_id})
    assert resp.status_code == 200
    assert any(i["id"] == item_id for i in resp.json()["items"])

    resp = client.patch(
        f"/entertainment/items/{item_id}",
        json={"user_id": user_id, "status": "in_progress"},
    )
    assert resp.status_code == 200
    assert resp.json()["item"]["status"] == "in_progress"

    resp = client.post(
        "/entertainment/consumption",
        json={
            "user_id": user_id,
            "item_id": item_id,
            "event_type": "watched",
            "duration_minutes": 120,
        },
    )
    assert resp.status_code == 200

    resp = client.get(
        "/entertainment/consumption",
        params={"user_id": user_id, "item_id": item_id},
    )
    assert resp.status_code == 200
    assert resp.json()["consumption"]

    class Stub:
        def model_dump(self):
            return {
                "title": "Example Show",
                "url": "https://example.com",
                "snippet": "Good show",
                "source": "stub",
                "retailer_domain": "example.com",
            }

    async def fake_discover_search(query: str, max_results: int = 6):
        return [Stub()]

    monkeypatch.setattr("app.services.entertainment_service.discover_search", fake_discover_search)

    resp = client.post(
        "/entertainment/recommendations",
        json={"user_id": user_id, "query": "sci fi shows", "content_type": "tv", "save": True},
    )
    assert resp.status_code == 200
    assert resp.json()["recommendations"]["results"]

    resp = client.post(
        "/entertainment/events",
        json={
            "user_id": user_id,
            "title": "Jazz Night",
            "event_type": "concert",
            "location": "New York",
        },
    )
    assert resp.status_code == 200
    event_id = resp.json()["event"]["id"]

    resp = client.get("/entertainment/events", params={"user_id": user_id})
    assert resp.status_code == 200
    assert any(e["id"] == event_id for e in resp.json()["events"])

    resp = client.patch(
        f"/entertainment/events/{event_id}",
        json={"user_id": user_id, "status": "bookmarked"},
    )
    assert resp.status_code == 200
    assert resp.json()["event"]["status"] == "bookmarked"

    class EventStub:
        def model_dump(self):
            return {
                "title": "Live Show",
                "url": "https://tickets.example.com/show",
                "snippet": "Great event",
                "source": "stub",
                "retailer_domain": "tickets.example.com",
            }

    async def fake_event_search(query: str, max_results: int = 6):
        return [EventStub()]

    monkeypatch.setattr("app.services.entertainment_events_service.discover_search", fake_event_search)

    resp = client.post(
        "/entertainment/events/discover",
        json={"user_id": user_id, "query": "music", "location": "NYC", "save": True},
    )
    assert resp.status_code == 200
    assert resp.json()["events"]["results"]

    resp = client.post(
        f"/entertainment/events/{event_id}/proposal",
        json={
            "user_id": user_id,
            "event_id": event_id,
            "quantity": 2,
            "total_price": 120.0,
            "currency": "USD",
            "require_approval": True,
        },
    )
    assert resp.status_code == 200
    booking_id = resp.json()["booking"]["id"]
    assert resp.json()["proposal"]["approval_url"]

    resp = client.get("/entertainment/events/bookings", params={"user_id": user_id})
    assert resp.status_code == 200
    assert any(b["id"] == booking_id for b in resp.json()["bookings"])

    resp = client.patch(
        f"/entertainment/events/bookings/{booking_id}",
        json={"user_id": user_id, "status": "approved"},
    )
    assert resp.status_code == 200
    assert resp.json()["booking"]["status"] == "approved"
