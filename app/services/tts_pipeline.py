from __future__ import annotations

import asyncio
import audioop
import io
import wave
from typing import AsyncGenerator, Optional

from app.core.config import settings
from app.services.llm_client import OpenAIProxy as OpenAI
from app.services.tts_elevenlabs import stream_tts_audio as _stream_elevenlabs_audio


def _openai_client() -> OpenAI:
    key = str(settings.OPENAI_API_KEY or "").strip()
    if not key:
        raise RuntimeError("OPENAI_API_KEY not configured")
    return OpenAI(api_key=key, timeout=45, max_retries=1)


def _read_response_bytes(response) -> bytes:
    if isinstance(response, (bytes, bytearray)):
        return bytes(response)
    read_fn = getattr(response, "read", None)
    if callable(read_fn):
        return bytes(read_fn())
    content = getattr(response, "content", None)
    if isinstance(content, (bytes, bytearray)):
        return bytes(content)
    raise RuntimeError("Unsupported OpenAI TTS response payload")


def _openai_tts_wav_bytes(text: str, *, voice: str | None = None) -> bytes:
    if not text.strip():
        return b""
    model = str(settings.OPENAI_TTS_MODEL or "tts-1").strip() or "tts-1"
    selected_voice = str(voice or settings.OPENAI_TTS_VOICE or "alloy").strip() or "alloy"
    response_format = str(settings.OPENAI_TTS_RESPONSE_FORMAT or "wav").strip().lower() or "wav"
    response = _openai_client().audio.speech.create(
        model=model,
        voice=selected_voice,
        input=text[:4000],
        response_format=response_format,
    )
    return _read_response_bytes(response)


def _wav_to_mulaw_8khz_chunks(audio_bytes: bytes, *, chunk_size: int = 320) -> list[bytes]:
    if not audio_bytes:
        return []
    with wave.open(io.BytesIO(audio_bytes), "rb") as wav:
        sample_rate = int(wav.getframerate() or 8000)
        channels = int(wav.getnchannels() or 1)
        sample_width = int(wav.getsampwidth() or 2)
        pcm = wav.readframes(wav.getnframes())

    if channels > 1:
        pcm = audioop.tomono(pcm, sample_width, 0.5, 0.5)
        channels = 1
    if sample_width != 2:
        pcm = audioop.lin2lin(pcm, sample_width, 2)
        sample_width = 2
    if sample_rate != 8000:
        pcm, _state = audioop.ratecv(pcm, sample_width, channels, sample_rate, 8000, None)
        sample_rate = 8000

    mulaw = audioop.lin2ulaw(pcm, sample_width)
    if not mulaw:
        return []
    return [mulaw[i: i + chunk_size] for i in range(0, len(mulaw), chunk_size) if mulaw[i: i + chunk_size]]


async def _stream_openai_mulaw(text: str, *, voice: str | None = None) -> AsyncGenerator[bytes, None]:
    wav_bytes = await asyncio.to_thread(_openai_tts_wav_bytes, text, voice=voice)
    for chunk in _wav_to_mulaw_8khz_chunks(wav_bytes):
        yield chunk


def _provider_order(provider_preference: str | None = None) -> list[str]:
    preferred = str(provider_preference or "").strip().lower()
    available: list[str] = []
    if str(settings.OPENAI_API_KEY or "").strip():
        available.append("openai")
    if str(settings.ELEVENLABS_API_KEY or "").strip():
        available.append("elevenlabs")
    if preferred in available:
        return [preferred] + [provider for provider in available if provider != preferred]
    return available


async def stream_tts_audio(
    text: str,
    voice_id: Optional[str] = None,
    provider_preference: str | None = None,
) -> AsyncGenerator[bytes, None]:
    providers = _provider_order(provider_preference=provider_preference)
    if not providers:
        raise RuntimeError("No TTS provider configured")

    last_error: Exception | None = None
    for provider in providers:
        try:
            if provider == "openai":
                async for chunk in _stream_openai_mulaw(text, voice=voice_id):
                    yield chunk
                return
            async for chunk in _stream_elevenlabs_audio(text, voice_id=voice_id):
                yield chunk
            return
        except Exception as exc:  # pragma: no cover - exercised via fallback tests
            last_error = exc
            continue

    if last_error is not None:
        raise RuntimeError(f"TTS pipeline failed: {last_error}")
    raise RuntimeError("TTS pipeline failed")
