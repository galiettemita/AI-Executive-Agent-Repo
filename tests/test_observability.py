from fastapi.testclient import TestClient

from app.main import app


def test_request_id_header_roundtrip():
    client = TestClient(app)

    resp = client.get("/")
    assert resp.status_code == 200
    assert "X-Request-ID" in resp.headers

    custom_id = "test-request-id-123"
    resp2 = client.get("/", headers={"X-Request-ID": custom_id})
    assert resp2.status_code == 200
    assert resp2.headers.get("X-Request-ID") == custom_id
