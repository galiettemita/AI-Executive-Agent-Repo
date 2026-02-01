# backend/app/services/inbound_events.py

from sqlalchemy.orm import Session
from app.db.models import InboundEvent


def already_processed(db: Session, external_id: str) -> bool:
    return db.query(InboundEvent).filter(InboundEvent.external_id == external_id).first() is not None


def record_inbound(db: Session, channel: str, external_id: str, user_id: str) -> None:
    ev = InboundEvent(channel=channel, external_id=external_id, user_id=user_id, processed=True)
    db.add(ev)
    db.commit()