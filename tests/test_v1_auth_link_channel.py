from __future__ import annotations

from fastapi.testclient import TestClient

from app.main import app


def test_v1_auth_link_channel_happy_path_and_conflict():
    client = TestClient(app)
    phone = "+15555550101"

    start = client.post(
        "/api/v1/auth/link-channel",
        json={"user_id": "link-user-1", "channel": "imessage", "channel_identifier": phone},
    )
    assert start.status_code == 200
    body = start.json()
    assert body["ok"] is True
    assert body["status"] in {"sent", "cooldown"}
    code = body.get("code")
    assert isinstance(code, str) and code

    verify = client.post(
        "/api/v1/auth/link-channel",
        json={
            "user_id": "link-user-1",
            "channel": "imessage",
            "channel_identifier": phone,
            "code": code,
        },
    )
    assert verify.status_code == 200
    assert verify.json()["ok"] is True

    # Conflict: linking same identifier to a different user must be rejected.
    start2 = client.post(
        "/api/v1/auth/link-channel",
        json={"user_id": "link-user-2", "channel": "imessage", "channel_identifier": phone},
    )
    assert start2.status_code == 200
    code2 = start2.json().get("code")
    assert isinstance(code2, str) and code2

    verify2 = client.post(
        "/api/v1/auth/link-channel",
        json={
            "user_id": "link-user-2",
            "channel": "imessage",
            "channel_identifier": phone,
            "code": code2,
        },
    )
    assert verify2.status_code == 409

