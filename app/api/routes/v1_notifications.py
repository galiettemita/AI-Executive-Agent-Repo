from __future__ import annotations

from datetime import datetime, timezone
from typing import Any

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel, Field
from sqlalchemy.orm import Session

from app.api.deps import get_db
from app.services.scheduled_notifications import run_due_scheduled_notifications, schedule_notification

router = APIRouter(prefix="/api/v1/notifications", tags=["notifications-v1"])


class ScheduleNotificationRequest(BaseModel):
    user_id: str
    notification_type: str = "custom"
    message: str
    scheduled_for: datetime | None = None
    channel: str | None = None
    timezone: str | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)


@router.post("/schedule")
def create_scheduled_notification(payload: ScheduleNotificationRequest, db: Session = Depends(get_db)):
    when = payload.scheduled_for or datetime.now(timezone.utc)
    if when.tzinfo is None:
        when = when.replace(tzinfo=timezone.utc)
    notif_id = schedule_notification(
        db,
        user_id=payload.user_id,
        notification_type=payload.notification_type,
        payload={"message": payload.message, **(payload.metadata or {})},
        scheduled_for=when,
        channel=payload.channel,
        timezone_name=payload.timezone,
    )
    return {"ok": True, "id": notif_id, "scheduled_for": when.isoformat()}


@router.post("/run")
def run_due_notifications(db: Session = Depends(get_db)):
    try:
        result = run_due_scheduled_notifications(db)
        return {"ok": True, "result": result}
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"scheduler_failed: {exc}")
