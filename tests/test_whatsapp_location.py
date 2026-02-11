from fastapi.testclient import TestClient

from app.main import app


def test_whatsapp_location_share(monkeypatch):
    client = TestClient(app)

    def fake_send(*args, **kwargs):
        return "wamid.test"

    monkeypatch.setattr("app.api.routes.webhooks_whatsapp.send_whatsapp_text", fake_send)

    payload = {
        "entry": [
            {
                "changes": [
                    {
                        "value": {
                            "messages": [
                                {
                                    "id": "wamid.location.1",
                                    "from": "15550001111",
                                    "type": "location",
                                    "location": {
                                        "latitude": 37.7749,
                                        "longitude": -122.4194,
                                        "name": "San Francisco",
                                        "address": "San Francisco, CA",
                                    },
                                }
                            ]
                        }
                    }
                ]
            }
        ]
    }

    resp = client.post("/webhooks/whatsapp", json=payload)
    assert resp.status_code == 200
    assert resp.json().get("location_saved") is True

    resp = client.get("/profile", params={"user_id": "+15550001111"})
    assert resp.status_code == 200
    profile = resp.json()["profile"]
    assert profile.get("home_lat") == 37.7749
