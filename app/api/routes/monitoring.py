# backend/app/api/routes/monitoring.py

from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.services.scheduler import run_price_monitoring, run_notification_delivery

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
