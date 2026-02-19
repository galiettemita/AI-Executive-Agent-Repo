from __future__ import annotations

import json
import logging
import threading
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Iterable

import httpx
from openai import OpenAI

from app.blueprint.contracts import LLMProvider, LLMProviderHealth, LLMRequest, LLMResponse, TokenUsage
from app.core.config import settings
from app.core.redis import cache_get_json, cache_set_json

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class ModelCost:
    input_per_1k: float
    output_per_1k: float


_MODEL_COSTS: dict[str, ModelCost] = {
    "gpt-4o": ModelCost(input_per_1k=0.0025, output_per_1k=0.01),
    "gpt-4o-mini": ModelCost(input_per_1k=0.00015, output_per_1k=0.0006),
    "claude-sonnet-4-20250514": ModelCost(input_per_1k=0.003, output_per_1k=0.015),
    "gemini-2.0-flash": ModelCost(input_per_1k=0.0001, output_per_1k=0.0004),
    "gemini-2.0-pro": ModelCost(input_per_1k=0.00125, output_per_1k=0.005),
    "llama-4-8b": ModelCost(input_per_1k=0.00005, output_per_1k=0.0001),
}


_TASK_ROUTING_TABLE: dict[str, dict[str, Any]] = {
    "intent_classification": {
        "primary": "openai:gpt-4o-mini",
        "fallback": ["google:gemini-2.0-flash", "local:llama-4-8b"],
    },
    "single_tool_call": {
        "primary": "openai:gpt-4o-mini",
        "fallback": ["anthropic:claude-sonnet-4-20250514"],
    },
    "email_drafting": {
        "primary": "anthropic:claude-sonnet-4-20250514",
        "fallback": ["openai:gpt-4o"],
    },
    "complex_reasoning": {
        "primary": "openai:gpt-4o",
        "fallback": ["anthropic:claude-sonnet-4-20250514"],
    },
    "knowledge_extraction": {
        "primary": "openai:gpt-4o-mini",
        "fallback": ["google:gemini-2.0-flash", "local:llama-4-8b"],
    },
    "general": {
        "primary": "openai:gpt-4o-mini",
        "fallback": ["anthropic:claude-sonnet-4-20250514", "google:gemini-2.0-flash"],
    },
}


def _now_utc() -> datetime:
    return datetime.now(timezone.utc)


def _health_cache_key(provider: LLMProvider) -> str:
    return f"bp:llm:health:{provider.value}"


def _parse_provider_model(selector: str) -> tuple[LLMProvider, str]:
    p, m = selector.split(":", 1)
    return LLMProvider(p), m


def _safe_int(value: Any) -> int:
    try:
        return int(value or 0)
    except Exception:
        return 0


def _safe_float(value: Any) -> float:
    try:
        return float(value or 0)
    except Exception:
        return 0.0


