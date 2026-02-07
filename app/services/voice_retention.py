from __future__ import annotations

from datetime import datetime, timedelta

from sqlalchemy.orm import Session

from app.db.models import VoiceCall


def purge_expired_calls(db: Session, retention_days: int) -> int:
    if retention_days <= 0:
        return 0

    cutoff = datetime.utcnow() - timedelta(days=retention_days)
    calls = (
        db.query(VoiceCall)
        .filter(
            (VoiceCall.ended_at.isnot(None) & (VoiceCall.ended_at < cutoff))
            | (VoiceCall.ended_at.is_(None) & (VoiceCall.created_at < cutoff))
        )
        .all()
    )

    purged = 0
    for call in calls:
        if any([call.recording_url, call.transcript, call.summary, call.action_items_json]):
            call.recording_url = None
            call.transcript = None
            call.summary = None
            call.action_items_json = None
            purged += 1

    if purged:
        db.commit()
    return purged
