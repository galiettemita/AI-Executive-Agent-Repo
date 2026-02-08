from fastapi.testclient import TestClient

from app.main import app


def test_beta_admin_add_list_remove():
    client = TestClient(app)

    baseline = client.get("/admin/beta/summary")
    assert baseline.status_code == 200
    baseline_total = baseline.json()["summary"]["total"]

    bulk_payload = {
        "testers": [
            {"user_id": "beta_bulk_user_1", "email": "bulk1@example.com"},
            {"user_id": "beta_bulk_user_2", "email": "bulk2@example.com"},
        ]
    }
    resp = client.post("/admin/beta/testers/bulk", json=bulk_payload)
    assert resp.status_code == 200
    assert resp.json()["count"] == 2

    summary = client.get("/admin/beta/summary")
    assert summary.status_code == 200
    assert summary.json()["summary"]["total"] >= baseline_total + 2

    resp = client.post("/admin/beta/testers", json={"user_id": "beta_user_1", "email": "beta@example.com"})
    assert resp.status_code == 200
    tester = resp.json()["tester"]
    tester_id = tester["id"]
    assert tester["user_id"] == "beta_user_1"

    resp = client.get("/admin/beta/testers")
    assert resp.status_code == 200
    items = resp.json()["items"]
    assert any(item["user_id"] == "beta_user_1" for item in items)
    assert any(item["user_id"] == "beta_bulk_user_1" for item in items)
    assert any(item["user_id"] == "beta_bulk_user_2" for item in items)

    resp = client.delete(f"/admin/beta/testers/{tester_id}")
    assert resp.status_code == 200
