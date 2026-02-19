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
from app.services.provisioning_pipeline import ProvisioningPipeline, record_declined
from app.blueprint.contracts import ProvisioningState
from app.services.provisioning_sessions import create_provisioning_session, get_provisioning_session
from app.services.account_deletion_pipeline import run_due_account_deletion_jobs


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


def test_v1_auth_me_delete_cleans_provisioning_artifacts() -> None:
    user_id = "delete-prov-user"
    db = SessionLocal()
    try:
        db.add(User(id=user_id))
        db.commit()
        pipeline = ProvisioningPipeline(db)
        req = pipeline.begin(
            user_id=user_id,
            server_id="duffel-mcp",
            reason="Need flights",
        )
        _ = pipeline.transition(
            request_id=req.id,
            new_state=ProvisioningState.AWAITING_AUTH,
            note="awaiting_auth",
        )
        _ = pipeline.transition(
            request_id=req.id,
            new_state=ProvisioningState.CANCELED,
            note="cancel_for_test",
        )
        _ = record_declined(db, user_id=user_id, server_id="duffel-mcp", reason="not_now")
        token = create_provisioning_session(
            {"user_id": user_id, "server_id": "duffel-mcp", "request_id": req.id},
            ttl_seconds=600,
        )
        assert get_provisioning_session(token) is not None
    finally:
        db.close()

    client = TestClient(app)
    resp = client.delete("/api/v1/auth/me", params={"user_id": user_id})
    assert resp.status_code == 200
    assert resp.json()["ok"] is True

    db2 = SessionLocal()
    try:
        db2.execute(
            text("update account_deletion_jobs set due_at = :now where user_id = :user_id"),
            {"now": datetime.utcnow().isoformat(sep=" "), "user_id": user_id},
        )
        db2.commit()
        _ = run_due_account_deletion_jobs(db2, limit=50)

        req_count = db2.execute(
            text("select count(1) from provisioning_requests where user_id = :user_id"),
            {"user_id": user_id},
        ).scalar()
        declined_count = db2.execute(
            text("select count(1) from provisioning_declined where user_id = :user_id"),
            {"user_id": user_id},
        ).scalar()
        assert int(req_count or 0) == 0
        assert int(declined_count or 0) == 0
    finally:
        db2.close()

    assert get_provisioning_session(token) is None
