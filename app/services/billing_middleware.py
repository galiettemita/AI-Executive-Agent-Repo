from __future__ import annotations

import json
import time
import uuid
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Any, Optional

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.core.config import settings
from app.core.redis import get_redis
from app.db.models import Subscription, User


SUBSCRIPTION_CACHE_TTL_SECONDS = 60


@dataclass(frozen=True)
class BillingBlock:
    reason: str
    message: str
    http_status: int = 402
    retry_after_seconds: int | None = None


@dataclass(frozen=True)
class BillingDecision:
    allowed: bool
    plan: str
    status: str
    daily_count: int | None = None
    monthly_mcp_cost_cents: int | None = None
    monthly_mcp_limit_cents: int | None = None
    block: BillingBlock | None = None


def _now_utc() -> datetime:
    return datetime.now(timezone.utc)


def _ensure_user_row(db: Session, user_id: str) -> None:
    user = db.get(User, user_id)
    if user is not None:
        return
    db.add(User(id=user_id))
    db.commit()


def _get_or_create_subscription(db: Session, user_id: str) -> Subscription:
    sub = db.query(Subscription).filter(Subscription.user_id == user_id).first()
    if sub is not None:
        return sub

    _ensure_user_row(db, user_id)

    now = _now_utc()
    trial_end = now + timedelta(days=int(settings.BILLING_TRIAL_DAYS))
    sub = Subscription(
        user_id=user_id,
        plan="free_trial",
        status="trialing",
        provider=None,
        provider_customer_id=None,
        provider_subscription_id=None,
        current_period_end=trial_end.replace(tzinfo=None),
        updated_at=now.replace(tzinfo=None),
    )
    db.add(sub)
    db.commit()
    return sub


def _subscription_cache_key(user_id: str) -> str:
    return f"billing:sub:{user_id}"


def _load_subscription_cached(db: Session, user_id: str) -> Subscription:
    r = get_redis()
    if r is not None:
        cached = r.get(_subscription_cache_key(user_id))
        if cached:
            try:
                data = json.loads(cached)
                sub = _get_or_create_subscription(db, user_id)
                # Keep object in sync with cached fields.
                sub.plan = str(data.get("plan") or sub.plan or "")
                sub.status = str(data.get("status") or sub.status or "")
                sub.provider = data.get("provider") or sub.provider
                sub.provider_customer_id = data.get("provider_customer_id") or sub.provider_customer_id
                sub.provider_subscription_id = data.get("provider_subscription_id") or sub.provider_subscription_id
                # current_period_end is persisted on the model; cache is best-effort.
                return sub
            except Exception:
                pass

    sub = _get_or_create_subscription(db, user_id)
    if r is not None:
        try:
            r.set(
                _subscription_cache_key(user_id),
                json.dumps(
                    {
                        "plan": sub.plan,
                        "status": sub.status,
                        "provider": sub.provider,
                        "provider_customer_id": sub.provider_customer_id,
                        "provider_subscription_id": sub.provider_subscription_id,
                    },
                    ensure_ascii=False,
                ),
                ex=SUBSCRIPTION_CACHE_TTL_SECONDS,
            )
        except Exception:
            pass
    return sub


def _seconds_until_midnight_utc(now: datetime | None = None) -> int:
    t = now or _now_utc()
    tomorrow = (t + timedelta(days=1)).date()
    midnight = datetime(tomorrow.year, tomorrow.month, tomorrow.day, tzinfo=timezone.utc)
    seconds = int((midnight - t).total_seconds())
    return max(1, seconds)


def _allow_burst(user_id: str) -> tuple[bool, int | None]:
    """
    Sliding-window burst limiter: at most N messages per 60s.
    Returns (allowed, retry_after_seconds).
    """
    r = get_redis()
    if r is None:
        return True, None

    limit = int(settings.BILLING_BURST_LIMIT_PER_MINUTE)
    window_s = int(settings.BILLING_BURST_WINDOW_SECONDS)
    key = f"billing:burst:{user_id}"
    now = time.time()
    cutoff = now - window_s

    member = f"{now}:{uuid.uuid4()}"
    try:
        pipe = r.pipeline()
        pipe.zremrangebyscore(key, 0, cutoff)
        pipe.zadd(key, {member: now})
        pipe.zcard(key)
        pipe.expire(key, window_s + 5)
        _, _, count, _ = pipe.execute()
        if int(count) <= limit:
            return True, None
        # Best-effort retry estimate: ask the oldest timestamp remaining.
        oldest = r.zrange(key, 0, 0, withscores=True)
        if oldest and oldest[0] and len(oldest[0]) == 2:
            retry_after = int(max(1.0, (oldest[0][1] + window_s) - now))
            return False, retry_after
        return False, window_s
    except Exception:
        # If Redis is flaky, prefer availability over strict limiting.
        return True, None


