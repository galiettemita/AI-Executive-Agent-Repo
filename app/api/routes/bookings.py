# app/api/routes/bookings.py
"""
Booking Management API

Endpoints for:
- Viewing booking details
- Sending confirmation emails with e-tickets
- Cancelling bookings
- Listing user bookings
"""

from __future__ import annotations

import json
import logging
from typing import Optional, Dict, Any, List

from fastapi import APIRouter, Depends, HTTPException, Query
from pydantic import BaseModel
from sqlalchemy.orm import Session
from datetime import datetime

from app.db.database import get_db
from app.db.models import Booking, Transaction, TravelerProfile, User, PaymentMethod
from app.services.amadeus_service import AmadeusService
from app.services.stripe_service import StripeService
from app.services.email_service import send_booking_confirmation, send_cancellation_confirmation
from app.services.eticket_service import generate_eticket_pdf
from app.services.encryption_service import decrypt_pii
from app.services.notification_delivery import send_whatsapp_message

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/bookings", tags=["bookings"])


# ==========================================
# REQUEST/RESPONSE MODELS
# ==========================================

class CancelBookingRequest(BaseModel):
    """Request to cancel a booking."""
    reason: Optional[str] = None
    request_refund: bool = True


class SendConfirmationRequest(BaseModel):
    """Request to send/resend confirmation email."""
    email: str


class ModifyBookingRequest(BaseModel):
    """Request to modify an existing booking by rebooking with a new offer."""
    new_flight_offer: Optional[Dict[str, Any]] = None
    new_hotel_offer: Optional[Dict[str, Any]] = None
    new_hotel_offer_id: Optional[str] = None
    travelers: Optional[List[Dict[str, Any]]] = None
    guests: Optional[List[Dict[str, Any]]] = None
    contact_email: Optional[str] = None
    contact_phone: Optional[str] = None
    cancel_existing: bool = True
    reason: Optional[str] = None


class BookingResponse(BaseModel):
    """Booking response model."""
    id: int
    user_id: str
    booking_type: str
    provider: str
    status: str
    confirmation_number: Optional[str]
    pnr: Optional[str]
    created_at: str
    details: dict


# ==========================================
# ENDPOINTS
# ==========================================

@router.get("/{booking_id}")
def get_booking(
    booking_id: int,
    db: Session = Depends(get_db),
):
    """
    Get booking details.

    Args:
        booking_id: Booking ID
    """
    booking = db.query(Booking).filter(Booking.id == booking_id).first()

    if not booking:
        raise HTTPException(status_code=404, detail="Booking not found")

    # Parse payload
    try:
        payload = json.loads(booking.payload_json) if booking.payload_json else {}
    except json.JSONDecodeError:
        payload = {}

    return {
        "ok": True,
        "booking": {
            "id": booking.id,
            "user_id": booking.user_id,
            "proposal_id": booking.proposal_id,
            "booking_type": booking.booking_type,
            "provider": booking.provider,
            "status": booking.status,
            "confirmation_number": booking.confirmation_number,
            "pnr": booking.pnr,
            "created_at": booking.created_at.isoformat() if booking.created_at else None,
            "updated_at": booking.updated_at.isoformat() if booking.updated_at else None,
            "canceled_at": booking.canceled_at.isoformat() if booking.canceled_at else None,
            "cancellation_reason": booking.cancellation_reason,
            "details": payload,
        },
    }


@router.get("/user/{user_id}")
def list_user_bookings(
    user_id: str,
    status: Optional[str] = Query(None, description="Filter by status"),
    booking_type: Optional[str] = Query(None, description="Filter by type"),
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
    db: Session = Depends(get_db),
):
    """
    List all bookings for a user.

    Args:
        user_id: User ID
        status: Optional status filter (confirmed, cancelled, etc.)
        booking_type: Optional type filter (flight, hotel, etc.)
        limit: Max results
        offset: Pagination offset
    """
    query = db.query(Booking).filter(Booking.user_id == user_id)

    if status:
        query = query.filter(Booking.status == status)
    if booking_type:
        query = query.filter(Booking.booking_type == booking_type)

    total = query.count()
    bookings = query.order_by(Booking.created_at.desc()).offset(offset).limit(limit).all()

    return {
        "ok": True,
        "bookings": [
            {
                "id": b.id,
                "booking_type": b.booking_type,
                "provider": b.provider,
                "status": b.status,
                "confirmation_number": b.confirmation_number,
                "pnr": b.pnr,
                "created_at": b.created_at.isoformat() if b.created_at else None,
            }
            for b in bookings
        ],
        "total": total,
        "limit": limit,
        "offset": offset,
    }


