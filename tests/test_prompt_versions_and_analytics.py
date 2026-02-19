from __future__ import annotations

import uuid

from app.blueprint.contracts import LLMProvider, LLMProviderHealth, LLMRequest, LLMResponse, TokenUsage
from app.blueprint.llm.router import LLMRouter
from app.db.database import SessionLocal
from app.services.analytics import aggregate_daily, emit_event
from app.services.prompt_versions import create_prompt_version, ensure_prompt_versions_table, rollback_prompt, select_prompt_version


def test_prompt_version_selection_and_rollback():
    prompt_group = f"system_prompt_{uuid.uuid4().hex}"
    db = SessionLocal()
    try:
        ensure_prompt_versions_table(db)
        active_id = create_prompt_version(
            db,
            prompt_group=prompt_group,
            version_label="v1",
            content="ACTIVE PROMPT",
            status="active",
            rollout_percentage=100,
        )
        canary_id = create_prompt_version(
            db,
            prompt_group=prompt_group,
            version_label="v2",
            content="CANARY PROMPT",
            status="canary",
            rollout_percentage=100,
        )
        selected = select_prompt_version(
            db,
            user_id="user-1",
            prompt_group=prompt_group,
            default_content="DEFAULT",
        )
        assert selected["prompt_version_id"] == canary_id
        assert selected["content"] == "CANARY PROMPT"

        rolled = rollback_prompt(db, prompt_group=prompt_group, target_version_id=active_id)
        assert rolled == active_id
    finally:
        db.close()


def test_llm_router_applies_prompt_version(monkeypatch):
    prompt_group = f"system_prompt_{uuid.uuid4().hex}"
    db = SessionLocal()
    try:
        ensure_prompt_versions_table(db)
        create_prompt_version(
            db,
            prompt_group=prompt_group,
            version_label="router-v1",
            content="OVERRIDDEN SYSTEM PROMPT",
            status="active",
            rollout_percentage=100,
        )
    finally:
        db.close()

    router = LLMRouter()
    monkeypatch.setattr(
        router,
        "all_provider_health",
        lambda force_refresh=False: {
            LLMProvider.OPENAI.value: LLMProviderHealth(provider=LLMProvider.OPENAI, is_healthy=True, error_rate_1h=0.0),
            LLMProvider.ANTHROPIC.value: LLMProviderHealth(provider=LLMProvider.ANTHROPIC, is_healthy=False, error_rate_1h=1.0),
            LLMProvider.GOOGLE.value: LLMProviderHealth(provider=LLMProvider.GOOGLE, is_healthy=False, error_rate_1h=1.0),
            LLMProvider.LOCAL.value: LLMProviderHealth(provider=LLMProvider.LOCAL, is_healthy=False, error_rate_1h=1.0),
        },
    )
    monkeypatch.setattr(router, "_provider_enabled", lambda provider: provider == LLMProvider.OPENAI)
    monkeypatch.setattr(
        router,
        "provider_health",
        lambda provider, force_refresh=False: LLMProviderHealth(provider=provider, is_healthy=(provider == LLMProvider.OPENAI), error_rate_1h=0.0 if provider == LLMProvider.OPENAI else 1.0),
    )

    captured = {"system_prompt": ""}

    def _fake_call_provider(provider, model, req):
        captured["system_prompt"] = str(req.messages[0].get("content") or "")
        return (
            LLMResponse(
                provider=provider,
                model=model,
                content="ok",
                usage=TokenUsage(input_tokens=1, output_tokens=1, total_tokens=2),
            ),
            None,
        )

    monkeypatch.setattr(router, "_call_provider", _fake_call_provider)

    resp = router.call(
        LLMRequest(
            user_id="user-42",
            prompt_group=prompt_group,
            messages=[
                {"role": "system", "content": "ORIGINAL SYSTEM PROMPT"},
                {"role": "user", "content": "hello"},
            ],
            task_type="general",
        )
    )
    assert captured["system_prompt"] == "OVERRIDDEN SYSTEM PROMPT"
    assert resp.prompt_version_id is not None


def test_analytics_emit_and_aggregate_daily():
    db = SessionLocal()
    try:
        emit_event(db, event_name="message_received", user_id="a-user", source="test", payload={"x": 1})
        emit_event(db, event_name="message_sent", user_id="a-user", source="test", payload={"x": 2})
        emit_event(db, event_name="tool_invoked", user_id="a-user", source="test", payload={"tool": "search"})
        daily = aggregate_daily(db)
        assert daily["message_volume"] >= 3
        assert daily["dau"] >= 1
        assert daily["tool_calls"] >= 1
    finally:
        db.close()
