from __future__ import annotations

import pytest

from app.blueprint.capability_tokens import issue_capability_token
from app.blueprint.contracts import ContentProvenance
from app.blueprint.security import CapabilityViolation, PrivilegeViolation, validate_tool_privilege


@pytest.mark.parametrize(
    "provenance,tool_name",
    [
        (ContentProvenance.WEB_SEARCH, "calendar.create"),
        (ContentProvenance.EMAIL_BODY, "email.send"),
        (ContentProvenance.MCP_RESULT, "calendar.delete"),
        (ContentProvenance.DOCUMENT, "email.send"),
    ],
)
def test_prompt_injection_guard_blocks_side_effect_tools(provenance: ContentProvenance, tool_name: str) -> None:
    with pytest.raises(PrivilegeViolation):
        validate_tool_privilege(
            tool_name=tool_name,
            provenance=provenance,
            required_capabilities=[],
            granted_capabilities=[],
        )


def test_capability_token_missing_required_capability() -> None:
    token = issue_capability_token(
        run_id="run-1",
        user_id="user-1",
        provenance=ContentProvenance.USER_DIRECT,
        capabilities=["email:read"],
    )
    with pytest.raises(CapabilityViolation):
        validate_tool_privilege(
            tool_name="email.send",
            provenance=ContentProvenance.USER_DIRECT,
            required_capabilities=["email:send"],
            granted_capabilities=["email:read"],
        )
    assert token
