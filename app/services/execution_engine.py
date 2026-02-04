# app/services/execution_engine.py

from __future__ import annotations

import json
import os
from datetime import datetime, timedelta
from typing import Dict, Optional, Any
from sqlalchemy.orm import Session
import jwt

from app.db.models import (
    Proposal,
    Transaction,
    ExecutionLog,
    Booking,
    User,
)
from app.services.stripe_service import StripeService
from app.services.webhook_service import WebhookService
from app.services.intervention_service import InterventionService


class ExecutionEngine:
    """
    Centralized orchestrator for executing approved proposals.

    Handles the complete flow from proposal approval to booking confirmation:
    1. Verify approval (JWT signature + expiry)
    2. Validate budget and spending limits
    3. Create payment intent
    4. Execute booking via provider API
    5. Handle failures with automatic rollback/refund
    6. Send confirmation notifications
    """

    @staticmethod
    def _log_execution_step(
        db: Session,
        proposal_id: int,
        transaction_id: Optional[int],
        step: str,
        status: str,
        error_message: Optional[str] = None,
        metadata: Optional[Dict] = None,
    ) -> ExecutionLog:
        """Log an execution step for audit and debugging"""
        log = ExecutionLog(
            proposal_id=proposal_id,
            transaction_id=transaction_id,
            step=step,
            status=status,
            error_message=error_message,
            metadata_json=json.dumps(metadata) if metadata else None,
        )
        db.add(log)
        db.commit()
        db.refresh(log)
        return log

    @staticmethod
    def verify_approval_token(token: str) -> Dict:
        """
        Verify JWT approval token.

        Returns:
            Dict with proposal_id and user_id if valid

        Raises:
            ValueError if token is invalid or expired
        """
        jwt_secret = os.getenv("JWT_SECRET")
        if not jwt_secret:
            raise ValueError("JWT_SECRET not configured")

        try:
            payload = jwt.decode(token, jwt_secret, algorithms=["HS256"])

            # Check expiration
            exp = payload.get("exp")
            if exp and datetime.utcnow().timestamp() > exp:
                raise ValueError("Approval link has expired")

            return {
                "proposal_id": payload.get("proposal_id"),
                "user_id": payload.get("user_id"),
                "action": payload.get("action", "approve"),
            }

        except jwt.ExpiredSignatureError:
            raise ValueError("Approval link has expired")
        except jwt.InvalidTokenError as e:
            raise ValueError(f"Invalid approval token: {str(e)}")

    @staticmethod
    def execute_proposal(
        db: Session,
        proposal_id: int,
        approval_token: str,
        dry_run: bool = False,
    ) -> Dict:
        """
        Execute an approved proposal.

        Args:
            db: Database session
            proposal_id: Proposal ID to execute
            approval_token: JWT approval token
            dry_run: If True, validate everything but don't charge or book

        Returns:
            Dict with execution result

        Raises:
            ValueError: If validation fails
            Exception: If execution fails
        """
        # Step 1: Verify approval token
        ExecutionEngine._log_execution_step(
            db, proposal_id, None, "verify_approval", "started"
        )

        try:
            token_data = ExecutionEngine.verify_approval_token(approval_token)
        except ValueError as e:
            ExecutionEngine._log_execution_step(
                db, proposal_id, None, "verify_approval", "failed", error_message=str(e)
            )
            raise

        if token_data["proposal_id"] != proposal_id:
            error_msg = "Token proposal_id mismatch"
            ExecutionEngine._log_execution_step(
                db, proposal_id, None, "verify_approval", "failed", error_message=error_msg
            )
            raise ValueError(error_msg)

        ExecutionEngine._log_execution_step(
            db, proposal_id, None, "verify_approval", "completed"
        )

        # Step 2: Load proposal
        proposal = db.query(Proposal).filter(Proposal.id == proposal_id).first()
        if not proposal:
            raise ValueError("Proposal not found")

        if proposal.status not in ["pending", "approved"]:
            raise ValueError(f"Proposal cannot be executed (status: {proposal.status})")

        # Update proposal status
        proposal.status = "approved" if proposal.status == "pending" else proposal.status
        db.commit()

        # Send webhook: execution started
        try:
            WebhookService.send_webhook(
                db=db,
                user_id=proposal.user_id,
                event_type=WebhookService.EVENT_EXECUTION_STARTED,
                payload={
                    "proposal_id": proposal_id,
                    "proposal_type": proposal.proposal_type,
                    "status": "started",
                },
                proposal_id=proposal_id,
            )
        except Exception as e:
            print(f"[Webhook] Failed to send execution.started: {e}")

        # Parse payload
        try:
            payload = json.loads(proposal.payload_json)
        except json.JSONDecodeError:
            raise ValueError("Invalid proposal payload")

        # Extract amount
        amount = payload.get("total_price") or payload.get("total_amount") or 0
        if amount <= 0:
            raise ValueError("Invalid transaction amount")

        # Step 3: Check spending limits
        ExecutionEngine._log_execution_step(
            db, proposal_id, None, "check_spending_limit", "started"
        )

        limit_check = StripeService.check_spending_limit(
            db=db,
            user_id=proposal.user_id,
            amount=amount,
        )

        if not limit_check["allowed"]:
            ExecutionEngine._log_execution_step(
                db,
                proposal_id,
                None,
                "check_spending_limit",
                "failed",
                error_message=limit_check["reason"],
            )
            raise ValueError(limit_check["reason"])

        ExecutionEngine._log_execution_step(
            db, proposal_id, None, "check_spending_limit", "completed"
        )

        # Step 3.5: Check if proposal should be flagged for manual review
        should_flag, flag_reason = InterventionService.should_flag_for_review(
            db=db,
            user_id=proposal.user_id,
            proposal_id=proposal_id,
            amount=amount,
        )

        if should_flag:
            ExecutionEngine._log_execution_step(
                db,
                proposal_id,
                None,
                "check_intervention",
                "flagged",
                error_message=f"Flagged for manual review: {flag_reason}",
            )

            # Add to intervention queue
            InterventionService.add_to_intervention_queue(
                db=db,
                proposal_id=proposal_id,
                reason=flag_reason,
                metadata={"amount": amount, "proposal_type": proposal.proposal_type},
            )

            raise ValueError(
                f"Proposal flagged for manual review: {flag_reason}. "
                "A team member will review this transaction before proceeding."
            )

        ExecutionEngine._log_execution_step(
            db, proposal_id, None, "check_intervention", "passed"
        )

        if dry_run:
            return {
                "success": True,
                "dry_run": True,
                "message": "Validation successful (dry run mode)",
                "proposal_id": proposal_id,
                "amount": amount,
            }

        # Step 4: Create payment intent
        ExecutionEngine._log_execution_step(
            db, proposal_id, None, "create_payment_intent", "started"
        )

        try:
            transaction = StripeService.create_payment_intent(
                db=db,
                user_id=proposal.user_id,
                proposal_id=proposal_id,
                amount=amount,
                currency=payload.get("currency", "usd"),
                transaction_type=proposal.proposal_type,
                description=payload.get("description") or f"{proposal.proposal_type} - Proposal #{proposal_id}",
                idempotency_key=f"proposal_{proposal_id}",
            )

            ExecutionEngine._log_execution_step(
                db,
                proposal_id,
                transaction.id,
                "create_payment_intent",
                "completed",
                metadata={"transaction_id": transaction.id},
            )

        except Exception as e:
            ExecutionEngine._log_execution_step(
                db, proposal_id, None, "create_payment_intent", "failed", error_message=str(e)
            )
            raise

        # Step 5: Confirm payment
        ExecutionEngine._log_execution_step(
            db, proposal_id, transaction.id, "confirm_payment", "started"
        )

        try:
            payment_result = StripeService.confirm_payment_intent(
                db=db,
                transaction_id=transaction.id,
            )

            if payment_result["status"] != "succeeded":
                raise Exception(f"Payment failed: {payment_result['stripe_status']}")

            ExecutionEngine._log_execution_step(
                db, proposal_id, transaction.id, "confirm_payment", "completed"
            )

            # Send webhook: payment succeeded
            try:
                WebhookService.send_webhook(
                    db=db,
                    user_id=proposal.user_id,
                    event_type=WebhookService.EVENT_PAYMENT_SUCCEEDED,
                    payload={
                        "proposal_id": proposal_id,
                        "transaction_id": transaction.id,
                        "amount": amount,
                        "currency": payload.get("currency", "usd"),
                        "status": "succeeded",
                    },
                    proposal_id=proposal_id,
                    transaction_id=transaction.id,
                )
            except Exception as e:
                print(f"[Webhook] Failed to send payment.succeeded: {e}")

        except Exception as e:
            ExecutionEngine._log_execution_step(
                db, proposal_id, transaction.id, "confirm_payment", "failed", error_message=str(e)
            )
            # Payment failed, no refund needed (payment wasn't captured)
            proposal.status = "failed"
            db.commit()
            raise

        # Step 6: Execute booking
        ExecutionEngine._log_execution_step(
            db, proposal_id, transaction.id, "execute_booking", "started"
        )

        booking_result = None
        try:
            # Dispatch to appropriate executor based on proposal type
            if proposal.proposal_type == "food_order":
                booking_result = ExecutionEngine._execute_food_order(db, proposal, payload, transaction.id)
            elif proposal.proposal_type == "flight_booking":
                booking_result = ExecutionEngine._execute_flight_booking(db, proposal, payload, transaction.id)
            elif proposal.proposal_type == "hotel_booking":
                booking_result = ExecutionEngine._execute_hotel_booking(db, proposal, payload, transaction.id)
            elif proposal.proposal_type == "retail_purchase":
                booking_result = ExecutionEngine._execute_retail_purchase(db, proposal, payload, transaction.id)
            else:
                raise ValueError(f"Unsupported proposal type: {proposal.proposal_type}")

            ExecutionEngine._log_execution_step(
                db,
                proposal_id,
                transaction.id,
                "execute_booking",
                "completed",
                metadata=booking_result,
            )

            # Send webhook: booking confirmed
            try:
                WebhookService.send_webhook(
                    db=db,
                    user_id=proposal.user_id,
                    event_type=WebhookService.EVENT_BOOKING_CONFIRMED,
                    payload={
                        "proposal_id": proposal_id,
                        "transaction_id": transaction.id,
                        "booking_type": proposal.proposal_type,
                        "booking_result": booking_result,
                        "status": "confirmed",
                    },
                    proposal_id=proposal_id,
                    transaction_id=transaction.id,
                )
            except Exception as e:
                print(f"[Webhook] Failed to send booking.confirmed: {e}")

            # Update proposal status
            proposal.status = "completed"
            db.commit()

        except Exception as e:
            ExecutionEngine._log_execution_step(
                db, proposal_id, transaction.id, "execute_booking", "failed", error_message=str(e)
            )

            # Booking failed - trigger automatic refund
            ExecutionEngine._log_execution_step(
                db, proposal_id, transaction.id, "automatic_refund", "started"
            )

            try:
                StripeService.refund_payment(
                    db=db,
                    transaction_id=transaction.id,
                    reason=f"Booking failed: {str(e)}",
                )

                ExecutionEngine._log_execution_step(
                    db, proposal_id, transaction.id, "automatic_refund", "completed"
                )

            except Exception as refund_error:
                ExecutionEngine._log_execution_step(
                    db,
                    proposal_id,
                    transaction.id,
                    "automatic_refund",
                    "failed",
                    error_message=str(refund_error),
                )

            # Send webhook: booking failed
            try:
                WebhookService.send_webhook(
                    db=db,
                    user_id=proposal.user_id,
                    event_type=WebhookService.EVENT_BOOKING_FAILED,
                    payload={
                        "proposal_id": proposal_id,
                        "transaction_id": transaction.id,
                        "error": str(e),
                        "status": "failed",
                    },
                    proposal_id=proposal_id,
                    transaction_id=transaction.id,
                )
            except Exception as webhook_e:
                print(f"[Webhook] Failed to send booking.failed: {webhook_e}")

            # Send webhook: execution failed
            try:
                WebhookService.send_webhook(
                    db=db,
                    user_id=proposal.user_id,
                    event_type=WebhookService.EVENT_EXECUTION_FAILED,
                    payload={
                        "proposal_id": proposal_id,
                        "transaction_id": transaction.id,
                        "error": str(e),
                        "status": "failed",
                        "refund_initiated": True,
                    },
                    proposal_id=proposal_id,
                    transaction_id=transaction.id,
                )
            except Exception as webhook_e:
                print(f"[Webhook] Failed to send execution.failed: {webhook_e}")

            proposal.status = "failed"
            db.commit()
            raise

        # Step 7: Send confirmation
        ExecutionEngine._log_execution_step(
            db, proposal_id, transaction.id, "send_confirmation", "started"
        )

        try:
            ExecutionEngine._send_confirmation(db, proposal, booking_result)
            ExecutionEngine._log_execution_step(
                db, proposal_id, transaction.id, "send_confirmation", "completed"
            )
        except Exception as e:
            # Log but don't fail execution if confirmation fails
            ExecutionEngine._log_execution_step(
                db, proposal_id, transaction.id, "send_confirmation", "failed", error_message=str(e)
            )

        # Send webhook: execution completed
        try:
            WebhookService.send_webhook(
                db=db,
                user_id=proposal.user_id,
                event_type=WebhookService.EVENT_EXECUTION_COMPLETED,
                payload={
                    "proposal_id": proposal_id,
                    "transaction_id": transaction.id,
                    "booking_result": booking_result,
                    "status": "completed",
                },
                proposal_id=proposal_id,
                transaction_id=transaction.id,
            )
        except Exception as e:
            print(f"[Webhook] Failed to send execution.completed: {e}")

        return {
            "success": True,
            "proposal_id": proposal_id,
            "transaction_id": transaction.id,
            "booking": booking_result,
            "message": "Booking completed successfully",
        }

    # -------------------
    # BOOKING EXECUTORS
    # -------------------

    @staticmethod
    def _execute_food_order(
        db: Session,
        proposal: Proposal,
        payload: Dict,
        transaction_id: int,
    ) -> Dict:
        """Execute food delivery order (will be implemented in Stage 12)"""
        # Placeholder for Stage 12 implementation
        # This will integrate with DoorDash/Uber Eats API

        booking = Booking(
            user_id=proposal.user_id,
            proposal_id=proposal.id,
            transaction_id=transaction_id,
            booking_type="food_order",
            provider="doordash",  # or from payload
            status="pending",
            payload_json=json.dumps(payload),
        )
        db.add(booking)
        db.commit()
        db.refresh(booking)

        # TODO: Call DoorDash API to place order
        # For now, return mock confirmation
        return {
            "booking_id": booking.id,
            "confirmation_number": f"MOCK-{booking.id}",
            "status": "confirmed",
        }

    @staticmethod
    def _execute_flight_booking(
        db: Session,
        proposal: Proposal,
        payload: Dict,
        transaction_id: int,
    ) -> Dict:
        """Execute flight booking (will be implemented in Stage 11)"""
        # Placeholder for Stage 11 implementation
        # This will integrate with Amadeus API

        booking = Booking(
            user_id=proposal.user_id,
            proposal_id=proposal.id,
            transaction_id=transaction_id,
            booking_type="flight",
            provider="amadeus",
            status="pending",
            payload_json=json.dumps(payload),
        )
        db.add(booking)
        db.commit()
        db.refresh(booking)

        # TODO: Call Amadeus API to book flight
        return {
            "booking_id": booking.id,
            "confirmation_number": f"MOCK-{booking.id}",
            "pnr": f"MOCK-PNR-{booking.id}",
            "status": "confirmed",
        }

    @staticmethod
    def _execute_hotel_booking(
        db: Session,
        proposal: Proposal,
        payload: Dict,
        transaction_id: int,
    ) -> Dict:
        """Execute hotel booking (will be implemented in Stage 11)"""
        booking = Booking(
            user_id=proposal.user_id,
            proposal_id=proposal.id,
            transaction_id=transaction_id,
            booking_type="hotel",
            provider="amadeus",
            status="pending",
            payload_json=json.dumps(payload),
        )
        db.add(booking)
        db.commit()
        db.refresh(booking)

        # TODO: Call Amadeus API to book hotel
        return {
            "booking_id": booking.id,
            "confirmation_number": f"MOCK-{booking.id}",
            "status": "confirmed",
        }

    @staticmethod
    def _execute_retail_purchase(
        db: Session,
        proposal: Proposal,
        payload: Dict,
        transaction_id: int,
    ) -> Dict:
        """Execute retail purchase (will be implemented in Stage 13)"""
        booking = Booking(
            user_id=proposal.user_id,
            proposal_id=proposal.id,
            transaction_id=transaction_id,
            booking_type="retail_purchase",
            provider=payload.get("retailer", "amazon"),
            status="pending",
            payload_json=json.dumps(payload),
        )
        db.add(booking)
        db.commit()
        db.refresh(booking)

        # TODO: Implement retail checkout
        return {
            "booking_id": booking.id,
            "confirmation_number": f"MOCK-{booking.id}",
            "status": "confirmed",
        }

    @staticmethod
    def _send_confirmation(
        db: Session,
        proposal: Proposal,
        booking_result: Dict,
    ):
        """Send booking confirmation via WhatsApp"""
        # TODO: Implement WhatsApp confirmation message
        # For now, just log
        print(f"[ExecutionEngine] Booking confirmed: {booking_result}")
        pass
