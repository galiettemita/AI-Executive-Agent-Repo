from __future__ import annotations

import uuid
from datetime import datetime, timedelta

from sqlalchemy import text

from app.core.config import settings
from app.db.database import SessionLocal
from app.db.models import Subscription, User
from app.services.billing_middleware import enforce_billing_for_inbound_message, enforce_billing_for_tool_call


class _NoIncrRedis:
    def __init__(self) -> None:
        self.values: dict[str, str] = {}

    def get(self, key: str):
        return self.values.get(key)

    def set(self, key: str, value: str, ex: int | None = None):
        self.values[key] = str(value)
        return True

    def incr(self, key: str):  # pragma: no cover - should never be called in tool-call billing test
        raise AssertionError("daily counter increment should not run for provision_server tool calls")

    def ttl(self, key: str):
        return -1

    def expire(self, key: str, seconds: int):
        return True

    def pipeline(self):
        return self

    def execute(self):
        return []

    def zremrangebyscore(self, *args, **kwargs):
        return 0

    def zadd(self, *args, **kwargs):
        return 1

    def zcard(self, *args, **kwargs):
        return 0

    def zrange(self, *args, **kwargs):
        return []


def _ensure_tool_executions_table(db) -> None:
    db.execute(
        text(
            """
            create table if not exists tool_executions (
              id text primary key,
              user_id text not null,
              is_mcp integer default 0,
              cost_cents integer default 0,
              created_at text
            )
            """
        )
    )
    db.commit()


def _insert_tool_execution(db, *, user_id: str, is_mcp: bool, cost_cents: int) -> None:
    db.execute(
        text(
            """
            insert into tool_executions (id, user_id, is_mcp, cost_cents, created_at)
            values (:id, :user_id, :is_mcp, :cost_cents, :created_at)
            """
        ),
        {
            "id": str(uuid.uuid4()),
            "user_id": user_id,
            "is_mcp": 1 if is_mcp else 0,
            "cost_cents": int(cost_cents),
            "created_at": datetime.utcnow().isoformat(sep=" "),
        },
    )
    db.commit()


def test_billing_middleware_blocks_when_mcp_budget_exceeded(monkeypatch) -> None:
    user_id = f"billing-mcp-block-{uuid.uuid4()}"
    monkeypatch.setattr(settings, "BILLING_MCP_MONTHLY_LIMIT_CENTS_TRIAL", 50)

    db = SessionLocal()
    try:
        _ensure_tool_executions_table(db)
        db.add(User(id=user_id))
        db.add(
            Subscription(
                user_id=user_id,
                plan="free_trial",
                status="trialing",
                current_period_end=datetime.utcnow() + timedelta(days=1),
                updated_at=datetime.utcnow(),
            )
        )
        db.commit()

        _insert_tool_execution(db, user_id=user_id, is_mcp=True, cost_cents=75)

        decision = enforce_billing_for_inbound_message(db, user_id)
        assert decision.allowed is False
        assert decision.block is not None
        assert decision.block.reason == "mcp_monthly_budget_exceeded"
        assert decision.monthly_mcp_cost_cents == 75
        assert decision.monthly_mcp_limit_cents == 50
    finally:
        db.close()


def test_billing_middleware_ignores_non_mcp_cost_for_budget(monkeypatch) -> None:
    user_id = f"billing-mcp-allow-{uuid.uuid4()}"
    monkeypatch.setattr(settings, "BILLING_MCP_MONTHLY_LIMIT_CENTS_TRIAL", 500)

    db = SessionLocal()
    try:
        _ensure_tool_executions_table(db)
        db.add(User(id=user_id))
        db.add(
            Subscription(
                user_id=user_id,
                plan="free_trial",
                status="trialing",
                current_period_end=datetime.utcnow() + timedelta(days=1),
                updated_at=datetime.utcnow(),
            )
        )
        db.commit()

        _insert_tool_execution(db, user_id=user_id, is_mcp=False, cost_cents=2_000)
        _insert_tool_execution(db, user_id=user_id, is_mcp=True, cost_cents=120)

        decision = enforce_billing_for_inbound_message(db, user_id)
        assert decision.allowed is True
        assert decision.monthly_mcp_cost_cents == 120
        assert decision.monthly_mcp_limit_cents == 500
    finally:
        db.close()


def test_billing_tool_call_allows_provision_server_without_daily_counter(monkeypatch) -> None:
    user_id = f"billing-tool-{uuid.uuid4()}"
    fake = _NoIncrRedis()
    monkeypatch.setattr("app.services.billing_middleware.get_redis", lambda: fake)

    db = SessionLocal()
    try:
        db.add(User(id=user_id))
        db.add(
            Subscription(
                user_id=user_id,
                plan="free_trial",
                status="trialing",
                current_period_end=datetime.utcnow() + timedelta(days=1),
                updated_at=datetime.utcnow(),
            )
        )
        db.commit()

        decision = enforce_billing_for_tool_call(db, user_id, tool_name="provision_server")
        assert decision.allowed is True
        assert decision.daily_count is None
    finally:
        db.close()


def test_billing_tool_call_blocks_inactive_subscription() -> None:
    user_id = f"billing-tool-inactive-{uuid.uuid4()}"
    db = SessionLocal()
    try:
        db.add(User(id=user_id))
        db.add(
            Subscription(
                user_id=user_id,
                plan="personal",
                status="inactive",
                current_period_end=datetime.utcnow() + timedelta(days=30),
                updated_at=datetime.utcnow(),
            )
        )
        db.commit()

        decision = enforce_billing_for_tool_call(db, user_id, tool_name="provision_server")
        assert decision.allowed is False
        assert decision.block is not None
        assert decision.block.reason == "subscription_inactive"
    finally:
        db.close()
