from __future__ import annotations

import asyncio

import pytest

from app.blueprint.mcp.contracts import MCPRunContext
from app.blueprint.mcp.hub import get_mcp_client_hub
from app.blueprint.mcp.mock_servers import mock_tools_for
from app.blueprint.mcp.registry import MCPServerRegistry
from app.blueprint.mcp.wave2_catalog import build_wave2_manifests, bootstrap_wave2_servers
from app.blueprint.mcp.wave3_catalog import build_wave3_manifests, bootstrap_wave3_servers
from app.blueprint.mcp.wave4_catalog import build_wave4_manifests, bootstrap_wave4_servers
from app.db.database import SessionLocal


@pytest.mark.parametrize(
    "builder,expected_count",
    [
        (build_wave2_manifests, 7),
        (build_wave3_manifests, 8),
        (build_wave4_manifests, 7),
    ],
)
def test_wave234_manifests_mock_mode_has_expected_servers(builder, expected_count) -> None:
    manifests = builder(transport_mode="mock")
    assert len(manifests) == expected_count
    assert all(manifest.transport.command and manifest.transport.command.startswith("mock://") for manifest in manifests)


@pytest.mark.parametrize(
    "builder",
    [build_wave2_manifests, build_wave3_manifests, build_wave4_manifests],
)
def test_wave234_http_mode_requires_urls(builder) -> None:
    with pytest.raises(ValueError):
        _ = builder(transport_mode="streamable_http")


@pytest.mark.parametrize(
    "wave_name,bootstrap,expected_count",
    [
        ("wave2", bootstrap_wave2_servers, 7),
        ("wave3", bootstrap_wave3_servers, 8),
        ("wave4", bootstrap_wave4_servers, 7),
    ],
)
def test_bootstrap_wave234_servers_mock_connects_and_invokes(wave_name, bootstrap, expected_count) -> None:
    user_id = f"{wave_name}-bootstrap-user"
    db = SessionLocal()
    try:
        summary = asyncio.run(
            bootstrap(
                db,
                user_id=user_id,
                transport_mode="mock",
                connect=True,
            )
        )
        registry = MCPServerRegistry()
        listed = registry.list_servers(db, user_id=user_id)
    finally:
        db.close()

    assert summary["ok"] is True
    assert summary["count"] == expected_count
    assert summary["connected_count"] == expected_count
    assert summary["failed_count"] == 0
    assert len(listed) == expected_count

    sample = (summary.get("items") or [])[0]
    server_id = str(sample.get("server_id") or "")
    assert server_id

    db2 = SessionLocal()
    try:
        hub = get_mcp_client_hub()
        tools = mock_tools_for(server_id)
        tool_name = str((tools[0].name if tools else "health.check") or "health.check")
        result = asyncio.run(
            hub.call_tool(
                db2,
                server_id=server_id,
                tool_name=tool_name,
                arguments={"probe": True},
                run_context=MCPRunContext(
                    run_id=f"{wave_name}-run",
                    user_id=user_id,
                    provenance="user_direct",
                ),
            )
        )
    finally:
        db2.close()

    assert result.is_error is False
    assert result.server_id == server_id
    assert result.content
