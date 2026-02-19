from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel, Field
from sqlalchemy import text
from sqlalchemy.orm import Session

from app.api.deps import get_db
from app.blueprint.mcp import MCPRunContext, MCPServerManifest, MCPToolExecuteRequest
from app.blueprint.mcp.hub import get_mcp_client_hub
from app.blueprint.mcp.registry import MCPServerRegistry
from app.blueprint.contracts import ToolCall, ContentProvenance
from app.blueprint.mcp.wave1_catalog import bootstrap_wave1_servers
from app.blueprint.mcp.wave1_catalog import WAVE1_SPECS
from app.blueprint.mcp.wave2_catalog import bootstrap_wave2_servers, WAVE2_SPECS
from app.blueprint.mcp.wave3_catalog import bootstrap_wave3_servers, WAVE3_SPECS
from app.blueprint.mcp.wave4_catalog import bootstrap_wave4_servers, WAVE4_SPECS
from app.blueprint.mcp.wave5_catalog import bootstrap_wave5_servers, WAVE5_SPECS
from app.blueprint.mcp.wave6_catalog import bootstrap_wave6_servers, WAVE6_SPECS
from app.core.config import settings
from app.services.billing_middleware import count_connected_mcp_servers, get_billing_subscription


router = APIRouter(prefix="/api/v1/mcp", tags=["mcp-v1"])


class MCPResourceSubscribeRequest(BaseModel):
    uri: str


class MCPPromptBuildRequest(BaseModel):
    name: str
    arguments: dict[str, object] = Field(default_factory=dict)


class MCPWaveBootstrapRequest(BaseModel):
    transport_mode: str | None = None
    connect: bool = True


def _is_trial_plan(plan: str | None, status: str | None) -> bool:
    p = (plan or "").strip().lower()
    s = (status or "").strip().lower()
    return p in {"free_trial", "trial"} or s == "trialing"


_PLAN_RANK = {
    "free": 0,
    "trial": 0,
    "free_trial": 0,
    "starter": 1,
    "personal": 1,
    "plus": 2,
    "professional": 3,
    "pro": 3,
    "enterprise": 4,
}


def _plan_rank(value: str | None) -> int:
    return _PLAN_RANK.get(str(value or "free").strip().lower(), 0)


def _wave56_server_ids() -> set[str]:
    raw = str(settings.WAVE56_PLAN_GATED_SERVER_IDS or "")
    return {
        item.strip().lower()
        for item in raw.replace("|", ",").split(",")
        if item.strip()
    }


def _enforce_mcp_connect_gate(db: Session, *, user_id: str, server_id: str) -> None:
    sub = get_billing_subscription(db, user_id)
    if _is_trial_plan(sub.plan, sub.status):
        allowed = {spec.server_id for spec in WAVE1_SPECS}
        if server_id not in allowed:
            raise HTTPException(status_code=402, detail="Upgrade required to connect this MCP server")

        registry = MCPServerRegistry()
        registry.ensure_tables(db)
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            existing = db.execute(
                text(
                    "select 1 from mcp_user_servers where user_id = :user_id and server_id = :server_id and is_enabled = 1 limit 1"
                ),
                {"user_id": user_id, "server_id": server_id},
            ).first()
        else:
            existing = db.execute(
                text(
                    "select 1 from mcp_user_servers where user_id::text = :user_id and server_id = :server_id and is_enabled = true limit 1"
                ),
                {"user_id": user_id, "server_id": server_id},
            ).first()
        if existing:
            return

        limit = int(getattr(settings, "BILLING_TRIAL_MAX_MCP_SERVERS", 3))
        connected = count_connected_mcp_servers(db, user_id)
        if connected >= limit:
            raise HTTPException(status_code=402, detail="Trial plan limit reached (max connected MCP servers)")

    if server_id in _wave56_server_ids():
        plan = str(getattr(sub, "plan", "") or "").strip().lower()
        status = str(getattr(sub, "status", "") or "").strip().lower()
        if status == "trialing" or plan in {"trial", "free_trial", "trialing", ""}:
            plan = "free"
        min_plan = str(settings.WAVE56_MIN_PLAN or "professional").strip().lower()
        if _plan_rank(plan) < _plan_rank(min_plan):
            raise HTTPException(status_code=402, detail=f"{server_id} requires the {min_plan} plan")


@router.get("/servers")
def list_mcp_servers(user_id: str, db: Session = Depends(get_db)):
    registry = MCPServerRegistry()
    registry.ensure_tables(db)
    items = registry.list_servers(db, user_id=user_id)
    return {"ok": True, "items": [item.model_dump() for item in items]}


@router.post("/servers")
async def create_mcp_server(user_id: str, payload: MCPServerManifest, db: Session = Depends(get_db)):
    hub = get_mcp_client_hub()
    result = await hub.register_server(db, user_id=user_id, manifest=payload)
    return {"ok": True, **result}


