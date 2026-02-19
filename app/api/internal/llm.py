from __future__ import annotations

from typing import Any

from fastapi import APIRouter
from fastapi import HTTPException
from pydantic import BaseModel, Field

from app.blueprint.contracts import LLMProvider, LLMRequest
from app.blueprint.llm.router import (
    clear_provider_health_override,
    get_llm_router,
    list_provider_health_overrides,
    set_provider_health_override,
)
from app.core.config import settings


router = APIRouter(prefix="/internal/llm", tags=["internal-llm"])


class HealthOverrideRequest(BaseModel):
    provider: LLMProvider
    healthy: bool
    ttl_seconds: int = Field(default=300, ge=30, le=3600)


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
    router_service = get_llm_router()
    if bool(payload.get("force_refresh")):
        router_service.all_provider_health(force_refresh=True)

    req = LLMRequest(
        model_preference=payload.get("model_preference"),
        user_id=(str(payload.get("user_id") or "").strip() or None),
        prompt_group=str(payload.get("prompt_group") or "system_prompt"),
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
    selected = router_service.select_route(req)
    return {
        "ok": True,
        "route": selected,
    }


def _require_non_prod_override() -> None:
    if settings.ENV == "production":
        raise HTTPException(status_code=403, detail="health overrides are disabled in production")


@router.get("/health/overrides")
def llm_health_overrides():
    _require_non_prod_override()
    return {"ok": True, "overrides": list_provider_health_overrides()}


@router.post("/health/override")
def llm_set_health_override(payload: HealthOverrideRequest):
    _require_non_prod_override()
    result = set_provider_health_override(
        provider=payload.provider,
        healthy=payload.healthy,
        ttl_seconds=payload.ttl_seconds,
    )
    # Refresh cache immediately to reduce stale-health windows.
    get_llm_router().provider_health(payload.provider, force_refresh=True)
    return {"ok": True, **result}


@router.delete("/health/override/{provider}")
def llm_clear_health_override(provider: LLMProvider):
    _require_non_prod_override()
    result = clear_provider_health_override(provider=provider)
    get_llm_router().provider_health(provider, force_refresh=True)
    return {"ok": True, **result}
