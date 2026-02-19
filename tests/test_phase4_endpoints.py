from __future__ import annotations

import io
import zipfile

from fastapi.testclient import TestClient
from sqlalchemy import text

from app.db.database import SessionLocal
from app.main import app
from app.api.routes import v1_core
from app.services.provisioning_catalog import all_catalog_entries, render_available_servers_section


def test_api_v1_export_zip_payload():
    client = TestClient(app)
    resp = client.get("/api/v1/export", params={"user_id": "phase4-export-user", "format": "zip"})
    assert resp.status_code == 200
    assert resp.headers.get("content-type", "").startswith("application/zip")

    with zipfile.ZipFile(io.BytesIO(resp.content)) as zf:
        names = set(zf.namelist())
    assert "manifest.json" in names
    assert "export.json" in names


def test_internal_experiments_create_and_list():
    client = TestClient(app)
    create = client.post(
        "/internal/experiments",
        json={
            "name": "Prompt Canary",
            "description": "Test MCP tool selection wording",
            "status": "canary",
            "prompt_group": "system_prompt",
            "allocation": {"control": 80, "candidate": 20},
            "config": {"metric": "tool_usage_score"},
            "created_by": "phase4-test",
        },
    )
    assert create.status_code == 200
    body = create.json()
    assert body["ok"] is True
    exp_id = body["experiment"]["id"]

    listed = client.get("/internal/experiments", params={"status": "canary", "user_id": "phase4-user"})
    assert listed.status_code == 200
    items = listed.json()["items"]
    assert any(str(item.get("id")) == exp_id for item in items)
    assert all("assigned_variant" in item for item in items)


def test_provision_start_uses_pipeline_and_onboarding_trigger():
    client = TestClient(app)
    resp = client.post(
        "/api/v1/provision/start",
        json={
            "user_id": "phase4-provision-user",
            "server_id": "google-calendar-mcp",
            "reason": "Connect from onboarding card",
            "trigger": "onboarding",
        },
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["ok"] is True
    assert body["trigger"] == "onboarding"
    request_id = body["request_id"]

    db = SessionLocal()
    try:
        row = db.execute(
            text("select trigger, server_id from provisioning_requests where id = :id"),
            {"id": request_id},
        ).mappings().first()
        assert row is not None
        assert str(row.get("trigger")) == "onboarding"
        assert str(row.get("server_id")) == "google-calendar-mcp"
    finally:
        db.close()


def test_onboarding_connect_flows_through_pipeline():
    client = TestClient(app)
    resp = client.post(
        "/onboarding/connect",
        json={
            "user_id": "phase4-onboarding-user",
            "server_id": "google-drive-mcp",
            "reason": "Connect during onboarding",
        },
    )
    assert resp.status_code == 200
    payload = resp.json()
    assert payload["ok"] is True
    assert payload["trigger"] == "onboarding"
    assert payload["server_id"] == "google-drive-mcp"


def test_catalog_contains_wave_expansion_and_tools_guidance():
    db = SessionLocal()
    try:
        entries = all_catalog_entries(db)
    finally:
        db.close()
    assert len(entries) >= 40
    ids = {str(item.get("server_id") or "") for item in entries}
    assert "slack-mcp" in ids
    assert "stripe-mcp" in ids
    assert "google-maps-mcp" in ids
    assert "booking-com-mcp" in ids
    assert "tesla-mcp" in ids

    section = render_available_servers_section(
        [
            {
                "server_id": "slack-mcp",
                "description": "Slack channels and threads",
                "auth_type": "oauth",
                "setup_seconds": 35,
            }
        ]
    )
    assert "Available Servers (Not Connected)" in section
    assert "How to Connect" in section


def test_docs_search_endpoint_uses_connected_search(monkeypatch):
    monkeypatch.setattr(
        v1_core,
        "search_connected_docs",
        lambda db, user_id, query, max_results=8: {
            "query": query,
            "results": [{"provider": "notion", "title": "Roadmap", "id": "123"}],
            "providers": {"notion": {"configured": True, "hits": 1}, "google_drive": {"configured": True, "hits": 0}},
        },
    )
    client = TestClient(app)
    resp = client.get("/api/v1/connectors/docs/search", params={"user_id": "docs-user", "query": "roadmap"})
    assert resp.status_code == 200
    body = resp.json()
    assert body["ok"] is True
    assert body["results"][0]["provider"] == "notion"
