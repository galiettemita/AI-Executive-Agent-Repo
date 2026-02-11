from fastapi.testclient import TestClient

from app.main import app


def test_public_site_and_location_capture(monkeypatch):
    client = TestClient(app)
    user_id = "+15551230000"

    resp = client.get("/")
    assert resp.status_code == 200
    assert "Executive AI Agent" in resp.text

    resp = client.post(
        "/public/location",
        json={
            "user_id": user_id,
            "consent": True,
            "source": "manual",
            "city": "Austin",
            "location": "Austin, TX",
        },
    )
    assert resp.status_code == 200

    async def fake_ip_lookup(ip: str):
        return {
            "ip": ip,
            "city": "New York",
            "region": "NY",
            "country": "United States",
            "latitude": 40.71,
            "longitude": -74.0,
            "timezone": "America/New_York",
        }

    monkeypatch.setattr("app.api.routes.public_site.resolve_ip_location", fake_ip_lookup)

    resp = client.post(
        "/public/location/ip",
        json={"user_id": user_id, "consent": True},
    )
    assert resp.status_code == 200

    resp = client.get("/profile", params={"user_id": user_id})
    assert resp.status_code == 200
    profile = resp.json()["profile"]
    assert profile.get("home_city") in {"Austin", "New York"}