@router.post("/{booking_id}/send-confirmation")
def send_confirmation_email(
    booking_id: int,
    request: SendConfirmationRequest,
    db: Session = Depends(get_db),
):
    """
    Send or resend booking confirmation email with e-ticket.

    Args:
        booking_id: Booking ID
        request: Contains email address
    """
    booking = db.query(Booking).filter(Booking.id == booking_id).first()

    if not booking:
        raise HTTPException(status_code=404, detail="Booking not found")

    if booking.status == "cancelled":
        raise HTTPException(status_code=400, detail="Cannot send confirmation for cancelled booking")

    # Parse payload
    try:
        payload = json.loads(booking.payload_json) if booking.payload_json else {}
    except json.JSONDecodeError:
        payload = {}

    # Generate e-ticket for flights
    eticket_path = None
    if booking.booking_type == "flight":
        eticket_path = generate_eticket_pdf(db, booking_id)

    # Prepare booking details
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

    # Send email
    success = send_booking_confirmation(
        to_email=request.email,
        booking_type=booking.booking_type,
        confirmation_number=booking.confirmation_number or str(booking_id),
        booking_details=booking_details,
        eticket_pdf_path=eticket_path,
    )

    if not success:
        raise HTTPException(
            status_code=500,
            detail="Failed to send confirmation email. Check SMTP configuration."
        )

    return {
        "ok": True,
        "message": f"Confirmation email sent to {request.email}",
        "eticket_generated": eticket_path is not None,
    }


@router.post("/{booking_id}/cancel")
def cancel_booking(
    booking_id: int,
    request: CancelBookingRequest,
    db: Session = Depends(get_db),
):
    """
    Cancel a booking.

    This will:
    1. Call provider API to cancel (if applicable)
    2. Update booking status
    3. Process refund (if requested and applicable)
    4. Send cancellation confirmation email

    Args:
        booking_id: Booking ID
        request: Cancellation details
    """
    booking = db.query(Booking).filter(Booking.id == booking_id).first()

    if not booking:
        raise HTTPException(status_code=404, detail="Booking not found")

    if booking.status == "cancelled":
        raise HTTPException(status_code=400, detail="Booking is already cancelled")

    # Get user email for notification
    user_email = None
    profile = (
        db.query(TravelerProfile)
        .filter(TravelerProfile.user_id == booking.user_id, TravelerProfile.is_default == True)
        .first()
    )
    if profile and profile.email:
        user_email = decrypt_pii(profile.email)

    refund_amount = None
    refund_result = None

    try:
        # 1. Cancel with provider
        if booking.provider == "amadeus":
            amadeus = AmadeusService()

            if booking.booking_type == "flight" and booking.provider_booking_id:
                try:
                    amadeus.cancel_flight_order(booking.provider_booking_id)
                except ValueError as e:
                    # Log but continue - we'll still mark as cancelled internally
                    logger.warning(f"Amadeus flight cancellation failed: {e}")

            elif booking.booking_type == "hotel" and booking.provider_booking_id:
                try:
                    amadeus.cancel_hotel_booking(booking.provider_booking_id)
                except ValueError as e:
                    logger.warning(f"Amadeus hotel cancellation failed: {e}")

        # 2. Process refund if requested
        if request.request_refund and booking.transaction_id:
            transaction = db.query(Transaction).filter(
                Transaction.id == booking.transaction_id
            ).first()

            if transaction and transaction.status == "succeeded":
                try:
                    refund_result = StripeService.refund_payment(
                        db=db,
                        transaction_id=transaction.id,
                        reason=request.reason or "Booking cancelled",
                    )
                    refund_amount = refund_result.get("refund_amount")
                except Exception as e:
                    logger.error(f"Refund failed for transaction {booking.transaction_id}: {e}")

        # 3. Update booking status
        booking.status = "cancelled"
        booking.canceled_at = datetime.utcnow()
        booking.cancellation_reason = request.reason

        db.commit()
        db.refresh(booking)

        # 4. Send cancellation email
        if user_email:
            try:
                payload = json.loads(booking.payload_json) if booking.payload_json else {}
                send_cancellation_confirmation(
                    to_email=user_email,
                    booking_type=booking.booking_type,
                    confirmation_number=booking.confirmation_number or str(booking_id),
                    refund_amount=refund_amount,
                    currency=payload.get("currency", "USD"),
                )
            except Exception as e:
                logger.error(f"Failed to send cancellation email: {e}")

        return {
            "ok": True,
            "message": "Booking cancelled successfully",
            "booking_id": booking_id,
            "status": "cancelled",
            "refund_processed": refund_amount is not None,
            "refund_amount": refund_amount,
        }

    except Exception as e:
        logger.error(f"Cancellation failed for booking {booking_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/{booking_id}/modify")
