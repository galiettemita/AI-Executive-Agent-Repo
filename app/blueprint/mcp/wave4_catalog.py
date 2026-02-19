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
class Wave4ServerSpec:
    server_id: str
    display_name: str
    description: str
    expected_tools: list[str]
    env_url_name: str
    tags: list[str]


WAVE4_SPECS: tuple[Wave4ServerSpec, ...] = (
    Wave4ServerSpec(
        server_id="google-maps-mcp",
        display_name="Google Maps MCP",
        description="Maps search, routing, and ETA capabilities.",
        expected_tools=["places.search", "routes.get", "eta.get"],
        env_url_name="MCP_GOOGLE_MAPS_URL",
        tags=["wave4", "lifestyle", "travel"],
    ),
    Wave4ServerSpec(
        server_id="uber-lyft-mcp",
        display_name="Uber/Lyft MCP",
        description="Rideshare estimates and trip coordination.",
        expected_tools=["rides.estimate", "rides.request", "rides.status"],
        env_url_name="MCP_UBER_LYFT_URL",
        tags=["wave4", "lifestyle", "travel"],
    ),
    Wave4ServerSpec(
        server_id="opentable-resy-mcp",
        display_name="OpenTable/Resy MCP",
        description="Restaurant discovery and reservation workflows.",
        expected_tools=["restaurants.search", "reservations.create", "reservations.cancel"],
        env_url_name="MCP_OPENTABLE_RESY_URL",
        tags=["wave4", "lifestyle", "food"],
    ),
    Wave4ServerSpec(
        server_id="homeassistant-mcp",
        display_name="Home Assistant MCP",
        description="Smart-home controls, scenes, and telemetry.",
        expected_tools=["devices.list", "devices.command", "scenes.activate"],
        env_url_name="MCP_HOMEASSISTANT_URL",
        tags=["wave4", "lifestyle", "smart_home"],
    ),
    Wave4ServerSpec(
        server_id="spotify-mcp",
        display_name="Spotify MCP",
        description="Spotify playlists and playback controls.",
        expected_tools=["playlists.list", "playback.start", "playback.pause"],
        env_url_name="MCP_SPOTIFY_URL",
        tags=["wave4", "lifestyle", "media"],
    ),
    Wave4ServerSpec(
        server_id="evernote-mcp",
        display_name="Evernote MCP",
        description="Evernote notes and notebook management.",
        expected_tools=["notes.search", "notes.create", "notes.update"],
        env_url_name="MCP_EVERNOTE_URL",
        tags=["wave4", "lifestyle", "notes"],
    ),
    Wave4ServerSpec(
        server_id="dropbox-mcp",
        display_name="Dropbox MCP",
        description="Dropbox document search and file operations.",
        expected_tools=["files.search", "files.get", "files.upload"],
        env_url_name="MCP_DROPBOX_URL",
        tags=["wave4", "lifestyle", "storage"],
    ),
)


def _mode(mode: str | None) -> str:
    value = (mode or settings.MCP_WAVE4_TRANSPORT_MODE or "mock").strip().lower()
    if value in {"http", "streamable", "streamable_http"}:
        return "streamable_http"
    if value in {"stdio"}:
        return "stdio"
    return "mock"


def _transport_for_spec(spec: Wave4ServerSpec, *, mode: str) -> MCPTransportConfig:
    if mode in {"mock", "stdio"}:
        return MCPTransportConfig(type=MCPTransportType.STDIO, command=f"mock://{spec.server_id}")

    url = getattr(settings, spec.env_url_name, None)
    if not url:
        raise ValueError(f"{spec.env_url_name} is required when MCP Wave 4 mode is '{mode}'")
    headers: dict[str, str] = {}
    if settings.MCP_HOST_TOKEN:
        headers["X-MCP-Host-Token"] = settings.MCP_HOST_TOKEN
    return MCPTransportConfig(
        type=MCPTransportType.STREAMABLE_HTTP,
        url=url,
        headers=headers,
        timeout_ms=15000,
    )


def build_wave4_manifests(*, transport_mode: str | None = None) -> list[MCPServerManifest]:
    mode = _mode(transport_mode)
    manifests: list[MCPServerManifest] = []
    for spec in WAVE4_SPECS:
        transport = _transport_for_spec(spec, mode=mode)
        manifests.append(
            MCPServerManifest(
                server_id=spec.server_id,
                display_name=spec.display_name,
                description=spec.description,
                transport=transport,
                expected_tools=list(spec.expected_tools),
                tags=list(spec.tags),
                rate_limit_per_min=40,
                daily_budget_cents=1800,
            )
        )
    return manifests


async def bootstrap_wave4_servers(
    db: Session,
    *,
    user_id: str,
    transport_mode: str | None = None,
    connect: bool = True,
) -> dict[str, Any]:
    manifests = build_wave4_manifests(transport_mode=transport_mode)
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


def bootstrap_wave4_servers_sync(
    db: Session,
    *,
    user_id: str,
    transport_mode: str | None = None,
    connect: bool = True,
) -> dict[str, Any]:
    return asyncio.run(
        bootstrap_wave4_servers(
            db,
            user_id=user_id,
            transport_mode=transport_mode,
            connect=connect,
        )
    )
