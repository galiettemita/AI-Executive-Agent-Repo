# backend/app/api/routes/email_intelligence.py

from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.middleware.rate_limiter import rate_limit_user
from app.services.consent_service import require_consent
from app.services.email_intelligence import summarize_inbox, draft_reply
from app.db.models import EmailDraft
from app.services.email_draft_service import (
    create_email_draft,
    get_latest_pending_draft,
    send_email_draft,
    cancel_pending_draft,
)

router = APIRouter(prefix="/email/intelligence", tags=["email"])


class DraftReplyRequest(BaseModel):
    user_id: str
    message_id: Optional[str] = None
    query: Optional[str] = None
    tone: Optional[str] = None
    instruction: Optional[str] = None
    provider: Optional[str] = None


class SendDraftRequest(BaseModel):
    user_id: str
    draft_id: Optional[int] = None


class CancelDraftRequest(BaseModel):
    user_id: str
    draft_id: Optional[int] = None


@rate_limit_user()
@router.get("/summary")
def inbox_summary(
    request: Request,
    user_id: str,
    max_results: int = 10,
    hours_back: int = 24,
    provider: Optional[str] = None,
    db: Session = Depends(get_db),
):
    try:
        require_consent(db, user_id, "email")
    except Exception as e:
        raise HTTPException(status_code=403, detail=str(e))
    result = summarize_inbox(
        db=db,
        user_id=user_id,
        max_results=max_results,
        hours_back=hours_back,
        provider=provider,
    )
    return result


@rate_limit_user()
@router.post("/reply/draft")
def draft_email_reply(
    request: Request,
    payload: DraftReplyRequest,
    db: Session = Depends(get_db),
):
    try:
        require_consent(db, payload.user_id, "email")
    except Exception as e:
        raise HTTPException(status_code=403, detail=str(e))

    try:
        reply = draft_reply(
            db=db,
            user_id=payload.user_id,
            message_id=payload.message_id,
            query=payload.query,
            tone=payload.tone,
            instruction=payload.instruction,
            provider=payload.provider,
        )
    except Exception as exc:
        raise HTTPException(status_code=400, detail=str(exc))

    draft = create_email_draft(
        db=db,
        user_id=payload.user_id,
        to_email=reply["to_email"],
        subject=reply["subject"],
        body_text=reply["body"],
        provider=reply.get("provider") or payload.provider,
        source_message_id=reply.get("source_message_id"),
        metadata={"origin": "email_intelligence"},
    )

    return {
        "draft_id": draft.id,
        "to_email": draft.to_email,
        "subject": draft.subject,
        "body": draft.body_text,
        "status": draft.status,
        "message": "Draft ready. Reply 'send' to send.",
    }


@rate_limit_user()
@router.post("/reply/send")
def send_email_reply(
    request: Request,
    payload: SendDraftRequest,
    db: Session = Depends(get_db),
):
    try:
        require_consent(db, payload.user_id, "email")
    except Exception as e:
        raise HTTPException(status_code=403, detail=str(e))

    draft = None
    if payload.draft_id:
        draft = db.query(EmailDraft).filter(EmailDraft.id == payload.draft_id, EmailDraft.user_id == payload.user_id).first()
    if not draft:
        draft = get_latest_pending_draft(db, payload.user_id)
    if not draft:
        raise HTTPException(status_code=404, detail="No pending draft found")

    try:
        draft = send_email_draft(db, draft)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"Send failed: {exc}")

    return {"ok": True, "draft_id": draft.id, "status": draft.status}


@rate_limit_user()
@router.post("/reply/cancel")
def cancel_email_reply(
    request: Request,
    payload: CancelDraftRequest,
    db: Session = Depends(get_db),
):
    draft = None
    if payload.draft_id:
        draft = db.query(EmailDraft).filter(EmailDraft.id == payload.draft_id, EmailDraft.user_id == payload.user_id).first()
    if not draft:
        draft = get_latest_pending_draft(db, payload.user_id)
    if not draft:
        raise HTTPException(status_code=404, detail="No pending draft found")

    draft = cancel_pending_draft(db, draft)
    return {"ok": True, "draft_id": draft.id, "status": draft.status}
