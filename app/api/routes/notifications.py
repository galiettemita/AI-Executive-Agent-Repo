from datetime import datetime
from fastapi import APIRouter, Depends, Query
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.db.models import NotificationQueue

router = APIRouter(prefix="/notifications", tags=["notifications"])


@router.get("")
def list_notifications(
    user_id: str = Query(...),
    db: Session = Depends(get_db),
):
    rows = (
        db.query(NotificationQueue)
        .filter(NotificationQueue.user_id == user_id, NotificationQueue.sent_at.is_(None))
        .order_by(NotificationQueue.created_at.desc())
        .all()
    )

    return {
        "items": [
            {
                "id": r.id,
                "event_type": r.event_type,
                "title": r.title,
                "message": r.message,
                "deep_link_url": r.deep_link_url,
                "prev_price": r.prev_price,
                "new_price": r.new_price,
                "currency": r.currency,
                "created_at": r.created_at,
            }
            for r in rows
        ]
    }


@router.post("/ack")
def ack_notifications(
    user_id: str = Query(...),
    db: Session = Depends(get_db),
):
    rows = (
        db.query(NotificationQueue)
        .filter(NotificationQueue.user_id == user_id, NotificationQueue.sent_at.is_(None))
        .all()
    )

    now = datetime.utcnow()
    for r in rows:
        r.sent_at = now
        r.is_sent = True

    db.commit()
    return {"ok": True, "acked": len(rows)}
