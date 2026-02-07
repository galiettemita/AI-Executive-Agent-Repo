# backend/app/services/google_calendar.py

from __future__ import annotations

from datetime import datetime, timedelta, timezone
import logging
from typing import Any, Dict, List, Optional

from googleapiclient.discovery import build
from sqlalchemy.orm import Session

from app.services.google_oauth import get_valid_google_credentials

logger = logging.getLogger(__name__)


def create_calendar_event(
    db: Session,
    user_id: str,
    title: str,
    start_utc: datetime,
    end_utc: datetime,
    description: Optional[str] = None,
    location: Optional[str] = None,
    reminders_minutes: Optional[List[int]] = None,
) -> Dict[str, Any]:
    creds = get_valid_google_credentials(db=db, user_id=user_id)
    if creds is None:
        raise RuntimeError("Google not connected. Ask the user to connect first.")

    service = build("calendar", "v3", credentials=creds)

    body: Dict[str, Any] = {
        "summary": title,
        "start": {"dateTime": start_utc.astimezone(timezone.utc).isoformat(), "timeZone": "UTC"},
        "end": {"dateTime": end_utc.astimezone(timezone.utc).isoformat(), "timeZone": "UTC"},
    }
    if description:
        body["description"] = description
    if location:
        body["location"] = location
    if reminders_minutes:
        body["reminders"] = {
            "useDefault": False,
            "overrides": [{"method": "popup", "minutes": int(m)} for m in reminders_minutes],
        }

    event = service.events().insert(calendarId="primary", body=body).execute()
    return {
        "id": event.get("id"),
        "htmlLink": event.get("htmlLink"),
        "summary": event.get("summary"),
        "start": event.get("start"),
        "end": event.get("end"),
    }


def update_calendar_event(
    db: Session,
    user_id: str,
    event_id: str,
    title: Optional[str] = None,
    start_utc: Optional[datetime] = None,
    end_utc: Optional[datetime] = None,
    description: Optional[str] = None,
    location: Optional[str] = None,
    reminders_minutes: Optional[List[int]] = None,
) -> Dict[str, Any]:
    creds = get_valid_google_credentials(db=db, user_id=user_id)
    if creds is None:
        raise RuntimeError("Google not connected. Ask the user to connect first.")

    service = build("calendar", "v3", credentials=creds)
    body: Dict[str, Any] = {}

    if title is not None:
        body["summary"] = title
    if start_utc is not None:
        body["start"] = {"dateTime": start_utc.astimezone(timezone.utc).isoformat(), "timeZone": "UTC"}
    if end_utc is not None:
        body["end"] = {"dateTime": end_utc.astimezone(timezone.utc).isoformat(), "timeZone": "UTC"}
    if description is not None:
        body["description"] = description
    if location is not None:
        body["location"] = location
    if reminders_minutes is not None:
        body["reminders"] = {
            "useDefault": False,
            "overrides": [{"method": "popup", "minutes": int(m)} for m in reminders_minutes],
        }

    if not body:
        raise ValueError("No fields provided for update.")

    event = service.events().patch(calendarId="primary", eventId=event_id, body=body).execute()
    return {
        "id": event.get("id"),
        "htmlLink": event.get("htmlLink"),
        "summary": event.get("summary"),
        "start": event.get("start"),
        "end": event.get("end"),
    }


def list_upcoming_events(
    db: Session,
    user_id: str,
    max_results: int = 10,
) -> List[Dict[str, Any]]:
    creds = get_valid_google_credentials(db=db, user_id=user_id)
    if creds is None:
        raise RuntimeError("Google not connected. Ask the user to connect first.")

    service = build("calendar", "v3", credentials=creds)
    now = datetime.now(timezone.utc).isoformat()

    events_result = (
        service.events()
        .list(
            calendarId="primary",
            timeMin=now,
            maxResults=max_results,
            singleEvents=True,
            orderBy="startTime",
        )
        .execute()
    )

    items = events_result.get("items", [])
    out = []
    for e in items:
        out.append(
            {
                "id": e.get("id"),
                "summary": e.get("summary"),
                "start": e.get("start"),
                "end": e.get("end"),
                "htmlLink": e.get("htmlLink"),
                "location": e.get("location"),
            }
        )
    return out


def list_events_in_range(
    db: Session,
    user_id: str,
    start_utc: datetime,
    end_utc: datetime,
    max_results: int = 20,
) -> List[Dict[str, Any]]:
    creds = get_valid_google_credentials(db=db, user_id=user_id)
    if creds is None:
        raise RuntimeError("Google not connected. Ask the user to connect first.")

    service = build("calendar", "v3", credentials=creds)

    events_result = (
        service.events()
        .list(
            calendarId="primary",
            timeMin=start_utc.astimezone(timezone.utc).isoformat(),
            timeMax=end_utc.astimezone(timezone.utc).isoformat(),
            maxResults=max_results,
            singleEvents=True,
            orderBy="startTime",
        )
        .execute()
    )

    items = events_result.get("items", [])
    out = []
    for e in items:
        out.append(
            {
                "id": e.get("id"),
                "summary": e.get("summary"),
                "start": e.get("start"),
                "end": e.get("end"),
                "htmlLink": e.get("htmlLink"),
                "location": e.get("location"),
            }
        )
    return out


def get_events_for_daily_brief(
    db: Session,
    user_id: str,
    days_ahead: int = 1,
) -> List[Dict[str, Any]]:
    """
    Fetch calendar events for the daily brief.

    Args:
        db: Database session
        user_id: User ID
        days_ahead: Number of days ahead to fetch (default: 1 = today + tomorrow)

    Returns:
        List of events with full details for summarization
    """
    creds = get_valid_google_credentials(db=db, user_id=user_id)
    if not creds:
        return []

    try:
        service = build("calendar", "v3", credentials=creds)

        # Time range: now to N days ahead
        now = datetime.now(timezone.utc)
        time_min = now.isoformat()
        time_max = (now + timedelta(days=days_ahead)).isoformat()

        events_result = (
            service.events()
            .list(
                calendarId="primary",
                timeMin=time_min,
                timeMax=time_max,
                maxResults=20,
                singleEvents=True,
                orderBy="startTime",
            )
            .execute()
        )

        events = events_result.get("items", [])

        # Extract relevant fields
        simplified_events = []
        for event in events:
            start = event.get("start", {})
            end = event.get("end", {})

            # Handle all-day events vs timed events
            start_time = start.get("dateTime", start.get("date"))
            end_time = end.get("dateTime", end.get("date"))

            simplified_events.append({
                "id": event.get("id"),
                "summary": event.get("summary", "No title"),
                "description": event.get("description"),
                "start": start_time,
                "end": end_time,
                "location": event.get("location"),
                "attendees": [
                    {"email": a.get("email"), "responseStatus": a.get("responseStatus")}
                    for a in event.get("attendees", [])
                ],
                "link": event.get("htmlLink"),
                "is_all_day": "date" in start,
            })

        return simplified_events

    except Exception as e:
        logger.error("Error fetching calendar events for daily brief (user %s): %s", user_id, e)
        return []
