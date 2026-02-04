# app/services/dashboard_service.py

from __future__ import annotations

from datetime import datetime, timedelta
from typing import Dict, List, Optional
from sqlalchemy.orm import Session
from sqlalchemy import func, and_

from app.db.models import (
    Proposal,
    Transaction,
    ExecutionLog,
    Booking,
    User,
)


class DashboardService:
    """Service for execution dashboard metrics and monitoring"""

    @staticmethod
    def get_execution_summary(db: Session, hours: int = 24) -> Dict:
        """
        Get execution summary metrics.

        Args:
            db: Database session
            hours: Time window in hours (default: 24)

        Returns:
            Dict with execution metrics
        """
        cutoff_time = datetime.utcnow() - timedelta(hours=hours)

        # Count proposals by status
        proposals_by_status = (
            db.query(Proposal.status, func.count(Proposal.id))
            .filter(Proposal.created_at >= cutoff_time)
            .group_by(Proposal.status)
            .all()
        )

        status_counts = {status: count for status, count in proposals_by_status}

        # Count transactions by status
        transactions_by_status = (
            db.query(Transaction.status, func.count(Transaction.id))
            .filter(Transaction.created_at >= cutoff_time)
            .group_by(Transaction.status)
            .all()
        )

        transaction_counts = {status: count for status, count in transactions_by_status}

        # Calculate success rate
        total_executions = sum(status_counts.values())
        successful = status_counts.get("completed", 0)
        success_rate = (successful / total_executions * 100) if total_executions > 0 else 0

        # Calculate total transaction volume
        total_volume = (
            db.query(func.sum(Transaction.amount))
            .filter(
                Transaction.created_at >= cutoff_time,
                Transaction.status == "succeeded",
            )
            .scalar() or 0
        )

        return {
            "time_window_hours": hours,
            "proposals": {
                "total": total_executions,
                "by_status": status_counts,
            },
            "transactions": {
                "total": sum(transaction_counts.values()),
                "by_status": transaction_counts,
            },
            "success_rate": round(success_rate, 2),
            "total_volume": round(total_volume, 2),
        }

    @staticmethod
    def get_pending_executions(db: Session, limit: int = 50) -> List[Dict]:
        """
        Get proposals in executing state.

        Args:
            db: Database session
            limit: Maximum number to return

        Returns:
            List of pending execution details
        """
        pending_proposals = (
            db.query(Proposal)
            .filter(Proposal.status.in_(["approved", "executing"]))
            .order_by(Proposal.created_at.asc())
            .limit(limit)
            .all()
        )

        result = []
        for proposal in pending_proposals:
            # Get latest execution log
            latest_log = (
                db.query(ExecutionLog)
                .filter(ExecutionLog.proposal_id == proposal.id)
                .order_by(ExecutionLog.created_at.desc())
                .first()
            )

            # Get associated transaction
            transaction = (
                db.query(Transaction)
                .filter(Transaction.proposal_id == proposal.id)
                .first()
            )

            age_minutes = int((datetime.utcnow() - proposal.created_at).total_seconds() / 60)

            result.append({
                "proposal_id": proposal.id,
                "user_id": proposal.user_id,
                "proposal_type": proposal.proposal_type,
                "status": proposal.status,
                "age_minutes": age_minutes,
                "current_step": latest_log.step if latest_log else "unknown",
                "transaction_id": transaction.id if transaction else None,
                "amount": transaction.amount if transaction else None,
                "created_at": proposal.created_at.isoformat(),
            })

        return result

    @staticmethod
    def get_failed_executions(db: Session, hours: int = 24, limit: int = 50) -> List[Dict]:
        """
        Get failed executions.

        Args:
            db: Database session
            hours: Time window in hours
            limit: Maximum number to return

        Returns:
            List of failed execution details
        """
        cutoff_time = datetime.utcnow() - timedelta(hours=hours)

        failed_proposals = (
            db.query(Proposal)
            .filter(
                Proposal.status == "failed",
                Proposal.created_at >= cutoff_time,
            )
            .order_by(Proposal.created_at.desc())
            .limit(limit)
            .all()
        )

        result = []
        for proposal in failed_proposals:
            # Get failure reason from execution log
            failure_log = (
                db.query(ExecutionLog)
                .filter(
                    ExecutionLog.proposal_id == proposal.id,
                    ExecutionLog.status == "failed",
                )
                .order_by(ExecutionLog.created_at.desc())
                .first()
            )

            # Get transaction details
            transaction = (
                db.query(Transaction)
                .filter(Transaction.proposal_id == proposal.id)
                .first()
            )

            result.append({
                "proposal_id": proposal.id,
                "user_id": proposal.user_id,
                "proposal_type": proposal.proposal_type,
                "failed_step": failure_log.step if failure_log else "unknown",
                "error_message": failure_log.error_message if failure_log else None,
                "transaction_id": transaction.id if transaction else None,
                "amount": transaction.amount if transaction else None,
                "refunded": transaction.status == "refunded" if transaction else False,
                "failed_at": proposal.updated_at.isoformat() if proposal.updated_at else None,
            })

        return result

    @staticmethod
    def get_execution_timeline(db: Session, hours: int = 24) -> List[Dict]:
        """
        Get execution timeline (hourly bucketed counts).

        Args:
            db: Database session
            hours: Time window in hours

        Returns:
            List of hourly execution counts
        """
        cutoff_time = datetime.utcnow() - timedelta(hours=hours)

        # Get proposals by hour
        proposals = (
            db.query(
                func.strftime("%Y-%m-%d %H:00:00", Proposal.created_at).label("hour"),
                Proposal.status,
                func.count(Proposal.id).label("count"),
            )
            .filter(Proposal.created_at >= cutoff_time)
            .group_by("hour", Proposal.status)
            .order_by("hour")
            .all()
        )

        # Organize by hour
        timeline = {}
        for hour, status, count in proposals:
            if hour not in timeline:
                timeline[hour] = {
                    "hour": hour,
                    "completed": 0,
                    "failed": 0,
                    "pending": 0,
                }

            if status == "completed":
                timeline[hour]["completed"] = count
            elif status == "failed":
                timeline[hour]["failed"] = count
            elif status in ["pending", "approved", "executing"]:
                timeline[hour]["pending"] += count

        return list(timeline.values())

    @staticmethod
    def get_user_execution_stats(db: Session, user_id: str) -> Dict:
        """
        Get execution statistics for a specific user.

        Args:
            db: Database session
            user_id: User ID

        Returns:
            Dict with user execution stats
        """
        # Total executions
        total_proposals = (
            db.query(func.count(Proposal.id))
            .filter(Proposal.user_id == user_id)
            .scalar() or 0
        )

        # By status
        proposals_by_status = (
            db.query(Proposal.status, func.count(Proposal.id))
            .filter(Proposal.user_id == user_id)
            .group_by(Proposal.status)
            .all()
        )

        status_counts = {status: count for status, count in proposals_by_status}

        # Total spending
        total_spent = (
            db.query(func.sum(Transaction.amount))
            .filter(
                Transaction.user_id == user_id,
                Transaction.status == "succeeded",
            )
            .scalar() or 0
        )

        # Success rate
        successful = status_counts.get("completed", 0)
        success_rate = (successful / total_proposals * 100) if total_proposals > 0 else 0

        # Last execution
        last_proposal = (
            db.query(Proposal)
            .filter(Proposal.user_id == user_id)
            .order_by(Proposal.created_at.desc())
            .first()
        )

        return {
            "user_id": user_id,
            "total_proposals": total_proposals,
            "by_status": status_counts,
            "success_rate": round(success_rate, 2),
            "total_spent": round(total_spent, 2),
            "last_execution_at": last_proposal.created_at.isoformat() if last_proposal else None,
        }

    @staticmethod
    def get_booking_stats(db: Session, hours: int = 24) -> Dict:
        """
        Get booking statistics by type and status.

        Args:
            db: Database session
            hours: Time window in hours

        Returns:
            Dict with booking statistics
        """
        cutoff_time = datetime.utcnow() - timedelta(hours=hours)

        # Bookings by type
        bookings_by_type = (
            db.query(Booking.booking_type, func.count(Booking.id))
            .filter(Booking.created_at >= cutoff_time)
            .group_by(Booking.booking_type)
            .all()
        )

        # Bookings by status
        bookings_by_status = (
            db.query(Booking.status, func.count(Booking.id))
            .filter(Booking.created_at >= cutoff_time)
            .group_by(Booking.status)
            .all()
        )

        return {
            "by_type": {booking_type: count for booking_type, count in bookings_by_type},
            "by_status": {status: count for status, count in bookings_by_status},
            "total": sum(count for _, count in bookings_by_type),
        }

    @staticmethod
    def get_error_summary(db: Session, hours: int = 24, limit: int = 10) -> List[Dict]:
        """
        Get summary of most common errors.

        Args:
            db: Database session
            hours: Time window in hours
            limit: Maximum number of error types to return

        Returns:
            List of error summaries
        """
        cutoff_time = datetime.utcnow() - timedelta(hours=hours)

        # Get error counts by step
        errors = (
            db.query(
                ExecutionLog.step,
                ExecutionLog.error_message,
                func.count(ExecutionLog.id).label("count"),
            )
            .filter(
                ExecutionLog.status == "failed",
                ExecutionLog.created_at >= cutoff_time,
            )
            .group_by(ExecutionLog.step, ExecutionLog.error_message)
            .order_by(func.count(ExecutionLog.id).desc())
            .limit(limit)
            .all()
        )

        return [
            {
                "step": step,
                "error_message": error_msg[:200] if error_msg else "Unknown error",
                "count": count,
            }
            for step, error_msg, count in errors
        ]
