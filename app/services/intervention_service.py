# app/services/intervention_service.py

from __future__ import annotations

import json
from datetime import datetime
from typing import Dict, List, Optional
from sqlalchemy.orm import Session

from app.db.models import Proposal, Transaction, User, ExecutionLog


class InterventionService:
    """
    Service for managing manual intervention queue.

    Handles transactions that are flagged for manual review before execution.
    """

    # Intervention reasons
    REASON_HIGH_AMOUNT = "high_amount"
    REASON_UNUSUAL_PATTERN = "unusual_pattern"
    REASON_NEW_USER = "new_user"
    REASON_VELOCITY_WARNING = "velocity_warning"
    REASON_SUSPICIOUS_LOCATION = "suspicious_location"
    REASON_MANUAL_REVIEW_REQUESTED = "manual_review_requested"

    @staticmethod
    def should_flag_for_review(
        db: Session,
        user_id: str,
        proposal_id: int,
        amount: float,
    ) -> tuple[bool, Optional[str]]:
        """
        Determine if a proposal should be flagged for manual review.

        Args:
            db: Database session
            user_id: User ID
            proposal_id: Proposal ID
            amount: Transaction amount

        Returns:
            tuple: (should_flag: bool, reason: str)
        """
        # Check 1: High amount threshold (> $1000)
        if amount > 1000:
            return (True, InterventionService.REASON_HIGH_AMOUNT)

        # Check 2: New user (first transaction)
        transaction_count = (
            db.query(Transaction)
            .filter(Transaction.user_id == user_id)
            .count()
        )

        if transaction_count == 0:
            return (True, InterventionService.REASON_NEW_USER)

        # Check 3: Velocity warning (more than 3 transactions in last hour)
        from datetime import timedelta
        one_hour_ago = datetime.utcnow() - timedelta(hours=1)
        recent_transactions = (
            db.query(Transaction)
            .filter(
                Transaction.user_id == user_id,
                Transaction.created_at >= one_hour_ago,
            )
            .count()
        )

        if recent_transactions >= 3:
            return (True, InterventionService.REASON_VELOCITY_WARNING)

        # Check 4: Unusual pattern (3x average transaction amount)
        avg_amount = (
            db.query(Transaction)
            .filter(Transaction.user_id == user_id)
            .with_entities(Transaction.amount)
            .all()
        )

        if avg_amount:
            avg = sum([t[0] for t in avg_amount]) / len(avg_amount)
            if amount > avg * 3:
                return (True, InterventionService.REASON_UNUSUAL_PATTERN)

        return (False, None)

    @staticmethod
    def add_to_intervention_queue(
        db: Session,
        proposal_id: int,
        reason: str,
        metadata: Optional[Dict] = None,
    ) -> None:
        """
        Add a proposal to the manual intervention queue.

        Args:
            db: Database session
            proposal_id: Proposal ID
            reason: Reason for flagging
            metadata: Optional additional metadata
        """
        proposal = db.query(Proposal).filter(Proposal.id == proposal_id).first()

        if not proposal:
            return

        # Update proposal status
        proposal.status = "pending_review"

        # Store intervention metadata in proposal metadata
        intervention_data = {
            "flagged_at": datetime.utcnow().isoformat(),
            "reason": reason,
            "metadata": metadata or {},
        }

        # Update or create metadata JSON
        try:
            existing_meta = json.loads(proposal.payload_json)
            existing_meta["intervention"] = intervention_data
            proposal.payload_json = json.dumps(existing_meta)
        except:
            pass

        # Log intervention
        log = ExecutionLog(
            proposal_id=proposal_id,
            step="manual_review_flagged",
            status="pending",
            error_message=f"Flagged for manual review: {reason}",
        )
        db.add(log)

        db.commit()

    @staticmethod
    def get_intervention_queue(
        db: Session,
        status: str = "pending_review",
        limit: int = 50,
    ) -> List[Proposal]:
        """
        Get proposals in the manual intervention queue.

        Args:
            db: Database session
            status: Filter by status (default: "pending_review")
            limit: Maximum number of proposals to return

        Returns:
            List of proposals pending review
        """
        proposals = (
            db.query(Proposal)
            .filter(Proposal.status == status)
            .order_by(Proposal.created_at.asc())  # FIFO
            .limit(limit)
            .all()
        )

        return proposals

    @staticmethod
    def approve_proposal(
        db: Session,
        proposal_id: int,
        reviewer_id: str,
        notes: Optional[str] = None,
    ) -> bool:
        """
        Approve a flagged proposal for execution.

        Args:
            db: Database session
            proposal_id: Proposal ID
            reviewer_id: ID of the reviewer approving the proposal
            notes: Optional reviewer notes

        Returns:
            bool: True if approved successfully
        """
        proposal = db.query(Proposal).filter(Proposal.id == proposal_id).first()

        if not proposal or proposal.status != "pending_review":
            return False

        # Update proposal status to approved
        proposal.status = "approved"

        # Add approval metadata
        try:
            existing_meta = json.loads(proposal.payload_json)
            existing_meta["intervention"]["approved_at"] = datetime.utcnow().isoformat()
            existing_meta["intervention"]["approved_by"] = reviewer_id
            if notes:
                existing_meta["intervention"]["approval_notes"] = notes
            proposal.payload_json = json.dumps(existing_meta)
        except:
            pass

        # Log approval
        log = ExecutionLog(
            proposal_id=proposal_id,
            step="manual_review_approved",
            status="completed",
            error_message=f"Approved by {reviewer_id}: {notes or 'No notes'}",
        )
        db.add(log)

        db.commit()
        return True

    @staticmethod
    def reject_proposal(
        db: Session,
        proposal_id: int,
        reviewer_id: str,
        reason: str,
    ) -> bool:
        """
        Reject a flagged proposal.

        Args:
            db: Database session
            proposal_id: Proposal ID
            reviewer_id: ID of the reviewer rejecting the proposal
            reason: Reason for rejection

        Returns:
            bool: True if rejected successfully
        """
        proposal = db.query(Proposal).filter(Proposal.id == proposal_id).first()

        if not proposal or proposal.status != "pending_review":
            return False

        # Update proposal status to rejected
        proposal.status = "rejected"

        # Add rejection metadata
        try:
            existing_meta = json.loads(proposal.payload_json)
            existing_meta["intervention"]["rejected_at"] = datetime.utcnow().isoformat()
            existing_meta["intervention"]["rejected_by"] = reviewer_id
            existing_meta["intervention"]["rejection_reason"] = reason
            proposal.payload_json = json.dumps(existing_meta)
        except:
            pass

        # Log rejection
        log = ExecutionLog(
            proposal_id=proposal_id,
            step="manual_review_rejected",
            status="completed",
            error_message=f"Rejected by {reviewer_id}: {reason}",
        )
        db.add(log)

        db.commit()
        return True

    @staticmethod
    def get_intervention_stats(db: Session) -> Dict:
        """
        Get statistics about the intervention queue.

        Returns:
            Dict with queue statistics
        """
        pending_count = (
            db.query(Proposal)
            .filter(Proposal.status == "pending_review")
            .count()
        )

        approved_count = (
            db.query(ExecutionLog)
            .filter(ExecutionLog.step == "manual_review_approved")
            .count()
        )

        rejected_count = (
            db.query(ExecutionLog)
            .filter(ExecutionLog.step == "manual_review_rejected")
            .count()
        )

        # Get oldest pending proposal
        oldest_pending = (
            db.query(Proposal)
            .filter(Proposal.status == "pending_review")
            .order_by(Proposal.created_at.asc())
            .first()
        )

        return {
            "pending_review": pending_count,
            "total_approved": approved_count,
            "total_rejected": rejected_count,
            "oldest_pending_age_minutes": (
                int((datetime.utcnow() - oldest_pending.created_at).total_seconds() / 60)
                if oldest_pending
                else None
            ),
        }
