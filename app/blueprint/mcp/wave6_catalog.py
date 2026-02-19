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
class Wave6ServerSpec:
    server_id: str
    display_name: str
    description: str
    expected_tools: list[str]
    env_url_name: str
    tags: list[str]


WAVE6_SPECS: tuple[Wave6ServerSpec, ...] = (
    Wave6ServerSpec(
        server_id="booking-com-mcp",
        display_name="Booking.com MCP",
        description="Hotel search, availability, and booking workflows.",
        expected_tools=["hotels.search", "availability.get", "bookings.create", "bookings.get"],
        env_url_name="MCP_BOOKING_COM_URL",
        tags=["wave6", "travel", "booking"],
    ),
    Wave6ServerSpec(
        server_id="docusign-mcp",
        display_name="DocuSign MCP",
        description="Envelope creation, sending, and status tracking.",
        expected_tools=["envelopes.create", "envelopes.send", "envelopes.get"],
        env_url_name="MCP_DOCUSIGN_URL",
        tags=["wave6", "legal", "documents"],
    ),
    Wave6ServerSpec(
        server_id="canva-mcp",
        display_name="Canva MCP",
        description="Template-based design generation and asset export.",
        expected_tools=["templates.list", "designs.create", "designs.export"],
        env_url_name="MCP_CANVA_URL",
        tags=["wave6", "creative", "design"],
    ),
    Wave6ServerSpec(
        server_id="instacart-mcp",
        display_name="Instacart MCP",
        description="Grocery cart building and checkout proposals.",
        expected_tools=["stores.list", "items.search", "cart.add_items", "cart.checkout"],
        env_url_name="MCP_INSTACART_URL",
        tags=["wave6", "commerce", "lifestyle"],
    ),
    Wave6ServerSpec(
        server_id="tesla-mcp",
        display_name="Tesla MCP",
        description="Vehicle status, climate, and charging controls.",
        expected_tools=["vehicles.list", "vehicle.status", "vehicle.command"],
        env_url_name="MCP_TESLA_URL",
        tags=["wave6", "vehicle", "lifestyle"],
    ),
)


def _mode(mode: str | None) -> str:
    value = (mode or settings.MCP_WAVE6_TRANSPORT_MODE or "mock").strip().lower()
    if value in {"http", "streamable", "streamable_http"}:
        return "streamable_http"
    if value in {"stdio"}:
        return "stdio"
    return "mock"


def _transport_for_spec(spec: Wave6ServerSpec, *, mode: str) -> MCPTransportConfig:
    if mode in {"mock", "stdio"}:
        return MCPTransportConfig(type=MCPTransportType.STDIO, command=f"mock://{spec.server_id}")

    url = getattr(settings, spec.env_url_name, None)
    if not url:
        raise ValueError(f"{spec.env_url_name} is required when MCP Wave 6 mode is '{mode}'")
    headers: dict[str, str] = {}
    if settings.MCP_HOST_TOKEN:
        headers["X-MCP-Host-Token"] = settings.MCP_HOST_TOKEN
    return MCPTransportConfig(
        type=MCPTransportType.STREAMABLE_HTTP,
        url=url,
        headers=headers,
        timeout_ms=15000,
    )


def build_wave6_manifests(*, transport_mode: str | None = None) -> list[MCPServerManifest]:
    mode = _mode(transport_mode)
    manifests: list[MCPServerManifest] = []
    for spec in WAVE6_SPECS:
        transport = _transport_for_spec(spec, mode=mode)
        manifests.append(
            MCPServerManifest(
                server_id=spec.server_id,
                display_name=spec.display_name,
                description=spec.description,
                transport=transport,
                expected_tools=list(spec.expected_tools),
                tags=list(spec.tags),
                rate_limit_per_min=30,
                daily_budget_cents=2500,
            )
        )
    return manifests


async def bootstrap_wave6_servers(
    db: Session,
    *,
    user_id: str,
    transport_mode: str | None = None,
    connect: bool = True,
) -> dict[str, Any]:
    manifests = build_wave6_manifests(transport_mode=transport_mode)
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


def bootstrap_wave6_servers_sync(
    db: Session,
    *,
    user_id: str,
    transport_mode: str | None = None,
    connect: bool = True,
) -> dict[str, Any]:
    return asyncio.run(
        bootstrap_wave6_servers(
            db,
            user_id=user_id,
            transport_mode=transport_mode,
            connect=connect,
        )
    )
