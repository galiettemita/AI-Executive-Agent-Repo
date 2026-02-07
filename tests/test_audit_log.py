from fastapi.testclient import TestClient

from app.main import app
from app.db.database import SessionLocal
from app.db.models import AuditLog


def test_audit_log_written():
    client = TestClient(app)
    resp = client.post("/contacts", json={"user_id": "audit_user", "name": "Bob", "phone": "+15551234567"})
    assert resp.status_code in (200, 201)

    db = SessionLocal()
    row = db.query(AuditLog).filter(AuditLog.path == "/contacts").order_by(AuditLog.id.desc()).first()
    assert row is not None
    db.close()
