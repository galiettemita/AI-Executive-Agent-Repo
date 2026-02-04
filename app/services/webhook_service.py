# app/services/webhook_service.py

from __future__ import annotations

import json
import hashlib
import hmac
import uuid
from datetime import datetime, timedelta
from typing import Dict, Optional, List
from sqlalchemy.orm import Session
import httpx

from app.db.models import WebhookEndpoint, WebhookDelivery, Proposal, Transaction


class WebhookService:
    """Service for managing and delivering webhooks"""

    # Webhook event types
    EVENT_PROPOSAL_APPROVED = "proposal.approved"
    EVENT_EXECUTION_STARTED = "execution.started"
    EVENT_EXECUTION_STEP = "execution.step"
    EVENT_EXECUTION_COMPLETED = "execution.completed"
    EVENT_EXECUTION_FAILED = "execution.failed"
    EVENT_PAYMENT_SUCCEEDED = "payment.succeeded"
    EVENT_PAYMENT_FAILED = "payment.failed"
    EVENT_BOOKING_CONFIRMED = "booking.confirmed"
    EVENT_BOOKING_FAILED = "booking.failed"

    @staticmethod
    def register_webhook(
        db: Session,
        user_id: str,
        url: str,
        secret: Optional[str] = None,
        event_types: Optional[List[str]] = None,
        description: Optional[str] = None,
    ) -> WebhookEndpoint:
        """
        Register a new webhook endpoint.

        Args:
            db: Database session
            user_id: User ID
            url: Webhook URL
            secret: Optional secret for HMAC signature
            event_types: List of event types to subscribe to (None = all events)
            description: Optional description

        Returns:
            WebhookEndpoint: Created webhook endpoint
        """
        # Generate secret if not provided
        if not secret:
            secret = WebhookService._generate_secret()

        # Convert event_types list to JSON
        event_types_json = json.dumps(event_types) if event_types else None

        webhook = WebhookEndpoint(
            user_id=user_id,
            url=url,
            secret=secret,
            event_types=event_types_json,
            description=description,
            is_active=True,
        )

        db.add(webhook)
        db.commit()
        db.refresh(webhook)

        return webhook

    @staticmethod
    def send_webhook(
        db: Session,
        user_id: str,
        event_type: str,
        payload: Dict,
        proposal_id: Optional[int] = None,
        transaction_id: Optional[int] = None,
    ) -> List[WebhookDelivery]:
        """
        Send webhook notification to all active endpoints for a user.

        Args:
            db: Database session
            user_id: User ID
            event_type: Event type (e.g., "execution.started")
            payload: Event payload data
            proposal_id: Optional proposal ID
            transaction_id: Optional transaction ID

        Returns:
            List[WebhookDelivery]: List of webhook deliveries created
        """
        # Get all active webhooks for the user
        webhooks = (
            db.query(WebhookEndpoint)
            .filter(
                WebhookEndpoint.user_id == user_id,
                WebhookEndpoint.is_active == True,
            )
            .all()
        )

        deliveries = []

        for webhook in webhooks:
            # Check if webhook is subscribed to this event type
            if webhook.event_types:
                subscribed_events = json.loads(webhook.event_types)
                if event_type not in subscribed_events:
                    continue  # Skip this webhook

            # Create delivery record
            event_id = str(uuid.uuid4())

            delivery_payload = {
                "event_id": event_id,
                "event_type": event_type,
                "timestamp": datetime.utcnow().isoformat(),
                "data": payload,
            }

            delivery = WebhookDelivery(
                webhook_endpoint_id=webhook.id,
                event_type=event_type,
                event_id=event_id,
                proposal_id=proposal_id,
                transaction_id=transaction_id,
                payload_json=json.dumps(delivery_payload),
                status="pending",
            )

            db.add(delivery)
            db.commit()
            db.refresh(delivery)

            # Attempt delivery
            WebhookService._attempt_delivery(db, webhook, delivery, delivery_payload)

            deliveries.append(delivery)

        return deliveries

    @staticmethod
    def _attempt_delivery(
        db: Session,
        webhook: WebhookEndpoint,
        delivery: WebhookDelivery,
        payload: Dict,
    ) -> bool:
        """
        Attempt to deliver a webhook.

        Args:
            db: Database session
            webhook: WebhookEndpoint
            delivery: WebhookDelivery
            payload: Payload to send

        Returns:
            bool: True if delivery succeeded, False otherwise
        """
        try:
            # Generate signature
            signature = WebhookService._generate_signature(
                payload, webhook.secret or ""
            )

            # Send POST request
            headers = {
                "Content-Type": "application/json",
                "X-Webhook-Signature": signature,
                "X-Webhook-Event": delivery.event_type,
                "X-Webhook-Event-Id": delivery.event_id,
            }

            with httpx.Client(timeout=10.0) as client:
                response = client.post(
                    webhook.url,
                    json=payload,
                    headers=headers,
                )

            # Update delivery record
            delivery.response_status_code = response.status_code
            delivery.response_body = response.text[:1000]  # Limit to 1000 chars

            if 200 <= response.status_code < 300:
                delivery.status = "delivered"
                delivery.delivered_at = datetime.utcnow()

                # Update webhook stats
                webhook.total_deliveries += 1
                webhook.last_delivery_at = datetime.utcnow()

                db.commit()
                return True
            else:
                delivery.status = "failed"
                delivery.error_message = f"HTTP {response.status_code}: {response.text[:500]}"
                delivery.retry_count += 1

                # Schedule retry
                delivery.next_retry_at = datetime.utcnow() + timedelta(
                    minutes=5 * (2 ** delivery.retry_count)  # Exponential backoff
                )

                # Update webhook stats
                webhook.failed_deliveries += 1
                webhook.last_failure_at = datetime.utcnow()

                db.commit()
                return False

        except Exception as e:
            # Handle delivery error
            delivery.status = "failed"
            delivery.error_message = str(e)
            delivery.retry_count += 1

            # Schedule retry
            delivery.next_retry_at = datetime.utcnow() + timedelta(
                minutes=5 * (2 ** delivery.retry_count)
            )

            # Update webhook stats
            webhook.failed_deliveries += 1
            webhook.last_failure_at = datetime.utcnow()

            db.commit()
            return False

    @staticmethod
    def retry_failed_deliveries(db: Session, limit: int = 100) -> int:
        """
        Retry failed webhook deliveries that are due for retry.

        Args:
            db: Database session
            limit: Maximum number of deliveries to retry

        Returns:
            int: Number of deliveries retried
        """
        # Get failed deliveries due for retry
        now = datetime.utcnow()
        failed_deliveries = (
            db.query(WebhookDelivery)
            .filter(
                WebhookDelivery.status == "failed",
                WebhookDelivery.retry_count < 5,  # Max 5 retries
                WebhookDelivery.next_retry_at <= now,
            )
            .limit(limit)
            .all()
        )

        retried = 0
        for delivery in failed_deliveries:
            # Get webhook endpoint
            webhook = (
                db.query(WebhookEndpoint)
                .filter(WebhookEndpoint.id == delivery.webhook_endpoint_id)
                .first()
            )

            if webhook and webhook.is_active:
                payload = json.loads(delivery.payload_json)
                if WebhookService._attempt_delivery(db, webhook, delivery, payload):
                    retried += 1

        return retried

    @staticmethod
    def _generate_signature(payload: Dict, secret: str) -> str:
        """
        Generate HMAC signature for webhook payload.

        Args:
            payload: Payload to sign
            secret: Secret key

        Returns:
            str: HMAC signature
        """
        payload_bytes = json.dumps(payload, sort_keys=True).encode("utf-8")
        signature = hmac.new(
            secret.encode("utf-8"),
            payload_bytes,
            hashlib.sha256,
        ).hexdigest()

        return f"sha256={signature}"

    @staticmethod
    def _generate_secret() -> str:
        """Generate a random webhook secret."""
        return "whsec_" + uuid.uuid4().hex

    @staticmethod
    def verify_signature(payload: Dict, signature: str, secret: str) -> bool:
        """
        Verify webhook signature.

        Args:
            payload: Webhook payload
            signature: Signature from X-Webhook-Signature header
            secret: Webhook secret

        Returns:
            bool: True if signature is valid
        """
        expected_signature = WebhookService._generate_signature(payload, secret)
        return hmac.compare_digest(signature, expected_signature)

    @staticmethod
    def get_user_webhooks(db: Session, user_id: str) -> List[WebhookEndpoint]:
        """Get all webhooks for a user."""
        return (
            db.query(WebhookEndpoint)
            .filter(WebhookEndpoint.user_id == user_id)
            .order_by(WebhookEndpoint.created_at.desc())
            .all()
        )

    @staticmethod
    def delete_webhook(db: Session, webhook_id: int, user_id: str) -> bool:
        """Delete a webhook endpoint."""
        webhook = (
            db.query(WebhookEndpoint)
            .filter(
                WebhookEndpoint.id == webhook_id,
                WebhookEndpoint.user_id == user_id,
            )
            .first()
        )

        if webhook:
            db.delete(webhook)
            db.commit()
            return True

        return False

    @staticmethod
    def get_webhook_deliveries(
        db: Session,
        webhook_id: int,
        limit: int = 50,
    ) -> List[WebhookDelivery]:
        """Get recent deliveries for a webhook."""
        return (
            db.query(WebhookDelivery)
            .filter(WebhookDelivery.webhook_endpoint_id == webhook_id)
            .order_by(WebhookDelivery.created_at.desc())
            .limit(limit)
            .all()
        )
