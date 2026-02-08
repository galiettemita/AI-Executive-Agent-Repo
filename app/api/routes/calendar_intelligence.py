# backend/app/api/routes/calendar_intelligence.py

from __future__ import annotations

from datetime import datetime
from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.middleware.rate_limiter import rate_limit_user
from app.services.consent_service import require_consent
from app.services.calendar_intelligence import meeting_prep_brief, suggest_buffer, generate_followup
from app.services.email_draft_service import create_email_draft
from app.db.models import TaskItem

router = APIRouter(prefix="/calendar/intelligence", tags=["calendar"])


class FollowupRequest(BaseModel):
    user_id: str
    event_id: str
    notes: Optional[str] = None
    provider: Optional[str] = None


class BufferRequest(BaseModel):
    user_id: str
    start_utc: str
    end_utc: str
    location: Optional[str] = None
    provider: Optional[str] = None


@rate_limit_user()
@router.get("/brief")
def calendar_meeting_brief(
    request: Request,
    user_id: str,
    event_id: str,
    provider: Optional[str] = None,
    db: Session = Depends(get_db),
):
    try:
        require_consent(db, user_id, "calendar")
    except Exception as e:
        raise HTTPException(status_code=403, detail=str(e))

    try:
        result = meeting_prep_brief(db=db, user_id=user_id, event_id=event_id, provider=provider)
    except Exception as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    return result


@rate_limit_user()
@router.post("/buffer")
def calendar_buffer(
    request: Request,
    payload: BufferRequest,
    db: Session = Depends(get_db),
):
    try:
        require_consent(db, payload.user_id, "calendar")
    except Exception as e:
        raise HTTPException(status_code=403, detail=str(e))

    try:
        start_utc = datetime.fromisoformat(payload.start_utc.replace("Z", "+00:00"))
        end_utc = datetime.fromisoformat(payload.end_utc.replace("Z", "+00:00"))
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid start_utc/end_utc")

    result = suggest_buffer(
        db=db,
        user_id=payload.user_id,
        start_utc=start_utc,
        end_utc=end_utc,
        location=payload.location,
        provider=payload.provider,
    )
    return result


@rate_limit_user()
@router.post("/followup")
def calendar_followup(
    request: Request,
    payload: FollowupRequest,
    db: Session = Depends(get_db),
):
    try:
        require_consent(db, payload.user_id, "calendar")
    except Exception as e:
        raise HTTPException(status_code=403, detail=str(e))
    try:
        require_consent(db, payload.user_id, "email")
    except Exception:
        # Email consent is optional; we still allow tasks-only followup
        pass

    try:
        result = generate_followup(
            db=db,
            user_id=payload.user_id,
            event_id=payload.event_id,
            notes=payload.notes,
            provider=payload.provider,
        )
    except Exception as exc:
        raise HTTPException(status_code=400, detail=str(exc))

    tasks = []
    for task in result.get("tasks") or []:
        row = TaskItem(user_id=payload.user_id, title=str(task))
        db.add(row)
        db.commit()
        db.refresh(row)
        tasks.append({"id": row.id, "title": row.title})

    draft_id = None
    if result.get("to_email") and result.get("email_body"):
        draft = create_email_draft(
            db=db,
            user_id=payload.user_id,
            to_email=result.get("to_email") or "",
            subject=result.get("email_subject") or "Follow-up",
            body_text=result.get("email_body") or "",
            provider=result.get("event", {}).get("provider"),
            metadata={"origin": "calendar_followup"},
        )
        draft_id = draft.id

    return {
        "ok": True,
        "tasks": tasks,
        "draft_id": draft_id,
        "to_email": result.get("to_email"),
        "message": "Follow-up ready. Reply 'send' to send the draft email." if draft_id else "Follow-up tasks created.",
    }
