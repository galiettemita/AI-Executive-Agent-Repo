from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest
from sqlalchemy import text

from app.blueprint.contracts import ProvisioningState
from app.core.config import settings
from app.db.database import SessionLocal
from app.services.provisioning_catalog import compute_catalog_signature
from app.services.provisioning_pipeline import (
    ProvisioningPipeline,
    get_request,
    record_declined,
    search_catalog_entries,
)


def test_provisioning_pipeline_transitions_and_invalid_transition():
    db = SessionLocal()
    try:
        pipeline = ProvisioningPipeline(db)
        req = pipeline.begin(
            user_id="prov-user-1",
            server_id="google-calendar-mcp",
            reason="Need calendar tools",
        )
        assert req.state == ProvisioningState.INITIATED

        req = pipeline.transition(request_id=req.id, new_state=ProvisioningState.AWAITING_AUTH, note="auth_link_sent")
        assert req.state == ProvisioningState.AWAITING_AUTH
        req = pipeline.transition(request_id=req.id, new_state=ProvisioningState.AUTH_RECEIVED, note="oauth_callback")
        assert req.state == ProvisioningState.AUTH_RECEIVED
        req = pipeline.transition(request_id=req.id, new_state=ProvisioningState.PROVISIONING, note="activating")
        assert req.state == ProvisioningState.PROVISIONING
        req = pipeline.transition(request_id=req.id, new_state=ProvisioningState.ACTIVE, note="tools_registered")
        assert req.state == ProvisioningState.ACTIVE

        with pytest.raises(ValueError):
            pipeline.transition(request_id=req.id, new_state=ProvisioningState.INITIATED, note="invalid")
    finally:
        db.close()


def test_provisioning_pipeline_seeds_catalog_defaults():
    db = SessionLocal()
    try:
        _ = ProvisioningPipeline(db)
        count = int(db.execute(text("select count(1) from server_catalog")).scalar() or 0)
        assert count >= 1
    finally:
        db.close()


def test_provisioning_pipeline_dedup_and_expiration():
    db = SessionLocal()
    try:
        pipeline = ProvisioningPipeline(db)
        req1 = pipeline.begin(
            user_id="prov-user-2",
            server_id="duffel-mcp",
            reason="Need flight booking",
        )
        req2 = pipeline.begin(
            user_id="prov-user-2",
            server_id="duffel-mcp",
            reason="Need flight booking again",
        )
        assert req1.id == req2.id

        pipeline.transition(request_id=req1.id, new_state=ProvisioningState.AWAITING_AUTH, note="awaiting_oauth")
        db.execute(
            text("update provisioning_requests set expires_at = :expired_at where id = :id"),
            {"id": req1.id, "expired_at": datetime.now(timezone.utc) - timedelta(minutes=1)},
        )
        db.commit()

        expired = pipeline.expire_timeouts()
        assert expired >= 1

        latest = get_request(db, request_id=req1.id)
        assert latest is not None
        assert latest.state == ProvisioningState.EXPIRED
    finally:
        db.close()


def test_provisioning_pipeline_failure_then_retry_transition():
    db = SessionLocal()
    try:
        pipeline = ProvisioningPipeline(db)
        req = pipeline.begin(
            user_id="prov-user-retry",
            server_id="google-drive-mcp",
            reason="Need drive access",
        )
        req = pipeline.transition(request_id=req.id, new_state=ProvisioningState.AWAITING_AUTH, note="auth_link_sent")
        req = pipeline.transition(request_id=req.id, new_state=ProvisioningState.AUTH_RECEIVED, note="auth_ok")
        req = pipeline.transition(request_id=req.id, new_state=ProvisioningState.PROVISIONING, note="activation_start")
        req = pipeline.transition(
            request_id=req.id,
            new_state=ProvisioningState.FAILED,
            note="activation_failed",
            error_message="mock failure",
        )
        retried = pipeline.transition(
            request_id=req.id,
            new_state=ProvisioningState.PROVISIONING,
            note="retry_activation",
        )
        assert retried.state == ProvisioningState.PROVISIONING
        assert int(retried.retry_count or 0) >= 1
    finally:
        db.close()


def test_catalog_search_plan_gating_and_decline_cooldown():
    db = SessionLocal()
    try:
        pipeline = ProvisioningPipeline(db)
        _ = pipeline  # keep table bootstrap explicit
        db.execute(
            text(
                """
                insert or replace into server_catalog (
                  server_id, display_name, description, auth_type, min_plan, setup_seconds, capabilities, keywords, status
                ) values (
                  :server_id, :display_name, :description, :auth_type, :min_plan, :setup_seconds, :capabilities, :keywords, 'active'
                )
                """
            ),
            {
                "server_id": "duffel-mcp",
                "display_name": "Duffel MCP",
                "description": "Flight search and booking",
                "auth_type": "api_key",
                "min_plan": "free",
                "setup_seconds": 30,
                "capabilities": '["flight","booking","travel"]',
                "keywords": '["duffel","trip"]',
            },
        )
        db.execute(
            text(
                """
                insert or replace into server_catalog (
                  server_id, display_name, description, auth_type, min_plan, setup_seconds, capabilities, keywords, status
                ) values (
                  :server_id, :display_name, :description, :auth_type, :min_plan, :setup_seconds, :capabilities, :keywords, 'active'
                )
                """
            ),
            {
                "server_id": "plaid-mcp",
                "display_name": "Plaid MCP",
                "description": "Bank balances and transactions",
                "auth_type": "plaid_link",
                "min_plan": "professional",
                "setup_seconds": 60,
                "capabilities": '["bank","transactions","finance"]',
                "keywords": '["plaid"]',
            },
        )
        db.commit()

        flight_hits = search_catalog_entries(
            db,
            user_id="prov-user-free",
            query="book me a flight",
        )
        assert any(str(item.get("server_id")) == "duffel-mcp" for item in flight_hits)

        finance_hits = search_catalog_entries(
            db,
            user_id="prov-user-free",
            query="bank transactions",
        )
        assert all(str(item.get("server_id")) != "plaid-mcp" for item in finance_hits)

        _ = record_declined(db, user_id="prov-user-free", server_id="duffel-mcp", reason="not_now")
        flight_hits_after_decline = search_catalog_entries(
            db,
            user_id="prov-user-free",
            query="book me a flight",
        )
        assert all(str(item.get("server_id")) != "duffel-mcp" for item in flight_hits_after_decline)
    finally:
        db.close()


