from __future__ import annotations

from app.api.internal.hands import _validate_side_effect_output
from app.blueprint.capability_tokens import CapabilityViolation, enforce_capability_token, issue_capability_token
from app.blueprint.contracts import ContentProvenance
from app.blueprint.security import PrivilegeViolation, validate_tool_privilege


def test_prompt_injection_paths_block_side_effect_tools() -> None:
    blocked_tools = ["email.send", "calendar.create", "stripe.charge", "microsoft.calendar.create"]
    for tool in blocked_tools:
        try:
            validate_tool_privilege(tool_name=tool, provenance=ContentProvenance.MCP_RESULT)
            assert False, f"{tool} should be blocked for MCP provenance"
        except PrivilegeViolation:
            pass


def test_wrong_recipient_detection_guards_side_effect_output() -> None:
    mismatch = _validate_side_effect_output(
        tool="email.send",
        args={"mode": "send", "to_email": "ceo@example.com", "subject": "Status"},
        output_payload={"status": "sent", "recipient": "wrong@example.com", "subject": "Status"},
    )
    assert mismatch is not None
    assert "recipient mismatch" in str(mismatch)


def test_capability_token_blocks_privilege_escalation() -> None:
    token = issue_capability_token(
        run_id="run-redteam",
        user_id="user-redteam",
        provenance=ContentProvenance.EMAIL_BODY,
        capabilities=["calendar:read"],
    )
    try:
        enforce_capability_token(
            token=token,
            run_id="run-redteam",
            user_id="user-redteam",
            tool_name="calendar.create",
            required_capabilities=["calendar:write"],
        )
        assert False, "capability escalation should fail"
    except CapabilityViolation:
        pass

