# backend/app/services/subscriptions.py

from __future__ import annotations

from datetime import datetime
from typing import Dict

from sqlalchemy.orm import Session

from app.db.models import Subscription
from app.core.config import settings
from app.core.redis import cache_get_json, cache_set_json, cache_delete


PREMIUM_PLANS = {"starter", "plus", "pro"}

PLAN_LIMITS = {
    "free": {"messages": 20, "tokens": 2000, "proposals": 0},
    "starter": {"messages": 300, "tokens": 30000, "proposals": 20},
    "plus": {"messages": 1000, "tokens": 100000, "proposals": 100},
    "pro": {"messages": 5000, "tokens": 500000, "proposals": 1000},
}


def is_premium_user(entitlements: Dict[str, str]) -> bool:
    plan = (entitlements.get("plan") or "free").lower()
    status = (entitlements.get("status") or "active").lower()
    return plan in PREMIUM_PLANS and status == "active"


def upgrade_prompt(user_id: str) -> str:
    return (
        "This feature is part of the Starter plan. "
        "Upgrade here: "
        f"https://ai-shopping-assistant-backend-6bgf.onrender.com/billing/stripe/checkout?user_id={user_id}"
    )


def limit_prompt(user_id: str) -> str:
    return (
        "You’ve hit your monthly limit. "
        "Upgrade to keep going: "
        f"https://ai-shopping-assistant-backend-6bgf.onrender.com/billing/stripe/checkout?user_id={user_id}"
    )


def get_plan_limits(entitlements: Dict[str, str]) -> Dict[str, int]:
    plan = (entitlements.get("plan") or "free").lower()
    return PLAN_LIMITS.get(plan, PLAN_LIMITS["free"])


def get_subscription(db: Session, user_id: str) -> Subscription | None:
    return db.query(Subscription).filter(Subscription.user_id == user_id).first()


def get_entitlements(db: Session, user_id: str) -> Dict[str, str]:
    """
    Minimal entitlements stub. Defaults to free if no record exists.
    """
    cache_key = f"entitlements:{user_id}"
    cached = cache_get_json(cache_key)
    if isinstance(cached, dict):
        return cached

    sub = get_subscription(db, user_id)
    if not sub:
        entitlements = {"plan": "free", "status": "active"}
        cache_set_json(cache_key, entitlements, ttl_seconds=settings.REDIS_ENTITLEMENTS_TTL_SECONDS)
        return entitlements
    entitlements = {
        "plan": sub.plan or "free",
        "status": sub.status or "active",
    }
    cache_set_json(cache_key, entitlements, ttl_seconds=settings.REDIS_ENTITLEMENTS_TTL_SECONDS)
    return entitlements


def upsert_subscription(
    db: Session,
    user_id: str,
    *,
    plan: str,
    status: str,
    provider: str | None = None,
    provider_customer_id: str | None = None,
    provider_subscription_id: str | None = None,
    current_period_end: datetime | None = None,
) -> None:
    sub = get_subscription(db, user_id)
    if not sub:
        sub = Subscription(
            user_id=user_id,
            plan=plan,
            status=status,
            provider=provider,
            provider_customer_id=provider_customer_id,
            provider_subscription_id=provider_subscription_id,
            current_period_end=current_period_end,
        )
        db.add(sub)
    else:
        sub.plan = plan
        sub.status = status
        sub.provider = provider
        sub.provider_customer_id = provider_customer_id
        sub.provider_subscription_id = provider_subscription_id
        sub.current_period_end = current_period_end
    db.commit()
    cache_delete(f"entitlements:{user_id}")
