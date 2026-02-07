# backend/app/services/calendar_scheduling.py

from __future__ import annotations

from datetime import date, datetime, timedelta, timezone
from typing import Any, Dict, Iterable, List, Optional, Tuple


def _parse_iso(value: Any) -> Optional[datetime]:
    if value is None:
        return None
    if isinstance(value, datetime):
        return value.astimezone(timezone.utc) if value.tzinfo else value.replace(tzinfo=timezone.utc)
    if isinstance(value, date):
        return datetime.combine(value, datetime.min.time(), tzinfo=timezone.utc)
    if isinstance(value, str):
        if len(value) == 10 and value[4] == "-" and value[7] == "-":
            return datetime.strptime(value, "%Y-%m-%d").replace(tzinfo=timezone.utc)
        return datetime.fromisoformat(value.replace("Z", "+00:00")).astimezone(timezone.utc)
    return None


def _extract_event_times(event: Dict[str, Any]) -> Tuple[Optional[datetime], Optional[datetime]]:
    start = event.get("start")
    end = event.get("end")

    if isinstance(start, dict):
        start = start.get("dateTime") or start.get("date")
    if isinstance(end, dict):
        end = end.get("dateTime") or end.get("date")

    return _parse_iso(start), _parse_iso(end)


def find_conflicts(
    events: Iterable[Dict[str, Any]],
    start_utc: datetime,
    end_utc: datetime,
) -> List[Dict[str, Any]]:
    conflicts = []
    for event in events:
        ev_start, ev_end = _extract_event_times(event)
        if not ev_start or not ev_end:
            continue
        if ev_start < end_utc and ev_end > start_utc:
            conflicts.append(event)
    return conflicts


def _merge_intervals(intervals: List[Tuple[datetime, datetime]]) -> List[Tuple[datetime, datetime]]:
    if not intervals:
        return []
    intervals.sort(key=lambda x: x[0])
    merged = [intervals[0]]
    for start, end in intervals[1:]:
        last_start, last_end = merged[-1]
        if start <= last_end:
            merged[-1] = (last_start, max(last_end, end))
        else:
            merged.append((start, end))
    return merged


def _within_working_hours(
    start_utc: datetime,
    end_utc: datetime,
    working_hours: Optional[Tuple[int, int]],
) -> bool:
    if not working_hours:
        return True
    start_hour, end_hour = working_hours
    if start_hour >= end_hour:
        return True
    start_t = start_utc.time()
    end_t = end_utc.time()
    return (
        start_t.hour >= start_hour
        and (end_t.hour < end_hour or (end_t.hour == end_hour and end_t.minute == 0))
    )


def find_available_slots(
    events: Iterable[Dict[str, Any]],
    window_start_utc: datetime,
    window_end_utc: datetime,
    duration_minutes: int,
    step_minutes: int = 15,
    buffer_minutes: int = 0,
    working_hours: Optional[Tuple[int, int]] = None,
    max_results: int = 5,
) -> List[Dict[str, str]]:
    duration = timedelta(minutes=duration_minutes)
    step = timedelta(minutes=step_minutes)
    buffer_td = timedelta(minutes=buffer_minutes)

    busy = []
    for event in events:
        ev_start, ev_end = _extract_event_times(event)
        if not ev_start or not ev_end:
            continue
        busy_start = max(window_start_utc, ev_start - buffer_td)
        busy_end = min(window_end_utc, ev_end + buffer_td)
        if busy_start < busy_end:
            busy.append((busy_start, busy_end))

    merged_busy = _merge_intervals(busy)

    free_intervals: List[Tuple[datetime, datetime]] = []
    cursor = window_start_utc
    for start, end in merged_busy:
        if cursor < start:
            free_intervals.append((cursor, start))
        cursor = max(cursor, end)
    if cursor < window_end_utc:
        free_intervals.append((cursor, window_end_utc))

    slots = []
    for free_start, free_end in free_intervals:
        candidate = free_start
        while candidate + duration <= free_end:
            candidate_end = candidate + duration
            if _within_working_hours(candidate, candidate_end, working_hours):
                slots.append(
                    {
                        "start": candidate.astimezone(timezone.utc).isoformat(),
                        "end": candidate_end.astimezone(timezone.utc).isoformat(),
                    }
                )
                if len(slots) >= max_results:
                    return slots
            candidate += step
    return slots