def _incr_daily_count(user_id: str, *, now: datetime | None = None) -> int | None:
    r = get_redis()
    if r is None:
        return None
    key = f"billing:daily:{user_id}"
    try:
        pipe = r.pipeline()
        pipe.incr(key)
        pipe.ttl(key)
        count, ttl = pipe.execute()
        ttl_i = int(ttl)
        if ttl_i < 0:
            r.expire(key, _seconds_until_midnight_utc(now=now))
        return int(count)
    except Exception:
        return None


def _month_bounds_utc(now: datetime | None = None) -> tuple[datetime, datetime]:
    t = now or _now_utc()
    start = datetime(t.year, t.month, 1, tzinfo=timezone.utc)
    if t.month == 12:
        end = datetime(t.year + 1, 1, 1, tzinfo=timezone.utc)
    else:
        end = datetime(t.year, t.month + 1, 1, tzinfo=timezone.utc)
    return start, end


def _mcp_monthly_cost_cache_key(user_id: str, month_key: str) -> str:
    return f"billing:mcp:monthly:{month_key}:{user_id}"


def _query_monthly_mcp_cost_cents(
    db: Session,
    *,
    user_id: str,
    month_start: datetime,
    month_end: datetime,
) -> int:
    dialect = db.bind.dialect.name if db.bind is not None else ""

    try:
        if dialect == "sqlite":
            row = db.execute(
                text(
                    """
                    select coalesce(sum(cost_cents), 0) as total
                    from tool_executions
                    where user_id = :user_id
                      and is_mcp = 1
                      and created_at >= :start
                      and created_at < :end
                    """
                ),
                {
                    "user_id": user_id,
                    "start": month_start.replace(tzinfo=None).isoformat(sep=" "),
                    "end": month_end.replace(tzinfo=None).isoformat(sep=" "),
                },
            ).mappings().first()
            return int(float((row or {}).get("total") or 0))

        row = db.execute(
            text(
                """
                select coalesce(sum(cost_cents), 0) as total
                from tool_executions
                where user_id::text = :user_id
                  and coalesce(is_mcp, false) = true
                  and created_at >= :start
                  and created_at < :end
                """
            ),
            {
                "user_id": user_id,
                "start": month_start.replace(tzinfo=None),
                "end": month_end.replace(tzinfo=None),
            },
        ).mappings().first()
        return int(float((row or {}).get("total") or 0))
    except Exception:
        # If MCP tables are absent (legacy DB) or query fails, don't block user traffic.
        return 0


def _load_monthly_mcp_cost_cents(db: Session, *, user_id: str, now: datetime | None = None) -> int:
    month_start, month_end = _month_bounds_utc(now)
    month_key = month_start.strftime("%Y-%m")
    cache_key = _mcp_monthly_cost_cache_key(user_id, month_key)
    r = get_redis()

    if r is not None:
        try:
            cached = r.get(cache_key)
            if cached is not None:
                return int(float(cached))
        except Exception:
            pass

    total = _query_monthly_mcp_cost_cents(
        db,
        user_id=user_id,
        month_start=month_start,
        month_end=month_end,
    )

    if r is not None:
        try:
            # Short cache to keep first-gate latency low while adapting to fresh spend quickly.
            r.set(cache_key, str(total), ex=60)
        except Exception:
            pass
    return total


def _resolve_mcp_monthly_budget_cents(*, plan: str, status: str) -> int:
    p = (plan or "").strip().lower()
    s = (status or "").strip().lower()
    if p in {"free_trial", "trial"} or s == "trialing":
        return int(settings.BILLING_MCP_MONTHLY_LIMIT_CENTS_TRIAL)
    if p == "professional":
        return int(settings.BILLING_MCP_MONTHLY_LIMIT_CENTS_PROFESSIONAL)
    if p in {"enterprise", "business", "team"}:
        return int(settings.BILLING_MCP_MONTHLY_LIMIT_CENTS_ENTERPRISE)
    return int(settings.BILLING_MCP_MONTHLY_LIMIT_CENTS_PERSONAL)


