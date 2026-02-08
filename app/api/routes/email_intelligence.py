# backend/app/api/routes/email_intelligence.py

from __future__ import annotations

import json
from typing import List, Optional

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
from app.services.email_monitoring import (
    list_email_monitor_configs,
    upsert_email_monitor_config,
    run_email_monitoring,
    list_email_alerts,
    create_test_email_alert,
)
from app.core.config import settings

router = APIRouter(prefix="/email/intelligence", tags=["email"])


def _safe_list(raw: Optional[str]) -> List[str]:
    try:
        data = json.loads(raw or "[]")
        if isinstance(data, list):
            return data
    except Exception:
        return []
    return []


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


class EmailMonitorConfigRequest(BaseModel):
    user_id: str
    config_id: Optional[int] = None
    provider: Optional[str] = None
    enabled: bool = True
    keywords: List[str] = []
    senders: List[str] = []
    subject_keywords: List[str] = []
    priority_threshold: Optional[int] = None
    use_ai_priority: bool = False
    alert_channel: str = "whatsapp"
    alert_title: Optional[str] = None
    window_minutes: int = 60
    max_results: int = 20


class EmailMonitorRunRequest(BaseModel):
    user_id: str


class EmailMonitorTestRequest(BaseModel):
    user_id: str
    subject: Optional[str] = None
    sender: Optional[str] = None
    snippet: Optional[str] = None
    priority: Optional[int] = None
    alert_channel: str = "whatsapp"


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


@rate_limit_user()
@router.get("/monitoring/configs")
def get_email_monitor_configs(
    request: Request,
    user_id: str,
    db: Session = Depends(get_db),
):
    try:
        require_consent(db, user_id, "email")
    except Exception as e:
        raise HTTPException(status_code=403, detail=str(e))
    configs = list_email_monitor_configs(db, user_id)
    return {
        "ok": True,
        "configs": [
            {
                "id": c.id,
                "provider": c.provider,
                "enabled": c.enabled,
                "keywords": _safe_list(c.keywords_json),
                "senders": _safe_list(c.sender_allowlist_json),
                "subject_keywords": _safe_list(c.subject_keywords_json),
                "priority_threshold": c.priority_threshold,
                "use_ai_priority": c.use_ai_priority,
                "alert_channel": c.alert_channel,
                "alert_title": c.alert_title,
                "window_minutes": c.window_minutes,
                "max_results": c.max_results,
                "last_checked_at": c.last_checked_at.isoformat() if c.last_checked_at else None,
                "created_at": c.created_at.isoformat() if c.created_at else None,
            }
            for c in configs
        ],
    }


@rate_limit_user()
@router.post("/monitoring/configs")
def upsert_email_monitor(
    request: Request,
    payload: EmailMonitorConfigRequest,
    db: Session = Depends(get_db),
):
    try:
        require_consent(db, payload.user_id, "email")
    except Exception as e:
        raise HTTPException(status_code=403, detail=str(e))

    config = upsert_email_monitor_config(
        db,
        user_id=payload.user_id,
        config_id=payload.config_id,
        provider=payload.provider,
        enabled=payload.enabled,
        keywords=payload.keywords,
        senders=payload.senders,
        subject_keywords=payload.subject_keywords,
        priority_threshold=payload.priority_threshold,
        use_ai_priority=payload.use_ai_priority,
        alert_channel=payload.alert_channel,
        alert_title=payload.alert_title,
        window_minutes=payload.window_minutes,
        max_results=payload.max_results,
    )
    return {"ok": True, "config_id": config.id, "enabled": config.enabled}


@rate_limit_user()
@router.post("/monitoring/run")
def run_email_monitoring_for_user(
    request: Request,
    payload: EmailMonitorRunRequest,
    db: Session = Depends(get_db),
):
    try:
        require_consent(db, payload.user_id, "email")
    except Exception as e:
        raise HTTPException(status_code=403, detail=str(e))

    result = run_email_monitoring(db, user_id=payload.user_id)
    return {"ok": True, "result": result}


@rate_limit_user()
@router.get("/monitoring/alerts")
def get_email_alerts(
    request: Request,
    user_id: str,
    limit: int = 50,
    db: Session = Depends(get_db),
):
    try:
        require_consent(db, user_id, "email")
    except Exception as e:
        raise HTTPException(status_code=403, detail=str(e))

    alerts = list_email_alerts(db, user_id, limit=limit)
    return {
        "ok": True,
        "alerts": [
            {
                "id": a.id,
                "provider": a.provider,
                "message_id": a.message_id,
                "subject": a.subject,
                "sender": a.sender,
                "priority": a.priority,
                "reason": a.reason,
                "alert_channel": a.alert_channel,
                "created_at": a.created_at.isoformat() if a.created_at else None,
            }
            for a in alerts
        ],
    }


@rate_limit_user()
@router.post("/monitoring/test")
def create_test_email_alert_route(
    request: Request,
    payload: EmailMonitorTestRequest,
    db: Session = Depends(get_db),
):
    try:
        require_consent(db, payload.user_id, "email")
    except Exception as e:
        raise HTTPException(status_code=403, detail=str(e))

    if settings.ENV == "production" and settings.EMAIL_MONITOR_TEST_MODE != "1":
        raise HTTPException(status_code=403, detail="Test mode disabled in production")

    alert = create_test_email_alert(
        db,
        user_id=payload.user_id,
        subject=payload.subject,
        sender=payload.sender,
        snippet=payload.snippet,
        priority=payload.priority,
        alert_channel=(payload.alert_channel or "whatsapp").lower(),
    )
    return {"ok": True, "alert_id": alert.id}
