# tests/test_calendar_scheduling.py

from datetime import datetime, timezone

from app.services.calendar_scheduling import find_conflicts, find_available_slots


def test_find_conflicts_detects_overlap():
    events = [
        {
            "summary": "Busy",
            "start": {"dateTime": "2026-02-07T10:00:00Z"},
            "end": {"dateTime": "2026-02-07T11:00:00Z"},
        }
    ]
    start = datetime(2026, 2, 7, 10, 30, tzinfo=timezone.utc)
    end = datetime(2026, 2, 7, 11, 30, tzinfo=timezone.utc)
    conflicts = find_conflicts(events, start, end)
    assert len(conflicts) == 1


def test_find_available_slots_basic():
    events = [
        {
            "summary": "Busy",
            "start": {"dateTime": "2026-02-07T10:00:00Z"},
            "end": {"dateTime": "2026-02-07T11:00:00Z"},
        }
    ]
    window_start = datetime(2026, 2, 7, 9, 0, tzinfo=timezone.utc)
    window_end = datetime(2026, 2, 7, 12, 0, tzinfo=timezone.utc)
    slots = find_available_slots(
        events,
        window_start_utc=window_start,
        window_end_utc=window_end,
        duration_minutes=30,
        step_minutes=30,
        max_results=4,
    )
    assert slots[0]["start"] == "2026-02-07T09:00:00+00:00"
    assert slots[0]["end"] == "2026-02-07T09:30:00+00:00"
    assert slots[-1]["end"] == "2026-02-07T12:00:00+00:00"
