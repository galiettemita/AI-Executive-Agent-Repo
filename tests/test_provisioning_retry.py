from __future__ import annotations

from app.blueprint.contracts import ServerProvisionedEvent
from app.blueprint.progress import get_run_result
from app.services import provisioning_retry


def test_retry_original_task_after_provisioning_sets_prefixed_run_result(monkeypatch):
    monkeypatch.setattr(
        provisioning_retry,
        "_load_original_task_context",
        lambda db, user_id, run_id: {
            "text": "Book me a flight to NYC",
            "tier": 2,
            "conversation_id": "conv-123",
        },
    )
    monkeypatch.setattr(
        provisioning_retry,
        "generate_reply",
        lambda **kwargs: ("I found flight options for you.", {"tier": 2, "provider": "openai"}),
    )

    event = ServerProvisionedEvent(
        request_id="req-123",
        user_id="user-123",
        server_id="duffel-mcp",
        original_task_id="run-retry-123",
        connected_tools=["duffel.search_flights"],
    )
    result = provisioning_retry.retry_original_task_after_provisioning(event)
    assert result.get("ok") is True
    assert "Server connected! ✓" in str(result.get("reply") or "")

    run_payload = get_run_result("run-retry-123") or {}
    assert run_payload.get("ok") is True
    assert "Server connected! ✓" in str(run_payload.get("reply") or "")


def test_retry_original_task_requires_original_task_id():
    event = ServerProvisionedEvent(
        request_id="req-no-task",
        user_id="user-123",
        server_id="duffel-mcp",
        original_task_id=None,
        connected_tools=[],
    )
    result = provisioning_retry.retry_original_task_after_provisioning(event)
    assert result.get("ok") is False
    assert result.get("reason") == "missing_original_task_id"
