from __future__ import annotations

import hashlib
import json
import logging
import time
from typing import Any

import httpx
from openai import OpenAI

from app.blueprint.contracts import ToolCall, ToolResult
from app.core.config import settings
from app.services.semantic_cache import get_cached_response, put_cached_response

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
    if any(
        t.startswith(x)
        for x in (
            "hi",
            "hello",
            "hey",
            "yo",
            "good morning",
            "good afternoon",
            "good evening",
        )
    ):
        return "Hey — what do you want to get done right now?"
    if t.startswith("thanks") or t.startswith("thank you") or t.startswith("thx"):
        return "Anytime. What’s next?"
    return "Got it. What should I do next?"


def _default_hands_base_url() -> str | None:
    if settings.HANDS_INTERNAL_BASE_URL:
        return settings.HANDS_INTERNAL_BASE_URL.rstrip("/")
    if settings.ENV in ("staging", "production"):
        # Requires ECS Cloud Map namespace `executive-os.local`.
        return "http://hands.executive-os.local:8000"
    return None


def _tool_idempotency_key(*, run_id: str | None, tool: str, args: dict[str, Any]) -> str:
    payload = json.dumps({"run_id": run_id, "tool": tool, "args": args}, sort_keys=True, ensure_ascii=False)
    digest = hashlib.sha256(payload.encode("utf-8")).hexdigest()
    return f"tool:{tool}:{digest}"


def _execute_tool(
    *,
    tool: str,
    args: dict[str, Any],
    user_id: str | None,
    run_id: str | None,
    timeout_s: float = 10.0,
) -> ToolResult:
    """
    Execute a tool via Hands internal API (Brain → Hands).
    """
    base = _default_hands_base_url()
    if not base:
        return ToolResult(tool=tool, ok=False, error="Hands service not configured")

    call = ToolCall(
        tool=tool,
        args=args or {},
        user_id=user_id,
        run_id=run_id,
        idempotency_key=_tool_idempotency_key(run_id=run_id, tool=tool, args=args or {}),
    )

    started = time.perf_counter()
    try:
        with httpx.Client(timeout=timeout_s) as client:
            resp = client.post(f"{base}/internal/hands/execute", json=call.model_dump())
            resp.raise_for_status()
            return ToolResult.model_validate(resp.json())
    except Exception as exc:
        elapsed_ms = int((time.perf_counter() - started) * 1000)
        logger.warning("Hands tool execution failed tool=%s latency_ms=%s err=%s", tool, elapsed_ms, exc)
        return ToolResult(tool=tool, ok=False, error=str(exc))


_WEB_SEARCH_TOOL = {
    "type": "function",
    "function": {
        "name": "web_search",
        "description": "Search the web for recent/factual information and return relevant snippets and URLs.",
        "parameters": {
            "type": "object",
            "properties": {"query": {"type": "string"}},
            "required": ["query"],
        },
    },
}


def generate_reply(
    *,
    user_text: str,
    tier: int,
    user_id: str | None = None,
    conversation_id: str | None = None,
    run_id: str | None = None,
    history_messages: list[dict[str, str]] | None = None,
) -> tuple[str, dict[str, Any]]:
    """
    Returns (reply_text, meta).
    Meta contains model + token usage when available.
    """
    if tier == 0:
        return tier0_reply(user_text), {"tier": 0, "model": "none"}

    model = settings.OPENAI_MODEL or "gpt-4o-mini"
    try:
        cache_ctx = {"conversation_id": conversation_id} if conversation_id else None
        if user_id:
            cached = get_cached_response(
                user_id=user_id,
                query_text=user_text,
                model=model,
                tier=tier,
                context=cache_ctx,
            )
            if cached:
                return cached, {"tier": tier, "model": model, "cache_hit": True}

        client = _client()

        messages: list[dict[str, Any]] = [{"role": "system", "content": SYSTEM_PROMPT}]
        if history_messages:
            messages.extend(history_messages)
        messages.append({"role": "user", "content": user_text})

        tools = [_WEB_SEARCH_TOOL] if tier >= 2 else []

        tool_calls_used = 0
        usage_obj = None
        while True:
            resp = client.chat.completions.create(
                model=model,
                messages=messages,
                tools=tools or None,
                temperature=0.4,
            )
            choice = resp.choices[0]
            m = choice.message
            usage_obj = getattr(resp, "usage", usage_obj)

            tool_calls = getattr(m, "tool_calls", None)
            if tool_calls and tools and tool_calls_used < 3:
                # Append assistant message with tool_calls
                messages.append(
                    {
                        "role": "assistant",
                        "content": m.content or "",
                        "tool_calls": [
                            {
                                "id": tc.id,
                                "type": "function",
                                "function": {"name": tc.function.name, "arguments": tc.function.arguments},
                            }
                            for tc in tool_calls
                        ],
                    }
                )

                # Execute tools and append tool results
                for tc in tool_calls:
                    tool_calls_used += 1
                    if tc.function.name != "web_search":
                        result = ToolResult(tool=tc.function.name, ok=False, error="Unsupported tool")
                    else:
                        try:
                            parsed = json.loads(tc.function.arguments or "{}")
                        except Exception:
                            parsed = {}
                        query = str(parsed.get("query") or "").strip()
                        result = _execute_tool(
                            tool="web.search",
                            args={"query": query},
                            user_id=user_id,
                            run_id=run_id,
                            timeout_s=10.0,
                        )
                    messages.append(
                        {
                            "role": "tool",
                            "tool_call_id": tc.id,
                            "content": json.dumps(result.model_dump(), ensure_ascii=False),
                        }
                    )
                continue

            # Final answer
            msg = (m.content or "").strip()
            break

        meta: dict[str, Any] = {"tier": tier, "model": model}
        if usage_obj:
            meta["usage"] = {
                "input_tokens": getattr(usage_obj, "prompt_tokens", None),
                "output_tokens": getattr(usage_obj, "completion_tokens", None),
                "total_tokens": getattr(usage_obj, "total_tokens", None),
            }

        if user_id and msg:
            try:
                put_cached_response(
                    user_id=user_id,
                    query_text=user_text,
                    assistant_message=msg,
                    model=model,
                    tier=tier,
                    context=cache_ctx,
                )
            except Exception:
                pass

        return msg or "I’m not sure I got that — can you rephrase?", meta
    except Exception:
        logger.exception("LLM reply generation failed")
        return (
            "I hit an internal error generating a reply. Try again in a minute.",
            {"tier": tier, "model": model, "error": True},
        )

