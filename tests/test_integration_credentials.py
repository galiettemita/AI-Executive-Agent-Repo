# tests/test_integration_credentials.py

import uuid

from app.db.database import SessionLocal
from app.services.integration_credentials import (
    upsert_integration_credential,
    get_integration_credential,
    get_decrypted_secret,
)


def test_integration_credentials_roundtrip():
    db = SessionLocal()
    user_id = f"test_user_{uuid.uuid4().hex[:8]}"

    upsert_integration_credential(
        db=db,
        user_id=user_id,
        provider="caldav",
        username="user@example.com",
        secret="app-specific-password",
        server_url="https://caldav.example.com",
        metadata={"calendar_name": "Primary"},
    )

    row = get_integration_credential(db, user_id, "caldav")
    assert row is not None
    assert row.secret_enc != "app-specific-password"
    assert get_decrypted_secret(row) == "app-specific-password"

    db.close()
