from __future__ import annotations

from app.services import provisioning_sessions


def test_delete_provisioning_sessions_for_user_memory(monkeypatch):
    monkeypatch.setattr(provisioning_sessions, "get_redis", lambda: None)

    token1 = provisioning_sessions.create_provisioning_session(
        {"user_id": "session-user", "server_id": "duffel-mcp", "request_id": "r1"},
        ttl_seconds=300,
    )
    token2 = provisioning_sessions.create_provisioning_session(
        {"user_id": "session-user", "server_id": "zoom-mcp", "request_id": "r2"},
        ttl_seconds=300,
    )
    token3 = provisioning_sessions.create_provisioning_session(
        {"user_id": "other-user", "server_id": "zoom-mcp", "request_id": "r3"},
        ttl_seconds=300,
    )

    assert provisioning_sessions.get_provisioning_session(token1) is not None
    assert provisioning_sessions.get_provisioning_session(token2) is not None
    assert provisioning_sessions.get_provisioning_session(token3) is not None

    deleted = provisioning_sessions.delete_provisioning_sessions_for_user("session-user")
    assert deleted >= 2
    assert provisioning_sessions.get_provisioning_session(token1) is None
    assert provisioning_sessions.get_provisioning_session(token2) is None
    assert provisioning_sessions.get_provisioning_session(token3) is not None
