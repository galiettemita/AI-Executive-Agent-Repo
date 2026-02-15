from __future__ import annotations

import json
import logging
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional

import requests
from sqlalchemy import func
from sqlalchemy.orm import Session

from app.core.config import settings
from app.db.models import UsageEvent

logger = logging.getLogger(__name__)


def _to_json(value: Any) -> str:
    return json.dumps(value or {}, ensure_ascii=False)


def _posthog_capture(
    *,
    user_id: str,
    event_type: str,
    source: Optional[str],
    channel: Optional[str],
    provider: Optional[str],
    tokens: Optional[int],
    cost_usd: Optional[float],
    metadata: Optional[Dict[str, Any]],
) -> None:
    api_key = settings.POSTHOG_API_KEY
    if not api_key:
        return
    host = (settings.POSTHOG_HOST or "https://app.posthog.com").rstrip("/")
    payload = {
        "api_key": api_key,
        "event": event_type,
        "distinct_id": user_id,
        "properties": {
            "$lib": "backend",
            "source": source,
            "channel": channel,
            "provider": provider,
            "tokens": tokens,
            "cost_usd": cost_usd,
            **(metadata or {}),
        },
        "timestamp": datetime.utcnow().isoformat() + "Z",
    }
    try:
        requests.post(f"{host}/capture/", json=payload, timeout=2)
    except Exception as exc:
        logger.warning("PostHog capture failed: %s", exc)


def record_usage_event(
    db: Session,
    *,
    user_id: str,
    event_type: str,
    source: Optional[str] = None,
    channel: Optional[str] = None,
    provider: Optional[str] = None,
    tokens: Optional[int] = None,
    cost_usd: Optional[float] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> UsageEvent:
    event = UsageEvent(
        user_id=user_id,
        event_type=event_type,
        source=source,
        channel=channel,
        provider=provider,
        tokens=tokens,
        cost_usd=cost_usd,
        metadata_json=_to_json(metadata) if metadata else None,
    )
    db.add(event)
    db.commit()
    db.refresh(event)
    _posthog_capture(
        user_id=user_id,
        event_type=event_type,
        source=source,
        channel=channel,
        provider=provider,
        tokens=tokens,
        cost_usd=cost_usd,
        metadata=metadata,
    )
    return event


def list_usage_events(
    db: Session,
    user_id: str,
    limit: int = 100,
    event_type: Optional[str] = None,
) -> List[UsageEvent]:
    query = db.query(UsageEvent).filter(UsageEvent.user_id == user_id)
    if event_type:
        query = query.filter(UsageEvent.event_type == event_type)
    return query.order_by(UsageEvent.created_at.desc()).limit(limit).all()


def summarize_usage(
    db: Session,
    user_id: str,
    hours_back: int = 24,
) -> Dict[str, Any]:
    since = datetime.utcnow() - timedelta(hours=hours_back)
    rows = (
        db.query(
            UsageEvent.event_type,
            func.count(UsageEvent.id),
            func.coalesce(func.sum(UsageEvent.tokens), 0),
            func.coalesce(func.sum(UsageEvent.cost_usd), 0.0),
        )
        .filter(UsageEvent.user_id == user_id, UsageEvent.created_at >= since)
        .group_by(UsageEvent.event_type)
        .all()
    )

    breakdown = []
    for event_type, count, tokens, cost in rows:
        breakdown.append(
            {
                "event_type": event_type,
                "count": int(count or 0),
                "tokens": int(tokens or 0),
                "cost_usd": float(cost or 0.0),
            }
        )

    totals = {
        "events": sum(item["count"] for item in breakdown),
        "tokens": sum(item["tokens"] for item in breakdown),
        "cost_usd": sum(item["cost_usd"] for item in breakdown),
    }
    return {"window_hours": hours_back, "totals": totals, "breakdown": breakdown}
