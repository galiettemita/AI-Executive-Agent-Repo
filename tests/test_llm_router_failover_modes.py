from __future__ import annotations

from app.blueprint.contracts import LLMProvider, LLMProviderHealth, LLMRequest, LLMResponse, TokenUsage
from app.blueprint.llm.router import LLMRouter


def test_system_mode_degraded_when_external_down_local_healthy():
    router = LLMRouter()

    health_map = {
        LLMProvider.OPENAI.value: LLMProviderHealth(provider=LLMProvider.OPENAI, is_healthy=False, error_rate_1h=1.0),
        LLMProvider.ANTHROPIC.value: LLMProviderHealth(provider=LLMProvider.ANTHROPIC, is_healthy=False, error_rate_1h=1.0),
        LLMProvider.GOOGLE.value: LLMProviderHealth(provider=LLMProvider.GOOGLE, is_healthy=False, error_rate_1h=1.0),
        LLMProvider.LOCAL.value: LLMProviderHealth(provider=LLMProvider.LOCAL, is_healthy=True, error_rate_1h=0.0),
    }
    assert router._system_mode_from_health(health_map) == "degraded"


def test_call_degraded_uses_local_for_tier12_tasks(monkeypatch):
    router = LLMRouter()
    providers_called: list[LLMProvider] = []

    monkeypatch.setattr(router, "system_mode", lambda force_refresh=False: "degraded")
    monkeypatch.setattr(router, "_provider_enabled", lambda provider: True)
    monkeypatch.setattr(
        router,
        "provider_health",
        lambda provider, force_refresh=False: LLMProviderHealth(provider=provider, is_healthy=True, error_rate_1h=0.0),
    )

    def _fake_call_provider(provider, model, req):
        providers_called.append(provider)
        return (
            LLMResponse(
                provider=provider,
                model=model,
                content="ok",
                usage=TokenUsage(input_tokens=1, output_tokens=1, total_tokens=2),
                latency_ms=1,
            ),
            None,
        )

    monkeypatch.setattr(router, "_call_provider", _fake_call_provider)
    req = LLMRequest(messages=[{"role": "user", "content": "hello"}], task_type="single_tool_call")
    resp = router.call(req)
    assert resp.provider == LLMProvider.LOCAL
    assert providers_called == [LLMProvider.LOCAL]


def test_call_degraded_rejects_tier3_tasks(monkeypatch):
    router = LLMRouter()
    monkeypatch.setattr(router, "system_mode", lambda force_refresh=False: "degraded")
    req = LLMRequest(messages=[{"role": "user", "content": "analyze this deeply"}], task_type="complex_reasoning")
    try:
        router.call(req)
        assert False, "expected degraded-mode rejection for tier3 task"
    except RuntimeError as exc:
        assert "degraded mode" in str(exc).lower()


def test_call_maintenance_mode_rejects_all(monkeypatch):
    router = LLMRouter()
    monkeypatch.setattr(router, "system_mode", lambda force_refresh=False: "maintenance")
    req = LLMRequest(messages=[{"role": "user", "content": "hi"}], task_type="general")
    try:
        router.call(req)
        assert False, "expected maintenance-mode rejection"
    except RuntimeError as exc:
        assert "maintenance mode" in str(exc).lower()
