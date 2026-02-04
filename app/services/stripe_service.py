# app/services/stripe_service.py

from __future__ import annotations

import os
import stripe
from typing import Dict, Optional
from sqlalchemy.orm import Session

from app.db.models import PaymentMethod, Transaction, SpendingLimit, User

# Initialize Stripe with secret key
stripe.api_key = os.getenv("STRIPE_SECRET_KEY")


class StripeService:
    """Service for handling Stripe payment operations"""

    @staticmethod
    def create_payment_method(
        db: Session,
        user_id: str,
        stripe_payment_method_id: str,
        is_default: bool = False,
    ) -> PaymentMethod:
        """
        Create or update a payment method for a user.

        Args:
            db: Database session
            user_id: User ID
            stripe_payment_method_id: Stripe payment method ID
            is_default: Whether this is the default payment method

        Returns:
            PaymentMethod: Created or updated payment method
        """
        # Retrieve payment method details from Stripe
        pm = stripe.PaymentMethod.retrieve(stripe_payment_method_id)

        # If this is the default, unset other defaults
        if is_default:
            db.query(PaymentMethod).filter(
                PaymentMethod.user_id == user_id,
                PaymentMethod.is_default == True
            ).update({"is_default": False})

        # Check if payment method already exists
        existing = db.query(PaymentMethod).filter(
            PaymentMethod.stripe_payment_method_id == stripe_payment_method_id
        ).first()

        if existing:
            existing.is_default = is_default
            db.commit()
            db.refresh(existing)
            return existing

        # Extract card details
        card = pm.get("card", {})
        payment_method = PaymentMethod(
            user_id=user_id,
            stripe_payment_method_id=stripe_payment_method_id,
            type=pm.get("type", "card"),
            brand=card.get("brand"),
            last4=card.get("last4"),
            exp_month=card.get("exp_month"),
            exp_year=card.get("exp_year"),
            is_default=is_default,
        )

        db.add(payment_method)
        db.commit()
        db.refresh(payment_method)

        return payment_method

    @staticmethod
    def get_default_payment_method(db: Session, user_id: str) -> Optional[PaymentMethod]:
        """Get the default payment method for a user"""
        return db.query(PaymentMethod).filter(
            PaymentMethod.user_id == user_id,
            PaymentMethod.is_default == True
        ).first()

    @staticmethod
    def create_payment_intent(
        db: Session,
        user_id: str,
        proposal_id: int,
        amount: float,
        currency: str = "usd",
        transaction_type: str = "unknown",
        description: Optional[str] = None,
        payment_method_id: Optional[int] = None,
        idempotency_key: Optional[str] = None,
    ) -> Transaction:
        """
        Create a Stripe payment intent and corresponding transaction record.

        Args:
            db: Database session
            user_id: User ID
            proposal_id: Proposal ID
            amount: Amount in dollars (will be converted to cents)
            currency: Currency code (default: usd)
            transaction_type: Type of transaction (food_order, flight_booking, etc.)
            description: Transaction description
            payment_method_id: Payment method ID to use
            idempotency_key: Idempotency key for Stripe API

        Returns:
            Transaction: Created transaction record
        """
        # Convert amount to cents
        amount_cents = int(amount * 100)

        # Get payment method
        if payment_method_id:
            pm = db.query(PaymentMethod).filter(PaymentMethod.id == payment_method_id).first()
        else:
            pm = StripeService.get_default_payment_method(db, user_id)

        if not pm:
            raise ValueError("No payment method found for user")

        # Create Stripe payment intent
        intent_params = {
            "amount": amount_cents,
            "currency": currency,
            "payment_method": pm.stripe_payment_method_id,
            "confirm": False,  # Don't confirm immediately, wait for explicit confirmation
            "metadata": {
                "user_id": user_id,
                "proposal_id": str(proposal_id),
                "transaction_type": transaction_type,
            },
        }

        if description:
            intent_params["description"] = description

        # Add idempotency key if provided
        request_options = {}
        if idempotency_key:
            request_options["idempotency_key"] = idempotency_key

        payment_intent = stripe.PaymentIntent.create(**intent_params, **request_options)

        # Create transaction record
        transaction = Transaction(
            user_id=user_id,
            proposal_id=proposal_id,
            amount=amount,
            currency=currency,
            status="pending",
            stripe_payment_intent_id=payment_intent.id,
            payment_method_id=pm.id,
            transaction_type=transaction_type,
            description=description,
        )

        db.add(transaction)
        db.commit()
        db.refresh(transaction)

        return transaction

    @staticmethod
    def confirm_payment_intent(
        db: Session,
        transaction_id: int,
    ) -> Dict:
        """
        Confirm a payment intent.

        Args:
            db: Database session
            transaction_id: Transaction ID

        Returns:
            Dict with confirmation status
        """
        transaction = db.query(Transaction).filter(Transaction.id == transaction_id).first()

        if not transaction:
            raise ValueError("Transaction not found")

        if not transaction.stripe_payment_intent_id:
            raise ValueError("No payment intent associated with transaction")

        # Confirm the payment intent
        intent = stripe.PaymentIntent.confirm(transaction.stripe_payment_intent_id)

        # Update transaction status based on intent status
        if intent.status == "succeeded":
            transaction.status = "succeeded"
            transaction.stripe_charge_id = intent.charges.data[0].id if intent.charges.data else None
        elif intent.status == "requires_action":
            transaction.status = "requires_action"
        elif intent.status == "processing":
            transaction.status = "processing"
        else:
            transaction.status = "pending"

        db.commit()
        db.refresh(transaction)

        return {
            "transaction_id": transaction.id,
            "status": transaction.status,
            "stripe_status": intent.status,
            "client_secret": intent.client_secret,  # For 3D Secure
        }

    @staticmethod
    def refund_payment(
        db: Session,
        transaction_id: int,
        amount: Optional[float] = None,
        reason: Optional[str] = None,
    ) -> Dict:
        """
        Refund a payment.

        Args:
            db: Database session
            transaction_id: Transaction ID
            amount: Refund amount in dollars (None for full refund)
            reason: Refund reason

        Returns:
            Dict with refund status
        """
        transaction = db.query(Transaction).filter(Transaction.id == transaction_id).first()

        if not transaction:
            raise ValueError("Transaction not found")

        if transaction.status != "succeeded":
            raise ValueError("Can only refund succeeded transactions")

        # Calculate refund amount in cents
        refund_amount_cents = int(amount * 100) if amount else int(transaction.amount * 100)

        # Create Stripe refund
        refund = stripe.Refund.create(
            payment_intent=transaction.stripe_payment_intent_id,
            amount=refund_amount_cents,
            reason=reason or "requested_by_customer",
        )

        # Update transaction
        transaction.status = "refunded" if not amount or amount == transaction.amount else "partially_refunded"
        transaction.refund_amount = amount or transaction.amount
        transaction.refund_reason = reason

        from datetime import datetime
        transaction.refunded_at = datetime.utcnow()

        db.commit()
        db.refresh(transaction)

        return {
            "transaction_id": transaction.id,
            "refund_id": refund.id,
            "refund_amount": transaction.refund_amount,
            "status": transaction.status,
        }

    @staticmethod
    def check_spending_limit(
        db: Session,
        user_id: str,
        amount: float,
    ) -> Dict:
        """
        Check if user is within spending limits.

        Args:
            db: Database session
            user_id: User ID
            amount: Transaction amount to check

        Returns:
            Dict with allowed status and details
        """
        from datetime import datetime, timedelta

        # Get user's subscription to determine limits
        from app.db.models import Subscription

        subscription = db.query(Subscription).filter(Subscription.user_id == user_id).first()
        plan = subscription.plan if subscription else "free"

        # Default limits by plan
        PLAN_LIMITS = {
            "free": {"daily": 50, "weekly": 200, "monthly": 500, "max_transaction": 100},
            "starter": {"daily": 200, "weekly": 1000, "monthly": 3000, "max_transaction": 500},
            "plus": {"daily": 500, "weekly": 3000, "monthly": 10000, "max_transaction": 2000},
            "pro": {"daily": 2000, "weekly": 10000, "monthly": 50000, "max_transaction": 10000},
        }

        limits = PLAN_LIMITS.get(plan, PLAN_LIMITS["free"])

        # Check max transaction amount
        if amount > limits["max_transaction"]:
            return {
                "allowed": False,
                "reason": f"Transaction amount ${amount} exceeds plan limit of ${limits['max_transaction']}",
                "limit_type": "max_transaction",
            }

        # Check daily limit
        today_start = datetime.utcnow().replace(hour=0, minute=0, second=0, microsecond=0)
        daily_spent = db.query(Transaction).filter(
            Transaction.user_id == user_id,
            Transaction.status == "succeeded",
            Transaction.created_at >= today_start,
        ).with_entities(Transaction.amount).all()
        daily_total = sum([t[0] for t in daily_spent]) + amount

        if daily_total > limits["daily"]:
            return {
                "allowed": False,
                "reason": f"Daily spending limit of ${limits['daily']} would be exceeded (current: ${sum([t[0] for t in daily_spent])}, attempting: ${amount})",
                "limit_type": "daily",
            }

        # Check velocity (max 5 transactions per hour)
        one_hour_ago = datetime.utcnow() - timedelta(hours=1)
        recent_count = db.query(Transaction).filter(
            Transaction.user_id == user_id,
            Transaction.created_at >= one_hour_ago,
        ).count()

        if recent_count >= 5:
            return {
                "allowed": False,
                "reason": "Too many transactions in the last hour (max 5 per hour)",
                "limit_type": "velocity",
            }

        return {"allowed": True}