@router.post("/bootstrap/wave1")
async def bootstrap_mcp_wave1(
    user_id: str,
    payload: MCPWaveBootstrapRequest,
    db: Session = Depends(get_db),
):
    # Always register servers; connecting is plan-gated.
    summary = await bootstrap_wave1_servers(
        db,
        user_id=user_id,
        transport_mode=payload.transport_mode,
        connect=False,
    )
    if payload.connect:
        sub = get_billing_subscription(db, user_id)
        ids = [spec.server_id for spec in WAVE1_SPECS]
        if _is_trial_plan(sub.plan, sub.status):
            ids = ids[: int(getattr(settings, "BILLING_TRIAL_MAX_MCP_SERVERS", 3))]
        hub = get_mcp_client_hub()
        connected = 0
        failed = 0
        for server_id in ids:
            try:
                _enforce_mcp_connect_gate(db, user_id=user_id, server_id=server_id)
                result = await hub.connect_server(db, user_id=user_id, server_id=server_id)
                if result.get("connected"):
                    connected += 1
            except Exception:
                failed += 1
        summary["connected_count"] = connected
        summary["failed_count"] = failed
    return summary


@router.post("/bootstrap/wave2")
async def bootstrap_mcp_wave2(
    user_id: str,
    payload: MCPWaveBootstrapRequest,
    db: Session = Depends(get_db),
):
    summary = await bootstrap_wave2_servers(
        db,
        user_id=user_id,
        transport_mode=payload.transport_mode,
        connect=False,
    )
    if payload.connect:
        hub = get_mcp_client_hub()
        connected = 0
        failed = 0
        for spec in WAVE2_SPECS:
            try:
                _enforce_mcp_connect_gate(db, user_id=user_id, server_id=spec.server_id)
                result = await hub.connect_server(db, user_id=user_id, server_id=spec.server_id)
                if result.get("connected"):
                    connected += 1
            except Exception:
                failed += 1
        summary["connected_count"] = connected
        summary["failed_count"] = failed
    return summary


@router.post("/bootstrap/wave3")
async def bootstrap_mcp_wave3(
    user_id: str,
    payload: MCPWaveBootstrapRequest,
    db: Session = Depends(get_db),
):
    summary = await bootstrap_wave3_servers(
        db,
        user_id=user_id,
        transport_mode=payload.transport_mode,
        connect=False,
    )
    if payload.connect:
        hub = get_mcp_client_hub()
        connected = 0
        failed = 0
        for spec in WAVE3_SPECS:
            try:
                _enforce_mcp_connect_gate(db, user_id=user_id, server_id=spec.server_id)
                result = await hub.connect_server(db, user_id=user_id, server_id=spec.server_id)
                if result.get("connected"):
                    connected += 1
            except Exception:
                failed += 1
        summary["connected_count"] = connected
        summary["failed_count"] = failed
    return summary


@router.post("/bootstrap/wave4")
async def bootstrap_mcp_wave4(
    user_id: str,
    payload: MCPWaveBootstrapRequest,
    db: Session = Depends(get_db),
):
    summary = await bootstrap_wave4_servers(
        db,
        user_id=user_id,
        transport_mode=payload.transport_mode,
        connect=False,
    )
    if payload.connect:
        hub = get_mcp_client_hub()
        connected = 0
        failed = 0
        for spec in WAVE4_SPECS:
            try:
                _enforce_mcp_connect_gate(db, user_id=user_id, server_id=spec.server_id)
                result = await hub.connect_server(db, user_id=user_id, server_id=spec.server_id)
                if result.get("connected"):
                    connected += 1
            except Exception:
                failed += 1
        summary["connected_count"] = connected
        summary["failed_count"] = failed
    return summary


@router.post("/bootstrap/wave5")
async def bootstrap_mcp_wave5(
    user_id: str,
    payload: MCPWaveBootstrapRequest,
    db: Session = Depends(get_db),
):
    summary = await bootstrap_wave5_servers(
        db,
        user_id=user_id,
        transport_mode=payload.transport_mode,
        connect=False,
    )
    if payload.connect:
        hub = get_mcp_client_hub()
        connected = 0
        failed = 0
        for spec in WAVE5_SPECS:
            try:
                _enforce_mcp_connect_gate(db, user_id=user_id, server_id=spec.server_id)
                result = await hub.connect_server(db, user_id=user_id, server_id=spec.server_id)
                if result.get("connected"):
                    connected += 1
            except Exception:
                failed += 1
        summary["connected_count"] = connected
        summary["failed_count"] = failed
    return summary


@router.post("/bootstrap/wave6")
async def bootstrap_mcp_wave6(
    user_id: str,
    payload: MCPWaveBootstrapRequest,
    db: Session = Depends(get_db),
):
    summary = await bootstrap_wave6_servers(
        db,
        user_id=user_id,
        transport_mode=payload.transport_mode,
        connect=False,
    )
    if payload.connect:
        hub = get_mcp_client_hub()
        connected = 0
        failed = 0
        for spec in WAVE6_SPECS:
            try:
                _enforce_mcp_connect_gate(db, user_id=user_id, server_id=spec.server_id)
                result = await hub.connect_server(db, user_id=user_id, server_id=spec.server_id)
                if result.get("connected"):
                    connected += 1
            except Exception:
                failed += 1
        summary["connected_count"] = connected
        summary["failed_count"] = failed
    return summary


