# app/services/twilio_voice.py

from __future__ import annotations

from typing import Optional
import time

from twilio.rest import Client
from twilio.base.exceptions import TwilioRestException
from twilio.twiml.voice_response import VoiceResponse, Connect
from app.core.config import settings


def _twilio_client() -> Client:
    account_sid = settings.TWILIO_ACCOUNT_SID or ""
    auth_token = settings.TWILIO_AUTH_TOKEN or ""
    if not account_sid or not auth_token:
        raise RuntimeError("Twilio credentials not configured")
    return Client(account_sid, auth_token)


def create_outbound_call(
    to_number: str,
    from_number: Optional[str],
    twiml_url: str,
    status_callback_url: str,
    recording_status_callback_url: str,
) -> str:
    client = _twilio_client()
    from_number = from_number or settings.TWILIO_PHONE_NUMBER or ""
    if not from_number:
        raise RuntimeError("TWILIO_PHONE_NUMBER not configured")

    def _do_call():
        return client.calls.create(
            to=to_number,
            from_=from_number,
            url=twiml_url,
            status_callback=status_callback_url,
            status_callback_event=["initiated", "ringing", "answered", "completed"],
            record=True,
            recording_status_callback=recording_status_callback_url,
            recording_status_callback_event=["completed"],
        )

    attempts = 3
    backoff_seconds = 0.5
    for attempt in range(1, attempts + 1):
        try:
            call = _do_call()
            return call.sid
        except TwilioRestException as exc:
            status = getattr(exc, "status", None)
            should_retry = status in (429,) or (isinstance(status, int) and status >= 500)
            if attempt >= attempts or not should_retry:
                raise
            time.sleep(backoff_seconds * (2 ** (attempt - 1)))


def generate_twiml_stream(
    websocket_url: str,
    greeting: str,
) -> str:
    response = VoiceResponse()
    response.say(greeting, voice="alice", language="en-US")

    connect = Connect()
    connect.stream(url=websocket_url, track="both_tracks")
    response.append(connect)

    return str(response)
