import json

from fastapi.testclient import TestClient

from app.main import app
from app.db.database import SessionLocal
from app.db.models import Proposal
from app.api.deps import get_or_create_user
from app.services.proposal_links import sign_token


def test_proposal_html_escapes_payload():
    client = TestClient(app)
    db = SessionLocal()
    try:
        user_id = "proposal_user"
        get_or_create_user(db, user_id)
        payload = {"note": "<script>alert('xss')</script>"}
        proposal = Proposal(
            user_id=user_id,
            proposal_type="gift_purchase",
            payload_json=json.dumps(payload),
        )
        db.add(proposal)
        db.commit()
        db.refresh(proposal)

        token = sign_token({"proposal_id": proposal.id})
    finally:
        db.close()

    resp = client.get(f"/proposals/{proposal.id}?token={token}")
    assert resp.status_code == 200
    assert "&lt;script&gt;alert(&#x27;xss&#x27;)&lt;/script&gt;" in resp.text
