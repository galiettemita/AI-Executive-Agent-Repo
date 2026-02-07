from fastapi.testclient import TestClient

from app.main import app


def test_beta_admin_add_list_remove():
    client = TestClient(app)

    resp = client.post("/admin/beta/testers", json={"user_id": "beta_user_1", "email": "beta@example.com"})
    assert resp.status_code == 200
    tester = resp.json()["tester"]
    tester_id = tester["id"]
    assert tester["user_id"] == "beta_user_1"

    resp = client.get("/admin/beta/testers")
    assert resp.status_code == 200
    items = resp.json()["items"]
    assert any(item["user_id"] == "beta_user_1" for item in items)

    resp = client.delete(f"/admin/beta/testers/{tester_id}")
    assert resp.status_code == 200
