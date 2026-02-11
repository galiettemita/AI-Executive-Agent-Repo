from fastapi.testclient import TestClient

from app.main import app


def test_learning_flow(monkeypatch):
    client = TestClient(app)
    user_id = "learn_user_1"

    resp = client.post(
        "/learning/language/goals",
        json={
            "user_id": user_id,
            "language": "Spanish",
            "daily_minutes": 20,
            "weekly_sessions": 4,
            "target_level": "B1",
        },
    )
    assert resp.status_code == 200
    goal_id = resp.json()["goal"]["id"]

    resp = client.get("/learning/language/goals", params={"user_id": user_id})
    assert resp.status_code == 200
    assert any(g["id"] == goal_id for g in resp.json()["goals"])

    resp = client.patch(
        f"/learning/language/goals/{goal_id}",
        json={"user_id": user_id, "daily_minutes": 30},
    )
    assert resp.status_code == 200
    assert resp.json()["goal"]["daily_minutes"] == 30

    resp = client.post(
        "/learning/language/sessions",
        json={
            "user_id": user_id,
            "language": "Spanish",
            "session_type": "conversation",
            "duration_minutes": 25,
        },
    )
    assert resp.status_code == 200

    resp = client.get("/learning/language/sessions", params={"user_id": user_id, "language": "Spanish"})
    assert resp.status_code == 200
    assert resp.json()["sessions"]

    resp = client.get("/learning/language/progress", params={"user_id": user_id, "language": "Spanish"})
    assert resp.status_code == 200
    assert resp.json()["progress"]["total_sessions"] >= 1

    resp = client.post(
        "/learning/resources",
        json={
            "user_id": user_id,
            "topic": "Spanish grammar",
            "title": "Grammar basics",
            "url": "https://example.com/grammar",
        },
    )
    assert resp.status_code == 200
    resource_id = resp.json()["resource"]["id"]

    resp = client.get("/learning/resources", params={"user_id": user_id})
    assert resp.status_code == 200
    assert any(r["id"] == resource_id for r in resp.json()["resources"])

    resp = client.patch(
        f"/learning/resources/{resource_id}",
        json={"user_id": user_id, "status": "in_progress"},
    )
    assert resp.status_code == 200
    assert resp.json()["resource"]["status"] == "in_progress"

    class ResourceStub:
        def model_dump(self):
            return {
                "title": "Spanish verbs guide",
                "url": "https://example.com/verbs",
                "snippet": "Verb practice",
                "source": "stub",
            }

    async def fake_resource_search(query: str, max_results: int = 6):
        return [ResourceStub()]

    monkeypatch.setattr("app.services.learning_service.discover_search", fake_resource_search)

    resp = client.post(
        "/learning/resources/recommendations",
        json={"user_id": user_id, "query": "verbs", "topic": "Spanish", "save": True},
    )
    assert resp.status_code == 200
    assert resp.json()["recommendations"]["results"]

    resp = client.post(
        "/learning/schedule",
        json={
            "user_id": user_id,
            "resource_id": resource_id,
            "duration_minutes": 30,
        },
    )
    assert resp.status_code == 200
    schedule_id = resp.json()["schedule"]["id"]

    resp = client.get("/learning/schedule", params={"user_id": user_id})
    assert resp.status_code == 200
    assert any(s["id"] == schedule_id for s in resp.json()["schedule"])

    resp = client.patch(
        f"/learning/schedule/{schedule_id}",
        json={"user_id": user_id, "status": "completed"},
    )
    assert resp.status_code == 200
    assert resp.json()["schedule"]["status"] == "completed"

    resp = client.delete(
        f"/learning/schedule/{schedule_id}",
        params={"user_id": user_id},
    )
    assert resp.status_code == 200
