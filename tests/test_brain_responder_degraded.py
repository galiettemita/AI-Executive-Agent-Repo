from __future__ import annotations

from app.blueprint.brain import responder
from app.blueprint.llm.router import LLMDegradedModeQueuedError


class _QueuedRouter:
    def call(self, req):  # pragma: no cover - exercised through generate_reply
        raise LLMDegradedModeQueuedError(task_type=req.task_type)

    def system_mode(self, force_refresh: bool = False) -> str:
        return "degraded"


def test_generate_reply_queues_tier3_request_in_degraded_mode(monkeypatch):
    monkeypatch.setattr(responder, "get_llm_router", lambda: _QueuedRouter())
    monkeypatch.setattr(
        responder,
        "compile_context_messages",
        lambda **kwargs: ([{"role": "system", "content": "x"}, {"role": "user", "content": kwargs.get("user_text") or ""}], [], []),
    )
    monkeypatch.setattr(responder, "compile_tool_schemas", lambda **kwargs: [])
    monkeypatch.setattr(responder, "orchestrate_tier3_plan", lambda **kwargs: ([], {}))
    monkeypatch.setattr(responder, "get_cached_response", lambda **kwargs: None)
    monkeypatch.setattr(responder, "enqueue_degraded_retry", lambda **kwargs: {"queued": True, "job_id": "job-1"})
    monkeypatch.setattr(responder, "should_send_degraded_notice", lambda user_id: True)

    text, meta = responder.generate_reply(
        user_text="Analyze this deeply and build a plan.",
        tier=3,
        user_id="user-1",
        conversation_id="conv-1",
        run_id="run-1",
    )

    assert "backup mode" in text.lower()
    assert "queued" in text.lower()
    assert meta.get("degraded_mode") is True
    assert meta.get("queued_for_retry") is True
