from __future__ import annotations

import io
import json
import zipfile
from datetime import datetime

from fastapi.testclient import TestClient
from sqlalchemy import text

from app.db.database import SessionLocal
from app.db.models import OAuthToken, User
from app.main import app


def test_v1_auth_me_delete_starts_pipeline_and_revokes_tokens() -> None:
    user_id = "delete-me-user"
    db = SessionLocal()
    try:
        db.add(User(id=user_id))
        db.add(
            OAuthToken(
                user_id=user_id,
                provider="google",
                access_token="access",
                refresh_token_enc="refresh",
                created_at=datetime.utcnow(),
                updated_at=datetime.utcnow(),
            )
        )
        db.commit()
    finally:
        db.close()

    client = TestClient(app)
    resp = client.delete("/api/v1/auth/me", params={"user_id": user_id})
    assert resp.status_code == 200
    body = resp.json()
    assert body["ok"] is True
    assert body["user_id"] == user_id
    assert set(body["stages"]) == {"purge_24h", "providers_7d", "verify_30d"}

    db2 = SessionLocal()
    try:
        row = db2.get(User, user_id)
        assert row is not None
        assert row.deletion_requested_at is not None

        token = (
            db2.query(OAuthToken)
            .filter(OAuthToken.user_id == user_id, OAuthToken.provider == "google")
            .one_or_none()
        )
        assert token is None

        jobs = db2.execute(
            text("select stage from account_deletion_jobs where user_id = :user_id"),
            {"user_id": user_id},
        ).mappings().all()
        assert {str(j.get("stage") or "") for j in jobs} == {"purge_24h", "providers_7d", "verify_30d"}
    finally:
        db2.close()


def test_v1_auth_me_export_returns_zip() -> None:
    user_id = "export-me-user"
    db = SessionLocal()
    try:
        db.add(User(id=user_id))
        db.commit()
    finally:
        db.close()

    client = TestClient(app)
    resp = client.get("/api/v1/auth/me/export", params={"user_id": user_id})
    assert resp.status_code == 200
    assert resp.headers.get("content-type", "").startswith("application/zip")

    with zipfile.ZipFile(io.BytesIO(resp.content), "r") as zf:
        names = set(zf.namelist())
        assert "manifest.json" in names
        assert "export.json" in names
        payload = json.loads(zf.read("export.json").decode("utf-8"))
        assert payload["user_id"] == user_id
