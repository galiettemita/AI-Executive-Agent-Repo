# backend/app/api/routes/monitoring.py

from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.services.scheduler import run_price_monitoring, run_notification_delivery, run_email_monitoring
from app.services.circuit_breaker import (
    get_all_circuit_breakers,
    reset_circuit_breaker,
    reset_all_circuit_breakers,
)

router = APIRouter(prefix="/monitoring", tags=["monitoring"])


@router.post("/trigger/price-check")
def trigger_price_check():
    """
    Manually trigger price monitoring for all watch items.
    Useful for testing and debugging.
    """
    run_price_monitoring()
    return {"ok": True, "message": "Price monitoring triggered"}


@router.post("/trigger/send-notifications")
def trigger_send_notifications():
    """
    Manually trigger notification delivery for all pending notifications.
    Useful for testing and debugging.
    """
    run_notification_delivery()
    return {"ok": True, "message": "Notification delivery triggered"}


@router.post("/trigger/email-monitoring")
def trigger_email_monitoring():
    """
    Manually trigger email monitoring for all configured users.
    Useful for testing and debugging.
    """
    run_email_monitoring()
    return {"ok": True, "message": "Email monitoring triggered"}


# -------------------
# CIRCUIT BREAKER ENDPOINTS
# -------------------

@router.get("/circuit-breakers")
def get_circuit_breaker_status():
    """
    Get status of all circuit breakers.

    Returns:
        Dict with status of each circuit breaker (amadeus, stripe, google)
    """
    breakers = get_all_circuit_breakers()
    return {
        "ok": True,
        "circuit_breakers": breakers,
    }


@router.post("/circuit-breakers/{name}/reset")
def reset_single_circuit_breaker(name: str):
    """
    Reset a specific circuit breaker.

    Args:
        name: Circuit breaker name (amadeus, stripe, google)
    """
    success = reset_circuit_breaker(name)
    if not success:
        raise HTTPException(status_code=404, detail=f"Circuit breaker '{name}' not found")

    return {
        "ok": True,
        "message": f"Circuit breaker '{name}' has been reset",
    }


@router.post("/circuit-breakers/reset-all")
def reset_all_breakers():
    """
    Reset all circuit breakers to closed state.
    """
    reset_all_circuit_breakers()
    return {
        "ok": True,
        "message": "All circuit breakers have been reset",
    }
