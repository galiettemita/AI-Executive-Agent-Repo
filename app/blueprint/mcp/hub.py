from __future__ import annotations

import asyncio
from contextlib import nullcontext
import logging
import time
from dataclasses import dataclass, field
from datetime import datetime
from typing import Any

from sqlalchemy.orm import Session

from app.blueprint.contracts import RiskLevel, ToolCall, ToolResult
from app.blueprint.knowledge_files import get_latest_knowledge_file, put_knowledge_file_version
from app.blueprint.mcp.contracts import (
    MCPContentBlock,
    MCPRunContext,
    MCPServerConfig,
    MCPServerHealth,
    MCPServerManifest,
    MCPToolResult,
    MCPToolSchema,
)
from app.blueprint.mcp.costing import MCPCostTracker, MCPRateLimiter
from app.blueprint.mcp.mock_servers import dispatch_mock_tool, mock_tools_for
from app.blueprint.mcp.normalization import classify_mcp_tool_risk, normalize_mcp_result, normalize_mcp_tool
from app.blueprint.mcp.registry import MCPServerRegistry
from app.blueprint.mcp.transports import MCPServerError, MCPTransport, MCPTransportError, create_transport
from app.blueprint.tools import get_tool_registry
from app.core.config import settings
from app.db.database import SessionLocal

logger = logging.getLogger(__name__)


def _mcp_span(name: str):
    try:
        from opentelemetry import trace

        tracer = trace.get_tracer("executive-os.mcp")
        return tracer.start_as_current_span(name)
    except Exception:
        return nullcontext()


@dataclass
class MCPServerConnection:
    config: MCPServerConfig
    transport: MCPTransport | None = None
    tools: list[MCPToolSchema] = field(default_factory=list)
    resources: list[dict[str, Any]] = field(default_factory=list)
    prompts: list[dict[str, Any]] = field(default_factory=list)
    is_connected: bool = False
    last_ping_at: datetime | None = None
    error_count: int = 0

    @property
    def is_mock(self) -> bool:
        cmd = (self.config.transport.command or "").strip().lower()
        return cmd.startswith("mock://")


class MCPBudgetExceeded(RuntimeError):
    pass


class MCPRateLimited(RuntimeError):
    pass


