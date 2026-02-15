from __future__ import annotations

import logging
from typing import Any

from openai import OpenAI

from app.core.config import settings

logger = logging.getLogger(__name__)


SYSTEM_PROMPT = (
    "You are Executive OS, a WhatsApp-first executive assistant.\n"
    "Priorities:\n"
    "1) Be concise and action-oriented.\n"
    "2) If required info is missing, ask exactly one clarifying question.\n"
    "3) Never invent user data; if unsure, ask.\n"
    "4) Keep replies under 1200 characters unless the user explicitly asks for detail.\n"
)


def _client() -> OpenAI:
    if not settings.OPENAI_API_KEY:
        raise RuntimeError("OPENAI_API_KEY not configured")
    return OpenAI(api_key=settings.OPENAI_API_KEY, timeout=25, max_retries=2)


def tier0_reply(user_text: str) -> str:
    t = (user_text or "").strip().lower()
    if any(t.startswith(x) for x in ("hi", "hello", "hey", "yo", "good morning", "good afternoon", "good evening")):
        return "Hey — what do you want to get done right now?"
    if t.startswith("thanks") or t.startswith("thank you") or t.startswith("thx"):
        return "Anytime. What’s next?"
    return "Got it. What should I do next?"


def generate_reply(*, user_text: str, tier: int) -> tuple[str, dict[str, Any]]:
    """
    Returns (reply_text, meta).
    Meta contains model + token usage when available.
    """
    if tier == 0:
        return tier0_reply(user_text), {"tier": 0, "model": "none"}

    model = settings.OPENAI_MODEL or "gpt-4o-mini"
    try:
        client = _client()
        resp = client.chat.completions.create(
            model=model,
            messages=[
                {"role": "system", "content": SYSTEM_PROMPT},
                {"role": "user", "content": user_text},
            ],
            temperature=0.4,
        )
        msg = (resp.choices[0].message.content or "").strip()
        usage = getattr(resp, "usage", None)
        meta = {"tier": tier, "model": model}
        if usage:
            meta["usage"] = {
                "input_tokens": getattr(usage, "prompt_tokens", None),
                "output_tokens": getattr(usage, "completion_tokens", None),
                "total_tokens": getattr(usage, "total_tokens", None),
            }
        return msg or "I’m not sure I got that — can you rephrase?", meta
    except Exception:
        logger.exception("LLM reply generation failed")
        return "I hit an internal error generating a reply. Try again in a minute.", {"tier": tier, "model": model, "error": True}

