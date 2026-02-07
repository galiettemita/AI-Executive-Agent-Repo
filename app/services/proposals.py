# backend/app/services/proposals.py

from __future__ import annotations

import json
from datetime import datetime, timedelta
from typing import Dict, Optional

from sqlalchemy.orm import Session

from app.db.models import Proposal, ProposalAuditLog
from app.services.proposal_links import sign_token
from app.core.config import settings


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

    # Log creation in audit log
    log_proposal_action(
        db,
        proposal_id=row.id,
        user_id=user_id,
        action="created",
        new_status="pending",
    )

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
    base = settings.APP_BASE_URL
    token = sign_token({"proposal_id": proposal_id})
    return f"{base}/proposals/{proposal_id}?token={token}"


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


def log_proposal_action(
    db: Session,
    *,
    proposal_id: int,
    user_id: str,
    action: str,
    old_status: Optional[str] = None,
    new_status: Optional[str] = None,
    changes: Optional[Dict] = None,
    metadata: Optional[Dict] = None,
) -> ProposalAuditLog:
    """
    Log a proposal action to the audit log.

    Args:
        db: Database session
        proposal_id: ID of the proposal
        user_id: User performing the action
        action: Action type (created, approved, canceled, edited)
        old_status: Previous status (if applicable)
        new_status: New status (if applicable)
        changes: Dict of changes for edit actions
        metadata: Optional metadata (IP, user agent, etc.)
    """
    log_entry = ProposalAuditLog(
        proposal_id=proposal_id,
        user_id=user_id,
        action=action,
        old_status=old_status,
        new_status=new_status,
        changes_json=json.dumps(changes, ensure_ascii=False) if changes else None,
        metadata_json=json.dumps(metadata, ensure_ascii=False) if metadata else None,
        created_at=datetime.utcnow(),
    )
    db.add(log_entry)
    db.commit()
    db.refresh(log_entry)
    return log_entry
