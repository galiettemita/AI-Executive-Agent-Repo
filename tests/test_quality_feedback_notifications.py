from __future__ import annotations

from datetime import datetime, timezone

from fastapi.testclient import TestClient
from sqlalchemy import text

from app.db.database import SessionLocal
from app.main import app
from app.services import quality_eval, scheduled_notifications, user_feedback


class _FakeRedis:
    def __init__(self) -> None:
        self.kv: dict[str, str] = {}
        self.zsets: dict[str, dict[str, float]] = {}

    def get(self, key: str):
        return self.kv.get(key)

    def set(self, key: str, value: str, ex: int | None = None, nx: bool = False):
        if nx and key in self.kv:
            return False
        self.kv[key] = value
        return True

    def zadd(self, key: str, mapping: dict[str, float]):
        bucket = self.zsets.setdefault(key, {})
        for member, score in mapping.items():
            bucket[member] = float(score)

    def zremrangebyscore(self, key: str, min_score: float, max_score: float):
        bucket = self.zsets.setdefault(key, {})
        to_drop = [m for m, s in bucket.items() if float(min_score) <= s <= float(max_score)]
        for member in to_drop:
            bucket.pop(member, None)
        return len(to_drop)

    def zcard(self, key: str):
        return len(self.zsets.get(key, {}))

    def zrange(self, key: str, start: int, stop: int, withscores: bool = False):
        items = sorted((self.zsets.get(key) or {}).items(), key=lambda item: item[1])
        if stop == -1:
            selected = items[start:]
        else:
            selected = items[start : stop + 1]
        if withscores:
            return selected
        return [item[0] for item in selected]

    def expire(self, key: str, seconds: int):
        return True


def test_live_quality_eval_inserts_result(monkeypatch):
    monkeypatch.setattr(quality_eval, "_sampled_for_live_eval", lambda **kwargs: True)
    monkeypatch.setattr(quality_eval, "_maybe_emit_quality_alerts", lambda db: None)

    row = quality_eval.evaluate_response_quality(
        user_id="quality-user",
        conversation_id="conv-1",
        run_id="run-quality-1",
        message_id="msg-1",
        user_text="Can you summarize this?",
        assistant_text="Here is a concise summary with next steps.",
        used_tools=False,
    )
    assert row is not None
    db = SessionLocal()
    try:
        count = db.execute(text("select count(*) from eval_results where run_id = :run_id"), {"run_id": "run-quality-1"}).scalar()
        assert int(count or 0) >= 1
    finally:
        db.close()


def test_feedback_endpoint_persists_and_enqueues_moderation(monkeypatch):
    monkeypatch.setattr(user_feedback, "send_alert", lambda *args, **kwargs: None)
    client = TestClient(app)
    resp = client.post(
        "/api/v1/feedback/response",
        json={
            "user_id": "feedback-user-1",
            "feedback": "down",
            "run_id": "run-feedback-1",
            "message_id": "msg-feedback-1",
            "comment": "This reply was not helpful",
        },
    )
    assert resp.status_code == 200
    assert resp.json()["ok"] is True
    assert resp.json()["moderation_item_created"] is True

    db = SessionLocal()
    try:
        feedback_count = db.execute(
            text("select count(*) from user_feedback where run_id = :run_id"),
            {"run_id": "run-feedback-1"},
        ).scalar()
        mod_count = db.execute(
            text("select count(*) from moderation_queue where run_id = :run_id"),
            {"run_id": "run-feedback-1"},
        ).scalar()
        assert int(feedback_count or 0) >= 1
        assert int(mod_count or 0) >= 1
    finally:
        db.close()


def test_scheduled_notifications_delivery_and_rate_limit(monkeypatch):
    fake = _FakeRedis()
    monkeypatch.setattr(scheduled_notifications, "get_redis", lambda: fake)
    monkeypatch.setattr(scheduled_notifications, "_quiet_hours_active", lambda now_local: False)

    db = SessionLocal()
    try:
        # Three due notifications for same user -> max 2/hour should send 2 and defer 1.
        now = datetime.now(timezone.utc)
        for idx in range(3):
            scheduled_notifications.schedule_notification(
                db,
                user_id="notify-user-1",
                notification_type="test",
                payload={"message": f"**Test** message {idx}"},
                scheduled_for=now,
                channel="imessage",
                timezone_name="UTC",
            )

        result = scheduled_notifications.run_due_scheduled_notifications(db)
        assert result["sent"] == 2
        assert result["deferred_rate_limit"] == 1

        sent_rows = db.execute(
            text("select count(*) from scheduled_notifications where user_id = :user_id and status = 'sent'"),
            {"user_id": "notify-user-1"},
        ).scalar()
        queued_rows = db.execute(
            text("select count(*) from outbound_messages where user_id = :user_id"),
            {"user_id": "notify-user-1"},
        ).scalar()
        assert int(sent_rows or 0) == 2
        assert int(queued_rows or 0) >= 2
    finally:
        db.close()
