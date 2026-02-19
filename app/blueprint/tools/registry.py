from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

from app.blueprint.contracts import RiskLevel, ToolSpec


@dataclass
class _RegistryEntry:
    spec: ToolSpec
    min_tier: int = 2
    tags: set[str] = field(default_factory=set)
    llm_name: str = ""


class ToolRegistry:
    """
    Single tool registry for all tool surfaces.

    Native connectors register here during startup.
    MCP tools will register into this same registry in Phase 3.
    """

    def __init__(self) -> None:
        self._entries: dict[str, _RegistryEntry] = {}
        self._llm_to_tool: dict[str, str] = {}

    def register(
        self,
        spec: ToolSpec,
        *,
        min_tier: int = 2,
        tags: list[str] | None = None,
        llm_name: str | None = None,
    ) -> None:
        canonical = spec.name.strip()
        if not canonical:
            raise ValueError("ToolSpec.name is required")
        fn_name = (llm_name or canonical.replace(".", "_")).strip()
        if not fn_name:
            raise ValueError("llm_name cannot be blank")
        entry = _RegistryEntry(
            spec=spec,
            min_tier=max(0, int(min_tier)),
            tags=set(tags or []),
            llm_name=fn_name,
        )
        self._entries[canonical] = entry
        self._llm_to_tool[fn_name] = canonical

    def get(self, name: str) -> ToolSpec:
        canonical = self.resolve_tool_name(name)
        entry = self._entries.get(canonical)
        if not entry:
            raise KeyError(f"Unknown tool: {name}")
        return entry.spec

    def resolve_tool_name(self, name: str) -> str:
        raw = (name or "").strip()
        if raw in self._entries:
            return raw
        return self._llm_to_tool.get(raw, raw)

    def llm_name_for(self, tool_name: str) -> str:
        canonical = self.resolve_tool_name(tool_name)
        entry = self._entries.get(canonical)
        if not entry:
            raise KeyError(f"Unknown tool: {tool_name}")
        return entry.llm_name

    def list_for_context(self, tier: int, tags: list[str] | None = None) -> list[ToolSpec]:
        tier_value = int(tier)
        tag_filter = set(tags or [])
        out: list[ToolSpec] = []
        for entry in self._entries.values():
            if tier_value < entry.min_tier:
                continue
            if tag_filter and not (entry.tags & tag_filter):
                continue
            out.append(entry.spec)
        return out

    def list_llm_tool_schemas(
        self,
        tier: int,
        tags: list[str] | None = None,
        user_id: str | None = None,
    ) -> list[dict[str, Any]]:
        schemas: list[dict[str, Any]] = []
        db = None
        resolve_prompt_content = None
        try:
            from app.db.database import SessionLocal
            from app.services.prompt_versions import resolve_prompt_content as _resolve_prompt_content

            db = SessionLocal()
            resolve_prompt_content = _resolve_prompt_content
        except Exception:
            db = None
            resolve_prompt_content = None

        try:
            for spec in self.list_for_context(tier=tier, tags=tags):
                entry = self._entries[spec.name]
                description = spec.description
                if db is not None and resolve_prompt_content is not None:
                    try:
                        description, _version_id, _status = resolve_prompt_content(
                            db,
                            user_id=user_id,
                            prompt_group=f"tool_description:{spec.name}",
                            default_content=description,
                        )
                    except Exception:
                        description = spec.description
                schemas.append(
                    {
                        "type": "function",
                        "function": {
                            "name": entry.llm_name,
                            "description": description,
                            "parameters": spec.input_schema or {"type": "object", "properties": {}},
                        },
                    }
                )
        finally:
            if db is not None:
                try:
                    db.close()
                except Exception:
                    pass
        return schemas


_REGISTRY: ToolRegistry | None = None


