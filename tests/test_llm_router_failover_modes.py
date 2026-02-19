from __future__ import annotations

import time

from app.blueprint.contracts import LLMProvider, LLMProviderHealth, LLMRequest, LLMResponse, TokenUsage
from app.blueprint.llm.router import LLMDegradedModeQueuedError, LLMMaintenanceModeError, LLMRouter


def _health_map(*, openai: bool, anthropic: bool, google: bool, local: bool) -> dict[str, LLMProviderHealth]:
    return {
        LLMProvider.OPENAI.value: LLMProviderHealth(provider=LLMProvider.OPENAI, is_healthy=openai, error_rate_1h=0.0 if openai else 1.0),
        LLMProvider.ANTHROPIC.value: LLMProviderHealth(
            provider=LLMProvider.ANTHROPIC,
            is_healthy=anthropic,
            error_rate_1h=0.0 if anthropic else 1.0,
        ),
        LLMProvider.GOOGLE.value: LLMProviderHealth(provider=LLMProvider.GOOGLE, is_healthy=google, error_rate_1h=0.0 if google else 1.0),
        LLMProvider.LOCAL.value: LLMProviderHealth(provider=LLMProvider.LOCAL, is_healthy=local, error_rate_1h=0.0 if local else 1.0),
    }


def test_system_mode_degraded_when_external_down_local_healthy():
    router = LLMRouter()
    health_map = _health_map(openai=False, anthropic=False, google=False, local=True)
    assert router._system_mode_from_health(health_map) == "degraded"


def test_call_degraded_uses_local_for_tier12_tasks(monkeypatch):
    router = LLMRouter()
    providers_called: list[LLMProvider] = []

    monkeypatch.setattr(router, "all_provider_health", lambda force_refresh=False: _health_map(openai=False, anthropic=False, google=False, local=True))
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
    monkeypatch.setattr(router, "all_provider_health", lambda force_refresh=False: _health_map(openai=False, anthropic=False, google=False, local=True))
    req = LLMRequest(messages=[{"role": "user", "content": "analyze this deeply"}], task_type="complex_reasoning")
    try:
        router.call(req)
        assert False, "expected degraded-mode rejection for tier3 task"
    except LLMDegradedModeQueuedError as exc:
        assert "degraded mode" in str(exc).lower()


def test_call_maintenance_mode_rejects_all(monkeypatch):
    router = LLMRouter()
    monkeypatch.setattr(router, "all_provider_health", lambda force_refresh=False: _health_map(openai=False, anthropic=False, google=False, local=False))
    req = LLMRequest(messages=[{"role": "user", "content": "hi"}], task_type="general")
    try:
        router.call(req)
        assert False, "expected maintenance-mode rejection"
    except LLMMaintenanceModeError as exc:
        assert "maintenance mode" in str(exc).lower()


def test_recovery_ramp_fraction_progresses_after_recovery():
    router = LLMRouter()
    router._last_system_mode = "degraded"

    assert router._recovery_ramp_fraction("normal") == 0.10

    router._recovery_started_monotonic = time.monotonic() - 120
    assert router._recovery_ramp_fraction("normal") == 0.25

    router._recovery_started_monotonic = time.monotonic() - 220
    assert router._recovery_ramp_fraction("normal") == 0.50

    router._recovery_started_monotonic = time.monotonic() - (router._recovery_ramp_seconds + 5)
    assert router._recovery_ramp_fraction("normal") == 1.0
    assert router._recovery_started_monotonic is None


def test_select_route_forces_local_during_recovery(monkeypatch):
    router = LLMRouter()
    health_map = _health_map(openai=True, anthropic=True, google=True, local=True)
    monkeypatch.setattr(router, "all_provider_health", lambda force_refresh=False: health_map)
    monkeypatch.setattr(router, "_request_bucket", lambda req: 99)

    # Simulate immediate recovery transition from degraded -> normal (10% ramp).
    router._last_system_mode = "degraded"
    route = router.select_route(LLMRequest(messages=[{"role": "user", "content": "hello"}], task_type="general"))

    assert route["system_mode"] == "normal"
    assert route["recovery_ramp_fraction"] == 0.1
    assert route["recovery_forced_local"] is True
    assert route["selected_provider"] == LLMProvider.LOCAL.value


def test_select_route_maintenance_has_no_selected_provider(monkeypatch):
    router = LLMRouter()
    monkeypatch.setattr(router, "all_provider_health", lambda force_refresh=False: _health_map(openai=False, anthropic=False, google=False, local=False))
    route = router.select_route(LLMRequest(messages=[{"role": "user", "content": "hello"}], task_type="general"))
    assert route["system_mode"] == "maintenance"
    assert route["selected_provider"] is None
