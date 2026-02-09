from datetime import datetime, timedelta

from fastapi.testclient import TestClient

from app.main import app


def test_fitness_workout_and_nutrition_flow():
    client = TestClient(app)
    user_id = "fitness_user_1"

    resp = client.post(
        "/fitness/workouts",
        json={
            "user_id": user_id,
            "workout_type": "strength",
            "duration_minutes": 45,
            "intensity": "moderate",
        },
    )
    assert resp.status_code == 200
    workout_id = resp.json()["workout"]["id"]

    resp = client.get("/fitness/workouts", params={"user_id": user_id})
    assert resp.status_code == 200
    assert any(w["id"] == workout_id for w in resp.json()["workouts"])

    resp = client.patch(
        f"/fitness/workouts/{workout_id}",
        json={"user_id": user_id, "notes": "Leg day"},
    )
    assert resp.status_code == 200
    assert resp.json()["workout"]["notes"] == "Leg day"

    resp = client.post(
        "/fitness/nutrition/logs",
        json={
            "user_id": user_id,
            "meal_type": "lunch",
            "calories": 650,
            "protein_g": 45,
        },
    )
    assert resp.status_code == 200
    log_id = resp.json()["log"]["id"]

    resp = client.get("/fitness/nutrition/logs", params={"user_id": user_id})
    assert resp.status_code == 200
    assert any(l["id"] == log_id for l in resp.json()["logs"])

    plan_date = (datetime.utcnow() + timedelta(days=1)).isoformat()
    resp = client.post(
        "/fitness/meal-plans",
        json={
            "user_id": user_id,
            "plan_date": plan_date,
            "meals": [{"meal": "breakfast", "target_calories": 400}],
            "calorie_target": 2000,
        },
    )
    assert resp.status_code == 200
    plan_id = resp.json()["meal_plan"]["id"]

    resp = client.get("/fitness/meal-plans", params={"user_id": user_id})
    assert resp.status_code == 200
    assert any(p["id"] == plan_id for p in resp.json()["meal_plans"])

    resp = client.get("/fitness/suggestions/workouts", params={"user_id": user_id})
    assert resp.status_code == 200
    assert resp.json()["suggestions"]["recommendations"]

    resp = client.get("/fitness/suggestions/meals", params={"user_id": user_id})
    assert resp.status_code == 200
    assert resp.json()["suggestions"]["target_calories"]

    resp = client.delete(f"/fitness/workouts/{workout_id}", params={"user_id": user_id})
    assert resp.status_code == 200
