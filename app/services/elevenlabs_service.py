# app/services/elevenlabs_service.py
"""
ElevenLabs Text-to-Speech Service

Provides natural-sounding text-to-speech conversion for voice calls
using ElevenLabs' API.
"""

from __future__ import annotations

import logging
from typing import Any, AsyncGenerator, Dict, List, Optional

import httpx

from app.services.circuit_breaker import elevenlabs_breaker, CircuitBreakerError
from app.core.config import settings

logger = logging.getLogger(__name__)


class ElevenLabsNotConfiguredError(RuntimeError):
    """Raised when ElevenLabs API key is not set."""
    pass


class ElevenLabsService:
    """
    Service for ElevenLabs text-to-speech synthesis.

    Supports:
    - Synchronous TTS generation
    - Streaming TTS for real-time playback
    - Multiple voice profiles
    """

    API_URL = "https://api.elevenlabs.io/v1"

    # Pre-defined voice profiles
    VOICES = {
        "professional_female": "21m00Tcm4TlvDq8ikWAM",  # Rachel
        "professional_male": "VR6AewLTigWG4xSOukaG",  # Arnold
        "friendly_female": "EXAVITQu4vr4xnSDxMaL",  # Bella
        "friendly_male": "ErXwobaYiN019PkySvjV",  # Antoni
        "neutral": "pNInz6obpgDQGcFmaJgB",  # Adam
    }

    @staticmethod
    def _get_api_key() -> str:
        """Get the ElevenLabs API key."""
        api_key = settings.ELEVENLABS_API_KEY
        if not api_key:
            raise ElevenLabsNotConfiguredError("ELEVENLABS_API_KEY must be set")
        return api_key

    @staticmethod
    def _get_default_voice() -> str:
        """Get the default voice ID."""
        return settings.ELEVENLABS_DEFAULT_VOICE

    @classmethod
    def get_headers(cls) -> Dict[str, str]:
        """Get authorization headers for ElevenLabs API."""
        return {
            "xi-api-key": cls._get_api_key(),
            "Content-Type": "application/json",
        }

    @classmethod
    def text_to_speech(
        cls,
        text: str,
        voice_id: Optional[str] = None,
        model_id: str = "eleven_monolingual_v1",
        stability: float = 0.5,
        similarity_boost: float = 0.75,
        output_format: str = "mp3_44100_128",
    ) -> bytes:
        """
        Convert text to speech audio.

        Args:
            text: Text to convert
            voice_id: ElevenLabs voice ID (uses default if not specified)
            model_id: TTS model to use
            stability: Voice stability (0-1)
            similarity_boost: Voice similarity boost (0-1)
            output_format: Audio output format

        Returns:
            Audio bytes (MP3)
        """
        voice_id = voice_id or cls._get_default_voice()

        try:
            with elevenlabs_breaker:
                response = httpx.post(
                    f"{cls.API_URL}/text-to-speech/{voice_id}",
                    headers=cls.get_headers(),
                    json={
                        "text": text,
                        "model_id": model_id,
                        "voice_settings": {
                            "stability": stability,
                            "similarity_boost": similarity_boost,
                        },
                    },
                    params={"output_format": output_format},
                    timeout=30.0,
                )
                response.raise_for_status()
                return response.content

        except CircuitBreakerError as e:
            logger.warning(f"ElevenLabs circuit breaker open: {e}")
            raise ValueError("Text-to-speech service is temporarily unavailable")
        except httpx.HTTPStatusError as e:
            logger.error(f"ElevenLabs API error: {e.response.status_code} - {e.response.text}")
            raise ValueError(f"TTS API error: {e.response.status_code}")
        except Exception as e:
            logger.error(f"ElevenLabs TTS error: {e}")
            raise

    @classmethod
    async def stream_tts(
        cls,
        text: str,
        voice_id: Optional[str] = None,
        model_id: str = "eleven_monolingual_v1",
        stability: float = 0.5,
        similarity_boost: float = 0.75,
        output_format: str = "ulaw_8000",  # Twilio-compatible format
    ) -> AsyncGenerator[bytes, None]:
        """
        Stream text-to-speech audio for real-time playback.

        Args:
            text: Text to convert
            voice_id: ElevenLabs voice ID
            model_id: TTS model to use
            stability: Voice stability (0-1)
            similarity_boost: Voice similarity boost (0-1)
            output_format: Audio output format (ulaw_8000 for Twilio)

        Yields:
            Audio chunks
        """
        voice_id = voice_id or cls._get_default_voice()

        try:
            with elevenlabs_breaker:
                async with httpx.AsyncClient(timeout=60.0) as client:
                    async with client.stream(
                        "POST",
                        f"{cls.API_URL}/text-to-speech/{voice_id}/stream",
                        headers=cls.get_headers(),
                        json={
                            "text": text,
                            "model_id": model_id,
                            "voice_settings": {
                                "stability": stability,
                                "similarity_boost": similarity_boost,
                            },
                        },
                        params={"output_format": output_format},
                    ) as response:
                        response.raise_for_status()
                        async for chunk in response.aiter_bytes(chunk_size=1024):
                            yield chunk

        except CircuitBreakerError as e:
            logger.warning(f"ElevenLabs circuit breaker open: {e}")
            raise ValueError("Text-to-speech service is temporarily unavailable")
        except Exception as e:
            logger.error(f"ElevenLabs streaming TTS error: {e}")
            raise

    @classmethod
    def get_available_voices(cls) -> List[Dict[str, Any]]:
        """
        Get list of available voices from ElevenLabs.

        Returns:
            List of voice objects with id, name, and description
        """
        try:
            with elevenlabs_breaker:
                response = httpx.get(
                    f"{cls.API_URL}/voices",
                    headers=cls.get_headers(),
                    timeout=10.0,
                )
                response.raise_for_status()
                data = response.json()

                voices = []
                for voice in data.get("voices", []):
                    voices.append({
                        "id": voice.get("voice_id"),
                        "name": voice.get("name"),
                        "description": voice.get("description", ""),
                        "preview_url": voice.get("preview_url"),
                        "category": voice.get("category", ""),
                        "labels": voice.get("labels", {}),
                    })

                return voices

        except CircuitBreakerError as e:
            logger.warning(f"ElevenLabs circuit breaker open: {e}")
            raise ValueError("Voice service is temporarily unavailable")
        except Exception as e:
            logger.error(f"Failed to get ElevenLabs voices: {e}")
            raise

    @classmethod
    def get_voice_by_name(cls, name: str) -> Optional[str]:
        """
        Get voice ID by friendly name.

        Args:
            name: Friendly name (e.g., "professional_female")

        Returns:
            Voice ID or None if not found
        """
        return cls.VOICES.get(name.lower())

    @classmethod
    def text_to_speech_mulaw(
        cls,
        text: str,
        voice_id: Optional[str] = None,
    ) -> bytes:
        """
        Convert text to mulaw audio for Twilio compatibility.

        Args:
            text: Text to convert
            voice_id: ElevenLabs voice ID

        Returns:
            Audio bytes in mulaw format (8kHz)
        """
        return cls.text_to_speech(
            text=text,
            voice_id=voice_id,
            output_format="ulaw_8000",
        )
