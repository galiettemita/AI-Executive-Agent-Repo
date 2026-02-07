# backend/app/api/routes/admin_caldav.py

from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel, Field
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.services.integration_credentials import (
    upsert_integration_credential,
    delete_integration_credential,
    get_connection_status,
)

router = APIRouter(prefix="/admin/caldav", tags=["admin"])


class CalDAVConnectRequest(BaseModel):
    user_id: str = Field(..., description="User ID (WhatsApp phone number or internal user ID).")
    server_url: str = Field(..., description="CalDAV server URL (e.g., iCloud CalDAV).")
    username: str = Field(..., description="CalDAV username (often your iCloud email).")
    password: str = Field(..., description="CalDAV password or app-specific password.")
    calendar_url: Optional[str] = Field(None, description="Optional calendar URL to pin.")
    calendar_name: Optional[str] = Field(None, description="Optional calendar name to pin.")
    verify: bool = Field(False, description="Verify credentials by listing calendars.")


@router.post("/connect")
def caldav_connect(
    req: CalDAVConnectRequest,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, req.user_id)
    metadata = {}
    if req.calendar_url:
        metadata["calendar_url"] = req.calendar_url
    if req.calendar_name:
        metadata["calendar_name"] = req.calendar_name

    upsert_integration_credential(
        db=db,
        user_id=req.user_id,
        provider="caldav",
        username=req.username,
        secret=req.password,
        server_url=req.server_url,
        metadata=metadata,
    )

    calendars = None
    if req.verify:
        try:
            from app.services.caldav_calendar import list_caldav_calendars
            calendars = list_caldav_calendars(db=db, user_id=req.user_id)
        except Exception as e:
            raise HTTPException(status_code=400, detail=str(e))

    return {"ok": True, "calendars": calendars}


@router.get("/status")
def caldav_status(
    user_id: str,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    return get_connection_status(db=db, user_id=user_id, provider="caldav")


@router.post("/disconnect")
def caldav_disconnect(
    user_id: str,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    delete_integration_credential(db=db, user_id=user_id, provider="caldav")
    return {"ok": True}
