from __future__ import annotations

import asyncio

import pytest

from app.blueprint.mcp.hub import get_mcp_client_hub
from app.blueprint.mcp.contracts import MCPRunContext
from app.blueprint.mcp.registry import MCPServerRegistry
from app.blueprint.mcp.wave1_catalog import build_wave1_manifests, bootstrap_wave1_servers
from app.db.database import SessionLocal


def test_build_wave1_manifests_mock_mode_has_all_servers() -> None:
    manifests = build_wave1_manifests(transport_mode="mock")
    ids = [manifest.server_id for manifest in manifests]
    assert len(ids) == 8
    assert "google-calendar-mcp" in ids
    assert "apple-reminders-mcp" in ids
    assert all(manifest.transport.command and manifest.transport.command.startswith("mock://") for manifest in manifests)


def test_build_wave1_manifests_http_mode_requires_urls() -> None:
    with pytest.raises(ValueError):
        _ = build_wave1_manifests(transport_mode="streamable_http")


def test_bootstrap_wave1_servers_mock_connects_and_invokes() -> None:
    db = SessionLocal()
    try:
        summary = asyncio.run(
            bootstrap_wave1_servers(
                db,
                user_id="wave1-bootstrap-user",
                transport_mode="mock",
                connect=True,
            )
        )
        registry = MCPServerRegistry()
        listed = registry.list_servers(db, user_id="wave1-bootstrap-user")
    finally:
        db.close()

    assert summary["ok"] is True
    assert summary["count"] == 8
    assert summary["connected_count"] == 8
    assert summary["failed_count"] == 0
    assert len(listed) == 8

    db2 = SessionLocal()
    try:
        hub = get_mcp_client_hub()
        result = asyncio.run(
            hub.call_tool(
                db2,
                server_id="google-calendar-mcp",
                tool_name="calendar.list",
                arguments={"start": "2026-02-01T00:00:00Z", "end": "2026-02-02T00:00:00Z"},
                run_context=MCPRunContext(
                    run_id="wave1-run",
                    user_id="wave1-bootstrap-user",
                    provenance="user_direct",
                ),
            )
        )
    finally:
        db2.close()
    assert result.is_error is False
    assert result.server_id == "google-calendar-mcp"
    assert result.content
