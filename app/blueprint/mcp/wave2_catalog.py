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
class Wave2ServerSpec:
    server_id: str
    display_name: str
    description: str
    expected_tools: list[str]
    env_url_name: str
    tags: list[str]


WAVE2_SPECS: tuple[Wave2ServerSpec, ...] = (
    Wave2ServerSpec(
        server_id="slack-mcp",
        display_name="Slack MCP",
        description="Slack channels, threads, and messaging workflows.",
        expected_tools=["channels.list", "messages.search", "messages.send"],
        env_url_name="MCP_SLACK_URL",
        tags=["wave2", "communication", "collaboration"],
    ),
    Wave2ServerSpec(
        server_id="outlook-mcp",
        display_name="Outlook MCP",
        description="Outlook email and calendar management.",
        expected_tools=["mail.search", "calendar.list", "mail.send"],
        env_url_name="MCP_OUTLOOK_URL",
        tags=["wave2", "communication", "microsoft"],
    ),
    Wave2ServerSpec(
        server_id="teams-mcp",
        display_name="Teams MCP",
        description="Microsoft Teams meetings and channel collaboration.",
        expected_tools=["teams.list", "channels.list", "messages.send"],
        env_url_name="MCP_TEAMS_URL",
        tags=["wave2", "collaboration", "microsoft"],
    ),
    Wave2ServerSpec(
        server_id="linear-mcp",
        display_name="Linear MCP",
        description="Linear issue tracking and sprint updates.",
        expected_tools=["issues.list", "issues.create", "issues.update"],
        env_url_name="MCP_LINEAR_URL",
        tags=["wave2", "engineering", "planning"],
    ),
    Wave2ServerSpec(
        server_id="asana-mcp",
        display_name="Asana MCP",
        description="Asana project, task, and milestone operations.",
        expected_tools=["tasks.list", "tasks.create", "tasks.update"],
        env_url_name="MCP_ASANA_URL",
        tags=["wave2", "project_management"],
    ),
    Wave2ServerSpec(
        server_id="discord-mcp",
        display_name="Discord MCP",
        description="Discord server and channel messaging tools.",
        expected_tools=["channels.list", "messages.send"],
        env_url_name="MCP_DISCORD_URL",
        tags=["wave2", "communication"],
    ),
    Wave2ServerSpec(
        server_id="whatsapp-business-mcp",
        display_name="WhatsApp Business MCP",
        description="WhatsApp Business message and template operations.",
        expected_tools=["templates.list", "messages.send"],
        env_url_name="MCP_WHATSAPP_BUSINESS_URL",
        tags=["wave2", "communication", "whatsapp"],
    ),
)


def _mode(mode: str | None) -> str:
    value = (mode or settings.MCP_WAVE2_TRANSPORT_MODE or "mock").strip().lower()
    if value in {"http", "streamable", "streamable_http"}:
        return "streamable_http"
    if value in {"stdio"}:
        return "stdio"
    return "mock"


def _transport_for_spec(spec: Wave2ServerSpec, *, mode: str) -> MCPTransportConfig:
    if mode in {"mock", "stdio"}:
        return MCPTransportConfig(type=MCPTransportType.STDIO, command=f"mock://{spec.server_id}")

    url = getattr(settings, spec.env_url_name, None)
    if not url:
        raise ValueError(f"{spec.env_url_name} is required when MCP Wave 2 mode is '{mode}'")
    headers: dict[str, str] = {}
    if settings.MCP_HOST_TOKEN:
        headers["X-MCP-Host-Token"] = settings.MCP_HOST_TOKEN
    return MCPTransportConfig(
        type=MCPTransportType.STREAMABLE_HTTP,
        url=url,
        headers=headers,
        timeout_ms=15000,
    )


def build_wave2_manifests(*, transport_mode: str | None = None) -> list[MCPServerManifest]:
    mode = _mode(transport_mode)
    manifests: list[MCPServerManifest] = []
    for spec in WAVE2_SPECS:
        transport = _transport_for_spec(spec, mode=mode)
        manifests.append(
            MCPServerManifest(
                server_id=spec.server_id,
                display_name=spec.display_name,
                description=spec.description,
                transport=transport,
                expected_tools=list(spec.expected_tools),
                tags=list(spec.tags),
                rate_limit_per_min=60,
                daily_budget_cents=2000,
            )
        )
    return manifests


async def bootstrap_wave2_servers(
    db: Session,
    *,
    user_id: str,
    transport_mode: str | None = None,
    connect: bool = True,
) -> dict[str, Any]:
    manifests = build_wave2_manifests(transport_mode=transport_mode)
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


def bootstrap_wave2_servers_sync(
    db: Session,
    *,
    user_id: str,
    transport_mode: str | None = None,
    connect: bool = True,
) -> dict[str, Any]:
    return asyncio.run(
        bootstrap_wave2_servers(
            db,
            user_id=user_id,
            transport_mode=transport_mode,
            connect=connect,
        )
    )
