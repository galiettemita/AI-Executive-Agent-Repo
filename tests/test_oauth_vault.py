from __future__ import annotations

import uuid

from app.db.database import SessionLocal
from app.db.models import OAuthToken, User
from app.services.oauth_vault import store_provider_tokens
from app.services.token_crypto import decrypt_token


def test_oauth_vault_upserts_legacy_oauth_tokens_table() -> None:
    user_id = f"vault-user-{uuid.uuid4()}"
    db = SessionLocal()
    try:
        db.add(User(id=user_id))
        db.commit()

        store_provider_tokens(
            db,
            user_id=user_id,
            provider="google",
            access_token="access-1",
            refresh_token="refresh-1",
            scopes=["openid", "email"],
            metadata={"source": "test"},
            email="vault@example.com",
        )
        first = (
            db.query(OAuthToken)
            .filter(OAuthToken.user_id == user_id, OAuthToken.provider == "google")
            .one_or_none()
        )
        assert first is not None
        assert first.access_token == "access-1"
        assert decrypt_token(first.refresh_token_enc) == "refresh-1"

        store_provider_tokens(
            db,
            user_id=user_id,
            provider="google",
            access_token="access-2",
            refresh_token="refresh-2",
            scopes=["openid"],
        )
        second = (
            db.query(OAuthToken)
            .filter(OAuthToken.user_id == user_id, OAuthToken.provider == "google")
            .one_or_none()
        )
        assert second is not None
        assert second.id == first.id
        assert second.access_token == "access-2"
        assert decrypt_token(second.refresh_token_enc) == "refresh-2"
    finally:
        db.close()
