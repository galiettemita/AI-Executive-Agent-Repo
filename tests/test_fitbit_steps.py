from datetime import datetime, timedelta

from fastapi.testclient import TestClient

from app.main import app
from app.db.database import SessionLocal
from app.db.models import User, OAuthToken


def test_fitbit_steps_fetch(monkeypatch):
    client = TestClient(app)
    user_id = "fitbit_user_1"

    db = SessionLocal()
    try:
        db.add(User(id=user_id))
        token = OAuthToken(
            user_id=user_id,
            provider="fitbit",
            access_token="token",
            refresh_token_enc="",
            expiry_utc=datetime.utcnow() + timedelta(days=1),
            scopes="activity",
        )
        db.add(token)
        db.commit()
    finally:
        db.close()

    def fake_fetch(access_token: str, step_date):
        assert access_token == "token"
        return 12345

    monkeypatch.setattr("app.services.fitbit_steps.fetch_fitbit_steps", fake_fetch)

    resp = client.get("/fitness/steps", params={"user_id": user_id, "refresh": True})
    assert resp.status_code == 200
    assert resp.json()["steps"] == 12345

    resp_cached = client.get("/fitness/steps", params={"user_id": user_id})
    assert resp_cached.status_code == 200
    assert resp_cached.json()["cached"] is True
