# app/services/execution_engine.py

from __future__ import annotations

import json
import logging
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
from app.core.config import settings

logger = logging.getLogger(__name__)


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
        jwt_secret = settings.JWT_SECRET
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
            logger.warning("[Webhook] Failed to send execution.started: %s", e)

        # Parse payload
        try:
            payload = json.loads(proposal.payload_json)
        except json.JSONDecodeError:
            raise ValueError("Invalid proposal payload")

        # Voice calls are executed without payments
        if proposal.proposal_type == "voice_call":
            return ExecutionEngine._execute_voice_call_proposal(
                db=db,
                proposal=proposal,
                payload=payload,
                dry_run=dry_run,
            )

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
                logger.warning("[Webhook] Failed to send payment.succeeded: %s", e)

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
                logger.warning("[Webhook] Failed to send booking.confirmed: %s", e)

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
                logger.warning("[Webhook] Failed to send booking.failed: %s", webhook_e)

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
                logger.warning("[Webhook] Failed to send execution.failed: %s", webhook_e)

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
            logger.warning("[Webhook] Failed to send execution.completed: %s", e)

        return {
            "success": True,
            "proposal_id": proposal_id,
            "transaction_id": transaction.id,
            "booking": booking_result,
            "message": "Booking completed successfully",
        }

    @staticmethod
    def _execute_voice_call_proposal(
        db: Session,
        proposal: Proposal,
        payload: Dict[str, Any],
        dry_run: bool = False,
    ) -> Dict[str, Any]:
        """
        Execute a voice call proposal (no payment required).
        """
        if settings.ENABLE_VOICE_CALLS != "1":
            raise ValueError("Voice calls are disabled")
        ExecutionEngine._log_execution_step(
            db, proposal.id, None, "voice_call_validate", "started"
        )

        to_number = payload.get("to_number")
        if not to_number:
            raise ValueError("Voice call proposal missing to_number")

        from_number = payload.get("from_number")
        purpose = payload.get("purpose")
        voice_profile = payload.get("voice_profile")
        script_id = payload.get("script_id")
        script_payload = payload.get("script")

        # Consent check
        from app.services.consent_service import require_consent
        require_consent(db, proposal.user_id, "voice")

        script_json = None
        if script_payload is not None:
            try:
                script_json = json.dumps(script_payload, ensure_ascii=False)
            except Exception:
                script_json = json.dumps({"script": str(script_payload)}, ensure_ascii=False)

        if script_id:
            from app.db.models import VoiceCallScript
            script_row = db.query(VoiceCallScript).filter(VoiceCallScript.id == script_id).first()
            if not script_row:
                raise ValueError("Voice call script not found")
            if not script_json:
                script_json = script_row.script_json

        ExecutionEngine._log_execution_step(
            db, proposal.id, None, "voice_call_validate", "completed"
        )

        if dry_run:
            return {
                "success": True,
                "dry_run": True,
                "message": "Voice call validated (dry run mode)",
                "proposal_id": proposal.id,
            }

        if not settings.APP_BASE_URL:
            raise ValueError("APP_BASE_URL not configured")

        from app.services.voice_call_service import create_voice_call
        from app.services.twilio_voice import create_outbound_call

        call = create_voice_call(
            db=db,
            user_id=proposal.user_id,
            direction="outbound",
            to_number=to_number,
            from_number=from_number,
            purpose=purpose,
            voice_profile=voice_profile,
            proposal_id=proposal.id,
            script_id=script_id,
            script_json=script_json,
            status="initiating",
        )

        twiml_url = f"{settings.APP_BASE_URL}/webhooks/voice/twiml?call_id={call.id}"
        status_url = f"{settings.APP_BASE_URL}/webhooks/voice/status"
        recording_url = f"{settings.APP_BASE_URL}/webhooks/voice/recording"

        ExecutionEngine._log_execution_step(
            db, proposal.id, None, "voice_call_dial", "started"
        )

        call_sid = create_outbound_call(
            to_number=to_number,
            from_number=from_number,
            twiml_url=twiml_url,
            status_callback_url=status_url,
            recording_status_callback_url=recording_url,
        )

        call.call_sid = call_sid
        call.status = "ringing"
        proposal.status = "executing"
        db.commit()
        db.refresh(call)

        ExecutionEngine._log_execution_step(
            db,
            proposal.id,
            None,
            "voice_call_dial",
            "completed",
            metadata={"call_id": call.id, "call_sid": call_sid},
        )

        return {
            "success": True,
            "proposal_id": proposal.id,
            "call_id": call.id,
            "call_sid": call_sid,
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
        """Execute flight booking via Amadeus API"""
        from app.services.amadeus_service import AmadeusService
        from app.db.models import TravelerProfile

        try:
            # Initialize Amadeus service
            amadeus = AmadeusService()

            # Extract booking details from payload
            offer_data = payload.get("flight_offer")
            if not offer_data:
                raise ValueError("Missing flight_offer in payload")

            # Get travelers from payload or user's default profile
            travelers_data = payload.get("travelers", [])
            if not travelers_data:
                # Use default traveler profile
                default_profile = (
                    db.query(TravelerProfile)
                    .filter(
                        TravelerProfile.user_id == proposal.user_id,
                        TravelerProfile.is_default == True,
                    )
                    .first()
                )

                if default_profile:
                    travelers_data = [{
                        "first_name": default_profile.first_name,
                        "last_name": default_profile.last_name,
                        "date_of_birth": default_profile.date_of_birth,
                        "gender": default_profile.gender,
                        "documents": [{
                            "documentType": "PASSPORT",
                            "number": default_profile.passport_number,
                            "expiryDate": default_profile.passport_expiry,
                            "issuanceCountry": default_profile.passport_country,
                            "nationality": default_profile.nationality,
                        }] if default_profile.passport_number else [],
                    }]

            if not travelers_data:
                raise ValueError("No traveler information provided")

            # Get contact info
            contact_email = payload.get("contact_email") or proposal.user_id
            contact_phone = payload.get("contact_phone", "1234567890")

            # Book flight via Amadeus
            booking_response = amadeus.book_flight(
                offer_id=offer_data["id"],
                offer_data=offer_data,
                travelers=travelers_data,
                contact_email=contact_email,
                contact_phone=contact_phone,
            )

            # Create booking record
            booking = Booking(
                user_id=proposal.user_id,
                proposal_id=proposal.id,
                transaction_id=transaction_id,
                booking_type="flight",
                provider="amadeus",
                status="confirmed",
                confirmation_number=booking_response["confirmation_number"],
                provider_booking_id=booking_response["booking_id"],
                pnr=booking_response.get("pnr"),
                payload_json=json.dumps({
                    **payload,
                    "booking_response": booking_response,
                }),
            )
            db.add(booking)
            db.commit()
            db.refresh(booking)

            return {
                "booking_id": booking.id,
                "confirmation_number": booking.confirmation_number,
                "pnr": booking.pnr,
                "status": "confirmed",
                "amadeus_booking_id": booking_response["booking_id"],
            }

        except Exception as e:
            # Log error and raise
            logger.error("[Flight Booking] Error: %s", e)
            raise ValueError(f"Flight booking failed: {str(e)}")

    @staticmethod
    def _execute_hotel_booking(
        db: Session,
        proposal: Proposal,
        payload: Dict,
        transaction_id: int,
    ) -> Dict:
        """Execute hotel booking via Amadeus API"""
        from app.services.amadeus_service import AmadeusService
        from app.db.models import TravelerProfile, PaymentMethod

        try:
            # Initialize Amadeus service
            amadeus = AmadeusService()

            # Extract booking details
            offer_data = payload.get("hotel_offer")
            offer_id = payload.get("offer_id")

            if not offer_data or not offer_id:
                raise ValueError("Missing hotel_offer or offer_id in payload")

            # Get guests from payload or user's default profile
            guests_data = payload.get("guests", [])
            if not guests_data:
                # Use default traveler profile
                default_profile = (
                    db.query(TravelerProfile)
                    .filter(
                        TravelerProfile.user_id == proposal.user_id,
                        TravelerProfile.is_default == True,
                    )
                    .first()
                )

                if default_profile:
                    guests_data = [{
                        "first_name": default_profile.first_name,
                        "last_name": default_profile.last_name,
                    }]

            if not guests_data:
                raise ValueError("No guest information provided")

            # Get contact info
            contact_email = payload.get("contact_email") or proposal.user_id
            contact_phone = payload.get("contact_phone", "1234567890")

            # Get payment method
            payment_method = (
                db.query(PaymentMethod)
                .filter(
                    PaymentMethod.user_id == proposal.user_id,
                    PaymentMethod.is_default == True,
                )
                .first()
            )

            if not payment_method:
                raise ValueError("No default payment method found")

            # Format payment card for Amadeus
            payment_card = {
                "vendorCode": payment_method.brand.upper() if payment_method.brand else "VI",
                "cardNumber": "4111111111111111",  # Note: Use actual card number in production
                "expiryDate": f"{payment_method.exp_year}-{payment_method.exp_month:02d}" if payment_method.exp_month and payment_method.exp_year else None,
            }

            # Book hotel via Amadeus
            booking_response = amadeus.book_hotel(
                offer_id=offer_id,
                offer_data=offer_data,
                guests=guests_data,
                contact_email=contact_email,
                contact_phone=contact_phone,
                payment_card=payment_card,
            )

            # Create booking record
            booking = Booking(
                user_id=proposal.user_id,
                proposal_id=proposal.id,
                transaction_id=transaction_id,
                booking_type="hotel",
                provider="amadeus",
                status="confirmed",
                confirmation_number=booking_response["confirmation_number"],
                provider_booking_id=booking_response["booking_id"],
                payload_json=json.dumps({
                    **payload,
                    "booking_response": booking_response,
                }),
            )
            db.add(booking)
            db.commit()
            db.refresh(booking)

            return {
                "booking_id": booking.id,
                "confirmation_number": booking.confirmation_number,
                "status": "confirmed",
                "hotel_name": booking_response.get("hotel_name"),
                "check_in": booking_response.get("check_in"),
                "check_out": booking_response.get("check_out"),
                "amadeus_booking_id": booking_response["booking_id"],
            }

        except Exception as e:
            # Log error and raise
            logger.error("[Hotel Booking] Error: %s", e)
            raise ValueError(f"Hotel booking failed: {str(e)}")

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
        from app.db.models import Booking, TravelerProfile
        from app.services.email_service import send_booking_confirmation
        from app.services.eticket_service import generate_eticket_pdf
        from app.services.encryption_service import decrypt_pii
        from app.services.notification_delivery import send_whatsapp_message

        booking_id = booking_result.get("booking_id")
        if not booking_id:
            logger.info("[ExecutionEngine] No booking_id in booking_result; skipping confirmation send")
            return

        booking = db.query(Booking).filter(Booking.id == booking_id).first()
        if not booking:
            logger.info("[ExecutionEngine] Booking %s not found; skipping confirmation send", booking_id)
            return

        try:
            payload = json.loads(booking.payload_json) if booking.payload_json else {}
        except json.JSONDecodeError:
            payload = {}

        # Determine contact email (payload, default profile, or user_id if it's an email)
        contact_email = payload.get("contact_email")
        if not contact_email:
            default_profile = (
                db.query(TravelerProfile)
                .filter(
                    TravelerProfile.user_id == booking.user_id,
                    TravelerProfile.is_default == True,
                )
                .first()
            )
            if default_profile and default_profile.email:
                contact_email = decrypt_pii(default_profile.email)

        if not contact_email and "@" in proposal.user_id:
            contact_email = proposal.user_id

        # Generate e-ticket for flight bookings
        eticket_path = None
        if booking.booking_type == "flight":
            eticket_path = generate_eticket_pdf(db, booking.id)

        booking_details = {
            "confirmation_number": booking.confirmation_number,
            "pnr": booking.pnr,
            "total_price": payload.get("total_price", 0),
            "currency": payload.get("currency", "USD"),
            "itineraries": payload.get("itineraries", []),
            "hotel_name": payload.get("hotel_name"),
            "check_in": payload.get("check_in"),
            "check_out": payload.get("check_out"),
        }

        # Send confirmation email (best-effort)
        if contact_email:
            email_ok = send_booking_confirmation(
                to_email=contact_email,
                booking_type=booking.booking_type,
                confirmation_number=booking.confirmation_number or str(booking.id),
                booking_details=booking_details,
                eticket_pdf_path=eticket_path,
            )
            if not email_ok:
                logger.warning("[ExecutionEngine] Confirmation email failed for booking %s", booking.id)

        # Send WhatsApp confirmation (best-effort)
        message_lines = [
            "Your booking is confirmed.",
            f"Type: {booking.booking_type}",
            f"Confirmation: {booking.confirmation_number or booking.id}",
        ]
        if booking.pnr:
            message_lines.append(f"PNR: {booking.pnr}")
        if booking.booking_type == "hotel":
            if booking_details.get("hotel_name"):
                message_lines.append(f"Hotel: {booking_details.get('hotel_name')}")
            if booking_details.get("check_in") and booking_details.get("check_out"):
                message_lines.append(
                    f"Dates: {booking_details.get('check_in')} to {booking_details.get('check_out')}"
                )
        if contact_email and booking.booking_type == "flight":
            message_lines.append(f"E-ticket sent to: {contact_email}")
        elif contact_email:
            message_lines.append(f"Confirmation sent to: {contact_email}")

        send_whatsapp_message(booking.user_id, "\n".join(message_lines))
