from __future__ import annotations

import asyncio
import io
import wave

from app.core.config import settings
from app.services import tts_pipeline


def _wav_bytes_for_test() -> bytes:
    out = io.BytesIO()
    with wave.open(out, "wb") as wav:
        wav.setnchannels(1)
        wav.setsampwidth(2)
        wav.setframerate(8000)
        wav.writeframes(b"\x00\x00" * 800)
    return out.getvalue()


def _collect_stream(text: str) -> bytes:
    async def _run() -> bytes:
        chunks: list[bytes] = []
        async for chunk in tts_pipeline.stream_tts_audio(text):
            chunks.append(chunk)
        return b"".join(chunks)

    return asyncio.run(_run())


def test_tts_pipeline_openai_primary(monkeypatch):
    monkeypatch.setattr(settings, "OPENAI_API_KEY", "test-key")
    monkeypatch.setattr(settings, "ELEVENLABS_API_KEY", None)
    monkeypatch.setattr(tts_pipeline, "_openai_tts_wav_bytes", lambda text, voice=None: _wav_bytes_for_test())

    audio = _collect_stream("hello from openai")
    assert isinstance(audio, (bytes, bytearray))
    assert len(audio) > 0


def test_tts_pipeline_falls_back_to_elevenlabs(monkeypatch):
    monkeypatch.setattr(settings, "OPENAI_API_KEY", "test-key")
    monkeypatch.setattr(settings, "ELEVENLABS_API_KEY", "eleven-key")

    def _raise_openai(*args, **kwargs):
        raise RuntimeError("openai unavailable")

    async def _fake_elevenlabs(text: str, voice_id: str | None = None):
        _ = text, voice_id
        yield b"abc"
        yield b"def"

    monkeypatch.setattr(tts_pipeline, "_openai_tts_wav_bytes", _raise_openai)
    monkeypatch.setattr(tts_pipeline, "_stream_elevenlabs_audio", _fake_elevenlabs)

    audio = _collect_stream("fallback")
    assert audio == b"abcdef"
