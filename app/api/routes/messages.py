from __future__ import annotations

from typing import Any, Dict, Optional

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.services.messaging_service import (
    queue_outbound_message,
    list_messages,
    get_message,
    deliver_pending_messages,
)
from app.middleware.rate_limiter import rate_limit_user

router = APIRouter(prefix="/messages", tags=["messages"])


class OutboundMessageRequest(BaseModel):
    user_id: str
    channel: str
    to_address: Optional[str] = None
    contact_id: Optional[int] = None
    body: str
    metadata: Optional[Dict[str, Any]] = None


@rate_limit_user()
@router.post("/outbound")
def queue_outbound(request: Request, payload: OutboundMessageRequest, db: Session = Depends(get_db)):
    try:
        msg = queue_outbound_message(
            db,
            user_id=payload.user_id,
            channel=payload.channel,
            to_address=payload.to_address,
            body=payload.body,
            contact_id=payload.contact_id,
            metadata=payload.metadata,
        )
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))

    return {"ok": True, "message_id": msg.id, "status": msg.status}


@rate_limit_user()
@router.get("/outbound")
def list_outbound(request: Request, user_id: str, limit: int = 50, db: Session = Depends(get_db)):
    items = list_messages(db, user_id, limit=limit)
    return {
        "items": [
            {
                "id": m.id,
                "channel": m.channel,
                "to": m.to_address,
                "status": m.status,
                "provider": m.provider,
                "provider_status": m.provider_status,
                "error": m.error_message,
                "created_at": m.created_at.isoformat() if m.created_at else None,
                "sent_at": m.sent_at.isoformat() if m.sent_at else None,
                "delivered_at": m.delivered_at.isoformat() if m.delivered_at else None,
                "failed_at": m.failed_at.isoformat() if m.failed_at else None,
            }
            for m in items
        ]
    }


@rate_limit_user()
@router.get("/outbound/{message_id}")
def get_outbound(request: Request, message_id: int, user_id: str, db: Session = Depends(get_db)):
    msg = get_message(db, message_id, user_id=user_id)
    if not msg:
        raise HTTPException(status_code=404, detail="Message not found")
    return {
        "id": msg.id,
        "channel": msg.channel,
        "to": msg.to_address,
        "body": msg.body,
        "status": msg.status,
        "provider": msg.provider,
        "provider_status": msg.provider_status,
        "error": msg.error_message,
        "created_at": msg.created_at.isoformat() if msg.created_at else None,
        "sent_at": msg.sent_at.isoformat() if msg.sent_at else None,
        "delivered_at": msg.delivered_at.isoformat() if msg.delivered_at else None,
        "failed_at": msg.failed_at.isoformat() if msg.failed_at else None,
    }


@rate_limit_user()
@router.post("/outbound/deliver")
def deliver_outbound(request: Request, user_id: str, db: Session = Depends(get_db)):
    result = deliver_pending_messages(db)
    return {"ok": True, **result}
