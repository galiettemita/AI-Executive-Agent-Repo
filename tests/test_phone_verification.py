import uuid

from app.db.database import SessionLocal
from app.db.models import User
from app.services.phone_verification import request_phone_verification, verify_phone_code
from app.services.preferences import get_preferences


def test_phone_verification_flow():
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"

    db.add(User(id=user_id))
    db.commit()

    result = request_phone_verification(db, user_id, "+15551234567", force_code="123456")
    assert result.get("ok") is True

    verify = verify_phone_code(db, user_id, "+15551234567", "123456")
    assert verify.get("status") == "verified"

    prefs = get_preferences(db, user_id)
    assert prefs.get("phone_verified") is True

    db.close()
