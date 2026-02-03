# backend/app/services/proposals.py

from __future__ import annotations

import json
import os
from datetime import datetime, timedelta
from typing import Dict

from sqlalchemy.orm import Session

from app.db.models import Proposal


def create_proposal(
    db: Session,
    *,
    user_id: str,
    proposal_type: str,
    payload: Dict,
    ttl_hours: int = 24,
) -> Proposal:
    row = Proposal(
        user_id=user_id,
        proposal_type=proposal_type,
        status="pending",
        payload_json=json.dumps(payload, ensure_ascii=False),
        created_at=datetime.utcnow(),
        expires_at=datetime.utcnow() + timedelta(hours=ttl_hours),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def get_proposal(db: Session, proposal_id: int) -> Proposal | None:
    return db.query(Proposal).filter(Proposal.id == proposal_id).first()


def update_proposal_status(db: Session, proposal_id: int, status: str) -> None:
    row = get_proposal(db, proposal_id)
    if not row:
        return
    row.status = status
    db.commit()


def build_proposal_link(proposal_id: int) -> str:
    base = os.getenv("APP_BASE_URL", "https://ai-shopping-assistant-backend-6bgf.onrender.com")
    return f"{base}/proposals/{proposal_id}"


def create_proposal_with_link(
    db: Session,
    *,
    user_id: str,
    proposal_type: str,
    payload: Dict,
    ttl_hours: int = 24,
) -> Dict[str, str]:
    row = create_proposal(
        db,
        user_id=user_id,
        proposal_type=proposal_type,
        payload=payload,
        ttl_hours=ttl_hours,
    )
    return {"proposal_id": str(row.id), "approval_url": build_proposal_link(row.id)}
