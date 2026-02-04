# app/api/routes/intervention.py

from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy.orm import Session
from typing import Optional
import json

from app.db.database import get_db
from app.db.models import Proposal
from app.services.intervention_service import InterventionService

router = APIRouter(prefix="/intervention", tags=["intervention"])


# -------------------
# REQUEST MODELS
# -------------------

class ApproveProposalRequest(BaseModel):
    reviewer_id: str
    notes: Optional[str] = None


class RejectProposalRequest(BaseModel):
    reviewer_id: str
    reason: str


# -------------------
# ENDPOINTS
# -------------------

@router.get("/queue")
def get_intervention_queue(
    db: Session = Depends(get_db),
    status: str = "pending_review",
    limit: int = 50,
):
    """
    Get proposals in the manual intervention queue.

    This endpoint returns proposals that have been flagged for manual review
    before execution can proceed.

    Args:
        db: Database session
        status: Filter by status (default: "pending_review")
        limit: Maximum number of proposals to return

    Returns:
        Dict with list of flagged proposals
    """
    proposals = InterventionService.get_intervention_queue(db, status, limit)

    return {
        "proposals": [
            {
                "id": p.id,
                "user_id": p.user_id,
                "proposal_type": p.proposal_type,
                "status": p.status,
                "payload": json.loads(p.payload_json) if p.payload_json else {},
                "created_at": p.created_at.isoformat(),
                "updated_at": p.updated_at.isoformat() if p.updated_at else None,
            }
            for p in proposals
        ],
        "count": len(proposals),
    }


@router.get("/stats")
def get_intervention_stats(
    db: Session = Depends(get_db),
):
    """
    Get statistics about the intervention queue.

    Returns metrics like pending count, approved count, rejected count,
    and oldest pending proposal age.

    Returns:
        Dict with intervention queue statistics
    """
    stats = InterventionService.get_intervention_stats(db)

    return {
        "ok": True,
        "stats": stats,
    }


@router.post("/{proposal_id}/approve")
def approve_proposal(
    proposal_id: int,
    request: ApproveProposalRequest,
    db: Session = Depends(get_db),
):
    """
    Approve a flagged proposal for execution.

    This allows a reviewer to approve a proposal that was flagged for
    manual review, allowing it to proceed to execution.

    Args:
        proposal_id: ID of the proposal to approve
        request: ApproveProposalRequest with reviewer info and notes
        db: Database session

    Returns:
        Dict with approval confirmation
    """
    success = InterventionService.approve_proposal(
        db=db,
        proposal_id=proposal_id,
        reviewer_id=request.reviewer_id,
        notes=request.notes,
    )

    if not success:
        raise HTTPException(
            status_code=404,
            detail="Proposal not found or not in pending_review status",
        )

    return {
        "ok": True,
        "proposal_id": proposal_id,
        "status": "approved",
        "message": "Proposal approved and ready for execution",
    }


@router.post("/{proposal_id}/reject")
def reject_proposal(
    proposal_id: int,
    request: RejectProposalRequest,
    db: Session = Depends(get_db),
):
    """
    Reject a flagged proposal.

    This allows a reviewer to reject a proposal that was flagged for
    manual review, preventing it from being executed.

    Args:
        proposal_id: ID of the proposal to reject
        request: RejectProposalRequest with reviewer info and reason
        db: Database session

    Returns:
        Dict with rejection confirmation
    """
    success = InterventionService.reject_proposal(
        db=db,
        proposal_id=proposal_id,
        reviewer_id=request.reviewer_id,
        reason=request.reason,
    )

    if not success:
        raise HTTPException(
            status_code=404,
            detail="Proposal not found or not in pending_review status",
        )

    return {
        "ok": True,
        "proposal_id": proposal_id,
        "status": "rejected",
        "message": "Proposal rejected",
    }


@router.get("/{proposal_id}")
def get_proposal_details(
    proposal_id: int,
    db: Session = Depends(get_db),
):
    """
    Get detailed information about a flagged proposal.

    Useful for reviewing proposal details before approving/rejecting.

    Args:
        proposal_id: ID of the proposal
        db: Database session

    Returns:
        Dict with proposal details including intervention metadata
    """
    proposal = db.query(Proposal).filter(Proposal.id == proposal_id).first()

    if not proposal:
        raise HTTPException(status_code=404, detail="Proposal not found")

    payload = json.loads(proposal.payload_json) if proposal.payload_json else {}

    return {
        "id": proposal.id,
        "user_id": proposal.user_id,
        "proposal_type": proposal.proposal_type,
        "status": proposal.status,
        "payload": payload,
        "intervention": payload.get("intervention", {}),
        "created_at": proposal.created_at.isoformat(),
        "updated_at": proposal.updated_at.isoformat() if proposal.updated_at else None,
    }
