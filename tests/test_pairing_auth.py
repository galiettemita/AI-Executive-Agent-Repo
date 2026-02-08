from app.db.database import SessionLocal
from app.services.pairing_service import generate_pairing_code, pair_with_code, consume_pairing_code


def test_pairing_code_flow():
    db = SessionLocal()
    user_id = "user_pair_test"

    record = generate_pairing_code(db, user_id, ttl_minutes=30)
    assert record.code

    result = pair_with_code(db, record.code)
    assert result is not None
    assert result["user_id"] == user_id
    assert result["access_token"]

    reused = consume_pairing_code(db, record.code)
    assert reused is None

    db.close()