def _register_native_tools(registry: ToolRegistry) -> None:
    registry.register(
        ToolSpec(
            name="provision_server",
            description=(
                "Connect a new MCP server to unlock missing capabilities. "
                "Use this when a required capability is not currently connected "
                "but exists in the Available Servers catalog."
            ),
            input_schema={
                "type": "object",
                "properties": {
                    "server_id": {"type": "string", "description": "Server ID from Available Servers catalog"},
                    "reason": {"type": "string", "description": "Why this server is needed for the current task"},
                },
                "required": ["server_id", "reason"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.LOW,
            tags=["system", "provisioning"],
            is_mcp=False,
        ),
        min_tier=2,
        tags=["system", "provisioning"],
        llm_name="provision_server",
    )

    registry.register(
        ToolSpec(
            name="web.search",
            description="Search the web for recent/factual information and return relevant snippets and URLs.",
            input_schema={
                "type": "object",
                "properties": {"query": {"type": "string"}},
                "required": ["query"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.NONE,
            tags=["web", "research"],
            is_mcp=False,
        ),
        min_tier=2,
        tags=["web", "research"],
        llm_name="web_search",
    )
    registry.register(
        ToolSpec(
            name="tavily.search",
            description="Search the web using Tavily and return sources/snippets for recent facts.",
            input_schema={
                "type": "object",
                "properties": {"query": {"type": "string"}},
                "required": ["query"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.NONE,
            tags=["web", "research"],
            capability_scope=["web:search"],
            is_mcp=False,
        ),
        min_tier=2,
        tags=["web", "research"],
        llm_name="tavily_search",
    )

    registry.register(
        ToolSpec(
            name="calendar.list",
            description="List calendar events in a time range.",
            input_schema={
                "type": "object",
                "properties": {
                    "start_utc": {"type": "string", "description": "ISO-8601 UTC timestamp"},
                    "end_utc": {"type": "string", "description": "ISO-8601 UTC timestamp"},
                    "max_results": {"type": "integer", "minimum": 1, "maximum": 100},
                    "provider": {"type": "string"},
                },
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.LOW,
            tags=["calendar", "productivity"],
        ),
        min_tier=2,
        tags=["calendar", "productivity"],
    )
    registry.register(
        ToolSpec(
            name="calendar.create",
            description="Create a calendar event.",
            input_schema={
                "type": "object",
                "properties": {
                    "title": {"type": "string"},
                    "start_utc": {"type": "string", "description": "ISO-8601 UTC timestamp"},
                    "end_utc": {"type": "string", "description": "ISO-8601 UTC timestamp"},
                    "description": {"type": "string"},
                    "location": {"type": "string"},
                    "provider": {"type": "string"},
                },
                "required": ["title", "start_utc", "end_utc"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.MEDIUM,
            tags=["calendar", "productivity"],
        ),
        min_tier=2,
        tags=["calendar", "productivity"],
    )
    registry.register(
        ToolSpec(
            name="calendar.update",
            description="Update an existing calendar event.",
            input_schema={
                "type": "object",
                "properties": {
                    "event_id": {"type": "string"},
                    "title": {"type": "string"},
                    "start_utc": {"type": "string", "description": "ISO-8601 UTC timestamp"},
                    "end_utc": {"type": "string", "description": "ISO-8601 UTC timestamp"},
                    "description": {"type": "string"},
                    "location": {"type": "string"},
                    "provider": {"type": "string"},
                },
                "required": ["event_id"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.MEDIUM,
            tags=["calendar", "productivity"],
        ),
        min_tier=2,
        tags=["calendar", "productivity"],
    )
    registry.register(
        ToolSpec(
            name="calendar.delete",
            description="Delete a calendar event by ID.",
            input_schema={
                "type": "object",
                "properties": {"event_id": {"type": "string"}, "provider": {"type": "string"}},
                "required": ["event_id"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.HIGH,
            tags=["calendar", "productivity"],
        ),
        min_tier=2,
        tags=["calendar", "productivity"],
    )
    registry.register(
        ToolSpec(
            name="calendar.find_free_slots",
            description="Find available calendar time slots in a date range.",
            input_schema={
                "type": "object",
                "properties": {
                    "start_utc": {"type": "string", "description": "ISO-8601 UTC timestamp"},
                    "end_utc": {"type": "string", "description": "ISO-8601 UTC timestamp"},
                    "duration_minutes": {"type": "integer", "minimum": 5, "maximum": 240},
                    "provider": {"type": "string"},
                },
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.LOW,
            tags=["calendar", "productivity"],
        ),
        min_tier=2,
        tags=["calendar", "productivity"],
    )
    registry.register(
        ToolSpec(
            name="microsoft.calendar.list",
            description="List Microsoft calendar events in a time range.",
            input_schema={
                "type": "object",
                "properties": {
                    "start_utc": {"type": "string"},
                    "end_utc": {"type": "string"},
                    "max_results": {"type": "integer", "minimum": 1, "maximum": 100},
                },
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.LOW,
            tags=["calendar", "microsoft"],
            capability_scope=["calendar:read"],
        ),
        min_tier=2,
        tags=["calendar", "microsoft"],
    )
    registry.register(
        ToolSpec(
            name="microsoft.calendar.create",
            description="Create a Microsoft calendar event.",
            input_schema={
                "type": "object",
                "properties": {
                    "title": {"type": "string"},
                    "start_utc": {"type": "string"},
                    "end_utc": {"type": "string"},
                    "description": {"type": "string"},
                    "location": {"type": "string"},
                },
                "required": ["title", "start_utc", "end_utc"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.MEDIUM,
            tags=["calendar", "microsoft"],
            capability_scope=["calendar:write"],
        ),
        min_tier=2,
        tags=["calendar", "microsoft"],
    )
    registry.register(
        ToolSpec(
            name="microsoft.calendar.update",
            description="Update a Microsoft calendar event.",
            input_schema={
                "type": "object",
                "properties": {
                    "event_id": {"type": "string"},
                    "title": {"type": "string"},
                    "start_utc": {"type": "string"},
                    "end_utc": {"type": "string"},
                    "description": {"type": "string"},
                    "location": {"type": "string"},
                },
                "required": ["event_id"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.MEDIUM,
            tags=["calendar", "microsoft"],
            capability_scope=["calendar:write"],
        ),
        min_tier=2,
        tags=["calendar", "microsoft"],
    )
    registry.register(
        ToolSpec(
            name="microsoft.calendar.delete",
            description="Delete a Microsoft calendar event.",
            input_schema={
                "type": "object",
                "properties": {"event_id": {"type": "string"}},
                "required": ["event_id"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.HIGH,
            tags=["calendar", "microsoft"],
            capability_scope=["calendar:write"],
        ),
        min_tier=2,
        tags=["calendar", "microsoft"],
    )
    registry.register(
        ToolSpec(
            name="gmail.list",
            description="List recent Gmail messages.",
            input_schema={
                "type": "object",
                "properties": {
                    "max_results": {"type": "integer", "minimum": 1, "maximum": 50},
                    "hours_back": {"type": "integer", "minimum": 1, "maximum": 720},
                    "unread_only": {"type": "boolean"},
                },
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.LOW,
            tags=["email", "google"],
            capability_scope=["email:read"],
        ),
        min_tier=2,
        tags=["email", "google"],
    )
    registry.register(
        ToolSpec(
            name="gmail.search",
            description="Search Gmail messages by query.",
            input_schema={
                "type": "object",
                "properties": {
                    "query": {"type": "string"},
                    "max_results": {"type": "integer", "minimum": 1, "maximum": 50},
                },
                "required": ["query"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.LOW,
            tags=["email", "google"],
            capability_scope=["email:read"],
        ),
        min_tier=2,
        tags=["email", "google"],
    )
    registry.register(
        ToolSpec(
            name="gmail.get",
            description="Get a specific Gmail message by ID.",
            input_schema={
                "type": "object",
                "properties": {"message_id": {"type": "string"}, "include_body": {"type": "boolean"}},
                "required": ["message_id"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.LOW,
            tags=["email", "google"],
            capability_scope=["email:read"],
        ),
        min_tier=2,
        tags=["email", "google"],
    )
    registry.register(
        ToolSpec(
            name="email.send",
            description="Send an email (approval required for side effects).",
            input_schema={
                "type": "object",
                "properties": {
                    "to_email": {"type": "string"},
                    "subject": {"type": "string"},
                    "body_text": {"type": "string"},
                    "provider": {"type": "string", "enum": ["google", "microsoft", "ses", "smtp"]},
                    "mode": {"type": "string", "enum": ["draft", "review", "send"]},
                    "approval_token": {"type": "string"},
                    "cc": {"type": "string"},
                    "bcc": {"type": "string"},
                },
                "required": ["to_email", "subject", "body_text"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.HIGH,
            requires_approval_above=RiskLevel.LOW,
            tags=["email", "side_effect"],
            capability_scope=["email:send"],
        ),
        min_tier=2,
        tags=["email", "side_effect"],
        llm_name="email_send",
    )
    registry.register(
        ToolSpec(
            name="microsoft.mail.list",
            description="List recent Outlook emails.",
            input_schema={
                "type": "object",
                "properties": {
                    "max_results": {"type": "integer", "minimum": 1, "maximum": 50},
                    "hours_back": {"type": "integer", "minimum": 1, "maximum": 720},
                    "unread_only": {"type": "boolean"},
                },
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.LOW,
            tags=["email", "microsoft"],
            capability_scope=["email:read"],
        ),
        min_tier=2,
        tags=["email", "microsoft"],
    )
    registry.register(
        ToolSpec(
            name="microsoft.mail.search",
            description="Search Outlook email by query.",
            input_schema={
                "type": "object",
                "properties": {
                    "query": {"type": "string"},
                    "max_results": {"type": "integer", "minimum": 1, "maximum": 50},
                },
                "required": ["query"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.LOW,
            tags=["email", "microsoft"],
            capability_scope=["email:read"],
        ),
        min_tier=2,
        tags=["email", "microsoft"],
    )
    registry.register(
        ToolSpec(
            name="microsoft.mail.get",
            description="Get a specific Outlook email message by ID.",
            input_schema={
                "type": "object",
                "properties": {"message_id": {"type": "string"}, "include_body": {"type": "boolean"}},
                "required": ["message_id"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.LOW,
            tags=["email", "microsoft"],
            capability_scope=["email:read"],
        ),
        min_tier=2,
        tags=["email", "microsoft"],
    )
    registry.register(
        ToolSpec(
            name="microsoft.contacts.search",
            description="Search Microsoft 365 contacts by name/email.",
            input_schema={
                "type": "object",
                "properties": {
                    "query": {"type": "string"},
                    "max_results": {"type": "integer", "minimum": 1, "maximum": 50},
                },
                "required": ["query"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.LOW,
            tags=["contacts", "microsoft"],
            capability_scope=["contacts:read"],
        ),
        min_tier=2,
        tags=["contacts", "microsoft"],
    )
    registry.register(
        ToolSpec(
            name="slack.messages.list",
            description="List recent messages in a Slack channel.",
            input_schema={
                "type": "object",
                "properties": {
                    "channel_id": {"type": "string"},
                    "limit": {"type": "integer", "minimum": 1, "maximum": 200},
                },
                "required": ["channel_id"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.LOW,
            tags=["slack", "communication"],
            capability_scope=["slack:read"],
        ),
        min_tier=2,
        tags=["slack", "communication"],
        llm_name="slack_messages_list",
    )
    registry.register(
        ToolSpec(
            name="slack.messages.send",
            description="Send a Slack message to a channel.",
            input_schema={
                "type": "object",
                "properties": {
                    "channel_id": {"type": "string"},
                    "text": {"type": "string"},
                    "thread_ts": {"type": "string"},
                },
                "required": ["channel_id", "text"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.HIGH,
            requires_approval_above=RiskLevel.LOW,
            tags=["slack", "communication", "side_effect"],
            capability_scope=["slack:send"],
        ),
        min_tier=2,
        tags=["slack", "communication", "side_effect"],
        llm_name="slack_messages_send",
    )
    registry.register(
        ToolSpec(
            name="slack.channel.summary",
            description="Summarize recent activity in a Slack channel.",
            input_schema={
                "type": "object",
                "properties": {
                    "channel_id": {"type": "string"},
                    "limit": {"type": "integer", "minimum": 1, "maximum": 200},
                },
                "required": ["channel_id"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.LOW,
            tags=["slack", "communication", "summary"],
            capability_scope=["slack:read"],
        ),
        min_tier=2,
        tags=["slack", "communication", "summary"],
        llm_name="slack_channel_summary",
    )
    registry.register(
        ToolSpec(
            name="plaid.accounts.list",
            description="List linked Plaid accounts (sandbox-first in Phase 3).",
            input_schema={
                "type": "object",
                "properties": {"stage": {"type": "string", "enum": ["staging", "prod"]}},
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.LOW,
            tags=["plaid", "finance"],
            capability_scope=["finance:read"],
        ),
        min_tier=2,
        tags=["plaid", "finance"],
        llm_name="plaid_accounts_list",
    )
    registry.register(
        ToolSpec(
            name="plaid.transactions.list",
            description="List recent Plaid transactions.",
            input_schema={
                "type": "object",
                "properties": {
                    "start_date": {"type": "string"},
                    "end_date": {"type": "string"},
                    "stage": {"type": "string", "enum": ["staging", "prod"]},
                },
                "required": ["start_date", "end_date"],
            },
            output_schema={"type": "object"},
            risk_level=RiskLevel.LOW,
            tags=["plaid", "finance"],
            capability_scope=["finance:read"],
        ),
        min_tier=2,
        tags=["plaid", "finance"],
        llm_name="plaid_transactions_list",
    )


def get_tool_registry() -> ToolRegistry:
    global _REGISTRY
    if _REGISTRY is None:
        _REGISTRY = ToolRegistry()
        _register_native_tools(_REGISTRY)
    return _REGISTRY
