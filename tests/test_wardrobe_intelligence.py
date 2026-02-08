from datetime import datetime, timezone

from fastapi.testclient import TestClient

from app.main import app


def test_outfit_suggestions_with_context(monkeypatch):
    client = TestClient(app)
    user_id = "wardrobe_user_suggest"

    resp = client.post(
        "/wardrobe/items",
        json={"user_id": user_id, "name": "Navy Blazer", "category": "outerwear", "color": "navy"},
    )
    assert resp.status_code == 200

    def fake_weather(*args, **kwargs):
        return {
            "provider": "test",
            "location": {"name": "Test City"},
            "date": "2026-02-08",
            "temp_c_min": 4,
            "temp_c_max": 9,
            "temp_f_min": 39.2,
            "temp_f_max": 48.2,
            "precip_mm": 0,
        }

    def fake_events(*args, **kwargs):
        return [
            {
                "summary": "Client presentation",
                "start": {"dateTime": datetime(2026, 2, 8, 14, 0, tzinfo=timezone.utc).isoformat()},
            }
        ]

    monkeypatch.setattr("app.services.wardrobe_intelligence.get_daily_weather", fake_weather)
    monkeypatch.setattr("app.services.wardrobe_intelligence.list_events_in_range", fake_events)

    resp = client.get(
        "/wardrobe/suggestions",
        params={"user_id": user_id, "date": "2026-02-08", "location": "New York"},
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["ok"] is True
    assert body["suggestions"]


def test_wardrobe_rotation_and_wear_tracking(monkeypatch):
    client = TestClient(app)
    user_id = "wardrobe_user_rotation"

    resp = client.post(
        "/wardrobe/items",
        json={"user_id": user_id, "name": "White Tee", "category": "tops"},
    )
    item_id = resp.json()["item"]["id"]

    resp = client.post(
        "/wardrobe/items",
        json={"user_id": user_id, "name": "Black Jeans", "category": "bottoms"},
    )
    item2_id = resp.json()["item"]["id"]

    resp = client.post(f"/wardrobe/items/{item_id}/wear", json={"user_id": user_id})
    assert resp.status_code == 200
    assert resp.json()["item"]["wear_count"] == 1

    resp = client.get(
        "/wardrobe/rotation",
        params={"user_id": user_id, "min_days_since_worn": 1, "limit": 5},
    )
    assert resp.status_code == 200
    rotation_items = resp.json()["rotation"]["items"]
    assert any(item["id"] == item2_id for item in rotation_items)
    assert all(item["id"] != item_id for item in rotation_items)


def test_shopping_recommendations(monkeypatch):
    client = TestClient(app)
    user_id = "wardrobe_user_shop"

    client.post(
        "/wardrobe/items",
        json={"user_id": user_id, "name": "Running Shorts", "category": "activewear"},
    )

    async def fake_discover_search(query: str, max_results: int = 6):
        return [
            type(
                "StubResult",
                (),
                {
                    "title": "Test Item",
                    "url": "https://example.com/item",
                    "snippet": "Great item",
                    "source": "stub",
                    "retailer_domain": "example.com",
                    "model_dump": lambda self: {
                        "title": "Test Item",
                        "url": "https://example.com/item",
                        "snippet": "Great item",
                        "source": "stub",
                        "retailer_domain": "example.com",
                    },
                },
            )()
        ]

    def fake_weather(*args, **kwargs):
        return {
            "provider": "test",
            "location": {"name": "Test City"},
            "date": "2026-02-08",
            "temp_c_min": 2,
            "temp_c_max": 6,
            "temp_f_min": 35.6,
            "temp_f_max": 42.8,
            "precip_mm": 3,
        }

    monkeypatch.setattr("app.services.wardrobe_intelligence.discover_search", fake_discover_search)
    monkeypatch.setattr("app.services.wardrobe_intelligence.get_daily_weather", fake_weather)

    resp = client.get(
        "/wardrobe/recommendations",
        params={"user_id": user_id, "date": "2026-02-08", "location": "New York"},
    )
    assert resp.status_code == 200
    data = resp.json()
    assert data["ok"] is True
    assert data["recommendations"]["results"]
