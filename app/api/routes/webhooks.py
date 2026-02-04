# app/api/routes/webhooks.py

from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel, HttpUrl
from sqlalchemy.orm import Session
from typing import List, Optional
import json

from app.db.database import get_db
from app.db.models import WebhookEndpoint, WebhookDelivery
from app.services.webhook_service import WebhookService

router = APIRouter(prefix="/webhooks", tags=["webhooks"])


# -------------------
# REQUEST MODELS
# -------------------

class RegisterWebhookRequest(BaseModel):
    url: HttpUrl
    secret: Optional[str] = None
    event_types: Optional[List[str]] = None
    description: Optional[str] = None


class UpdateWebhookRequest(BaseModel):
    is_active: Optional[bool] = None
    event_types: Optional[List[str]] = None
    description: Optional[str] = None


# -------------------
# ENDPOINTS
# -------------------

@router.post("/register")
def register_webhook(
    request: RegisterWebhookRequest,
    user_id: str,  # In production, get from auth token
    db: Session = Depends(get_db),
):
    """
    Register a new webhook endpoint.

    This endpoint allows users to register a URL to receive webhook notifications
    for execution status updates.

    Args:
        request: RegisterWebhookRequest with URL and optional configuration
        user_id: User ID (from auth token in production)
        db: Database session

    Returns:
        Dict with webhook details including secret
    """
    try:
        webhook = WebhookService.register_webhook(
            db=db,
            user_id=user_id,
            url=str(request.url),
            secret=request.secret,
            event_types=request.event_types,
            description=request.description,
        )

        return {
            "ok": True,
            "webhook": {
                "id": webhook.id,
                "url": webhook.url,
                "secret": webhook.secret,
                "event_types": json.loads(webhook.event_types) if webhook.event_types else None,
                "description": webhook.description,
                "is_active": webhook.is_active,
                "created_at": webhook.created_at.isoformat(),
            },
        }

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/list/{user_id}")
def list_webhooks(
    user_id: str,
    db: Session = Depends(get_db),
):
    """
    List all webhooks for a user.

    Returns:
        Dict with list of webhooks
    """
    webhooks = WebhookService.get_user_webhooks(db, user_id)

    return {
        "webhooks": [
            {
                "id": w.id,
                "url": w.url,
                "event_types": json.loads(w.event_types) if w.event_types else None,
                "description": w.description,
                "is_active": w.is_active,
                "total_deliveries": w.total_deliveries,
                "failed_deliveries": w.failed_deliveries,
                "last_delivery_at": w.last_delivery_at.isoformat() if w.last_delivery_at else None,
                "created_at": w.created_at.isoformat(),
            }
            for w in webhooks
        ]
    }


@router.patch("/{webhook_id}")
def update_webhook(
    webhook_id: int,
    request: UpdateWebhookRequest,
    user_id: str,  # In production, get from auth token
    db: Session = Depends(get_db),
):
    """
    Update a webhook endpoint.

    Can update is_active, event_types, and description.
    """
    webhook = (
        db.query(WebhookEndpoint)
        .filter(
            WebhookEndpoint.id == webhook_id,
            WebhookEndpoint.user_id == user_id,
        )
        .first()
    )

    if not webhook:
        raise HTTPException(status_code=404, detail="Webhook not found")

    # Update fields
    if request.is_active is not None:
        webhook.is_active = request.is_active

    if request.event_types is not None:
        webhook.event_types = json.dumps(request.event_types)

    if request.description is not None:
        webhook.description = request.description

    db.commit()
    db.refresh(webhook)

    return {
        "ok": True,
        "webhook": {
            "id": webhook.id,
            "url": webhook.url,
            "is_active": webhook.is_active,
            "event_types": json.loads(webhook.event_types) if webhook.event_types else None,
            "description": webhook.description,
            "updated_at": webhook.updated_at.isoformat(),
        },
    }


@router.delete("/{webhook_id}")
def delete_webhook(
    webhook_id: int,
    user_id: str,  # In production, get from auth token
    db: Session = Depends(get_db),
):
    """Delete a webhook endpoint."""
    success = WebhookService.delete_webhook(db, webhook_id, user_id)

    if not success:
        raise HTTPException(status_code=404, detail="Webhook not found")

    return {"ok": True, "message": "Webhook deleted"}


@router.get("/{webhook_id}/deliveries")
def get_webhook_deliveries(
    webhook_id: int,
    user_id: str,  # In production, get from auth token
    db: Session = Depends(get_db),
    limit: int = 50,
):
    """
    Get recent deliveries for a webhook.

    Useful for debugging and monitoring webhook delivery status.
    """
    # Verify webhook belongs to user
    webhook = (
        db.query(WebhookEndpoint)
        .filter(
            WebhookEndpoint.id == webhook_id,
            WebhookEndpoint.user_id == user_id,
        )
        .first()
    )

    if not webhook:
        raise HTTPException(status_code=404, detail="Webhook not found")

    deliveries = WebhookService.get_webhook_deliveries(db, webhook_id, limit)

    return {
        "webhook_id": webhook_id,
        "deliveries": [
            {
                "id": d.id,
                "event_type": d.event_type,
                "event_id": d.event_id,
                "status": d.status,
                "response_status_code": d.response_status_code,
                "error_message": d.error_message,
                "retry_count": d.retry_count,
                "created_at": d.created_at.isoformat(),
                "delivered_at": d.delivered_at.isoformat() if d.delivered_at else None,
            }
            for d in deliveries
        ],
    }


@router.post("/test")
def test_webhook(
    user_id: str,
    url: HttpUrl,
    db: Session = Depends(get_db),
):
    """
    Test a webhook URL by sending a test event.

    Useful for verifying webhook configuration before registering.
    """
    try:
        # Send test event
        test_payload = {
            "message": "This is a test webhook event",
            "timestamp": WebhookService._generate_secret(),
        }

        deliveries = WebhookService.send_webhook(
            db=db,
            user_id=user_id,
            event_type="webhook.test",
            payload=test_payload,
        )

        if deliveries:
            delivery = deliveries[0]
            return {
                "ok": True,
                "status": delivery.status,
                "response_code": delivery.response_status_code,
                "error_message": delivery.error_message,
            }
        else:
            return {
                "ok": False,
                "error": "No active webhooks found for user",
            }

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
