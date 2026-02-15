# app/services/email_service.py
"""
Email Service

Sends transactional emails for:
- Booking confirmations with e-ticket attachments
- Cancellation confirmations
- Refund notifications

Supports SES (default) and SMTP fallback.
"""

from __future__ import annotations

import logging
import smtplib
import boto3
from botocore.exceptions import BotoCoreError, ClientError
from email.mime.multipart import MIMEMultipart
from email.mime.text import MIMEText
from email.mime.application import MIMEApplication
from typing import List, Optional, Dict, Any
from pathlib import Path
from app.core.config import settings

logger = logging.getLogger(__name__)


class EmailService:
    """Service for sending transactional emails."""

    def __init__(self):
        """Initialize email service with provider settings from environment."""
        self.email_provider = (settings.EMAIL_PROVIDER or "smtp").lower()
        self.smtp_host = settings.SMTP_HOST
        self.smtp_port = settings.SMTP_PORT
        self.smtp_user = settings.SMTP_USER
        self.smtp_password = settings.SMTP_PASSWORD
        self.from_email = settings.FROM_EMAIL
        self.from_name = settings.FROM_NAME
        self.ses_region = settings.SES_REGION or settings.AWS_REGION or settings.S3_REGION or "us-east-1"
        self.ses_configuration_set = settings.SES_CONFIGURATION_SET

    def _get_smtp_connection(self):
        """Create SMTP connection."""
        server = smtplib.SMTP(self.smtp_host, self.smtp_port)
        server.starttls()
        if self.smtp_user and self.smtp_password:
            server.login(self.smtp_user, self.smtp_password)
        return server

    def send_email(
        self,
        to_email: str,
        subject: str,
        html_body: str,
        text_body: Optional[str] = None,
        attachments: Optional[List[Dict[str, Any]]] = None,
    ) -> bool:
        """
        Send an email.

        Args:
            to_email: Recipient email address
            subject: Email subject
            html_body: HTML content
            text_body: Plain text content (optional, derived from html if not provided)
            attachments: List of {"filename": str, "content": bytes, "content_type": str}

        Returns:
            True if sent successfully, False otherwise
        """
        try:
            msg = MIMEMultipart("mixed")
            msg["From"] = f"{self.from_name} <{self.from_email}>"
            msg["To"] = to_email
            msg["Subject"] = subject

            # Create alternative part for text/html
            alt_part = MIMEMultipart("alternative")

            # Add text version
            if text_body:
                alt_part.attach(MIMEText(text_body, "plain"))

            # Add HTML version
            alt_part.attach(MIMEText(html_body, "html"))
            msg.attach(alt_part)

            # Add attachments
            if attachments:
                for attachment in attachments:
                    part = MIMEApplication(
                        attachment["content"],
                        Name=attachment["filename"]
                    )
                    part["Content-Disposition"] = f'attachment; filename="{attachment["filename"]}"'
                    msg.attach(part)

            if self.email_provider == "ses":
                return self._send_via_ses(msg, to_email)

            if not self.smtp_user:
                logger.warning("SMTP not configured - skipping email send")
                return False

            # Send email via SMTP
            with self._get_smtp_connection() as server:
                server.sendmail(self.from_email, to_email, msg.as_string())

            logger.info(f"Email sent successfully to {to_email}: {subject}")
            return True

        except Exception as e:
            logger.error(f"Failed to send email to {to_email}: {e}")
            return False

    def _send_via_ses(self, msg: MIMEMultipart, to_email: str) -> bool:
        try:
            client = boto3.client("ses", region_name=self.ses_region)
            params: Dict[str, Any] = {
                "Source": self.from_email,
                "Destinations": [to_email],
                "RawMessage": {"Data": msg.as_string()},
            }
            if self.ses_configuration_set:
                params["ConfigurationSetName"] = self.ses_configuration_set
            client.send_raw_email(**params)
            logger.info(f"Email sent via SES to {to_email}")
            return True
        except (BotoCoreError, ClientError) as exc:
            logger.error("SES send failed: %s", exc)
            return False

    def send_booking_confirmation(
        self,
        to_email: str,
        booking_type: str,
        confirmation_number: str,
        booking_details: Dict[str, Any],
        eticket_pdf_path: Optional[str] = None,
    ) -> bool:
        """
        Send booking confirmation email.

        Args:
            to_email: Customer email
            booking_type: "flight" or "hotel"
            confirmation_number: Booking confirmation number
            booking_details: Dict with booking info
            eticket_pdf_path: Path to e-ticket PDF (for flights)

        Returns:
            True if sent successfully
        """
        if booking_type == "flight":
            subject = f"Flight Booking Confirmed - {confirmation_number}"
            html_body = self._render_flight_confirmation_email(
                confirmation_number, booking_details
            )
        elif booking_type == "hotel":
            subject = f"Hotel Booking Confirmed - {confirmation_number}"
            html_body = self._render_hotel_confirmation_email(
                confirmation_number, booking_details
            )
        else:
            subject = f"Booking Confirmed - {confirmation_number}"
            html_body = self._render_generic_confirmation_email(
                confirmation_number, booking_details
            )

        # Prepare attachments
        attachments = []
        if eticket_pdf_path and Path(eticket_pdf_path).exists():
            with open(eticket_pdf_path, "rb") as f:
                attachments.append({
                    "filename": f"eticket_{confirmation_number}.pdf",
                    "content": f.read(),
                    "content_type": "application/pdf",
                })

        return self.send_email(to_email, subject, html_body, attachments=attachments)

    def send_cancellation_confirmation(
        self,
        to_email: str,
        booking_type: str,
        confirmation_number: str,
        refund_amount: Optional[float] = None,
        currency: str = "USD",
    ) -> bool:
        """Send cancellation confirmation email."""
        subject = f"Booking Cancelled - {confirmation_number}"

        refund_text = ""
        if refund_amount:
            refund_text = f"""
            <p style="color: #28a745; font-size: 18px;">
                <strong>Refund: ${refund_amount:.2f} {currency}</strong>
            </p>
            <p>Your refund will be processed within 5-10 business days.</p>
            """

        html_body = f"""
        <!DOCTYPE html>
        <html>
        <head>
            <style>
                body {{ font-family: Arial, sans-serif; line-height: 1.6; color: #333; }}
                .container {{ max-width: 600px; margin: 0 auto; padding: 20px; }}
                .header {{ background: #dc3545; color: white; padding: 20px; text-align: center; }}
                .content {{ padding: 20px; background: #f9f9f9; }}
                .footer {{ text-align: center; padding: 20px; font-size: 12px; color: #666; }}
            </style>
        </head>
        <body>
            <div class="container">
                <div class="header">
                    <h1>Booking Cancelled</h1>
                </div>
                <div class="content">
                    <p>Your {booking_type} booking has been cancelled.</p>
                    <p><strong>Confirmation Number:</strong> {confirmation_number}</p>
                    {refund_text}
                    <p>If you have any questions, please contact our support team.</p>
                </div>
                <div class="footer">
                    <p>AI Shopping Assistant</p>
                </div>
            </div>
        </body>
        </html>
        """

        return self.send_email(to_email, subject, html_body)

    def _render_flight_confirmation_email(
        self,
        confirmation_number: str,
        details: Dict[str, Any],
    ) -> str:
        """Render flight confirmation HTML email."""
        # Extract flight details
        pnr = details.get("pnr", confirmation_number)
        total_price = details.get("total_price", 0)
        currency = details.get("currency", "USD")
        itineraries = details.get("itineraries", [])

        # Build flight segments HTML
        segments_html = ""
        for idx, itinerary in enumerate(itineraries):
            direction = "Outbound" if idx == 0 else "Return"
            segments = itinerary.get("segments", [])

            for seg in segments:
                dep = seg.get("departure", {})
                arr = seg.get("arrival", {})
                segments_html += f"""
                <tr>
                    <td style="padding: 10px; border-bottom: 1px solid #ddd;">
                        <strong>{direction}</strong><br>
                        {seg.get("carrier", "")} {seg.get("flight_number", "")}
                    </td>
                    <td style="padding: 10px; border-bottom: 1px solid #ddd;">
                        {dep.get("airport", "")} → {arr.get("airport", "")}<br>
                        <small>{dep.get("time", "")[:16] if dep.get("time") else ""}</small>
                    </td>
                </tr>
                """

        return f"""
        <!DOCTYPE html>
        <html>
        <head>
            <style>
                body {{ font-family: Arial, sans-serif; line-height: 1.6; color: #333; }}
                .container {{ max-width: 600px; margin: 0 auto; padding: 20px; }}
                .header {{ background: #007bff; color: white; padding: 20px; text-align: center; }}
                .confirmation {{ background: #28a745; color: white; padding: 15px; text-align: center; margin: 20px 0; }}
                .content {{ padding: 20px; background: #f9f9f9; }}
                table {{ width: 100%; border-collapse: collapse; }}
                .price {{ font-size: 24px; color: #007bff; text-align: center; margin: 20px 0; }}
                .footer {{ text-align: center; padding: 20px; font-size: 12px; color: #666; }}
            </style>
        </head>
        <body>
            <div class="container">
                <div class="header">
                    <h1>✈️ Flight Booking Confirmed!</h1>
                </div>
                <div class="confirmation">
                    <h2>Confirmation: {confirmation_number}</h2>
                    <p>PNR: {pnr}</p>
                </div>
                <div class="content">
                    <h3>Flight Details</h3>
                    <table>
                        {segments_html}
                    </table>
                    <div class="price">
                        <strong>Total: ${total_price:.2f} {currency}</strong>
                    </div>
                    <p><strong>Important:</strong></p>
                    <ul>
                        <li>Please arrive at the airport at least 2 hours before departure</li>
                        <li>Bring a valid government-issued ID</li>
                        <li>Your e-ticket is attached to this email</li>
                    </ul>
                </div>
                <div class="footer">
                    <p>AI Shopping Assistant</p>
                    <p>Questions? Reply to this email or contact support.</p>
                </div>
            </div>
        </body>
        </html>
        """

    def _render_hotel_confirmation_email(
        self,
        confirmation_number: str,
        details: Dict[str, Any],
    ) -> str:
        """Render hotel confirmation HTML email."""
        hotel_name = details.get("hotel_name", "Hotel")
        check_in = details.get("check_in", "TBD")
        check_out = details.get("check_out", "TBD")
        total_price = details.get("total_price", 0)
        currency = details.get("currency", "USD")

        return f"""
        <!DOCTYPE html>
        <html>
        <head>
            <style>
                body {{ font-family: Arial, sans-serif; line-height: 1.6; color: #333; }}
                .container {{ max-width: 600px; margin: 0 auto; padding: 20px; }}
                .header {{ background: #6f42c1; color: white; padding: 20px; text-align: center; }}
                .confirmation {{ background: #28a745; color: white; padding: 15px; text-align: center; margin: 20px 0; }}
                .content {{ padding: 20px; background: #f9f9f9; }}
                .details {{ background: white; padding: 15px; border-radius: 8px; margin: 10px 0; }}
                .price {{ font-size: 24px; color: #6f42c1; text-align: center; margin: 20px 0; }}
                .footer {{ text-align: center; padding: 20px; font-size: 12px; color: #666; }}
            </style>
        </head>
        <body>
            <div class="container">
                <div class="header">
                    <h1>🏨 Hotel Booking Confirmed!</h1>
                </div>
                <div class="confirmation">
                    <h2>Confirmation: {confirmation_number}</h2>
                </div>
                <div class="content">
                    <div class="details">
                        <h3>{hotel_name}</h3>
                        <p><strong>Check-in:</strong> {check_in}</p>
                        <p><strong>Check-out:</strong> {check_out}</p>
                    </div>
                    <div class="price">
                        <strong>Total: ${total_price:.2f} {currency}</strong>
                    </div>
                    <p><strong>Important:</strong></p>
                    <ul>
                        <li>Check-in time is typically 3:00 PM</li>
                        <li>Check-out time is typically 11:00 AM</li>
                        <li>Bring a valid ID and the credit card used for booking</li>
                    </ul>
                </div>
                <div class="footer">
                    <p>AI Shopping Assistant</p>
                    <p>Questions? Reply to this email or contact support.</p>
                </div>
            </div>
        </body>
        </html>
        """

    def _render_generic_confirmation_email(
        self,
        confirmation_number: str,
        details: Dict[str, Any],
    ) -> str:
        """Render generic confirmation email."""
        total_price = details.get("total_price", 0)
        currency = details.get("currency", "USD")
        description = details.get("description", "Your order")

        return f"""
        <!DOCTYPE html>
        <html>
        <head>
            <style>
                body {{ font-family: Arial, sans-serif; line-height: 1.6; color: #333; }}
                .container {{ max-width: 600px; margin: 0 auto; padding: 20px; }}
                .header {{ background: #28a745; color: white; padding: 20px; text-align: center; }}
                .content {{ padding: 20px; background: #f9f9f9; }}
                .footer {{ text-align: center; padding: 20px; font-size: 12px; color: #666; }}
            </style>
        </head>
        <body>
            <div class="container">
                <div class="header">
                    <h1>Order Confirmed!</h1>
                </div>
                <div class="content">
                    <p><strong>Confirmation Number:</strong> {confirmation_number}</p>
                    <p><strong>Description:</strong> {description}</p>
                    <p><strong>Total:</strong> ${total_price:.2f} {currency}</p>
                </div>
                <div class="footer">
                    <p>AI Shopping Assistant</p>
                </div>
            </div>
        </body>
        </html>
        """


# Singleton instance
_email_service = None


def get_email_service() -> EmailService:
    """Get singleton email service instance."""
    global _email_service
    if _email_service is None:
        _email_service = EmailService()
    return _email_service


def send_booking_confirmation(
    to_email: str,
    booking_type: str,
    confirmation_number: str,
    booking_details: Dict[str, Any],
    eticket_pdf_path: Optional[str] = None,
) -> bool:
    """Convenience function to send booking confirmation."""
    return get_email_service().send_booking_confirmation(
        to_email, booking_type, confirmation_number, booking_details, eticket_pdf_path
    )


def send_cancellation_confirmation(
    to_email: str,
    booking_type: str,
    confirmation_number: str,
    refund_amount: Optional[float] = None,
    currency: str = "USD",
) -> bool:
    """Convenience function to send cancellation confirmation."""
    return get_email_service().send_cancellation_confirmation(
        to_email, booking_type, confirmation_number, refund_amount, currency
    )
