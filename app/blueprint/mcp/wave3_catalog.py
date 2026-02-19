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
class Wave3ServerSpec:
    server_id: str
    display_name: str
    description: str
    expected_tools: list[str]
    env_url_name: str
    tags: list[str]


WAVE3_SPECS: tuple[Wave3ServerSpec, ...] = (
    Wave3ServerSpec(
        server_id="stripe-mcp",
        display_name="Stripe MCP",
        description="Stripe billing and payments workflows.",
        expected_tools=["customers.list", "invoices.create", "payments.create"],
        env_url_name="MCP_STRIPE_URL",
        tags=["wave3", "finance", "billing"],
    ),
    Wave3ServerSpec(
        server_id="quickbooks-mcp",
        display_name="QuickBooks MCP",
        description="QuickBooks accounting and ledger operations.",
        expected_tools=["invoices.create", "payments.create", "reports.get"],
        env_url_name="MCP_QUICKBOOKS_URL",
        tags=["wave3", "finance", "accounting"],
    ),
    Wave3ServerSpec(
        server_id="hubspot-mcp",
        display_name="HubSpot MCP",
        description="HubSpot CRM contacts, deals, and pipeline updates.",
        expected_tools=["contacts.search", "deals.create", "deals.update"],
        env_url_name="MCP_HUBSPOT_URL",
        tags=["wave3", "sales", "crm"],
    ),
    Wave3ServerSpec(
        server_id="salesforce-mcp",
        display_name="Salesforce MCP",
        description="Salesforce account and opportunity management.",
        expected_tools=["accounts.search", "opportunities.update", "leads.create"],
        env_url_name="MCP_SALESFORCE_URL",
        tags=["wave3", "sales", "crm"],
    ),
    Wave3ServerSpec(
        server_id="google-sheets-mcp",
        display_name="Google Sheets MCP",
        description="Google Sheets read/write and formula workflows.",
        expected_tools=["sheets.read", "sheets.write", "sheets.append"],
        env_url_name="MCP_GOOGLE_SHEETS_URL",
        tags=["wave3", "analysis", "google"],
    ),
    Wave3ServerSpec(
        server_id="airtable-mcp",
        display_name="Airtable MCP",
        description="Airtable base search and record updates.",
        expected_tools=["records.search", "records.create", "records.update"],
        env_url_name="MCP_AIRTABLE_URL",
        tags=["wave3", "analysis", "data"],
    ),
    Wave3ServerSpec(
        server_id="jira-mcp",
        display_name="Jira MCP",
        description="Jira issue lifecycle and sprint board tooling.",
        expected_tools=["issues.search", "issues.create", "issues.update"],
        env_url_name="MCP_JIRA_URL",
        tags=["wave3", "engineering", "project_tracking"],
    ),
    Wave3ServerSpec(
        server_id="sentry-mcp",
        display_name="Sentry MCP",
        description="Sentry errors, traces, and issue triage.",
        expected_tools=["issues.list", "issues.assign", "projects.list"],
        env_url_name="MCP_SENTRY_URL",
        tags=["wave3", "engineering", "observability"],
    ),
)


def _mode(mode: str | None) -> str:
    value = (mode or settings.MCP_WAVE3_TRANSPORT_MODE or "mock").strip().lower()
    if value in {"http", "streamable", "streamable_http"}:
        return "streamable_http"
    if value in {"stdio"}:
        return "stdio"
    return "mock"


def _transport_for_spec(spec: Wave3ServerSpec, *, mode: str) -> MCPTransportConfig:
    if mode in {"mock", "stdio"}:
        return MCPTransportConfig(type=MCPTransportType.STDIO, command=f"mock://{spec.server_id}")

    url = getattr(settings, spec.env_url_name, None)
    if not url:
        raise ValueError(f"{spec.env_url_name} is required when MCP Wave 3 mode is '{mode}'")
    headers: dict[str, str] = {}
    if settings.MCP_HOST_TOKEN:
        headers["X-MCP-Host-Token"] = settings.MCP_HOST_TOKEN
    return MCPTransportConfig(
        type=MCPTransportType.STREAMABLE_HTTP,
        url=url,
        headers=headers,
        timeout_ms=15000,
    )


def build_wave3_manifests(*, transport_mode: str | None = None) -> list[MCPServerManifest]:
    mode = _mode(transport_mode)
    manifests: list[MCPServerManifest] = []
    for spec in WAVE3_SPECS:
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
                daily_budget_cents=2200,
            )
        )
    return manifests


async def bootstrap_wave3_servers(
    db: Session,
    *,
    user_id: str,
    transport_mode: str | None = None,
    connect: bool = True,
) -> dict[str, Any]:
    manifests = build_wave3_manifests(transport_mode=transport_mode)
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


def bootstrap_wave3_servers_sync(
    db: Session,
    *,
    user_id: str,
    transport_mode: str | None = None,
    connect: bool = True,
) -> dict[str, Any]:
    return asyncio.run(
        bootstrap_wave3_servers(
            db,
            user_id=user_id,
            transport_mode=transport_mode,
            connect=connect,
        )
    )
