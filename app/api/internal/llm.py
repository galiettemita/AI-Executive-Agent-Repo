from __future__ import annotations

from typing import Any

from fastapi import APIRouter

from app.blueprint.contracts import LLMRequest
from app.blueprint.llm.router import get_llm_router


router = APIRouter(prefix="/internal/llm", tags=["internal-llm"])


@router.get("/health")
def llm_health(force_refresh: bool = False):
    router_service = get_llm_router()
    health = router_service.all_provider_health(force_refresh=force_refresh)
    return {
        "ok": True,
        "providers": {
            provider: payload.model_dump(mode="json") for provider, payload in health.items()
        },
    }


@router.post("/route-test")
def llm_route_test(payload: dict[str, Any]):
    req = LLMRequest(
        model_preference=payload.get("model_preference"),
        messages=payload.get("messages")
        or [{"role": "user", "content": str(payload.get("prompt") or "test message")}],
        tools=payload.get("tools"),
        temperature=float(payload.get("temperature", 0.7)),
        max_tokens=int(payload.get("max_tokens", 600)),
        structured_output=payload.get("structured_output"),
        task_type=str(payload.get("task_type") or "general"),
        max_cost_cents=float(payload.get("max_cost_cents", 10.0)),
        max_latency_ms=int(payload.get("max_latency_ms", 15000)),
        pii_content=bool(payload.get("pii_content", False)),
        requires_safety_check=bool(payload.get("requires_safety_check", False)),
        stream=False,
    )

    router_service = get_llm_router()
    selected = router_service.select_route(req)
    return {
        "ok": True,
        "route": selected,
    }
