# backend/app/services/notification_delivery.py

from __future__ import annotations

import os
from datetime import datetime
from typing import Dict, List

import httpx
from sqlalchemy.orm import Session

from app.db.models import NotificationQueue


def send_whatsapp_message(phone_number: str, message: str) -> bool:
    """
    Send a WhatsApp message using the Meta WhatsApp Business API.
    
    Args:
        phone_number: Recipient phone number
        message: Message text to send
    
    Returns:
        True if sent successfully, False otherwise
    """
    token = os.getenv("WHATSAPP_TOKEN")
    phone_number_id = os.getenv("WHATSAPP_PHONE_NUMBER_ID")
    
    if not token or not phone_number_id:
        print("[Notification] WhatsApp credentials not configured")
        return False
    
    url = f"https://graph.facebook.com/v21.0/{phone_number_id}/messages"
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }
    
    payload = {
        "messaging_product": "whatsapp",
        "to": phone_number,
        "type": "text",
        "text": {"body": message},
    }
    
    try:
        resp = httpx.post(url, headers=headers, json=payload, timeout=10)
        resp.raise_for_status()
        print(f"[Notification] ✓ Sent WhatsApp to {phone_number}")
        return True
    except Exception as e:
        print(f"[Notification] ✗ Failed to send WhatsApp to {phone_number}: {e}")
        return False


def deliver_pending_notifications(db: Session) -> Dict[str, int]:
    """
    Deliver all pending notifications via WhatsApp.
    
    Args:
        db: Database session
    
    Returns:
        Dict with counts: {"sent": 0, "failed": 0, "skipped": 0}
    """
    # Get all unsent notifications
    pending = (
        db.query(NotificationQueue)
        .filter(NotificationQueue.is_sent == False)
        .order_by(NotificationQueue.created_at.asc())
        .all()
    )
    
    if not pending:
        return {"sent": 0, "failed": 0, "skipped": 0}
    
    print(f"[Notification] Found {len(pending)} pending notifications")
    
    sent_count = 0
    failed_count = 0
    skipped_count = 0
    
    for notification in pending:
        user_id = notification.user_id
        
        # Format the message
        message = f"🔔 {notification.title}\n\n{notification.message}"
        
        if notification.deep_link_url:
            message += f"\n\n🔗 {notification.deep_link_url}"
        
        # Send via WhatsApp
        # user_id is the phone number from WhatsApp (e.g., "1234567890")
        success = send_whatsapp_message(user_id, message)
        
        if success:
            # Mark as sent
            notification.is_sent = True
            notification.sent_at = datetime.utcnow()
            sent_count += 1
        else:
            # Keep retrying on next run
            failed_count += 1
    
    db.commit()
    
    print(f"[Notification] Delivery complete: {sent_count} sent, {failed_count} failed")
    
    return {"sent": sent_count, "failed": failed_count, "skipped": skipped_count}


def deliver_notifications_for_user(db: Session, user_id: str) -> Dict[str, int]:
    """
    Deliver all pending notifications for a specific user.
    
    Args:
        db: Database session
        user_id: User ID to deliver notifications for
    
    Returns:
        Dict with counts: {"sent": 0, "failed": 0}
    """
    pending = (
        db.query(NotificationQueue)
        .filter(
            NotificationQueue.user_id == user_id,
            NotificationQueue.is_sent == False
        )
        .order_by(NotificationQueue.created_at.asc())
        .all()
    )
    
    if not pending:
        return {"sent": 0, "failed": 0}
    
    sent_count = 0
    failed_count = 0
    
    for notification in pending:
        message = f"🔔 {notification.title}\n\n{notification.message}"
        
        if notification.deep_link_url:
            message += f"\n\n🔗 {notification.deep_link_url}"
        
        success = send_whatsapp_message(user_id, message)
        
        if success:
            notification.is_sent = True
            notification.sent_at = datetime.utcnow()
            sent_count += 1
        else:
            failed_count += 1
    
    db.commit()
    
    return {"sent": sent_count, "failed": failed_count}