def test_provisioning_pipeline_enforces_catalog_signature(monkeypatch):
    db = SessionLocal()
    try:
        _ = ProvisioningPipeline(db)
        monkeypatch.setattr(settings, "PROVISIONING_REQUIRE_CATALOG_SIGNATURE", True)
        monkeypatch.setattr(settings, "PROVISIONING_CATALOG_SIGNING_SECRET", "catalog-secret")

        db.execute(
            text(
                """
                insert or replace into server_catalog (
                  server_id, display_name, description, auth_type, min_plan, setup_seconds,
                  capabilities, keywords, hosting_model, container_image, source, signature, status
                ) values (
                  :server_id, :display_name, :description, :auth_type, :min_plan, :setup_seconds,
                  :capabilities, :keywords, :hosting_model, :container_image, :source, :signature, :status
                )
                """
            ),
            {
                "server_id": "signed-server-mcp",
                "display_name": "Signed Server",
                "description": "Signed entry",
                "auth_type": "oauth",
                "min_plan": "free",
                "setup_seconds": 30,
                "capabilities": '["signed"]',
                "keywords": '["signed"]',
                "hosting_model": "sidecar",
                "container_image": "123456789012.dkr.ecr.us-east-1.amazonaws.com/signed:latest",
                "source": "catalog",
                "signature": "",
                "status": "active",
            },
        )
        db.commit()

        with pytest.raises(ValueError, match="catalog_signature_missing"):
            ProvisioningPipeline(db).begin(
                user_id="security-user",
                server_id="signed-server-mcp",
                reason="Need signed server",
            )

        payload = {
            "server_id": "signed-server-mcp",
            "display_name": "Signed Server",
            "description": "Signed entry",
            "auth_type": "oauth",
            "min_plan": "free",
            "setup_seconds": 30,
            "capabilities": ["signed"],
            "keywords": ["signed"],
            "hosting_model": "sidecar",
            "container_image": "123456789012.dkr.ecr.us-east-1.amazonaws.com/signed:latest",
            "source": "catalog",
            "status": "active",
        }
        signature = compute_catalog_signature(payload, secret="catalog-secret")
        db.execute(
            text("update server_catalog set signature = :signature where server_id = :server_id"),
            {"signature": signature, "server_id": "signed-server-mcp"},
        )
        db.commit()

        req = ProvisioningPipeline(db).begin(
            user_id="security-user",
            server_id="signed-server-mcp",
            reason="Need signed server",
        )
        assert req.server_id == "signed-server-mcp"
    finally:
        monkeypatch.setattr(settings, "PROVISIONING_REQUIRE_CATALOG_SIGNATURE", False)
        monkeypatch.setattr(settings, "PROVISIONING_CATALOG_SIGNING_SECRET", None)
        db.close()


def test_provisioning_pipeline_enforces_ecr_prefix(monkeypatch):
    db = SessionLocal()
    try:
        _ = ProvisioningPipeline(db)
        monkeypatch.setattr(settings, "PROVISIONING_ECR_ALLOWED_PREFIXES", "123456789012.dkr.ecr.us-east-1.amazonaws.com/")

        db.execute(
            text(
                """
                insert or replace into server_catalog (
                  server_id, display_name, description, auth_type, min_plan, setup_seconds,
                  capabilities, keywords, hosting_model, container_image, source, signature, status
                ) values (
                  :server_id, :display_name, :description, :auth_type, :min_plan, :setup_seconds,
                  :capabilities, :keywords, :hosting_model, :container_image, :source, :signature, :status
                )
                """
            ),
            {
                "server_id": "bad-image-mcp",
                "display_name": "Bad Image",
                "description": "Image should be blocked",
                "auth_type": "oauth",
                "min_plan": "free",
                "setup_seconds": 30,
                "capabilities": '["files"]',
                "keywords": '["files"]',
                "hosting_model": "sidecar",
                "container_image": "docker.io/library/alpine:latest",
                "source": "catalog",
                "signature": None,
                "status": "active",
            },
        )
        db.commit()

        with pytest.raises(ValueError, match="container_image_not_allowed"):
            ProvisioningPipeline(db).begin(
                user_id="security-image-user",
                server_id="bad-image-mcp",
                reason="Need image",
            )
    finally:
        monkeypatch.setattr(settings, "PROVISIONING_ECR_ALLOWED_PREFIXES", "")
        db.close()
