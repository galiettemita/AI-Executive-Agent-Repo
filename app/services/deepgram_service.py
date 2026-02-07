# app/services/deepgram_service.py
"""
Deepgram Speech-to-Text Service

Handles real-time speech transcription for voice calls using Deepgram's
streaming API.
"""

from __future__ import annotations

import asyncio
import json
import logging
from typing import Any, Callable, Dict, Optional

import httpx

from app.services.circuit_breaker import deepgram_breaker, CircuitBreakerError

logger = logging.getLogger(__name__)


class DeepgramNotConfiguredError(RuntimeError):
    """Raised when Deepgram API key is not set."""
    pass


class DeepgramService:
    """
    Service for Deepgram speech-to-text transcription.

    Supports:
    - Real-time streaming transcription
    - Pre-recorded audio file transcription
    - Multiple language support
    """

    DEEPGRAM_API_URL = "https://api.deepgram.com/v1"

    def __init__(self):
        self.api_key = settings.DEEPGRAM_API_KEY
        if not self.api_key:
            raise DeepgramNotConfiguredError("DEEPGRAM_API_KEY must be set")

    def _get_headers(self) -> Dict[str, str]:
        """Get authorization headers for Deepgram API."""
        return {
            "Authorization": f"Token {self.api_key}",
            "Content-Type": "application/json",
        }

    async def transcribe_audio_file(
        self,
        audio_url: str,
        language: str = "en-US",
        punctuate: bool = True,
        diarize: bool = False,
    ) -> Dict[str, Any]:
        """
        Transcribe a pre-recorded audio file.

        Args:
            audio_url: URL of the audio file
            language: Language code (e.g., "en-US")
            punctuate: Whether to add punctuation
            diarize: Whether to identify different speakers

        Returns:
            Transcription result with text and metadata
        """
        try:
            with deepgram_breaker:
                params = {
                    "url": audio_url,
                    "punctuate": punctuate,
                    "diarize": diarize,
                    "language": language,
                    "model": "nova-2",  # Latest model
                }

                async with httpx.AsyncClient(timeout=60.0) as client:
                    response = await client.post(
                        f"{self.DEEPGRAM_API_URL}/listen",
                        headers=self._get_headers(),
                        json=params,
                    )
                    response.raise_for_status()
                    data = response.json()

                # Extract transcript
                results = data.get("results", {})
                channels = results.get("channels", [])

                if not channels:
                    return {"transcript": "", "confidence": 0.0, "words": []}

                alternatives = channels[0].get("alternatives", [])
                if not alternatives:
                    return {"transcript": "", "confidence": 0.0, "words": []}

                best = alternatives[0]
                return {
                    "transcript": best.get("transcript", ""),
                    "confidence": best.get("confidence", 0.0),
                    "words": best.get("words", []),
                }

        except CircuitBreakerError as e:
            logger.warning(f"Deepgram circuit breaker open: {e}")
            raise ValueError("Transcription service is temporarily unavailable")
        except Exception as e:
            logger.error(f"Deepgram transcription error: {e}")
            raise


class DeepgramLiveTranscription:
    """
    Handles real-time streaming transcription via WebSocket.

    Usage:
        transcriber = DeepgramLiveTranscription(api_key, on_transcript)
        await transcriber.connect()
        await transcriber.send_audio(audio_bytes)
        await transcriber.close()
    """

    DEEPGRAM_WS_URL = "wss://api.deepgram.com/v1/listen"

    def __init__(
        self,
        api_key: str,
        on_transcript: Callable[[str, float, bool], None],
        language: str = "en-US",
        sample_rate: int = 8000,
        encoding: str = "mulaw",
    ):
        """
        Initialize live transcription.

        Args:
            api_key: Deepgram API key
            on_transcript: Callback function(text, confidence, is_final)
            language: Language code
            sample_rate: Audio sample rate (8000 for Twilio)
            encoding: Audio encoding (mulaw for Twilio)
        """
        self.api_key = api_key
        self.on_transcript = on_transcript
        self.language = language
        self.sample_rate = sample_rate
        self.encoding = encoding
        self.websocket = None
        self._receive_task = None
        self._closed = False

    async def connect(self):
        """Establish WebSocket connection to Deepgram."""
        import websockets

        params = (
            f"?encoding={self.encoding}"
            f"&sample_rate={self.sample_rate}"
            f"&language={self.language}"
            f"&punctuate=true"
            f"&interim_results=true"
            f"&endpointing=200"
            f"&model=nova-2"
        )

        headers = {"Authorization": f"Token {self.api_key}"}

        try:
            self.websocket = await websockets.connect(
                f"{self.DEEPGRAM_WS_URL}{params}",
                extra_headers=headers,
                ping_interval=20,
                ping_timeout=20,
            )
            logger.info("Connected to Deepgram streaming API")

            # Start receiving transcripts
            self._receive_task = asyncio.create_task(self._receive_loop())

        except Exception as e:
            logger.error(f"Failed to connect to Deepgram: {e}")
            raise

    async def _receive_loop(self):
        """Receive and process transcription results."""
        try:
            async for message in self.websocket:
                if self._closed:
                    break

                try:
                    data = json.loads(message)

                    # Check for transcript
                    channel = data.get("channel", {})
                    alternatives = channel.get("alternatives", [])

                    if alternatives:
                        transcript = alternatives[0].get("transcript", "")
                        confidence = alternatives[0].get("confidence", 0.0)
                        is_final = data.get("is_final", False)

                        if transcript.strip():
                            self.on_transcript(transcript, confidence, is_final)

                except json.JSONDecodeError:
                    logger.warning(f"Invalid JSON from Deepgram: {message[:100]}")
                except Exception as e:
                    logger.error(f"Error processing Deepgram message: {e}")

        except Exception as e:
            if not self._closed:
                logger.error(f"Deepgram receive loop error: {e}")

    async def send_audio(self, audio_bytes: bytes):
        """
        Send audio data for transcription.

        Args:
            audio_bytes: Raw audio bytes (mulaw encoded for Twilio)
        """
        if self.websocket and not self._closed:
            try:
                await self.websocket.send(audio_bytes)
            except Exception as e:
                logger.error(f"Error sending audio to Deepgram: {e}")

    async def close(self):
        """Close the WebSocket connection."""
        self._closed = True

        if self._receive_task:
            self._receive_task.cancel()
            try:
                await self._receive_task
            except asyncio.CancelledError:
                pass

        if self.websocket:
            try:
                # Send close message
                await self.websocket.send(json.dumps({"type": "CloseStream"}))
                await self.websocket.close()
            except Exception as e:
                logger.warning(f"Error closing Deepgram connection: {e}")

        logger.info("Closed Deepgram connection")


def create_live_transcription(
    on_transcript: Callable[[str, float, bool], None],
    language: str = "en-US",
    sample_rate: int = 8000,
    encoding: str = "mulaw",
) -> DeepgramLiveTranscription:
    """
    Factory function to create a live transcription instance.

    Args:
        on_transcript: Callback for transcript updates
        language: Language code
        sample_rate: Audio sample rate
        encoding: Audio encoding

    Returns:
        DeepgramLiveTranscription instance
    """
    api_key = settings.DEEPGRAM_API_KEY
    if not api_key:
        raise DeepgramNotConfiguredError("DEEPGRAM_API_KEY must be set")

    return DeepgramLiveTranscription(
        api_key=api_key,
        on_transcript=on_transcript,
        language=language,
        sample_rate=sample_rate,
        encoding=encoding,
    )
