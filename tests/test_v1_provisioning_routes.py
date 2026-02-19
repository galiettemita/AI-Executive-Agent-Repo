from __future__ import annotations

from fastapi.testclient import TestClient

from app.blueprint.contracts import ProvisioningState
from app.db.database import SessionLocal
from app.db.models import Subscription
from app.main import app
from app.services.provisioning_pipeline import ProvisioningPipeline, get_request
from app.services.provisioning_sessions import create_provisioning_session
from app.services.url_shortener import shorten_url


def test_provision_callback_redirects_to_expired_for_invalid_state():
    client = TestClient(app)
    resp = client.get("/api/v1/provision/callback?state=missing-token&code=abc", follow_redirects=False)
    assert resp.status_code in (302, 307)
    assert "/api/v1/provision/expired" in str(resp.headers.get("location") or "")


def test_short_url_redirect_round_trip():
    short = shorten_url("https://example.com/provision", ttl_seconds=300)
    token = str(short.get("token") or "")
    assert token

    client = TestClient(app)
    resp = client.get(f"/api/v1/provision/short/{token}", follow_redirects=False)
    assert resp.status_code in (302, 307)
    assert resp.headers.get("location") == "https://example.com/provision"


def test_provision_callback_transitions_request_to_active(monkeypatch):
    async def _fake_activate_server(db, *, user_id: str, server_id: str):  # pragma: no cover - executed via route
        return {"ok": True, "server_id": server_id, "connected": True}

    monkeypatch.setattr("app.api.routes.v1_provisioning.activate_server", _fake_activate_server)
    monkeypatch.setattr(
        "app.api.routes.v1_provisioning.retry_original_task_after_provisioning",
        lambda event: {"ok": True, "reply": "Server connected! ✓ done"},
    )

    db = SessionLocal()
    try:
        pipeline = ProvisioningPipeline(db)
        request_row = pipeline.begin(
            user_id="provision-callback-user",
            server_id="duffel-mcp",
            reason="Need flights",
        )
        request_row = pipeline.transition(
            request_id=request_row.id,
            new_state=ProvisioningState.AWAITING_AUTH,
            note="auth_link_sent",
        )
        state = create_provisioning_session(
            {
                "request_id": request_row.id,
                "user_id": "provision-callback-user",
                "server_id": "duffel-mcp",
                "original_task_id": "run-1",
            },
            ttl_seconds=300,
        )
    finally:
        db.close()

    client = TestClient(app)
    resp = client.get(f"/api/v1/provision/callback?state={state}&code=oauth-code", follow_redirects=False)
    assert resp.status_code in (302, 307)
    assert "/api/v1/provision/success" in str(resp.headers.get("location") or "")
    assert "retried=1" in str(resp.headers.get("location") or "")

    db = SessionLocal()
    try:
        final = get_request(db, request_id=request_row.id)
        assert final is not None
        assert final.state == ProvisioningState.ACTIVE
    finally:
        db.close()


def test_provision_callback_fails_when_security_validation_fails(monkeypatch):
    async def _fake_activate_server(db, *, user_id: str, server_id: str):  # pragma: no cover - route should fail before activate
        return {"ok": True, "server_id": server_id, "connected": True}

    monkeypatch.setattr("app.api.routes.v1_provisioning.activate_server", _fake_activate_server)
    monkeypatch.setattr(
        "app.api.routes.v1_provisioning.validate_catalog_security_for_server",
        lambda db, server_id: (_ for _ in ()).throw(ValueError("catalog_signature_invalid")),
    )

    db = SessionLocal()
    try:
        pipeline = ProvisioningPipeline(db)
        request_row = pipeline.begin(
            user_id="provision-security-user",
            server_id="duffel-mcp",
            reason="Need flights",
        )
        request_row = pipeline.transition(
            request_id=request_row.id,
            new_state=ProvisioningState.AWAITING_AUTH,
            note="auth_link_sent",
        )
        state = create_provisioning_session(
            {
                "request_id": request_row.id,
                "user_id": "provision-security-user",
                "server_id": "duffel-mcp",
                "original_task_id": "run-security",
            },
            ttl_seconds=300,
        )
    finally:
        db.close()

    client = TestClient(app)
    resp = client.get(f"/api/v1/provision/callback?state={state}&code=oauth-code", follow_redirects=False)
    assert resp.status_code in (302, 307)
    assert "reason=security_validation_failed" in str(resp.headers.get("location") or "")

    db2 = SessionLocal()
    try:
        final = get_request(db2, request_id=request_row.id)
        assert final is not None
        assert final.state == ProvisioningState.FAILED
    finally:
        db2.close()


def test_provision_callback_remains_valid_after_channel_switch(monkeypatch):
    async def _fake_activate_server(db, *, user_id: str, server_id: str):  # pragma: no cover - executed via route
        return {"ok": True, "server_id": server_id, "connected": True}

    monkeypatch.setattr("app.api.routes.v1_provisioning.activate_server", _fake_activate_server)
    monkeypatch.setattr(
        "app.api.routes.v1_provisioning.retry_original_task_after_provisioning",
        lambda event: {"ok": False, "reason": "no_original_task"},
    )

    db = SessionLocal()
    try:
        pipeline = ProvisioningPipeline(db)
        request_row = pipeline.begin(
            user_id="provision-channel-user",
            server_id="duffel-mcp",
            reason="Need flights",
        )
        request_row = pipeline.transition(
            request_id=request_row.id,
            new_state=ProvisioningState.AWAITING_AUTH,
            note="auth_link_sent_whatsapp",
        )
        # Provisioning session is intentionally server-side and channel-agnostic.
        state = create_provisioning_session(
            {
                "request_id": request_row.id,
                "user_id": "provision-channel-user",
                "server_id": "duffel-mcp",
                "original_task_id": "run-channel",
                "started_channel": "whatsapp",
                "latest_channel": "imessage",
            },
            ttl_seconds=300,
        )
    finally:
        db.close()

    client = TestClient(app)
    resp = client.get(f"/api/v1/provision/callback?state={state}&code=oauth-code", follow_redirects=False)
    assert resp.status_code in (302, 307)
    assert "/api/v1/provision/success" in str(resp.headers.get("location") or "")

    db2 = SessionLocal()
    try:
        final = get_request(db2, request_id=request_row.id)
        assert final is not None
        assert final.state == ProvisioningState.ACTIVE
    finally:
        db2.close()


def test_provision_start_wave56_requires_professional_plan():
    db = SessionLocal()
    try:
        sub = db.query(Subscription).filter(Subscription.user_id == "prov-start-free-user").first()
        if sub is None:
            sub = Subscription(user_id="prov-start-free-user", plan="free", status="active")
            db.add(sub)
        else:
            sub.plan = "free"
            sub.status = "active"
        db.commit()
    finally:
        db.close()

    client = TestClient(app)
    resp = client.post(
        "/api/v1/provision/start",
        json={
            "user_id": "prov-start-free-user",
            "server_id": "duffel-mcp",
            "reason": "Connect from card",
            "trigger": "onboarding",
        },
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["ok"] is False
    assert "requires the professional plan" in str(body.get("error") or "").lower()
