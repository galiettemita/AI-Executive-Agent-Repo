# app/services/stt_deepgram.py

from __future__ import annotations

import asyncio
import json
from typing import Awaitable, Callable, Optional

import websockets
from app.core.config import settings


TranscriptHandler = Callable[[str, bool], Awaitable[None]]


class DeepgramStream:
    """
    Minimal Deepgram streaming client using websockets.
    """

    def __init__(self, on_transcript: TranscriptHandler):
        self.api_key = settings.DEEPGRAM_API_KEY or ""
        self.on_transcript = on_transcript
        self._ws: Optional[websockets.WebSocketClientProtocol] = None
        self._recv_task: Optional[asyncio.Task] = None

    async def connect(self) -> None:
        if not self.api_key:
            raise RuntimeError("DEEPGRAM_API_KEY not configured")

        url = (
            "wss://api.deepgram.com/v1/listen"
            "?encoding=mulaw&sample_rate=8000&channels=1"
            "&interim_results=true&endpointing=150&punctuate=true"
        )
        self._ws = await websockets.connect(
            url,
            extra_headers={"Authorization": f"Token {self.api_key}"},
            ping_interval=20,
            ping_timeout=20,
            max_size=2 ** 24,
        )
        self._recv_task = asyncio.create_task(self._recv_loop())

    async def _recv_loop(self) -> None:
        assert self._ws is not None
        async for message in self._ws:
            try:
                data = json.loads(message)
            except Exception:
                continue

            if data.get("type") == "Results":
                channel = data.get("channel", {})
                alternatives = channel.get("alternatives", [])
                if not alternatives:
                    continue
                transcript = alternatives[0].get("transcript", "").strip()
                is_final = data.get("is_final", False)
                if transcript:
                    await self.on_transcript(transcript, is_final)

    async def send_audio(self, audio_bytes: bytes) -> None:
        if self._ws is None:
            return
        await self._ws.send(audio_bytes)

    async def close(self) -> None:
        if self._ws is not None:
            try:
                await self._ws.close()
            except Exception:
                pass
        if self._recv_task:
            self._recv_task.cancel()
