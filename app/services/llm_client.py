from __future__ import annotations

import logging
from types import SimpleNamespace
from typing import Any, Iterable

from openai import OpenAI as RealOpenAI

from app.blueprint.contracts import LLMProvider, LLMRequest
from app.blueprint.llm.router import get_llm_router
from app.core.config import settings

logger = logging.getLogger(__name__)


def _infer_task_type(messages: list[dict[str, Any]] | None, tools: list[dict[str, Any]] | None) -> str:
    text = " ".join(str((m or {}).get("content") or "") for m in (messages or []))
    lower = text.lower()
    if "intent" in lower and "classif" in lower:
        return "intent_classification"
    if "email" in lower and ("draft" in lower or "rewrite" in lower):
        return "email_drafting"
    if tools:
        return "single_tool_call"
    if any(k in lower for k in ("plan", "strategy", "roadmap", "analyze")):
        return "complex_reasoning"
    return "general"


def _message_content_from_input(input_payload: Any) -> str:
    if isinstance(input_payload, str):
        return input_payload
    if isinstance(input_payload, list):
        parts: list[str] = []
        for item in input_payload:
            if isinstance(item, str):
                parts.append(item)
                continue
            if isinstance(item, dict):
                if isinstance(item.get("content"), str):
                    parts.append(item["content"])
                    continue
                content = item.get("content")
                if isinstance(content, list):
                    for sub in content:
                        if isinstance(sub, dict) and isinstance(sub.get("text"), str):
                            parts.append(sub["text"])
        return "\n".join(p for p in parts if p)
    return str(input_payload or "")


class _ToolCallFunction:
    def __init__(self, name: str, arguments: str):
        self.name = name
        self.arguments = arguments


class _ToolCall:
    def __init__(self, tool_call: dict[str, Any]):
        function = tool_call.get("function") or {}
        self.id = str(tool_call.get("id") or "tool_call")
        self.type = "function"
        self.function = _ToolCallFunction(
            name=str(function.get("name") or "tool"),
            arguments=str(function.get("arguments") or "{}"),
        )


class _ChatCompletionsAPI:
    def __init__(self, parent: "OpenAIProxy"):
        self._parent = parent

    def create(self, **kwargs):
        return self._parent._chat_create(**kwargs)


class _ChatAPI:
    def __init__(self, parent: "OpenAIProxy"):
        self.completions = _ChatCompletionsAPI(parent)


class _ResponsesAPI:
    def __init__(self, parent: "OpenAIProxy"):
        self._parent = parent

    def create(self, **kwargs):
        return self._parent._responses_create(**kwargs)


class OpenAIProxy:
    """
    Backwards-compatible OpenAI client facade.

    - Routes chat/responses calls through the Blueprint LLM router when enabled.
    - Falls back to direct OpenAI SDK if router is disabled or routing fails.
    """

    def __init__(self, *args, **kwargs):
        self._raw = RealOpenAI(*args, **kwargs)
        self.chat = _ChatAPI(self)
        self.responses = _ResponsesAPI(self)
        self.embeddings = self._raw.embeddings
        self.audio = self._raw.audio
        self.images = getattr(self._raw, "images", None)

    def _router_enabled(self) -> bool:
        return bool(settings.FEATURE_MULTI_PROVIDER_LLM)

    def _chat_create(self, **kwargs):
        if not self._router_enabled():
            return self._raw.chat.completions.create(**kwargs)

        messages = kwargs.get("messages") or []
        tools = kwargs.get("tools")
        temperature = float(kwargs.get("temperature", 0.7))
        max_tokens = int(kwargs.get("max_tokens") or 2000)
        stream = bool(kwargs.get("stream", False))
        task_type = str(kwargs.get("task_type") or _infer_task_type(messages, tools))

        req = LLMRequest(
            messages=messages,
            tools=tools,
            temperature=temperature,
            max_tokens=max_tokens,
            task_type=task_type,
            stream=stream,
        )

        router = get_llm_router()

        if stream:
            try:
                return self._stream_wrapper(router.stream_text(req))
            except Exception as exc:
                logger.warning("router_stream_failed fallback=openai err=%s", exc)
                return self._raw.chat.completions.create(**kwargs)

        try:
            resp = router.call(req)
        except Exception as exc:
            logger.warning("router_chat_failed fallback=openai err=%s", exc)
            return self._raw.chat.completions.create(**kwargs)

        tool_calls = []
        for tc in resp.tool_calls or []:
            tool_calls.append(_ToolCall(tc))

        message = SimpleNamespace(
            content=resp.content,
            tool_calls=tool_calls or None,
        )
        choice = SimpleNamespace(message=message)
        usage = SimpleNamespace(
            prompt_tokens=resp.usage.input_tokens,
            completion_tokens=resp.usage.output_tokens,
            total_tokens=resp.usage.total_tokens,
        )
        return SimpleNamespace(choices=[choice], usage=usage, model=resp.model)

    def _responses_create(self, **kwargs):
        if not self._router_enabled():
            return self._raw.responses.create(**kwargs)

        model = kwargs.get("model")
        input_payload = kwargs.get("input")
        temperature = float(kwargs.get("temperature", 0.7))
        max_output_tokens = int(kwargs.get("max_output_tokens") or kwargs.get("max_tokens") or 2000)

        user_text = _message_content_from_input(input_payload)
        if not user_text.strip():
            user_text = "Please respond to the user request."

        messages = [{"role": "user", "content": user_text}]
        req = LLMRequest(
            messages=messages,
            temperature=temperature,
            max_tokens=max_output_tokens,
            task_type="general",
        )

        if isinstance(model, str):
            lower = model.lower()
            if "claude" in lower:
                req.model_preference = LLMProvider.ANTHROPIC
            elif "gemini" in lower:
                req.model_preference = LLMProvider.GOOGLE

        router = get_llm_router()
        try:
            routed = router.call(req)
            return SimpleNamespace(output_text=routed.content)
        except Exception as exc:
            logger.warning("router_responses_failed fallback=openai err=%s", exc)
            return self._raw.responses.create(**kwargs)

    @staticmethod
    def _stream_wrapper(iterable: Iterable[str]):
        def _gen():
            for token in iterable:
                delta = SimpleNamespace(content=token)
                choice = SimpleNamespace(delta=delta)
                yield SimpleNamespace(choices=[choice])

        return _gen()
