from __future__ import annotations

import asyncio
from dataclasses import dataclass
from typing import Any

from sqlalchemy.orm import Session

from app.blueprint.mcp.contracts import MCPServerManifest, MCPTransportConfig, MCPTransportType
from app.blueprint.mcp.hub import get_mcp_client_hub
from app.blueprint.mcp.registry import MCPServerRegistry
from app.core.config import settings


@dataclass(frozen=True)
class Wave5ServerSpec:
    server_id: str
    display_name: str
    description: str
    expected_tools: list[str]
    env_url_name: str
    tags: list[str]


WAVE5_SPECS: tuple[Wave5ServerSpec, ...] = (
    Wave5ServerSpec(
        server_id="duffel-mcp",
        display_name="Duffel MCP",
        description="Flight search, offers, and booking operations.",
        expected_tools=["flights.search", "offers.get", "orders.create", "orders.get"],
        env_url_name="MCP_DUFFEL_URL",
        tags=["wave5", "travel", "booking"],
    ),
    Wave5ServerSpec(
        server_id="zoom-mcp",
        display_name="Zoom MCP",
        description="Meeting scheduling, recordings, and transcript retrieval.",
        expected_tools=["meetings.list", "meetings.create", "recordings.list", "transcripts.get"],
        env_url_name="MCP_ZOOM_URL",
        tags=["wave5", "communication", "zoom"],
    ),
    Wave5ServerSpec(
        server_id="calendly-mcp",
        display_name="Calendly MCP",
        description="Scheduling links, availability, and invitee management.",
        expected_tools=["availability.get", "events.list", "invitees.create"],
        env_url_name="MCP_CALENDLY_URL",
        tags=["wave5", "productivity", "calendar"],
    ),
    Wave5ServerSpec(
        server_id="plaid-mcp",
        display_name="Plaid MCP",
        description="Bank accounts, balances, and transaction intelligence.",
        expected_tools=["accounts.list", "transactions.list", "balances.get"],
        env_url_name="MCP_PLAID_URL",
        tags=["wave5", "finance", "plaid"],
    ),
    Wave5ServerSpec(
        server_id="crunchbase-mcp",
        display_name="Crunchbase MCP",
        description="Company research, funding rounds, and market intelligence.",
        expected_tools=["companies.search", "companies.get", "funding_rounds.list"],
        env_url_name="MCP_CRUNCHBASE_URL",
        tags=["wave5", "research", "finance"],
    ),
)


def _mode(mode: str | None) -> str:
    value = (mode or settings.MCP_WAVE5_TRANSPORT_MODE or "mock").strip().lower()
    if value in {"http", "streamable", "streamable_http"}:
        return "streamable_http"
    if value in {"stdio"}:
        return "stdio"
    return "mock"


def _transport_for_spec(spec: Wave5ServerSpec, *, mode: str) -> MCPTransportConfig:
    if mode in {"mock", "stdio"}:
        return MCPTransportConfig(type=MCPTransportType.STDIO, command=f"mock://{spec.server_id}")

    url = getattr(settings, spec.env_url_name, None)
    if not url:
        raise ValueError(f"{spec.env_url_name} is required when MCP Wave 5 mode is '{mode}'")
    headers: dict[str, str] = {}
    if settings.MCP_HOST_TOKEN:
        headers["X-MCP-Host-Token"] = settings.MCP_HOST_TOKEN
    return MCPTransportConfig(
        type=MCPTransportType.STREAMABLE_HTTP,
        url=url,
        headers=headers,
        timeout_ms=15000,
    )


def build_wave5_manifests(*, transport_mode: str | None = None) -> list[MCPServerManifest]:
    mode = _mode(transport_mode)
    manifests: list[MCPServerManifest] = []
    for spec in WAVE5_SPECS:
        transport = _transport_for_spec(spec, mode=mode)
        manifests.append(
            MCPServerManifest(
                server_id=spec.server_id,
                display_name=spec.display_name,
                description=spec.description,
                transport=transport,
                expected_tools=list(spec.expected_tools),
                tags=list(spec.tags),
                rate_limit_per_min=45,
                daily_budget_cents=2500,
            )
        )
    return manifests


async def bootstrap_wave5_servers(
    db: Session,
    *,
    user_id: str,
    transport_mode: str | None = None,
    connect: bool = True,
) -> dict[str, Any]:
    manifests = build_wave5_manifests(transport_mode=transport_mode)
    mode = _mode(transport_mode)
    registry = MCPServerRegistry()
    registry.ensure_tables(db)
    hub = get_mcp_client_hub()
    await hub.initialize(db)

    items: list[dict[str, Any]] = []
    for manifest in manifests:
        config = registry.upsert_server(db, manifest)
        registry.bind_user_server(db, user_id=user_id, server_id=config.server_id)
        item: dict[str, Any] = {
            "server_id": config.server_id,
            "display_name": config.display_name,
            "registered": True,
            "connected": False,
            "transport_type": config.transport.type.value,
        }
        if connect:
            try:
                result = await hub.connect_server(db, user_id=user_id, server_id=config.server_id)
                item["connected"] = bool(result.get("connected"))
            except Exception as exc:
                item["error"] = str(exc)
        items.append(item)

    connected_count = sum(1 for item in items if item.get("connected"))
    failed_count = sum(1 for item in items if item.get("error"))
    return {
        "ok": True,
        "mode": mode,
        "user_id": user_id,
        "count": len(items),
        "connected_count": connected_count,
        "failed_count": failed_count,
        "items": items,
    }


def bootstrap_wave5_servers_sync(
    db: Session,
    *,
    user_id: str,
    transport_mode: str | None = None,
    connect: bool = True,
) -> dict[str, Any]:
    return asyncio.run(
        bootstrap_wave5_servers(
            db,
            user_id=user_id,
            transport_mode=transport_mode,
            connect=connect,
        )
    )
