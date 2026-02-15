import uuid

import httpx
from fastapi import APIRouter, Request
from fastapi.responses import StreamingResponse

from app.blueprint.contracts import Channel, InboundMessage
from app.core.config import settings
from app.schemas.chat import ChatRequest, ChatResponse
from app.middleware.rate_limiter import rate_limit_user

router = APIRouter()


def _default_brain_base_url() -> str | None:
    if settings.BRAIN_INTERNAL_BASE_URL:
        return settings.BRAIN_INTERNAL_BASE_URL.rstrip("/")
    if settings.ENV in ("staging", "production"):
        return "http://brain.executive-os.local:8000"
    return None


@rate_limit_user()
@router.post("", response_model=ChatResponse)
def chat(request: Request, req: ChatRequest):
    return ChatResponse(reply=f"You said: {req.message}")


@rate_limit_user()
@router.post("/stream")
async def chat_stream(request: Request, req: ChatRequest):
    """
    Gateway → Brain SSE proxy for web clients.
    """
    base = _default_brain_base_url()
    if not base:
        # Local fallback: no Brain service; stream a single chunk.
        async def fallback():
            yield f"data: {req.message}\n\n"
            yield "data: [DONE]\n\n"
        return StreamingResponse(fallback(), media_type="text/event-stream")

    msg = InboundMessage(
        channel=Channel.WEB,
        channel_msg_id=str(uuid.uuid4()),
        user_id=req.user_id,
        text=req.message,
        raw={"source": "web_chat"},
    )

    timeout = httpx.Timeout(connect=5.0, read=None, write=30.0, pool=5.0)

    async def gen():
        async with httpx.AsyncClient(timeout=timeout) as client:
            async with client.stream(
                "POST",
                f"{base}/internal/brain/respond_stream",
                json=msg.model_dump(),
            ) as resp:
                resp.raise_for_status()
                async for chunk in resp.aiter_raw():
                    yield chunk

    return StreamingResponse(gen(), media_type="text/event-stream")
