# app/api/routes/payment.py

from __future__ import annotations

import os
import stripe
from fastapi import APIRouter, Depends, HTTPException, Request, Header
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.db.models import PaymentMethod, Transaction, User
from app.services.stripe_service import StripeService

router = APIRouter(prefix="/payment", tags=["payment"])


# -------------------
# REQUEST MODELS
# -------------------

class AddPaymentMethodRequest(BaseModel):
    user_id: str
    stripe_payment_method_id: str
    is_default: bool = False


class CreatePaymentIntentRequest(BaseModel):
    user_id: str
    proposal_id: int
    amount: float
    currency: str = "usd"
    transaction_type: str
    description: str | None = None
    payment_method_id: int | None = None


class ConfirmPaymentRequest(BaseModel):
    transaction_id: int


class RefundRequest(BaseModel):
    transaction_id: int
    amount: float | None = None
    reason: str | None = None


# -------------------
# ENDPOINTS
# -------------------

@router.post("/methods")
def add_payment_method(
    request: AddPaymentMethodRequest,
    db: Session = Depends(get_db),
):
    """
    Add a payment method for a user.

    The client should create a payment method using Stripe.js/Elements,
    then send the payment method ID to this endpoint.
    """
    try:
        payment_method = StripeService.create_payment_method(
            db=db,
            user_id=request.user_id,
            stripe_payment_method_id=request.stripe_payment_method_id,
            is_default=request.is_default,
        )

        return {
            "ok": True,
            "payment_method": {
                "id": payment_method.id,
                "type": payment_method.type,
                "brand": payment_method.brand,
                "last4": payment_method.last4,
                "exp_month": payment_method.exp_month,
                "exp_year": payment_method.exp_year,
                "is_default": payment_method.is_default,
            },
        }
    except Exception as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.get("/methods/{user_id}")
def list_payment_methods(
    user_id: str,
    db: Session = Depends(get_db),
):
    """List all payment methods for a user"""
    methods = db.query(PaymentMethod).filter(PaymentMethod.user_id == user_id).all()

    return {
        "payment_methods": [
            {
                "id": pm.id,
                "type": pm.type,
                "brand": pm.brand,
                "last4": pm.last4,
                "exp_month": pm.exp_month,
                "exp_year": pm.exp_year,
                "is_default": pm.is_default,
            }
            for pm in methods
        ]
    }


@router.post("/intent/create")
def create_payment_intent(
    request: CreatePaymentIntentRequest,
    db: Session = Depends(get_db),
):
    """
    Create a payment intent for a proposal.

    This does NOT charge the card immediately - it creates an intent
    that must be confirmed later during execution.
    """
    try:
        # Check spending limits first
        limit_check = StripeService.check_spending_limit(
            db=db,
            user_id=request.user_id,
            amount=request.amount,
        )

        if not limit_check["allowed"]:
            raise HTTPException(
                status_code=403,
                detail={
                    "error": "spending_limit_exceeded",
                    "message": limit_check["reason"],
                    "limit_type": limit_check["limit_type"],
                },
            )

        # Create payment intent
        transaction = StripeService.create_payment_intent(
            db=db,
            user_id=request.user_id,
            proposal_id=request.proposal_id,
            amount=request.amount,
            currency=request.currency,
            transaction_type=request.transaction_type,
            description=request.description,
            payment_method_id=request.payment_method_id,
            idempotency_key=f"proposal_{request.proposal_id}",
        )

        return {
            "ok": True,
            "transaction_id": transaction.id,
            "stripe_payment_intent_id": transaction.stripe_payment_intent_id,
            "amount": transaction.amount,
            "currency": transaction.currency,
            "status": transaction.status,
        }

    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.post("/intent/confirm")
def confirm_payment_intent(
    request: ConfirmPaymentRequest,
    db: Session = Depends(get_db),
):
    """
    Confirm a payment intent.

    This is called during execution after proposal approval.
    """
    try:
        result = StripeService.confirm_payment_intent(
            db=db,
            transaction_id=request.transaction_id,
        )

        return {
            "ok": True,
            **result,
        }

    except Exception as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.post("/refund")
def refund_payment(
    request: RefundRequest,
    db: Session = Depends(get_db),
):
    """
    Refund a payment.

    Used when booking fails or user requests cancellation.
    """
    try:
        result = StripeService.refund_payment(
            db=db,
            transaction_id=request.transaction_id,
            amount=request.amount,
            reason=request.reason,
        )

        return {
            "ok": True,
            **result,
        }

    except Exception as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.get("/transactions/{user_id}")
def list_transactions(
    user_id: str,
    db: Session = Depends(get_db),
    limit: int = 50,
):
    """List transactions for a user"""
    transactions = (
        db.query(Transaction)
        .filter(Transaction.user_id == user_id)
        .order_by(Transaction.created_at.desc())
        .limit(limit)
        .all()
    )

    return {
        "transactions": [
            {
                "id": t.id,
                "proposal_id": t.proposal_id,
                "amount": t.amount,
                "currency": t.currency,
                "status": t.status,
                "transaction_type": t.transaction_type,
                "description": t.description,
                "created_at": t.created_at.isoformat(),
            }
            for t in transactions
        ]
    }


