from __future__ import annotations

from app.tasks import google_gmail_sync as gmail_sync


class _Exec:
    def __init__(self, payload=None, error: Exception | None = None):
        self._payload = payload or {}
        self._error = error

    def execute(self):
        if self._error is not None:
            raise self._error
        return self._payload


class _UsersApi:
    def __init__(
        self,
        *,
        history_payload=None,
        history_error: Exception | None = None,
        list_payload=None,
        profile_payload=None,
    ):
        self._history_payload = history_payload or {"history": [], "historyId": "0"}
        self._history_error = history_error
        self._list_payload = list_payload or {"messages": []}
        self._profile_payload = profile_payload or {"historyId": "0"}

    def messages(self):
        return self

    def history(self):
        return self

    def list(self, **kwargs):
        if "startHistoryId" in kwargs:
            if self._history_error is not None:
                return _Exec(error=self._history_error)
            return _Exec(payload=self._history_payload)
        return _Exec(payload=self._list_payload)

    def getProfile(self, **kwargs):
        return _Exec(payload=self._profile_payload)


class _Service:
    def __init__(self, users_api: _UsersApi):
        self._users_api = users_api

    def users(self):
        return self._users_api


class _DummyDB:
    def close(self):
        return None


def test_gmail_delta_sync_bootstrap(monkeypatch):
    captured: dict[str, str] = {}

    monkeypatch.setattr(gmail_sync, "SessionLocal", lambda: _DummyDB())
    monkeypatch.setattr(gmail_sync, "get_valid_google_credentials", lambda db, user_id: object())
    monkeypatch.setattr(gmail_sync, "_get_cursor", lambda db, user_id: None)
    monkeypatch.setattr(
        gmail_sync,
        "_upsert_cursor",
        lambda db, user_id, cursor_value: captured.update({"user_id": user_id, "cursor": cursor_value}),
    )

    users_api = _UsersApi(
        list_payload={"messages": [{"id": "m1"}, {"id": "m2"}]},
        profile_payload={"historyId": "200"},
    )
    monkeypatch.setattr(
        gmail_sync,
        "build",
        lambda *args, **kwargs: _Service(users_api),
    )

    out = gmail_sync.sync_google_gmail_delta.run(user_id="user-1", max_results=25)
    assert out["ok"] is True
    assert out["cursor_before"] is None
    assert out["cursor_after"] == "200"
    assert out["changed_message_ids"] == ["m1", "m2"]
    assert captured == {"user_id": "user-1", "cursor": "200"}


def test_gmail_delta_sync_falls_back_when_cursor_is_stale(monkeypatch):
    captured: dict[str, str] = {}

    monkeypatch.setattr(gmail_sync, "SessionLocal", lambda: _DummyDB())
    monkeypatch.setattr(gmail_sync, "get_valid_google_credentials", lambda db, user_id: object())
    monkeypatch.setattr(gmail_sync, "_get_cursor", lambda db, user_id: "stale-history-id")
    monkeypatch.setattr(
        gmail_sync,
        "_upsert_cursor",
        lambda db, user_id, cursor_value: captured.update({"user_id": user_id, "cursor": cursor_value}),
    )

    users_api = _UsersApi(
        history_error=RuntimeError("history id too old"),
        list_payload={"messages": [{"id": "m3"}, {"id": "m4"}, {"id": "m3"}]},
        profile_payload={"historyId": "300"},
    )
    monkeypatch.setattr(
        gmail_sync,
        "build",
        lambda *args, **kwargs: _Service(users_api),
    )

    out = gmail_sync.sync_google_gmail_delta.run(user_id="user-2", max_results=25)
    assert out["ok"] is True
    assert out["cursor_before"] == "stale-history-id"
    assert out["cursor_after"] == "300"
    assert out["changed_message_ids"] == ["m3", "m4"]
    assert captured == {"user_id": "user-2", "cursor": "300"}
