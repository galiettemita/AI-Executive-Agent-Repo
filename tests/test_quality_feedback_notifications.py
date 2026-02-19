from __future__ import annotations

import json
from datetime import datetime, timezone

from fastapi.testclient import TestClient
from sqlalchemy import text

from app.db.database import SessionLocal
from app.db.models import Subscription
from app.main import app
from app.services.preferences import update_preferences
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


def test_live_quality_eval_scores_mcp_tool_usage(monkeypatch):
    monkeypatch.setattr(quality_eval, "_sampled_for_live_eval", lambda **kwargs: True)
    monkeypatch.setattr(quality_eval, "_maybe_emit_quality_alerts", lambda db: None)

    row = quality_eval.evaluate_response_quality(
        user_id="quality-mcp-user",
        conversation_id="conv-mcp",
        run_id="run-quality-mcp",
        message_id="msg-mcp-1",
        user_text="Summarize my connected MCP data sources.",
        assistant_text="I pulled your connected tools and summarized MCP activity.",
        used_tools=True,
        metadata={"content_provenance": "mcp_result", "used_mcp_tools": True},
    )
    assert row is not None

    db = SessionLocal()
    try:
        eval_row = db.execute(
            text(
                "select tool_usage_score, metadata_json from eval_results "
                "where run_id = :run_id order by created_at desc limit 1"
            ),
            {"run_id": "run-quality-mcp"},
        ).mappings().first()
        assert eval_row is not None
        assert float(eval_row.get("tool_usage_score") or 0.0) >= 4.0
        metadata_json = str(eval_row.get("metadata_json") or "")
        assert "mcp_result" in metadata_json
    finally:
        db.close()


def test_live_quality_eval_adds_financial_and_booking_domain_scores(monkeypatch):
    monkeypatch.setattr(quality_eval, "_sampled_for_live_eval", lambda **kwargs: True)
    monkeypatch.setattr(quality_eval, "_maybe_emit_quality_alerts", lambda db: None)

    row = quality_eval.evaluate_response_quality(
        user_id="quality-domain-user",
        conversation_id="conv-domain",
        run_id="run-quality-domain",
        message_id="msg-domain",
        user_text="Summarize Plaid transactions and confirm my Duffel booking.",
        assistant_text=(
            "Total spent is $420 USD across transactions. "
            "Booking confirmed with itinerary and booking ID ZX12."
        ),
        used_tools=True,
        metadata={"content_provenance": "mcp_result", "provider": "plaid+duffel"},
    )
    assert row is not None

    db = SessionLocal()
    try:
        eval_row = db.execute(
            text("select metadata_json from eval_results where run_id = :run_id order by created_at desc limit 1"),
            {"run_id": "run-quality-domain"},
        ).mappings().first()
        assert eval_row is not None
        metadata_raw = eval_row.get("metadata_json")
        if isinstance(metadata_raw, dict):
            metadata_obj = metadata_raw
        else:
            metadata_obj = json.loads(str(metadata_raw or "{}"))
        domain_scores = metadata_obj.get("domain_scores") or {}
        assert float(domain_scores.get("financial_accuracy") or 0.0) > 0
        assert float(domain_scores.get("booking_verification") or 0.0) > 0
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


def test_scheduled_notifications_respects_user_opt_out(monkeypatch):
    fake = _FakeRedis()
    monkeypatch.setattr(scheduled_notifications, "get_redis", lambda: fake)
    monkeypatch.setattr(scheduled_notifications, "_quiet_hours_active", lambda now_local: False)

    db = SessionLocal()
    try:
        update_preferences(db, "notify-optout-user", {"proactive_notifications_enabled": False})
        scheduled_notifications.schedule_notification(
            db,
            user_id="notify-optout-user",
            notification_type="morning_briefing",
            payload={"calendar_events": [], "tasks": []},
            scheduled_for=datetime.now(timezone.utc),
            channel="web",
            timezone_name="UTC",
        )

        result = scheduled_notifications.run_due_scheduled_notifications(db)
        assert int(result.get("sent") or 0) == 0
        assert int(result.get("skipped_preferences") or 0) == 1

        status = db.execute(
            text("select status from scheduled_notifications where user_id = :user_id order by created_at desc limit 1"),
            {"user_id": "notify-optout-user"},
        ).scalar()
        assert str(status) == "canceled"
    finally:
        db.close()


def test_scheduled_notifications_pauses_proactive_on_absence(monkeypatch):
    fake = _FakeRedis()
    monkeypatch.setattr(scheduled_notifications, "get_redis", lambda: fake)
    monkeypatch.setattr(scheduled_notifications, "_quiet_hours_active", lambda now_local: False)

    db = SessionLocal()
    try:
        scheduled_notifications.schedule_notification(
            db,
            user_id="notify-absent-user",
            notification_type="morning_briefing",
            payload={"calendar_events": [], "tasks": []},
            scheduled_for=datetime.now(timezone.utc),
            channel="web",
            timezone_name="UTC",
        )
        result = scheduled_notifications.run_due_scheduled_notifications(db)
        assert int(result.get("paused_inactive") or 0) == 1
        assert int(result.get("sent") or 0) == 0

        status = db.execute(
            text("select status from scheduled_notifications where user_id = :user_id order by created_at desc limit 1"),
            {"user_id": "notify-absent-user"},
        ).scalar()
        assert str(status) == "paused"
    finally:
        db.close()


