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
class Wave1ServerSpec:
    server_id: str
    display_name: str
    description: str
    expected_tools: list[str]
    env_url_name: str
    tags: list[str]


WAVE1_SPECS: tuple[Wave1ServerSpec, ...] = (
    Wave1ServerSpec(
        server_id="google-calendar-mcp",
        display_name="Google Calendar MCP",
        description="Calendar read/write operations for scheduling and conflict handling.",
        expected_tools=["calendar.list", "calendar.create", "calendar.update", "calendar.delete"],
        env_url_name="MCP_GOOGLE_CALENDAR_URL",
        tags=["wave1", "calendar", "google"],
    ),
    Wave1ServerSpec(
        server_id="google-drive-mcp",
        display_name="Google Drive MCP",
        description="Drive file discovery and retrieval for context injection.",
        expected_tools=["drive.search", "drive.get_file", "drive.list_recent"],
        env_url_name="MCP_GOOGLE_DRIVE_URL",
        tags=["wave1", "drive", "google"],
    ),
    Wave1ServerSpec(
        server_id="gmail-mcp",
        display_name="Gmail MCP",
        description="Gmail read/search/send tool surface.",
        expected_tools=["gmail.search", "gmail.get_message", "gmail.send"],
        env_url_name="MCP_GMAIL_URL",
        tags=["wave1", "gmail", "google"],
    ),
    Wave1ServerSpec(
        server_id="notion-mcp",
        display_name="Notion MCP",
        description="Notion search/read/write operations for workspace docs.",
        expected_tools=["notion.search", "notion.get_page", "notion.update_page"],
        env_url_name="MCP_NOTION_URL",
        tags=["wave1", "notion", "knowledge"],
    ),
    Wave1ServerSpec(
        server_id="todoist-mcp",
        display_name="Todoist MCP",
        description="Task capture and task-state synchronization.",
        expected_tools=["todoist.list_tasks", "todoist.create_task", "todoist.complete_task"],
        env_url_name="MCP_TODOIST_URL",
        tags=["wave1", "todoist", "tasks"],
    ),
    Wave1ServerSpec(
        server_id="brave-search-mcp",
        display_name="Brave Search MCP",
        description="Web search enrichment for research and grounding.",
        expected_tools=["brave.search", "brave.news", "brave.images"],
        env_url_name="MCP_BRAVE_SEARCH_URL",
        tags=["wave1", "search", "brave"],
    ),
    Wave1ServerSpec(
        server_id="github-mcp",
        display_name="GitHub MCP",
        description="Repository/issue/PR operations for engineering workflows.",
        expected_tools=["github.list_repos", "github.search_issues", "github.create_issue"],
        env_url_name="MCP_GITHUB_URL",
        tags=["wave1", "github", "engineering"],
    ),
    Wave1ServerSpec(
        server_id="apple-reminders-mcp",
        display_name="Apple Reminders MCP",
        description="Custom MCP server for reminders list/read/write/complete operations.",
        expected_tools=["reminders.list", "reminders.create", "reminders.complete"],
        env_url_name="MCP_APPLE_REMINDERS_URL",
        tags=["wave1", "apple", "reminders", "custom"],
    ),
)


def _mode(mode: str | None) -> str:
    value = (mode or settings.MCP_WAVE1_TRANSPORT_MODE or "mock").strip().lower()
    if value in {"http", "streamable", "streamable_http"}:
        return "streamable_http"
    if value in {"stdio"}:
        return "stdio"
    return "mock"


def _transport_for_spec(spec: Wave1ServerSpec, *, mode: str) -> MCPTransportConfig:
    if mode == "mock":
        return MCPTransportConfig(type=MCPTransportType.STDIO, command=f"mock://{spec.server_id}")

    if mode == "stdio" and spec.server_id == "apple-reminders-mcp":
        return MCPTransportConfig(
            type=MCPTransportType.STDIO,
            command="python3",
            args=["-m", "app.blueprint.mcp.custom.apple_reminders.server"],
            timeout_ms=15000,
        )

    url = getattr(settings, spec.env_url_name, None)
    if not url:
        raise ValueError(f"{spec.env_url_name} is required when MCP Wave 1 mode is '{mode}'")
    headers: dict[str, str] = {}
    if settings.MCP_HOST_TOKEN:
        headers["X-MCP-Host-Token"] = settings.MCP_HOST_TOKEN
    return MCPTransportConfig(
        type=MCPTransportType.STREAMABLE_HTTP,
        url=url,
        headers=headers,
        timeout_ms=15000,
    )


def build_wave1_manifests(*, transport_mode: str | None = None) -> list[MCPServerManifest]:
    mode = _mode(transport_mode)
    manifests: list[MCPServerManifest] = []
    for spec in WAVE1_SPECS:
        transport = _transport_for_spec(spec, mode=mode)
        manifests.append(
            MCPServerManifest(
                server_id=spec.server_id,
                display_name=spec.display_name,
                description=spec.description,
                transport=transport,
                expected_tools=list(spec.expected_tools),
                tags=list(spec.tags),
                rate_limit_per_min=90 if "search" in spec.tags else 60,
                daily_budget_cents=2500 if "search" in spec.tags else 1800,
            )
        )
    return manifests


async def bootstrap_wave1_servers(
    db: Session,
    *,
    user_id: str,
    transport_mode: str | None = None,
    connect: bool = True,
) -> dict[str, Any]:
    manifests = build_wave1_manifests(transport_mode=transport_mode)
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


def bootstrap_wave1_servers_sync(
    db: Session,
    *,
    user_id: str,
    transport_mode: str | None = None,
    connect: bool = True,
) -> dict[str, Any]:
    return asyncio.run(
        bootstrap_wave1_servers(
            db,
            user_id=user_id,
            transport_mode=transport_mode,
            connect=connect,
        )
    )