def enforce_billing_for_inbound_message(db: Session, user_id: str) -> BillingDecision:
    """
    Enforce subscription state + burst limiting + daily caps.

    This function is designed to run before any LLM/tool work.
    """
    sub = _load_subscription_cached(db, user_id)
    plan = (sub.plan or "free_trial").strip().lower()
    status = (sub.status or "active").strip().lower()
    now = _now_utc()

    # Subscription state gate
    if status in {"canceled", "cancelled", "inactive", "paused", "unpaid"}:
        return BillingDecision(
            allowed=False,
            plan=plan,
            status=status,
            block=BillingBlock(
                reason="subscription_inactive",
                message="Your subscription is inactive. Please update billing to continue.",
                http_status=402,
            ),
        )

    # Past-due grace period gate
    if status == "past_due":
        # We treat Subscription.updated_at as "last status update" (good enough for v1).
        updated = sub.updated_at.replace(tzinfo=timezone.utc) if sub.updated_at else now
        grace_days = int(settings.BILLING_PAST_DUE_GRACE_DAYS)
        if (now - updated) > timedelta(days=grace_days):
            return BillingDecision(
                allowed=False,
                plan=plan,
                status=status,
                block=BillingBlock(
                    reason="past_due_grace_exceeded",
                    message="Your payment is past due. Please update your payment method to continue.",
                    http_status=402,
                ),
            )

    # Free trial expiry gate
    if plan in {"free_trial", "trial", "trialing"} or status == "trialing":
        trial_end = sub.current_period_end.replace(tzinfo=timezone.utc) if sub.current_period_end else None
        if trial_end is not None and now > trial_end:
            return BillingDecision(
                allowed=False,
                plan=plan,
                status="trial_expired",
                block=BillingBlock(
                    reason="trial_expired",
                    message="Your free trial has ended. Upgrade to keep using your assistant.",
                    http_status=402,
                ),
            )

    # Monthly MCP spend cap gate (plan-based).
    monthly_mcp_limit = _resolve_mcp_monthly_budget_cents(plan=plan, status=status)
    monthly_mcp_cost = _load_monthly_mcp_cost_cents(db, user_id=user_id, now=now)
    if monthly_mcp_limit > 0 and monthly_mcp_cost >= monthly_mcp_limit:
        return BillingDecision(
            allowed=False,
            plan=plan,
            status=status,
            monthly_mcp_cost_cents=monthly_mcp_cost,
            monthly_mcp_limit_cents=monthly_mcp_limit,
            block=BillingBlock(
                reason="mcp_monthly_budget_exceeded",
                message="You've reached your monthly MCP usage budget for this plan. Upgrade to continue MCP usage.",
                http_status=402,
            ),
        )

    # Burst limiter
    burst_ok, retry_after = _allow_burst(user_id)
    if not burst_ok:
        return BillingDecision(
            allowed=False,
            plan=plan,
            status=status,
            block=BillingBlock(
                reason="rate_limited",
                message="You’re sending messages too quickly. Please wait a moment and try again.",
                http_status=429,
                retry_after_seconds=retry_after,
            ),
        )

    # Daily message caps
    daily_limit: int | None = None
    if plan in {"free_trial", "trial"} or status == "trialing":
        daily_limit = int(settings.BILLING_TRIAL_DAILY_MESSAGE_LIMIT)

    daily_count = None
    if daily_limit is not None:
        daily_count = _incr_daily_count(user_id, now=now)
        if daily_count is not None and daily_count > daily_limit:
            return BillingDecision(
                allowed=False,
                plan=plan,
                status=status,
                daily_count=daily_count,
                block=BillingBlock(
                    reason="daily_limit_exceeded",
                    message="You’ve hit today’s message limit for your trial. Upgrade to keep going.",
                    http_status=402,
                ),
            )

    return BillingDecision(
        allowed=True,
        plan=plan,
        status=status,
        daily_count=daily_count,
        monthly_mcp_cost_cents=monthly_mcp_cost,
        monthly_mcp_limit_cents=monthly_mcp_limit if monthly_mcp_limit > 0 else None,
        block=None,
    )


def get_billing_subscription(db: Session, user_id: str) -> Subscription:
    """
    Returns a subscription record, auto-creating the free trial if missing.
    Does not increment any usage counters.
    """
    return _load_subscription_cached(db, user_id)


def count_connected_mcp_servers(db: Session, user_id: str) -> int:
    """
    Counts enabled MCP servers for a user. Works across SQLite dev and Postgres.
    """
    dialect = db.bind.dialect.name if db.bind is not None else ""
    if dialect == "sqlite":
        row = db.execute(
            text("select count(1) as cnt from mcp_user_servers where user_id = :user_id and is_enabled = 1"),
            {"user_id": user_id},
        ).mappings().first()
        return int((row or {}).get("cnt") or 0)

    # Postgres schema may store UUID user_id.
    row = db.execute(
        text("select count(1) as cnt from mcp_user_servers where user_id::text = :user_id and is_enabled = true"),
        {"user_id": user_id},
    ).mappings().first()
    return int((row or {}).get("cnt") or 0)
