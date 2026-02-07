# backend/app/services/caldav_calendar.py

from __future__ import annotations

import json
from datetime import date, datetime, timedelta, timezone
import logging
from typing import Any, Dict, List, Optional

from sqlalchemy.orm import Session

from app.services.integration_credentials import get_integration_credential, get_decrypted_secret

logger = logging.getLogger(__name__)


CALDAV_PROVIDER = "caldav"


def _require_caldav():
    try:
        import caldav  # type: ignore
        from icalendar import Calendar, Event  # type: ignore
        return caldav, Calendar, Event
    except Exception as e:
        raise RuntimeError(
            "CalDAV libraries not installed. Add 'caldav' and 'icalendar' to requirements."
        ) from e


def _metadata(row) -> Dict[str, Any]:
    if not row or not row.metadata_json:
        return {}
    try:
        data = json.loads(row.metadata_json)
        return data if isinstance(data, dict) else {}
    except Exception:
        return {}


def _get_calendar(row) -> Any:
    caldav, _, _ = _require_caldav()
    secret = get_decrypted_secret(row) if row else None
    if not row or not row.server_url or not row.username or not secret:
        raise RuntimeError("CalDAV credentials are incomplete. Connect CalDAV first.")

    client = caldav.DAVClient(url=row.server_url, username=row.username, password=secret)
    principal = client.principal()
    calendars = principal.calendars()
    if not calendars:
        raise RuntimeError("No CalDAV calendars found for this account.")

    meta = _metadata(row)
    wanted_url = meta.get("calendar_url")
    if wanted_url:
        for cal in calendars:
            if str(getattr(cal, "url", "")) == wanted_url:
                return cal

    wanted_name = meta.get("calendar_name")
    if wanted_name:
        for cal in calendars:
            name = getattr(cal, "name", None)
            if name == wanted_name:
                return cal

    return calendars[0]


def list_caldav_calendars(
    db: Session,
    user_id: str,
) -> List[Dict[str, str]]:
    caldav, _, _ = _require_caldav()
    row = get_integration_credential(db, user_id, CALDAV_PROVIDER)
    secret = get_decrypted_secret(row) if row else None
    if not row or not row.server_url or not row.username or not secret:
        raise RuntimeError("CalDAV credentials are incomplete. Connect CalDAV first.")

    client = caldav.DAVClient(url=row.server_url, username=row.username, password=secret)
    principal = client.principal()
    calendars = principal.calendars()
    output = []
    for cal in calendars:
        output.append(
            {
                "name": getattr(cal, "name", None) or "calendar",
                "url": str(getattr(cal, "url", "")),
            }
        )
    return output


def create_caldav_event(
    db: Session,
    user_id: str,
    title: str,
    start_utc: datetime,
    end_utc: datetime,
    description: Optional[str] = None,
    location: Optional[str] = None,
) -> Dict[str, Any]:
    _, Calendar, Event = _require_caldav()
    row = get_integration_credential(db, user_id, CALDAV_PROVIDER)
    calendar = _get_calendar(row)

    cal = Calendar()
    event = Event()
    event.add("summary", title)
    event.add("dtstart", start_utc.astimezone(timezone.utc))
    event.add("dtend", end_utc.astimezone(timezone.utc))
    event.add("dtstamp", datetime.utcnow())
    if description:
        event.add("description", description)
    if location:
        event.add("location", location)
    cal.add_component(event)

    created = calendar.add_event(cal.to_ical())
    return {
        "id": getattr(created, "url", None),
        "summary": title,
        "start": {"dateTime": start_utc.astimezone(timezone.utc).isoformat()},
        "end": {"dateTime": end_utc.astimezone(timezone.utc).isoformat()},
        "location": location,
    }


def _to_iso(value: Any) -> str:
    if isinstance(value, datetime):
        return value.astimezone(timezone.utc).isoformat()
    if isinstance(value, date):
        return datetime.combine(value, datetime.min.time(), tzinfo=timezone.utc).isoformat()
    if isinstance(value, str):
        return value
    return ""


def list_events_in_range(
    db: Session,
    user_id: str,
    start_utc: datetime,
    end_utc: datetime,
) -> List[Dict[str, Any]]:
    row = get_integration_credential(db, user_id, CALDAV_PROVIDER)
    calendar = _get_calendar(row)
    try:
        events = calendar.date_search(start_utc, end_utc)
    except Exception as e:
        raise RuntimeError(f"CalDAV event search failed: {e}") from e

    output = []
    for item in events:
        comp = getattr(item, "icalendar_component", None)
        if not comp:
            continue
        summary = comp.get("summary")
        dtstart = comp.decoded("dtstart") if hasattr(comp, "decoded") else None
        dtend = comp.decoded("dtend") if hasattr(comp, "decoded") else None
        output.append(
            {
                "id": getattr(item, "url", None),
                "summary": str(summary) if summary else "No title",
                "start": {"dateTime": _to_iso(dtstart)},
                "end": {"dateTime": _to_iso(dtend)},
                "location": str(comp.get("location")) if comp.get("location") else None,
            }
        )
    return output


def get_caldav_events_for_daily_brief(
    db: Session,
    user_id: str,
    days_ahead: int = 1,
) -> List[Dict[str, Any]]:
    row = get_integration_credential(db, user_id, CALDAV_PROVIDER)
    if not row:
        return []

    try:
        now = datetime.now(timezone.utc)
        window_end = now + timedelta(days=days_ahead)
        events = list_events_in_range(db=db, user_id=user_id, start_utc=now, end_utc=window_end)
        simplified = []
        for event in events:
            start = event.get("start") or {}
            end = event.get("end") or {}
            simplified.append(
                {
                    "id": event.get("id"),
                    "summary": event.get("summary") or "No title",
                    "description": None,
                    "start": start.get("dateTime"),
                    "end": end.get("dateTime"),
                    "location": event.get("location"),
                    "attendees": [],
                    "link": None,
                    "is_all_day": False,
                }
            )
        return simplified
    except Exception as e:
        logger.error("Error fetching CalDAV events for daily brief (user %s): %s", user_id, e)
        return []
