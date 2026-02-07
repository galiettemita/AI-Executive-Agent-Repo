# app/services/tts_elevenlabs.py

from __future__ import annotations

from typing import AsyncGenerator, Optional

import httpx
from app.core.config import settings


async def stream_tts_audio(
    text: str,
    voice_id: Optional[str] = None,
) -> AsyncGenerator[bytes, None]:
    """
    Stream ElevenLabs TTS audio in 8k mulaw suitable for Twilio Media Streams.
    """
    api_key = settings.ELEVENLABS_API_KEY or ""
    if not api_key:
        raise RuntimeError("ELEVENLABS_API_KEY not configured")

    voice_id = voice_id or settings.ELEVENLABS_VOICE_ID
    url = f"https://api.elevenlabs.io/v1/text-to-speech/{voice_id}/stream"

    headers = {
        "xi-api-key": api_key,
        "accept": "audio/mpeg",
        "content-type": "application/json",
    }

    payload = {
        "text": text,
        "model_id": settings.ELEVENLABS_MODEL_ID,
        "voice_settings": {
            "stability": 0.4,
            "similarity_boost": 0.6,
            "style": 0.2,
            "use_speaker_boost": True,
        },
        "output_format": "ulaw_8000",
    }

    async with httpx.AsyncClient(timeout=30) as client:
        async with client.stream("POST", url, headers=headers, json=payload) as resp:
            resp.raise_for_status()
            async for chunk in resp.aiter_bytes():
                if chunk:
                    yield chunk
