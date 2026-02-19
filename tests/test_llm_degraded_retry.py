from __future__ import annotations

from app.blueprint.llm import degraded_retry


def test_enqueue_degraded_retry_deduplicates_run_id(monkeypatch):
    monkeypatch.setattr(degraded_retry, "get_redis", lambda: None)
    degraded_retry._mem_queue.clear()
    degraded_retry._mem_seen_runs.clear()

    first = degraded_retry.enqueue_degraded_retry(
        run_id="run-123",
        user_id="user-1",
        conversation_id="conv-1",
        user_text="plan my quarter",
        tier=3,
        task_type="complex_reasoning",
        reason="degraded_mode",
    )
    second = degraded_retry.enqueue_degraded_retry(
        run_id="run-123",
        user_id="user-1",
        conversation_id="conv-1",
        user_text="plan my quarter",
        tier=3,
        task_type="complex_reasoning",
        reason="degraded_mode",
    )

    assert first["queued"] is True
    assert second["queued"] is False
    assert second["duplicate"] is True


def test_should_send_degraded_notice_once_per_cooldown(monkeypatch):
    monkeypatch.setattr(degraded_retry, "get_redis", lambda: None)
    degraded_retry._mem_notice_until.clear()

    assert degraded_retry.should_send_degraded_notice("user-abc") is True
    assert degraded_retry.should_send_degraded_notice("user-abc") is False
