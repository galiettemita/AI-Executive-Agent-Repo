from __future__ import annotations

from datetime import datetime
from enum import Enum
from typing import Any, Literal

from pydantic import BaseModel, Field
from pydantic.config import ConfigDict


class MCPStrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)


class MCPTransportType(str, Enum):
    STREAMABLE_HTTP = "streamable_http"
    SSE = "sse"
    STDIO = "stdio"


class MCPTransportConfig(MCPStrictModel):
    type: MCPTransportType = MCPTransportType.STREAMABLE_HTTP
    url: str | None = None
    headers: dict[str, str] = Field(default_factory=dict)
    command: str | None = None
    args: list[str] = Field(default_factory=list)
    env: dict[str, str] = Field(default_factory=dict)
    timeout_ms: int = 10000


class MCPServerConfig(MCPStrictModel):
    server_id: str
    display_name: str
    description: str | None = None
    transport: MCPTransportConfig
    tags: list[str] = Field(default_factory=list)
    expected_tools: list[str] = Field(default_factory=list)
    expected_resources: list[str] = Field(default_factory=list)
    expected_prompts: list[str] = Field(default_factory=list)
    rate_limit_per_min: int = 60
    daily_budget_cents: int = 500
    state: str = "registered"


class MCPToolSchema(MCPStrictModel):
    name: str
    description: str | None = None
    inputSchema: dict[str, Any] = Field(default_factory=dict)
    annotations: dict[str, Any] | None = None


class MCPResourceSchema(MCPStrictModel):
    uri: str
    name: str
    description: str | None = None
    mimeType: str | None = None


class MCPPromptSchema(MCPStrictModel):
    name: str
    description: str | None = None
    arguments: list[dict[str, Any]] | None = None


class MCPResourceContent(MCPStrictModel):
    uri: str
    text: str | None = None
    mimeType: str | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)


class MCPContentBlock(MCPStrictModel):
    type: Literal["text", "image", "resource"]
    text: str | None = None
    data: str | None = None
    mimeType: str | None = None
    resource: MCPResourceContent | None = None


class MCPToolResult(MCPStrictModel):
    content: list[MCPContentBlock] = Field(default_factory=list)
    is_error: bool = False
    latency_ms: int = 0
    cost_cents: float = 0
    server_id: str = ""


class MCPServerHealth(MCPStrictModel):
    server_id: str
    state: str
    is_healthy: bool
    latency_p50_ms: int = 0
    latency_p95_ms: int = 0
    error_rate_1h: float = 0
    total_calls_24h: int = 0
    total_cost_24h_cents: float = 0
    last_error_at: datetime | None = None
    consecutive_failures: int = 0


class MCPServerManifest(MCPStrictModel):
    server_id: str
    display_name: str
    description: str | None = None
    transport: MCPTransportConfig
    tags: list[str] = Field(default_factory=list)
    expected_tools: list[str] = Field(default_factory=list)
    expected_resources: list[str] = Field(default_factory=list)
    expected_prompts: list[str] = Field(default_factory=list)
    rate_limit_per_min: int = 60
    daily_budget_cents: int = 500


class MCPRunContext(MCPStrictModel):
    run_id: str | None = None
    user_id: str | None = None
    provenance: str = "user_direct"


class MCPServerSummary(MCPStrictModel):
    server_id: str
    display_name: str
    state: str
    tools_count: int = 0
    resources_count: int = 0
    prompts_count: int = 0
    health: MCPServerHealth | None = None


class MCPToolExecuteRequest(MCPStrictModel):
    user_id: str
    run_id: str | None = None
    tool_name: str
    arguments: dict[str, Any] = Field(default_factory=dict)
    capability_token: str | None = None


class MCPToolExecuteResponse(MCPStrictModel):
    ok: bool
    tool_name: str
    server_id: str
    output: dict[str, Any] = Field(default_factory=dict)
    error: dict[str, Any] | None = None
    latency_ms: int = 0
    cost_cents: float = 0
