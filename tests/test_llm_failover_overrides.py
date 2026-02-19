from __future__ import annotations

from fastapi.testclient import TestClient

from app.blueprint.contracts import LLMProvider
import app.blueprint.llm.router as llm_router_mod
from app.main import app


class _FakeRedis:
    def __init__(self) -> None:
        self.kv: dict[str, str] = {}

    def set(self, key: str, value: str, ex: int | None = None):
        self.kv[key] = value
        return True

    def get(self, key: str):
        return self.kv.get(key)

    def delete(self, key: str):
        self.kv.pop(key, None)
        return 1


def test_llm_router_provider_health_override_helpers(monkeypatch):
    fake = _FakeRedis()
    monkeypatch.setattr(llm_router_mod, "get_redis", lambda: fake)

    llm_router_mod.set_provider_health_override(
        provider=LLMProvider.LOCAL,
        healthy=True,
        ttl_seconds=120,
    )
    router = llm_router_mod.LLMRouter()
    healthy = router.provider_health(LLMProvider.LOCAL, force_refresh=True)
    assert healthy.is_healthy is True

    llm_router_mod.set_provider_health_override(
        provider=LLMProvider.LOCAL,
        healthy=False,
        ttl_seconds=120,
    )
    unhealthy = router.provider_health(LLMProvider.LOCAL, force_refresh=True)
    assert unhealthy.is_healthy is False

    llm_router_mod.clear_provider_health_override(provider=LLMProvider.LOCAL)
    overrides = llm_router_mod.list_provider_health_overrides()
    assert "local" not in overrides


def test_internal_llm_health_override_endpoints(monkeypatch):
    fake = _FakeRedis()
    monkeypatch.setattr(llm_router_mod, "get_redis", lambda: fake)

    client = TestClient(app)
    set_resp = client.post(
        "/internal/llm/health/override",
        json={"provider": "openai", "healthy": False, "ttl_seconds": 120},
    )
    assert set_resp.status_code == 200
    assert set_resp.json()["ok"] is True

    list_resp = client.get("/internal/llm/health/overrides")
    assert list_resp.status_code == 200
    payload = list_resp.json()
    assert payload["ok"] is True
    assert payload["overrides"]["openai"]["healthy"] is False

    clear_resp = client.delete("/internal/llm/health/override/openai")
    assert clear_resp.status_code == 200
    assert clear_resp.json()["ok"] is True
