from __future__ import annotations

from dataclasses import dataclass

from app.blueprint.contracts import ContentProvenance


class PrivilegeViolation(RuntimeError):
    pass


class CapabilityViolation(RuntimeError):
    pass


@dataclass(frozen=True)
class PrivilegePolicy:
    allowed_tools: set[str] | None
    can_send_email: bool = False
    can_modify_calendar: bool = False
    can_access_financial: bool = False


def _policy(allowed_tools: list[str] | str, **flags: bool) -> PrivilegePolicy:
    if allowed_tools == "*":
        allowed = None
    else:
        allowed = set(allowed_tools)
    return PrivilegePolicy(allowed_tools=allowed, **flags)


# Blueprint Section 32 + MCP addendum:
# MCP results are treated like external/untrusted content.
READ_ONLY_EXTERNAL_TOOLS = [
    "web.search",
    "calendar.list",
    "calendar.find_free_slots",
    "microsoft.calendar.list",
    "gmail.list",
    "gmail.search",
    "gmail.get",
    "microsoft.mail.list",
    "microsoft.mail.search",
    "microsoft.mail.get",
    "microsoft.contacts.search",
]


PRIVILEGE_LEVELS: dict[ContentProvenance, PrivilegePolicy] = {
    ContentProvenance.USER_DIRECT: _policy(
        "*",
        can_send_email=True,
        can_modify_calendar=True,
        can_access_financial=True,
    ),
    ContentProvenance.EMAIL_BODY: _policy(
        READ_ONLY_EXTERNAL_TOOLS,
    ),
    ContentProvenance.WEB_SEARCH: _policy(
        ["web.search"],
    ),
    ContentProvenance.CALENDAR_DESC: _policy(
        ["calendar.list", "calendar.find_free_slots"],
    ),
    ContentProvenance.DOCUMENT: _policy(
        READ_ONLY_EXTERNAL_TOOLS,
    ),
    ContentProvenance.MCP_RESULT: _policy(
        ["web.search"],
    ),
}


def _coerce_provenance(value: ContentProvenance | str) -> ContentProvenance:
    if isinstance(value, ContentProvenance):
        return value
    raw = str(value or "").strip()
    if raw == "mcp_response":
        raw = "mcp_result"
    return ContentProvenance(raw or ContentProvenance.USER_DIRECT.value)


def validate_tool_privilege(
    *,
    tool_name: str,
    provenance: ContentProvenance | str,
    required_capabilities: list[str] | None = None,
    granted_capabilities: list[str] | None = None,
) -> None:
    source = _coerce_provenance(provenance)
    policy = PRIVILEGE_LEVELS.get(source, PRIVILEGE_LEVELS[ContentProvenance.USER_DIRECT])
    allowed = policy.allowed_tools
    if allowed is not None and tool_name not in allowed:
        raise PrivilegeViolation(
            f"Tool {tool_name} not allowed for content provenance {source.value}. "
            "Potential prompt-injection path blocked."
        )

    required = set(required_capabilities or [])
    if required:
        granted = set(granted_capabilities or [])
        missing = sorted(required - granted)
        if missing:
            raise CapabilityViolation(
                f"Missing required capabilities for {tool_name}: {', '.join(missing)}"
            )
