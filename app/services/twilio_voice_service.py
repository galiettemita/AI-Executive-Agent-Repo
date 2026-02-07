# app/services/twilio_voice_service.py
"""
Twilio Voice Service

Handles voice call initiation, status management, and TwiML generation
for AI-powered phone calls.
"""

from __future__ import annotations

import json
import logging
from datetime import datetime
from typing import Any, Dict, Optional

from sqlalchemy.orm import Session
from twilio.rest import Client
from twilio.twiml.voice_response import VoiceResponse, Connect, Stream

from app.db.models import VoiceCall
from app.services.circuit_breaker import twilio_breaker, CircuitBreakerError
from app.core.config import settings

logger = logging.getLogger(__name__)


class TwilioNotConfiguredError(RuntimeError):
    """Raised when Twilio credentials are not set."""
    pass


class TwilioVoiceService:
    """
    Service for Twilio Voice call management.

    Handles:
    - Outbound call initiation
    - TwiML response generation
    - Call status updates
    - Recording management
    """

    def __init__(self):
        self.account_sid = settings.TWILIO_ACCOUNT_SID
        self.auth_token = settings.TWILIO_AUTH_TOKEN
        self.phone_number = settings.TWILIO_PHONE_NUMBER

        if not self.account_sid or not self.auth_token:
            raise TwilioNotConfiguredError(
                "TWILIO_ACCOUNT_SID and TWILIO_AUTH_TOKEN must be set"
            )

        self.client = Client(self.account_sid, self.auth_token)

    def initiate_call(
        self,
        db: Session,
        user_id: str,
        to_number: str,
        purpose: str,
        purpose_details: Optional[Dict[str, Any]] = None,
        record: bool = False,
        webhook_base_url: str = "",
    ) -> VoiceCall:
        """
        Initiate an outbound voice call.

        Args:
            db: Database session
            user_id: User initiating the call
            to_number: Phone number to call (E.164 format)
            purpose: Call purpose (restaurant_reservation, appointment, etc.)
            purpose_details: Additional context for the call
            record: Whether to record the call
            webhook_base_url: Base URL for Twilio webhooks

        Returns:
            VoiceCall record
        """
        if not self.phone_number:
            raise TwilioNotConfiguredError("TWILIO_PHONE_NUMBER must be set")

        # Create the voice call record
        voice_call = VoiceCall(
            user_id=user_id,
            direction="outbound",
            to_number=to_number,
            from_number=self.phone_number,
            purpose=purpose,
            purpose_details=json.dumps(purpose_details) if purpose_details else None,
            status="initiating",
            recording_enabled=record,
        )
        db.add(voice_call)
        db.commit()
        db.refresh(voice_call)

        try:
            with twilio_breaker:
                # Configure callback URLs
                status_callback = f"{webhook_base_url}/webhooks/twilio/voice/status"
                answer_url = f"{webhook_base_url}/webhooks/twilio/voice/answer?call_id={voice_call.id}"

                # Initiate the call
                call = self.client.calls.create(
                    to=to_number,
                    from_=self.phone_number,
                    url=answer_url,
                    status_callback=status_callback,
                    status_callback_event=["initiated", "ringing", "answered", "completed"],
                    status_callback_method="POST",
                    record=record,
                    machine_detection="Enable",  # Detect voicemail
                )

                # Update with Twilio call SID
                voice_call.twilio_call_sid = call.sid
                voice_call.status = "ringing"
                db.commit()

                logger.info(
                    f"Initiated call {voice_call.id} to {to_number}, SID: {call.sid}"
                )

        except CircuitBreakerError as e:
            voice_call.status = "failed"
            voice_call.error_message = "Twilio service temporarily unavailable"
            db.commit()
            logger.warning(f"Twilio circuit breaker open: {e}")
            raise ValueError("Voice calling service is temporarily unavailable")
        except Exception as e:
            voice_call.status = "failed"
            voice_call.error_message = str(e)
            db.commit()
            logger.error(f"Failed to initiate call: {e}")
            raise

        return voice_call

    def generate_twiml_connect_stream(
        self,
        call_id: int,
        websocket_url: str,
        greeting: Optional[str] = None,
    ) -> str:
        """
        Generate TwiML to connect the call to a WebSocket stream.

        Args:
            call_id: Internal call ID
            websocket_url: WebSocket URL for media streaming
            greeting: Optional initial greeting to speak

        Returns:
            TwiML XML string
        """
        response = VoiceResponse()

        # Speak initial greeting if provided
        if greeting:
            response.say(greeting, voice="Polly.Joanna")

        # Connect to media stream
        connect = Connect()
        stream = Stream(url=websocket_url)
        stream.parameter(name="call_id", value=str(call_id))
        connect.append(stream)
        response.append(connect)

        return str(response)

    def generate_twiml_say(self, text: str, voice: str = "Polly.Joanna") -> str:
        """
        Generate simple TwiML to speak text.

        Args:
            text: Text to speak
            voice: Twilio voice to use

        Returns:
            TwiML XML string
        """
        response = VoiceResponse()
        response.say(text, voice=voice)
        return str(response)

    def update_call_status(
        self,
        db: Session,
        twilio_call_sid: str,
        status: str,
        duration: Optional[int] = None,
        recording_url: Optional[str] = None,
        recording_sid: Optional[str] = None,
    ) -> Optional[VoiceCall]:
        """
        Update call status from Twilio webhook.

        Args:
            db: Database session
            twilio_call_sid: Twilio call SID
            status: New status from Twilio
            duration: Call duration in seconds
            recording_url: URL of the recording
            recording_sid: Twilio recording SID

        Returns:
            Updated VoiceCall or None if not found
        """
        voice_call = (
            db.query(VoiceCall)
            .filter(VoiceCall.twilio_call_sid == twilio_call_sid)
            .first()
        )

        if not voice_call:
            logger.warning(f"No call found for SID: {twilio_call_sid}")
            return None

        # Map Twilio status to our status
        status_map = {
            "initiated": "initiating",
            "ringing": "ringing",
            "in-progress": "connected",
            "answered": "connected",
            "completed": "ended",
            "busy": "failed",
            "no-answer": "failed",
            "canceled": "failed",
            "failed": "failed",
        }

        new_status = status_map.get(status.lower(), status.lower())
        voice_call.status = new_status

        if new_status == "connected" and not voice_call.answered_at:
            voice_call.answered_at = datetime.utcnow()

        if new_status in ("ended", "failed"):
            voice_call.ended_at = datetime.utcnow()
            if duration:
                voice_call.duration_seconds = duration

        if recording_url:
            voice_call.recording_url = recording_url
        if recording_sid:
            voice_call.recording_sid = recording_sid

        db.commit()
        logger.info(f"Updated call {voice_call.id} status to {new_status}")

        return voice_call

    def end_call(self, db: Session, call_id: int) -> VoiceCall:
        """
        End an active call.

        Args:
            db: Database session
            call_id: Internal call ID

        Returns:
            Updated VoiceCall
        """
        voice_call = db.query(VoiceCall).filter(VoiceCall.id == call_id).first()

        if not voice_call:
            raise ValueError(f"Call {call_id} not found")

        if voice_call.status in ("ended", "failed"):
            return voice_call

        if not voice_call.twilio_call_sid:
            voice_call.status = "ended"
            voice_call.ended_at = datetime.utcnow()
            db.commit()
            return voice_call

        try:
            with twilio_breaker:
                self.client.calls(voice_call.twilio_call_sid).update(status="completed")
                voice_call.status = "ended"
                voice_call.ended_at = datetime.utcnow()
                db.commit()
                logger.info(f"Ended call {call_id}")
        except CircuitBreakerError as e:
            logger.warning(f"Twilio circuit breaker open when ending call: {e}")
            raise ValueError("Voice calling service is temporarily unavailable")
        except Exception as e:
            logger.error(f"Failed to end call {call_id}: {e}")
            raise

        return voice_call

    def get_recording_url(self, recording_sid: str) -> str:
        """
        Get the download URL for a call recording.

        Args:
            recording_sid: Twilio recording SID

        Returns:
            Recording download URL
        """
        try:
            with twilio_breaker:
                recording = self.client.recordings(recording_sid).fetch()
                return f"https://api.twilio.com{recording.uri.replace('.json', '.mp3')}"
        except CircuitBreakerError as e:
            logger.warning(f"Twilio circuit breaker open: {e}")
            raise ValueError("Voice calling service is temporarily unavailable")
        except Exception as e:
            logger.error(f"Failed to get recording URL: {e}")
            raise
