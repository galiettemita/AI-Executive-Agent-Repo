from __future__ import annotations

import json
from datetime import datetime
from typing import Any, Dict, Optional

from sqlalchemy.orm import Session

from app.db.models import EmailDraft
from app.services.email_router import send_email


def _to_json(value: Any) -> str:
    return json.dumps(value or {}, ensure_ascii=False)


def create_email_draft(
    db: Session,
    *,
    user_id: str,
    to_email: str,
    subject: str,
    body_text: str,
    cc: Optional[str] = None,
    bcc: Optional[str] = None,
    provider: Optional[str] = None,
    source_message_id: Optional[str] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> EmailDraft:
    draft = EmailDraft(
        user_id=user_id,
        provider=provider,
        to_email=to_email,
        cc=cc,
        bcc=bcc,
        subject=subject,
        body_text=body_text,
        source_message_id=source_message_id,
        status="pending",
        metadata_json=_to_json(metadata) if metadata else None,
    )
    db.add(draft)
    db.commit()
    db.refresh(draft)
    return draft


def get_latest_pending_draft(db: Session, user_id: str) -> Optional[EmailDraft]:
    return (
        db.query(EmailDraft)
        .filter(EmailDraft.user_id == user_id, EmailDraft.status == "pending")
        .order_by(EmailDraft.created_at.desc())
        .first()
    )


def cancel_pending_draft(db: Session, draft: EmailDraft) -> EmailDraft:
    draft.status = "canceled"
    draft.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(draft)
    return draft


def send_email_draft(db: Session, draft: EmailDraft) -> EmailDraft:
    result = send_email(
        db=db,
        user_id=draft.user_id,
        to_email=draft.to_email,
        subject=draft.subject,
        body_text=draft.body_text,
        cc=draft.cc,
        bcc=draft.bcc,
        provider=draft.provider,
    )
    draft.status = "sent"
    draft.sent_at = datetime.utcnow()
    draft.updated_at = datetime.utcnow()
    if isinstance(result, dict) and result.get("id"):
        draft.provider_draft_id = result.get("id")
    db.commit()
    db.refresh(draft)
    return draft
