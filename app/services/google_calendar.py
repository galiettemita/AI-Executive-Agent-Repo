# backend/app/services/google_calendar.py

from __future__ import annotations

from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

from googleapiclient.discovery import build
from sqlalchemy.orm import Session

from app.services.google_oauth import get_valid_google_credentials


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