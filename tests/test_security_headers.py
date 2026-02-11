from fastapi.testclient import TestClient

from app.main import app


def test_security_headers_present():
    client = TestClient(app)
    resp = client.get("/health/ready")
    assert resp.status_code == 200

    headers = resp.headers
    assert headers.get("x-content-type-options") == "nosniff"
    assert headers.get("x-frame-options") == "DENY"
    assert headers.get("referrer-policy") == "strict-origin-when-cross-origin"
    assert headers.get("permissions-policy") == "geolocation=(self)"


def test_csp_on_html_response(monkeypatch):
    client = TestClient(app)

    # Create a minimal proposal to reach the HTML view
    from app.db.database import SessionLocal
    from app.db.models import Proposal
    from app.api.deps import get_or_create_user
    from app.services.proposal_links import sign_token

    db = SessionLocal()
    try:
        user_id = "csp_user"
        get_or_create_user(db, user_id)
        proposal = Proposal(user_id=user_id, proposal_type="gift_purchase", payload_json="{}")
        db.add(proposal)
        db.commit()
        db.refresh(proposal)

        token = sign_token({"proposal_id": proposal.id})
        resp = client.get(f"/proposals/{proposal.id}?token={token}")
    finally:
        db.close()

    assert resp.status_code == 200
    assert "content-security-policy" in resp.headers