class LLMRouter:
    def __init__(self) -> None:
        self._openai_client: OpenAI | None = None
        self._probe_thread: threading.Thread | None = None
        self._probe_stop = threading.Event()
        self._probe_interval_seconds = max(10, settings.LLM_ROUTER_HEALTH_CHECK_INTERVAL)
        if settings.ENV in {"staging", "production"}:
            self.start_health_probe_loop()

    def start_health_probe_loop(self) -> None:
        if self._probe_thread and self._probe_thread.is_alive():
            return
        self._probe_stop.clear()
        self._probe_thread = threading.Thread(
            target=self._health_probe_loop,
            name="llm-provider-health-probe",
            daemon=True,
        )
        self._probe_thread.start()

    def stop_health_probe_loop(self) -> None:
        self._probe_stop.set()

    def _health_probe_loop(self) -> None:
        while not self._probe_stop.is_set():
            try:
                self.all_provider_health(force_refresh=True)
            except Exception:
                logger.exception("llm_router_health_probe_loop_error")
            if self._probe_stop.wait(self._probe_interval_seconds):
                break

    def _openai(self) -> OpenAI:
        if not settings.OPENAI_API_KEY:
            raise RuntimeError("OPENAI_API_KEY not configured")
        if self._openai_client is None:
            self._openai_client = OpenAI(
                api_key=settings.OPENAI_API_KEY,
                organization=settings.OPENAI_ORG_ID or None,
                timeout=max(15, settings.LLM_ROUTER_FAILOVER_TIMEOUT_S),
                max_retries=1,
            )
        return self._openai_client

    def _route_chain(self, req: LLMRequest) -> list[tuple[LLMProvider, str]]:
        if req.model_preference:
            if req.model_preference == LLMProvider.OPENAI:
                return [(LLMProvider.OPENAI, settings.OPENAI_MODEL or "gpt-4o-mini")]
            if req.model_preference == LLMProvider.ANTHROPIC:
                return [(LLMProvider.ANTHROPIC, "claude-sonnet-4-20250514")]
            if req.model_preference == LLMProvider.GOOGLE:
                return [(LLMProvider.GOOGLE, "gemini-2.0-flash")]
            if req.model_preference == LLMProvider.LOCAL:
                return [(LLMProvider.LOCAL, "llama-4-8b")]

        if req.pii_content and settings.LOCAL_LLM_ENDPOINT:
            return [(LLMProvider.LOCAL, "llama-4-8b"), (LLMProvider.OPENAI, "gpt-4o-mini")]

        routing = _TASK_ROUTING_TABLE.get(req.task_type) or _TASK_ROUTING_TABLE["general"]
        chain = [_parse_provider_model(routing["primary"])]
        for candidate in routing.get("fallback") or []:
            chain.append(_parse_provider_model(candidate))
        return chain

    def _provider_enabled(self, provider: LLMProvider) -> bool:
        if provider == LLMProvider.OPENAI:
            return bool(settings.OPENAI_API_KEY)
        if provider == LLMProvider.ANTHROPIC:
            return bool(settings.ANTHROPIC_API_KEY)
        if provider == LLMProvider.GOOGLE:
            return bool(settings.GOOGLE_AI_API_KEY)
        if provider == LLMProvider.LOCAL:
            return bool(settings.LOCAL_LLM_ENDPOINT)
        return False

    def _probe_provider(self, provider: LLMProvider) -> LLMProviderHealth:
        started = time.perf_counter()
        try:
            if not self._provider_enabled(provider):
                return LLMProviderHealth(provider=provider, is_healthy=False, error_rate_1h=1.0)

            timeout = max(5, settings.LLM_ROUTER_FAILOVER_TIMEOUT_S)
            if provider == LLMProvider.OPENAI:
                url = "https://api.openai.com/v1/models"
                headers = {"Authorization": f"Bearer {settings.OPENAI_API_KEY}"}
                resp = httpx.get(url, headers=headers, timeout=timeout)
                healthy = resp.status_code < 500
            elif provider == LLMProvider.ANTHROPIC:
                url = "https://api.anthropic.com/v1/models"
                headers = {
                    "x-api-key": settings.ANTHROPIC_API_KEY or "",
                    "anthropic-version": "2023-06-01",
                }
                resp = httpx.get(url, headers=headers, timeout=timeout)
                healthy = resp.status_code < 500
            elif provider == LLMProvider.GOOGLE:
                url = "https://generativelanguage.googleapis.com/v1/models"
                params = {"key": settings.GOOGLE_AI_API_KEY or ""}
                resp = httpx.get(url, params=params, timeout=timeout)
                healthy = resp.status_code < 500
            elif provider == LLMProvider.LOCAL:
                endpoint = (settings.LOCAL_LLM_ENDPOINT or "").rstrip("/")
                url = f"{endpoint}/health"
                resp = httpx.get(url, timeout=timeout)
                healthy = resp.status_code < 500
            else:
                healthy = False

            latency = int((time.perf_counter() - started) * 1000)
            return LLMProviderHealth(
                provider=provider,
                is_healthy=healthy,
                latency_p50_ms=latency,
                latency_p95_ms=latency,
                error_rate_1h=0.0 if healthy else 1.0,
            )
        except Exception:
            logger.exception("Provider health probe failed provider=%s", provider.value)
            latency = int((time.perf_counter() - started) * 1000)
            return LLMProviderHealth(
                provider=provider,
                is_healthy=False,
                latency_p50_ms=latency,
                latency_p95_ms=latency,
                error_rate_1h=1.0,
                last_error_at=_now_utc(),
            )

    def provider_health(self, provider: LLMProvider, force_refresh: bool = False) -> LLMProviderHealth:
        ttl = max(10, settings.LLM_ROUTER_HEALTH_CHECK_INTERVAL)
        key = _health_cache_key(provider)

        if not force_refresh:
            cached = cache_get_json(key)
            if cached:
                try:
                    return LLMProviderHealth.model_validate(cached)
                except Exception:
                    pass

        health = self._probe_provider(provider)
        cache_set_json(key, health.model_dump(mode="json"), ttl_seconds=ttl)
        return health

    def all_provider_health(self, force_refresh: bool = False) -> dict[str, LLMProviderHealth]:
        out: dict[str, LLMProviderHealth] = {}
        for provider in (LLMProvider.OPENAI, LLMProvider.ANTHROPIC, LLMProvider.GOOGLE, LLMProvider.LOCAL):
            out[provider.value] = self.provider_health(provider, force_refresh=force_refresh)
        return out

    def _system_mode_from_health(self, health_map: dict[str, LLMProviderHealth]) -> str:
        external_any_healthy = any(
            bool((health_map.get(p.value) or LLMProviderHealth(provider=p, is_healthy=False)).is_healthy)
            for p in (LLMProvider.OPENAI, LLMProvider.ANTHROPIC, LLMProvider.GOOGLE)
        )
        local_healthy = bool(
            (health_map.get(LLMProvider.LOCAL.value) or LLMProviderHealth(provider=LLMProvider.LOCAL, is_healthy=False)).is_healthy
        )
        if external_any_healthy:
            return "normal"
        if local_healthy:
            return "degraded"
        return "maintenance"

    def system_mode(self, force_refresh: bool = False) -> str:
        return self._system_mode_from_health(self.all_provider_health(force_refresh=force_refresh))

    def _degraded_allows_task(self, req: LLMRequest) -> bool:
        task = (req.task_type or "").strip().lower()
        if task in {"complex_reasoning", "deep_research", "research", "research_engine"}:
            return False
        return not task.startswith("research")

    def select_route(self, req: LLMRequest) -> dict[str, Any]:
        health_map = self.all_provider_health(force_refresh=False)
        mode = self._system_mode_from_health(health_map)
        chain = self._route_chain(req)
        if mode == "degraded":
            chain = [(LLMProvider.LOCAL, "llama-4-8b")]
        selected: tuple[LLMProvider, str] | None = None
        checks: list[dict[str, Any]] = []

        for provider, model in chain:
            health = health_map.get(provider.value) or LLMProviderHealth(provider=provider, is_healthy=False, error_rate_1h=1.0)
            checks.append(
                {
                    "provider": provider.value,
                    "model": model,
                    "healthy": health.is_healthy,
                    "latency_p95_ms": health.latency_p95_ms,
                }
            )
            if selected is None and health.is_healthy:
                selected = (provider, model)

        if mode == "maintenance":
            selected = None
        if selected is None and chain:
            selected = chain[0]

        return {
            "task_type": req.task_type,
            "system_mode": mode,
            "requested_model_preference": req.model_preference.value if req.model_preference else None,
            "selected_provider": selected[0].value if selected else None,
            "selected_model": selected[1] if selected else None,
            "route_chain": checks,
        }

    def _estimate_cost_cents(self, *, model: str, usage: TokenUsage) -> float:
        costs = _MODEL_COSTS.get(model)
        if not costs:
            return 0.0
        input_cost = (usage.input_tokens / 1000.0) * costs.input_per_1k
        output_cost = (usage.output_tokens / 1000.0) * costs.output_per_1k
        # configured costs are in USD, keep cents in DB/telemetry as float cents
        return round((input_cost + output_cost) * 100, 4)

    def _validate_structured_output(self, req: LLMRequest, response: LLMResponse) -> tuple[bool, str | None]:
        schema = req.structured_output or {}
        if not schema:
            if (response.content or "").strip() or (response.tool_calls or []):
                return True, None
            return False, "empty llm response"
        if not isinstance(schema, dict):
            return False, "structured_output schema must be an object"
        try:
            payload = json.loads(response.content or "{}")
        except Exception:
            return False, "response is not valid JSON"
        if not isinstance(payload, dict):
            return False, "structured response must be a JSON object"

        required = [str(k) for k in (schema.get("required") or [])]
        for field in required:
            if field not in payload:
                return False, f"missing required field '{field}'"

        type_map = {
            "string": str,
            "boolean": bool,
            "integer": int,
            "number": (int, float),
            "object": dict,
            "array": list,
        }
        for key, spec in (schema.get("properties") or {}).items():
            if key not in payload or not isinstance(spec, dict):
                continue
            expected_type = str(spec.get("type") or "")
            py_type = type_map.get(expected_type)
            if py_type is None:
                continue
            if not isinstance(payload.get(key), py_type):
                return False, f"field '{key}' is not of type {expected_type}"
        return True, None

    def _response_from_openai(self, *, model: str, req: LLMRequest) -> tuple[LLMResponse, Any]:
        started = time.perf_counter()
        client = self._openai()
        resp = client.chat.completions.create(
            model=model,
            messages=req.messages,
            tools=req.tools,
            temperature=req.temperature,
            max_tokens=req.max_tokens,
            stream=False,
        )
        latency_ms = int((time.perf_counter() - started) * 1000)

        choice = resp.choices[0]
        message = choice.message
        tool_calls = None
        if getattr(message, "tool_calls", None):
            tool_calls = [
                {
                    "id": tc.id,
                    "type": "function",
                    "function": {
                        "name": tc.function.name,
                        "arguments": tc.function.arguments,
                    },
                }
                for tc in (message.tool_calls or [])
            ]

        usage = TokenUsage(
            input_tokens=_safe_int(getattr(resp.usage, "prompt_tokens", 0)),
            output_tokens=_safe_int(getattr(resp.usage, "completion_tokens", 0)),
            total_tokens=_safe_int(getattr(resp.usage, "total_tokens", 0)),
        )
        result = LLMResponse(
            provider=LLMProvider.OPENAI,
            model=model,
            content=(message.content or "").strip(),
            tool_calls=tool_calls,
            usage=usage,
            cost_cents=self._estimate_cost_cents(model=model, usage=usage),
            latency_ms=latency_ms,
            was_failover=False,
        )
        return result, resp

    def _response_from_anthropic(self, *, model: str, req: LLMRequest) -> LLMResponse:
        started = time.perf_counter()

        system_msgs = [m.get("content") for m in req.messages if m.get("role") == "system" and m.get("content")]
        system_prompt = "\n\n".join(system_msgs)
        messages = [
            {"role": m.get("role"), "content": m.get("content")}
            for m in req.messages
            if m.get("role") in ("user", "assistant")
        ]

        payload = {
            "model": model,
            "max_tokens": req.max_tokens,
            "temperature": req.temperature,
            "messages": messages,
        }
        if system_prompt:
            payload["system"] = system_prompt

        headers = {
            "x-api-key": settings.ANTHROPIC_API_KEY or "",
            "anthropic-version": "2023-06-01",
            "content-type": "application/json",
        }

        with httpx.Client(timeout=max(15, settings.LLM_ROUTER_FAILOVER_TIMEOUT_S)) as client:
            resp = client.post("https://api.anthropic.com/v1/messages", headers=headers, json=payload)
            resp.raise_for_status()
            data = resp.json()

        text_chunks = []
        for item in data.get("content") or []:
            if isinstance(item, dict) and item.get("type") == "text":
                text_chunks.append(item.get("text") or "")
        content = "".join(text_chunks).strip()

        usage = TokenUsage(
            input_tokens=_safe_int((data.get("usage") or {}).get("input_tokens")),
            output_tokens=_safe_int((data.get("usage") or {}).get("output_tokens")),
            total_tokens=_safe_int((data.get("usage") or {}).get("input_tokens"))
            + _safe_int((data.get("usage") or {}).get("output_tokens")),
        )

        return LLMResponse(
            provider=LLMProvider.ANTHROPIC,
            model=model,
            content=content,
            usage=usage,
            cost_cents=self._estimate_cost_cents(model=model, usage=usage),
            latency_ms=int((time.perf_counter() - started) * 1000),
            was_failover=False,
        )

    def _response_from_google(self, *, model: str, req: LLMRequest) -> LLMResponse:
        started = time.perf_counter()

        prompt_parts: list[str] = []
        for m in req.messages:
            role = str(m.get("role") or "user")
            content = str(m.get("content") or "")
            prompt_parts.append(f"{role.upper()}: {content}")
        prompt = "\n\n".join(prompt_parts)

        endpoint = f"https://generativelanguage.googleapis.com/v1/models/{model}:generateContent"
        params = {"key": settings.GOOGLE_AI_API_KEY or ""}
        payload = {
            "contents": [{"parts": [{"text": prompt}]}],
            "generationConfig": {
                "temperature": req.temperature,
                "maxOutputTokens": req.max_tokens,
            },
        }

        with httpx.Client(timeout=max(15, settings.LLM_ROUTER_FAILOVER_TIMEOUT_S)) as client:
            resp = client.post(endpoint, params=params, json=payload)
            resp.raise_for_status()
            data = resp.json()

        content = ""
        candidates = data.get("candidates") or []
        if candidates:
            parts = ((candidates[0] or {}).get("content") or {}).get("parts") or []
            content = "".join(str((p or {}).get("text") or "") for p in parts).strip()

        usage_meta = data.get("usageMetadata") or {}
        usage = TokenUsage(
            input_tokens=_safe_int(usage_meta.get("promptTokenCount")),
            output_tokens=_safe_int(usage_meta.get("candidatesTokenCount")),
            total_tokens=_safe_int(usage_meta.get("totalTokenCount")),
        )

        return LLMResponse(
            provider=LLMProvider.GOOGLE,
            model=model,
            content=content,
            usage=usage,
            cost_cents=self._estimate_cost_cents(model=model, usage=usage),
            latency_ms=int((time.perf_counter() - started) * 1000),
            was_failover=False,
        )

    def _response_from_local(self, *, model: str, req: LLMRequest) -> LLMResponse:
        endpoint = (settings.LOCAL_LLM_ENDPOINT or "").rstrip("/")
        if not endpoint:
            raise RuntimeError("LOCAL_LLM_ENDPOINT not configured")

        started = time.perf_counter()
        payload = {
            "model": model,
            "messages": req.messages,
            "temperature": req.temperature,
            "max_tokens": req.max_tokens,
            "tools": req.tools,
        }
        with httpx.Client(timeout=max(15, settings.LLM_ROUTER_FAILOVER_TIMEOUT_S)) as client:
            resp = client.post(f"{endpoint}/v1/chat/completions", json=payload)
            resp.raise_for_status()
            data = resp.json()

        choice = ((data.get("choices") or [{}])[0] or {}).get("message") or {}
        usage_raw = data.get("usage") or {}
        usage = TokenUsage(
            input_tokens=_safe_int(usage_raw.get("prompt_tokens")),
            output_tokens=_safe_int(usage_raw.get("completion_tokens")),
            total_tokens=_safe_int(usage_raw.get("total_tokens")),
        )
        return LLMResponse(
            provider=LLMProvider.LOCAL,
            model=model,
            content=str(choice.get("content") or "").strip(),
            usage=usage,
            cost_cents=self._estimate_cost_cents(model=model, usage=usage),
            latency_ms=int((time.perf_counter() - started) * 1000),
            was_failover=False,
        )

    def _call_provider(self, provider: LLMProvider, model: str, req: LLMRequest) -> tuple[LLMResponse, Any | None]:
        if provider == LLMProvider.OPENAI:
            return self._response_from_openai(model=model, req=req)
        if provider == LLMProvider.ANTHROPIC:
            return self._response_from_anthropic(model=model, req=req), None
        if provider == LLMProvider.GOOGLE:
            return self._response_from_google(model=model, req=req), None
        if provider == LLMProvider.LOCAL:
            return self._response_from_local(model=model, req=req), None
        raise RuntimeError(f"Unsupported provider: {provider.value}")

    def call(self, req: LLMRequest) -> LLMResponse:
        mode = self.system_mode(force_refresh=False)
        if mode == "maintenance":
            raise RuntimeError("LLM Router maintenance mode: no healthy providers available")

        if mode == "degraded":
            if not self._degraded_allows_task(req):
                raise RuntimeError("LLM Router degraded mode: task queued for retry")
            route = [(LLMProvider.LOCAL, "llama-4-8b")]
        else:
            route = self._route_chain(req)
        if not route:
            raise RuntimeError("No providers configured")

        errors: list[str] = []
        first_provider = route[0][0]

        for idx, (provider, model) in enumerate(route):
            if not self._provider_enabled(provider):
                errors.append(f"{provider.value}:not_configured")
                continue

            health = self.provider_health(provider)
            if not health.is_healthy:
                errors.append(f"{provider.value}:unhealthy")
                continue

            try:
                max_output_validation_retries = 2
                last_validation_error: str | None = None
                for attempt in range(max_output_validation_retries + 1):
                    result, _raw = self._call_provider(provider, model, req)
                    valid, validation_error = self._validate_structured_output(req, result)
                    if valid:
                        result.was_failover = idx > 0
                        logger.info(
                            "llm_router_call provider=%s model=%s task_type=%s latency_ms=%s cost_cents=%.4f failover=%s",
                            result.provider.value,
                            result.model,
                            req.task_type,
                            result.latency_ms,
                            result.cost_cents,
                            result.was_failover,
                        )
                        return result
                    last_validation_error = validation_error or "structured_output_validation_failed"
                    logger.warning(
                        "llm_router_structured_validation_failed provider=%s model=%s task_type=%s attempt=%s err=%s",
                        provider.value,
                        model,
                        req.task_type,
                        attempt + 1,
                        last_validation_error,
                    )
                raise RuntimeError(f"Structured output validation failed: {last_validation_error}")
            except Exception as exc:
                errors.append(f"{provider.value}:{exc.__class__.__name__}")
                logger.warning(
                    "llm_router_provider_failed provider=%s model=%s task_type=%s err=%s",
                    provider.value,
                    model,
                    req.task_type,
                    exc,
                )
                # Force refresh the provider health after a failed call.
                self.provider_health(provider, force_refresh=True)

        raise RuntimeError(f"All providers failed for task={req.task_type}; attempted={','.join(errors)}")

    def stream_text(self, req: LLMRequest) -> Iterable[str]:
        """
        Streaming currently prioritizes OpenAI in phase 1.
        If OpenAI is unavailable, falls back to a single non-streamed chunk.
        """
        route = self._route_chain(req)
        openai_route: tuple[LLMProvider, str] | None = None
        for provider, model in route:
            if provider == LLMProvider.OPENAI and self._provider_enabled(provider):
                openai_route = (provider, model)
                break

        if openai_route is None:
            fallback = self.call(req)
            if fallback.content:
                yield fallback.content
            return

        _provider, model = openai_route
        client = self._openai()
        stream = client.chat.completions.create(
            model=model,
            messages=req.messages,
            tools=req.tools,
            temperature=req.temperature,
            max_tokens=req.max_tokens,
            stream=True,
        )
        for chunk in stream:
            try:
                delta = chunk.choices[0].delta
                text = getattr(delta, "content", None)
            except Exception:
                text = None
            if text:
                yield text


_router_singleton: LLMRouter | None = None


def get_llm_router() -> LLMRouter:
    global _router_singleton
    if _router_singleton is None:
        _router_singleton = LLMRouter()
    return _router_singleton