# -------------------
# STRIPE WEBHOOKS
# -------------------

@router.post("/webhooks/stripe")
async def stripe_webhook(
    request: Request,
    stripe_signature: str = Header(None, alias="stripe-signature"),
    db: Session = Depends(get_db),
):
    """
    Handle Stripe webhook events.

    Supports:
    - payment_intent.succeeded
    - payment_intent.payment_failed
    - charge.refunded
    """
    webhook_secret = os.getenv("STRIPE_WEBHOOK_SECRET")

    if not webhook_secret:
        raise HTTPException(status_code=500, detail="Webhook secret not configured")

    # Get raw body
    payload = await request.body()

    try:
        # Verify webhook signature
        event = stripe.Webhook.construct_event(
            payload, stripe_signature, webhook_secret
        )
    except ValueError:
        raise HTTPException(status_code=400, detail="Invalid payload")
    except stripe.error.SignatureVerificationError:
        raise HTTPException(status_code=400, detail="Invalid signature")

    # Handle event
    event_type = event["type"]
    data = event["data"]["object"]

    if event_type == "payment_intent.succeeded":
        # Update transaction status
        payment_intent_id = data["id"]
        transaction = db.query(Transaction).filter(
            Transaction.stripe_payment_intent_id == payment_intent_id
        ).first()

        if transaction:
            transaction.status = "succeeded"
            transaction.stripe_charge_id = data.get("latest_charge")

            # Generate invoice PDF if not already generated
            if not transaction.invoice_pdf_path:
                try:
                    from app.services.invoice_service import InvoiceService
                    invoice_path = InvoiceService.generate_invoice_pdf(db, transaction.id)
                    transaction.invoice_pdf_path = invoice_path
                except Exception as e:
                    print(f"[Stripe Webhook] Failed to generate invoice for transaction {transaction.id}: {e}")

            db.commit()

            print(f"[Stripe Webhook] Payment succeeded for transaction {transaction.id}")

    elif event_type == "payment_intent.payment_failed":
        # Update transaction status
        payment_intent_id = data["id"]
        transaction = db.query(Transaction).filter(
            Transaction.stripe_payment_intent_id == payment_intent_id
        ).first()

        if transaction:
            transaction.status = "failed"
            db.commit()

            print(f"[Stripe Webhook] Payment failed for transaction {transaction.id}")

    elif event_type == "charge.refunded":
        # Handle refund
        charge_id = data["id"]
        transaction = db.query(Transaction).filter(
            Transaction.stripe_charge_id == charge_id
        ).first()

        if transaction and transaction.status != "refunded":
            refund_amount = data["amount_refunded"] / 100  # Convert cents to dollars
            transaction.status = "refunded"
            transaction.refund_amount = refund_amount

            from datetime import datetime
            transaction.refunded_at = datetime.utcnow()

            db.commit()

            print(f"[Stripe Webhook] Refund processed for transaction {transaction.id}")

    return {"received": True}


@router.get("/invoice/{transaction_id}")
def get_invoice(
    transaction_id: int,
    db: Session = Depends(get_db),
):
    """
    Generate or retrieve the PDF invoice for a transaction.

    This endpoint will:
    1. Check if invoice already exists
    2. If not, generate a new invoice PDF
    3. Return the PDF file

    Args:
        transaction_id: ID of the transaction
        db: Database session

    Returns:
        FileResponse: PDF invoice file
    """
    from app.services.invoice_service import InvoiceService
    from fastapi.responses import FileResponse
    import os

    # Check if invoice already exists
    invoice_path = InvoiceService.get_invoice_path(db, transaction_id)

    # If not exists or file is missing, generate new invoice
    if not invoice_path or not os.path.exists(invoice_path):
        try:
            invoice_path = InvoiceService.generate_invoice_pdf(db, transaction_id)
            InvoiceService.update_invoice_path(db, transaction_id, invoice_path)
        except ValueError as e:
            raise HTTPException(status_code=404, detail=str(e))
        except Exception as e:
            raise HTTPException(status_code=500, detail=f"Failed to generate invoice: {str(e)}")

    # Return PDF file
    if not os.path.exists(invoice_path):
        raise HTTPException(status_code=404, detail="Invoice file not found")

    return FileResponse(
        invoice_path,
        media_type="application/pdf",
        filename=f"invoice_{transaction_id}.pdf",
    )


@router.post("/invoice/{transaction_id}/regenerate")
def regenerate_invoice(
    transaction_id: int,
    db: Session = Depends(get_db),
):
    """
    Force regeneration of an invoice PDF.

    Useful if transaction details were updated and invoice needs to be regenerated.

    Args:
        transaction_id: ID of the transaction
        db: Database session

    Returns:
        Dict with success status and invoice path
    """
    from app.services.invoice_service import InvoiceService

    try:
        invoice_path = InvoiceService.generate_invoice_pdf(db, transaction_id)
        InvoiceService.update_invoice_path(db, transaction_id, invoice_path)

        return {
            "ok": True,
            "transaction_id": transaction_id,
            "invoice_path": invoice_path,
        }
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to regenerate invoice: {str(e)}")
