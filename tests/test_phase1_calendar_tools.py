from __future__ import annotations

from datetime import datetime, timezone

from fastapi.testclient import TestClient

from app.planes.hands_app import app


client = TestClient(app)


def _tool_payload(tool_name: str, args: dict):
    return {
        "tool_name": tool_name,
        "tool": tool_name,
        "arguments": args,
        "args": args,
        "user_id": "test-user",
    }


def test_calendar_list_tool_executes(monkeypatch):
    from app.api.internal import hands as hands_mod

    def _fake_list_events(**kwargs):
        return [
            {
                "id": "evt_1",
                "summary": "Team sync",
                "start": {"dateTime": datetime.now(timezone.utc).isoformat()},
                "end": {"dateTime": datetime.now(timezone.utc).isoformat()},
            }
        ]

    monkeypatch.setattr(hands_mod, "calendar_list_events", _fake_list_events)

    resp = client.post(
        "/internal/hands/execute",
        json=_tool_payload(
            "calendar.list",
            {"start_utc": "2026-02-16T10:00:00Z", "end_utc": "2026-02-16T18:00:00Z"},
        ),
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["ok"] is True
    assert body["tool_name"] == "calendar.list"
    assert isinstance(body["output"]["events"], list)


def test_calendar_find_free_slots_executes(monkeypatch):
    from app.api.internal import hands as hands_mod

    def _fake_find_slots(**kwargs):
        return [{"start": "2026-02-16T12:00:00+00:00", "end": "2026-02-16T12:30:00+00:00"}]

    monkeypatch.setattr(hands_mod, "calendar_find_free_slots", _fake_find_slots)

    resp = client.post(
        "/internal/hands/execute",
        json=_tool_payload(
            "calendar.find_free_slots",
            {"start_utc": "2026-02-16T10:00:00Z", "end_utc": "2026-02-16T18:00:00Z", "duration_minutes": 30},
        ),
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["ok"] is True
    assert body["tool_name"] == "calendar.find_free_slots"
    assert isinstance(body["output"]["slots"], list)


def test_calendar_create_missing_dates_returns_400():
    resp = client.post(
        "/internal/hands/execute",
        json=_tool_payload("calendar.create", {"title": "Missing times"}),
    )
    assert resp.status_code == 400
