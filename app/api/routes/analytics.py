from __future__ import annotations

import json
from typing import Optional

from fastapi import APIRouter, Depends, Request
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.middleware.rate_limiter import rate_limit_user
from app.services.analytics_service import list_usage_events, summarize_usage

router = APIRouter(prefix="/analytics", tags=["analytics"])


@rate_limit_user()
@router.get("/events")
def list_events(
    request: Request,
    user_id: str,
    limit: int = 100,
    event_type: Optional[str] = None,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    events = list_usage_events(db, user_id=user_id, limit=limit, event_type=event_type)
    return {
        "ok": True,
        "events": [
            {
                "id": e.id,
                "event_type": e.event_type,
                "source": e.source,
                "channel": e.channel,
                "provider": e.provider,
                "tokens": e.tokens,
                "cost_usd": e.cost_usd,
                "metadata": json.loads(e.metadata_json) if e.metadata_json else {},
                "created_at": e.created_at.isoformat() if e.created_at else None,
            }
            for e in events
        ],
    }


@rate_limit_user()
@router.get("/summary")
def usage_summary(
    request: Request,
    user_id: str,
    hours_back: int = 24,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    return summarize_usage(db, user_id=user_id, hours_back=hours_back)
