# app/api/routes/dashboard.py

from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.services.dashboard_service import DashboardService

router = APIRouter(prefix="/dashboard", tags=["dashboard"])


@router.get("/summary")
def get_execution_summary(
    db: Session = Depends(get_db),
    hours: int = 24,
):
    """
    Get execution summary metrics.

    Provides overview of execution success rate, volumes, and status breakdown.

    Args:
        db: Database session
        hours: Time window in hours (default: 24)

    Returns:
        Dict with execution summary metrics
    """
    summary = DashboardService.get_execution_summary(db, hours)

    return {
        "ok": True,
        **summary,
    }


@router.get("/pending")
def get_pending_executions(
    db: Session = Depends(get_db),
    limit: int = 50,
):
    """
    Get proposals currently in execution.

    Shows proposals that are approved or executing, with their current step.

    Args:
        db: Database session
        limit: Maximum number to return

    Returns:
        Dict with list of pending executions
    """
    pending = DashboardService.get_pending_executions(db, limit)

    return {
        "ok": True,
        "pending_executions": pending,
        "count": len(pending),
    }


@router.get("/failed")
def get_failed_executions(
    db: Session = Depends(get_db),
    hours: int = 24,
    limit: int = 50,
):
    """
    Get failed executions.

    Shows proposals that failed, with error details.

    Args:
        db: Database session
        hours: Time window in hours
        limit: Maximum number to return

    Returns:
        Dict with list of failed executions
    """
    failed = DashboardService.get_failed_executions(db, hours, limit)

    return {
        "ok": True,
        "failed_executions": failed,
        "count": len(failed),
    }


@router.get("/timeline")
def get_execution_timeline(
    db: Session = Depends(get_db),
    hours: int = 24,
):
    """
    Get execution timeline.

    Returns hourly bucketed execution counts for charting.

    Args:
        db: Database session
        hours: Time window in hours

    Returns:
        Dict with timeline data
    """
    timeline = DashboardService.get_execution_timeline(db, hours)

    return {
        "ok": True,
        "timeline": timeline,
    }


@router.get("/user/{user_id}")
def get_user_stats(
    user_id: str,
    db: Session = Depends(get_db),
):
    """
    Get execution statistics for a specific user.

    Args:
        user_id: User ID
        db: Database session

    Returns:
        Dict with user execution stats
    """
    stats = DashboardService.get_user_execution_stats(db, user_id)

    return {
        "ok": True,
        **stats,
    }


@router.get("/bookings")
def get_booking_stats(
    db: Session = Depends(get_db),
    hours: int = 24,
):
    """
    Get booking statistics by type and status.

    Args:
        db: Database session
        hours: Time window in hours

    Returns:
        Dict with booking statistics
    """
    stats = DashboardService.get_booking_stats(db, hours)

    return {
        "ok": True,
        "bookings": stats,
    }


@router.get("/errors")
def get_error_summary(
    db: Session = Depends(get_db),
    hours: int = 24,
    limit: int = 10,
):
    """
    Get summary of most common errors.

    Useful for identifying systemic issues.

    Args:
        db: Database session
        hours: Time window in hours
        limit: Maximum number of error types to return

    Returns:
        Dict with error summary
    """
    errors = DashboardService.get_error_summary(db, hours, limit)

    return {
        "ok": True,
        "errors": errors,
        "count": len(errors),
    }


@router.get("/health")
def get_system_health(
    db: Session = Depends(get_db),
):
    """
    Get overall system health metrics.

    Combines multiple metrics to provide a health score.

    Returns:
        Dict with health metrics
    """
    # Get 1-hour window metrics
    summary = DashboardService.get_execution_summary(db, hours=1)
    pending = DashboardService.get_pending_executions(db, limit=100)
    failed = DashboardService.get_failed_executions(db, hours=1, limit=100)

    # Calculate health score
    success_rate = summary["success_rate"]
    pending_count = len(pending)
    failed_count = len(failed)

    # Simple health scoring
    health_score = 100

    # Deduct for low success rate
    if success_rate < 95:
        health_score -= (95 - success_rate) * 2

    # Deduct for high pending count
    if pending_count > 10:
        health_score -= min(pending_count - 10, 20)

    # Deduct for failures
    health_score -= min(failed_count * 2, 30)

    health_score = max(0, health_score)

    # Determine status
    if health_score >= 90:
        status = "healthy"
    elif health_score >= 70:
        status = "degraded"
    else:
        status = "unhealthy"

    return {
        "ok": True,
        "status": status,
        "health_score": round(health_score, 2),
        "metrics": {
            "success_rate_1h": success_rate,
            "pending_executions": pending_count,
            "failed_executions_1h": failed_count,
            "total_executions_1h": summary["proposals"]["total"],
        },
    }
