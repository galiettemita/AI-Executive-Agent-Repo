from __future__ import annotations

from app.services.provisioning_handlers import ProvisionAuthContext, get_auth_handler


def test_get_auth_handler_specialized_variants():
    context = ProvisionAuthContext(
        request_id="req-1",
        user_id="user-1",
        server_id="google-calendar-mcp",
        reason="Need calendar",
        original_task_id="run-1",
    )

    consolidated = get_auth_handler("oauth2_consolidated").begin(context)
    assert consolidated.get("auth_type") == "oauth2_consolidated"
    assert consolidated.get("status") == "awaiting_auth"

    plaid = get_auth_handler("plaid_link").begin(context)
    assert plaid.get("auth_type") == "plaid_link"
    assert plaid.get("status") == "awaiting_auth"

    tesla = get_auth_handler("tesla_sso").begin(context)
    assert tesla.get("auth_type") == "tesla_sso"
    assert tesla.get("status") == "awaiting_auth"
