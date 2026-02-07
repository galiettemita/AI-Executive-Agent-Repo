# app/api/routes/execution.py

from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException, Query, Request
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.db.models import Proposal, ExecutionLog, Booking
from app.services.execution_engine import ExecutionEngine
from app.middleware.rate_limiter import rate_limit_user

router = APIRouter(prefix="/execution", tags=["execution"])


# -------------------
# REQUEST MODELS
# -------------------

class ExecuteProposalRequest(BaseModel):
    proposal_id: int
    approval_token: str
    dry_run: bool = False


# -------------------
# ENDPOINTS
# -------------------

@rate_limit_user()
@router.post("/execute")
def execute_proposal(
    request: Request,
    payload: ExecuteProposalRequest,
    db: Session = Depends(get_db),
):
    """
    Execute an approved proposal.

    This is the main entry point for the execution engine.
    It handles the complete flow from approval verification to booking confirmation.

    Args:
        request: ExecuteProposalRequest with proposal_id and approval_token
        db: Database session

    Returns:
        Dict with execution result
    """
    try:
        result = ExecutionEngine.execute_proposal(
            db=db,
            proposal_id=payload.proposal_id,
            approval_token=payload.approval_token,
            dry_run=payload.dry_run,
        )

        return {
            "ok": True,
            **result,
        }

    except ValueError as e:
        # Validation error (bad token, spending limit, etc.)
        raise HTTPException(status_code=400, detail=str(e))

    except Exception as e:
        # Execution error
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/logs/{proposal_id}")
def get_execution_logs(
    proposal_id: int,
    db: Session = Depends(get_db),
):
    """
    Get execution logs for a proposal.

    Useful for debugging and showing execution progress to users.
    """
    logs = (
        db.query(ExecutionLog)
        .filter(ExecutionLog.proposal_id == proposal_id)
        .order_by(ExecutionLog.created_at.asc())
        .all()
    )

    return {
        "proposal_id": proposal_id,
        "logs": [
            {
                "step": log.step,
                "status": log.status,
                "error_message": log.error_message,
                "created_at": log.created_at.isoformat(),
            }
            for log in logs
        ],
    }


@router.get("/bookings/{user_id}")
def get_user_bookings(
    user_id: str,
    db: Session = Depends(get_db),
    booking_type: str | None = Query(None),
    status: str | None = Query(None),
    limit: int = 50,
):
    """
    Get bookings for a user.

    Can filter by booking_type and status.
    """
    query = db.query(Booking).filter(Booking.user_id == user_id)

    if booking_type:
        query = query.filter(Booking.booking_type == booking_type)

    if status:
        query = query.filter(Booking.status == status)

    bookings = query.order_by(Booking.created_at.desc()).limit(limit).all()

    return {
        "bookings": [
            {
                "id": b.id,
                "proposal_id": b.proposal_id,
                "booking_type": b.booking_type,
                "provider": b.provider,
                "status": b.status,
                "confirmation_number": b.confirmation_number,
                "pnr": b.pnr,
                "created_at": b.created_at.isoformat(),
            }
            for b in bookings
        ]
    }


@router.get("/booking/{booking_id}")
def get_booking_details(
    booking_id: int,
    db: Session = Depends(get_db),
):
    """Get detailed information about a specific booking"""
    booking = db.query(Booking).filter(Booking.id == booking_id).first()

    if not booking:
        raise HTTPException(status_code=404, detail="Booking not found")

    import json

    return {
        "id": booking.id,
        "user_id": booking.user_id,
        "proposal_id": booking.proposal_id,
        "transaction_id": booking.transaction_id,
        "booking_type": booking.booking_type,
        "provider": booking.provider,
        "status": booking.status,
        "confirmation_number": booking.confirmation_number,
        "pnr": booking.pnr,
        "payload": json.loads(booking.payload_json) if booking.payload_json else {},
        "created_at": booking.created_at.isoformat(),
        "updated_at": booking.updated_at.isoformat() if booking.updated_at else None,
    }


@router.post("/test/dry-run")
def test_dry_run(
    proposal_id: int,
    approval_token: str,
    db: Session = Depends(get_db),
):
    """
    Test execution in dry-run mode.

    Validates everything without actually charging or booking.
    Useful for testing approval flows.
    """
    try:
        result = ExecutionEngine.execute_proposal(
            db=db,
            proposal_id=proposal_id,
            approval_token=approval_token,
            dry_run=True,
        )

        return {
            "ok": True,
            **result,
        }

    except Exception as e:
        raise HTTPException(status_code=400, detail=str(e))
