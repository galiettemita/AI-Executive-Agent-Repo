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