def modify_booking(
    booking_id: int,
    request: ModifyBookingRequest,
    db: Session = Depends(get_db),
):
    """
    Modify a booking by canceling the existing provider order and rebooking a new offer.

    This will:
    1. Cancel the existing booking with the provider (if possible)
    2. Book the new offer with the provider
    3. Update the booking record with new confirmation details
    4. Send confirmation via email/WhatsApp
    """
    booking = db.query(Booking).filter(Booking.id == booking_id).first()

    if not booking:
        raise HTTPException(status_code=404, detail="Booking not found")

    if booking.status == "cancelled":
        raise HTTPException(status_code=400, detail="Cannot modify a cancelled booking")

    if booking.provider != "amadeus":
        raise HTTPException(status_code=400, detail="Modification supported only for Amadeus bookings")

    try:
        payload = json.loads(booking.payload_json) if booking.payload_json else {}
    except json.JSONDecodeError:
        payload = {}

    amadeus = AmadeusService()

    # 1. Cancel existing provider booking if requested and possible
    if request.cancel_existing and booking.provider_booking_id:
        try:
            if booking.booking_type == "flight":
                amadeus.cancel_flight_order(booking.provider_booking_id)
            elif booking.booking_type == "hotel":
                amadeus.cancel_hotel_booking(booking.provider_booking_id)
        except ValueError as e:
            logger.warning(f"Provider cancellation failed (continuing): {e}")

    # 2. Book new offer
    if booking.booking_type == "flight":
        offer_data = request.new_flight_offer or payload.get("flight_offer")
        if not offer_data:
            raise HTTPException(status_code=400, detail="Missing new_flight_offer for flight modification")

        travelers = request.travelers or payload.get("travelers", [])
        if not travelers:
            default_profile = (
                db.query(TravelerProfile)
                .filter(
                    TravelerProfile.user_id == booking.user_id,
                    TravelerProfile.is_default == True,
                )
                .first()
            )
            if default_profile:
                travelers = [{
                    "first_name": default_profile.first_name,
                    "last_name": default_profile.last_name,
                    "date_of_birth": decrypt_pii(default_profile.date_of_birth),
                    "gender": default_profile.gender,
                    "documents": [{
                        "documentType": "PASSPORT",
                        "number": decrypt_pii(default_profile.passport_number),
                        "expiryDate": decrypt_pii(default_profile.passport_expiry),
                        "issuanceCountry": default_profile.passport_country,
                        "nationality": default_profile.nationality,
                    }] if default_profile.passport_number else [],
                }]

        contact_email = request.contact_email or payload.get("contact_email")
        contact_phone = request.contact_phone or payload.get("contact_phone", "1234567890")

        booking_response = amadeus.book_flight(
            offer_id=offer_data["id"],
            offer_data=offer_data,
            travelers=travelers,
            contact_email=contact_email or booking.user_id,
            contact_phone=contact_phone,
        )

        new_payload = {
            **payload,
            "flight_offer": offer_data,
            "travelers": travelers,
            "contact_email": contact_email or payload.get("contact_email"),
            "contact_phone": contact_phone,
            "booking_response": booking_response,
            "modification_reason": request.reason,
        }

    elif booking.booking_type == "hotel":
        offer_data = request.new_hotel_offer or payload.get("hotel_offer")
        offer_id = request.new_hotel_offer_id or payload.get("offer_id")
        if not offer_data or not offer_id:
            raise HTTPException(status_code=400, detail="Missing new_hotel_offer and/or new_hotel_offer_id")

        guests = request.guests or payload.get("guests", [])
        if not guests:
            default_profile = (
                db.query(TravelerProfile)
                .filter(
                    TravelerProfile.user_id == booking.user_id,
                    TravelerProfile.is_default == True,
                )
                .first()
            )
            if default_profile:
                guests = [{
                    "first_name": default_profile.first_name,
                    "last_name": default_profile.last_name,
                }]

        contact_email = request.contact_email or payload.get("contact_email")
        contact_phone = request.contact_phone or payload.get("contact_phone", "1234567890")

        payment_method = (
            db.query(PaymentMethod)
            .filter(
                PaymentMethod.user_id == booking.user_id,
                PaymentMethod.is_default == True,
            )
            .first()
        )

        if not payment_method:
            raise HTTPException(status_code=400, detail="No default payment method found")

        payment_card = {
            "vendorCode": payment_method.brand.upper() if payment_method.brand else "VI",
            "cardNumber": "4111111111111111",
            "expiryDate": f"{payment_method.exp_year}-{payment_method.exp_month:02d}"
            if payment_method.exp_month and payment_method.exp_year
            else None,
        }

        booking_response = amadeus.book_hotel(
            offer_id=offer_id,
            offer_data=offer_data,
            guests=guests,
            contact_email=contact_email or booking.user_id,
            contact_phone=contact_phone,
            payment_card=payment_card,
        )

        new_payload = {
            **payload,
            "hotel_offer": offer_data,
            "offer_id": offer_id,
            "guests": guests,
            "contact_email": contact_email or payload.get("contact_email"),
            "contact_phone": contact_phone,
            "booking_response": booking_response,
            "modification_reason": request.reason,
        }
    else:
        raise HTTPException(status_code=400, detail="Modification not supported for this booking type")

    # 3. Update booking record
    modification_entry = {
        "modified_at": datetime.utcnow().isoformat(),
        "old_confirmation_number": booking.confirmation_number,
        "old_provider_booking_id": booking.provider_booking_id,
        "reason": request.reason,
    }

    history = payload.get("modification_history", [])
    if isinstance(history, list):
        history.append(modification_entry)
    else:
        history = [modification_entry]

    new_payload["modification_history"] = history

    booking.confirmation_number = booking_response.get("confirmation_number")
    booking.pnr = booking_response.get("pnr", booking.pnr)
    booking.provider_booking_id = booking_response.get("booking_id")
    booking.status = "confirmed"
    booking.payload_json = json.dumps(new_payload)

    db.commit()
    db.refresh(booking)

    # 4. Send confirmation (best-effort)
    eticket_path = None
    if booking.booking_type == "flight":
        eticket_path = generate_eticket_pdf(db, booking.id)

    booking_details = {
        "confirmation_number": booking.confirmation_number,
        "pnr": booking.pnr,
        "total_price": new_payload.get("total_price", 0),
        "currency": new_payload.get("currency", "USD"),
        "itineraries": new_payload.get("itineraries", []),
        "hotel_name": new_payload.get("hotel_name"),
        "check_in": new_payload.get("check_in"),
        "check_out": new_payload.get("check_out"),
    }

    if contact_email:
        send_booking_confirmation(
            to_email=contact_email,
            booking_type=booking.booking_type,
            confirmation_number=booking.confirmation_number or str(booking.id),
            booking_details=booking_details,
            eticket_pdf_path=eticket_path,
        )

    whatsapp_lines = [
        "Your booking has been modified and confirmed.",
        f"Type: {booking.booking_type}",
        f"Confirmation: {booking.confirmation_number or booking.id}",
    ]
    if booking.pnr:
        whatsapp_lines.append(f"PNR: {booking.pnr}")
    if contact_email:
        whatsapp_lines.append(f"Confirmation sent to: {contact_email}")

    send_whatsapp_message(booking.user_id, "\n".join(whatsapp_lines))

    return {
        "ok": True,
        "message": "Booking modified successfully",
        "booking_id": booking.id,
        "confirmation_number": booking.confirmation_number,
        "status": booking.status,
    }


@router.get("/{booking_id}/eticket")
def get_eticket(
    booking_id: int,
    regenerate: bool = Query(False, description="Regenerate e-ticket"),
    db: Session = Depends(get_db),
):
    """
    Get or generate e-ticket for a flight booking.

    Args:
        booking_id: Booking ID
        regenerate: Force regenerate e-ticket
    """
    booking = db.query(Booking).filter(Booking.id == booking_id).first()

    if not booking:
        raise HTTPException(status_code=404, detail="Booking not found")

    if booking.booking_type != "flight":
        raise HTTPException(status_code=400, detail="E-tickets are only available for flight bookings")

    # Generate e-ticket
    eticket_path = generate_eticket_pdf(db, booking_id)

    if not eticket_path:
        raise HTTPException(status_code=500, detail="Failed to generate e-ticket")

    return {
        "ok": True,
        "eticket_path": eticket_path,
        "booking_id": booking_id,
        "confirmation_number": booking.confirmation_number,
    }