def test_scheduled_notifications_use_active_channel_and_mcp_payload(monkeypatch):
    fake = _FakeRedis()
    fake.set("bp:v1:active-channel:notify-mcp-user", "whatsapp")
    monkeypatch.setattr(scheduled_notifications, "get_redis", lambda: fake)
    monkeypatch.setattr(scheduled_notifications, "_quiet_hours_active", lambda now_local: False)
    monkeypatch.setattr(scheduled_notifications, "_inactivity_days", lambda db, user_id, now_utc: 0.0)

    db = SessionLocal()
    try:
        scheduled_notifications.schedule_notification(
            db,
            user_id="notify-mcp-user",
            notification_type="morning_briefing",
            payload={
                "calendar_events": [
                    {"title": "MCP Calendar Sync", "starts_at": "2026-02-19T09:00:00Z", "source": "mcp_result"}
                ],
                "tasks": [{"title": "Review MCP rollout"}],
            },
            scheduled_for=datetime.now(timezone.utc),
            channel="web",
            timezone_name="UTC",
        )
        result = scheduled_notifications.run_due_scheduled_notifications(db)
        assert int(result.get("sent") or 0) == 1

        row = db.execute(
            text(
                "select channel, body from outbound_messages "
                "where user_id = :user_id order by created_at desc limit 1"
            ),
            {"user_id": "notify-mcp-user"},
        ).mappings().first()
        assert row is not None
        assert str(row.get("channel") or "") == "whatsapp"
        body = str(row.get("body") or "")
        assert "calendar item" in body.lower()
    finally:
        db.close()


def test_scheduled_notifications_plan_gate_travel_financial(monkeypatch):
    fake = _FakeRedis()
    monkeypatch.setattr(scheduled_notifications, "get_redis", lambda: fake)
    monkeypatch.setattr(scheduled_notifications, "_quiet_hours_active", lambda now_local: False)
    monkeypatch.setattr(scheduled_notifications, "_inactivity_days", lambda db, user_id, now_utc: 0.0)

    db = SessionLocal()
    try:
        sub = db.query(Subscription).filter(Subscription.user_id == "notify-plan-user").first()
        if sub is None:
            sub = Subscription(user_id="notify-plan-user", plan="free", status="active")
            db.add(sub)
        else:
            sub.plan = "free"
            sub.status = "active"
        db.commit()

        scheduled_notifications.schedule_notification(
            db,
            user_id="notify-plan-user",
            notification_type="travel_alert",
            payload={"flight": "AA100", "event": "flight_delay"},
            scheduled_for=datetime.now(timezone.utc),
            channel="web",
            timezone_name="UTC",
        )
        result = scheduled_notifications.run_due_scheduled_notifications(db)
        assert int(result.get("skipped_plan_gating") or 0) == 1
        assert int(result.get("sent") or 0) == 0
    finally:
        db.close()


def test_scheduled_notifications_send_financial_alert_for_professional(monkeypatch):
    fake = _FakeRedis()
    monkeypatch.setattr(scheduled_notifications, "get_redis", lambda: fake)
    monkeypatch.setattr(scheduled_notifications, "_quiet_hours_active", lambda now_local: False)
    monkeypatch.setattr(scheduled_notifications, "_inactivity_days", lambda db, user_id, now_utc: 0.0)

    db = SessionLocal()
    try:
        sub = db.query(Subscription).filter(Subscription.user_id == "notify-pro-user").first()
        if sub is None:
            sub = Subscription(user_id="notify-pro-user", plan="professional", status="active")
            db.add(sub)
        else:
            sub.plan = "professional"
            sub.status = "active"
        db.commit()

        scheduled_notifications.schedule_notification(
            db,
            user_id="notify-pro-user",
            notification_type="financial_alert",
            payload={"merchant": "Acme", "amount": "199.99", "currency": "USD", "reason": "Unusual transaction"},
            scheduled_for=datetime.now(timezone.utc),
            channel="web",
            timezone_name="UTC",
        )
        result = scheduled_notifications.run_due_scheduled_notifications(db)
        assert int(result.get("sent") or 0) == 1

        row = db.execute(
            text("select body from outbound_messages where user_id = :user_id order by created_at desc limit 1"),
            {"user_id": "notify-pro-user"},
        ).mappings().first()
        assert row is not None
        assert "financial alert" in str(row.get("body") or "").lower()
    finally:
        db.close()
