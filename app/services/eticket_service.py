# app/services/eticket_service.py
"""
E-Ticket Service

Generates e-ticket PDFs for flight bookings.
Uses ReportLab (same as invoice_service.py).
"""

from __future__ import annotations

import os
import json
import logging
from datetime import datetime
from pathlib import Path
from typing import Dict, Any, Optional

from reportlab.lib import colors
from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.units import inch
from reportlab.platypus import (
    SimpleDocTemplate,
    Paragraph,
    Spacer,
    Table,
    TableStyle,
    Image,
)
from reportlab.graphics.barcode import qr
from reportlab.graphics.shapes import Drawing
from reportlab.graphics import renderPDF

from sqlalchemy.orm import Session
from app.db.models import Booking, TravelerProfile
from app.services.encryption_service import decrypt_pii

logger = logging.getLogger(__name__)

# Ensure etickets directory exists
ETICKETS_DIR = Path("etickets")
ETICKETS_DIR.mkdir(exist_ok=True)


class ETicketService:
    """Service for generating e-ticket PDFs."""

    @staticmethod
    def generate_eticket_pdf(
        db: Session,
        booking_id: int,
    ) -> Optional[str]:
        """
        Generate an e-ticket PDF for a flight booking.

        Args:
            db: Database session
            booking_id: Booking ID

        Returns:
            Path to generated PDF, or None if generation fails
        """
        # Get booking
        booking = db.query(Booking).filter(Booking.id == booking_id).first()
        if not booking:
            logger.error(f"Booking {booking_id} not found")
            return None

        if booking.booking_type != "flight":
            logger.error(f"Booking {booking_id} is not a flight booking")
            return None

        # Parse booking payload
        try:
            payload = json.loads(booking.payload_json) if booking.payload_json else {}
        except json.JSONDecodeError:
            payload = {}

        # Get traveler info
        travelers = payload.get("travelers", [])
        if not travelers:
            # Try to get from traveler profiles
            profiles = (
                db.query(TravelerProfile)
                .filter(TravelerProfile.user_id == booking.user_id)
                .all()
            )
            for profile in profiles:
                travelers.append({
                    "first_name": profile.first_name,
                    "last_name": profile.last_name,
                })

        # Generate PDF
        timestamp = datetime.utcnow().strftime("%Y%m%d_%H%M%S")
        filename = f"eticket_{booking.confirmation_number or booking_id}_{timestamp}.pdf"
        filepath = ETICKETS_DIR / filename

        try:
            ETicketService._create_eticket_pdf(
                filepath=str(filepath),
                booking=booking,
                payload=payload,
                travelers=travelers,
            )
            logger.info(f"E-ticket generated: {filepath}")
            return str(filepath)
        except Exception as e:
            logger.error(f"Failed to generate e-ticket: {e}")
            return None

    @staticmethod
    def _create_eticket_pdf(
        filepath: str,
        booking: Booking,
        payload: Dict[str, Any],
        travelers: list,
    ):
        """Create the actual e-ticket PDF."""
        doc = SimpleDocTemplate(
            filepath,
            pagesize=letter,
            rightMargin=0.5 * inch,
            leftMargin=0.5 * inch,
            topMargin=0.5 * inch,
            bottomMargin=0.5 * inch,
        )

        styles = getSampleStyleSheet()
        elements = []

        # Custom styles
        title_style = ParagraphStyle(
            "Title",
            parent=styles["Heading1"],
            fontSize=24,
            textColor=colors.HexColor("#007bff"),
            alignment=1,  # Center
        )

        header_style = ParagraphStyle(
            "Header",
            parent=styles["Heading2"],
            fontSize=14,
            textColor=colors.HexColor("#333333"),
        )

        # ====== HEADER ======
        elements.append(Paragraph("✈️ E-TICKET", title_style))
        elements.append(Spacer(1, 0.2 * inch))

        # Confirmation info
        conf_data = [
            ["Confirmation Number:", booking.confirmation_number or "N/A"],
            ["PNR:", booking.pnr or booking.confirmation_number or "N/A"],
            ["Booking Date:", booking.created_at.strftime("%B %d, %Y") if booking.created_at else "N/A"],
            ["Status:", booking.status.upper()],
        ]

        conf_table = Table(conf_data, colWidths=[2 * inch, 4 * inch])
        conf_table.setStyle(TableStyle([
            ("BACKGROUND", (0, 0), (0, -1), colors.HexColor("#f0f0f0")),
            ("FONTNAME", (0, 0), (0, -1), "Helvetica-Bold"),
            ("FONTNAME", (1, 0), (1, -1), "Helvetica"),
            ("FONTSIZE", (0, 0), (-1, -1), 11),
            ("PADDING", (0, 0), (-1, -1), 8),
            ("GRID", (0, 0), (-1, -1), 0.5, colors.grey),
        ]))
        elements.append(conf_table)
        elements.append(Spacer(1, 0.3 * inch))

        # ====== PASSENGER INFO ======
        elements.append(Paragraph("Passenger Information", header_style))
        elements.append(Spacer(1, 0.1 * inch))

        passenger_data = [["#", "Passenger Name"]]
        for idx, traveler in enumerate(travelers, 1):
            name = f"{traveler.get('first_name', '')} {traveler.get('last_name', '')}".strip()
            passenger_data.append([str(idx), name or "Guest"])

        if len(passenger_data) > 1:
            passenger_table = Table(passenger_data, colWidths=[0.5 * inch, 5.5 * inch])
            passenger_table.setStyle(TableStyle([
                ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#007bff")),
                ("TEXTCOLOR", (0, 0), (-1, 0), colors.white),
                ("FONTNAME", (0, 0), (-1, 0), "Helvetica-Bold"),
                ("FONTNAME", (0, 1), (-1, -1), "Helvetica"),
                ("FONTSIZE", (0, 0), (-1, -1), 11),
                ("PADDING", (0, 0), (-1, -1), 8),
                ("GRID", (0, 0), (-1, -1), 0.5, colors.grey),
                ("ALIGN", (0, 0), (0, -1), "CENTER"),
            ]))
            elements.append(passenger_table)
        elements.append(Spacer(1, 0.3 * inch))

        # ====== FLIGHT DETAILS ======
        elements.append(Paragraph("Flight Details", header_style))
        elements.append(Spacer(1, 0.1 * inch))

        # Get itineraries from payload
        itineraries = payload.get("itineraries", [])
        raw_order = payload.get("raw_order", {})

        if not itineraries and raw_order:
            # Try to get from raw_order
            flight_offers = raw_order.get("flightOffers", [])
            if flight_offers:
                itineraries = flight_offers[0].get("itineraries", [])

        for idx, itinerary in enumerate(itineraries):
            direction = "OUTBOUND FLIGHT" if idx == 0 else "RETURN FLIGHT"
            elements.append(Paragraph(f"<b>{direction}</b>", styles["Normal"]))
            elements.append(Spacer(1, 0.05 * inch))

            segments = itinerary.get("segments", [])
            flight_data = [["Flight", "From", "To", "Departure", "Arrival"]]

            for seg in segments:
                dep = seg.get("departure", {})
                arr = seg.get("arrival", {})

                flight_num = f"{seg.get('carrier', '')} {seg.get('flight_number', seg.get('number', ''))}"
                from_airport = dep.get("airport", dep.get("iataCode", ""))
                to_airport = arr.get("airport", arr.get("iataCode", ""))

                dep_time = dep.get("time", dep.get("at", ""))
                arr_time = arr.get("time", arr.get("at", ""))

                # Format times
                if dep_time and len(dep_time) >= 16:
                    dep_time = dep_time[:16].replace("T", " ")
                if arr_time and len(arr_time) >= 16:
                    arr_time = arr_time[:16].replace("T", " ")

                flight_data.append([
                    flight_num,
                    from_airport,
                    to_airport,
                    dep_time,
                    arr_time,
                ])

            if len(flight_data) > 1:
                flight_table = Table(
                    flight_data,
                    colWidths=[1 * inch, 1 * inch, 1 * inch, 1.5 * inch, 1.5 * inch]
                )
                flight_table.setStyle(TableStyle([
                    ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#6c757d")),
                    ("TEXTCOLOR", (0, 0), (-1, 0), colors.white),
                    ("FONTNAME", (0, 0), (-1, 0), "Helvetica-Bold"),
                    ("FONTNAME", (0, 1), (-1, -1), "Helvetica"),
                    ("FONTSIZE", (0, 0), (-1, -1), 9),
                    ("PADDING", (0, 0), (-1, -1), 6),
                    ("GRID", (0, 0), (-1, -1), 0.5, colors.grey),
                    ("ALIGN", (0, 0), (-1, -1), "CENTER"),
                ]))
                elements.append(flight_table)
                elements.append(Spacer(1, 0.2 * inch))

        # ====== PRICE SUMMARY ======
        elements.append(Paragraph("Price Summary", header_style))
        elements.append(Spacer(1, 0.1 * inch))

        total_price = payload.get("total_price", 0)
        currency = payload.get("currency", "USD")

        price_data = [
            ["Total Fare:", f"${total_price:.2f} {currency}"],
        ]

        price_table = Table(price_data, colWidths=[2 * inch, 4 * inch])
        price_table.setStyle(TableStyle([
            ("FONTNAME", (0, 0), (-1, -1), "Helvetica-Bold"),
            ("FONTSIZE", (0, 0), (-1, -1), 14),
            ("TEXTCOLOR", (1, 0), (1, -1), colors.HexColor("#28a745")),
            ("PADDING", (0, 0), (-1, -1), 8),
        ]))
        elements.append(price_table)
        elements.append(Spacer(1, 0.3 * inch))

        # ====== QR CODE ======
        # Generate QR code with booking reference
        qr_data = f"BOOKING:{booking.confirmation_number or booking_id}"
        qr_code = qr.QrCodeWidget(qr_data)
        bounds = qr_code.getBounds()
        width = bounds[2] - bounds[0]
        height = bounds[3] - bounds[1]
        d = Drawing(1.5 * inch, 1.5 * inch, transform=[1.5 * inch / width, 0, 0, 1.5 * inch / height, 0, 0])
        d.add(qr_code)
        elements.append(d)
        elements.append(Spacer(1, 0.1 * inch))
        elements.append(Paragraph(
            "<para align='center'><font size='8'>Scan for mobile check-in</font></para>",
            styles["Normal"]
        ))
        elements.append(Spacer(1, 0.3 * inch))

        # ====== IMPORTANT NOTES ======
        elements.append(Paragraph("Important Information", header_style))
        elements.append(Spacer(1, 0.1 * inch))

        notes = [
            "• Please arrive at the airport at least 2 hours before domestic flights and 3 hours before international flights.",
            "• Carry a valid government-issued photo ID.",
            "• Check baggage allowance with your airline before traveling.",
            "• This e-ticket is your official travel document. Present it at check-in.",
            "• For changes or cancellations, contact support or visit our website.",
        ]

        for note in notes:
            elements.append(Paragraph(note, styles["Normal"]))
            elements.append(Spacer(1, 0.05 * inch))

        # ====== FOOTER ======
        elements.append(Spacer(1, 0.5 * inch))
        footer_style = ParagraphStyle(
            "Footer",
            parent=styles["Normal"],
            fontSize=8,
            textColor=colors.grey,
            alignment=1,
        )
        elements.append(Paragraph(
            f"Generated by AI Shopping Assistant on {datetime.utcnow().strftime('%Y-%m-%d %H:%M UTC')}",
            footer_style
        ))
        elements.append(Paragraph(
            "This is an electronic ticket. No physical ticket is required.",
            footer_style
        ))

        # Build PDF
        doc.build(elements)


def generate_eticket_pdf(db: Session, booking_id: int) -> Optional[str]:
    """Convenience function to generate e-ticket PDF."""
    return ETicketService.generate_eticket_pdf(db, booking_id)