class MCPClientHub:
    def __init__(self) -> None:
        self._connections: dict[str, MCPServerConnection] = {}
        self._registry = MCPServerRegistry()
        self._cost_tracker = MCPCostTracker()
        self._rate_limiter = MCPRateLimiter()
        self._health_task: asyncio.Task | None = None
        self._lock = asyncio.Lock()
        self._tool_cache: dict[str, tuple[float, MCPToolResult]] = {}
        self._resource_cache: dict[str, tuple[float, list[dict[str, Any]]]] = {}
        self._prompt_cache: dict[str, tuple[float, list[dict[str, Any]]]] = {}
        self._reconnect_after: dict[str, float] = {}

    def flush_caches(self) -> dict[str, int]:
        tool_count = len(self._tool_cache)
        resource_count = len(self._resource_cache)
        prompt_count = len(self._prompt_cache)
        self._tool_cache.clear()
        self._resource_cache.clear()
        self._prompt_cache.clear()
        return {
            "tool_cache_entries": tool_count,
            "resource_cache_entries": resource_count,
            "prompt_cache_entries": prompt_count,
        }

    async def initialize(self, db: Session) -> None:
        self._registry.ensure_tables(db)
        if self._health_task is None or self._health_task.done():
            self._health_task = asyncio.create_task(self._health_loop())

    async def register_server(self, db: Session, *, user_id: str, manifest: MCPServerManifest) -> dict[str, Any]:
        config = self._registry.upsert_server(db, manifest)
        self._registry.bind_user_server(db, user_id=user_id, server_id=config.server_id)

        conn = await self._connect_or_refresh(db, config)
        self._register_tools_in_registry(conn)
        self._registry.set_server_capabilities(
            db,
            server_id=config.server_id,
            tools=conn.tools,
            resources=conn.resources,
            prompts=conn.prompts,
            state="approved",
            health_status="healthy" if conn.is_connected else "unhealthy",
        )
        self._refresh_tools_knowledge_file(db, user_id=user_id)
        return {
            "server_id": config.server_id,
            "state": "approved" if conn.is_connected else "registered",
            "tools_count": len(conn.tools),
            "resources_count": len(conn.resources),
            "prompts_count": len(conn.prompts),
            "connected": conn.is_connected,
        }

    async def update_server(self, db: Session, *, user_id: str, server_id: str, manifest: MCPServerManifest) -> dict[str, Any]:
        # Preserve server_id from path to avoid accidental rename drift.
        manifest = manifest.model_copy(update={"server_id": server_id})
        return await self.register_server(db, user_id=user_id, manifest=manifest)

    async def connect_server(self, db: Session, *, user_id: str, server_id: str) -> dict[str, Any]:
        config = self._registry.get_server_config(db, server_id)
        self._registry.bind_user_server(db, user_id=user_id, server_id=server_id)
        conn = await self._connect_or_refresh(db, config)
        self._register_tools_in_registry(conn)
        self._registry.set_server_capabilities(
            db,
            server_id=server_id,
            tools=conn.tools,
            resources=conn.resources,
            prompts=conn.prompts,
            state="approved" if conn.is_connected else "registered",
            health_status="healthy" if conn.is_connected else "unhealthy",
        )
        self._refresh_tools_knowledge_file(db, user_id=user_id)
        return {"ok": True, "server_id": server_id, "connected": conn.is_connected}

    async def disconnect_server(self, db: Session, *, user_id: str, server_id: str) -> dict[str, Any]:
        async with self._lock:
            conn = self._connections.get(server_id)
            if conn and conn.transport:
                try:
                    await conn.transport.close()
                except Exception:
                    pass
            self._connections.pop(server_id, None)
        self._registry.set_health(db, server_id=server_id, state="registered", is_healthy=False)
        self._refresh_tools_knowledge_file(db, user_id=user_id)
        return {"ok": True, "server_id": server_id, "connected": False}

    async def call_tool(
        self,
        db: Session,
        *,
        server_id: str,
        tool_name: str,
        arguments: dict[str, Any],
        run_context: MCPRunContext,
    ) -> MCPToolResult:
        with _mcp_span("mcp.tool.call") as span:
            if span:
                span.set_attribute("mcp.server_id", server_id)
                span.set_attribute("mcp.tool_name", tool_name)
                span.set_attribute("mcp.run_id", str(run_context.run_id or ""))
            config = self._registry.get_server_config(db, server_id)
            conn = await self._connect_or_refresh(db, config)
            self._validate_sandbox(conn, tool_name, arguments)

            cache_key = f"{server_id}:{tool_name}:{json_dumps(arguments or {})}"
            if settings.SEMANTIC_CACHE_ENABLED:
                cached = self._tool_cache.get(cache_key)
                if cached and cached[0] > time.time():
                    if span:
                        span.set_attribute("mcp.cache_hit", True)
                    return cached[1]

            allowed = self._rate_limiter.allow(
                server_id=server_id,
                user_id=run_context.user_id,
                per_min=config.rate_limit_per_min,
            )
            if not allowed:
                if span:
                    span.set_attribute("mcp.rate_limited", True)
                raise MCPRateLimited(f"MCP server rate limit exceeded for {server_id}")

            spent = self._cost_tracker.get_daily_server_cost(server_id)
            if spent >= float(config.daily_budget_cents):
                if span:
                    span.set_attribute("mcp.budget_exceeded", True)
                raise MCPBudgetExceeded(f"MCP daily budget exceeded for {server_id}")

            started = time.perf_counter()
            try:
                if conn.is_mock:
                    result = await dispatch_mock_tool(server_id, tool_name, arguments)
                else:
                    result_payload = await self._call_transport_tool(
                        conn,
                        tool_name,
                        arguments,
                        run_context=run_context,
                    )
                    result = self._payload_to_result(server_id, result_payload)
            except Exception as exc:
                self._registry.set_health(db, server_id=server_id, state="approved", is_healthy=False, total_calls_delta=1, total_errors_delta=1)
                if span:
                    span.set_attribute("mcp.error", True)
                    span.set_attribute("mcp.error_type", exc.__class__.__name__)
                raise

            result.latency_ms = int((time.perf_counter() - started) * 1000)
            result.cost_cents = self._cost_tracker.record(
                server_id=server_id,
                user_id=run_context.user_id,
                run_id=run_context.run_id,
                latency_ms=result.latency_ms,
            )
            if settings.SEMANTIC_CACHE_ENABLED and not result.is_error:
                ttl = max(60, int(settings.SEMANTIC_CACHE_TTL_SECONDS))
                self._tool_cache[cache_key] = (time.time() + ttl, result)
            self._registry.set_health(
                db,
                server_id=server_id,
                state="approved",
                is_healthy=not result.is_error,
                total_calls_delta=1,
                total_errors_delta=1 if result.is_error else 0,
                total_cost_delta=result.cost_cents,
            )
            if span:
                span.set_attribute("mcp.latency_ms", result.latency_ms)
                span.set_attribute("mcp.cost_cents", float(result.cost_cents or 0))
                span.set_attribute("mcp.is_error", bool(result.is_error))
            return result

    async def list_resources(self, db: Session, *, server_id: str) -> list[dict[str, Any]]:
        cache_key = f"{server_id}:resources"
        cached = self._resource_cache.get(cache_key)
        if cached and cached[0] > time.time():
            return cached[1]

        config = self._registry.get_server_config(db, server_id)
        conn = await self._connect_or_refresh(db, config)
        if conn.is_mock:
            resources: list[dict[str, Any]] = conn.resources
        else:
            payload = await self._safe_request(conn.transport, "resources/list", {}) if conn.transport else {}
            resources = [item for item in (payload.get("resources") or []) if isinstance(item, dict)]
            conn.resources = resources
        self._resource_cache[cache_key] = (time.time() + 120, resources)
        return resources

    async def subscribe_resource(self, db: Session, *, server_id: str, uri: str) -> bool:
        config = self._registry.get_server_config(db, server_id)
        conn = await self._connect_or_refresh(db, config)
        if conn.is_mock:
            return True
        if not conn.transport:
            return False
        payload = {"uri": uri}
        try:
            await conn.transport.send_request("resources/subscribe", payload)
            return True
        except Exception:
            return False

    async def list_prompts(self, db: Session, *, server_id: str) -> list[dict[str, Any]]:
        cache_key = f"{server_id}:prompts"
        cached = self._prompt_cache.get(cache_key)
        if cached and cached[0] > time.time():
            return cached[1]

        config = self._registry.get_server_config(db, server_id)
        conn = await self._connect_or_refresh(db, config)
        if conn.is_mock:
            prompts = conn.prompts
        else:
            payload = await self._safe_request(conn.transport, "prompts/list", {}) if conn.transport else {}
            prompts = [item for item in (payload.get("prompts") or []) if isinstance(item, dict)]
            conn.prompts = prompts
        self._prompt_cache[cache_key] = (time.time() + 120, prompts)
        return prompts

    async def build_prompt_context(
        self,
        db: Session,
        *,
        server_id: str,
        prompt_name: str,
        prompt_args: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        config = self._registry.get_server_config(db, server_id)
        conn = await self._connect_or_refresh(db, config)
        if conn.is_mock or not conn.transport:
            return {
                "name": prompt_name,
                "messages": [],
                "source": server_id,
            }
        payload = await self._safe_request(
            conn.transport,
            "prompts/get",
            {"name": prompt_name, "arguments": prompt_args or {}},
        )
        messages = payload.get("messages") or []
        return {
            "name": prompt_name,
            "messages": messages if isinstance(messages, list) else [],
            "source": server_id,
        }

    async def get_health(self, db: Session, server_id: str) -> MCPServerHealth:
        row = db.execute(
            # Works for both postgres and sqlite because selected columns exist in both variants.
            # pylint: disable=line-too-long
            __import__("sqlalchemy").text(
                "select server_id, state, health_status, total_calls, total_cost_cents, consecutive_failures, last_health_check_at from mcp_servers where server_id=:server_id"
            ),
            {"server_id": server_id},
        ).mappings().first()
        if not row:
            raise KeyError(f"Unknown MCP server: {server_id}")
        return MCPServerHealth(
            server_id=str(row.get("server_id")),
            state=str(row.get("state") or "registered"),
            is_healthy=str(row.get("health_status") or "unknown") == "healthy",
            total_calls_24h=int(row.get("total_calls") or 0),
            total_cost_24h_cents=float(row.get("total_cost_cents") or 0),
            consecutive_failures=int(row.get("consecutive_failures") or 0),
            last_error_at=row.get("last_health_check_at"),
        )

    async def _call_transport_tool(
        self,
        conn: MCPServerConnection,
        tool_name: str,
        arguments: dict[str, Any],
        *,
        run_context: MCPRunContext,
    ) -> dict[str, Any]:
        if not conn.transport:
            raise MCPTransportError("MCP transport not initialized")

        payload_args = dict(arguments or {})
        # Reserved context fields for self-hosted MCP runtimes.
        if run_context.user_id and "_eo_user_id" not in payload_args:
            payload_args["_eo_user_id"] = str(run_context.user_id)
        if run_context.run_id and "_eo_run_id" not in payload_args:
            payload_args["_eo_run_id"] = str(run_context.run_id)
        if run_context.provenance and "_eo_provenance" not in payload_args:
            payload_args["_eo_provenance"] = str(run_context.provenance)

        try:
            return await conn.transport.send_request("tools/call", {"name": tool_name, "arguments": payload_args})
        except MCPServerError:
            # Some implementations still expose tool/call naming.
            return await conn.transport.send_request("tool/call", {"name": tool_name, "arguments": payload_args})

    def _payload_to_result(self, server_id: str, payload: dict[str, Any]) -> MCPToolResult:
        content_blocks = payload.get("content")
        blocks: list[MCPContentBlock] = []
        if isinstance(content_blocks, list):
            for item in content_blocks:
                if isinstance(item, dict):
                    try:
                        blocks.append(MCPContentBlock(**item))
                    except Exception:
                        continue

        if not blocks:
            text_payload = payload.get("text")
            if text_payload is None:
                text_payload = json_dumps(payload)
            blocks = [MCPContentBlock(type="text", text=str(text_payload))]

        return MCPToolResult(
            content=blocks,
            is_error=bool(payload.get("isError") or payload.get("is_error") or False),
            server_id=server_id,
        )

    def _validate_sandbox(self, conn: MCPServerConnection, tool_name: str, arguments: dict[str, Any]) -> None:
        known_tools = {item.name for item in conn.tools}
        if known_tools and tool_name not in known_tools:
            raise MCPTransportError(f"Tool {tool_name} is not declared by server {conn.config.server_id}")

        blocked_keys = {"system_prompt", "shell", "command", "exec"}
        if any(k in blocked_keys for k in (arguments or {}).keys()):
            raise MCPTransportError("Blocked MCP argument key requested by sandbox policy")
        self._validate_sampling_request(arguments or {})

    def _validate_sampling_request(self, arguments: dict[str, Any]) -> None:
        sampling = arguments.get("sampling")
        if sampling is None:
            return
        if not isinstance(sampling, dict):
            raise MCPTransportError("sampling payload must be an object")
        max_tokens = int(sampling.get("max_tokens") or 0)
        if max_tokens > 2048:
            raise MCPTransportError("sampling.max_tokens exceeds policy limit (2048)")
        prompt_text = str(sampling.get("prompt") or "")
        sensitive_markers = ("api_key", "secret", "private_key", "authorization:")
        lowered = prompt_text.lower()
        if any(marker in lowered for marker in sensitive_markers):
            raise MCPTransportError("sampling payload appears to include sensitive material")

    async def _connect_or_refresh(self, db: Session, config: MCPServerConfig) -> MCPServerConnection:
        async with self._lock:
            existing = self._connections.get(config.server_id)
            if existing and existing.is_connected:
                return existing

            conn = MCPServerConnection(config=config)
            if conn.is_mock:
                conn.tools = mock_tools_for(config.server_id)
                conn.resources = []
                conn.prompts = []
                conn.is_connected = True
                conn.last_ping_at = datetime.utcnow()
                self._connections[config.server_id] = conn
                return conn

            transport = create_transport(config.transport)
            conn.transport = transport
            await transport.connect()

            init_payload = await transport.send_request(
                "initialize",
                {
                    "protocolVersion": "2025-03-26",
                    "capabilities": {
                        "tools": {},
                        "resources": {"subscribe": True},
                        "prompts": {},
                    },
                    "clientInfo": {
                        "name": "executive-os-mcp-client",
                        "version": "4.0.0",
                    },
                },
            )
            _ = init_payload
            try:
                await transport.send_notification("notifications/initialized", {})
            except Exception:
                pass

            tools_payload = await self._safe_request(transport, "tools/list", {})
            conn.tools = [
                MCPToolSchema(**item)
                for item in (tools_payload.get("tools") or [])
                if isinstance(item, dict)
            ]

            resources_payload = await self._safe_request(transport, "resources/list", {})
            conn.resources = [item for item in (resources_payload.get("resources") or []) if isinstance(item, dict)]

            prompts_payload = await self._safe_request(transport, "prompts/list", {})
            conn.prompts = [item for item in (prompts_payload.get("prompts") or []) if isinstance(item, dict)]

            conn.is_connected = True
            conn.last_ping_at = datetime.utcnow()
            self._connections[config.server_id] = conn
            return conn

    async def _safe_request(self, transport: MCPTransport, method: str, params: dict[str, Any]) -> dict[str, Any]:
        try:
            return await transport.send_request(method, params)
        except Exception:
            return {}

    def _register_tools_in_registry(self, conn: MCPServerConnection) -> None:
        registry = get_tool_registry()
        for tool in conn.tools:
            risk = classify_mcp_tool_risk(tool.name, tool.annotations)
            spec = normalize_mcp_tool(
                server_id=conn.config.server_id,
                mcp_tool=tool,
                risk=risk,
                server_config=conn.config,
            )
            llm_name = spec.name.replace(".", "_")
            registry.register(
                spec,
                min_tier=2,
                tags=spec.tags,
                llm_name=llm_name,
            )

    def _refresh_tools_knowledge_file(self, db: Session, *, user_id: str) -> None:
        try:
            servers = self._registry.list_servers(db, user_id=user_id)
            lines = ["# TOOLS.md", "## Connected Services", ""]
            if not servers:
                lines.append("- No MCP servers connected.")
            else:
                for server in servers:
                    lines.append(
                        f"- {server.display_name} ({server.server_id}) — state: {server.state}, tools: {server.tools_count}"
                    )

            lines.extend(["", "## Tool Preferences", "-", "", "## Cost Limits", "-"])
            content = "\n".join(lines)
            latest = get_latest_knowledge_file(db, user_id=user_id, file_path="TOOLS.md")
            if latest and str(latest.get("content") or "").strip() == content.strip():
                return
            put_knowledge_file_version(
                db,
                user_id=user_id,
                file_path="TOOLS.md",
                content=content,
                metadata={"source": "mcp_registry_refresh"},
            )
        except RuntimeError as exc:
            if "knowledge_files table does not exist" in str(exc).lower():
                logger.info("Skipping TOOLS.md refresh because knowledge_files table is unavailable")
            else:
                logger.warning("Skipping TOOLS.md refresh during MCP registry update", exc_info=True)
        except Exception:
            logger.warning("Skipping TOOLS.md refresh during MCP registry update", exc_info=True)

    async def _health_loop(self) -> None:
        while True:
            await asyncio.sleep(30)
            try:
                for server_id, conn in list(self._connections.items()):
                    db = SessionLocal()
                    try:
                        if conn.is_mock:
                            self._registry.set_health(db, server_id=server_id, state="approved", is_healthy=True)
                            continue
                        if not conn.transport:
                            continue
                        healthy = True
                        try:
                            await conn.transport.send_request("tools/list", {})
                        except Exception:
                            healthy = False
                        self._registry.set_health(db, server_id=server_id, state="approved", is_healthy=healthy)
                        if healthy:
                            conn.error_count = 0
                            self._reconnect_after.pop(server_id, None)
                            continue

                        conn.error_count += 1
                        now_ts = time.monotonic()
                        next_allowed = float(self._reconnect_after.get(server_id, 0.0))
                        if now_ts < next_allowed:
                            continue
                        backoff_seconds = min(300, 2 ** min(conn.error_count, 8))
                        self._reconnect_after[server_id] = now_ts + backoff_seconds
                        try:
                            if conn.transport:
                                await conn.transport.close()
                        except Exception:
                            pass
                        self._connections.pop(server_id, None)
                        try:
                            await self._connect_or_refresh(db, conn.config)
                            conn.error_count = 0
                            self._registry.set_health(db, server_id=server_id, state="approved", is_healthy=True)
                            self._reconnect_after.pop(server_id, None)
                        except Exception:
                            # keep degraded state; next retry controlled by backoff
                            pass
                    finally:
                        try:
                            db.close()
                        except Exception:
                            pass
            except Exception:
                logger.exception("MCP health loop iteration failed")


def json_dumps(payload: dict[str, Any]) -> str:
    import json

    return json.dumps(payload, ensure_ascii=False)


_HUB: MCPClientHub | None = None


def get_mcp_client_hub() -> MCPClientHub:
    global _HUB
    if _HUB is None:
        _HUB = MCPClientHub()
    return _HUB


async def invoke_mcp_tool(
    db: Session,
    *,
    spec_server_id: str,
    call: ToolCall,
) -> ToolResult:
    hub = get_mcp_client_hub()
    await hub.initialize(db)
    tool_name = call.tool_name.split(".")[-1]
    result = await hub.call_tool(
        db,
        server_id=spec_server_id,
        tool_name=tool_name,
        arguments=call.arguments,
        run_context=MCPRunContext(
            run_id=call.run_id,
            user_id=call.user_id,
            provenance=call.input_provenance.value,
        ),
    )
    return normalize_mcp_result(result, call)