@router.get("/servers/{server_id}")
async def get_mcp_server(server_id: str, user_id: str, db: Session = Depends(get_db)):
    registry = MCPServerRegistry()
    registry.ensure_tables(db)
    try:
        item = registry.get_server(db, server_id)
    except KeyError:
        raise HTTPException(status_code=404, detail="MCP server not found")

    hub = get_mcp_client_hub()
    health = await hub.get_health(db, server_id)
    return {
        "ok": True,
        "config": item.config.model_dump(),
        "tools": [tool.model_dump() for tool in item.tools],
        "resources": item.resources,
        "prompts": item.prompts,
        "health": health.model_dump(),
    }


@router.put("/servers/{server_id}")
async def update_mcp_server(server_id: str, user_id: str, payload: MCPServerManifest, db: Session = Depends(get_db)):
    hub = get_mcp_client_hub()
    result = await hub.update_server(db, user_id=user_id, server_id=server_id, manifest=payload)
    return {"ok": True, **result}


@router.post("/servers/{server_id}/connect")
async def connect_mcp_server(server_id: str, user_id: str, db: Session = Depends(get_db)):
    _enforce_mcp_connect_gate(db, user_id=user_id, server_id=server_id)
    hub = get_mcp_client_hub()
    return await hub.connect_server(db, user_id=user_id, server_id=server_id)


@router.post("/servers/{server_id}/disconnect")
async def disconnect_mcp_server(server_id: str, user_id: str, db: Session = Depends(get_db)):
    hub = get_mcp_client_hub()
    return await hub.disconnect_server(db, user_id=user_id, server_id=server_id)


@router.get("/servers/{server_id}/tools")
def list_mcp_server_tools(server_id: str, db: Session = Depends(get_db)):
    registry = MCPServerRegistry()
    registry.ensure_tables(db)
    try:
        item = registry.get_server(db, server_id)
    except KeyError:
        raise HTTPException(status_code=404, detail="MCP server not found")
    return {"ok": True, "tools": [t.model_dump() for t in item.tools]}


@router.get("/servers/{server_id}/resources")
async def list_mcp_server_resources(server_id: str, db: Session = Depends(get_db)):
    hub = get_mcp_client_hub()
    resources = await hub.list_resources(db, server_id=server_id)
    return {"ok": True, "resources": resources}


@router.post("/servers/{server_id}/resources/subscribe")
async def subscribe_mcp_server_resource(server_id: str, payload: MCPResourceSubscribeRequest, db: Session = Depends(get_db)):
    hub = get_mcp_client_hub()
    ok = await hub.subscribe_resource(db, server_id=server_id, uri=payload.uri)
    return {"ok": ok, "server_id": server_id, "uri": payload.uri}


@router.get("/servers/{server_id}/prompts")
async def list_mcp_server_prompts(server_id: str, db: Session = Depends(get_db)):
    hub = get_mcp_client_hub()
    prompts = await hub.list_prompts(db, server_id=server_id)
    return {"ok": True, "prompts": prompts}


@router.post("/servers/{server_id}/prompts/build")
async def build_mcp_prompt(server_id: str, payload: MCPPromptBuildRequest, db: Session = Depends(get_db)):
    hub = get_mcp_client_hub()
    merged = await hub.build_prompt_context(
        db,
        server_id=server_id,
        prompt_name=payload.name,
        prompt_args=payload.arguments,
    )
    return {"ok": True, "prompt": merged}


@router.post("/servers/{server_id}/tools/{tool_name}/execute")
async def execute_mcp_tool(server_id: str, tool_name: str, payload: MCPToolExecuteRequest, db: Session = Depends(get_db)):
    call = ToolCall(
        tool_name=f"mcp.{server_id}.{tool_name}",
        tool=f"mcp_{server_id}_{tool_name}",
        arguments=payload.arguments,
        args=payload.arguments,
        user_id=payload.user_id,
        run_id=payload.run_id,
        capability_token=payload.capability_token,
        input_provenance=ContentProvenance.USER_DIRECT,
    )
    hub = get_mcp_client_hub()
    result = await hub.call_tool(
        db,
        server_id=server_id,
        tool_name=tool_name,
        arguments=payload.arguments,
        run_context=MCPRunContext(run_id=payload.run_id, user_id=payload.user_id, provenance="user_direct"),
    )
    from app.blueprint.mcp.normalization import normalize_mcp_result

    normalized = normalize_mcp_result(result, call)
    return {
        "ok": normalized.ok,
        "tool_name": tool_name,
        "server_id": server_id,
        "output": normalized.output or {},
        "error": normalized.error,
        "latency_ms": normalized.latency_ms,
        "cost_cents": normalized.cost_cents,
    }
