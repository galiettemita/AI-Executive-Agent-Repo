from fastapi.testclient import TestClient

from app.main import app


def test_entitlements_gate_blocks_premium_for_free():
    client = TestClient(app)
    user_id = "test_user"

    # Complete onboarding flow
    client.post("/agent/chat", json={"user_id": user_id, "message": "hi"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "minimal"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "$50-$150"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "weekdays after 6pm"})

    # Premium intent should be gated for free users
    resp = client.post(
        "/agent/chat",
        json={"user_id": user_id, "message": "Please check my calendar"},
    )
    assert resp.status_code == 200
    data = resp.json()
    assert "billing/stripe/checkout" in data.get("reply", "")
